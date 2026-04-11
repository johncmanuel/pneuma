// @ts-check

import eslint from "@eslint/js";
import { defineConfig, globalIgnores } from "eslint/config";
import tseslint from "typescript-eslint";
import { fileURLToPath } from "node:url";
import { includeIgnoreFile } from "@eslint/compat";

// https://eslint.org/docs/latest/use/configure/ignore#include-gitignore-files
const gitignorePath = fileURLToPath(new URL(".gitignore", import.meta.url));

export default defineConfig(
  eslint.configs.recommended,
  tseslint.configs.recommended,
  includeIgnoreFile(gitignorePath, "Imported .gitignore patterns"),
  globalIgnores(["frontend/wailsjs/", "eslint.config.mjs"])
);
