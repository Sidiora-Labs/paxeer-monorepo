import type { BridgeChain } from './chains';
import type { readConfig } from './config';

export function effectiveForeignRpcUrl(
  chain: BridgeChain,
  config: ReturnType<typeof readConfig>,
): string {
  const override = config.foreignRpcUrls[chain.key as keyof typeof config.foreignRpcUrls];
  const endpoint = override ?? chain.viemChain?.rpcUrls.default.http[0];
  if (!endpoint) throw new Error(`No RPC is configured for ${ chain.label }.`);
  return endpoint;
}
