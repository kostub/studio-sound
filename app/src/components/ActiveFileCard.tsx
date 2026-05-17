import type { JSX } from 'react';
import { useWorkspace } from '../state/workspace';

function dotClassFor(status: string, supported?: boolean): string {
  switch (status) {
    case 'PROBING':
    case 'RETRYING':
    case 'FILE_LOADED':
      return 'status-dot spinner';
    case 'READY':
      return supported ? 'status-dot green' : 'status-dot yellow';
    case 'ERROR':
      return 'status-dot red';
    default:
      return 'status-dot gray';
  }
}

function basename(path?: string): string {
  if (!path) return '';
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return i >= 0 ? path.slice(i + 1) : path;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function formatDuration(s?: number): string {
  if (s == null) return 'Unknown duration';
  const mm = Math.floor(s / 60);
  const ss = Math.floor(s % 60).toString().padStart(2, '0');
  return `${mm}:${ss}`;
}

function statusLabel(status: string, supported?: boolean): string {
  switch (status) {
    case 'PROBING':
      return 'Probing…';
    case 'RETRYING':
      return 'Retrying…';
    case 'FILE_LOADED':
      return 'Loading…';
    case 'READY':
      return supported ? 'Ready' : 'Unsupported';
    case 'ERROR':
      return 'Error';
    default:
      return '';
  }
}

export function ActiveFileCard(): JSX.Element {
  const status = useWorkspace((s) => s.status);
  const path = useWorkspace((s) => s.path);
  const result = useWorkspace((s) => s.result);
  const clearFile = useWorkspace((s) => s.clearFile);
  const retry = useWorkspace((s) => s.retry);

  const supported = result?.compatibility?.supported;

  return (
    <section className="active-file-card" aria-label="Active file">
      <div className="active-file-card__header">
        <div className="active-file-card__thumb" aria-hidden="true">
          🎬
        </div>
        <div className="active-file-card__title">
          <div className="active-file-card__filename">{basename(path)}</div>
          <div className="active-file-card__status">
            <span data-testid="status-dot" className={dotClassFor(status, supported)} />
            <span className="active-file-card__status-label">
              {statusLabel(status, supported)}
            </span>
          </div>
        </div>
      </div>
      {result && (
        <dl className="active-file-card__meta">
          <div>
            <dt>Duration</dt>
            <dd>{formatDuration(result.durationSeconds ?? undefined)}</dd>
          </div>
          <div>
            <dt>Size</dt>
            <dd>{formatBytes(result.sizeBytes)}</dd>
          </div>
          <div>
            <dt>Container</dt>
            <dd>{result.container.format}</dd>
          </div>
          {result.video && (
            <div>
              <dt>Video</dt>
              <dd>
                {result.video.codec} {result.video.width}×{result.video.height} @{' '}
                {result.video.fps.toFixed(2)}fps
              </dd>
            </div>
          )}
          {result.audio && (
            <div>
              <dt>Audio</dt>
              <dd>
                {result.audio.codec} {result.audio.channels}ch {result.audio.sampleRate}Hz
              </dd>
            </div>
          )}
        </dl>
      )}
      <div className="active-file-card__actions">
        {status === 'ERROR' && (
          <button type="button" onClick={() => void retry()}>
            Retry
          </button>
        )}
        <button type="button" onClick={clearFile}>
          Remove
        </button>
      </div>
    </section>
  );
}
