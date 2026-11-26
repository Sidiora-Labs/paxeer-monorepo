import { HttpError, type StreamTransportV2 } from '@kindle/sdk';

function waitForSnapshot(retryAt: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const cleanup = () => {
      if (timer !== undefined) clearTimeout(timer);
      signal.removeEventListener('abort', abort);
      if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', resume);
    };
    const abort = () => { cleanup(); reject(signal.reason); };
    const resume = () => {
      if (timer !== undefined) clearTimeout(timer);
      timer = undefined;
      if (signal.aborted) { abort(); return; }
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
      const remaining = retryAt - Date.now();
      if (remaining > 0) timer = setTimeout(resume, Math.min(remaining, 2 ** 31 - 1));
      else { cleanup(); resolve(); }
    };
    signal.addEventListener('abort', abort, { once: true });
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', resume);
    resume();
  });
}

export function beforeFirstFrameTransport(transport: StreamTransportV2, onWaiting?: () => void): StreamTransportV2 {
  let retryAt = 0;
  return {
    ...transport,
    snapshot: async (topics, signal) => {
      for (;;) {
        signal.throwIfAborted();
        if (retryAt > 0) await waitForSnapshot(retryAt, signal);
        signal.throwIfAborted();
        try {
          const snapshot = await transport.snapshot(topics, signal);
          retryAt = 0;
          return snapshot;
        } catch (error) {
          signal.throwIfAborted();
          if (!(error instanceof HttpError) || error.status !== 404 || error.code !== 'TOPIC_NOT_STARTED') throw error;
          onWaiting?.();
          retryAt = Date.now() + Math.max(5000, error.envelope?.retryAfterMs ?? 0);
        }
      }
    },
  };
}
