import { getEventListeners, once } from 'node:events';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { expect, test } from 'vitest';
import { fetchChatStream } from './stream';

const transcript = 'data: {"choices":[{"delta":{"content":"Hello"}}]}\n\n';

async function serveSse(produce: (request: IncomingMessage, response: ServerResponse) => void) {
  let closed = false;
  const requests: Array<{ method: string | undefined; path: string | undefined; body: string }> =
    [];
  const server = createServer(async (request, response) => {
    response.on('close', () => {
      closed = true;
    });
    let body = '';
    for await (const chunk of request) body += chunk.toString();
    requests.push({ method: request.method, path: request.url, body });
    response.setHeader('Content-Type', 'text/event-stream');
    produce(request, response);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
  return {
    url: `http://127.0.0.1:${address.port}/v1/chat/completions`,
    requests,
    isClosed: () => closed,
    async close() {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      });
    },
  };
}

function openChat(url: string, signal: AbortSignal) {
  return fetchChatStream(
    url,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ stream: true, messages: [{ role: 'user', content: 'Hello' }] }),
    },
    signal
  );
}

async function chat_stream_timeout_aborts_stalled_sse_after_headers() {
  const fixture = await serveSse((_request, response) => {
    response.write(transcript);
  });
  const parent = new AbortController();
  const startedAt = Date.now();
  try {
    const stream = await openChat(fixture.url, parent.signal);
    const reader = stream.getReader();
    const first = await reader.read();
    expect(new TextDecoder().decode(first.value)).toBe(transcript);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(1);
    await expect(reader.read()).rejects.toMatchObject({ name: 'AbortError' });
    expect(Date.now() - startedAt).toBeGreaterThanOrEqual(20_000);
    await expect.poll(fixture.isClosed).toBe(true);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
    expect(fixture.requests).toEqual([
      {
        method: 'POST',
        path: '/v1/chat/completions',
        body: JSON.stringify({ stream: true, messages: [{ role: 'user', content: 'Hello' }] }),
      },
    ]);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_cancel_aborts_upstream_with_a_pending_read() {
  const fixture = await serveSse((_request, response) => {
    response.write(transcript);
  });
  const parent = new AbortController();
  try {
    const reader = (await openChat(fixture.url, parent.signal)).getReader();
    expect(new TextDecoder().decode((await reader.read()).value)).toBe(transcript);
    const pending = expect(reader.read()).resolves.toEqual({ done: true, value: undefined });
    await reader.cancel('client stopped reading');
    await pending;
    await expect.poll(fixture.isClosed).toBe(true);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_cancel_without_reading_aborts_upstream() {
  const fixture = await serveSse((_request, response) => {
    response.write(transcript);
  });
  const parent = new AbortController();
  try {
    const stream = await openChat(fixture.url, parent.signal);
    await stream.cancel('client disconnected before reading');
    await expect.poll(fixture.isClosed).toBe(true);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_request_abort_errors_consumer_and_aborts_upstream() {
  const fixture = await serveSse((_request, response) => {
    response.write(transcript);
  });
  const parent = new AbortController();
  try {
    const reader = (await openChat(fixture.url, parent.signal)).getReader();
    await reader.read();
    const reason = new Error('request disconnected');
    const pending = expect(reader.read()).rejects.toBe(reason);
    parent.abort(reason);
    await pending;
    await expect.poll(fixture.isClosed).toBe(true);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_byte_limit_aborts_upstream_before_excess_bytes_escape() {
  let upstream: ServerResponse | undefined;
  const limit = 262_144;
  const fixture = await serveSse((_request, response) => {
    upstream = response;
    response.write(Buffer.alloc(limit, 97));
  });
  const parent = new AbortController();
  try {
    const reader = (await openChat(fixture.url, parent.signal)).getReader();
    let received = 0;
    while (received < limit) {
      const chunk = await reader.read();
      expect(chunk.done).toBe(false);
      if (!chunk.value) throw new Error('Expected response bytes');
      expect(chunk.value.every((byte) => byte === 97)).toBe(true);
      received += chunk.value.byteLength;
    }
    expect(received).toBe(limit);
    if (!upstream) throw new Error('Expected an upstream response');
    upstream.write('b');
    await expect(reader.read()).rejects.toThrow('Response stream exceeded the size limit');
    await expect.poll(fixture.isClosed).toBe(true);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_close_preserves_the_exact_byte_limit_and_disposes_deadline() {
  const payload = Buffer.alloc(262_144, 97);
  const fixture = await serveSse((_request, response) => {
    response.end(payload);
  });
  const parent = new AbortController();
  try {
    const response = new Response(await openChat(fixture.url, parent.signal));
    expect(Buffer.from(await response.arrayBuffer())).toEqual(payload);
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
    parent.abort(new Error('after stream close'));
    expect(response.bodyUsed).toBe(true);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_transport_error_rejects_consumer_and_disposes_deadline() {
  let upstream: ServerResponse | undefined;
  const fixture = await serveSse((_request, response) => {
    upstream = response;
    response.write(transcript);
  });
  const parent = new AbortController();
  try {
    const reader = (await openChat(fixture.url, parent.signal)).getReader();
    expect(new TextDecoder().decode((await reader.read()).value)).toBe(transcript);
    const pending = expect(reader.read()).rejects.toThrow();
    if (!upstream) throw new Error('Expected an upstream response');
    upstream.destroy();
    await pending;
    expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
  } finally {
    parent.abort();
    await fixture.close();
  }
}

async function chat_stream_invalid_upstream_aborts_body_and_disposes_deadline() {
  for (const status of [200, 503]) {
    const fixture = await serveSse((_request, response) => {
      response.statusCode = status;
      response.setHeader('Content-Type', 'application/json');
      response.write('{}');
    });
    const parent = new AbortController();
    try {
      await expect(openChat(fixture.url, parent.signal)).rejects.toMatchObject({
        status: 502,
        code: status === 200 ? 'CHAT_UPSTREAM_INVALID' : 'CHAT_UPSTREAM_FAILED',
      });
      await expect.poll(fixture.isClosed).toBe(true);
      expect(getEventListeners(parent.signal, 'abort')).toHaveLength(0);
    } finally {
      parent.abort();
      await fixture.close();
    }
  }
}

test(
  chat_stream_timeout_aborts_stalled_sse_after_headers,
  chat_stream_timeout_aborts_stalled_sse_after_headers,
  25_000
);
test(
  chat_stream_cancel_aborts_upstream_with_a_pending_read,
  chat_stream_cancel_aborts_upstream_with_a_pending_read
);
test(
  chat_stream_cancel_without_reading_aborts_upstream,
  chat_stream_cancel_without_reading_aborts_upstream
);
test(
  chat_stream_request_abort_errors_consumer_and_aborts_upstream,
  chat_stream_request_abort_errors_consumer_and_aborts_upstream
);
test(
  chat_stream_byte_limit_aborts_upstream_before_excess_bytes_escape,
  chat_stream_byte_limit_aborts_upstream_before_excess_bytes_escape
);
test(
  chat_stream_close_preserves_the_exact_byte_limit_and_disposes_deadline,
  chat_stream_close_preserves_the_exact_byte_limit_and_disposes_deadline
);
test(
  chat_stream_transport_error_rejects_consumer_and_disposes_deadline,
  chat_stream_transport_error_rejects_consumer_and_disposes_deadline
);
test(
  chat_stream_invalid_upstream_aborts_body_and_disposes_deadline,
  chat_stream_invalid_upstream_aborts_body_and_disposes_deadline
);
