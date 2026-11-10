// Package config loads server configuration from the environment and an
// optional .env file in the working directory.
package config

import (
	"errors"
	"fmt"
	"os"

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

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
