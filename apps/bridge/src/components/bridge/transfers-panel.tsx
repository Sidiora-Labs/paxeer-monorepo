'use client';

import { DepositStatus, type TransferStatusItem } from '@/components/trading/deposit-status';
import { TransferHistory, type TransferRecord } from '@/components/trading/transfer-history';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/sonner';
import { Spinner } from '@/components/ui/spinner';
import { formatAmount } from '@/lib/bridge/amount';
import { attestationPollInterval, fetchDepositAttestation, fetchReleaseAttestation } from '@/lib/bridge/attestation-api';
import { bridgeChainByKey, finalityEstimate, paxeer } from '@/lib/bridge/chains';
import { bridgeConfig } from '@/lib/bridge/config';
import { enrichRecord } from '@/lib/bridge/enrich';
import { describeWriteError } from '@/lib/bridge/errors';
import { foreignTxUrl, paxeerTxUrl } from '@/lib/bridge/explorer';
import { historyStatusOf, phaseOf, stepOf } from '@/lib/bridge/phase';
import { settlementBroadcastPatch, settlementPatch } from '@/lib/bridge/settlement';
import type { BridgeTransfer } from '@/lib/bridge/transfers';
import {
  attestationApiConfigured,
  useDepositNullified,
} from '@/lib/bridge/use-bridge-state';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import { queryOptions, useQueries, useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAccount, useChainId, useSwitchChain, useWriteContract } from 'wagmi';
import { getTransactionReceipt } from 'wagmi/actions';

import { ImportTransfer } from './import-transfer';
import {
  buildVerifiedClaim,
  buildVerifiedRelease,
  decodeTransferLive,
  readTransferLive,
  resolveWithdrawalVault,
  settledRefusal,
  type TransferLive,
  type TransferReadSettings,
} from './transfer-actions';
import { listBridgeTransfers, updateTransfer } from './transfer-ledger';

function sourceChainIdOf(transfer: BridgeTransfer): number | undefined {
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  return transfer.direction === 'in' ? chain?.viemChain?.id : paxeer.id;
}

/** Enrich local records with the ids only the receipt knows (log index, nonce). */
async function enrichTransfers(list: Array<BridgeTransfer>): Promise<Array<BridgeTransfer>> {
  let changed = false;
  const next: Array<BridgeTransfer> = [];
  for (const stored of list) {
    const transfer = await resolveWithdrawalVault(stored);
    if (transfer !== stored) {
      updateTransfer(transfer.id, { vault: transfer.vault });
      changed = true;
    }
    const needsLogIndex = transfer.direction === 'in' && transfer.logIndex === undefined;
    const needsNonce = transfer.direction === 'out' && transfer.bridgeOutNonce === undefined;
    const chainId = sourceChainIdOf(transfer);
    if ((!needsLogIndex && !needsNonce) || chainId === undefined) {
      next.push(transfer);
      continue;
    }
    const receipt = await getTransactionReceipt(wagmiConfig, {
      hash: transfer.sourceTxHash,
      chainId,
    }).catch((error: unknown) => {
      if (error instanceof Error && error.name === 'TransactionReceiptNotFoundError') return null;
      throw error;
    });
    if (!receipt) {
      next.push(transfer);
      continue;
    }
    const enriched = enrichRecord(transfer, receipt);
    if (enriched !== transfer) {
      updateTransfer(enriched.id, {
        logIndex: enriched.logIndex,
        nonce: enriched.nonce,
        bridgeOutNonce: enriched.bridgeOutNonce,
      });
      changed = true;
    }
    next.push(enriched);
  }
  return changed ? next : list;
}

/** The page and its consumers use the same storage/enrichment crossing. */
export function transferListOptions(address: BridgeTransfer['account'] | undefined, refreshKey: number, version: number) {
  return queryOptions({
    queryKey: [ 'bridge-transfers', address, refreshKey, version ],
    placeholderData: (previous, query) => query?.queryKey[1] === address ? previous : undefined,
    queryFn: async() => enrichTransfers(listBridgeTransfers(address)),
  });
}

export function transferLiveOptions(transfer: BridgeTransfer, settings: TransferReadSettings = {}) {
  return queryOptions({
    queryKey: [ 'transfer-live', transfer.id, transfer.claimTxHash, transfer.releaseTxHash ],
    placeholderData: (previous, query) => query?.queryKey[1] === transfer.id ? previous : undefined,
    queryFn: async() => {
      const live = await readTransferLive(transfer, settings);
      const patch = settlementPatch(transfer, live, Date.now());
      if (patch) updateTransfer(transfer.id, patch);
      return live;
    },
    select: decodeTransferLive,
  });
}

