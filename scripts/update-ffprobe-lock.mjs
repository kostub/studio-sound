#!/usr/bin/env node
// Maintenance script: regenerates the bundled ffprobe binaries and
// third_party/ffprobe.lock.json from upstream BtbN/FFmpeg-Builds releases.
//
// For each configured platform it downloads the upstream archive, extracts
// ffprobe, stores it gzip-compressed under third_party/ffprobe/ (so the
// file stays under platform/host size limits), and records the binary's
// SHA-256 so fetch-ffprobe.mjs can verify it offline. All checksums are
// computed here in code -- never copied by hand.
//
// We pin the GPL builds because they are statically linked into a single
// self-contained ffprobe executable. BtbN's LGPL builds are shared (they
// ship ~90 MB of av*/sw* DLLs to satisfy the LGPL relinking requirement),
// which a single-file bundle cannot represent. ffprobe is invoked as a
// separate process, so the GPL build is used at arm's length.
//
// Requires network access. Usage: node scripts/update-ffprobe-lock.mjs

import {
  createReadStream,
  createWriteStream,
  existsSync,
  mkdirSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { dirname, join, resolve, basename } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir, homedir } from 'node:os';
import { createGzip } from 'node:zlib';
import { pipeline } from 'node:stream/promises';

import {
  sha256OfFile,
  download,
  extractInner,
  extractZipEntry,
} from './lib/ffprobe-archive.mjs';

const rootDir   = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const bundleDir = join(rootDir, 'third_party/ffprobe');
const cacheDir  = join(homedir(), '.cache/studio-sound/ffprobe');
mkdirSync(bundleDir, { recursive: true });
mkdirSync(cacheDir, { recursive: true });

const VERSION = '7.1';
const TAG = 'latest';
const BASE = `https://github.com/BtbN/FFmpeg-Builds/releases/download/${TAG}`;

// Self-contained (statically linked) GPL builds: one ffprobe binary, no DLLs.
const PLATFORMS = {
  'windows-amd64': {
    archive: `ffmpeg-n${VERSION}-${TAG}-win64-gpl-${VERSION}.zip`,
    dir: `ffmpeg-n${VERSION}-${TAG}-win64-gpl-${VERSION}`,
    innerExe: 'bin/ffprobe.exe',
    bundledName: 'ffprobe-x86_64-pc-windows-msvc.exe.gz',
  },
};

async function gzipFile(src, dest) {
  await pipeline(createReadStream(src), createGzip({ level: 9 }), createWriteStream(dest));
}

const platforms = {};
for (const [key, p] of Object.entries(PLATFORMS)) {
  const url = `${BASE}/${p.archive}`;
  const cachedArchive = join(cacheDir, basename(p.archive));
  if (!existsSync(cachedArchive)) {
    console.log(`update-ffprobe-lock: downloading ${url}`);
    await download(url, cachedArchive);
  } else {
    console.log(`update-ffprobe-lock: using cached ${cachedArchive}`);
  }

  const innerPath = `${p.dir}/${p.innerExe}`;
  const tmpExe = join(tmpdir(), `ffprobe-${key}-${process.pid}`);
  console.log(`update-ffprobe-lock: extracting ${innerPath}`);
  await extractInner(cachedArchive, url, innerPath, tmpExe);

  const binarySha256 = await sha256OfFile(tmpExe);
  const bundledRel = `third_party/ffprobe/${p.bundledName}`;
  console.log(`update-ffprobe-lock: writing ${bundledRel}`);
  await gzipFile(tmpExe, join(bundleDir, p.bundledName));
  rmSync(tmpExe, { force: true });

  // Extract the upstream license text alongside the binary.
  const licenseInner = `${p.dir}/LICENSE.txt`;
  extractZipEntry(cachedArchive, licenseInner, join(bundleDir, 'LICENSE.ffmpeg-gpl.txt'));

  platforms[key] = {
    bundled: bundledRel,
    binarySha256,
    url,
    sha256: await sha256OfFile(cachedArchive),
    innerPath,
  };
}

const lock = {
  version: VERSION,
  source: `https://github.com/BtbN/FFmpeg-Builds (GPL static builds, release tag: ${TAG})`,
  note: 'Binaries are checked in (gzip-compressed) under third_party/ffprobe/. Regenerate with scripts/update-ffprobe-lock.mjs.',
  platforms,
};

const lockPath = join(rootDir, 'third_party/ffprobe.lock.json');
writeFileSync(lockPath, JSON.stringify(lock, null, 2) + '\n');
console.log('update-ffprobe-lock: done');
