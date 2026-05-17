import { useEffect, useState } from 'react';
import { getCurrentWebview } from '@tauri-apps/api/webview';

export interface UseDropTargetOptions {
  onDrop: (path: string) => void;
  onMultiFileIgnored?: () => void;
}

export function useDropTarget(opts: UseDropTargetOptions): { isDragOver: boolean } {
  const [isDragOver, setIsDragOver] = useState(false);

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
          if (paths.length > 1) opts.onMultiFileIgnored?.();
          if (paths.length >= 1) opts.onDrop(paths[0] as string);
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
    // opts is intentionally excluded from deps to avoid re-subscribing on every render.
    // Callers should memoize callbacks if stable identity matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { isDragOver };
}
