import '@testing-library/jest-dom/vitest';

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';

vi.mock('./ipc/client', () => ({ probe: vi.fn() }));
vi.mock('@tauri-apps/api/webview', () => ({
  getCurrentWebview: () => ({ onDragDropEvent: () => Promise.resolve(() => {}) }),
}));

import { App } from './App';

afterEach(() => cleanup());

describe('App', () => {
  it('renders WorkspaceShell (EmptyState visible by default)', () => {
    render(<App />);

    expect(screen.getByText(/drop a video here/i)).toBeInTheDocument();
  });
});
