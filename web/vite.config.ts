import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";
import fs from "fs";

// for versioning the service worker and other assets
// to ensure clients get updates when a new version is deployed
const buildID = Date.now().toString(36);

/** Plugin that injects the build ID into the service worker file */
const injectSWVersionPlugin = () => ({
  name: "inject-sw-version",
  configureServer(server: any) {
    server.middlewares.use((req: any, res: any, next: any) => {
      const targetUrl = server.config.base + "sw.js";
      if (req.url === targetUrl || req.url === "/sw.js") {
        const swPath = path.resolve(__dirname, "public", "sw.js");
        if (fs.existsSync(swPath)) {
          let content = fs.readFileSync(swPath, "utf-8");
          content = `// BUILD_ID: ${buildID}\n` + content;
          res.setHeader("Content-Type", "application/javascript");
          res.end(content);
          return;
        }
      }
      next();
    });
  },
  closeBundle() {
    const swPath = path.resolve(__dirname, "dist", "sw.js");
    if (fs.existsSync(swPath)) {
      let content = fs.readFileSync(swPath, "utf-8");
      content = `// BUILD_ID: ${buildID}\n` + content;
      fs.writeFileSync(swPath, content);
    }
  }
});

export default defineConfig(({ command }) => ({
  plugins: [svelte(), injectSWVersionPlugin()],
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
