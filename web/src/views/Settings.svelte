<script lang="ts">
  import { Check, CircleAlert } from "@lucide/svelte";
  import { type StreamQuality } from "../lib/stream-quality";
  import { streamQuality } from "../lib/stores/settings";

  type StreamPresetOption = {
    value: StreamQuality;
    label: string;
    meta: string;
    description: string;
  };

  const streamPresetOptions: StreamPresetOption[] = [
    {
      value: "auto",
      label: "Auto",
      meta: "Adaptive quality",
      description:
        "Adjusts Opus quality to suit your connection while avoiding source-quality spikes."
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
      description:
        "Balanced quality and bandwidth. Recommended for most phones."
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
      meta: "Source quality • variable data usage",
      description:
        "Streams the source file as-is (for example, FLAC). Highest quality and largest transfer size."
    }
  ];

  function handlePresetClick(option: StreamPresetOption) {
    streamQuality.set(option.value);
  }
</script>

<section class="settings-view">
  <article class="quality-panel" aria-labelledby="stream-quality-heading">
    <header class="panel-header">
      <h1 id="stream-quality-heading">Audio streaming quality</h1>
      <p class="panel-note text-2">
        <CircleAlert size={16} aria-hidden="true" />
        <span>
          Quality changes on the next track. If a higher-quality cached copy is
          already available, playback may continue at that quality.
        </span>
      </p>
    </header>

    <section class="quality-group" aria-labelledby="wifi-streaming-heading">
      <h2 id="wifi-streaming-heading">Wi-Fi streaming quality</h2>
      <p class="group-description text-3">
        Choose the quality used while streaming from your library.
      </p>

      <ul class="preset-list" aria-label="Available streaming quality presets">
        {#each streamPresetOptions as option}
          <li>
            <button
              type="button"
              class="preset-item"
              class:active={option.value === $streamQuality}
              aria-pressed={option.value === $streamQuality}
              onclick={() => handlePresetClick(option)}
            >
              <div class="preset-main">
                <span class="preset-label">{option.label}</span>
                <span class="preset-meta text-3">{option.meta}</span>
                <span class="preset-description text-3"
                  >{option.description}</span
                >
              </div>
              <span class="check-slot" aria-hidden="true">
                {#if option.value === $streamQuality}
                  <Check size={20} class="check-icon" />
                {/if}
              </span>
            </button>
          </li>
        {/each}
      </ul>
    </section>
  </article>
</section>

<style>
  .settings-view {
    width: 100%;
    margin: 0;
  }

  .quality-panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 0;
    border: none;
    border-radius: 0;
    background: transparent;
  }

  .panel-header {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .panel-header h1 {
    margin: 0;
    font-size: 24px;
    font-weight: 700;
    line-height: 1.2;
  }

  .panel-note {
    margin: 0;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    max-width: 72ch;
    font-size: 12px;
    line-height: 1.4;
  }

  .panel-note :global(svg) {
    margin-top: 2px;
    flex-shrink: 0;
  }

  .quality-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .quality-group h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 700;
    line-height: 1.18;
  }

  .group-description {
    margin: 0;
    font-size: 12px;
    max-width: 64ch;
  }

  .preset-list {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
    display: grid;
    gap: 0;
  }

  .preset-list li {
    border-bottom: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
  }

  .preset-list li:first-child {
    border-top: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
  }

  .preset-item {
    border: 1px solid transparent;
    border-radius: 8px;
    padding: 10px 8px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 14px;
    background: transparent;
    cursor: pointer;
    font: inherit;
    text-align: left;
    width: 100%;
    transition:
      background 0.12s,
      border-color 0.12s;
  }

  .preset-item:hover {
    background: color-mix(in srgb, var(--surface-hover) 70%, transparent);
  }

  .preset-item:focus-visible {
    outline: none;
    border-color: var(--accent);
  }

  .preset-item.active {
    border-color: color-mix(in srgb, var(--accent) 44%, var(--border));
    background: color-mix(in srgb, var(--accent) 11%, transparent);
  }

  .preset-main {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .preset-label {
    font-size: 13px;
    font-weight: 700;
    line-height: 1.2;
  }

  .preset-meta {
    font-size: 12px;
    line-height: 1.3;
  }

  .preset-description {
    margin-top: 2px;
    font-size: 12px;
    line-height: 1.35;
  }

  .check-slot {
    min-height: 22px;
    min-width: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 1px;
    flex-shrink: 0;
  }

  .check-slot :global(.check-icon) {
    color: var(--accent);
  }

  @media (max-width: 980px) {
    .panel-header h1 {
      font-size: clamp(22px, 5.4vw, 24px);
    }

    .quality-group h2 {
      font-size: clamp(17px, 4.8vw, 18px);
    }

    .preset-item {
      padding: 10px 2px;
    }

    .preset-item.active {
      border-color: transparent;
      background: color-mix(in srgb, var(--accent) 9%, transparent);
    }

    .preset-label {
      font-size: 13px;
    }

    .preset-meta {
      font-size: 12px;
    }

    .preset-description {
      font-size: 12px;
    }
  }
</style>
