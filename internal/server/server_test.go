package server_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"cinema/internal/db"
	"cinema/internal/images"
	"cinema/internal/scanner"
	"cinema/internal/server"
	"cinema/internal/subtitles"
	"cinema/internal/tmdb"
	"cinema/internal/transcode"
)

const movieJSON = `{
  "id": 10378, "imdb_id": "tt1254207", "title": "Big Buck Bunny", "original_title": "Big Buck Bunny",
  "original_language": "en", "overview": "A giant rabbit.", "tagline": "Hop", "status": "Released",
  "release_date": "2008-05-30", "runtime": 10, "budget": 150000, "revenue": 0, "popularity": 5.1,
  "vote_average": 6.5, "vote_count": 900, "adult": false, "video": false, "homepage": "https://peach.blender.org",
  "poster_path": "/poster1.jpg", "backdrop_path": "/backdrop1.jpg", "origin_country": ["NL"],
  "belongs_to_collection": {"id": 1, "name": "Blender Open Movies", "poster_path": null, "backdrop_path": null},
  "genres": [{"id": 16, "name": "Animation"}, {"id": 35, "name": "Comedy"}],
  "production_companies": [{"id": 5, "name": "Blender Foundation", "logo_path": null, "origin_country": "NL"}],
  "production_countries": [{"iso_3166_1": "NL", "name": "Netherlands"}],
  "spoken_languages": [{"iso_639_1": "en", "english_name": "English", "name": "English"}],
  "credits": {
    "cast": [{"id": 50, "name": "Bunny", "original_name": "Bunny", "gender": 0, "known_for_department": "Acting",
              "profile_path": null, "popularity": 1.2, "adult": false, "cast_id": 1, "character": "Himself",
              "credit_id": "c1", "order": 0}],
    "crew": [{"id": 51, "name": "Sacha Goedegebure", "original_name": "Sacha Goedegebure", "gender": 2,
              "known_for_department": "Directing", "profile_path": "/p51.jpg", "popularity": 0.6, "adult": false,
              "credit_id": "c2", "department": "Directing", "job": "Director"}]
  },
  "images": {"backdrops": [{"file_path": "/backdrop1.jpg", "aspect_ratio": 1.78, "width": 1920, "height": 1080, "iso_639_1": null, "vote_average": 5, "vote_count": 1}],
             "logos": [], "posters": [{"file_path": "/poster1.jpg", "aspect_ratio": 0.67, "width": 1000, "height": 1500, "iso_639_1": "en", "vote_average": 5, "vote_count": 2}]},
  "videos": {"results": [{"id": "v1", "iso_639_1": "en", "iso_3166_1": "US", "name": "Trailer", "key": "abc", "site": "YouTube", "size": 1080, "type": "Trailer", "official": true, "published_at": "2008-05-01T00:00:00.000Z"}]},
  "external_ids": {"id": 10378, "imdb_id": "tt1254207", "wikidata_id": "Q282166", "facebook_id": null},
  "keywords": {"keywords": [{"id": 9, "name": "rabbit"}]},
  "release_dates": {"results": [{"iso_3166_1": "US", "release_dates": [{"certification": "G", "descriptors": [], "iso_639_1": "", "note": "", "release_date": "2008-05-30T00:00:00.000Z", "type": 3}]}]},
  "alternative_titles": {"titles": [{"iso_3166_1": "FR", "title": "Gros Lapin", "type": ""}]},
  "translations": {"translations": [{"iso_3166_1": "FR", "iso_639_1": "fr", "name": "Français", "english_name": "French", "data": {"title": "Gros Lapin", "overview": "Un lapin.", "homepage": "", "tagline": "", "runtime": 10}}]},
  "recommendations": {"page": 1, "results": [{"id": 45745, "title": "Sintel", "media_type": "movie", "poster_path": "/s.jpg", "release_date": "2010-09-30"}], "total_pages": 1, "total_results": 1},
  "similar": {"page": 1, "results": [], "total_pages": 0, "total_results": 0},
  "watch/providers": {"results": {"US": {"link": "https://example.test", "flatrate": [{"provider_id": 8, "provider_name": "Netflix", "logo_path": "/n.jpg", "display_priority": 1}]}}},
  "reviews": {"results": [{"id": "r1", "author": "critic", "author_details": {"name": "", "username": "critic", "avatar_path": null, "rating": 8.0}, "content": "Fluffy.", "created_at": "2010-01-01", "updated_at": "2010-01-01", "url": "https://example.test/r1"}]},
  "lists": {"results": [{"id": 77, "name": "Open films", "description": "", "favorite_count": 0, "item_count": 3, "iso_639_1": "en", "list_type": "movie", "poster_path": null}]}
}`

