// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, test, inject } from 'vitest';
import { XWEB_PRECOMPILE, abiSelector } from '@sidiora/layerx-sdk/browser';
import {
  GasStationError,
  LEDGER_PRECOMPILE,
  WEB_DATA_EVENTS,
  WEB_DATA_KIND_SEARCH,
  webData,
  type WalletCapsState,
} from '@paxeer/wallet';
import { toHex } from 'viem';
import { WebDataView, buildWebDataRequest } from './WebDataView';
import {
  type SurfaceCapsSnapshot,
  GRANT_LOG_WINDOW,
  GRANT_LOG_WINDOW_CEILING,
  boundDid,
  decodePayerGrantIssued,
  readGrantCaps,
  useWebDataCaps,
} from './useSurface';
import {
  SurfaceChain,
  click,
  eventVectors,
  expectedField,
  mountSurface,
  radio,
  text,
  typeInto,
  until,
  vectorLog,
  type Mounted,
} from './test/chain';

import { RecordedSurfaceChain } from './test/recordedReceipts';

let chain: SurfaceChain;
let mounted: Mounted | null = null;

beforeEach(() => {
  chain = new RecordedSurfaceChain();
  chain.xwebFee = 1_000_000_000_000_000n;
});

afterEach(async () => {
  await mounted?.unmount();
  mounted = null;
});

const BUDGET_ID = 'b1'.repeat(32);
const GRANT_ID = 'a1'.repeat(32);
const CAPS: SurfaceCapsSnapshot = {
  budgetAvailability: 'available',
  state: 'ready',
  did: 'did:layerx:caps-fixture',
  account_id: 'c3'.repeat(32),
  network_id: 229,
  context: {
    principal: 'caps-fixture',
    session_id: 'session-1',
    address: `0x${'11'.repeat(20)}`,
    chain_id: 229,
    expires_at: '1900000000',
  },
  observation: { rung: 'committed', block_number: '42', state_root: 'e5'.repeat(32) },
  budgets: [
    {
      id: BUDGET_ID,
      owner: 'c3'.repeat(32),
      asset: 'd4'.repeat(32),
      account: 'c3'.repeat(32),
      source_account: null,
      limit: '250000',
      configured_limit: '250000',
      spent: '1000',
      remaining: '249000',
      carry_cap: '0',
      carried: '0',
      period_start: '1800000000',
      period_length: '86400',
      expiry: '1900000000',
      revocation_height: '0',
      closed: false,
      revoked: false,
    },
  ],
  grants: [
    {
      id: GRANT_ID,
      owner: 'c3'.repeat(32),
      recipient: 'f6'.repeat(32),
      asset: 'd4'.repeat(32),
      per_draw_maximum: '1000',
      allowance: '50000',
      drawn_total: '0',
      drawn_this_period: '0',
      recurring: true,
      window_length: '86400',
      window_start: '1800000000',
      expiration: '1900000000',
      revocation_height: '0',
      revoked_at_height: '0',
      revoked: false,
      invoice_settled: false,
    },
  ],
};
const UNAVAILABLE: WalletCapsState = { state: 'unavailable', reason: 'consensus_unavailable' };

const send = (container: HTMLElement) =>
  container.querySelector<HTMLButtonElement>('[data-action="send"]');

import {
  DID,
  ACCOUNT_ID,
  FLOOR,
  HEAD,
  GET_PAYER_GRANT,
  PAYER_GRANT_ISSUED,
  OWNED_GRANT,
  FOREIGN_GRANT,
  issuingTx,
  GrantChain,
  type IssuedGrant,
} from './test/grants';

function GrantSurface() {
  const { caps, refreshCaps } = useWebDataCaps();
  return <WebDataView sidRate={null} caps={caps} refreshCaps={refreshCaps} />;
}

