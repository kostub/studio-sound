#!/usr/bin/env node
// Downloads, verifies, and extracts the bundled ffprobe per third_party/ffprobe.lock.json.
// Outputs to app/src-tauri/binaries/ffprobe-<target-triple>{,.exe}.

import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync, statSync, chmodSync, createReadStream } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { homedir } from 'node:os';
import { Readable } from 'node:stream';

const rootDir  = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const lockPath = join(rootDir, 'third_party/ffprobe.lock.json');
const lock     = JSON.parse(readFileSync(lockPath, 'utf8'));
const outDir   = join(rootDir, 'app/src-tauri/binaries');
const cacheDir = join(homedir(), '.cache/studio-sound/ffprobe');
mkdirSync(outDir, { recursive: true });
mkdirSync(cacheDir, { recursive: true });

const triples = {
  'windows-amd64': { triple: 'x86_64-pc-windows-msvc',  exe: 'ffprobe-x86_64-pc-windows-msvc.exe' },
  'macos-amd64':   { triple: 'x86_64-apple-darwin',     exe: 'ffprobe-x86_64-apple-darwin'        },
  'macos-arm64':   { triple: 'aarch64-apple-darwin',    exe: 'ffprobe-aarch64-apple-darwin'       },
};

async function sha256OfFile(path) {
  const hash = createHash('sha256');
  await new Promise((res, rej) => {
    createReadStream(path).on('data', (d) => hash.update(d)).on('end', res).on('error', rej);
  });
  return hash.digest('hex');
}

async function download(url, dest) {
  const r = await fetch(url, { redirect: 'follow' });
  if (!r.ok) throw new Error(`HTTP ${r.status} for ${url}`);
  const ws = (await import('node:fs')).createWriteStream(dest);
  await new Promise((res, rej) => Readable.fromWeb(r.body).pipe(ws).on('finish', res).on('error', rej));
}

function extractInner(archivePath, sourceUrl, innerPath, destExe) {
  const MAX_BUF = 512 * 1024 * 1024; // 512 MiB — ffprobe.exe is ~108 MiB
  if (sourceUrl.endsWith('.zip')) {
    const r = spawnSync('unzip', ['-p', archivePath, innerPath], { stdio: ['ignore', 'pipe', 'inherit'], maxBuffer: MAX_BUF });
    if (r.status !== 0) throw new Error(`unzip failed for ${archivePath}`);
    writeFileSync(destExe, r.stdout);
  } else if (sourceUrl.endsWith('.tar.xz') || sourceUrl.endsWith('.txz')) {
    const r = spawnSync('tar', ['-xJf', archivePath, '-O', innerPath], { stdio: ['ignore', 'pipe', 'inherit'], maxBuffer: MAX_BUF });
    if (r.status !== 0) throw new Error(`tar -xJ failed for ${archivePath}`);
    writeFileSync(destExe, r.stdout);
  } else {
    throw new Error(`unsupported archive format: ${sourceUrl}`);
  }
  if (process.platform !== 'win32') chmodSync(destExe, 0o755);
}

for (const [platformKey, spec] of Object.entries(lock.platforms)) {
  const t = triples[platformKey];
  if (!t) throw new Error(`unknown platform in lock: ${platformKey}`);
  const destExe = join(outDir, t.exe);

  const cachedArchive = join(cacheDir, `${spec.sha256.slice(0, 12)}-${platformKey}`);
  if (existsSync(cachedArchive)) {
    const h = await sha256OfFile(cachedArchive);
    if (h !== spec.sha256) throw new Error(`cached archive checksum mismatch for ${platformKey}: ${h} != ${spec.sha256}`);
  } else {
    console.log(`fetch-ffprobe: downloading ${platformKey} from ${spec.url}`);
    await download(spec.url, cachedArchive);
    const h = await sha256OfFile(cachedArchive);
    if (h !== spec.sha256) {
      throw new Error(`downloaded archive checksum mismatch for ${platformKey}: ${h} != ${spec.sha256}`);
    }
  }

  if (!existsSync(destExe) || statSync(destExe).size === 0) {
    console.log(`fetch-ffprobe: extracting ${t.exe} from ${cachedArchive}`);
    extractInner(cachedArchive, spec.url, spec.innerPath, destExe);
  } else {
    console.log(`fetch-ffprobe: ${t.exe} already present, skipping extract`);
  }
}

console.log('fetch-ffprobe: done');
