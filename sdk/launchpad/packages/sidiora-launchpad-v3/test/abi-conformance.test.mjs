import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { toFunctionSelector } from 'viem';
import { factoryAbi, routerAbi, quoterAbi, protocolConfigAbi, erc20Abi } from '../src/abis.ts';

function canonicalInputType(input) {
  if (input.type.startsWith('tuple')) {
    return `(${input.components.map(canonicalInputType).join(',')})${input.type.slice(5)}`;
  }
  return input.type;
}

export function abiIdentities(abi) {
  return new Set(
    abi.map(entry =>
      `${entry.type}:${entry.name ?? ''}(${(entry.inputs ?? []).map(canonicalInputType).join(',')})`,
    ),
  );
}

function canonicalParameter(parameter) {
  const result = { name: parameter.name, type: parameter.type };
  if ('indexed' in parameter) result.indexed = parameter.indexed;
  if ('components' in parameter) result.components = parameter.components.map(canonicalParameter);
  return result;
}

export function canonicalAbiEntries(abi) {
  return abi.map(entry => {
    const result = { type: entry.type };
    if ('name' in entry) result.name = entry.name;
    if ('inputs' in entry) result.inputs = entry.inputs.map(canonicalParameter);
    if ('outputs' in entry) result.outputs = entry.outputs.map(canonicalParameter);
    if ('anonymous' in entry) result.anonymous = entry.anonymous;
    if ('stateMutability' in entry) result.stateMutability = entry.stateMutability;
    return JSON.stringify(result);
  }).sort();
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
    assert.deepEqual(canonicalAbiEntries(abi), canonicalAbiEntries(roleAbi), `${name}: complete ABI must match ${path}`);
    assert.deepEqual(
      abi.filter(entry => entry.type === 'function').map(toFunctionSelector).sort(),
      roleAbi.filter(entry => entry.type === 'function').map(toFunctionSelector).sort(),
      `${name}: function selectors must match ${path}`,
    );
  }
}

test(sidioraAbisMatchPrecompileRoleAbis);
