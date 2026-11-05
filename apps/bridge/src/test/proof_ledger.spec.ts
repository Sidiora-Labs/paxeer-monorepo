/**
 * Proves: submitted.ts depositRecord/withdrawalRecord, import.ts importDeposit/importWithdrawal,
 * enrich.ts enrichRecord -> transfers.ts transferIdOf/saveTransfer/localStorage/listTransfers ->
 * import-transfer.tsx ImportTransfer/importTransferReceipt and this consumer.
 * Repairs: transfers.ts isBridgeTransfer accepted malformed decimal, identity and optional fields;
 * enrich.ts enrichRecord accepted reverted/unrelated receipts and missed partial deposit identity;
 * import.ts imported removed logs. The component's receipt consumer is shared with this proof.
 * Receipt inputs use the real contract ABI encoders; every ledger row comes from a real producer.
 */
// @vitest-environment jsdom

import { ledgerWithdrawalReopensIntoAuthoritativeReleaseVerification } from './repair-22-33-266.proofs';

import { ImportTransfer, importTransferReceipt } from '@/components/bridge/import-transfer';
import { bridgeChainByKey, buildBridgeChains, type BridgeChain } from '@/lib/bridge/chains';
import { readConfig } from '@/lib/bridge/config';
import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from '@/lib/bridge/contracts';
import { paxeerRecipient } from '@/lib/bridge/digests';
import { enrichRecord } from '@/lib/bridge/enrich';
import { importDeposit, importWithdrawal } from '@/lib/bridge/import';
import { depositRecord, withdrawalRecord } from '@/lib/bridge/submitted';
import { listTransfers, saveTransfer, transferIdOf, updateTransfer, type BridgeTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { createElement } from 'react';
import type { Address, Hex, Log, TransactionReceipt } from 'viem';
import { encodeAbiParameters, encodeEventTopics, getAddress, zeroAddress, zeroHash } from 'viem';
import { WagmiProvider } from 'wagmi';

import { afterEach, beforeEach, expect, test, vi } from 'vitest';

const storageKey = 'paxeer-bridge:transfers:v1';
const account: Address = '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const recipient: Address = '0xdddddddddddddddddddddddddddddddddddddddd';
const vault: Address = '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const other: Address = '0x000000000000000000000000000000000000dead';
const hash: Hex = `0x${ 'Ab'.repeat(32) }`;
const otherHash: Hex = `0x${ 'cd'.repeat(32) }`;
const amount = 9007199254740993n;

function configuredBase(): BridgeChain {
  const chain = buildBridgeChains(readConfig({ NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault })).find((entry) => entry.key === 'base');
  if (!chain) throw new Error('Base must be registered.');
  return chain;
}

function sourceReceipt(direction: 'in' | 'out', nonce = 9n): TransactionReceipt {
  const topics = direction === 'in' ? encodeEventTopics({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    args: { asset: zeroAddress, sender: account, paxeerRecipient: paxeerRecipient(recipient) },
  }) : encodeEventTopics({
    abi: layerxBridgeAbi,
    eventName: 'BridgeOut',
    args: { chain: 8453n, asset: zeroAddress, nonce },
  });
  const [ signature, ...indexed ] = topics;
  const log: Log<bigint, number, false> = {
    address: direction === 'in' ? vault : LAYERX_BRIDGE_ADDRESS,
    topics: [ signature, ...indexed.map((topic) => {
      if (typeof topic !== 'string') throw new Error('Expected a concrete indexed event argument.');
      return topic;
    }) ],
    data: direction === 'in' ?
      encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ amount, nonce ]) :
      encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ amount, recipient ]),
    logIndex: 7,
    blockHash: zeroHash,
    blockNumber: 400n,
    transactionHash: hash,
    transactionIndex: 0,
    removed: false,
  };
  return {
    blockHash: log.blockHash,
    blockNumber: log.blockNumber,
    contractAddress: null,
    cumulativeGasUsed: 95000n,
    effectiveGasPrice: 1000000000n,
    from: account,
    gasUsed: 95000n,
    logs: [ log ],
    logsBloom: `0x${ '00'.repeat(256) }`,
    status: 'success',
    to: log.address,
    transactionHash: hash,
    transactionIndex: 0,
    type: 'eip1559',
  };
}

