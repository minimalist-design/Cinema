import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { api, episodeCode, streamUrl } from "../api";
import { useFetch } from "../useFetch";

function PlayerLayout({ fileId, children }: { fileId: number; children: ReactNode }) {
  return (
    <div className="player-page">
      {/* key forces a fresh <video> element when the file changes */}
      <video key={fileId} className="player-video" src={streamUrl(fileId)} controls autoPlay preload="metadata" />
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
