// @vitest-environment jsdom
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { act, createElement, Fragment } from 'react';
import { expect, test } from 'vitest';
import { AccountProvider } from './AccountProvider';
import { AssetList } from './AssetList';
import { ACCOUNT_ENV, readAccountConfig, resolveAccountConfig } from './config';
import { TransactionLadder } from './StatusLadder';
import { OWNER, recordings } from './test/endpoint';
import { mount, type Mounted } from './test/render';
import {
  committedAnswer,
  receiptedAnswer,
  submittedAnswer,
  receiptHash,
  matchingExplorer,
  recordedReceipt,
  startReceiptEndpoint,
} from './test/receiptEndpoint';
import './AssetList.test';
import './KernelPlan.test';
import './StatusLadder.test';

function reached(view: Mounted, rung: string): string | null {
  return view.query(`[data-rung="${rung}"]`)?.getAttribute('data-reached') ?? null;
}

async function receiptView(
  endpoint: Awaited<ReturnType<typeof startReceiptEndpoint>>,
  attempts = 30,
  intervalMs = 50
) {
  return mount(
    createElement(AccountProvider, {
      config: endpoint.config,
      children: createElement(TransactionLadder, {
        hash: receiptHash,
        observation: { attempts, intervalMs },
      }),
    })
  );
}

async function ladder_observes_pending_receipts_until_commitment() {
  const endpoint = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    endpoint.state.sequence = [null, submittedAnswer, receiptedAnswer, committedAnswer];
    endpoint.state.receipt = committedAnswer;
    endpoint.state.explorer = matchingExplorer;
    view = await receiptView(endpoint);
    const mounted = view;
    await mounted.until(
      () => mounted.query('[data-observation="watching"]') !== null,
      'pending observation'
    );
    expect(reached(mounted, 'receipted')).toBe('false');
    expect(mounted.text('[role="status"]')).toBe('Checking for block commitment…');
    await mounted.until(
      () => mounted.query('[data-rung="submitted"][data-source="receipt"]') !== null,
      'submitted receipt'
    );
    expect(reached(mounted, 'receipted')).toBe('false');
    await mounted.until(() => reached(mounted, 'receipted') === 'true', 'receipted');
    expect(reached(mounted, 'committed')).toBe('false');
    expect(recordedReceipt('receipted').rung).toBe('receipted');
    await mounted.until(
      () => mounted.query('[data-rung="committed"][data-source="explorer"]') !== null,
      'committed'
    );
    expect(recordedReceipt('committed').rung).toBe('committed');
    expect(reached(mounted, 'committed')).toBe('true');
    expect(mounted.query('[aria-busy]')?.getAttribute('aria-busy')).toBe('false');
    expect(mounted.query('[role="status"]')).toBeNull();
    expect(mounted.query('[role="alert"]')).toBeNull();
    expect(
      endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt').length
    ).toBeGreaterThanOrEqual(4);
    expect(
      endpoint.seen.filter((entry) => entry.method !== 'GET').every((entry) => entry.path === '/')
    ).toBe(true);
    expect(endpoint.unexpected).toEqual([]);
  } finally {
    await view?.unmount();
    await endpoint.close();
  }
}

async function ladder_keeps_committed_receipts_with_unavailable_or_lagging_explorer() {
  for (const mode of ['unavailable', 'lagging', 'wrong-block', 'matching'] as const) {
    const endpoint = await startReceiptEndpoint();
    let view: Mounted | null = null;
    try {
      endpoint.state.receipt = committedAnswer;
      endpoint.state.explorerStatus = mode === 'unavailable' ? 503 : 200;
      endpoint.state.explorer =
        mode === 'lagging'
          ? { rung: 'receipted', block_number: null, committed_height: null }
          : mode === 'wrong-block'
            ? { ...matchingExplorer, block_number: 1 }
            : matchingExplorer;
      view = await receiptView(endpoint);
      const mounted = view;
      await mounted.until(
        () =>
          mounted.query(`[data-explorer="${mode === 'unavailable' ? 'error' : 'ready'}"]`) !== null,
        mode
      );
      expect(reached(mounted, 'committed')).toBe('true');
      expect(mounted.query('[data-rung="committed"]')?.getAttribute('data-source')).toBe(
        mode === 'matching' ? 'explorer' : 'receipt'
      );
      expect(mounted.query('[data-rung="committed"]')?.getAttribute('data-evidence')).toBe(
        'committed'
      );
      expect(mounted.query('[aria-busy]')?.getAttribute('aria-busy')).toBe('false');
      expect(endpoint.unexpected).toEqual([]);
      if (mode === 'unavailable') expect(mounted.text('[role="alert"]')).toContain('HTTP 503');
      else expect(mounted.query('[role="alert"]')).toBeNull();
    } finally {
      await view?.unmount();
      await endpoint.close();
    }
  }
}

