// Shared helpers for provisioning the bundled ffprobe binary.
// Used by both fetch-ffprobe.mjs (build/CI) and update-ffprobe-lock.mjs (maintenance).

import { createHash } from 'node:crypto';
import {
  closeSync,
  createReadStream,
  createWriteStream,
  openSync,
  readSync,
  statSync,
  writeFileSync,
  chmodSync,
} from 'node:fs';
import { spawn } from 'node:child_process';
import { createGunzip } from 'node:zlib';
import { inflateRawSync } from 'node:zlib';
import { pipeline } from 'node:stream/promises';

// Maps lock-file platform keys to Tauri target triples and output file names.
export const triples = {
  'windows-amd64': { triple: 'x86_64-pc-windows-msvc',   exe: 'ffprobe-x86_64-pc-windows-msvc.exe' },
  'macos-amd64':   { triple: 'x86_64-apple-darwin',      exe: 'ffprobe-x86_64-apple-darwin'        },
  'macos-arm64':   { triple: 'aarch64-apple-darwin',     exe: 'ffprobe-aarch64-apple-darwin'       },
  'linux-amd64':   { triple: 'x86_64-unknown-linux-gnu', exe: 'ffprobe-x86_64-unknown-linux-gnu'   },
};

export async function sha256OfFile(path) {
  const hash = createHash('sha256');
  await new Promise((res, rej) => {
    createReadStream(path).on('data', (d) => hash.update(d)).on('end', res).on('error', rej);
  });
  return hash.digest('hex');
}

export async function download(url, dest) {
  const r = await fetch(url, { redirect: 'follow' });
  if (!r.ok) throw new Error(`HTTP ${r.status} for ${url}`);
  if (!r.body) throw new Error(`no response body for ${url}`);
  await pipeline(r.body, createWriteStream(dest));
}

// Decompresses a gzip file to destExe and marks it executable (non-Windows).
export async function gunzipToFile(srcGz, destExe) {
  await pipeline(createReadStream(srcGz), createGunzip(), createWriteStream(destExe));
  if (process.platform !== 'win32') chmodSync(destExe, 0o755);
}

// Extracts a single named entry from a non-Zip64 .zip archive using
// node:zlib. Used instead of shelling out to `unzip`, which is not
// available on stock Windows.
export function extractZipEntry(archivePath, innerPath, destExe) {
  const fd = openSync(archivePath, 'r');
  try {
    const fileSize = statSync(archivePath).size;

    // The End-of-Central-Directory record sits within the last (22 + 65535)
    // bytes of the archive; scan that window for its 0x06054b50 signature.
    const scanLen = Math.min(22 + 0xffff, fileSize);
    const tail = Buffer.alloc(scanLen);
    readSync(fd, tail, 0, scanLen, fileSize - scanLen);
    let eocd = -1;
    for (let i = tail.length - 22; i >= 0; i--) {
      if (tail.readUInt32LE(i) === 0x06054b50) { eocd = i; break; }
    }
    if (eocd < 0) throw new Error(`zip: EOCD not found in ${archivePath}`);

    const cdEntries = tail.readUInt16LE(eocd + 10);
    const cdSize    = tail.readUInt32LE(eocd + 12);
    const cdOffset  = tail.readUInt32LE(eocd + 16);
    if (cdSize === 0xffffffff || cdOffset === 0xffffffff || cdEntries === 0xffff) {
      throw new Error(`zip: Zip64 archives are not supported (${archivePath})`);
    }

    const cd = Buffer.alloc(cdSize);
    readSync(fd, cd, 0, cdSize, cdOffset);

    let p = 0;
    for (let i = 0; i < cdEntries; i++) {
      if (cd.readUInt32LE(p) !== 0x02014b50) throw new Error(`zip: bad central directory entry at offset ${p}`);
      const method           = cd.readUInt16LE(p + 10);
      const compressedSize   = cd.readUInt32LE(p + 20);
      const uncompressedSize = cd.readUInt32LE(p + 24);
      const nameLen          = cd.readUInt16LE(p + 28);
      const extraLen         = cd.readUInt16LE(p + 30);
      const commentLen       = cd.readUInt16LE(p + 32);
      const lfhOffset        = cd.readUInt32LE(p + 42);
      const name             = cd.toString('utf8', p + 46, p + 46 + nameLen);
      p += 46 + nameLen + extraLen + commentLen;
      if (name !== innerPath) continue;

      if (compressedSize === 0xffffffff || uncompressedSize === 0xffffffff) {
        throw new Error(`zip: Zip64 entry not supported (${innerPath})`);
      }

      // Local file header has variable-length name/extra fields; read it to
      // compute the actual file-data offset.
      const lfh = Buffer.alloc(30);
      readSync(fd, lfh, 0, 30, lfhOffset);
      if (lfh.readUInt32LE(0) !== 0x04034b50) throw new Error(`zip: bad local file header for ${innerPath}`);
      const dataStart = lfhOffset + 30 + lfh.readUInt16LE(26) + lfh.readUInt16LE(28);

      const compressed = Buffer.alloc(compressedSize);
      readSync(fd, compressed, 0, compressedSize, dataStart);

      let raw;
      if (method === 0) raw = compressed;
      else if (method === 8) raw = inflateRawSync(compressed);
      else throw new Error(`zip: unsupported compression method ${method} for ${innerPath}`);

      if (raw.length !== uncompressedSize) {
        throw new Error(`zip: decompressed size ${raw.length} != expected ${uncompressedSize} for ${innerPath}`);
      }
      writeFileSync(destExe, raw);
      return;
    }
    throw new Error(`zip: ${innerPath} not found in ${archivePath}`);
  } finally {
    closeSync(fd);
  }
}

// Streams the named entry out of a .tar.xz via the system `tar` binary
// (available on macOS, Linux, and modern Windows). Piped to disk so we
// never buffer the ~100 MiB binary in memory.
export async function extractTarXzEntry(archivePath, innerPath, destExe) {
  const child = spawn('tar', ['-xJf', archivePath, '-O', innerPath], { stdio: ['ignore', 'pipe', 'inherit'] });
  const out = createWriteStream(destExe);
  const piped = pipeline(child.stdout, out);
  const exited = new Promise((res, rej) => {
    child.on('error', rej);
    child.on('exit', (code, signal) => {
      if (code === 0) res();
      else rej(new Error(`tar -xJ failed for ${archivePath}: exit ${code}${signal ? ` signal ${signal}` : ''}`));
    });
  });
  await Promise.all([piped, exited]);
}

export async function extractInner(archivePath, sourceUrl, innerPath, destExe) {
  if (sourceUrl.endsWith('.zip')) {
    extractZipEntry(archivePath, innerPath, destExe);
  } else if (sourceUrl.endsWith('.tar.xz') || sourceUrl.endsWith('.txz')) {
    await extractTarXzEntry(archivePath, innerPath, destExe);
  } else {
    throw new Error(`unsupported archive format: ${sourceUrl}`);
  }
  if (process.platform !== 'win32') chmodSync(destExe, 0o755);
}
