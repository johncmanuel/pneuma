import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";
import { resolveAppVersion } from "../scripts/app-version";

const appVersion = resolveAppVersion();

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
  },
  define: {
    __APP_VERSION__: JSON.stringify(appVersion)
  }
});
