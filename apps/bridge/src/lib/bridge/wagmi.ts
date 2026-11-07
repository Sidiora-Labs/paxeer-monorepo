/**
 * Wagmi config: Paxeer + every EVM bridge chain, injected (EIP-6963)
 * discovery only — no project ids, no remote wallet services. The Paxeer
 * transport comes from deploy config; foreign transports fall back to the
 * viem chain defaults unless a deploy RPC override is set.
 */
import { createConfig, http } from 'wagmi';
import { injected } from 'wagmi/connectors';

import { buildBridgeChains, walletBridgeChains, paxeer } from './chains';
import { bridgeConfig } from './config';
import { effectiveForeignRpcUrl } from './rpc';

export function createBridgeWagmiConfig(config: typeof bridgeConfig) {
  const evmBridgeChains = walletBridgeChains(buildBridgeChains(config));
  const evmViemChains = evmBridgeChains.map((chain) => chain.viemChain);
  return createConfig({
    chains: [ paxeer, ...evmViemChains ],
    connectors: [ injected() ],
    transports: {
      ...Object.fromEntries(
        evmBridgeChains.map((c) => [
          c.viemChain.id,
          http(effectiveForeignRpcUrl(c, config)),
        ]),
      ),
      [paxeer.id]: http(config.paxeerRpcUrl),
    },
    ssr: true,
  });
}

export const wagmiConfig = createBridgeWagmiConfig(bridgeConfig);

declare module 'wagmi' {
  interface Register {
    config: typeof wagmiConfig;
  }
}
