import { expect, test } from 'vitest';
import precompileRouterAbi from '../../../../../../../precompiles/launchpad/abi/Router.json';
import webRouterAbi from './Router.json';
import { canonicalAbi } from './canonical-abi';

export function webRouterAbiMatchesPrecompileRole() {
  expect(canonicalAbi(webRouterAbi)).toEqual(canonicalAbi(precompileRouterAbi));
}

test(webRouterAbiMatchesPrecompileRole);
