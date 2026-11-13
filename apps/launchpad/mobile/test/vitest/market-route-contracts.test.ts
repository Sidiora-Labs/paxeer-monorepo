import { test } from 'vitest';
import { marketSessionUsesV2Snapshots } from './market-session-proof.ts';
import { routeModulesPreserveHeadPaths } from './routes-scaffold-proof.ts';
import { routeQuoteUsesChain125Interface } from './routes-quote-proof.ts';

async function mobileMarketTopicTest() {
  await marketSessionUsesV2Snapshots();
}

async function mobileRouteScaffoldDrift() {
  await routeModulesPreserveHeadPaths();
}

async function mobileRouteOnchainQuote() {
  await routeQuoteUsesChain125Interface();
}

test(mobileMarketTopicTest, mobileMarketTopicTest);
test(mobileRouteScaffoldDrift, mobileRouteScaffoldDrift);
test(mobileRouteOnchainQuote, mobileRouteOnchainQuote);
