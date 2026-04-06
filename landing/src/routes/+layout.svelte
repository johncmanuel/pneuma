<script lang="ts">
  import "@pneuma/shared/style.css";
  import { seo } from "$lib/seo";

  let { children } = $props();
</script>

<svelte:head>
  <title>{seo.title}</title>
  <meta name="description" content={seo.description} />
  <meta name="keywords" content={seo.keywords.join(", ")} />
  <meta name="author" content={seo.author} />
  <link rel="canonical" href={seo.siteUrl} />

  <meta property="og:type" content="website" />
  <meta property="og:title" content={seo.title} />
  <meta property="og:description" content={seo.description} />
  <meta property="og:url" content={seo.siteUrl} />
  <meta property="og:site_name" content={seo.siteName} />
  <meta property="og:locale" content={seo.locale} />

  <meta name="twitter:card" content={seo.twitterCard} />
  <meta name="twitter:title" content={seo.title} />
  <meta name="twitter:description" content={seo.description} />

  <!-- kinda feel iffy about using @html, but should work for now -->
  {@html `<script type="application/ld+json">${seo.structuredDataJson}</script>`}
  <link rel="icon" href="/favicon.svg" />
</svelte:head>

{@render children()}

<style>
  :global(html) {
    scroll-behavior: smooth;
  }

  :global(body) {
    font-size: 16px;
    line-height: 1.6;
    -moz-osx-font-smoothing: grayscale;
    user-select: text;
    overflow: auto;
  }

  :global(::selection) {
    background: var(--accent);
    color: var(--bg);
  }

  :global(:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
    border-radius: var(--r-sm);
  }

  :global(.skip-link) {
    position: absolute;
    top: -100%;
    left: 16px;
    z-index: 9999;
    background: var(--accent);
    color: var(--bg);
    padding: 8px 16px;
    border-radius: var(--r-md);
    font-weight: 600;
    transition: top 0.15s ease;
  }

  :global(.skip-link:focus) {
    top: 16px;
  }

  @media (prefers-reduced-motion: reduce) {
    :global(html) {
      scroll-behavior: auto;
    }

    :global(*),
    :global(*::before),
    :global(*::after) {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
      scroll-behavior: auto !important;
    }
  }
</style>
