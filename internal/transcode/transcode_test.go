package transcode

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
	info, err := parseProbe([]byte(probeJSON))
	if err != nil {
		t.Fatal(err)
	}
	if info.Duration != 5400.25 || info.StartTime != 0.007 {
		t.Errorf("duration/start = %v/%v", info.Duration, info.StartTime)
	}
	if info.Video == nil || info.Video.Index != 0 || !info.CopyVideo() {
		t.Errorf("video = %+v", info.Video)
	}
	if info.Audio == nil || info.Audio.Index != 2 || !info.CopyAudio() {
		t.Errorf("audio should be the default AAC track, got %+v", info.Audio)
	}

	// Cover art only: no usable video.
	info, err = parseProbe([]byte(`{"streams": [
		{"index": 0, "codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}},
		{"index": 1, "codec_type": "audio", "codec_name": "flac", "disposition": {}}], "format": {"duration": "60"}}`))
	if err != nil || info.Video != nil || info.Audio.Index != 1 || info.CopyAudio() {
		t.Errorf("cover art probe = %+v, %v", info, err)
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
		info := &Info{Video: &Stream{Codec: tc.codec, PixFmt: tc.pix}}
		if got := info.CopyVideo(); got != tc.want {
			t.Errorf("CopyVideo(%s, %s) = %v", tc.codec, tc.pix, got)
		}
	}
}

func TestArgs(t *testing.T) {
	info, _ := parseProbe([]byte(probeJSON))
	args := strings.Join(Args("/m/a.mkv", info, 0), " ")
	for _, want := range []string{"-i /m/a.mkv", "-map 0:0 -c:v copy", "-map 0:2 -c:a copy", "-sn", "frag_keyframe+empty_moov", "-f mp4 pipe:1"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	if strings.Contains(args, "-ss") {
		t.Errorf("unexpected -ss at start 0: %s", args)
	}

	info.Video.Codec, info.Audio.Codec = "hevc", "dts"
	got := Args("/m/a.mkv", info, 61.5)
	args = strings.Join(got, " ")
	for _, want := range []string{"-ss 61.500 -i /m/a.mkv", "-c:v libx264", "-pix_fmt yuv420p", "-c:a aac -ac 2"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	if i := slices.Index(got, "-ss"); i < 0 || i > slices.Index(got, "-i") {
		t.Errorf("-ss must be an input option: %s", args)
	}

	// Audio-only file.
	args = strings.Join(Args("/m/a.mkv", &Info{Audio: &Stream{Index: 0, Codec: "flac"}}, 0), " ")
	if strings.Contains(args, "-c:v") || !strings.Contains(args, "-map 0:0 -c:a aac") {
		t.Errorf("audio-only args: %s", args)
	}
}

// TestFFmpeg runs the real binaries against a generated MKV.
func TestFFmpeg(t *testing.T) {
	tc, err := New("ffmpeg", "ffprobe")
	if err != nil {
		t.Skip("ffmpeg not installed:", err)
	}
	ctx := context.Background()
	dir := t.TempDir()

	// 12s of H.264 (keyframe every 5s) + AC3: video is copied, audio transcoded.
	copyMKV := filepath.Join(dir, "copy.mkv")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24", "-f", "lavfi", "-i", "sine",
		"-t", "12", "-c:v", "libx264", "-g", "120", "-keyint_min", "120", "-sc_threshold", "0", "-c:a", "ac3", copyMKV)
	info, err := tc.Probe(ctx, copyMKV)
	if err != nil {
		t.Fatal(err)
	}
	if !info.CopyVideo() || info.CopyAudio() || info.Duration < 11.9 || info.Duration > 12.1 {
		t.Fatalf("probe = %+v video=%+v audio=%+v", info, info.Video, info.Audio)
	}
	if cached, _ := tc.Probe(ctx, copyMKV); cached != info {
		t.Error("probe result not cached")
	}
	if got := tc.StartFor(ctx, copyMKV, info, 7); got < 4.99 || got > 5.01 {
		t.Errorf("StartFor(7) = %v, want the keyframe at 5", got)
	}
	if got := tc.StartFor(ctx, copyMKV, info, 0); got != 0 {
		t.Errorf("StartFor(0) = %v", got)
	}
	checkOutput(t, tc, copyMKV, info, 5, 7, "h264", "aac")

	// HEVC + Opus: both re-encoded, and seeking is exact.
	encodeMKV := filepath.Join(dir, "encode.mkv")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24", "-f", "lavfi", "-i", "sine",
		"-t", "6", "-c:v", "libx265", "-x265-params", "log-level=error", "-c:a", "libopus", encodeMKV)
	info, err = tc.Probe(ctx, encodeMKV)
	if err != nil {
		t.Fatal(err)
	}
	if info.CopyVideo() || info.CopyAudio() {
		t.Fatalf("hevc/opus should be re-encoded: %+v %+v", info.Video, info.Audio)
	}
	if got := tc.StartFor(ctx, encodeMKV, info, 2.5); got != 2.5 {
		t.Errorf("StartFor(2.5) = %v, want exact", got)
	}
	checkOutput(t, tc, encodeMKV, info, 2.5, 3.5, "h264", "aac")
}

// checkOutput transcodes from start and checks the result is an MP4 with
// the expected codecs and roughly duration-start seconds long.
func checkOutput(t *testing.T, tc *Transcoder, path string, info *Info, start, wantDuration float64, vcodec, acodec string) {
	t.Helper()
	var out bytes.Buffer
	if err := tc.Transcode(context.Background(), &out, path, info, start); err != nil {
		t.Fatal(err)
	}
	if out.Len() < 1000 || !bytes.Contains(out.Bytes()[:64], []byte("ftyp")) {
		t.Fatalf("output doesn't look like MP4 (%d bytes)", out.Len())
	}
	mp4 := filepath.Join(t.TempDir(), "out.mp4")
	if err := writeFile(mp4, out.Bytes()); err != nil {
		t.Fatal(err)
	}
	got, err := (&Transcoder{ffprobe: tc.ffprobe, cache: map[string]cacheEntry{}}).Probe(context.Background(), mp4)
	if err != nil {
		t.Fatal(err)
	}
	if got.Video.Codec != vcodec || got.Audio.Codec != acodec {
		t.Errorf("output codecs = %s/%s", got.Video.Codec, got.Audio.Codec)
	}
	if d := got.Duration - wantDuration; d < -0.3 || d > 0.3 {
		t.Errorf("output duration = %v, want ~%v", got.Duration, wantDuration)
	}
}

func ffmpeg(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate test media (missing encoder?): %v: %s", err, out)
	}
}

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }
