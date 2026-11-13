// Package subtitles lists a video's text subtitle tracks, both built into
// the file and in side files next to it, picks the one to show when
// playback starts, and converts them to WebVTT for the browser.
//
// Image-based subtitles (PGS, VobSub) can't be converted to text, so they're
// left out.
package subtitles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"cinema/internal/scanner"
	"cinema/internal/transcode"
)

// Track is a subtitle track as the player sees it.
type Track struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Language        string `json:"language"` // BCP 47, e.g. "en"; "" if unknown
	Forced          bool   `json:"forced"`
	HearingImpaired bool   `json:"hearingImpaired"`
	External        bool   `json:"external"` // a side file rather than built in
	// Default marks the track to show when playback starts (at most one).
	Default bool `json:"default"`

	title   string // from the file, if any
	flagged bool   // flagged default in the file, or a ".default" side file
}

// ErrNotFound is returned for a track that doesn't exist.
var ErrNotFound = errors.New("subtitle track not found")

// textCodecs are subtitle codecs ffmpeg can convert to WebVTT.
var textCodecs = map[string]bool{
	"subrip": true, "srt": true, "ass": true, "ssa": true, "webvtt": true, "mov_text": true, "text": true,
}

// sideExts are the side file formats that are picked up.
var sideExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true}

// Library gives ExtractLibrary the library's files and their indexes (see
// transcode.Store).
type Library interface {
	LinkedFiles(ctx context.Context) ([]transcode.File, error)
	Get(ctx context.Context, fileID int64) (string, *transcode.Index, error)
}

// Service finds, converts and caches subtitles.
type Service struct {
	ffmpeg   string
	cacheDir string
	lib      Library

	prefsMu sync.RWMutex
	prefs   []language.Base

	ctx    context.Context
	cancel context.CancelFunc

	mu         sync.Mutex
	extracting map[string]*extraction
}

type extraction struct {
	done chan struct{}
	err  error
}

