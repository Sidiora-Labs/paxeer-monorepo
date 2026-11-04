import { expect, test } from 'vitest';
import precompileFeesRouterAbi from '../../../../../../../precompiles/launchpad/abi/FeesRouter.json';
import webFeesRouterAbi from './FeesRouter.json';
import { canonicalAbi } from './canonical-abi';

export function webFeesRouterAbiMatchesPrecompileRole() {
  expect(canonicalAbi(webFeesRouterAbi)).toEqual(
    canonicalAbi(precompileFeesRouterAbi),
  );
}

test(webFeesRouterAbiMatchesPrecompileRole);