/** Shared with the settlement rows so each observation uses the real HTTP decoder. */
export function depositAttestationOptions(transfer: BridgeTransfer, settled: boolean, base = bridgeConfig.attestationApiUrl) {
  return queryOptions({
    queryKey: [ 'deposit-attestation', transfer.id, base ],
    enabled: Boolean(base) && !settled && transfer.direction === 'in' && transfer.logIndex !== undefined,
    queryFn: () => fetchDepositAttestation({
      chainId: BigInt(transfer.foreignChainId),
      txHash: transfer.sourceTxHash,
      logIndex: BigInt(transfer.logIndex!),
    }, base),
    refetchInterval: (query) => attestationPollInterval(query.state.data, 12_000),
  });
}

export function releaseAttestationOptions(transfer: BridgeTransfer, settled: boolean, base = bridgeConfig.attestationApiUrl) {
  return queryOptions({
    queryKey: [ 'release-attestation', transfer.id, base ],
    enabled: Boolean(base) && !settled && transfer.direction === 'out' && transfer.bridgeOutNonce !== undefined,
    queryFn: () => fetchReleaseAttestation({
      chainId: BigInt(transfer.foreignChainId),
      paxeerTxHash: transfer.sourceTxHash,
      paxeerNonce: BigInt(transfer.bridgeOutNonce!),
    }, base),
    refetchInterval: (query) => attestationPollInterval(query.state.data, 12_000),
  });
}

export function transferHistoryRecords(transfers: Array<BridgeTransfer>, live: Array<Parameters<typeof phaseOf>[1]>): Array<TransferRecord> {
  return transfers.map((transfer, index) => {
    const chain = bridgeChainByKey(transfer.foreignChainKey);
    const state = live[index];
    const phase = phaseOf(transfer, state, state?.attestationReady === true);
    return {
      id: transfer.id,
      type: transfer.direction === 'in' ? 'deposit' : 'withdraw',
      token: transfer.symbol,
      amount: Number(formatAmount(BigInt(transfer.amount), transfer.decimals)),
      decimals: Math.min(transfer.decimals, 6),
      fee: null,
      network: chain?.label,
      status: historyStatusOf(phase),
      time: transfer.submittedAt,
      txHash: transfer.sourceTxHash,
    };
  });
}

export function transferFinalityEstimate(transfer: BridgeTransfer): string | undefined {
  if (transfer.direction === 'out') return 'At the committed Paxeer block';
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  return chain ? finalityEstimate({ ...chain, finalityDepth: transfer.requiredConfirmations }) : undefined;
}

/** RPC errors retain query data; Solana recovery re-observes the automatic relay. */
export function TransferObservation({ transfer, live, error, retry, retrying = false }: {
  transfer: BridgeTransfer;
  live?: TransferLive;
  error?: Error | null;
  retry: () => void;
  retrying?: boolean;
}) {
  const solana = transfer.direction === 'out' && bridgeChainByKey(transfer.foreignChainKey)?.kind === 'solana';
  return (
    <>
      { error ? (
        <div role="alert" className="px-1 text-xs text-destructive">
          RPC unavailable. Last verified progress is retained. { error.message }
          <Button size="sm" variant="tonal" disabled={ retrying } onClick={ retry }>Retry RPC</Button>
        </div>
      ) : null }
      { solana ? (
        <div className="flex flex-col items-start gap-2 px-1 text-xs text-muted-foreground">
          <p>{ live?.released ? 'Solana payout confirmed.' : 'Solana relay pending. The relay submits the payout automatically after the burn is final.' }</p>
          { live?.solanaRelease ? <a href={ live.solanaRelease.payoutUrl ?? live.solanaRelease.accountUrl } target="_blank" rel="noreferrer">{ live.released ? 'View Solana payout record' : 'View Solana release account' }</a> : null }
          { !live?.released ? <Button size="sm" variant="tonal" disabled={ retrying } onClick={ retry }>Refresh relay status</Button> : null }
        </div>
      ) : null }
    </>
  );
}

