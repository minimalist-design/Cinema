import { useParams } from "react-router-dom";
import { Box, Flex, Spoiler, Stack, Text, Title } from "@mantine/core";
import { useDocumentTitle } from "@mantine/hooks";
import { api, formatDate } from "../api";
import { useFetch } from "../useFetch";
import { Artwork } from "../components/Artwork";
import { EpisodeBrowser } from "../components/EpisodeBrowser";
import { ErrorAlert, PageLoader } from "../components/States";

export default function ShowPage() {
  const { id = "" } = useParams();
  const { data: show, error, loading } = useFetch(() => api.show(id), [id]);
  useDocumentTitle(show ? `${show.name} · Cinema` : "Cinema");

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  if (loading || !show) return <PageLoader />;

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

      <EpisodeBrowser show={show} />
    </Stack>
  );
}
