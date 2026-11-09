import { readFileSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';
import {
  authenticateSequencerReceipt,
  decodeSequencerReceipt,
  readReceiptKeyPins,
  requireAuthenticatedReceipt,
  LEDGER_PRECOMPILE,
  type SequencerReceiptAnswer,
} from '@paxeer/wallet';
import { ethers } from 'ethers';
import { keccak256, hexToBytes, bytesToHex } from 'viem';
import { verifySequencerReceipt } from '@sidiora/layerx-sdk/browser';
import { CHAIN_KEY, SurfaceChain, nodeReceiptCall, nodeReceiptVectors } from './chain';
import {
  EXCHANGE_RECORDING_KEY,
  RecordedBridgeChain,
  RecordedSurfaceChain,
  recordedLedgerProof,
  surfaceReceiptProvenance,
  surfaceReceiptVectors,
} from './recordedReceipts';
import { decodePayerGrantIssued } from '../useSurface';

function repair_22_39_55_every_baseline_assertion_has_an_explicit_settlement() {
  const record = readFileSync(path.resolve(__dirname, '../repair-22-39-55.kvx'), 'utf8');
  const sections = record.split(/(?=^\[baseline\.\d+\]$)/m);
  const metadata = sections.shift()!;
  expect(metadata).toContain('baseline_count = 27');
  expect(metadata).toContain('starting_revision = "a9167fbc68cb22ef78b223a753236832c99cda2f"');
  expect(metadata).toContain('GOTCHA.kvx [gotcha.3433]');
  expect(metadata).toContain('not a fresh starting-revision run');
  expect(sections).toHaveLength(27);
  const identities = new Set<string>();
  const suites = new Map<string, number>();
  for (const [index, rawSection] of sections.entries()) {
    const section = rawSection.split(/\n\[/, 1)[0];
    expect(section.startsWith(`[baseline.${index + 1}]\n`)).toBe(true);
    const entry = Object.fromEntries(
      Array.from(section.matchAll(/^(\w+) = (".*")$/gm), ([, key, value]) => [
        key,
        JSON.parse(value) as string,
      ])
    );
    expect(Object.keys(entry).sort()).toEqual(['failure', 'file', 'settlement', 'status', 'test']);
    expect(entry.failure.length).toBeGreaterThan(0);
    expect(entry.settlement.length).toBeGreaterThan(0);
    expect(entry.status, entry.test).toBe('settled');
    expect(entry.settlement).not.toMatch(/^Blocked: /);
    expect(entry.file).toMatch(/^apps\/wallet\/src\/surfaces\/(?:test\/)?[\w.-]+\.test\.tsx?$/);
    const source = readFileSync(path.resolve(__dirname, '../../../../..', entry.file), 'utf8');
    expect(source, entry.test).toContain(entry.test);
    const identity = `${entry.file}:${entry.test}`;
    expect(identities.has(identity), identity).toBe(false);
    identities.add(identity);
    suites.set(entry.file, (suites.get(entry.file) ?? 0) + 1);
  }
  expect(Object.fromEntries(suites)).toEqual({
    'apps/wallet/src/surfaces/WebDataView.test.tsx': 18,
    'apps/wallet/src/surfaces/ExchangeView.test.tsx': 4,
    'apps/wallet/src/surfaces/BridgeView.test.tsx': 1,
    'apps/wallet/src/surfaces/FeeChoice.test.tsx': 1,
    'apps/wallet/src/surfaces/LaunchpadView.test.tsx': 1,
    'apps/wallet/src/surfaces/test/chain.test.tsx': 1,
    'apps/wallet/src/surfaces/test/repair-22-33-206.test.ts': 1,
  });
}

test(
  repair_22_39_55_every_baseline_assertion_has_an_explicit_settlement.name,
  repair_22_39_55_every_baseline_assertion_has_an_explicit_settlement
);

async function repair_22_39_55_surface_receipt_fixtures_match_the_live_wallet_sdk_shape() {
  const exchangeAddress = new ethers.Wallet(EXCHANGE_RECORDING_KEY).address.toLowerCase();
  const entries = [...nodeReceiptVectors(), ...surfaceReceiptVectors()];
  expect(new Set(entries.map((entry) => entry.id)).size).toBe(entries.length);
  for (const entry of entries) {
    const chain = entry.id.startsWith('V')
      ? new SurfaceChain()
      : new RecordedSurfaceChain({
          recordingIds: [entry.id],
          privateKey:
            entry.receiptJson.from === exchangeAddress ? EXCHANGE_RECORDING_KEY : CHAIN_KEY,
        });
    const fixture: SequencerReceiptAnswer = entry.receiptJson;
    expect(keccak256(entry.rawTransaction), entry.id).toBe(fixture.txHash);
    const answer = await chain.request({
      method: 'paxeer_sendTransactionReceipted',
      params: [nodeReceiptCall(entry)],
    });
    const rpc = (method: string, params: readonly unknown[]) => chain.request({ method, params });
    const pins = await readReceiptKeyPins(rpc);
    const decoded = decodeSequencerReceipt(answer, fixture.txHash, pins);
    expect(decoded, entry.id).toEqual(fixture);
    expect(Object.keys(decoded).sort()).toEqual(Object.keys(fixture).sort());
    const authenticated = await authenticateSequencerReceipt(rpc, answer, fixture.txHash);
    requireAuthenticatedReceipt(authenticated);
    expect(authenticated.authentication.rawTransaction).toBe(entry.rawTransaction);
    expect(authenticated.authentication.receiptDigest).toBe(entry.receiptDigest);
    expect(authenticated.receipt).toBe(entry.signedReceipt);
    expect(authenticated.rung).toBe('receipted');
    expect(authenticated).not.toHaveProperty('blockNumber');
    expect(authenticated).not.toHaveProperty('blockHash');
    for (const field of Object.keys(fixture)) {
      const incomplete = structuredClone(fixture);
      Reflect.deleteProperty(incomplete, field);
      expect(
        () => decodeSequencerReceipt(incomplete, fixture.txHash, pins),
        `${entry.id}: ${field}`
      ).toThrow();
    }
  }

  const chain = new RecordedBridgeChain('committed-payExact');
  const fixture: SequencerReceiptAnswer = chain.recording.receiptJson;
  const rpc = (method: string, params: readonly unknown[]) => chain.request({ method, params });
  const pins = await readReceiptKeyPins(rpc);
  expect(decodeSequencerReceipt(fixture, fixture.txHash, pins)).toEqual(fixture);
  const authenticated = await authenticateSequencerReceipt(rpc, fixture, fixture.txHash);
  requireAuthenticatedReceipt(authenticated);
  expect(authenticated.rung).toBe('committed');
  if (authenticated.rung !== 'committed') throw new Error('expected committed producer recording');
  expect(authenticated.blockNumber).toBe(fixture.blockNumber);
  expect(authenticated.blockHash).toBe(fixture.blockHash);
  expect(authenticated.blockHash).toMatch(/^0x[0-9a-f]{64}$/);
  expect(BigInt(authenticated.blockNumber)).toBeGreaterThanOrEqual(BigInt(authenticated.height));
  expect(authenticated.authentication.rawTransaction).toBe(chain.recording.rawTransaction);
  for (const field of ['blockNumber', 'blockHash']) {
    const incomplete = structuredClone(fixture);
    Reflect.deleteProperty(incomplete, field);
    expect(() => decodeSequencerReceipt(incomplete, fixture.txHash, pins), field).toThrow();
  }
}

test(
  repair_22_39_55_surface_receipt_fixtures_match_the_live_wallet_sdk_shape.name,
  repair_22_39_55_surface_receipt_fixtures_match_the_live_wallet_sdk_shape
);

async function repair_22_39_55_missing_surface_calls_have_exact_signed_recordings() {
  const entries = surfaceReceiptVectors();
  expect(entries.map(({ id }) => id)).toEqual([
    'perps-approve-50123456',
    'perps-approve-1000000',
    'perps-open-long',
    'perps-open-short',
    'perps-cancel',
    'perps-deposit',
    'sponsored-cancel-1',
    'sponsored-cancel-2',
    'fee-set',
    'fee-clear',
    'launchpad-create-approve',
    'launchpad-buy-approve',
    'launchpad-sell-approve',
    'launchpad-create',
    'launchpad-buy',
    'launchpad-sell',
    'xweb-search',
    'xweb-refund',
    'launchpad-approve-reverted',
    'ledger-pay-exact',
    'ledger-grant-retired',
  ]);
  expect(surfaceReceiptProvenance()).toMatchObject({
    kind: 'documented-node-receipt-model',
    receiptModel: 'spec/layerx-merge/contracts/sequencer.md#sequencer-receipt',
    ledgerModel: 'spec/layerx-merge/contracts/ledger-auth.md#layerxledger-0x101a',
  });
  const exchangeAddress = new ethers.Wallet(EXCHANGE_RECORDING_KEY).address.toLowerCase();
  for (const entry of entries) {
    const chain = new RecordedSurfaceChain({
      recordingIds: [entry.id],
      privateKey: entry.receiptJson.from === exchangeAddress ? EXCHANGE_RECORDING_KEY : CHAIN_KEY,
    });
    const call = nodeReceiptCall(entry);
    for (const changed of [
      { ...call, to: ethers.ZeroAddress },
      { ...call, data: `${call.data}00` },
      { ...call, value: ethers.toQuantity(BigInt(call.value) + 1n) },
    ]) {
      await expect(
        chain.request({ method: 'paxeer_sendTransactionReceipted', params: [changed] })
      ).rejects.toThrow('no node receipt vector for this call');
      expect(chain.sent).toEqual([]);
    }
    chain.logsFor = () => [{ address: call.to, topics: [], data: '0x' }];
    await expect(
      chain.request({ method: 'paxeer_sendTransactionReceipted', params: [call] })
    ).rejects.toThrow('surface logs differ from the signed node receipt');
    expect(chain.sent).toEqual([]);
    chain.logsFor = () => [];
    const answer = await chain.request({
      method: 'paxeer_sendTransactionReceipted',
      params: [call],
    });
    expect(answer).toEqual(entry.receiptJson);
    const rpc = (method: string, params: readonly unknown[]) => chain.request({ method, params });
    for (const field of Object.keys(entry.receiptJson)) {
      const incomplete = structuredClone(entry.receiptJson);
      Reflect.deleteProperty(incomplete, field);
      await expect(
        authenticateSequencerReceipt(rpc, incomplete, entry.receiptJson.txHash),
        `${entry.id}: ${field}`
      ).rejects.toThrow();
    }
    const verified = await verifySequencerReceipt(
      hexToBytes(entry.signedReceipt),
      hexToBytes(entry.rawTransaction),
      [
        {
          keyId: hexToBytes(entry.pinnedKey.keyId),
          publicKey: hexToBytes(entry.pinnedKey.publicKey),
          first_height: BigInt(entry.pinnedKey.firstHeight),
          last_height: BigInt(entry.pinnedKey.lastHeight),
        },
      ]
    );
    expect(verified.receipt.resultCode).toBe(entry.receiptJson.status === '0x1' ? 0 : 1);
    for (const field of ['status', 'receipt', 'returnData', 'logs'] as const) {
      const tampered = structuredClone(entry.receiptJson);
      if (field === 'status') tampered.status = tampered.status === '0x1' ? '0x0' : '0x1';
      if (field === 'receipt') {
        const bytes = hexToBytes(tampered.receipt);
        bytes[bytes.length - 1] ^= 1;
        tampered.receipt = bytesToHex(bytes);
      }
      if (field === 'returnData') tampered.returnData = `${tampered.returnData}00`;
      if (field === 'logs')
        tampered.logs = [
          { address: call.to as `0x${string}`, topics: [], data: '0x', logIndex: '0x0' },
        ];
      await expect(
        authenticateSequencerReceipt(rpc, tampered, entry.receiptJson.txHash),
        `${entry.id}: tampered ${field}`
      ).rejects.toThrow();
    }
  }

  const reverted = entries.find(({ id }) => id === 'launchpad-approve-reverted')!;
  const chain = new RecordedSurfaceChain({ recordingIds: [reverted.id] });
  await chain.request({
    method: 'paxeer_sendTransactionReceipted',
    params: [nodeReceiptCall(reverted)],
  });
  expect(chain.allowances.size).toBe(0);
}

test(
  repair_22_39_55_missing_surface_calls_have_exact_signed_recordings.name,
  repair_22_39_55_missing_surface_calls_have_exact_signed_recordings
);

function repair_22_39_55_ledger_proof_context_is_bound_to_recorded_payment_and_refusal() {
  const proof = recordedLedgerProof();
  const entries = surfaceReceiptVectors();
  const payment = entries.find((entry) => entry.id === proof.paymentReceiptId)!;
  const grant = entries.find((entry) => entry.id === proof.grantReceiptId)!;
  expect(proof.provenance.ledgerModel).toBe(
    'spec/layerx-merge/contracts/ledger-auth.md#layerxledger-0x101a'
  );
  expect(proof.paymentLogs).toEqual(
    payment.receiptJson.logs.map((log) => ({ ...log, transactionHash: payment.receiptJson.txHash }))
  );
  expect(proof.grantLogs).toEqual(grant.receiptJson.logs);
  expect(grant.receiptJson.status).toBe('0x0');
  const errorAbi = new ethers.Interface(['error Error(string)']);
  expect(errorAbi.decodeErrorResult('Error', grant.receiptJson.returnData)[0]).toBe(
    'LXP_NO_PAYER_GRANT'
  );
  expect(proof.grantLogs).toHaveLength(0);
  expect(proof.paymentLogs).toHaveLength(2);
  expect(proof.paymentLogs.map((log) => log.address)).toEqual([
    LEDGER_PRECOMPILE,
    LEDGER_PRECOMPILE,
  ]);
  expect(proof.paymentLogs.map((log) => log.topics[0])).toEqual([
    ethers.id('Transfer(bytes32,bytes32,bytes32,uint128,uint8)'),
    ethers.id('PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)'),
  ]);
  for (const log of proof.paymentLogs)
    expect(() => decodePayerGrantIssued(log)).toThrow('invalid grant log');
}

test(
  repair_22_39_55_ledger_proof_context_is_bound_to_recorded_payment_and_refusal.name,
  repair_22_39_55_ledger_proof_context_is_bound_to_recorded_payment_and_refusal
);
