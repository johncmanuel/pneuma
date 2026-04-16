import "@pneuma/shared/style.css";
import App from "./App.svelte";
import { mount } from "svelte";
import { initThemeMode } from "@pneuma/ui";
import { registerPWAServiceWorker } from "./lib/pwa";

initThemeMode();

const app = mount(App, {
  target: document.getElementById("app")!
});

registerPWAServiceWorker();

export default app;
