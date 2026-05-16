/**
 * Diagnostics screen — hidden in production builds (gated by
 * `import.meta.env.DEV` in App.tsx).
 *
 * Provides:
 * - Sidecar connectivity status (via system.ping on mount)
 * - Echo round-trip: text input + Send button
 */

import type { JSX } from 'react';
import { useEffect, useRef, useState } from 'react';

import { echo, ping } from '../ipc/client';
import type { IpcError } from '../ipc/client';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type StatusKind = 'idle' | 'loading' | 'connected' | 'error';

interface StatusState {
  kind: StatusKind;
  message?: string;
}

function StatusBadge({ status }: { status: StatusState }): JSX.Element {
  const label =
    status.kind === 'connected'
      ? 'connected'
      : status.kind === 'loading'
        ? 'checking…'
        : status.kind === 'error'
          ? `error: ${status.message ?? 'unknown'}`
          : 'idle';

  return <span data-testid="ping-status">{label}</span>;
}

function IpcErrorBlock({ err }: { err: IpcError }): JSX.Element {
  return (
    <div role="alert" aria-label="IPC error">
      <strong>{err.code}</strong>: {err.message}
      {err.details !== undefined && (
        <details>
          <summary>Details</summary>
          <pre>{JSON.stringify(err.details, null, 2)}</pre>
        </details>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function Diagnostics(): JSX.Element {
  // --- Ping state ---
  const [pingStatus, setPingStatus] = useState<StatusState>({ kind: 'idle' });
  const [pingError, setPingError] = useState<IpcError | null>(null);

  // --- Echo state ---
  const [echoText, setEchoText] = useState('');
  const [echoResult, setEchoResult] = useState<string | null>(null);
  const [echoLoading, setEchoLoading] = useState(false);
  const [echoError, setEchoError] = useState<IpcError | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  // Run a ping on mount to check connectivity.
  useEffect(() => {
    setPingStatus({ kind: 'loading' });
    setPingError(null);

    ping()
      .then(() => {
        setPingStatus({ kind: 'connected' });
      })
      .catch((err: unknown) => {
        const ipcErr = err as IpcError;
        setPingStatus({ kind: 'error', message: ipcErr.message });
        setPingError(ipcErr);
      });
  }, []);

  // --- Echo handler ---
  function handleEcho(): void {
    if (!echoText) return;
    setEchoLoading(true);
    setEchoError(null);
    setEchoResult(null);

    echo(echoText)
      .then((res) => {
        setEchoResult(res.text);
      })
      .catch((err: unknown) => {
        setEchoError(err as IpcError);
      })
      .finally(() => {
        setEchoLoading(false);
      });
  }

  return (
    <section aria-label="IPC Diagnostics">
      <h2>IPC Diagnostics</h2>

      {/* Connectivity */}
      <div>
        <strong>Sidecar status:</strong>{' '}
        <StatusBadge status={pingStatus} />
        {pingError !== null && <IpcErrorBlock err={pingError} />}
      </div>

      {/* Echo */}
      <div>
        <label htmlFor="echo-input">Echo</label>
        <input
          id="echo-input"
          ref={inputRef}
          type="text"
          value={echoText}
          onChange={(e) => {
            setEchoText(e.target.value);
          }}
          placeholder="Type something…"
          disabled={echoLoading}
        />
        <button
          type="button"
          onClick={handleEcho}
          disabled={echoLoading || !echoText}
        >
          {echoLoading ? 'Sending…' : 'Echo'}
        </button>

        {echoResult !== null && (
          <p data-testid="echo-result">Result: {echoResult}</p>
        )}
        {echoError !== null && <IpcErrorBlock err={echoError} />}
      </div>
    </section>
  );
}
