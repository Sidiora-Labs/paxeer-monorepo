import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { Block } from 'types/api/block';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import { base as blockFixture } from 'mocks/blocks/block';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { expect } from 'vitest';

import BlocksListItem from './BlocksListItem';
import BlocksTable from './BlocksTable';
import BlocksTableItem from './BlocksTableItem';

const alpha: ClusterChainConfig = {
  id: '2001',
  slug: 'tenant-alpha',
  name: 'Tenant Alpha',
  app_config: {
    ...config,
    chain: {
      ...config.chain,
      currency: { ...config.chain.currency, decimals: 6, symbol: 'ALPHA', weiName: 'drop', gweiName: 'Gdrop' },
    },
    features: { ...config.features, rollup: { title: 'Rollup', isEnabled: false } },
    UI: {
      ...config.UI,
      views: {
        ...config.UI.views,
        block: {
          ...config.UI.views.block,
          hiddenFields: { miner: false, total_reward: false, burnt_fees: false, base_fee: false, nonce: false, L1_status: false, batch: false },
        },
      },
    },
  },
};

const beta: ClusterChainConfig = {
  ...alpha,
  id: '2002',
  slug: 'tenant-beta',
  name: 'Tenant Beta',
  app_config: {
    ...alpha.app_config,
    chain: {
      ...alpha.app_config.chain,
      currency: { ...alpha.app_config.chain.currency, decimals: 8, symbol: 'BETA', weiName: 'grain', gweiName: 'Ggrain' },
    },
    UI: {
      ...alpha.app_config.UI,
      views: {
        ...alpha.app_config.UI.views,
        block: {
          ...alpha.app_config.UI.views.block,
          hiddenFields: { miner: true, total_reward: true, burnt_fees: false, base_fee: true, nonce: false, L1_status: false, batch: false },
        },
      },
    },
  },
};

async function withBlocksView(
  content: React.ReactElement,
  chain: ClusterChainConfig | undefined,
  run: (container: HTMLElement, update: (content: React.ReactElement, chain?: ClusterChainConfig) => void) => void,
) {
  const initialData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  const pathname = chain ? '/chain/[chain_slug]/blocks' : '/blocks';
  const query = chain ? { chain_slug: chain.slug } : {};
  window.__NEXT_DATA__ = { props: {}, page: pathname, query, buildId: 'block-chain-regression' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });

  function Page() {
    return content;
  }

  const router = new Router(pathname, query, chain ? `/chain/${ chain.slug }/blocks` : '/blocks', {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('block-chain-regression', ''),
    App,
    Component: Page,
    wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => <WrappedApp { ...props } router={ router }/>,
    subscription: async(data, CurrentApp) => {
      renderToStaticMarkup(<CurrentApp { ...data.props } Component={ data.Component } router={ router }/>);
    },
    isFallback: false,
  });

  const tree = (content: React.ReactElement, chain?: ClusterChainConfig) => (
    <RouterContext.Provider value={ router }>
      <MultichainContext.Provider value={ chain ? { chain } : null }>
        { content }
      </MultichainContext.Provider>
    </RouterContext.Provider>
  );
  let unmount: (() => void) | undefined;

  try {
    await router._initialMatchesMiddlewarePromise;
    const view = render(tree(content, chain));
    unmount = view.unmount;
    run(view.container, (content, chain) => view.rerender(tree(content, chain)));
  } finally {
    unmount?.();
    window.removeEventListener('popstate', router.onPopState);
    window.__NEXT_DATA__ = initialData;
    if (middlewareMatchers) {
      Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
    } else {
      Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
    }
  }
}

const row = (chain?: ClusterChainConfig, data: Block = blockFixture) => (
  <TableRoot><TableBody><BlocksTableItem data={ data } chainData={ chain }/></TableBody></TableRoot>
);

export async function mobileBlockChainLink() {
  await withBlocksView(<BlocksListItem data={ blockFixture } chainData={ alpha }/>, alpha, (container, update) => {
    const count = container.querySelector(`a[href="/chain/tenant-alpha/block/${ blockFixture.height }?tab=txs"]`);
    expect(count?.textContent).toBe(String(blockFixture.transactions_count));
    expect(container.querySelector(`a[href="/chain/tenant-alpha/block/${ blockFixture.height }"]`)).toBeTruthy();
    expect(container.textContent).toContain('Reward ALPHA');
    expect(container.textContent).toContain('2,026,853,607,510');
    expect(container.textContent).toContain('5,449,200,000');
    expect(container.textContent).toContain('10\u2009Gdrop');
    expect(container.querySelector(`a[href="/block/${ blockFixture.height }?tab=txs"]`)).toBeNull();

    update(<BlocksListItem data={ blockFixture } chainData={ beta }/>, beta);
    expect(container.textContent).not.toContain('Reward');
    expect(container.textContent).not.toContain('Validator');
    expect(container.textContent).not.toContain('Base fee');
    expect(container.textContent).toContain('Burnt fees');
    expect(container.textContent).toContain('54,492,000');
    expect(container.querySelector(`a[href="/chain/tenant-beta/block/${ blockFixture.height }?tab=txs"]`)).toBeTruthy();

    update(<BlocksListItem data={ blockFixture }/>);
    expect(container.querySelector(`a[href="/block/${ blockFixture.height }?tab=txs"]`)).toBeTruthy();
    expect(container.textContent).toContain('Reward ETH');
    expect(container.textContent).toContain('2.02685361');
  });
}

