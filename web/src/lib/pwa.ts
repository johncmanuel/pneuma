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

export async function registerPWAServiceWorker() {
  if (typeof window === "undefined") return;
  if (!("serviceWorker" in navigator)) {
    console.info("PWA: service workers are not supported in this browser");
    return;
  }

  try {
    const swURL = trustedScriptURL(`${import.meta.env.BASE_URL}sw.js`);
    const registration = await navigator.serviceWorker.register(swURL, {
      scope: import.meta.env.BASE_URL
    });
    console.info("PWA: service worker registered", {
      scope: registration.scope
    });
  } catch (error) {
    console.warn("Service worker registration failed", error);
  }
}
