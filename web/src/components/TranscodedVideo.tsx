import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { api } from "../api";
import type { Playback } from "../api";

function formatTime(seconds: number) {
  const s = Math.max(0, Math.floor(seconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

// TranscodedVideo plays a stream the server transcodes on the fly. The
// browser only ever sees the stream from `start` onwards and doesn't know the
// total duration, so this draws its own controls: positions are absolute
// (start + currentTime), and seeking outside what's already loaded fetches a
// new stream from the server.
export function TranscodedVideo({ fileId, initial }: { fileId: number; initial: Playback }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const seekRequest = useRef(0);
  const [stream, setStream] = useState(initial);
  const [time, setTime] = useState(initial.start);
  const [scrub, setScrub] = useState<number | null>(null);
  const [paused, setPaused] = useState(false);
  const [waiting, setWaiting] = useState(true);
  const [muted, setMuted] = useState(false);
  const [volume, setVolume] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const duration = initial.duration ?? 0;

  useEffect(() => {
    const v = videoRef.current;
    if (v) {
      v.volume = volume;
      v.muted = muted;
    }
  }, [volume, muted, stream]);

  async function seek(target: number) {
    const v = videoRef.current;
    if (!v) return;
    target = Math.min(Math.max(0, target), Math.max(0, duration - 1));
    setTime(target);

    // Already loaded: seek within the current stream.
    const rel = target - stream.start;
    for (let i = 0; i < v.buffered.length; i++) {
      if (rel >= v.buffered.start(i) && rel <= v.buffered.end(i)) {
        v.currentTime = rel;
        return;
      }
    }

    // Otherwise restart the stream there. The old request is dropped when
    // the src changes, which stops its ffmpeg process on the server.
    const id = ++seekRequest.current;
    setWaiting(true);
    setError(null);
    try {
      const next = await api.playback(fileId, target);
      if (id === seekRequest.current) setStream(next);
    } catch (err) {
      if (id === seekRequest.current) setError((err as Error).message);
    }
  }

  function togglePlay() {
    const v = videoRef.current;
    if (!v) return;
    if (v.paused) void v.play().catch(() => {});
    else v.pause();
  }

  function toggleFullscreen() {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void containerRef.current?.requestFullscreen();
  }

  function onKeyDown(e: KeyboardEvent) {
    if ((e.target as HTMLElement).tagName === "INPUT") return;
    const actions: Record<string, () => void> = {
      " ": togglePlay,
      k: togglePlay,
      ArrowLeft: () => void seek(time - 10),
      ArrowRight: () => void seek(time + 10),
      f: toggleFullscreen,
      m: () => setMuted((m) => !m),
    };
    const action = actions[e.key];
    if (action) {
      e.preventDefault();
      action();
    }
  }

  const commitScrub = () => {
    if (scrub !== null) {
      void seek(scrub);
      setScrub(null);
    }
  };
  const shown = scrub ?? time;

  return (
    <div ref={containerRef} className="transcoded-player" tabIndex={0} onKeyDown={onKeyDown}>
      <video
        ref={videoRef}
        className="player-video"
        src={stream.url}
        // Keep playing after a seek unless the user had paused.
        autoPlay={!paused}
        onClick={togglePlay}
        onDoubleClick={toggleFullscreen}
        onTimeUpdate={(e) => setTime(stream.start + e.currentTarget.currentTime)}
        onPlay={() => setPaused(false)}
        onPause={() => setPaused(true)}
        onWaiting={() => setWaiting(true)}
        onPlaying={() => setWaiting(false)}
        onCanPlay={() => setWaiting(false)}
        onError={() => setError("Playback failed. The server log may have details.")}
      />
      <div className="player-controls">
        <button type="button" onClick={togglePlay}>
          {paused ? "Play" : "Pause"}
        </button>
        <span className="player-time">
          {formatTime(shown)} / {formatTime(duration)}
        </span>
        <input
          className="player-seek"
          type="range"
          aria-label="Seek"
          min={0}
          max={duration}
          step={1}
          value={shown}
          onChange={(e) => setScrub(Number(e.target.value))}
          onPointerUp={commitScrub}
          onKeyUp={commitScrub}
          onBlur={commitScrub}
        />
        <button type="button" onClick={() => setMuted((m) => !m)}>
          {muted ? "Unmute" : "Mute"}
        </button>
        <input
          className="player-volume"
          type="range"
          aria-label="Volume"
          min={0}
          max={1}
          step={0.05}
          value={muted ? 0 : volume}
          onChange={(e) => {
            setVolume(Number(e.target.value));
            setMuted(false);
          }}
        />
        <button type="button" onClick={toggleFullscreen}>
          Fullscreen
        </button>
      </div>
      {error ? <p role="alert">{error}</p> : waiting ? <p className="player-status">Loading…</p> : null}
    </div>
  );
}
