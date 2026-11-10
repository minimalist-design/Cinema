// Package ingest writes TMDB responses into the normalized SQLite schema.
// Each top-level entity (movie, show) is written in a single transaction.
// Child rows for an owner are replaced wholesale, so re-ingesting is a clean
// refresh.
package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cinema/internal/tmdb"
)

// SeasonData groups a season with the full details of each of its episodes.
type SeasonData struct {
	Season   *tmdb.Season
	Episodes []*tmdb.Episode
}

// Movie stores a movie and everything appended to it. col may be nil.
func Movie(ctx context.Context, db *sql.DB, m *tmdb.Movie, col *tmdb.Collection) error {
	return inTx(ctx, db, func(w *writer) {
		var collectionID any
		if col != nil {
			w.collection(col)
			collectionID = col.ID
		} else if ref := m.BelongsToCollection; ref != nil && ref.ID != 0 {
			w.exec(`INSERT INTO collections (id, name, poster_path, backdrop_path) VALUES (?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET name = excluded.name, poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path`,
				ref.ID, ref.Name, ns(ref.PosterPath), ns(ref.BackdropPath))
			collectionID = ref.ID
		}

		w.upsert("movies", "id",
			"id", m.ID,
			"imdb_id", ns(m.IMDbID),
			"title", m.Title,
			"original_title", ns(m.OriginalTitle),
			"original_language", ns(m.OriginalLanguage),
			"overview", ns(m.Overview),
			"tagline", ns(m.Tagline),
			"status", ns(m.Status),
			"release_date", ns(m.ReleaseDate),
			"runtime", m.Runtime,
			"budget", m.Budget,
			"revenue", m.Revenue,
			"popularity", m.Popularity,
			"vote_average", m.VoteAverage,
			"vote_count", m.VoteCount,
			"adult", m.Adult,
			"video", m.Video,
			"homepage", ns(m.Homepage),
			"poster_path", ns(m.PosterPath),
			"backdrop_path", ns(m.BackdropPath),
			"origin_country", jsonText(m.OriginCountry),
			"collection_id", collectionID,
			"raw_json", string(m.Raw),
			"fetched_at", now(),
		)

		for _, t := range []string{"movie_genres", "movie_companies", "movie_countries", "movie_spoken_languages", "movie_keywords", "release_dates"} {
			w.exec("DELETE FROM "+t+" WHERE movie_id = ?", m.ID)
		}
		w.clearOwner("movie", m.ID)

		for _, g := range m.Genres {
			w.genre(g)
			w.exec(`INSERT OR IGNORE INTO movie_genres (movie_id, genre_id) VALUES (?, ?)`, m.ID, g.ID)
		}
		for _, c := range m.ProductionCompanies {
			w.company("companies", c)
			w.exec(`INSERT OR IGNORE INTO movie_companies (movie_id, company_id) VALUES (?, ?)`, m.ID, c.ID)
		}
		for _, c := range m.ProductionCountries {
			w.country(c)
			w.exec(`INSERT OR IGNORE INTO movie_countries (movie_id, iso_3166_1) VALUES (?, ?)`, m.ID, c.ISO31661)
		}
		for _, l := range m.SpokenLanguages {
			w.language(l)
			w.exec(`INSERT OR IGNORE INTO movie_spoken_languages (movie_id, iso_639_1) VALUES (?, ?)`, m.ID, l.ISO6391)
		}
		for _, k := range m.Keywords.All() {
			w.keyword(k)
			w.exec(`INSERT OR IGNORE INTO movie_keywords (movie_id, keyword_id) VALUES (?, ?)`, m.ID, k.ID)
		}
		if rd := m.ReleaseDates; rd != nil {
			for _, country := range rd.Results {
				for _, r := range country.ReleaseDates {
					w.insert("release_dates",
						"movie_id", m.ID,
						"iso_3166_1", country.ISO31661,
						"certification", ns(r.Certification),
						"descriptors", jsonText(r.Descriptors),
						"iso_639_1", ns(r.ISO6391),
						"note", ns(r.Note),
						"release_date", ns(r.ReleaseDate),
						"type", r.Type,
					)
				}
			}
		}

		w.credits("movie", m.ID, m.Credits)
		w.images("movie", m.ID, m.Images)
		w.videos("movie", m.ID, m.Videos)
		w.externalIDs("movie", m.ID, m.ExternalIDs)
		w.altTitles("movie", m.ID, m.AlternativeTitles)
		w.translations("movie", m.ID, m.Translations)
		w.related("movie", m.ID, "recommendation", "movie", m.Recommendations)
		w.related("movie", m.ID, "similar", "movie", m.Similar)
		w.watchProviders("movie", m.ID, m.WatchProviders)
		w.reviews("movie", m.ID, m.Reviews)
		if l := m.Lists; l != nil {
			for _, x := range l.Results {
				w.exec(`INSERT OR REPLACE INTO lists (owner_type, owner_id, list_id, name, description, favorite_count, item_count, iso_639_1, list_type, poster_path)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					"movie", m.ID, x.ID, ns(x.Name), ns(x.Description), x.FavoriteCount, x.ItemCount, ns(x.ISO6391), ns(x.ListType), ns(x.PosterPath))
			}
		}
	})
}

// Show stores a TV show, its seasons and episodes, and everything appended.
func Show(ctx context.Context, db *sql.DB, s *tmdb.TVShow, seasons []SeasonData) error {
	return inTx(ctx, db, func(w *writer) {
		var lastEp, nextEp any
		if s.LastEpisodeToAir != nil {
			lastEp = s.LastEpisodeToAir.ID
		}
		if s.NextEpisodeToAir != nil {
			nextEp = s.NextEpisodeToAir.ID
		}
		w.upsert("tv_shows", "id",
			"id", s.ID,
			"name", s.Name,
			"original_name", ns(s.OriginalName),
			"original_language", ns(s.OriginalLanguage),
			"overview", ns(s.Overview),
			"tagline", ns(s.Tagline),
			"status", ns(s.Status),
			"type", ns(s.Type),
			"first_air_date", ns(s.FirstAirDate),
			"last_air_date", ns(s.LastAirDate),
			"in_production", s.InProduction,
			"number_of_seasons", s.NumberOfSeasons,
			"number_of_episodes", s.NumberOfEpisodes,
			"episode_run_time", jsonText(s.EpisodeRunTime),
			"languages", jsonText(s.Languages),
			"origin_country", jsonText(s.OriginCountry),
			"popularity", s.Popularity,
			"vote_average", s.VoteAverage,
			"vote_count", s.VoteCount,
			"adult", s.Adult,
			"homepage", ns(s.Homepage),
			"poster_path", ns(s.PosterPath),
			"backdrop_path", ns(s.BackdropPath),
			"last_episode_to_air_id", lastEp,
			"next_episode_to_air_id", nextEp,
			"raw_json", string(s.Raw),
			"fetched_at", now(),
		)

		for _, t := range []string{"show_genres", "show_companies", "show_networks", "show_countries", "show_spoken_languages", "show_keywords", "show_creators", "content_ratings", "episode_groups", "screened_theatrically"} {
			w.exec("DELETE FROM "+t+" WHERE show_id = ?", s.ID)
		}
		w.clearOwner("tv", s.ID)

		for _, g := range s.Genres {
			w.genre(g)
			w.exec(`INSERT OR IGNORE INTO show_genres (show_id, genre_id) VALUES (?, ?)`, s.ID, g.ID)
		}
		for _, c := range s.ProductionCompanies {
			w.company("companies", c)
			w.exec(`INSERT OR IGNORE INTO show_companies (show_id, company_id) VALUES (?, ?)`, s.ID, c.ID)
		}
		for _, n := range s.Networks {
			w.company("networks", n)
			w.exec(`INSERT OR IGNORE INTO show_networks (show_id, network_id) VALUES (?, ?)`, s.ID, n.ID)
		}
		for _, c := range s.ProductionCountries {
			w.country(c)
			w.exec(`INSERT OR IGNORE INTO show_countries (show_id, iso_3166_1) VALUES (?, ?)`, s.ID, c.ISO31661)
		}
		for _, l := range s.SpokenLanguages {
			w.language(l)
			w.exec(`INSERT OR IGNORE INTO show_spoken_languages (show_id, iso_639_1) VALUES (?, ?)`, s.ID, l.ISO6391)
		}
		for _, k := range s.Keywords.All() {
			w.keyword(k)
			w.exec(`INSERT OR IGNORE INTO show_keywords (show_id, keyword_id) VALUES (?, ?)`, s.ID, k.ID)
		}
		for _, c := range s.CreatedBy {
			w.person(tmdb.Person{ID: c.ID, Name: c.Name, OriginalName: c.OriginalName, Gender: c.Gender, ProfilePath: c.ProfilePath})
			w.exec(`INSERT OR IGNORE INTO show_creators (show_id, person_id, credit_id) VALUES (?, ?, ?)`, s.ID, c.ID, ns(c.CreditID))
		}
		if cr := s.ContentRatings; cr != nil {
			for _, r := range cr.Results {
				w.insert("content_ratings",
					"show_id", s.ID,
					"iso_3166_1", r.ISO31661,
					"rating", ns(r.Rating),
					"descriptors", jsonText(r.Descriptors),
				)
			}
		}
		if eg := s.EpisodeGroups; eg != nil {
			for _, g := range eg.Results {
				var netID, netName any
				if g.Network != nil {
					netID, netName = g.Network.ID, g.Network.Name
				}
				w.exec(`INSERT OR REPLACE INTO episode_groups (id, show_id, name, description, type, group_count, episode_count, network_id, network_name)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					g.ID, s.ID, ns(g.Name), ns(g.Description), g.Type, g.GroupCount, g.EpisodeCount, netID, netName)
			}
		}
		if st := s.ScreenedTheatrically; st != nil {
			for _, e := range st.Results {
				w.exec(`INSERT OR REPLACE INTO screened_theatrically (show_id, episode_id, season_number, episode_number) VALUES (?, ?, ?, ?)`,
					s.ID, e.ID, e.SeasonNumber, e.EpisodeNumber)
			}
		}

		w.aggregateCredits("tv", s.ID, s.AggregateCredits)
		w.credits("tv", s.ID, s.Credits)
		w.images("tv", s.ID, s.Images)
		w.videos("tv", s.ID, s.Videos)
		w.externalIDs("tv", s.ID, s.ExternalIDs)
		w.altTitles("tv", s.ID, s.AlternativeTitles)
		w.translations("tv", s.ID, s.Translations)
		w.related("tv", s.ID, "recommendation", "tv", s.Recommendations)
		w.related("tv", s.ID, "similar", "tv", s.Similar)
		w.watchProviders("tv", s.ID, s.WatchProviders)
		w.reviews("tv", s.ID, s.Reviews)

		for _, sd := range seasons {
			w.season(s.ID, sd)
		}
	})
}