// New returns a Service that runs the given ffmpeg binary and caches
// extracted tracks under cacheDir. prefs are the preferred subtitle
// languages in order (any ISO 639 code, e.g. "en", "eng" or "fre"). With no
// preferences, the track flagged default in the file is shown. lib is used
// by ExtractLibrary and may be nil if that isn't needed.
func New(ffmpeg, cacheDir string, prefs []string, lib Library) (*Service, error) {
	s := &Service{ffmpeg: ffmpeg, cacheDir: cacheDir, lib: lib, extracting: map[string]*extraction{}}
	// Leftovers of extractions interrupted by a crash or shutdown.
	if tmp, _ := filepath.Glob(filepath.Join(cacheDir, "tmp-*")); len(tmp) > 0 {
		for _, t := range tmp {
			os.RemoveAll(t)
		}
	}
	if err := s.SetPreferences(prefs); err != nil {
		return nil, err
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s, nil
}

// SetPreferences replaces the preferred subtitle languages (see New).
func (s *Service) SetPreferences(prefs []string) error {
	bases, err := parseLanguages(prefs)
	if err != nil {
		return err
	}
	s.prefsMu.Lock()
	s.prefs = bases
	s.prefsMu.Unlock()
	return nil
}

// ValidateLanguages checks that every code is a language SetPreferences
// accepts.
func ValidateLanguages(codes []string) error {
	_, err := parseLanguages(codes)
	return err
}

func parseLanguages(codes []string) ([]language.Base, error) {
	var out []language.Base
	for _, c := range codes {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		b, ok := parseLanguage(c)
		if !ok {
			return nil, fmt.Errorf("unknown subtitle language %q", c)
		}
		out = append(out, b)
	}
	return out, nil
}

// Close stops running extractions.
func (s *Service) Close() { s.cancel() }

// parseLanguage accepts ISO 639-1/2 codes ("en", "eng", "fre", "fra") and
// BCP 47 tags ("pt-BR"). "und" counts as unknown.
func parseLanguage(code string) (language.Base, bool) {
	tag, err := language.Parse(code)
	if err != nil {
		return language.Base{}, false
	}
	b, conf := tag.Base()
	if conf == language.No || b.String() == "und" {
		return language.Base{}, false
	}
	return b, true
}

// ---------------------------------------------------------------- listing

// Tracks lists the text subtitles of the video at videoPath: side files
// first, then the tracks built into the file (from its index, which may be
// nil), with one marked Default according to the language preferences.
func (s *Service) Tracks(videoPath string, ix *transcode.Index) []Track {
	tracks := sideTracks(videoPath)
	if ix != nil {
		for _, st := range ix.Subtitles {
			if !textCodecs[st.Codec] {
				continue
			}
			t := Track{
				ID:              "embedded-" + strconv.Itoa(st.Index),
				Forced:          st.Disposition["forced"] == 1,
				HearingImpaired: st.Disposition["hearing_impaired"] == 1,
				title:           strings.TrimSpace(st.Tags["title"]),
				flagged:         st.Disposition["default"] == 1,
			}
			if b, ok := parseLanguage(st.Tags["language"]); ok {
				t.Language = b.String()
			}
			tracks = append(tracks, t)
		}
	}
	label(tracks)
	if i := s.pickDefault(tracks); i >= 0 {
		tracks[i].Default = true
	}
	return tracks
}

// sideTracks finds side files for a video: files in the same directory named
// after it, plus optional language and flags, Plex style:
//
//	Movie (2020).srt
//	Movie (2020).en.srt
//	Movie (2020).eng.forced.srt
//	Movie (2020).en.sdh.srt
func sideTracks(videoPath string) []Track {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Other videos in the directory whose names extend this one's
	// ("Movie" and "Movie.Part2"): their side files aren't ours.
	var longer []string
	for _, e := range entries {
		n := e.Name()
		if b := strings.TrimSuffix(n, filepath.Ext(n)); !e.IsDir() && scanner.IsVideo(n) && b != base && strings.HasPrefix(b, base+".") {
			longer = append(longer, b+".")
		}
	}

	var tracks []Track
next:
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !sideExts[strings.ToLower(filepath.Ext(name))] || !strings.HasPrefix(name, base+".") {
			continue
		}
		for _, l := range longer {
			if strings.HasPrefix(name, l) {
				continue next
			}
		}
		t := Track{ID: "file-" + strings.TrimPrefix(name, base+"."), External: true}
		// The parts between the video's name and the extension.
		middle := strings.TrimSuffix(strings.TrimPrefix(name, base), filepath.Ext(name))
		for _, p := range strings.Split(middle, ".") {
			switch strings.ToLower(p) {
			case "forced":
				t.Forced = true
			case "sdh", "cc":
				t.HearingImpaired = true
			case "default":
				t.flagged = true
			case "hi":
				// "hi" is also Hindi: it means hearing impaired only after
				// a language ("en.hi").
				if t.Language != "" {
					t.HearingImpaired = true
				} else {
					t.Language = "hi"
				}
			default:
				if b, ok := parseLanguage(p); ok && t.Language == "" {
					t.Language = b.String()
				}
			}
		}
		tracks = append(tracks, t)
	}
	sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].ID < tracks[j].ID })
	return tracks
}

