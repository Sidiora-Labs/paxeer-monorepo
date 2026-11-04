// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import { readFileSync } from 'node:fs';
import React from 'react';

import ts from 'typescript';

import { getResourceKey } from 'lib/api/useApiQuery';
import { base as stats, indexingStatus } from 'mocks/stats/index';
import AppError from 'ui/shared/AppError/AppError';
import { afterEach, expect, test, vi } from 'vitest';
import { within } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

import Layout from './Layout';
import LayoutError from './LayoutError';
import LayoutHome from './LayoutHome';
import { nextRouterModule, render } from './testWrapper';

function collectCalls(node: ts.Node): Array<ts.CallExpression> {
  const calls: Array<ts.CallExpression> = [];
  function visit(child: ts.Node) {
    if (ts.isCallExpression(child)) {
      calls.push(child);
    }
    ts.forEachChild(child, visit);
  }
  visit(node);
  return calls;
}

function assertScreenshotSuite(filename: string, subject: string, titles: Array<string>) {
  const source = ts.createSourceFile(
    filename,
    readFileSync(new URL(filename, import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const calls = collectCalls(source);
  const registrations = calls.filter((call) => call.expression.getText(source) === 'test');
  expect(registrations.map((call) => {
    const title = call.arguments[0];
    expect(ts.isStringLiteral(title)).toBe(true);
    return ts.isStringLiteral(title) ? title.text : undefined;
  })).toEqual(titles);
  expect(calls.filter((call) => /^test\.(?:describe\.)?(?:skip|fixme|fail|only)\b/.test(call.expression.getText(source))))
    .toHaveLength(0);

  const subjectImport = source.statements.find((statement) =>
    ts.isImportDeclaration(statement) && ts.isStringLiteral(statement.moduleSpecifier) && statement.moduleSpecifier.text === `./${ subject }`,
  );
  expect(subjectImport && ts.isImportDeclaration(subjectImport) && subjectImport.importClause?.name?.text).toBe(subject);

  for (const registration of registrations) {
    const callback = registration.arguments[1];
    expect(ts.isArrowFunction(callback)).toBe(true);
    const bodyCalls = collectCalls(callback);
    const renderCalls = bodyCalls.filter((call) => call.expression.getText(source) === 'render');
    expect(renderCalls).toHaveLength(1);
    const element = renderCalls[0].arguments[0];
    expect(ts.isJsxElement(element) && element.openingElement.tagName.getText(source)).toBe(subject);

    const screenshots = bodyCalls.filter((call) =>
      ts.isPropertyAccessExpression(call.expression) && call.expression.name.text === 'toHaveScreenshot',
    );
    expect(screenshots).toHaveLength(1);
    expect(screenshots[0].arguments).toHaveLength(0);
    expect(screenshots[0].expression.getText(source)).toBe('expect(component).toHaveScreenshot');
    expect(ts.isAwaitExpression(screenshots[0].parent)).toBe(true);

    const readiness = bodyCalls.filter((call) =>
      ts.isPropertyAccessExpression(call.expression) && [ 'toBeVisible', 'toHaveText' ].includes(call.expression.name.text),
    );
    expect(readiness.some((call) => call.getText(source).includes('component.locator(\'footer\')'))).toBe(true);
    expect(readiness.some((call) => call.getText(source).includes('10% Blocks Indexed'))).toBe(true);
    expect(readiness.some((call) => call.getText(source).includes(subject === 'LayoutHome' ? 'hero-band' : 'component.locator(\'main\')')))
      .toBe(true);
    for (const condition of readiness) {
      expect(ts.isAwaitExpression(condition.parent)).toBe(true);
      expect(condition.end).toBeLessThan(screenshots[0].pos);
    }
    const fontWaits = bodyCalls.filter((call) => call.expression.getText(source) === 'component.evaluate');
    expect(fontWaits).toHaveLength(1);
    expect(ts.isAwaitExpression(fontWaits[0].parent)).toBe(true);
    expect(fontWaits[0].getText(source)).toContain('await document.fonts.ready');
    expect(fontWaits[0].pos).toBeGreaterThan(readiness[readiness.length - 1].end);
    expect(fontWaits[0].end).toBeLessThan(screenshots[0].pos);
    expect(bodyCalls.some((call) => call.expression.getText(source).endsWith('waitForTimeout'))).toBe(false);
  }
  return { source, calls, registrations };
}

const clients: Array<QueryClient> = [];

afterEach(() => {
  clients.forEach((client) => client.clear());
  clients.length = 0;
});

function renderLayout(component: React.ReactElement) {
  const client = new QueryClient();
  clients.push(client);
  client.setQueryData(getResourceKey('general:homepage_indexing_status'), indexingStatus);
  client.setQueryData(getResourceKey('general:stats'), stats);
  const router = nextRouterModule().useRouter();
  return render(React.createElement(QueryClientProvider, { client },
    React.createElement(RouterContext.Provider, { value: router }, component),
  ));
}

test('layout_skipped: restores base/mobile and both xxl navigation screenshots after readiness', async function layoutSkipped() {
  const { source, calls, registrations } = assertScreenshotSuite('Layout.pw.tsx', 'Layout', [
    'base view +@mobile', 'vertical navigation', 'horizontal navigation',
  ]);
  const xxl = calls.find((call) => call.expression.getText(source) === 'test.describe');
  expect(xxl?.arguments[0].getText(source)).toBe('\'xxl screen\'');
  expect(xxl?.getText(source)).toContain('viewport: pwConfig.viewport.xxl');
  expect(xxl && collectCalls(xxl).filter((call) => call.expression.getText(source) === 'test'))
    .toEqual(registrations.slice(1));
  for (const [ index, navigation ] of [ 'vertical', 'horizontal' ].entries()) {
    expect(registrations[index + 1].getText(source)).toContain(`[ 'NEXT_PUBLIC_NAVIGATION_LAYOUT', '${ navigation }' ]`);
  }

  const { container } = renderLayout(React.createElement(Layout, null, 'Page Content'));
  const main = await within(container).findByRole('main');
  expect(main.textContent).toBe('Page Content');
  expect(container.querySelector('footer')).toBeTruthy();
  expect(await within(container).findByText(/10% Blocks Indexed/)).toBeTruthy();
  await document.fonts.ready;
});

test('error_layout_wrong_subject: screenshots the production error layout and error content after readiness', async function errorLayoutWrongSubject() {
  const { source, registrations } = assertScreenshotSuite('LayoutError.pw.tsx', 'LayoutError', [ 'base view +@mobile' ]);
  expect(registrations[0].getText(source)).toContain('<AppError error=');
  expect(registrations[0].getText(source)).toContain('Oops! Something went wrong');

  const { container } = renderLayout(React.createElement(LayoutError, null,
    React.createElement(AppError, { error: new Error('An unexpected error has occurred') }),
  ));
  const main = await within(container).findByRole('main');
  expect(within(main).getByText('Oops! Something went wrong')).toBeTruthy();
  expect(within(main).getByRole('button', { name: 'Back to home' })).toBeTruthy();
  expect(container.querySelector('[data-label="hero-band"]')).toBeNull();
  expect(container.querySelector('footer')).toBeTruthy();
  expect(await within(container).findByText(/10% Blocks Indexed/)).toBeTruthy();
  await document.fonts.ready;
});

test('home_layout_skipped: restores base/mobile home screenshots after readiness', async function homeLayoutSkipped() {
  assertScreenshotSuite('LayoutHome.pw.tsx', 'LayoutHome', [ 'base view +@mobile' ]);
  const { container } = renderLayout(React.createElement(LayoutHome, null, 'Page Content'));
  expect(await within(container).findByText('Page Content')).toBeTruthy();
  expect(container.querySelector('[data-label="hero-band"]')).toBeTruthy();
  expect(container.querySelector('footer')).toBeTruthy();
  expect(await within(container).findByText(/10% Blocks Indexed/)).toBeTruthy();
  await document.fonts.ready;
});
