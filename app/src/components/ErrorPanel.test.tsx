import '@testing-library/jest-dom/vitest';

import { fireEvent, render, screen, cleanup } from '@testing-library/react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { ErrorPanel } from './ErrorPanel';
import type { IpcError } from '../ipc/client';

afterEach(() => cleanup());

function makeError(code: string): IpcError {
  return {
    code,
    message: `msg for ${code}`,
    ...(code === 'FFPROBE_FAILURE'
      ? { details: { exitCode: 1, stderrTail: 'bad' } }
      : {}),
  };
}

describe('ErrorPanel', () => {
  it.each([
    ['FILE_NOT_FOUND', /file not found/i],
    ['ACCESS_DENIED', /can.?t read/i],
    ['CORRUPT_MEDIA', /corrupt|unreadable/i],
    ['FFPROBE_FAILURE', /couldn.?t analyze|ffprobe/i],
    ['FFPROBE_MISSING', /ffprobe.*missing|reinstall/i],
  ])('renders headline for %s', (code, headline) => {
    render(<ErrorPanel error={makeError(code)} onRetry={() => {}} onOpenDetails={() => {}} />);
    expect(screen.getByRole('heading')).toHaveTextContent(headline);
  });

  it('hides Retry for FFPROBE_MISSING (terminal: not user-recoverable)', () => {
    render(
      <ErrorPanel
        error={makeError('FFPROBE_MISSING')}
        onRetry={() => {}}
        onOpenDetails={() => {}}
      />,
    );
    expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
  });

  it('invokes onRetry and onOpenDetails', () => {
    const onRetry = vi.fn();
    const onOpenDetails = vi.fn();
    render(
      <ErrorPanel
        error={makeError('FFPROBE_FAILURE')}
        onRetry={onRetry}
        onOpenDetails={onOpenDetails}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    fireEvent.click(screen.getByRole('button', { name: /details/i }));
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onOpenDetails).toHaveBeenCalledTimes(1);
  });

  it('falls back to a generic headline for unknown codes', () => {
    render(
      <ErrorPanel
        error={{ code: 'UNKNOWN_CODE', message: 'x' }}
        onRetry={() => {}}
        onOpenDetails={() => {}}
      />,
    );
    expect(screen.getByRole('heading')).toHaveTextContent(/something went wrong/i);
  });
});