async function expectRetiredGrantCaps(grants: GrantChain, container?: HTMLElement) {
  const head = grants.finalised;
  if (!head) throw new Error('retired caps require a finalized observation');
  const reason = grants.xwebAvailable
    ? `Payer grants are retired. Web requests are available and pay ${grants.xwebFee} wei as transaction value.`
    : 'Payer grants are retired. Web requests are currently paused.';
  const caps = await readGrantCaps(grants, grants.address);
  expect(caps).toEqual({
    state: 'retired',
    reason,
    payment: {
      model: 'request-value',
      available: grants.xwebAvailable,
      fee: grants.xwebFee,
      payerGrants: 'retired',
    },
    did: DID,
    network_id: grants.capsChainId,
    observation: {
      rung: 'committed',
      block_number: BigInt(head.number).toString(),
      state_root: head.stateRoot.slice(2),
    },
  });
  expect(grants.blockQueries.every((block) => block === 'finalized')).toBe(true);
  expect(grants.calls).toContainEqual({
    to: XWEB_PRECOMPILE,
    data: abiSelector('getParams()'),
  });
  expect(grants.logQueries).toEqual([]);
  expect(grants.holderQueries).toEqual([]);
  expect(grants.txQueries).toEqual([]);
  expect(grants.peakInFlight).toBe(0);
  expect(grants.calls.some((call) => call.data.startsWith(GET_PAYER_GRANT))).toBe(false);
  expect(grants.methods).not.toContain('lx_getWalletCaps');
  if (container) {
    await until(
      () => container.querySelector('[data-caps-state="retired"]') !== null,
      'retired caps'
    );
    expect(
      text(
        container,
        '[data-caps-state="retired"] [role="status"]:not([data-role="budgets-unavailable"])'
      )
    ).toBe(reason);
    expect(text(container, '[data-role="budgets-unavailable"]')).toBe(
      'Budget discovery is unavailable. Budget balances have not been read.'
    );
    expect(container.querySelector('[data-budget], [data-grant]')).toBeNull();
    expect(
      container.querySelector('[data-role="caps-empty"], [data-role="caps-discovery"]')
    ).toBeNull();
    expect(
      container.querySelector('[data-role="caps-expiry"], [data-role="caps-history"]')
    ).toBeNull();
  }
  return caps;
}

