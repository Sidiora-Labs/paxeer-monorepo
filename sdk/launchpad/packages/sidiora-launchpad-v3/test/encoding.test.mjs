import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  decodeEventLog, decodeFunctionData, decodeFunctionResult, encodeAbiParameters,
  encodeEventTopics, encodeFunctionData, encodeFunctionResult, getAddress,
  maxUint256, zeroAddress,
} from 'viem';
import { erc20Abi, factoryAbi, protocolConfigAbi, quoterAbi, routerAbi } from '../dist/index.js';

const repositoryRoot = new URL('../../../../../', import.meta.url);
const document = JSON.parse(readFileSync(
  new URL('sdk/launchpad/shared/config/deployment/current.json', repositoryRoot), 'utf8',
));
const factory = getAddress(document.roles.SidioraFactory.address);
const router = getAddress(document.roles.Router.address);
const usdl = getAddress(document.roles.USDL.address);

function roleAbi(role) {
  return JSON.parse(readFileSync(new URL(`precompiles/launchpad/abi/${role}.json`, repositoryRoot), 'utf8'));
}

function contractCallsPreserveRawAmountsAddressesAndPermitHashes() {
  const permit = {
    value: maxUint256, deadline: 1_900_000_001n, v: 28,
    r: document.roles.Router.runtimeHash, s: document.roles.USDL.runtimeHash,
  };
  const calls = [
    [factoryAbi, 'SidioraFactory', 'createMarket', ['Market Δ', 'MKT', 3, zeroAddress]],
    [factoryAbi, 'SidioraFactory', 'createMarketFor', [factory, 'Market Δ', 'MKT', 3, router]],
    [factoryAbi, 'SidioraFactory', 'predictPoolAddress', [usdl, factory, zeroAddress]],
    [routerAbi, 'Router', 'buy', [factory, BigInt(document.creationFeeRaw), 9_007_199_254_740_993n, 1_900_000_000n]],
    [routerAbi, 'Router', 'sellWithPermit', [factory, maxUint256, 1n, 1_900_000_000n, permit]],
    [routerAbi, 'Router', 'swapTokenForTokenWithPermit', [factory, usdl, 1n, 0n, 1_900_000_000n, permit]],
    [quoterAbi, 'Quoter', 'quoteExactInput', [factory, maxUint256, false]],
    [protocolConfigAbi, 'ProtocolConfig', 'setCreationFee', [BigInt(document.creationFeeRaw)]],
    [protocolConfigAbi, 'ProtocolConfig', 'grantRole', [permit.r, router]],
    [erc20Abi, 'SidioraERC20', 'approve', [router, maxUint256]],
    [erc20Abi, 'SidioraERC20', 'transfer', [factory, 5_000_000n]],
    [erc20Abi, 'SidioraERC20', 'transferFrom', [factory, router, 25_000_000n]],
    [erc20Abi, 'SidioraERC20', 'permit', [factory, router, maxUint256, permit.deadline, permit.v, permit.r, permit.s]],
  ];
  for (const [abi, role, functionName, args] of calls) {
    const authoritative = roleAbi(role);
    const data = encodeFunctionData({ abi, functionName, args });
    assert.equal(data, encodeFunctionData({ abi: authoritative, functionName, args }), `${role}.${functionName}`);
    assert.deepEqual(decodeFunctionData({ abi: authoritative, data }), { functionName, args });
  }
}

function contractResultsPreserveTupleNamesAndIntegerPrecision() {
  const cases = [
    [factoryAbi, 'SidioraFactory', 'createMarket', [usdl, router, maxUint256]],
    [quoterAbi, 'Quoter', 'quoteExactInput', { amountOut: maxUint256, feeAmount: 1n, priceImpactBps: 2n }],
    [quoterAbi, 'Quoter', 'quoteMultihop', {
      amountOut: maxUint256, intermediateUsdl: 9_007_199_254_740_993n,
      sellFeeAmount: 3n, buyFeeAmount: 4n, sellPriceImpactBps: 5n,
      buyPriceImpactBps: 6n, combinedPriceImpactBps: 7n, poolA: factory, poolB: router,
    }],
    [protocolConfigAbi, 'ProtocolConfig', 'creationFee', BigInt(document.creationFeeRaw)],
    [erc20Abi, 'SidioraERC20', 'balanceOf', maxUint256],
  ];
  for (const [abi, role, functionName, result] of cases) {
    const data = encodeFunctionResult({ abi: roleAbi(role), functionName, result });
    assert.deepEqual(decodeFunctionResult({ abi, functionName, data }), result, `${role}.${functionName}`);
  }
}

function contractEventsPreserveIndexedAddressesAndHashes() {
  const cases = [
    [factoryAbi, 'SidioraFactory', 'MarketCreated', { token: usdl, pool: router, creator: factory, nftId: maxUint256, optical: zeroAddress }],
    [routerAbi, 'Router', 'Buy', { pool: factory, buyer: router, usdlIn: 1_000_000n, tokensOut: maxUint256 }],
    [quoterAbi, 'Quoter', 'RoleGranted', { role: document.roles.Quoter.runtimeHash, account: router, sender: factory }],
    [protocolConfigAbi, 'ProtocolConfig', 'ConfigUpdated', { key: document.roles.ProtocolConfig.runtimeHash, oldValue: 1n, newValue: maxUint256 }],
    [erc20Abi, 'SidioraERC20', 'Transfer', { from: factory, to: router, value: maxUint256 }],
  ];
  for (const [abi, role, eventName, args] of cases) {
    const authoritative = roleAbi(role);
    const event = authoritative.find(entry => entry.type === 'event' && entry.name === eventName);
    const parameters = event.inputs.filter(input => !input.indexed);
    const data = encodeAbiParameters(parameters, parameters.map(input => args[input.name]));
    const topics = encodeEventTopics({ abi: authoritative, eventName, args });
    assert.deepEqual(decodeEventLog({ abi, eventName, data, topics }), { eventName, args });
  }
}

test(contractCallsPreserveRawAmountsAddressesAndPermitHashes);
test(contractResultsPreserveTupleNamesAndIntegerPrecision);
test(contractEventsPreserveIndexedAddressesAndHashes);
