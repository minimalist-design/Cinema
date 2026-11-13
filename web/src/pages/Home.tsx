import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Button, Group, List, Loader, Paper, Stack, Text, ThemeIcon, Title } from "@mantine/core";
import { useDocumentTitle } from "@mantine/hooks";
import { api, type Library } from "../api";
import { useFetch } from "../useFetch";
import { useScan } from "../scan";
import { ForYou } from "../components/ForYou";
import { MediaRow } from "../components/MediaRow";
import { PosterGrid, type MediaItem } from "../components/PosterCard";
import { SearchBox } from "../components/SearchBox";
import { ErrorAlert, Message, PageLoader } from "../components/States";
import { SettingsIcon } from "../components/icons";

const ROW_LIMIT = 40;

export function toItems(lib: Library) {
  const movies: MediaItem[] = lib.movies.map((m) => ({ ...m, kind: "movie" }));
  const shows: MediaItem[] = lib.shows.map((s) => ({ ...s, kind: "show" }));
  return { movies, shows };
}

const matches = (item: MediaItem, q: string) => item.title.toLowerCase().includes(q) || String(item.year ?? "").includes(q);

function SearchResults({ q, movies, shows }: { q: string; movies: MediaItem[]; shows: MediaItem[] }) {
  const m = movies.filter((i) => matches(i, q));
  const s = shows.filter((i) => matches(i, q));
  if (m.length + s.length === 0) return <Message title="No matches" description={`Nothing in your library matches “${q}”.`} />;
  return (
    <Stack gap="xl">
      {[
        { title: "Movies", items: m },
        { title: "TV Shows", items: s },
      ].map((section) =>
        section.items.length > 0 ? (
          <section key={section.title}>
            <Title order={2} fz="h3" mb="sm">
              {section.title}{" "}
              <Text span c="dimmed" fz="md" fw={400}>
                {section.items.length}
              </Text>
            </Title>
            <PosterGrid items={section.items} />
          </section>
        ) : null,
      )}
    </Stack>
  );
}

// Shown while the library is empty: what's left to set up.
function GettingStarted() {
  const { status } = useScan();
  const { data: settings } = useFetch(api.settings, []);
  const { data: folders } = useFetch(api.folders, []);
  const scanning = !!status?.running;

  if (scanning) {
    return (
      <Message
        title={
          <Group gap="xs" justify="center">
            <Loader size="xs" /> Scanning your library
          </Group>
        }
        description="Titles show up here once the scan is done."
      />
    );
  }
  if (!settings || !folders) return <PageLoader />;

  const steps = [
    { done: settings.tmdbTokenSet, label: "Add your TMDB token, used to fetch posters and details", to: "/settings/metadata" },
    { done: folders.length > 0, label: "Add the folders that hold your movies and TV shows", to: "/settings/library" },
    { done: false, label: "Scan the library", to: "/settings/library" },
  ];
  const next = steps.find((s) => !s.done)!;
  return (
    <Paper withBorder p="xl" maw={560} mx="auto" mt="xl">
      <Title order={2} fz="h3">
        Set up your library
      </Title>
      <Text c="dimmed" mt={4} mb="lg">
        Your library is empty. A few steps and it fills up with posters.
      </Text>
      <List spacing="sm" center>
        {steps.map((s, i) => (
          <List.Item
            key={s.label}
            icon={
              <ThemeIcon size={24} radius="xl" color={s.done ? "teal" : "gray"} variant={s.done ? "filled" : "light"}>
                <Text fz="xs" fw={700}>
                  {s.done ? "✓" : i + 1}
                </Text>
              </ThemeIcon>
            }
          >
            <Text c={s.done ? "dimmed" : undefined} td={s.done ? "line-through" : undefined}>
              {s.label}
            </Text>
          </List.Item>
        ))}
      </List>
      <Button component={Link} to={next.to} mt="xl" leftSection={<SettingsIcon />}>
        Open settings
      </Button>
    </Paper>
  );
}

export default function Home() {
  useDocumentTitle("Cinema");
  const [params] = useSearchParams();
  const q = (params.get("q") ?? "").trim().toLowerCase();
  const { finished } = useScan();
  const { data, error, loading } = useFetch(api.library, [finished]);
  const { data: home } = useFetch(api.home, [finished]);

  const items = useMemo(() => (data ? toItems(data) : null), [data]);
  const recent = useMemo(() => {
    if (!items) return [];
    return [...items.movies, ...items.shows]
      .filter((i) => i.addedAt)
      .sort((a, b) => b.addedAt!.localeCompare(a.addedAt!))
      .slice(0, 20);
  }, [items]);

  return (
    <Stack gap={40}>
      <SearchBox hiddenFrom="sm" />
      {error ? <ErrorAlert>{error}</ErrorAlert> : null}
      {loading && !items ? <PageLoader /> : null}
      {items && q ? <SearchResults q={q} movies={items.movies} shows={items.shows} /> : null}
      {items && !q && items.movies.length + items.shows.length === 0 ? <GettingStarted /> : null}
      {items && !q ? (
        <>
          {home ? <ForYou data={home} hasShows={items.shows.length > 0} /> : null}
          <MediaRow title="Recently added" items={recent} withKind />
          <MediaRow title="Movies" items={items.movies.slice(0, ROW_LIMIT)} seeAll="/movies" />
          <MediaRow title="TV Shows" items={items.shows.slice(0, ROW_LIMIT)} seeAll="/shows" />
        </>
      ) : null}
    </Stack>
  );
}
