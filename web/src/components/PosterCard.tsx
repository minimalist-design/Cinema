import { Link } from "react-router-dom";
import { Progress, SimpleGrid, Text, UnstyledButton } from "@mantine/core";
import type { LibraryItem } from "../api";
import { Artwork } from "./Artwork";
import classes from "./PosterCard.module.css";

export type MediaKind = "movie" | "show";

export interface MediaItem extends LibraryItem {
  kind: MediaKind;
  // How much was watched (0 to 1), shown as a bar under the poster.
  progress?: number | null;
}

export const itemLink = (item: MediaItem) => (item.kind === "movie" ? `/watch/movie/${item.id}` : `/shows/${item.id}`);

// PosterCard is a title's poster with its name and year. withKind adds
// "Movie" / "TV show" for rows that mix both.
export function PosterCard({ item, withKind = false }: { item: MediaItem; withKind?: boolean }) {
  const meta = [item.year, withKind ? (item.kind === "movie" ? "Movie" : "TV show") : null].filter(Boolean).join(" · ");
  return (
    <UnstyledButton component={Link} to={itemLink(item)} className={classes.card}>
      <Artwork src={item.posterUrl} ratio={2 / 3} />
      {item.progress ? (
        <Progress value={item.progress * 100} size={4} mt={6} aria-label={`${Math.round(item.progress * 100)}% watched`} />
      ) : null}
      <Text fw={500} size="sm" mt="xs" lineClamp={2}>
        {item.title}
      </Text>
      {meta ? (
        <Text size="xs" c="dimmed">
          {meta}
        </Text>
      ) : null}
    </UnstyledButton>
  );
}

export function PosterGrid({ items, withKind = false }: { items: MediaItem[]; withKind?: boolean }) {
  return (
    <SimpleGrid cols={{ base: 2, xs: 3, sm: 4, md: 5, lg: 6 }} spacing="md" verticalSpacing="xl">
      {items.map((item) => (
        <PosterCard key={`${item.kind}-${item.id}`} item={item} withKind={withKind} />
      ))}
    </SimpleGrid>
  );
}
