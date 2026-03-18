/*
 * Real-Time Event Stream
 *
 * Keeps the forum UI live-updated by maintaining a persistent server connection
 * for push events (new messages, posts, notifications).
 *
 * Shows a persistent error banner after 3 failed reconnect attempts so users
 * know live updates are degraded.
 */

import { getAccessToken } from './auth.js';

export function connectSSE(url, onMessage, requireAuth = false) {
  let es = null;
  let reconnectTimer = null;
  let cancelled = false;
  let reconnectAttempts = 0;

  async function connect() {
    if (cancelled) return;

    let fullUrl = url;
    if (requireAuth) {
      const token = await getAccessToken();
      if (!token) {
        reconnectTimer = setTimeout(connect, 3000);
        return;
      }
      const sep = fullUrl.includes('?') ? '&' : '?';
      fullUrl = `${fullUrl}${sep}access_token=${encodeURIComponent(token)}`;
    }

    es = new EventSource(fullUrl);

    es.onopen = () => {
      if (reconnectAttempts > 0) {
        import('./toast.js').then(({ hideErrorBanner }) => hideErrorBanner());
      }
      reconnectAttempts = 0;
    };

    es.onmessage = event => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch {
        // Ignore heartbeats
      }
    };

    es.onerror = () => {
      if (cancelled) return;
      es?.close();
      es = null;
      reconnectAttempts++;
      if (reconnectAttempts >= 3) {
        import('./toast.js').then(({ toast }) => {
          toast.error('Live updates unavailable — reconnecting in background');
        });
      }
      reconnectTimer = setTimeout(connect, 3000);
    };
  }

  connect();

  return () => {
    cancelled = true;
    es?.close();
    if (reconnectTimer) clearTimeout(reconnectTimer);
  };
}