async function ladder_bounds_pending_reads_and_retries_visibly() {
  const endpoint = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    endpoint.state.receipt = receiptedAnswer;
    view = await receiptView(endpoint, 2, 20);
    const mounted = view;
    await mounted.until(
      () => mounted.query('[data-observation="error"]') !== null,
      'bounded retry'
    );
    expect(mounted.text('[role="alert"]')).toContain('allotted attempts');
    expect(reached(mounted, 'receipted')).toBe('true');
    expect(reached(mounted, 'committed')).toBe('false');
    expect(endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt')).toHaveLength(
      2
    );
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 70));
    });
    expect(endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt')).toHaveLength(
      2
    );
    endpoint.state.receipt = committedAnswer;
    endpoint.state.explorer = matchingExplorer;
    await mounted.click('[role="alert"] button');
    await mounted.until(() => reached(mounted, 'committed') === 'true', 'retry commitment');
    expect(mounted.query('[role="alert"]')).toBeNull();
    expect(endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt')).toHaveLength(
      3
    );
  } finally {
    await view?.unmount();
    await endpoint.close();
  }
}

async function ladder_cancels_waiting_and_inflight_reads_and_reopens_from_node_evidence() {
  const endpoint = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    view = await receiptView(endpoint, 30, 200);
    const mounted = view;
    await mounted.until(() => mounted.query('[data-observation="watching"]') !== null, 'pending');
    await view.unmount();
    view = null;
    const reads = endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt').length;
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 250));
    });
    expect(endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt')).toHaveLength(
      reads
    );
    endpoint.state.holdReceipt = true;
    view = await receiptView(endpoint);
    const inflight = view;
    await inflight.until(() => endpoint.state.releaseReceipt !== undefined, 'inflight read');
    await view.unmount();
    view = null;
    await inflight.until(() => endpoint.state.abortedReceipts === 1, 'aborted transport');
    endpoint.state.holdReceipt = false;
    endpoint.state.receipt = committedAnswer;
    endpoint.state.explorer = matchingExplorer;
    view = await receiptView(endpoint);
    const reopened = view;
    await reopened.until(() => reached(reopened, 'committed') === 'true', 'reopened commitment');
    expect(reopened.query('[data-rung="committed"]')?.getAttribute('data-evidence')).toBe(
      'committed'
    );
    expect(endpoint.unexpected).toEqual([]);
  } finally {
    await view?.unmount();
    await endpoint.close();
  }
}

async function ladder_refuses_tampered_receipt_evidence() {
  const endpoint = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    endpoint.state.receipt = { ...committedAnswer, feeAmount: '0x0' };
    view = await receiptView(endpoint);
    const mounted = view;
    await mounted.until(
      () => mounted.query('[data-observation="error"]') !== null,
      'receipt refusal'
    );
    expect(mounted.text('[role="alert"]')).toContain('unauthenticated');
    expect(reached(mounted, 'receipted')).toBe('false');
    expect(reached(mounted, 'committed')).toBe('false');
    expect(endpoint.seen.filter((entry) => entry.method === 'pax_sequencerReceipt')).toHaveLength(
      1
    );
  } finally {
    await view?.unmount();
    await endpoint.close();
  }
}

