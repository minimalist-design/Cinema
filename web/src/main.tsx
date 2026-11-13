import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import { StrictMode, Suspense, lazy } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Link, Route, Routes } from "react-router-dom";
import {
  ActionIcon,
  Anchor,
  AppShell,
  Button,
  Container,
  Group,
  Loader,
  MantineProvider,
  Tooltip,
  createTheme,
  useComputedColorScheme,
  useMantineColorScheme,
} from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import Home from "./pages/Home";
import Browse from "./pages/Browse";
import ShowPage from "./pages/ShowPage";
import { EpisodePlayer, MoviePlayer } from "./pages/Player";
import { ScanProvider, phaseLabels, useScan } from "./scan";
import { Message, PageLoader } from "./components/States";
import { SearchBox } from "./components/SearchBox";
import { MoonIcon, SettingsIcon, SunIcon } from "./components/icons";

// Settings are visited rarely: load them on demand.
const SettingsPage = lazy(() => import("./pages/settings/SettingsPage"));

const theme = createTheme({
  primaryColor: "orange",
  autoContrast: true,
  defaultRadius: "md",
});

function ColorSchemeToggle() {
  const { setColorScheme } = useMantineColorScheme();
  const scheme = useComputedColorScheme("dark", { getInitialValueInEffect: true });
  const next = scheme === "dark" ? "light" : "dark";
  return (
    <Tooltip label={`Switch to ${next} theme`}>
      <ActionIcon variant="subtle" color="gray" size="lg" onClick={() => setColorScheme(next)} aria-label={`Switch to ${next} theme`}>
        {scheme === "dark" ? <SunIcon /> : <MoonIcon />}
      </ActionIcon>
    </Tooltip>
  );
}

// ScanIndicator shows a running scan in the header, linking to its details.
function ScanIndicator() {
  const { status } = useScan();
  if (!status?.running) return null;
  const phase = phaseLabels[status.phase] ?? status.phase;
  const progress = status.phaseTotal > 0 ? ` (${status.phaseDone} of ${status.phaseTotal})` : "";
  return (
    <Tooltip label={`${phase}${progress}`}>
      <Button
        component={Link}
        to="/settings/library"
        variant="subtle"
        color="gray"
        size="compact-sm"
        leftSection={<Loader size={14} color="orange" />}
      >
        Scanning
      </Button>
    </Tooltip>
  );
}

function App() {
  return (
    <AppShell header={{ height: 60 }}>
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between" wrap="nowrap" gap="md">
            <Anchor component={Link} to="/" c="inherit" fw={800} fz="xl" underline="never" style={{ letterSpacing: "-0.02em" }}>
              Cinema
            </Anchor>
            <SearchBox visibleFrom="sm" style={{ flex: 1, maxWidth: 480 }} />
            <Group gap={4} wrap="nowrap">
              <ScanIndicator />
              <Tooltip label="Settings">
                <ActionIcon component={Link} to="/settings" variant="subtle" color="gray" size="lg" aria-label="Settings">
                  <SettingsIcon />
                </ActionIcon>
              </Tooltip>
              <ColorSchemeToggle />
            </Group>
          </Group>
        </Container>
      </AppShell.Header>
      <AppShell.Main>
        <Container size="xl" py="lg">
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="/" element={<Home />} />
              <Route path="/movies" element={<Browse kind="movie" />} />
              <Route path="/shows" element={<Browse kind="show" />} />
              <Route path="/shows/:id" element={<ShowPage />} />
              <Route path="/watch/movie/:id" element={<MoviePlayer />} />
              <Route path="/watch/episode/:id" element={<EpisodePlayer />} />
              <Route path="/settings/:section?" element={<SettingsPage />} />
              <Route path="*" element={<Message title="Page not found" />} />
            </Routes>
          </Suspense>
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="dark">
      <Notifications position="bottom-right" />
      <BrowserRouter>
        <ScanProvider>
          <App />
        </ScanProvider>
      </BrowserRouter>
    </MantineProvider>
  </StrictMode>,
);
