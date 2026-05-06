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

export type StreamPresetOption = {
  value: StreamQuality;
  label: string;
  meta: string;
  description: string;
};

export const streamPresetOptions: StreamPresetOption[] = [
  {
    value: "auto",
    label: "Auto",
    meta: "Adaptive quality",
    description:
      "Automatically adjusts the audio quality to suit your connection."
  },
  {
    value: "low",
    label: "Low",
    meta: "64 kbps",
    description:
      "Lowest bandwidth. Best for weak connections and strict data limits."
  },
  {
    value: "medium",
    label: "Medium",
    meta: "96 kbps",
    description: "Balanced quality and bandwidth. Recommended for most phones."
  },
  {
    value: "high",
    label: "High",
    meta: "160 kbps",
    description:
      "Higher quality with more data usage and decode cost than Medium."
  },
  {
    value: "original",
    label: "Original",
    meta: "Source quality, data usage varies",
    description:
      "Streams the source file as-is. Highest quality and largest transfer size."
  }
];