async function asset_rows_decode_numeric_heights_and_nullable_metadata() {
  const exchanges = recordings('chain');
  const balance = exchanges.find((entry) => entry.request.method === 'px_getBalances')!.response
    .result;
  const seen: string[] = [];
  const server = createServer(async (req, res) => {
    let text = '';
    for await (const chunk of req) text += chunk;
    const call = JSON.parse(text) as { id: number; method: string };
    seen.push(`${req.method} ${req.url} ${call.method}`);
    const result =
      call.method === 'px_getBalances'
        ? {
            ...(balance as object),
            height: 1000,
            balances: [{ denom: 'uopaque', amount: '9007199254740993' }],
          }
        : call.method === 'px_listAssets'
          ? {
              height: 1000,
              joined_limit: 16,
              assets: [
                {
                  asset_id: 'denom:uopaque',
                  denom: 'uopaque',
                  kind: 'bank',
                  name: null,
                  symbol: null,
                  decimals: null,
                  supply: null,
                },
              ],
            }
          : undefined;
    res.setHeader('content-type', 'application/json');
    res.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: call.id,
        ...(result === undefined
          ? { error: { code: -32601, message: 'unrecorded request' } }
          : { result }),
      })
    );
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const node = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    node.state.receipt = committedAnswer;
    node.state.explorer = matchingExplorer;
    const env = {
      [ACCOUNT_ENV.endpointUrl]: node.base,
      [ACCOUNT_ENV.hostedUrl]: `${base}/`,
      [ACCOUNT_ENV.humanUrl]: node.base,
      [ACCOUNT_ENV.explorerUrl]: node.base,
    };
    const config = readAccountConfig(env);
    expect(config.hostedUrl).toBe(base);
    for (const hostedUrl of ['', 'invalid', 'ftp://hosted.test', 'https://user:pass@hosted.test']) {
      const refused = resolveAccountConfig({ ...env, [ACCOUNT_ENV.hostedUrl]: hostedUrl });
      expect(refused.ok).toBe(false);
      if (!refused.ok) expect(refused.error.variable).toBe(ACCOUNT_ENV.hostedUrl);
    }
    view = await mount(
      createElement(AccountProvider, {
        config,
        children: createElement(
          Fragment,
          null,
          createElement(AssetList, { account: OWNER }),
          createElement(TransactionLadder, { hash: receiptHash })
        ),
      })
    );
    const mounted = view;
    await mounted.until(
      () => mounted.query('[data-denom="uopaque"]') !== null,
      'nullable bank metadata'
    );
    await mounted.until(
      () => mounted.query('[data-rung="committed"][data-source="explorer"]') !== null,
      'separate node receipt transport'
    );
    expect(mounted.all('[data-denom]')).toHaveLength(1);
    expect(mounted.text('[aria-label="uopaque balance"]')).toBe('9007199254740993uopaque');
    expect(mounted.query('[data-side]')).toBeNull();
    expect(mounted.query('[role="alert"]')).toBeNull();
    expect(seen.sort()).toEqual(['POST / px_getBalances', 'POST / px_listAssets']);
    expect(node.seen.filter((entry) => entry.method !== 'GET')).toEqual([
      { method: 'pax_sequencerReceipt', params: [receiptHash], path: '/' },
      { method: 'px_getReceiptKeys', params: [], path: '/' },
      { method: 'eth_getRawTransactionByHash', params: [receiptHash], path: '/' },
    ]);
    expect(node.unexpected).toEqual([]);
  } finally {
    await view?.unmount();
    await node.close();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

async function account_bank_reads_require_an_explicit_hosted_origin() {
  const endpoint = await startReceiptEndpoint();
  let view: Mounted | null = null;
  try {
    view = await mount(
      createElement(AccountProvider, {
        config: endpoint.config,
        children: createElement(AssetList, { account: OWNER }),
      })
    );
    const mounted = view;
    await mounted.until(() => mounted.query('[role="alert"]') !== null, 'missing hosted origin');
    expect(mounted.text('[role="alert"]')).toContain('hostedUrl required for hosted px_ reads');
    expect(mounted.query('[data-denom]')).toBeNull();
    expect(endpoint.seen).toEqual([]);
    expect(endpoint.unexpected).toEqual([]);
  } finally {
    await view?.unmount();
    await endpoint.close();
  }
}

test(
  ladder_observes_pending_receipts_until_commitment.name,
  ladder_observes_pending_receipts_until_commitment
);
test(
  ladder_keeps_committed_receipts_with_unavailable_or_lagging_explorer.name,
  ladder_keeps_committed_receipts_with_unavailable_or_lagging_explorer
);
test(
  ladder_bounds_pending_reads_and_retries_visibly.name,
  ladder_bounds_pending_reads_and_retries_visibly
);
test(
  ladder_cancels_waiting_and_inflight_reads_and_reopens_from_node_evidence.name,
  ladder_cancels_waiting_and_inflight_reads_and_reopens_from_node_evidence
);
test(ladder_refuses_tampered_receipt_evidence.name, ladder_refuses_tampered_receipt_evidence);
test(
  asset_rows_decode_numeric_heights_and_nullable_metadata.name,
  asset_rows_decode_numeric_heights_and_nullable_metadata
);
test(
  account_bank_reads_require_an_explicit_hosted_origin.name,
  account_bank_reads_require_an_explicit_hosted_origin
);
