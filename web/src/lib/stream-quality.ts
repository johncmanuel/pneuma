export const streamQualityValues = [
  "auto",
  "low",
  "medium",
  "high",
  "original"
] as const;

export type StreamQuality = (typeof streamQualityValues)[number];

export function isStreamQuality(value: string): value is StreamQuality {
  return (streamQualityValues as readonly string[]).includes(value);
}
