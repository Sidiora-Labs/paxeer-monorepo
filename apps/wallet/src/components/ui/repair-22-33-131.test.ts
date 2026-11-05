// @vitest-environment jsdom
import { createRequire } from 'node:module';
import { createElement, useState } from 'react';
import { MotionConfig } from 'framer-motion';
import { ethers } from 'ethers';
import tailwindcss from 'tailwindcss';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { mount, type Mounted } from '@/account/test/render';
import {
  MIN_SWAP_SLIPPAGE_BPS,
  MAX_SWAP_SLIPPAGE_BPS,
  validateSlippageBps,
} from '@/lib/txValidation';
import { prepareV4Multihop } from '@/lib/swap/executor';
import { CONTRACTS, PECOR_ROUTER_ABI, TOKENS } from '@/lib/swap/sdk';
import type { SwapQuote } from '@/lib/swap/types';
import { announcementRepository } from '@/platform/storage/repositories';
import { SwapConfirmDialog } from '@/widgets/swap/SwapConfirmDialog';
import { SlippageDrawer } from './SlippageDrawer';
import { WhatsNewModal } from './WhatsNewModal';

// Use the processor shipped with Tailwind so its plugin and AST share one PostCSS version.
const tailwindRequire = createRequire(createRequire(import.meta.url).resolve('tailwindcss'));
type TailwindProcessor = Extract<ReturnType<typeof tailwindcss>, { process: unknown }>;
const postcss: (plugins: ReturnType<typeof tailwindcss>[]) => TailwindProcessor =
  tailwindRequire('postcss');

const owner = '0x0000000000000000000000000000000000000125';
const quote: SwapQuote = {
  protocol: 'pecor-v4-multihop',
  protocolLabel: 'PECOR → Sidiora',
  tokenIn: { ...TOKENS.USDC },
  tokenOut: {
    address: '0x0000000000000000000000000000000000000321',
    name: 'Launchpad',
    symbol: 'LAUNCH',
    decimals: 6,
    isLaunchpad: true,
  },
  amountIn: '2000000',
  amountOut: '1000000',
  amountOutDisplay: '1',
  fee: '0',
  feeBps: 0,
  priceImpact: 0,
  sufficient: true,
};

interface FlowRecord {
  changes: number[];
  transactions: ethers.TransactionRequest[];
}

function SlippageFlow({ initialBps, record }: { initialBps: number; record: FlowRecord }) {
  const [bps, setBps] = useState(initialBps);
  const [settings, setSettings] = useState(true);
  return createElement(
    MotionConfig,
    { isStatic: true },
    createElement(SlippageDrawer, {
      open: settings,
      slippageBps: bps,
      onChange(value) {
        record.changes.push(value);
        setBps(value);
      },
      onClose: () => setSettings(false),
    }),
    createElement(SwapConfirmDialog, {
      open: !settings,
      loading: false,
      execError: '',
      bestQuote: quote,
      fromToken: quote.tokenIn,
      toToken: quote.tokenOut,
      fromAmount: '2',
      slippageBps: bps,
      onClose: () => setSettings(true),
      async onConfirm() {
        validateSlippageBps(bps);
        const prepared = prepareV4Multihop(new ethers.VoidSigner(owner), quote, {
          tokenIn: quote.tokenIn,
          tokenOut: quote.tokenOut,
          amountIn: quote.amountIn,
          slippageBps: bps,
          recipient: owner,
        });
        record.transactions.push(await prepared.method.populateTransaction(...prepared.args));
      },
    })
  );
}

let views: Mounted[];
let styles: HTMLStyleElement[];

beforeEach(() => {
  views = [];
  styles = [];
  localStorage.clear();
});

afterEach(async () => {
  for (const view of views) await view.unmount();
  for (const style of styles) style.remove();
  localStorage.clear();
});

async function flow(initialBps = 50) {
  const record: FlowRecord = { changes: [], transactions: [] };
  const view = await mount(createElement(SlippageFlow, { initialBps, record }));
  views.push(view);
  return { view, record };
}

function doneButton(view: Mounted): HTMLButtonElement {
  const button = view
    .all<HTMLButtonElement>('button')
    .find((entry) => entry.textContent === 'Done');
  if (!button) throw new Error('Slippage Done button is missing');
  button.dataset.slippageDone = 'true';
  return button;
}

