import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@pneuma/shared": path.resolve(__dirname, "../packages/shared/src")
    }
  },
  build: {
    minify: "esbuild"
  },
  esbuild: {
    drop: ["debugger"]
  }
});
