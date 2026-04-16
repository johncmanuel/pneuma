import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

// for versioning the service worker and other assets
// to ensure clients get updates when a new version is deployed
const buildID = Date.now().toString(36);

export default defineConfig(({ command }) => ({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@pneuma/shared": path.resolve(__dirname, "../packages/shared/src")
    }
  },
  base: command === "build" ? "/player/" : "/",
  server: {
    proxy: {
      "/api": "http://localhost:8989",
      "/ws": { target: "ws://localhost:8989", ws: true }
    }
  },
  build: {
    minify: "esbuild"
  },
  esbuild: {
    drop: ["debugger"]
  },
  define: {
    __PWA_BUILD_ID__: JSON.stringify(buildID)
  }
}));
