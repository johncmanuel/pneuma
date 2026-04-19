import { access, copyFile as copyFileFs, mkdir } from "node:fs/promises";
import { constants } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:ur/l";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, "..");
const sourceDir = path.resolve(rootDir, "assets/icons");

// exclude appicon.png, which is only used for the desktop build 
const browserIconFiles = [
  "favicon.ico",
  "favicon.svg",
  "favicon-96x96.png",
  "apple-touch-icon.png",
  "web-app-manifest-192x192.png",
  "web-app-manifest-512x512.png"
];

const browserTargets = [
  "web/public",
  "dashboard/public",
  "landing/static"
];

const nativeTargets = [
  { source: "appicon.png", destination: "build/appicon.png" }
];

async function assertFileExists(filePath: string) {
  try {
    await access(filePath, constants.F_OK);
  } catch {
    throw new Error(`Missing icon source file: ${filePath}`);
  }
}

async function copyFile(sourcePath: string, destinationPath: string) {
  await assertFileExists(sourcePath);
  await mkdir(path.dirname(destinationPath), { recursive: true });
  await copyFileFs(sourcePath, destinationPath);
}

try {
  await Promise.all(
    browserTargets.flatMap((targetDir) =>
      browserIconFiles.map((fileName) =>
        copyFile(
          path.resolve(sourceDir, fileName),
          path.resolve(rootDir, targetDir, fileName)
        )
      )
    )
  );

  await Promise.all(
    nativeTargets.map((target) =>
      copyFile(
        path.resolve(sourceDir, target.source),
        path.resolve(rootDir, target.destination)
      )
    )
  );

  const browserCopies = browserTargets.length * browserIconFiles.length;
  const nativeCopies = nativeTargets.length;
  console.log(
    `Synced ${browserCopies + nativeCopies} icon files from assets/icons.`
  );
} catch (error) {
  console.error("Failed to sync icons:", error);
  process.exit(1);
}
