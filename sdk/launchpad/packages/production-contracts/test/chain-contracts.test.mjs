import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { ContractValidationError, decodeContract, validateContract } from '../dist/index.js';

const deployment = JSON.parse(await readFile(new URL('../../../shared/config/deployment/current.json', import.meta.url), 'utf8'));
const addressRoles = {
  eventEmitter: 'EventEmitter',
  factory: 'SidioraFactory',
  feeAccumulator: 'FeeAccumulator',
  nft: 'SidioraNFT',
  poolRegistry: 'PoolRegistry',
  protocolConfig: 'ProtocolConfig',
  quoter: 'Quoter',
  router: 'Router',
  usdl: 'USDL',
};
const addresses = Object.fromEntries(Object.entries(addressRoles).map(([field, role]) => [field, deployment.roles[role].address]));
const lineBreaks = ['\n', '\r', '\r\n', '\u2028', '\u2029'];

function deploymentConventionsMatch() {
  const conventions = {
    chainId: String(deployment.chainId),
    creationFeeRaw: deployment.creationFeeRaw,
    finality: 'committed',
    naming: 'camelCase',
    nativeDecimals: deployment.native.decimals,
    nativeSymbol: deployment.native.symbol,
    quoteDecimals: deployment.settlement.decimals,
    quoteSymbol: deployment.settlement.symbol,
  };
  assert.deepEqual(decodeContract('Conventions', conventions), conventions);
  const data = {
    chainId: String(deployment.chainId),
    nativeSymbol: deployment.native.symbol,
    contractAddresses: addresses,
    manifestHash: `0x${deployment.sourceManifestSha256}`,
    capabilities: {
      bounties: false,
      conditionalOrders: false,
      creation: true,
      embeddedWallet: false,
      nativeFunding: true,
      rewards: false,
      swap: true,
      voice: false,
    },
  };
  assert.deepEqual(decodeContract('DeploymentData', data), data);
  assert.equal(validateContract('Conventions', { ...conventions, chainId: '1' }), false);
  assert.equal(validateContract('Conventions', { ...conventions, quoteDecimals: 18 }), false);
  assert.equal(validateContract('Conventions', { ...conventions, creationFeeRaw: '1000000000000000000' }), false);
}
test(deploymentConventionsMatch);

function deploymentAddressesPreserveExactBytes() {
  const decoded = decodeContract('ContractAddresses', addresses);
  for (const [field, role] of Object.entries(addressRoles)) {
    const address = deployment.roles[role].address;
    assert.equal(decoded[field], address);
    assert.equal(decodeContract('ContractAddresses', { ...addresses, [field]: address.toLowerCase() })[field], address.toLowerCase());
    const { [field]: omitted, ...missing } = addresses;
    assert.equal(omitted, address);
    assert.equal(validateContract('ContractAddresses', missing), false, field);
    for (const invalid of [address.slice(2), address.slice(0, -1), `${address}00`, ...lineBreaks.map(suffix => address + suffix)]) {
      const input = { ...addresses, [field]: invalid };
      assert.equal(validateContract('ContractAddresses', input), false, field);
      assert.throws(() => decodeContract('ContractAddresses', input), error => {
        assert.ok(error instanceof ContractValidationError);
        assert.equal(error.contract, 'ContractAddresses');
        assert.ok(error.violations.some(violation => violation.instancePath === `/${field}`));
        return true;
      });
      assert.equal(input[field], invalid);
    }
  }
}
test(deploymentAddressesPreserveExactBytes);

function rawAmountsRequireCompleteDecimalStrings() {
  const max = ((1n << 256n) - 1n).toString();
  for (const name of ['UInt256', 'PositiveUInt256']) {
    for (const value of ['1', '9007199254740993', deployment.creationFeeRaw, max]) {
      assert.equal(decodeContract(name, value), value);
    }
    for (const invalid of [1, 1n, '01', '-1', '1.0', '1e0', ' 1', '1 ', (BigInt(max) + 1n).toString()]) {
      assert.equal(validateContract(name, invalid), false, name);
    }
  }
  assert.equal(decodeContract('UInt256', '0'), '0');
  assert.equal(validateContract('PositiveUInt256', '0'), false);
  for (const name of ['UInt256', 'PositiveUInt256', 'UInt63', 'AggregateUInt', 'SignedAggregateInteger', 'PriceComponent', 'PositivePriceComponent']) {
    for (const suffix of lineBreaks) {
      assert.equal(validateContract(name, `1${suffix}`), false, name);
      assert.throws(() => decodeContract(name, `1${suffix}`), ContractValidationError);
    }
  }
}
test(rawAmountsRequireCompleteDecimalStrings);

function chainHashesRequireCompleteBytes32() {
  const hash = `0x${deployment.sourceManifestSha256}`;
  const position = {
    chainId: String(deployment.chainId),
    height: '9007199254740993',
    transactionIndex: '0',
    logIndex: '0',
    blockHash: hash,
    transactionHash: hash,
  };
  assert.deepEqual(decodeContract('ChainPosition', position), position);
  assert.deepEqual(decodeContract('TxStatusRequest', { transactionHash: hash }), { transactionHash: hash });
  for (const field of ['blockHash', 'transactionHash']) {
    for (const invalid of [hash.slice(2), hash.slice(0, -2), `${hash}00`, ...lineBreaks.map(suffix => hash + suffix)]) {
      assert.equal(validateContract('ChainPosition', { ...position, [field]: invalid }), false, field);
      assert.throws(() => decodeContract('ChainPosition', { ...position, [field]: invalid }), ContractValidationError);
      if (field === 'transactionHash') assert.equal(validateContract('TxStatusRequest', { transactionHash: invalid }), false);
    }
  }
}
test(chainHashesRequireCompleteBytes32);

function transactionRequestsPreserveChainEncoding() {
  const transaction = {
    chainId: String(deployment.chainId),
    from: addresses.eventEmitter,
    to: addresses.router,
    data: '0x',
    valueRaw: '0',
    gasLimitRaw: null,
    maxFeePerGasRaw: null,
    maxPriorityFeePerGasRaw: null,
  };
  for (const data of ['0x', '0x00', '0xaAbB', `0x${'ff'.repeat(131072)}`]) {
    const value = { ...transaction, data };
    assert.deepEqual(decodeContract('TransactionRequest', value), value);
  }
  for (const data of ['0x0', '0xgg', 'aabb', `0x${'ff'.repeat(131073)}`, ...lineBreaks.map(suffix => `0xaabb${suffix}`)]) {
    assert.equal(validateContract('TransactionRequest', { ...transaction, data }), false);
    assert.throws(() => decodeContract('TransactionRequest', { ...transaction, data }), ContractValidationError);
  }
  for (const field of ['from', 'to']) {
    for (const suffix of lineBreaks) assert.equal(validateContract('TransactionRequest', { ...transaction, [field]: transaction[field] + suffix }), false, field);
  }
  for (const field of ['valueRaw', 'gasLimitRaw', 'maxFeePerGasRaw', 'maxPriorityFeePerGasRaw']) {
    const value = { ...transaction, [field]: '9007199254740993' };
    assert.equal(decodeContract('TransactionRequest', value)[field], '9007199254740993');
    assert.equal(validateContract('TransactionRequest', { ...transaction, [field]: 9007199254740993 }), false, field);
    for (const suffix of lineBreaks) assert.equal(validateContract('TransactionRequest', { ...transaction, [field]: `1${suffix}` }), false, field);
  }
  assert.equal(validateContract('TransactionRequest', { ...transaction, chainId: '1' }), false);
}
test(transactionRequestsPreserveChainEncoding);
