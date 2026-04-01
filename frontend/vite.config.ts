import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
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
  build: {
    minify: "esbuild"
  },
  esbuild: {
    drop: ["debugger"]
  }
});
