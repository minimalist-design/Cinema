import { useEffect, useState } from "react";
import { ActionIcon, Box, Button, Code, Group, Modal, NavLink, Paper, ScrollArea, Stack, Text, TextInput, Tooltip } from "@mantine/core";
import { api, type BrowseResult, type FolderKind, type LibraryFolder } from "../../api";
import { ArrowUpIcon, FolderIcon, HomeIcon } from "../../components/icons";

const layouts: Record<FolderKind, { title: string; example: string[] }> = {
  movie: {
    title: "Add a movie folder",
    example: ["Heat (1995)/Heat (1995).mkv", "Alien (1979).mp4"],
  },
  tv: {
    title: "Add a TV show folder",
    example: ["Breaking Bad (2008)/Season 01/Breaking Bad (2008) - s01e01.mkv"],
  },
};

// FolderPicker adds a library folder: type a path, or browse the server's
// folders to it.
export function FolderPicker({
  kind,
  onClose,
  onAdded,
}: {
  kind: FolderKind | null;
  onClose: () => void;
  onAdded: (f: LibraryFolder) => void;
}) {
  const [path, setPath] = useState("");
  const [listing, setListing] = useState<BrowseResult | null>(null);
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  const browse = async (p?: string) => {
    try {
      const res = await api.browse(p);
      setListing(res);
      setPath(res.path);
      setBrowseError(null);
    } catch (e) {
      setBrowseError((e as Error).message);
    }
  };

  useEffect(() => {
    if (!kind) return;
    setError(null);
    browse(listing?.path);
    // Reopening starts where the last browse ended.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind]);

  const add = async () => {
    if (!kind) return;
    setAdding(true);
    setError(null);
    try {
      onAdded(await api.addFolder(kind, path));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setAdding(false);
    }
  };

  const layout = layouts[kind ?? "movie"];

  return (
    <Modal opened={kind !== null} onClose={onClose} title={layout.title} size="lg">
      <Stack gap="md">
        <Box>
          <Text size="sm" c="dimmed">
            Titles are matched by their file and folder names, for example:
          </Text>
          <Stack gap={4} mt={6}>
            {layout.example.map((e) => (
              <Code key={e} block>
                {e}
              </Code>
            ))}
          </Stack>
        </Box>

        <TextInput
          label="Folder"
          description="Type a path and press Enter, or pick a folder below."
          value={path}
          onChange={(e) => setPath(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              browse(path);
            }
          }}
          error={error}
          ff="monospace"
          data-autofocus
        />

        <Paper withBorder>
          <Group gap={4} p={6} wrap="nowrap" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
            <Tooltip label="Up one folder">
              <ActionIcon variant="subtle" color="gray" disabled={!listing?.parent} onClick={() => browse(listing?.parent ?? undefined)} aria-label="Up one folder">
                <ArrowUpIcon />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Home folder">
              <ActionIcon variant="subtle" color="gray" onClick={() => browse(listing?.home)} aria-label="Home folder">
                <HomeIcon />
              </ActionIcon>
            </Tooltip>
            <Text size="sm" ff="monospace" truncate="start" title={listing?.path}>
              {listing?.path ?? "…"}
            </Text>
          </Group>
          <ScrollArea h={260} type="auto">
            {browseError ? (
              <Text size="sm" c="red" p="md">
                {browseError}
              </Text>
            ) : listing && listing.dirs.length === 0 ? (
              <Text size="sm" c="dimmed" p="md">
                No folders in here.
              </Text>
            ) : (
              listing?.dirs.map((d) => (
                <NavLink key={d.path} label={d.name} leftSection={<FolderIcon size={16} />} onClick={() => browse(d.path)} />
              ))
            )}
          </ScrollArea>
        </Paper>

        <Group justify="flex-end" gap="sm">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={add} loading={adding} disabled={!path.trim()}>
            Add folder
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
