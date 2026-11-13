// Package server exposes the JSON API, the file streaming endpoint, cached
// images and the embedded React app.
package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cinema/internal/images"
	"cinema/internal/scanner"
	"cinema/internal/subtitles"
	"cinema/internal/transcode"
)

type Server struct {
	db      *sql.DB
	scanner *scanner.Scanner
	images  *images.Cache
	web     fs.FS
	Media
	// Admin enables the settings and library folder endpoints; set it
	// before calling Handler.
	Admin Admin
}

// Media holds the parts that need ffmpeg. They're all nil when it isn't
// available: files are then served as-is, without subtitles.
type Media struct {
	Store     *transcode.Store
	HLS       *transcode.HLS
	Subtitles *subtitles.Service
}

func New(db *sql.DB, sc *scanner.Scanner, img *images.Cache, web fs.FS, media Media) *Server {
	return &Server{db: db, scanner: sc, images: img, web: web, Media: media}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library", s.library)
	mux.HandleFunc("GET /api/movies/{id}", s.movie)
	mux.HandleFunc("GET /api/shows/{id}", s.show)
	mux.HandleFunc("GET /api/episodes/{id}", s.episode)
	mux.HandleFunc("GET /api/stream/{id}", s.stream)
	mux.HandleFunc("GET /api/playback/{id}", s.playback)
	mux.HandleFunc("GET /api/hls/{id}/{session}/index.m3u8", s.hlsPlaylist)
	mux.HandleFunc("GET /api/hls/{id}/{session}/{segment}", s.hlsSegment)
	mux.HandleFunc("GET /api/subtitles/{id}/{track}", s.subtitle)
	mux.HandleFunc("GET /api/scan", s.scanStatus)
	mux.HandleFunc("POST /api/scan", s.startScan)
	mux.HandleFunc("GET /api/home", s.home)
	mux.HandleFunc("POST /api/progress", s.saveProgress)
	s.adminRoutes(mux)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	mux.HandleFunc("GET /images/{size}/{file}", s.images.Handler)
	mux.HandleFunc("/", s.spa)
	return logRequests(mux)
}

// ---------------------------------------------------------------- library

const posterSize = "w500"
const stillSize = "w300"

func imageURL(size string, p sql.NullString) *string {
	if !p.Valid || p.String == "" {
		return nil
	}
	u := "/images/" + size + p.String
	return &u
}

func nullStr(n sql.NullString) *string {
	if !n.Valid || n.String == "" {
		return nil
	}
	return &n.String
}

func year(date sql.NullString) *int {
	if !date.Valid || len(date.String) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date.String[:4])
	if err != nil {
		return nil
	}
	return &y
}

type libraryItem struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	Year      *int    `json:"year"`
	PosterURL *string `json:"posterUrl"`
	// When its most recent file was added to the library.
	AddedAt *string `json:"addedAt"`
}

func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	query := func(q string) ([]libraryItem, error) {
		rows, err := s.db.QueryContext(r.Context(), q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []libraryItem{}
		for rows.Next() {
			var it libraryItem
			var date, poster, added sql.NullString
			if err := rows.Scan(&it.ID, &it.Title, &date, &poster, &added); err != nil {
				return nil, err
			}
			it.Year, it.PosterURL, it.AddedAt = year(date), imageURL(posterSize, poster), nullStr(added)
			items = append(items, it)
		}
		return items, rows.Err()
	}
	movies, err := query(`SELECT m.id, m.title, m.release_date, m.poster_path, MAX(f.added_at) FROM movies m
		JOIN files f ON f.movie_id = m.id
		GROUP BY m.id
		ORDER BY m.title COLLATE NOCASE`)
	if err != nil {
		serverError(w, err)
		return
	}
	shows, err := query(`SELECT s.id, s.name, s.first_air_date, s.poster_path, MAX(f.added_at) FROM tv_shows s
		JOIN episodes e ON e.show_id = s.id
		JOIN files f ON f.episode_id = e.id
		GROUP BY s.id
		ORDER BY s.name COLLATE NOCASE`)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]any{"movies": movies, "shows": shows})
}

