import React from 'react';
import ReactDOM from 'react-dom/client';

import { App } from './App';
import './styles.css';
import './styles/tokens.css';

export function renderApp(): void {
  const root = document.getElementById('root');

  if (!root) {
    throw new Error('Root element not found');
  }

  ReactDOM.createRoot(root).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
}

renderApp();
