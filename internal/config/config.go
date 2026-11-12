// Package config loads server configuration from the environment and an
// optional .env file in the working directory.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TMDBToken    string
	TMDBLanguage string
	MoviesDir    string
	TVDir        string
	Port         string
	DBPath       string
	CacheDir     string
	FFmpegPath   string
	FFprobePath  string
}

// Load reads .env (if present) and returns the config. Real environment
// variables take precedence over values in .env.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}
	c := &Config{
		TMDBToken:    os.Getenv("TMDB_TOKEN"),
		TMDBLanguage: getenv("TMDB_LANGUAGE", "en-US"),
		MoviesDir:    os.Getenv("MOVIES_DIR"),
		TVDir:        os.Getenv("TV_DIR"),
		Port:         getenv("PORT", "8080"),
		DBPath:       getenv("DB_PATH", "./data/cinema.db"),
		CacheDir:     getenv("CACHE_DIR", "./data/cache"),
		FFmpegPath:   getenv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:  getenv("FFPROBE_PATH", "ffprobe"),
	}
	for _, p := range []*string{&c.MoviesDir, &c.TVDir, &c.DBPath, &c.CacheDir, &c.FFmpegPath, &c.FFprobePath} {
		expanded, err := expandHome(*p)
		if err != nil {
			return nil, err
		}
		*p = expanded
	}
	var missing []string
	if c.TMDBToken == "" {
		missing = append(missing, "TMDB_TOKEN")
	}
	if c.MoviesDir == "" {
		missing = append(missing, "MOVIES_DIR")
	}
	if c.TVDir == "" {
		missing = append(missing, "TV_DIR")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return c, nil
}

// expandHome replaces a leading "~" or "~/" with the user's home directory,
// as a shell would. Other paths, including "~user/...", are returned as is.
func expandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand %q: %w", p, err)
	}
	return filepath.Join(home, p[1:]), nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