describe('WebDataView', () => {
  it('web_data_view_sends_a_search_carrying_the_precompile_fee_and_shows_the_decoded_event', async () => {
    const vector = eventVectors('webData').find((candidate) => candidate.event === 'XWebRequested');
    if (!vector) throw new Error('no XWebRequested vector');
    chain.logsFor = () => [vectorLog(XWEB_PRECOMPILE, WEB_DATA_EVENTS, vector)];
    let refreshed = 0;
    mounted = await mountSurface(
      chain,
      <WebDataView
        sidRate={null}
        caps={UNAVAILABLE}
        refreshCaps={() => {
          refreshed += 1;
        }}
      />
    );
    const { container } = mounted;

    await until(
      () => text(container, '[data-role="call-value"]') === 'This call carries 0.001 PAX',
      'fee read'
    );
    await click(radio(container, 'Web data action', 'Search'), 'search action');
    await typeInto(container, 'query', 'paxeer network');
    await typeInto(container, 'callbackGas', '150000');
    await until(() => send(container)?.disabled === false, 'send enabled');
    await click(send(container), 'send');
    await until(
      () => container.querySelector('[data-role="sent"], [data-role="send-error"]') !== null,
      'receipt'
    );
    expect(text(container, '[data-role="send-error"]')).toBe('');
    expect(text(container, '[data-role="status"]')).toBe('confirmed');

    const expected = webData(chain).search('paxeer network', 150_000n, chain.xwebFee);
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: XWEB_PRECOMPILE,
        data: expected.data,
        value: `0x${chain.xwebFee.toString(16)}`,
      },
    ]);
    const data = chain.sent[0]?.data ?? '';
    expect(data.startsWith(abiSelector('request(uint8,bytes,uint64)'))).toBe(true);
    expect(BigInt(`0x${data.slice(10, 74)}`)).toBe(BigInt(WEB_DATA_KIND_SEARCH));
    expect(
      chain.calls.some((call) => call.to === XWEB_PRECOMPILE && call.data === abiSelector('fee()'))
    ).toBe(true);

    const event = container.querySelector('[data-event="XWebRequested"]');
    expect(event).not.toBeNull();
    const spec = WEB_DATA_EVENTS.find((candidate) => candidate.name === 'XWebRequested');
    if (!spec) throw new Error('no XWebRequested spec');
    for (const [name, value] of Object.entries(vector.fields)) {
      expect(event?.querySelector(`[data-field="${name}"]`)?.textContent).toBe(
        expectedField(spec, name, value)
      );
    }

    expect(
      container.querySelector(
        '[data-caps-state="unavailable"] [role="status"]:not([data-role="budgets-unavailable"])'
      )?.textContent
    ).toBe('consensus_unavailable');
    expect(container.querySelector('[data-grant]')).toBeNull();
    expect(refreshed).toBeGreaterThan(0);
  });

  it('web_data_view_requests_a_refund_through_the_module', async () => {
    mounted = await mountSurface(
      chain,
      <WebDataView sidRate={null} caps={UNAVAILABLE} refreshCaps={() => undefined} />
    );
    const { container } = mounted;
    await click(radio(container, 'Web data action', 'Refund'), 'refund action');
    await typeInto(container, 'requestId', '42');
    await until(() => send(container)?.disabled === false, 'send enabled');
    await click(send(container), 'send');
    await until(
      () => container.querySelector('[data-role="sent"], [data-role="send-error"]') !== null,
      'receipt'
    );
    expect(text(container, '[data-role="send-error"]')).toBe('');
    expect(text(container, '[data-role="status"]')).toBe('confirmed');
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: XWEB_PRECOMPILE,
        data: webData(chain).refund(42n).data,
        value: '0x0',
      },
    ]);
    expect(chain.sent[0]?.data.startsWith(abiSelector('refund(uint64)'))).toBe(true);
  });

  it('web_data_view_shows_402_draws_with_their_budget_and_allowance_caps', async () => {
    mounted = await mountSurface(
      chain,
      <WebDataView sidRate={null} caps={CAPS} refreshCaps={() => undefined} />
    );
    const { container } = mounted;
    expect(text(container, '[data-role="caps-observation"]')).toBe('committed · block 42');
    expect(text(container, '[data-role="caps-height"]')).toBe('Observed at block 42');
    expect(text(container, '[data-role="caps-state-root"]')).toBe(`State root ${'e5'.repeat(32)}`);
    expect(text(container, '[data-role="caps-principal"]')).toBe('Principal caps-fixture');
    expect(text(container, '[data-role="caps-expiry"]')).toBe('Earliest grant expiry 1900000000');
    const budget = container.querySelector(`[data-budget="${BUDGET_ID}"]`);
    expect(budget?.querySelector('[data-role="budget-cap"]')?.textContent).toBe(
      'Period cap: 250000 units · spent 1000 · remaining 249000'
    );
    const grant = container.querySelector(`[data-grant="${GRANT_ID}"]`);
    expect(grant?.querySelector('[data-role="draw-cap"]')?.textContent).toBe(
      'Per-draw cap: 1000 units'
    );
    expect(grant?.querySelector('[data-role="allowance-cap"]')?.textContent).toBe(
      'Allowance cap: 50000 units · drawn 0'
    );
    expect(grant?.textContent).toContain('Window 1800000000 + 86400');
  });

  it('web_data_view_reads_the_connected_account_caps_by_grant_id', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.issued = [FOREIGN_GRANT, OWNED_GRANT];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  async function web_data_view_shows_caps_observed_at_the_committed_block() {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.finalised = { ...HEAD, number: '0x384' };
    grants.issued = [{ ...OWNED_GRANT, block: 899n }];
    grants.holderRecords = () => ({
      items: [
        {
          kind: 'grant',
          holder: grants.address.toLowerCase(),
          contract: 'ledger',
          record_id: GRANT_ID,
          state: 'active',
        },
      ],
      next_cursor: null,
      indexedHeight: 900,
      chainHeight: 900,
    });
    const caps = await expectRetiredGrantCaps(grants);
    expect(grants.blockQueries).toEqual(['finalized']);
    expect(grants.callBlocks).toEqual(['0x384', '0x384']);
    expect(caps).not.toHaveProperty('grants');
    expect(caps).not.toHaveProperty('context');
    mounted = await mountSurface(
      grants,
      <WebDataView sidRate={null} caps={caps} refreshCaps={() => undefined} />
    );
    await expectRetiredGrantCaps(grants, mounted.container);
  }

  test(
    web_data_view_shows_caps_observed_at_the_committed_block.name,
    web_data_view_shows_caps_observed_at_the_committed_block
  );

  it('web_data_view_shows_empty_caps_for_a_bound_account_with_no_grant', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.issued = [FOREIGN_GRANT];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_refuses_caps_for_an_address_with_no_bound_did', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.didPublicKey = `0x${'00'.repeat(32)}`;
    grants.issued = [OWNED_GRANT];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;

    await until(
      () => container.querySelector('[data-caps-state="refused"]') !== null,
      'caps refused'
    );
    expect(
      container.querySelector(
        '[data-caps-state="refused"] [role="status"]:not([data-role="budgets-unavailable"])'
      )?.textContent
    ).toBe('The account or its evidence was refused.');
    expect(grants.methods).not.toContain('eth_getLogs');
  });

  it('web_data_view_quote_did_is_refused_for_an_address_with_no_bound_did', async () => {
    const grants = new GrantChain();
    await expect(boundDid(grants, grants.address)).resolves.toBe(DID);
    grants.didPublicKey = `0x${'00'.repeat(32)}`;
    const refusal = await boundDid(grants, grants.address).then(
      () => null,
      (cause: unknown) => cause
    );
    expect(refusal).toBeInstanceOf(GasStationError);
    expect((refusal as GasStationError).refusal).toEqual({ code: 'refused', field: 'did' });
  });

  it('web_data_view_reports_caps_unavailable_without_a_finalised_checkpoint', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.finalised = null;
    grants.issued = [OWNED_GRANT];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;

    await until(
      () => container.querySelector('[data-caps-state="unavailable"]') !== null,
      'caps unavailable'
    );
    expect(
      container.querySelector(
        '[data-caps-state="unavailable"] [role="status"]:not([data-role="budgets-unavailable"])'
      )?.textContent
    ).toBe('consensus_unavailable');
    expect(container.querySelector('[data-role="caps-observation"]')).toBeNull();
    expect(grants.methods).not.toContain('eth_getLogs');
  });

  it('web_data_view_reads_grant_logs_back_from_the_finalised_head_in_bounded_windows', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.finalised = { ...HEAD, number: toHex(FLOOR + 2n * GRANT_LOG_WINDOW + 5n) };
    grants.issued = [
      { ...OWNED_GRANT, block: FLOOR + 3n },
      { ...FOREIGN_GRANT, block: FLOOR + 2n * GRANT_LOG_WINDOW },
    ];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_keeps_every_grant_and_the_earliest_expiry_across_all_windows', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    const head = FLOOR + 30n * GRANT_LOG_WINDOW;
    const older: IssuedGrant = {
      ...OWNED_GRANT,
      grantId: `0x${'a3'.repeat(32)}`,
      block: FLOOR + 2n,
      expiration: 1_850_000_000n,
    };
    grants.finalised = { ...HEAD, number: toHex(head) };
    grants.issued = [{ ...OWNED_GRANT, block: head - 5n }, older];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_scans_every_window_down_to_the_ledger_floor_and_never_below_it', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    const head = FLOOR + 25n * GRANT_LOG_WINDOW + 7n;
    grants.finalised = { ...HEAD, number: toHex(head) };
    grants.issued = [{ ...OWNED_GRANT, block: FLOOR - 1n }];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_reads_grant_windows_in_bounded_parallel_batches', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.finalised = { ...HEAD, number: toHex(FLOOR + 20n * GRANT_LOG_WINDOW) };
    grants.issued = [OWNED_GRANT];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_reports_caps_unavailable_when_any_grant_window_fails', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.finalised = { ...HEAD, number: toHex(FLOOR + 12n * GRANT_LOG_WINDOW) };
    grants.issued = [{ ...OWNED_GRANT, block: FLOOR + 12n * GRANT_LOG_WINDOW }];
    grants.failLogsFrom = FLOOR;
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });
  it('web_data_view_reads_a_sponsored_grant_sent_to_the_account_and_no_foreign_grant', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    const sponsoredForeign: IssuedGrant = {
      ...FOREIGN_GRANT,
      grantId: `0x${'a4'.repeat(32)}`,
      sponsored: true,
    };
    grants.issued = [FOREIGN_GRANT, sponsoredForeign, { ...OWNED_GRANT, sponsored: true }];
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_reports_caps_unavailable_past_the_window_ceiling_without_scanning', async () => {
    const grants = new GrantChain();
    grants.finalised = {
      ...HEAD,
      number: toHex(FLOOR + GRANT_LOG_WINDOW_CEILING * GRANT_LOG_WINDOW),
    };
    grants.issued = [OWNED_GRANT];
    await expectRetiredGrantCaps(grants);

    const atCeiling = new GrantChain();
    atCeiling.xwebFee = chain.xwebFee;
    atCeiling.finalised = {
      ...HEAD,
      number: toHex(FLOOR + GRANT_LOG_WINDOW_CEILING * GRANT_LOG_WINDOW - 1n),
    };
    atCeiling.issued = [OWNED_GRANT];
    await expectRetiredGrantCaps(atCeiling);
  });

  it('web_data_view_reports_caps_unavailable_on_a_chain_without_a_configured_floor', async () => {
    const grants = new GrantChain();
    grants.capsChainId = 229;
    grants.issued = [{ ...OWNED_GRANT, block: 1n }];
    grants.finalised = { ...HEAD, number: toHex(40_000_000n) };
    await expectRetiredGrantCaps(grants);
  });

  it('web_data_view_reads_grants_from_holder_records_when_the_endpoint_serves_them', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    const second: IssuedGrant = {
      ...OWNED_GRANT,
      grantId: `0x${'a3'.repeat(32)}`,
      expiration: 1_850_000_000n,
    };
    grants.issued = [FOREIGN_GRANT, OWNED_GRANT, second];
    const holder = grants.address.toLowerCase();
    const record = (grant: IssuedGrant) => ({
      kind: 'grant',
      holder,
      contract: 'ledger',
      record_id: grant.grantId.slice(2),
      state: 'active',
    });
    const cursor = '00000000000000070000000000000000';
    grants.holderRecords = (params) =>
      params[2] === null
        ? {
            items: [record(OWNED_GRANT), record(FOREIGN_GRANT)],
            next_cursor: cursor,
            indexedHeight: Number(FLOOR + 42n),
            chainHeight: Number(FLOOR + 50n),
          }
        : {
            items: [record(second)],
            next_cursor: null,
            indexedHeight: Number(FLOOR + 45n),
            chainHeight: Number(FLOOR + 51n),
          };
    mounted = await mountSurface(grants, <GrantSurface />);
    const { container } = mounted;
    await expectRetiredGrantCaps(grants, container);
  });

  it('web_data_view_probes_holder_records_once_and_keeps_the_chain_scan_when_not_served', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.issued = [OWNED_GRANT];
    const first = await expectRetiredGrantCaps(grants);
    const again = await expectRetiredGrantCaps(grants);
    expect(again).toEqual(first);
    expect(grants.blockQueries).toEqual(['finalized', 'finalized']);
    expect(grants.callBlocks).toEqual([HEAD.number, HEAD.number, HEAD.number, HEAD.number]);
  });

  it('web_data_view_reports_caps_unavailable_when_served_holder_records_fail_and_never_falls_back_to_the_scan', async () => {
    const grants = new GrantChain();
    grants.xwebFee = chain.xwebFee;
    grants.issued = [OWNED_GRANT];
    let fail = false;
    grants.holderRecords = () => {
      if (fail) throw Object.assign(new Error('History unavailable'), { code: -32001 });
      return { items: [], next_cursor: null, indexedHeight: 40, chainHeight: 42 };
    };
    const initial = await expectRetiredGrantCaps(grants);
    grants.holderRecords = () => {
      if (fail) throw Object.assign(new Error('History unavailable'), { code: -32001 });
      return {
        items: [{ kind: 'grant', holder: grants.address, record_id: GRANT_ID }],
        next_cursor: null,
        indexedHeight: Number(FLOOR + 42n),
        chainHeight: Number(FLOOR + 42n),
      };
    };
    expect(await expectRetiredGrantCaps(grants)).toEqual(initial);
    fail = true;
    expect(await expectRetiredGrantCaps(grants)).toEqual(initial);
    grants.holderRecords = null;
    expect(await expectRetiredGrantCaps(grants)).toEqual(initial);
  });
});

