import { writable, derived } from "svelte/store"

export type PanelName = "queue" | "devices" | null

export const activePanel = writable<PanelName>(null)

/** The currently active main view (library | downloads | settings). */
export const currentView = writable<string>("library")

// Convenience derived stores for backward compat
export const queuePanelOpen = derived(activePanel, $p => $p === "queue")

export function togglePanel(name: "queue" | "devices") {
  activePanel.update(v => (v === name ? null : name))
}

export function toggleQueuePanel() {
  togglePanel("queue")
}

export function closePanel() {
  activePanel.set(null)
}
