/** Shared, validated node-root RPC selection for public and wallet transports. */
export const DEFAULT_PAXEER_RPC_URL = 'https://api5.mainnet-beta.paxeer.network';

const RETIRED_RPC_HOSTS = new Set(['public-mainnet.rpcpaxeer.online', 'node.hyperpaxeer.com']);
const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);

export class RpcConfigurationError extends Error {
  constructor(readonly variable: string) {
    super(
      `${variable} must name an approved HTTPS node root without credentials, query or fragment`,
    );
    this.name = 'RpcConfigurationError';
  }
}

function checkedRpc(variable: string, value: string): string {
  const invalid = () => new RpcConfigurationError(variable);
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw invalid();
  }
  const localQualification =
    process.env.NODE_ENV !== 'production' && LOOPBACK_HOSTS.has(url.hostname);
  if (
    (url.protocol !== 'https:' && !(localQualification && url.protocol === 'http:')) ||
    RETIRED_RPC_HOSTS.has(url.hostname.replace(/\.$/, '')) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    (value !== url.origin && value !== `${url.origin}/`)
  )
    throw invalid();
  return url.origin;
}

export function getPaxeerRpcUrls(): string[] {
  const out: string[] = [];
  for (const [variable, candidate] of [
    ['NEXT_PUBLIC_RPC_URL', process.env.NEXT_PUBLIC_RPC_URL],
    ['NEXT_PUBLIC_RPC_URL_FALLBACK', process.env.NEXT_PUBLIC_RPC_URL_FALLBACK],
    ['NEXT_PUBLIC_PAXEER_RPC_URL', process.env.NEXT_PUBLIC_PAXEER_RPC_URL],
  ] as const) {
    if (candidate === undefined) continue;
    const url = checkedRpc(variable, candidate);
    if (!out.includes(url)) out.push(url);
  }
  return out.length ? out : [DEFAULT_PAXEER_RPC_URL];
}

const PAXEER_RPC_URL = () => getPaxeerRpcUrls()[0];

/** The primary read RPC for this deployment. */
export function getPaxeerRpcUrl(): string {
  return PAXEER_RPC_URL();
}
