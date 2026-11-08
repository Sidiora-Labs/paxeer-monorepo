import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { ed25519 } from '@noble/curves/ed25519.js';
import { secp256k1 } from '@noble/curves/secp256k1.js';
import { keccak_256 } from '@noble/hashes/sha3.js';
import { encodeSequencerReceipt } from '../../../../../../services/agent/sdk/typescript/src/sequencer-receipt-codec.ts';
import { encodeAbiCall } from '../../../../../../services/agent/sdk/typescript/src/exchange.ts';
import {
  PERPS_EVENTS,
  PERPS_PRECOMPILE,
  LAUNCHPAD_EVENTS,
  LAUNCHPAD_PRECOMPILE,
} from '../../../../../../services/agent/sdk/typescript/src/generated/precompile-writes.ts';

// Task 22.39.55 explicitly permits missing recordings from the documented node
// receipt model. These are model vectors, not claims of application execution.
const root = new URL('../../../../../../', import.meta.url);
const read = (name) => JSON.parse(readFileSync(new URL(name, root), 'utf8'));
const source = read('sdk/platform/conformance/fixtures/sequencer-receipt-v1.json');
const vectors = read('sdk/wallet/test/fixtures/modules/events.json');
const calls = read('sdk/wallet/test/fixtures/modules/calls.json');
const ledgerAbi = read('precompiles/layerxledger/abi.json');
const xwebAbi = read('precompiles/xweb/abi.json');
const hex = (bytes) => `0x${Buffer.from(bytes).toString('hex')}`;
const bytes = (value) => Buffer.from(value.slice(2), 'hex');
const concat = (...parts) => Buffer.concat(parts);
const hash = (...parts) =>
  createHash('sha256')
    .update(concat(...parts))
    .digest();
const text = (value) => Buffer.from(value, 'utf8');
const uint = (value, width) => {
  const n = BigInt(value);
  assert.ok(n >= 0n && n < 1n << BigInt(width * 8));
  return Buffer.from(n.toString(16).padStart(width * 2, '0'), 'hex');
};
const zero = Buffer.alloc(32);
const zeroAddress = `0x${'00'.repeat(20)}`;
const addressWord = (address) => concat(Buffer.alloc(12), bytes(address));
const chainKey = bytes('0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63');
const exchangeKey = keccak_256(text('paxeer-wallet-sdk-fixture'));
const seed = Buffer.alloc(32, 0x5e);
const pinnedKey = source.cases[0].pinnedKey;
assert.equal(hex(ed25519.getPublicKey(seed)), pinnedKey.publicKey);
const sender = (key) => hex(keccak_256(secp256k1.getPublicKey(key, false).slice(1)).slice(12));

// The shared ABI writer covers the construct's fixed words and strings. The
// ledger and XWeb byte payloads additionally need Solidity's dynamic bytes form.
function modelAbiCall(name, types, values) {
  let offset = types.length * 32;
  const tails = [];
  const heads = types.map((type, index) => {
    const value = values[index];
    if (type !== 'bytes' && type !== 'string')
      return bytes(`0x${encodeAbiCall('word', [type], [value]).slice(10)}`);
    const body = type === 'bytes' ? bytes(value) : text(value);
    const padded = Buffer.alloc(Math.ceil(body.length / 32) * 32);
    body.copy(padded);
    const tail = concat(uint(body.length, 32), padded);
    const head = uint(offset, 32);
    tails.push(tail);
    offset += tail.length;
    return head;
  });
  return hex(
    concat(keccak_256(text(`${name}(${types.join(',')})`)).slice(0, 4), ...heads, ...tails)
  );
}

function rlp(value) {
  const list = Array.isArray(value);
  const body = list ? concat(...value.map(rlp)) : Buffer.from(value);
  if (!list && body.length === 1 && body[0] < 128) return body;
  const prefix = list ? 192 : 128;
  if (body.length < 56) return concat(Buffer.from([prefix + body.length]), body);
  const length = integer(body.length);
  return concat(Buffer.from([prefix + 55 + length.length]), length, body);
}

function integer(value) {
  if (BigInt(value) === 0n) return Buffer.alloc(0);
  const encoded = BigInt(value).toString(16);
  return Buffer.from(encoded.padStart(encoded.length + (encoded.length % 2), '0'), 'hex');
}

function transaction(key, nonce, call) {
  const fields = [
    integer(125),
    integer(nonce),
    integer(1_000_000_000),
    integer(3_000_000_000),
    integer(2_000_000),
    bytes(call.to),
    integer(call.value ?? 0n),
    bytes(call.data),
    [],
  ];
  const signature = secp256k1.sign(keccak_256(concat(Buffer.from([2]), rlp(fields))), key, {
    prehash: false,
    format: 'recovered',
  });
  const parsed = secp256k1.Signature.fromBytes(signature, 'recovered');
  return concat(
    Buffer.from([2]),
    rlp([...fields, integer(signature[0]), integer(parsed.r), integer(parsed.s)])
  );
}

