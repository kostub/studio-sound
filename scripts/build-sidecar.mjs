import { existsSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sidecarDir = resolve(rootDir, 'sidecar');
const binariesDir = resolve(rootDir, 'app', 'src-tauri', 'binaries');

const targets = [
  {
    goos: 'windows',
    goarch: 'amd64',
    artifact: 'studio-sidecar-x86_64-pc-windows-msvc.exe',
  },
  {
    goos: 'darwin',
    goarch: 'amd64',
    artifact: 'studio-sidecar-x86_64-apple-darwin',
  },
  {
    goos: 'darwin',
    goarch: 'arm64',
    artifact: 'studio-sidecar-aarch64-apple-darwin',
  },
];

mkdirSync(binariesDir, { recursive: true });

for (const target of targets) {
  const outputPath = resolve(binariesDir, target.artifact);
  const isWin = process.platform === 'win32';
  const result = spawnSync('go', ['build', '-trimpath', '-o', outputPath, './cmd/sidecar'], {
    cwd: sidecarDir,
    shell: isWin,
    env: {
  ...process.env,
      CGO_ENABLED: '0',
      GOOS: target.goos,
      GOARCH: target.goarch,
    },
    stdio: 'inherit',
  });

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

const missingArtifacts = targets
  .map((target) => resolve(binariesDir, target.artifact))
  .filter((artifactPath) => !existsSync(artifactPath));

if (missingArtifacts.length > 0) {
  console.error('Missing sidecar artifacts:');
  for (const artifactPath of missingArtifacts) {
    console.error(`- ${artifactPath}`);
  }
  process.exit(1);
}

console.log('Built sidecar artifacts:');
for (const target of targets) {
  console.log(`- ${resolve(binariesDir, target.artifact)}`);
}
