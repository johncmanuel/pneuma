<script lang="ts">
  import { login } from "../lib/api";
  import { pushNav } from "../lib/stores/ui";
  import { addToast } from "@pneuma/shared";

  let { onSwitch }: { onSwitch?: () => void } = $props();

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let loading = $state(false);

  async function handleLogin(e: SubmitEvent) {
    e.preventDefault();
    if (!username.trim() || !password) return;
    loading = true;
    error = "";

    const result = await login(username.trim(), password);
    if (result) {
      error = result;
      addToast(result, "error");
    } else {
      pushNav({ view: "library" });
    }

    loading = false;
  }
</script>

<div class="auth-card">
  <h1 class="logo">pneuma</h1>
  <p class="subtitle">Sign in to your account</p>

  <form onsubmit={handleLogin}>
    <div class="field">
      <label for="username">Username</label>
      <input
        id="username"
        type="text"
        bind:value={username}
        autocomplete="username"
      />
    </div>
    <div class="field">
      <label for="password">Password</label>
      <input
        id="password"
        type="password"
        bind:value={password}
        autocomplete="current-password"
      />
    </div>

    {#if error}
      <p class="error">{error}</p>
    {/if}

    <button type="submit" class="submit-btn" disabled={loading}>
      {loading ? "Signing in..." : "Sign in"}
    </button>
  </form>

  <p class="switch-text">
    Don't have an account?{" "}
    <button class="switch-btn" onclick={onSwitch}>Register</button>
  </p>
</div>

<style>
  .auth-card {
    width: 100%;
    max-width: 360px;
    padding: 40px 32px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }

  .logo {
    font-size: 24px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 3px;
    text-align: center;
    margin: 0 0 8px;
  }

  .subtitle {
    text-align: center;
    color: var(--text-2);
    font-size: 13px;
    margin: 0 0 28px;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-2);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .error {
    color: var(--danger);
    font-size: 13px;
    margin: 0;
  }

  .submit-btn {
    background: var(--accent-dim);
    color: #fff;
    padding: 10px;
    border-radius: var(--r-md);
    font-size: 14px;
    font-weight: 600;
    margin-top: 4px;
    transition: filter 0.15s;
  }

  .submit-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .switch-text {
    text-align: center;
    font-size: 13px;
    color: var(--text-3);
    margin: 20px 0 0;
  }

  .switch-btn {
    color: var(--accent);
    font-size: inherit;
    padding: 0;
  }

  .switch-btn:hover {
    text-decoration: underline;
  }
</style>
