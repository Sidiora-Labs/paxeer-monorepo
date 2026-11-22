import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { chromium } from 'playwright';
import { createConfig, http, WagmiProvider } from 'wagmi';
import { expect, it } from 'vitest';
import { PathnameContext } from 'next/dist/shared/lib/hooks-client-context.shared-runtime';
import Loading from '@/app/loading';
import { paxeer } from '@/core/wagmi-config';
import { queryKeys } from '@/core/query-keys';
import type { PlatformMetricsV2 } from '@/core/market/platform-metrics';
import { marketLimits } from '@/page-modules/token/market-session';
import { ShellFooter } from '@/shell/platform-metrics';
import { defaultSettings, type Density } from './column-settings';
import { ExploreTable } from './explore-table';
import { ExploreView } from './explore-view';
import { PresetTabs } from './preset-tabs';
import { buildExploreRow } from './use-explore-rows';

const signedOut = createConfig({
  chains: [paxeer],
  connectors: [],
  transports: { [paxeer.id]: http('http://127.0.0.1:1') },
  multiInjectedProviderDiscovery: false,
  ssr: true,
});
const figures: PlatformMetricsV2 = {
  window: '24h',
  volumeUsdlRaw: '2000000',
  totalMarketCapUsdlRaw: '1000000',
  trades: '2',
  activeWallets: '1',
  pools: '1',
  newPools: '1',
  generatedAt: '2026-10-12T15:00:01.000Z',
};
const rows = Array.from({ length: marketLimits.rankingPage }, (_, index) =>
  buildExploreRow({
    poolAddress: `0x${(index + 1).toString(16).padStart(40, '0')}`,
    rank: index + 1,
    stats: null,
    metadata: null,
    now: 0,
  }),
);

function markup(content: React.ReactNode, loaded: boolean, pathname = '/explore') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (loaded) client.setQueryData(queryKeys.platformStats(), figures);
  const html = renderToStaticMarkup(
    <WagmiProvider config={signedOut}>
      <QueryClientProvider client={client}>
        <PathnameContext.Provider value={pathname}>
          <div data-content>{content}</div>
        </PathnameContext.Provider>
        <ShellFooter />
      </QueryClientProvider>
    </WagmiProvider>,
  );
  client.clear();
  return html;
}

function table(density: Density, loading: boolean, tableRows = rows) {
  return (
    <ExploreTable
      rows={loading ? [] : tableRows}
      loading={loading}
      columns={defaultSettings.columns}
      density={density}
      sort={null}
      onSort={() => undefined}
      isStarred={() => false}
      onToggleStar={() => undefined}
      hasMore={false}
      loadingMore={false}
      moreError={false}
      onLoadMore={() => undefined}
      empty={null}
      disclosure={null}
      caption="New coins"
    />
  );
}

async function waitingFormsReserveLoadedBlockSizes() {
  const loadedPage = (
    <div className="mx-auto flex w-full max-w-content-max-width flex-col gap-4 px-4 py-4 md:px-6">
      <h1 className="text-lg font-medium text-text">Explore coins</h1>
      <ExploreView
        rows={rows}
        loading={false}
        hasMore={false}
        loadingMore={false}
        moreError={null}
        onLoadMore={() => undefined}
        onRetryMore={() => undefined}
        defaultSort={null}
        sortResetKey="movers"
        sourceLabel="coins"
        leading={<PresetTabs active="movers" />}
        empty={null}
        caption="Movers coins"
      />
    </div>
  );
  const pairs = [
    ...['/new', '/explore'].map((pathname) => ({
      pending: markup(<Loading />, false, pathname),
      loaded: markup(loadedPage, true, pathname),
    })),
    ...(['dense', 'default', 'comfortable'] as const).map((density) => ({
      pending: markup(table(density, true), false),
      loaded: markup(table(density, false), true),
    })),
  ];
  const require = createRequire(import.meta.url);
  const nodeEntry = createRequire(require.resolve('@tailwindcss/postcss')).resolve(
    '@tailwindcss/node',
  );
  const { compile } = (await import(pathToFileURL(nodeEntry).href)) as {
    compile(
      css: string,
      options: { base: string; onDependency(path: string): void },
    ): Promise<{ build(candidates: string[]): string }>;
  };
  const appDirectory = join(process.cwd(), 'src/app');
  const compiler = await compile(readFileSync(join(appDirectory, 'globals.css'), 'utf8'), {
    base: appDirectory,
    onDependency() {},
  });
  const candidates = [
    ...pairs
      .flatMap((pair) => [pair.pending, pair.loaded])
      .join('')
      .matchAll(/class="([^"]+)"/g),
  ].flatMap((match) => match[1].replaceAll('&#x27;', "'").replaceAll('&amp;', '&').split(/\s+/));
  const css = compiler.build(candidates);
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage();
    const sizes = async (html: string) => {
      await page.setContent(
        `<!doctype html><html><head><style>${css}</style></head><body>${html}</body></html>`,
      );
      return page.evaluate(() => {
        const height = (selector: string) =>
          document.querySelector(selector)!.getBoundingClientRect().height;
        return {
          content: height('[data-content]'),
          table: height('table'),
          body: height('tbody'),
          row: height('tbody tr'),
          footer: height('footer'),
          rows: document.querySelectorAll('tbody tr').length,
          figures: [...document.querySelectorAll('footer dd')].map(
            (element) => element.getBoundingClientRect().width,
          ),
        };
      });
    };
    for (const width of [360, 390, 430, 768, 834, 1024, 1280, 1440, 1920, 2560, 844]) {
      await page.setViewportSize({ width, height: width === 844 ? 390 : 900 });
      for (const pair of pairs) {
        const pending = await sizes(pair.pending);
        const loaded = await sizes(pair.loaded);
        expect(pending.rows).toBe(marketLimits.rankingPage);
        expect(loaded.rows).toBe(marketLimits.rankingPage);
        expect(pending.row).toBe(loaded.row);
        expect(pending.body).toBe(loaded.body);
        expect(pending.table).toBe(loaded.table);
        expect(pending.content).toBe(loaded.content);
        expect(pending.footer).toBe(loaded.footer);
        expect(pending.figures).toEqual(loaded.figures);
      }
      const pending = await sizes(markup(table('default', true), false));
      const loaded = await sizes(markup(table('default', false, rows.slice(0, 1)), true));
      expect(pending.table).toBeGreaterThanOrEqual(loaded.table);
    }
  } finally {
    await browser.close();
  }
}

it(
  'waiting forms reserve loaded block sizes across device widths',
  waitingFormsReserveLoadedBlockSizes,
  60_000,
);
