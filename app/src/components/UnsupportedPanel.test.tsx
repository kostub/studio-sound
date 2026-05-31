import '@testing-library/jest-dom/vitest';

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { UnsupportedPanel } from './UnsupportedPanel';

afterEach(() => cleanup());

describe('UnsupportedPanel', () => {
  it('lists each compatibility issue', () => {
    render(
      <UnsupportedPanel
        issues={['File has no audio streams', 'opus is not supported in this build']}
        onReplace={() => {}}
        onOpenDetails={() => {}}
      />,
    );
    expect(screen.getByText(/no audio streams/i)).toBeInTheDocument();
    expect(screen.getByText(/opus is not supported/i)).toBeInTheDocument();
  });

  it('invokes onReplace and onOpenDetails', () => {
    const onReplace = vi.fn();
    const onOpenDetails = vi.fn();
    render(
      <UnsupportedPanel
        issues={['wav containers are not supported']}
        onReplace={onReplace}
        onOpenDetails={onOpenDetails}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /replace.*file/i }));
    fireEvent.click(screen.getByRole('button', { name: /details/i }));
    expect(onReplace).toHaveBeenCalledTimes(1);
    expect(onOpenDetails).toHaveBeenCalledTimes(1);
  });

  it('renders fallback copy when issues is empty (defensive)', () => {
    render(<UnsupportedPanel issues={[]} onReplace={() => {}} onOpenDetails={() => {}} />);
    expect(screen.getByRole('heading')).toHaveTextContent(/unsupported/i);
  });
});
