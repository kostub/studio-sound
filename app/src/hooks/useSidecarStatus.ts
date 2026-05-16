/**
 * useSidecarStatus — React hook that polls the sidecar via system.ping and
 * returns the current connectivity status.
 *
 * Returns:
 *   status  — 'connected' | 'disconnected' | 'error'
 *   error   — error message string when status === 'error'
 *
 * Behaviour:
 *   - Pings immediately on mount.
 *   - Re-pings every POLL_INTERVAL_MS milliseconds.
 *   - Cleans up the interval on unmount.
 *   - ping() success → 'connected'; rejection → 'error' + message.
 */

import { useEffect, useState } from 'react';

import { ping } from '../ipc/client';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type SidecarStatus = 'connected' | 'disconnected' | 'error';

export interface SidecarStatusState {
  status: SidecarStatus;
  error?: string;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 5_000;

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * Periodically pings the sidecar and returns `{ status, error? }`.
 *
 * @param intervalMs  Polling interval in milliseconds (default: 5 000).
 */
export function useSidecarStatus(intervalMs: number = POLL_INTERVAL_MS): SidecarStatusState {
  const [state, setState] = useState<SidecarStatusState>({
    status: 'disconnected',
  });

  useEffect(() => {
    let cancelled = false;

    async function doPing(): Promise<void> {
      try {
        await ping();
        if (!cancelled) {
          setState({ status: 'connected' });
        }
      } catch (err: unknown) {
        if (!cancelled) {
          const message =
            err !== null && typeof err === 'object' && 'message' in err
              ? String((err as { message: unknown }).message)
              : String(err);
          setState({ status: 'error', error: message });
        }
      }
    }

    // Ping immediately on mount.
    void doPing();

    // Then poll at the given interval.
    const id = setInterval(() => {
      void doPing();
    }, intervalMs);

    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs]);

  return state;
}
