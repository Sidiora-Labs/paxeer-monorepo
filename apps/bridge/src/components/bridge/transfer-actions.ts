'use client';

/**
 * The two settlement writes, shared by the bridge card's follow-through and
 * the transfer panels: claim a deposit with bridgeIn on Paxeer, and release
 * a withdrawal with PaxeerXVault.release on the foreign chain. Everything
 * the backend says is re-derived and re-verified against chain state
 * before the wallet is asked for a signature.
 */

import type { DepositAttestation, ReleaseAttestation } from '@/lib/bridge/attestation-api';
import { bridgeChainByKey, paxeer } from '@/lib/bridge/chains';
import { layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from '@/lib/bridge/contracts';
import { inboundDigest, outboundDigest, paxeerRecipient, releaseNullifier, verifyThresholdSignatures } from '@/lib/bridge/digests';
import { nonceFromBridgeOutLogs } from '@/lib/bridge/import';
import { phaseOf, type SettlementStatus } from '@/lib/bridge/settlement';
import type { BridgeTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import type { Log } from 'viem';
import { getAddress, zeroAddress } from 'viem';
import type { Config } from 'wagmi';
import { getBlockNumber, readContract, getTransaction, getTransactionReceipt } from 'wagmi/actions';

import { readSolanaRelease, type SolanaReleaseObservation, type SolanaReleaseSettings } from './solana-release';
import { migrateBridgeTransfer } from './transfer-ledger';

export type TransferPhase =
  | 'confirming' |
  'attesting' |
  'ready' |
  'settling' |
  'settled' |
  'failed';

export type TransferLive = {
  confirmations: number;
  requiredConfirmations: number;
  credited: boolean;
  released: boolean;
  attestationReady: boolean;
  sourceFailed: boolean;
  settlementStatus?: SettlementStatus;
  solanaRelease?: SolanaReleaseObservation;
};

/** Refuse malformed live state at the query-to-component boundary. */
export function decodeTransferLive(value: unknown): TransferLive {
  if (typeof value !== 'object' || value === null) throw new Error('Invalid live transfer state.');
  const state = value as Record<string, unknown>;
  if (
    !Number.isSafeInteger(state.confirmations) || Number(state.confirmations) < 0 ||
    !Number.isSafeInteger(state.requiredConfirmations) || Number(state.requiredConfirmations) < 0 ||
    typeof state.credited !== 'boolean' || typeof state.released !== 'boolean' ||
    typeof state.attestationReady !== 'boolean' || typeof state.sourceFailed !== 'boolean' ||
    (state.settlementStatus !== undefined && ![ 'pending', 'dropped', 'reverted', 'confirmed' ].includes(String(state.settlementStatus)))
  ) {
    throw new Error('Invalid live transfer state.');
  }
  const relay = state.solanaRelease;
  if (relay !== undefined && (typeof relay !== 'object' || relay === null ||
    !('status' in relay) || ![ 'waiting', 'released' ].includes(String(relay.status)) ||
    !('accountUrl' in relay) || typeof relay.accountUrl !== 'string' ||
    !/^https:\/\/explorer\.solana\.com\/address\/[1-9A-HJ-NP-Za-km-z]+$/.test(relay.accountUrl) ||
    ('payoutUrl' in relay && relay.payoutUrl !== relay.accountUrl))) throw new Error('Invalid Solana release observation.');
  return {
    ...(relay === undefined ? {} : { solanaRelease: relay as SolanaReleaseObservation }),
    confirmations: Number(state.confirmations),
    requiredConfirmations: Number(state.requiredConfirmations),
    credited: state.credited,
    released: state.released,
    attestationReady: state.attestationReady,
    sourceFailed: state.sourceFailed,
    ...(state.settlementStatus === undefined ? {} : { settlementStatus: state.settlementStatus as SettlementStatus }),
  };
}

/** Resolve old and new withdrawal rows against the authoritative chain registry. */
export async function resolveWithdrawalVault(transfer: BridgeTransfer, config: Config = wagmiConfig): Promise<BridgeTransfer> {
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  if (transfer.direction !== 'out' || chain?.kind !== 'evm') return transfer;
  if (chain.chainId !== BigInt(transfer.foreignChainId)) throw new Error('Unknown or mismatched bridge chain.');
  const info = await readContract(config, {
    abi: layerxBridgeAbi,
    address: LAYERX_BRIDGE_ADDRESS,
    functionName: 'getChain',
    args: [ chain.chainId ],
    chainId: paxeer.id,
  }) as readonly [boolean, `0x${ string }`, bigint, boolean];
  if (!info[0] || !info[3] || getAddress(info[1]) === zeroAddress) {
    throw new Error('This chain is not registered and enabled on Paxeer.');
  }
  const vault = getAddress(info[1]);
  if ((transfer.vault && transfer.vault.toLowerCase() !== vault.toLowerCase()) ||
    (chain.vault && chain.vault.toLowerCase() !== vault.toLowerCase())) {
    throw new Error('The withdrawal vault does not match the vault registered on Paxeer.');
  }
  return transfer.vault === vault ? transfer : { ...transfer, vault };
}

export function releaseCheckOf(transfer: BridgeTransfer) {
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  if (transfer.direction !== 'out' || !transfer.vault || transfer.bridgeOutNonce === undefined || !chain?.viemChain) {
    return undefined;
  }
  return {
    abi: paxeerXVaultAbi,
    address: transfer.vault,
    functionName: 'nullified',
    args: [ releaseNullifier(transfer.sourceTxHash, BigInt(transfer.bridgeOutNonce)) ],
    chainId: chain.viemChain.id,
  } as const;
}

export function settledRefusal(transfer: BridgeTransfer, live: TransferLive | undefined): string | undefined {
  if (phaseOf(transfer, live, false) !== 'settled') return undefined;
  if (transfer.direction === 'in') {
    return 'This deposit is already credited on Paxeer.';
  }
  if (transfer.direction === 'out') {
    return 'This withdrawal is already released from the vault.';
  }
  return undefined;
}

/** Source-chain confirmation count and receipt status for a transfer. */
export type TransferReadSettings = { config?: Config; solana?: SolanaReleaseSettings };

export async function readTransferLive(transfer: BridgeTransfer, settings: TransferReadSettings = {}): Promise<TransferLive> {
  const config = settings.config ?? wagmiConfig;
  transfer = migrateBridgeTransfer(transfer);
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  if (!chain || chain.chainId !== BigInt(transfer.foreignChainId)) {
    throw new Error('Unknown or mismatched bridge chain.');
  }
  const sourceChainId = transfer.direction === 'in' ? chain?.viemChain?.id : paxeer.id;
  if (sourceChainId === undefined) throw new Error('Source receipt observation is unavailable for this route.');
  const [ receipt, head ] = await Promise.all([
    getTransactionReceipt(config, { hash: transfer.sourceTxHash, chainId: sourceChainId }).catch((error: unknown) => {
      if (error instanceof Error && error.name === 'TransactionReceiptNotFoundError') return null;
      throw error;
    }),
    getBlockNumber(config, { chainId: sourceChainId, cacheTime: 0 }),
  ]);
  let credited = false;
  if (transfer.direction === 'in' && transfer.logIndex !== undefined) {
    credited = await readContract(config, {
      abi: layerxBridgeAbi,
      address: LAYERX_BRIDGE_ADDRESS,
      functionName: 'isNullified',
      args: [ BigInt(transfer.foreignChainId), transfer.sourceTxHash, BigInt(transfer.logIndex) ],
      chainId: paxeer.id,
    }) as boolean;
  }
  const resolved = await resolveWithdrawalVault(transfer, config);
  const releaseCheck = releaseCheckOf(resolved);
  const solanaRelease = transfer.direction === 'out' && chain.kind === 'solana' ?
    await readSolanaRelease(transfer, settings.solana) : undefined;
  const released = solanaRelease ? solanaRelease.status === 'released' : releaseCheck ? await readContract(config, releaseCheck) : false;
  let settlementStatus: SettlementStatus | undefined;
  const settlementHash = transfer.direction === 'in' ? transfer.claimTxHash : transfer.releaseTxHash;
  const settlementChainId = transfer.direction === 'in' ? paxeer.id : chain.viemChain?.id;
  if (settlementHash && settlementChainId !== undefined) {
    const committed = await getTransactionReceipt(config, { hash: settlementHash, chainId: settlementChainId })
      .catch((error: unknown) => {
        if (error instanceof Error && error.name === 'TransactionReceiptNotFoundError') return null;
        throw error;
      });
    if (credited || released) settlementStatus = 'confirmed';
    else if (committed?.status === 'reverted') settlementStatus = 'reverted';
    else if (committed) settlementStatus = 'pending';
    else {
      const transaction = await getTransaction(config, { hash: settlementHash, chainId: settlementChainId })
        .catch((error: unknown) => {
          if (error instanceof Error && error.name === 'TransactionNotFoundError') return null;
          throw error;
        });
      settlementStatus = transaction || Date.now() - (transfer.settlementSubmittedAt ?? transfer.submittedAt) < 120_000 ? 'pending' : 'dropped';
    }
  }
  if (released || credited) settlementStatus = 'confirmed';
  const confirmations =
    receipt && head !== null ? Math.max(0, Number(head - receipt.blockNumber) + 1) : 0;
  return {
    confirmations,
    ...(solanaRelease ? { solanaRelease } : {}),
    requiredConfirmations: transfer.requiredConfirmations,
    credited,
    released,
    attestationReady: false,
    sourceFailed: receipt !== null && receipt.status === 'reverted',
    ...(settlementStatus === undefined ? {} : { settlementStatus }),
  };
}

export type VerifiedClaim = {
  request: {
    abi: typeof layerxBridgeAbi;
    address: typeof LAYERX_BRIDGE_ADDRESS;
    functionName: 'bridgeIn';
    args: [bigint, `0x${ string }`, `0x${ string }`, bigint, `0x${ string }`, `0x${ string }`, bigint, Array<`0x${ string }`>];
    chainId: number;
  };
};

export function assertClaimMatches(transfer: BridgeTransfer, attestation: DepositAttestation): void {
  if (
    attestation.txHash.toLowerCase() !== transfer.sourceTxHash.toLowerCase() ||
    BigInt(attestation.logIndex) !== BigInt(transfer.logIndex ?? '-1') ||
    BigInt(attestation.amount) !== BigInt(transfer.amount) ||
    attestation.recipient32.toLowerCase() !== paxeerRecipient(getAddress(transfer.recipient)).toLowerCase() ||
    getAddress(attestation.asset).toLowerCase() !== transfer.assetId.toLowerCase() ||
    BigInt(attestation.chainId) !== BigInt(transfer.foreignChainId)
  ) {
    throw new Error('The backend attestation does not match this deposit.');
  }
}

export async function buildVerifiedClaim(
  transfer: BridgeTransfer,
  attestation: DepositAttestation,
): Promise<VerifiedClaim> {
  if (transfer.direction !== 'in') throw new Error('Only deposits can be claimed.');
  if (attestation.status !== 'ready' || attestation.signatures.length === 0) {
    throw new Error('The deposit attestation is not ready.');
  }
  assertClaimMatches(transfer, attestation);
  const chainId = BigInt(transfer.foreignChainId);
  const chainInfo = (await readContract(wagmiConfig, {
    abi: layerxBridgeAbi,
    address: LAYERX_BRIDGE_ADDRESS,
    functionName: 'getChain',
    args: [ chainId ],
    chainId: paxeer.id,
  })) as readonly [boolean, `0x${ string }`, bigint, boolean];
  if (!chainInfo[0] || !chainInfo[3]) {
    throw new Error('This chain is not registered and enabled on Paxeer.');
  }
  const registeredVault = getAddress(chainInfo[1]);
  if (transfer.vault && registeredVault.toLowerCase() !== transfer.vault.toLowerCase()) {
    throw new Error('The vault registered on Paxeer does not match the deposit vault.');
  }
  if (getAddress(attestation.vault).toLowerCase() !== registeredVault.toLowerCase()) {
    throw new Error('The backend attested a different vault than the one registered on Paxeer.');
  }
  const digest = inboundDigest({
    chainId,
    vault: registeredVault,
    txHash: transfer.sourceTxHash,
    logIndex: BigInt(attestation.logIndex),
    recipient32: attestation.recipient32,
    asset: attestation.asset,
    amount: BigInt(attestation.amount),
  });
  if (digest.toLowerCase() !== attestation.digest.toLowerCase()) {
    throw new Error('The backend digest does not match the deposit.');
  }
  const attestors = (await readContract(wagmiConfig, {
    abi: layerxBridgeAbi,
    address: LAYERX_BRIDGE_ADDRESS,
    functionName: 'getAttestors',
    chainId: paxeer.id,
  })) as readonly [ReadonlyArray<`0x${ string }`>, ReadonlyArray<bigint>, number];
  const check = await verifyThresholdSignatures({
    digest,
    signatures: attestation.signatures,
    attestors: attestors[0],
    threshold: Number(attestors[2]),
  });
  if (!check.ok) throw new Error(check.reason);
  return {
    request: {
      abi: layerxBridgeAbi,
      address: LAYERX_BRIDGE_ADDRESS,
      functionName: 'bridgeIn',
      args: [
        chainId,
        registeredVault,
        transfer.sourceTxHash,
        BigInt(attestation.logIndex),
        attestation.recipient32,
        getAddress(attestation.asset),
        BigInt(attestation.amount),
        [ ...attestation.signatures ],
      ],
      chainId: paxeer.id,
    },
  };
}

export type VerifiedRelease = {
  request: {
    abi: typeof paxeerXVaultAbi;
    address: `0x${ string }`;
    functionName: 'release';
    args: [`0x${ string }`, bigint, `0x${ string }`, `0x${ string }`, bigint, Array<`0x${ string }`>];
    chainId: number;
  };
};

export function assertReleaseMatches(transfer: BridgeTransfer, attestation: ReleaseAttestation): void {
  if (
    attestation.paxeerTxHash.toLowerCase() !== transfer.sourceTxHash.toLowerCase() ||
    BigInt(attestation.paxeerNonce) !== BigInt(transfer.bridgeOutNonce ?? '-1') ||
    BigInt(attestation.amount) !== BigInt(transfer.amount) ||
    getAddress(attestation.recipient).toLowerCase() !== transfer.recipient.toLowerCase() ||
    getAddress(attestation.asset).toLowerCase() !== transfer.assetId.toLowerCase() ||
    BigInt(attestation.chainId) !== BigInt(transfer.foreignChainId)
  ) {
    throw new Error('The backend attestation does not match this withdrawal.');
  }
  if (!transfer.vault || getAddress(attestation.vault).toLowerCase() !== transfer.vault.toLowerCase()) {
    throw new Error('The backend attested a different vault.');
  }
}

export async function buildVerifiedRelease(
  transfer: BridgeTransfer,
  attestation: ReleaseAttestation,
): Promise<VerifiedRelease> {
  if (transfer.direction !== 'out') throw new Error('Only withdrawals can be released.');
  if (attestation.status !== 'ready' || attestation.signatures.length === 0) {
    throw new Error('The release attestation is not ready.');
  }
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  if (!chain?.viemChain) {
    throw new Error('This withdrawal cannot be released from an EVM wallet.');
  }
  transfer = await resolveWithdrawalVault(transfer);
  if (!transfer.vault) throw new Error('The registered withdrawal vault is unavailable.');
  assertReleaseMatches(transfer, attestation);
  const nonce = BigInt(transfer.bridgeOutNonce ?? '-1');
  const assetAddress = getAddress(attestation.asset);
  const onChainDigest = await readContract(wagmiConfig, {
    abi: paxeerXVaultAbi,
    address: transfer.vault,
    functionName: 'releaseDigest',
    args: [ transfer.sourceTxHash, nonce, getAddress(attestation.recipient), assetAddress, BigInt(attestation.amount) ],
    chainId: chain.viemChain.id,
  });
  const localDigest = outboundDigest({
    chainId: BigInt(chain.viemChain.id),
    vault: transfer.vault,
    paxeerTxHash: transfer.sourceTxHash,
    paxeerNonce: nonce,
    recipient: getAddress(attestation.recipient),
    asset: assetAddress,
    amount: BigInt(attestation.amount),
  });
  if (
    localDigest.toLowerCase() !== attestation.digest.toLowerCase() ||
    (onChainDigest as string).toLowerCase() !== attestation.digest.toLowerCase()
  ) {
    throw new Error('The backend digest does not match the withdrawal.');
  }
  const [ attestors, threshold ] = await Promise.all([
    readContract(wagmiConfig, {
      abi: paxeerXVaultAbi,
      address: transfer.vault,
      functionName: 'attestors',
      chainId: chain.viemChain.id,
    }),
    readContract(wagmiConfig, {
      abi: paxeerXVaultAbi,
      address: transfer.vault,
      functionName: 'threshold',
      chainId: chain.viemChain.id,
    }),
  ]);
  const check = await verifyThresholdSignatures({
    digest: attestation.digest,
    signatures: attestation.signatures,
    attestors: attestors as ReadonlyArray<`0x${ string }`>,
    threshold: Number(threshold),
  });
  if (!check.ok) throw new Error(check.reason);
  return {
    request: {
      abi: paxeerXVaultAbi,
      address: transfer.vault,
      functionName: 'release',
      args: [
        assetAddress,
        BigInt(attestation.amount),
        getAddress(attestation.recipient),
        transfer.sourceTxHash,
        nonce,
        [ ...attestation.signatures ],
      ],
      chainId: chain.viemChain.id,
    },
  };
}

/** Parse the BridgeOut nonce out of a Paxeer burn receipt. */
export function bridgeOutNonceFromReceipt(logs: ReadonlyArray<Log>): bigint | undefined {
  return nonceFromBridgeOutLogs(logs);
}
