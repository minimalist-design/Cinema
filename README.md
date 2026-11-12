# Cinema

A small media server for movies and TV shows. Written in Go with SQLite, it pulls metadata from TMDB and has a React UI. Video files are served as-is with HTTP range support, except MKVs, which are transcoded on the fly with ffmpeg.

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
| `FFMPEG_PATH` | | `ffmpeg` (looked up on `PATH`) |
| `FFPROBE_PATH` | | `ffprobe` (looked up on `PATH`) |

Paths can be absolute, start with `~/` (your home directory), or be relative to the directory you start the server from. The `.env` file is also read from that directory.

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

The player asks `GET /api/playback/{fileId}` how to play a file.

- **Direct** (everything except MKV): the file is served untouched from `/api/stream/{fileId}` and played with the browser's native `<video>` controls, so the codecs must be supported by the browser. MP4/H.264 and WebM play everywhere; HEVC depends on the browser.
- **Transcode** (MKV, when ffmpeg and ffprobe are installed): `/api/transcode/{fileId}?start=<seconds>` streams a fragmented MP4 made by ffmpeg.
  - H.264 (8-bit) video and AAC/MP3 audio are copied without re-encoding, so the common H.264 + AC3/DTS MKV costs almost no CPU. Other video (HEVC, 10-bit H.264, MPEG-4, …) is encoded to H.264 at up to 1080p, and other audio to stereo AAC.
  - The default audio track is used. Subtitles are dropped.
  - The stream can't be seeked by byte ranges, so the player has its own controls and seeks by requesting a new stream from that time (`/api/playback/{fileId}?start=<seconds>` returns its URL). With copied video a stream has to begin on a keyframe, so it may start a few seconds before the requested time.
  - Closing the player or seeking stops the ffmpeg process.

If ffmpeg isn't found, a warning is logged at startup and MKVs are served as-is.

Known limits: Safari won't play transcoded streams (it requires range requests). HDR video is converted to H.264 without tone mapping, so colours look washed out.
