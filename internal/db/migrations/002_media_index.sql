-- Playback index for files streamed through HLS (see internal/transcode):
-- streams, duration and keyframe times, as JSON. size and mtime are the
-- file's when it was indexed; a mismatch means the index is stale.
CREATE TABLE media_index (
    file_id    INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    size       INTEGER NOT NULL,
    mtime      INTEGER NOT NULL,
    version    INTEGER NOT NULL,
    data       TEXT NOT NULL,
    indexed_at TEXT NOT NULL
);
