package config

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for in, want := range map[string]string{
		"~":              home,
		"~/":             home,
		"~/Movies":       filepath.Join(home, "Movies"),
		"~/a/../b":       filepath.Join(home, "b"),
		"/srv/movies":    "/srv/movies",
		"./data/db":      "./data/db",
		"~other/Movies":  "~other/Movies",
		"/media/~/films": "/media/~/films",
		"ffmpeg":         "ffmpeg",
		"":               "",
	} {
		got, err := expandHome(in)
		if err != nil || got != want {
			t.Errorf("expandHome(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestLoadExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Chdir(t.TempDir()) // no .env here
	t.Setenv("HOME", home)
	t.Setenv("TMDB_TOKEN", "x")
	t.Setenv("MOVIES_DIR", "~/Movies")
	t.Setenv("TV_DIR", "/srv/tv")
	t.Setenv("DB_PATH", "~/cinema/cinema.db")
	t.Setenv("CACHE_DIR", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MoviesDir != filepath.Join(home, "Movies") || c.TVDir != "/srv/tv" ||
		c.DBPath != filepath.Join(home, "cinema", "cinema.db") || c.CacheDir != "./data/cache" {
		t.Errorf("config = %+v", c)
	}
}
