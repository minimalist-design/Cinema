import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { api, episodeCode } from "../api";
import { HlsVideo } from "../components/HlsVideo";
import { useFetch } from "../useFetch";

function Video({ fileId }: { fileId: number }) {
  const { data: playback, error, loading } = useFetch(() => api.playback(fileId), [fileId]);

  if (error) return <p role="alert">{error}</p>;
  if (loading || !playback) return <p>Loading…</p>;
  if (playback.mode === "hls") return <HlsVideo src={playback.url} />;
  return <video className="player-video" src={playback.url} controls autoPlay preload="metadata" />;
}

function PlayerLayout({ fileId, children }: { fileId: number; children: ReactNode }) {
  return (
    <div className="player-page">
      {/* key gives each file a fresh player (and fresh playback info) */}
      <Video key={fileId} fileId={fileId} />
      <div className="player-info">{children}</div>
    </div>
  );
}

export function MoviePlayer() {
  const { id = "" } = useParams();
  const { data: movie, error, loading } = useFetch(() => api.movie(id), [id]);

  if (error) return <p>{error}</p>;
  if (loading || !movie) return <p>Loading…</p>;

  return (
    <PlayerLayout fileId={movie.fileId}>
      <h1>{movie.title}</h1>
      {movie.releaseDate ? <p>Released {movie.releaseDate}</p> : null}
    </PlayerLayout>
  );
}

export function EpisodePlayer() {
  const { id = "" } = useParams();
  const { data: ep, error, loading } = useFetch(() => api.episode(id), [id]);

  if (error) return <p>{error}</p>;
  if (loading || !ep) return <p>Loading…</p>;

  return (
    <PlayerLayout fileId={ep.fileId}>
      <p>
        <Link to={`/shows/${ep.show.id}`}>{ep.show.name}</Link> · {episodeCode(ep.seasonNumber, ep.episodeNumber)}
      </p>
      <h1>{ep.name ?? `Episode ${ep.episodeNumber}`}</h1>
      {ep.airDate ? <p>Aired {ep.airDate}</p> : null}
      {ep.nextEpisodeId ? (
        <p>
          <Link to={`/watch/episode/${ep.nextEpisodeId}`}>Next episode →</Link>
        </p>
      ) : null}
    </PlayerLayout>
  );
}
