// Package images downloads TMDB images into a local cache and serves them.
package images

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"cinema/internal/tmdb"
)

// Sizes accepted by the cache (a subset of what TMDB's CDN offers).
var allowedSizes = map[string]bool{
	"w92": true, "w154": true, "w185": true, "w300": true, "w342": true,
	"w500": true, "w780": true, "w1280": true, "h632": true, "original": true,
}

// TMDB file paths look like "/kqjL17yufvn9OVLyXYpvtyrFfak.jpg".
var validFilePath = regexp.MustCompile(`^/[A-Za-z0-9_-]+\.(jpg|jpeg|png|svg|webp)$`)

type Cache struct {
	dir    string
	db     *sql.DB
	client *tmdb.Client

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func New(dir string, db *sql.DB, client *tmdb.Client) *Cache {
	return &Cache{dir: dir, db: db, client: client, locks: map[string]*sync.Mutex{}}
}

// Valid reports whether size and filePath are acceptable cache keys.
func Valid(size, filePath string) bool {
	return allowedSizes[size] && validFilePath.MatchString(filePath)
}

func (c *Cache) localPath(size, filePath string) string {
	return filepath.Join(c.dir, "images", size, filepath.FromSlash(filePath))
}

func (c *Cache) lock(key string) func() {
	c.mu.Lock()
	l, ok := c.locks[key]
	if !ok {
		l = &sync.Mutex{}
		c.locks[key] = l
	}
	c.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// Ensure returns the local path of an image, downloading it if needed.
func (c *Cache) Ensure(ctx context.Context, size, filePath string) (string, error) {
	if !Valid(size, filePath) {
		return "", fmt.Errorf("invalid image %s%s", size, filePath)
	}
	local := c.localPath(size, filePath)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	unlock := c.lock(size + filePath)
	defer unlock()
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	data, err := c.client.FetchImage(ctx, size, filePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return "", err
	}
	tmp := local + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, local); err != nil {
		os.Remove(tmp)
		return "", err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO image_cache (file_path, size, local_path, fetched_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(file_path, size) DO UPDATE SET local_path = excluded.local_path, fetched_at = excluded.fetched_at`,
		filePath, size, local, time.Now().UTC().Format(time.RFC3339))
	return local, err
}

// Handler serves GET /images/{size}/{file}.
func (c *Cache) Handler(w http.ResponseWriter, r *http.Request) {
	size, file := r.PathValue("size"), "/"+r.PathValue("file")
	if !Valid(size, file) {
		http.NotFound(w, r)
		return
	}
	local, err := c.Ensure(r.Context(), size, file)
	if err != nil {
		if errors.Is(err, tmdb.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "image unavailable", http.StatusBadGateway)
		return
	}
	// TMDB image paths are content-addressed, so they never change.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, local)
}
