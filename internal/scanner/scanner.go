// Package scanner walks the media directories, matches files to TMDB titles,
// ingests metadata and links files to movies and episodes.
package scanner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cinema/internal/images"
	"cinema/internal/ingest"
	"cinema/internal/tmdb"
)

const maxStatusMessages = 200

// Status describes the current or most recent scan.
type Status struct {
	Running      bool       `json:"running"`
	Refresh      bool       `json:"refresh"`
	Phase        string     `json:"phase"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	FilesSeen    int        `json:"filesSeen"`
	FilesLinked  int        `json:"filesLinked"`
	FilesIgnored int        `json:"filesIgnored"`
	FilesRemoved int        `json:"filesRemoved"`
	MoviesAdded  int        `json:"moviesAdded"`
	ShowsAdded   int        `json:"showsAdded"`
	Ignored      []string   `json:"ignored"`
	Errors       []string   `json:"errors"`
}

type Scanner struct {
	ctx       context.Context
	db        *sql.DB
	tmdb      *tmdb.Client
	images    *images.Cache
	moviesDir string
	tvDir     string

	mu     sync.Mutex
	status Status
}

func New(ctx context.Context, db *sql.DB, client *tmdb.Client, img *images.Cache, moviesDir, tvDir string) *Scanner {
	return &Scanner{ctx: ctx, db: db, tmdb: client, images: img, moviesDir: moviesDir, tvDir: tvDir}
}

// Status returns a snapshot of the scan status.
func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status
	st.Ignored = append([]string(nil), st.Ignored...)
	st.Errors = append([]string(nil), st.Errors...)
	return st
}

// Start begins a scan in the background. It returns false if a scan is
// already running. With refresh, metadata is re-fetched for known titles.
func (s *Scanner) Start(refresh bool) bool {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return false
	}
	now := time.Now()
	s.status = Status{Running: true, Refresh: refresh, StartedAt: &now, Phase: "starting"}
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.addError(fmt.Sprintf("scan panicked: %v", r))
			}
			s.update(func(st *Status) {
				done := time.Now()
				st.Running, st.FinishedAt, st.Phase = false, &done, "done"
			})
			st := s.Status()
			log.Printf("scan finished: %d seen, %d linked, %d ignored, %d removed, %d errors",
				st.FilesSeen, st.FilesLinked, st.FilesIgnored, st.FilesRemoved, len(st.Errors))
		}()
		s.run(s.ctx, refresh)
	}()
	return true
}

// Run performs a scan synchronously (used by tests).
func (s *Scanner) Run(ctx context.Context, refresh bool) Status {
	now := time.Now()
	s.mu.Lock()
	s.status = Status{Running: true, Refresh: refresh, StartedAt: &now}
	s.mu.Unlock()
	s.run(ctx, refresh)
	s.update(func(st *Status) { done := time.Now(); st.Running, st.FinishedAt, st.Phase = false, &done, "done" })
	return s.Status()
}

func (s *Scanner) update(fn func(*Status)) {
	s.mu.Lock()
	fn(&s.status)
	s.mu.Unlock()
}

func (s *Scanner) addError(msg string) {
	log.Print("scan error: ", msg)
	s.update(func(st *Status) {
		if len(st.Errors) < maxStatusMessages {
			st.Errors = append(st.Errors, msg)
		}
	})
}

func (s *Scanner) ignore(path, reason string) {
	log.Printf("scan: ignoring %s: %s", path, reason)
	s.update(func(st *Status) {
		st.FilesIgnored++
		if len(st.Ignored) < maxStatusMessages {
			st.Ignored = append(st.Ignored, path+": "+reason)
		}
	})
}

// ---------------------------------------------------------------- scan

type diskFile struct {
	abs   string
	rel   string
	size  int64
	mtime int64
}

type fileRow struct {
	id        int64
	size      int64
	mtime     int64
	movieID   sql.NullInt64
	episodeID sql.NullInt64
}

func (s *Scanner) run(ctx context.Context, refresh bool) {
	s.update(func(st *Status) { st.Phase = "reference data" })
	s.fetchReference(ctx)

	known, err := s.loadFiles(ctx)
	if err != nil {
		s.addError("load files: " + err.Error())
		return
	}

	s.update(func(st *Status) { st.Phase = "movies" })
	if files, err := walk(s.moviesDir); err != nil {
		s.addError(fmt.Sprintf("walk movies dir: %v (skipping movie library)", err))
	} else {
		seen := s.scanMovies(ctx, files, known, refresh)
		s.removeMissing(ctx, "movie", seen)
	}

	s.update(func(st *Status) { st.Phase = "tv" })
	if files, err := walk(s.tvDir); err != nil {
		s.addError(fmt.Sprintf("walk tv dir: %v (skipping tv library)", err))
	} else {
		seen := s.scanTV(ctx, files, known, refresh)
		s.removeMissing(ctx, "tv", seen)
	}
}

func (s *Scanner) fetchReference(ctx context.Context) {
	for _, key := range []string{"configuration", "genre/movie/list", "genre/tv/list"} {
		raw, err := s.tmdb.Get(ctx, "/"+key, nil)
		if err != nil {
			s.addError(fmt.Sprintf("fetch %s: %v", key, err))
			continue
		}
		if err := ingest.Config(ctx, s.db, key, raw); err != nil {
			s.addError(fmt.Sprintf("store %s: %v", key, err))
		}
	}
}

// walk returns every main-content video file under root.
func walk(root string) ([]diskFile, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var out []diskFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			log.Printf("scan: walk %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsVideo(path) || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || IsExtra(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		abs, _ := filepath.Abs(path)
		out = append(out, diskFile{abs: abs, rel: rel, size: info.Size(), mtime: info.ModTime().Unix()})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, err
}

func (s *Scanner) loadFiles(ctx context.Context) (map[string]fileRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, path, size, mtime, movie_id, episode_id FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]fileRow{}
	for rows.Next() {
		var r fileRow
		var path string
		if err := rows.Scan(&r.id, &path, &r.size, &r.mtime, &r.movieID, &r.episodeID); err != nil {
			return nil, err
		}
		out[path] = r
	}
	return out, rows.Err()
}

func unchanged(f diskFile, known map[string]fileRow, linked func(fileRow) bool) bool {
	r, ok := known[f.abs]
	return ok && r.size == f.size && r.mtime == f.mtime && linked(r)
}

// ---------------------------------------------------------------- movies

func (s *Scanner) scanMovies(ctx context.Context, files []diskFile, known map[string]fileRow, refresh bool) map[string]bool {
	seen := map[string]bool{}
	groups := map[string][]diskFile{}
	titles := map[string]Title{}
	var order []string
	for _, f := range files {
		s.update(func(st *Status) { st.FilesSeen++ })
		t := ParseMovie(f.rel)
		if t.Name == "" && t.TMDBID == 0 {
			s.ignore(f.abs, "could not parse title")
			continue
		}
		k := t.Key()
		if _, ok := groups[k]; !ok {
			order = append(order, k)
			titles[k] = t
		}
		groups[k] = append(groups[k], f)
	}

	for _, k := range order {
		if ctx.Err() != nil {
			return seen
		}
		group, t := groups[k], titles[k]

		if !refresh {
			allDone := true
			for _, f := range group {
				if !unchanged(f, known, func(r fileRow) bool { return r.movieID.Valid }) {
					allDone = false
				}
			}
			if allDone {
				for _, f := range group {
					seen[f.abs] = true
				}
				continue
			}
		}

		id := t.TMDBID
		if id == 0 {
			var err error
			id, err = s.tmdb.SearchMovie(ctx, t.Name, t.Year)
			if err != nil {
				s.addError(fmt.Sprintf("search movie %q: %v", t.Name, err))
				keepMovies(group, seen)
				continue
			}
		}
		if id == 0 {
			for _, f := range group {
				s.ignore(f.abs, fmt.Sprintf("no TMDB match for %q (%d)", t.Name, t.Year))
			}
			continue
		}

		exists, err := s.exists(ctx, "movies", id)
		if err != nil {
			s.addError(err.Error())
			keepMovies(group, seen)
			continue
		}
		if !exists || refresh {
			if err := s.fetchMovie(ctx, id); err != nil {
				s.addError(fmt.Sprintf("movie %d (%q): %v", id, t.Name, err))
				if !exists {
					continue
				}
			} else if !exists {
				s.update(func(st *Status) { st.MoviesAdded++ })
			}
		}

		for _, f := range group {
			seen[f.abs] = true // on a link error, keep any existing row
			if err := s.linkFile(ctx, "movie", f, id, 0); err != nil {
				s.addError(fmt.Sprintf("link %s: %v", f.abs, err))
			}
		}
	}
	return seen
}

// keepMovies marks a group's files as seen so a transient error doesn't
// remove previously linked files.
func keepMovies(group []diskFile, seen map[string]bool) {
	for _, f := range group {
		seen[f.abs] = true
	}
}

func (s *Scanner) fetchMovie(ctx context.Context, id int) error {
	m, err := s.tmdb.Movie(ctx, id)
	if err != nil {
		return err
	}
	var col *tmdb.Collection
	if m.BelongsToCollection != nil && m.BelongsToCollection.ID != 0 {
		col, err = s.tmdb.Collection(ctx, m.BelongsToCollection.ID)
		if err != nil {
			log.Printf("scan: collection %d: %v", m.BelongsToCollection.ID, err)
			col = nil
		}
	}
	if err := ingest.Movie(ctx, s.db, m, col); err != nil {
		return err
	}
	log.Printf("scan: ingested movie %d %q", m.ID, m.Title)
	s.prefetch(ctx, "w500", m.PosterPath)
	return nil
}

// ---------------------------------------------------------------- tv

func (s *Scanner) scanTV(ctx context.Context, files []diskFile, known map[string]fileRow, refresh bool) map[string]bool {
	seen := map[string]bool{}
	type parsed struct {
		f  diskFile
		ep Episode
	}
	groups := map[string][]parsed{}
	titles := map[string]Title{}
	var order []string
	for _, f := range files {
		s.update(func(st *Status) { st.FilesSeen++ })
		ep, ok := ParseEpisode(f.rel)
		if !ok {
			s.ignore(f.abs, "could not parse show folder or SxxEyy")
			continue
		}
		k := ep.Show.Key()
		if _, ok := groups[k]; !ok {
			order = append(order, k)
			titles[k] = ep.Show
		}
		groups[k] = append(groups[k], parsed{f, ep})
	}

	for _, k := range order {
		if ctx.Err() != nil {
			return seen
		}
		group, t := groups[k], titles[k]

		if !refresh {
			allDone := true
			for _, p := range group {
				if !unchanged(p.f, known, func(r fileRow) bool { return r.episodeID.Valid }) {
					allDone = false
				}
			}
			if allDone {
				for _, p := range group {
					seen[p.f.abs] = true
				}
				continue
			}
		}

		id := t.TMDBID
		if id == 0 {
			var err error
			id, err = s.tmdb.SearchTV(ctx, t.Name, t.Year)
			if err != nil {
				s.addError(fmt.Sprintf("search tv %q: %v", t.Name, err))
				for _, p := range group {
					seen[p.f.abs] = true
				}
				continue
			}
		}
		if id == 0 {
			for _, p := range group {
				s.ignore(p.f.abs, fmt.Sprintf("no TMDB match for show %q (%d)", t.Name, t.Year))
			}
			continue
		}

		exists, err := s.exists(ctx, "tv_shows", id)
		if err != nil {
			s.addError(err.Error())
			for _, p := range group {
				seen[p.f.abs] = true
			}
			continue
		}
		fetched := false
		if !exists || refresh {
			if err := s.fetchShow(ctx, id); err != nil {
				s.addError(fmt.Sprintf("show %d (%q): %v", id, t.Name, err))
				if !exists {
					continue
				}
			} else {
				fetched = true
				if !exists {
					s.update(func(st *Status) { st.ShowsAdded++ })
				}
			}
		}

		// If a file refers to an episode we don't know yet (e.g. it aired
		// after the show was last fetched), refresh the show once.
		if !fetched {
			for _, p := range group {
				if epID, _ := s.episodeID(ctx, id, p.ep.Season, p.ep.Episode); epID == 0 {
					if err := s.fetchShow(ctx, id); err != nil {
						s.addError(fmt.Sprintf("refresh show %d: %v", id, err))
					}
					break
				}
			}
		}

		for _, p := range group {
			epID, err := s.episodeID(ctx, id, p.ep.Season, p.ep.Episode)
			if err != nil {
				s.addError(err.Error())
				seen[p.f.abs] = true
				continue
			}
			if epID == 0 {
				s.ignore(p.f.abs, fmt.Sprintf("no TMDB episode S%02dE%02d for show %d", p.ep.Season, p.ep.Episode, id))
				continue
			}
			seen[p.f.abs] = true // on a link error, keep any existing row
			if err := s.linkFile(ctx, "tv", p.f, 0, epID); err != nil {
				s.addError(fmt.Sprintf("link %s: %v", p.f.abs, err))
				continue
			}
			s.prefetchStill(ctx, epID)
		}
	}
	return seen
}

func (s *Scanner) fetchShow(ctx context.Context, id int) error {
	show, err := s.tmdb.TV(ctx, id)
	if err != nil {
		return err
	}
	var seasons []ingest.SeasonData
	for _, ref := range show.Seasons {
		se, err := s.tmdb.Season(ctx, id, ref.SeasonNumber)
		if err != nil {
			s.addError(fmt.Sprintf("show %d season %d: %v", id, ref.SeasonNumber, err))
			continue
		}
		seasons = append(seasons, ingest.SeasonData{Season: se, Episodes: s.fetchEpisodes(ctx, id, se)})
	}
	if err := ingest.Show(ctx, s.db, show, seasons); err != nil {
		return err
	}
	log.Printf("scan: ingested show %d %q (%d seasons)", show.ID, show.Name, len(seasons))
	s.prefetch(ctx, "w500", show.PosterPath)
	for _, sd := range seasons {
		s.prefetch(ctx, "w500", sd.Season.PosterPath)
	}
	return nil
}

// fetchEpisodes fetches full details for every episode in a season,
// concurrently (the client's rate limiter bounds the request rate). If an
// episode's detail request fails, the season's summary of it is used.
func (s *Scanner) fetchEpisodes(ctx context.Context, showID int, se *tmdb.Season) []*tmdb.Episode {
	out := make([]*tmdb.Episode, len(se.Episodes))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, raw := range se.Episodes {
		var summary tmdb.Episode
		if err := json.Unmarshal(raw, &summary); err != nil {
			s.addError(fmt.Sprintf("show %d season %d: decode episode: %v", showID, se.SeasonNumber, err))
			continue
		}
		summary.Raw = raw
		wg.Add(1)
		go func(i int, summary tmdb.Episode) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			full, err := s.tmdb.Episode(ctx, showID, summary.SeasonNumber, summary.EpisodeNumber)
			if err != nil {
				log.Printf("scan: show %d S%02dE%02d details: %v (using season summary)", showID, summary.SeasonNumber, summary.EpisodeNumber, err)
				out[i] = &summary
				return
			}
			out[i] = full
		}(i, summary)
	}
	wg.Wait()
	eps := out[:0]
	for _, e := range out {
		if e != nil {
			eps = append(eps, e)
		}
	}
	return eps
}

// ---------------------------------------------------------------- db helpers

func (s *Scanner) exists(ctx context.Context, table string, id int) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM "+table+" WHERE id = ?", id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Scanner) episodeID(ctx context.Context, showID, season, episode int) (int, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM episodes WHERE show_id = ? AND season_number = ? AND episode_number = ?`,
		showID, season, episode).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (s *Scanner) linkFile(ctx context.Context, library string, f diskFile, movieID, episodeID int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var mid, eid any
	if movieID != 0 {
		mid = movieID
	}
	if episodeID != 0 {
		eid = episodeID
	}
	container := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.abs)), ".")
	_, err := s.db.ExecContext(ctx, `INSERT INTO files (path, library, size, mtime, container, movie_id, episode_id, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET library = excluded.library, size = excluded.size, mtime = excluded.mtime,
			container = excluded.container, movie_id = excluded.movie_id, episode_id = excluded.episode_id,
			updated_at = excluded.updated_at`,
		f.abs, library, f.size, f.mtime, container, mid, eid, now, now)
	if err == nil {
		s.update(func(st *Status) { st.FilesLinked++ })
	}
	return err
}

// removeMissing deletes file rows for a library that weren't seen (or were
// ignored) in this scan. Metadata is kept.
func (s *Scanner) removeMissing(ctx context.Context, library string, seen map[string]bool) {
	if ctx.Err() != nil {
		return // partial scan; don't remove anything
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, path FROM files WHERE library = ?`, library)
	if err != nil {
		s.addError("list files: " + err.Error())
		return
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err == nil && !seen[path] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id); err != nil {
			s.addError("remove file: " + err.Error())
			continue
		}
		s.update(func(st *Status) { st.FilesRemoved++ })
	}
}

// ---------------------------------------------------------------- images

func (s *Scanner) prefetch(ctx context.Context, size, filePath string) {
	if s.images == nil || filePath == "" {
		return
	}
	if _, err := s.images.Ensure(ctx, size, filePath); err != nil {
		log.Printf("scan: image %s%s: %v", size, filePath, err)
	}
}

func (s *Scanner) prefetchStill(ctx context.Context, episodeID int) {
	var still sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT still_path FROM episodes WHERE id = ?`, episodeID).Scan(&still); err == nil && still.Valid {
		s.prefetch(ctx, "w300", still.String)
	}
}
