package transcode

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Once this many finished segments are ready past the latest request,
	// ffmpeg is paused until the player catches up.
	aheadSegments = 4
	// A request for a segment at most this far past the one being written
	// waits for it; further away, ffmpeg is restarted at the requested one.
	waitSegments = 1
	// Finished segments more than this far behind the latest request are
	// deleted. Asking for one again restarts ffmpeg there.
	keepSegments = 10
	// At most this many sessions run at once; starting another stops the
	// least recently used.
	maxSessions = 8
	// How often idle sessions and old segments are cleaned up.
	tick = time.Second
)

// Sessions with no requests for this long are stopped and deleted. (A
// variable for tests.)
var sessionIdle = 90 * time.Second

var (
	// ErrBadSession is returned for a malformed session id, or one that
	// belongs to another file.
	ErrBadSession = errors.New("invalid session")
	// ErrBadSegment is returned for a segment number outside the playlist.
	ErrBadSegment = errors.New("no such segment")
	// ErrSuperseded is returned to a request that was waiting for a segment
	// when another request for the same session restarted ffmpeg elsewhere
	// (the player seeked).
	ErrSuperseded = errors.New("superseded by a newer request")
)

var sessionIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// HLS serves files as HLS playlists and segments. Each viewer gets a
// session (an id chosen by the client) with its own ffmpeg process, which
// writes segments to a directory under dir.
type HLS struct {
	store *Store
	tc    *Transcoder
	dir   string

	mu       sync.Mutex
	sessions map[string]*session
	closed   bool
	seq      atomic.Int64 // names session and job directories

	stop chan struct{}
	done chan struct{}
}

// NewHLS returns an HLS server that keeps segments under dir. Anything
// already in dir is deleted.
func NewHLS(store *Store, tc *Transcoder, dir string) (*HLS, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	h := &HLS{store: store, tc: tc, dir: dir, sessions: map[string]*session{},
		stop: make(chan struct{}), done: make(chan struct{})}
	go h.janitor()
	return h, nil
}

// Close stops every ffmpeg process and deletes the segment directory.
func (h *HLS) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	sessions := h.sessions
	h.sessions = map[string]*session{}
	h.mu.Unlock()

	close(h.stop)
	<-h.done
	for _, s := range sessions {
		s.close()
	}
	os.RemoveAll(h.dir)
}

// Playlist returns the VOD playlist of a file. Segment URIs are relative
// ("<n>.ts").
func (h *HLS) Playlist(ctx context.Context, fileID int64) ([]byte, error) {
	_, ix, err := h.store.Get(ctx, fileID)
	if err != nil {
		return nil, err
	}
	starts := ix.Segments()
	durations := make([]float64, len(starts))
	longest := 0.0
	for i, s := range starts {
		end := ix.Duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		durations[i] = end - s
		longest = math.Max(longest, durations[i])
	}

	var b bytes.Buffer
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", int(math.Ceil(longest)))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	for i, d := range durations {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n%d.ts\n", d, i)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.Bytes(), nil
}

// Segment returns the path of segment n of a file, transcoding it first if
// needed. The file stays valid until the session moves on or expires, so
// callers should serve it right away.
func (h *HLS) Segment(ctx context.Context, fileID int64, sessionID string, n int) (string, error) {
	s, err := h.session(ctx, fileID, sessionID)
	if err != nil {
		return "", err
	}
	return s.segment(ctx, n)
}

func (h *HLS) session(ctx context.Context, fileID int64, id string) (*session, error) {
	if !sessionIDRe.MatchString(id) {
		return nil, ErrBadSession
	}
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil {
		// Load the index outside the lock; it may need building.
		path, ix, err := h.store.Get(ctx, fileID)
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			return nil, errors.New("shutting down")
		}
		if s = h.sessions[id]; s == nil {
			s = &session{h: h, id: id, fileID: fileID, path: path, ix: ix, starts: ix.Segments(),
				dir: filepath.Join(h.dir, fmt.Sprintf("%s-%d", id, h.seq.Add(1))), lastUsed: time.Now()}
			h.sessions[id] = s
			h.evictLocked()
		}
		h.mu.Unlock()
	}
	if s.fileID != fileID {
		return nil, ErrBadSession
	}
	return s, nil
}

// evictLocked stops the least recently used sessions beyond maxSessions.
func (h *HLS) evictLocked() {
	for len(h.sessions) > maxSessions {
		var oldest *session
		for _, s := range h.sessions {
			if oldest == nil || s.used().Before(oldest.used()) {
				oldest = s
			}
		}
		delete(h.sessions, oldest.id)
		go oldest.close()
	}
}

// janitor deletes old segments and stops idle sessions.
func (h *HLS) janitor() {
	defer close(h.done)
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-t.C:
		}
		h.mu.Lock()
		var idle []*session
		for id, s := range h.sessions {
			if time.Since(s.used()) > sessionIdle {
				delete(h.sessions, id)
				idle = append(idle, s)
			}
		}
		active := make([]*session, 0, len(h.sessions))
		for _, s := range h.sessions {
			active = append(active, s)
		}
		h.mu.Unlock()
		for _, s := range idle {
			s.close()
		}
		for _, s := range active {
			s.prune()
		}
	}
}

// ---------------------------------------------------------------- session

// Lock order: HLS.mu before session.mu.
type session struct {
	h      *HLS
	id     string
	fileID int64
	path   string
	ix     *Index
	starts []float64
	dir    string

	mu       sync.Mutex
	job      *job
	lastReq  int
	lastUsed time.Time
	closed   bool
}

func (s *session) used() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUsed
}

