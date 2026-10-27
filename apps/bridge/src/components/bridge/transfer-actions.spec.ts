import { expect, test } from 'vitest';

import type { DepositAttestation, ReleaseAttestation } from '@/lib/bridge/attestation-api';
import type { BridgeTransfer } from '@/lib/bridge/transfers';

import { assertClaimMatches, assertReleaseMatches } from './transfer-actions';

const deposit: BridgeTransfer = {
  id: 'deposit:0',
  direction: 'in',
  foreignChainKey: 'base',
  foreignChainId: '8453',
  symbol: 'USDC',
  decimals: 6,
  amount: '1000000',
  assetId: '0x1111111111111111111111111111111111111111',
  vault: '0x2222222222222222222222222222222222222222',
  account: '0x3333333333333333333333333333333333333333',
  recipient: '0x4444444444444444444444444444444444444444',
  sourceTxHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  requiredConfirmations: 12,
  submittedAt: 1,
  logIndex: '0',
};

const claim: DepositAttestation = {
  chainId: '8453',
  vault: '0x2222222222222222222222222222222222222222',
  txHash: '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  logIndex: '0',
  recipient32: '0x0000000000000000000000004444444444444444444444444444444444444444',
  asset: '0x1111111111111111111111111111111111111111',
  amount: '1000000',
  digest: '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  signatures: [],
  status: 'ready',
};

const withdrawal: BridgeTransfer = {
  id: 'withdrawal:7',
  direction: 'out',
  foreignChainKey: 'base',
  foreignChainId: '8453',
  symbol: 'USDC',
  decimals: 6,
  amount: '1000000',
  assetId: '0x1111111111111111111111111111111111111111',
  vault: '0x2222222222222222222222222222222222222222',
  account: '0x3333333333333333333333333333333333333333',
  recipient: '0x4444444444444444444444444444444444444444',
  sourceTxHash: '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  requiredConfirmations: 12,
  submittedAt: 1,
  bridgeOutNonce: '7',
};

const release: ReleaseAttestation = {
  chainId: '8453',
  vault: '0x2222222222222222222222222222222222222222',
  paxeerTxHash: '0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  paxeerNonce: '7',
  recipient: '0x4444444444444444444444444444444444444444',
  asset: '0x1111111111111111111111111111111111111111',
  amount: '1000000',
  digest: '0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
  signatures: [],
  status: 'ready',
};

function assertClaimMatchesRefusesAnotherRecipient() {
  expect(() => assertClaimMatches(deposit, claim)).not.toThrow();
  expect(() => assertClaimMatches(deposit, {
    ...claim,
    recipient32: '0x0000000000000000000000005555555555555555555555555555555555555555',
  })).toThrow('The backend attestation does not match this deposit.');
}

function assertClaimMatchesRefusesAnotherAsset() {
  expect(() => assertClaimMatches(deposit, claim)).not.toThrow();
  expect(() => assertClaimMatches(deposit, {
    ...claim,
    asset: '0x5555555555555555555555555555555555555555',
  })).toThrow('The backend attestation does not match this deposit.');
}

function assertClaimMatchesRefusesAnotherChainId() {
  expect(() => assertClaimMatches(deposit, claim)).not.toThrow();
  expect(() => assertClaimMatches(deposit, {
    ...claim,
    chainId: '1',
  })).toThrow('The backend attestation does not match this deposit.');
}

function assertReleaseMatchesRefusesAnotherAsset() {
  expect(() => assertReleaseMatches(withdrawal, release)).not.toThrow();
  expect(() => assertReleaseMatches(withdrawal, {
    ...release,
    asset: '0x5555555555555555555555555555555555555555',
  })).toThrow('The backend attestation does not match this withdrawal.');
  expect(() => assertReleaseMatches(withdrawal, {
    ...release,
    chainId: '1',
  })).toThrow('The backend attestation does not match this withdrawal.');
}

test(assertClaimMatchesRefusesAnotherRecipient);
test(assertClaimMatchesRefusesAnotherAsset);
test(assertClaimMatchesRefusesAnotherChainId);
test(assertReleaseMatchesRefusesAnotherAsset);
