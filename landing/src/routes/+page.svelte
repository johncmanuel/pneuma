<script lang="ts">
  import { onMount } from "svelte";
  import Apple from "$lib/assets/icons/Apple.svelte";
  import Linux from "$lib/assets/icons/Linux.svelte";
  import Windows from "$lib/assets/icons/Windows.svelte";
  import {
    WifiOff,
    DiscAlbum,
    Play,
    RotateCw,
    MonitorSmartphone,
    Users,
    Github,
    ChevronLeft,
    ChevronRight
  } from "@lucide/svelte";

  const screenshotModules = import.meta.glob<string>(
    "$lib/screenshots/*.{png,jpg,jpeg,webp}",
    {
      eager: true,
      import: "default"
    }
  );
  const screenshots = Object.values(screenshotModules);

  let currentIndex = $state(0);
  let isPaused = $state(false);
  let autoTimer: ReturnType<typeof setInterval> | undefined;

  function next() {
    currentIndex = (currentIndex + 1) % screenshots.length;
  }

  function prev() {
    currentIndex = (currentIndex - 1 + screenshots.length) % screenshots.length;
  }

  function goTo(index: number) {
    currentIndex = index;
  }

  const autoAdvanceTimeoutMs = 5000;

  function startAutoAdvance() {
    autoTimer = setInterval(() => {
      if (!isPaused) next();
    }, autoAdvanceTimeoutMs);
  }

  function stopAutoAdvance() {
    if (autoTimer) {
      clearInterval(autoTimer);
      autoTimer = undefined;
    }
  }

  onMount(() => {
    if (screenshots.length > 1) {
      startAutoAdvance();
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            (entry.target as HTMLElement).classList.add("visible");
            observer.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.15 }
    );

    document.querySelectorAll(".animate-on-scroll").forEach((el) => {
      observer.observe(el);
    });

    function handleKeydown(e: KeyboardEvent) {
      if (e.key === "ArrowLeft") prev();
      if (e.key === "ArrowRight") next();
    }

    window.addEventListener("keydown", handleKeydown);

    return () => {
      observer.disconnect();
      stopAutoAdvance();
      window.removeEventListener("keydown", handleKeydown);
    };
  });

  const features = [
    {
      icon: DiscAlbum,
      title: "Self-organizing library",
      description:
        "Metadata-driven library with fingerprint-based duplicate detection. Your music organizes itself."
    },
    {
      icon: Play,
      title: "Real-time playback sync",
      description:
        "WebSocket-driven playback keeps state, queues, and progress in sync across all your devices."
    },
    {
      icon: RotateCw,
      title: "Library monitoring",
      description:
        "Background directory watchers automatically detect newly added or removed music files in real time."
    },
    {
      icon: MonitorSmartphone,
      title: "Cross-platform",
      description:
        "Native support for Windows, macOS, and Linux. One player for every machine you own."
    },
    {
      icon: WifiOff,
      title: "Offline-first",
      description:
        "Designed to work entirely offline. Local playback for music on your own machine, no server required."
    },
    {
      icon: Users,
      title: "Multi-user ready",
      description:
        "Built-in admin dashboard with isolated profiles and custom playlists for every user on a single instance."
    }
  ];
</script>

