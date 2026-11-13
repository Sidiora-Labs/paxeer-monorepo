import { expect, test } from 'vitest';
import {
  AgentSendTxBody,
  AgentSignTxBody,
  AgentSimulateBody,
} from '../src/schemas/agent.js';
import { SendTxBody, SignTxBody, TxRequest } from '../src/schemas/tx.js';

/**
 * Task 22.33.184 / audit 22.30.119: tx-extra-fields-discarded is proven by
 * rejectsUnsupportedTransactionFields and rejectsCombinedUnsupportedFields.
 * These exercise the production request codecs used before route signing;
 * a refusal must expose no parsed transaction for the signer to consume.
 * Baseline verification was not run: the bounded gate order requires complete
 * implementation before hf-build and permits verification only via hf-done.
 * Wiring request (wallet/domain 12): update the obsolete stripping expectation
 * in services/wallet/gateway/test/schemas.injection.test.ts to require refusal.
 */

const supportedTx = {
  to: '0x1111111111111111111111111111111111111111',
  value: '1000000000000000000',
  data: '0xa9059cbb',
  gas: '80000',
  maxFeePerGas: '2000000000',
  maxPriorityFeePerGas: '1000000000',
  nonce: 7,
  chainId: 125,
};

const unsupportedFields = {
  accessList: [{ address: supportedTx.to, storageKeys: ['0x' + '00'.repeat(32)] }],
  authorizationList: [
    {
      address: supportedTx.to,
      chainId: 125,
      nonce: 0,
      r: '0x' + '01'.repeat(32),
      s: '0x' + '02'.repeat(32),
      yParity: 0,
    },
  ],
  type: 'eip1559',
  gasPrice: '2000000000',
  blobVersionedHashes: ['0x01' + '00'.repeat(31)],
  maxFeePerBlobGas: '3000000000',
  futureTransactionField: { enabled: true },
};

const envelopes = {
  SignTxBody,
  SendTxBody,
  AgentSignTxBody,
  AgentSendTxBody,
  AgentSimulateBody,
};

function rejectsUnsupportedTransactionFields() {
  for (const [field, value] of Object.entries(unsupportedFields)) {
    const tx = { ...supportedTx, [field]: value };
    const direct = TxRequest.safeParse(JSON.parse(JSON.stringify(tx)));
    expect(direct.success, `TxRequest must refuse ${field}`).toBe(false);
    expect(direct).not.toHaveProperty('data');
    expect(direct.error?.issues).toEqual([
      expect.objectContaining({ code: 'unrecognized_keys', path: [], keys: [field] }),
    ]);

    for (const [name, schema] of Object.entries(envelopes)) {
      const requestBody = JSON.parse(JSON.stringify({ tx }));
      const result = schema.safeParse(requestBody);
      expect(result.success, `${name} must refuse tx.${field}`).toBe(false);
      expect(result).not.toHaveProperty('data');
      expect(result.error?.issues).toEqual([
        expect.objectContaining({ code: 'unrecognized_keys', path: ['tx'], keys: [field] }),
      ]);
    }
  }
}

function rejectsCombinedUnsupportedFields() {
  for (const [name, schema] of Object.entries(envelopes)) {
    const result = schema.safeParse(
      JSON.parse(JSON.stringify({ tx: { ...supportedTx, ...unsupportedFields } }))
    );
    expect(result.success, `${name} must refuse all unsupported fields`).toBe(false);
    expect(result).not.toHaveProperty('data');
    expect(result.error?.issues).toEqual([
      expect.objectContaining({
        code: 'unrecognized_keys',
        path: ['tx'],
        keys: Object.keys(unsupportedFields),
      }),
    ]);
  }
}

function preservesSupportedTransactionFields() {
  expect(TxRequest.parse(JSON.parse(JSON.stringify(supportedTx)))).toEqual(supportedTx);
  for (const schema of Object.values(envelopes)) {
    expect(schema.parse(JSON.parse(JSON.stringify({ tx: supportedTx })))).toEqual({
      tx: supportedTx,
    });
    expect(schema.parse({ tx: {} })).toEqual({ tx: {} });
    expect(schema.parse({ tx: { to: supportedTx.to, value: '0' } })).toEqual({
      tx: { to: supportedTx.to, value: '0' },
    });
  }
}

test(rejectsUnsupportedTransactionFields.name, rejectsUnsupportedTransactionFields);
test(rejectsCombinedUnsupportedFields.name, rejectsCombinedUnsupportedFields);
test(preservesSupportedTransactionFields.name, preservesSupportedTransactionFields);