func (w *writer) season(showID int, sd SeasonData) {
	se := sd.Season
	// TMDB occasionally re-creates a season under a new id.
	w.exec(`DELETE FROM seasons WHERE show_id = ? AND season_number = ? AND id <> ?`, showID, se.SeasonNumber, se.ID)
	w.upsert("seasons", "id",
		"id", se.ID,
		"show_id", showID,
		"season_number", se.SeasonNumber,
		"name", ns(se.Name),
		"overview", ns(se.Overview),
		"air_date", ns(se.AirDate),
		"episode_count", len(se.Episodes),
		"poster_path", ns(se.PosterPath),
		"vote_average", se.VoteAverage,
		"raw_json", string(se.Raw),
		"fetched_at", now(),
	)
	w.clearOwner("season", se.ID)
	w.credits("season", se.ID, se.Credits)
	w.aggregateCredits("season", se.ID, se.AggregateCredits)
	w.images("season", se.ID, se.Images)
	w.videos("season", se.ID, se.Videos)
	w.externalIDs("season", se.ID, se.ExternalIDs)
	w.translations("season", se.ID, se.Translations)

	for _, ep := range sd.Episodes {
		w.episode(showID, se.ID, ep)
	}
}

func (w *writer) episode(showID, seasonID int, ep *tmdb.Episode) {
	w.exec(`DELETE FROM episodes WHERE show_id = ? AND season_number = ? AND episode_number = ? AND id <> ?`,
		showID, ep.SeasonNumber, ep.EpisodeNumber, ep.ID)
	w.upsert("episodes", "id",
		"id", ep.ID,
		"show_id", showID,
		"season_id", seasonID,
		"season_number", ep.SeasonNumber,
		"episode_number", ep.EpisodeNumber,
		"name", ns(ep.Name),
		"overview", ns(ep.Overview),
		"air_date", ns(ep.AirDate),
		"runtime", ep.Runtime,
		"production_code", ns(ep.ProductionCode),
		"episode_type", ns(ep.EpisodeType),
		"still_path", ns(ep.StillPath),
		"vote_average", ep.VoteAverage,
		"vote_count", ep.VoteCount,
		"raw_json", string(ep.Raw),
		"fetched_at", now(),
	)
	w.clearOwner("episode", ep.ID)
	credits := ep.Credits
	if credits == nil {
		credits = &tmdb.Credits{Crew: ep.Crew, GuestStars: ep.GuestStars}
	}
	w.credits("episode", ep.ID, credits)
	w.images("episode", ep.ID, ep.Images)
	w.videos("episode", ep.ID, ep.Videos)
	w.externalIDs("episode", ep.ID, ep.ExternalIDs)
	w.translations("episode", ep.ID, ep.Translations)
}

