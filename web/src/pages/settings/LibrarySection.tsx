import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Accordion,
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Code,
  Group,
  Modal,
  Progress,
  ScrollArea,
  Stack,
  Text,
  ThemeIcon,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { api, type FolderKind, type LibraryFolder, type Settings } from "../../api";
import { useFetch } from "../../useFetch";
import { phaseLabels, useScan } from "../../scan";
import { ErrorAlert } from "../../components/States";
import { AlertIcon, FolderIcon, PlusIcon, RefreshIcon, TrashIcon } from "../../components/icons";
import { SettingsCard, plural } from "./common";
import { FolderPicker } from "./FolderPicker";

const nouns: Record<FolderKind, { titles: [string, string]; files: [string, string] }> = {
  movie: { titles: ["movie", "movies"], files: ["file", "files"] },
  tv: { titles: ["show", "shows"], files: ["episode", "episodes"] },
};

function FolderRow({ folder, onRemove }: { folder: LibraryFolder; onRemove: () => void }) {
  const n = nouns[folder.kind];
  return (
    <Group wrap="nowrap" gap="sm" py="xs">
      <ThemeIcon variant="light" color={folder.exists ? "gray" : "red"} size="lg">
        {folder.exists ? <FolderIcon /> : <AlertIcon />}
      </ThemeIcon>
      <Box miw={0} style={{ flex: 1 }}>
        <Text size="sm" ff="monospace" truncate="start" title={folder.path}>
          {folder.path}
        </Text>
        <Group gap="xs">
          {!folder.exists ? (
            <Badge size="xs" color="red" variant="light">
              Not found
            </Badge>
          ) : null}
          <Text size="xs" c="dimmed">
            {plural(folder.titles, ...n.titles)} · {plural(folder.files, ...n.files)}
          </Text>
        </Group>
      </Box>
      <Tooltip label="Remove from library">
        <ActionIcon variant="subtle" color="gray" onClick={onRemove} aria-label={`Remove ${folder.path}`}>
          <TrashIcon />
        </ActionIcon>
      </Tooltip>
    </Group>
  );
}

function FolderCard({
  kind,
  folders,
  onAdd,
  onRemove,
}: {
  kind: FolderKind;
  folders: LibraryFolder[];
  onAdd: () => void;
  onRemove: (f: LibraryFolder) => void;
}) {
  const movie = kind === "movie";
  return (
    <SettingsCard
      title={movie ? "Movie folders" : "TV show folders"}
      description={
        movie
          ? "Each movie is a video file, on its own or in a folder named after it."
          : "Each show has a folder, with a folder per season inside."
      }
      footer={
        <Button variant="light" leftSection={<PlusIcon />} onClick={onAdd}>
          Add folder
        </Button>
      }
    >
      {folders.length === 0 ? (
        <Text size="sm" c="dimmed">
          No {movie ? "movie" : "TV show"} folders yet.
        </Text>
      ) : (
        <Stack gap={0}>
          {folders.map((f) => (
            <FolderRow key={f.id} folder={f} onRemove={() => onRemove(f)} />
          ))}
        </Stack>
      )}
    </SettingsCard>
  );
}

function RemoveFolderModal({ folder, onClose, onRemoved }: { folder: LibraryFolder | null; onClose: () => void; onRemoved: () => void }) {
  const [removing, setRemoving] = useState(false);
  const remove = async () => {
    if (!folder) return;
    setRemoving(true);
    try {
      const { filesRemoved } = await api.removeFolder(folder.id);
      notifications.show({
        title: "Folder removed",
        message: `${plural(filesRemoved, ...nouns[folder.kind].files)} left the library.`,
      });
      onRemoved();
    } catch (e) {
      notifications.show({ color: "red", title: "Couldn't remove the folder", message: (e as Error).message });
    } finally {
      setRemoving(false);
    }
  };
  const n = folder ? nouns[folder.kind] : nouns.movie;
  return (
    <Modal opened={folder !== null} onClose={onClose} title="Remove folder from the library?">
      {folder ? (
        <Stack>
          <Code block>{folder.path}</Code>
          <Text size="sm">
            Its {plural(folder.titles, ...n.titles)} leave the library. Nothing is deleted from the disk, and you can add the folder
            again later.
          </Text>
          <Group justify="flex-end" gap="sm">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button color="red" onClick={remove} loading={removing}>
              Remove
            </Button>
          </Group>
        </Stack>
      ) : null}
    </Modal>
  );
}

function MessageList({ items }: { items: string[] }) {
  return (
    <ScrollArea.Autosize mah={240}>
      <Stack gap={4}>
        {items.map((m, i) => (
          <Code key={i} block>
            {m}
          </Code>
        ))}
      </Stack>
    </ScrollArea.Autosize>
  );
}

