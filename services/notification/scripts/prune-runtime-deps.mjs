// Removes packages that are only reachable through dev dependencies or
// optional peers (the lockfile flags them "devOptional"). `npm prune
// --omit=dev` keeps them because @prisma/client declares the prisma CLI and
// typescript as optional peers.
import { readFileSync, rmSync } from 'node:fs';

const lock = JSON.parse(readFileSync('package-lock.json', 'utf8'));
let removed = 0;
for (const [path, entry] of Object.entries(lock.packages)) {
  if (path.startsWith('node_modules/') && entry.devOptional === true) {
    rmSync(path, { recursive: true, force: true });
    removed += 1;
  }
}
process.stdout.write(`pruned ${removed} dev-only optional packages\n`);