async function finding40_invalid_custom_input_never_reaches_confirmation() {
  const { view, record } = await flow();
  expect(doneButton(view).disabled).toBe(false);
  for (const invalid of [
    '5.01',
    '49.99',
    '50',
    '0',
    '-1',
    '0.001',
    '5.001',
    '1.234',
    '1junk',
    'NaN',
    'Infinity',
    '1e0',
    '0x1',
    ' ',
    '.',
    '',
  ]) {
    await view.type('input', invalid);
    const input = view.query<HTMLInputElement>('input')!;
    expect(input.value).toBe(invalid);
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(input.getAttribute('aria-describedby')).toBe(view.query('[role="alert"]')?.id);
    expect(view.text('[role="alert"]')).toMatch(/percentage|Slippage/);
    expect(doneButton(view).disabled).toBe(true);
    await view.click('[data-slippage-done]');
    expect(view.container.textContent).toContain('Swap Settings');
    expect(view.container.textContent).not.toContain('Confirm Swap');
    expect(record.changes).toEqual([]);
  }
  await view.type('input', '5.01');
  expect(view.text('[role="alert"]')).toBe(
    `Slippage must be between ${MIN_SWAP_SLIPPAGE_BPS / 100}% and ${MAX_SWAP_SLIPPAGE_BPS / 100}%.`
  );
  const preset = view.all<HTMLButtonElement>('button').find((entry) => entry.textContent === '1%')!;
  preset.dataset.slippagePreset = 'true';
  await view.click('[data-slippage-preset]');
  expect(record.changes).toEqual([100]);
  expect(view.query('[role="alert"]')).toBeNull();
  expect(view.query<HTMLInputElement>('input')!.value).toBe('');
  expect(doneButton(view).disabled).toBe(false);
}

async function finding40_canonical_boundaries_and_preset_values_are_executable() {
  const { view, record } = await flow();
  for (const [value, bps] of [
    ['0.01', MIN_SWAP_SLIPPAGE_BPS],
    ['.29', 29],
    ['5', MAX_SWAP_SLIPPAGE_BPS],
  ] as const) {
    await view.type('input', value);
    expect(record.changes.at(-1)).toBe(bps);
    expect(() => validateSlippageBps(record.changes.at(-1)!)).not.toThrow();
    expect(view.query('[role="alert"]')).toBeNull();
    expect(doneButton(view).disabled).toBe(false);
  }
  for (const bps of [25, 50, 100, 200]) {
    const preset = view
      .all<HTMLButtonElement>('button')
      .find((entry) => entry.textContent === `${bps / 100}%`)!;
    preset.dataset.slippagePreset = 'true';
    await view.click('[data-slippage-preset]');
    delete preset.dataset.slippagePreset;
    expect(record.changes.at(-1)).toBe(bps);
    expect(() => validateSlippageBps(bps)).not.toThrow();
  }
  for (const initialBps of [0, 501, 4900]) {
    const invalid = await flow(initialBps);
    expect(doneButton(invalid.view).disabled).toBe(true);
    expect(invalid.view.query('[role="alert"]')).not.toBeNull();
    expect(invalid.record.changes).toEqual([]);
  }
}

async function finding40_drawer_confirmation_and_executor_preserve_basis_points() {
  const { view, record } = await flow();
  await view.type('input', '1.37');
  expect(record.changes).toEqual([137]);
  expect(view.query<HTMLInputElement>('input')!.value).toBe('1.37');
  expect(doneButton(view).disabled).toBe(false);
  await view.click('[data-slippage-done]');
  const slippageRow = view
    .all('span')
    .find((entry) => entry.textContent === 'Slippage')!.parentElement!;
  expect(slippageRow.textContent).toBe('Slippage1.37%');
  const minimumRow = view
    .all('span')
    .find((entry) => entry.textContent === 'Min. received')!.parentElement!;
  expect(minimumRow.textContent).toBe('Min. received0.9863 LAUNCH');
  const confirm = view
    .all<HTMLButtonElement>('button')
    .find((entry) => entry.textContent === 'Confirm Swap')!;
  confirm.dataset.swapConfirm = 'true';
  await view.click('[data-swap-confirm]');
  await view.until(() => record.transactions.length === 1, 'executor calldata');
  const transaction = record.transactions[0];
  expect(transaction.to).toBe(CONTRACTS.PECORRouter);
  const decoded = new ethers.Interface(PECOR_ROUTER_ABI).parseTransaction({
    data: String(transaction.data),
  })!;
  expect(decoded.name).toBe('swapMultiHop');
  expect(decoded.args[2]).toBe(986300n);
  expect(decoded.args[0][1].minAmountOut).toBe(986300n);
}

