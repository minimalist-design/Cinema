import { Link, useParams } from "react-router-dom";
import { Box, Flex, Spoiler, Stack, Tabs, Text, Title, UnstyledButton } from "@mantine/core";
import { useDocumentTitle } from "@mantine/hooks";
import { api, episodeCode, formatDate, type Season } from "../api";
import { useFetch } from "../useFetch";
import { Artwork } from "../components/Artwork";
import { ErrorAlert, Message, PageLoader } from "../components/States";
import classes from "./ShowPage.module.css";

function EpisodeList({ season }: { season: Season }) {
  if (season.episodes.length === 0) return <Message title="No episodes in your library" />;
  return (
    <Stack gap={4} component="ol" className={classes.list}>
      {season.episodes.map((ep) => (
        <li key={ep.id}>
          <UnstyledButton component={Link} to={`/watch/episode/${ep.id}`} className={classes.episode}>
            <Box w={{ base: 128, sm: 176 }} style={{ flexShrink: 0 }}>
              <Artwork src={ep.stillUrl} ratio={16 / 9} />
            </Box>
            <Box miw={0}>
              <Text size="xs" c="dimmed" ff="monospace">
                {episodeCode(season.seasonNumber, ep.episodeNumber)}
              </Text>
              <Text fw={500} lineClamp={2}>
                {ep.name ?? `Episode ${ep.episodeNumber}`}
              </Text>
              {ep.airDate ? (
                <Text size="sm" c="dimmed">
                  {formatDate(ep.airDate)}
                </Text>
              ) : null}
            </Box>
          </UnstyledButton>
        </li>
      ))}
    </Stack>
  );
}

export default function ShowPage() {
  const { id = "" } = useParams();
  const { data: show, error, loading } = useFetch(() => api.show(id), [id]);
  useDocumentTitle(show ? `${show.name} · Cinema` : "Cinema");

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading || !show) return <PageLoader />;

  // Open on the first regular season rather than Specials (season 0).
  const firstSeason = show.seasons.find((s) => s.seasonNumber > 0) ?? show.seasons[0];
  const seasonName = (s: Season) => s.name ?? `Season ${s.seasonNumber}`;

  return (
    <Stack gap="xl">
      <Flex direction={{ base: "column", xs: "row" }} gap="xl" align={{ base: "center", xs: "flex-start" }}>
        <Box w={{ base: 180, sm: 220 }} style={{ flexShrink: 0 }}>
          <Artwork src={show.posterUrl} ratio={2 / 3} lazy={false} />
        </Box>
        <Stack gap="xs" maw={720}>
          <Title order={1}>{show.name}</Title>
          <Text c="dimmed">
            {[
              show.firstAirDate ? `First aired ${formatDate(show.firstAirDate)}` : null,
              `${show.seasons.length} ${show.seasons.length === 1 ? "season" : "seasons"}`,
            ]
              .filter(Boolean)
              .join(" · ")}
          </Text>
          {show.overview ? (
            <Spoiler maxHeight={120} showLabel="Show more" hideLabel="Show less" mt="xs">
              <Text>{show.overview}</Text>
            </Spoiler>
          ) : null}
        </Stack>
      </Flex>

      {firstSeason ? (
        <Tabs key={show.id} defaultValue={String(firstSeason.seasonNumber)} keepMounted={false}>
          <Tabs.List mb="md" className={classes.tabs}>
            {show.seasons.map((s) => (
              <Tabs.Tab key={s.seasonNumber} value={String(s.seasonNumber)}>
                {seasonName(s)}
              </Tabs.Tab>
            ))}
          </Tabs.List>
          {show.seasons.map((s) => (
            <Tabs.Panel key={s.seasonNumber} value={String(s.seasonNumber)}>
              <EpisodeList season={s} />
            </Tabs.Panel>
          ))}
        </Tabs>
      ) : (
        <Message title="No episodes in your library" />
      )}
    </Stack>
  );
}
