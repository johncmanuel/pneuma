import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig(({ command }) => ({
  plugins: [svelte()],
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
  }
}));
