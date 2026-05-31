import '@testing-library/jest-dom/vitest';

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import { useWorkspace, resetWorkspaceForTest } from '../state/workspace';
import type { ProbeResult } from '../ipc/generated/media.probe';
import { ActiveFileCard } from './ActiveFileCard';

vi.mock('../ipc/client', () => ({ probe: vi.fn() }));

describe('ActiveFileCard', () => {
  beforeEach(() => resetWorkspaceForTest());
  afterEach(() => cleanup());

  it('renders filename and probing dot during PROBING', () => {
    useWorkspace.setState({ status: 'PROBING', path: '/x/hello.mp4' });
    render(<ActiveFileCard />);
    expect(screen.getByText('hello.mp4')).toBeInTheDocument();
    expect(screen.getByTestId('status-dot').className).toMatch(/spinner|blue/);
  });

  it('renders green dot + metadata when READY and supported', () => {
    useWorkspace.setState({
      status: 'READY', path: '/x/hello.mp4',
      result: {
        id: 'A', path: '/x/hello.mp4', filename: 'hello.mp4', sizeBytes: 1024,
        durationSeconds: 5,
        container: { format: 'mp4', longName: 'MP4' },
        video: { codec: 'h264', width: 1280, height: 720, fps: 30 },
        audio: {
          codec: 'aac', channels: 2, sampleRate: 48000, trackIndex: 0, trackCount: 1,
          tracks: [{ index: 0, codec: 'aac', channels: 2, sampleRate: 48000, isDefault: true }],
        },
        compatibility: { supported: true, issues: [], warnings: [] },
      } satisfies ProbeResult,
    });
    render(<ActiveFileCard />);
    expect(screen.getByText(/h264/)).toBeInTheDocument();
    expect(screen.getByText(/aac/)).toBeInTheDocument();
    expect(screen.getByTestId('status-dot').className).toMatch(/green|ready/);
  });

  it('renders yellow dot when READY but supported=false', () => {
    useWorkspace.setState({
      status: 'READY', path: '/x/hello.wmv',
      result: {
        id: 'A', path: '/x/hello.wmv', filename: 'hello.wmv', sizeBytes: 1,
        container: { format: 'asf', longName: 'ASF' },
        audio: null,
        compatibility: { supported: false, issues: ['Unsupported container: asf'], warnings: [] },
      } satisfies ProbeResult,
    });
    render(<ActiveFileCard />);
    expect(screen.getByTestId('status-dot').className).toMatch(/yellow|unsupported/);
  });

  it('renders red dot when ERROR', () => {
    useWorkspace.setState({
      status: 'ERROR', path: '/x/missing.mp4',
      error: { code: 'FILE_NOT_FOUND', message: 'missing' },
    });
    render(<ActiveFileCard />);
    expect(screen.getByTestId('status-dot').className).toMatch(/red|error/);
  });

  it('Retry button is shown only in ERROR', () => {
    useWorkspace.setState({ status: 'READY', path: '/x.mp4' });
    const { rerender } = render(<ActiveFileCard />);
    expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
    useWorkspace.setState({ status: 'ERROR', error: { code: 'X', message: '' } });
    rerender(<ActiveFileCard />);
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('Remove button clears workspace', () => {
    useWorkspace.setState({ status: 'READY', path: '/x.mp4' });
    render(<ActiveFileCard />);
    const btn = screen.getByRole('button', { name: /remove/i });
    btn.click();
    expect(useWorkspace.getState().status).toBe('EMPTY');
  });

  it('renders ErrorPanel when state is ERROR', () => {
    useWorkspace.setState({
      status: 'ERROR',
      path: '/tmp/x.mp4',
      error: { code: 'FFPROBE_FAILURE', message: 'failed' },
    });
    render(<ActiveFileCard />);
    expect(screen.getByRole('region', { name: /error/i })).toBeInTheDocument();
  });

  it('renders UnsupportedPanel when READY but supported=false', () => {
    useWorkspace.setState({
      status: 'READY',
      path: '/tmp/x.wmv',
      result: {
        id: 'A',
        path: '/tmp/x.wmv',
        filename: 'x.wmv',
        sizeBytes: 1,
        container: { format: 'asf', longName: 'ASF' },
        audio: null,
        compatibility: { supported: false, issues: ['Unsupported container: asf'], warnings: [] },
      } satisfies ProbeResult,
    });
    render(<ActiveFileCard />);
    expect(screen.getByRole('region', { name: /unsupported/i })).toBeInTheDocument();
  });

  it('opens DiagnosticsDrawer when Details button is clicked', () => {
    useWorkspace.setState({
      status: 'READY',
      path: '/tmp/x.mp4',
      result: {
        id: 'A',
        path: '/tmp/x.mp4',
        filename: 'x.mp4',
        sizeBytes: 1024,
        durationSeconds: 5,
        container: { format: 'mp4', longName: 'MP4' },
        audio: {
          codec: 'aac',
          channels: 2,
          sampleRate: 48000,
          trackIndex: 0,
          trackCount: 1,
          tracks: [{ index: 0, codec: 'aac', channels: 2, sampleRate: 48000, isDefault: true }],
        },
        compatibility: { supported: true, issues: [], warnings: [] },
      } satisfies ProbeResult,
    });
    render(<ActiveFileCard />);
    expect(screen.queryByRole('dialog', { name: /diagnostics/i })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /details/i }));
    expect(screen.getByRole('dialog', { name: /diagnostics/i })).toBeInTheDocument();
  });
});
