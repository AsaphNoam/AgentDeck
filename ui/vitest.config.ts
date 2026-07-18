import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "jsdom",
    exclude: ["scripts/**", "**/node_modules/**", "**/dist/**"],
    setupFiles: ["./src/test/setup.ts"],
  },
});
