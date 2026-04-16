import { storageKeys } from "@pneuma/shared";

try {
  const mode = localStorage.getItem(storageKeys.themeMode);

  if (mode === "light" || mode === "dark") {
    document.documentElement.setAttribute("data-theme", mode);
  }
} catch {
  console.warn("Could not set theme mode");
}
