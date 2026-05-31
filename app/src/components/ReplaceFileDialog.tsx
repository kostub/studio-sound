import type { JSX } from 'react';
import { useEffect } from 'react';
import { basename } from '../utils/path';

interface Props {
  incomingPath: string | null;
  currentFilename: string | null;
  onConfirm: (path: string) => void;
  onCancel: () => void;
}

export function ReplaceFileDialog({
  incomingPath,
  currentFilename,
  onConfirm,
  onCancel,
}: Props): JSX.Element | null {
  useEffect(() => {
    if (!incomingPath) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [incomingPath, onCancel]);

  if (!incomingPath) return null;

  return (
    <div className="dialog-overlay">
      <div className="dialog-backdrop" onClick={onCancel} />
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="replace-title">
        <h2 id="replace-title">Replace current file?</h2>
        <p>
          Replace <strong>{currentFilename ?? 'current file'}</strong> with{' '}
          <strong>{basename(incomingPath)}</strong>?
        </p>
        <div className="dialog__actions">
          <button type="button" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            onClick={() => onConfirm(incomingPath)}
            className="primary"
          >
            Replace
          </button>
        </div>
      </div>
    </div>
  );
}
