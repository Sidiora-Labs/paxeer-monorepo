import { HttpBoundaryError, timeoutSignal } from '@/server/http';

export async function fetchChatStream(
  url: string,
  init: RequestInit,
  parent: AbortSignal
): Promise<ReadableStream<Uint8Array>> {
  const timeout = timeoutSignal(parent, 20_000);
  const transport = new AbortController();
  let terminate: ((reason: unknown) => void) | undefined;
  const abort = () => {
    transport.abort(timeout.signal.reason);
    terminate?.(timeout.signal.reason);
  };
  const dispose = () => {
    timeout.dispose();
    timeout.signal.removeEventListener('abort', abort);
  };
  timeout.signal.addEventListener('abort', abort, { once: true });
  if (timeout.signal.aborted) abort();

  try {
    const upstream = await fetch(url, { ...init, signal: transport.signal });
    if (!upstream.ok || !upstream.body) {
      throw new HttpBoundaryError(
        502,
        'CHAT_UPSTREAM_FAILED',
        'The assistant is currently unavailable'
      );
    }
    if (!upstream.headers.get('content-type')?.toLowerCase().startsWith('text/event-stream')) {
      throw new HttpBoundaryError(
        502,
        'CHAT_UPSTREAM_INVALID',
        'The assistant returned an invalid response'
      );
    }

    const reader = upstream.body.getReader();
    let finished = false;
    let total = 0;
    const finish = () => {
      finished = true;
      dispose();
      reader.releaseLock();
    };
    return new ReadableStream<Uint8Array>({
      start(controller) {
        terminate = (reason) => {
          if (finished) return;
          controller.error(reason);
          finish();
        };
        if (timeout.signal.aborted) terminate(timeout.signal.reason);
      },
      async pull(controller) {
        try {
          const next = await reader.read();
          if (finished) return;
          if (next.done) {
            controller.close();
            finish();
            return;
          }
          total += next.value.byteLength;
          if (total > 262_144) {
            throw new Error('Response stream exceeded the size limit');
          }
          controller.enqueue(next.value);
        } catch (error) {
          if (finished) return;
          controller.error(error);
          transport.abort(error);
          finish();
        }
      },
      async cancel(reason) {
        if (finished) return;
        finished = true;
        try {
          const cancellation = reader.cancel(reason);
          transport.abort(reason);
          await cancellation;
        } finally {
          dispose();
          reader.releaseLock();
        }
      },
    });
  } catch (error) {
    transport.abort(error);
    dispose();
    throw error;
  }
}