function submitted(direction: 'in' | 'out'): BridgeTransfer {
  const foreign = configuredBase();
  const input = { hash, foreign, asset: foreign.assets[0], account, amount, requiredConfirmations: direction === 'in' ? 1800 : 1 };
  return direction === 'in' ? depositRecord({ ...input, vault, recipient }) : withdrawalRecord({ ...input, destination: recipient });
}

async function observeAfterReopen(record: BridgeTransfer): Promise<void> {
  saveTransfer(record);
  expect(JSON.parse(window.localStorage.getItem(storageKey)!)).toContainEqual(record);
  // Unload and reopen the actual consumer module, retaining the browser's actual localStorage.
  vi.resetModules();
  const reopened = await import('@/lib/bridge/transfers');
  const first = reopened.listTransfers(getAddress(account)).find((entry) => entry.id === record.id);
  const second = reopened.listTransfers(account).find((entry) => entry.id === record.id);
  expect(first).toEqual(record);
  expect(second).toEqual(record);
  expect(second).not.toBe(first);
  if (!first) throw new Error('The reopened ledger lost the producer record.');
  first.amount = '1';
  expect(reopened.listTransfers(account).find((entry) => entry.id === record.id)).toEqual(record);
  expect(reopened.listTransfers(other)).toEqual([]);
}

beforeEach(() => window.localStorage.clear());
afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

async function submittedProducersCrossTheLedgerAndSurviveReopening(): Promise<void> {
  for (const direction of [ 'in', 'out' ] as const) {
    const record = submitted(direction);
    expect(record.id).toBe(`${ hash.toLowerCase() }:${ direction }`);
    expect(transferIdOf(direction, hash)).toBe(record.id);
    expect(record.amount).toBe('9007199254740993');
    expect(record.foreignChainId).toBe('8453');
    expect(record.requiredConfirmations).toBe(direction === 'in' ? 1800 : 1);
    expect(record.recipient).toBe(recipient);
    expect(record.vault).toBe(direction === 'in' ? vault : undefined);
    await observeAfterReopen(record);
    const next = { ...record, amount: (amount + 1n).toString() };
    updateTransfer(record.id, { amount: next.amount });
    const consumer = await import('@/lib/bridge/transfers');
    expect(consumer.listTransfers(account).find((entry) => entry.id === record.id)).toEqual(next);
  }
  expect(listTransfers(account)).toHaveLength(2);
  expect(transferIdOf('in', hash)).not.toBe(transferIdOf('out', hash));
  const solana = bridgeChainByKey('solana');
  if (!solana) throw new Error('Solana must be registered.');
  const destination = 'So11111111111111111111111111111111111111112';
  const record = withdrawalRecord({
    hash: otherHash, foreign: solana, asset: solana.assets[0], account, amount, requiredConfirmations: 1, destination,
  });
  await observeAfterReopen(record);
  expect(listTransfers(account).find((entry) => entry.id === record.id)?.recipient).toBe(destination);
}

