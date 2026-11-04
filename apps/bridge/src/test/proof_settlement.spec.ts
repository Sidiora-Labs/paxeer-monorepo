// @vitest-environment jsdom

/**
 * Proves: import.ts/settlement.ts/transfer-actions.ts -> exported modules/phase.ts -> enrichRecord/viem/DepositStatus/TransferHistory
 * Covers matchDepositLog, nonceFromBridgeOutLogs, phaseOf, settlementPatch, releaseCheckOf,
 * settledRefusal, stepOf and historyStatusOf through their real receipt, wallet and UI consumers.
 * Repairs: settlement.ts phaseOf ignored live withdrawal releases and saved settlement times;
 * transfer-actions.ts settledRefusal ignored saved settlements; phase.ts now delegates to the
 * same settlement rule. import.ts matchDepositLog accepted removed/foreign-transaction logs,
 * and nonceFromBridgeOutLogs accepted removed logs.
 * The anchor is stateless: every observation calls the real exports afresh, including after
 * changing the input. Receipt inputs use the production ABIs; matcher results, ledger patches,
 * phases and wallet requests passed to consumers are produced by the real modules, never doubles.
 */
import { settlementSubmissionReopensThroughBroadcastRevertAndConfirmation } from './repair-22-33-266.proofs';

import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import {
  decodeFunctionData,
  encodeAbiParameters,
  encodeEventTopics,
  encodeFunctionData,
  encodePacked,
  keccak256,
  zeroAddress,
  zeroHash,
} from 'viem';
import type { Address, Hex, TransactionReceipt } from 'viem';
import { expect, test } from 'vitest';

import { bridgeOutNonceFromReceipt, decodeTransferLive, releaseCheckOf, settledRefusal } from '@/components/bridge/transfer-actions';
import type { TransferLive } from '@/components/bridge/transfer-actions';
import { DepositStatus } from '@/components/trading/deposit-status';
import { TransferHistory } from '@/components/trading/transfer-history';
import { layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from '@/lib/bridge/contracts';
import { paxeerRecipient } from '@/lib/bridge/digests';
import { enrichRecord } from '@/lib/bridge/enrich';
import { matchDepositLog, nonceFromBridgeOutLogs } from '@/lib/bridge/import';
import { historyStatusOf, phaseOf, stepOf } from '@/lib/bridge/phase';
import { phaseOf as settlementPhaseOf, settlementPatch } from '@/lib/bridge/settlement';
import { transferIdOf, type BridgeTransfer } from '@/lib/bridge/transfers';

type ReceiptLog = TransactionReceipt['logs'][number];

const account: Address = '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const recipient: Address = '0xdddddddddddddddddddddddddddddddddddddddd';
const vault: Address = '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const other: Address = '0x000000000000000000000000000000000000dead';
const sourceTxHash: Hex = '0x1111111111111111111111111111111111111111111111111111111111111111';

function sourceTransfer(direction: 'in' | 'out'): BridgeTransfer {
  return {
    id: transferIdOf(direction, sourceTxHash),
    direction,
    foreignChainKey: 'ethereum',
    foreignChainId: '1',
    symbol: 'ETH',
    decimals: 18,
    amount: '1250000000000000000',
    assetId: zeroAddress,
    vault,
    account,
    recipient,
    sourceTxHash,
    requiredConfirmations: 12,
    submittedAt: 1000,
  };
}

function sourceLive(): TransferLive {
  return {
    confirmations: 12,
    requiredConfirmations: 12,
    credited: false,
    released: false,
    attestationReady: false,
    sourceFailed: false,
  };
}

function sourceLog(address: Address, topics: ReturnType<typeof encodeEventTopics>, data: Hex): ReceiptLog {
  const [ signature, ...indexed ] = topics;
  return {
    address,
    topics: [ signature, ...indexed.map((topic) => {
      if (typeof topic !== 'string') throw new Error('A receipt must contain concrete event topics.');
      return topic;
    }) ],
    data,
    blockHash: zeroHash,
    blockNumber: 100n,
    transactionHash: sourceTxHash,
    transactionIndex: 0,
    logIndex: 7,
    removed: false,
  };
}

function depositLog(transfer: BridgeTransfer, nonce: bigint): ReceiptLog {
  return sourceLog(vault, encodeEventTopics({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    args: { asset: transfer.assetId, sender: transfer.account, paxeerRecipient: paxeerRecipient(recipient) },
  }), encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ BigInt(transfer.amount), nonce ]));
}

