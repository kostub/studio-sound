import type { JSX } from 'react';

export interface EmptyStateProps {
  onDrop: (path: string) => void;
  isDragOver: boolean;
}

export function EmptyState({ isDragOver }: EmptyStateProps): JSX.Element {
  return (
    <div
      className={`empty-state${isDragOver ? ' drag-over' : ''}`}
      role="region"
      aria-label="Drop a video file to begin"
    >
      <div className="empty-state__icon" aria-hidden="true">
        📼
      </div>
      <h2 className="empty-state__title">Drop a video here to begin</h2>
      <p className="empty-state__formats">Supports MP4, MOV, WebM, and MKV.</p>
      <p className="empty-state__privacy">
        Your files never leave your device — everything stays local.
      </p>
    </div>
  );
}
