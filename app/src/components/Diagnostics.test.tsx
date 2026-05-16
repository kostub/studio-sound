import '@testing-library/jest-dom/vitest';

import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

// ---------------------------------------------------------------------------
// Mock @tauri-apps/api/core before importing anything that uses it.
// ---------------------------------------------------------------------------

vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}));

// Import the mock *after* vi.mock so we can control the return value per test.
import { invoke } from '@tauri-apps/api/core';

import { Diagnostics } from './Diagnostics';

// Cast to a typed mock helper.
const mockInvoke = vi.mocked(invoke);

// Unmount rendered components after each test so the DOM is clean.
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Diagnostics', () => {
  it('renders without crashing', () => {
    // Return a never-resolving promise so the component stays in loading state.
    mockInvoke.mockReturnValue(new Promise(() => undefined));

    render(<Diagnostics />);

    expect(screen.getByRole('region', { name: /ipc diagnostics/i })).toBeInTheDocument();
  });

  it('shows "connected" after a successful ping', async () => {
    // Simulate a successful ping response matching the PingResult shape.
    const pingResponse = {
      pong: true,
      sidecarVersion: '0.1.0',
      uptimeMs: 123,
      supportedProtocolVersions: [1],
    };
    mockInvoke.mockResolvedValue(pingResponse);

    render(<Diagnostics />);

    // Wait for the async ping to resolve and the component to re-render.
    await waitFor(() => {
      expect(screen.getByTestId('ping-status')).toHaveTextContent('connected');
    });
  });

  it('shows an error message when ping fails', async () => {
    // Simulate a Tauri command rejection (plain string as per Rust error mapping).
    mockInvoke.mockRejectedValue('SIDECAR_UNAVAILABLE: sidecar is not running');

    render(<Diagnostics />);

    await waitFor(() => {
      expect(screen.getByTestId('ping-status')).toHaveTextContent('error');
    });
  });
});
