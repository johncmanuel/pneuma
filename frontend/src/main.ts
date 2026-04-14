import "@pneuma/shared/style.css";
import App from "./App.svelte";
import { mount } from "svelte";
import { initThemeMode } from "@pneuma/ui";

initThemeMode();

const app = mount(App, {
  target: document.getElementById("app")!
});

export default app;
