// @vitest-environment jsdom
import { createElement } from 'react';
import { expect, test } from 'vitest';
import {
  bridge,
  decodeSequencerReceipt,
  exchange,
  readReceiptKeyPins,
  readSequencerReceipt,
  sendModuleTransactionReceipted,
  type ModuleTransaction,
} from '@paxeer/wallet';
import { PERPS_PRECOMPILE, abiSelector, encodeAbiCall } from '@sidiora/layerx-sdk';
import { verifySequencerReceipt } from '@sidiora/layerx-sdk/browser';
import { bytesToHex, hexToBytes, keccak256, parseTransaction, toHex } from 'viem';
import { mount } from '@/account/test/render';
import { ExchangeView, USDL } from './ExchangeView';
import { SentResult } from './Surface';
import { WebDataView } from './WebDataView';
import { readGrantCaps, surfaceReceipt, useWebDataCaps, type SentTransaction } from './useSurface';
import {
  buildPerpsPosition,
  perpsCollateralApproval,
  perpsCollateralEndpoint,
  readPerpsCollateral,
} from './perpsCollateral';
import { proveBridgeRung } from './test/bridgeRung';
import { SurfaceChain, click, mountSurface, radio, text, typeInto, until } from './test/chain';
import { FLOOR, GrantChain, OWNED_GRANT } from './test/grants';
import { PerpsConfiguration } from './test/perpsConfiguration';
import { RecordedBridgeChain } from './test/recordedReceipts';

async function bridge_rung_consumer_renders_authenticated_receipted_result_and_events() {
  const result = await proveBridgeRung();
  expect(result.rung).toBe('receipted');
}

test(
  bridge_rung_consumer_renders_authenticated_receipted_result_and_events.name,
  bridge_rung_consumer_renders_authenticated_receipted_result_and_events
);

async function rung_not_rendered_distinguishes_authenticated_committed_outcomes() {
  const receiptedChain = new RecordedBridgeChain('V1-pax-transfer');
  expect(keccak256(receiptedChain.recording.rawTransaction)).toBe(
    receiptedChain.recording.receiptJson.txHash
  );
  expect(parseTransaction(receiptedChain.recording.rawTransaction).chainId).toBe(125);
  const receipted = await readSequencerReceipt(
    (method, params) => receiptedChain.request({ method, params }),
    receiptedChain.recording.receiptJson.txHash
  );
  if (receipted === null) throw new Error('Missing recorded receipted transfer');
  expect(receipted.rung).toBe('receipted');
  expect(receipted).not.toHaveProperty('blockNumber');
  expect(receipted).not.toHaveProperty('blockHash');
  const receiptedState = {
    sending: false,
    approving: false,
    sent: surfaceReceipt(receipted, bridge(receiptedChain)),
    error: null,
    send: async (tx: ModuleTransaction) => {
      await sendModuleTransactionReceipted(receiptedChain, receiptedChain.address, tx);
    },
  };
  const receiptedRendered = await mount(createElement(SentResult, { state: receiptedState }));
  try {
    expect(receiptedRendered.query('[data-role="send-error"]')).toBeNull();
    expect(receiptedRendered.text('[data-role="status"]')).toBe('confirmed');
    expect(receiptedRendered.text('[data-role="rung"]')).toBe('receipted');
    expect(receiptedRendered.text('[data-role="hash"]')).toBe(
      receiptedChain.recording.receiptJson.txHash
    );
    expect(receiptedChain.methods).toContain('px_getReceiptKeys');
    expect(receiptedChain.methods).toContain('eth_getRawTransactionByHash');
  } finally {
    await receiptedRendered.unmount();
  }
  const chain = new RecordedBridgeChain('committed-payExact');
  // This committed recording belongs to the conformance network (713714).
  // Authenticate its captured pair with the chain-independent SDK codecs before
  // supplying the result model to the shared renderer. The V1 case above also
  // exercises the wallet's chain-125 authenticator and surfaceReceipt adapter.
  const pins = await readReceiptKeyPins((method, params) => chain.request({ method, params }));
  const receipt = decodeSequencerReceipt(
    await chain.request({
      method: 'pax_sequencerReceipt',
      params: [chain.recording.receiptJson.txHash],
    }),
    chain.recording.receiptJson.txHash,
    pins
  );
  expect(receipt.rung).toBe('committed');
  if (receipt.rung !== 'committed' || !receipt.receipt || !receipt.logs)
    throw new Error('Missing recorded committed receipt');
  expect(
    await chain.request({
      method: 'eth_getRawTransactionByHash',
      params: [receipt.txHash],
    })
  ).toBe(chain.recording.rawTransaction);
  const authenticated = await verifySequencerReceipt(
    hexToBytes(receipt.receipt),
    hexToBytes(chain.recording.rawTransaction),
    pins.map((pin) => ({
      keyId: hexToBytes(pin.keyId),
      publicKey: hexToBytes(pin.publicKey),
      first_height: pin.firstHeight,
      last_height: pin.lastHeight,
    }))
  );
  expect(authenticated.level).toBe('sequencer-signed');
  expect(bytesToHex(authenticated.receipt.activityId)).toBe(receipt.txHash);
  expect(bytesToHex(authenticated.canonicalBytes)).toBe(receipt.receipt);
  expect(authenticated.height).toBe(BigInt(receipt.height!));
  expect(parseTransaction(chain.recording.rawTransaction).chainId).toBe(713714);
  expect(receipt.status).toBe('0x1');
  expect(receipt.blockHash).toBe(chain.recording.receiptJson.blockHash);
  const module = bridge(chain);
  const sent: SentTransaction = {
    hash: receipt.txHash,
    rung: receipt.rung,
    status: receipt.status === '0x1' ? 'confirmed' : 'reverted',
    events: receipt.logs
      .filter((log) => log.address.toLowerCase() === module.address.toLowerCase())
      .map((log) => module.decodeEvent(log)),
  };
  const state = {
    sending: false,
    approving: false,
    sent,
    error: null,
    send: async (tx: ModuleTransaction) => {
      await sendModuleTransactionReceipted(chain, chain.address, tx);
    },
  };
  const rendered = await mount(createElement(SentResult, { state }));
  try {
    expect(rendered.query('[data-role="send-error"]')).toBeNull();
    expect(rendered.text('[data-role="status"]')).toBe('confirmed');
    expect(rendered.text('[data-role="rung"]')).toBe('committed');
    expect(rendered.text('[data-role="hash"]')).toBe(chain.recording.receiptJson.txHash);
    expect(chain.methods).toContain('px_getReceiptKeys');
    expect(chain.methods).toContain('eth_getRawTransactionByHash');
  } finally {
    await rendered.unmount();
  }
}

