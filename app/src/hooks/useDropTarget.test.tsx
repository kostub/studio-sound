import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// Stub the Tauri webview API.
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

import { useDropTarget } from './useDropTarget';

beforeEach(() => { handlers.length = 0; });

describe('useDropTarget', () => {
  it('calls onPath with the dropped file path for single-file drops', async () => {
    const onPath = vi.fn();
    renderHook(() => useDropTarget({ onPath }));
    // Wait for the subscription to register.
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/a/b.mp4'] } });
    });
    expect(onPath).toHaveBeenCalledWith('/a/b.mp4');
  });

  it('sets isDragOver=true on dragenter', async () => {
    const { result } = renderHook(() => useDropTarget({ onPath: () => {} }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'enter' } });
    });
    expect(result.current.isDragOver).toBe(true);
  });

  it('resets isDragOver=false on dragleave', async () => {
    const { result } = renderHook(() => useDropTarget({ onPath: () => {} }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'enter' } });
    });
    expect(result.current.isDragOver).toBe(true);
    await act(async () => {
      handlers[0]?.({ payload: { type: 'leave' } });
    });
    expect(result.current.isDragOver).toBe(false);
  });

  it('resets isDragOver=false on cancel', async () => {
    const { result } = renderHook(() => useDropTarget({ onPath: () => {} }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'enter' } });
    });
    expect(result.current.isDragOver).toBe(true);
    await act(async () => {
      handlers[0]?.({ payload: { type: 'cancel' } });
    });
    expect(result.current.isDragOver).toBe(false);
  });

  it('does not call onPath for multi-file drops', async () => {
    const onPath = vi.fn();
    renderHook(() => useDropTarget({ onPath }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/a.mp4', '/b.mp4'] } });
    });
    expect(onPath).not.toHaveBeenCalled();
  });
});
