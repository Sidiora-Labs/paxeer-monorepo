import type { RequestOptions, SdkClient } from '@kindle/sdk';
import { marketResourceKey, marketResourcePaceKey, marketRetryPace } from '@/core/retry-pace';

const clients = new WeakMap<SdkClient, SdkClient>();
let sessionScope = 0;
const readMethods = new Set(['request', 'requestV2', 'requestV3', 'requestV1Generated', 'requestV2Validated']);

export function pacedMarketClient(client: SdkClient): SdkClient {
  const existing = clients.get(client);
  if (existing) return existing;
  const scope = `session-${++sessionScope}`;
  let sessionCredential: string | undefined;
  let privateScope = scope;
  const proxy = new Proxy(client, {
    get(target, property) {
      const method = Reflect.get(target, property, target);
      if (typeof property !== 'string' || !readMethods.has(property)) return typeof method === 'function' ? method.bind(target) : method;
      return (path: string, response: unknown, options: RequestOptions = {}) => {
        if ((options.method ?? 'GET') !== 'GET' && !(options.method === 'POST' && path === '/v2/pools/stats:batch')) return Reflect.apply(method, target, [path, response, options]);
        const origin = target.config[options.service === 'media' ? 'mediaOrigin' : options.service === 'identity' ? 'identityOrigin' : 'apiOrigin'];
        const url = new URL(path, origin);
        const body = options.serializedBody ?? options.body;
        const credential = options.authenticated || options.cookieSession ? target.config.session?.mode === 'bearer' ? target.config.session.token() : target.config.session?.csrfToken() : undefined;
        if (credential !== sessionCredential) { sessionCredential = credential; privateScope = `session-${++sessionScope}`; }
        const session = options.authenticated || options.cookieSession ? privateScope : '';
        const key = marketResourceKey(url, body, session);
        return marketRetryPace.read(key, async signal => Reflect.apply(method, target, [path, response, { ...options, signal, idempotentRead: false }]), options.signal, marketResourcePaceKey(url, body, session));
      };
    },
  });
  clients.set(client, proxy);
  clients.set(proxy, proxy);
  return proxy;
}
