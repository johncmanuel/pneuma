import { execSync } from "child_process";

/**
 * Resolves the app version for build-time injection.
 * - In CI, this expects VITE_APP_VERSION to be set to github.ref_name.
 * - In local dev, reference git for the version.
 * Note: use JSON.stringify since defining this in Vite and logging the output is replaced literally rather as a string literal.
 * e.g.
 * define: {
 *   __APP_VERSION__: appVersion
 * }
 *
 * will result in:
 *
 * __APP_VERSION__ = v1.0.0
 * rather than:
 * __APP_VERSION__ = "v1.0.0"
 */
export function resolveAppVersion(): string {
  let version = "";
  if (process.env.VITE_APP_VERSION) {
    version = process.env.VITE_APP_VERSION;
  } else {
    try {
      version = execSync("git describe --tags --abbrev=0", {
        encoding: "utf-8"
      }).trim();
    } catch {
      version = "unknown";
    }
  }

  // currently, git tags are in the format "v1.0.0"
  // just for nicer readability on the UI, remove the leading "v" so
  // it shows as "1.0.0"
  if (version.startsWith("v")) {
    version = version.slice(1);
  }
  return version;
}
