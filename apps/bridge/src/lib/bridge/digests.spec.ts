import { privateKeyToAccount } from 'viem/accounts';

import { expect, test } from 'vitest';

import { inboundDigest, paxeerRecipient, verifyThresholdSignatures } from './digests';

const attestor = privateKeyToAccount('0x0000000000000000000000000000000000000000000000000000000000000001');
const digest = inboundDigest({
  chainId: BigInt(1),
  vault: '0x0000000000000000000000000000000000000002',
  txHash: '0x0000000000000000000000000000000000000000000000000000000000000003',
  logIndex: BigInt(0),
  recipient32: paxeerRecipient(attestor.address),
  asset: '0x0000000000000000000000000000000000000004',
  amount: BigInt(100),
});

async function verifyThresholdRefusesAZeroThreshold() {
  const signature = await attestor.sign({ hash: digest });

  await expect(verifyThresholdSignatures({
    digest,
    signatures: [ signature ],
    attestors: [ attestor.address ],
    threshold: 1,
  })).resolves.toEqual({ ok: true, signers: [ attestor.address ] });

  for (const signatures of [ [], [ signature ] ]) {
    await expect(verifyThresholdSignatures({
      digest,
      signatures,
      attestors: [ attestor.address ],
      threshold: Number(BigInt(0)),
    })).resolves.toEqual({ ok: false, reason: 'vault threshold is zero' });
  }
}

async function verifyThresholdRefusesAnEmptyAttestorSet() {
  const signature = await attestor.sign({ hash: digest });

  for (const threshold of [ 0, 1, 2 ]) {
    for (const signatures of [ [], [ signature ] ]) {
      await expect(verifyThresholdSignatures({
        digest,
        signatures,
        attestors: [],
        threshold,
      })).resolves.toEqual({
        ok: false,
        reason: threshold === 0 ? 'vault threshold is zero' : 'vault attestor set is empty',
      });
    }
  }
}

test(verifyThresholdRefusesAZeroThreshold);
test(verifyThresholdRefusesAnEmptyAttestorSet);
