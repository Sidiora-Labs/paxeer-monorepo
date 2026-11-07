// @vitest-environment jsdom
import { createElement } from 'react';
import { expect, test } from 'vitest';
import {
  bridge,
  exchange,
  readSequencerReceipt,
  requireAuthenticatedReceipt,
  sendModuleTransactionReceipted,
  type ModuleTransaction,
} from '@paxeer/wallet';
import {
  PERPS_PRECOMPILE,
  XWEB_PRECOMPILE,
  abiSelector,
  encodeAbiCall,
} from '@sidiora/layerx-sdk/browser';
import { keccak256, parseTransaction } from 'viem';
import { mount } from '@/account/test/render';
import { ExchangeView, USDL } from './ExchangeView';
import { SentResult } from './Surface';
import { WebDataView } from './WebDataView';
import { readGrantCaps, surfaceReceipt, useWebDataCaps } from './useSurface';
import {
  buildPerpsPosition,
  perpsCollateralApproval,
  perpsCollateralEndpoint,
  readPerpsCollateral,
} from './perpsCollateral';
import { proveBridgeRung } from './test/bridgeRung';
import { SurfaceChain, click, mountSurface, radio, text, typeInto, until } from './test/chain';
import { DID, GrantChain, HEAD } from './test/grants';
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
  const receipt = await readSequencerReceipt(
    (method, params) => chain.request({ method, params }),
    chain.recording.receiptJson.txHash
  );
  if (receipt === null) throw new Error('Missing recorded committed receipt');
  requireAuthenticatedReceipt(receipt);
  expect(receipt.rung).toBe('committed');
  if (receipt.rung !== 'committed' || !receipt.receipt || !receipt.logs)
    throw new Error('Missing recorded committed receipt');
  expect(
    await chain.request({
      method: 'eth_getRawTransactionByHash',
      params: [receipt.txHash],
    })
  ).toBe(chain.recording.rawTransaction);
  expect(keccak256(chain.recording.rawTransaction)).toBe(receipt.txHash);
  expect(receipt.authentication.level).toBe('sequencer-signed');
  expect(receipt.authentication.rawTransaction).toBe(chain.recording.rawTransaction);
  expect(receipt.authentication.keyId).toBe(chain.recording.pinnedKey.keyId);
  expect(receipt.receipt).toBe(chain.recording.receiptJson.receipt);
  expect(receipt.height).toBe(chain.recording.receiptJson.height);
  expect(receipt.authentication.height).toBe(BigInt(receipt.height!));
  expect(parseTransaction(chain.recording.rawTransaction).chainId).toBe(125);
  expect(parseTransaction(chain.recording.rawTransaction).type).toBe('eip1559');
  expect(receipt.status).toBe('0x1');
  expect(receipt.blockHash).toBe(chain.recording.receiptJson.blockHash);
  const sent = surfaceReceipt(receipt, bridge(chain));
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
  chain.xwebFee = 123n;
  const expected = {
    state: 'retired',
    reason:
      'Payer grants are retired. Web requests are available and pay 123 wei as transaction value.',
    payment: { model: 'request-value', available: true, fee: 123n, payerGrants: 'retired' },
    did: DID,
    network_id: 125,
    observation: {
      rung: 'committed',
      block_number: BigInt(HEAD.number).toString(),
      state_root: HEAD.stateRoot.slice(2),
    },
  };
  expect(await readGrantCaps(chain, chain.address)).toEqual(expected);
  expect(chain.calls).toContainEqual({
    to: XWEB_PRECOMPILE.toLowerCase(),
    data: abiSelector('getParams()'),
  });
  expect(chain.callBlocks.length).toBeGreaterThan(0);
  expect(chain.callBlocks.every((block) => block === HEAD.number)).toBe(true);
  const mounted = await mountSurface(chain, createElement(GrantSurface));
  try {
    await until(
      () => mounted.container.querySelector('[data-caps-state="retired"]') !== null,
      'retired payer grants'
    );
    expect(mounted.container.textContent).toContain(expected.reason);
    expect(mounted.container.querySelector('[data-role="caps-empty"]')).toBeNull();
    expect(mounted.container.querySelector('[data-role="caps-discovery"]')).toBeNull();
    expect(mounted.container.querySelector('[data-grant]')).toBeNull();
    chain.xwebAvailable = false;
    chain.xwebFee = 456n;
    const pausedReason = 'Payer grants are retired. Web requests are currently paused.';
    await click(
      Array.from(mounted.container.querySelectorAll('button')).find(
        (button) => button.textContent === 'Refresh caps'
      ) ?? null,
      'refresh caps'
    );
    await until(
      () => mounted.container.textContent?.includes(pausedReason) === true,
      'paused payment after refresh'
    );
    expect(mounted.container.querySelector('[data-caps-state="retired"]')).not.toBeNull();
    expect(await readGrantCaps(chain, chain.address)).toEqual({
      ...expected,
      reason: pausedReason,
      payment: { model: 'request-value', available: false, fee: 456n, payerGrants: 'retired' },
    });
    expect(chain.holderQueries).toEqual([]);
    expect(chain.methods).not.toContain('eth_getLogs');
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
  for (const available of [true, false]) {
    chain.xwebAvailable = available;
    chain.xwebFee = 2n ** 200n + 17n;
    expect(await readGrantCaps(chain, chain.address)).toEqual({
      state: 'retired',
      reason: available
        ? `Payer grants are retired. Web requests are available and pay ${chain.xwebFee} wei as transaction value.`
        : 'Payer grants are retired. Web requests are currently paused.',
      payment: {
        model: 'request-value',
        available,
        fee: chain.xwebFee,
        payerGrants: 'retired',
      },
      did: DID,
      network_id: 125,
      observation: {
        rung: 'committed',
        block_number: BigInt(HEAD.number).toString(),
        state_root: HEAD.stateRoot.slice(2),
      },
    });
  }
  expect(chain.blockQueries).toEqual(['finalized', 'finalized']);
  expect(chain.callBlocks.length).toBeGreaterThan(0);
  expect(chain.callBlocks.every((block) => block === HEAD.number)).toBe(true);
  expect(chain.calls.filter((call) => call.data === abiSelector('getParams()'))).toEqual([
    { to: XWEB_PRECOMPILE.toLowerCase(), data: abiSelector('getParams()') },
    { to: XWEB_PRECOMPILE.toLowerCase(), data: abiSelector('getParams()') },
  ]);
  expect(chain.holderQueries).toEqual([]);
  expect(chain.methods).not.toContain('eth_getLogs');
  expect(
    chain.calls.some((call) => call.data.startsWith(abiSelector('getPayerGrant(bytes32)')))
  ).toBe(false);
}

