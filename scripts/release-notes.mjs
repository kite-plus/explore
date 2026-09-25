// Builds the GitHub Release body for a tag from the changelogs, the way
// mq-studio does: a release body is one document, so the Chinese section is
// the body and the English one is a link to its own section of CHANGELOG.md.
// After the changes comes where to get the build: the images and the
// archives the release workflow attaches.
//
// Usage: node scripts/release-notes.mjs v0.1.5 > notes.md
// PREVIOUS_TAG, when set, ends the notes with a compare link. NATIVE_BUILDS=false
// leaves the archives out, for releases of tags that predate them.
import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repository = "https://github.com/kite-plus/explore";

const tag = process.env.RELEASE_TAG ?? process.argv[2];
if (!tag || !/^v\d+\.\d+\.\d+(-[\w.]+)?$/.test(tag)) {
  throw new Error("usage: node scripts/release-notes.mjs v<major>.<minor>.<patch>");
}
const version = tag.slice(1);
const heading = `## [${version}]`;

// The body of a version's section, without its heading. A tag with no
// section fails here, so a release never goes out with empty notes.
function section(changelog, path) {
  const lines = changelog.split("\n");
  const start = lines.findIndex((line) => line.startsWith(heading));
  if (start === -1) throw new Error(`${path} has no section for ${version}`);
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => line.startsWith("## "));
  const body = (end === -1 ? rest : rest.slice(0, end)).join("\n").trim();
  if (!body) throw new Error(`${path} has an empty section for ${version}`);
  return body;
}

// GitHub's anchor for the version's heading: lower case, punctuation dropped,
// spaces made hyphens, so "[0.1.4] - 2026-09-25" becomes "014---2026-09-25".
function anchor(changelog, path) {
  const line = changelog.split("\n").find((l) => l.startsWith(heading));
  if (!line) throw new Error(`${path} has no section for ${version}`);
  return line.replace(/^##\s+/, "").toLowerCase().replace(/[^\w -]/g, "").trim().replace(/ /g, "-");
}

const zh = await readFile(resolve(root, "CHANGELOG.zh-CN.md"), "utf8");
const en = await readFile(resolve(root, "CHANGELOG.md"), "utf8");
// The English section is only linked to, but it has to be written too.
section(en, "CHANGELOG.md");

const archives = [
  `- **原生**：\`explore-${version}-<系统>-<架构>\` 是服务端、抓取器和数据库迁移共用的程序，有 Linux、macOS 和 Windows 的 amd64、arm64 版本；\`explore-web-${version}.tar.gz\` 是前台，用 Node.js 22 运行。运行方法见 [README](${repository}/blob/main/README.zh-CN.md#不用-docker)。`,
  "- `SHA256SUMS.txt` 是所有文件的 SHA-256 校验值。",
];
const downloads = [
  "### 下载",
  "",
  `- **Docker**：\`ghcr.io/kite-plus/explore:${version}\`（服务端、抓取器和数据库迁移）和 \`ghcr.io/kite-plus/explore-web:${version}\`（前台），支持 amd64 和 arm64。部署方法见 [README](${repository}/blob/main/README.zh-CN.md#部署)。`,
  ...(process.env.NATIVE_BUILDS === "false" ? [] : archives),
].join("\n");

const previous = process.env.PREVIOUS_TAG?.trim();
const footer = previous
  ? `**完整变更**：${repository}/compare/${previous}...${tag}`
  : `**更新日志**：${repository}/blob/main/CHANGELOG.zh-CN.md`;

const body = [
  `[English](${repository}/blob/main/CHANGELOG.md#${anchor(en, "CHANGELOG.md")})`,
  section(zh, "CHANGELOG.zh-CN.md"),
  downloads,
  "---",
  footer,
];
process.stdout.write(`${body.join("\n\n")}\n`);