async function receiptProducersReachTheComponentsRealConsumerTwice(): Promise<void> {
  for (const direction of [ 'in', 'out' ] as const) {
    const receipt = sourceReceipt(direction);
    const chain = direction === 'in' ? configuredBase() : undefined;
    const produced = chain ? importDeposit(chain, receipt) : importWithdrawal(receipt);
    expect(produced).toMatchObject({
      id: transferIdOf(direction, hash), direction, account: direction === 'in' ? getAddress(account) : account, recipient: getAddress(recipient),
      assetId: zeroAddress, amount: amount.toString(), foreignChainKey: 'base', foreignChainId: '8453',
    });
    expect(direction === 'in' ? produced.nonce : produced.bridgeOutNonce).toBe('9');
    if (direction === 'in') expect(produced.logIndex).toBe('7');
    const imported = importTransferReceipt(chain, receipt, account);
    expect(imported.existing).toBe(false);
    expect(imported.transfer).toMatchObject({ ...produced, submittedAt: expect.any(Number) });
    await observeAfterReopen(imported.transfer);
    const patch = direction === 'in' ? { claimedAt: 1000, claimTxHash: otherHash } : { releasedAt: 1000, releaseTxHash: otherHash };
    updateTransfer(produced.id, patch);
    const repeated = importTransferReceipt(chain, sourceReceipt(direction), account);
    expect(repeated.existing).toBe(true);
    expect(repeated.transfer).toEqual({ ...imported.transfer, ...patch });
    expect(listTransfers(account).filter((entry) => entry.id === produced.id)).toEqual([ repeated.transfer ]);
  }
}

function componentConsumerRefusesTheWrongWalletWithoutWriting(): void {
  for (const direction of [ 'in', 'out' ] as const) {
    const chain = direction === 'in' ? configuredBase() : undefined;
    expect(() => importTransferReceipt(chain, sourceReceipt(direction), other)).toThrow('Connect the wallet that started this transfer');
    expect(listTransfers()).toEqual([]);
    expect(window.localStorage.getItem(storageKey)).toBeNull();
  }
}

async function enrichmentCrossesTheLedgerButRefusesUnrelatedOrRevertedReceipts(): Promise<void> {
  for (const direction of [ 'in', 'out' ] as const) {
    const record = submitted(direction);
    const receipt = sourceReceipt(direction);
    expect(enrichRecord(record, { ...receipt, transactionHash: otherHash })).toBe(record);
    expect(enrichRecord(record, { ...receipt, status: 'reverted' })).toBe(record);
    expect(enrichRecord(record, { ...receipt, logs: [] })).toBe(record);
    const enriched = enrichRecord(record, receipt);
    expect(direction === 'in' ? enriched.nonce : enriched.bridgeOutNonce).toBe('9');
    expect(direction === 'in' ? record.logIndex : record.bridgeOutNonce).toBeUndefined();
    await observeAfterReopen(enriched);
    const repeated = enrichRecord(record, sourceReceipt(direction, 10n));
    expect(direction === 'in' ? repeated.nonce : repeated.bridgeOutNonce).toBe('10');
    await observeAfterReopen(repeated);
    expect(enrichRecord(enriched, sourceReceipt(direction, 10n))).toBe(enriched);
    if (direction === 'in') {
      expect(enrichRecord({ ...record, logIndex: '7' }, receipt)).toEqual(enriched);
      expect(enrichRecord({ ...record, recipient: other }, receipt).nonce).toBeUndefined();
    }
  }
}

function receiptDecodersRefuseRemovedAndMalformedLogs(): void {
  for (const direction of [ 'in', 'out' ] as const) {
    const receipt = sourceReceipt(direction);
    const decode = (value: TransactionReceipt) => direction === 'in' ? importDeposit(configuredBase(), value) : importWithdrawal(value);
    for (const logs of [ [], receipt.logs.map((log) => ({ ...log, removed: true })), receipt.logs.map((log) => ({ ...log, data: '0x' as const })) ]) {
      const invalid = { ...receipt, logs };
      expect(() => decode(invalid)).toThrow(direction === 'in' ? 'No BridgeDeposit log' : 'No valid BridgeOut log');
      expect(enrichRecord(submitted(direction), invalid)).toEqual(submittedWithoutTimestamp(direction));
    }
    expect(() => decode({ ...receipt, status: 'reverted' })).toThrow('reverted');
  }
  expect(listTransfers()).toEqual([]);
}

