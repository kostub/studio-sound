import '@testing-library/jest-dom/vitest';

import { fireEvent, render, screen, act, cleanup } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { DiagnosticsDrawer } from './DiagnosticsDrawer';
import type { ProbeResult } from '../ipc/generated/media.probe';

const writeText = vi.fn().mockResolvedValue(undefined);
Object.assign(navigator, { clipboard: { writeText } });

const sample: ProbeResult = {
  id: '01H...ULID',
  path: '/tmp/a.mp4',
  filename: 'a.mp4',
  sizeBytes: 204800,
  durationSeconds: 12.345,
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

describe('DiagnosticsDrawer', () => {
  beforeEach(() => writeText.mockClear());
  afterEach(() => cleanup());

  it('renders nothing when closed', () => {
    const { container } = render(
      <DiagnosticsDrawer open={false} onClose={() => {}} result={sample} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders metadata sections when open', () => {
    render(<DiagnosticsDrawer open={true} onClose={() => {}} result={sample} />);
    // Shows the probe id
    expect(screen.getByText(/01H\.\.\.ULID/)).toBeInTheDocument();
    // Shows video codec
    expect(screen.getByText(/h264/)).toBeInTheDocument();
    // Shows audio codec (may appear in summary + track list)
    expect(screen.getAllByText(/aac/).length).toBeGreaterThan(0);
  });

  it('copies diagnostics JSON to the clipboard and shows a 2s toast', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    render(<DiagnosticsDrawer open={true} onClose={() => {}} result={sample} />);
    // Click copy button
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));
      // flush the clipboard promise
      await Promise.resolve();
    });
    expect(writeText).toHaveBeenCalledTimes(1);
    const firstCallArg = writeText.mock.calls[0]?.[0] as string;
    expect(JSON.parse(firstCallArg)).toMatchObject({ path: '/tmp/a.mp4' });
    expect(screen.getByText(/copied/i)).toBeInTheDocument();
    // Advance past the 2s toast
    await act(async () => {
      vi.advanceTimersByTime(2100);
    });
    expect(screen.queryByText(/copied/i)).toBeNull();
    vi.useRealTimers();
  });

  it('marks the drawer as a modal dialog for assistive tech', () => {
    render(<DiagnosticsDrawer open={true} onClose={() => {}} result={sample} />);
    expect(screen.getByRole('dialog', { name: /diagnostics/i })).toHaveAttribute(
      'aria-modal',
      'true',
    );
  });

  it('renders unsupported without a trailing colon when there are no issues', () => {
    const unsupported: ProbeResult = {
      ...sample,
      compatibility: { supported: false, issues: [], warnings: [] },
    };
    render(<DiagnosticsDrawer open={true} onClose={() => {}} result={unsupported} />);
    expect(screen.getByText('unsupported')).toBeInTheDocument();
    expect(screen.queryByText(/unsupported:\s*$/)).toBeNull();
  });

  it('invokes onClose on Esc and on backdrop click and on the close button', () => {
    const onClose = vi.fn();
    render(<DiagnosticsDrawer open={true} onClose={onClose} result={sample} />);
    fireEvent.keyDown(window, { key: 'Escape' });
    fireEvent.click(screen.getByTestId('diagnostics-backdrop'));
    fireEvent.click(screen.getByRole('button', { name: /close diagnostics/i }));
    expect(onClose).toHaveBeenCalledTimes(3);
  });
});