test(
  rung_not_rendered_distinguishes_authenticated_committed_outcomes.name,
  rung_not_rendered_distinguishes_authenticated_committed_outcomes
);

const unsupported = `0x${'a1'.repeat(20)}`;
const order = { marketId: '3', collateralAmount: '1.234567', leverage: '5', isLong: true };
const allowanceSelector = abiSelector('allowance(address,address)');

async function unsupported_collateral_success_is_rejected_before_allowance_or_signing() {
  const configuration = new PerpsConfiguration();
  await configuration.open();
  const chain = new SurfaceChain();
  try {
    const tokens = await readPerpsCollateral(configuration.endpoint);
    const module = exchange(chain);
    expect(() =>
      buildPerpsPosition(module, tokens, { ...order, collateralToken: unsupported })
    ).toThrow('Unsupported perps collateral.');
    const unsupportedTx = module.openPosition({
      marketId: 3n,
      collateralToken: unsupported,
      collateralAmount: 1_234_567n,
      leverage: 5n * 10n ** 18n,
      isLong: true,
    });
    await expect(
      perpsCollateralApproval(
        module,
        chain,
        configuration.endpoint,
        chain.address,
        unsupportedTx,
        tokens[0]!
      )
    ).rejects.toThrow('Unsupported perps collateral.');
    expect(chain.calls.filter((call) => call.data.startsWith(allowanceSelector))).toEqual([]);
    expect(chain.sent).toEqual([]);
    expect(
      chain.methods.some((method) => method.includes('sendTransaction') || method.includes('sign'))
    ).toBe(false);
    const supported = buildPerpsPosition(module, tokens, {
      ...order,
      collateralToken: USDL.address,
    });
    expect(supported.pull).toBe(1_234_567n);
    expect(supported.tx.data).toBe(
      module.openPosition({
        marketId: 3n,
        collateralToken: USDL.address,
        collateralAmount: supported.pull,
        leverage: 5n * 10n ** 18n,
        isLong: true,
      }).data
    );
    const approval = await perpsCollateralApproval(
      module,
      chain,
      configuration.endpoint,
      chain.address,
      supported.tx,
      tokens[0]!
    );
    expect(approval).toEqual({
      to: USDL.address.toLowerCase(),
      data: encodeAbiCall('approve', ['address', 'uint256'], [PERPS_PRECOMPILE, 1_234_567n]),
      value: 0n,
    });
    expect(chain.calls.filter((call) => call.data.startsWith(allowanceSelector))).toEqual([
      {
        to: USDL.address.toLowerCase(),
        data: encodeAbiCall('allowance', ['address', 'address'], [chain.address, PERPS_PRECOMPILE]),
      },
    ]);
    expect(configuration.queries).toHaveLength(3);
  } finally {
    await configuration.close();
  }
}

