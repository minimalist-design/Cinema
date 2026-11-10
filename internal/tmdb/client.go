// Package tmdb is a minimal client for The Movie Database v3 API.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	DefaultBaseURL      = "https://api.themoviedb.org/3"
	DefaultImageBaseURL = "https://image.tmdb.org/t/p"
)

// ErrNotFound is returned for 404 responses.
var ErrNotFound = errors.New("tmdb: not found")

type Client struct {
	BaseURL      string
	ImageBaseURL string
	token        string
	language     string
	http         *http.Client
	limiter      *rate.Limiter
}

func New(token, language string) *Client {
	return &Client{
		BaseURL:      DefaultBaseURL,
		ImageBaseURL: DefaultImageBaseURL,
		token:        token,
		language:     language,
		http:         &http.Client{Timeout: 30 * time.Second},
		// TMDB allows roughly 50 req/s; stay well under it.
		limiter: rate.NewLimiter(rate.Limit(20), 10),
	}
}

// Get performs a GET against the API and returns the raw body. It retries on
// 429 and 5xx responses and network errors.
func (c *Client) Get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	if q == nil {
		q = url.Values{}
	}
	if c.language != "" && q.Get("language") == "" {
		q.Set("language", c.language)
	}
	u := c.BaseURL + path + "?" + q.Encode()

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			sleep(ctx, backoff(attempt, ""))
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			sleep(ctx, backoff(attempt, ""))
			continue
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("tmdb %s: status %d", path, resp.StatusCode)
			sleep(ctx, backoff(attempt, resp.Header.Get("Retry-After")))
			continue
		default:
			return nil, fmt.Errorf("tmdb %s: status %d: %s", path, resp.StatusCode, truncate(string(body), 300))
		}
	}
	return nil, lastErr
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, v any) ([]byte, error) {
	body, err := c.Get(ctx, path, q)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return body, nil
}

const (
	movieAppend   = "credits,images,videos,external_ids,keywords,release_dates,alternative_titles,translations,recommendations,similar,watch/providers,reviews,lists"
	tvAppend      = "aggregate_credits,credits,images,videos,external_ids,keywords,content_ratings,alternative_titles,translations,recommendations,similar,watch/providers,reviews,episode_groups,screened_theatrically"
	seasonAppend  = "credits,aggregate_credits,images,videos,external_ids,translations"
	episodeAppend = "credits,images,videos,external_ids,translations"
)

func (c *Client) imageLangs() string {
	lang := c.language
	if i := strings.IndexByte(lang, '-'); i > 0 {
		lang = lang[:i]
	}
	if lang == "" || lang == "en" {
		return "en,null"
	}
	return lang + ",en,null"
}

func (c *Client) withAppend(appendTo string) url.Values {
	q := url.Values{}
	q.Set("append_to_response", appendTo)
	q.Set("include_image_language", c.imageLangs())
	return q
}

func (c *Client) Movie(ctx context.Context, id int) (*Movie, error) {
	var m Movie
	raw, err := c.getJSON(ctx, "/movie/"+strconv.Itoa(id), c.withAppend(movieAppend), &m)
	if err != nil {
		return nil, err
	}
	m.Raw = raw
	return &m, nil
}

func (c *Client) Collection(ctx context.Context, id int) (*Collection, error) {
	var col Collection
	raw, err := c.getJSON(ctx, "/collection/"+strconv.Itoa(id), nil, &col)
	if err != nil {
		return nil, err
	}
	col.Raw = raw
	return &col, nil
}

func (c *Client) TV(ctx context.Context, id int) (*TVShow, error) {
	var s TVShow
	raw, err := c.getJSON(ctx, "/tv/"+strconv.Itoa(id), c.withAppend(tvAppend), &s)
	if err != nil {
		return nil, err
	}
	s.Raw = raw
	return &s, nil
}

func (c *Client) Season(ctx context.Context, showID, season int) (*Season, error) {
	var s Season
	path := fmt.Sprintf("/tv/%d/season/%d", showID, season)
	raw, err := c.getJSON(ctx, path, c.withAppend(seasonAppend), &s)
	if err != nil {
		return nil, err
	}
	s.Raw = raw
	return &s, nil
}

func (c *Client) Episode(ctx context.Context, showID, season, episode int) (*Episode, error) {
	var e Episode
	path := fmt.Sprintf("/tv/%d/season/%d/episode/%d", showID, season, episode)
	raw, err := c.getJSON(ctx, path, c.withAppend(episodeAppend), &e)
	if err != nil {
		return nil, err
	}
	e.Raw = raw
	return &e, nil
}

// SearchMovie returns the best match for title (and optional year), or 0.
func (c *Client) SearchMovie(ctx context.Context, title string, year int) (int, error) {
	q := url.Values{}
	q.Set("query", title)
	if year > 0 {
		q.Set("primary_release_year", strconv.Itoa(year))
	}
	id, err := c.searchFirst(ctx, "/search/movie", q)
	if err == nil && id == 0 && year > 0 {
		// Release years are often off by one between sources; retry loosely.
		q.Del("primary_release_year")
		q.Set("year", strconv.Itoa(year))
		id, err = c.searchFirst(ctx, "/search/movie", q)
	}
	return id, err
}

// SearchTV returns the best match for name (and optional first-air year), or 0.
func (c *Client) SearchTV(ctx context.Context, name string, year int) (int, error) {
	q := url.Values{}
	q.Set("query", name)
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	return c.searchFirst(ctx, "/search/tv", q)
}

func (c *Client) searchFirst(ctx context.Context, path string, q url.Values) (int, error) {
	var res struct {
		Results []Summary `json:"results"`
	}
	if _, err := c.getJSON(ctx, path, q, &res); err != nil {
		return 0, err
	}
	if len(res.Results) == 0 {
		return 0, nil
	}
	return res.Results[0].ID, nil
}

// ImageURL builds a CDN URL for an image file path at a given size.
func (c *Client) ImageURL(size, filePath string) string {
	return c.ImageBaseURL + "/" + size + filePath
}

// FetchImage downloads an image from the TMDB image CDN.
func (c *Client) FetchImage(ctx context.Context, size, filePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ImageURL(size, filePath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image %s/%s: status %d", size, filePath, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(retryAfter); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return time.Duration(500*(1<<attempt)) * time.Millisecond
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
