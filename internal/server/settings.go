package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"cinema/internal/config"
	"cinema/internal/settings"
	"cinema/internal/subtitles"
	"cinema/internal/tmdb"
)

// Admin holds what the settings and library folder endpoints need. With a
// nil Settings, they answer 501.
type Admin struct {
	Settings *settings.Store
	TMDB     *tmdb.Client // verifies new tokens
	Info     Info
}

// Info describes the running server, for the settings page.
type Info struct {
	Port     string
	DBPath   string
	CacheDir string
	// The ffmpeg and ffprobe settings in effect since startup, and why
	// transcoding is disabled if it is.
	FFmpegPath  string
	FFprobePath string
	MediaError  string
}

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", s.admin(s.getSettings))
	mux.HandleFunc("PUT /api/settings", s.admin(s.putSettings))
	mux.HandleFunc("GET /api/folders", s.admin(s.listFolders))
	mux.HandleFunc("POST /api/folders", s.admin(s.addFolder))
	mux.HandleFunc("DELETE /api/folders/{id}", s.admin(s.removeFolder))
	mux.HandleFunc("GET /api/browse", s.admin(s.browse))
}

func (s *Server) admin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Admin.Settings == nil {
			writeError(w, http.StatusNotImplemented, "settings unavailable")
			return
		}
		h(w, r)
	}
}

// ---------------------------------------------------------------- settings

type settingsDefaults struct {
	TMDBLanguage      string   `json:"tmdbLanguage"`
	SubtitleLanguages []string `json:"subtitleLanguages"`
	FFmpegPath        string   `json:"ffmpegPath"`
	FFprobePath       string   `json:"ffprobePath"`
}

type settingsResponse struct {
	// The token itself is never sent back.
	TMDBTokenSet bool `json:"tmdbTokenSet"`
	// "settings" (saved in the UI), "environment", or "" when not set.
	TMDBTokenSource   string           `json:"tmdbTokenSource"`
	TMDBLanguage      string           `json:"tmdbLanguage"`
	SubtitleLanguages []string         `json:"subtitleLanguages"`
	FFmpegPath        string           `json:"ffmpegPath"`
	FFprobePath       string           `json:"ffprobePath"`
	Defaults          settingsDefaults `json:"defaults"`
	Media             struct {
		Available bool    `json:"available"`
		Error     *string `json:"error"`
		// Saved ffmpeg paths that differ from the ones in use apply after a
		// restart.
		RestartRequired bool `json:"restartRequired"`
	} `json:"media"`
	Server struct {
		Port     string `json:"port"`
		DBPath   string `json:"dbPath"`
		CacheDir string `json:"cacheDir"`
	} `json:"server"`
}

func (s *Server) settingsResponse() settingsResponse {
	st, info := s.Admin.Settings, s.Admin.Info
	v, d := st.Values(), st.Defaults()
	var out settingsResponse
	out.TMDBTokenSet = v.TMDBToken != ""
	switch {
	case !out.TMDBTokenSet:
	case st.TokenSaved():
		out.TMDBTokenSource = "settings"
	default:
		out.TMDBTokenSource = "environment"
	}
	out.TMDBLanguage = v.TMDBLanguage
	out.SubtitleLanguages = nonNil(v.SubtitleLanguages)
	out.FFmpegPath, out.FFprobePath = v.FFmpegPath, v.FFprobePath
	out.Defaults = settingsDefaults{d.TMDBLanguage, nonNil(d.SubtitleLanguages), d.FFmpegPath, d.FFprobePath}
	out.Media.Available = s.HLS != nil
	if info.MediaError != "" {
		out.Media.Error = &info.MediaError
	}
	out.Media.RestartRequired = v.FFmpegPath != info.FFmpegPath || v.FFprobePath != info.FFprobePath
	out.Server.Port, out.Server.DBPath, out.Server.CacheDir = info.Port, info.DBPath, info.CacheDir
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.settingsResponse())
}

