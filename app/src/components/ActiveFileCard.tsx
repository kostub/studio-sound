import type { JSX } from 'react';
import { useState } from 'react';
import { useWorkspace } from '../state/workspace';
import { basename } from '../utils/path';
import { DiagnosticsDrawer } from './DiagnosticsDrawer';
import { ErrorPanel } from './ErrorPanel';
import { UnsupportedPanel } from './UnsupportedPanel';

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
      // EMPTY/REMOVED never reach this card (REMOVED is a transient state in
      // replaceFile, synchronously overwritten by loadFile before render).
      return 'status-dot gray';
  }
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

// Drop trailing zeros so common frame rates render as "30fps", not "30.00fps".
function formatFps(fps: number): string {
  return `${parseFloat(fps.toFixed(2))}fps`;
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
      // EMPTY/REMOVED never reach this card (see dotClassFor).
      return '';
  }
}

export function ActiveFileCard(): JSX.Element {
  const status = useWorkspace((s) => s.status);
  const path = useWorkspace((s) => s.path);
  const result = useWorkspace((s) => s.result);
  const error = useWorkspace((s) => s.error);
  const clearFile = useWorkspace((s) => s.clearFile);
  const retry = useWorkspace((s) => s.retry);
  const [diagOpen, setDiagOpen] = useState(false);

  const supported = result?.compatibility?.supported;
  const showError = status === 'ERROR' && error;
  const showUnsupported = status === 'READY' && result && !result.compatibility.supported;

  return (
    <>
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
        {result && !showError && !showUnsupported && (
          <dl className="active-file-card__meta">
            <div>
              <dt>Duration</dt>
              <dd>{formatDuration(result.durationSeconds)}</dd>
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
                  {formatFps(result.video.fps)}
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
        {showError && (
          <ErrorPanel
            error={error}
            onRetry={() => void retry()}
            onOpenDetails={() => setDiagOpen(true)}
          />
        )}
        {showUnsupported && (
          <UnsupportedPanel
            issues={result.compatibility.issues}
            onReplace={clearFile}
            onOpenDetails={() => setDiagOpen(true)}
          />
        )}
        <div className="active-file-card__actions">
          {result && !showError && !showUnsupported && (
            <button type="button" onClick={() => setDiagOpen(true)} aria-label="Details">
              Details
            </button>
          )}
          <button type="button" onClick={clearFile}>
            Remove
          </button>
        </div>
      </section>
      {result && (
        <DiagnosticsDrawer open={diagOpen} onClose={() => setDiagOpen(false)} result={result} />
      )}
    </>
  );
}
