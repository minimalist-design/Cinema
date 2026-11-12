package transcode

import (
	"context"
	"database/sql"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"cinema/internal/db"
)

const probeJSON = `{
  "streams": [
    {"index": 0, "codec_type": "video", "codec_name": "h264", "pix_fmt": "yuv420p", "width": 1920, "height": 1080, "disposition": {"default": 1}},
    {"index": 1, "codec_type": "audio", "codec_name": "ac3", "channels": 6, "disposition": {"default": 0}},
    {"index": 2, "codec_type": "audio", "codec_name": "aac", "channels": 2, "disposition": {"default": 1}},
    {"index": 3, "codec_type": "subtitle", "codec_name": "subrip", "disposition": {"default": 1}},
    {"index": 4, "codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}}
  ],
  "format": {"duration": "5400.250000", "start_time": "0.007000"}
}`

func TestParseProbe(t *testing.T) {
	ix, err := parseProbe([]byte(probeJSON))
	if err != nil {
		t.Fatal(err)
	}
	if ix.Duration != 5400.25 || ix.StartTime != 0.007 {
		t.Errorf("duration/start = %v/%v", ix.Duration, ix.StartTime)
	}
	if ix.Video == nil || ix.Video.Index != 0 || !ix.CopyVideo() {
		t.Errorf("video = %+v", ix.Video)
	}
	if ix.Audio == nil || ix.Audio.Index != 2 || !ix.CopyAudio() {
		t.Errorf("audio should be the default AAC track, got %+v", ix.Audio)
	}

	// Cover art only: no usable video.
	ix, err = parseProbe([]byte(`{"streams": [
		{"index": 0, "codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}},
		{"index": 1, "codec_type": "audio", "codec_name": "flac", "disposition": {}}], "format": {"duration": "60"}}`))
	if err != nil || ix.Video != nil || ix.Audio.Index != 1 || ix.CopyAudio() {
		t.Errorf("cover art probe = %+v, %v", ix, err)
	}

	if _, err := parseProbe([]byte(`{"streams": [], "format": {}}`)); err == nil {
		t.Error("expected error for a file without streams")
	}
}

func TestCopyVideo(t *testing.T) {
	for _, tc := range []struct {
		codec, pix string
		want       bool
	}{
		{"h264", "yuv420p", true},
		{"h264", "yuvj420p", true},
		{"h264", "yuv420p10le", false}, // Hi10P isn't decodable by browsers
		{"hevc", "yuv420p", false},
		{"mpeg4", "yuv420p", false},
	} {
		ix := &Index{Video: &Stream{Codec: tc.codec, PixFmt: tc.pix}}
		if got := ix.CopyVideo(); got != tc.want {
			t.Errorf("CopyVideo(%s, %s) = %v", tc.codec, tc.pix, got)
		}
	}
}

func TestParseKeyframes(t *testing.T) {
	out := "0.507000,K__\n6.507000,K__\n6.507000,K__\n3.000000,___\n12.4,K_D\n\nbad\n"
	got := parseKeyframes([]byte(out), 0.5)
	if want := []float64{0.007, 6.007, 11.9}; !slices.Equal(got, want) {
		t.Errorf("keyframes = %v, want %v", got, want)
	}
}

func TestSegments(t *testing.T) {
	encode := &Index{Duration: 20, Video: &Stream{Codec: "hevc"}}
	if got, want := encode.Segments(), []float64{0, 6, 12, 18}; !slices.Equal(got, want) {
		t.Errorf("encode segments = %v, want %v", got, want)
	}
	// The final segment absorbs a remainder under a second.
	encode.Duration = 18.5
	if got, want := encode.Segments(), []float64{0, 6, 12}; !slices.Equal(got, want) {
		t.Errorf("encode segments = %v, want %v", got, want)
	}

	copied := &Index{Duration: 30.5, Video: &Stream{Codec: "h264", PixFmt: "yuv420p"},
		// 0.083: too close to the start; 6.1: too close to 5.9; 30: too close to the end.
		Keyframes: []float64{0, 0.083, 5.9, 6.1, 12, 24, 30}}
	if got, want := copied.Segments(), []float64{0, 5.9, 12, 24}; !slices.Equal(got, want) {
		t.Errorf("copy segments = %v, want %v", got, want)
	}

	audioOnly := &Index{Duration: 12.5, Audio: &Stream{Codec: "flac"}}
	if got, want := audioOnly.Segments(), []float64{0, 6}; !slices.Equal(got, want) {
		t.Errorf("audio-only segments = %v, want %v", got, want)
	}
}

