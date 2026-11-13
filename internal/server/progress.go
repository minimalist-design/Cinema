package server

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// resumeMin is how far into a title playback must get (in seconds) to be
// remembered: opening something for a few seconds doesn't count.
const resumeMin = 10.0

// finished reports whether a title watched up to position counts as seen:
// the last 2 minutes (credits) or 5% of it may be skipped.
func finished(position, duration float64) bool {
	remaining := duration - position
	return remaining <= math.Min(120, duration*0.1) || position >= duration*0.95
}

// saveProgress records how far a movie or episode was watched.
func (s *Server) saveProgress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string  `json:"kind"`
		ID       int     `json:"id"`
		Position float64 `json:"position"`
		Duration float64 `json:"duration"`
		// The player reached the end.
		Ended bool `json:"ended"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	table := map[string]string{"movie": "movies", "episode": "episodes"}[body.Kind]
	if table == "" || body.ID <= 0 || !(body.Duration > 0) || math.IsInf(body.Duration, 0) || math.IsNaN(body.Position) {
		writeError(w, http.StatusBadRequest, "invalid progress")
		return
	}
	var one int
	if err := s.db.QueryRowContext(r.Context(), "SELECT 1 FROM "+table+" WHERE id = ?", body.ID).Scan(&one); notFound(w, err) {
		return
	}
	pos := math.Max(0, math.Min(body.Position, body.Duration))
	if body.Ended {
		pos = body.Duration
	}
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO watch_progress (kind, item_id, position, duration, finished, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, item_id) DO UPDATE SET position = excluded.position, duration = excluded.duration,
			finished = excluded.finished, updated_at = excluded.updated_at`,
		body.Kind, body.ID, pos, body.Duration, body.Ended || finished(pos, body.Duration), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resumeAt returns where to resume a title, or nil to start from the
// beginning (never played, barely started, or finished).
func (s *Server) resumeAt(ctx context.Context, kind string, id int) (*float64, error) {
	var pos float64
	var done bool
	err := s.db.QueryRowContext(ctx, `SELECT position, finished FROM watch_progress WHERE kind = ? AND item_id = ?`, kind, id).Scan(&pos, &done)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (done || pos < resumeMin)) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pos, nil
}

// ---------------------------------------------------------------- home

type continueEpisode struct {
	// "resume" an unfinished episode, or play the "next" one.
	Reason   string   `json:"reason"`
	Position *float64 `json:"position"`
	Duration *float64 `json:"duration"`
	Episode  struct {
		ID            int     `json:"id"`
		SeasonNumber  int     `json:"seasonNumber"`
		EpisodeNumber int     `json:"episodeNumber"`
		Name          *string `json:"name"`
		Overview      *string `json:"overview"`
		AirDate       *string `json:"airDate"`
		StillURL      *string `json:"stillUrl"`
	} `json:"episode"`
	Show struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		PosterURL   *string `json:"posterUrl"`
		BackdropURL *string `json:"backdropUrl"`
	} `json:"show"`
}

type suggestion struct {
	libraryItem
	// How much of it was watched (0 to 1), for movies already started.
	Progress *float64 `json:"progress"`
}

const (
	suggestionCount = 8
	backdropSize    = "w780"
)

// home returns what the home page offers at the top: the TV episode to
// continue with, and movies not watched yet.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	cont, err := s.continueWatching(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	sugg, err := s.movieSuggestions(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]any{"continue": cont, "suggestions": sugg})
}