func (w *writer) collection(c *tmdb.Collection) {
	w.upsert("collections", "id",
		"id", c.ID,
		"name", c.Name,
		"overview", ns(c.Overview),
		"poster_path", ns(c.PosterPath),
		"backdrop_path", ns(c.BackdropPath),
		"raw_json", string(c.Raw),
		"fetched_at", now(),
	)
	w.exec(`DELETE FROM related_titles WHERE owner_type = 'collection' AND owner_id = ?`, c.ID)
	w.related("collection", c.ID, "collection_part", "movie", &tmdb.Paged{Results: c.Parts})
}

// Config stores a raw TMDB reference response (configuration, genre lists).
// Genre list responses also populate the genres table.
func Config(ctx context.Context, db *sql.DB, key string, raw []byte) error {
	return inTx(ctx, db, func(w *writer) {
		w.upsert("tmdb_config", "key", "key", key, "raw_json", string(raw), "fetched_at", now())
		if strings.HasPrefix(key, "genre/") {
			var gl struct {
				Genres []tmdb.Genre `json:"genres"`
			}
			if err := json.Unmarshal(raw, &gl); err != nil {
				w.fail(err)
				return
			}
			for _, g := range gl.Genres {
				w.genre(g)
			}
		}
	})
}

// ---------------------------------------------------------------- writer

