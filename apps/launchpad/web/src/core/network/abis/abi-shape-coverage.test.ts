import { expect, test } from 'vitest';
import {
  decodeEventLog,
  decodeFunctionData,
  decodeFunctionResult,
  encodeAbiParameters,
  encodeEventTopics,
  encodeFunctionData,
  encodeFunctionResult,
} from 'viem';
import precompileFeesRouterAbi from '../../../../../../../precompiles/launchpad/abi/FeesRouter.json';
import precompileQuoterAbi from '../../../../../../../precompiles/launchpad/abi/Quoter.json';
import precompileRouterAbi from '../../../../../../../precompiles/launchpad/abi/Router.json';
import webFeesRouterAbi from './FeesRouter.json';
import webQuoterAbi from './Quoter.json';
import webRouterAbi from './Router.json';
import { canonicalAbi } from './canonical-abi';
import { webFeesRouterAbiMatchesPrecompileRole } from './feesrouter-abi.test';
import { webQuoterAbiMatchesPrecompileRole } from './quoter-abi.test';
import { webRouterAbiMatchesPrecompileRole } from './router-abi.test';

interface Parameter {
  name: string;
  type: string;
  internalType: string;
  indexed?: boolean;
  components?: Parameter[];
}

interface Entry {
  name?: string;
  type: string;
  inputs: Parameter[];
  outputs?: Parameter[];
  stateMutability?: string;
  anonymous?: boolean;
}

const pool = '0x1111111111111111111111111111111111111111';
const owner = '0x2222222222222222222222222222222222222222';
const amount = (1n << 200n) + 123n;

function entryNamed(abi: Entry[], name: string): Entry {
  const entry = abi.find((candidate) => candidate.name === name);
  if (!entry) throw new Error(`Missing ABI entry ${name}`);
  return entry;
}

function outputOf(abi: Entry[], name: string): Parameter {
  const output = entryNamed(abi, name).outputs?.[0];
  if (!output) throw new Error(`Missing ABI output ${name}`);
  return output;
}

function componentsOf(parameter: Parameter): Parameter[] {
  if (!parameter.components) throw new Error('Missing tuple components');
  return parameter.components;
}

function rejectsMutation(
  abi: Entry[],
  check: () => void,
  mutate: (entries: Entry[]) => void,
) {
  const original = structuredClone(abi);
  try {
    mutate(abi);
    expect(check).toThrow();
  } finally {
    abi.splice(0, abi.length, ...original);
  }
  check();
}

function rejectsSharedShapeDrift(abi: Entry[], check: () => void) {
  rejectsMutation(abi, check, (entries) => {
    entryNamed(entries, 'hasRole').stateMutability = 'nonpayable';
  });
  rejectsMutation(abi, check, (entries) => {
    entryNamed(entries, 'RoleGranted').inputs[0].indexed = false;
  });
  rejectsMutation(abi, check, (entries) => {
    entryNamed(entries, 'RoleGranted').anonymous = true;
  });
  rejectsMutation(abi, check, (entries) => {
    entries.push(structuredClone(entries[0]));
  });
  rejectsMutation(abi, check, (entries) => {
    const duplicates = entries
      .map((entry, index) => ({ entry, index }))
      .filter(({ entry }) => entry.name === 'InvalidImplementation');
    expect(duplicates).toHaveLength(2);
    entries.splice(duplicates[0].index, 1);
  });
  rejectsMutation(abi, check, (entries) => {
    entryNamed(entries, 'RoleGranted').inputs.reverse();
  });
  const original = structuredClone(abi);
  try {
    abi.reverse();
    for (const entry of abi) {
      const reordered = Object.fromEntries(Object.entries(entry).reverse());
      Object.keys(entry).forEach((key) => Reflect.deleteProperty(entry, key));
      Object.assign(entry, reordered);
    }
    check();
  } finally {
    abi.splice(0, abi.length, ...original);
  }
}

function repairFeesRouterAbiShapeCoverage() {
  rejectsMutation(webFeesRouterAbi, webFeesRouterAbiMatchesPrecompileRole, (abi) => {
    outputOf(abi, 'claimFees').type = 'address';
  });
  rejectsMutation(webFeesRouterAbi, webFeesRouterAbiMatchesPrecompileRole, (abi) => {
    outputOf(abi, 'claimFees').name = 'incorrectAmount';
  });
  rejectsMutation(webFeesRouterAbi, webFeesRouterAbiMatchesPrecompileRole, (abi) => {
    entryNamed(abi, 'claimFees').outputs = [];
  });
  rejectsSharedShapeDrift(webFeesRouterAbi, webFeesRouterAbiMatchesPrecompileRole);

  const data = encodeFunctionData({
    abi: webFeesRouterAbi,
    functionName: 'claimFees',
    args: [19n],
  });
  expect(decodeFunctionData({ abi: precompileFeesRouterAbi, data })).toEqual({
    functionName: 'claimFees',
    args: [19n],
  });
  const result = encodeFunctionResult({
    abi: precompileFeesRouterAbi,
    functionName: 'claimFees',
    result: amount,
  });
  expect(decodeFunctionResult({
    abi: webFeesRouterAbi,
    functionName: 'claimFees',
    data: result,
  })).toBe(amount);

  const [signature, ...indexedTopics] = encodeEventTopics({
    abi: precompileFeesRouterAbi,
    eventName: 'FeesClaimed',
    args: { nftId: 19n, owner },
  });
  if (typeof signature !== 'string') throw new Error('Missing event signature');
  const topics: [`0x${string}`, ...`0x${string}`[]] = [
    signature,
    ...indexedTopics.map((topic) => {
      if (typeof topic !== 'string') throw new Error('Expected concrete event topic');
      return topic;
    }),
  ];
  expect(decodeEventLog({
    abi: webFeesRouterAbi,
    eventName: 'FeesClaimed',
    topics,
    data: encodeAbiParameters([{ type: 'uint256' }], [amount]),
    strict: true,
  })).toEqual({ eventName: 'FeesClaimed', args: { nftId: 19n, owner, amount } });
}

