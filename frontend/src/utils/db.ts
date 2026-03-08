import { AppDBGet, AppDBSet, AppDBDelete } from "../../wailsjs/go/desktop/App"

/**
 * Async key-value store backed by the app's local SQLite database.
 *
 * All methods are fire-safe: errors are swallowed so a DB hiccup never
 * crashes the UI.  The Go backend returns "" for missing keys, which is
 * translated to `null` here so callers can distinguish "not stored" from
 * an explicitly empty string.
 *
 * When running outside Wails (e.g. `npm run dev` in a browser), the
 * window.go bridge is absent and every call falls through to the `catch`
 * blocks, returning null / doing nothing — the app still works, just
 * without persistence.
 */
export const db = {
  async get(key: string): Promise<string | null> {
    try {
      const v = await AppDBGet(key)
      return v === "" ? null : v
    } catch {
      return null
    }
  },

  async set(key: string, value: string): Promise<void> {
    try {
      await AppDBSet(key, value)
    } catch { /* non-fatal */ }
  },

  async del(key: string): Promise<void> {
    try {
      await AppDBDelete(key)
    } catch { /* non-fatal */ }
  },
}