function submittedWithoutTimestamp(direction: 'in' | 'out') {
  return { ...submitted(direction), submittedAt: expect.any(Number) };
}

async function malformedProducerRowsAreRefusedAtTheRealConsumerDecoder(): Promise<void> {
  const valid = enrichRecord(submitted('in'), sourceReceipt('in'));
  saveTransfer(valid);
  const raw = window.localStorage.getItem(storageKey);
  if (!raw) throw new Error('The producer was not stored.');
  const rows: Array<BridgeTransfer> = JSON.parse(raw);
  const invalidFields: Record<string, unknown> = {
    id: `${ otherHash }:in`, account: 'wallet', amount: 'NaN', foreignChainId: '-1', sourceTxHash: '0x1234',
    decimals: -1, requiredConfirmations: 0, submittedAt: -1, assetId: 'asset', vault: 'vault',
    recipient: '', logIndex: 'not-an-index', nonce: '-9', bridgeOutNonce: {}, claimTxHash: '0x', releasedAt: 'yesterday',
    destinationHeldBack: 'yes',
  };
  for (const [ field, value ] of Object.entries(invalidFields)) {
    // Corrupt a real producer result at the serialization boundary, preserving the real decoder.
    window.localStorage.setItem(storageKey, JSON.stringify([ { ...rows[0], [field]: value }, valid ]));
    vi.resetModules();
    const consumer = await import('@/lib/bridge/transfers');
    expect(consumer.listTransfers(account), field).toEqual([ valid ]);
    expect(consumer.listTransfers(account), field).toEqual([ valid ]);
  }
}

function importFormExecutesWithRealProvidersAndRefusesMalformedInput(): void {
  let imported = 0;
  const onImported = () => {
    imported++;
  };
  for (let attempt = 0; attempt < 2; attempt++) {
    const view = render(createElement(WagmiProvider, { config: wagmiConfig }, createElement(ImportTransfer, { onImported })));
    const form = screen.getByRole('form', { name: 'Import a bridge transfer' });
    const input = screen.getByRole('textbox', { name: 'Source transaction hash' });
    fireEvent.change(input, { target: { value: '0xbroken' } });
    fireEvent.submit(form);
    expect(screen.getByRole('alert').textContent).toBe('Enter a source transaction hash with 64 hexadecimal characters after 0x.');
    fireEvent.change(input, { target: { value: hash } });
    fireEvent.submit(form);
    expect(screen.getByRole('alert').textContent).toBe('Connect the wallet that started the transfer.');
    expect(listTransfers()).toEqual([]);
    view.unmount();
  }
  expect(imported).toBe(0);
}

test(submittedProducersCrossTheLedgerAndSurviveReopening.name, submittedProducersCrossTheLedgerAndSurviveReopening);
test(receiptProducersReachTheComponentsRealConsumerTwice.name, receiptProducersReachTheComponentsRealConsumerTwice);
test(componentConsumerRefusesTheWrongWalletWithoutWriting.name, componentConsumerRefusesTheWrongWalletWithoutWriting);
test(enrichmentCrossesTheLedgerButRefusesUnrelatedOrRevertedReceipts.name, enrichmentCrossesTheLedgerButRefusesUnrelatedOrRevertedReceipts);
test(receiptDecodersRefuseRemovedAndMalformedLogs.name, receiptDecodersRefuseRemovedAndMalformedLogs);
test(malformedProducerRowsAreRefusedAtTheRealConsumerDecoder.name, malformedProducerRowsAreRefusedAtTheRealConsumerDecoder);
test(importFormExecutesWithRealProvidersAndRefusesMalformedInput.name, importFormExecutesWithRealProvidersAndRefusesMalformedInput);

test(ledgerWithdrawalReopensIntoAuthoritativeReleaseVerification.name, ledgerWithdrawalReopensIntoAuthoritativeReleaseVerification);