func (s *session) segment(ctx context.Context, n int) (string, error) {
	if n < 0 || n >= len(s.starts) {
		return "", ErrBadSegment
	}
	var waitingOn *job
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return "", errors.New("session closed")
		}
		j := s.job
		if waitingOn != nil && j != waitingOn {
			// Another request restarted ffmpeg while this one waited. Don't
			// restart it back: requests would take turns killing each
			// other's jobs.
			if j == nil || n < j.first || n > j.completed+waitSegments {
				s.mu.Unlock()
				return "", ErrSuperseded
			}
		}
		s.lastUsed, s.lastReq = time.Now(), n
		restart := false
		switch {
		case j == nil || n < j.first || n > j.completed+waitSegments:
			restart = true // first request, or a seek
		case n < j.completed:
			if j.exists(n) {
				s.throttleLocked()
				s.mu.Unlock()
				return j.file(n), nil
			}
			restart = true // deleted after the player moved on
		default:
			// n is being written or is next. Wait, unless ffmpeg is gone.
			if finished, err := j.finished(); finished {
				s.job = nil
				s.mu.Unlock()
				if err == nil {
					err = fmt.Errorf("ffmpeg ended after segment %d of %d", j.completed-1, len(s.starts)-1)
				}
				return "", err
			}
		}
		if restart {
			if j != nil {
				j.stop()
			}
			var err error
			if j, err = s.startLocked(n); err != nil {
				s.mu.Unlock()
				return "", err
			}
			s.job = j
		}
		s.throttleLocked()
		progress := j.progress
		waitingOn = j
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-progress:
		}
	}
}

func (s *session) startLocked(first int) (*job, error) {
	dir := filepath.Join(s.dir, fmt.Sprintf("%d-%d", s.h.seq.Add(1), first))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{first: first, completed: first, pruned: first, dir: dir,
		cancel: cancel, done: make(chan struct{}), progress: make(chan struct{})}
	cmd := exec.CommandContext(ctx, s.h.tc.ffmpeg, segmentArgs(s.path, s.ix, s.starts, first, dir)...)
	cmd.Stderr = &j.stderr
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		cancel()
		os.RemoveAll(dir)
		return nil, err
	}
	j.proc = cmd.Process

	go func() {
		// ffmpeg prints each segment's file name once it's complete.
		lines := bufio.NewScanner(stdout)
		for lines.Scan() {
			n, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(lines.Text()), ".ts"))
			if err != nil {
				continue
			}
			s.mu.Lock()
			j.completed = max(j.completed, n+1)
			if s.job == j {
				s.throttleLocked()
			}
			j.notifyLocked()
			s.mu.Unlock()
		}
		err := cmd.Wait()
		if err != nil && !j.stopped.Load() {
			err = fmt.Errorf("ffmpeg %s (from segment %d): %w: %s", s.path, first, err, bytes.TrimSpace(j.stderr.b))
			log.Print("hls: ", err)
		}
		j.err = err
		close(j.done)
		s.mu.Lock()
		j.notifyLocked()
		s.mu.Unlock()
	}()
	return j, nil
}

// throttleLocked pauses ffmpeg when it's far enough ahead of the player and
// resumes it when the player catches up.
func (s *session) throttleLocked() {
	j := s.job
	if j == nil {
		return
	}
	if finished, _ := j.finished(); finished {
		return
	}
	ahead := j.completed - 1 - s.lastReq
	if !j.paused && ahead > aheadSegments {
		if pause(j.proc) == nil {
			j.paused = true
		}
	} else if j.paused && ahead <= aheadSegments {
		if resume(j.proc) == nil {
			j.paused = false
		}
	}
}

// prune deletes segments the player has moved well past.
func (s *session) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.job
	if s.closed || j == nil {
		return
	}
	for ; j.pruned < s.lastReq-keepSegments && j.pruned < j.completed; j.pruned++ {
		os.Remove(j.file(j.pruned))
	}
}

func (s *session) close() {
	s.mu.Lock()
	s.closed = true
	j := s.job
	s.job = nil
	s.mu.Unlock()
	if j != nil {
		j.stop()
		<-j.done
	}
	os.RemoveAll(s.dir)
}

// ---------------------------------------------------------------- job

// job is one ffmpeg run writing segments first, first+1, … into dir.
type job struct {
	first  int
	dir    string
	proc   *os.Process
	cancel context.CancelFunc
	stderr tailBuffer

	done    chan struct{} // closed when ffmpeg exits
	err     error         // set before done is closed
	stopped atomic.Bool   // killed on purpose; exit errors are expected

	// Guarded by the session's mutex.
	completed int           // segments in [first, completed) are fully written
	pruned    int           // segments in [first, pruned) have been deleted
	paused    bool          // stopped with SIGSTOP
	progress  chan struct{} // closed (and replaced) when completed changes or ffmpeg exits
}

func (j *job) file(n int) string { return filepath.Join(j.dir, strconv.Itoa(n)+".ts") }

func (j *job) exists(n int) bool {
	_, err := os.Stat(j.file(n))
	return err == nil
}

func (j *job) notifyLocked() {
	close(j.progress)
	j.progress = make(chan struct{})
}

func (j *job) finished() (bool, error) {
	select {
	case <-j.done:
		return true, j.err
	default:
		return false, nil
	}
}

// stop kills ffmpeg and deletes its segments once it has exited.
func (j *job) stop() {
	j.stopped.Store(true)
	j.cancel() // SIGKILL, which also works on a paused process
	go func() {
		<-j.done
		os.RemoveAll(j.dir)
	}()
}
