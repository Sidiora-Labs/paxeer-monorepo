import { expect, test } from 'vitest';
import precompileQuoterAbi from '../../../../../../../precompiles/launchpad/abi/Quoter.json';
import webQuoterAbi from './Quoter.json';
import { canonicalAbi } from './canonical-abi';

export function webQuoterAbiMatchesPrecompileRole() {
  expect(canonicalAbi(webQuoterAbi)).toEqual(
    canonicalAbi(precompileQuoterAbi),
  );
}

test(webQuoterAbiMatchesPrecompileRole);
