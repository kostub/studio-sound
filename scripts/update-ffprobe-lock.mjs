#!/usr/bin/env node
// Maintenance script: regenerates the bundled ffprobe binaries and
// third_party/ffprobe.lock.json from upstream releases. Windows comes from
// BtbN/FFmpeg-Builds; macOS from osxexperts.net (BtbN ships no macOS builds).
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
  readFileSync,
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
const BTBN = `https://github.com/BtbN/FFmpeg-Builds/releases/download/${TAG}`;
const WIN_DIR = `ffmpeg-n${VERSION}-${TAG}-win64-gpl-${VERSION}`;

// Each platform pins a self-contained (statically linked) GPL ffprobe:
// one executable, no DLLs. Windows comes from BtbN/FFmpeg-Builds; macOS
// from osxexperts.net, the only source publishing static single-file macOS
// builds (BtbN ships no macOS artifacts). `innerPath` is the entry inside
// the archive; `licenseInner`, when present, is extracted alongside the
// binary as LICENSE.ffmpeg-gpl.txt (the osxexperts zips carry no license).
const PLATFORMS = {
  'windows-amd64': {
    url: `${BTBN}/${WIN_DIR}.zip`,
    innerPath: `${WIN_DIR}/bin/ffprobe.exe`,
    bundledName: 'ffprobe-x86_64-pc-windows-msvc.exe.gz',
    licenseInner: `${WIN_DIR}/LICENSE.txt`,
  },
  'macos-arm64': {
    url: 'https://www.osxexperts.net/ffprobe71arm.zip',
    innerPath: 'ffprobe',
    bundledName: 'ffprobe-aarch64-apple-darwin.gz',
  },
  'macos-amd64': {
    url: 'https://www.osxexperts.net/ffprobe71intel.zip',
    innerPath: 'ffprobe',
    bundledName: 'ffprobe-x86_64-apple-darwin.gz',
  },
};

async function gzipFile(src, dest) {
  await pipeline(createReadStream(src), createGzip({ level: 9 }), createWriteStream(dest));
}

// Reuse an already-provisioned platform verbatim when its checked-in,
// gzip-compressed binary is still present, so regenerating one platform
// never re-downloads (and silently bumps) another from a rolling "latest"
// tag. Delete the bundled .gz to force a platform to be re-fetched.
const lockPath = join(rootDir, 'third_party/ffprobe.lock.json');
const prevLock = existsSync(lockPath)
  ? JSON.parse(readFileSync(lockPath, 'utf8'))
  : { platforms: {} };

const platforms = {};
for (const [key, p] of Object.entries(PLATFORMS)) {
  const bundledRel  = `third_party/ffprobe/${p.bundledName}`;
  const bundledPath = join(bundleDir, p.bundledName);
  const prev = prevLock.platforms?.[key];

  if (prev && existsSync(bundledPath)) {
    console.log(`update-ffprobe-lock: reusing existing ${key} (${bundledRel})`);
    platforms[key] = prev;
    continue;
  }

  const cachedArchive = join(cacheDir, basename(new URL(p.url).pathname));
  if (!existsSync(cachedArchive)) {
    console.log(`update-ffprobe-lock: downloading ${p.url}`);
    await download(p.url, cachedArchive);
  } else {
    console.log(`update-ffprobe-lock: using cached ${cachedArchive}`);
  }

  const tmpExe = join(tmpdir(), `ffprobe-${key}-${process.pid}`);
  console.log(`update-ffprobe-lock: extracting ${p.innerPath}`);
  await extractInner(cachedArchive, p.url, p.innerPath, tmpExe);

  const binarySha256 = await sha256OfFile(tmpExe);
  console.log(`update-ffprobe-lock: writing ${bundledRel}`);
  await gzipFile(tmpExe, bundledPath);
  rmSync(tmpExe, { force: true });

  // The GPL builds ship FFmpeg's license; extract it when the archive
  // carries one (BtbN does; the osxexperts single-file zips do not).
  if (p.licenseInner) {
    extractZipEntry(cachedArchive, p.licenseInner, join(bundleDir, 'LICENSE.ffmpeg-gpl.txt'));
  }

  platforms[key] = {
    bundled: bundledRel,
    binarySha256,
    url: p.url,
    sha256: await sha256OfFile(cachedArchive),
    innerPath: p.innerPath,
  };
}

const lock = {
  version: VERSION,
  source:
    `Windows from https://github.com/BtbN/FFmpeg-Builds (GPL static builds, release tag: ${TAG}); ` +
    `macOS from https://www.osxexperts.net (GPL static builds, FFmpeg ${VERSION}).`,
  note: 'Binaries are checked in (gzip-compressed) under third_party/ffprobe/. Regenerate with scripts/update-ffprobe-lock.mjs.',
  platforms,
};

writeFileSync(lockPath, JSON.stringify(lock, null, 2) + '\n');
console.log('update-ffprobe-lock: done');