// label names tracks after their language and flags ("English (SDH)"), and
// tells apart tracks that would get the same name: by file name for side
// files, by title for built-in tracks, and by number as a last resort.
func label(tracks []Track) {
	names := display.English.Languages()
	for i := range tracks {
		t := &tracks[i]
		if t.Language != "" {
			t.Label = names.Name(language.MustParseBase(t.Language))
		}
		switch {
		case t.Label != "":
		case t.title != "":
			t.Label = t.title
		default:
			t.Label = "Unknown language"
		}
		if t.Forced {
			t.Label += " (forced)"
		}
		if t.HearingImpaired {
			t.Label += " (SDH)"
		}
	}

	count := func(external bool) map[string]int {
		c := map[string]int{}
		for _, t := range tracks {
			if !external || t.External {
				c[t.Label]++
			}
		}
		return c
	}
	all, files := count(false), count(true)
	for i := range tracks {
		t := &tracks[i]
		switch {
		case all[t.Label] < 2:
		case t.External && files[t.Label] > 1:
			t.Label += " (" + strings.TrimPrefix(t.ID, "file-") + ")"
		case t.External:
			t.Label += " (file)"
		case t.title != "" && t.title != t.Label:
			t.Label += " – " + t.title
		default:
			t.Label += " (built-in)"
		}
	}
	all, seen := count(false), map[string]int{}
	for i := range tracks {
		if t := &tracks[i]; all[t.Label] > 1 {
			seen[t.Label]++
			t.Label += " #" + strconv.Itoa(seen[t.Label])
		}
	}
}

// pickDefault returns the index of the track to show at start, or -1.
//
// With language preferences, it's the first preferred language that has a
// track: a full one rather than SDH, and forced only if that's all there is.
// Side files come first, so a downloaded subtitle beats the built-in one.
// Without preferences, it's the track flagged default in the file.
func (s *Service) pickDefault(tracks []Track) int {
	s.prefsMu.RLock()
	prefs := s.prefs
	s.prefsMu.RUnlock()
	if len(prefs) == 0 {
		for i, t := range tracks {
			if t.flagged {
				return i
			}
		}
		return -1
	}
	for _, pref := range prefs {
		lang := pref.String()
		for _, ok := range []func(Track) bool{
			func(t Track) bool { return !t.Forced && !t.HearingImpaired },
			func(t Track) bool { return !t.Forced },
			func(t Track) bool { return true },
		} {
			for i, t := range tracks {
				if t.Language == lang && ok(t) {
					return i
				}
			}
		}
	}
	return -1
}

// ---------------------------------------------------------------- converting

// WebVTT returns a track converted to WebVTT. Built-in tracks are extracted
// once and cached; that means reading through the whole video file, so the
// first request for a file's subtitles can take a while.
func (s *Service) WebVTT(ctx context.Context, fileID int64, videoPath string, ix *transcode.Index, id string) ([]byte, error) {
	if name, ok := strings.CutPrefix(id, "file-"); ok {
		for _, t := range sideTracks(videoPath) {
			if t.ID == id {
				base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
				return s.convertSideFile(ctx, filepath.Join(filepath.Dir(videoPath), base+"."+name))
			}
		}
		return nil, ErrNotFound
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "embedded-"))
	if err != nil || !strings.HasPrefix(id, "embedded-") || ix == nil {
		return nil, ErrNotFound
	}
	var stream *transcode.Stream
	for _, st := range ix.Subtitles {
		if st.Index == n && textCodecs[st.Codec] {
			stream = st
		}
	}
	if stream == nil {
		return nil, ErrNotFound
	}
	dir, err := s.extract(ctx, fileID, videoPath, ix)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, strconv.Itoa(n)+".vtt"))
}

// convertSideFile converts a side file to WebVTT. Files that aren't UTF-8
// (or UTF-16 with a byte order mark, which ffmpeg detects) are assumed to
// be Windows-1252, the usual encoding of older European subtitles.
func (s *Service) convertSideFile(ctx context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	utf16 := bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF})
	if !utf16 && !utf8.Valid(data) {
		args = append(args, "-sub_charenc", "CP1252")
	}
	args = append(args, "-i", path, "-f", "webvtt", "pipe:1")
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, s.ffmpeg, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convert %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// extract converts every built-in text track of a file to WebVTT in one
// pass (the file is read once whichever tracks are wanted) and returns the
// directory holding <stream index>.vtt for each. Results are cached per
// file size and modification time.
func (s *Service) extract(ctx context.Context, fileID int64, videoPath string, ix *transcode.Index) (string, error) {
	dir, prefix, cached, err := s.cached(fileID, videoPath)
	if err != nil || cached {
		return dir, err
	}

	s.mu.Lock()
	e := s.extracting[dir]
	if e == nil {
		e = &extraction{done: make(chan struct{})}
		s.extracting[dir] = e
		// Runs on its own context: other requests may be waiting for it,
		// and the result is cached for later ones.
		go func() {
			e.err = s.runExtraction(dir, prefix, videoPath, ix)
			s.mu.Lock()
			delete(s.extracting, dir)
			s.mu.Unlock()
			close(e.done)
		}()
	}
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-e.done:
		if e.err != nil {
			return "", e.err
		}
		return dir, nil
	}
}

