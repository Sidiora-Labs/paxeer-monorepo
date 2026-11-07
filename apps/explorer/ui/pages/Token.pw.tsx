import BigNumber from 'bignumber.js';
import React from 'react';

import config from 'configs/app';
import * as verifiedAddressesMocks from 'mocks/account/verifiedAddresses';
import { token as contract } from 'mocks/address/address';
import { tokenInfo, tokenCounters, bridgedTokenA } from 'mocks/tokens/tokenInfo';
import { ENVS_MAP } from 'playwright/fixtures/mockEnvs';
import * as socketServer from 'playwright/fixtures/socketServer';
import { test, expect, devices } from 'playwright/lib';
import * as pwConfig from 'playwright/utils/config';

import Token from './Token';

const hash = tokenInfo.address_hash;
const chainId = config.chain.id;

const hooksConfig = {
  router: {
    query: { hash, tab: 'metadata' },
    isReady: true,
  },
};

// FIXME
// test cases which use socket cannot run in parallel since the socket server always run on the same port
test.describe.configure({ mode: 'serial' });

test.beforeEach(async({ mockApiResponse, mockTextAd }) => {
  await mockApiResponse('general:token', tokenInfo, { pathParams: { hash } });
  await mockApiResponse('general:address', contract, { pathParams: { hash } });
  await mockApiResponse('general:token_counters', tokenCounters, { pathParams: { hash } });
  await mockApiResponse('general:token_transfers', { items: [], next_page_params: null }, { pathParams: { hash } });
  await mockTextAd();
});

test('base view +@dark-mode', async({ render, page, createSocket }) => {
  const component = await render(<Token/>, { hooksConfig }, { withSocket: true });

  const socket = await createSocket();
  await socketServer.joinChannel(socket, `tokens:${ hash }`);

  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});

test('with verified info +@dark-mode', async({ render, page, createSocket, mockApiResponse, mockAssetResponse }) => {
  test.slow();
  await mockApiResponse('contractInfo:token_verified_info', verifiedAddressesMocks.TOKEN_INFO_APPLICATION.APPROVED, { pathParams: { chainId, hash } });
  await mockAssetResponse(tokenInfo.icon_url as string, './playwright/mocks/image_s.jpg');

  const component = await render(<Token/>, { hooksConfig }, { withSocket: true });

  const socket = await createSocket();
  const channel = await socketServer.joinChannel(socket, `tokens:${ hash }`);
  socket.send(JSON.stringify([ ...channel, 'total_supply', { total_supply: '100000000000000000000' } ]));
  await component.getByText('100 ARIA').waitFor({ state: 'visible', timeout: 10_000 });

  await page.getByLabel('Show info').click();

  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});

test('bridged token +@dark-mode', async({ render, page, createSocket, mockApiResponse, mockAssetResponse, mockEnvs }) => {
  const hash = bridgedTokenA.address_hash;
  const hooksConfig = {
    router: {
      query: { hash, tab: 'metadata' },
    },
  };

  await mockEnvs(ENVS_MAP.bridgedTokens);
  await mockApiResponse('general:token', bridgedTokenA, { pathParams: { hash } });
  await mockApiResponse('general:address', contract, { pathParams: { hash } });
  await mockApiResponse('general:token_counters', tokenCounters, { pathParams: { hash } });
  await mockApiResponse('general:token_transfers', { items: [], next_page_params: null }, { pathParams: { hash } });
  await mockApiResponse('contractInfo:token_verified_info', verifiedAddressesMocks.TOKEN_INFO_APPLICATION.APPROVED, { pathParams: { chainId, hash } });
  await mockAssetResponse(tokenInfo.icon_url as string, './playwright/mocks/image_s.jpg');

  const component = await render(<Token/>, { hooksConfig }, { withSocket: true });
  const socket = await createSocket();
  await socketServer.joinChannel(socket, `tokens:${ hash.toLowerCase() }`);
  await component.getByText('369,000,000 HyFi').waitFor({ state: 'visible' });

  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});

