import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve as resolvePath } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Worker } from 'node:worker_threads';
import { delay } from 'es-toolkit';

import type { ClusterChainConfig } from 'types/multichain';
import type appConfig from 'configs/app';

const currentFilePath = fileURLToPath(import.meta.url);
const currentDir = dirname(currentFilePath);

// const EXPLORER_URLS = [
//   'https://optimism.blockscout.com',
//   'https://base.blockscout.com',
//   'https://arbitrum.blockscout.com',
//   'https://unichain.blockscout.com',
//   'https://explorer.redstone.xyz',
//   'https://matchscan.io'
//   // 'https://optimism-interop-alpha-0.blockscout.com',
//   // 'https://optimism-interop-alpha-1.blockscout.com',
// ];

function getSlug(chainName: string) {
  return chainName.toLowerCase().replace(/ /g, '-').replace(/[^a-z0-9-]/g, '');
}

async function getChainscoutInfo(chainIds: Array<string>) {
  const controller = new AbortController();
  const timeout = setTimeout(() => {
    controller.abort(`Request to Chainscout API timed out`);
  }, 30_000);

  try {
    const response = await fetch(`https://chains.blockscout.com/api/chains?chain_ids=${ chainIds.join(',') }`, { signal: controller.signal });
    if (!response.ok) {
      throw new Error(`Failed to fetch chains info from Chainscout API`);
    }
    const chainsInfo = await response.json() as Record<string, { explorers: [ { url: string } ], logo: string }>;
  
    return chainIds.map((chainId) => ({
      id: chainId,
      logoUrl: chainsInfo[chainId]?.logo,
    }))
  } catch (error) {
    throw error;
  } finally {
    clearTimeout(timeout);
  }
}

async function computeChainConfig(url: string): Promise<unknown> {
  const workerPath = resolvePath(currentDir, 'worker.js');

  const worker = new Worker(workerPath, {
    workerData: { url },
    env: {} // Start with empty environment
  });
  const controller = new AbortController();

  const configPromise = new Promise((resolve, reject) => {
    worker.on('message', (config) => {
      resolve(config);
    });

    worker.on('error', (error) => {
      console.error('Worker error:', error);
      reject(error);
    });

    worker.on('exit', (code) => {
      controller.abort();
      reject(new Error(`Worker stopped with exit code ${ code }`));
    });

  });

  return Promise.race([
    configPromise,
    delay(30_000, { signal: controller.signal })
  ]).finally(() => {
    worker.terminate();
  })
}

export const EXPLORER_REQUEST_TIMEOUT_MS = 30_000;

export async function getExplorerUrls(url: URL, timeoutMs = EXPLORER_REQUEST_TIMEOUT_MS): Promise<Array<string>> {
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    throw new Error('Multichain request timeout must be a positive finite number.');
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => {
    controller.abort(new Error(`Request to multichain aggregator API timed out after ${ timeoutMs }ms`));
  }, timeoutMs);

  try {
    const response = await fetch(url.toString(), { signal: controller.signal });
    if (!response.ok) {
      throw new Error(`Failed to fetch chains from multichain aggregator API: HTTP ${ response.status }`);
    }

    const data: unknown = await response.json();
    if (typeof data !== 'object' || data === null || !('items' in data) || !Array.isArray(data.items)) {
      throw new Error('Invalid multichain aggregator response: expected an items array.');
    }

    const explorerUrls = data.items.map((item: unknown) => {
      if (typeof item !== 'object' || item === null || !('explorer_url' in item) || typeof item.explorer_url !== 'string') {
        throw new Error('Invalid multichain aggregator response: expected an explorer_url string for every chain.');
      }

      const explorerUrl = new URL(item.explorer_url);
      if (explorerUrl.protocol !== 'http:' && explorerUrl.protocol !== 'https:') {
        throw new Error('Invalid multichain aggregator response: explorer_url must use HTTP or HTTPS.');
      }
      return item.explorer_url;
    });

    console.log(`💡 Found ${ data.items.length } chains in cluster.`);

    return explorerUrls;
  } catch (error) {
    if (controller.signal.aborted) {
      throw controller.signal.reason;
    }
    throw error;
  } finally {
    clearTimeout(timeout);
    controller.abort();
  }
}

interface GeneratorOptions {
  aggregatorHost?: string;
  cluster?: string;
  requestTimeoutMs?: number;
}

export async function run({
  aggregatorHost = process.env.NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST,
  cluster = process.env.NEXT_PUBLIC_MULTICHAIN_CLUSTER,
  requestTimeoutMs = EXPLORER_REQUEST_TIMEOUT_MS,
}: GeneratorOptions = {}) {
  try {
    if (!aggregatorHost) {
      console.log('ℹ️  NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST is not set, skipping multichain config generation\n');
      return;
    }

    console.log('🌀 Generating multichain config...');

    const basePath = `/api/v1/clusters/${ cluster }/chains`;
    const explorerUrls = await getExplorerUrls(new URL(basePath, aggregatorHost), requestTimeoutMs);

    if (!explorerUrls.length) {
      throw new Error('No chains found in the cluster.');
    }

    const configs: Array<typeof appConfig> = [];
    for (const url of explorerUrls) {
      const chainConfig = (await computeChainConfig(url)) as typeof appConfig | undefined;
      if (!chainConfig) {
        throw new Error(`Failed to fetch chain config for ${ url }`);
      }
      configs.push(chainConfig);
    }

    const chainscoutInfo = await getChainscoutInfo(
      configs
      .map((config) => config.chain.id)
      .filter((chainId) => chainId !== undefined)
    );

    const config = {
      chains: configs.map((config, index) => {
        const chainId = config.chain.id;
        const chainName = (config as { chain: { name: string } })?.chain?.name ?? `Chain ${ chainId }`;
        return {
          id: chainId || '',
          name: chainName,
          logo: chainscoutInfo.find((chain) => chain.id === chainId)?.logoUrl,
          explorer_url: explorerUrls[index],
          slug: getSlug(chainName),
          app_config: config,
        } satisfies ClusterChainConfig;
      }),
    };

    const outputDir = resolvePath(currentDir, '../../../../public/assets/multichain');
    mkdirSync(outputDir, { recursive: true });

    const outputPathJson = resolvePath(outputDir, 'config.json');
    writeFileSync(outputPathJson, JSON.stringify(config, null, 2));

    const outputPathJs = resolvePath(outputDir, 'config.js');
    writeFileSync(outputPathJs, `window.__multichainConfig = ${ JSON.stringify(config) };`);

    console.log('👍 Done!\n');
  } catch (error) {
    console.error('🚨 Error generating multichain config:', error);
    console.log('\n');
    throw error;
  }
}

if (process.argv[1] && resolvePath(process.argv[1]) === currentFilePath) {
  run().catch(() => {
    process.exitCode = 1;
  });
}
