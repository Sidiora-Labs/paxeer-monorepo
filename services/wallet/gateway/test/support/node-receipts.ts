import { strict as assert } from 'node:assert';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire, registerHooks, stripTypeScriptTypes } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import {
  decodeFunctionData,
  fromRlp,
  hexToBytes,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  sliceHex,
  toHex,
  type Hex,
} from 'viem';
import type { SequencerReceiptAnswer, ReceiptKeyPin } from '../../../../../sdk/wallet/src/types.js';
import type { SequencerReceiptVerification } from '../../../../agent/sdk/typescript/src/verifier.js';
import { startPostgresEngine } from './postgres.js';
import {
  ADDR_PRECOMPILE,
  addrAbi,
  bindMessage,
  sponsoredBindAbi,
  verifyEd25519,
} from '../../src/provision/protocol.js';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../..');
const walletSource = resolve(root, 'sdk/wallet/src');
const agentSource = resolve(root, 'services/agent/sdk/typescript/src');
const walletRequire = createRequire(resolve(root, 'sdk/wallet/package.json'));

type ReceiptCodec = {
  decodeSequencerReceipt(
    value: unknown,
    hash: string,
    pins: readonly ReceiptKeyPin[]
  ): SequencerReceiptAnswer;
};
type ReceiptVerifier = {
  verifySequencerReceipt(
    receipt: Uint8Array,
    transaction: Uint8Array,
    keys: readonly {
      keyId: Uint8Array;
      publicKey: Uint8Array;
      first_height: bigint;
      last_height: bigint;
    }[]
  ): Promise<SequencerReceiptVerification>;
};

// Load the committed production sources without requiring an out-of-scope SDK
// build. The aliases select the same implementations as the SDK browser export.
function withProductionSources<T>(load: () => T): T {
  const hook = registerHooks({
    resolve(specifier, context, next) {
      const parent = context.parentURL?.startsWith('file:') ? fileURLToPath(context.parentURL) : '';
      if (parent.startsWith(walletSource + '/') || parent.startsWith(agentSource + '/')) {
        let target: string | undefined;
        if (specifier === '@sidiora/layerx-sdk/browser')
          target = resolve(agentSource, 'browser.ts');
        else if (specifier.startsWith('@noble/')) target = walletRequire.resolve(specifier);
        else if (specifier.startsWith('.') && specifier.endsWith('.js')) {
          const source = resolve(dirname(parent), specifier.slice(0, -3) + '.ts');
          if (existsSync(source)) target = source;
        }
        if (target) return { url: pathToFileURL(target).href, shortCircuit: true };
      }
      return next(specifier, context);
    },
    load(url, context, next) {
      if (url.startsWith('file:')) {
        const source = fileURLToPath(url);
        if (
          source.endsWith('.ts') &&
          (source.startsWith(walletSource + '/') || source.startsWith(agentSource + '/'))
        ) {
          return {
            format: 'module',
            source: stripTypeScriptTypes(readFileSync(source, 'utf8'), {
              mode: 'transform',
              sourceUrl: url,
            }),
            shortCircuit: true,
          };
        }
      }
      return next(url, context);
    },
  });
  try {
    return load();
  } finally {
    hook.deregister();
  }
}

function productionReceiptModules(): ReceiptCodec & ReceiptVerifier {
  return withProductionSources(() => ({
    ...(walletRequire(resolve(walletSource, 'receipt.ts')) as ReceiptCodec),
    ...(walletRequire(resolve(agentSource, 'verifier.ts')) as ReceiptVerifier),
  }));
}

export function loadProductionWalletSdk(): typeof import('../../../../../sdk/wallet/src/index.js') {
  return withProductionSources(
    () =>
      walletRequire(
        resolve(walletSource, 'index.ts')
      ) as typeof import('../../../../../sdk/wallet/src/index.js')
  );
}

let modules: (ReceiptCodec & ReceiptVerifier) | undefined;

export interface NodeReceiptEvidence {
  id: string;
  rawTransaction: Hex;
  signedReceipt: Hex;
  receiptJson: Exclude<SequencerReceiptAnswer, { rung: 'submitted' }>;
  pinnedKey: ReceiptKeyPin;
  expected: { verifier: string };
  receiptDigest?: Hex;
}