test('scam token +@dark-mode', async({ render, page, createSocket, mockApiResponse, mockEnvs }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_VIEWS_TOKEN_SCAM_TOGGLE_ENABLED', 'true' ],
  ]);
  await mockApiResponse('general:token', { ...tokenInfo, reputation: 'scam' }, { pathParams: { hash } });
  const component = await render(<Token/>, { hooksConfig }, { withSocket: true });

  const socket = await createSocket();
  await socketServer.joinChannel(socket, `tokens:${ hash }`);

  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});

test.describe('mobile', () => {
  test.use({ viewport: devices['iPhone 13 Pro'].viewport });

  test('base view +@dark-mode', async({ render, page, createSocket }) => {
    test.slow();
    const component = await render(<Token/>, { hooksConfig }, { withSocket: true });
    const socket = await createSocket();
    const channel = await socketServer.joinChannel(socket, `tokens:${ hash }`);
    socket.send(JSON.stringify([ ...channel, 'total_supply', { total_supply: '100000000000000000000' } ]));
    await component.getByText('100 ARIA').waitFor({ state: 'visible', timeout: 10_000 });

    await expect(component).toHaveScreenshot({
      mask: [ page.locator(pwConfig.adsBannerSelector) ],
      maskColor: pwConfig.maskColor,
    });
  });

  test('with verified info +@dark-mode', async({ render, page, createSocket, mockApiResponse, mockAssetResponse }) => {
    test.slow();
    await mockApiResponse('contractInfo:token_verified_info', verifiedAddressesMocks.TOKEN_INFO_APPLICATION.APPROVED, { pathParams: { chainId, hash } });
    await mockAssetResponse(tokenInfo.icon_url as string, './playwright/mocks/image_s.jpg');

    const component = await render(<Token/>, { hooksConfig }, { withSocket: true });
    const socket = await createSocket();
    const channel = await socketServer.joinChannel(socket, `tokens:${ hash }`);
    socket.send(JSON.stringify([ ...channel, 'total_supply', { total_supply: '100000000000000000000' } ]));
    await component.getByText('100 ARIA').waitFor({ state: 'visible', timeout: 10_000 });

    await expect(component).toHaveScreenshot({
      mask: [ page.locator(pwConfig.adsBannerSelector) ],
      maskColor: pwConfig.maskColor,
    });
  });
});

test('decimal string supplies preserve precision and zero +@dark-mode', async({ render, page, createSocket }) => {
  const component = await render(<Token/>, { hooksConfig }, { withSocket: true });
  const socket = await createSocket();
  const channel = await socketServer.joinChannel(socket, `tokens:${ hash }`);
  for (const totalSupply of [ '9007199254740993000000000000000001', '0' ]) {
    socket.send(JSON.stringify([ ...channel, 'total_supply', { total_supply: totalSupply } ]));
    const expected = new BigNumber(totalSupply).shiftedBy(-Number(tokenInfo.decimals)).dp(3).toFormat();
    await expect(component.locator('[data-token-card="TOKEN INFO"]')).toContainText(`${ expected } ARIA`);
    if (totalSupply !== '0') {
      await component.getByText(expected, { exact: true }).hover();
      await expect(page.getByRole('tooltip')).toContainText(new BigNumber(totalSupply).shiftedBy(-Number(tokenInfo.decimals)).toFormat());
    }
  }
});

test('markets without a rate +@mobile +@dark-mode', async({ render, mockApiResponse, page }) => {
  await mockApiResponse('general:token', { ...tokenInfo, exchange_rate: null, circulating_market_cap: null }, { pathParams: { hash } });
  const component = await render(<Token/>, { hooksConfig: { router: { query: { hash, tab: 'markets' }, isReady: true } } });
  await expect(component.locator('[data-stat-tile="Price"] [data-stat-value]')).toHaveText('—');
  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});
