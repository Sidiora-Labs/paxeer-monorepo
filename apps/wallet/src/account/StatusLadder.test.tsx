// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { decodeSequencerReceipt, LADDER_RUNGS, statusLadder } from '@paxeer/wallet';
import { AccountProvider } from './AccountProvider';
import { StatusLadder, TransactionLadder } from './StatusLadder';
import {
  committedAnswer,
  receiptedAnswer,
  submittedAnswer,
  receiptHash,
  receiptPins,
  matchingExplorer,
  startReceiptEndpoint,
} from './test/receiptEndpoint';
import { mount, type Mounted } from './test/render';

let view: Mounted | null = null;
afterEach(async () => {
  await view?.unmount();
  view = null;
});

function rungs(mounted: Mounted): Array<[string, string, string | null]> {
  return mounted
    .all('[data-rung]')
    .map((rung) => [
      rung.getAttribute('data-rung') ?? '',
      rung.getAttribute('data-reached') ?? '',
      rung.getAttribute('data-source'),
    ]);
}

describe('StatusLadder', () => {
  async function transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer() {
    const endpoint = await startReceiptEndpoint();
    const cases = [
      {
        receipt: receiptedAnswer,
        explorer: { rung: 'submitted', block_number: null, committed_height: null },
        committed: false,
      },
      {
        receipt: receiptedAnswer,
        explorer: { rung: 'receipted', block_number: null, committed_height: null },
        committed: false,
      },
      { receipt: receiptedAnswer, explorer: matchingExplorer, committed: false },
      {
        receipt: receiptedAnswer,
        explorer: { ...matchingExplorer, block_number: 1 },
        committed: false,
      },
      {
        receipt: receiptedAnswer,
        explorer: { ...matchingExplorer, block_number: null },
        committed: false,
      },
      {
        receipt: committedAnswer,
        explorer: { rung: 'submitted', block_number: null, committed_height: null },
        committed: true,
      },
      {
        receipt: committedAnswer,
        explorer: { ...matchingExplorer, block_number: 1 },
        committed: true,
      },
      { receipt: committedAnswer, explorer: matchingExplorer, committed: true },
      { receipt: null, explorer: matchingExplorer, committed: false },
    ];
    try {
      for (const scenario of cases) {
        endpoint.state.receipt = scenario.receipt;
        endpoint.state.explorer = scenario.explorer;
        endpoint.state.holdExplorer = true;
        endpoint.state.releaseExplorer = undefined;
        endpoint.seen.length = 0;
        view = await mount(
          <AccountProvider config={endpoint.config}>
            <TransactionLadder hash={receiptHash} observation={{ attempts: 2, intervalMs: 5000 }} />
          </AccountProvider>
        );
        const mounted = view;
        await mounted.until(
          () => endpoint.state.releaseExplorer !== undefined,
          'receipt before explorer'
        );
        const hasReceipt = scenario.receipt !== null;
        const nodeCommitted = scenario.receipt?.rung === 'committed';
        expect(rungs(mounted)).toEqual([
          ['submitted', String(hasReceipt), hasReceipt ? 'receipt' : null],
          ['receipted', String(hasReceipt), hasReceipt ? 'receipt' : null],
          ['committed', String(nodeCommitted), nodeCommitted ? 'receipt' : null],
        ]);
        expect(mounted.query('[aria-busy]')?.getAttribute('aria-busy')).toBe('true');
        expect(endpoint.seen[0]).toEqual({
          method: 'pax_sequencerReceipt',
          params: [receiptHash],
          path: '/',
        });
        expect(endpoint.seen.filter((request) => request.method === 'GET')).toEqual([
          { method: 'GET', path: `/api/v2/transactions/${receiptHash}/status` },
        ]);
        expect(
          endpoint.seen.filter((request) => request.method === 'px_getReceiptKeys')
        ).toHaveLength(hasReceipt ? 1 : 0);
        expect(
          endpoint.seen.filter((request) => request.method === 'eth_getRawTransactionByHash')
        ).toEqual(
          hasReceipt
            ? [{ method: 'eth_getRawTransactionByHash', params: [receiptHash], path: '/' }]
            : []
        );
        endpoint.state.releaseExplorer!();
        await mounted.until(
          () => mounted.query('[data-explorer="ready"]') !== null,
          'explorer merged'
        );
        const preferred = nodeCommitted && scenario.explorer === matchingExplorer;
        expect(rungs(mounted)).toEqual([
          ['submitted', String(hasReceipt), hasReceipt ? 'receipt' : null],
          ['receipted', String(hasReceipt), hasReceipt ? 'receipt' : null],
          [
            'committed',
            String(scenario.committed),
            scenario.committed ? (preferred ? 'explorer' : 'receipt') : null,
          ],
        ]);
        expect(mounted.query('[role="alert"]')).toBeNull();
        expect(endpoint.unexpected).toEqual([]);
        await mounted.unmount();
        view = null;
      }
    } finally {
      endpoint.state.releaseExplorer?.();
      await view?.unmount();
      view = null;
      await endpoint.close();
    }
  }
  it(
    transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer.name,
    transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer
  );

  it.each([
    ['missing', null],
    ['submitted', submittedAnswer],
    ['receipted', receiptedAnswer],
    ['committed', committedAnswer],
  ] as const)(
    'climbs the current ladder from recorded %s evidence at the RPC root',
    async (state, receipt) => {
      const endpoint = await startReceiptEndpoint();
      try {
        endpoint.state.receipt = receipt;
        endpoint.state.explorer = matchingExplorer;
        if (state === 'missing') endpoint.state.explorerStatus = 503;
        view = await mount(
          <AccountProvider config={endpoint.config}>
            <TransactionLadder hash={receiptHash} observation={{ attempts: 2, intervalMs: 5000 }} />
          </AccountProvider>
        );
        const mounted = view;
        await mounted.until(
          () =>
            mounted.query(`[data-explorer="${state === 'missing' ? 'error' : 'ready'}"]`) !== null,
          'explorer settled'
        );
        // The documented rung order determines exactly which receipt evidence reaches each row.
        const receiptRank =
          receipt === null
            ? -1
            : LADDER_RUNGS.indexOf(decodeSequencerReceipt(receipt, receiptHash, receiptPins).rung);
        const expected = LADDER_RUNGS.map((rung, index) => [
          rung,
          String(index <= receiptRank),
          index > receiptRank ? null : rung === 'committed' ? 'explorer' : 'receipt',
        ]);
        expect(rungs(mounted)).toEqual(expected);
        expect(mounted.query('[data-observation="error"]')).toBeNull();
        expect(
          endpoint.seen
            .filter((request) => request.method !== 'GET')
            .every((request) => request.path === '/')
        ).toBe(true);
        expect(endpoint.unexpected).toEqual([]);
      } finally {
        await view?.unmount();
        view = null;
        await endpoint.close();
      }
    }
  );

  it('takes the source of the step at each rung and of the highest step above it', async () => {
    const mounted = await mount(
      <StatusLadder
        steps={[statusLadder.fromExplorer('submitted'), statusLadder.fromJourney('done')]}
      />
    );
    view = mounted;
    expect(rungs(mounted)).toEqual([
      ['submitted', 'true', 'explorer'],
      ['receipted', 'true', 'journey'],
      ['committed', 'false', null],
    ]);
    expect(mounted.query('ol')?.getAttribute('aria-label')).toBe('Status');
  });

  async function status_ladder_labels_submitted_receipted_and_committed() {
    const mounted = await mount(
      <StatusLadder
        steps={[
          statusLadder.fromExplorer('submitted'),
          { rung: 'receipted', source: 'receipt', state: 'receipted' },
          statusLadder.fromJourney('done-finalised'),
        ]}
      />
    );
    view = mounted;
    expect(mounted.all('ol > li > span:first-child').map((label) => label.textContent)).toEqual([
      'Submitted',
      'Receipted',
      'Committed',
    ]);
    expect(rungs(mounted)).toEqual([
      ['submitted', 'true', 'explorer'],
      ['receipted', 'true', 'receipt'],
      ['committed', 'true', 'journey'],
    ]);
    expect(mounted.all('ol > li > span:last-child').map((source) => source.textContent)).toEqual([
      'explorer',
      'receipt',
      'journey',
    ]);
    expect(mounted.query('[data-source="anchor"]')).toBeNull();
    expect(mounted.text('ol')).not.toContain('anchor');
  }

  it(
    status_ladder_labels_submitted_receipted_and_committed.name,
    status_ladder_labels_submitted_receipted_and_committed
  );

  async function status_ladder_prefers_explorer_evidence_at_the_committed_rung() {
    const mounted = await mount(
      <StatusLadder
        steps={[
          { rung: 'committed', source: 'receipt', state: 'committed:0xabc' },
          { rung: 'committed', source: 'explorer', state: 'committed:0xabc' },
        ]}
      />
    );
    view = mounted;
    expect(rungs(mounted)).toEqual([
      ['submitted', 'true', 'receipt'],
      ['receipted', 'true', 'receipt'],
      ['committed', 'true', 'explorer'],
    ]);
    expect(mounted.query('li[data-rung="committed"]')?.getAttribute('data-evidence')).toBe(
      'committed:0xabc'
    );
    await mounted.unmount();
    view = null;

    const receiptOnly = await mount(
      <StatusLadder steps={[{ rung: 'committed', source: 'receipt', state: 'committed:0xabc' }]} />
    );
    view = receiptOnly;
    expect(rungs(receiptOnly)[2]).toEqual(['committed', 'true', 'receipt']);
  }

  it(
    status_ladder_prefers_explorer_evidence_at_the_committed_rung.name,
    status_ladder_prefers_explorer_evidence_at_the_committed_rung
  );

  it('reaches nothing for a journey state without a rung', async () => {
    const mounted = await mount(<StatusLadder steps={[statusLadder.fromJourney('refused')]} />);
    view = mounted;
    expect(rungs(mounted).every(([, reached]) => reached === 'false')).toBe(true);
  });
});