const collectionJSON = `{"id": 1, "name": "Blender Open Movies", "overview": "Films", "poster_path": null, "backdrop_path": null,
  "parts": [{"id": 10378, "title": "Big Buck Bunny", "media_type": "movie"}, {"id": 45745, "title": "Sintel", "media_type": "movie"}]}`

const tvJSON = `{
  "id": 100, "name": "Test Show", "original_name": "Test Show", "original_language": "en", "overview": "A show.",
  "tagline": "", "status": "Ended", "type": "Scripted", "first_air_date": "2020-01-01", "last_air_date": "2020-02-01",
  "in_production": false, "number_of_seasons": 1, "number_of_episodes": 2, "episode_run_time": [30],
  "languages": ["en"], "origin_country": ["US"], "popularity": 3, "vote_average": 7, "vote_count": 10,
  "adult": false, "homepage": "", "poster_path": "/tvposter.jpg", "backdrop_path": null,
  "last_episode_to_air": {"id": 1002}, "next_episode_to_air": null,
  "created_by": [{"id": 60, "credit_id": "cr1", "name": "Creator", "original_name": "Creator", "gender": 1, "profile_path": null}],
  "genres": [{"id": 18, "name": "Drama"}],
  "networks": [{"id": 49, "name": "HBO", "logo_path": "/hbo.png", "origin_country": "US"}],
  "production_companies": [], "production_countries": [{"iso_3166_1": "US", "name": "United States of America"}],
  "spoken_languages": [{"iso_639_1": "en", "english_name": "English", "name": "English"}],
  "seasons": [{"id": 500, "season_number": 1}],
  "aggregate_credits": {"cast": [{"id": 61, "name": "Lead", "original_name": "Lead", "gender": 2, "known_for_department": "Acting", "profile_path": null, "popularity": 1, "adult": false,
     "roles": [{"credit_id": "ag1", "character": "Hero", "episode_count": 2}], "total_episode_count": 2, "order": 0}],
     "crew": [{"id": 60, "name": "Creator", "original_name": "Creator", "gender": 1, "known_for_department": "Writing", "profile_path": null, "popularity": 1, "adult": false,
     "jobs": [{"credit_id": "ag2", "job": "Writer", "episode_count": 2}, {"credit_id": "ag3", "job": "Producer", "episode_count": 1}], "department": "Writing", "total_episode_count": 2}]},
  "credits": {"cast": [], "crew": []},
  "images": {"backdrops": [], "logos": [], "posters": [{"file_path": "/tvposter.jpg", "aspect_ratio": 0.67, "width": 500, "height": 750, "iso_639_1": "en", "vote_average": 0, "vote_count": 0}]},
  "videos": {"results": []},
  "external_ids": {"id": 100, "tvdb_id": 12345, "imdb_id": "tt999"},
  "keywords": {"results": [{"id": 10, "name": "drama"}]},
  "content_ratings": {"results": [{"descriptors": [], "iso_3166_1": "US", "rating": "TV-MA"}]},
  "alternative_titles": {"results": []},
  "translations": {"translations": []},
  "recommendations": {"page": 1, "results": [{"id": 200, "name": "Other Show", "media_type": "tv", "first_air_date": "2019-01-01"}], "total_pages": 1, "total_results": 1},
  "similar": {"page": 1, "results": [], "total_pages": 0, "total_results": 0},
  "watch/providers": {"results": {}},
  "reviews": {"results": []},
  "episode_groups": {"results": [{"id": "eg1", "name": "DVD order", "description": "", "type": 3, "group_count": 1, "episode_count": 2, "network": null}]},
  "screened_theatrically": {"results": [{"id": 1001, "season_number": 1, "episode_number": 1}]}
}`

