import "@mantine/core/styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Link, Route, Routes } from "react-router-dom";
import {
  ActionIcon,
  Anchor,
  AppShell,
  Container,
  Group,
  MantineProvider,
  createTheme,
  useComputedColorScheme,
  useMantineColorScheme,
} from "@mantine/core";
import Home from "./pages/Home";
import ShowPage from "./pages/ShowPage";
import { EpisodePlayer, MoviePlayer } from "./pages/Player";
import { Message } from "./components/States";
import { MoonIcon, SunIcon } from "./components/icons";

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
    <ActionIcon variant="subtle" color="gray" size="lg" onClick={() => setColorScheme(next)} aria-label={`Switch to ${next} theme`}>
      {scheme === "dark" ? <SunIcon /> : <MoonIcon />}
    </ActionIcon>
  );
}

function App() {
  return (
    <AppShell header={{ height: 56 }}>
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between">
            <Anchor component={Link} to="/" c="inherit" fw={700} fz="xl" underline="never">
              Cinema
            </Anchor>
            <ColorSchemeToggle />
          </Group>
        </Container>
      </AppShell.Header>
      <AppShell.Main>
        <Container size="xl" py="lg">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/shows/:id" element={<ShowPage />} />
            <Route path="/watch/movie/:id" element={<MoviePlayer />} />
            <Route path="/watch/episode/:id" element={<EpisodePlayer />} />
            <Route path="*" element={<Message title="Page not found" />} />
          </Routes>
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="dark">
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </MantineProvider>
  </StrictMode>,
);
