package transcode

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// indexVersion is stored with each index. Bump it when the Index format or
// how it's built changes, so existing indexes are rebuilt.
const indexVersion = 1

// ErrNotFound is returned for an unknown file id.
var ErrNotFound = errors.New("file not found")

// Store keeps file indexes in the media_index table.
type Store struct {
	db *sql.DB
	tc *Transcoder

	// building serializes index builds per file, so concurrent requests for
	// a file that isn't indexed yet run ffprobe once.
	mu       sync.Mutex
	building map[int64]*sync.Mutex
}

func NewStore(db *sql.DB, tc *Transcoder) *Store {
	return &Store{db: db, tc: tc, building: map[int64]*sync.Mutex{}}
}

// Get returns the path and index of a file, building the index first if
// it's missing or out of date.
func (s *Store) Get(ctx context.Context, fileID int64) (string, *Index, error) {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path FROM files WHERE id = ?`, fileID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrNotFound
	} else if err != nil {
		return "", nil, err
	}
	ix, _, err := s.ensure(ctx, fileID, path)
	return path, ix, err
}

// ensure returns the file's index, building and storing it if needed.
// built reports whether it had to be built.
func (s *Store) ensure(ctx context.Context, fileID int64, path string) (ix *Index, built bool, err error) {
	s.mu.Lock()
	m := s.building[fileID]
	if m == nil {
		m = &sync.Mutex{}
		s.building[fileID] = m
	}
	s.mu.Unlock()
	m.Lock()
	defer m.Unlock()

	st, err := os.Stat(path)
	if err != nil {
		return nil, false, err // includes fs.ErrNotExist
	}
	size, mtime := st.Size(), st.ModTime().Unix()

	var data string
	err = s.db.QueryRowContext(ctx, `SELECT data FROM media_index WHERE file_id = ? AND size = ? AND mtime = ? AND version = ?`,
		fileID, size, mtime, indexVersion).Scan(&data)
	if err == nil {
		ix = &Index{}
		if err := json.Unmarshal([]byte(data), ix); err == nil {
			return ix, false, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	ix, err = s.tc.BuildIndex(ctx, path)
	if err != nil {
		return nil, false, err
	}
	b, err := json.Marshal(ix)
	if err != nil {
		return nil, false, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO media_index (file_id, size, mtime, version, data, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
			version = excluded.version, data = excluded.data, indexed_at = excluded.indexed_at`,
		fileID, size, mtime, indexVersion, string(b), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, false, err
	}
	return ix, true, nil
}

// IndexLibrary indexes every linked file that will be streamed through HLS
// and doesn't have an up-to-date index yet. It returns how many files it
// indexed and an error per file that failed.
func (s *Store) IndexLibrary(ctx context.Context) (indexed int, errs []error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, path FROM files WHERE movie_id IS NOT NULL OR episode_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return 0, []error{err}
	}
	type file struct {
		id   int64
		path string
	}
	var files []file
	for rows.Next() {
		var f file
		if err := rows.Scan(&f.id, &f.path); err != nil {
			rows.Close()
			return 0, []error{err}
		}
		if NeedsTranscode(f.path) {
			files = append(files, f)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, []error{err}
	}

	for _, f := range files {
		if ctx.Err() != nil {
			return indexed, append(errs, ctx.Err())
		}
		_, built, err := s.ensure(ctx, f.id, f.path)
		if err != nil {
			errs = append(errs, fmt.Errorf("index %s: %w", f.path, err))
		} else if built {
			indexed++
		}
	}
	return indexed, errs
}
