import { useState } from "react";
import type { ReactNode, Ref } from "react";
import { Link, useParams } from "react-router-dom";
import { Anchor, AspectRatio, Box, Breadcrumbs, Button, Center, Divider, Group, Loader, Stack, Text, Title } from "@mantine/core";
import { useDocumentTitle } from "@mantine/hooks";
import { api, episodeCode, formatDate, formatDuration } from "../api";
import type { SubtitleTrack } from "../api";
import { EpisodeBrowser } from "../components/EpisodeBrowser";
import { HlsVideo } from "../components/HlsVideo";
import { ErrorAlert, PageLoader } from "../components/States";
import { ArrowRightIcon } from "../components/icons";
import { useFetch } from "../useFetch";
import { useWatchProgress } from "../useWatchProgress";
import classes from "./Player.module.css";

// Subtitles show up in the browser's own captions menu.
function SubtitleTracks({ tracks }: { tracks: SubtitleTrack[] }) {
  return tracks.map((t) => (
    <track key={t.id} kind="subtitles" src={t.url} label={t.label} srcLang={t.language || undefined} default={t.default} />
  ));
}

function Video({ fileId, startAt, videoRef }: { fileId: number; startAt: number | null; videoRef: Ref<HTMLVideoElement> }) {
  const { data: playback, error, loading } = useFetch(() => api.playback(fileId), [fileId]);

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading || !playback) {
    return (
      <AspectRatio ratio={16 / 9} className={classes.stage}>
        <Center>
          <Loader color="gray" />
        </Center>
      </AspectRatio>
    );
  }
  const tracks = <SubtitleTracks tracks={playback.subtitles} />;
  if (playback.mode === "hls") {
    return (
      <HlsVideo src={playback.url} className={classes.video} frameClassName={classes.stage} startAt={startAt} videoRef={videoRef}>
        {tracks}
      </HlsVideo>
    );
  }
  return (
    <Box className={classes.stage}>
      <video ref={videoRef} className={classes.video} src={playback.url} controls autoPlay preload="metadata">
        {tracks}
      </video>
    </Box>
  );
}

interface PlayerLayoutProps {
  fileId: number;
  item: { kind: "movie" | "episode"; id: number };
  resumeAt: number | null;
  children: ReactNode;
}

// PlayerLayout plays a title's file, saving how far it's watched and
// resuming where it was left.
function PlayerLayout({ fileId, item, resumeAt, children }: PlayerLayoutProps) {
  const [video, setVideo] = useState<HTMLVideoElement | null>(null);
  const [startedOver, setStartedOver] = useState(false);
  useWatchProgress(video, item, resumeAt);

  return (
    <Stack gap="lg">
      {/* key gives each file a fresh player (and fresh playback info) */}
      <Video key={fileId} fileId={fileId} startAt={resumeAt} videoRef={setVideo} />
      {resumeAt && !startedOver ? (
        <Group gap="xs" mt={-8}>
          <Text size="sm" c="dimmed">
            Resumed at {formatDuration(resumeAt)}.
          </Text>
          <Button
            variant="subtle"
            size="compact-sm"
            onClick={() => {
              if (video) video.currentTime = 0;
              setStartedOver(true);
            }}
          >
            Start over
          </Button>
        </Group>
      ) : null}
      {children}
    </Stack>
  );
}

function Overview({ text }: { text: string | null }) {
  return text ? (
    <Text maw={760} mt="xs">
      {text}
    </Text>
  ) : null;
}

export function MoviePlayer() {
  const { id = "" } = useParams();
  const { data: movie, error, loading } = useFetch(() => api.movie(id), [id]);
  useDocumentTitle(movie ? `${movie.title} · Cinema` : "Cinema");

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading || !movie) return <PageLoader />;

  return (
    <PlayerLayout fileId={movie.fileId} item={{ kind: "movie", id: movie.id }} resumeAt={movie.resumeAt}>
      <Box>
        <Title order={2}>{movie.title}</Title>
        {movie.releaseDate ? <Text c="dimmed">Released {formatDate(movie.releaseDate)}</Text> : null}
        <Overview text={movie.overview} />
      </Box>
    </PlayerLayout>
  );
}

// ShowEpisodes lists the show's episodes below the player, marking the one
// playing.
function ShowEpisodes({ showId, current }: { showId: number; current: { id: number; seasonNumber: number } }) {
  const { data: show, error } = useFetch(() => api.show(String(showId)), [showId]);
  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (!show) return <PageLoader />;
  return (
    <Box component="section">
      <Title order={3} mb="md">
        Episodes
      </Title>
      <EpisodeBrowser show={show} current={current} />
    </Box>
  );
}

export function EpisodePlayer() {
  const { id = "" } = useParams();
  const { data: ep, error, loading } = useFetch(() => api.episode(id), [id]);
  const code = ep ? episodeCode(ep.seasonNumber, ep.episodeNumber) : "";
  useDocumentTitle(ep ? `${ep.show.name} ${code} · Cinema` : "Cinema");

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading || !ep) return <PageLoader />;

  return (
    <PlayerLayout fileId={ep.fileId} item={{ kind: "episode", id: ep.id }} resumeAt={ep.resumeAt}>
      <Group justify="space-between" align="flex-start" gap="md">
        <Box miw={0} style={{ flex: 1 }}>
          <Breadcrumbs separator="·" mb={4}>
            <Anchor component={Link} to={`/shows/${ep.show.id}`} size="sm">
              {ep.show.name}
            </Anchor>
            <Text size="sm" c="dimmed" ff="monospace">
              {code}
            </Text>
          </Breadcrumbs>
          <Title order={2}>{ep.name ?? `Episode ${ep.episodeNumber}`}</Title>
          {ep.airDate ? <Text c="dimmed">Aired {formatDate(ep.airDate)}</Text> : null}
          <Overview text={ep.overview} />
        </Box>
        {ep.nextEpisodeId ? (
          <Button component={Link} to={`/watch/episode/${ep.nextEpisodeId}`} rightSection={<ArrowRightIcon />}>
            Next episode
          </Button>
        ) : null}
      </Group>
      <Divider />
      <ShowEpisodes showId={ep.show.id} current={{ id: ep.id, seasonNumber: ep.seasonNumber }} />
    </PlayerLayout>
  );
}
