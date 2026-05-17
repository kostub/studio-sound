/**
 * Frontend IPC client — thin wrappers around Tauri's `invoke` for the three
 * Phase 1 IPC methods.
 *
 * All three Tauri commands return `serde_json::Value` on success and a
 * structured `{ code, message, details? }` error (`SerializableIpcError`)
 * on failure.  `toIpcError` normalises rejections — preferring the structured
 * shape, with a string/unknown fallback that maps to `code: 'UNKNOWN'`.
 */

import { invoke } from '@tauri-apps/api/core';

import type { ProbeResult } from './generated/media.probe';

// ---------------------------------------------------------------------------
// Result types (mirror the JSON Schemas for system.ping / system.echo / system.shutdown)
// ---------------------------------------------------------------------------

export interface PingResult {
  pong: true;
  sidecarVersion: string;
  uptimeMs: number;
  supportedProtocolVersions: number[];
}

export interface EchoResult {
  text: string;
}

export interface ShutdownResult {
  accepted: true;
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

export interface IpcError {
  code: string;
  message: string;
  details?: unknown;
}

export function toIpcError(err: unknown): IpcError {
  if (
    err !== null &&
    typeof err === 'object' &&
    typeof (err as { code?: unknown }).code === 'string' &&
    typeof (err as { message?: unknown }).message === 'string'
  ) {
    const e = err as { code: string; message: string; details?: unknown };
    return { code: e.code, message: e.message, ...(e.details !== undefined ? { details: e.details } : {}) };
  }
  if (typeof err === 'string') {
    return { code: 'UNKNOWN', message: err };
  }
  return { code: 'UNKNOWN', message: String(err ?? 'unknown error') };
}

// ---------------------------------------------------------------------------
// IPC wrappers
// ---------------------------------------------------------------------------

/** Sends a `system.ping` request to the sidecar. */
export async function ping(): Promise<PingResult> {
  try {
    return await invoke<PingResult>('ipc_ping');
  } catch (err) {
    throw toIpcError(err);
  }
}

/** Sends a `system.echo` request with the given `text` to the sidecar. */
export async function echo(text: string): Promise<EchoResult> {
  try {
    return await invoke<EchoResult>('ipc_echo', { text });
  } catch (err) {
    throw toIpcError(err);
  }
}

/** Sends a `system.shutdown` request to the sidecar. */
export async function shutdown(): Promise<ShutdownResult> {
  try {
    return await invoke<ShutdownResult>('ipc_shutdown');
  } catch (err) {
    throw toIpcError(err);
  }
}

/** Probes a media file and returns the canonical MediaProbeResult. */
export async function probe(path: string): Promise<ProbeResult> {
  try {
    return (await invoke('media_probe', { path })) as ProbeResult;
  } catch (e) {
    throw toIpcError(e);
  }
}

/** Opens the platform log directory in the system file manager. */
export async function openLogsFolder(): Promise<void> {
  return invoke('open_logs_folder');
}
