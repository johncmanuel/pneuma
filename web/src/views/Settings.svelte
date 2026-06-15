<script lang="ts">
  import { Check, CircleAlert } from "@lucide/svelte";
  import {
    type StreamPresetOption,
    streamPresetOptions,
    streamQuality,
    crossfadeConfig
  } from "@pneuma/shared";

  const githubUrl = "https://github.com/johncmanuel/pneuma";

  function handlePresetClick(option: StreamPresetOption) {
    streamQuality.set(option.value);
  }

  function toggleCrossfade() {
    crossfadeConfig.update((c) => ({ ...c, enabled: !c.enabled }));
  }

  function setCrossfadeDuration(e: Event) {
    const sec = Number((e.target as HTMLInputElement).value);
    crossfadeConfig.update((c) => ({ ...c, durationSec: sec }));
  }
</script>

<section class="settings-view">
  <article class="quality-panel" aria-labelledby="stream-quality-heading">
    <header class="panel-header">
      <h1 id="stream-quality-heading">Settings</h1>
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

  <div class="group">
    <h2 class="group-heading">Crossfade</h2>
    <p class="group-description text-3">
      Smoothly blend between tracks at the end of each song.
    </p>

    <label class="toggle-row">
      <span class="toggle-label">Enable crossfade</span>
      <input
        type="checkbox"
        id="crossfade-toggle"
        checked={$crossfadeConfig.enabled}
        onchange={toggleCrossfade}
        aria-describedby="crossfade-desc"
      />
    </label>

    {#if $crossfadeConfig.enabled}
      <div class="slider-row" id="crossfade-desc">
        <label for="crossfade-duration" class="slider-label text-3">
          Duration: <strong>{$crossfadeConfig.durationSec} s</strong>
        </label>
        <input
          type="range"
          id="crossfade-duration"
          min="1"
          max="12"
          step="1"
          value={$crossfadeConfig.durationSec}
          oninput={setCrossfadeDuration}
          class="duration-slider"
          aria-label="Crossfade duration in seconds"
        />
        <div class="slider-ticks" aria-hidden="true">
          <span>1s</span><span>6s</span><span>12s</span>
        </div>
      </div>
    {/if}
  </div>

  <div class="group">
    <h3>About</h3>
    <p class="text-3">
      pneuma, an open-source, self-hostable, and local-first music player and
      server.
    </p>
    <p class="text-3">
      Source code available on <a
        target="_blank"
        rel="noopener noreferrer"
        href={githubUrl}
        aria-label="GitHub repository"
        style="text-decoration: underline;"
      >
        GitHub
      </a>.
    </p>
  </div>

  <p class="version-label">Version: {__APP_VERSION__}</p>
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

  .group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .group-heading {
    margin: 0;
    font-size: 18px;
    font-weight: 700;
    line-height: 1.18;
  }

  .toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    cursor: pointer;
    max-width: 360px;
  }

  .toggle-label {
    font-size: 13px;
    font-weight: 500;
  }

  .slider-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-width: 360px;
  }

  .slider-label {
    font-size: 12px;
  }

  .duration-slider {
    width: 100%;
    accent-color: var(--accent);
    height: 4px;
    padding: 0;
    margin: 0;
  }

  .slider-ticks {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
    color: var(--text-3);
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

  .version-label {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--text-3);
  }
</style>
