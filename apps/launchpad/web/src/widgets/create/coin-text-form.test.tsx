// @vitest-environment jsdom
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { CreateFormFields, fieldMessage, NAME_MAX_BYTES, SYMBOL_MAX_BYTES } from './create-form-fields';
import { validateCreationDraft, type CreationDraftV2 } from './creation-storage';

function coinDraft(changes: Partial<CreationDraftV2> = {}): CreationDraftV2 {
  return {
    draftId: 'coin-text', version: 2, creator: '0x1234567890abcdef1234567890abcdef12345678', chainId: 125,
    name: 'Coin', symbol: 'MiXeD', description: '', tags: [], socialLinks: [], uploadRefs: [], firstBuyRaw: '0',
    opticalSelection: 'standard', updatedAt: Date.now(), feeStrategy: 0, ...changes,
  };
}

function coinCreationFormReportsControlAndDirectionCharacters() {
  for (const field of ['name', 'symbol'] as const) {
    for (const character of ['\u0000', '\u0085', '\n', '\u061c', '\u200e', '\u200f', '\u202e', '\u2066', '\u2069', '\u206f']) {
      const draft = coinDraft({ [field]: `A${character}` });
      const errors = validateCreationDraft(draft);
      expect(errors[field]).toBe('unsafe_text');
      const message = `${field === 'name' ? 'Name' : 'Ticker'} must not contain control or text direction characters`;
      expect(fieldMessage(field, errors[field])).toBe(message);
      const markup = renderToStaticMarkup(<CreateFormFields draft={draft} errors={errors} locked={false} onEdit={() => {}} registeredOpticals={[]} registry={{ state: 'ready' }} opticalReason={() => null} />);
      const doc = new DOMParser().parseFromString(markup, 'text/html');
      expect(doc.querySelector(`#create-${field}`)?.getAttribute('aria-invalid')).toBe('true');
      expect(doc.body.textContent).toContain(message);
    }
  }
}
test(coinCreationFormReportsControlAndDirectionCharacters.name, coinCreationFormReportsControlAndDirectionCharacters);

function coinCreationFormKeepsItsTextBounds() {
  expect(validateCreationDraft(coinDraft())).toEqual({});
  for (const field of ['name', 'symbol'] as const) {
    const max = field === 'name' ? NAME_MAX_BYTES : SYMBOL_MAX_BYTES;
    expect(validateCreationDraft(coinDraft({ [field]: 'A'.repeat(max) }))).toEqual({});
    expect(validateCreationDraft(coinDraft({ [field]: 'A'.repeat(max + 1) }))[field]).toBe('invalid_text');
    expect(validateCreationDraft(coinDraft({ [field]: '🪙'.repeat(Math.floor(max / 4) + 1) }))[field]).toBe('invalid_text');
    expect(validateCreationDraft(coinDraft({ [field]: '' }))[field]).toBe('invalid_text');
    expect(validateCreationDraft(coinDraft({ [field]: ' ' }))).toEqual({});
    expect(validateCreationDraft(coinDraft({ [field]: '\u0000' }))[field]).toBe('unsafe_text');
    expect(validateCreationDraft(coinDraft({ [field]: 'A'.repeat(100) }))[field]).toBe('invalid_text');
  }
}
test(coinCreationFormKeepsItsTextBounds.name, coinCreationFormKeepsItsTextBounds);