func (s *Server) movie(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var out struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseDate *string `json:"releaseDate"`
		Overview    *string `json:"overview"`
		PosterURL   *string `json:"posterUrl"`
		FileID      int64   `json:"fileId"`
		// Where to resume playback (seconds), or null to start over.
		ResumeAt *float64 `json:"resumeAt"`
	}
	var date, overview, poster sql.NullString
	var fileID sql.NullInt64
	err := s.db.QueryRowContext(r.Context(), `SELECT m.id, m.title, m.release_date, m.overview, m.poster_path,
			(SELECT MIN(f.id) FROM files f WHERE f.movie_id = m.id)
		FROM movies m WHERE m.id = ?`, id).Scan(&out.ID, &out.Title, &date, &overview, &poster, &fileID)
	if notFound(w, err) {
		return
	}
	if !fileID.Valid {
		writeError(w, http.StatusNotFound, "no file for movie")
		return
	}
	out.FileID = fileID.Int64
	out.ReleaseDate, out.Overview, out.PosterURL = nullStr(date), nullStr(overview), imageURL(posterSize, poster)
	if out.ResumeAt, err = s.resumeAt(r.Context(), "movie", id); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, out)
}

type episodeItem struct {
	ID            int     `json:"id"`
	EpisodeNumber int     `json:"episodeNumber"`
	Name          *string `json:"name"`
	AirDate       *string `json:"airDate"`
	StillURL      *string `json:"stillUrl"`
	FileID        int64   `json:"fileId"`
}

type seasonItem struct {
	SeasonNumber int           `json:"seasonNumber"`
	Name         *string       `json:"name"`
	PosterURL    *string       `json:"posterUrl"`
	Episodes     []episodeItem `json:"episodes"`
}