function eventLog(address, specs, vector) {
  const spec = specs.find((entry) => entry.name === vector.event);
  assert.ok(spec, vector.event);
  const signature = `${spec.name}(${spec.inputs.map((entry) => entry.type).join(',')})`;
  const topics = [hex(keccak_256(text(signature)))];
  const body = spec.inputs.filter((entry) => !entry.indexed);
  for (const input of spec.inputs.filter((entry) => entry.indexed)) {
    const value = vector.fields[input.name];
    topics.push(
      input.type === 'bytes32'
        ? value
        : hex(uint(input.type === 'bool' ? Number(value) : value, 32))
    );
  }
  return {
    address: address.toLowerCase(),
    topics,
    data: `0x${modelAbiCall(
      'event',
      body.map((entry) => entry.type),
      body.map((entry) => {
        const value = vector.fields[entry.name];
        return /^(?:u?int)/.test(entry.type) ? BigInt(value) : value;
      })
    ).slice(10)}`,
  };
}

const cases = [];
let previous = zero;
let balance = 100n * 10n ** 18n;
let cumulative = 0n;
function record(id, key, call, logs = [], status = 1, vmError = '', returnData = '0x') {
  const sequence = BigInt(cases.length + 1);
  const nonce = cases.filter((entry) => entry.receiptJson.from === sender(key)).length;
  const raw = transaction(key, nonce, call);
  const activity = keccak_256(raw);
  const resulting = hash(text('PAXEER/v1/sequence\0'), previous, uint(sequence, 8), activity);
  const gasUsed = 50_000n;
  const gasPrice = 2_000_000_000n;
  const fee = gasUsed * gasPrice;
  const value = BigInt(call.value ?? 0n);
  const after = balance - fee - (status === 1 ? value : 0n);
  cumulative += gasUsed;
  const effects = [
    concat(
      uint(gasUsed, 8),
      uint(cumulative, 8),
      uint(gasPrice, 8),
      uint(sequence, 4),
      Buffer.alloc(40),
      hash(text(vmError)),
      uint(bytes(returnData).length, 4),
      uint(logs.length, 4),
      Buffer.from([4]),
      text('uhpx')
    ),
    ...logs.map((log) =>
      concat(
        bytes(log.address),
        Buffer.from([log.topics.length]),
        ...log.topics.map(bytes),
        hash(bytes(log.data))
      )
    ),
  ].map((body, ordinal) => ({
    moduleId: 12,
    ordinal,
    eventType: 0,
    kind: ordinal === 0 ? 1 : 3,
    monetary: false,
    transferSetRoot: zero,
    body,
  }));
  const receipt = {
    protocolVersion: 2,
    activityId: activity,
    globalSequence: sequence,
    previousStateRoot: previous,
    resultingStateRoot: resulting,
    activityRoot: hash(bytes(returnData)),
    resultCode: status === 1 ? 0 : 1,
    effects,
    feeCharged: fee,
    batchId: uint(1000, 32),
    moduleId: 12,
    moduleVersion: 1,
    parameterVersion: 0,
    operation: 2,
    asset: hash(text('uhpx')),
    amount: value,
    from: addressWord(sender(key)),
    fromBalanceBefore: balance,
    fromBalanceAfter: after,
    fromSequence: BigInt(nonce),
    to: addressWord(call.to),
    toBalanceBefore: 0n,
    toBalanceAfter: status === 1 ? value : 0n,
    transferSetRoot: zero,
    authorizationHash: zero,
    contextHash: bytes(pinnedKey.keyId),
    timestamp: 1_750_000_400_000n,
    sequencerSignature: Buffer.alloc(64),
  };
  const digest = hash(text('LXP/v1/receipt\0'), encodeSequencerReceipt(receipt, false));
  receipt.sequencerSignature = ed25519.sign(digest, seed);
  const signed = hex(encodeSequencerReceipt(receipt));
  const quantity = (n) => `0x${BigInt(n).toString(16)}`;
  const entry = {
    id,
    rawTransaction: hex(raw),
    signedReceipt: signed,
    receiptDigest: hex(digest),
    prevSeqHash: hex(previous),
    pinnedKey,
    receiptJson: {
      rung: 'receipted',
      txHash: hex(activity),
      nonce: quantity(nonce),
      seq: quantity(sequence),
      height: '0x3e8',
      index: quantity(sequence),
      timestamp: quantity(receipt.timestamp),
      status: quantity(status),
      returnData,
      logs: logs.map((log, index) => ({ ...log, logIndex: quantity(index) })),
      gasUsed: quantity(gasUsed),
      cumulativeGasUsed: quantity(cumulative),
      effectiveGasPrice: quantity(gasPrice),
      paymentDenom: 'uhpx',
      feeDenom: 'uhpx',
      feeAmount: quantity(fee),
      sponsor: zeroAddress,
      from: sender(key),
      to: call.to.toLowerCase(),
      contractAddress: zeroAddress,
      vmError,
      keyId: pinnedKey.keyId,
      signingKey: pinnedKey.publicKey,
      receipt: signed,
    },
  };
  cases.push(entry);
  previous = resulting;
  balance = after;
  return entry;
}

