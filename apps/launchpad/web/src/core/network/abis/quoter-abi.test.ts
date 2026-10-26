import { expect, test } from 'vitest';
import precompileQuoterAbi from '../../../../../../../precompiles/launchpad/abi/Quoter.json';
import webQuoterAbi from './Quoter.json';

function webQuoterAbiMatchesPrecompileRole() {
  const identity = (entry: (typeof precompileQuoterAbi)[number]) =>
    `${entry.type}:${entry.name ?? ''}(${entry.inputs.map((input) => input.type).join(',')})`;

  expect(new Set(webQuoterAbi.map(identity))).toEqual(
    new Set(precompileQuoterAbi.map(identity)),
  );
}

test(webQuoterAbiMatchesPrecompileRole);
