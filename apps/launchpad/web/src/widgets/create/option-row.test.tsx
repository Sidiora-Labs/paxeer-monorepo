// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, expect, test } from 'vitest';
import { CreateFormFields } from './create-form-fields';
import { validateCreationDraft, type CreationDraftV2 } from './creation-storage';

let root: Root | undefined;
let host: HTMLDivElement;
const draft: CreationDraftV2 = {
  draftId: 'options', version: 2, creator: '0x1234567890abcdef1234567890abcdef12345678', chainId: 125,
  name: 'Science Inu', symbol: 'SCINU', description: 'A coin for science.', tags: [], socialLinks: [],
  uploadRefs: [], firstBuyRaw: '0', opticalSelection: 'standard', feeStrategy: 0, updatedAt: Date.now(),
};

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});
afterEach(() => {
  act(() => root?.unmount());
  host.remove();
  root = undefined;
});

function show(value: CreationDraftV2, errors: Record<string, string>, registryFailed = false) {
  if (!root) {
    host = document.createElement('div');
    document.body.appendChild(host);
    root = createRoot(host);
  }
  act(() => root!.render(<CreateFormFields draft={value} errors={errors} locked={false}
    onEdit={() => { throw new Error('Opening an option must preserve its value'); }} registeredOpticals={[]}
    registry={registryFailed ? { state: 'error', message: 'Registry unavailable' } : { state: 'ready' }} opticalReason={() => null} />));
}

function closedOptionOpensWhenItsFieldFails() {
  show(draft, {});
  const links = host.querySelector<HTMLElement>('#create-social-links')!;
  expect(links.getAttribute('aria-expanded')).toBe('false');
  const invalid = { ...draft, socialLinks: ['http://example.org'] };
  const errors = validateCreationDraft(invalid);
  expect(errors.socialLinks).toBe('unsafe_url');
  show(invalid, errors);
  expect(links.getAttribute('aria-expanded')).toBe('true');
  expect(host.querySelector('#create-social-links-panel [role="alert"]')?.textContent).toBe('Add up to 5 links, each starting with https://');
  expect(host.querySelector<HTMLInputElement>('#create-link-website')?.value).toBe('http://example.org');
  expect(host.querySelector('#create-link-website')?.getAttribute('aria-invalid')).toBe('true');
  expect(host.querySelector('#create-rewards')?.getAttribute('aria-expanded')).toBe('false');
  expect(host.querySelector('#create-rules')?.getAttribute('aria-expanded')).toBe('false');
}

function optionRowsShowPresentChoicesAndKeepValuesWhenToggled() {
  const restored = { ...draft, socialLinks: ['https://example.org'], feeStrategy: 2 as const };
  show(restored, {});
  expect(host.querySelector('#create-social-links')?.textContent).toContain('1 added');
  expect(host.querySelector('#create-rewards')?.textContent).toContain('Holders');
  expect(host.querySelector('#create-pair')?.textContent).toContain('USDL');
  expect(host.querySelector('#create-rules')?.textContent).toContain('Standard');
  const rows = [...host.querySelectorAll<HTMLElement>('summary')];
  expect(rows).toHaveLength(4);
  for (const row of rows) {
    expect(row.getAttribute('aria-expanded')).toBe('false');
    act(() => row.click());
    expect(row.getAttribute('aria-expanded')).toBe('true');
    expect(host.querySelector(`#${row.getAttribute('aria-controls')}`)?.getAttribute('hidden')).toBeNull();
    act(() => row.click());
    expect(row.getAttribute('aria-expanded')).toBe('false');
  }
}

function registryFailureOpensTradingRulesWithItsMessage() {
  show(draft, {});
  expect(host.querySelector('#create-rules')?.getAttribute('aria-expanded')).toBe('false');
  show(draft, {}, true);
  expect(host.querySelector('#create-rules')?.getAttribute('aria-expanded')).toBe('true');
  expect(host.querySelector('#create-rules-panel')?.textContent).toContain("Custom rules can't load right now, so only Standard is available.");
  expect(host.querySelector('#create-rewards')?.getAttribute('aria-expanded')).toBe('false');
}

test(closedOptionOpensWhenItsFieldFails, closedOptionOpensWhenItsFieldFails);
test(optionRowsShowPresentChoicesAndKeepValuesWhenToggled, optionRowsShowPresentChoicesAndKeepValuesWhenToggled);
test(registryFailureOpensTradingRulesWithItsMessage, registryFailureOpensTradingRulesWithItsMessage);

function validationOpensRewardsAndTradingRulesWithoutChangingTheDraft() {
  show(draft, {});
  show(draft, { feeStrategy: 'Pick where creator rewards go.', opticalSelection: 'Pick Standard to continue.' });
  expect(host.querySelector('#create-rewards')?.getAttribute('aria-expanded')).toBe('true');
  expect(host.querySelector('#create-rewards-panel [role="alert"]')?.textContent).toBe('Pick where creator rewards go.');
  expect(host.querySelector('#create-rules')?.getAttribute('aria-expanded')).toBe('true');
  expect(host.querySelector('#create-rules-panel [role="alert"]')?.textContent).toBe('Pick Standard to continue.');
  expect(host.querySelector('#create-pair')?.getAttribute('aria-expanded')).toBe('false');
}

test(validationOpensRewardsAndTradingRulesWithoutChangingTheDraft, validationOpensRewardsAndTradingRulesWithoutChangingTheDraft);