function withdrawalLog(transfer: BridgeTransfer, nonce: bigint): ReceiptLog {
  return sourceLog(LAYERX_BRIDGE_ADDRESS, encodeEventTopics({
    abi: layerxBridgeAbi,
    eventName: 'BridgeOut',
    args: { chain: BigInt(transfer.foreignChainId), asset: transfer.assetId, nonce },
  }), encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ BigInt(transfer.amount), recipient ]));
}

function sourceReceipt(transfer: BridgeTransfer, logs: Array<ReceiptLog>): TransactionReceipt {
  return {
    blockHash: zeroHash,
    blockNumber: 100n,
    contractAddress: null,
    cumulativeGasUsed: 95000n,
    effectiveGasPrice: 1000000000n,
    from: account,
    gasUsed: 95000n,
    logs,
    logsBloom: `0x${ '00'.repeat(256) }`,
    status: 'success',
    to: transfer.direction === 'in' ? vault : LAYERX_BRIDGE_ADDRESS,
    transactionHash: sourceTxHash,
    transactionIndex: 0,
    type: 'eip1559',
  };
}

function observeTransfer(transfer: BridgeTransfer, live: TransferLive | undefined, ready: boolean) {
  // transfers-panel.tsx uses this query selector before passing live results to phaseOf.
  const phase = phaseOf(transfer, live === undefined ? undefined : decodeTransferLive(live), ready);
  const step = stepOf(phase);
  const status = historyStatusOf(phase);
  const type = transfer.direction === 'in' ? 'deposit' : 'withdraw';
  return {
    phase,
    step,
    status,
    chip: renderToStaticMarkup(createElement(DepositStatus, {
      items: [ { id: transfer.id, type, token: transfer.symbol, amount: 1.25, step } ],
    })),
    history: renderToStaticMarkup(createElement(TransferHistory, {
      records: [ { id: transfer.id, type, token: transfer.symbol, amount: 1.25, status, time: '12:00' } ],
    })),
  };
}

function proofSettlementDepositReachesReceiptAndStatusConsumers(): void {
  const transfer = sourceTransfer('in');
  const live = sourceLive();
  for (const nonce of [ 9n, 10n ]) {
    const receipt = sourceReceipt(transfer, [ depositLog(transfer, nonce) ]);
    for (let read = 0; read < 2; read++) {
      const produced = matchDepositLog(transfer, receipt.logs);
      expect(produced?.logIndex).toBe(7);
      expect(produced?.args.nonce).toBe(nonce);
      const consumed = enrichRecord(transfer, receipt);
      expect(consumed).toEqual({ ...transfer, logIndex: '7', nonce: String(nonce) });
      expect(transfer.logIndex).toBeUndefined();

      const pending = observeTransfer(consumed, live, false);
      expect(pending.phase).toBe(settlementPhaseOf(consumed, live, false));
      expect(pending.step).toBe('attesting');
      expect(pending.status).toBe('processing');
      expect(pending.chip).toContain('You have a pending transaction');
      expect(pending.history).toContain('Processing');

      const credited = { ...live, credited: true };
      const patch = settlementPatch(consumed, credited, 2000);
      expect(patch).toEqual({ claimedAt: 2000 });
      const settled = { ...consumed, ...patch };
      expect(settlementPatch(settled, credited, 3000)).toBeUndefined();
      // No storage belongs to this anchor; consume the actual patch again without live data.
      const observed = observeTransfer(settled, undefined, false);
      expect(settlementPhaseOf(settled, undefined, false)).toBe('settled');
      expect(observed.phase).toBe('settled');
      expect(observed.step).toBe('credited');
      expect(observed.status).toBe('completed');
      expect(observed.chip).toContain('You have a successful transaction');
      expect(observed.history).toContain('Completed');
      expect(settledRefusal(settled, undefined)).toBe('This deposit is already credited on Paxeer.');
      expect(settledRefusal(consumed, credited)).toBe('This deposit is already credited on Paxeer.');
      expect(settledRefusal(consumed, live)).toBeUndefined();
      expect(settlementPatch(consumed, live, 2000)).toBeUndefined();
    }
  }
}

