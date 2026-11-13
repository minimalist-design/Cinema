import { Link } from "react-router-dom";
import { Badge, Box, Group, Stack, Tabs, Text, UnstyledButton } from "@mantine/core";
import { episodeCode, formatDate, type Season, type Show } from "../api";
import { Artwork } from "./Artwork";
import { Message } from "./States";
import classes from "./EpisodeBrowser.module.css";

function EpisodeList({ season, currentId }: { season: Season; currentId?: number }) {
  if (season.episodes.length === 0) return <Message title="No episodes in your library" />;
  return (
    <Stack gap={4} component="ol" className={classes.list}>
      {season.episodes.map((ep) => {
        const current = ep.id === currentId;
        const content = (
          <>
            <Box w={{ base: 128, sm: 176 }} style={{ flexShrink: 0 }}>
              <Artwork src={ep.stillUrl} ratio={16 / 9} />
            </Box>
            <Box miw={0}>
              <Group gap="xs">
                <Text size="xs" c="dimmed" ff="monospace">
                  {episodeCode(season.seasonNumber, ep.episodeNumber)}
                </Text>
                {current ? (
                  <Badge size="xs" variant="filled">
                    Playing
                  </Badge>
                ) : null}
              </Group>
              <Text fw={500} lineClamp={2}>
                {ep.name ?? `Episode ${ep.episodeNumber}`}
              </Text>
              {ep.airDate ? (
                <Text size="sm" c="dimmed">
                  {formatDate(ep.airDate)}
                </Text>
              ) : null}
            </Box>
          </>
        );
        return (
          <li key={ep.id}>
            {current ? (
              <div className={`${classes.episode} ${classes.current}`} aria-current="true">
                {content}
              </div>
            ) : (
              <UnstyledButton component={Link} to={`/watch/episode/${ep.id}`} className={classes.episode}>
                {content}
              </UnstyledButton>
            )}
          </li>
        );
      })}
    </Stack>
  );
}

// EpisodeBrowser lists a show's episodes in the library, a tab per season.
// With current (the episode playing), it opens on that episode's season and
// marks it; otherwise on the first regular season rather than Specials.
export function EpisodeBrowser({ show, current }: { show: Show; current?: { id: number; seasonNumber: number } }) {
  const start =
    show.seasons.find((s) => s.seasonNumber === current?.seasonNumber) ??
    show.seasons.find((s) => s.seasonNumber > 0) ??
    show.seasons[0];
  if (!start) return <Message title="No episodes in your library" />;

  return (
    // Keyed by season, so moving to an episode of another season follows it.
    <Tabs key={`${show.id}-${start.seasonNumber}`} defaultValue={String(start.seasonNumber)} keepMounted={false}>
      <Tabs.List mb="md" className={classes.tabs}>
        {show.seasons.map((s) => (
          <Tabs.Tab key={s.seasonNumber} value={String(s.seasonNumber)}>
            {s.name ?? `Season ${s.seasonNumber}`}
          </Tabs.Tab>
        ))}
      </Tabs.List>
      {show.seasons.map((s) => (
        <Tabs.Panel key={s.seasonNumber} value={String(s.seasonNumber)}>
          <EpisodeList season={s} currentId={current?.id} />
        </Tabs.Panel>
      ))}
    </Tabs>
  );
}
