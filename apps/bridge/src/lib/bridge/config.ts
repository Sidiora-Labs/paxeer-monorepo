/**
 * Client configuration — deploy inputs only.
 *
 * The chain configs in contracts/bridge/** ship every deployment value as a
 * PLACEHOLDER (owner, attestors, Solana program id). Nothing here invents a
 * value: an unset variable surfaces in the UI as "not deployed" / "backend
 * not configured", and write actions stay disabled. NEXT_PUBLIC_* variables
 * are inlined at build time, so a deployer must rebuild after changing them.
 */
import { getAddress, isAddress, zeroAddress, type Address } from 'viem';

import { isSolanaAddress } from './solana';

function readUrl(value: string | undefined): string | undefined {
  const raw = value?.trim();
  if (!raw) return undefined;
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return undefined;
    if (parsed.username || parsed.password) return undefined;
    return raw.replace(/\/+$/, '');
  } catch {
    return undefined;
  }
}

function readAddress(value: string | undefined): Address | undefined {
  const raw = value?.trim();
  return raw && raw !== zeroAddress && isAddress(raw, { strict: false }) ? getAddress(raw) : undefined;
}

export function readConfig(env: Record<string, string | undefined>) {
  const solanaProgramId = env.NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID?.trim();
  return {

    /** Paxeer JSON-RPC. Unset → Paxeer reads are disabled, never guessed. */
    paxeerRpcUrl: readUrl(env.NEXT_PUBLIC_PAXEER_RPC_URL),
    paxeerExplorerUrl: readUrl(env.NEXT_PUBLIC_PAXEER_EXPLORER_URL) ?? 'https://paxscan.io',

    /** Attestation API base (the backend your devs run). Unset → claim/release disabled. */
    attestationApiUrl: readUrl(env.NEXT_PUBLIC_BRIDGE_API_URL),
    solanaProgramId: solanaProgramId && isSolanaAddress(solanaProgramId) ? solanaProgramId : undefined,

    /** PaxeerXVault deployments per foreign EVM chain, from the deploy records. */
    vaults: {
      ethereum: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM),
      base: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_BASE),
      arbitrum: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM),
      optimism: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM),
      bnb: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_BNB),
      polygon: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_POLYGON),
      avalanche: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE),
      hyperevm: readAddress(env.NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM),
    },

    /** Foreign RPC overrides (wagmi `http(url)`); public viem defaults otherwise. */
    foreignRpcUrls: {
      ethereum: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM),
      base: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_BASE),
      arbitrum: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM),
      optimism: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM),
      bnb: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_BNB),
      polygon: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_POLYGON),
      avalanche: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE),
      hyperevm: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM),
      solana: readUrl(env.NEXT_PUBLIC_BRIDGE_RPC_SOLANA),
    },
  } as const;
}

export const bridgeConfig = readConfig({
  NEXT_PUBLIC_PAXEER_RPC_URL: process.env.NEXT_PUBLIC_PAXEER_RPC_URL,
  NEXT_PUBLIC_PAXEER_EXPLORER_URL: process.env.NEXT_PUBLIC_PAXEER_EXPLORER_URL,
  NEXT_PUBLIC_BRIDGE_API_URL: process.env.NEXT_PUBLIC_BRIDGE_API_URL,
  NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: process.env.NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID,
  NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: process.env.NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM,
  NEXT_PUBLIC_BRIDGE_VAULT_BASE: process.env.NEXT_PUBLIC_BRIDGE_VAULT_BASE,
  NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM: process.env.NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM,
  NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM: process.env.NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM,
  NEXT_PUBLIC_BRIDGE_VAULT_BNB: process.env.NEXT_PUBLIC_BRIDGE_VAULT_BNB,
  NEXT_PUBLIC_BRIDGE_VAULT_POLYGON: process.env.NEXT_PUBLIC_BRIDGE_VAULT_POLYGON,
  NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE: process.env.NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE,
  NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM: process.env.NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM,
  NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: process.env.NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM,
  NEXT_PUBLIC_BRIDGE_RPC_BASE: process.env.NEXT_PUBLIC_BRIDGE_RPC_BASE,
  NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM: process.env.NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM,
  NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM: process.env.NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM,
  NEXT_PUBLIC_BRIDGE_RPC_BNB: process.env.NEXT_PUBLIC_BRIDGE_RPC_BNB,
  NEXT_PUBLIC_BRIDGE_RPC_POLYGON: process.env.NEXT_PUBLIC_BRIDGE_RPC_POLYGON,
  NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE: process.env.NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE,
  NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM: process.env.NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM,
  NEXT_PUBLIC_BRIDGE_RPC_SOLANA: process.env.NEXT_PUBLIC_BRIDGE_RPC_SOLANA,
});
