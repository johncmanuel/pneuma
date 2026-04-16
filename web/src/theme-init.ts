try {
  const mode = localStorage.getItem("pneuma_theme_mode");

  if (mode === "light" || mode === "dark") {
    document.documentElement.setAttribute("data-theme", mode);
  }
} catch {
  console.warn("Could not set theme mode");
}
