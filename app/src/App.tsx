import type { JSX } from 'react';

export function App(): JSX.Element {
  return (
    <main className="app-shell">
      <section className="status-panel" aria-labelledby="app-title">
        <p className="eyebrow">Desktop bootstrap</p>
        <h1 id="app-title">Studio Sound App</h1>
        <p className="status-message">Phase 0 Bootstrap Successful</p>
      </section>
    </main>
  );
}
