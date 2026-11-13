import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { api, type ScanStatus } from "./api";

interface ScanContextValue {
  status: ScanStatus | null;
  // Starts a scan (refresh re-fetches all metadata).
  start: (refresh?: boolean) => Promise<void>;
  // Re-reads the status, e.g. after an action that started a scan.
  refresh: () => void;
  // Increments each time a scan finishes, so pages can reload.
  finished: number;
}

const ScanContext = createContext<ScanContextValue | null>(null);

// ScanProvider shares the scan status, polling while a scan is running.
export function ScanProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ScanStatus | null>(null);
  const [finished, setFinished] = useState(0);

  const refresh = useCallback(() => {
    api.scanStatus().then(setStatus, () => {});
  }, []);

  useEffect(refresh, [refresh]);

  const active = !!status && (status.running || status.queued);
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(async () => {
      try {
        const next = await api.scanStatus();
        setStatus(next);
        if (!next.running && !next.queued) setFinished((n) => n + 1);
      } catch {
        // keep polling; the server may be restarting
      }
    }, 2000);
    return () => clearInterval(timer);
  }, [active]);

  const start = useCallback(async (refreshAll = false) => {
    setStatus(await api.startScan(refreshAll));
  }, []);

  return <ScanContext.Provider value={{ status, start, refresh, finished }}>{children}</ScanContext.Provider>;
}

export function useScan() {
  const ctx = useContext(ScanContext);
  if (!ctx) throw new Error("useScan outside ScanProvider");
  return ctx;
}

export const phaseLabels: Record<string, string> = {
  starting: "Starting",
  "reference data": "Fetching reference data",
  movies: "Scanning movies",
  tv: "Scanning TV shows",
  indexing: "Indexing keyframes",
  subtitles: "Extracting subtitles",
};
