import { writable } from "svelte/store";
import {
  type CrossfadeConfig,
  crossfadeDefaults,
  parseCrossfadeConfig
} from "./crossfade";
import { storageKeys } from "./storage";

const initial = parseCrossfadeConfig(
  typeof localStorage === "undefined"
    ? null
    : localStorage.getItem(storageKeys.crossfade)
);

/** Writable store for crossfade settings, persisted to localStorage. */
export const crossfadeConfig = writable<CrossfadeConfig>(initial);

crossfadeConfig.subscribe((value) => {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(storageKeys.crossfade, JSON.stringify(value));
});

export { crossfadeDefaults };
