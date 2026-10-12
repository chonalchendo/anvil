import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    environmentOptions: { jsdom: { url: "http://localhost/" } },
    setupFiles: ["src/test/setup.ts"],
    // Seeding a vault shells out to `anvil init` and `anvil reindex`.
    hookTimeout: 30000,
  },
});
