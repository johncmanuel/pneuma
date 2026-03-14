import { writable, derived, get } from "svelte/store";

export type PanelName = "queue" | null;

export const activePanel = writable<PanelName>(null);

/** The currently active main view (library | downloads | settings). */
export const currentView = writable<string>("library");

export function togglePanel(name: "queue") {
  activePanel.update((v) => (v === name ? null : name));
}

export function toggleQueuePanel() {
  togglePanel("queue");
}

export function closePanel() {
  activePanel.set(null);
}

export type LibTab = "library" | "local";
export type LocalSubTab = "albums";

export const activeTab = writable<LibTab>("library");
export const localSubTab = writable<LocalSubTab>("albums");
export const selectedAlbum = writable<string | null>(null);

/** Currently selected playlist ID (drives the playlist detail view). */
export const selectedPlaylistView = writable<string | null>(null);

export interface NavState {
  view: string;
  tab: LibTab;
  subTab: LocalSubTab;
  albumKey: string | null;
  playlistId: string | null;
}

function currentNavState(): NavState {
  return {
    view: get(currentView),
    tab: get(activeTab),
    subTab: get(localSubTab),
    albumKey: get(selectedAlbum),
    playlistId: get(selectedPlaylistView)
  };
}

function applyNavState(s: NavState) {
  currentView.set(s.view);
  activeTab.set(s.tab);
  localSubTab.set(s.subTab);
  selectedAlbum.set(s.albumKey);
  selectedPlaylistView.set(s.playlistId ?? null);
}

const _navStack = writable<NavState[]>([
  {
    view: "library",
    tab: "library",
    subTab: "albums",
    albumKey: null,
    playlistId: null
  }
]);
const _navIndex = writable<number>(0);

export const canGoBack = derived(_navIndex, (i) => i > 0);
export const canGoForward = derived(
  [_navIndex, _navStack],
  ([$i, $s]) => $i < $s.length - 1
);

/**
 * Record a navigation action. Truncates any forward history, appends new
 * state, and advances the index. Utilized whenever the user navigates to
 * a new "page" (tab switch, album click, sidebar item, etc.).
 */
export function pushNav(partial?: Partial<NavState>) {
  const cur = currentNavState();
  const next: NavState = { ...cur, ...partial };

  applyNavState(next);

  _navStack.update((stack) => {
    const idx = get(_navIndex);

    // Truncate forward entries and avoid duplicate consecutive entries
    const trimmed = stack.slice(0, idx + 1);
    const prev = trimmed[trimmed.length - 1];

    if (
      prev &&
      prev.view === next.view &&
      prev.tab === next.tab &&
      prev.subTab === next.subTab &&
      prev.albumKey === next.albumKey &&
      prev.playlistId === next.playlistId
    ) {
      return trimmed;
    }

    trimmed.push(next);
    _navIndex.set(trimmed.length - 1);
    return trimmed;
  });
}

export function goBack() {
  const idx = get(_navIndex);

  if (idx <= 0) return;

  const newIdx = idx - 1;
  _navIndex.set(newIdx);

  const stack = get(_navStack);
  applyNavState(stack[newIdx]);
}

export function goForward() {
  const idx = get(_navIndex);
  const stack = get(_navStack);

  if (idx >= stack.length - 1) return;

  const newIdx = idx + 1;

  _navIndex.set(newIdx);
  applyNavState(stack[newIdx]);
}
