// @vitest-environment jsdom

import type { Address, Hex } from 'viem';
import { decodeFunctionData, encodeFunctionData, zeroAddress, zeroHash } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';

import { expect, test } from 'vitest';

import { BRIDGE_CHAINS } from './chains';
import { depositRecord, withdrawalRecord } from './submitted';
import { solanaRecipientHandle } from './solana';
import { transferIdOf, type BridgeTransfer } from './transfers';
import { paxeerXVaultAbi } from './contracts';
import { openBridgeRepairFixture } from './fixtures/repair-22-33-265';
import type { ReleaseAttestation } from './attestation-api';

const hash: Hex = '0xABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789AB';
const account: Address = '0x1111111111111111111111111111111111111111';
const vault: Address = '0x2222222222222222222222222222222222222222';
const recipient: Address = '0xAbCdEf0123456789aBcDeF0123456789aBCdEf01';

function depositRecordCarriesTheVaultAndFinalityDepth(): void {
  const foreign = BRIDGE_CHAINS.find((chain) => chain.key === 'base')!;
  const asset = foreign.assets[0];
  const amount = 1234567890123456789n;
  const startedAt = Date.now();

  for (const requiredConfirmations of [ foreign.finalityDepth, foreign.finalityDepth + 17 ]) {
    const record: BridgeTransfer = depositRecord({
      hash, foreign, asset, account, amount, vault, recipient, requiredConfirmations,
    });

    expect(record).toEqual({
      id: transferIdOf('in', hash, foreign.chainId.toString()),
      direction: 'in',
      foreignChainKey: foreign.key,
      foreignChainId: foreign.chainId.toString(),
      symbol: asset.symbol,
      decimals: asset.decimals,
      amount: '1234567890123456789',
      assetId: asset.assetId,
      vault,
      account,
      recipient,
      sourceTxHash: hash,
      requiredConfirmations,
      submittedAt: expect.any(Number),
    });
    expect(record.id).toBe(`${ foreign.chainId }:${ hash.toLowerCase() }:in:pending`);
    expect(record.submittedAt).toBeGreaterThanOrEqual(startedAt);
    expect(record.submittedAt).toBeLessThanOrEqual(Date.now());
    expect(() => JSON.stringify(record)).not.toThrow();
  }
}

function withdrawalRecordKeepsTheTypedDestination(): void {
  for (const key of [ 'base', 'solana' ]) {
    const foreign = { ...BRIDGE_CHAINS.find((chain) => chain.key === key)!, vault };
    const asset = foreign.assets[0];
    const destination = key === 'solana' ? 'So11111111111111111111111111111111111111112' : recipient;
    const amount = 9007199254740993n;
    const startedAt = Date.now();
    const record: BridgeTransfer = withdrawalRecord({
      hash, foreign, asset, account, amount, destination, requiredConfirmations: foreign.finalityDepth,
    });

    expect(record).toEqual({
      id: transferIdOf('out', hash, foreign.chainId.toString()),
      direction: 'out',
      foreignChainKey: key,
      foreignChainId: key === 'solana' ? '91600046870081' : '8453',
      symbol: asset.symbol,
      decimals: asset.decimals,
      amount: '9007199254740993',
      assetId: asset.assetId,
      account,
      recipient: destination,
      ...(key === 'solana' ? { recipientHandle: solanaRecipientHandle(destination) } : {}),
      sourceTxHash: hash,
      requiredConfirmations: foreign.finalityDepth,
      submittedAt: expect.any(Number),
    });
    expect(record.id).toBe(`${ foreign.chainId }:${ hash.toLowerCase() }:out:pending`);
    expect(record).not.toHaveProperty('vault');
    expect(record.submittedAt).toBeGreaterThanOrEqual(startedAt);
    expect(record.submittedAt).toBeLessThanOrEqual(Date.now());
    expect(() => JSON.stringify(record)).not.toThrow();
  }
}

async function withdrawalRecordReloadResolvesTheAuthoritativeVaultAndBuildsVerifiedRelease(): Promise<void> {
  const fixture = await openBridgeRepairFixture();
  try {
    const record = await fixture.submit('out');
    expect(record).not.toHaveProperty('vault');
    expect(record.bridgeOutNonce).toBeUndefined();
    expect(record.recipient).toBe(fixture.recipient);
    expect(record.recipient).not.toBe(record.account);
    expect(fixture.ledger.listTransfers(record.account)).toEqual([ record ]);

    const reloaded = await fixture.reopen(record.id);
    expect(reloaded.vault).toBe(fixture.vault);
    expect(reloaded.bridgeOutNonce).toBe('0');
    expect(reloaded.recipient).toBe(record.recipient);
    expect(reloaded.amount).toBe('9007199254740993');
    expect(reloaded.account).toBe(record.account);
    expect(fixture.ledger.listTransfers(record.account)).toEqual([ reloaded ]);
    expect(await fixture.reopen(reloaded.id)).toEqual(reloaded);

    const attestation = await fixture.releaseAttestation(reloaded);
    const verified = await fixture.actions.buildVerifiedRelease(reloaded, attestation);
    expect(verified.request.address).toBe(fixture.vault);
    expect(verified.request.chainId).toBe(8453);
    expect(decodeFunctionData({ abi: paxeerXVaultAbi, data: encodeFunctionData(verified.request) })).toEqual({
      functionName: 'release',
      args: [ reloaded.assetId, BigInt(reloaded.amount), fixture.recipient, record.sourceTxHash, 0n, attestation.signatures ],
    });
    expect(fixture.requests.some((request) => request.method === 'GET' &&
      request.path === `/v1/release/8453/${ record.sourceTxHash.toLowerCase() }/0`)).toBe(true);

    // The consumer also resolves old submitted rows whose nonce was enriched without a saved vault.
    expect((await fixture.actions.buildVerifiedRelease({ ...reloaded, vault: undefined }, attestation)).request)
      .toEqual(verified.request);
    expect((await fixture.actions.resolveWithdrawalVault(reloaded))).toEqual(reloaded);
  } finally {
    await fixture.close();
  }
}

