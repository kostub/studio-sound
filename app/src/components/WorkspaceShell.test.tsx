import '@testing-library/jest-dom/vitest';

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';

vi.mock('../ipc/client', () => ({ probe: vi.fn() }));
vi.mock('@tauri-apps/api/webview', () => ({
  getCurrentWebview: () => ({ onDragDropEvent: () => Promise.resolve(() => {}) }),
}));

import { resetWorkspaceForTest, useWorkspace } from '../state/workspace';
import { WorkspaceShell } from './WorkspaceShell';

beforeEach(() => resetWorkspaceForTest());
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
});