async function announcement() {
  const view = await mount(
    createElement(MotionConfig, { isStatic: true }, createElement(WhatsNewModal))
  );
  views.push(view);
  return view;
}

async function finding38_first_launch_sheet_stacks_above_backdrop_and_close_recovers() {
  expect(announcementRepository.read()).toBeNull();
  const view = await announcement();
  const sheet = view.query<HTMLElement>('[role="dialog"]')!;
  expect(sheet).not.toBeNull();
  expect(sheet.textContent).toContain("What's New");
  expect(sheet.textContent).toContain('Version 1.2.0');
  for (const title of [
    'Instant Swap',
    'Discover Tokens',
    'PNS Names',
    'Portfolio Charts',
    'Safer Approvals',
    'Get Started',
  ]) {
    expect(sheet.textContent).toContain(title);
  }
  // Compile the actual rendered utility classes: jsdom does not ship Tailwind's stylesheet.
  const css = await postcss([
    tailwindcss({
      content: [{ raw: view.container.innerHTML, extension: 'html' }],
      corePlugins: { preflight: false },
    }),
  ]).process('@tailwind utilities;', { from: undefined });
  const style = document.createElement('style');
  style.textContent = css.css;
  styles.push(style);
  document.head.appendChild(style);
  const backdrop = sheet.previousElementSibling!;
  const parent = sheet.parentElement!;
  expect(getComputedStyle(parent).position).toBe('fixed');
  expect(getComputedStyle(parent).isolation).toBe('isolate');
  expect(getComputedStyle(parent).zIndex).toBe('60');
  expect(backdrop.parentElement).toBe(parent);
  expect(getComputedStyle(backdrop).position).toBe('absolute');
  expect(getComputedStyle(sheet).position).toBe('relative');
  expect(getComputedStyle(backdrop).zIndex).toBe('0');
  expect(getComputedStyle(sheet).zIndex).toBe('10');
  await view.click('[aria-label="Close What\'s New"]');
  await view.until(() => view.query('[role="dialog"]') === null, 'close dismissal');
  expect(announcementRepository.read()).toBe('1.2.0');
  await view.unmount();
  views.splice(views.indexOf(view), 1);
  const reopened = await announcement();
  expect(reopened.query('[role="dialog"]')).toBeNull();
}

async function finding38_backdrop_and_get_started_dismiss_and_persist() {
  for (const action of ['backdrop', 'get-started']) {
    announcementRepository.remove();
    const view = await announcement();
    const sheet = view.query<HTMLElement>('[role="dialog"]')!;
    expect(sheet).not.toBeNull();
    const target =
      action === 'backdrop'
        ? (sheet.previousElementSibling as HTMLElement)
        : view
            .all<HTMLButtonElement>('button')
            .find((entry) => entry.textContent?.includes('Get Started'))!;
    target.dataset.announcementAction = 'true';
    await view.click('[data-announcement-action]');
    await view.until(() => view.query('[role="dialog"]') === null, `${action} dismissal`);
    expect(announcementRepository.read()).toBe('1.2.0');
    await view.unmount();
    views.splice(views.indexOf(view), 1);
    const reopened = await announcement();
    expect(reopened.query('[role="dialog"]')).toBeNull();
  }
}

test(
  finding40_invalid_custom_input_never_reaches_confirmation.name,
  finding40_invalid_custom_input_never_reaches_confirmation
);
test(
  finding40_canonical_boundaries_and_preset_values_are_executable.name,
  finding40_canonical_boundaries_and_preset_values_are_executable
);
test(
  finding40_drawer_confirmation_and_executor_preserve_basis_points.name,
  finding40_drawer_confirmation_and_executor_preserve_basis_points
);
test(
  finding38_first_launch_sheet_stacks_above_backdrop_and_close_recovers.name,
  finding38_first_launch_sheet_stacks_above_backdrop_and_close_recovers
);
test(
  finding38_backdrop_and_get_started_dismiss_and_persist.name,
  finding38_backdrop_and_get_started_dismiss_and_persist
);
