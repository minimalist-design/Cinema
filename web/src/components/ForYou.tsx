import { Link } from "react-router-dom";
import { Anchor, AspectRatio, Badge, Box, Button, Center, Grid, Group, Image, Paper, Progress, SimpleGrid, Stack, Text, Title, UnstyledButton } from "@mantine/core";
import { episodeCode, formatRemaining, type ContinueEpisode, type HomeData } from "../api";
import { PosterCard } from "./PosterCard";
import { FilmIcon, PlayIcon } from "./icons";
import classes from "./ForYou.module.css";

function ContinueCard({ item }: { item: ContinueEpisode }) {
  const { episode: ep, show } = item;
  const to = `/watch/episode/${ep.id}`;
  const image = ep.stillUrl ?? show.backdropUrl;
  const resume = item.reason === "resume" && item.position != null && item.duration;
  const title = `${episodeCode(ep.seasonNumber, ep.episodeNumber)} · ${ep.name ?? `Episode ${ep.episodeNumber}`}`;

  return (
    <Paper withBorder className={classes.card}>
      <UnstyledButton component={Link} to={to} className={classes.still} aria-label={`Play ${show.name} ${title}`}>
        <AspectRatio ratio={16 / 9}>
          {image ? (
            <Image src={image} alt="" />
          ) : (
            <Center className={classes.placeholder}>
              <FilmIcon size={48} />
            </Center>
          )}
        </AspectRatio>
        <span className={classes.play}>
          <PlayIcon size={28} />
        </span>
        {resume ? (
          <Progress value={(item.position! / item.duration!) * 100} size={5} radius={0} className={classes.progress} aria-hidden />
        ) : null}
      </UnstyledButton>
      <Stack gap={4} p="md">
        <Group gap="xs" justify="space-between" wrap="nowrap">
          <Anchor component={Link} to={`/shows/${show.id}`} size="sm" c="dimmed" truncate>
            {show.name}
          </Anchor>
          <Badge variant="light" size="sm" style={{ flexShrink: 0 }}>
            {resume ? "In progress" : "Up next"}
          </Badge>
        </Group>
        <Text fw={600} size="lg" lineClamp={1} title={title}>
          {title}
        </Text>
        {ep.overview ? (
          <Text size="sm" c="dimmed" lineClamp={2}>
            {ep.overview}
          </Text>
        ) : null}
        <Group gap="sm" mt="sm">
          <Button component={Link} to={to} leftSection={<PlayIcon />}>
            {resume ? "Resume" : "Play"}
          </Button>
          {resume ? (
            <Text size="sm" c="dimmed">
              {formatRemaining(item.duration! - item.position!)}
            </Text>
          ) : null}
        </Group>
      </Stack>
    </Paper>
  );
}

function NothingToContinue() {
  return (
    <Paper withBorder p="xl" h="100%">
      <Center h="100%" mih={200}>
        <Stack gap={4} align="center" ta="center">
          <Text fw={600}>Pick up where you left off</Text>
          <Text size="sm" c="dimmed" maw={320}>
            Start watching a show, and the episode to continue with shows up here.
          </Text>
        </Stack>
      </Center>
    </Paper>
  );
}

// ForYou is the top of the home page: the TV episode to continue with, and
// movies not watched yet, side by side.
export function ForYou({ data, hasShows }: { data: HomeData; hasShows: boolean }) {
  const left = data.continue ? <ContinueCard item={data.continue} /> : hasShows ? <NothingToContinue /> : null;
  const movies = data.suggestions.map((m) => ({ ...m, kind: "movie" as const }));
  if (!left && movies.length === 0) return null;

  return (
    <Grid gap="xl">
      {left ? (
        <Grid.Col span={{ base: 12, md: movies.length ? 6 : 12 }}>
          <Stack gap="sm" h="100%" maw={movies.length ? undefined : 640}>
            <Title order={2} fz="h3">
              Continue watching
            </Title>
            <Box style={{ flex: 1 }}>{left}</Box>
          </Stack>
        </Grid.Col>
      ) : null}
      {movies.length ? (
        <Grid.Col span={{ base: 12, md: left ? 6 : 12 }}>
          <Stack gap="sm">
            <Group justify="space-between" align="baseline">
              <Title order={2} fz="h3">
                Movies to watch
              </Title>
              <Anchor component={Link} to="/movies" size="sm" fw={500}>
                See all
              </Anchor>
            </Group>
            <SimpleGrid cols={{ base: 4, md: left ? 4 : 8 }} spacing="sm" verticalSpacing="md">
              {movies.map((m) => (
                <PosterCard key={m.id} item={m} />
              ))}
            </SimpleGrid>
          </Stack>
        </Grid.Col>
      ) : null}
    </Grid>
  );
}
