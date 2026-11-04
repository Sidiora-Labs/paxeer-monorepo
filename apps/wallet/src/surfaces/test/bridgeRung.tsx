import { expect } from 'vitest';
import { bridge } from '@paxeer/wallet';
import { LAYERX_BRIDGE_PRECOMPILE } from '@sidiora/layerx-sdk';
import { formatUnits, parseTransaction } from 'viem';
import { BridgeView } from '../BridgeView';
import { RecordedBridgeChain } from './recordedReceipts';
import { click, mountSurface, text, typeInto, until } from './chain';

export async function proveBridgeRung(observe?: (container: HTMLDivElement) => void) {
  const chain = new RecordedBridgeChain();
  const mounted = await mountSurface(chain, <BridgeView sidRate={null} />);
  const { container } = mounted;
  try {
    const log = chain.recording.receiptJson.logs?.[0];
    if (!log) throw new Error('No recorded bridge log');
    const decoded = bridge(chain).decodeEvent(log);
    const expected = bridge(chain).bridgeOut(
      BigInt(String(decoded.fields.chain)),
      String(decoded.fields.asset),
      BigInt(String(decoded.fields.amount)),
      String(decoded.fields.recipient)
    );
    expect(expected.data).toBe(parseTransaction(chain.recording.rawTransaction).data);
    await typeInto(container, 'chain', String(decoded.fields.chain));
    await typeInto(container, 'asset', String(decoded.fields.asset));
    await typeInto(container, 'decimals', '18');
    await typeInto(container, 'amount', formatUnits(BigInt(String(decoded.fields.amount)), 18));
    await typeInto(container, 'recipient', String(decoded.fields.recipient));
    const send = () => container.querySelector<HTMLButtonElement>('[data-action="send"]');
    await until(() => send()?.disabled === false, 'send enabled');
    await click(send(), 'send');
    await until(
      () => container.querySelector('[data-role="sent"], [data-role="send-error"]') !== null,
      'receipted bridge result'
    );
    expect(
      chain.methods.filter((method) => method === 'paxeer_sendTransactionReceipted')
    ).toHaveLength(1);
    expect(chain.methods).not.toContain('eth_sendTransaction');
    expect(chain.methods).not.toContain('eth_getTransactionReceipt');
    expect(chain.methods).toContain('px_getReceiptKeys');
    expect(chain.methods).toContain('eth_getRawTransactionByHash');
    expect(container.querySelector('[data-role="send-error"]')).toBeNull();
    expect(text(container, '[data-role="status"]')).toBe('confirmed');
    expect(text(container, '[data-role="rung"]')).toBe('receipted');
    expect(chain.sent).toEqual([
      { from: chain.address, to: LAYERX_BRIDGE_PRECOMPILE, data: expected.data, value: '0x0' },
    ]);
    expect(text(container, '[data-role="hash"]')).toBe(chain.recording.receiptJson.txHash);
    expect(text(container, '[data-role="hash"]')).toMatch(/^0x[0-9a-f]{64}$/);
    const event = container.querySelector('[data-event="BridgeOut"]');
    expect(event).not.toBeNull();
    for (const [name, value] of Object.entries(decoded.fields))
      expect(event?.querySelector(`[data-field="${name}"]`)?.textContent).toBe(String(value));
    observe?.(container);
    return {
      hash: text(container, '[data-role="hash"]'),
      rung: text(container, '[data-role="rung"]'),
    };
  } finally {
    await mounted.unmount();
  }
}
