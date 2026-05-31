import '@testing-library/jest-dom/vitest';

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, cleanup, act, fireEvent } from '@testing-library/react';
import type { ProbeResult } from '../ipc/generated/media.probe';

// Handlers registered by useDropTarget stub
type DragDropHandler = (e: { payload?: { type?: string; paths?: string[] } }) => void;
const handlers: DragDropHandler[] = [];
vi.mock('@tauri-apps/api/webview', () => ({
  getCurrentWebview: () => ({
    onDragDropEvent: (cb: DragDropHandler) => {
      handlers.push(cb);
      return Promise.resolve(() => {
        const i = handlers.indexOf(cb);
        if (i >= 0) handlers.splice(i, 1);
      });
    },
  }),
}));

vi.mock('../ipc/client', () => ({ probe: vi.fn() }));

import { probe } from '../ipc/client';
import { resetWorkspaceForTest, useWorkspace } from '../state/workspace';
import { WorkspaceShell } from './WorkspaceShell';

const mockProbe = vi.mocked(probe);

const validResult: ProbeResult = {
  id: 'A',
  path: '/tmp/tiny-h264-aac-stereo.mp4',
  filename: 'tiny-h264-aac-stereo.mp4',
  sizeBytes: 204800,
  durationSeconds: 3,
  container: { format: 'mov,mp4,m4a,3gp,3g2,mj2', longName: 'QuickTime / MOV' },
  video: { codec: 'h264', width: 1920, height: 1080, fps: 30 },
  audio: {
    codec: 'aac',
    channels: 2,
    sampleRate: 48000,
    trackIndex: 0,
    trackCount: 1,
    tracks: [{ index: 0, codec: 'aac', channels: 2, sampleRate: 48000, isDefault: true }],
  },
  compatibility: { supported: true, issues: [], warnings: [] },
};

const unsupportedResult = {
  id: 'B',
  path: '/tmp/tiny-no-audio.mp4',
  filename: 'tiny-no-audio.mp4',
  sizeBytes: 102400,
  durationSeconds: 2,
  container: { format: 'mov,mp4,m4a,3gp,3g2,mj2', longName: 'QuickTime / MOV' },
  video: { codec: 'h264', width: 1280, height: 720, fps: 24 },
  audio: null,
  compatibility: { supported: false, issues: ['No audio stream found'], warnings: [] },
};

beforeEach(() => {
  resetWorkspaceForTest();
  handlers.length = 0;
  mockProbe.mockReset();
});
afterEach(() => cleanup());

describe('WorkspaceShell', () => {
  it('shows EmptyState when status is EMPTY', () => {
    render(<WorkspaceShell />);
    expect(screen.getByText(/drop a video here/i)).toBeInTheDocument();
  });

  it('shows ActiveFileCard when a file is loaded', () => {
    useWorkspace.setState({ status: 'READY', path: '/x/hello.mp4' });
    render(<WorkspaceShell />);
    expect(screen.getByText('hello.mp4')).toBeInTheDocument();
  });

  // ── Smoke-test golden paths (item 59) ──────────────────────────────────────

  it('[smoke] valid drop → PROBING spinner then READY with green dot and metadata', async () => {
    mockProbe.mockResolvedValue(validResult);
    render(<WorkspaceShell />);
    // Wait for drop handler to register
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/tiny-h264-aac-stereo.mp4'] } });
    });

    // Probe resolves → READY
    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.getByText('tiny-h264-aac-stereo.mp4')).toBeInTheDocument();
    expect(screen.getByTestId('status-dot').className).toMatch(/green|ready/);
    // Video and audio codec metadata visible
    expect(screen.getAllByText(/h264/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/aac/).length).toBeGreaterThan(0);
  });

  it('[smoke] unsupported drop → READY with yellow dot and Unsupported label', async () => {
    mockProbe.mockResolvedValue(unsupportedResult);
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/tiny-no-audio.mp4'] } });
    });
    await act(async () => { await Promise.resolve(); });

    expect(screen.getByText('tiny-no-audio.mp4')).toBeInTheDocument();
    expect(screen.getByTestId('status-dot').className).toMatch(/yellow|unsupported/);
    expect(screen.getAllByText(/unsupported/i).length).toBeGreaterThan(0);
  });

  it('[smoke] corrupt drop → ERROR with red dot, Retry visible', async () => {
    mockProbe.mockRejectedValue({ code: 'CORRUPT_MEDIA', message: 'Truncated file' });
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/corrupt-truncated.mp4'] } });
    });
    await act(async () => { await Promise.resolve(); });

    expect(screen.getByTestId('status-dot').className).toMatch(/red|error/);
    expect(screen.getByText(/error/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('[smoke] Remove after READY resets to EmptyState', async () => {
    mockProbe.mockResolvedValue(validResult);
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/tiny-h264-aac-stereo.mp4'] } });
    });
    await act(async () => { await Promise.resolve(); });

    // Click Remove
    act(() => {
      screen.getByRole('button', { name: /remove/i }).click();
    });

    expect(screen.getByText(/drop a video here/i)).toBeInTheDocument();
  });

  // ── ReplaceFileDialog wiring (item 67) ─────────────────────────────────────

  it('calls loadFile when a drop arrives and workspace is EMPTY', async () => {
    // Use a never-resolving probe so we can assert the PROBING state before it completes.
    mockProbe.mockReturnValue(new Promise<ProbeResult>(() => {}));
    useWorkspace.setState({ status: 'EMPTY', path: undefined, result: undefined, error: undefined });
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/a.mp4'] } });
    });

    expect(useWorkspace.getState().status).toBe('PROBING');
    expect(useWorkspace.getState().path).toBe('/tmp/a.mp4');
  });

  it('opens ReplaceFileDialog when a drop arrives and workspace is not EMPTY', async () => {
    useWorkspace.setState({
      status: 'READY',
      path: '/tmp/old.mp4',
      result: validResult,
      error: undefined,
    });
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/new.mp4'] } });
    });

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText(/new\.mp4/)).toBeInTheDocument();
  });

  it('Replace button confirms replacement and closes the dialog', async () => {
    mockProbe.mockResolvedValue(validResult);
    useWorkspace.setState({
      status: 'READY',
      path: '/tmp/old.mp4',
      result: validResult,
      error: undefined,
    });
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/new.mp4'] } });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /replace/i }));
    });

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(useWorkspace.getState().path).toBe('/tmp/new.mp4');
  });

  it('Cancel closes the dialog without changing state', async () => {
    useWorkspace.setState({
      status: 'READY',
      path: '/tmp/old.mp4',
      result: validResult,
      error: undefined,
    });
    render(<WorkspaceShell />);
    await Promise.resolve();

    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/tmp/new.mp4'] } });
    });

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(useWorkspace.getState().path).toBe('/tmp/old.mp4');
  });
});
