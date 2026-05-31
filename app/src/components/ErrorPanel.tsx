import type { JSX } from 'react';
import type { IpcError } from '../ipc/client';

interface Props {
  error: IpcError;
  onRetry: () => void;
  onOpenDetails: () => void;
}

interface Copy {
  headline: string;
  body: string;
  retry: boolean;
}

function copyFor(code: string): Copy {
  switch (code) {
    case 'FILE_NOT_FOUND':
      return {
        headline: 'File not found',
        body: 'The file you dropped is no longer at that location. It may have been moved or deleted.',
        retry: true,
      };
    case 'ACCESS_DENIED':
      return {
        headline: "Can't read this file",
        body: 'Studio Sound does not have permission to read this file. Check the file permissions and try again.',
        retry: true,
      };
    case 'CORRUPT_MEDIA':
      return {
        headline: 'Corrupt or unreadable media',
        body: 'This file appears to be truncated or corrupt. Try re-exporting or re-downloading it.',
        retry: true,
      };
    case 'FFPROBE_FAILURE':
      return {
        headline: "Couldn't analyze this file",
        body: 'ffprobe exited with an error. Open Details to see the stderr tail.',
        retry: true,
      };
    case 'FFPROBE_MISSING':
      return {
        headline: 'ffprobe is missing — reinstall required',
        body: 'The bundled ffprobe binary could not be located. Reinstall Studio Sound to restore it.',
        retry: false,
      };
    default:
      return {
        headline: 'Something went wrong',
        body: 'An unexpected error occurred while analyzing this file.',
        retry: true,
      };
  }
}

export function ErrorPanel({ error, onRetry, onOpenDetails }: Props): JSX.Element {
  const c = copyFor(error.code);
  return (
    <section className="error-panel" role="region" aria-label="Error">
      <h2>{c.headline}</h2>
      <p>{c.body}</p>
      <div className="error-panel__actions">
        {c.retry && (
          <button type="button" onClick={onRetry}>
            Retry
          </button>
        )}
        <button type="button" onClick={onOpenDetails}>
          Details
        </button>
      </div>
    </section>
  );
}