const conformancePath = 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json';
const daemonPath = 'daemon/committed-fixture/vectors/vector-v1-receipted-positive.json';
const conformance = JSON.parse(readFileSync(resolve(root, conformancePath), 'utf8')) as {
  provenance: { generator: string; command: string };
  cases: NodeReceiptEvidence[];
};
const daemon = JSON.parse(readFileSync(resolve(root, daemonPath), 'utf8')) as {
  cases: NodeReceiptEvidence[];
};

interface SponsoredExecutionRecording {
  id: string;
  rawTransaction: Hex;
  signedReceipt: Hex;
  receiptJson: NodeReceiptEvidence['receiptJson'];
  receiptDigest: Hex;
  sponsorPaidFee: Hex;
  sponsorBalanceBefore: Hex;
  sponsorBalanceAfter: Hex;
  userBalanceBefore: Hex;
  userBalanceAfter: Hex;
  delegationDesignator: Hex;
}

interface RecordedReceiptKey {
  key_id: string;
  public_key: string;
  first_height: number;
  last_height: string;
}

const sponsoredPath = 'sdk/platform/conformance/fixtures/vector-v6-sponsored-bind.json';
// Preserve the producer's int64 upper bound before JSON number parsing can round it.
export const sponsoredBindRecording = JSON.parse(
  readFileSync(resolve(root, sponsoredPath), 'utf8').replace(
    /("last_height"\s*:\s*)(\d+)/g,
    '$1"$2"'
  )
) as {
  provenance: { revision: string; generator: string; command: string };
  chainId: number;
  sponsor: Hex;
  user: Hex;
  delegate: Hex;
  delegateCode: Hex;
  didPublicKey: Hex;
  bindCalldata: Hex;
  boundDid: string;
  pinnedKey: RecordedReceiptKey;
  receiptKeys: RecordedReceiptKey[];
  delegateDeployment: SponsoredExecutionRecording;
  positive: SponsoredExecutionRecording;
  refusal: SponsoredExecutionRecording;
};

function recordedPin(key: RecordedReceiptKey): ReceiptKeyPin {
  return {
    keyId: `0x${Buffer.from(key.key_id, 'base64').toString('hex')}`,
    publicKey: `0x${Buffer.from(key.public_key, 'base64').toString('hex')}`,
    firstHeight: BigInt(key.first_height),
    lastHeight: BigInt(key.last_height),
  };
}

const sponsoredPin = recordedPin(sponsoredBindRecording.pinnedKey);
assert(
  sponsoredBindRecording.receiptKeys.some(
    (key) => JSON.stringify(key) === JSON.stringify(sponsoredBindRecording.pinnedKey)
  ),
  'V6 receipt key must be governed by the recorded genesis pins'
);
const sponsoredCases: NodeReceiptEvidence[] = [
  sponsoredBindRecording.delegateDeployment,
  sponsoredBindRecording.positive,
  sponsoredBindRecording.refusal,
].map((recording) => ({
  ...recording,
  pinnedKey: sponsoredPin,
  expected: { verifier: 'accepts' },
}));

export function recordedSponsoredSubmit(): import('zod').infer<
  typeof import('../../src/routes/wallet.js').SponsoredSubmitBody
> {
  const recording = sponsoredBindRecording;
  const transaction = parseTransaction(recording.positive.rawTransaction);
  assert(transaction.type === 'eip7702' && transaction.data);
  const execution = decodeFunctionData({ abi: sponsoredBindAbi, data: transaction.data });
  assert(execution.functionName === 'executeSponsored');
  const [calls, quote, accountSignature, relayerSignature] = execution.args;
  const auth = transaction.authorizationList[0]!;
  return {
    chain_id: String(recording.chainId),
    account: recording.user,
    to: recording.user,
    value: '0',
    data: transaction.data,
    construction: {
      kind: 'sponsored_batch',
      chainId: String(recording.chainId),
      account: recording.user,
      nonce: '0',
      calls: calls.map((call) => ({ ...call, value: call.value.toString() })),
      quote: {
        sponsor: quote.sponsor,
        token: quote.token,
        maxTokenAmount: quote.maxTokenAmount.toString(),
        tokenAmount: quote.tokenAmount.toString(),
        deadline: quote.deadline.toString(),
        quoteNonce: quote.quoteNonce.toString(),
        gasCost: quote.gasCost.toString(),
      },
    },
    account_signature: accountSignature,
    relayer_signature: relayerSignature,
    authorization: {
      chainId: String(auth.chainId),
      address: auth.address,
      nonce: String(auth.nonce),
      r: auth.r!,
      s: auth.s!,
      yParity: auth.yParity!,
    },
    quote_decimals: 6,
  };
}

