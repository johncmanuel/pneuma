export interface LrcLine {
  timeMs: number;
  text: string;
}

// Parse LRC format: [mm:ss.xx] text
export function parseLrc(raw: string): LrcLine[] {
  const parsed: LrcLine[] = [];

  // Example line in .lrc file that the regex would find:
  // [02:35.47] Never gonna give you up
  //  │  │  │   └──── group 4: "Never gonna give you up" (lyrics)
  //  │  │  └──────── group 3: "47"  (centiseconds or milliseconds if 3 digits)
  //  │  └─────────── group 2: "35"  (seconds)
  //  └────────────── group 1: "02"  (minutes)
  const lineRegex = /^\[(\d{1,3}):(\d{2})(?:[.:])(\d{2,3})\]\s*(.*)$/;

  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;

    const match = trimmed.match(lineRegex);
    if (!match) continue;

    const minutes = parseInt(match[1], 10);
    const seconds = parseInt(match[2], 10);
    // convert to ms from cs if there're 3 digits else it's cs
    const cs =
      match[3].length === 3
        ? parseInt(match[3], 10)
        : parseInt(match[3], 10) * 10;

    const timeMs = minutes * 60000 + seconds * 1000 + cs;
    parsed.push({ timeMs, text: match[4] });
  }

  // Sort by time to be safe
  parsed.sort((a, b) => a.timeMs - b.timeMs);

  return parsed;
}