function proofSettlementWithdrawalReachesWalletAndStatusConsumers(): void {
  const transfer = sourceTransfer('out');
  const live = sourceLive();
  expect(releaseCheckOf(transfer)).toBeUndefined();
  for (const nonce of [ 0n, 42n ]) {
    const receipt = sourceReceipt(transfer, [ withdrawalLog(transfer, nonce) ]);
    for (let read = 0; read < 2; read++) {
      expect(nonceFromBridgeOutLogs(receipt.logs)).toBe(nonce);
      expect(bridgeOutNonceFromReceipt(receipt.logs)).toBe(nonce);
      const consumed = enrichRecord(transfer, receipt);
      expect(consumed.bridgeOutNonce).toBe(String(nonce));
      expect(transfer.bridgeOutNonce).toBeUndefined();
      const request = releaseCheckOf(consumed);
      if (!request) throw new Error('The enriched withdrawal must produce a release-state request.');
      expect(request.address).toBe(vault);
      expect(request.chainId).toBe(1);
      expect(decodeFunctionData({ abi: request.abi, data: encodeFunctionData(request) })).toEqual({
        functionName: 'nullified',
        args: [ keccak256(encodePacked([ 'bytes32', 'uint64' ], [ sourceTxHash, nonce ])) ],
      });
      expect(releaseCheckOf({ ...consumed, foreignChainKey: 'unknown' })).toBeUndefined();
      expect(releaseCheckOf({ ...consumed, vault: undefined })).toBeUndefined();

      const ready = observeTransfer(consumed, live, true);
      expect(ready.phase).toBe('ready');
      expect(ready.step).toBe('ready');
      expect(ready.status).toBe('processing');
      expect(settledRefusal(consumed, live)).toBeUndefined();
      const released = { ...live, released: true };
      const observed = observeTransfer(consumed, released, true);
      expect(settlementPhaseOf(consumed, released, true)).toBe('settled');
      expect(observed.step).toBe('credited');
      expect(observed.status).toBe('completed');
      expect(observed.chip).toContain('You have a successful transaction');
      expect(observed.history).toContain('Completed');
      expect(settledRefusal(consumed, released)).toBe('This withdrawal is already released from the vault.');
      expect(settlementPatch(consumed, released, 2000)).toBeUndefined();
      expect(phaseOf({ ...consumed, releasedAt: 0 }, undefined, false)).toBe('settled');
      expect(settledRefusal({ ...consumed, releasedAt: 0 }, undefined)).toBe('This withdrawal is already released from the vault.');
    }
  }
}

function proofSettlementRefusesMalformedAndUnrelatedReceiptResults(): void {
  const deposit = sourceTransfer('in');
  const withdrawal = sourceTransfer('out');
  const depositEvent = depositLog(deposit, 9n);
  const withdrawalEvent = withdrawalLog(withdrawal, 42n);
  const malformedDeposit: ReceiptLog = { ...depositEvent, data: '0x01' };
  const malformedWithdrawal: ReceiptLog = { ...withdrawalEvent, data: '0x01' };
  const removedDeposit: ReceiptLog = { ...depositEvent, removed: true };
  const removedWithdrawal: ReceiptLog = { ...withdrawalEvent, removed: true };
  const unrelatedDeposit: ReceiptLog = { ...depositEvent, transactionHash: zeroHash };
  const wrongSender = depositLog({ ...deposit, account: other }, 9n);
  for (let read = 0; read < 2; read++) {
    for (const log of [ malformedDeposit, removedDeposit, unrelatedDeposit, wrongSender ]) {
      expect(matchDepositLog(deposit, [ log ])).toBeUndefined();
      expect(enrichRecord(deposit, sourceReceipt(deposit, [ log ]))).toBe(deposit);
    }
    for (const log of [ malformedWithdrawal, removedWithdrawal, { ...withdrawalEvent, address: other } ]) {
      expect(nonceFromBridgeOutLogs([ log ])).toBeUndefined();
      expect(bridgeOutNonceFromReceipt([ log ])).toBeUndefined();
      const consumed = enrichRecord(withdrawal, sourceReceipt(withdrawal, [ log ]));
      expect(consumed).toBe(withdrawal);
      expect(releaseCheckOf(consumed)).toBeUndefined();
    }
    // A malformed first event must not mask a later valid producer result.
    expect(enrichRecord(deposit, sourceReceipt(deposit, [ malformedDeposit, depositEvent ])).nonce).toBe('9');
    expect(enrichRecord(withdrawal, sourceReceipt(withdrawal, [ malformedWithdrawal, withdrawalEvent ])).bridgeOutNonce).toBe('42');
  }
}

