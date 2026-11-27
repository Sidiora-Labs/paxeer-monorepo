// @vitest-environment jsdom
import { act, useState, type ReactElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, beforeAll, expect, test } from 'vitest';
import {
  Button, CoinBadge, CoinCard, CoinRow, DirectionChip, EmptyState, ErrorState,
  OptionRow, Panel, ProgressLine, SegmentedControl, Sheet, TextTabs, VerifiedMark,
} from './index';

const roots: Array<{ root: Root; container: HTMLElement }> = [];
beforeAll(() => { Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true }); });
afterEach(() => {
  for (const { root, container } of roots.splice(0).reverse()) {
    act(() => root.unmount());
    container.remove();
  }
});

function mount(node: ReactElement) {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  roots.push({ root, container });
  act(() => root.render(node));
  return { container, render: (next: ReactElement) => act(() => root.render(next)) };
}

function inverseActionKeepsItsLabelAndStates() {
  const view = mount(<Button variant="inverse">Sign in</Button>);
  const button = view.container.querySelector('button')!;
  expect(button.classList).toContain('bg-surface-inverse');
  expect(button.classList).toContain('text-canvas');
  expect(button.classList).toContain('whitespace-nowrap');
  expect(button.textContent).toBe('Sign in');
  view.render(<Button variant="inverse" loading>Sign in</Button>);
  expect(button.disabled).toBe(true);
  expect(button.getAttribute('aria-busy')).toBe('true');
  expect(button.textContent).toBe('Sign in');
  view.render(<Button variant="inverse" href="/profile">Sign in</Button>);
  expect(view.container.querySelector('a')?.getAttribute('href')).toBe('/profile');
}

function textLinksKeepTheirTextFlowAndOtherButtonsKeepSingleLineLabels() {
  const variants = ['primary', 'secondary', 'ghost', 'danger', 'danger-text', 'inverse', 'link'] as const;
  for (const variant of variants) {
    for (const href of [undefined, '/profile']) {
      const view = mount(<Button variant={variant} href={href}>Max</Button>);
      const control = view.container.querySelector('button, a')!;
      const label = control.querySelector('span')!;
      if (variant === 'link') {
        expect(control.classList).toContain('flex-wrap');
        expect(control.classList).not.toContain('min-w-0');
        expect(control.classList).not.toContain('whitespace-nowrap');
        expect(label.classList).not.toContain('truncate');
      } else {
        expect(control.classList).not.toContain('flex-wrap');
        expect(control.classList).toContain('min-w-0');
        expect(control.classList).toContain('whitespace-nowrap');
        expect(label.classList).toContain('truncate');
      }
      expect(control.textContent).toBe('Max');
    }
  }
}

function directionChipShowsArrowAndPercentage() {
  const view = mount(<DirectionChip value={12.5} />);
  const chip = view.container.firstElementChild!;
  expect(chip.classList).toContain('text-percent-up');
  expect(chip.classList).toContain('bg-percent-up-soft');
  expect(chip.classList).toContain('rounded-direction');
  expect(chip.classList).toContain('text-num-sm');
  expect(chip.textContent).toContain('up');
  expect(chip.querySelector('svg')).not.toBeNull();
  view.render(<DirectionChip value={-2} />);
  expect(view.container.textContent).toContain('down');
  expect(view.container.textContent).toContain('-2.00%');
  expect(chip.classList).toContain('bg-percent-down-soft');
  view.render(<DirectionChip value={0} />);
  expect(chip.classList).toContain('bg-surface-2');
  expect(chip.querySelector('svg')).toBeNull();
  view.render(<DirectionChip value={null} />);
  expect(view.container.textContent).toBe('—');
  expect(view.container.firstElementChild?.getAttribute('aria-label')).toBe('No change yet');
}

