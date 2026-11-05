/**
 * Proves: parseDepositAttestation/parseReleaseAttestation -> attestation-api.ts exports -> transfer-actions.ts assertClaimMatches/assertReleaseMatches.
 * depositAttestationPath/releaseAttestationPath produce the route identifiers;
 * attestationPollInterval consumes the parser results as the polling hooks do.
 * Repairs readDecimal accepting values outside the uint256 amount/chain and uint64
 * logIndex/paxeerNonce domains consumed by digests.ts. Inputs are signed with real
 * accounts; every attestation consumed below is emitted by the exported parsers.
 * Attestations are stateless. The persistent anchor is transfers.ts localStorage;
 * its module session is closed and reopened before consuming the saved transfers.
 */
// @vitest-environment jsdom

import { attestationWireFixturesSurviveListenerReopening } from './repair-22-33-266.proofs';

import {
  AttestationApiError,
  attestationPollInterval,
  depositAttestationPath,
  parseDepositAttestation,
  parseReleaseAttestation,
  releaseAttestationPath,
} from '@/lib/bridge/attestation-api';
import { inboundDigest, outboundDigest, paxeerRecipient, verifyThresholdSignatures } from '@/lib/bridge/digests';
import { listTransfers, saveTransfer, transferIdOf } from '@/lib/bridge/transfers';
import type { BridgeTransfer } from '@/lib/bridge/transfers';

import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { expect, test, vi } from 'vitest';

async function produceAttestations() {
  const signer = privateKeyToAccount(generatePrivateKey());
  const txHash = '0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' as const;
  const paxeerTxHash = '0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB' as const;
  const vault = '0x1111111111111111111111111111111111111111' as const;
  const recipient = '0x2222222222222222222222222222222222222222' as const;
  const asset = '0x3333333333333333333333333333333333333333' as const;
  const amount = (1n << 256n) - 1n;
  const index = (1n << 64n) - 1n;
  const depositRequest = { chainId: 8453n, txHash, logIndex: index };
  const releaseRequest = { chainId: 8453n, paxeerTxHash, paxeerNonce: index };
  const depositDigest = inboundDigest({
    ...depositRequest, vault, recipient32: paxeerRecipient(recipient), asset, amount,
  });
  const releaseDigest = outboundDigest({ ...releaseRequest, vault, recipient, asset, amount });
  const common = { chainId: '8453', vault, asset, amount: amount.toString(), status: 'ready' };
  const depositBody = {
    ...common,
    txHash,
    logIndex: index.toString(),
    recipient: paxeerRecipient(recipient),
    digest: depositDigest,
    signatures: [ await signer.sign({ hash: depositDigest }) ],
  };
  const releaseBody = {
    ...common,
    paxeerTxHash,
    paxeerNonce: index.toString(),
    recipient,
    digest: releaseDigest,
    signatures: [ await signer.sign({ hash: releaseDigest }) ],
  };
  const depositTransfer: BridgeTransfer = {
    id: transferIdOf('in', txHash),
    direction: 'in',
    foreignChainKey: 'base',
    foreignChainId: '8453',
    symbol: 'TEST',
    decimals: 18,
    amount: amount.toString(),
    assetId: asset,
    vault,
    account: signer.address,
    recipient,
    sourceTxHash: txHash,
    requiredConfirmations: 12,
    submittedAt: 1,
    logIndex: index.toString(),
  };
  const releaseTransfer: BridgeTransfer = {
    ...depositTransfer,
    id: transferIdOf('out', paxeerTxHash),
    direction: 'out',
    sourceTxHash: paxeerTxHash,
    logIndex: undefined,
    bridgeOutNonce: index.toString(),
    submittedAt: 2,
  };
  return { signer, depositRequest, releaseRequest, depositBody, releaseBody, depositTransfer, releaseTransfer };
}