// Search the committed producer collections. Do not synthesize a receipt when
// neither collection records the submitted envelope.
export const nodeReceiptCorpus = {
  provenance: conformance.provenance,
  sources: [conformancePath, daemonPath, sponsoredPath],
  cases: [...conformance.cases, ...daemon.cases, ...sponsoredCases],
};

export function nodeEvidenceForTransaction(raw: Hex): NodeReceiptEvidence {
  const evidence = nodeReceiptCorpus.cases.find(
    (entry) => entry.rawTransaction === raw && entry.expected.verifier === 'accepts'
  );
  assert(
    evidence,
    `missing canonical node execution transcript for ${keccak256(raw)} in ${nodeReceiptCorpus.sources.join(', ')}; owner domain 5 must record this transaction`
  );
  return structuredClone(evidence);
}

export async function authenticateNodeReceipt(
  value: unknown,
  evidence: NodeReceiptEvidence
): Promise<SequencerReceiptAnswer> {
  modules ??= productionReceiptModules();
  const pin = {
    ...evidence.pinnedKey,
    firstHeight: BigInt(evidence.pinnedKey.firstHeight),
    lastHeight: BigInt(evidence.pinnedKey.lastHeight),
  };
  const answer = modules.decodeSequencerReceipt(value, keccak256(evidence.rawTransaction), [pin]);
  assert.notEqual(answer.rung, 'submitted');
  if (answer.rung === 'submitted') throw new Error('execution receipt required');
  const verified = await modules.verifySequencerReceipt(
    hexToBytes(answer.receipt),
    hexToBytes(evidence.rawTransaction),
    [
      {
        keyId: hexToBytes(pin.keyId),
        publicKey: hexToBytes(pin.publicKey),
        first_height: pin.firstHeight,
        last_height: pin.lastHeight,
      },
    ]
  );
  assert.equal(verified.level, 'sequencer-signed');
  if (evidence.receiptDigest !== undefined) {
    assert.equal(toHex(verified.receiptDigest), evidence.receiptDigest);
  }
  return answer;
}

