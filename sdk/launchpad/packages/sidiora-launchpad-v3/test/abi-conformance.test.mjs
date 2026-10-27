import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { factoryAbi, routerAbi, quoterAbi, protocolConfigAbi, erc20Abi } from '../dist/abis.js';

function canonicalInputType(input) {
  if (input.type.startsWith('tuple')) {
    return `(${input.components.map(canonicalInputType).join(',')})${input.type.slice(5)}`;
  }
  return input.type;
}

function abiIdentities(abi) {
  return new Set(
    abi.map(entry =>
      `${entry.type}:${entry.name ?? ''}(${entry.inputs.map(canonicalInputType).join(',')})`,
    ),
  );
}

function sidioraAbisMatchPrecompileRoleAbis() {
  const repositoryRoot = new URL('../../../../../', import.meta.url);
  const comparisons = [
    ['factoryAbi', factoryAbi, 'precompiles/launchpad/abi/SidioraFactory.json'],
    ['routerAbi', routerAbi, 'precompiles/launchpad/abi/Router.json'],
    ['quoterAbi', quoterAbi, 'precompiles/launchpad/abi/Quoter.json'],
    ['protocolConfigAbi', protocolConfigAbi, 'precompiles/launchpad/abi/ProtocolConfig.json'],
    ['erc20Abi', erc20Abi, 'precompiles/launchpad/abi/SidioraERC20.json'],
    ['routerAbi', routerAbi, 'apps/launchpad/web/src/core/network/abis/Router.json'],
    ['quoterAbi', quoterAbi, 'apps/launchpad/web/src/core/network/abis/Quoter.json'],
  ];

  for (const [name, abi, path] of comparisons) {
    const roleAbi = JSON.parse(readFileSync(new URL(path, repositoryRoot), 'utf8'));
    assert.deepEqual(abiIdentities(abi), abiIdentities(roleAbi), `${name} must match ${path}`);
  }
}

test(sidioraAbisMatchPrecompileRoleAbis);
