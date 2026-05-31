import '@testing-library/jest-dom/vitest';

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EmptyState } from './EmptyState';

describe('EmptyState', () => {
  it('renders the drop zone, supported-formats line, and privacy line', () => {
    render(<EmptyState isDragOver={false} />);
    expect(screen.getByText(/drag.*drop|drop a video/i)).toBeInTheDocument();
    expect(screen.getByText(/mp4|mov|webm/i)).toBeInTheDocument();
    expect(screen.getByText(/never leaves|stays on your device|local/i)).toBeInTheDocument();
  });

  it('does NOT render a Browse button (drop-only Phase 3)', () => {
    render(<EmptyState isDragOver={false} />);
    expect(screen.queryByRole('button', { name: /browse/i })).toBeNull();
  });

  it('applies drag-over visual state', () => {
    const { container } = render(<EmptyState isDragOver />);
    const el = container.firstChild as HTMLElement;
    expect(el.className).toMatch(/drag-?over|drop-?active/);
  });
});