async function withdrawalReloadPreservesAuthorityDestinationAndSignatureRefusals(): Promise<void> {
  const fixture = await openBridgeRepairFixture();
  try {
    const record = await fixture.submit('out');
    const reloaded = await fixture.reopen(record.id);
    const attestation = await fixture.releaseAttestation(reloaded);
    const build = (patch: Partial<ReleaseAttestation> = {}, transfer = reloaded) =>
      fixture.actions.buildVerifiedRelease(transfer, { ...attestation, ...patch });
    await expect(build()).resolves.toHaveProperty('request.address', fixture.vault);

    await expect(build({}, { ...reloaded, direction: 'in' })).rejects.toThrow('Only withdrawals');
    await expect(build({}, { ...reloaded, foreignChainKey: 'solana' })).rejects.toThrow('cannot be released from an EVM wallet');
    await expect(build({}, { ...reloaded, foreignChainId: '1' })).rejects.toThrow('Unknown or mismatched bridge chain');
    await expect(build({}, { ...reloaded, vault: record.account })).rejects.toThrow('does not match the vault registered on Paxeer');
    await expect(build({ vault: record.account })).rejects.toThrow('different vault');

    fixture.registry.registered = false;
    await expect(build()).rejects.toThrow('not registered and enabled');
    fixture.registry.registered = true;
    fixture.registry.enabled = false;
    await expect(build()).rejects.toThrow('not registered and enabled');
    fixture.registry.enabled = true;
    fixture.registry.vault = zeroAddress;
    await expect(build()).rejects.toThrow('not registered and enabled');
    fixture.registry.vault = record.account;
    await expect(build({}, { ...reloaded, vault: undefined })).rejects.toThrow('does not match the vault registered on Paxeer');
    fixture.registry.vault = fixture.vault;

    for (const patch of [
      { recipient: record.account }, { asset: record.account }, { amount: '9007199254740994' },
      { chainId: '1' }, { paxeerNonce: '1' }, { paxeerTxHash: zeroHash },
    ]) {
      await expect(build(patch)).rejects.toThrow('does not match this withdrawal');
    }
    await expect(build({}, { ...reloaded, bridgeOutNonce: undefined })).rejects.toThrow('does not match this withdrawal');
    await expect(build({ status: 'attesting' })).rejects.toThrow('not ready');
    await expect(build({ signatures: [] })).rejects.toThrow('not ready');
    await expect(build({ digest: zeroHash })).rejects.toThrow('digest does not match');
    fixture.authority.digestOverride = zeroHash;
    await expect(build()).rejects.toThrow('digest does not match');
    fixture.authority.digestOverride = undefined;

    await expect(build({ signatures: [ attestation.signatures[0] ] })).rejects.toThrow('Only 1 of 2 required signatures');
    fixture.authority.threshold = 0n;
    await expect(build()).rejects.toThrow('vault threshold is zero');
    fixture.authority.threshold = 1n << 60n;
    await expect(build()).rejects.toThrow('not a non-negative safe integer');
    fixture.authority.threshold = 2n;
    fixture.authority.attestors = [];
    await expect(build()).rejects.toThrow('vault attestor set is empty');
    fixture.authority.attestors = fixture.signers.map((signer) => signer.address);

    const signature = attestation.signatures[0];
    const invalid: Array<[Hex, string]> = [
      [ '0x12', 'not 65 bytes' ],
      [ `0x${ signature.slice(2, -2) }00`, 'invalid recovery id' ],
      [ `0x${ signature.slice(2, 66) }${ 'ff'.repeat(32) }1b`, 'not low-s' ],
      [ `0x${ '00'.repeat(64) }1b`, 'does not recover a signer' ],
    ];
    for (const [ invalidSignature, refusal ] of invalid) {
      await expect(build({ signatures: [ invalidSignature, attestation.signatures[1] ] })).rejects.toThrow(refusal);
    }
    const outsider = privateKeyToAccount(`0x${ 'bc'.repeat(32) }`);
    await expect(build({ signatures: [ await outsider.sign({ hash: attestation.digest }), attestation.signatures[1] ] }))
      .rejects.toThrow('not from a registered attestor');
    await expect(build({ signatures: [ signature, signature ] })).rejects.toThrow('not ordered by ascending signer');
    await expect(build({ signatures: [ ...attestation.signatures ].reverse() })).rejects.toThrow('not ordered by ascending signer');
    await expect(build()).resolves.toHaveProperty('request.address', fixture.vault);
    expect((await fixture.reopen(reloaded.id)).recipient).toBe(record.recipient);
  } finally {
    await fixture.close();
  }
}

test(depositRecordCarriesTheVaultAndFinalityDepth.name, depositRecordCarriesTheVaultAndFinalityDepth);
test(withdrawalRecordKeepsTheTypedDestination.name, withdrawalRecordKeepsTheTypedDestination);
test(withdrawalRecordReloadResolvesTheAuthoritativeVaultAndBuildsVerifiedRelease.name,
  withdrawalRecordReloadResolvesTheAuthoritativeVaultAndBuildsVerifiedRelease);
test(withdrawalReloadPreservesAuthorityDestinationAndSignatureRefusals.name,
  withdrawalReloadPreservesAuthorityDestinationAndSignatureRefusals);
