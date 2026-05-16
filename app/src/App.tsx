import type { JSX } from 'react';
import { useEffect, useState } from 'react';

import { Diagnostics } from './components/Diagnostics';

export function App(): JSX.Element {
  // Diagnostics panel is only available in dev builds and hidden by default.
  const [showDiagnostics, setShowDiagnostics] = useState(false);

  useEffect(() => {
    if (!import.meta.env.DEV) return;

    function handleKeyDown(e: KeyboardEvent): void {
      // Ctrl+Shift+D (Windows/Linux) or Cmd+Shift+D (macOS)
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'D') {
        e.preventDefault();
        setShowDiagnostics((prev) => !prev);
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  return (
    <main className="app-shell">
      <section className="status-panel" aria-labelledby="app-title">
        <p className="eyebrow">Desktop bootstrap</p>
        <h1 id="app-title">Studio Sound App</h1>
        <p className="status-message">Phase 0 Bootstrap Successful</p>
      </section>

      {import.meta.env.DEV && showDiagnostics && <Diagnostics />}
    </main>
  );
}
