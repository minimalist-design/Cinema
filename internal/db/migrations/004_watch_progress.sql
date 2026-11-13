-- How far each movie or episode was watched (see internal/server/progress.go).
-- Keyed by TMDB id rather than by file or with foreign keys, so progress
-- survives rescans and metadata refreshes (which may re-insert episodes).
CREATE TABLE watch_progress (
    kind       TEXT NOT NULL CHECK (kind IN ('movie', 'episode')),
    item_id    INTEGER NOT NULL,
    position   REAL NOT NULL, -- seconds
    duration   REAL NOT NULL, -- seconds
    finished   INTEGER NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (kind, item_id)
);
CREATE INDEX watch_progress_updated ON watch_progress (kind, updated_at);
