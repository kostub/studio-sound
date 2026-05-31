import type { JSX } from 'react';
import { useState } from 'react';
import { useWorkspace } from '../state/workspace';
import { useDropTarget } from '../hooks/useDropTarget';
import { basename } from '../utils/path';
import { EmptyState } from './EmptyState';
import { ActiveFileCard } from './ActiveFileCard';
import { ReplaceFileDialog } from './ReplaceFileDialog';

export function WorkspaceShell(): JSX.Element {
  const status = useWorkspace((s) => s.status);
  const filename = useWorkspace((s) => s.path);
  const loadFile = useWorkspace((s) => s.loadFile);
  const replaceFile = useWorkspace((s) => s.replaceFile);
  const [incoming, setIncoming] = useState<string | null>(null);

  const { isDragOver } = useDropTarget({
    onPath: (path) => {
      if (status === 'EMPTY') {
        void loadFile(path);
      } else {
        setIncoming(path);
      }
    },
  });

  const currentFilename = filename ? basename(filename) : null;

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
      <ReplaceFileDialog
        incomingPath={incoming}
        currentFilename={currentFilename}
        onConfirm={(path) => {
          void replaceFile(path);
          setIncoming(null);
        }}
        onCancel={() => setIncoming(null)}
      />
    </div>
  );
}
