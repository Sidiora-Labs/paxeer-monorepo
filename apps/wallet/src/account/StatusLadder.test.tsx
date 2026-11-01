// @vitest-environment jsdom
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import {
  decodeExplorerStatus,
  decodeSequencerReceipt,
  statusLadder,
  type ExplorerTransactionStatus,
  type SequencerReceiptAnswer,
} from '@paxeer/wallet';
import receiptAnswers from '../../../../sdk/wallet/test/fixtures/sequencer-receipt.json';
import explorerAnswers from '../../../../sdk/wallet/test/fixtures/explorer-status.json';
import { AccountProvider } from './AccountProvider';
import { StatusLadder, TransactionLadder } from './StatusLadder';
import { startAccountEndpoint, type AccountEndpoint } from './test/endpoint';
import { mount, type Mounted } from './test/render';

let endpoint: AccountEndpoint;
let view: Mounted | null = null;

beforeAll(async () => {
  endpoint = await startAccountEndpoint();
});

afterAll(async () => {
  await endpoint.close();
});

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
    const receipted = decodeSequencerReceipt({
      ...receiptAnswers.receipted,
      blockNumber: receiptAnswers.committed.blockNumber,
      blockHash: receiptAnswers.committed.blockHash,
    });
    const committed = decodeSequencerReceipt(receiptAnswers.committed);
    const hash = receipted.txHash;
    let receipt: SequencerReceiptAnswer | null = receipted;
    let explorer: ExplorerTransactionStatus = decodeExplorerStatus(explorerAnswers.submitted);
    const pending: { releaseExplorer?: () => void } = {};
    const requests: Array<{ method: string; params?: unknown[]; path: string }> = [];
    const server = createServer(async (req, res) => {
      res.setHeader('content-type', 'application/json');
      if (req.method === 'POST' && req.url === '/rpc') {
        let body = '';
        for await (const chunk of req) body += chunk;
        const call = JSON.parse(body) as { id: number; method: string; params: unknown[] };
        requests.push({ method: call.method, params: call.params, path: req.url });
        if (
          call.method === 'pax_sequencerReceipt' &&
          JSON.stringify(call.params) === JSON.stringify([hash])
        ) {
          res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result: receipt }));
          return;
        }
      } else if (req.method === 'GET' && req.url === `/api/v2/transactions/${hash}/status`) {
        requests.push({ method: 'GET', path: req.url });
        pending.releaseExplorer = () => res.end(JSON.stringify(explorer));
        return;
      }
      res.writeHead(404);
      res.end(JSON.stringify({ message: 'Unrecorded request' }));
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    const cases = [
      { receipt: receipted, explorer: explorerAnswers.submitted, committed: false },
      { receipt: receipted, explorer: explorerAnswers.receipted, committed: false },
      { receipt: receipted, explorer: explorerAnswers.committed, committed: true },
      {
        receipt: receipted,
        explorer: { ...explorerAnswers.committed, block_number: 999 },
        committed: false,
      },
      {
        receipt: receipted,
        explorer: { ...explorerAnswers.committed, block_number: null },
        committed: false,
      },
      { receipt: committed, explorer: explorerAnswers.submitted, committed: false },
      {
        receipt: committed,
        explorer: { ...explorerAnswers.committed, block_number: 999 },
        committed: false,
      },
      { receipt: committed, explorer: explorerAnswers.committed, committed: true },
      { receipt: null, explorer: explorerAnswers.committed, committed: false },
    ];
    try {
      for (const scenario of cases) {
        receipt = scenario.receipt;
        explorer = decodeExplorerStatus(scenario.explorer);
        pending.releaseExplorer = undefined;
        requests.length = 0;
        const mounted = await mount(
          <AccountProvider
            config={{
              ...endpoint.config('available'),
              endpointUrl: `${base}/rpc`,
              explorerUrl: base,
            }}
          >
            <TransactionLadder hash={hash} />
          </AccountProvider>
        );
        view = mounted;
        await mounted.until(
          () => mounted.query('[data-rung]') !== null && pending.releaseExplorer !== undefined,
          'receipt ladder before the explorer response'
        );
        expect(rungs(mounted)).toEqual([
          ['submitted', receipt !== null ? 'true' : 'false', receipt !== null ? 'receipt' : null],
          ['receipted', receipt !== null ? 'true' : 'false', receipt !== null ? 'receipt' : null],
          ['committed', 'false', null],
        ]);
        expect(mounted.query('[aria-busy]')?.getAttribute('aria-busy')).toBe('true');
        expect(requests).toEqual([
          { method: 'pax_sequencerReceipt', params: [hash], path: '/rpc' },
          { method: 'GET', path: `/api/v2/transactions/${hash}/status` },
        ]);
        pending.releaseExplorer!();
        pending.releaseExplorer = undefined;
        await mounted.until(
          () => mounted.query('[aria-busy]')?.getAttribute('aria-busy') === 'false',
          'explorer evidence merged'
        );
        expect(rungs(mounted)).toEqual([
          [
            'submitted',
            receipt !== null || scenario.committed ? 'true' : 'false',
            receipt !== null ? 'receipt' : scenario.committed ? 'explorer' : null,
          ],
          [
            'receipted',
            receipt !== null || scenario.committed ? 'true' : 'false',
            receipt !== null ? 'receipt' : scenario.committed ? 'explorer' : null,
          ],
          ['committed', String(scenario.committed), scenario.committed ? 'explorer' : null],
        ]);
        expect(mounted.query('[role="alert"]')).toBeNull();
        await mounted.unmount();
        view = null;
      }
    } finally {
      pending.releaseExplorer?.();
      await view?.unmount();
      view = null;
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve()))
      );
    }
  }

  it(transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer);

  it.each([
    [
      'e1',
      [
        ['instant', 'false', null],
        ['sealed', 'false', null],
        ['final', 'false', null],
      ],
    ],
    [
      'e2',
      [
        ['instant', 'true', 'explorer'],
        ['sealed', 'false', null],
        ['final', 'false', null],
      ],
    ],
    [
      'e3',
      [
        ['instant', 'true', 'explorer'],
        ['sealed', 'true', 'explorer'],
        ['final', 'false', null],
      ],
    ],
    [
      'e4',
      [
        ['instant', 'true', 'explorer'],
        ['sealed', 'true', 'explorer'],
        ['final', 'true', 'explorer'],
      ],
    ],
  ] as const)(
    'climbs the ladder from the recorded explorer status of 0x%s',
    async (byte, expected) => {
      const mounted = await mount(
        <AccountProvider config={endpoint.config('available')}>
          <TransactionLadder hash={`0x${byte.repeat(32)}`} />
        </AccountProvider>
      );
      view = mounted;
      await mounted.until(() => mounted.query('[data-rung]') !== null, 'ladder');
      expect(rungs(mounted)).toEqual(expected);
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

  it(status_ladder_labels_submitted_receipted_and_committed);

  it('reaches nothing for a journey state without a rung', async () => {
    const mounted = await mount(<StatusLadder steps={[statusLadder.fromJourney('refused')]} />);
    view = mounted;
    expect(rungs(mounted).every(([, reached]) => reached === 'false')).toBe(true);
  });
});
