import { bridgeChainByKey } from '@/lib/bridge/chains';
import { bridgeConfig } from '@/lib/bridge/config';
import { releaseNullifier } from '@/lib/bridge/digests';
import { base58Encode, bytesToHex, solanaProgramAddress, solanaRecipientHandle } from '@/lib/bridge/solana';
import type { BridgeTransfer } from '@/lib/bridge/transfers';
import { hexToBytes, isAddress } from 'viem';

export type SolanaReleaseObservation = {
  status: 'waiting' | 'released';
  accountUrl: string;
  payoutUrl?: string;
};

export type SolanaReleaseSettings = { programId?: string; rpcUrl?: string };

export async function readSolanaRelease(
  transfer: BridgeTransfer,
  settings: SolanaReleaseSettings = { programId: bridgeConfig.solanaProgramId, rpcUrl: bridgeConfig.foreignRpcUrls.solana },
): Promise<SolanaReleaseObservation> {
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  if (transfer.direction !== 'out' || chain?.kind !== 'solana' || chain.chainId !== BigInt(transfer.foreignChainId)) {
    throw new Error('This is not a Solana withdrawal.');
  }
  if (!settings.programId || !settings.rpcUrl || transfer.bridgeOutNonce === undefined) {
    throw new Error('Solana release observation requires the custody program, RPC and burn nonce.');
  }
  const nullifier = releaseNullifier(transfer.sourceTxHash, BigInt(transfer.bridgeOutNonce));
  const pda = solanaProgramAddress(settings.programId, [ new TextEncoder().encode('nullifier'), hexToBytes(nullifier) ]);
  if (!pda) throw new Error('Invalid Solana custody program.');
  const accountUrl = `https://explorer.solana.com/address/${ pda.address }`;
  const response = await fetch(settings.rpcUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0', id: 1, method: 'getAccountInfo',
      params: [ pda.address, { encoding: 'base64', commitment: 'finalized' } ],
    }),
    cache: 'no-store',
    signal: AbortSignal.timeout(10_000),
  });
  if (!response.ok) throw new Error(`Solana RPC unavailable (${ response.status }).`);
  const payload: unknown = await response.json();
  if (typeof payload !== 'object' || payload === null || !('jsonrpc' in payload) || payload.jsonrpc !== '2.0' ||
    !('id' in payload) || payload.id !== 1 || 'error' in payload || !('result' in payload)) {
    throw new Error('Invalid Solana release RPC response.');
  }
  const result = payload.result;
  if (typeof result !== 'object' || result === null || !('value' in result) || !('context' in result) ||
    typeof result.context !== 'object' || result.context === null || !('slot' in result.context) ||
    !Number.isSafeInteger(result.context.slot) || Number(result.context.slot) < 0) throw new Error('Invalid Solana release context.');
  if (result.value === null) return { status: 'waiting', accountUrl };
  const account = result.value;
  if (typeof account !== 'object' || account === null || !('owner' in account) || account.owner !== settings.programId ||
    !('executable' in account) || account.executable !== false || !('data' in account) || !Array.isArray(account.data) ||
    account.data.length !== 2 || typeof account.data[0] !== 'string' || account.data[1] !== 'base64') {
    throw new Error('Invalid Solana release account owner or encoding.');
  }
  const bytes = Uint8Array.from(atob(account.data[0]), (character) => character.charCodeAt(0));
  if (bytes.length !== 130 || new TextDecoder().decode(bytes.subarray(0, 8)) !== 'PXBRNUL0' ||
    new DataView(bytes.buffer).getUint16(8, false) !== 1) throw new Error('Invalid Solana release record layout.');
  const view = new DataView(bytes.buffer);
  const mint = base58Encode(bytes.subarray(50, 82));
  const recipient = base58Encode(bytes.subarray(82, 114));
  const asset = chain.assets.find((entry) => entry.assetId.toLowerCase() === transfer.assetId.toLowerCase());
  const expectedHandle = isAddress(transfer.recipient, { strict: false }) ? transfer.recipient : solanaRecipientHandle(transfer.recipient);
  if (bytesToHex(bytes.subarray(10, 42)).toLowerCase() !== transfer.sourceTxHash.toLowerCase() ||
    view.getBigUint64(42, false) !== BigInt(transfer.bridgeOutNonce) || mint !== asset?.solanaMint ||
    !expectedHandle || solanaRecipientHandle(recipient)?.toLowerCase() !== expectedHandle.toLowerCase() ||
    view.getBigUint64(114, false) !== BigInt(transfer.amount) || view.getBigUint64(122, false) > BigInt(Number(result.context.slot))) {
    throw new Error('Solana release record does not match the withdrawal.');
  }
  return { status: 'released', accountUrl, payoutUrl: accountUrl };
}