test(
  unsupported_collateral_success_is_rejected_before_allowance_or_signing.name,
  unsupported_collateral_success_is_rejected_before_allowance_or_signing
);

async function collateral_selection_uses_served_asset_precision_and_refuses_stale_configuration() {
  const configuration = new PerpsConfiguration();
  await configuration.open();
  const chain = new SurfaceChain();
  const mounted = await mountSurface(
    chain,
    createElement(ExchangeView, { sidRate: null, collateralEndpoint: configuration.endpoint })
  );
  try {
    const { container } = mounted;
    const select = () =>
      container.querySelector<HTMLSelectElement>('select[name="collateralToken"]');
    await until(() => select()?.disabled === false, 'configured collateral control');
    expect(Array.from(select()!.options, (option) => option.value)).toEqual([
      USDL.address.toLowerCase(),
    ]);
    expect(select()!.value).toBe(USDL.address.toLowerCase());
    expect(container.textContent).toContain('Collateral amount (USDL)');
    await typeInto(container, 'marketId', '3');
    await typeInto(container, 'collateralAmount', '1.2345678');
    await typeInto(container, 'leverage', '5');
    const send = () => container.querySelector<HTMLButtonElement>('[data-action="send"]');
    expect(send()?.disabled).toBe(true);
    expect(text(container, '[data-role="build-error"]')).toContain('asset precision');
    await typeInto(container, 'collateralAmount', '1.234567');
    await until(() => send()?.disabled === false, 'supported position enabled');
    expect(text(container, '[data-role="approve-step"]')).toContain('exact USDL amount');
    // The served configuration changes after preview; the send path must validate it again.
    configuration.answer = {
      data: { collateralTokens: [{ tokenAddress: unsupported, decimals: 8, isActive: true }] },
    };
    await click(send(), 'send against changed configuration');
    await until(
      () => container.querySelector('[data-role="send-error"]') !== null,
      'unsupported collateral rejected'
    );
    expect(text(container, '[data-role="send-error"]')).toBe('Unsupported perps collateral.');
    expect(chain.sent).toEqual([]);
    expect(chain.calls.filter((call) => call.data.startsWith(allowanceSelector))).toEqual([]);
    await click(radio(container, 'Exchange action', 'Deposit'), 'deposit');
    expect(container.textContent).toContain('Amount (USDL)');
  } finally {
    await mounted.unmount();
    await configuration.close();
  }

  // Precision and units come from the served configuration, including assets unknown to the wallet catalogue.
  const alternative = new PerpsConfiguration();
  alternative.answer = {
    data: { collateralTokens: [{ tokenAddress: unsupported, decimals: 8, isActive: true }] },
  };
  await alternative.open();
  const other = await mountSurface(
    new SurfaceChain(),
    createElement(ExchangeView, { sidRate: null, collateralEndpoint: alternative.endpoint })
  );
  try {
    await until(
      () => other.container.querySelector<HTMLSelectElement>('select')?.value === unsupported,
      'alternative configured asset'
    );
    expect(other.container.textContent).toContain(`Collateral amount (${unsupported})`);
    const tokens = await readPerpsCollateral(alternative.endpoint);
    expect(
      buildPerpsPosition(exchange(chain), tokens, {
        ...order,
        collateralToken: unsupported,
        collateralAmount: '1.23456789',
      }).pull
    ).toBe(123_456_789n);
  } finally {
    await other.unmount();
    await alternative.close();
  }
}

