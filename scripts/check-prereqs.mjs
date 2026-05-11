import { spawnSync } from 'node:child_process';

const checks = [
  { name: 'Node.js', command: 'node', args: ['--version'] },
  { name: 'npm', command: 'npm', args: ['--version'] },
  { name: 'Go', command: 'go', args: ['version'] },
  { name: 'Rust Cargo', command: 'cargo', args: ['--version'] },
];

let failed = false;

for (const check of checks) {
  const result = spawnSync(check.command, check.args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  if (result.status !== 0) {
    failed = true;
    console.error(`Missing prerequisite: ${check.name} (${check.command})`);
    const stderr = result.stderr?.trim() ?? '';
    if (stderr.length > 0) {
      console.error(stderr);
    }
    continue;
  }

  console.log(`${check.name}: ${result.stdout.trim()}`);
}

if (failed) {
  console.error('Install the missing prerequisites and rerun npm run setup.');
  process.exit(1);
}
