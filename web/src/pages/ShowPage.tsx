import { Link, useParams } from "react-router-dom";
import { api, episodeCode } from "../api";
import { useFetch } from "../useFetch";

export default function ShowPage() {
  const { id = "" } = useParams();
  const { data: show, error, loading } = useFetch(() => api.show(id), [id]);

  if (error) return <p>{error}</p>;
  if (loading || !show) return <p>Loading…</p>;

  return (
    <div className="show-page">
      <div className="show-header">
        <div className="poster-frame show-poster">
          {show.posterUrl ? <img src={show.posterUrl} alt="" /> : null}
        </div>
        <div>
          <h1>{show.name}</h1>
          {show.firstAirDate ? <p>First aired {show.firstAirDate}</p> : null}
          {show.overview ? <p>{show.overview}</p> : null}
        </div>
      </div>

      {show.seasons.map((season) => (
        <section key={season.seasonNumber} className="season">
          <h2>{season.name ?? `Season ${season.seasonNumber}`}</h2>
          <ol className="episode-list">
            {season.episodes.map((ep) => (
              <li key={ep.id}>
                <Link to={`/watch/episode/${ep.id}`} className="episode-row">
                  <div className="still-frame">
                    {ep.stillUrl ? <img src={ep.stillUrl} alt="" loading="lazy" /> : null}
                  </div>
                  <div>
                    <div>
                      {episodeCode(season.seasonNumber, ep.episodeNumber)} · {ep.name ?? `Episode ${ep.episodeNumber}`}
                    </div>
                    {ep.airDate ? <div>{ep.airDate}</div> : null}
                  </div>
                </Link>
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}