const seasonJSON = `{"id": 500, "season_number": 1, "name": "Season 1", "overview": "", "air_date": "2020-01-01", "poster_path": "/s1.jpg", "vote_average": 7,
  "episodes": [
    {"id": 1001, "show_id": 100, "season_number": 1, "episode_number": 1, "name": "Pilot", "air_date": "2020-01-01", "runtime": 30, "still_path": "/still1.jpg", "crew": [], "guest_stars": []},
    {"id": 1002, "show_id": 100, "season_number": 1, "episode_number": 2, "name": "Second", "air_date": "2020-01-08", "runtime": 30, "still_path": null,
     "crew": [{"id": 62, "name": "Dir", "credit_id": "e2c", "department": "Directing", "job": "Director"}], "guest_stars": [{"id": 63, "name": "Guest", "credit_id": "e2g", "character": "Visitor", "order": 0}]}
  ],
  "credits": {"cast": [], "crew": []}, "aggregate_credits": {"cast": [], "crew": []},
  "images": {"posters": []}, "videos": {"results": []}, "external_ids": {"id": 500, "tvdb_id": 1}, "translations": {"translations": []}}`

const episode1JSON = `{"id": 1001, "show_id": 100, "season_number": 1, "episode_number": 1, "name": "Pilot", "overview": "It begins.", "air_date": "2020-01-01",
  "runtime": 31, "production_code": "101", "episode_type": "standard", "still_path": "/still1.jpg", "vote_average": 7.5, "vote_count": 4,
  "crew": [], "guest_stars": [],
  "credits": {"cast": [{"id": 61, "name": "Lead", "credit_id": "x1", "character": "Hero", "order": 0}], "crew": [], "guest_stars": [{"id": 64, "name": "Cameo", "credit_id": "x2", "character": "Cameo", "order": 1}]},
  "images": {"stills": [{"file_path": "/still1.jpg", "aspect_ratio": 1.78, "width": 1280, "height": 720}]},
  "videos": {"results": []}, "external_ids": {"id": 1001, "imdb_id": "tt1001"}, "translations": {"translations": []}}`

type fakeTMDB struct {
	*httptest.Server
	movieFetches atomic.Int32
	tvFetches    atomic.Int32
}

func newFakeTMDB(t *testing.T) *fakeTMDB {
	f := &fakeTMDB{}
	routes := map[string]string{
		"/configuration":             `{"images": {"secure_base_url": "https://image.tmdb.org/t/p/"}}`,
		"/genre/movie/list":          `{"genres": [{"id": 16, "name": "Animation"}, {"id": 28, "name": "Action"}]}`,
		"/genre/tv/list":             `{"genres": [{"id": 18, "name": "Drama"}]}`,
		"/collection/1":              collectionJSON,
		"/tv/100/season/1":           seasonJSON,
		"/tv/100/season/1/episode/1": episode1JSON,
	}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/img/") && r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		p := r.URL.Path
		switch {
		case p == "/search/movie":
			if strings.Contains(r.URL.Query().Get("query"), "Bunny") {
				io.WriteString(w, `{"results": [{"id": 10378}]}`)
			} else {
				io.WriteString(w, `{"results": []}`)
			}
			return
		case p == "/search/tv":
			io.WriteString(w, `{"results": [{"id": 100}]}`)
			return
		case p == "/movie/10378":
			f.movieFetches.Add(1)
			if !strings.Contains(r.URL.Query().Get("append_to_response"), "watch/providers") {
				t.Errorf("movie request missing appends: %s", r.URL.RawQuery)
			}
			io.WriteString(w, movieJSON)
			return
		case p == "/tv/100":
			f.tvFetches.Add(1)
			io.WriteString(w, tvJSON)
			return
		case strings.HasPrefix(p, "/img/"):
			w.Header().Set("Content-Type", "image/jpeg")
			io.WriteString(w, "JPEGDATA"+p)
			return
		}
		if body, ok := routes[p]; ok {
			io.WriteString(w, body)
			return
		}
		// e.g. /tv/100/season/1/episode/2: forces the season-summary fallback.
		http.NotFound(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, d *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func getJSON(t *testing.T, h http.Handler, url string, v any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
	if v != nil && rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s: %v: %s", url, err, rec.Body.String())
		}
	}
	return rec.Code
}

func TestEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	moviesDir, tvDir := filepath.Join(tmp, "movies"), filepath.Join(tmp, "tv")
	movieContent := "0123456789abcdefghijklmnopqrstuvwxyz"
	writeFile(t, filepath.Join(moviesDir, "Big Buck Bunny (2008)", "Big Buck Bunny (2008).mp4"), movieContent)
	writeFile(t, filepath.Join(moviesDir, "Big Buck Bunny (2008)", "Extras", "Making of.mp4"), "x")
	writeFile(t, filepath.Join(moviesDir, "Unknown Thing (1900).mkv"), "x")
	writeFile(t, filepath.Join(moviesDir, "notes.txt"), "x")
	writeFile(t, filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e01 - Pilot.mkv"), "ep1")
	writeFile(t, filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e02.mp4"), "ep2")
	writeFile(t, filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e09.mp4"), "ep9")

	fake := newFakeTMDB(t)
	database, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	client := tmdb.New("test-token", "en-US")
	client.BaseURL, client.ImageBaseURL = fake.URL, fake.URL+"/img"
	img := images.New(filepath.Join(tmp, "cache"), database, client)
	sc := scanner.New(context.Background(), database, client, img, nil, nil, moviesDir, tvDir)

	st := sc.Run(context.Background(), false)
	if len(st.Errors) > 0 {
		t.Fatalf("scan errors: %v", st.Errors)
	}
	if st.FilesSeen != 5 || st.FilesLinked != 3 || st.FilesIgnored != 2 {
		t.Errorf("status = %+v", st)
	}
	if fake.movieFetches.Load() != 1 || fake.tvFetches.Load() != 1 {
		t.Errorf("fetches: movie=%d tv=%d", fake.movieFetches.Load(), fake.tvFetches.Load())
	}

	// ---- normalized metadata
	checks := map[string]int{
		`SELECT COUNT(*) FROM movies WHERE id = 10378 AND collection_id = 1 AND length(raw_json) > 100`: 1,
		`SELECT COUNT(*) FROM collections WHERE id = 1 AND raw_json IS NOT NULL`:                        1,
		`SELECT COUNT(*) FROM related_titles WHERE owner_type = 'collection' AND owner_id = 1`:          2,
		`SELECT COUNT(*) FROM movie_genres WHERE movie_id = 10378`:                                      2,
		`SELECT COUNT(*) FROM genres`:                                                                          4,
		`SELECT COUNT(*) FROM movie_companies`:                                                                 1,
		`SELECT COUNT(*) FROM movie_keywords`:                                                                  1,
		`SELECT COUNT(*) FROM release_dates WHERE certification = 'G'`:                                         1,
		`SELECT COUNT(*) FROM credits WHERE owner_type = 'movie'`:                                              2,
		`SELECT COUNT(*) FROM images WHERE owner_type = 'movie'`:                                               2,
		`SELECT COUNT(*) FROM videos WHERE owner_type = 'movie'`:                                               1,
		`SELECT COUNT(*) FROM external_ids WHERE owner_type = 'movie'`:                                         2,
		`SELECT COUNT(*) FROM alternative_titles WHERE owner_type = 'movie'`:                                   1,
		`SELECT COUNT(*) FROM translations WHERE owner_type = 'movie' AND runtime = 10`:                        1,
		`SELECT COUNT(*) FROM related_titles WHERE owner_type = 'movie' AND relation = 'recommendation'`:       1,
		`SELECT COUNT(*) FROM watch_providers WHERE owner_type = 'movie' AND category = 'flatrate'`:            1,
		`SELECT COUNT(*) FROM reviews WHERE author_rating = 8`:                                                 1,
		`SELECT COUNT(*) FROM lists WHERE list_id = 77`:                                                        1,
		`SELECT COUNT(*) FROM tv_shows WHERE id = 100 AND last_episode_to_air_id = 1002`:                       1,
		`SELECT COUNT(*) FROM show_networks`:                                                                   1,
		`SELECT COUNT(*) FROM show_creators`:                                                                   1,
		`SELECT COUNT(*) FROM content_ratings WHERE rating = 'TV-MA'`:                                          1,
		`SELECT COUNT(*) FROM episode_groups`:                                                                  1,
		`SELECT COUNT(*) FROM screened_theatrically`:                                                           1,
		`SELECT COUNT(*) FROM credits WHERE owner_type = 'tv' AND is_aggregate = 1`:                            3,
		`SELECT COUNT(*) FROM external_ids WHERE owner_type = 'tv' AND source = 'tvdb_id' AND value = '12345'`: 1,
		`SELECT COUNT(*) FROM seasons WHERE show_id = 100`:                                                     1,
		`SELECT COUNT(*) FROM episodes WHERE show_id = 100`:                                                    2,
		`SELECT COUNT(*) FROM episodes WHERE id = 1001 AND runtime = 31 AND production_code = '101'`:           1,
		`SELECT COUNT(*) FROM credits WHERE owner_type = 'episode' AND owner_id = 1001`:                        2,
		// Episode 2 detail 404s, so it falls back to the season summary.
		`SELECT COUNT(*) FROM credits WHERE owner_type = 'episode' AND owner_id = 1002`:     2,
		`SELECT COUNT(*) FROM images WHERE owner_type = 'episode' AND image_type = 'still'`: 1,
		`SELECT COUNT(*) FROM files WHERE movie_id IS NOT NULL`:                             1,
		`SELECT COUNT(*) FROM files WHERE episode_id IS NOT NULL`:                           2,
		`SELECT COUNT(*) FROM image_cache`:                                                  4, // movie poster, show poster, season poster, still
	}
	for q, want := range checks {
		if got := count(t, database, q); got != want {
			t.Errorf("%s = %d, want %d", q, got, want)
		}
	}

	// ---- API
	h := server.New(database, sc, img, fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}, server.Media{}).Handler()

	var lib struct {
		Movies []struct {
			ID        int     `json:"id"`
			Title     string  `json:"title"`
			Year      int     `json:"year"`
			PosterURL *string `json:"posterUrl"`
		} `json:"movies"`
		Shows []struct {
			ID int `json:"id"`
		} `json:"shows"`
	}
	getJSON(t, h, "/api/library", &lib)
	if len(lib.Movies) != 1 || lib.Movies[0].Year != 2008 || lib.Movies[0].PosterURL == nil || *lib.Movies[0].PosterURL != "/images/w500/poster1.jpg" {
		t.Errorf("library movies = %+v", lib.Movies)
	}
	if len(lib.Shows) != 1 || lib.Shows[0].ID != 100 {
		t.Errorf("library shows = %+v", lib.Shows)
	}

	var movie struct {
		Title       string `json:"title"`
		ReleaseDate string `json:"releaseDate"`
		FileID      int    `json:"fileId"`
	}
	getJSON(t, h, "/api/movies/10378", &movie)
	if movie.Title != "Big Buck Bunny" || movie.ReleaseDate != "2008-05-30" || movie.FileID == 0 {
		t.Errorf("movie = %+v", movie)
	}

	var show struct {
		Seasons []struct {
			SeasonNumber int `json:"seasonNumber"`
			Episodes     []struct {
				ID int `json:"id"`
			} `json:"episodes"`
		} `json:"seasons"`
	}
	getJSON(t, h, "/api/shows/100", &show)
	if len(show.Seasons) != 1 || len(show.Seasons[0].Episodes) != 2 {
		t.Errorf("show = %+v", show)
	}

	var ep struct {
		Name          string `json:"name"`
		AirDate       string `json:"airDate"`
		NextEpisodeID *int   `json:"nextEpisodeId"`
		Show          struct {
			Name string `json:"name"`
		} `json:"show"`
	}
	getJSON(t, h, "/api/episodes/1001", &ep)
	if ep.Name != "Pilot" || ep.Show.Name != "Test Show" || ep.NextEpisodeID == nil || *ep.NextEpisodeID != 1002 {
		t.Errorf("episode = %+v", ep)
	}
	ep.NextEpisodeID = nil
	getJSON(t, h, "/api/episodes/1002", &ep)
	if ep.NextEpisodeID != nil {
		t.Errorf("last episode has next = %v", *ep.NextEpisodeID)
	}
	// Empty lists are [] rather than null.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/scan", nil))
	if body := rec.Body.String(); !strings.Contains(body, `"errors":[]`) || strings.Contains(body, "null") {
		t.Errorf("scan status = %s", body)
	}
	if code := getJSON(t, h, "/api/movies/999", nil); code != 404 {
		t.Errorf("missing movie code = %d", code)
	}

	// ---- streaming with range requests
	req := httptest.NewRequest("GET", "/api/stream/"+itoa(movie.FileID), nil)
	req.Header.Set("Range", "bytes=10-19")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != movieContent[10:20] || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("range stream: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	// ---- images and SPA
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/images/w500/poster1.jpg", nil))
	if rec.Code != 200 || rec.Body.String() != "JPEGDATA/img/w500/poster1.jpg" {
		t.Errorf("image: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/images/w500/..%2f..%2fetc%2fpasswd", nil))
	if rec.Code != 404 {
		t.Errorf("bad image path code = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/watch/movie/10378", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Errorf("spa fallback: %d", rec.Code)
	}

	// ---- rescan: unchanged files cause no metadata fetches; deleted files are removed
	os.Remove(filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e02.mp4"))
	os.Remove(filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e09.mp4"))
	st = sc.Run(context.Background(), false)
	if fake.movieFetches.Load() != 1 || fake.tvFetches.Load() != 1 {
		t.Errorf("rescan refetched: movie=%d tv=%d", fake.movieFetches.Load(), fake.tvFetches.Load())
	}
	if st.FilesRemoved != 1 || count(t, database, `SELECT COUNT(*) FROM files`) != 2 {
		t.Errorf("rescan status = %+v", st)
	}

	// ---- refresh re-fetches and replaces child rows without duplicating
	st = sc.Run(context.Background(), true)
	if len(st.Errors) > 0 || fake.movieFetches.Load() != 2 {
		t.Errorf("refresh: errors=%v movieFetches=%d", st.Errors, fake.movieFetches.Load())
	}
	if got := count(t, database, `SELECT COUNT(*) FROM credits WHERE owner_type = 'movie'`); got != 2 {
		t.Errorf("credits after refresh = %d", got)
	}

	testPlayback(t, database, sc, img, movie.FileID,
		filepath.Join(tvDir, "Test Show (2020)", "Season 01", "Test Show (2020) - s01e01 - Pilot.mkv"))
}

type playbackInfo struct {
	Mode      string `json:"mode"`
	URL       string `json:"url"`
	Subtitles []struct {
		Label   string `json:"label"`
		Default bool   `json:"default"`
		URL     string `json:"url"`
	} `json:"subtitles"`
}

// testPlayback checks that MP4s play directly and MKVs are streamed as HLS
// when ffmpeg is available.
func testPlayback(t *testing.T, database *sql.DB, sc *scanner.Scanner, img *images.Cache, mp4ID int, mkvPath string) {
	var mkvID int
	if err := database.QueryRow(`SELECT id FROM files WHERE path = ?`, mkvPath).Scan(&mkvID); err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{}

	// Without ffmpeg everything is served as-is.
	h := server.New(database, sc, img, web, server.Media{}).Handler()
	var pb playbackInfo
	getJSON(t, h, "/api/playback/"+itoa(mkvID), &pb)
	if pb.Mode != "direct" || pb.URL != "/api/stream/"+itoa(mkvID) {
		t.Errorf("mkv playback without ffmpeg = %+v", pb)
	}
	if code := getJSON(t, h, "/api/hls/"+itoa(mkvID)+"/abcdefgh/index.m3u8", nil); code != http.StatusNotImplemented {
		t.Errorf("hls without ffmpeg = %d", code)
	}

	tc, err := transcode.New("ffmpeg", "ffprobe")
	if err != nil {
		t.Skip("ffmpeg not installed:", err)
	}
	store := transcode.NewStore(database, tc)
	hls, err := transcode.NewHLS(store, tc, filepath.Join(t.TempDir(), "hls"))
	if err != nil {
		t.Fatal(err)
	}
	defer hls.Close()
	subs, err := subtitles.New(tc.FFmpeg(), filepath.Join(t.TempDir(), "subtitles"), []string{"eng"}, store)
	if err != nil {
		t.Fatal(err)
	}
	defer subs.Close()
	h = server.New(database, sc, img, web, server.Media{Store: store, HLS: hls, Subtitles: subs}).Handler()
	getJSON(t, h, "/api/playback/"+itoa(mp4ID), &pb)
	if pb.Mode != "direct" {
		t.Errorf("mp4 playback = %+v", pb)
	}

	// Replace the placeholder episode with a real 8s MKV (H.264 + AC3) with
	// a French subtitle track, and add an English side file.
	dir := filepath.Dir(mkvPath)
	srt := filepath.Join(t.TempDir(), "fr.srt")
	writeFile(t, srt, "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n")
	cmd := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-t", "8", "-i", "testsrc2=size=160x120:rate=24",
		"-f", "lavfi", "-t", "8", "-i", "sine", "-i", srt, "-map", "0", "-map", "1", "-map", "2",
		"-c:v", "libx264", "-g", "24", "-c:a", "ac3", "-c:s", "srt", "-metadata:s:s:0", "language=fre",
		"-disposition:s:0", "default", "-f", "matroska", mkvPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate test media: %v: %s", err, out)
	}
	writeFile(t, filepath.Join(dir, "Test Show (2020) - s01e01 - Pilot.en.srt"), "1\n00:00:03,000 --> 00:00:04,000\nHello\n")
	pb = playbackInfo{}
	getJSON(t, h, "/api/playback/"+itoa(mkvID), &pb)
	prefix := "/api/hls/" + itoa(mkvID) + "/"
	if pb.Mode != "hls" || !strings.HasPrefix(pb.URL, prefix) || !strings.HasSuffix(pb.URL, "/index.m3u8") {
		t.Fatalf("mkv playback = %+v", pb)
	}
	// English is preferred, so the side file is shown at start.
	if len(pb.Subtitles) != 2 || pb.Subtitles[0].Label != "English" || !pb.Subtitles[0].Default ||
		pb.Subtitles[1].Label != "French" || pb.Subtitles[1].Default {
		t.Errorf("subtitles = %+v", pb.Subtitles)
	}
	for _, sub := range pb.Subtitles {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", sub.URL, nil))
		want := map[string]string{"English": "Hello", "French": "Bonjour"}[sub.Label]
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/vtt; charset=utf-8" ||
			!strings.HasPrefix(rec.Body.String(), "WEBVTT") || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %q\n%s", sub.URL, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
	for _, u := range []string{"/api/subtitles/" + itoa(mkvID) + "/embedded-9.vtt", "/api/subtitles/" + itoa(mkvID) + "/file-de.srt.vtt",
		"/api/subtitles/" + itoa(mkvID) + "/file-..%2F..%2Fetc%2Fpasswd.vtt", "/api/subtitles/999999/embedded-2.vtt"} {
		if code := getJSON(t, h, u, nil); code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", u, code)
		}
	}
	// The MP4 has no subtitles.
	var mp4 playbackInfo
	getJSON(t, h, "/api/playback/"+itoa(mp4ID), &mp4)
	if mp4.Subtitles == nil || len(mp4.Subtitles) != 0 {
		t.Errorf("mp4 subtitles = %#v", mp4.Subtitles)
	}

	var other playbackInfo
	getJSON(t, h, "/api/playback/"+itoa(mkvID), &other)
	if other.URL == pb.URL {
		t.Error("playbacks share a session")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", pb.URL, nil))
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" ||
		strings.Count(body, "#EXTINF:") != 2 || !strings.Contains(body, "\n0.ts\n") || !strings.HasSuffix(body, "\n1.ts\n#EXT-X-ENDLIST\n") {
		t.Errorf("playlist: %d %q\n%s", rec.Code, rec.Header().Get("Content-Type"), body)
	}

	base := strings.TrimSuffix(pb.URL, "index.m3u8")
	for _, seg := range []string{"1.ts", "0.ts"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", base+seg, nil))
		// MPEG-TS packets start with the sync byte 0x47.
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "video/mp2t" || rec.Body.Len() < 188 || rec.Body.Bytes()[0] != 0x47 {
			t.Errorf("segment %s: %d %q, %d bytes", seg, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
	for _, u := range []string{base + "2.ts", base + "x.ts", prefix + "bad!/0.ts", "/api/hls/999999/abcdefgh/index.m3u8"} {
		if code := getJSON(t, h, u, nil); code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", u, code)
		}
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
