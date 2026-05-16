import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');

// Paths owned by `npm run gen:schemas`. Keep in sync with scripts/gen-schemas.mjs.
const generatedPaths = [
  'app/src/ipc/generated',
  'sidecar/internal/ipc/generated',
  'app/src-tauri/src/ipc/generated.rs',
];

const gen = spawnSync('npm', ['run', 'gen:schemas'], { cwd: rootDir, stdio: 'inherit' });
if (gen.status !== 0) {
  console.error("verify-codegen: 'npm run gen:schemas' failed");
  process.exit(gen.status ?? 1);
}

const status = spawnSync('git', ['status', '--porcelain=v1', '--', ...generatedPaths], {
  cwd: rootDir,
  encoding: 'utf8',
});
if (status.status !== 0) {
  process.stderr.write(status.stderr ?? '');
  process.exit(status.status ?? 1);
}

if (status.stdout.trim().length > 0) {
  console.error(
    '\nverify-codegen: generated files differ from the committed tree after running gen:schemas.\n' +
      'Commit the regenerated output to make CI happy:\n',
  );
  process.stdout.write(status.stdout);
  const diff = spawnSync('git', ['--no-pager', 'diff', '--', ...generatedPaths], {
    cwd: rootDir,
    stdio: 'inherit',
  });
  process.exit(diff.status ?? 1);
}

console.log('verify-codegen: generated files are up to date.');