const call = (to, name, types, values, value = 0n) => ({
  to,
  data: modelAbiCall(name, types, values),
  value,
});
const approve = (to, spender, amount) =>
  call(to, 'approve', ['address', 'uint256'], [spender, amount]);
const usdl = '0x85fcd13735f4309833a503ee804ea32395851479';
const launchpadUsdl = `0x${'a1'.repeat(20)}`;
const launchpadToken = `0x${'b2'.repeat(20)}`;
const perpsOrder = vectors.exchange.find((entry) => entry.event === 'OrderPlaced');
const perpsLog = eventLog(PERPS_PRECOMPILE, PERPS_EVENTS, perpsOrder);
for (const amount of [50_123_456n, 1_000_000n])
  record(`perps-approve-${amount}`, exchangeKey, approve(usdl, PERPS_PRECOMPILE, amount));
for (const isLong of [true, false])
  record(
    `perps-open-${isLong ? 'long' : 'short'}`,
    exchangeKey,
    call(
      PERPS_PRECOMPILE,
      'openPosition',
      ['uint256', 'address', 'uint256', 'uint256', 'bool'],
      [3n, usdl, 50_123_456n, 5n * 10n ** 18n, isLong]
    ),
    [perpsLog]
  );
record('perps-cancel', exchangeKey, call(PERPS_PRECOMPILE, 'cancelOrder', ['uint256'], [9n]));
record(
  'perps-deposit',
  exchangeKey,
  call(PERPS_PRECOMPILE, 'deposit', ['address', 'uint256'], [usdl, 1_000_000n])
);
for (const attempt of [1, 2])
  record(
    `sponsored-cancel-${attempt}`,
    chainKey,
    call(PERPS_PRECOMPILE, 'cancelOrder', ['bytes32'], [`0x${'22'.repeat(32)}`])
  );
const feeToken = '0x0000000000000000000000000000000000001018';
record('fee-set', chainKey, call(feeToken, 'setFeeDenom', ['string'], ['usid']));
record('fee-clear', chainKey, call(feeToken, 'clearFeeDenom', [], []));
record(
  'launchpad-create-approve',
  chainKey,
  approve(launchpadUsdl, LAUNCHPAD_PRECOMPILE, 5_000_000n)
);
record('launchpad-buy-approve', chainKey, approve(launchpadUsdl, LAUNCHPAD_PRECOMPILE, 1_000_000n));
record(
  'launchpad-sell-approve',
  chainKey,
  approve(launchpadToken, LAUNCHPAD_PRECOMPILE, BigInt(calls.launchpad.sell.amountIn))
);
record(
  'launchpad-create',
  chainKey,
  call(
    LAUNCHPAD_PRECOMPILE,
    'createMarket',
    ['string', 'string', 'uint8', 'address'],
    [
      calls.launchpad.createMarket.name,
      calls.launchpad.createMarket.symbol,
      BigInt(calls.launchpad.createMarket.feeStrategy),
      zeroAddress,
    ]
  ),
  [
    eventLog(
      LAUNCHPAD_PRECOMPILE,
      LAUNCHPAD_EVENTS,
      vectors.launchpad.find((entry) => entry.event === 'MarketCreated')
    ),
  ]
);
record(
  'launchpad-buy',
  chainKey,
  call(
    LAUNCHPAD_PRECOMPILE,
    'buy',
    ['address', 'uint256', 'uint256', 'uint256'],
    [
      calls.launchpad.buy.pool,
      1_000_000n,
      BigInt(calls.launchpad.buy.minOut),
      BigInt(calls.launchpad.buy.deadline),
    ]
  ),
  [
    eventLog(
      LAUNCHPAD_PRECOMPILE,
      LAUNCHPAD_EVENTS,
      vectors.launchpad.find((entry) => entry.event === 'Swap')
    ),
  ]
);
record(
  'launchpad-sell',
  chainKey,
  call(
    LAUNCHPAD_PRECOMPILE,
    'sell',
    ['address', 'uint256', 'uint256', 'uint256'],
    [
      calls.launchpad.sell.pool,
      BigInt(calls.launchpad.sell.amountIn),
      2_500_000n,
      BigInt(calls.launchpad.sell.deadline),
    ]
  )
);
const xweb = '0x0000000000000000000000000000000000001019';
record(
  'xweb-search',
  chainKey,
  call(
    xweb,
    'request',
    ['uint8', 'bytes', 'uint64'],
    [2n, hex(text('paxeer network')), 150_000n],
    1_000_000_000_000_000n
  ),
  [
    eventLog(
      xweb,
      xwebAbi.filter((entry) => entry.type === 'event'),
      vectors.webData.find((entry) => entry.event === 'XWebRequested')
    ),
  ]
);
record('xweb-refund', chainKey, call(xweb, 'refund', ['uint64'], [42n]));
record(
  'launchpad-approve-reverted',
  chainKey,
  approve(launchpadUsdl, LAUNCHPAD_PRECOMPILE, 1_000_000n),
  [],
  0,
  'execution reverted'
);

