import { useMemo } from "react";
import { Group, SegmentedControl, Stack, Text, Title } from "@mantine/core";
import { useDocumentTitle, useLocalStorage } from "@mantine/hooks";
import { api } from "../api";
import { useFetch } from "../useFetch";
import { useScan } from "../scan";
import { PosterGrid, type MediaItem } from "../components/PosterCard";
import { ErrorAlert, Message, PageLoader } from "../components/States";
import { toItems } from "./Home";

type Sort = "title" | "year" | "added";

const sorters: Record<Sort, (a: MediaItem, b: MediaItem) => number> = {
  title: () => 0, // the server sorts by title
  year: (a, b) => (b.year ?? 0) - (a.year ?? 0),
  added: (a, b) => (b.addedAt ?? "").localeCompare(a.addedAt ?? ""),
};

// Browse lists every movie or every TV show ("See all" from the home rows).
export default function Browse({ kind }: { kind: "movie" | "show" }) {
  const title = kind === "movie" ? "Movies" : "TV Shows";
  useDocumentTitle(`${title} · Cinema`);
  const { finished } = useScan();
  const { data, error, loading } = useFetch(api.library, [finished]);
  const [sort, setSort] = useLocalStorage<Sort>({ key: "cinema-browse-sort", defaultValue: "title" });

  const items = useMemo(() => {
    if (!data) return null;
    const { movies, shows } = toItems(data);
    return [...(kind === "movie" ? movies : shows)].sort(sorters[sort]);
  }, [data, kind, sort]);

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading && !items) return <PageLoader />;
  if (!items) return null;

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-end">
        <Title order={1} fz="h2">
          {title}{" "}
          <Text span c="dimmed" fz="lg" fw={400}>
            {items.length}
          </Text>
        </Title>
        <SegmentedControl
          size="xs"
          value={sort}
          onChange={(v) => setSort(v as Sort)}
          data={[
            { value: "title", label: "A–Z" },
            { value: "year", label: "Newest" },
            { value: "added", label: "Recently added" },
          ]}
          aria-label="Sort by"
        />
      </Group>
      {items.length > 0 ? <PosterGrid items={items} /> : <Message title={`No ${title.toLowerCase()} yet`} />}
    </Stack>
  );
}
