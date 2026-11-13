// Package settings stores the settings that can be changed from the UI, and
// the library folders. Settings default to the environment (see
// internal/config); a value saved here takes precedence over it.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"cinema/internal/config"
)

// Values are the effective settings.
type Values struct {
	TMDBToken    string
	TMDBLanguage string
	// SubtitleLanguages are the preferred subtitle languages, in order.
	SubtitleLanguages []string
	FFmpegPath        string
	FFprobePath       string
}

// Patch changes some settings. Nil fields are left alone. An empty string
// removes the saved value, so the default applies again. SubtitleLanguages
// is always saved as given: an empty list means no preference.
type Patch struct {
	TMDBToken         *string   `json:"tmdbToken"`
	TMDBLanguage      *string   `json:"tmdbLanguage"`
	SubtitleLanguages *[]string `json:"subtitleLanguages"`
	FFmpegPath        *string   `json:"ffmpegPath"`
	FFprobePath       *string   `json:"ffprobePath"`
}

const (
	keyTMDBToken     = "tmdb_token"
	keyTMDBLanguage  = "tmdb_language"
	keySubtitleLangs = "subtitle_languages"
	keyFFmpegPath    = "ffmpeg_path"
	keyFFprobePath   = "ffprobe_path"
	keyFoldersSeeded = "folders_seeded"
)

// InvalidError is a user mistake, such as a folder that doesn't exist. Its
// message is meant for the UI.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

type Store struct {
	db       *sql.DB
	defaults Values

	mu        sync.Mutex
	saved     map[string]string
	listeners []func(Values)
}

// Open loads the saved settings. defaults are the values used for settings
// that were never saved.
func Open(ctx context.Context, db *sql.DB, defaults Values) (*Store, error) {
	s := &Store{db: db, defaults: defaults, saved: map[string]string{}}
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		s.saved[k] = v
	}
	return s, rows.Err()
}

// Values returns the effective settings.
func (s *Store) Values() Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values()
}

func (s *Store) values() Values {
	v := s.defaults
	v.SubtitleLanguages = slices.Clone(v.SubtitleLanguages)
	str := func(key string, dst *string) {
		if saved, ok := s.saved[key]; ok {
			*dst = saved
		}
	}
	str(keyTMDBToken, &v.TMDBToken)
	str(keyTMDBLanguage, &v.TMDBLanguage)
	str(keyFFmpegPath, &v.FFmpegPath)
	str(keyFFprobePath, &v.FFprobePath)
	if saved, ok := s.saved[keySubtitleLangs]; ok {
		var langs []string
		if json.Unmarshal([]byte(saved), &langs) == nil {
			v.SubtitleLanguages = langs
		}
	}
	return v
}

// Defaults returns the values used for settings that were never saved.
func (s *Store) Defaults() Values {
	d := s.defaults
	d.SubtitleLanguages = slices.Clone(d.SubtitleLanguages)
	return d
}

// TokenSaved reports whether the TMDB token was saved here (rather than
// coming from the environment).
func (s *Store) TokenSaved() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.saved[keyTMDBToken]
	return ok
}

// OnChange registers fn to be called with the new values after each Update.
func (s *Store) OnChange(fn func(Values)) {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	s.mu.Unlock()
}

// Update saves a patch (validated by the caller) and notifies listeners.
func (s *Store) Update(ctx context.Context, p Patch) (Values, error) {
	type change struct {
		key, value string
		remove     bool
	}
	var changes []change
	str := func(key string, val *string) {
		if val == nil {
			return
		}
		if v := strings.TrimSpace(*val); v != "" {
			changes = append(changes, change{key: key, value: v})
		} else {
			changes = append(changes, change{key: key, remove: true})
		}
	}
	str(keyTMDBToken, p.TMDBToken)
	str(keyTMDBLanguage, p.TMDBLanguage)
	str(keyFFmpegPath, p.FFmpegPath)
	str(keyFFprobePath, p.FFprobePath)
	if p.SubtitleLanguages != nil {
		langs := []string{}
		for _, l := range *p.SubtitleLanguages {
			if l = strings.TrimSpace(l); l != "" && !slices.Contains(langs, l) {
				langs = append(langs, l)
			}
		}
		b, _ := json.Marshal(langs)
		changes = append(changes, change{key: keySubtitleLangs, value: string(b)})
	}

	s.mu.Lock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.mu.Unlock()
		return Values{}, err
	}
	for _, c := range changes {
		if c.remove {
			_, err = tx.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, c.key)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
				ON CONFLICT(key) DO UPDATE SET value = excluded.value`, c.key, c.value)
		}
		if err != nil {
			tx.Rollback()
			s.mu.Unlock()
			return Values{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		s.mu.Unlock()
		return Values{}, err
	}
	for _, c := range changes {
		if c.remove {
			delete(s.saved, c.key)
		} else {
			s.saved[c.key] = c.value
		}
	}
	v := s.values()
	listeners := slices.Clone(s.listeners)
	s.mu.Unlock()

	for _, fn := range listeners {
		fn(v)
	}
	return v, nil
}

// ---------------------------------------------------------------- folders

// Kinds of library folder, matching files.library.
const (
	Movies = "movie"
	TV     = "tv"
)

type Folder struct {
	ID      int64  `json:"id"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	AddedAt string `json:"addedAt"`
}

