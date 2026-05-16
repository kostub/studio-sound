/**
 * Frontend IPC client — thin wrappers around Tauri's `invoke` for the three
 * Phase 1 IPC methods.
 *
 * All three Tauri commands return `serde_json::Value` on success and a plain
 * string on error.  The wrappers here re-throw string rejections as `IpcError`
 * objects so callers have a uniform error type.
 */

import { invoke } from '@tauri-apps/api/core';

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

/** Parse the string rejection from Tauri into a structured IpcError. */
function toIpcError(err: unknown): IpcError {
  if (typeof err === 'string') {
    // Tauri serialises command errors as plain strings.
    return { code: 'UNKNOWN', message: err };
  }
  if (err !== null && typeof err === 'object' && 'code' in err && 'message' in err) {
    return err as IpcError;
  }
  return { code: 'UNKNOWN', message: String(err) };
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

/** Opens the platform log directory in the system file manager. */
export async function openLogsFolder(): Promise<void> {
  return invoke('open_logs_folder');
}