async function provesAttestationConsumersAfterReopeningTheTransferStore() {
  const state = await produceAttestations();
  const deposit = parseDepositAttestation(state.depositRequest, state.depositBody);
  const release = parseReleaseAttestation(state.releaseRequest, state.releaseBody);
  expect(depositAttestationPath(state.depositRequest))
    .toBe(`/v1/deposit/8453/${ state.depositRequest.txHash.toLowerCase() }/18446744073709551615`);
  expect(releaseAttestationPath(state.releaseRequest))
    .toBe(`/v1/release/8453/${ state.releaseRequest.paxeerTxHash.toLowerCase() }/18446744073709551615`);
  expect(deposit).toEqual({
    chainId: '8453',
    vault: state.depositBody.vault,
    txHash: state.depositBody.txHash,
    logIndex: '18446744073709551615',
    recipient32: state.depositBody.recipient,
    asset: state.depositBody.asset,
    amount: state.depositBody.amount,
    digest: state.depositBody.digest,
    signatures: state.depositBody.signatures,
    status: 'ready',
  });
  expect(release).toEqual(state.releaseBody);

  const storageKey = 'paxeer-bridge:transfers:v1';
  const previous = window.localStorage.getItem(storageKey);
  try {
    window.localStorage.removeItem(storageKey);
    saveTransfer(state.depositTransfer);
    saveTransfer(state.releaseTransfer);
    const saved = listTransfers(state.signer.address);
    expect(saved).toEqual([ state.releaseTransfer, state.depositTransfer ]);
    const persisted = window.localStorage.getItem(storageKey);
    expect(persisted).not.toBeNull();

    vi.resetModules();
    const reopened = await import('@/lib/bridge/transfers');
    const consumers = await import('@/components/bridge/transfer-actions');
    const api = await import('@/lib/bridge/attestation-api');
    expect(reopened.listTransfers).not.toBe(listTransfers);
    expect(reopened.listTransfers(state.signer.address)).toEqual(saved);
    expect(window.localStorage.getItem(storageKey)).toBe(persisted);
    const restoredDeposit = reopened.listTransfers(state.signer.address).find((entry) => entry.direction === 'in');
    const restoredRelease = reopened.listTransfers(state.signer.address).find((entry) => entry.direction === 'out');
    if (!restoredDeposit || !restoredRelease) throw new Error('Both transfers must survive reopening.');
    expect(() => consumers.assertClaimMatches(restoredDeposit, deposit)).not.toThrow();
    expect(() => consumers.assertReleaseMatches(restoredRelease, release)).not.toThrow();
    for (const attestation of [ deposit, release ]) {
      expect(api.attestationPollInterval(attestation, 12_000)).toBe(false);
      expect(await verifyThresholdSignatures({
        digest: attestation.digest,
        signatures: attestation.signatures,
        attestors: [ state.signer.address ],
        threshold: 1,
      })).toEqual({ ok: true, signers: [ state.signer.address ] });
    }

    reopened.saveTransfer(state.depositTransfer);
    reopened.saveTransfer(state.releaseTransfer);
    expect(reopened.listTransfers(state.signer.address)).toEqual(saved);
    expect(window.localStorage.getItem(storageKey)).toBe(persisted);
    expect(api.parseDepositAttestation(state.depositRequest, state.depositBody)).toEqual(deposit);
    expect(api.parseReleaseAttestation(state.releaseRequest, state.releaseBody)).toEqual(release);

    const wrongRecipient = '0x4444444444444444444444444444444444444444' as const;
    const mismatchedDeposit = api.parseDepositAttestation(state.depositRequest, {
      ...state.depositBody, recipient: paxeerRecipient(wrongRecipient),
    });
    const mismatchedRelease = api.parseReleaseAttestation(state.releaseRequest, {
      ...state.releaseBody, recipient: wrongRecipient,
    });
    expect(() => consumers.assertClaimMatches(restoredDeposit, mismatchedDeposit))
      .toThrow('The backend attestation does not match this deposit.');
    expect(() => consumers.assertReleaseMatches(restoredRelease, mismatchedRelease))
      .toThrow('The backend attestation does not match this withdrawal.');
    const outsider = privateKeyToAccount(generatePrivateKey());
    const untrusted = api.parseReleaseAttestation(state.releaseRequest, {
      ...state.releaseBody, signatures: [ await outsider.sign({ hash: release.digest }) ],
    });
    expect(await verifyThresholdSignatures({
      digest: untrusted.digest,
      signatures: untrusted.signatures,
      attestors: [ state.signer.address ],
      threshold: 1,
    })).toEqual({ ok: false, reason: 'Signature 1 is not from a registered attestor.' });
  } finally {
    if (previous === null) window.localStorage.removeItem(storageKey);
    else window.localStorage.setItem(storageKey, previous);
  }
}

