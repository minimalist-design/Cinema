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
  filesIndexed: number;
  subtitlesExtracted: number;
  // Progress through the current phase, when it reports any (0/0 otherwise).
  phaseDone: number;
  phaseTotal: number;
  ignored: string[];
  errors: string[];
}

// How to play a file. "direct" files are served as-is (with range support);
// "hls" ones are transcoded on the server and streamed as HLS.
export interface Playback {
  mode: "direct" | "hls";
  url: string;
  subtitles: SubtitleTrack[];
}

// A text subtitle track, built into the file or from a side file, served
// as WebVTT. At most one is marked default: the one to show at start.
export interface SubtitleTrack {
  id: string;
  label: string;
  language: string;
  forced: boolean;
  hearingImpaired: boolean;
  external: boolean;
  default: boolean;
  url: string;
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
  playback: (fileId: number) => request<Playback>(`/api/playback/${fileId}`),
};

export const episodeCode = (season: number, episode: number) =>
  `S${String(season).padStart(2, "0")}E${String(episode).padStart(2, "0")}`;

// formatDate turns a TMDB date (YYYY-MM-DD) into a readable local one.
export function formatDate(date: string) {
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric" });
}