export async function blockTableConfiguration() {
  await withBlocksView(<BlocksTable data={ [ blockFixture ] } top={ 0 } page={ 2 } chainData={ alpha }/>, alpha, (container, update) => {
    const headings = () => Array.from(container.querySelectorAll('thead th'), cell => cell.textContent);
    const cells = () => Array.from(container.querySelectorAll('tbody tr td'));
    expect(headings()).toEqual([ '', 'Block', 'Size, bytes', 'Validator', 'Txn', 'Gas used', 'Reward ALPHA', 'Burnt fees ALPHA', 'Base fee' ]);
    expect(cells()).toHaveLength(9);
    expect(cells()[6].textContent).toBe('2,026,853,607,510');
    expect(cells()[7].textContent).toContain('5,449,200,000');

    update(<BlocksTable data={ [ blockFixture ] } top={ 0 } page={ 2 } chainData={ beta }/>, beta);
    expect(headings()).toEqual([ '', 'Block', 'Size, bytes', 'Txn', 'Gas used', 'Burnt fees BETA' ]);
    expect(cells()).toHaveLength(headings().length);
    expect(cells()[5].textContent).toContain('54,492,000');
    expect(container.querySelector('a[href*="/address/"]')).toBeNull();
    expect(container.querySelector(`a[href="/chain/tenant-beta/block/${ blockFixture.height }?tab=txs"]`)).toBeTruthy();

    update(<BlocksTable data={ [ blockFixture ] } top={ 0 } page={ 2 }/>);
    expect(headings()).toEqual([ 'Block', 'Size, bytes', 'Validator', 'Txn', 'Gas used', 'Reward ETH', 'Burnt fees ETH', 'Base fee' ]);
    expect(cells()).toHaveLength(headings().length);
    expect(cells()[5].textContent).toBe('2.02685361');
    expect(cells()[6].textContent).toContain('0.0054492');
    expect(container.querySelector(`a[href="/block/${ blockFixture.height }?tab=txs"]`)).toBeTruthy();
  });
}

export async function desktopBlockValueScale() {
  const preciseBlock: Block = {
    ...blockFixture,
    rewards: [ { reward: '9007199254740993', type: 'Validator Reward' }, { reward: '7', type: 'Emission Reward' } ],
  };
  await withBlocksView(row(alpha, preciseBlock), alpha, (container, update) => {
    const cells = () => Array.from(container.querySelectorAll('td'));
    expect(cells()[6].textContent).toBe('9,007,199,254.741');
    expect(cells()[7].textContent).toContain('5,449,200,000');
    expect(cells()[8].textContent).toBe('10\u2009Gdrop');

    const visibleBeta: ClusterChainConfig = {
      ...beta,
      app_config: { ...beta.app_config, UI: alpha.app_config.UI },
    };
    update(row(visibleBeta, preciseBlock), visibleBeta);
    expect(cells()[6].textContent).toBe('90,071,992.54741');
    expect(cells()[7].textContent).toContain('54,492,000');
    expect(cells()[8].textContent).toBe('10\u2009Ggrain');

    update(row(visibleBeta, { ...preciseBlock, base_fee_per_gas: '99999' }), visibleBeta);
    expect(cells()[8].textContent).toBe('99,999\u2009grain');
    update(row(visibleBeta, { ...preciseBlock, base_fee_per_gas: '100000' }), visibleBeta);
    expect(cells()[8].textContent).toBe('0.0001\u2009Ggrain');
  });
}

export async function desktopBlockChainLink() {
  await withBlocksView(row(alpha), alpha, (container, update) => {
    const countHref = () => container.querySelector('a[href$="?tab=txs"]')?.getAttribute('href');
    expect(countHref()).toBe(`/chain/tenant-alpha/block/${ blockFixture.height }?tab=txs`);
    expect(container.querySelector(`a[href="/chain/tenant-alpha/block/${ blockFixture.height }"]`)).toBeTruthy();
    update(row());
    expect(countHref()).toBe(`/block/${ blockFixture.height }?tab=txs`);
    expect(container.querySelector(`a[href="/block/${ blockFixture.height }"]`)).toBeTruthy();
  });
}

export async function selectedRollupBlockConfiguration() {
  const rollup: ClusterChainConfig = {
    ...alpha,
    app_config: {
      ...alpha.app_config,
      features: {
        ...alpha.app_config.features,
        rollup: {
          title: 'Rollup', isEnabled: true, type: 'optimistic', stageIndex: undefined, layerNumber: 2,
          homepage: { showLatestBlocks: true }, outputRootsEnabled: false, interopEnabled: false,
          L2WithdrawalUrl: undefined, parentChain: { baseUrl: 'https://parent.example' },
          DA: { celestia: { namespace: undefined, celeniumUrl: undefined } },
        },
      },
    },
  };
  await withBlocksView(<BlocksTable data={ [ blockFixture ] } top={ 0 } page={ 2 } chainData={ rollup }/>, rollup, (container, update) => {
    expect(Array.from(container.querySelectorAll('thead th'), cell => cell.textContent))
      .toEqual([ '', 'Block', 'Size, bytes', 'Validator', 'Txn', 'Gas used' ]);
    expect(container.querySelectorAll('tbody td')).toHaveLength(6);
    update(<BlocksListItem data={ blockFixture } chainData={ rollup }/>, rollup);
    expect(container.textContent).not.toContain('Reward');
    expect(container.textContent).not.toContain('Burnt fees');
    expect(container.textContent).not.toContain('Base fee');
    expect(container.querySelector(`a[href="/chain/tenant-alpha/block/${ blockFixture.height }?tab=txs"]`)).toBeTruthy();
  });
}
