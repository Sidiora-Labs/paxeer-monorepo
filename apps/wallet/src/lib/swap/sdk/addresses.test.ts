import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';
import { TOKENS as WALLET_TOKENS } from '@/lib/constants';
import { SIDIORA_CONTRACTS, TOKENS } from './addresses';

const SWAP_SDK_TOKENS = TOKENS;

function sidiora_contracts_are_the_generation_2_surfaces() {
  const source = readFileSync(
    resolve(__dirname, '../../../../../../modules/launchpad/keeper/surfaces.go'),
    'utf8'
  );
  const surfaces = /^var Generation2Surfaces = map\[string\]common\.Address\{([\s\S]*?)^\}/m.exec(
    source
  );
  expect(surfaces).not.toBeNull();

  for (const role of ['Router', 'Quoter', 'PoolRegistry'] as const) {
    const entry = new RegExp(
      `^\\s*types\\.Role${role}:\\s*common\\.HexToAddress\\("(0x[0-9a-fA-F]{40})"\\),\\s*$`,
      'm'
    ).exec(surfaces![1]);
    expect(entry, `Generation2Surfaces.Role${role}`).not.toBeNull();
    expect(SIDIORA_CONTRACTS[role].toLowerCase()).toBe(entry![1].toLowerCase());
  }
}

test(sidiora_contracts_are_the_generation_2_surfaces);

function wallet_token_registries_agree() {
  for (const symbol of ['USDC', 'USDT', 'USDL', 'WPAX9'] as const) {
    expect(WALLET_TOKENS[symbol].address?.toLowerCase(), symbol).toBe(
      SWAP_SDK_TOKENS[symbol].address.toLowerCase()
    );
  }

  const deployment = JSON.parse(
    readFileSync(
      resolve(
        __dirname,
        '../../../../../../contracts/launchpad/deployments/paxeer-x-network-addresses.json'
      ),
      'utf8'
    )
  ) as { _meta: { usdl: string } };
  const usdl = deployment._meta.usdl.toLowerCase();

  expect(usdl).toBe('0x85FcD13735F4309833A503EE804ea32395851479'.toLowerCase());
  expect(WALLET_TOKENS.USDL.address?.toLowerCase()).toBe(usdl);
  expect(SWAP_SDK_TOKENS.USDL.address.toLowerCase()).toBe(usdl);
}

test(wallet_token_registries_agree);
