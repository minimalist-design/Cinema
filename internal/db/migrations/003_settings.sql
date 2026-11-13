-- Settings changed in the UI (see internal/settings). Keys missing here use
-- their default from the environment / .env.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- The folders scanned into the library. kind matches files.library.
CREATE TABLE library_folders (
    id       INTEGER PRIMARY KEY,
    kind     TEXT NOT NULL CHECK (kind IN ('movie', 'tv')),
    path     TEXT NOT NULL UNIQUE,
    added_at TEXT NOT NULL
);
