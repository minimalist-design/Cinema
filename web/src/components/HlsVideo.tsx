import { useEffect, useRef, useState } from "react";
import type Hls from "hls.js";

// HlsVideo plays an HLS playlist with the browser's native controls. Safari
// (and other browsers with built-in HLS) play it directly; elsewhere hls.js
// feeds it to the video element through Media Source Extensions. hls.js is
// loaded on demand so it's only downloaded when needed.
export function HlsVideo({ src }: { src: string }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    setError(null);

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = src;
      return () => {
        video.removeAttribute("src");
        video.load(); // stops downloading
      };
    }

    let hls: Hls | undefined;
    let cancelled = false;
    import("hls.js/light").then(
      ({ default: HlsJs }) => {
        if (cancelled) return;
        if (!HlsJs.isSupported()) {
          setError("This browser can't play this video.");
          return;
        }
        const player = new HlsJs();
        hls = player;
        let recovered = false;
        player.on(HlsJs.Events.ERROR, (_event, data) => {
          if (!data.fatal) return;
          // A decoding error can often be recovered from once.
          if (data.type === HlsJs.ErrorTypes.MEDIA_ERROR && !recovered) {
            recovered = true;
            player.recoverMediaError();
            return;
          }
          setError(`Playback failed (${data.details}). The server log may have details.`);
          player.destroy();
        });
        player.loadSource(src);
        player.attachMedia(video);
      },
      () => !cancelled && setError("Couldn't load the video player."),
    );
    return () => {
      cancelled = true;
      hls?.destroy();
    };
  }, [src]);

  return (
    <>
      <video ref={videoRef} className="player-video" controls autoPlay />
      {error ? <p role="alert">{error}</p> : null}
    </>
  );
}