func (s *Server) show(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var out struct {
		ID           int          `json:"id"`
		Name         string       `json:"name"`
		FirstAirDate *string      `json:"firstAirDate"`
		Overview     *string      `json:"overview"`
		PosterURL    *string      `json:"posterUrl"`
		Seasons      []seasonItem `json:"seasons"`
	}
	var date, overview, poster sql.NullString
	err := s.db.QueryRowContext(r.Context(), `SELECT id, name, first_air_date, overview, poster_path FROM tv_shows WHERE id = ?`, id).
		Scan(&out.ID, &out.Name, &date, &overview, &poster)
	if notFound(w, err) {
		return
	}
	out.FirstAirDate, out.Overview, out.PosterURL = nullStr(date), nullStr(overview), imageURL(posterSize, poster)

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT e.season_number, se.name, se.poster_path,
		       e.id, e.episode_number, e.name, e.air_date, e.still_path, MIN(f.id)
		FROM episodes e
		JOIN files f ON f.episode_id = e.id
		LEFT JOIN seasons se ON se.id = e.season_id
		WHERE e.show_id = ?
		GROUP BY e.id
		ORDER BY e.season_number, e.episode_number`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	out.Seasons = []seasonItem{}
	for rows.Next() {
		var seasonNum int
		var seasonName, seasonPoster, name, air, still sql.NullString
		var ep episodeItem
		if err := rows.Scan(&seasonNum, &seasonName, &seasonPoster, &ep.ID, &ep.EpisodeNumber, &name, &air, &still, &ep.FileID); err != nil {
			serverError(w, err)
			return
		}
		ep.Name, ep.AirDate, ep.StillURL = nullStr(name), nullStr(air), imageURL(stillSize, still)
		if n := len(out.Seasons); n == 0 || out.Seasons[n-1].SeasonNumber != seasonNum {
			out.Seasons = append(out.Seasons, seasonItem{
				SeasonNumber: seasonNum, Name: nullStr(seasonName), PosterURL: imageURL(posterSize, seasonPoster),
			})
		}
		last := &out.Seasons[len(out.Seasons)-1]
		last.Episodes = append(last.Episodes, ep)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) episode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var out struct {
		ID            int     `json:"id"`
		Name          *string `json:"name"`
		SeasonNumber  int     `json:"seasonNumber"`
		EpisodeNumber int     `json:"episodeNumber"`
		AirDate       *string `json:"airDate"`
		Overview      *string `json:"overview"`
		FileID        int64   `json:"fileId"`
		NextEpisodeID *int    `json:"nextEpisodeId"`
		// Where to resume playback (seconds), or null to start over.
		ResumeAt *float64 `json:"resumeAt"`
		Show     struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"show"`
	}
	var name, air, overview sql.NullString
	var fileID sql.NullInt64
	err := s.db.QueryRowContext(r.Context(), `
		SELECT e.id, e.name, e.season_number, e.episode_number, e.air_date, e.overview,
		       (SELECT MIN(f.id) FROM files f WHERE f.episode_id = e.id),
		       s.id, s.name
		FROM episodes e JOIN tv_shows s ON s.id = e.show_id
		WHERE e.id = ?`, id).
		Scan(&out.ID, &name, &out.SeasonNumber, &out.EpisodeNumber, &air, &overview, &fileID, &out.Show.ID, &out.Show.Name)
	if notFound(w, err) {
		return
	}
	if !fileID.Valid {
		writeError(w, http.StatusNotFound, "no file for episode")
		return
	}
	out.FileID = fileID.Int64
	out.Name, out.AirDate, out.Overview = nullStr(name), nullStr(air), nullStr(overview)

	var next int
	err = s.db.QueryRowContext(r.Context(), `
		SELECT e.id FROM episodes e
		WHERE e.show_id = ?
		  AND EXISTS (SELECT 1 FROM files f WHERE f.episode_id = e.id)
		  AND (e.season_number > ? OR (e.season_number = ? AND e.episode_number > ?))
		ORDER BY e.season_number, e.episode_number LIMIT 1`,
		out.Show.ID, out.SeasonNumber, out.SeasonNumber, out.EpisodeNumber).Scan(&next)
	if err == nil {
		out.NextEpisodeID = &next
	} else if !errors.Is(err, sql.ErrNoRows) {
		serverError(w, err)
		return
	}
	if out.ResumeAt, err = s.resumeAt(r.Context(), "episode", id); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, out)
}

// ---------------------------------------------------------------- streaming

var contentTypes = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm",
	".mkv": "video/x-matroska", ".mov": "video/quicktime", ".avi": "video/x-msvideo",
	".ts": "video/mp2t", ".wmv": "video/x-ms-wmv",
}

// stream serves the original file untouched. http.ServeContent handles Range
// requests (206 Partial Content), so the browser can seek.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	p, ok := s.filePath(w, r)
	if !ok {
		return
	}
	f, err := os.Open(p)
	if err != nil {
		writeError(w, http.StatusNotFound, "file missing on disk; rescan the library")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		serverError(w, err)
		return
	}
	if ct, ok := contentTypes[strings.ToLower(filepath.Ext(p))]; ok {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, filepath.Base(p), info.ModTime(), f)
}

// filePath looks up a file's path, writing an error response if it fails.
func (s *Server) filePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return "", false
	}
	var p string
	err := s.db.QueryRowContext(r.Context(), `SELECT path FROM files WHERE id = ?`, id).Scan(&p)
	if notFound(w, err) {
		return "", false
	}
	return p, true
}

// playback tells the player how to play a file: "direct" files are served
// as-is from /api/stream, "hls" ones are transcoded to HLS. Each HLS
// playback gets its own session (and ffmpeg process) on the server.
type subtitleTrack struct {
	subtitles.Track
	URL string `json:"url"`
}

