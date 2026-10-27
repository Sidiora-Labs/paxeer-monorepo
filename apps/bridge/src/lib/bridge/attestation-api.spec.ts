import { expect, test } from 'vitest';

import { AttestationApiError, attestationPollInterval, parseDepositAttestation, parseReleaseAttestation } from './attestation-api';
import type { DepositAttestation, ReleaseAttestation } from './attestation-api';

function parseDepositAttestationRefusesAnotherChainId() {
  const request = {
    chainId: BigInt(1),
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' as const,
    logIndex: BigInt(7),
  };
  const body = {
    chainId: '8453',
    vault: '0x1111111111111111111111111111111111111111',
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    logIndex: '7',
    recipient: '0x0000000000000000000000002222222222222222222222222222222222222222',
    asset: '0x3333333333333333333333333333333333333333',
    amount: '100',
    digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    signatures: [],
    status: 'ready',
  };

  expect(() => parseDepositAttestation(request, body)).toThrow(AttestationApiError);
  expect(() => parseDepositAttestation(request, body)).toThrow(/chainId/);
  const { recipient, ...fields } = body;
  expect(parseDepositAttestation(request, { ...body, chainId: '1' })).toEqual({
    ...fields,
    chainId: '1',
    recipient32: recipient,
  });
}

function parseDepositAttestationRefusesAnotherLogIndex() {
  const request = {
    chainId: BigInt(1),
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' as const,
    logIndex: BigInt(7),
  };
  const body = {
    chainId: '1',
    vault: '0x1111111111111111111111111111111111111111',
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    logIndex: '8',
    recipient: '0x0000000000000000000000002222222222222222222222222222222222222222',
    asset: '0x3333333333333333333333333333333333333333',
    amount: '100',
    digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    signatures: [],
    status: 'ready',
  };

  expect(() => parseDepositAttestation(request, body)).toThrow(AttestationApiError);
  expect(() => parseDepositAttestation(request, body)).toThrow(/logIndex/);
  const anotherTransaction = {
    ...body,
    logIndex: '7',
    txHash: '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  };
  expect(() => parseDepositAttestation(request, anotherTransaction)).toThrow(AttestationApiError);
  expect(() => parseDepositAttestation(request, anotherTransaction)).toThrow(/txHash/);
  const upperHash = '0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' as const;
  expect(parseDepositAttestation(request, { ...body, logIndex: '7', txHash: upperHash }).txHash).toBe(upperHash);
  expect(parseDepositAttestation({ ...request, txHash: upperHash }, { ...body, logIndex: '7' }).txHash).toBe(body.txHash);
}

function parseReleaseAttestationRefusesAnotherNonce() {
  const request = {
    chainId: BigInt(1),
    paxeerTxHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' as const,
    paxeerNonce: BigInt(3),
  };
  const body = {
    chainId: '1',
    vault: '0x1111111111111111111111111111111111111111',
    paxeerTxHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    paxeerNonce: '4',
    recipient: '0x2222222222222222222222222222222222222222',
    asset: '0x3333333333333333333333333333333333333333',
    amount: '100',
    digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    signatures: [],
    status: 'ready',
  };

  expect(() => parseReleaseAttestation(request, body)).toThrow(AttestationApiError);
  expect(() => parseReleaseAttestation(request, body)).toThrow(/paxeerNonce/);
  const anotherChain = { ...body, paxeerNonce: '3', chainId: '8453' };
  expect(() => parseReleaseAttestation(request, anotherChain)).toThrow(AttestationApiError);
  expect(() => parseReleaseAttestation(request, anotherChain)).toThrow(/chainId/);
  const anotherTransaction = {
    ...body,
    paxeerNonce: '3',
    paxeerTxHash: '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  };
  expect(() => parseReleaseAttestation(request, anotherTransaction)).toThrow(AttestationApiError);
  expect(() => parseReleaseAttestation(request, anotherTransaction)).toThrow(/paxeerTxHash/);
  expect(parseReleaseAttestation(request, { ...body, paxeerNonce: '3' })).toEqual({ ...body, paxeerNonce: '3' });
  const upperHash = '0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' as const;
  expect(parseReleaseAttestation(request, { ...body, paxeerNonce: '3', paxeerTxHash: upperHash }).paxeerTxHash).toBe(upperHash);
  expect(parseReleaseAttestation({ ...request, paxeerTxHash: upperHash }, { ...body, paxeerNonce: '3' }).paxeerTxHash).toBe(body.paxeerTxHash);
}

function pollIntervalContinuesForAReadyBundleWithoutSignatures() {
  const deposit: DepositAttestation = {
    chainId: '1',
    vault: '0x1111111111111111111111111111111111111111',
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    logIndex: '7',
    recipient32: '0x0000000000000000000000002222222222222222222222222222222222222222',
    asset: '0x3333333333333333333333333333333333333333',
    amount: '100',
    digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    signatures: [],
    status: 'ready',
  };

  expect(attestationPollInterval(deposit, 12_000)).toBe(12_000);
  expect(attestationPollInterval(deposit, 5_000)).toBe(5_000);
  expect(attestationPollInterval(null, 12_000)).toBe(12_000);
  expect(attestationPollInterval(undefined, 12_000)).toBe(12_000);
}

function pollIntervalStopsForASignedReadyBundle() {
  const release: ReleaseAttestation = {
    chainId: '1',
    vault: '0x1111111111111111111111111111111111111111',
    paxeerTxHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    paxeerNonce: '3',
    recipient: '0x2222222222222222222222222222222222222222',
    asset: '0x3333333333333333333333333333333333333333',
    amount: '100',
    digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    signatures: [
      '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1b',
    ],
    status: 'ready',
  };

  expect(attestationPollInterval(release, 12_000)).toBe(false);
  expect(attestationPollInterval({ ...release, signatures: [] }, 12_000)).toBe(12_000);
  for (const status of [ 'observed', 'finalizing', 'attesting', 'unknown' ] as const) {
    expect(attestationPollInterval({ ...release, status }, 12_000)).toBe(12_000);
  }
}

test(pollIntervalContinuesForAReadyBundleWithoutSignatures);
test(pollIntervalStopsForASignedReadyBundle);
test(parseDepositAttestationRefusesAnotherChainId);
test(parseDepositAttestationRefusesAnotherLogIndex);
test(parseReleaseAttestationRefusesAnotherNonce);