function repairQuoterAbiShapeCoverage() {
  rejectsMutation(webQuoterAbi, webQuoterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(outputOf(abi, 'quoteExactInput'))[0].type = 'address';
  });
  rejectsMutation(webQuoterAbi, webQuoterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(outputOf(abi, 'quoteExactInput'))[0].name = 'incorrectAmount';
  });
  rejectsMutation(webQuoterAbi, webQuoterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(outputOf(abi, 'quoteExactInput')).pop();
  });
  rejectsMutation(webQuoterAbi, webQuoterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(outputOf(abi, 'quoteExactInput')).reverse();
  });
  rejectsMutation(webQuoterAbi, webQuoterAbiMatchesPrecompileRole, (abi) => {
    outputOf(abi, 'quoteExactInput').type = 'bytes';
  });
  rejectsSharedShapeDrift(webQuoterAbi, webQuoterAbiMatchesPrecompileRole);

  const args = [pool, 500n, true];
  const data = encodeFunctionData({ abi: webQuoterAbi, functionName: 'quoteExactInput', args });
  expect(decodeFunctionData({ abi: precompileQuoterAbi, data })).toEqual({
    functionName: 'quoteExactInput', args,
  });
  const quote = { amountOut: amount, feeAmount: 3n, priceImpactBps: 17n };
  const result = encodeFunctionResult({
    abi: precompileQuoterAbi, functionName: 'quoteExactInput', result: quote,
  });
  expect(decodeFunctionResult({
    abi: webQuoterAbi, functionName: 'quoteExactInput', data: result,
  })).toEqual(quote);
}

function repairRouterAbiShapeCoverage() {
  rejectsMutation(webRouterAbi, webRouterAbiMatchesPrecompileRole, (abi) => {
    outputOf(abi, 'buy').type = 'address';
  });
  rejectsMutation(webRouterAbi, webRouterAbiMatchesPrecompileRole, (abi) => {
    const permit = entryNamed(abi, 'buyWithPermit').inputs[4];
    componentsOf(permit)[2].type = 'uint256';
  });
  rejectsMutation(webRouterAbi, webRouterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(entryNamed(abi, 'buyWithPermit').inputs[4]).pop();
  });
  rejectsMutation(webRouterAbi, webRouterAbiMatchesPrecompileRole, (abi) => {
    componentsOf(entryNamed(abi, 'buyWithPermit').inputs[4]).reverse();
  });
  rejectsMutation(webRouterAbi, webRouterAbiMatchesPrecompileRole, (abi) => {
    const outputs = entryNamed(abi, 'swapTokenForToken').outputs;
    if (!outputs) throw new Error('Missing swap outputs');
    outputs.reverse();
  });
  rejectsSharedShapeDrift(webRouterAbi, webRouterAbiMatchesPrecompileRole);

  const permit = { value: amount, deadline: 900n, v: 27, r: `0x${'ab'.repeat(32)}`, s: `0x${'cd'.repeat(32)}` };
  const args = [pool, 500n, 400n, 900n, permit];
  const data = encodeFunctionData({ abi: webRouterAbi, functionName: 'buyWithPermit', args });
  expect(decodeFunctionData({ abi: precompileRouterAbi, data })).toEqual({
    functionName: 'buyWithPermit', args,
  });
  const result = encodeFunctionResult({
    abi: precompileRouterAbi, functionName: 'buyWithPermit', result: amount,
  });
  expect(decodeFunctionResult({
    abi: webRouterAbi, functionName: 'buyWithPermit', data: result,
  })).toBe(amount);
}

function canonicalAbiPreservesNestedArraysAndEntryMultiplicity() {
  const original = canonicalAbi(precompileQuoterAbi);
  const reordered = precompileQuoterAbi.map((entry) =>
    JSON.parse(JSON.stringify(entry), (_key, value: unknown) => {
      if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
        return Object.fromEntries(Object.entries(value).reverse());
      }
      return value;
    }),
  ).reverse();
  expect(canonicalAbi(reordered)).toEqual(original);
  expect(canonicalAbi([...precompileQuoterAbi, precompileQuoterAbi[0]])).not.toEqual(original);
}

test(repairFeesRouterAbiShapeCoverage);
test(repairQuoterAbiShapeCoverage);
test(repairRouterAbiShapeCoverage);
test(canonicalAbiPreservesNestedArraysAndEntryMultiplicity);