export function TransferRow({
  transfer,
  onChanged,
  readSettings,
}: {
  transfer: BridgeTransfer;
  onChanged: () => void;
  readSettings?: TransferReadSettings;
}) {
  const chain = bridgeChainByKey(transfer.foreignChainKey);
  const isSolanaWithdrawal = transfer.direction === 'out' && chain?.kind === 'solana';
  const walletChainId = useChainId();
  const { switchChain } = useSwitchChain();
  const settle = useWriteContract();
  const liveQuery = useQuery({
    ...transferLiveOptions(transfer, readSettings),
    refetchInterval: 15_000,
  });
  const live = liveQuery.data;
  const nullified = useDepositNullified(transfer);
  const credited = nullified.data === true || live?.credited === true;
  const released = live?.released === true;

  const depositAttestation = useQuery(depositAttestationOptions(transfer, credited));
  const releaseAttestation = useQuery(releaseAttestationOptions(transfer, released || isSolanaWithdrawal));
  const attestation = transfer.direction === 'in' ? depositAttestation.data : releaseAttestation.data;
  const attestationReady = attestation?.status === 'ready' && attestation.signatures.length > 0;

  const phase = phaseOf(transfer, live ? { ...live, credited } : undefined, attestationReady);

  // Persist a claim the chain already knows about (claimed from elsewhere).
  useEffect(() => {
    const patch = settlementPatch(transfer, live ? { ...live, credited } : undefined, Date.now());
    if (patch) {
      updateTransfer(transfer.id, patch);
      onChanged();
    }
  }, [ credited, live, onChanged, transfer ]);

  const settleChainId = transfer.direction === 'in' ? paxeer.id : chain?.viemChain?.id;
  const onSettleChain = settleChainId !== undefined && walletChainId === settleChainId;

  const runSettle = async() => {
    try {
      const refusal = settledRefusal(transfer, await readTransferLive(transfer));
      if (refusal) throw new Error(refusal);
      if (transfer.direction === 'in') {
        if (!depositAttestation.data) throw new Error('The attestation is not available yet.');
        const { request } = await buildVerifiedClaim(transfer, depositAttestation.data);
        settle.writeContract(request, {
          onSuccess: (hash) => {
            updateTransfer(transfer.id, settlementBroadcastPatch(transfer, hash, Date.now()));
            toast.success('Claim submitted', { description: 'The bridged balance lands once the Paxeer transaction confirms.' });
            onChanged();
          },
          onError: (error) => toast.error('Claim failed', { description: describeWriteError(error) }),
        });
      } else {
        if (!releaseAttestation.data) throw new Error('The attestation is not available yet.');
        const { request } = await buildVerifiedRelease(transfer, releaseAttestation.data);
        settle.writeContract(request, {
          onSuccess: (hash) => {
            updateTransfer(transfer.id, settlementBroadcastPatch(transfer, hash, Date.now()));
            toast.success('Release submitted', { description: `The vault pays out on ${ chain?.label ?? 'the foreign chain' } once it confirms.` });
            onChanged();
          },
          onError: (error) => toast.error('Release failed', { description: describeWriteError(error) }),
        });
      }
    } catch (error) {
      toast.error('Could not verify the attestation', {
        description: error instanceof Error ? error.message : 'Verification failed.',
      });
    }
  };

  const amountNumber = Number(formatAmount(BigInt(transfer.amount), transfer.decimals));
  const sourceUrl =
    transfer.direction === 'in' ?
      chain ?
        foreignTxUrl(chain, transfer.sourceTxHash) :
        undefined :
      paxeerTxUrl(transfer.sourceTxHash);

  const statusItem: TransferStatusItem = {
    id: transfer.id,
    type: transfer.direction === 'in' ? 'deposit' : 'withdraw',
    token: transfer.symbol,
    amount: amountNumber,
    network: chain?.label,
    step: stepOf(phase),
    confirmations: live?.confirmations,
    requiredConfirmations: live?.requiredConfirmations ?? transfer.requiredConfirmations,
    time: new Date(transfer.submittedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    eta: phase === 'confirming' ? transferFinalityEstimate(transfer) : undefined,
    txHash: transfer.sourceTxHash,
  };

  return (
    <div className="flex flex-col gap-2">
      <DepositStatus items={ [ statusItem ] } explorerUrl={ () => sourceUrl } defaultOpen={ phase === 'ready' }/>
      <TransferObservation transfer={ transfer } live={ live } error={ liveQuery.error } retrying={ liveQuery.isFetching } retry={ () => void liveQuery.refetch() }/>
      { !isSolanaWithdrawal && phase === 'attesting' && !attestationApiConfigured ? (
        <p className="px-1 text-[11px] text-muted-foreground">
          Finality reached. The signature bundle comes from the bridge backend, which is not configured in this
          deployment — the { transfer.direction === 'in' ? 'claim' : 'release' } button appears once it is.
        </p>
      ) : null }
      { !isSolanaWithdrawal && phase === 'attesting' && attestationApiConfigured ? (
        <p className="flex items-center gap-1.5 px-1 text-[11px] text-muted-foreground">
          <Spinner className="size-3"/> Waiting for the attestor threshold…
        </p>
      ) : null }
      { phase === 'settling' ? (
        <p className="px-1 text-[11px] text-muted-foreground">Settlement broadcast. Waiting for a committed receipt and the bridge nullifier. You can retry if the transaction was dropped or replaced.</p>
      ) : null }
      { live?.settlementStatus === 'reverted' || live?.settlementStatus === 'dropped' ? (
        <p className="px-1 text-[11px] text-destructive">The settlement attempt { live.settlementStatus }. Retry to submit another attempt.</p>
      ) : null }
      { !isSolanaWithdrawal && (phase === 'ready' || phase === 'settling') && attestationReady ? (
        onSettleChain ? (
          <Button size="sm" className="self-start" disabled={ settle.isPending } onClick={ () => void runSettle() }>
            { settle.isPending ? <Spinner/> : null }
            { transfer.claimTxHash || transfer.releaseTxHash ? 'Retry settlement' : transfer.direction === 'in' ? 'Claim on Paxeer' : `Release on ${ chain?.label ?? 'destination' }` }
          </Button>
        ) : (
          <Button
            size="sm"
            variant="tonal"
            className="self-start"
            onClick={ () => settleChainId && switchChain({ chainId: settleChainId }) }
          >
            Switch to { transfer.direction === 'in' ? 'Paxeer' : (chain?.label ?? 'destination') } to{ ' ' }
            { transfer.direction === 'in' ? 'claim' : 'release' }
          </Button>
        )
      ) : null }
      { phase === 'failed' ? (
        <p className="px-1 text-[11px] text-destructive">The source transaction reverted. No funds moved.</p>
      ) : null }
    </div>
  );
}

export function TransfersPanel({ refreshKey }: { refreshKey: number }) {
  const { address, isConnected } = useAccount();
  const [ version, setVersion ] = useState(0);
  const onChanged = useCallback(() => setVersion((v) => v + 1), []);

  const transfersQuery = useQuery({
    ...transferListOptions(address, refreshKey, version),
    enabled: isConnected && address !== undefined,
    refetchInterval: 30_000,
  });
  const transfers = useMemo(() => transfersQuery.data ?? [], [ transfersQuery.data ]);
  const liveQueries = useQueries({
    queries: transfers.map((transfer) => ({
      ...transferLiveOptions(transfer),
      refetchInterval: 15_000,
    })),
  });

  const records = transferHistoryRecords(transfers, liveQueries.map((query) => query.data));

  const pending = transfers.filter((transfer, index) => phaseOf(transfer, liveQueries[index]?.data, false) !== 'settled' ||
    (transfer.direction === 'out' && bridgeChainByKey(transfer.foreignChainKey)?.kind === 'solana'));

  return (
    <section id="transfers" aria-label="Your bridge transfers" className="flex w-full flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-lg font-semibold tracking-tight">Your transfers</h2>
        <span className="text-xs text-muted-foreground">This browser's record — chain state is re-read live.</span>
      </div>

      { !isConnected ? (
        <Alert variant="info">
          <AlertTitle>Connect a wallet to see your transfers</AlertTitle>
          <AlertDescription>
            Deposits and withdrawals you start here are tracked per wallet. A transfer started elsewhere is still
            claimable — its source transaction is the record.
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <ImportTransfer onImported={ onChanged }/>
          { transfersQuery.error ? <div role="alert">RPC unavailable. Last verified transfers are retained. <Button size="sm" disabled={ transfersQuery.isFetching } onClick={ () => void transfersQuery.refetch() }>Retry RPC</Button></div> : null }
          { pending.length > 0 ? (
            <div className="flex flex-col gap-3">
              { pending.map((transfer) => (
                <TransferRow key={ transfer.id } transfer={ transfer } onChanged={ onChanged }/>
              )) }
            </div>
          ) : null }
          <TransferHistory
            records={ records }
            filters={ [ 'all', 'deposit', 'withdraw' ] }
            loading={ transfersQuery.isLoading }
            emptyText="No bridge transfers from this wallet yet. Your first deposit or withdrawal will appear here."
            explorerUrl={ (record) => {
              const transfer = transfers.find((t) => t.id === record.id);
              if (!transfer) return undefined;
              const chain = bridgeChainByKey(transfer.foreignChainKey);
              return transfer.direction === 'in' ?
                chain ?
                  foreignTxUrl(chain, transfer.sourceTxHash) :
                  undefined :
                paxeerTxUrl(transfer.sourceTxHash);
            } }
            onCopy={ () => toast.success('Copied') }
          />
        </>
      ) }
    </section>
  );
}