func (s *Server) playback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.filePath(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	out := struct {
		Mode      string          `json:"mode"`
		URL       string          `json:"url"`
		Subtitles []subtitleTrack `json:"subtitles"`
	}{Mode: "direct", URL: "/api/stream/" + id, Subtitles: []subtitleTrack{}}
	if s.HLS != nil && transcode.NeedsTranscode(p) {
		out.Mode, out.URL = "hls", "/api/hls/"+id+"/"+rand.Text()+"/index.m3u8"
	}
	if s.Subtitles != nil {
		// Side files are listed even if the file can't be indexed.
		fileID, _ := strconv.ParseInt(id, 10, 64) // validated by filePath
		_, ix, err := s.Store.Get(r.Context(), fileID)
		if err != nil {
			log.Printf("index %s: %v", p, err)
			ix = nil
		}
		for _, t := range s.Subtitles.Tracks(p, ix) {
			out.Subtitles = append(out.Subtitles, subtitleTrack{t, "/api/subtitles/" + id + "/" + url.PathEscape(t.ID) + ".vtt"})
		}
	}
	writeJSON(w, out)
}

// subtitle serves a subtitle track as WebVTT.
func (s *Server) subtitle(w http.ResponseWriter, r *http.Request) {
	if s.Subtitles == nil {
		writeError(w, http.StatusNotImplemented, "subtitles unavailable: ffmpeg not found")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	track, found := strings.CutSuffix(r.PathValue("track"), ".vtt")
	if !found {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	p, ix, err := s.Store.Get(r.Context(), int64(id))
	if errors.Is(err, transcode.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusNotFound, "file missing on disk; rescan the library")
		return
	} else if err != nil {
		// Side files don't need the index.
		log.Printf("index %d: %v", id, err)
		if err := s.db.QueryRowContext(r.Context(), `SELECT path FROM files WHERE id = ?`, id).Scan(&p); err != nil {
			serverError(w, err)
			return
		}
		ix = nil
	}
	vtt, err := s.Subtitles.WebVTT(r.Context(), int64(id), p, ix, track)
	switch {
	case r.Context().Err() != nil:
		return
	case errors.Is(err, subtitles.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "not found")
		return
	case err != nil:
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(vtt)
}

func (s *Server) hlsPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := s.hlsFileID(w, r)
	if !ok {
		return
	}
	pl, err := s.HLS.Playlist(r.Context(), id)
	if err != nil {
		hlsError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(pl)
}

func (s *Server) hlsSegment(w http.ResponseWriter, r *http.Request) {
	id, ok := s.hlsFileID(w, r)
	if !ok {
		return
	}
	num, found := strings.CutSuffix(r.PathValue("segment"), ".ts")
	n, err := strconv.Atoi(num)
	if !found || err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	p, err := s.HLS.Segment(r.Context(), id, r.PathValue("session"), n)
	if err != nil {
		hlsError(w, r, err)
		return
	}
	// Open right away: the session may delete the file once the player has
	// moved on, but an open file stays readable.
	f, err := os.Open(p)
	if err != nil {
		serverError(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func (s *Server) hlsFileID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if s.HLS == nil {
		writeError(w, http.StatusNotImplemented, "transcoding unavailable: ffmpeg not found")
		return 0, false
	}
	id, ok := pathID(w, r)
	return int64(id), ok
}

func hlsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case r.Context().Err() != nil:
		// The player gave up on this request (e.g. it seeked); nothing to send.
	case errors.Is(err, transcode.ErrNotFound), errors.Is(err, transcode.ErrBadSession), errors.Is(err, transcode.ErrBadSegment):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, transcode.ErrSuperseded):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "file missing on disk; rescan the library")
	default:
		serverError(w, err)
	}
}

// ---------------------------------------------------------------- scanning

func (s *Server) scanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.scanner.Status())
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("refresh") == "true"
	if !s.scanner.Start(refresh) {
		writeError(w, http.StatusConflict, "a scan is already running")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(s.scanner.Status())
}

// ---------------------------------------------------------------- spa

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" {
		if st, err := fs.Stat(s.web, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, s.web, name)
			return
		}
	}
	index, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		http.Error(w, "frontend not built: run `make build`", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(index)
}

// ---------------------------------------------------------------- helpers

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// notFound writes a 404 for sql.ErrNoRows or a 500 for other errors and
// reports whether a response was written.
func notFound(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
	} else {
		serverError(w, err)
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer's Flush.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}
