/** Base HTTP URL for the pneuma API server. */
export function apiBase(): string {
  return `http://127.0.0.1:${(window as any).__PNEUMA_PORT__ || 8989}`
}

/** Base WebSocket URL for the pneuma server. */
export function wsBase(): string {
  return `ws://127.0.0.1:${(window as any).__PNEUMA_PORT__ || 8989}`
}
