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
  it('emits the dropped path via onDrop', async () => {
    const onDrop = vi.fn();
    const { result } = renderHook(() => useDropTarget({ onDrop }));
    // Wait for the subscription to register.
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/a/b.mp4'] } });
    });
    expect(onDrop).toHaveBeenCalledWith('/a/b.mp4');
    expect(result.current.isDragOver).toBe(false);
  });

  it('sets isDragOver=true on dragenter', async () => {
    const { result } = renderHook(() => useDropTarget({ onDrop: () => {} }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'enter' } });
    });
    expect(result.current.isDragOver).toBe(true);
  });

  it('resets isDragOver=false on dragleave', async () => {
    const { result } = renderHook(() => useDropTarget({ onDrop: () => {} }));
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
    const { result } = renderHook(() => useDropTarget({ onDrop: () => {} }));
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

  it('ignores multi-file drops beyond the first path', async () => {
    const onDrop = vi.fn();
    const onIgnored = vi.fn();
    renderHook(() => useDropTarget({ onDrop, onMultiFileIgnored: onIgnored }));
    await Promise.resolve();
    await act(async () => {
      handlers[0]?.({ payload: { type: 'drop', paths: ['/a.mp4', '/b.mp4'] } });
    });
    expect(onDrop).toHaveBeenCalledWith('/a.mp4');
    expect(onIgnored).toHaveBeenCalled();
  });
});
