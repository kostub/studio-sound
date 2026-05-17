import { existsSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const schemasDir = resolve(rootDir, 'schemas');
const tsOutDir = resolve(rootDir, 'app/src/ipc/generated');
const goOutDir = resolve(rootDir, 'sidecar/internal/ipc/generated');
const rustOutFile = resolve(rootDir, 'app/src-tauri/src/ipc/generated.rs');

const schemaFiles = [
  'envelope.schema.json',
  'system.ping.schema.json',
  'system.echo.schema.json',
  'system.shutdown.schema.json',
  'media.probe.schema.json',
];

function run(cmd, args, opts = {}) {
  const isWin = process.platform === 'win32';
  const r = spawnSync(cmd, args, { stdio: 'inherit', cwd: rootDir, shell: isWin, ...opts });
  if (r.status !== 0) {
    console.error(`gen-schemas: '${cmd} ${args.join(' ')}' failed (${r.status})`);
    process.exit(r.status ?? 1);
  }
}

mkdirSync(tsOutDir, { recursive: true });
mkdirSync(goOutDir, { recursive: true });
mkdirSync(dirname(rustOutFile), { recursive: true });

for (const file of schemaFiles) {
  const base = file.replace(/\.schema\.json$/, '');
  const out = resolve(tsOutDir, `${base}.ts`);
  // --unreachableDefinitions emits TS types for $defs that are not referenced
  // from the schema root. Our method schemas (system.echo etc.) keep
  // request/result types in $defs, so without this flag the generated file
  // would only contain an empty root interface.
  run('npx', [
    '--no-install',
    'json-schema-to-typescript',
    '--unreachableDefinitions',
    resolve(schemasDir, file),
    '-o',
    out,
  ]);
}

const goJsonschema = process.env.GO_JSONSCHEMA_BIN ?? 'go-jsonschema';
for (const file of schemaFiles) {
  const base = file.replace(/\.schema\.json$/, '').replace(/\./g, '_');
  const out = resolve(goOutDir, `${base}.go`);
  if (existsSync(out)) rmSync(out);
  // Use relative path for input to avoid absolute path issues with $ref on Windows
  run(goJsonschema, ['--package', 'generated', '--output', out, `schemas/${file}`]);
}

const typify = process.env.TYPIFY_BIN ?? 'cargo-typify';
run(typify, ['typify', resolve(schemasDir, 'envelope.schema.json'), '--output', rustOutFile]);

// Normalize generated TS through prettier so the committed output matches what
// `npm run format` would produce. Without this, the project's singleQuote/etc.
// rules drift the checked-in files away from raw generator output and CI's
// codegen-clean assertion fails.
run('npx', ['--no-install', 'prettier', '--write', '--log-level', 'warn', `${tsOutDir}/**/*.ts`]);

console.log('gen-schemas: wrote', tsOutDir, goOutDir, rustOutFile);
