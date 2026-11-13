import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Accordion,
  Badge,
  Box,
  Button,
  Code,
  Group,
  Paper,
  Progress,
  ScrollArea,
  SimpleGrid,
  Stack,
  Tabs,
  Text,
  TextInput,
  UnstyledButton,
} from "@mantine/core";
import { useDocumentTitle, useLocalStorage } from "@mantine/hooks";
import { api, type LibraryItem, type ScanStatus } from "../api";
import { useFetch } from "../useFetch";
import { Artwork } from "../components/Artwork";
import { ErrorAlert, Message, PageLoader } from "../components/States";
import { RefreshIcon, SearchIcon } from "../components/icons";
import classes from "./Home.module.css";

function PosterCard({ item, to }: { item: LibraryItem; to: string }) {
  return (
    <UnstyledButton component={Link} to={to} className={classes.card}>
      <Artwork src={item.posterUrl} ratio={2 / 3} />
      <Text fw={500} size="sm" mt="xs" lineClamp={2}>
        {item.title}
      </Text>
      {item.year ? (
        <Text size="xs" c="dimmed">
          {item.year}
        </Text>
      ) : null}
    </UnstyledButton>
  );
}

function PosterGrid({ items, linkTo, empty }: { items: LibraryItem[]; linkTo: (id: number) => string; empty: string }) {
  if (items.length === 0) return <Message title={empty} />;
  return (
    <SimpleGrid cols={{ base: 2, xs: 3, sm: 4, md: 5, lg: 6 }} spacing="md" verticalSpacing="xl">
      {items.map((item) => (
        <PosterCard key={item.id} item={item} to={linkTo(item.id)} />
      ))}
    </SimpleGrid>
  );
}

const phaseLabels: Record<string, string> = {
  starting: "Starting",
  "reference data": "Fetching reference data",
  movies: "Scanning movies",
  tv: "Scanning TV shows",
  indexing: "Indexing keyframes",
  subtitles: "Extracting subtitles",
};

function FileList({ files }: { files: string[] }) {
  return (
    <ScrollArea.Autosize mah={240}>
      <Stack gap={4}>
        {files.map((f) => (
          <Code key={f} block>
            {f}
          </Code>
        ))}
      </Stack>
    </ScrollArea.Autosize>
  );
}

function ScanPanel({ onFinished }: { onFinished: () => void }) {
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

  const running = status?.running ?? false;
  const hasProgress = running && status!.phaseTotal > 0;

  let summary: string;
  if (running) {
    const phase = phaseLabels[status!.phase] ?? status!.phase;
    const progress = hasProgress ? ` (${status!.phaseDone} of ${status!.phaseTotal})` : "";
    summary = `${phase}${progress} · ${status!.filesSeen} files seen, ${status!.filesLinked} linked`;
  } else if (status?.finishedAt) {
    const parts = [
      `${status.filesLinked} linked`,
      `${status.filesIgnored} ignored`,
      `${status.filesRemoved} removed`,
      `${status.filesIndexed} indexed`,
    ];
    if (status.subtitlesExtracted > 0) parts.push(`subtitles extracted from ${status.subtitlesExtracted} files`);
    summary = `Last scan ${new Date(status.finishedAt).toLocaleString()}: ${parts.join(", ")}`;
  } else {
    summary = "Not scanned yet. Scan to pick up files from your movie and TV folders.";
  }

  const showDetails = !running && status && (status.ignored.length > 0 || status.errors.length > 0);

  return (
    <Paper withBorder p="md">
      <Group justify="space-between" wrap="nowrap" align="center">
        <Box miw={0}>
          <Text fw={600}>Library</Text>
          <Text size="sm" c="dimmed">
            {summary}
          </Text>
        </Box>
        <Button onClick={start} loading={running} loaderProps={{ type: "dots" }} leftSection={<RefreshIcon />} variant="light" style={{ flexShrink: 0 }}>
          Rescan
        </Button>
      </Group>

      {running ? (
        <Progress
          mt="md"
          size="sm"
          value={hasProgress ? (status!.phaseDone / status!.phaseTotal) * 100 : 100}
          striped={!hasProgress}
          animated={!hasProgress}
          aria-label="Scan progress"
        />
      ) : null}

      {error ? (
        <Text size="sm" c="red" mt="sm" role="alert">
          {error}
        </Text>
      ) : null}

      {showDetails ? (
        <Accordion variant="contained" mt="md" chevronPosition="left">
          {status.ignored.length > 0 ? (
            <Accordion.Item value="ignored">
              <Accordion.Control>
                <Group gap="xs">
                  <Text size="sm">Ignored files</Text>
                  <Badge size="sm" variant="light" color="gray">
                    {status.ignored.length}
                  </Badge>
                </Group>
              </Accordion.Control>
              <Accordion.Panel>
                <FileList files={status.ignored} />
              </Accordion.Panel>
            </Accordion.Item>
          ) : null}
          {status.errors.length > 0 ? (
            <Accordion.Item value="errors">
              <Accordion.Control>
                <Group gap="xs">
                  <Text size="sm">Errors</Text>
                  <Badge size="sm" variant="light" color="red">
                    {status.errors.length}
                  </Badge>
                </Group>
              </Accordion.Control>
              <Accordion.Panel>
                <FileList files={status.errors} />
              </Accordion.Panel>
            </Accordion.Item>
          ) : null}
        </Accordion>
      ) : null}
    </Paper>
  );
}