test(
  incomplete_grant_discovery_requires_coverage_on_every_page_and_refuses_unknown_heights.name,
  incomplete_grant_discovery_requires_coverage_on_every_page_and_refuses_unknown_heights
);

async function budgets_not_discovered_is_exposed_separately_from_verified_grant_absence() {
  const chain = new GrantChain();
  const caps = await readGrantCaps(chain, chain.address);
  expect(caps).toEqual({
    state: 'retired',
    reason:
      'Payer grants are retired. Web requests are available and pay 0 wei as transaction value.',
    payment: { model: 'request-value', available: true, fee: 0n, payerGrants: 'retired' },
    did: DID,
    network_id: 125,
    observation: {
      rung: 'committed',
      block_number: BigInt(HEAD.number).toString(),
      state_root: HEAD.stateRoot.slice(2),
    },
  });
  const mounted = await mountSurface(chain, createElement(GrantSurface));
  try {
    await until(
      () => mounted.container.querySelector('[data-caps-state="retired"]') !== null,
      'retired grants with default payment availability'
    );
    expect(text(mounted.container, '[data-role="budgets-unavailable"]')).toBe(
      'Budget discovery is unavailable. Budget balances have not been read.'
    );
    expect(mounted.container.textContent).toContain(
      'Payer grants are retired. Web requests are available and pay 0 wei as transaction value.'
    );
    expect(mounted.container.querySelector('[data-role="caps-empty"]')).toBeNull();
    expect(mounted.container.querySelector('[data-role="caps-discovery"]')).toBeNull();
    expect(mounted.container.querySelector('[data-budget]')).toBeNull();
    expect(mounted.container.querySelector('[data-grant]')).toBeNull();
    expect(chain.holderQueries).toEqual([]);
    expect(chain.methods).not.toContain('eth_getLogs');
  } finally {
    await mounted.unmount();
  }
}

test(
  budgets_not_discovered_is_exposed_separately_from_verified_grant_absence.name,
  budgets_not_discovered_is_exposed_separately_from_verified_grant_absence
);
