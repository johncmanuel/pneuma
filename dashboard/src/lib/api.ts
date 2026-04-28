import { initApiClient } from "@pneuma/shared";

export {
  currentUser,
  loggedIn,
  apiFetch,
  wsBase,
  login,
  register,
  logout,
  tryAutoAuth
} from "@pneuma/shared";

/**
 * API base URL.
 * In production, the dashboard UI is served from the same origin as the API,
 * so use a relative path. During `vite dev` set VITE_API_BASE if needed.
 */
export function apiBase(): string {
  return (import.meta.env?.VITE_API_BASE as string) ?? "";
}

initApiClient({
  apiBase
});
