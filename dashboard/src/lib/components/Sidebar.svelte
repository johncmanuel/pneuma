<script lang="ts">
</script>

<nav class="sidebar">
  <div class="brand">
    <span class="brand-text">pneuma</span>
  </div>

  <ul class="nav-list">
    <li>
      <button class="active" onclick={() => (window.location.href = "/player")}
        >Player</button
      >
    </li>
  </ul>

  <div class="spacer"></div>
</nav>

<style>
  .sidebar {
    width: var(--sidebar-w);
    height: 100%;
    background: var(--surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    padding: 16px 0;
    overflow-y: auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 16px 20px;
    font-weight: 700;
    font-size: 18px;
  }

  .brand-text {
    color: var(--accent);
    letter-spacing: 2px;
  }

  .nav-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .nav-list li button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    font-size: 13px;
    color: var(--text-2);
    border-radius: 0;
    transition:
      background 0.1s,
      color 0.1s;
  }
  .nav-list li button:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }
  .nav-list li button.active {
    color: var(--text-1);
    font-weight: 600;
  }

  .spacer {
    flex: 1;
  }
</style>