// continueWatching picks the most recently watched show that has something
// left to watch: its last played episode if unfinished, or else the next
// episode in the library not watched yet.
func (s *Server) continueWatching(ctx context.Context) (*continueEpisode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.show_id FROM watch_progress p JOIN episodes e ON e.id = p.item_id
		WHERE p.kind = 'episode'
		GROUP BY e.show_id
		ORDER BY MAX(p.updated_at) DESC
		LIMIT 20`)
	if err != nil {
		return nil, err
	}
	var shows []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		shows = append(shows, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, showID := range shows {
		var epID, season, number int
		var pos, dur float64
		var done bool
		err := s.db.QueryRowContext(ctx, `
			SELECT p.item_id, p.position, p.duration, p.finished, e.season_number, e.episode_number
			FROM watch_progress p JOIN episodes e ON e.id = p.item_id
			WHERE p.kind = 'episode' AND e.show_id = ?
			ORDER BY p.updated_at DESC LIMIT 1`, showID).Scan(&epID, &pos, &dur, &done, &season, &number)
		if err != nil {
			return nil, err
		}
		if !done && s.hasFile(ctx, epID) {
			c, err := s.continueItem(ctx, epID, "resume")
			if err == nil {
				c.Position, c.Duration = &pos, &dur
			}
			return c, err
		}
		var next int
		err = s.db.QueryRowContext(ctx, `
			SELECT e.id FROM episodes e
			WHERE e.show_id = ?
			  AND EXISTS (SELECT 1 FROM files f WHERE f.episode_id = e.id)
			  AND (e.season_number > ? OR (e.season_number = ? AND e.episode_number > ?))
			  AND NOT EXISTS (SELECT 1 FROM watch_progress p WHERE p.kind = 'episode' AND p.item_id = e.id AND p.finished)
			ORDER BY e.season_number, e.episode_number LIMIT 1`, showID, season, season, number).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) {
			continue // watched to the end of what's in the library
		} else if err != nil {
			return nil, err
		}
		return s.continueItem(ctx, next, "next")
	}
	return nil, nil
}

func (s *Server) hasFile(ctx context.Context, episodeID int) bool {
	var one int
	return s.db.QueryRowContext(ctx, `SELECT 1 FROM files WHERE episode_id = ? LIMIT 1`, episodeID).Scan(&one) == nil
}

func (s *Server) continueItem(ctx context.Context, episodeID int, reason string) (*continueEpisode, error) {
	c := &continueEpisode{Reason: reason}
	var name, overview, air, still, poster, backdrop sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.season_number, e.episode_number, e.name, e.overview, e.air_date, e.still_path,
		       s.id, s.name, s.poster_path, s.backdrop_path
		FROM episodes e JOIN tv_shows s ON s.id = e.show_id
		WHERE e.id = ?`, episodeID).Scan(&c.Episode.ID, &c.Episode.SeasonNumber, &c.Episode.EpisodeNumber,
		&name, &overview, &air, &still, &c.Show.ID, &c.Show.Name, &poster, &backdrop)
	if err != nil {
		return nil, err
	}
	c.Episode.Name, c.Episode.Overview, c.Episode.AirDate = nullStr(name), nullStr(overview), nullStr(air)
	c.Episode.StillURL = imageURL(backdropSize, still)
	c.Show.PosterURL, c.Show.BackdropURL = imageURL(posterSize, poster), imageURL(backdropSize, backdrop)
	return c, nil
}

// movieSuggestions returns movies in the library not watched to the end:
// the ones already started first (most recent first), then the others in
// an order that changes once a day.
func (s *Server) movieSuggestions(ctx context.Context) ([]suggestion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.title, m.release_date, m.poster_path, MAX(f.added_at), p.position, p.duration, p.updated_at
		FROM movies m
		JOIN files f ON f.movie_id = m.id
		LEFT JOIN watch_progress p ON p.kind = 'movie' AND p.item_id = m.id
		WHERE p.finished IS NULL OR NOT p.finished
		GROUP BY m.id
		ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		suggestion
		updated string
	}
	var started, unseen []row
	for rows.Next() {
		var it row
		var date, poster, added, updated sql.NullString
		var pos, dur sql.NullFloat64
		if err := rows.Scan(&it.ID, &it.Title, &date, &poster, &added, &pos, &dur, &updated); err != nil {
			return nil, err
		}
		it.Year, it.PosterURL, it.AddedAt = year(date), imageURL(posterSize, poster), nullStr(added)
		if pos.Valid && pos.Float64 >= resumeMin && dur.Float64 > 0 {
			p := pos.Float64 / dur.Float64
			it.Progress, it.updated = &p, updated.String
			started = append(started, it)
		} else {
			unseen = append(unseen, it)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(started, func(i, j int) bool { return started[i].updated > started[j].updated })
	day, _ := strconv.ParseUint(time.Now().Format("20060102"), 10, 64)
	rng := rand.New(rand.NewPCG(day, 0x63696e656d61))
	rng.Shuffle(len(unseen), func(i, j int) { unseen[i], unseen[j] = unseen[j], unseen[i] })

	out := []suggestion{}
	for _, it := range append(started, unseen...) {
		if len(out) == suggestionCount {
			break
		}
		out = append(out, it.suggestion)
	}
	return out, nil
}
