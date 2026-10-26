import { expect, test } from 'vitest';
import precompileFeesRouterAbi from '../../../../../../../precompiles/launchpad/abi/FeesRouter.json';
import webFeesRouterAbi from './FeesRouter.json';

function webFeesRouterAbiMatchesPrecompileRole() {
  const identity = (entry: (typeof precompileFeesRouterAbi)[number]) =>
    `${entry.type}:${entry.name ?? ''}(${entry.inputs.map(input => input.type).join(',')})`;

  expect(new Set(webFeesRouterAbi.map(identity))).toEqual(
    new Set(precompileFeesRouterAbi.map(identity)),
  );
}

test(webFeesRouterAbiMatchesPrecompileRole);
