import { readFileSync, readdirSync } from "node:fs";
import { extname, join, relative, resolve } from "node:path";

const webRoot = resolve(import.meta.dirname, "..");
const sourceRoot = join(webRoot, "src");
const sourceExtensions = new Set([".css", ".ts", ".tsx"]);
const violations = [];

function checkFile(file) {
  const contents = readFileSync(file, "utf8");
  const displayPath = relative(webRoot, file);

  for (const [index, line] of contents.split("\n").entries()) {
    if (line.includes("font-display")) {
      violations.push(`${displayPath}:${index + 1}: use the base sans font instead of font-display`);
    }

    if (line.includes("font-mono") && line.includes("uppercase")) {
      violations.push(`${displayPath}:${index + 1}: reserve font-mono for technical data, not uppercase labels`);
    }

    if (line.includes("font-mono") && /font-(?:semibold|bold)/.test(line)) {
      violations.push(`${displayPath}:${index + 1}: IBM Plex Mono only loads weights 400 and 500`);
    }
  }
}

function walk(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      walk(path);
    } else if (sourceExtensions.has(extname(entry.name))) {
      checkFile(path);
    }
  }
}

walk(sourceRoot);
checkFile(join(webRoot, "tailwind.config.cjs"));

const indexHtml = readFileSync(join(webRoot, "index.html"), "utf8");
if (indexHtml.includes("Instrument Sans")) {
  violations.push("index.html: Instrument Sans is not part of the Warden typography system");
}
if (!indexHtml.includes("family=DM+Sans:wght@400;500;600;700")) {
  violations.push("index.html: DM Sans must load weights 400, 500, 600 and 700");
}
if (!indexHtml.includes("family=IBM+Plex+Mono:wght@400;500")) {
  violations.push("index.html: IBM Plex Mono must load weights 400 and 500");
}

if (violations.length > 0) {
  console.error("Typography contract violations:\n");
  console.error(violations.map((violation) => `- ${violation}`).join("\n"));
  process.exit(1);
}

console.log("Typography contract passed.");
