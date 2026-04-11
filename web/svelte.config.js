import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
  compilerOptions: {
    runes: ({ filename }) =>
      filename.split(/[/\\]/).includes("node_modules") ? undefined : true
  },
  preprocess: vitePreprocess()
};
