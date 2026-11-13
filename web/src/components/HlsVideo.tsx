import { useEffect, useRef, useState } from "react";
import type { ReactNode, Ref } from "react";
import type Hls from "hls.js";
import { Box } from "@mantine/core";
import { useMergedRef } from "@mantine/hooks";
import { ErrorAlert } from "./States";

// HlsVideo plays an HLS playlist with the browser's native controls. Safari
// (and other browsers with built-in HLS) play it directly; elsewhere hls.js
// feeds it to the video element through Media Source Extensions. hls.js is
// loaded on demand so it's only downloaded when needed. children go inside
// the video element (e.g. <track>s); frameClassName styles the box around it.
// startAt (seconds) makes hls.js start loading there rather than at the
// beginning; with native HLS, the caller seeks once metadata is loaded.
export function HlsVideo({
  src,
  className,
  frameClassName,
  startAt,
  videoRef: outerRef,
  children,
}: {
  src: string;
  className?: string;
  frameClassName?: string;
  startAt?: number | null;
  videoRef?: Ref<HTMLVideoElement>;
  children?: ReactNode;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const mergedRef = useMergedRef(videoRef, outerRef);
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
        const player = new HlsJs(startAt ? { startPosition: startAt } : undefined);
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
    // startAt only matters when the source loads.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [src]);

  return (
    <>
      <Box className={frameClassName}>
        <video ref={mergedRef} className={className} controls autoPlay>
          {children}
        </video>
      </Box>
      {error ? <ErrorAlert>{error}</ErrorAlert> : null}
    </>
  );
}
