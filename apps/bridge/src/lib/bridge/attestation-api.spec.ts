import { expect, test } from 'vitest';

import { AttestationApiError, parseDepositAttestation, parseReleaseAttestation } from './attestation-api';

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

test(parseDepositAttestationRefusesAnotherChainId);
test(parseDepositAttestationRefusesAnotherLogIndex);
test(parseReleaseAttestationRefusesAnotherNonce);
