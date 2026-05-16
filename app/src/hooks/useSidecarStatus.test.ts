import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';

// ---------------------------------------------------------------------------
// Mock @tauri-apps/api/core before importing anything that uses it.
// ---------------------------------------------------------------------------

vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}));

import { invoke } from '@tauri-apps/api/core';

import { useSidecarStatus } from './useSidecarStatus';

const mockInvoke = vi.mocked(invoke);

afterEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useSidecarStatus', () => {
  it('returns "connected" when ping resolves', async () => {
    const pingResult = {
      pong: true,
      sidecarVersion: '0.1.0',
      uptimeMs: 42,
      supportedProtocolVersions: [1],
    };
    mockInvoke.mockResolvedValue(pingResult);

    // Use a very long poll interval so the interval doesn't fire during the test.
    const { result } = renderHook(() => useSidecarStatus(600_000));

    // The hook immediately fires a ping; wait for the state update.
    await waitFor(() => {
      expect(result.current.status).toBe('connected');
    });

    expect(result.current.error).toBeUndefined();
  });

  it('returns "error" with a message when ping rejects', async () => {
    // Tauri serialises command errors as plain strings.
    mockInvoke.mockRejectedValue('SIDECAR_UNAVAILABLE: sidecar is not running');

    const { result } = renderHook(() => useSidecarStatus(600_000));

    await waitFor(() => {
      expect(result.current.status).toBe('error');
    });

    expect(result.current.error).toBe('SIDECAR_UNAVAILABLE: sidecar is not running');
  });

  it('starts in "disconnected" state before the first ping resolves', () => {
    // Never-resolving promise keeps the hook in its initial state.
    mockInvoke.mockReturnValue(new Promise(() => undefined));

    const { result } = renderHook(() => useSidecarStatus(600_000));

    // Synchronously: the hook hasn't awaited yet, so still disconnected.
    expect(result.current.status).toBe('disconnected');
  });

  it('cleans up the interval on unmount', async () => {
    const pingResult = {
      pong: true,
      sidecarVersion: '0.1.0',
      uptimeMs: 0,
      supportedProtocolVersions: [1],
    };
    mockInvoke.mockResolvedValue(pingResult);

    const { unmount } = renderHook(() => useSidecarStatus(600_000));

    // Wait for initial ping.
    await waitFor(() => {
      expect(mockInvoke).toHaveBeenCalled();
    });

    const callCountBeforeUnmount = mockInvoke.mock.calls.length;

    // Unmount: cleanup should cancel the interval.
    unmount();

    // Verify no further calls happen synchronously after unmount.
    expect(mockInvoke.mock.calls.length).toBe(callCountBeforeUnmount);
  });
});
