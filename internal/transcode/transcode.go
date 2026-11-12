// Package transcode turns files that browsers can't play directly (MKV) into
// a fragmented MP4 stream with ffmpeg, on the fly.
//
// Streams that browsers already support are copied rather than re-encoded:
// H.264 (8-bit 4:2:0) video and AAC/MP3 audio. Everything else is encoded to
// H.264 + stereo AAC. Copying makes most MKVs (H.264 with AC3/DTS audio) cost
// almost no CPU.
//
// The output isn't seekable byte-wise, so seeking is done by starting a new
// stream at a time offset. When video is copied, a stream can only start on
// a keyframe, so StartFor reports where a stream asked to start at t will
// actually begin; the player uses that as its time offset.
package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transcoder runs ffprobe and ffmpeg. The zero value is not usable; use New.
type Transcoder struct {
	ffmpeg, ffprobe string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	size  int64
	mtime time.Time
	info  *Info
}

// New returns a Transcoder for the given ffmpeg and ffprobe binaries (names
// on PATH or paths). It fails if either can't be found.
func New(ffmpeg, ffprobe string) (*Transcoder, error) {
	var err error
	if ffmpeg, err = exec.LookPath(ffmpeg); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	if ffprobe, err = exec.LookPath(ffprobe); err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	return &Transcoder{ffmpeg: ffmpeg, ffprobe: ffprobe, cache: map[string]cacheEntry{}}, nil
}

// Stream describes one stream of a media file.
type Stream struct {
	Index    int    `json:"index"`
	Type     string `json:"codec_type"`
	Codec    string `json:"codec_name"`
	PixFmt   string `json:"pix_fmt"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Channels int    `json:"channels"`
	// Disposition flags, e.g. "default" or "attached_pic".
	Disposition map[string]int `json:"disposition"`
}

// Info is what ffprobe reports about a file, reduced to what playback needs.
type Info struct {
	Duration  float64 // seconds
	StartTime float64 // container start timestamp, seconds
	Video     *Stream // nil if the file has no video
	Audio     *Stream // nil if the file has no audio
}

// CopyVideo reports whether the video stream can be passed through as is.
func (i *Info) CopyVideo() bool {
	v := i.Video
	return v != nil && v.Codec == "h264" && (v.PixFmt == "yuv420p" || v.PixFmt == "yuvj420p")
}

// CopyAudio reports whether the audio stream can be passed through as is.
func (i *Info) CopyAudio() bool {
	a := i.Audio
	return a != nil && (a.Codec == "aac" || a.Codec == "mp3")
}

// Probe returns information about the file at path. Results are cached
// until the file's size or modification time changes.
func (t *Transcoder) Probe(ctx context.Context, path string) (*Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	e, ok := t.cache[path]
	t.mu.Unlock()
	if ok && e.size == st.Size() && e.mtime.Equal(st.ModTime()) {
		return e.info, nil
	}

	out, err := t.run(ctx, t.ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		return nil, err
	}
	info, err := parseProbe(out)
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	t.mu.Lock()
	t.cache[path] = cacheEntry{size: st.Size(), mtime: st.ModTime(), info: info}
	t.mu.Unlock()
	return info, nil
}

func parseProbe(data []byte) (*Info, error) {
	var raw struct {
		Streams []Stream `json:"streams"`
		Format  struct {
			Duration  string `json:"duration"`
			StartTime string `json:"start_time"`
		} `json:"format"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	info := &Info{}
	info.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	info.StartTime, _ = strconv.ParseFloat(raw.Format.StartTime, 64)
	for i := range raw.Streams {
		s := &raw.Streams[i]
		switch s.Type {
		case "video":
			// Skip cover art, which MKVs store as an attached picture stream.
			if info.Video == nil && s.Disposition["attached_pic"] == 0 {
				info.Video = s
			}
		case "audio":
			// The first audio stream, unless a later one is flagged default.
			if info.Audio == nil || (s.Disposition["default"] == 1 && info.Audio.Disposition["default"] == 0) {
				info.Audio = s
			}
		}
	}
	if info.Video == nil && info.Audio == nil {
		return nil, errors.New("no audio or video streams")
	}
	return info, nil
}

// StartFor returns the time (seconds from the start of the file) at which a
// stream requested to start at `at` really begins. When video is re-encoded
// that's `at` itself; when it's copied it's the keyframe at or before `at`.
func (t *Transcoder) StartFor(ctx context.Context, path string, info *Info, at float64) float64 {
	at = math.Max(0, at)
	if info.Duration > 0 {
		at = math.Min(at, info.Duration)
	}
	if at == 0 || !info.CopyVideo() {
		return at
	}
	// read_intervals seeks the same way ffmpeg's -ss does (to the keyframe at
	// or before the target, using absolute timestamps), then reads a few packets.
	out, err := t.run(ctx, t.ffprobe, "-v", "error",
		"-read_intervals", fmt.Sprintf("%.3f%%+#16", at+info.StartTime),
		"-select_streams", strconv.Itoa(info.Video.Index),
		"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path)
	if err != nil {
		return at
	}
	for _, line := range strings.Split(string(out), "\n") {
		pts, flags, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}
		if kf, err := strconv.ParseFloat(pts, 64); err == nil {
			return math.Max(0, kf-info.StartTime)
		}
	}
	return at
}

// Args returns the ffmpeg arguments that write a fragmented MP4 of the file
// to stdout, starting at `start` seconds from the beginning of the file.
func Args(path string, info *Info, start float64) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	if start > 0 {
		ss := start
		if info.CopyVideo() {
			// Copied video starts at the keyframe at or before the seek
			// target, and the output's time zero is that keyframe. ffmpeg
			// seeks ~0.13s before -ss for video with B-frames, so -ss exactly
			// on a keyframe (as StartFor returns) would land on the previous
			// one. Aim a little past it instead.
			ss += 0.2
		}
		args = append(args, "-ss", strconv.FormatFloat(ss, 'f', 3, 64))
	}
	args = append(args, "-i", path)

	if v := info.Video; v != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(v.Index))
		if info.CopyVideo() {
			args = append(args, "-c:v", "copy")
		} else {
			args = append(args,
				"-c:v", "libx264", "-preset", "veryfast", "-crf", "22",
				"-pix_fmt", "yuv420p",
				// Cap at 1080p so real-time encoding stays feasible.
				"-vf", "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
				// Frequent keyframes so each fragment is short and playback starts quickly.
				"-force_key_frames", "expr:gte(t,n_forced*2)")
		}
	}
	if a := info.Audio; a != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(a.Index))
		if info.CopyAudio() {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
		}
	}

	return append(args,
		"-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1",
		"-max_muxing_queue_size", "1024",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4", "pipe:1")
}

// Transcode writes a fragmented MP4 of the file to w, starting at `start`
// seconds (use StartFor to pick it). It runs until the file ends, w fails or
// ctx is cancelled; cancellation isn't reported as an error.
func (t *Transcoder) Transcode(ctx context.Context, w io.Writer, path string, info *Info, start float64) error {
	cmd := exec.CommandContext(ctx, t.ffmpeg, Args(path, info, start)...)
	cmd.Stdout = w
	var stderr tailBuffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ffmpeg %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (t *Transcoder) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr tailBuffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// tailBuffer keeps the last 4 KiB written to it, for error messages.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	const max = 4096
	t.b = append(t.b, p...)
	if len(t.b) > max {
		t.b = t.b[len(t.b)-max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.b) }
