import { useEffect } from "react";
import { api } from "./api";

// Progress isn't saved before this many seconds, so opening something for a
// moment doesn't count as watching it (the server ignores it too).
const MIN_POSITION = 10;
const SAVE_EVERY_MS = 10_000;

// useWatchProgress saves how far a video is watched: every 10 seconds while
// it plays, on pause, at the end, and when leaving. With startAt, it seeks
// there once the video's metadata is loaded.
export function useWatchProgress(
  video: HTMLVideoElement | null,
  item: { kind: "movie" | "episode"; id: number },
  startAt: number | null,
) {
  const { kind, id } = item;
  useEffect(() => {
    if (!video) return;
    let lastSave = Date.now();
    let ended = false;

    const save = (opts: { ended?: boolean; keepalive?: boolean } = {}) => {
      const duration = video.duration;
      const position = video.currentTime;
      if (!Number.isFinite(duration) || duration <= 0) return;
      if (!opts.ended && position < MIN_POSITION) return;
      lastSave = Date.now();
      api.saveProgress({ kind, id, position, duration, ended: opts.ended }, opts.keepalive).catch(() => {});
    };

    const seekToStart = () => {
      if (startAt && Math.abs(video.currentTime - startAt) > 2) video.currentTime = startAt;
      video.removeEventListener("loadedmetadata", seekToStart);
    };
    const onTimeUpdate = () => {
      if (!video.paused && Date.now() - lastSave >= SAVE_EVERY_MS) save();
    };
    const onPause = () => !video.ended && save();
    const onEnded = () => {
      ended = true;
      save({ ended: true });
    };
    const onPlay = () => (ended = false);
    const onPageHide = () => !ended && save({ keepalive: true });

    if (startAt) {
      if (video.readyState >= HTMLMediaElement.HAVE_METADATA) seekToStart();
      else video.addEventListener("loadedmetadata", seekToStart);
    }
    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("pause", onPause);
    video.addEventListener("ended", onEnded);
    video.addEventListener("play", onPlay);
    window.addEventListener("pagehide", onPageHide);
    return () => {
      // Leaving the page within the app (e.g. to the next episode).
      if (!ended) save({ keepalive: true });
      video.removeEventListener("loadedmetadata", seekToStart);
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("ended", onEnded);
      video.removeEventListener("play", onPlay);
      window.removeEventListener("pagehide", onPageHide);
    };
  }, [video, kind, id, startAt]);
}
