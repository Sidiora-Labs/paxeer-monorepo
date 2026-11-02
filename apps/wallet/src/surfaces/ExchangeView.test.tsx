// @vitest-environment jsdom
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
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
import { startAccountEndpoint, type AccountEndpoint } from '@/account/test/endpoint';
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

let endpoint: AccountEndpoint;
const previousRpcUrl = process.env.NEXT_PUBLIC_PAXEER_RPC_URL;

beforeAll(async () => {
  endpoint = await startAccountEndpoint();
  process.env.NEXT_PUBLIC_PAXEER_RPC_URL = endpoint.config('deposit').endpointUrl;
});

afterAll(async () => {
  await endpoint.close();
  if (previousRpcUrl === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  else process.env.NEXT_PUBLIC_PAXEER_RPC_URL = previousRpcUrl;
});

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

async function exchange_view_opens_a_perps_position_through_the_v2_call_and_shows_the_decoded_event() {
  const vector = eventVectors('exchange').find((candidate) => candidate.event === 'OrderPlaced');
  if (!vector) throw new Error('no OrderPlaced vector');
  chain.logsFor = () => [vectorLog(PERPS_PRECOMPILE, PERPS_EVENTS, vector)];
  mounted = await mountSurface(chain, <ExchangeView sidRate={null} />);
  const { container } = mounted;

  await typeInto(container, 'marketId', '3');
  await click(radio(container, 'Side', 'Short'), 'short side');
  await typeInto(container, 'size', '50');
  await typeInto(container, 'leverage', '5');
  await until(() => !sendButton(container).disabled, 'send enabled');

  const order = {
    marketId: `0x${'0'.repeat(63)}3`,
    accountId: `0x${'e1'.repeat(32)}`,
    side: 2 as const,
    size: 50n * 10n ** 18n,
    leverage: 5n * 10n ** 18n,
  };
  const expected = perpsOpenPositionCall(order);
  const signature = 'openPosition(bytes32,bytes32,uint8,uint256,uint256)';
  expect(text(container, '[data-role="preview"]')).toContain(abiSelector(signature));

  await click(sendButton(container), 'send');
  await until(() => text(container, '[data-role="status"]') === 'confirmed', 'receipt');

  expect(chain.sent).toEqual([
    { from: chain.address, to: PERPS_PRECOMPILE, data: expected.data, value: '0x0' },
  ]);
  expect(chain.sent[0]?.data.startsWith(abiSelector(signature))).toBe(true);
  expect(chain.sent[0]?.data).toBe(exchange(chain).openPosition(order).data);
  expect(
    chain.calls.some((call) => call.data.startsWith(abiSelector('allowance(address,address)')))
  ).toBe(false);
  expect(exchange(chain).cancelOrder(`0x${'0'.repeat(63)}9`, order.accountId).data).toBe(
    encodeAbiCall('cancelOrder', ['bytes32', 'bytes32'], [`0x${'0'.repeat(63)}9`, order.accountId])
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
  it(
    'exchange_view_opens_a_perps_position_through_the_v2_call_and_shows_the_decoded_event',
    exchange_view_opens_a_perps_position_through_the_v2_call_and_shows_the_decoded_event
  );

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
