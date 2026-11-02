import { expect, test } from 'vitest';
import precompileRouterAbi from '../../../../../../../precompiles/launchpad/abi/Router.json';
import webRouterAbi from './Router.json';

function abiIdentity(entry: {
  type: string;
  name?: string;
  inputs: { type: string }[];
}): string {
  return `${entry.type}:${entry.name ?? ''}(${entry.inputs.map((input) => input.type).join(',')})`;
}

function webRouterAbiMatchesPrecompileRole() {
  const webIdentities = new Set(webRouterAbi.map(abiIdentity));
  const precompileIdentities = new Set(precompileRouterAbi.map(abiIdentity));

  expect(webIdentities).toEqual(precompileIdentities);
}

test(webRouterAbiMatchesPrecompileRole);
