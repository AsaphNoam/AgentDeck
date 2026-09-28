import { resolve } from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go server serves the built UI from an embedded copy of `dist`, so base is
// "/" and the output dir is "dist". During development, `vite dev` runs on :5173
// and proxies /api to the Go server (CORS allows the dev origin).
export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    // Two entries share one build and embed: the desktop app and the phone app
    // the tailnet listener serves (TS-08.R73, TS-13.R14).
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
        remote: resolve(__dirname, "remote.html"),
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:4317",
    },
  },
});
