const siteName = "pneuma";
const siteUrl = "https://pneuma.johncarlomanuel.com/";
const repoUrl = "https://github.com/johncmanuel/pneuma";

const title = "pneuma - Your music, your way";
const description =
  "pneuma is an open-source, self-hostable, local-first music player. Enjoy a Spotify-like experience with full control over your music library.";
const keywords = [
  "music player",
  "open-source",
  "self-hosted",
  "local-first",
  "streaming",
  "desktop app",
  "cross-platform"
];

const structuredData = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "pneuma",
  description:
    "Open-source, self-hostable, local-first music player designed to give a Spotify-like experience.",
  url: siteUrl,
  applicationCategory: "MultimediaApplication",
  operatingSystem: "Windows, macOS, Linux",
  license: `${repoUrl}/blob/main/LICENSE`,
  codeRepository: repoUrl
};

export const seo = {
  siteName,
  siteUrl,
  repoUrl,
  title,
  description,
  keywords,
  author: "John Carlo Manuel",
  locale: "en_US",
  twitterCard: "summary_large_image",
  structuredData,
  structuredDataJson: JSON.stringify(structuredData)
};