// Ledger event shapes and the retirement of grant issuance come from
// contracts/ledger-auth.md, not from the failing wallet assertion output.
const ledger = '0x000000000000000000000000000000000000101a';
const payment = source.cases.find((entry) => entry.id === 'V1-layerxledger-pay-exact');
const paymentLog = payment.receiptJson.logs[0];
const [reference, from, to] = paymentLog.topics.slice(1);
const asset = `0x${paymentLog.data.slice(2, 66)}`;
const amount = BigInt(`0x${paymentLog.data.slice(66)}`);
const transfer = eventLog(
  ledger,
  ledgerAbi.filter((entry) => entry.type === 'event'),
  {
    event: 'Transfer',
    fields: { from, to, assetId: asset, amount: amount.toString(), kind: '5' },
  }
);
const exact = record(
  'ledger-pay-exact',
  chainKey,
  call(
    ledger,
    'payExact',
    ['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'],
    [from, to, asset, amount, 1_900_000_000_000n, reference]
  ),
  [transfer, { address: paymentLog.address, topics: paymentLog.topics, data: paymentLog.data }]
);
const refusalData = `0x${hex(keccak_256(text('Error(string)'))).slice(2, 10)}${encodeAbiCall('error', ['string'], ['LXP_NO_PAYER_GRANT']).slice(10)}`;
const grantBody = concat(
  bytes(from),
  bytes(to),
  bytes(asset),
  uint(0, 8),
  uint(40, 8),
  uint(0, 8),
  uint(100, 8),
  Buffer.from([0]),
  uint(0, 8),
  uint(1_900_000_000_000n, 8),
  Buffer.alloc(32, 0x22),
  Buffer.from([0]),
  zero,
  uint(0, 8),
  ed25519.getPublicKey(Buffer.alloc(32, 1))
);
const grantId = hash(text('LXP/v1/authority-hash\0'), text('LXP:GRANT:v1'), grantBody);
const grantPayload = concat(grantId, grantBody, ed25519.sign(grantId, Buffer.alloc(32, 1)));
assert.equal(grantPayload.length, 346);
const retired = record(
  'ledger-grant-retired',
  chainKey,
  call(ledger, 'issueGrant', ['bytes'], [hex(grantPayload)]),
  [],
  0,
  'execution reverted',
  refusalData
);
const provenance = {
  kind: 'documented-node-receipt-model',
  generator: 'apps/wallet/src/surfaces/test/fixtures/generate-surface-receipts.mjs',
  command: 'node apps/wallet/src/surfaces/test/fixtures/generate-surface-receipts.mjs',
  receiptModel: 'spec/layerx-merge/contracts/sequencer.md#sequencer-receipt',
  ledgerModel: 'spec/layerx-merge/contracts/ledger-auth.md#layerxledger-0x101a',
  encoder: 'services/agent/sdk/typescript/src/sequencer-receipt-codec.ts#encodeSequencerReceipt',
  callSource: 'sdk/wallet/test/fixtures/modules/calls.json and surface input scenarios',
  eventSource: 'sdk/wallet/test/fixtures/modules/events.json and precompile ABIs',
  seedSource: 'rpc/sequencer_receipt_vectors_test.go#seqVecReceiptSeed',
  sourceFixture: 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json',
};
writeFileSync(
  new URL('./surface-receipts-v1.json', import.meta.url),
  `${JSON.stringify({ provenance, cases }, null, 2)}\n`
);
writeFileSync(
  new URL('./ledger-proof-v1.json', import.meta.url),
  `${JSON.stringify(
    {
      provenance,
      paymentReceiptId: exact.id,
      grantReceiptId: retired.id,
      paymentLogs: exact.receiptJson.logs.map((log) => ({
        ...log,
        transactionHash: exact.receiptJson.txHash,
      })),
      grantLogs: retired.receiptJson.logs,
      grantId: hex(grantId),
    },
    null,
    2
  )}\n`
);
console.log(
  `Recorded ${cases.length} documented node receipt model vectors in ${fileURLToPath(new URL('.', import.meta.url))}`
);
