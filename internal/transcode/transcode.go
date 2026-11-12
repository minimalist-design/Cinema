// Package transcode streams files that browsers can't play directly (MKV)
// as HLS, transcoding them on the fly with ffmpeg.
//
// Streams that browsers already support are copied rather than re-encoded:
// H.264 (8-bit 4:2:0) video and AAC/MP3 audio. Everything else is encoded to
// H.264 + stereo AAC. Copying makes most MKVs (H.264 with AC3/DTS audio) cost
// almost no CPU.
//
// An HLS playlist lists every segment of the file up front, so the player
// knows the duration and can seek natively. That requires fixed segment
// boundaries. Re-encoded video gets a keyframe forced every SegmentLength
// seconds. Copied video can only be cut on its existing keyframes, so each
// file has an Index with a keyframe near every SegmentLength seconds, built
// once (normally during the library scan) and stored in the database.
package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// SegmentLength is the target HLS segment duration in seconds.
const SegmentLength = 6.0

// tsOffset is added to every timestamp in the MPEG-TS segments. Players
// align the segments' timestamps with the playlist, so its value doesn't
// matter as long as it's the same in every segment.
const tsOffset = 10.0

// transcodeExts are containers browsers can't play natively.
var transcodeExts = map[string]bool{".mkv": true}

// NeedsTranscode reports whether the file at path is streamed through HLS
// rather than served as-is.
func NeedsTranscode(path string) bool {
	return transcodeExts[strings.ToLower(filepath.Ext(path))]
}

// Transcoder runs ffprobe and ffmpeg.
type Transcoder struct {
	ffmpeg, ffprobe string
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
	return &Transcoder{ffmpeg: ffmpeg, ffprobe: ffprobe}, nil
}

