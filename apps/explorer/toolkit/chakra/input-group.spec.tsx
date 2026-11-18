// @vitest-environment jsdom

import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { marketStyleRules } from 'ui/home/marketData.test-utils';
import { render, Wrapper } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';

import { Input } from './input';
import { InputGroup } from './input-group';

test('shows input content before measurement with explicit offsets', function explicitOffsetsShowInputContentBeforeMeasurement() {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(
    <Wrapper>
      <InputGroup startElement="start" endElement="end" startOffset="10" endOffset="12">
        <Input value="PAX" placeholder="Search" readOnly/>
      </InputGroup>
    </Wrapper>,
  );

  expect(container.querySelector('input')?.value).toBe('PAX');
  expect(container.querySelector('input')?.placeholder).toBe('Search');
  const { container: mounted } = render(
    <InputGroup startElement="start" endElement="end" startOffset="10" endOffset="12">
      <Input value="PAX" placeholder="Search" readOnly/>
    </InputGroup>,
  );
  const initialInput = container.querySelector('input')!;
  expect(initialInput.className).toBe(mounted.querySelector('input')?.className);
  expect(marketStyleRules(initialInput)).toContain('padding-inline-start:var(--chakra-spacing-10)');
  expect(marketStyleRules(initialInput)).toContain('padding-inline-end:var(--chakra-spacing-12)');
});

test('keeps measured groups waiting for element widths', function measuredGroupsKeepTheirInitialContentBehavior() {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(
    <Wrapper>
      <InputGroup startElement="start" endElement="end">
        <Input value="PAX" placeholder="Search" readOnly/>
      </InputGroup>
    </Wrapper>,
  );

  expect(container.querySelector('input')?.value).toBe('');
  expect(container.querySelector('input')?.placeholder).toBe('');
});