// cached returns the cache directory for a file's current version (a
// prefix per file id, then its size and modification time), and whether
// it's already there.
func (s *Service) cached(fileID int64, videoPath string) (dir, prefix string, ok bool, err error) {
	st, err := os.Stat(videoPath)
	if err != nil {
		return "", "", false, err
	}
	prefix = filepath.Join(s.cacheDir, strconv.FormatInt(fileID, 10)+"-")
	dir = prefix + strconv.FormatInt(st.Size(), 10) + "-" + strconv.FormatInt(st.ModTime().Unix(), 10)
	_, err = os.Stat(dir)
	return dir, prefix, err == nil, nil
}

func hasText(ix *transcode.Index) bool {
	for _, st := range ix.Subtitles {
		if textCodecs[st.Codec] {
			return true
		}
	}
	return false
}

// ExtractLibrary extracts the built-in text subtitles of every library file
// that has some and isn't cached yet, so they're ready when the file is
// played. Each file is read in full once. It also deletes cached subtitles
// of files no longer in the library. progress is called before each file
// that may need extracting. It returns how many files it extracted and an
// error per file that failed.
func (s *Service) ExtractLibrary(ctx context.Context, progress func(done, total int)) (extracted int, errs []error) {
	files, err := s.lib.LinkedFiles(ctx)
	if err != nil {
		return 0, []error{err}
	}

	type todo struct {
		id   int64
		path string
		ix   *transcode.Index
	}
	var work []todo
	linked := map[string]bool{}
	for _, f := range files {
		linked[strconv.FormatInt(f.ID, 10)] = true
		// Files that can't be indexed were already reported by indexing.
		if path, ix, err := s.lib.Get(ctx, f.ID); err == nil && hasText(ix) {
			work = append(work, todo{f.ID, path, ix})
		}
	}

	if entries, err := os.ReadDir(s.cacheDir); err == nil {
		for _, e := range entries {
			if id, _, ok := strings.Cut(e.Name(), "-"); ok && id != "tmp" && !linked[id] {
				os.RemoveAll(filepath.Join(s.cacheDir, e.Name()))
			}
		}
	}

	for i, w := range work {
		if ctx.Err() != nil {
			return extracted, append(errs, ctx.Err())
		}
		progress(i, len(work))
		if _, _, ok, err := s.cached(w.id, w.path); err != nil {
			errs = append(errs, err)
			continue
		} else if ok {
			continue
		}
		if _, err := s.extract(ctx, w.id, w.path, w.ix); err != nil {
			errs = append(errs, err)
		} else {
			extracted++
		}
	}
	progress(len(work), len(work))
	return extracted, errs
}

func (s *Service) runExtraction(dir, prefix, videoPath string, ix *transcode.Index) error {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(s.cacheDir, "tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-i", videoPath}
	for _, st := range ix.Subtitles {
		if textCodecs[st.Codec] {
			args = append(args, "-map", "0:"+strconv.Itoa(st.Index), "-f", "webvtt",
				filepath.Join(tmp, strconv.Itoa(st.Index)+".vtt"))
		}
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(s.ctx, s.ffmpeg, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract subtitles from %s: %w: %s", videoPath, err, strings.TrimSpace(stderr.String()))
	}

	// Drop extractions of older versions of the file.
	if old, _ := filepath.Glob(prefix + "*"); len(old) > 0 {
		for _, o := range old {
			os.RemoveAll(o)
		}
	}
	return os.Rename(tmp, dir)
}
