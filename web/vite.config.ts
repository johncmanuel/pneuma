import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

export default defineConfig(({ command }) => ({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@pneuma/shared": path.resolve(
        __dirname,
        "../packages/shared/src/index.ts"
      ),
      "@pneuma/ui": path.resolve(__dirname, "../packages/ui/src/lib/index.ts")
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
  }
}));
