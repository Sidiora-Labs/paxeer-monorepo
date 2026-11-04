import { test } from 'vitest';
import { assertHealthRpcChainReadiness } from './health-test-fixture';

async function health_wrong_chain_degrades_readiness() {
  await assertHealthRpcChainReadiness([
    { chainId: '0x7d', status: 'ok' },
    { chainId: '0x1', status: 'degraded' },
    { chainId: '0x89', status: 'degraded' },
    { chainId: '0x1000000000000007d', status: 'degraded' },
    { chainId: '0X7D', status: 'ok' },
    { chainId: '125', status: 'degraded' },
    { chainId: 125, status: 'degraded' },
    { chainId: null, status: 'degraded' },
    { chainId: '0x7d', status: 'ok' },
  ]);
}

test(health_wrong_chain_degrades_readiness, health_wrong_chain_degrades_readiness);
