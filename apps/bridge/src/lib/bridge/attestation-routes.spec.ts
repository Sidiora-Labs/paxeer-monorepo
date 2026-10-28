import { expect, test } from 'vitest';

import {
  depositAttestationPath,
  parseDepositAttestation,
  parseReleaseAttestation,
  releaseAttestationPath,
} from './attestation-api';
import depositReady from './fixtures/deposit-ready.json';
import releaseReady from './fixtures/release-ready.json';

function depositAttestationPathLowercasesTheHash() {
  const txHash = '0xAaBbCcDdEeFf00112233445566778899AaBbCcDdEeFf00112233445566778899' as const;
  const hash = '0xaabbccddeeff00112233445566778899aabbccddeeff00112233445566778899';

  expect(depositAttestationPath({ chainId: BigInt(1), txHash, logIndex: BigInt(7) }))
    .toBe(`/v1/deposit/1/${ hash }/7`);
  expect(depositAttestationPath({ chainId: BigInt(8453), txHash, logIndex: BigInt(0) }))
    .toBe(`/v1/deposit/8453/${ hash }/0`);
  expect(depositAttestationPath({
    chainId: BigInt('9007199254740993'),
    txHash,
    logIndex: BigInt('18446744073709551615'),
  })).toBe(`/v1/deposit/9007199254740993/${ hash }/18446744073709551615`);
  expect(releaseAttestationPath({ chainId: BigInt(1), paxeerTxHash: txHash, paxeerNonce: BigInt(3) }))
    .toBe(`/v1/release/1/${ hash }/3`);
}

function theCommittedFixturesParseAsReadyBundles() {
  const deposit = parseDepositAttestation({
    chainId: BigInt(1),
    txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    logIndex: BigInt(7),
  }, depositReady);
  const release = parseReleaseAttestation({
    chainId: BigInt(1),
    paxeerTxHash: '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
    paxeerNonce: BigInt(3),
  }, releaseReady);
  const { recipient, ...depositFields } = depositReady;

  expect(deposit).toEqual({ ...depositFields, recipient32: recipient });
  expect(deposit.status).toBe('ready');
  expect(deposit.signatures.length).toBeGreaterThan(0);
  expect(release).toEqual(releaseReady);
  expect(release.status).toBe('ready');
  expect(release.signatures.length).toBeGreaterThan(0);
}

test(depositAttestationPathLowercasesTheHash.name, depositAttestationPathLowercasesTheHash);
test(theCommittedFixturesParseAsReadyBundles.name, theCommittedFixturesParseAsReadyBundles);
