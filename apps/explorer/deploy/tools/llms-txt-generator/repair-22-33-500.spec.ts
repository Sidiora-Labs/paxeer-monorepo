import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { match } from 'path-to-regexp';

import { expect, test } from 'vitest';

import config from '../../../configs/app';
import buildUrl from '../../../lib/api/buildUrl';

import { run } from './index';

const backendDirectory = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web');
const fixtureConfig: typeof config = {
  ...config,
  apis: {
    ...config.apis,
    general: {
      endpoint: 'https://explorer.example',
      basePath: '/tenant/ethereum',
      host: 'explorer.example',
      protocol: 'https',
      socketEndpoint: 'wss://explorer.example',
    },
  },
  features: {
    ...config.features,
    multichain: { title: 'Multichain explorer', isEnabled: false },
    beaconChain: {
      title: 'Beacon chain',
      isEnabled: true,
      currency: { symbol: 'ETH' },
      validatorUrlTemplate: undefined,
      withdrawalsOnly: false,
    },
  },
};

function generateBeaconDepositLink() {
  const directory = mkdtempSync(join(tmpdir(), 'llms-beacon-'));
  try {
    const destination = join(directory, 'llms.txt');
    run(fixtureConfig, destination);
    const document = readFileSync(destination, 'utf8');
    const section = document.match(/### Deposits by Block\s+```bash\s+curl --request GET --url '([^']+)'\s+```/);
    expect(section).not.toBeNull();
    if (!section) {
      throw new Error('Generated llms.txt has no block beacon deposit example');
    }
    return section[1];
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

function registeredBeaconDepositRoute() {
  const router = readFileSync(join(backendDirectory, 'routers/api_router.ex'), 'utf8');
  expect(router).toContain('scope "/v2", as: :api_v2 do');
  const blocksScope = router.slice(router.indexOf('scope "/blocks" do'), router.indexOf('scope "/addresses" do'));
  const registration = blocksScope.match(/if @chain_type == :ethereum do\s+get\("([^"]+)", V2\.BlockController, :beacon_deposits\)/);
  expect(registration).not.toBeNull();
  if (!registration) {
    throw new Error('Backend has no registered block beacon deposit handler');
  }
  return `/api/v2/blocks${ registration[1] }`;
}

function llmsBeaconLinkMatchesRegisteredHandler() {
  const registeredPath = registeredBeaconDepositRoute();
  expect(registeredPath).toBe('/api/v2/blocks/:block_hash_or_number_param/beacon/deposits');
  const controller = readFileSync(join(backendDirectory, 'controllers/api/v2/block_controller.ex'), 'utf8');
  expect(controller).toContain('def beacon_deposits(conn, %{block_hash_or_number_param: block_hash_or_number} = params) do');
  expect(controller).toContain('with {:ok, block} <- block_param_to_block(block_hash_or_number) do');
  const documentedPath = registeredPath.replace(':block_hash_or_number_param', '{block_hash_or_number_param}');
  expect(generateBeaconDepositLink()).toBe(`https://explorer.example/tenant/ethereum${ documentedPath }`);
}

function llmsBeaconLinkSupportsBlockNumberAndHash() {
  const link = generateBeaconDepositLink();
  const matchRoute = match(registeredBeaconDepositRoute());
  for (const identifier of [ '123456', `0x${ 'ab'.repeat(32) }` ]) {
    const url = link.replace('{block_hash_or_number_param}', identifier);
    expect(url).toBe(buildUrl('general:block_deposits', { height_or_hash: identifier }, undefined, true, {
      id: '1',
      slug: 'ethereum',
      name: 'Ethereum',
      app_config: fixtureConfig,
    }));
    const route = matchRoute(new URL(url).pathname.slice('/tenant/ethereum'.length));
    expect(route).not.toBe(false);
    if (!route) {
      throw new Error('Generated beacon deposit request does not match the registered route');
    }
    expect(route.params).toEqual({ block_hash_or_number_param: identifier });
  }
}

test(llmsBeaconLinkMatchesRegisteredHandler);
test(llmsBeaconLinkSupportsBlockNumberAndHash);