type writer struct {
	ctx context.Context
	tx  *sql.Tx
	err error
}

func inTx(ctx context.Context, db *sql.DB, fn func(w *writer)) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	w := &writer{ctx: ctx, tx: tx}
	fn(w)
	if w.err != nil {
		tx.Rollback()
		return w.err
	}
	return tx.Commit()
}

func (w *writer) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

func (w *writer) exec(query string, args ...any) {
	if w.err != nil {
		return
	}
	if _, err := w.tx.ExecContext(w.ctx, query, args...); err != nil {
		w.err = fmt.Errorf("%w\nquery: %s", err, strings.Join(strings.Fields(query), " "))
	}
}

func splitKV(kv []any) (cols []string, vals []any) {
	for i := 0; i+1 < len(kv); i += 2 {
		cols = append(cols, kv[i].(string))
		vals = append(vals, kv[i+1])
	}
	return
}

func (w *writer) insert(table string, kv ...any) {
	cols, vals := splitKV(kv)
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), placeholders(len(cols)))
	w.exec(q, vals...)
}

// upsert inserts a row or updates every given column on conflict.
func (w *writer) upsert(table, conflict string, kv ...any) {
	cols, vals := splitKV(kv)
	sets := make([]string, 0, len(cols))
	for _, c := range cols {
		if c != conflict {
			sets = append(sets, c+" = excluded."+c)
		}
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s",
		table, strings.Join(cols, ", "), placeholders(len(cols)), conflict, strings.Join(sets, ", "))
	w.exec(q, vals...)
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

var ownerTables = []string{"credits", "images", "videos", "external_ids", "alternative_titles", "translations", "watch_providers", "reviews", "related_titles", "lists"}

func (w *writer) clearOwner(ownerType string, id int) {
	for _, t := range ownerTables {
		w.exec("DELETE FROM "+t+" WHERE owner_type = ? AND owner_id = ?", ownerType, id)
	}
}

// ---------------------------------------------------------------- lookups

func (w *writer) genre(g tmdb.Genre) {
	w.upsert("genres", "id", "id", g.ID, "name", g.Name)
}

func (w *writer) company(table string, c tmdb.Company) {
	w.upsert(table, "id", "id", c.ID, "name", c.Name, "logo_path", ns(c.LogoPath), "origin_country", ns(c.OriginCountry))
}

func (w *writer) country(c tmdb.Country) {
	w.upsert("countries", "iso_3166_1", "iso_3166_1", c.ISO31661, "name", ns(c.Name))
}

func (w *writer) language(l tmdb.SpokenLanguage) {
	w.upsert("languages", "iso_639_1", "iso_639_1", l.ISO6391, "english_name", ns(l.EnglishName), "name", ns(l.Name))
}

func (w *writer) keyword(k tmdb.Keyword) {
	w.upsert("keywords", "id", "id", k.ID, "name", k.Name)
}

func (w *writer) person(p tmdb.Person) {
	// Creator entries carry fewer fields than credit entries; don't let them
	// blank out richer data already stored.
	w.exec(`INSERT INTO people (id, name, original_name, gender, known_for_department, profile_path, popularity, adult)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			original_name = COALESCE(excluded.original_name, people.original_name),
			gender = excluded.gender,
			known_for_department = COALESCE(excluded.known_for_department, people.known_for_department),
			profile_path = COALESCE(excluded.profile_path, people.profile_path),
			popularity = COALESCE(excluded.popularity, people.popularity),
			adult = excluded.adult`,
		p.ID, p.Name, ns(p.OriginalName), p.Gender, ns(p.KnownForDepartment), ns(p.ProfilePath), nf(p.Popularity), p.Adult)
}

// ---------------------------------------------------------------- children

func (w *writer) credits(owner string, id int, c *tmdb.Credits) {
	if c == nil {
		return
	}
	cast := func(kind string, list []tmdb.CastMember) {
		for _, m := range list {
			w.person(m.Person)
			w.insert("credits",
				"owner_type", owner, "owner_id", id, "person_id", m.ID,
				"credit_id", ns(m.CreditID), "kind", kind, "is_aggregate", false,
				"character", ns(m.Character), "cast_order", m.Order,
			)
		}
	}
	cast("cast", c.Cast)
	cast("guest_star", c.GuestStars)
	for _, m := range c.Crew {
		w.person(m.Person)
		w.insert("credits",
			"owner_type", owner, "owner_id", id, "person_id", m.ID,
			"credit_id", ns(m.CreditID), "kind", "crew", "is_aggregate", false,
			"department", ns(m.Department), "job", ns(m.Job),
		)
	}
}

func (w *writer) aggregateCredits(owner string, id int, a *tmdb.AggregateCredits) {
	if a == nil {
		return
	}
	for _, m := range a.Cast {
		w.person(m.Person)
		for _, r := range m.Roles {
			w.insert("credits",
				"owner_type", owner, "owner_id", id, "person_id", m.ID,
				"credit_id", ns(r.CreditID), "kind", "cast", "is_aggregate", true,
				"character", ns(r.Character), "cast_order", m.Order,
				"episode_count", r.EpisodeCount, "total_episode_count", m.TotalEpisodeCount,
			)
		}
	}
	for _, m := range a.Crew {
		w.person(m.Person)
		for _, j := range m.Jobs {
			w.insert("credits",
				"owner_type", owner, "owner_id", id, "person_id", m.ID,
				"credit_id", ns(j.CreditID), "kind", "crew", "is_aggregate", true,
				"department", ns(m.Department), "job", ns(j.Job),
				"episode_count", j.EpisodeCount, "total_episode_count", m.TotalEpisodeCount,
			)
		}
	}
}

func (w *writer) images(owner string, id int, imgs *tmdb.Images) {
	if imgs == nil {
		return
	}
	for typ, list := range map[string][]tmdb.Image{
		"poster": imgs.Posters, "backdrop": imgs.Backdrops, "logo": imgs.Logos, "still": imgs.Stills,
	} {
		for _, im := range list {
			w.insert("images",
				"owner_type", owner, "owner_id", id, "image_type", typ,
				"file_path", im.FilePath, "aspect_ratio", im.AspectRatio,
				"width", im.Width, "height", im.Height,
				"iso_639_1", ns(im.ISO6391), "iso_3166_1", ns(im.ISO31661),
				"vote_average", im.VoteAverage, "vote_count", im.VoteCount,
			)
		}
	}
}

func (w *writer) videos(owner string, id int, v *tmdb.Videos) {
	if v == nil {
		return
	}
	for _, x := range v.Results {
		w.exec(`INSERT OR REPLACE INTO videos (id, owner_type, owner_id, key, site, type, name, official, published_at, size, iso_639_1, iso_3166_1)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			x.ID, owner, id, ns(x.Key), ns(x.Site), ns(x.Type), ns(x.Name), x.Official, ns(x.PublishedAt), x.Size, ns(x.ISO6391), ns(x.ISO31661))
	}
}

func (w *writer) externalIDs(owner string, id int, ids map[string]any) {
	for k, v := range ids {
		if k == "id" {
			continue
		}
		var s string
		switch x := v.(type) {
		case string:
			s = x
		case float64:
			s = strconv.FormatFloat(x, 'f', -1, 64)
		}
		if s == "" {
			continue
		}
		w.exec(`INSERT OR REPLACE INTO external_ids (owner_type, owner_id, source, value) VALUES (?, ?, ?, ?)`, owner, id, k, s)
	}
}

func (w *writer) altTitles(owner string, id int, a *tmdb.AltTitles) {
	for _, t := range a.All() {
		w.insert("alternative_titles",
			"owner_type", owner, "owner_id", id,
			"iso_3166_1", ns(t.ISO31661), "title", ns(t.Title), "type", ns(t.Type),
		)
	}
}

func (w *writer) translations(owner string, id int, t *tmdb.Translations) {
	if t == nil {
		return
	}
	for _, x := range t.Translations {
		w.insert("translations",
			"owner_type", owner, "owner_id", id,
			"iso_639_1", ns(x.ISO6391), "iso_3166_1", ns(x.ISO31661),
			"name", ns(x.Name), "english_name", ns(x.EnglishName),
			"title", ns(firstNonEmpty(x.Data.Title, x.Data.Name)),
			"overview", ns(x.Data.Overview), "tagline", ns(x.Data.Tagline),
			"homepage", ns(x.Data.Homepage), "runtime", ni(x.Data.Runtime),
		)
	}
}

func (w *writer) related(owner string, id int, relation, defaultType string, p *tmdb.Paged) {
	if p == nil {
		return
	}
	for i, raw := range p.Results {
		var s tmdb.Summary
		if err := json.Unmarshal(raw, &s); err != nil {
			w.fail(err)
			return
		}
		typ := s.MediaType
		if typ == "" {
			typ = defaultType
		}
		w.insert("related_titles",
			"owner_type", owner, "owner_id", id, "relation", relation, "rank", i,
			"target_type", typ, "target_id", s.ID,
			"title", ns(firstNonEmpty(s.Title, s.Name)),
			"original_title", ns(firstNonEmpty(s.OriginalTitle, s.OriginalName)),
			"overview", ns(s.Overview),
			"release_date", ns(firstNonEmpty(s.ReleaseDate, s.FirstAirDate)),
			"poster_path", ns(s.PosterPath), "backdrop_path", ns(s.BackdropPath),
			"popularity", s.Popularity, "vote_average", s.VoteAverage, "vote_count", s.VoteCount,
			"raw_json", string(raw),
		)
	}
}

func (w *writer) watchProviders(owner string, id int, wp *tmdb.WatchProviders) {
	if wp == nil {
		return
	}
	for region, r := range wp.Results {
		for cat, list := range map[string][]tmdb.Provider{
			"flatrate": r.Flatrate, "rent": r.Rent, "buy": r.Buy, "ads": r.Ads, "free": r.Free,
		} {
			for _, p := range list {
				w.insert("watch_providers",
					"owner_type", owner, "owner_id", id, "region", region, "link", ns(r.Link),
					"category", cat, "provider_id", p.ProviderID, "provider_name", ns(p.ProviderName),
					"logo_path", ns(p.LogoPath), "display_priority", p.DisplayPriority,
				)
			}
		}
	}
}

func (w *writer) reviews(owner string, id int, r *tmdb.Reviews) {
	if r == nil {
		return
	}
	for _, x := range r.Results {
		var rating any
		if x.AuthorDetails.Rating != nil {
			rating = *x.AuthorDetails.Rating
		}
		w.exec(`INSERT OR REPLACE INTO reviews (id, owner_type, owner_id, author, author_name, author_username, author_avatar_path, author_rating, content, created_at, updated_at, url)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			x.ID, owner, id, ns(x.Author), ns(x.AuthorDetails.Name), ns(x.AuthorDetails.Username), ns(x.AuthorDetails.AvatarPath),
			rating, ns(x.Content), ns(x.CreatedAt), ns(x.UpdatedAt), ns(x.URL))
	}
}

// ---------------------------------------------------------------- helpers

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ns maps empty strings to NULL.
func ns(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ni maps zero ints to NULL.
func ni(i int) any {
	if i == 0 {
		return nil
	}
	return i
}

// nf maps zero floats to NULL.
func nf(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func jsonText(v any) any {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return nil
	}
	return string(b)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
