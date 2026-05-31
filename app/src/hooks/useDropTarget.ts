import { useEffect, useRef, useState } from 'react';
import { getCurrentWebview } from '@tauri-apps/api/webview';

export interface UseDropTargetOptions {
  onPath: (path: string) => void;
}

export function useDropTarget(opts: UseDropTargetOptions): { isDragOver: boolean } {
  const [isDragOver, setIsDragOver] = useState(false);
  // Track the latest opts in a ref so the stable (subscribe-once) event listener
  // always calls the current callbacks instead of the closure captured at the
  // first render. Without this, callbacks that read component state (e.g. the
  // `status` check in WorkspaceShell that decides load-vs-replace) would observe
  // stale values on later drops.
  const optsRef = useRef(opts);
  optsRef.current = opts;

  useEffect(() => {
    let unsub: (() => void) | undefined;
    let active = true;
    getCurrentWebview()
      .onDragDropEvent((e: unknown) => {
        const payload = (e as { payload?: { type?: string; paths?: string[] } }).payload;
        const t = payload?.type;
        if (t === 'enter' || t === 'over') {
          setIsDragOver(true);
        } else if (t === 'leave' || t === 'cancel') {
          setIsDragOver(false);
        } else if (t === 'drop') {
          setIsDragOver(false);
          const paths: string[] = payload?.paths ?? [];
          // Multi-file drops are ignored entirely in Phase 3.
          if (paths.length === 1) {
            optsRef.current.onPath(paths[0] as string);
          }
        }
      })
      .then((u: () => void) => {
        if (!active) u();
        else unsub = u;
      })
      .catch(() => {
        // Silently ignore subscription errors (e.g. in test environments).
      });
    return () => {
      active = false;
      if (unsub) unsub();
    };
  }, []);

  return { isDragOver };
}