function coinBadgesAndVerifiedMarkIdentifyTheirMeaning() {
  const view = mount(<><CoinBadge kind="hot" /><CoinBadge kind="new" /><CoinBadge kind="graduated" /><CoinBadge kind="creator" /><CoinBadge kind="pool" /><VerifiedMark /></>);
  const badges = view.container.querySelectorAll('.sl-badge-outline');
  expect(Array.from(badges, badge => badge.textContent)).toEqual(['hot', 'new', 'graduated', 'creator', 'pool']);
  expect(badges[0].classList).toContain('text-danger');
  expect(badges[1].classList).toContain('text-info');
  expect(badges[2].classList).toContain('text-accent');
  expect(view.container.querySelector('[aria-label="Verified"] svg')).not.toBeNull();
}

function listTabsAndPanelSegmentsKeepKeyboardSelection() {
  function Choices() {
    const [value, setValue] = useState('one');
    return <>
      <TextTabs label="Lists" active={value} onChange={setValue} idPrefix="lists" tabs={[{ id: 'one', label: 'One' }, { id: 'two', label: 'Two' }]} />
      <SegmentedControl label="Time frame" value={value} onChange={setValue} options={[{ value: 'one', label: 'One' }, { value: 'two', label: 'Two' }]} />
    </>;
  }
  const view = mount(<Choices />);
  const first = view.container.querySelector<HTMLButtonElement>('[role="tab"]')!;
  expect(first.querySelector('.bg-text')).not.toBeNull();
  expect(view.container.querySelector('[role="tablist"]')?.classList).toContain('overflow-x-auto');
  act(() => first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })));
  expect(view.container.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toBe('Two');
  const radio = view.container.querySelector<HTMLButtonElement>('[role="radio"][aria-checked="true"]')!;
  expect(radio.textContent).toBe('Two');
  expect(radio.classList).toContain('bg-surface-3');
  expect(radio.classList).not.toContain('underline');
  act(() => radio.dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true })));
  expect(view.container.querySelector('[role="radio"][aria-checked="true"]')?.textContent).toBe('One');
}

function coinRowIsOneLinkWithAnImageAndDirectionWash() {
  const props = { href: '/coin/0x1111111111111111111111111111111111111111', image: null, name: 'Science', symbol: 'SCI', createdAt: null, mcapRaw: '1000000', percentChange: 5 };
  const view = mount(<CoinRow {...props} />);
  const link = view.container.querySelector('a')!;
  expect(view.container.querySelectorAll('a')).toHaveLength(1);
  expect(link.getAttribute('href')).toBe(props.href);
  expect(link.classList).toContain('sl-row-wash-up');
  expect(link.querySelector('.rounded-coin')?.textContent).toBe('SC');
  expect(link.textContent).toContain('$1.00');
  expect(link.querySelectorAll('button')).toHaveLength(0);
  expect(link.querySelector('[title="Science"]')?.classList).toContain('truncate');
  view.render(<CoinRow {...props} percentChange={0} />);
  expect(link.classList).not.toContain('sl-row-wash-up');
  view.render(<CoinRow {...props} loading />);
  expect(view.container.querySelector('[aria-busy="true"]')).not.toBeNull();
  expect(view.container.querySelector('a')).toBeNull();
  expect(view.container.querySelectorAll('.sl-skeleton')).toHaveLength(4);
}

function progressLineLabelsAnAccentTrackAndUnknownValue() {
  const view = mount(<ProgressLine label="Curve progress" value={56} />);
  expect(view.container.textContent).toContain('Curve progress56%');
  const progress = view.container.querySelector('progress')!;
  expect(progress.getAttribute('aria-label')).toBe('Curve progress');
  expect(progress.value).toBe(56);
  expect(progress.style.getPropertyValue('--sl-progress-fill')).toBe('var(--sl-track-fill)');
  expect(progress.classList).toContain('h-1.5');
  view.render(<ProgressLine label="Curve progress" value={null} />);
  expect(progress.hasAttribute('value')).toBe(false);
  expect(progress.getAttribute('aria-valuetext')).toBe('unknown, Curve progress');
  expect(view.container.textContent).toContain('—');
}

