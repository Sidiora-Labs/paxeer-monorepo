// @vitest-environment jsdom

import { act, waitFor } from '@testing-library/react';
import { createHash, createPublicKey, verify } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import React from 'react';
import { keccak256 } from 'viem';

import { expect, test } from 'vitest';

import {
  answer,
  committedAnswer,
  exchanges,
  hash,
  refuseSignature,
  render,
  requests,
  serveSignature,
  settlement,
  signatureQueryKey,
  transaction,
  TxInfo,
} from './signatureTestFixture';

import './TxDetailsSequencerSignature.spec';
import './TxInfo.spec';

function signatureFixtureBindsCanonicalReceiptTransactionAndGovernedKey() {
  const fixture = readFileSync(join(__dirname, 'fixtures/vector-v4-committed.json'));
  expect(createHash('sha256').update(new Uint8Array(fixture)).digest('hex'))
    .toBe('5013cd232ee94ece8d25fca4c802ca60c892f3a5f637a70962d9909dc71c4e7b');
  const receipt = Buffer.from(answer.receipt.slice(2), 'hex');
  const publicKey = Buffer.from(answer.receipt_key.public_key.slice(2), 'hex');
  const signature = Buffer.from(answer.signature.slice(2), 'hex');
  expect(receipt.subarray(0, 10).toString('hex')).toBe('00025201000200000020');
  expect(`0x${ receipt.subarray(10, 42).toString('hex') }`).toBe(hash);
  expect(keccak256(new Uint8Array(Buffer.from(settlement.raw_transaction.slice(2), 'hex')))).toBe(hash);
  expect(`0x${ createHash('sha256').update(new Uint8Array(publicKey)).digest('hex') }`).toBe(answer.receipt_key.key_id);
  expect(receipt.subarray(-113, -109).toString('hex')).toBe('00000020');
  expect(`0x${ receipt.subarray(-109, -77).toString('hex') }`).toBe(answer.receipt_key.key_id);
  expect(receipt.subarray(-69, -64).toString('hex')).toBe('0100000040');
  expect(receipt.subarray(-64)).toEqual(signature);
  const digest = createHash('sha256')
    .update('LXP/v1/receipt\0')
    .update(new Uint8Array(receipt.subarray(0, -69)))
    .update(new Uint8Array([ 0 ]))
    .digest();
  const key = createPublicKey({
    format: 'pem',
    type: 'spki',
    key: `-----BEGIN PUBLIC KEY-----\n${ Buffer.from(`302a300506032b6570032100${ answer.receipt_key.public_key.slice(2) }`, 'hex').toString('base64') }\n-----END PUBLIC KEY-----`,
  });
  expect(verify(null, new Uint8Array(digest), key, new Uint8Array(signature))).toBe(true);
  const tampered = new Uint8Array(digest);
  tampered[0] ^= 1;
  expect(verify(null, tampered, key, new Uint8Array(signature))).toBe(false);
}

async function receiptedCanonicalSignatureReachesMountedTxInfo() {
  const view = render(React.createElement(TxInfo, { data: transaction, isLoading: false }));
  expect(await view.findByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeDefined();
  const signature = await view.findByText(answer.signature);
  expect(signature.parentElement?.querySelector('[data-rung="receipted"]')?.textContent).toBe('Receipted');
  expect(view.client.getQueryData(signatureQueryKey())).toEqual(answer);
  expect(requests).toEqual([ `/api/v2/transactions/${ hash }/signature` ]);
  expect(exchanges).toEqual([ { path: requests[0], status: 200, body: answer } ]);
  expect(Array.from(view.container.querySelectorAll('[data-tx-info] > [data-detailed-info] [data-detailed-info-label]'))
    .map(label => label.textContent).slice(0, 4)).toEqual([ 'Transaction hash', 'Status and method', 'Sequencer signature', 'Block' ]);
}

async function committedCanonicalSignatureReachesMountedTxInfo() {
  serveSignature(committedAnswer);
  const view = render(React.createElement(TxInfo, { data: transaction, isLoading: false }));
  expect(await view.findByText(`Key id: ${ committedAnswer.receipt_key.key_id }`)).toBeDefined();
  const signature = await view.findByText(committedAnswer.signature);
  expect(signature.parentElement?.querySelector('[data-rung="committed"]')?.textContent).toBe('Committed');
  expect(signature.parentElement?.querySelector('[data-rung="receipted"]')).toBeNull();
  expect(committedAnswer.committed_height).toBe(10);
  expect(view.client.getQueryData(signatureQueryKey())).toEqual(committedAnswer);
  expect(requests).toEqual([ `/api/v2/transactions/${ hash }/signature` ]);
  expect(exchanges).toEqual([ { path: requests[0], status: 200, body: committedAnswer } ]);
}

async function controllerRefusalHidesSignatureInMountedTxInfo() {
  refuseSignature();
  const view = render(React.createElement(TxInfo, { data: transaction, isLoading: false }));
  await waitFor(() => expect(view.client.getQueryState(signatureQueryKey())?.status).toBe('error'));
  expect(view.client.getQueryState(signatureQueryKey())?.error).toMatchObject({ status: 404, payload: { message: 'Not found' } });
  expect(view.client.getQueryData(signatureQueryKey())).toBeUndefined();
  expect(view.queryByText('Sequencer signature')).toBeNull();
  expect(view.queryByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeNull();
  expect(view.queryByText(answer.signature)).toBeNull();
  expect(view.container.querySelector('[data-rung="receipted"], [data-rung="committed"]')).toBeNull();
  expect(requests).toEqual([ `/api/v2/transactions/${ hash }/signature` ]);
  expect(exchanges).toEqual([ { path: requests[0], status: 404, body: { message: 'Not found' } } ]);
  expect(view.getByText('Block')).toBeDefined();
}

async function controllerRefusalRemovesPreviouslyDisplayedSignature() {
  const view = render(React.createElement(TxInfo, { data: transaction, isLoading: false }));
  expect(await view.findByText(answer.signature)).toBeDefined();
  refuseSignature();
  await act(async() => {
    await view.client.invalidateQueries({ queryKey: signatureQueryKey() });
  });
  await waitFor(() => expect(view.client.getQueryState(signatureQueryKey())?.status).toBe('error'));
  expect(view.client.getQueryState(signatureQueryKey())?.error).toMatchObject({ status: 404 });
  expect(view.queryByText('Sequencer signature')).toBeNull();
  expect(view.queryByText(answer.signature)).toBeNull();
  expect(view.queryByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeNull();
  expect(exchanges.map(exchange => exchange.status)).toEqual([ 200, 404 ]);
}

test(signatureFixtureBindsCanonicalReceiptTransactionAndGovernedKey);
test(receiptedCanonicalSignatureReachesMountedTxInfo);
test(committedCanonicalSignatureReachesMountedTxInfo);
test(controllerRefusalHidesSignatureInMountedTxInfo);
test(controllerRefusalRemovesPreviouslyDisplayedSignature);