function ScanCard({ settings }: { settings: Settings }) {
  const { status, start } = useScan();
  const [error, setError] = useState<string | null>(null);
  const running = !!status?.running;
  const hasProgress = running && status!.phaseTotal > 0;

  const run = async (refresh: boolean) => {
    setError(null);
    try {
      await start(refresh);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  let summary: string;
  if (running) {
    const phase = phaseLabels[status!.phase] ?? status!.phase;
    const progress = hasProgress ? ` (${status!.phaseDone} of ${status!.phaseTotal})` : "";
    summary = `${phase}${progress} · ${status!.filesSeen} files seen, ${status!.filesLinked} linked`;
    if (status!.queued) summary += " · another scan is queued";
  } else if (status?.finishedAt) {
    const parts = [
      `${status.filesLinked} linked`,
      `${status.filesIgnored} ignored`,
      `${status.filesRemoved} removed`,
      `${status.filesIndexed} indexed`,
    ];
    if (status.subtitlesExtracted > 0) parts.push(`subtitles extracted from ${status.subtitlesExtracted} files`);
    summary = `Last scan ${new Date(status.finishedAt).toLocaleString()}: ${parts.join(", ")}.`;
  } else {
    summary = "No scan since Cinema started.";
  }

  const details = !running && status && (status.ignored.length > 0 || status.errors.length > 0);

  return (
    <SettingsCard
      title="Scan"
      description="Adding a folder scans it. Scan again after you add, rename or delete files."
      footer={
        <>
          <Tooltip label="Download details and artwork again for every title, e.g. after changing the metadata language. Takes a while on a large library." w={280} multiline>
            <Button variant="default" disabled={running || !settings.tmdbTokenSet} onClick={() => run(true)}>
              Refresh all metadata
            </Button>
          </Tooltip>
          <Button leftSection={<RefreshIcon />} loading={running} loaderProps={{ type: "dots" }} disabled={!settings.tmdbTokenSet} onClick={() => run(false)}>
            Scan for changes
          </Button>
        </>
      }
    >
      <Stack gap="sm">
        {!settings.tmdbTokenSet ? (
          <Alert color="yellow" variant="light" icon={<AlertIcon />}>
            Scanning needs a TMDB token.{" "}
            <Text component={Link} to="/settings/metadata" inherit td="underline">
              Add one in Metadata
            </Text>
            .
          </Alert>
        ) : null}
        <Text size="sm">{summary}</Text>
        {running ? (
          <Progress
            size="sm"
            value={hasProgress ? (status!.phaseDone / status!.phaseTotal) * 100 : 100}
            striped={!hasProgress}
            animated={!hasProgress}
            aria-label="Scan progress"
          />
        ) : null}
        {error ? (
          <Text size="sm" c="red" role="alert">
            {error}
          </Text>
        ) : null}
        {details ? (
          <Accordion variant="contained" chevronPosition="left">
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
                  <MessageList items={status.ignored} />
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
                  <MessageList items={status.errors} />
                </Accordion.Panel>
              </Accordion.Item>
            ) : null}
          </Accordion>
        ) : null}
      </Stack>
    </SettingsCard>
  );
}

export function LibrarySection({ settings }: { settings: Settings }) {
  const scan = useScan();
  // Reload after each scan, for the counts.
  const { data: folders, error, reload } = useFetch(api.folders, [scan.finished]);
  const [adding, setAdding] = useState<FolderKind | null>(null);
  const [removing, setRemoving] = useState<LibraryFolder | null>(null);

  if (error) return <ErrorAlert>{error}</ErrorAlert>;
  const list = folders ?? [];

  return (
    <Stack gap="lg">
      <FolderCard kind="movie" folders={list.filter((f) => f.kind === "movie")} onAdd={() => setAdding("movie")} onRemove={setRemoving} />
      <FolderCard kind="tv" folders={list.filter((f) => f.kind === "tv")} onAdd={() => setAdding("tv")} onRemove={setRemoving} />
      <ScanCard settings={settings} />

      <FolderPicker
        kind={adding}
        onClose={() => setAdding(null)}
        onAdded={(f) => {
          setAdding(null);
          reload();
          scan.refresh(); // adding a folder starts a scan
          notifications.show({
            color: "teal",
            title: "Folder added",
            message: settings.tmdbTokenSet ? `Scanning ${f.path}…` : `${f.path} will be scanned once a TMDB token is set.`,
          });
        }}
      />
      <RemoveFolderModal
        folder={removing}
        onClose={() => setRemoving(null)}
        onRemoved={() => {
          setRemoving(null);
          reload();
        }}
      />
    </Stack>
  );
}
