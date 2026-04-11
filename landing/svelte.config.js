import adapter from "@sveltejs/adapter-static";
import process from "node:process";

/** @type {import('@sveltejs/kit').Config} */
const config = {
  compilerOptions: {
    runes: ({ filename }) =>
      filename.split(/[/\\]/).includes("node_modules") ? undefined : true
  },
  kit: {
    adapter: adapter({
      fallback: "404.html",
      precompress: true
    }),
    paths: {
      base: process.env.NODE_ENV === "development" ? "" : "/pneuma"
    },
    prerender: {
      handleMissingId: "warn"
    }
  }
};

export default config;
