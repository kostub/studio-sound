import type { JSX } from 'react';
import { useWorkspace } from '../state/workspace';
import { useDropTarget } from '../hooks/useDropTarget';
import { EmptyState } from './EmptyState';
import { ActiveFileCard } from './ActiveFileCard';

export function WorkspaceShell(): JSX.Element {
  const status = useWorkspace((s) => s.status);
  const loadFile = useWorkspace((s) => s.loadFile);
  const { isDragOver } = useDropTarget({
    onDrop: (path) => {
      if (status === 'EMPTY') {
        void loadFile(path);
      }
      // PR 8 wires the ReplaceFileDialog for non-EMPTY drops.
    },
  });

  return (
    <div className="workspace-shell">
      <header className="workspace-shell__header">Studio Sound</header>
      <main className="workspace-shell__main">
        {status === 'EMPTY' ? (
          <EmptyState isDragOver={isDragOver} />
        ) : (
          <ActiveFileCard />
        )}
      </main>
      <footer className="workspace-shell__footer" />
    </div>
  );
}
