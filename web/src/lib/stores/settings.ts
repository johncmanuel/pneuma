import { writable } from "svelte/store";
import { type StreamQuality, isStreamQuality } from "../stream-quality";

const streamQualityStorageKey = "pneuma_stream_quality";

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
    : localStorage.getItem(streamQualityStorageKey)
);

export const streamQuality = writable<StreamQuality>(initialQuality);

streamQuality.subscribe((value) => {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(streamQualityStorageKey, value);
});
