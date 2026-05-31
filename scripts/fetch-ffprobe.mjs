#!/usr/bin/env node
// Provisions the bundled ffprobe per third_party/ffprobe.lock.json.
// Outputs to app/src-tauri/binaries/ffprobe-<target-triple>{,.exe}.
//
// Platforms whose lock entry has a "bundled" path are served from a
// gzip-compressed binary checked into the repo (no network). Any other
// platform falls back to downloading and verifying the upstream archive.

import { existsSync, mkdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { homedir } from 'node:os';

import {
  triples,
  sha256OfFile,
  download,
  gunzipToFile,
  extractInner,
} from './lib/ffprobe-archive.mjs';

const rootDir  = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const lockPath = join(rootDir, 'third_party/ffprobe.lock.json');
const lock     = JSON.parse(readFileSync(lockPath, 'utf8'));
const outDir   = join(rootDir, 'app/src-tauri/binaries');
const cacheDir = join(homedir(), '.cache/studio-sound/ffprobe');
mkdirSync(outDir, { recursive: true });
mkdirSync(cacheDir, { recursive: true });

for (const [platformKey, spec] of Object.entries(lock.platforms)) {
  const t = triples[platformKey];
  if (!t) throw new Error(`unknown platform in lock: ${platformKey}`);
  const destExe = join(outDir, t.exe);

  // Preferred path: a binary checked into the repo, stored gzip-compressed.
  if (spec.bundled) {
    const bundled = join(rootDir, spec.bundled);
    if (!existsSync(bundled)) {
      throw new Error(`bundled ffprobe missing for ${platformKey}: ${spec.bundled}`);
    }
    const fresh =
      existsSync(destExe) &&
      statSync(destExe).size > 0 &&
      (await sha256OfFile(destExe)) === spec.binarySha256;
    if (!fresh) {
      console.log(`fetch-ffprobe: decompressing bundled ${t.exe}`);
      await gunzipToFile(bundled, destExe);
      const h = await sha256OfFile(destExe);
      if (h !== spec.binarySha256) {
        throw new Error(`bundled ffprobe checksum mismatch for ${platformKey}: ${h} != ${spec.binarySha256}`);
      }
    } else {
      console.log(`fetch-ffprobe: ${t.exe} already present, skipping decompress`);
    }
    continue;
  }

  // Fallback path: download and verify the upstream archive, then extract.
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
    await extractInner(cachedArchive, spec.url, spec.innerPath, destExe);
  } else {
    console.log(`fetch-ffprobe: ${t.exe} already present, skipping extract`);
  }
}

console.log('fetch-ffprobe: done');
