// @vitest-environment jsdom

import { waitFor } from '@testing-library/react';

import { expect, test } from 'vitest';

import { answer, hash, missingHash, renderSignature, requests } from './signatureTestFixture';

async function signature_row_renders_key_and_signature() {
  const { container, findByText } = renderSignature(hash);

  expect(await findByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeDefined();
  expect(await findByText(answer.signature)).toBeDefined();
  expect(answer.signature).toMatch(/^0x[0-9a-f]{128}$/);
  expect(container.querySelector('[data-rung="receipted"]')?.textContent).toBe('Receipted');
  expect(container.textContent).toContain('Sequencer signature');
  expect(requests).toEqual([ `/api/v2/transactions/${ hash }/signature` ]);
}

async function signature_row_hidden_on_404() {
  const { container, client: queryClient } = renderSignature(missingHash);
  const queryKey = [ 'general:paxeer_x_tx_signature', { hash: missingHash } ];

  await waitFor(() => expect(queryClient.getQueryState(queryKey)?.status).toBe('error'));

  expect(queryClient.getQueryState(queryKey)?.error).toMatchObject({ status: 404 });
  expect(requests).toEqual([ `/api/v2/transactions/${ missingHash }/signature` ]);
  expect(container.innerHTML).toBe('');
}

test(signature_row_renders_key_and_signature);
test(signature_row_hidden_on_404);
