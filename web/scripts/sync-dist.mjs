import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";

const repoRoot = resolve(process.cwd(), "..", "internal", "static", "dist");
const distDir = resolve(process.cwd(), "dist");
const gitkeep = join(repoRoot, ".gitkeep");

if (!existsSync(distDir)) {
  console.error("Frontend build directory not found. Run 'npm run build' first.");
  process.exit(1);
}

if (!existsSync(repoRoot)) {
  mkdirSync(repoRoot, { recursive: true });
}

for (const entry of readdirSync(repoRoot)) {
  if (entry === ".gitkeep") {
    continue;
  }
  rmSync(join(repoRoot, entry), { recursive: true, force: true });
}

cpSync(distDir, repoRoot, { recursive: true });

if (!existsSync(gitkeep)) {
  writeFileSync(gitkeep, "");
}

console.log(`Copied frontend assets to ${repoRoot}`);