/** Authenticate execution, then require the recorded sponsored binding effects. */
export async function authenticateSponsoredBindReceipt(
  value: unknown,
  evidence: NodeReceiptEvidence
): Promise<NodeReceiptEvidence['receiptJson']> {
  const answer = await authenticateNodeReceipt(value, evidence);
  if (answer.rung === 'submitted') throw new Error('execution receipt required');
  const recording = sponsoredBindRecording;
  const transaction = parseTransaction(evidence.rawTransaction);
  assert.equal(transaction.type, 'eip7702', 'sponsored set-code execution required');
  assert.equal(transaction.to?.toLowerCase(), recording.user);
  const envelope = fromRlp(sliceHex(evidence.rawTransaction, 1), 'hex');
  assert(Array.isArray(envelope));
  assert.equal(envelope[6], '0x', 'sponsored transaction must encode zero native value');
  assert.equal(answer.sponsor, recording.sponsor);
  assert.equal(answer.from, recording.sponsor);
  assert.equal(answer.status, '0x1');
  assert.equal(answer.nonce, toHex(transaction.nonce!));
  assert.equal(
    (
      await recoverTransactionAddress({ serializedTransaction: evidence.rawTransaction })
    ).toLowerCase(),
    recording.sponsor
  );
  assert(transaction.data, 'sponsored binding calldata required');
  const execution = decodeFunctionData({ abi: sponsoredBindAbi, data: transaction.data });
  assert.equal(execution.functionName, 'executeSponsored');
  assert(execution.functionName === 'executeSponsored');
  const [calls, quote] = execution.args;
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.to.toLowerCase(), ADDR_PRECOMPILE);
  assert.equal(calls[0]!.value, 0n);
  assert.equal(calls[0]!.data, recording.bindCalldata);
  const bind = decodeFunctionData({ abi: addrAbi, data: calls[0]!.data });
  assert.equal(bind.functionName, 'bindLayerX');
  assert(bind.functionName === 'bindLayerX');
  assert.equal(bind.args[0], recording.didPublicKey);
  assert(
    verifyEd25519(bind.args[0], bindMessage(recording.chainId, recording.user, 0n), bind.args[1])
  );
  assert.equal(recording.boundDid, `did:layerx:${bind.args[0].slice(2)}`);
  assert.equal(quote.sponsor.toLowerCase(), answer.sponsor);
  const bound = answer.logs.filter((log) => log.address === ADDR_PRECOMPILE);
  assert.equal(bound.length, 1);
  assert.deepEqual(bound[0]!.topics, [
    keccak256(Buffer.from('LayerXBound(address,bytes32,uint64)')),
    `0x${recording.user.slice(2).padStart(64, '0')}`,
    recording.didPublicKey,
  ]);
  assert.equal(BigInt(bound[0]!.data), 0n);
  const sponsored = answer.logs.filter(
    (log) => log.topics[0] === keccak256(Buffer.from('Sponsored(address,address,uint256,uint256)'))
  );
  assert.equal(sponsored.length, 1);
  assert.equal(sponsored[0]!.address, recording.user);
  assert.deepEqual(sponsored[0]!.topics.slice(1), [
    `0x${recording.sponsor.slice(2).padStart(64, '0')}`,
    `0x${quote.token.slice(2).toLowerCase().padStart(64, '0')}`,
  ]);
  assert.equal(BigInt(`0x${sponsored[0]!.data.slice(2, 66)}`), quote.tokenAmount);
  assert.equal(BigInt(`0x${sponsored[0]!.data.slice(66)}`), quote.quoteNonce);
  const fee = BigInt(answer.gasUsed) * BigInt(answer.effectiveGasPrice);
  assert(fee > 0n);
  assert.equal(BigInt(answer.feeAmount), fee);
  assert.equal(BigInt(recording.positive.sponsorPaidFee), fee);
  assert.equal(
    BigInt(recording.positive.sponsorBalanceBefore) -
      BigInt(recording.positive.sponsorBalanceAfter),
    fee
  );
  assert.equal(BigInt(recording.positive.userBalanceBefore), 0n);
  assert.equal(BigInt(recording.positive.userBalanceAfter), 0n);
  assert.equal(recording.positive.delegationDesignator, `0xef0100${recording.delegate.slice(2)}`);
  assert.equal(answer.receipt, recording.positive.signedReceipt);
  return answer;
}

/** Start the production pool against a PostgreSQL engine owned by the test. */
export async function openReceiptDatabase() {
  const database = await import('../../src/db/pool.js');
  const { env } = await import('../../src/env.js');
  // env.ts may have been loaded by RpcPool before setup. A previously opened
  // singleton must not keep the setup.ts localhost URL after this assignment.
  await database.closePool();
  // Provisioning holds a subject-lock session while other queries run. It
  // needs independent PostgreSQL sessions rather than PGlite's single session.
  const db = await startPostgresEngine();
  try {
    const url = db.url;
    process.env.DATABASE_URL = url;
    env.DATABASE_URL = url;
    await (await import('../../src/db/migrate.js')).runMigrations();
    const ready = await database.query<{ ready: number }>('select 1::int as ready');
    assert.deepEqual(ready.rows, [{ ready: 1 }]);
    return {
      url,
      async stop(): Promise<void> {
        await database.closePool();
        await db.stop();
      },
    };
  } catch (error) {
    await database.closePool();
    await db.stop();
    throw error;
  }
}
