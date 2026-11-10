-- Initial schema. Entity tables use TMDB ids as primary keys. Child data that
-- can belong to several kinds of entity uses (owner_type, owner_id), where
-- owner_type is one of: movie, collection, tv, season, episode.
-- Every core entity keeps the complete TMDB response in raw_json.

CREATE TABLE tmdb_config (
    key        TEXT PRIMARY KEY,           -- 'configuration', 'genre/movie/list', ...
    raw_json   TEXT NOT NULL,
    fetched_at TEXT NOT NULL
);

-- ---------------------------------------------------------------- lookups

CREATE TABLE genres (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE companies (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL,
    logo_path      TEXT,
    origin_country TEXT
);

CREATE TABLE networks (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL,
    logo_path      TEXT,
    origin_country TEXT
);

CREATE TABLE countries (
    iso_3166_1 TEXT PRIMARY KEY,
    name       TEXT
);

CREATE TABLE languages (
    iso_639_1    TEXT PRIMARY KEY,
    english_name TEXT,
    name         TEXT
);

CREATE TABLE keywords (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE people (
    id                   INTEGER PRIMARY KEY,
    name                 TEXT NOT NULL,
    original_name        TEXT,
    gender               INTEGER,
    known_for_department TEXT,
    profile_path         TEXT,
    popularity           REAL,
    adult                INTEGER
);

-- ---------------------------------------------------------------- movies

CREATE TABLE collections (
    id            INTEGER PRIMARY KEY,
    name          TEXT NOT NULL,
    overview      TEXT,
    poster_path   TEXT,
    backdrop_path TEXT,
    raw_json      TEXT,
    fetched_at    TEXT
);

CREATE TABLE movies (
    id                INTEGER PRIMARY KEY,
    imdb_id           TEXT,
    title             TEXT NOT NULL,
    original_title    TEXT,
    original_language TEXT,
    overview          TEXT,
    tagline           TEXT,
    status            TEXT,
    release_date      TEXT,
    runtime           INTEGER,
    budget            INTEGER,
    revenue           INTEGER,
    popularity        REAL,
    vote_average      REAL,
    vote_count        INTEGER,
    adult             INTEGER,
    video             INTEGER,
    homepage          TEXT,
    poster_path       TEXT,
    backdrop_path     TEXT,
    origin_country    TEXT,                -- JSON array
    collection_id     INTEGER REFERENCES collections(id) ON DELETE SET NULL,
    raw_json          TEXT NOT NULL,
    fetched_at        TEXT NOT NULL
);

CREATE TABLE movie_genres (
    movie_id INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres(id),
    PRIMARY KEY (movie_id, genre_id)
);

CREATE TABLE movie_companies (
    movie_id   INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    company_id INTEGER NOT NULL REFERENCES companies(id),
    PRIMARY KEY (movie_id, company_id)
);

CREATE TABLE movie_countries (
    movie_id   INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    iso_3166_1 TEXT NOT NULL REFERENCES countries(iso_3166_1),
    PRIMARY KEY (movie_id, iso_3166_1)
);

CREATE TABLE movie_spoken_languages (
    movie_id  INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    iso_639_1 TEXT NOT NULL REFERENCES languages(iso_639_1),
    PRIMARY KEY (movie_id, iso_639_1)
);

CREATE TABLE movie_keywords (
    movie_id   INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    keyword_id INTEGER NOT NULL REFERENCES keywords(id),
    PRIMARY KEY (movie_id, keyword_id)
);

CREATE TABLE release_dates (
    movie_id      INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    iso_3166_1    TEXT NOT NULL,
    certification TEXT,
    descriptors   TEXT,                    -- JSON array
    iso_639_1     TEXT,
    note          TEXT,
    release_date  TEXT,
    type          INTEGER
);
CREATE INDEX release_dates_movie ON release_dates(movie_id);

-- ---------------------------------------------------------------- tv

CREATE TABLE tv_shows (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT NOT NULL,
    original_name          TEXT,
    original_language      TEXT,
    overview               TEXT,
    tagline                TEXT,
    status                 TEXT,
    type                   TEXT,
    first_air_date         TEXT,
    last_air_date          TEXT,
    in_production          INTEGER,
    number_of_seasons      INTEGER,
    number_of_episodes     INTEGER,
    episode_run_time       TEXT,           -- JSON array
    languages              TEXT,           -- JSON array
    origin_country         TEXT,           -- JSON array
    popularity             REAL,
    vote_average           REAL,
    vote_count             INTEGER,
    adult                  INTEGER,
    homepage               TEXT,
    poster_path            TEXT,
    backdrop_path          TEXT,
    last_episode_to_air_id INTEGER,
    next_episode_to_air_id INTEGER,
    raw_json               TEXT NOT NULL,
    fetched_at             TEXT NOT NULL
);

CREATE TABLE show_genres (
    show_id  INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres(id),
    PRIMARY KEY (show_id, genre_id)
);

CREATE TABLE show_companies (
    show_id    INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    company_id INTEGER NOT NULL REFERENCES companies(id),
    PRIMARY KEY (show_id, company_id)
);

CREATE TABLE show_networks (
    show_id    INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    network_id INTEGER NOT NULL REFERENCES networks(id),
    PRIMARY KEY (show_id, network_id)
);

CREATE TABLE show_countries (
    show_id    INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    iso_3166_1 TEXT NOT NULL REFERENCES countries(iso_3166_1),
    PRIMARY KEY (show_id, iso_3166_1)
);

CREATE TABLE show_spoken_languages (
    show_id   INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    iso_639_1 TEXT NOT NULL REFERENCES languages(iso_639_1),
    PRIMARY KEY (show_id, iso_639_1)
);

CREATE TABLE show_keywords (
    show_id    INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    keyword_id INTEGER NOT NULL REFERENCES keywords(id),
    PRIMARY KEY (show_id, keyword_id)
);

CREATE TABLE show_creators (
    show_id   INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id),
    credit_id TEXT,
    PRIMARY KEY (show_id, person_id)
);

CREATE TABLE content_ratings (
    show_id     INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    iso_3166_1  TEXT NOT NULL,
    rating      TEXT,
    descriptors TEXT                       -- JSON array
);
CREATE INDEX content_ratings_show ON content_ratings(show_id);

CREATE TABLE episode_groups (
    id            TEXT NOT NULL,
    show_id       INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    name          TEXT,
    description   TEXT,
    type          INTEGER,
    group_count   INTEGER,
    episode_count INTEGER,
    network_id    INTEGER,
    network_name  TEXT,
    PRIMARY KEY (show_id, id)
);

CREATE TABLE screened_theatrically (
    show_id        INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    episode_id     INTEGER NOT NULL,
    season_number  INTEGER,
    episode_number INTEGER,
    PRIMARY KEY (show_id, episode_id)
);

CREATE TABLE seasons (
    id            INTEGER PRIMARY KEY,
    show_id       INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    name          TEXT,
    overview      TEXT,
    air_date      TEXT,
    episode_count INTEGER,
    poster_path   TEXT,
    vote_average  REAL,
    raw_json      TEXT NOT NULL,
    fetched_at    TEXT NOT NULL,
    UNIQUE (show_id, season_number)
);

CREATE TABLE episodes (
    id              INTEGER PRIMARY KEY,
    show_id         INTEGER NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    season_id       INTEGER REFERENCES seasons(id) ON DELETE CASCADE,
    season_number   INTEGER NOT NULL,
    episode_number  INTEGER NOT NULL,
    name            TEXT,
    overview        TEXT,
    air_date        TEXT,
    runtime         INTEGER,
    production_code TEXT,
    episode_type    TEXT,
    still_path      TEXT,
    vote_average    REAL,
    vote_count      INTEGER,
    raw_json        TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    UNIQUE (show_id, season_number, episode_number)
);

-- ---------------------------------------------------------------- polymorphic children

CREATE TABLE credits (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_type          TEXT NOT NULL,
    owner_id            INTEGER NOT NULL,
    person_id           INTEGER NOT NULL REFERENCES people(id),
    credit_id           TEXT,
    kind                TEXT NOT NULL CHECK (kind IN ('cast', 'crew', 'guest_star')),
    is_aggregate        INTEGER NOT NULL DEFAULT 0,
    character           TEXT,
    department          TEXT,
    job                 TEXT,
    cast_order          INTEGER,
    episode_count       INTEGER,
    total_episode_count INTEGER
);
CREATE INDEX credits_owner ON credits(owner_type, owner_id);
CREATE INDEX credits_person ON credits(person_id);

CREATE TABLE images (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_type   TEXT NOT NULL,
    owner_id     INTEGER NOT NULL,
    image_type   TEXT NOT NULL,            -- poster, backdrop, logo, still
    file_path    TEXT NOT NULL,
    aspect_ratio REAL,
    width        INTEGER,
    height       INTEGER,
    iso_639_1    TEXT,
    iso_3166_1   TEXT,
    vote_average REAL,
    vote_count   INTEGER
);
CREATE INDEX images_owner ON images(owner_type, owner_id);

-- Locally cached copies of TMDB images, per size.
CREATE TABLE image_cache (
    file_path  TEXT NOT NULL,
    size       TEXT NOT NULL,
    local_path TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    PRIMARY KEY (file_path, size)
);

CREATE TABLE videos (
    id           TEXT NOT NULL,
    owner_type   TEXT NOT NULL,
    owner_id     INTEGER NOT NULL,
    key          TEXT,
    site         TEXT,
    type         TEXT,
    name         TEXT,
    official     INTEGER,
    published_at TEXT,
    size         INTEGER,
    iso_639_1    TEXT,
    iso_3166_1   TEXT,
    PRIMARY KEY (owner_type, owner_id, id)
);

CREATE TABLE external_ids (
    owner_type TEXT NOT NULL,
    owner_id   INTEGER NOT NULL,
    source     TEXT NOT NULL,              -- imdb_id, tvdb_id, wikidata_id, ...
    value      TEXT NOT NULL,
    PRIMARY KEY (owner_type, owner_id, source)
);

CREATE TABLE alternative_titles (
    owner_type TEXT NOT NULL,
    owner_id   INTEGER NOT NULL,
    iso_3166_1 TEXT,
    title      TEXT,
    type       TEXT
);
CREATE INDEX alternative_titles_owner ON alternative_titles(owner_type, owner_id);

CREATE TABLE translations (
    owner_type   TEXT NOT NULL,
    owner_id     INTEGER NOT NULL,
    iso_639_1    TEXT,
    iso_3166_1   TEXT,
    name         TEXT,
    english_name TEXT,
    title        TEXT,
    overview     TEXT,
    tagline      TEXT,
    homepage     TEXT,
    runtime      INTEGER
);
CREATE INDEX translations_owner ON translations(owner_type, owner_id);

CREATE TABLE watch_providers (
    owner_type       TEXT NOT NULL,
    owner_id         INTEGER NOT NULL,
    region           TEXT NOT NULL,
    link             TEXT,
    category         TEXT NOT NULL,        -- flatrate, rent, buy, ads, free
    provider_id      INTEGER NOT NULL,
    provider_name    TEXT,
    logo_path        TEXT,
    display_priority INTEGER
);
CREATE INDEX watch_providers_owner ON watch_providers(owner_type, owner_id);

CREATE TABLE reviews (
    id                 TEXT NOT NULL,
    owner_type         TEXT NOT NULL,
    owner_id           INTEGER NOT NULL,
    author             TEXT,
    author_name        TEXT,
    author_username    TEXT,
    author_avatar_path TEXT,
    author_rating      REAL,
    content            TEXT,
    created_at         TEXT,
    updated_at         TEXT,
    url                TEXT,
    PRIMARY KEY (owner_type, owner_id, id)
);

-- Recommendations, similar titles and collection parts.
CREATE TABLE related_titles (
    owner_type     TEXT NOT NULL,
    owner_id       INTEGER NOT NULL,
    relation       TEXT NOT NULL,          -- recommendation, similar, collection_part
    rank           INTEGER NOT NULL,
    target_type    TEXT NOT NULL,          -- movie, tv
    target_id      INTEGER NOT NULL,
    title          TEXT,
    original_title TEXT,
    overview       TEXT,
    release_date   TEXT,
    poster_path    TEXT,
    backdrop_path  TEXT,
    popularity     REAL,
    vote_average   REAL,
    vote_count     INTEGER,
    raw_json       TEXT
);
CREATE INDEX related_titles_owner ON related_titles(owner_type, owner_id);

-- User lists that contain a movie.
CREATE TABLE lists (
    owner_type     TEXT NOT NULL,
    owner_id       INTEGER NOT NULL,
    list_id        INTEGER NOT NULL,
    name           TEXT,
    description    TEXT,
    favorite_count INTEGER,
    item_count     INTEGER,
    iso_639_1      TEXT,
    list_type      TEXT,
    poster_path    TEXT,
    PRIMARY KEY (owner_type, owner_id, list_id)
);

-- ---------------------------------------------------------------- files on disk

CREATE TABLE files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT NOT NULL UNIQUE,
    library    TEXT NOT NULL CHECK (library IN ('movie', 'tv')),
    size       INTEGER NOT NULL,
    mtime      INTEGER NOT NULL,
    container  TEXT,
    movie_id   INTEGER REFERENCES movies(id) ON DELETE SET NULL,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    added_at   TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX files_movie ON files(movie_id);
CREATE INDEX files_episode ON files(episode_id);
