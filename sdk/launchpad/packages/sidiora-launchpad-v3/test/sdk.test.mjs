import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { decodeFunctionData, encodeFunctionData, zeroAddress } from 'viem';
import { factoryAbi, routerAbi, quoterAbi, protocolConfigAbi, validateDeployment } from '../dist/index.js';
import { erc20Abi } from '../dist/abis.js';

function erc20AbiMatchesSidioraErc20() {
  const sidioraErc20 = JSON.parse(
    readFileSync(
      new URL('../../../../../precompiles/launchpad/abi/SidioraERC20.json', import.meta.url),
      'utf8',
    ),
  );
  const identities = abi => new Set(
    abi.map(entry =>
      `${entry.type}:${entry.name ?? ''}(${entry.inputs.map(input => input.type).join(',')})`,
    ),
  );
  assert.deepEqual(identities(erc20Abi), identities(sidioraErc20));
  assert.deepEqual(erc20Abi, sidioraErc20);
}

test(erc20AbiMatchesSidioraErc20);

test('current factory creation ABI preserves parameters exactly', () => {
  const args = ['Market', 'MKT', 3, zeroAddress];
  const encoded = encodeFunctionData({ abi: factoryAbi, functionName: 'createMarket', args });
  const decoded = decodeFunctionData({ abi: factoryAbi, data: encoded });
  assert.equal(decoded.functionName, 'createMarket');
  assert.deepEqual(decoded.args, args);
});

test('current contracts expose AMM and creation fee capabilities', () => {
  assert.ok(routerAbi.some(entry => entry.type === 'function' && entry.name === 'buy'));
  assert.ok(quoterAbi.some(entry => entry.type === 'function' && entry.name === 'quoteExactInput'));
  assert.ok(protocolConfigAbi.some(entry => entry.type === 'function' && entry.name === 'creationFee'));
});

test('deployment validation rejects absent, zero, malformed addresses and wrong chain', () => {
  assert.throws(() => validateDeployment({ chainId: 1 }), /chain 125/);
  assert.throws(() => validateDeployment({ chainId: 125, factory: zeroAddress }), /factory/);
  assert.throws(() => validateDeployment({ chainId: 125, factory: 'bad' }), /factory/);
});
