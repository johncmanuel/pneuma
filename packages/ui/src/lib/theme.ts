import { writable, get } from "svelte/store";
import { storageKeys } from "@pneuma/shared";

export type ThemeMode = "light" | "dark" | "system";

const DEFAULT_THEME_MODE: ThemeMode = "system";

function isBrowser(): boolean {
  return typeof window !== "undefined" && typeof document !== "undefined";
}

function normalizeThemeMode(raw: string | null | undefined): ThemeMode {
  return raw === "light" || raw === "dark" || raw === "system"
    ? raw
    : DEFAULT_THEME_MODE;
}

function readStoredThemeMode(): ThemeMode {
  if (!isBrowser()) return DEFAULT_THEME_MODE;

  try {
    return normalizeThemeMode(localStorage.getItem(storageKeys.themeMode));
  } catch {
    return DEFAULT_THEME_MODE;
  }
}

function persistThemeMode(mode: ThemeMode) {
  if (!isBrowser()) return;

  try {
    localStorage.setItem(storageKeys.themeMode, mode);
  } catch {
    // Ignore private-mode and quota failures.
  }
}

export function applyThemeMode(mode: ThemeMode) {
  if (!isBrowser()) return;

  const root = document.documentElement;

  if (mode === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.setAttribute("data-theme", mode);
  }
}

const initialThemeMode = readStoredThemeMode();
export const themeMode = writable<ThemeMode>(initialThemeMode);

let initialized = false;

export function initThemeMode(): ThemeMode {
  if (initialized) return get(themeMode);

  const mode = readStoredThemeMode();
  applyThemeMode(mode);
  themeMode.set(mode);
  initialized = true;

  return mode;
}

export function setThemeMode(mode: ThemeMode) {
  persistThemeMode(mode);
  applyThemeMode(mode);
  themeMode.set(mode);
}

if (isBrowser()) {
  window.addEventListener("storage", (event) => {
    if (event.key !== storageKeys.themeMode) return;

    const mode = normalizeThemeMode(event.newValue);
    applyThemeMode(mode);
    themeMode.set(mode);
  });
}