async function refusesAnotherAttestationIdentityAndKeepsPollingIncompleteBundles() {
  const state = await produceAttestations();
  const otherHash = '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc';
  for (const change of [ { chainId: '1' }, { txHash: otherHash }, { logIndex: '0' } ]) {
    expect(() => parseDepositAttestation(state.depositRequest, { ...state.depositBody, ...change }))
      .toThrow(AttestationApiError);
  }
  for (const change of [ { chainId: '1' }, { paxeerTxHash: otherHash }, { paxeerNonce: '0' } ]) {
    expect(() => parseReleaseAttestation(state.releaseRequest, { ...state.releaseBody, ...change }))
      .toThrow(AttestationApiError);
  }
  for (const status of [ 'observed', 'finalizing', 'attesting', 'unrecognized' ]) {
    const deposit = parseDepositAttestation(state.depositRequest, { ...state.depositBody, status });
    const release = parseReleaseAttestation(state.releaseRequest, { ...state.releaseBody, status });
    expect(deposit.status).toBe(status === 'unrecognized' ? 'unknown' : status);
    expect(release.status).toBe(deposit.status);
    expect(attestationPollInterval(deposit, 12_000)).toBe(12_000);
    expect(attestationPollInterval(release, 12_000)).toBe(12_000);
  }
  expect(attestationPollInterval(parseDepositAttestation(state.depositRequest, {
    ...state.depositBody, signatures: [],
  }), 12_000)).toBe(12_000);
  expect(attestationPollInterval(parseReleaseAttestation(state.releaseRequest, {
    ...state.releaseBody, signatures: [],
  }), 12_000)).toBe(12_000);
  expect(attestationPollInterval(null, 12_000)).toBe(12_000);
  expect(attestationPollInterval(undefined, 12_000)).toBe(12_000);
}

async function refusesIntegersThatCannotReachTheDigestConsumer() {
  const state = await produceAttestations();
  for (const amount of [ (1n << 256n).toString(), '-1', '1.5', '', '1e18' ]) {
    expect(() => parseDepositAttestation(state.depositRequest, { ...state.depositBody, amount })).toThrow(/bad "amount"/);
    expect(() => parseReleaseAttestation(state.releaseRequest, { ...state.releaseBody, amount })).toThrow(/bad "amount"/);
  }
  const chainId = 1n << 256n;
  expect(() => parseDepositAttestation({ ...state.depositRequest, chainId }, {
    ...state.depositBody, chainId: chainId.toString(),
  })).toThrow(/bad "chainId"/);
  expect(() => parseReleaseAttestation({ ...state.releaseRequest, chainId }, {
    ...state.releaseBody, chainId: chainId.toString(),
  })).toThrow(/bad "chainId"/);
  const index = 1n << 64n;
  expect(() => parseDepositAttestation({ ...state.depositRequest, logIndex: index }, {
    ...state.depositBody, logIndex: index.toString(),
  })).toThrow(/bad "logIndex"/);
  expect(() => parseReleaseAttestation({ ...state.releaseRequest, paxeerNonce: index }, {
    ...state.releaseBody, paxeerNonce: index.toString(),
  })).toThrow(/bad "paxeerNonce"/);
  expect(() => inboundDigest({
    ...state.depositRequest,
    vault: state.depositBody.vault,
    recipient32: state.depositBody.recipient,
    asset: state.depositBody.asset,
    amount: 1n << 256n,
  })).toThrow();
  expect(() => outboundDigest({
    ...state.releaseRequest,
    vault: state.releaseBody.vault,
    recipient: state.releaseBody.recipient,
    asset: state.releaseBody.asset,
    amount: 1n << 256n,
  })).toThrow();
}

test(provesAttestationConsumersAfterReopeningTheTransferStore.name, provesAttestationConsumersAfterReopeningTheTransferStore);
test(refusesAnotherAttestationIdentityAndKeepsPollingIncompleteBundles.name, refusesAnotherAttestationIdentityAndKeepsPollingIncompleteBundles);
test(refusesIntegersThatCannotReachTheDigestConsumer.name, refusesIntegersThatCannotReachTheDigestConsumer);

test(attestationWireFixturesSurviveListenerReopening.name, attestationWireFixturesSurviveListenerReopening, 15000);
