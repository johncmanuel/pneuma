import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

export default defineConfig(({ command }) => ({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@pneuma/shared": path.resolve(__dirname, "../packages/shared/src"),
      "@pneuma/ui": path.resolve(__dirname, "../packages/ui/src/lib/index.ts")
    }
  },
  base: command === "build" ? "/dashboard/" : "/",
  build: {
    outDir: "dist",
    emptyOutDir: true
  }
}));