func TestSegmentArgs(t *testing.T) {
	ix, _ := parseProbe([]byte(probeJSON))
	starts := []float64{0, 6, 12.5, 18}
	args := strings.Join(segmentArgs("/m/a.mkv", ix, starts, 0, "/tmp/s"), " ")
	for _, want := range []string{"-copyts -i /m/a.mkv", "-map 0:0 -c:v copy", "-map 0:2 -c:a copy", "-sn",
		"-avoid_negative_ts disabled -output_ts_offset 10.000", "-segment_format_options mpegts_copyts=1:flush_packets=1 -individual_header_trailer 0",
		"-segment_times 6.000,12.500,18.000,9000.250", "-segment_start_number 0",
		"-segment_list pipe:1 -segment_list_type flat", "/tmp/s/%d.ts"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	if strings.Contains(args, "-ss") {
		t.Errorf("unexpected -ss for the first segment: %s", args)
	}

	// Restarting at a later segment: seek just past its keyframe, and split
	// points become relative to it.
	args = strings.Join(segmentArgs("/m/a.mkv", ix, starts, 2, "/tmp/s"), " ")
	for _, want := range []string{"-ss 12.700 -noaccurate_seek -copyts -i", "-segment_times 5.500,8987.750", "-segment_start_number 2"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}

	ix.Video.Codec, ix.Audio.Codec = "hevc", "dts"
	got := segmentArgs("/m/a.mkv", ix, starts, 1, "/tmp/s")
	args = strings.Join(got, " ")
	for _, want := range []string{"-ss 6.000 -copyts -i", "-c:v libx264", "-pix_fmt yuv420p",
		"-force_key_frames expr:gte(t,n_forced*6)", "-c:a aac -ac 2"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	if strings.Contains(args, "noaccurate_seek") {
		t.Errorf("re-encoded video should seek accurately: %s", args)
	}

	// Audio only.
	args = strings.Join(segmentArgs("/m/a.mkv", &Index{Duration: 60, Audio: &Stream{Index: 0, Codec: "flac"}}, []float64{0}, 0, "/tmp/s"), " ")
	if strings.Contains(args, "-c:v") || !strings.Contains(args, "-map 0:0 -c:a aac") {
		t.Errorf("audio-only args: %s", args)
	}
}

// ---------------------------------------------------------------- ffmpeg

func newTestHLS(t *testing.T) (*Transcoder, *sql.DB, *Store, *HLS) {
	t.Helper()
	tc, err := New("ffmpeg", "ffprobe")
	if err != nil {
		t.Skip("ffmpeg not installed:", err)
	}
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	store := NewStore(database, tc)
	h, err := NewHLS(store, tc, filepath.Join(t.TempDir(), "hls"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return tc, database, store, h
}

// makeMKV generates a test file and registers it in the files table.
func makeMKV(t *testing.T, database *sql.DB, name string, args ...string) (int64, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("ffmpeg", append(append([]string{"-v", "error", "-y"}, args...), path)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate test media (missing encoder?): %v: %s", err, out)
	}
	st, _ := os.Stat(path)
	res, err := database.Exec(`INSERT INTO files (path, library, size, mtime, movie_id, added_at, updated_at)
		VALUES (?, 'movie', ?, ?, NULL, '', '')`, path, st.Size(), st.ModTime().Unix())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id, path
}

// firstVideoPTS returns the earliest video timestamp in a segment and
// whether the segment starts with a keyframe.
func firstVideoPTS(t *testing.T, path string) (float64, bool) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		t.Fatalf("%s has no video", path)
	}
	first := math.Inf(1)
	for _, l := range lines {
		pts, _, _ := strings.Cut(l, ",")
		f, _ := strconv.ParseFloat(pts, 64)
		first = math.Min(first, f)
	}
	return first, strings.HasPrefix(strings.SplitN(lines[0], ",", 2)[1], "K")
}

var (
	// 60s of H.264 with B-frames and a keyframe every 2s, plus 5.1 AC3:
	// video copied, audio transcoded.
	copyArgs = []string{"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24", "-f", "lavfi", "-i", "sine",
		"-t", "60", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "2", "-g", "48", "-keyint_min", "48",
		"-sc_threshold", "0", "-c:a", "ac3", "-ac", "6"}
	// 20s of HEVC + Opus: both re-encoded.
	encodeArgs = []string{"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24", "-f", "lavfi", "-i", "sine",
		"-t", "20", "-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error", "-c:a", "libopus"}
)

// TestHLSSegmentsLineUp requests segments out of order, which restarts
// ffmpeg at different points, and checks every segment starts on a keyframe
// at the time the playlist says.
func TestHLSSegmentsLineUp(t *testing.T) {
	_, database, store, h := newTestHLS(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		args []string
		copy bool
	}{
		{"copy.mkv", copyArgs, true},
		{"encode.mkv", encodeArgs, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, _ := makeMKV(t, database, tc.name, tc.args...)
			_, ix, err := store.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if ix.CopyVideo() != tc.copy || ix.CopyAudio() {
				t.Fatalf("index = %+v video=%+v audio=%+v", ix, ix.Video, ix.Audio)
			}
			starts := ix.Segments()
			if tc.copy && len(ix.Keyframes) < 9 {
				t.Errorf("keyframes = %v", ix.Keyframes)
			}

			pl, err := h.Playlist(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(pl), "#EXT-X-ENDLIST") || strings.Count(string(pl), "#EXTINF") != len(starts) {
				t.Errorf("playlist:\n%s", pl)
			}

			last := len(starts) - 1
			for _, n := range []int{last, 0, 1, 2, last / 2, last/2 + 1} {
				path, err := h.Segment(ctx, id, "session-"+strings.TrimSuffix(tc.name, ".mkv"), n)
				if err != nil {
					t.Fatalf("segment %d: %v", n, err)
				}
				pts, key := firstVideoPTS(t, path)
				// Segments are offset by tsOffset. Allow one frame of slack,
				// or a few for the first segment, where the first frame
				// can be delayed by B-frames.
				slack := 0.05
				if n == 0 {
					slack = 0.15
				}
				if want := starts[n] + ix.StartTime + tsOffset; !key || pts < want-0.01 || pts > want+slack {
					t.Errorf("segment %d starts at %.3f (keyframe %v), want %.3f", n, pts, key, want)
				}
			}
		})
	}
}

func TestHLSThrottlesAndCleansUp(t *testing.T) {
	_, database, store, h := newTestHLS(t)
	ctx := context.Background()
	id, _ := makeMKV(t, database, "copy.mkv", copyArgs...)
	_, ix, err := store.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ix.Segments()); n < aheadSegments+4 {
		t.Fatalf("test file too short: %d segments", n)
	}

	if _, err := h.Segment(ctx, id, "throttle-session", 0); err != nil {
		t.Fatal(err)
	}
	s := h.sessions["throttle-session"]
	// Copying runs far faster than real time, so without throttling ffmpeg
	// would finish the file almost at once.
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		paused, next := s.job.paused, s.job.completed
		s.mu.Unlock()
		if paused {
			// Paused as soon as more than aheadSegments are ready past segment 0.
			if next > aheadSegments+3 {
				t.Errorf("paused only after %d segments", next)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ffmpeg never paused (next=%d)", next)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Requesting the next segment resumes it.
	if _, err := h.Segment(ctx, id, "throttle-session", 1); err != nil {
		t.Fatal(err)
	}

	if _, err := h.Segment(ctx, id, "../escape", 0); err != ErrBadSession {
		t.Errorf("bad session id: %v", err)
	}
	if _, err := h.Segment(ctx, id+1, "throttle-session", 0); err == nil {
		t.Error("session reused for another file")
	}
	if _, err := h.Segment(ctx, id, "throttle-session", 999); err != ErrBadSegment {
		t.Errorf("segment out of range: %v", err)
	}

	h.Close()
	if _, err := os.Stat(h.dir); !os.IsNotExist(err) {
		t.Errorf("segment dir not removed: %v", err)
	}
}

func TestStore(t *testing.T) {
	_, database, store, _ := newTestHLS(t)
	ctx := context.Background()
	id, path := makeMKV(t, database, "copy.mkv", copyArgs...)
	// Linked files are indexed by IndexLibrary; non-MKVs are skipped.
	if _, err := database.Exec(`INSERT INTO movies (id, title, raw_json, fetched_at) VALUES (1, 'x', '{}', '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE files SET movie_id = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	database.Exec(`INSERT INTO files (path, library, size, mtime, movie_id, added_at, updated_at) VALUES ('/x.mp4', 'movie', 1, 1, 1, '', '')`)

	n, errs := store.IndexLibrary(ctx)
	if n != 1 || len(errs) != 0 {
		t.Fatalf("IndexLibrary = %d, %v", n, errs)
	}
	if n, errs = store.IndexLibrary(ctx); n != 0 || len(errs) != 0 {
		t.Errorf("second IndexLibrary = %d, %v", n, errs)
	}

	// A changed file is re-indexed.
	future := time.Now().Add(time.Hour)
	os.Chtimes(path, future, future)
	if n, _ = store.IndexLibrary(ctx); n != 1 {
		t.Errorf("IndexLibrary after change = %d", n)
	}

	if _, _, err := store.Get(ctx, 9999); err != ErrNotFound {
		t.Errorf("Get(unknown) = %v", err)
	}
	os.Remove(path)
	if _, _, err := store.Get(ctx, id); !os.IsNotExist(err) {
		t.Errorf("Get(deleted file) = %v", err)
	}
}

func TestHLSIdleSessionsExpire(t *testing.T) {
	defer func(d time.Duration) { sessionIdle = d }(sessionIdle)
	sessionIdle = 500 * time.Millisecond
	_, database, _, h := newTestHLS(t)
	id, _ := makeMKV(t, database, "copy.mkv", copyArgs...)
	if _, err := h.Segment(context.Background(), id, "idle-session", 0); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	dir := h.sessions["idle-session"].dir
	h.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.mu.Lock()
		_, alive := h.sessions["idle-session"]
		h.mu.Unlock()
		_, err := os.Stat(dir)
		if !alive && os.IsNotExist(err) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("idle session not cleaned up (alive=%v, dir err=%v)", alive, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestHLSSeekSupersedesWaiting checks that a request waiting for a segment
// gives up, rather than restarting ffmpeg back, when a seek moves the
// session elsewhere.
func TestHLSSeekSupersedesWaiting(t *testing.T) {
	_, database, store, h := newTestHLS(t)
	ctx := context.Background()
	id, _ := makeMKV(t, database, "encode.mkv", encodeArgs...)
	if _, _, err := store.Get(ctx, id); err != nil {
		t.Fatal(err)
	}
	s, err := h.session(ctx, id, "seek-session")
	if err != nil {
		t.Fatal(err)
	}
	// Start a job at 0 by hand and pause it, so a request for segment 1
	// waits.
	s.mu.Lock()
	j, err := s.startLocked(0)
	if err == nil {
		s.job = j
		pause(j.proc)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	waiting := make(chan error, 1)
	go func() {
		_, err := h.Segment(ctx, id, "seek-session", 1)
		waiting <- err
	}()
	time.Sleep(200 * time.Millisecond)
	// A seek to the end restarts ffmpeg there.
	last := len(s.starts) - 1
	if _, err := h.Segment(ctx, id, "seek-session", last); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-waiting:
		if err != ErrSuperseded {
			t.Errorf("waiting request: %v, want ErrSuperseded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waiting request never returned")
	}
	s.mu.Lock()
	first := s.job.first
	s.mu.Unlock()
	if first != last {
		t.Errorf("job restarted at %d, want %d", first, last)
	}
}

// TestHLSSegmentsCompleteWhenServed checks that a segment is fully written
// when Segment returns it, by comparing its size then with its size once
// ffmpeg has exited.
func TestHLSSegmentsCompleteWhenServed(t *testing.T) {
	_, database, _, h := newTestHLS(t)
	ctx := context.Background()
	id, _ := makeMKV(t, database, "copy.mkv", copyArgs...)
	sizes := map[string]int64{}
	for n := 0; ; n++ {
		path, err := h.Segment(ctx, id, "complete-session", n)
		if err == ErrBadSegment {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		sizes[path] = st.Size()
	}
	h.mu.Lock()
	s := h.sessions["complete-session"]
	h.mu.Unlock()
	s.mu.Lock()
	j := s.job
	s.mu.Unlock()
	<-j.done
	for path, size := range sizes {
		if st, err := os.Stat(path); err != nil || st.Size() != size {
			t.Errorf("%s was %d bytes when served, %v later (%v)", filepath.Base(path), size, st.Size(), err)
		}
	}
}
