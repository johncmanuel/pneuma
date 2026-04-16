import { writable } from "svelte/store";

type TrustedTypesPolicyLike = {
  createScriptURL?: (value: string) => unknown;
};

type TrustedTypesLike = {
  getPolicy?: (name: string) => TrustedTypesPolicyLike | null;
  createPolicy: (
    name: string,
    rules: { createScriptURL?: (value: string) => string }
  ) => TrustedTypesPolicyLike;
};

function trustedScriptURL(url: string): string {
  if (typeof window === "undefined") return url;

  const trustedTypesGlobal = (
    window as Window & { trustedTypes?: TrustedTypesLike }
  ).trustedTypes;

  if (!trustedTypesGlobal) return url;

  const getPolicy =
    typeof trustedTypesGlobal.getPolicy === "function"
      ? trustedTypesGlobal.getPolicy.bind(trustedTypesGlobal)
      : null;
  const createPolicy =
    typeof trustedTypesGlobal.createPolicy === "function"
      ? trustedTypesGlobal.createPolicy.bind(trustedTypesGlobal)
      : null;

  if (!getPolicy && !createPolicy) return url;

  const existing =
    getPolicy?.("pneuma-pwa") ??
    getPolicy?.("default") ??
    getPolicy?.("svelte-trusted-html");

  if (existing?.createScriptURL) {
    return existing.createScriptURL(url) as string;
  }

  try {
    if (!createPolicy) return url;

    const policy = createPolicy("pneuma-pwa", {
      createScriptURL: (value) => value
    });

    if (!policy.createScriptURL) return url;

    return policy.createScriptURL(url) as string;
  } catch {
    return url;
  }
}

const UPDATE_CHECK_INTERVAL_MS = 15 * 60 * 1000;

let hasReloadedAfterControllerChange = false;
let shouldReloadOnControllerChange = false;
let waitingServiceWorker: ServiceWorker | null = null;
let registeredServiceWorker: ServiceWorkerRegistration | null = null;
let updateCheckTimer: ReturnType<typeof setInterval> | null = null;

export const pwaUpdateAvailable = writable(false);

function setWaitingServiceWorker(worker: ServiceWorker | null) {
  waitingServiceWorker = worker;
  pwaUpdateAvailable.set(Boolean(worker));
}

function watchInstallingWorker(registration: ServiceWorkerRegistration) {
  const installingWorker = registration.installing;
  if (!installingWorker) return;

  installingWorker.addEventListener("statechange", () => {
    if (installingWorker.state !== "installed") return;
    if (!navigator.serviceWorker.controller) return;

    setWaitingServiceWorker(registration.waiting ?? installingWorker);
  });
}

function scheduleUpdateChecks(registration: ServiceWorkerRegistration) {
  if (updateCheckTimer) {
    clearInterval(updateCheckTimer);
  }

  updateCheckTimer = setInterval(() => {
    if (!navigator.onLine) return;

    registration.update().catch(() => {
      console.info("PWA: periodic update check skipped");
    });
  }, UPDATE_CHECK_INTERVAL_MS);
}

export function applyPWAUpdate() {
  if (!waitingServiceWorker) return;

  shouldReloadOnControllerChange = true;
  waitingServiceWorker.postMessage({ type: "SKIP_WAITING" });
}

export async function checkForPWAUpdate() {
  if (!registeredServiceWorker) return;

  await registeredServiceWorker.update();

  if (registeredServiceWorker.waiting) {
    setWaitingServiceWorker(registeredServiceWorker.waiting);
  }
}

export async function registerPWAServiceWorker() {
  if (typeof window === "undefined") return;
  if (!("serviceWorker" in navigator)) {
    console.info("PWA: service workers are not supported in this browser");
    return;
  }

  try {
    const buildID =
      typeof __PWA_BUILD_ID__ === "string" && __PWA_BUILD_ID__.trim().length > 0
        ? __PWA_BUILD_ID__
        : "dev";
    const swURL = trustedScriptURL(
      `${import.meta.env.BASE_URL}sw.js?v=${encodeURIComponent(buildID)}`
    );
    const registration = await navigator.serviceWorker.register(swURL, {
      scope: import.meta.env.BASE_URL
    });

    registeredServiceWorker = registration;

    if (registration.waiting) {
      setWaitingServiceWorker(registration.waiting);
    }

    watchInstallingWorker(registration);

    registration.addEventListener("updatefound", () => {
      watchInstallingWorker(registration);
    });

    scheduleUpdateChecks(registration);

    navigator.serviceWorker.addEventListener("controllerchange", () => {
      if (!shouldReloadOnControllerChange) return;
      if (hasReloadedAfterControllerChange) return;

      hasReloadedAfterControllerChange = true;
      shouldReloadOnControllerChange = false;
      setWaitingServiceWorker(null);
      window.location.reload();
    });

    console.info("PWA: service worker registered", {
      scope: registration.scope
    });
  } catch (error) {
    console.warn("Service worker registration failed", error);
  }
}

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    if (updateCheckTimer) {
      clearInterval(updateCheckTimer);
      updateCheckTimer = null;
    }
  });
}