function matches(item: LibraryItem, query: string) {
  return item.title.toLowerCase().includes(query) || String(item.year ?? "").includes(query);
}

export default function Home() {
  useDocumentTitle("Cinema");
  const { data, error, loading, reload } = useFetch(api.library, []);
  const [query, setQuery] = useState("");
  const [tab, setTab] = useLocalStorage<string>({ key: "cinema-home-tab", defaultValue: "movies" });

  const q = query.trim().toLowerCase();
  const movies = useMemo(() => (data ? data.movies.filter((m) => matches(m, q)) : []), [data, q]);
  const shows = useMemo(() => (data ? data.shows.filter((s) => matches(s, q)) : []), [data, q]);

  return (
    <Stack gap="lg">
      <ScanPanel onFinished={reload} />
      {error ? <ErrorAlert>{error}</ErrorAlert> : null}
      {loading && !data ? <PageLoader /> : null}
      {data ? (
        <Tabs value={tab} onChange={(v) => v && setTab(v)} keepMounted={false}>
          <Group justify="space-between" align="flex-end" gap="sm" mb="lg">
            <Tabs.List>
              <Tabs.Tab value="movies" rightSection={<Badge size="sm" variant="light" color="gray">{movies.length}</Badge>}>
                Movies
              </Tabs.Tab>
              <Tabs.Tab value="shows" rightSection={<Badge size="sm" variant="light" color="gray">{shows.length}</Badge>}>
                TV Shows
              </Tabs.Tab>
            </Tabs.List>
            <TextInput
              placeholder="Filter by title or year"
              aria-label="Filter by title or year"
              leftSection={<SearchIcon />}
              value={query}
              onChange={(e) => setQuery(e.currentTarget.value)}
              w={{ base: "100%", xs: 260 }}
            />
          </Group>
          <Tabs.Panel value="movies">
            <PosterGrid items={movies} linkTo={(id) => `/watch/movie/${id}`} empty={q ? "No movies match" : "No movies yet"} />
          </Tabs.Panel>
          <Tabs.Panel value="shows">
            <PosterGrid items={shows} linkTo={(id) => `/shows/${id}`} empty={q ? "No shows match" : "No TV shows yet"} />
          </Tabs.Panel>
        </Tabs>
      ) : null}
    </Stack>
  );
}
