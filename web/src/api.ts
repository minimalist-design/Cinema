export interface LibraryItem {
  id: number;
  title: string;
  year: number | null;
  posterUrl: string | null;
}

export interface Library {
  movies: LibraryItem[];
  shows: LibraryItem[];
}

export interface Movie {
  id: number;
  title: string;
  releaseDate: string | null;
  overview: string | null;
  posterUrl: string | null;
  fileId: number;
}

export interface EpisodeItem {
  id: number;
  episodeNumber: number;
  name: string | null;
  airDate: string | null;
  stillUrl: string | null;
  fileId: number;
}

export interface Season {
  seasonNumber: number;
  name: string | null;
  posterUrl: string | null;
  episodes: EpisodeItem[];
}

export interface Show {
  id: number;
  name: string;
  firstAirDate: string | null;
  overview: string | null;
  posterUrl: string | null;
  seasons: Season[];
}

export interface Episode {
  id: number;
  name: string | null;
  seasonNumber: number;
  episodeNumber: number;
  airDate: string | null;
  overview: string | null;
  fileId: number;
  nextEpisodeId: number | null;
  show: { id: number; name: string };
}

export interface ScanStatus {
  running: boolean;
  refresh: boolean;
  phase: string;
  startedAt: string | null;
  finishedAt: string | null;
  filesSeen: number;
  filesLinked: number;
  filesIgnored: number;
  filesRemoved: number;
  moviesAdded: number;
  showsAdded: number;
  ignored: string[];
  errors: string[];
}

// How to play a file. "direct" files are served as-is with range support and
// use the browser's native controls. "transcode" streams can't be seeked by
// the browser: seek by asking for a new Playback with a start time, then add
// `start` to the video element's currentTime.
export interface Playback {
  mode: "direct" | "transcode";
  url: string;
  start: number;
  duration: number | null;
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    throw new Error(body?.error ?? `${res.status} ${res.statusText}`);
  }
  return body as T;
}

export const api = {
  library: () => request<Library>("/api/library"),
  movie: (id: string) => request<Movie>(`/api/movies/${id}`),
  show: (id: string) => request<Show>(`/api/shows/${id}`),
  episode: (id: string) => request<Episode>(`/api/episodes/${id}`),
  scanStatus: () => request<ScanStatus>("/api/scan"),
  startScan: () => request<ScanStatus>("/api/scan", { method: "POST" }),
  playback: (fileId: number, start = 0) => request<Playback>(`/api/playback/${fileId}?start=${start.toFixed(3)}`),
};

export const episodeCode = (season: number, episode: number) =>
  `S${String(season).padStart(2, "0")}E${String(episode).padStart(2, "0")}`;
