import { writable } from "svelte/store";
import { type StreamQuality, isStreamQuality } from "./stream-quality";
import { storageKeys } from "./storage";

const streamQualityDefault: StreamQuality = "auto";

function parseStreamQuality(raw: string | null): StreamQuality {
  if (!raw) return streamQualityDefault;
  if (isStreamQuality(raw)) {
    return raw as StreamQuality;
  }
  return streamQualityDefault;
}

const initialQuality = parseStreamQuality(
  typeof localStorage === "undefined"
    ? null
    : localStorage.getItem(storageKeys.streamQuality)
);

export const streamQuality = writable<StreamQuality>(initialQuality);

streamQuality.subscribe((value) => {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(storageKeys.streamQuality, value);
});
