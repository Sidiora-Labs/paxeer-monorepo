// @vitest-environment jsdom

import NextApp from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { makePublicRouterInstance, useRouter } from 'next/router';
import React from 'react';
import type { Root } from 'react-dom/client';
import { createRoot } from 'react-dom/client';

import { afterEach, beforeEach, expect, test } from 'vitest';

import ErrorBoundary from './ErrorBoundary';

const FAILED_HASH = `0x${ '1'.repeat(64) }`;
const RECOVERED_HASH = `0x${ '2'.repeat(64) }`;
const NEXT_FAILED_HASH = `0x${ '3'.repeat(64) }`;
const resources = new Map<string, string | Error>();
const caughtErrors: Array<Error> = [];
const restoredGlobals = new Map<string, PropertyDescriptor | undefined>();

let container: HTMLDivElement;
let root: Root;
let router: Router | undefined;

function installBootstrapValue(name: string, value: unknown) {
  restoredGlobals.set(name, Object.getOwnPropertyDescriptor(window, name));
  Object.defineProperty(window, name, { configurable: true, writable: true, value });
}

function Resource() {
  const { query } = useRouter();
  const hash = query.hash;
  if (typeof hash !== 'string' || !resources.has(hash)) {
    throw new Error('Resource hash is absent from the committed fixture');
  }
  const resource = resources.get(hash);
  if (resource instanceof Error) {
    throw resource;
  }
  return React.createElement('article', { 'data-resource': hash }, resource);
}

function renderErrorScreen(error?: Error) {
  return React.createElement('p', { role: 'alert' }, error?.message);
}

function onError(error: Error) {
  caughtErrors.push(error);
}

function renderRoute() {
  if (!router) {
    throw new Error('Expected an initialized Next.js router');
  }
  root.render(React.createElement(RouterContext.Provider, { value: makePublicRouterInstance(router) },
    React.createElement(ErrorBoundary, {
      renderErrorScreen,
      onError,
      children: React.createElement(Resource),
    }),
  ));
}

beforeEach(() => {
  Object.defineProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT', { configurable: true, value: true });
  resources.clear();
  caughtErrors.length = 0;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async() => {
  await React.act(async() => root.unmount());
  container.remove();
  if (router) {
    window.removeEventListener('popstate', router.onPopState);
    router = undefined;
  }
  for (const [ name, descriptor ] of restoredGlobals) {
    if (descriptor) {
      Object.defineProperty(window, name, descriptor);
    } else {
      Reflect.deleteProperty(window, name);
    }
  }
  restoredGlobals.clear();
  window.history.replaceState(null, '', '/');
});

async function assertDynamicResourceRecovery(kind: 'tx' | 'address') {
  const pathname = `/${ kind }/[hash]`;
  const failedPath = `/${ kind }/${ FAILED_HASH }`;
  const recoveredPath = `/${ kind }/${ RECOVERED_HASH }`;
  const firstError = new Error(`${ kind } resource failed`);
  const nextError = new Error(`Another ${ kind } resource failed`);
  resources.set(FAILED_HASH, firstError);
  resources.set(RECOVERED_HASH, `${ kind } resource recovered`);
  resources.set(NEXT_FAILED_HASH, nextError);

  // Next's real router and page loader consume the same bootstrap manifests as a browser page.
  installBootstrapValue('__NEXT_DATA__', {
    props: { pageProps: {} }, page: pathname, query: { hash: FAILED_HASH }, buildId: 'repair-22-33-320', gssp: true,
  });
  installBootstrapValue('__BUILD_MANIFEST', {
    __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] }, sortedPages: [ pathname ], [pathname]: [],
  });
  installBootstrapValue('__DEV_PAGES_MANIFEST', { pages: [ pathname ] });
  installBootstrapValue('__DEV_MIDDLEWARE_MATCHERS', []);
  installBootstrapValue('__MIDDLEWARE_MATCHERS', []);
  installBootstrapValue('__SSG_MANIFEST', new Set<string>());
  window.history.replaceState(null, '', failedPath);

  router = new Router(pathname, { hash: FAILED_HASH }, failedPath, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('repair-22-33-320', ''),
    App: NextApp,
    wrapApp: App => App,
    Component: Resource,
    isFallback: false,
    subscription: async() => renderRoute(),
  });
  const activeRouter = router;

  await React.act(async() => renderRoute());
  expect(container.querySelector('[role="alert"]')?.textContent).toBe(firstError.message);
  expect(caughtErrors).toEqual([ firstError ]);
  expect(container.querySelector('article')).toBeNull();

  resources.set(FAILED_HASH, 'Data is available again on the same route');
  await React.act(async() => renderRoute());
  expect(container.querySelector('[role="alert"]')?.textContent).toBe(firstError.message);
  expect(container.querySelector('article')).toBeNull();
  expect(caughtErrors).toEqual([ firstError ]);

  let navigated: boolean | undefined;
  await React.act(async() => {
    navigated = await activeRouter.push({ pathname, query: { hash: RECOVERED_HASH } }, recoveredPath, { shallow: true, scroll: false });
  });
  expect(navigated).toBe(true);
  expect(activeRouter.pathname).toBe(pathname);
  expect(activeRouter.asPath).toBe(recoveredPath);
  expect(window.location.pathname).toBe(recoveredPath);
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.querySelector('article')?.getAttribute('data-resource')).toBe(RECOVERED_HASH);
  expect(container.querySelector('article')?.textContent).toBe(`${ kind } resource recovered`);
  expect(caughtErrors).toEqual([ firstError ]);

  await React.act(async() => {
    await activeRouter.push({ pathname, query: { hash: NEXT_FAILED_HASH } }, `/${ kind }/${ NEXT_FAILED_HASH }`, { shallow: true, scroll: false });
  });
  expect(container.querySelector('[role="alert"]')?.textContent).toBe(nextError.message);
  expect(container.querySelector('article')).toBeNull();
  expect(caughtErrors).toEqual([ firstError, nextError ]);

  await React.act(async() => {
    await activeRouter.push({ pathname, query: { hash: RECOVERED_HASH } }, recoveredPath, { shallow: true, scroll: false });
  });
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.querySelector('article')?.textContent).toBe(`${ kind } resource recovered`);
  expect(caughtErrors).toEqual([ firstError, nextError ]);
}

async function boundaryDynamicNavigationTransactionHashes() {
  await assertDynamicResourceRecovery('tx');
}

async function boundaryDynamicNavigationAddressHashes() {
  await assertDynamicResourceRecovery('address');
}

test(boundaryDynamicNavigationTransactionHashes.name, boundaryDynamicNavigationTransactionHashes);
test(boundaryDynamicNavigationAddressHashes.name, boundaryDynamicNavigationAddressHashes);
