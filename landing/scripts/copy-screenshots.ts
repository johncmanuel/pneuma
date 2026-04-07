import { rm, mkdir, readdir } from "fs/promises";

const screenshotsDir = "src/lib/screenshots";
const srcDir = `${import.meta.dir}/../../.github/imgs`;
const destDir = `${import.meta.dir}/../${screenshotsDir}`;

const supportedExtensions = [".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg"];

try {
  // clean it first before copying new screenshots over
  await rm(destDir, { recursive: true, force: true });
  await mkdir(destDir, { recursive: true });

  const files = await readdir(srcDir);
  const imgFiles = files.filter((f) =>
    supportedExtensions.some((ext) => f.toLowerCase().endsWith(ext))
  );

  for (const img of imgFiles) {
    const src = Bun.file(`${srcDir}/${img}`);
    await Bun.write(`${destDir}/${img}`, src);
    console.log(`Copied ${img} to ${destDir}/`);
  }

  console.log(`Copied ${imgFiles.length} screenshot(s) to ${destDir}/`);
} catch (err) {
  console.error("Failed to copy screenshots:", err);
  process.exit(1);
}