function web_data_view_refuses_nondecimal_callback_gas_before_building() {
  const module = webData(chain);
  const expected = module.search('network', 200000n, 1n);
  expect(buildWebDataRequest(module, 'search', 'network', '200000', '', 1n)).toEqual(expected);
  for (const gas of ['-1', '0x10', '1e3', '18446744073709551616'])
    expect(() => buildWebDataRequest(module, 'search', 'network', gas, '', 1n)).toThrow();
}
test(
  web_data_view_refuses_nondecimal_callback_gas_before_building.name,
  web_data_view_refuses_nondecimal_callback_gas_before_building
);

function payer_grant_issued_decoder_refuses_data_on_the_indexed_only_event() {
  const log = {
    address: LEDGER_PRECOMPILE,
    topics: [PAYER_GRANT_ISSUED, `0x${GRANT_ID}`],
    data: '0x',
    transactionHash: issuingTx(OWNED_GRANT),
  };
  expect(decodePayerGrantIssued(log)).toEqual([`0x${GRANT_ID}`, issuingTx(OWNED_GRANT)]);
  expect(() => decodePayerGrantIssued({ ...log, data: `0x${'00'.repeat(32)}` })).toThrow(
    'invalid grant log'
  );
  expect(() => decodePayerGrantIssued({ ...log, topics: [...log.topics, ACCOUNT_ID] })).toThrow(
    'invalid grant log'
  );
}

