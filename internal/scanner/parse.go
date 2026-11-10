package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".webm": true,
	".mov": true, ".avi": true, ".ts": true, ".wmv": true,
}

// Plex local-extras folder names; anything inside them is skipped.
var extrasDirs = map[string]bool{
	"extras": true, "featurettes": true, "behind the scenes": true, "deleted scenes": true,
	"interviews": true, "scenes": true, "shorts": true, "trailers": true, "other": true,
	"samples": true, "sample": true,
}

// Plex local-extras filename suffixes, e.g. "Movie (2000)-trailer.mp4".
var extrasSuffix = regexp.MustCompile(`(?i)-(trailer|behindthescenes|deleted|featurette|interview|scene|short|other|sample)$`)

var (
	reTMDBTag   = regexp.MustCompile(`(?i)\{tmdb-(\d+)\}`)
	reAnyTag    = regexp.MustCompile(`\{[^}]*\}|\[[^\]]*\]`)
	reTitleYear = regexp.MustCompile(`^(.*?)\s*\((\d{4})\)`)
	reEpisode   = regexp.MustCompile(`[Ss](\d{1,3})[ ._-]?[Ee](\d{1,4})`)
)

// Title is a parsed movie or show name.
type Title struct {
	Name   string
	Year   int // 0 if unknown
	TMDBID int // 0 if no {tmdb-ID} tag
}

// Key identifies a title for grouping during a scan.
func (t Title) Key() string {
	if t.TMDBID != 0 {
		return "id:" + strconv.Itoa(t.TMDBID)
	}
	return strings.ToLower(t.Name) + "|" + strconv.Itoa(t.Year)
}

// IsVideo reports whether path has a recognised video extension.
func IsVideo(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

// IsExtra reports whether a file (relative to its library root) is a Plex
// local extra or sample rather than main content.
func IsExtra(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, dir := range parts[:len(parts)-1] {
		if extrasDirs[strings.ToLower(dir)] {
			return true
		}
	}
	base := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))
	if extrasSuffix.MatchString(base) {
		return true
	}
	return strings.EqualFold(base, "sample")
}

// ParseTitle parses "Name (Year) {tmdb-123}" style folder or file names.
func ParseTitle(s string) Title {
	var t Title
	if m := reTMDBTag.FindStringSubmatch(s); m != nil {
		t.TMDBID, _ = strconv.Atoi(m[1])
	}
	s = strings.TrimSpace(reAnyTag.ReplaceAllString(s, ""))
	if m := reTitleYear.FindStringSubmatch(s); m != nil {
		t.Name = cleanName(m[1])
		t.Year, _ = strconv.Atoi(m[2])
		return t
	}
	t.Name = cleanName(s)
	return t
}

// ParseMovie parses a movie file path relative to the movies root. Files in a
// folder ("Title (Year)/Title (Year).mkv") take their title from the
// top-level folder; loose files use their own name.
func ParseMovie(rel string) Title {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	fileTitle := ParseTitle(strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel)))
	if len(parts) == 1 {
		return fileTitle
	}
	t := ParseTitle(parts[0])
	if t.TMDBID == 0 {
		t.TMDBID = fileTitle.TMDBID
	}
	if t.Year == 0 {
		t.Year = fileTitle.Year
	}
	if t.Name == "" {
		t.Name = fileTitle.Name
	}
	return t
}

// Episode is a parsed TV episode file.
type Episode struct {
	Show    Title
	Season  int
	Episode int
}

// ParseEpisode parses a TV file path relative to the TV root:
// "Show (Year) {tmdb-123}/Season 01/Show (Year) - s01e01 - Title.mkv".
// ok is false if the path has no show folder or no SxxEyy marker.
func ParseEpisode(rel string) (ep Episode, ok bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return ep, false
	}
	base := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))
	m := reEpisode.FindStringSubmatch(base)
	if m == nil {
		return ep, false
	}
	ep.Show = ParseTitle(parts[0])
	ep.Season, _ = strconv.Atoi(m[1])
	ep.Episode, _ = strconv.Atoi(m[2])
	return ep, ep.Show.Name != "" || ep.Show.TMDBID != 0
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	// Dotted/underscored names ("The.Matrix") become spaced.
	if !strings.Contains(s, " ") {
		s = strings.NewReplacer(".", " ", "_", " ").Replace(s)
	}
	return strings.Join(strings.Fields(strings.TrimRight(s, " -.")), " ")
}
