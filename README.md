# Cinema

A small media server for movies and TV shows. Written in Go with SQLite, it pulls metadata from TMDB and has a React UI. Video files are served as-is (no transcoding) with HTTP range support, so seeking works.

## Setup

Add these to `.env` in the project root:

| Variable | Required | Default |
|---|---|---|
| `TMDB_TOKEN` | yes | TMDB API read access token (v4 bearer) |
| `MOVIES_DIR` | yes | |
| `TV_DIR` | yes | |
| `PORT` | | `8080` |
| `DB_PATH` | | `./data/cinema.db` |
| `CACHE_DIR` | | `./data/cache` (downloaded images) |
| `TMDB_LANGUAGE` | | `en-US` |

```sh
make build     # npm install + vite build, then go build (frontend embedded)
./cinema       # open http://localhost:8080 and click "Rescan library"
make test      # go test ./...
```

For development, run `make dev-server` and `make dev-web` in two terminals, then open http://localhost:5173.

## Library layout (Plex-style)

```
MOVIES_DIR/
  The Matrix (1999)/The Matrix (1999).mkv
  Heat (1995) {tmdb-949}/Heat (1995).mp4      # optional {tmdb-ID} forces the match
  Alien (1979).mkv                            # loose files work too
TV_DIR/
  Breaking Bad (2008)/Season 01/Breaking Bad (2008) - s01e01 - Pilot.mkv
```

- Titles are matched by name and year. A `{tmdb-ID}` tag in the folder or file name overrides the search.
- Files that can't be matched (no TMDB result, or no such episode) are ignored and listed in the scan status (`GET /api/scan`).
- Plex extras folders (`Extras`, `Featurettes`, …), `-trailer` style suffixes and samples are skipped.

## Scanning

Scans only run on demand: the UI button, or `curl -X POST localhost:8080/api/scan`.

- Unchanged files (same path, size and mtime) aren't re-fetched.
- `POST /api/scan?refresh=1` re-fetches metadata for every title.
- Files removed from disk are dropped from the library. Their metadata is kept.

## Data

For each title, every TMDB `append_to_response` extra is fetched. For shows, that includes every season and every episode. The data is stored in normalized tables (see `internal/db/migrations/001_init.sql`), and the complete JSON response is also kept in each entity's `raw_json` column.

## Playback

Playback uses the browser's native `<video>` element, so the container and codecs must be supported by the browser. MP4/H.264 and WebM play everywhere. MKV and HEVC depend on the browser.