test(
  payer_grant_issued_decoder_refuses_data_on_the_indexed_only_event.name,
  payer_grant_issued_decoder_refuses_data_on_the_indexed_only_event
);

interface LedgerProducerProof {
  grantLogs: { address: string; topics: string[]; data: string; transactionHash: string }[];
  grantId: string;
  paymentLogs: LedgerProducerProof['grantLogs'];
}

declare module 'vitest' {
  export interface ProvidedContext {
    ledgerProof: LedgerProducerProof;
  }
}

// The Go proof injects application logs; the full suite supplies the documented
// node-model recording through Vitest's context.
function proof_layerxledger_0x101a() {
  const proof = inject('ledgerProof');
  expect(
    proof,
    'ledger producer context (application proof or documented node receipt model)'
  ).toBeDefined();
  // Decision 2340: the real issuance call refused and emitted no grant.
  expect(proof.grantLogs).toHaveLength(0);
  expect(proof.grantLogs.map(decodePayerGrantIssued)).toEqual([]);
  expect(proof.paymentLogs).toHaveLength(2);
  // A served payment event cannot masquerade as grant discovery.
  for (const log of proof.paymentLogs)
    expect(() => decodePayerGrantIssued(log)).toThrow('invalid grant log');
}
test('proof_layerxledger_0x101a', proof_layerxledger_0x101a);