test(
  collateral_selection_uses_served_asset_precision_and_refuses_stale_configuration.name,
  collateral_selection_uses_served_asset_precision_and_refuses_stale_configuration
);

async function collateral_selection_fails_closed_when_configuration_is_unavailable_or_invalid() {
  expect(perpsCollateralEndpoint(undefined)).toBeNull();
  expect(perpsCollateralEndpoint('https://perps.example')).toBe('https://perps.example/graphql');
  for (const origin of [
    'https://user:password@perps.example',
    'https://perps.example/path',
    'file:///',
  ])
    expect(() => perpsCollateralEndpoint(origin)).toThrow();
  const configuration = new PerpsConfiguration();
  await configuration.open();
  try {
    for (const answer of [
      { errors: [{ message: 'unavailable' }], data: { collateralTokens: [] } },
      { data: { collateralTokens: [] } },
      {
        data: { collateralTokens: [{ tokenAddress: USDL.address, decimals: 256, isActive: true }] },
      },
      {
        data: { collateralTokens: [{ tokenAddress: USDL.address, decimals: 6, isActive: false }] },
      },
      {
        data: {
          collateralTokens: [{ tokenAddress: `0x${'00'.repeat(20)}`, decimals: 6, isActive: true }],
        },
      },
    ]) {
      configuration.answer = answer;
      await expect(readPerpsCollateral(configuration.endpoint)).rejects.toThrow();
    }
    configuration.status = 503;
    await expect(readPerpsCollateral(configuration.endpoint)).rejects.toThrow('unavailable');
  } finally {
    await configuration.close();
  }
  const chain = new SurfaceChain();
  const mounted = await mountSurface(
    chain,
    createElement(ExchangeView, { sidRate: null, collateralEndpoint: null })
  );
  try {
    await typeInto(mounted.container, 'marketId', '3');
    await typeInto(mounted.container, 'collateralAmount', '1');
    await typeInto(mounted.container, 'leverage', '5');
    await until(
      () => text(mounted.container, '[data-role="collateral-status"]').includes('unavailable'),
      'configuration unavailable'
    );
    expect(
      mounted.container.querySelector<HTMLButtonElement>('[data-action="send"]')?.disabled
    ).toBe(true);
    expect(mounted.container.querySelector('[data-role="preview"]')).toBeNull();
    expect(chain.sent).toEqual([]);
    expect(chain.calls.filter((call) => call.data.startsWith(allowanceSelector))).toEqual([]);
  } finally {
    await mounted.unmount();
  }
}

test(
  collateral_selection_fails_closed_when_configuration_is_unavailable_or_invalid.name,
  collateral_selection_fails_closed_when_configuration_is_unavailable_or_invalid
);

function GrantSurface() {
  const { caps, refreshCaps } = useWebDataCaps();
  return createElement(WebDataView, { sidRate: null, caps, refreshCaps });
}

async function empty_lagging_index_test_stays_incomplete_until_the_issued_grant_is_discovered() {
  const chain = new GrantChain();
  chain.issued = [{ ...OWNED_GRANT, block: FLOOR + 41n }];
  let caughtUp = false;
  chain.holderRecords = () => ({
    items: caughtUp
      ? [{ kind: 'grant', holder: chain.address, record_id: OWNED_GRANT.grantId.slice(2) }]
      : [],
    next_cursor: null,
    indexedHeight: Number(FLOOR + (caughtUp ? 42n : 40n)),
    chainHeight: Number(FLOOR + 42n),
  });
  const mounted = await mountSurface(chain, createElement(GrantSurface));
  try {
    await until(
      () => mounted.container.querySelector('[data-caps-state="incomplete"]') !== null,
      'incomplete discovery'
    );
    expect(mounted.container.querySelector('[data-role="caps-empty"]')).toBeNull();
    expect(mounted.container.querySelector('[data-role="caps-observation"]')).toBeNull();
    expect(text(mounted.container, '[data-role="caps-discovery"]')).toContain(
      `required finalized height ${FLOOR + 42n}`
    );
    expect(mounted.container.textContent).toContain(
      'Grant discovery has not covered the finalized block.'
    );
    caughtUp = true;
    await click(
      Array.from(mounted.container.querySelectorAll('button')).find(
        (button) => button.textContent === 'Refresh caps'
      ) ?? null,
      'refresh caps'
    );
    await until(
      () => mounted.container.querySelector('[data-caps-state="ready"]') !== null,
      'grant after index catch-up'
    );
    expect(mounted.container.querySelectorAll('[data-grant]')).toHaveLength(1);
    expect(
      mounted.container.querySelector(`[data-grant="${OWNED_GRANT.grantId.slice(2)}"]`)
    ).not.toBeNull();
    expect(text(mounted.container, '[data-role="caps-observation"]')).toBe(
      `committed · block ${FLOOR + 42n}`
    );
    expect(chain.methods).not.toContain('eth_getLogs');
    expect(chain.grantCallBlocks.length).toBeGreaterThanOrEqual(5);
    expect(chain.grantCallBlocks.every((block) => block === toHex(FLOOR + 42n))).toBe(true);
  } finally {
    await mounted.unmount();
  }
}

