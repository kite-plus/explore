// @ts-check
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

import node from "@astrojs/node";
import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

// The theme script runs inline before the first paint. Astro hashes only the
// scripts it bundles, so the CSP gets this one's hash from here.
const themeScript = readFileSync(new URL("./src/scripts/theme.js", import.meta.url), "utf8");
const entryCheckScript = readFileSync(new URL("./src/scripts/entry-check.js", import.meta.url), "utf8");
const entryStreamScript = readFileSync(new URL("./src/scripts/entry-stream.js", import.meta.url), "utf8");
/** @type {`sha256-${string}`} */
const themeHash = `sha256-${createHash("sha256").update(themeScript).digest("base64")}`;
/** @type {`sha256-${string}`} */
const entryCheckHash = `sha256-${createHash("sha256").update(entryCheckScript).digest("base64")}`;
/** @type {`sha256-${string}`} */
const entryStreamHash = `sha256-${createHash("sha256").update(entryStreamScript).digest("base64")}`;

// See docs/design/frontend.md: pages render on the server. Entry lists also
// ship the link-check and entry-stream scripts; Chinese is at the root and
// English under /en.
export default defineConfig({
  output: "server",
  adapter: node({ mode: "standalone" }),
  integrations: [react()],
  i18n: {
    defaultLocale: "zh",
    locales: ["zh", "en"],
    routing: { prefixDefaultLocale: false },
  },
  markdown: {
    // Shiki colors code with inline style attributes, which the CSP would
    // have to allow.
    syntaxHighlight: false,
  },
  build: {
    // One cacheable stylesheet instead of inline <style> blocks.
    inlineStylesheets: "never",
  },
  security: {
    checkOrigin: true,
    csp: {
      scriptDirective: { hashes: [themeHash, entryCheckHash, entryStreamHash] },
      directives: [
        "default-src 'self'",
        "img-src 'self' data:",
        "font-src 'self'",
        "connect-src 'self'",
        "form-action 'self'",
        "base-uri 'self'",
        "object-src 'none'",
        // Rendered pages get the policy as a header, where this works.
        "frame-ancestors 'none'",
      ],
    },
  },
  vite: {
    plugins: [tailwindcss()],
  },
});