// Stream describes one stream of a media file.
type Stream struct {
	Index    int    `json:"index"`
	Type     string `json:"codec_type"`
	Codec    string `json:"codec_name"`
	PixFmt   string `json:"pix_fmt,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Channels int    `json:"channels,omitempty"`
	// Disposition flags, e.g. "default" or "attached_pic".
	Disposition map[string]int `json:"disposition,omitempty"`
}

// Index is what playback needs to know about a file.
type Index struct {
	Duration  float64 `json:"duration"`  // seconds
	StartTime float64 `json:"startTime"` // container start timestamp, seconds
	Video     *Stream `json:"video"`     // nil if the file has no video
	Audio     *Stream `json:"audio"`     // nil if the file has no audio
	// Keyframes are video keyframe times (seconds from StartTime), at most
	// one per SegmentLength. Only set when video is copied.
	Keyframes []float64 `json:"keyframes,omitempty"`
}

// CopyVideo reports whether the video stream can be passed through as is.
func (ix *Index) CopyVideo() bool {
	v := ix.Video
	return v != nil && v.Codec == "h264" && (v.PixFmt == "yuv420p" || v.PixFmt == "yuvj420p")
}

// CopyAudio reports whether the audio stream can be passed through as is.
func (ix *Index) CopyAudio() bool {
	a := ix.Audio
	return a != nil && (a.Codec == "aac" || a.Codec == "mp3")
}

// Segments returns the start time of each HLS segment. Segment i covers
// [starts[i], starts[i+1]), and the last one runs to Duration.
func (ix *Index) Segments() []float64 {
	starts := []float64{0}
	last := func() float64 { return starts[len(starts)-1] }
	// Don't leave a final segment shorter than a second.
	end := ix.Duration - 1
	if ix.CopyVideo() {
		// Cut on keyframes, skipping any that would make a segment shorter
		// than half the target length.
		for _, kf := range ix.Keyframes {
			if kf >= last()+SegmentLength/2 && kf <= end {
				starts = append(starts, kf)
			}
		}
		return starts
	}
	for t := SegmentLength; t <= end; t += SegmentLength {
		starts = append(starts, t)
	}
	return starts
}

// BuildIndex probes the file at path and, when its video will be copied,
// finds its keyframes.
func (t *Transcoder) BuildIndex(ctx context.Context, path string) (*Index, error) {
	out, err := t.run(ctx, t.ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		return nil, err
	}
	ix, err := parseProbe(out)
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	if ix.Duration <= 0 {
		return nil, fmt.Errorf("ffprobe %s: unknown duration", path)
	}
	if ix.CopyVideo() {
		if ix.Keyframes, err = t.keyframes(ctx, path, ix); err != nil {
			return nil, err
		}
	}
	return ix, nil
}

func parseProbe(data []byte) (*Index, error) {
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
	ix := &Index{}
	ix.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	ix.StartTime, _ = strconv.ParseFloat(raw.Format.StartTime, 64)
	for i := range raw.Streams {
		s := &raw.Streams[i]
		switch s.Type {
		case "video":
			// Skip cover art, which MKVs store as an attached picture stream.
			if ix.Video == nil && s.Disposition["attached_pic"] == 0 {
				ix.Video = s
			}
		case "audio":
			// The first audio stream, unless a later one is flagged default.
			if ix.Audio == nil || (s.Disposition["default"] == 1 && ix.Audio.Disposition["default"] == 0) {
				ix.Audio = s
			}
		}
	}
	if ix.Video == nil && ix.Audio == nil {
		return nil, errors.New("no audio or video streams")
	}
	return ix, nil
}

// keyframes finds the keyframe at or before every multiple of SegmentLength.
// Rather than reading every packet of the file, ffprobe seeks to each point
// (using the MKV's cue index, so only a little data is read per seek) and
// reports the first packet there, which is the keyframe the seek landed on.
func (t *Transcoder) keyframes(ctx context.Context, path string, ix *Index) ([]float64, error) {
	var intervals []string
	for at := 0.0; at < ix.Duration; at += SegmentLength {
		intervals = append(intervals, fmt.Sprintf("%.3f%%+#1", at+ix.StartTime))
	}
	out, err := t.run(ctx, t.ffprobe, "-v", "error",
		"-read_intervals", strings.Join(intervals, ","),
		"-select_streams", strconv.Itoa(ix.Video.Index),
		"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path)
	if err != nil {
		return nil, err
	}
	return parseKeyframes(out, ix.StartTime), nil
}

func parseKeyframes(out []byte, startTime float64) []float64 {
	var kfs []float64
	for _, line := range strings.Split(string(out), "\n") {
		pts, flags, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}
		if kf, err := strconv.ParseFloat(pts, 64); err == nil {
			kfs = append(kfs, math.Round((kf-startTime)*1000)/1000)
		}
	}
	slices.Sort(kfs)
	return slices.Compact(kfs)
}

// segmentArgs returns the ffmpeg arguments that write the file's HLS
// segments as MPEG-TS files <dir>/<n>.ts, starting with segment first.
//
// Every run uses the file's own timestamps (-copyts), so a run restarted at
// any segment produces segments that line up with those of earlier runs.
func segmentArgs(path string, ix *Index, starts []float64, first int, dir string) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	// -ss counts from the file's start time, like the segment start times.
	if first > 0 {
		if ix.CopyVideo() {
			// Copied video starts at the keyframe at or before the seek
			// target. ffmpeg seeks ~0.13s before -ss for video with
			// B-frames, so aim a little past the keyframe to land on it.
			// Keep the audio from there too rather than trimming it.
			args = append(args, "-ss", ffTime(starts[first]+0.2), "-noaccurate_seek")
		} else {
			args = append(args, "-ss", ffTime(starts[first]))
		}
	}
	args = append(args, "-copyts", "-i", path)

	if v := ix.Video; v != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(v.Index))
		if ix.CopyVideo() {
			args = append(args, "-c:v", "copy")
		} else {
			args = append(args,
				"-c:v", "libx264", "-preset", "veryfast", "-crf", "22",
				"-pix_fmt", "yuv420p",
				// Cap at 1080p so real-time encoding stays feasible.
				"-vf", "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
				// A keyframe at every segment boundary. t counts from this
				// run's start, which is itself a boundary.
				"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%g)", SegmentLength))
		}
	}
	if a := ix.Audio; a != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(a.Index))
		if ix.CopyAudio() {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
		}
	}

	// Split points, relative to where this run starts. A final one past the
	// end stops the segment muxer from falling back to its 2s default.
	var times []string
	for _, s := range starts[first+1:] {
		times = append(times, ffTime(s-starts[first]))
	}
	times = append(times, ffTime(ix.Duration-starts[first]+3600))

	return append(args,
		"-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1",
		"-max_muxing_queue_size", "1024",
		// Timestamps must be identical whichever segment a run starts at,
		// so nothing may shift them depending on the first packet: no
		// shifting to zero, and the MPEG-TS muxer keeps them as is
		// (mpegts_copyts). A fixed offset keeps them positive instead,
		// since B-frames give the first video frames negative DTS.
		"-avoid_negative_ts", "disabled",
		"-output_ts_offset", ffTime(tsOffset),
		"-f", "segment", "-segment_format", "mpegts",
		"-segment_format_options", "mpegts_copyts=1",
		// One MPEG-TS muxer for all segments, so continuity counters carry
		// on from one segment to the next. The segment muxer still starts
		// each segment with the PAT/PMT tables, so each plays on its own.
		"-individual_header_trailer", "0",
		"-segment_times", strings.Join(times, ","),
		// Tolerance for rounding in the split points.
		"-segment_time_delta", "0.25",
		"-segment_start_number", strconv.Itoa(first),
		// Print each segment's name to stdout once it's complete.
		"-segment_list", "pipe:1", "-segment_list_type", "flat",
		filepath.Join(dir, "%d.ts"))
}

func ffTime(s float64) string { return strconv.FormatFloat(s, 'f', 3, 64) }

func (t *Transcoder) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr tailBuffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(name), err, strings.TrimSpace(stderr.String()))
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
