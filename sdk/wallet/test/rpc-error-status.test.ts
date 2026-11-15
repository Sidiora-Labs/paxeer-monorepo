import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { JsonRpcClient, JsonRpcError, JsonRpcTransportError } from '../src/rpc.js';

async function withRpcServer(work: (url: string) => Promise<void>): Promise<void> {
  const server = createServer(async (request, response) => {
    let text = '';
    for await (const chunk of request) text += chunk;
    const payload = JSON.parse(text);
    const requests: { id: number }[] = Array.isArray(payload) ? payload : [payload];
    const [status, variant] = request.url!.slice(1).split('/');
    const answers = requests.map(({ id }) => ({ jsonrpc: '2.0', id, result: 'answer' }));
    const first = answers[0]!;
    let body: unknown;
    switch (variant) {
      case 'object': body = { message: 'service unavailable' }; break;
      case 'array': body = []; break;
      case 'scalar': body = null; break;
      case 'wrong-id': body = { ...first, id: first.id + 100 }; break;
      case 'wrong-version': body = { ...first, jsonrpc: '1.0' }; break;
      case 'neither': body = { jsonrpc: '2.0', id: first.id }; break;
      case 'both': body = { ...first, error: { code: -32603, message: 'unavailable' } }; break;
      case 'malformed-error':
        body = { jsonrpc: '2.0', id: first.id, error: { code: 'unavailable' } };
        break;
      case 'batch-wrong-id': body = [first, { ...answers[1], id: first.id + 100 }]; break;
      case 'batch-duplicate': body = [first, first]; break;
      case 'batch-missing': body = [first]; break;
      case 'batch-extra': body = [...answers, { ...first, id: first.id + 100 }]; break;
      case 'batch-malformed': body = [first, { id: requests[1]!.id }]; break;
      case 'batch-malformed-error':
        body = [first, { jsonrpc: '2.0', id: requests[1]!.id, error: { message: 'unavailable' } }];
        break;
      case 'batch-result': body = answers.toReversed(); break;
      case 'error':
        body = { jsonrpc: '2.0', id: first.id, error: { code: -32603, message: 'unavailable' } };
        break;
      case 'batch-error':
        body = requests.toReversed().map(({ id }) => ({
          jsonrpc: '2.0', id, error: { code: -32603, message: 'unavailable' },
        }));
        break;
      default: body = first;
    }
    response.writeHead(Number(status), { 'content-type': 'application/json' });
    response.end(variant === 'text' ? 'service unavailable' : JSON.stringify(body));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    await work(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
  }
}

async function unsuccessful_http_responses_report_the_status_for_invalid_rpc_answers(): Promise<void> {
  await withRpcServer(async (url) => {
    const calls = [{ method: 'first', params: [] }, { method: 'second', params: [] }];
    for (const status of [400, 502]) {
      const invalid = [
        'object', 'array', 'scalar', 'wrong-id', 'wrong-version', 'neither', 'both', 'malformed-error',
      ];
      for (const variant of invalid) {
        const client = new JsonRpcClient({ url: `${url}/${status}/${variant}` });
        await expect(client.call('first')).rejects.toMatchObject({
          name: 'JsonRpcTransportError', message: `HTTP ${status}`, status,
        });
      }
      for (const variant of [...invalid, 'result', 'batch-wrong-id', 'batch-duplicate',
        'batch-missing', 'batch-extra', 'batch-malformed', 'batch-malformed-error']) {
        const client = new JsonRpcClient({ url: `${url}/${status}/${variant}` });
        await expect(client.batch(calls)).rejects.toMatchObject({
          name: 'JsonRpcTransportError', message: `HTTP ${status}`, status,
        });
      }
      await expect(new JsonRpcClient({ url: `${url}/${status}/text` }).call('first'))
        .rejects.toEqual(new JsonRpcTransportError(`response is not JSON (HTTP ${status})`, status));
      await expect(new JsonRpcClient({ url: `${url}/${status}/result` }).call('first'))
        .resolves.toBe('answer');
      await expect(new JsonRpcClient({ url: `${url}/${status}/error` }).call('first'))
        .rejects.toEqual(new JsonRpcError({ code: -32603, message: 'unavailable' }));
      await expect(new JsonRpcClient({ url: `${url}/${status}/batch-result` }).batch(calls))
        .resolves.toEqual([{ ok: true, result: 'answer' }, { ok: true, result: 'answer' }]);
      await expect(new JsonRpcClient({ url: `${url}/${status}/batch-error` }).batch(calls))
        .resolves.toEqual(calls.map(() => ({
          ok: false, error: { code: -32603, message: 'unavailable' },
        })));
    }
  });
}

async function successful_http_responses_keep_the_wrong_request_id_error(): Promise<void> {
  await withRpcServer(async (url) => {
    const client = new JsonRpcClient({ url: `${url}/200/wrong-id` });
    await expect(client.call('first')).rejects.toMatchObject({
      name: 'JsonRpcTransportError',
      message: 'response does not answer the request id',
      status: null,
    });
  });
}

test(unsuccessful_http_responses_report_the_status_for_invalid_rpc_answers,
  unsuccessful_http_responses_report_the_status_for_invalid_rpc_answers);
test(successful_http_responses_keep_the_wrong_request_id_error,
  successful_http_responses_keep_the_wrong_request_id_error);
