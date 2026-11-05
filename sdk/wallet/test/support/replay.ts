import { isDeepStrictEqual } from 'node:util';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';

export interface Exchange {
  path: string;
  method: string;
  request?: unknown;
  authorization?: string;
  status?: number;
  response: unknown;
}
export interface SeenRequest {
  path: string;
  method: string;
  body: unknown;
  authorization?: string;
}

// This is a transcript transport. It has no signer, transaction execution,
// receipt construction, or producer journal to substitute for the node.
export async function replay(exchanges: readonly Exchange[]) {
  const seen: SeenRequest[] = [];
  const errors: string[] = [];
  let server: Server;
  const start = async () => {
    server = createServer(async (req, res) => {
      try {
        let text = '';
        for await (const chunk of req) text += chunk;
        const body = text ? JSON.parse(text) : undefined;
        seen.push({
          path: req.url!,
          method: req.method!,
          body,
          authorization: req.headers.authorization,
        });
        const isRpc = body?.jsonrpc === '2.0';
        const request = isRpc ? { method: body.method, params: body.params } : body;
        const exchange = exchanges.find(
          (exchange) =>
            exchange.path === req.url &&
            exchange.method === req.method &&
            isDeepStrictEqual(exchange.request, request) &&
            exchange.authorization === req.headers.authorization
        );
        if (!exchange)
          throw new Error(`unrecorded request ${req.method} ${req.url} ${JSON.stringify(request)}`);
        res.writeHead(exchange.status ?? 200, { 'content-type': 'application/json' });
        const answer = isRpc
          ? { ...(exchange.response as object), id: body.id }
          : exchange.response;
        res.end(JSON.stringify(answer));
      } catch (error) {
        errors.push(String(error));
        res.writeHead(500, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: String(error) }));
      }
    });
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  };
  const stop = () =>
    new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  let url = await start();
  return {
    get url() {
      return url;
    },
    seen,
    errors,
    close: stop,
    async restart() {
      await stop();
      url = await start();
    },
  };
}

export function rpcExchange(
  method: string,
  params: unknown[],
  result: unknown,
  path = '/node'
): Exchange {
  return {
    path,
    method: 'POST',
    request: { method, params },
    response: { jsonrpc: '2.0', result },
  };
}
