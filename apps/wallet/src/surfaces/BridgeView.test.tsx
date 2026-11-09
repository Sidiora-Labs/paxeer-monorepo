// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, test } from 'vitest';
import {
  BRIDGE_EVENTS,
  LAYERX_BRIDGE_PRECOMPILE,
  abiSelector,
  bridgeOutCall,
} from '@sidiora/layerx-sdk/browser';
import { bridge } from '@paxeer/wallet';
import { formatUnits } from 'viem';
import { BridgeView } from './BridgeView';
import { proveBridgeRung } from './test/bridgeRung';
import {
  SurfaceChain,
  click,
  expectedField,
  mountSurface,
  text,
  typeInto,
  until,
  nodeReceiptVectors,
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
});

describe('BridgeView', () => {
  async function bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt() {
    const result = await proveBridgeRung((container) => {
      expect(text(container, '[data-role="rung"]')).toBe('receipted');
    });
    expect(result.rung).toBe('receipted');
  }

  test(
    bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt.name,
    bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt
  );

  it('bridge_view_bridges_out_through_the_module_and_shows_the_decoded_event', async () => {
    const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-layerxbridge-bridge-out');
    if (!entry) throw new Error('no producer BridgeOut receipt');
    const decoded = bridge(chain).decodeEvent(entry.receiptJson.logs[0]!);
    const fixture = {
      chain: String(decoded.fields.chain),
      asset: String(decoded.fields.asset),
      amount: String(decoded.fields.amount),
      recipient: String(decoded.fields.recipient),
    };
    const vector = {
      fields: Object.fromEntries(
        Object.entries(decoded.fields).map(([key, value]) => [key, String(value)])
      ),
    };
    mounted = await mountSurface(chain, <BridgeView sidRate={null} />);
    const { container } = mounted;

    await typeInto(container, 'chain', fixture.chain);
    await typeInto(container, 'asset', fixture.asset);
    await typeInto(container, 'decimals', '18');
    await typeInto(container, 'amount', formatUnits(BigInt(fixture.amount), 18));
    await typeInto(container, 'recipient', fixture.recipient);
    const send = () => container.querySelector<HTMLButtonElement>('[data-action="send"]');
    await until(() => send()?.disabled === false, 'send enabled');
    await click(send(), 'send');
    await until(
      () => container.querySelector('[data-role="sent"], [data-role="send-error"]') !== null,
      'receipt'
    );
    expect(text(container, '[data-role="send-error"]')).toBe('');
    expect(text(container, '[data-role="status"]')).toBe('confirmed');

    const expected = bridgeOutCall(
      BigInt(fixture.chain),
      fixture.asset,
      BigInt(fixture.amount),
      fixture.recipient
    );
    expect(chain.sent).toEqual([
      { from: chain.address, to: LAYERX_BRIDGE_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
    expect(
      chain.sent[0]?.data.startsWith(abiSelector('bridgeOut(uint64,address,uint256,address)'))
    ).toBe(true);
    expect(chain.sent[0]?.data).toBe(
      bridge(chain).bridgeOut(
        BigInt(fixture.chain),
        fixture.asset,
        BigInt(fixture.amount),
        fixture.recipient
      ).data
    );

    expect(text(container, '[data-role="hash"]')).toBe(entry.receiptJson.txHash);
    expect(text(container, '[data-role="rung"]')).toBe(entry.receiptJson.rung);
    expect(chain.methods).toContain('px_getReceiptKeys');
    expect(chain.methods).toContain('eth_getRawTransactionByHash');
    const spec = BRIDGE_EVENTS.find((candidate) => candidate.name === 'BridgeOut');
    if (!spec) throw new Error('no BridgeOut spec');
    const event = container.querySelector('[data-event="BridgeOut"]');
    expect(event).not.toBeNull();
    for (const [name, value] of Object.entries(vector.fields)) {
      expect(event?.querySelector(`[data-field="${name}"]`)?.textContent).toBe(
        expectedField(spec, name, value)
      );
    }
  });
});
