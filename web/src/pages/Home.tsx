import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, type LibraryItem, type ScanStatus } from "../api";
import { useFetch } from "../useFetch";

function PosterGrid({ items, linkTo }: { items: LibraryItem[]; linkTo: (id: number) => string }) {
  if (items.length === 0) return <p>Nothing here yet.</p>;
  return (
    <ul className="poster-grid">
      {items.map((item) => (
        <li key={item.id} className="poster-card">
          <Link to={linkTo(item.id)}>
            <div className="poster-frame">
              {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : null}
            </div>
            <div>{item.title}</div>
            {item.year ? <div>{item.year}</div> : null}
          </Link>
        </li>
      ))}
    </ul>
  );
}

function ScanControl({ onFinished }: { onFinished: () => void }) {
  const [status, setStatus] = useState<ScanStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.scanStatus().then(setStatus, () => {});
  }, []);

  // Poll while a scan is running; refresh the library when it ends.
  useEffect(() => {
    if (!status?.running) return;
    const timer = setInterval(async () => {
      try {
        const next = await api.scanStatus();
        setStatus(next);
        if (!next.running) onFinished();
      } catch (e) {
        setError((e as Error).message);
      }
    }, 2000);
    return () => clearInterval(timer);
  }, [status?.running, onFinished]);

  const start = async () => {
    setError(null);
    try {
      setStatus(await api.startScan());
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <div className="scan-control">
      <button onClick={start} disabled={status?.running}>
        {status?.running ? "Scanning…" : "Rescan library"}
      </button>
      {status?.running ? (
        <span>
          {status.phase}: {status.filesSeen} files seen, {status.filesLinked} linked
        </span>
      ) : status?.finishedAt ? (
        <span>
          Last scan: {status.filesLinked} linked, {status.filesIgnored} ignored, {status.filesRemoved} removed
          {status.errors.length > 0 ? `, ${status.errors.length} errors` : ""}
        </span>
      ) : null}
      {error ? <span>{error}</span> : null}
    </div>
  );
}

export default function Home() {
  const { data, error, loading, reload } = useFetch(api.library, []);

  return (
    <div className="home">
      <ScanControl onFinished={reload} />
      {error ? <p>{error}</p> : null}
      {loading && !data ? <p>Loading…</p> : null}
      {data ? (
        <>
          <section>
            <h2>Movies</h2>
            <PosterGrid items={data.movies} linkTo={(id) => `/watch/movie/${id}`} />
          </section>
          <section>
            <h2>TV Shows</h2>
            <PosterGrid items={data.shows} linkTo={(id) => `/shows/${id}`} />
          </section>
        </>
      ) : null}
    </div>
  );
}