var tmdbLanguageRe = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var p settings.Patch
	if !decodeBody(w, r, &p) {
		return
	}
	trim := func(v *string) {
		if v != nil {
			*v = strings.TrimSpace(*v)
		}
	}
	trim(p.TMDBToken)
	trim(p.TMDBLanguage)
	trim(p.FFmpegPath)
	trim(p.FFprobePath)

	if p.TMDBToken != nil && *p.TMDBToken != "" {
		err := s.Admin.TMDB.VerifyToken(r.Context(), *p.TMDBToken)
		if errors.Is(err, tmdb.ErrUnauthorized) {
			writeError(w, http.StatusBadRequest, "TMDB rejected this token. Use the API Read Access Token (a long token), not the short API key.")
			return
		} else if err != nil {
			writeError(w, http.StatusBadRequest, "Couldn't check the token: "+err.Error())
			return
		}
	}
	if p.TMDBLanguage != nil && *p.TMDBLanguage != "" && !tmdbLanguageRe.MatchString(*p.TMDBLanguage) {
		writeError(w, http.StatusBadRequest, "Use a language code like en-US, pt-BR or fr.")
		return
	}
	if p.SubtitleLanguages != nil {
		if err := subtitles.ValidateLanguages(*p.SubtitleLanguages); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	for _, f := range []struct {
		name string
		path *string
	}{{"ffmpeg", p.FFmpegPath}, {"ffprobe", p.FFprobePath}} {
		if f.path == nil || *f.path == "" {
			continue
		}
		expanded, err := config.ExpandHome(*f.path)
		if err != nil {
			serverError(w, err)
			return
		}
		if _, err := exec.LookPath(expanded); err != nil {
			writeError(w, http.StatusBadRequest, "Can't find "+f.name+" at "+*f.path+".")
			return
		}
		*f.path = expanded
	}

	if _, err := s.Admin.Settings.Update(r.Context(), p); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, s.settingsResponse())
}

// ---------------------------------------------------------------- folders

type folderItem struct {
	settings.Folder
	// Whether the folder can be read right now.
	Exists bool `json:"exists"`
	// Movies or shows in the library from this folder, and their files.
	Titles int `json:"titles"`
	Files  int `json:"files"`
}

func (s *Server) listFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := s.Admin.Settings.Folders(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]folderItem, 0, len(folders))
	for _, f := range folders {
		it := folderItem{Folder: f}
		if info, err := os.Stat(f.Path); err == nil && info.IsDir() {
			it.Exists = true
		}
		prefix := settings.PathPrefix(f.Path)
		n := utf8.RuneCountInString(prefix)
		q := `SELECT COUNT(*), COUNT(DISTINCT movie_id) FROM files
			WHERE library = 'movie' AND movie_id IS NOT NULL AND substr(path, 1, ?) = ?`
		if f.Kind == settings.TV {
			q = `SELECT COUNT(*), COUNT(DISTINCT e.show_id) FROM files f JOIN episodes e ON e.id = f.episode_id
				WHERE f.library = 'tv' AND substr(f.path, 1, ?) = ?`
		}
		if err := s.db.QueryRowContext(r.Context(), q, n, prefix).Scan(&it.Files, &it.Titles); err != nil {
			serverError(w, err)
			return
		}
		out = append(out, it)
	}
	writeJSON(w, out)
}

func (s *Server) addFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	f, err := s.Admin.Settings.AddFolder(r.Context(), body.Kind, body.Path)
	var inv *settings.InvalidError
	if errors.As(err, &inv) {
		writeError(w, http.StatusBadRequest, inv.Msg)
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	s.scanner.Queue() // pick up the new folder's files
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(folderItem{Folder: f, Exists: true})
}

func (s *Server) removeFolder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	files, found, err := s.Admin.Settings.RemoveFolder(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	// A running scan listed the folders when it started, and would add the
	// files back: scan again after it.
	if s.scanner.Status().Running {
		s.scanner.Queue()
	}
	writeJSON(w, map[string]int64{"filesRemoved": files})
}

// browse lists the subfolders of a folder on the server, to pick library
// folders. Hidden folders are left out.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	p := strings.TrimSpace(r.URL.Query().Get("path"))
	if p == "" {
		p = home
	}
	if p == "" {
		p = string(filepath.Separator)
	}
	p, err := config.ExpandHome(p)
	if err != nil || !filepath.IsAbs(p) {
		writeError(w, http.StatusBadRequest, "Use a full path.")
		return
	}
	p = filepath.Clean(p)
	entries, err := os.ReadDir(p)
	if err != nil {
		msg := "Can't open " + p + "."
		if errors.Is(err, os.ErrNotExist) {
			msg = p + " doesn't exist."
		} else if errors.Is(err, os.ErrPermission) {
			msg = "No permission to open " + p + "."
		}
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	type dir struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	dirs := []dir{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(p, e.Name())
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(full)
			isDir = err == nil && info.IsDir()
		}
		if isDir {
			dirs = append(dirs, dir{e.Name(), full})
		}
	}
	slices.SortFunc(dirs, func(a, b dir) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	var parent *string
	if up := filepath.Dir(p); up != p {
		parent = &up
	}
	writeJSON(w, map[string]any{"path": p, "parent": parent, "home": home, "dirs": dirs})
}

// decodeBody decodes a JSON request body, writing a 400 if it's invalid.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return false
	}
	return true
}