// Folders lists the library folders, by kind then path.
func (s *Store) Folders(ctx context.Context) ([]Folder, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, path, added_at FROM library_folders ORDER BY kind, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Folder{}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Kind, &f.Path, &f.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FolderPaths returns the movie and TV folder paths (for the scanner).
func (s *Store) FolderPaths(ctx context.Context) (movies, tv []string, err error) {
	folders, err := s.Folders(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range folders {
		if f.Kind == Movies {
			movies = append(movies, f.Path)
		} else {
			tv = append(tv, f.Path)
		}
	}
	return movies, tv, nil
}

// AddFolder adds a library folder. The path must be an existing directory,
// absolute (or starting with ~/), and not overlap another library folder.
func (s *Store) AddFolder(ctx context.Context, kind, path string) (Folder, error) {
	if kind != Movies && kind != TV {
		return Folder{}, invalid("kind must be %q or %q", Movies, TV)
	}
	path, err := config.ExpandHome(strings.TrimSpace(path))
	if err != nil {
		return Folder{}, err
	}
	if path == "" {
		return Folder{}, invalid("Enter a folder path.")
	}
	if !filepath.IsAbs(path) {
		return Folder{}, invalid("Use a full path, such as /media/movies or ~/Movies.")
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Folder{}, invalid("%s doesn't exist.", path)
		}
		return Folder{}, invalid("Can't open %s: %v", path, err)
	}
	if !info.IsDir() {
		return Folder{}, invalid("%s is a file, not a folder.", path)
	}

	s.mu.Lock() // serializes adds, so two can't overlap
	defer s.mu.Unlock()
	existing, err := s.Folders(ctx)
	if err != nil {
		return Folder{}, err
	}
	for _, e := range existing {
		switch {
		case e.Path == path:
			return Folder{}, invalid("%s is already in the library.", path)
		case within(path, e.Path):
			return Folder{}, invalid("%s is inside %s, which is already in the library.", path, e.Path)
		case within(e.Path, path):
			return Folder{}, invalid("%s contains %s, which is already in the library. Remove that one first.", path, e.Path)
		}
	}
	f := Folder{Kind: kind, Path: path, AddedAt: time.Now().UTC().Format(time.RFC3339)}
	res, err := s.db.ExecContext(ctx, `INSERT INTO library_folders (kind, path, added_at) VALUES (?, ?, ?)`, f.Kind, f.Path, f.AddedAt)
	if err != nil {
		return Folder{}, err
	}
	f.ID, err = res.LastInsertId()
	return f, err
}

// RemoveFolder removes a library folder, and its files from the library
// (titles' metadata is kept). It returns how many files were removed, and
// whether the folder existed.
func (s *Store) RemoveFolder(ctx context.Context, id int64) (files int64, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	var kind, path string
	err = tx.QueryRowContext(ctx, `SELECT kind, path FROM library_folders WHERE id = ?`, id).Scan(&kind, &path)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_folders WHERE id = ?`, id); err != nil {
		return 0, false, err
	}
	prefix := PathPrefix(path)
	res, err := tx.ExecContext(ctx, `DELETE FROM files WHERE library = ? AND substr(path, 1, ?) = ?`,
		kind, utf8.RuneCountInString(prefix), prefix)
	if err != nil {
		return 0, false, err
	}
	files, _ = res.RowsAffected()
	return files, true, tx.Commit()
}

// PathPrefix is the prefix of the paths of files inside dir.
func PathPrefix(dir string) string {
	return filepath.Clean(dir) + string(filepath.Separator)
}

// SeedFolders adds the folders from the environment the first time it runs,
// so a setup from before folders were managed in the UI carries over. They
// are added as they are, even if missing (e.g. a drive that isn't mounted
// yet): the scan reports that.
func (s *Store) SeedFolders(ctx context.Context, moviesDir, tvDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, seeded := s.saved[keyFoldersSeeded]; seeded {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range []struct{ kind, path string }{{Movies, moviesDir}, {TV, tvDir}} {
		if f.path == "" {
			continue
		}
		path, err := filepath.Abs(f.path)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO library_folders (kind, path, added_at) VALUES (?, ?, ?)`, f.kind, path, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO settings (key, value) VALUES (?, '1')`, keyFoldersSeeded); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.saved[keyFoldersSeeded] = "1"
	return nil
}

// within reports whether path is inside dir (not equal to it).
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