<main id="main" class="page">
  <section class="hero" aria-label="Introduction">
    <div class="hero-glow" aria-hidden="true"></div>
    <h1 class="hero-title">pneuma</h1>
    <p class="hero-tagline">Your music, your way.</p>
    <p class="hero-subtitle">
      Open-source, self-hostable, local-first music player.
      <br />
      Enjoy a Spotify-like experience with full control over your library.
    </p>
    <div class="hero-actions">
      <a
        href="https://github.com/johncmanuel/pneuma/releases"
        class="btn btn-primary"
        aria-label="Download pneuma from GitHub Releases"
      >
        Download
      </a>
      <a
        href="https://pneuma.johncarlomanuel.com/"
        class="btn btn-secondary"
        aria-label="Try the live demo of pneuma"
      >
        Try Demo
      </a>
    </div>
  </section>

  <section class="features" id="features" aria-labelledby="features-heading">
    <h2 id="features-heading" class="section-heading animate-on-scroll">
      Features
    </h2>
    <div class="features-grid">
      {#each features as feature, i (feature.title)}
        <article
          class="feature-card animate-on-scroll"
          style="--delay: {i * 80}ms"
          aria-label={feature.title}
        >
          <div class="feature-icon" aria-hidden="true">
            <feature.icon
              size={28}
              strokeWidth={1.5}
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </div>
          <h3>{feature.title}</h3>
          <p>{feature.description}</p>
        </article>
      {/each}
    </div>
  </section>

  <section
    class="screenshots"
    id="screenshots"
    aria-labelledby="screenshots-heading"
  >
    <h2 id="screenshots-heading" class="section-heading animate-on-scroll">
      Screenshots
    </h2>
    {#if screenshots.length > 0}
      <div
        class="carousel animate-on-scroll"
        role="region"
        onpointerenter={() => (isPaused = true)}
        onpointerleave={() => (isPaused = false)}
        aria-roledescription="carousel"
        aria-label="Screenshots of pneuma"
      >
        <div class="carousel-track">
          {#each screenshots as src, i (src)}
            <div
              class="carousel-slide"
              class:active={i === currentIndex}
              role="group"
              aria-roledescription="slide"
              aria-label="Slide {i + 1} of {screenshots.length}"
              aria-hidden={i !== currentIndex}
            >
              <img {src} alt="Screenshot of pneuma" />
            </div>
          {/each}
        </div>

        {#if screenshots.length > 1}
          <button
            class="carousel-btn carousel-prev"
            onclick={prev}
            aria-label="Previous screenshot"
          >
            <ChevronLeft size={24} />
          </button>
          <button
            class="carousel-btn carousel-next"
            onclick={next}
            aria-label="Next screenshot"
          >
            <ChevronRight size={24} />
          </button>

          <div
            class="carousel-dots"
            role="tablist"
            aria-label="Slide indicators"
          >
            {#each screenshots as _, i (i)}
              <button
                class="carousel-dot"
                class:active={i === currentIndex}
                onclick={() => goTo(i)}
                role="tab"
                aria-label="Go to slide {i + 1}"
                aria-selected={i === currentIndex}
              ></button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </section>

  <section class="download" id="download" aria-labelledby="download-heading">
    <div class="download-inner animate-on-scroll">
      <h2 id="download-heading" class="section-heading">Get Started</h2>
      <p class="download-subtitle">
        Available for all major desktop platforms.
      </p>
      <a
        href="https://github.com/johncmanuel/pneuma/releases"
        class="btn btn-primary btn-large"
        aria-label="Download pneuma for Windows, macOS, or Linux from GitHub Releases"
      >
        Download for free
      </a>
      <div class="platforms" role="list" aria-label="Supported platforms">
        <span class="platform" role="listitem">
          <Windows width="20" height="20" aria-hidden="true" />
          Windows
        </span>
        <span class="platform" role="listitem">
          <Apple width="20" height="20" aria-hidden="true" />
          macOS
        </span>
        <span class="platform" role="listitem">
          <Linux width="20" height="20" aria-hidden="true" />
          Linux
        </span>
      </div>
    </div>
  </section>

  <footer class="footer animate-on-scroll" aria-label="Footer">
    <div class="footer-top">
      <div class="footer-brand">
        <h3>pneuma</h3>
        <p>Open-source, self-hostable music player.</p>
      </div>

      <nav class="footer-nav" aria-label="Footer navigation">
        <h4>Navigate</h4>
        <ul>
          <li><a href="#features">Features</a></li>
          <li><a href="#download">Download</a></li>
          <li>
            <a
              href="https://pneuma.johncarlomanuel.com/"
              target="_blank"
              rel="noopener noreferrer">Live Demo</a
            >
          </li>
        </ul>
      </nav>

      <div class="footer-socials">
        <h4>Connect</h4>
        <ul>
          <li>
            <a
              href="https://github.com/johncmanuel/pneuma"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="GitHub repository"
            >
              <Github size={20} />
              GitHub
            </a>
          </li>
        </ul>
      </div>
    </div>

    <div class="footer-bottom">
      <p>&copy; {new Date().getFullYear()} John Carlo Manuel.</p>
    </div>
  </footer>
</main>

<style>
  .page {
    min-height: 100vh;
  }

  .hero {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 120px 24px 80px;
    min-height: 90vh;
    overflow: hidden;
  }

  .hero-glow {
    position: absolute;
    width: 500px;
    height: 500px;
    border-radius: 50%;
    background: radial-gradient(
      circle,
      rgba(79, 244, 191, 0.12) 0%,
      transparent 70%
    );
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    animation: pulse 4s ease-in-out infinite;
    pointer-events: none;
  }

  @keyframes pulse {
    0%,
    100% {
      transform: translate(-50%, -50%) scale(1);
      opacity: 0.6;
    }
    50% {
      transform: translate(-50%, -50%) scale(1.15);
      opacity: 1;
    }
  }

  .hero-title {
    font-size: clamp(3rem, 8vw, 6rem);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1;
    margin-bottom: 12px;
    background: linear-gradient(135deg, var(--text-1) 0%, var(--accent) 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    animation: fadeInUp 0.8s ease-out both;
  }

  .hero-tagline {
    font-size: clamp(1.25rem, 3vw, 1.75rem);
    font-weight: 500;
    color: var(--text-2);
    margin-bottom: 16px;
    animation: fadeInUp 0.8s ease-out 0.15s both;
  }

  .hero-subtitle {
    font-size: clamp(1rem, 2vw, 1.15rem);
    color: var(--text-3);
    max-width: 520px;
    line-height: 1.7;
    margin-bottom: 40px;
    animation: fadeInUp 0.8s ease-out 0.3s both;
  }

  .hero-actions {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    justify-content: center;
    animation: fadeInUp 0.8s ease-out 0.45s both;
  }

  @keyframes fadeInUp {
    from {
      opacity: 0;
      transform: translateY(24px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 12px 28px;
    border-radius: var(--r-lg);
    font-size: 1rem;
    font-weight: 600;
    transition:
      transform 0.2s ease,
      box-shadow 0.2s ease,
      background 0.2s ease;
    text-decoration: none;
  }

  .btn:hover {
    transform: translateY(-2px);
  }

  .btn:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 3px;
  }

  .btn-primary {
    background: var(--accent);
    color: var(--bg);
  }

  .btn-primary:hover {
    box-shadow: 0 4px 24px rgba(79, 244, 191, 0.3);
  }

  .btn-secondary {
    background: var(--surface-2);
    color: var(--text-1);
    border: 1px solid var(--border);
  }

  .btn-secondary:hover {
    background: var(--surface-hover);
    border-color: var(--accent);
  }

  .btn-large {
    padding: 16px 40px;
    font-size: 1.1rem;
  }

  .features {
    padding: 90px 24px;
    max-width: 1100px;
    margin: 0 auto;
  }

  .section-heading {
    font-size: clamp(1.75rem, 4vw, 2.5rem);
    font-weight: 700;
    text-align: center;
    margin-bottom: 48px;
    letter-spacing: -0.02em;
  }

  .features .section-heading.animate-on-scroll {
    opacity: 0;
    transform: translateY(24px);
    transition:
      opacity 0.5s ease,
      transform 0.5s ease;

    &:global(.visible) {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .features-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 20px;
  }

  .feature-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    padding: 28px;
    transition:
      transform 0.25s ease,
      border-color 0.25s ease,
      box-shadow 0.25s ease;
    opacity: 0;
    transform: translateY(30px);

    &:global(.visible) {
      opacity: 1;
      transform: translateY(0);
      transition:
        opacity 0.5s ease var(--delay),
        transform 0.5s ease var(--delay),
        border-color 0.25s ease,
        box-shadow 0.25s ease;
    }

    &:hover {
      transform: translateY(-4px);
      border-color: var(--accent);
      box-shadow: 0 8px 32px rgba(79, 244, 191, 0.08);
    }

    &:global(.visible):hover {
      transform: translateY(-4px);
    }
  }

  .feature-icon {
    color: var(--accent);
    margin-bottom: 16px;
    display: flex;
  }

  .feature-card h3 {
    font-size: 1.15rem;
    font-weight: 600;
    margin-bottom: 8px;
  }

  .feature-card p {
    color: var(--text-2);
    line-height: 1.6;
    font-size: 0.95rem;
  }

  .screenshots {
    padding: 90px 24px;
    max-width: 1100px;
    margin: 0 auto;
  }

  .carousel {
    position: relative;
    opacity: 0;
    transform: translateY(30px);
    transition:
      opacity 0.5s ease,
      transform 0.5s ease;

    &:global(.visible) {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .carousel-track {
    position: relative;
    overflow: hidden;
    border-radius: var(--r-lg);
    border: 1px solid var(--border);
    background: var(--surface);
  }

  .carousel-slide {
    position: absolute;
    inset: 0;
    opacity: 0;
    transition: opacity 0.4s ease;
    pointer-events: none;

    &.active {
      position: relative;
      opacity: 1;
      pointer-events: auto;
    }
  }

  .carousel-slide img {
    display: block;
    width: 100%;
    height: auto;
    object-fit: contain;
  }

  .carousel-btn {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-1);
    cursor: pointer;
    transition:
      background 0.2s ease,
      border-color 0.2s ease,
      transform 0.2s ease;
    z-index: 2;

    &:hover {
      background: var(--surface-hover);
      border-color: var(--accent);
      transform: translateY(-50%) scale(1.05);
    }

    &:focus-visible {
      outline: 2px solid var(--accent);
      outline-offset: 2px;
    }
  }

  .carousel-prev {
    left: -22px;
  }

  .carousel-next {
    right: -22px;
  }

  .carousel-dots {
    display: flex;
    justify-content: center;
    gap: 8px;
    margin-top: 16px;
  }

  .carousel-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    border: none;
    background: var(--text-3);
    opacity: 0.4;
    cursor: pointer;
    padding: 0;
    transition:
      opacity 0.2s ease,
      background 0.2s ease,
      transform 0.2s ease;

    &:hover {
      opacity: 0.7;
    }

    &.active {
      opacity: 1;
      background: var(--accent);
      transform: scale(1.2);
    }

    &:focus-visible {
      outline: 2px solid var(--accent);
      outline-offset: 2px;
    }
  }

  .download {
    padding: 80px 24px 100px;
    display: flex;
    justify-content: center;
  }

  .download-inner {
    text-align: center;
    max-width: 560px;
    opacity: 0;
    transform: translateY(30px);
    transition:
      opacity 0.6s ease,
      transform 0.6s ease;

    &:global(.visible) {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .download-subtitle {
    color: var(--text-2);
    font-size: 1.1rem;
    margin-bottom: 32px;
    line-height: 1.6;
  }

  .platforms {
    display: flex;
    justify-content: center;
    gap: 24px;
    margin-top: 32px;
    flex-wrap: wrap;
  }

  .platform {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-3);
    font-size: 0.9rem;
  }

  .footer {
    border-top: 1px solid var(--border);
    padding: 48px 24px 24px;
    max-width: 1100px;
    margin: 0 auto;
    color: var(--text-3);
    font-size: 0.875rem;
    opacity: 0;
    transform: translateY(20px);
    transition:
      opacity 0.5s ease,
      transform 0.5s ease;

    &:global(.visible) {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .footer-top {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr;
    gap: 48px;
    margin-bottom: 40px;
  }

  .footer-brand h3 {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text-1);
    margin-bottom: 8px;
  }

  .footer-brand p {
    color: var(--text-3);
    line-height: 1.5;
    max-width: 280px;
  }

  .footer-nav h4,
  .footer-socials h4 {
    font-size: 0.8rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-2);
    margin-bottom: 16px;
  }

  .footer-nav ul,
  .footer-socials ul {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .footer a {
    color: var(--text-3);
    transition: color 0.15s ease;
    text-decoration: none;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .footer a:hover {
    color: var(--accent);
  }

  .footer-bottom {
    padding-top: 24px;
    border-top: 1px solid var(--border);
    text-align: center;
  }

  @media (prefers-reduced-motion: reduce) {
    .hero-glow {
      animation: none;
    }

    .hero-title,
    .hero-tagline,
    .hero-subtitle,
    .hero-actions {
      animation: none;
      opacity: 1;
      transform: none;
    }

    .feature-card {
      opacity: 1;
      transform: none;
    }

    .download-inner {
      opacity: 1;
      transform: none;
    }
  }

  @media (max-width: 640px) {
    .hero {
      padding: 80px 20px 60px;
      min-height: 80vh;
    }

    .features-grid {
      grid-template-columns: 1fr;
    }

    .hero-actions {
      flex-direction: column;
      width: 100%;
      max-width: 280px;
    }

    .carousel-prev {
      left: 8px;
    }

    .carousel-next {
      right: 8px;
    }

    .carousel-btn {
      width: 36px;
      height: 36px;
    }

    .footer-top {
      grid-template-columns: 1fr;
      gap: 32px;
    }

    .footer-bottom {
      text-align: left;
    }
  }
</style>
