// @vitest-environment jsdom
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, test, vi } from 'vitest';
import {
  LAUNCHPAD_EVENTS,
  LAUNCHPAD_PRECOMPILE,
  abiSelector,
  launchpadBuyCall,
  launchpadSellCall,
  launchpadApproveCall,
  launchpadCreateMarketCall,
} from '@sidiora/layerx-sdk';
import { launchpad } from '@paxeer/wallet';
import * as walletModules from '@paxeer/wallet';
import { LaunchpadView } from './LaunchpadView';
import * as surface from './Surface';
import { SPONSORED_APPROVE_UNSUPPORTED } from './useSurface';
import {
  SurfaceChain,
  callFixtures,
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

let chain: SurfaceChain;
let mounted: Mounted | null = null;

beforeEach(() => {
  chain = new SurfaceChain();
});

afterEach(async () => {
  await mounted?.unmount();
  mounted = null;
  vi.restoreAllMocks();
});

const send = (container: HTMLElement) =>
  container.querySelector<HTMLButtonElement>('[data-action="send"]');

describe('LaunchpadView', () => {
  async function launchpad_view_refuses_a_deadline_that_is_not_a_positive_whole_number_of_minutes() {
    const fixture = callFixtures().launchpad.buy as { pool: string; minOut: string };
    const now = () => 1_800_000_000_123;
    const factory = vi.spyOn(walletModules, 'launchpad');
    try {
      mounted = await mountSurface(chain, <LaunchpadView sidRate={null} now={now} />);
      const { container } = mounted;
      const result = factory.mock.results.find((entry) => entry.type === 'return');
      if (!result || result.type !== 'return') throw new Error('no launchpad module');
      const buy = vi.spyOn(result.value, 'buy');
      const sell = vi.spyOn(result.value, 'sell');

      await typeInto(container, 'deadlineMinutes', '');
      await typeInto(container, 'pool', fixture.pool);
      await typeInto(container, 'amountIn', '1');
      await typeInto(container, 'minOut', fixture.minOut);
      for (const minutes of ['', '0', '-5', '1.5']) {
        await typeInto(container, 'deadlineMinutes', minutes);
        expect(text(container, '[data-role="build-error"]')).toBe(
          'the deadline must be a positive whole number of minutes'
        );
        expect(container.querySelector('[data-role="preview"]')).toBeNull();
        expect(send(container)?.disabled).toBe(true);
        await click(send(container), 'disabled send');
        expect(buy).not.toHaveBeenCalled();
        expect(sell).not.toHaveBeenCalled();
        expect(chain.sent).toHaveLength(0);
      }

      await typeInto(container, 'deadlineMinutes', '10');
      const order = {
        pool: fixture.pool,
        amountIn: 1_000_000n,
        minOut: BigInt(fixture.minOut),
        deadline: BigInt(Math.floor(now() / 1000) + 600),
      };
      expect(buy).toHaveBeenCalledExactlyOnceWith(order);
      expect(buy.mock.results[0]?.value).toEqual(launchpadBuyCall(order));
      expect(container.querySelector('[data-role="build-error"]')).toBeNull();
      expect(container.querySelector('[data-role="preview"]')).not.toBeNull();
      await until(() => send(container)?.disabled === false, 'send enabled');

      await typeInto(container, 'deadlineMinutes', '');
      await click(radio(container, 'Launchpad action', 'Sell'), 'sell action');
      for (const minutes of ['', '0', '-5', '1.5']) {
        await typeInto(container, 'deadlineMinutes', minutes);
        expect(text(container, '[data-role="build-error"]')).toBe(
          'the deadline must be a positive whole number of minutes'
        );
        expect(container.querySelector('[data-role="preview"]')).toBeNull();
        expect(send(container)?.disabled).toBe(true);
        await click(send(container), 'disabled send');
        expect(buy).toHaveBeenCalledExactlyOnceWith(order);
        expect(sell).not.toHaveBeenCalled();
        expect(chain.sent).toHaveLength(0);
      }

      await typeInto(container, 'deadlineMinutes', '10');
      expect(sell).toHaveBeenCalledExactlyOnceWith({
        pool: fixture.pool,
        amountIn: 1n,
        minOut: BigInt(fixture.minOut) * 10n ** 6n,
        deadline: order.deadline,
      });
      expect(container.querySelector('[data-role="build-error"]')).toBeNull();
      expect(container.querySelector('[data-role="preview"]')).not.toBeNull();
      await until(() => send(container)?.disabled === false, 'sell send enabled');
    } finally {
      vi.restoreAllMocks();
    }
  }

  test(launchpad_view_refuses_a_deadline_that_is_not_a_positive_whole_number_of_minutes);

  async function launchpad_view_refuses_the_approve_under_the_sponsored_fee_and_sends_nothing() {
    const fixture = callFixtures().launchpad.buy as { pool: string; minOut: string };
    const sender = vi.spyOn(surface, 'SendButton');
    mounted = await mountSurface(chain, <LaunchpadView sidRate="3114000" />);
    const { container } = mounted;
    await typeInto(container, 'pool', fixture.pool);
    await typeInto(container, 'amountIn', '1');
    await typeInto(container, 'minOut', fixture.minOut);
    await until(() => send(container)?.disabled === false, 'buy ready');
    await click(container.querySelector('[data-fee-choice="sid_sponsored"]'), 'SID sponsored');
    expect(
      container.querySelector('[data-fee-choice="sid_sponsored"]')?.getAttribute('aria-checked')
    ).toBe('true');
    expect(chain.allowances.size).toBe(0);

    const props = sender.mock.lastCall?.[0];
    if (!props?.transaction) throw new Error('no buy transaction');
    await act(async () => {
      await props.state.send(props.transaction!);
    });
    await until(
      () => container.querySelector('[data-role="send-error"]') !== null,
      'approve refusal'
    );
    expect(text(container, '[data-role="send-error"]')).toBe(SPONSORED_APPROVE_UNSUPPORTED);
    expect(chain.calls).toContainEqual({
      to: chain.launchpadUsdl,
      data: `${abiSelector('allowance(address,address)')}${chain.address.slice(2).padStart(64, '0')}${LAUNCHPAD_PRECOMPILE.slice(2).padStart(64, '0')}`,
    });
    expect(chain.sent).toEqual([]);
    expect(chain.methods).not.toContain('eth_sendTransaction');
    expect(chain.methods).not.toContain('paxeer_sendTransactionReceipted');
    expect(chain.methods).not.toContain('eth_sign');
    expect(container.querySelector('[data-role="sent"]')).toBeNull();
  }

  test(launchpad_view_refuses_the_approve_under_the_sponsored_fee_and_sends_nothing);

  async function launchpad_view_stops_the_write_when_the_approve_receipt_fails() {
    class RevertedApprovalChain extends SurfaceChain {
      override async request(args: Parameters<SurfaceChain['request']>[0]): Promise<unknown> {
        const answer = await super.request(args);
        if (args.method !== 'paxeer_sendTransactionReceipted') return answer;
        const call = this.sent.at(-1);
        expect(call?.to).toBe(this.launchpadUsdl);
        expect(call?.data).toBe(launchpadApproveCall(this.launchpadUsdl, 1_000_000n).data);
        if (typeof answer !== 'object' || answer === null) throw new Error('no approve receipt');
        return { ...answer, status: '0x0' };
      }
    }

    chain = new RevertedApprovalChain();
    const fixture = callFixtures().launchpad.buy as { pool: string; minOut: string };
    mounted = await mountSurface(chain, <LaunchpadView sidRate={null} />);
    const { container } = mounted;
    await typeInto(container, 'pool', fixture.pool);
    await typeInto(container, 'amountIn', '1');
    await typeInto(container, 'minOut', fixture.minOut);
    await until(() => send(container)?.disabled === false, 'buy ready');
    await click(send(container), 'buy');
    await until(
      () => container.querySelector('[data-role="send-error"]') !== null,
      'approve failed'
    );
    expect(text(container, '[data-role="send-error"]')).toMatch(
      /^the approve 0x[0-9a-f]{64} did not confirm$/
    );
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: chain.launchpadUsdl,
        data: launchpadApproveCall(chain.launchpadUsdl, 1_000_000n).data,
        value: '0x0',
      },
    ]);
    expect(
      chain.methods.filter((method) => method === 'paxeer_sendTransactionReceipted')
    ).toHaveLength(1);
    expect(chain.methods).not.toContain('eth_sendTransaction');
    expect(container.querySelector('[data-role="sent"]')).toBeNull();
    expect(container.querySelector('[data-role="approve-step"]')?.textContent).not.toContain(
      'Approving the exact'
    );
  }

  test(launchpad_view_stops_the_write_when_the_approve_receipt_fails);

  async function launchpad_view_buys_through_the_module_and_shows_the_decoded_swap() {
    const fixture = callFixtures().launchpad.buy as {
      pool: string;
      minOut: string;
      deadline: string;
    };
    const vector = eventVectors('launchpad').find((candidate) => candidate.event === 'Swap');
    if (!vector) throw new Error('no Swap vector');
    chain.logsFor = () => [vectorLog(LAUNCHPAD_PRECOMPILE, LAUNCHPAD_EVENTS, vector)];
    const now = () => (Number(fixture.deadline) - 600) * 1000;
    mounted = await mountSurface(chain, <LaunchpadView sidRate={null} now={now} />);
    const { container } = mounted;

    expect(container.querySelector('input[name="amountIn"]')?.closest('label')?.textContent).toBe(
      'Pay (USDL)'
    );
    await typeInto(container, 'pool', fixture.pool);
    await typeInto(container, 'amountIn', '1');
    await typeInto(container, 'minOut', fixture.minOut);
    await until(() => send(container)?.disabled === false, 'send enabled');
    await click(send(container), 'send');
    await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

    const order = {
      pool: fixture.pool,
      amountIn: 1_000_000n,
      minOut: BigInt(fixture.minOut),
      deadline: BigInt(fixture.deadline),
    };
    const expected = launchpadBuyCall(order);
    const approve = launchpadApproveCall(chain.launchpadUsdl, order.amountIn);
    expect(chain.sent).toEqual([
      { from: chain.address, to: chain.launchpadUsdl, data: approve.data, value: '0x0' },
      { from: chain.address, to: LAUNCHPAD_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
    expect(
      chain.sent[1]?.data.startsWith(abiSelector('buy(address,uint256,uint256,uint256)'))
    ).toBe(true);
    expect(chain.sent[1]?.data).toBe(launchpad(chain).buy(order).data);

    const spec = LAUNCHPAD_EVENTS.find((candidate) => candidate.name === 'Swap');
    if (!spec) throw new Error('no Swap spec');
    const event = container.querySelector('[data-event="Swap"]');
    expect(event).not.toBeNull();
    for (const [name, value] of Object.entries(vector.fields)) {
      expect(event?.querySelector(`[data-field="${name}"]`)?.textContent).toBe(
        expectedField(spec, name, value)
      );
    }
  }

  test(launchpad_view_buys_through_the_module_and_shows_the_decoded_swap);

  async function launchpad_view_sells_with_a_usdl_minimum_at_six_decimals() {
    const fixture = callFixtures().launchpad.sell as {
      pool: string;
      amountIn: string;
      deadline: string;
    };
    const now = () => (Number(fixture.deadline) - 600) * 1000;
    mounted = await mountSurface(chain, <LaunchpadView sidRate={null} now={now} />);
    const { container } = mounted;

    await click(radio(container, 'Launchpad action', 'Sell'), 'sell action');
    expect(container.querySelector('input[name="minOut"]')?.closest('label')?.textContent).toBe(
      'Receive at least (USDL)'
    );
    await typeInto(container, 'pool', fixture.pool);
    await typeInto(container, 'amountIn', fixture.amountIn);
    await typeInto(container, 'minOut', '2.5');
    await until(() => send(container)?.disabled === false, 'sell send enabled');
    await click(send(container), 'send');
    await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

    const order = {
      pool: fixture.pool,
      amountIn: BigInt(fixture.amountIn),
      minOut: 2_500_000n,
      deadline: BigInt(fixture.deadline),
    };
    const expected = launchpadSellCall(order);
    const approve = launchpadApproveCall(chain.launchpadToken, order.amountIn);
    expect(chain.sent).toEqual([
      { from: chain.address, to: chain.launchpadToken, data: approve.data, value: '0x0' },
      { from: chain.address, to: LAUNCHPAD_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
    expect(
      chain.sent[1]?.data.startsWith(abiSelector('sell(address,uint256,uint256,uint256)'))
    ).toBe(true);
    expect(chain.sent[1]?.data).toBe(launchpad(chain).sell(order).data);
  }

  test(launchpad_view_sells_with_a_usdl_minimum_at_six_decimals);

  async function launchpad_view_shows_the_created_market_from_the_receipt_logs() {
    const fixture = callFixtures().launchpad.createMarket as {
      name: string;
      symbol: string;
      feeStrategy: number;
    };
    const vector = eventVectors('launchpad').find(
      (candidate) => candidate.event === 'MarketCreated'
    );
    const spec = LAUNCHPAD_EVENTS.find((candidate) => candidate.name === 'MarketCreated');
    if (!vector || !spec) throw new Error('no MarketCreated fixture');
    chain.logsFor = () => [vectorLog(LAUNCHPAD_PRECOMPILE, LAUNCHPAD_EVENTS, vector)];
    mounted = await mountSurface(chain, <LaunchpadView sidRate={null} />);
    const { container } = mounted;

    expect(container.querySelector('[data-role="created-market"]')).toBeNull();
    await click(radio(container, 'Launchpad action', 'Create'), 'create action');
    await typeInto(container, 'name', fixture.name);
    await typeInto(container, 'symbol', fixture.symbol);
    await typeInto(container, 'feeStrategy', String(fixture.feeStrategy));
    await until(() => send(container)?.disabled === false, 'send enabled');
    expect(container.querySelector('[data-role="created-market"]')).toBeNull();
    await click(send(container), 'send');
    await until(
      () => container.querySelector('[data-role="sent"], [data-role="send-error"]') !== null,
      'receipted market result'
    );

    expect(container.querySelector('[data-role="send-error"]')).toBeNull();
    expect(text(container, '[data-role="status"]')).toBe('confirmed');
    expect(text(container, '[data-role="created-market"]')).toBe(
      expectedField(spec, 'pool', vector.fields.pool)
    );
    expect(
      chain.methods.filter((method) => method === 'paxeer_sendTransactionReceipted')
    ).toHaveLength(1);
    expect(chain.methods).not.toContain('eth_sendTransaction');
    expect(chain.methods).not.toContain('eth_getTransactionReceipt');
    const expected = launchpadCreateMarketCall(fixture.name, fixture.symbol, fixture.feeStrategy);
    expect(chain.sent).toEqual([
      { from: chain.address, to: LAUNCHPAD_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
  }

  test(launchpad_view_shows_the_created_market_from_the_receipt_logs);

  it('launchpad_view_creates_a_market_through_the_module', async () => {
    const fixture = callFixtures().launchpad.createMarket as {
      name: string;
      symbol: string;
      feeStrategy: number;
    };
    const vector = eventVectors('launchpad').find(
      (candidate) => candidate.event === 'MarketCreated'
    );
    if (!vector) throw new Error('no MarketCreated vector');
    chain.logsFor = () => [vectorLog(LAUNCHPAD_PRECOMPILE, LAUNCHPAD_EVENTS, vector)];
    chain.launchpadCreationFee = 5_000_000n;
    mounted = await mountSurface(chain, <LaunchpadView sidRate={null} />);
    const { container } = mounted;

    await click(radio(container, 'Launchpad action', 'Create'), 'create action');
    await typeInto(container, 'name', fixture.name);
    await typeInto(container, 'symbol', fixture.symbol);
    await typeInto(container, 'feeStrategy', String(fixture.feeStrategy));
    await until(() => send(container)?.disabled === false, 'send enabled');
    await click(send(container), 'send');
    await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

    const expected = launchpadCreateMarketCall(fixture.name, fixture.symbol, fixture.feeStrategy);
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: chain.launchpadUsdl,
        data: launchpadApproveCall(chain.launchpadUsdl, chain.launchpadCreationFee).data,
        value: '0x0',
      },
      { from: chain.address, to: LAUNCHPAD_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
    expect(
      chain.sent[1]?.data.startsWith(abiSelector('createMarket(string,string,uint8,address)'))
    ).toBe(true);
    const spec = LAUNCHPAD_EVENTS.find((candidate) => candidate.name === 'MarketCreated');
    if (!spec) throw new Error('no MarketCreated spec');
    expect(
      container.querySelector('[data-event="MarketCreated"] [data-field="pool"]')?.textContent
    ).toBe(expectedField(spec, 'pool', vector.fields.pool));
  });
});
