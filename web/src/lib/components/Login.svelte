<script lang="ts">
  import { login, register } from "../api";

  let username = "";
  let password = "";
  let error = "";
  let mode: "login" | "register" = "login";
  let loading = false;

  async function handleSubmit() {
    error = "";

    if (!username.trim() || !password) {
      error = "Username and password required";
      return;
    }

    loading = true;

    const err =
      mode === "login"
        ? await login(username.trim(), password)
        : await register(username.trim(), password);

    loading = false;
    if (err) error = err;
  }
</script>

<div class="login-page">
  <div class="card">
    <h1 class="logo">♫ Pneuma</h1>
    <p class="subtitle text-2">
      {mode === "login" ? "Sign in to your server" : "Create an account"}
    </p>

    <form on:submit|preventDefault={handleSubmit}>
      <label>
        <span class="text-2">Username</span>
        <input type="text" bind:value={username} autocomplete="username" />
      </label>
      <label>
        <span class="text-2">Password</span>
        <input
          type="password"
          bind:value={password}
          autocomplete={mode === "login" ? "current-password" : "new-password"}
        />
      </label>

      {#if error}
        <p class="error">{error}</p>
      {/if}

      <button type="submit" class="primary-btn" disabled={loading}>
        {loading ? "..." : mode === "login" ? "Sign In" : "Register"}
      </button>
    </form>

    <p class="toggle text-3">
      {#if mode === "login"}
        Don't have an account? <button
          class="link-btn"
          on:click={() => {
            mode = "register";
            error = "";
          }}>Register</button
        >
      {:else}
        Already have an account? <button
          class="link-btn"
          on:click={() => {
            mode = "login";
            error = "";
          }}>Sign In</button
        >
      {/if}
    </p>
  </div>
</div>

<style>
  .login-page {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
  }

  .card {
    width: 100%;
    max-width: 360px;
    padding: 40px 32px;
    background: var(--surface);
    border-radius: var(--r-lg);
    border: 1px solid var(--border);
  }

  .logo {
    font-size: 28px;
    font-weight: 700;
    text-align: center;
    margin-bottom: 4px;
    color: var(--accent);
  }

  .subtitle {
    text-align: center;
    font-size: 13px;
    margin-bottom: 24px;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;
  }

  .error {
    font-size: 13px;
    color: var(--danger);
  }

  .primary-btn {
    background: var(--accent);
    color: #000;
    font-weight: 600;
    border-radius: var(--r-md);
    padding: 10px;
    font-size: 14px;
    transition: opacity 0.15s;
  }
  .primary-btn:hover:not(:disabled) {
    opacity: 0.9;
  }

  .toggle {
    text-align: center;
    font-size: 13px;
    margin-top: 16px;
  }

  .link-btn {
    color: var(--accent);
    font-size: 13px;
    text-decoration: underline;
    padding: 0;
  }
</style>
