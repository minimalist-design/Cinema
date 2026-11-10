import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Link, Route, Routes } from "react-router-dom";
import Home from "./pages/Home";
import ShowPage from "./pages/ShowPage";
import { EpisodePlayer, MoviePlayer } from "./pages/Player";
import "./layout.css";

function App() {
  return (
    <div className="app">
      <header className="app-header">
        <Link to="/">Cinema</Link>
      </header>
      <main className="app-main">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/shows/:id" element={<ShowPage />} />
          <Route path="/watch/movie/:id" element={<MoviePlayer />} />
          <Route path="/watch/episode/:id" element={<EpisodePlayer />} />
          <Route path="*" element={<p>Page not found.</p>} />
        </Routes>
      </main>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