function proofSettlementStatusConsumersObserveEveryPhaseAfresh(): void {
  for (const direction of [ 'in', 'out' ] as const) {
    const transfer = sourceTransfer(direction);
    const live = sourceLive();
    for (let read = 0; read < 2; read++) {
      const confirming = observeTransfer(transfer, undefined, false);
      expect(confirming.phase).toBe('confirming');
      expect(confirming.step).toBe('confirming');
      expect(confirming.status).toBe('processing');
      expect(stepOf('settling')).toBe('submitted');
      expect(historyStatusOf('settling')).toBe('processing');
      const failed = observeTransfer(transfer, { ...live, sourceFailed: true }, true);
      expect(failed.phase).toBe('failed');
      expect(failed.step).toBe('failed');
      expect(failed.status).toBe('failed');
      expect(failed.chip).toContain('A transaction failed');
      expect(failed.history).toContain('Failed');
      expect(observeTransfer(transfer, live, true).phase).toBe('ready');
      expect(observeTransfer(transfer, live, false).phase).toBe('attesting');
      expect(observeTransfer(transfer, { ...live, confirmations: 1 }, false).phase).toBe('confirming');
    }
  }
}

function proofSettlementLiveConsumerRefusesMalformedProducerResults(): void {
  const transfer = sourceTransfer('out');
  for (let read = 0; read < 2; read++) {
    const live = sourceLive();
    const decoded = decodeTransferLive(live);
    expect(decoded).toEqual(live);
    expect(decoded).not.toBe(live);
    expect(observeTransfer(transfer, decoded, false).phase).toBe('attesting');
    live.released = true;
    expect(decodeTransferLive(live).released).toBe(true);
    expect(decoded.released).toBe(false);
    expect(observeTransfer(transfer, decodeTransferLive(live), false).phase).toBe('settled');
    for (const malformed of [
      null,
      { ...live, released: 'true' },
      { ...live, credited: undefined },
      { ...live, confirmations: -1 },
      { ...live, confirmations: Number.NaN },
      { ...live, requiredConfirmations: 0.5 },
    ]) {
      expect(() => decodeTransferLive(malformed)).toThrow('Invalid live transfer state.');
    }
  }
}

test(proofSettlementDepositReachesReceiptAndStatusConsumers.name, proofSettlementDepositReachesReceiptAndStatusConsumers);
test(proofSettlementWithdrawalReachesWalletAndStatusConsumers.name, proofSettlementWithdrawalReachesWalletAndStatusConsumers);
test(proofSettlementRefusesMalformedAndUnrelatedReceiptResults.name, proofSettlementRefusesMalformedAndUnrelatedReceiptResults);
test(proofSettlementStatusConsumersObserveEveryPhaseAfresh.name, proofSettlementStatusConsumersObserveEveryPhaseAfresh);
test(proofSettlementLiveConsumerRefusesMalformedProducerResults.name, proofSettlementLiveConsumerRefusesMalformedProducerResults);

test(settlementSubmissionReopensThroughBroadcastRevertAndConfirmation.name, settlementSubmissionReopensThroughBroadcastRevertAndConfirmation);
