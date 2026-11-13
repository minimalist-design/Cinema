import { useState } from "react";
import type { ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Stack, Tabs, Title } from "@mantine/core";
import { useDocumentTitle, useMediaQuery } from "@mantine/hooks";
import { api, type Settings } from "../../api";
import { useFetch } from "../../useFetch";
import { ErrorAlert, PageLoader } from "../../components/States";
import { CaptionsIcon, InfoIcon, LibraryIcon, ServerIcon } from "../../components/icons";
import { LibrarySection } from "./LibrarySection";
import { MetadataSection } from "./MetadataSection";
import { PlaybackSection } from "./PlaybackSection";
import { ServerSection } from "./ServerSection";

const sections: { value: string; label: string; icon: ReactNode }[] = [
  { value: "library", label: "Library", icon: <LibraryIcon /> },
  { value: "metadata", label: "Metadata", icon: <InfoIcon /> },
  { value: "playback", label: "Playback", icon: <CaptionsIcon /> },
  { value: "server", label: "Server", icon: <ServerIcon /> },
];

export default function SettingsPage() {
  useDocumentTitle("Settings · Cinema");
  const { section = "library" } = useParams();
  const navigate = useNavigate();
  const wide = useMediaQuery("(min-width: 62em)", true);
  const { data, error, loading } = useFetch(api.settings, []);
  // The latest settings, as returned by a save.
  const [saved, setSaved] = useState<Settings | null>(null);
  const settings = saved ?? data;

  const active = sections.some((s) => s.value === section) ? section : "library";

  let content: ReactNode;
  if (error) content = <ErrorAlert>{error}</ErrorAlert>;
  else if (loading || !settings) content = <PageLoader />;
  else if (active === "library") content = <LibrarySection settings={settings} />;
  else if (active === "metadata") content = <MetadataSection settings={settings} onSaved={setSaved} />;
  else if (active === "playback") content = <PlaybackSection settings={settings} onSaved={setSaved} />;
  else content = <ServerSection settings={settings} />;

  return (
    <Stack gap="lg">
      <Title order={1} fz="h2">
        Settings
      </Title>
      <Tabs
        value={active}
        onChange={(v) => v && navigate(`/settings/${v}`, { replace: true })}
        orientation={wide ? "vertical" : "horizontal"}
        variant="pills"
        keepMounted={false}
      >
        <Tabs.List w={wide ? 200 : undefined} mb={wide ? 0 : "lg"} style={wide ? { alignSelf: "flex-start" } : undefined}>
          {sections.map((s) => (
            <Tabs.Tab key={s.value} value={s.value} leftSection={s.icon}>
              {s.label}
            </Tabs.Tab>
          ))}
        </Tabs.List>
        {sections.map((s) => (
          <Tabs.Panel key={s.value} value={s.value} pl={wide ? "xl" : 0} maw={wide ? 960 : undefined}>
            {content}
          </Tabs.Panel>
        ))}
      </Tabs>
    </Stack>
  );
}
