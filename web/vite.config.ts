import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In development, run the Go server on :8080 and use `npm run dev`;
// API, stream and image requests are proxied to it.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/images": "http://localhost:8080",
    },
  },
});
