// @vitest-environment jsdom
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it, test } from 'vitest';
import {
  PERPS_EVENTS,
  PERPS_PRECOMPILE,
  abiSelector,
  encodeAbiCall,
  perpsDepositCall,
  perpsOpenPositionCall,
} from '@sidiora/layerx-sdk';
import { exchange } from '@paxeer/wallet';
import { ethers } from 'ethers';
import { FIXTURE_KEY } from '@/wallet/test/gateway';
import { ExchangeView, USDL } from './ExchangeView';
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

class ExchangeChain extends SurfaceChain {
  override readonly wallet = new ethers.Wallet(FIXTURE_KEY);
  override readonly address = this.wallet.address.toLowerCase();
}

const perpsAbi = new ethers.Interface(
  JSON.parse(
    readFileSync(path.resolve(__dirname, '../../../../precompiles/perps/abi/abi.json'), 'utf8')
  ) as ethers.InterfaceAbi
);

let chain: SurfaceChain;
let mounted: Mounted | null = null;

beforeEach(() => {
  chain = new ExchangeChain();
});

afterEach(async () => {
  await mounted?.unmount();
  mounted = null;
});

function sendButton(container: HTMLElement): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>('[data-action="send"]');
  if (!button) throw new Error('no send button');
  return button;
}

const usdl = USDL.address;
const approve = (amount: bigint) =>
  encodeAbiCall('approve', ['address', 'uint256'], [PERPS_PRECOMPILE, amount]);

async function exchange_view_opens_a_perps_position_through_the_precompile_call_and_shows_the_decoded_event() {
  const vector = eventVectors('exchange').find((candidate) => candidate.event === 'OrderPlaced');
  if (!vector) throw new Error('no OrderPlaced vector');
  chain.logsFor = () => [vectorLog(PERPS_PRECOMPILE, PERPS_EVENTS, vector)];
  mounted = await mountSurface(chain, <ExchangeView sidRate={null} />);
  const { container } = mounted;

  await typeInto(container, 'marketId', '3');
  await typeInto(container, 'collateralToken', usdl);
  await typeInto(container, 'collateralAmount', '50.123456');
  await typeInto(container, 'leverage', '5');
  await until(() => !sendButton(container).disabled, 'send enabled');

  const order = {
    marketId: 3n,
    collateralToken: usdl,
    collateralAmount: 50_123_456n,
    leverage: 5n * 10n ** 18n,
    isLong: false,
  };
  const longExpected = perpsOpenPositionCall({ ...order, isLong: true });
  await click(sendButton(container), 'send long');
  await until(() => text(container, '[data-role="status"]') === 'confirmed', 'long receipt');
  expect(chain.sent[0]?.data).toBe(longExpected.data);
  expect(chain.sent[0]?.data).toBe(
    perpsAbi.encodeFunctionData('openPosition', [
      order.marketId,
      order.collateralToken,
      order.collateralAmount,
      order.leverage,
      true,
    ])
  );
  await click(radio(container, 'Side', 'Short'), 'short side');
  const expected = perpsOpenPositionCall(order);
  const openAbi = perpsAbi.getFunction('openPosition');
  if (!openAbi) throw new Error('no openPosition ABI');
  expect(openAbi.format('sighash')).toBe('openPosition(uint256,address,uint256,uint256,bool)');
  expect(text(container, '[data-role="preview"]')).toContain(openAbi.selector);
  expect(container.querySelector('input[name="size"]')).toBeNull();
  expect(container.querySelector('input[name="accountId"]')).toBeNull();

  await click(sendButton(container), 'send');
  await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

  expect(chain.sent).toEqual([
    { from: chain.address, to: PERPS_PRECOMPILE, data: longExpected.data, value: '0x0' },
    { from: chain.address, to: PERPS_PRECOMPILE, data: expected.data, value: '0x0' },
  ]);
  expect(chain.sent[1]?.data.slice(0, 10)).toBe(openAbi.selector);
  expect(chain.sent[1]?.data.slice(10).match(/.{64}/g)).toEqual([
    '3'.padStart(64, '0'),
    usdl.slice(2).toLowerCase().padStart(64, '0'),
    50_123_456n.toString(16).padStart(64, '0'),
    (5n * 10n ** 18n).toString(16).padStart(64, '0'),
    '0'.repeat(64),
  ]);
  expect(chain.sent[1]?.data).toBe(exchange(chain).openPosition(order).data);
  expect(
    chain.calls.some((call) => call.data.startsWith(abiSelector('allowance(address,address)')))
  ).toBe(false);
  expect(exchange(chain).cancelOrder(9n).data).toBe(
    perpsAbi.encodeFunctionData('cancelOrder', [9n])
  );

  const spec = PERPS_EVENTS.find((candidate) => candidate.name === 'OrderPlaced');
  if (!spec) throw new Error('no OrderPlaced spec');
  const event = container.querySelector('[data-event="OrderPlaced"]');
  expect(event).not.toBeNull();
  for (const [name, value] of Object.entries(vector.fields)) {
    expect(event?.querySelector(`[data-field="${name}"]`)?.textContent).toBe(
      expectedField(spec, name, value)
    );
  }
}

describe('ExchangeView', () => {
  test(
    exchange_view_opens_a_perps_position_through_the_precompile_call_and_shows_the_decoded_event.name,
    exchange_view_opens_a_perps_position_through_the_precompile_call_and_shows_the_decoded_event
  );

  it('exchange_view_cancels_an_order_through_the_precompile_call', async () => {
    mounted = await mountSurface(chain, <ExchangeView sidRate={null} />);
    const { container } = mounted;

    await click(radio(container, 'Exchange action', 'Cancel'), 'cancel action');
    await typeInto(container, 'id', '9');
    await until(() => !sendButton(container).disabled, 'send enabled');
    const cancelAbi = perpsAbi.getFunction('cancelOrder');
    if (!cancelAbi) throw new Error('no cancelOrder ABI');
    expect(cancelAbi.format('sighash')).toBe('cancelOrder(uint256)');
    await click(sendButton(container), 'send');
    await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: PERPS_PRECOMPILE,
        data: perpsAbi.encodeFunctionData(cancelAbi, [9n]),
        value: '0x0',
      },
    ]);
    expect(chain.sent[0]?.data).toBe(cancelAbi.selector + '9'.padStart(64, '0'));
  });

  it('exchange_view_deposits_usdl_without_native_value', async () => {
    mounted = await mountSurface(chain, <ExchangeView sidRate={null} />);
    const { container } = mounted;

    await click(radio(container, 'Exchange action', 'Deposit'), 'deposit action');
    await typeInto(container, 'amount', '1');
    await until(() => !sendButton(container).disabled, 'send enabled');
    await click(sendButton(container), 'send');
    await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

    expect(chain.sent).toEqual([
      { from: chain.address, to: usdl.toLowerCase(), data: approve(1_000_000n), value: '0x0' },
      {
        from: chain.address,
        to: PERPS_PRECOMPILE,
        data: perpsDepositCall(usdl, 1_000_000n).data,
        value: '0x0',
      },
    ]);
    expect(chain.sent[1]?.data.startsWith(abiSelector('deposit(address,uint256)'))).toBe(true);
    expect(container.querySelectorAll('[data-event]')).toHaveLength(0);
  });
});