test(
  empty_lagging_index_test_stays_incomplete_until_the_issued_grant_is_discovered.name,
  empty_lagging_index_test_stays_incomplete_until_the_issued_grant_is_discovered
);

async function incomplete_grant_discovery_requires_coverage_on_every_page_and_refuses_unknown_heights() {
  const chain = new GrantChain();
  chain.issued = [OWNED_GRANT];
  const cursor = '00000000000000070000000000000000';
  for (const indexedHeight of [null, Number(FLOOR + 40n)]) {
    chain.holderRecords = (params) => ({
      items:
        params[2] === null
          ? [{ kind: 'grant', holder: chain.address, record_id: OWNED_GRANT.grantId.slice(2) }]
          : [],
      next_cursor: params[2] === null ? cursor : null,
      indexedHeight: params[2] === null ? Number(FLOOR + 42n) : indexedHeight,
      chainHeight: Number(FLOOR + 50n),
    });
    expect(await readGrantCaps(chain, chain.address)).toEqual({
      state: 'incomplete',
      reason: 'Grant discovery has not covered the finalized block.',
      indexedHeight: indexedHeight?.toString() ?? null,
      finalizedHeight: (FLOOR + 42n).toString(),
    });
  }
  expect(chain.methods).not.toContain('eth_getLogs');
  expect(
    chain.calls.some((call) => call.data.startsWith(abiSelector('getPayerGrant(bytes32)')))
  ).toBe(false);
  chain.holderRecords = () => ({ items: [], next_cursor: null, chainHeight: Number(FLOOR + 42n) });
  await expect(readGrantCaps(chain, chain.address)).rejects.toMatchObject({
    code: 'invalid_answer',
    field: 'px_listHolderRecords.indexedHeight',
  });
}

test(
  incomplete_grant_discovery_requires_coverage_on_every_page_and_refuses_unknown_heights.name,
  incomplete_grant_discovery_requires_coverage_on_every_page_and_refuses_unknown_heights
);

async function budgets_not_discovered_is_exposed_separately_from_verified_grant_absence() {
  const chain = new GrantChain();
  chain.holderRecords = () => ({
    items: [],
    next_cursor: null,
    indexedHeight: Number(FLOOR + 42n),
    chainHeight: Number(FLOOR + 42n),
  });
  const caps = await readGrantCaps(chain, chain.address);
  expect(caps.state).toBe('empty');
  if (caps.state !== 'empty') throw new Error('Expected verified grant absence');
  expect(caps.budgetAvailability).toBe('unavailable');
  expect(caps.grants).toEqual([]);
  const mounted = await mountSurface(chain, createElement(GrantSurface));
  try {
    await until(
      () => mounted.container.querySelector('[data-caps-state="empty"]') !== null,
      'verified grant absence'
    );
    expect(text(mounted.container, '[data-role="budgets-unavailable"]')).toBe(
      'Budget discovery is unavailable. Budget balances have not been read.'
    );
    expect(text(mounted.container, '[data-role="caps-empty"]')).toBe(
      'No grants at this verified observation.'
    );
    expect(mounted.container.textContent).not.toContain('No budgets or grants');
    expect(mounted.container.querySelector('[data-budget]')).toBeNull();
    expect(chain.holderQueries.every((params) => params[1] === 'grant')).toBe(true);
  } finally {
    await mounted.unmount();
  }
}

test(
  budgets_not_discovered_is_exposed_separately_from_verified_grant_absence.name,
  budgets_not_discovered_is_exposed_separately_from_verified_grant_absence
);
