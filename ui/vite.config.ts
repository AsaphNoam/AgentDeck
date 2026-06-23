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
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:4317",
    },
  },
});