function optionRowOpensInPlaceAndRevealsValidation() {
  const view = mount(<OptionRow label="Rewards" choice="Creator"><label htmlFor="receiver">Receiver</label><input id="receiver" /></OptionRow>);
  const summary = view.container.querySelector('summary')!;
  const panel = view.container.querySelector<HTMLElement>('[role="region"]')!;
  expect(summary.textContent).toBe('RewardsCreator');
  expect(panel.hidden).toBe(true);
  act(() => summary.click());
  expect(panel.hidden).toBe(false);
  expect(summary.getAttribute('aria-expanded')).toBe('true');
  act(() => summary.click());
  expect(panel.hidden).toBe(true);
  view.render(<OptionRow label="Rewards" choice="Creator" error="Enter the receiver"><input aria-label="Receiver" /></OptionRow>);
  expect(panel.hidden).toBe(false);
  expect(panel.querySelector('[role="alert"]')?.textContent).toBe('Enter the receiver');
}

function panelUsesSurfaceDepthAndContentSizedStates() {
  const view = mount(<Panel aria-label="Coins"><EmptyState title="No coins yet" body="Coins appear here after launch." /></Panel>);
  const panel = view.container.querySelector('section')!;
  expect(panel.classList).toContain('bg-surface-1');
  expect(panel.classList).toContain('rounded-md');
  expect(panel.classList).toContain('shadow-none');
  expect(panel.querySelector('h3')?.textContent).toBe('No coins yet');
  expect(panel.querySelector('button')).toBeNull();
  view.render(<Panel nested><ErrorState message="Coins could not load" onRetry={() => {}} /></Panel>);
  expect(panel.classList).toContain('bg-surface-2');
  expect(panel.classList).toContain('rounded-sm');
  expect(panel.querySelector('[role="alert"]')?.textContent).toContain('Coins could not load');
  expect(panel.querySelector('button')?.textContent).toBe('Try again');
}

function sheetSharesDialogFocusAndCloseBehaviour() {
  let closes = 0;
  const view = mount(<><button type="button">Open trade</button><Sheet open={false} onClose={() => { closes += 1; }} title="Trade"><input aria-label="Amount" /></Sheet></>);
  const opener = view.container.querySelector('button')!;
  opener.focus();
  view.render(<><button type="button">Open trade</button><Sheet open onClose={() => { closes += 1; }} title="Trade"><input aria-label="Amount" /></Sheet></>);
  const dialog = document.querySelector<HTMLDialogElement>('dialog')!;
  expect(dialog.classList).toContain('sl-sheet');
  expect(dialog.classList).toContain('sl-floating');
  expect(dialog.getAttribute('aria-modal')).toBe('true');
  expect(dialog.contains(document.activeElement)).toBe(true);
  act(() => dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })));
  expect(closes).toBe(1);
  view.render(<><button type="button">Open trade</button><Sheet open={false} onClose={() => {}} title="Trade">Amount</Sheet></>);
  expect(document.activeElement).toBe(opener);
  expect(document.querySelector('dialog')).toBeNull();
}

function coinCardShowsDescriptionAndCurveProgress() {
  const out = renderToStaticMarkup(<CoinCard image={null} name="Science" symbol="SCI" mcapRaw="1000000" percentChange={2} createdAt={null} href="/coin/science" creator="Ada" description="A coin for science" progress={56} />);
  expect(out).toContain('Ada');
  expect(out).toContain('line-clamp-2');
  expect(out).toContain('A coin for science');
  expect(out).toContain('Curve progress');
}

for (const check of [inverseActionKeepsItsLabelAndStates, textLinksKeepTheirTextFlowAndOtherButtonsKeepSingleLineLabels, directionChipShowsArrowAndPercentage,
  coinBadgesAndVerifiedMarkIdentifyTheirMeaning, listTabsAndPanelSegmentsKeepKeyboardSelection,
  coinRowIsOneLinkWithAnImageAndDirectionWash, progressLineLabelsAnAccentTrackAndUnknownValue,
  optionRowOpensInPlaceAndRevealsValidation, panelUsesSurfaceDepthAndContentSizedStates,
  sheetSharesDialogFocusAndCloseBehaviour, coinCardShowsDescriptionAndCurveProgress]) test(check.name, check);
