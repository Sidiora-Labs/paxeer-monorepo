import { PAXEER_CONFIG } from './constants';

export interface Network {
  chainId: number;
  name: string;
  rpcUrl: string;
  explorerUrl: string;
  kind: 'root' | 'virtual';
}

export const VIRTUAL_CHAIN_ID_BASE = 125000000;

export function isVirtualChainId(id: number): boolean {
  return (
    Number.isSafeInteger(id) &&
    id > VIRTUAL_CHAIN_ID_BASE &&
    id <= VIRTUAL_CHAIN_ID_BASE + 4294967295
  );
}

export function rootNetwork(): Network {
  return {
    chainId: PAXEER_CONFIG.chainId,
    name: 'Paxeer',
    rpcUrl: PAXEER_CONFIG.rpcUrl,
    explorerUrl: PAXEER_CONFIG.explorerBase,
    kind: 'root',
  };
}

export function virtualNetwork(id: number, name: string): Network {
  if (!isVirtualChainId(id)) throw new TypeError('Virtual chain id is invalid');
  if (typeof name !== 'string' || !name.trim() || name.length > 80) {
    throw new TypeError('Network name is invalid');
  }
  const virtualId = id - VIRTUAL_CHAIN_ID_BASE;
  return {
    chainId: id,
    name: name.trim(),
    rpcUrl: `${PAXEER_CONFIG.rpcUrl.replace(/\/+$/, '')}/chain/${virtualId}`,
    explorerUrl: `${PAXEER_CONFIG.explorerBase.replace(/\/+$/, '')}/chains/${virtualId}`,
    kind: 'virtual',
  };
}

export function networkForChainId(id: number | null, saved: readonly Network[]): Network {
  if (id !== null && isVirtualChainId(id)) {
    const network = saved.find((entry) => entry.chainId === id && entry.kind === 'virtual');
    if (network) return network;
  }
  return rootNetwork();
}
