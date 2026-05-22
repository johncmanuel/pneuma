export type CrossfadeConfig = {
  enabled: boolean;
  /** Duration of the crossfade in seconds. Valid range: 1–12. */
  durationSec: number;
};

export const crossfadeDefaults: CrossfadeConfig = {
  enabled: false,
  durationSec: 5
};

/** Clamp and validate a raw crossfade config from storage. */
export function parseCrossfadeConfig(raw: string | null): CrossfadeConfig {
  if (!raw) return crossfadeDefaults;
  try {
    const parsed = JSON.parse(raw) as Partial<CrossfadeConfig>;
    const enabled =
      typeof parsed.enabled === "boolean"
        ? parsed.enabled
        : crossfadeDefaults.enabled;
    const rawDur =
      typeof parsed.durationSec === "number"
        ? parsed.durationSec
        : crossfadeDefaults.durationSec;
    const durationSec = Math.max(1, Math.min(12, rawDur));
    return { enabled, durationSec };
  } catch {
    console.warn("crossfade: failed to parse config from storage");
    return crossfadeDefaults;
  }
}
