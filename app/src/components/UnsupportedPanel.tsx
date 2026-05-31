import type { JSX } from 'react';

interface Props {
  issues: string[];
  onReplace: () => void;
  onOpenDetails: () => void;
}

export function UnsupportedPanel({ issues, onReplace, onOpenDetails }: Props): JSX.Element {
  return (
    <section className="unsupported-panel" role="region" aria-label="Unsupported file">
      <h2>Unsupported file</h2>
      <p>Studio Sound can read this file&apos;s metadata, but cannot process it for editing.</p>
      {issues.length > 0 && (
        <ul className="unsupported-panel__issues">
          {issues.map((issue, i) => (
            <li key={i}>{issue}</li>
          ))}
        </ul>
      )}
      <div className="unsupported-panel__actions">
        <button type="button" onClick={onReplace} className="primary">
          Replace file
        </button>
        <button type="button" onClick={onOpenDetails}>
          Details
        </button>
      </div>
    </section>
  );
}
