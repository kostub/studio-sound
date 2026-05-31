import { useEffect, useRef, useState } from 'react';
import { getCurrentWebview } from '@tauri-apps/api/webview';

export interface UseDropTargetOptions {
  onDrop: (path: string) => void;
  onMultiFileIgnored?: () => void;
}

export function useDropTarget(opts: UseDropTargetOptions): { isDragOver: boolean } {
  const [isDragOver, setIsDragOver] = useState(false);
  // Track the latest opts in a ref so the stable (subscribe-once) event listener
  // always calls the current callbacks instead of the closure captured at the
  // first render. Without this, callbacks that read component state (e.g. the
  // `status` check in WorkspaceShell) would observe stale values on later drops.
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
          if (paths.length > 1) optsRef.current.onMultiFileIgnored?.();
          if (paths.length >= 1) optsRef.current.onDrop(paths[0] as string);
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
