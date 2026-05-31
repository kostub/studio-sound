import '@testing-library/jest-dom/vitest';

import { fireEvent, render, screen, cleanup } from '@testing-library/react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { ReplaceFileDialog } from './ReplaceFileDialog';

afterEach(() => cleanup());

describe('ReplaceFileDialog', () => {
  it('renders nothing when no incoming path', () => {
    const { container } = render(
      <ReplaceFileDialog
        incomingPath={null}
        currentFilename="old.mp4"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('shows incoming filename + current filename', () => {
    render(
      <ReplaceFileDialog
        incomingPath="/tmp/new.mp4"
        currentFilename="old.mp4"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    expect(screen.getByText(/new\.mp4/)).toBeInTheDocument();
    expect(screen.getByText(/old\.mp4/)).toBeInTheDocument();
  });

  it('invokes onConfirm with the incoming path on Replace and onCancel otherwise', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ReplaceFileDialog
        incomingPath="/tmp/new.mp4"
        currentFilename="old.mp4"
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /replace/i }));
    expect(onConfirm).toHaveBeenCalledWith('/tmp/new.mp4');
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});
