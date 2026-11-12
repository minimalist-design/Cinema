package subtitles

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"cinema/internal/transcode"
)

func touch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// theMovie mirrors a real file: French forced (flagged default), French
// full and English full SRT tracks, plus a PGS track that's left out.
func theMovie() *transcode.Index {
	sub := func(i int, codec, lang, title string, def, forced int) *transcode.Stream {
		return &transcode.Stream{Index: i, Type: "subtitle", Codec: codec,
			Tags:        map[string]string{"language": lang, "title": title},
			Disposition: map[string]int{"default": def, "forced": forced}}
	}
	return &transcode.Index{Subtitles: []*transcode.Stream{
		sub(4, "subrip", "fre", "FR Forced : SRT", 1, 1),
		sub(5, "subrip", "fre", "FR Full : SRT", 0, 0),
		sub(6, "subrip", "eng", "ENG Full : SRT", 0, 0),
		sub(7, "hdmv_pgs_subtitle", "eng", "", 0, 0),
	}}
}

func summary(tracks []Track) string {
	var parts []string
	for _, t := range tracks {
		s := t.ID + "=" + t.Label
		if t.Default {
			s += "*"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func TestTracksPreferences(t *testing.T) {
	video := filepath.Join(t.TempDir(), "Movie (2020).mkv")
	touch(t, video, "")

	for _, tc := range []struct {
		prefs []string
		want  string
	}{
		// No preference: the file's default flag wins.
		{nil, "embedded-4=French (forced)*, embedded-5=French, embedded-6=English"},
		// English preferred over the flagged French forced track.
		{[]string{"eng"}, "embedded-4=French (forced), embedded-5=French, embedded-6=English*"},
		{[]string{"en"}, "embedded-4=French (forced), embedded-5=French, embedded-6=English*"},
		// Falls through to the next preference; full beats forced.
		{[]string{"de", "fra"}, "embedded-4=French (forced), embedded-5=French*, embedded-6=English"},
		// No preferred language available: nothing shown.
		{[]string{"ja"}, "embedded-4=French (forced), embedded-5=French, embedded-6=English"},
	} {
		s, err := New("ffmpeg", t.TempDir(), tc.prefs, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := summary(s.Tracks(video, theMovie())); got != tc.want {
			t.Errorf("prefs %v:\n got %s\nwant %s", tc.prefs, got, tc.want)
		}
	}

	if _, err := New("ffmpeg", t.TempDir(), []string{"english"}, nil); err == nil {
		t.Error("expected an error for an unknown language")
	}
}

func TestSideFiles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie (2020).mkv")
	for _, name := range []string{
		"Movie (2020).mkv",
		"Movie (2020).en.srt",
		"Movie (2020).eng.forced.srt",
		"Movie (2020).en.sdh.srt",
		"Movie (2020).fr.hi.ass",      // hearing impaired after a language
		"Movie (2020).hi.vtt",         // Hindi on its own
		"Movie (2020).srt",            // no language
		"Movie (2020).Part2.mkv",      // another video…
		"Movie (2020).Part2.en.srt",   // …whose side file isn't ours
		"Movie (2020).en.sub",         // not a text format
		"Other Movie.en.srt",          // another video's
		"Movie (2020).en.default.srt", // flagged default
		"Movie (2020)-trailer.en.srt", // not the same base name
	} {
		touch(t, filepath.Join(dir, name), "")
	}
	tracks := sideTracks(video)
	label(tracks)
	want := "file-en.default.srt=English (en.default.srt), file-en.sdh.srt=English (SDH), file-en.srt=English (en.srt), " +
		"file-eng.forced.srt=English (forced), file-fr.hi.ass=French (SDH), file-hi.vtt=Hindi, file-srt=Unknown language"
	if got := summary(tracks); got != want {
		t.Errorf("side files:\n got %s\nwant %s", got, want)
	}

	// Side files come before built-in tracks, so a downloaded English
	// subtitle is preferred over the built-in one.
	s, _ := New("ffmpeg", t.TempDir(), []string{"en"}, nil)
	all := s.Tracks(video, theMovie())
	if all[0].ID != "file-en.default.srt" || !all[0].Default {
		t.Errorf("tracks: %s", summary(all))
	}
	// Without preferences, the ".default" side file is the default.
	s, _ = New("ffmpeg", t.TempDir(), nil, nil)
	if all = s.Tracks(video, theMovie()); !all[0].Default {
		t.Errorf("tracks: %s", summary(all))
	}
}

func TestLabelDuplicates(t *testing.T) {
	tracks := []Track{
		{ID: "a", Language: "en", title: "Commentary"},
		{ID: "b", Language: "en"},
		{ID: "c", Language: "en", External: true},
		{ID: "d", title: "Signs & Songs"},
		{ID: "e", Language: "fr", title: "Full"},
		{ID: "f", Language: "fr", title: "Full"},
	}
	label(tracks)
	if got := summary(tracks); got != "a=English – Commentary, b=English (built-in), c=English (file), d=Signs & Songs, "+
		"e=French – Full #1, f=French – Full #2" {
		t.Errorf("labels: %s", got)
	}
}

// ---------------------------------------------------------------- ffmpeg

func ffmpegOrSkip(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
}

func TestWebVTT(t *testing.T) {
	ffmpegOrSkip(t)
	ctx := context.Background()
	dir := t.TempDir()
	en := filepath.Join(dir, "en.srt")
	touch(t, en, "1\n00:00:01,000 --> 00:00:02,000\nHello there\n\n2\n00:00:05,000 --> 00:00:06,000\n<i>General Kenobi</i>\n")
	ass := filepath.Join(dir, "fr.ass")
	touch(t, ass, "[Script Info]\nScriptType: v4.00+\n\n[V4+ Styles]\n"+
		"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n"+
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1\n\n"+
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
		"Dialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,{\\an8}Bonjour\n")

	video := filepath.Join(dir, "Movie (2020).mkv")
	cmd := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-t", "8", "-i", "testsrc2=size=160x120:rate=24",
		"-i", en, "-i", ass,
		"-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-c:s:0", "srt", "-c:s:1", "ass",
		"-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=fre", video)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate test media: %v: %s", err, out)
	}
	tc, err := transcode.New("ffmpeg", "ffprobe")
	if err != nil {
		t.Skip(err)
	}
	ix, err := tc.BuildIndex(ctx, video)
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Subtitles) != 2 {
		t.Fatalf("subtitle streams = %d", len(ix.Subtitles))
	}

	// A Windows-1252 side file, as old French subtitles often are.
	touch(t, filepath.Join(dir, "Movie (2020).fr.srt"), "1\r\n00:00:01,000 --> 00:00:02,500\r\nBonjour, \xe7a va tr\xe8s bien\r\n")

	cache := filepath.Join(t.TempDir(), "cache")
	s, err := New("ffmpeg", cache, []string{"en"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tracks := s.Tracks(video, ix)
	if got := summary(tracks); got != "file-fr.srt=French (file), embedded-1=English*, embedded-2=French (built-in)" {
		t.Errorf("tracks: %s", got)
	}

	// Concurrent requests share one extraction.
	var wg sync.WaitGroup
	results := make([]string, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := []string{"embedded-1", "embedded-2"}[i%2]
			vtt, err := s.WebVTT(ctx, 7, video, ix, id)
			if err != nil {
				t.Error(err)
			}
			results[i] = string(vtt)
		}()
	}
	wg.Wait()
	if !strings.HasPrefix(results[0], "WEBVTT") || !strings.Contains(results[0], "00:05.000 --> 00:06.000\n<i>General Kenobi</i>") {
		t.Errorf("english:\n%s", results[0])
	}
	if !strings.Contains(results[1], "00:03.000 --> 00:04.000") || !strings.Contains(results[1], "Bonjour") || strings.Contains(results[1], `\an8`) {
		t.Errorf("french (from ASS):\n%s", results[1])
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "7-") {
		t.Errorf("cache entries: %v", entries)
	}

	vtt, err := s.WebVTT(ctx, 7, video, ix, "file-fr.srt")
	if err != nil || !strings.Contains(string(vtt), "Bonjour, ça va très bien") {
		t.Errorf("side file: %v\n%s", err, vtt)
	}

	for _, id := range []string{"embedded-3", "embedded-99", "file-de.srt", "file-../../etc/passwd", "bogus"} {
		if _, err := s.WebVTT(ctx, 7, video, ix, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v, want ErrNotFound", id, err)
		}
	}

	// A changed file is extracted again, replacing the old cache entry.
	later := time.Now().Add(time.Hour)
	os.Chtimes(video, later, later)
	if _, err := s.WebVTT(ctx, 7, video, ix, "embedded-1"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 1 {
		t.Errorf("cache entries after change: %v", entries)
	}
}

// fakeLibrary is a Library over a fixed set of files.
type fakeLibrary map[int64]string

func (l fakeLibrary) LinkedFiles(context.Context) ([]transcode.File, error) {
	var files []transcode.File
	for id, path := range l {
		files = append(files, transcode.File{ID: id, Path: path})
	}
	return files, nil
}

func (l fakeLibrary) Get(ctx context.Context, id int64) (string, *transcode.Index, error) {
	tc, err := transcode.New("ffmpeg", "ffprobe")
	if err != nil {
		return "", nil, err
	}
	ix, err := tc.BuildIndex(ctx, l[id])
	return l[id], ix, err
}

func TestExtractLibrary(t *testing.T) {
	ffmpegOrSkip(t)
	ctx := context.Background()
	dir := t.TempDir()
	srt := filepath.Join(dir, "en.srt")
	touch(t, srt, "1\n00:00:01,000 --> 00:00:02,000\nHello there\n")
	withSubs, without, broken := filepath.Join(dir, "a.mkv"), filepath.Join(dir, "b.mkv"), filepath.Join(dir, "c.mkv")
	for _, args := range [][]string{
		{"-f", "lavfi", "-t", "4", "-i", "testsrc2=size=160x120:rate=24", "-i", srt, "-map", "0", "-map", "1", "-c:v", "libx264", "-c:s", "srt", withSubs},
		{"-f", "lavfi", "-t", "4", "-i", "testsrc2=size=160x120:rate=24", "-c:v", "libx264", without},
	} {
		if out, err := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("generate test media: %v: %s", err, out)
		}
	}
	touch(t, broken, "not a video")

	cache := filepath.Join(t.TempDir(), "cache")
	// Leftovers: an interrupted extraction and a file that left the library.
	os.MkdirAll(filepath.Join(cache, "tmp-123"), 0o755)
	os.MkdirAll(filepath.Join(cache, "99-1-1"), 0o755)
	lib := fakeLibrary{1: withSubs, 2: without, 3: broken}
	s, err := New("ffmpeg", cache, nil, lib)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Join(cache, "tmp-123")); !os.IsNotExist(err) {
		t.Error("interrupted extraction not cleaned up at startup")
	}

	var calls []string
	progress := func(done, total int) { calls = append(calls, strconv.Itoa(done)+"/"+strconv.Itoa(total)) }
	n, errs := s.ExtractLibrary(ctx, progress)
	if n != 1 || len(errs) != 0 {
		t.Fatalf("ExtractLibrary = %d, %v", n, errs)
	}
	// Only the file with text subtitles needs work.
	if got := strings.Join(calls, " "); got != "0/1 1/1" {
		t.Errorf("progress = %s", got)
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "1-") {
		t.Errorf("cache = %v", entries)
	}

	// Playback now finds it cached.
	_, ix, _ := lib.Get(ctx, 1)
	if vtt, err := s.WebVTT(ctx, 1, withSubs, ix, "embedded-1"); err != nil || !strings.Contains(string(vtt), "Hello there") {
		t.Errorf("WebVTT = %q, %v", vtt, err)
	}

	// A second scan has nothing to do.
	if n, errs = s.ExtractLibrary(ctx, func(int, int) {}); n != 0 || len(errs) != 0 {
		t.Errorf("second ExtractLibrary = %d, %v", n, errs)
	}

	// Removed from the library: its cache goes on the next scan.
	delete(lib, 1)
	s.ExtractLibrary(ctx, func(int, int) {})
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Errorf("cache after removal = %v", entries)
	}
}
