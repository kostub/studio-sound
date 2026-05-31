import type { JSX } from 'react';
import { useEffect, useState } from 'react';
import type { ProbeResult } from '../ipc/generated/media.probe';

interface Props {
  open: boolean;
  onClose: () => void;
  result: ProbeResult;
}

export function DiagnosticsDrawer({ open, onClose, result }: Props): JSX.Element | null {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, onClose]);

  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(t);
  }, [copied]);

  if (!open) return null;

  const handleCopy = async () => {
    await navigator.clipboard.writeText(JSON.stringify(result, null, 2));
    setCopied(true);
  };

  return (
    <div className="diagnostics-overlay">
      <div
        data-testid="diagnostics-backdrop"
        className="diagnostics-backdrop"
        onClick={onClose}
      />
      <aside className="diagnostics-drawer" role="dialog" aria-modal="true" aria-label="Diagnostics">
        <header className="diagnostics-drawer__header">
          <h2>Diagnostics</h2>
          <button type="button" onClick={onClose} aria-label="Close diagnostics">
            ×
          </button>
        </header>
        <section className="diagnostics-drawer__body">
          <dl>
            <div>
              <dt>id</dt>
              <dd>{result.id}</dd>
            </div>
            <div>
              <dt>path</dt>
              <dd>{result.path}</dd>
            </div>
            <div>
              <dt>container</dt>
              <dd>{result.container.format}</dd>
            </div>
            {result.durationSeconds !== undefined && (
              <div>
                <dt>durationSeconds</dt>
                <dd>{result.durationSeconds}</dd>
              </div>
            )}
            {result.video && (
              <div>
                <dt>video</dt>
                <dd>
                  {result.video.codec} · {result.video.width}×{result.video.height} ·{' '}
                  {result.video.fps}fps
                </dd>
              </div>
            )}
            {result.audio && (
              <>
                <div>
                  <dt>audio</dt>
                  <dd>
                    {result.audio.codec} · {result.audio.sampleRate}Hz · {result.audio.channels}ch
                  </dd>
                </div>
                {result.audio.tracks.length > 0 && (
                  <div>
                    <dt>tracks</dt>
                    <dd>
                      <ul>
                        {result.audio.tracks.map((track) => (
                          <li key={track.index}>
                            #{track.index} {track.codec} {track.channels}ch {track.sampleRate}Hz
                            {track.title ? ` (${track.title})` : ''}
                            {track.isDefault ? ' [default]' : ''}
                          </li>
                        ))}
                      </ul>
                    </dd>
                  </div>
                )}
              </>
            )}
            <div>
              <dt>compatibility</dt>
              <dd>
                {result.compatibility.supported
                  ? 'supported'
                  : `unsupported${result.compatibility.issues.length > 0 ? `: ${result.compatibility.issues.join(', ')}` : ''}`}
              </dd>
            </div>
          </dl>
        </section>
        <footer className="diagnostics-drawer__footer">
          <button type="button" onClick={() => void handleCopy()}>
            Copy diagnostics
          </button>
          {copied && <span role="status">Copied</span>}
        </footer>
      </aside>
    </div>
  );
}
