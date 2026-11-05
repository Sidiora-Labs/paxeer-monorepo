import Fastify, { type FastifyInstance } from 'fastify';
import { randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect } from 'vitest';
import {
  encodeAbiParameters,
  encodeFunctionData,
  keccak256,
  parseAbi,
  parseTransaction,
  toHex,
  type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { RpcPool, sharedRpcPool, type SequencerReceiptResult } from '../../src/rpc/pool.js';
import { startIdentityProvider, type IdentityProvider } from './identity.js';
import {
  authenticateNodeReceipt,
  authenticateSponsoredBindReceipt,
  nodeEvidenceForTransaction,
  nodeReceiptCorpus,
  openReceiptDatabase,
  recordedSponsoredSubmit,
  sponsoredBindRecording,
} from './node-receipts.js';

// Public conformance key from rpc/sequencer_receipt_vectors_test.go. Signing the
// same transaction must reproduce the node's envelope byte for byte.
const privateKey = '0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63';
const account = privateKeyToAccount(privateKey);
const vector = nodeReceiptCorpus.cases.find((entry) => entry.id === 'V1-pax-transfer');
if (!vector) throw new Error('missing node transfer transcript');
const evidence = nodeEvidenceForTransaction(vector.rawTransaction);
const transaction = parseTransaction(evidence.rawTransaction);
let app: FastifyInstance;
let rpc: RpcPool;
let receiptRpc: RpcPool;
let receiptDatabase: Awaited<ReturnType<typeof openReceiptDatabase>>;
let node: Server;
let identity: IdentityProvider;
let database: typeof import('../../src/db/pool.js');
let variant: SequencerReceiptResult['rung'] | null = 'receipted';
const writes: { method: string; params: unknown[] }[] = [];
const errors: unknown[] = [];
let broadcast = false;
const sponsoredEvidence = nodeEvidenceForTransaction(
  sponsoredBindRecording.positive.rawTransaction
);
const sponsoredTransaction = parseTransaction(sponsoredEvidence.rawTransaction);
const sponsoredPayload = recordedSponsoredSubmit();
type SponsorMode =
  | 'normal'
  | 'status-disconnected'
  | 'submit-disconnected'
  | 'sequencer-disconnected';
let sponsorMode: SponsorMode = 'normal';
let sponsorBroadcast = false;
let sponsorNonce = sponsoredTransaction.nonce!;
const sponsorWrites: Hex[] = [];
const sponsorRequests: { path: string; body: unknown }[] = [];

function servedReceipt(): SequencerReceiptResult | null {
  if (variant === null) return null;
  if (variant === 'submitted')
    return {
      rung: 'submitted',
      txHash: evidence.receiptJson.txHash,
      nonce: evidence.receiptJson.nonce,
    };
  if (variant === 'committed')
    return {
      ...evidence.receiptJson,
      rung: 'committed',
      blockNumber: evidence.receiptJson.height,
      blockHash: keccak256(evidence.receiptJson.height),
    };
  return evidence.receiptJson;
}

beforeAll(async () => {
  identity = await startIdentityProvider();
  const { env } = await import('../../src/env.js');
  env.SUPABASE_URL = identity.url;
  env.HYPERPAXEER_CHAIN_ID = transaction.chainId!;
  receiptDatabase = await openReceiptDatabase();
  expect(env.DATABASE_URL).toBe(receiptDatabase.url);
  database = await import('../../src/db/pool.js');

  node = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number;
        method: string;
        params: unknown[];
      };
      if (request.url?.startsWith('/station/')) {
        sponsorRequests.push({ path: request.url, body });
        expect(request.method).toBe('POST');
        expect(request.headers['content-type']).toBe('application/json');
        response.setHeader('content-type', 'application/json');
        if (request.url === '/station/submit') {
          const c = sponsoredPayload.construction;
          expect(body).toEqual({
            call: { to: sponsoredPayload.to, value: '0', data: sponsoredPayload.data },
            authorization: sponsoredPayload.authorization,
            batch: {
              chainId: c.chainId,
              account: c.account,
              nonce: c.nonce,
              calls: c.calls,
              quote: { ...c.quote, decimals: 6 },
            },
            accountSignature: sponsoredPayload.account_signature,
            relayerSignature: sponsoredPayload.relayer_signature,
          });
          if (!sponsorBroadcast) await rpc.sendRawTransaction(sponsoredEvidence.rawTransaction);
          if (sponsorMode === 'submit-disconnected') response.destroy();
          else
            response.end(
              JSON.stringify({ transactionHash: sponsoredEvidence.receiptJson.txHash })
            );
          return;
        }
        expect(request.url).toBe('/station/status');
        expect(body).toEqual({
          account: sponsoredPayload.account,
          sponsor: sponsoredPayload.construction.quote.sponsor,
          quoteNonce: sponsoredPayload.construction.quote.quoteNonce,
          relayerSignature: sponsoredPayload.relayer_signature,
        });
        expect(sponsorBroadcast).toBe(true);
        if (sponsorMode === 'status-disconnected') response.destroy();
        else
          response.end(
            JSON.stringify({
              state: 'completed',
              deadline: sponsoredPayload.construction.quote.deadline,
              submission: {
                sponsorNonce: String(sponsoredTransaction.nonce),
                transactionHash: sponsoredEvidence.receiptJson.txHash,
              },
              replacement: null,
              completion: {
                outcome: 'included',
                transactionHash: sponsoredEvidence.receiptJson.txHash,
                blockNumber: BigInt(sponsoredEvidence.receiptJson.height).toString(),
                sidCollected: sponsoredPayload.construction.quote.tokenAmount,
                paxSpent: BigInt(sponsoredEvidence.receiptJson.feeAmount).toString(),
              },
            })
          );
        return;
      }
      let result: unknown;
      switch (body.method) {
        case 'eth_chainId':
          result = toHex(transaction.chainId!);
          break;
        case 'eth_blockNumber':
          result = evidence.receiptJson.height;
          break;
        case 'eth_getTransactionCount':
          if ((body.params[0] as string).toLowerCase() === sponsoredBindRecording.sponsor) {
            result = toHex(sponsorNonce);
            break;
          }
          expect((body.params[0] as string).toLowerCase()).toBe(account.address.toLowerCase());
          expect(body.params[1]).toBe('latest');
          result = toHex(transaction.nonce! + (broadcast ? 1 : 0));
          break;
        case 'eth_call': {
          const call = body.params[0] as { from: Hex; to: Hex; value: Hex };
          if (call.to.toLowerCase() === '0x0000000000000000000000000000000000001004') {
            expect(body.params).toEqual([
              {
                to: call.to,
                data: encodeFunctionData({
                  abi: parseAbi([
                    'function getLayerXDid(address account) view returns (bytes32,string)',
                  ]),
                  functionName: 'getLayerXDid',
                  args: [sponsoredBindRecording.user],
                }),
              },
              'latest',
            ]);
            result = encodeAbiParameters(
              [{ type: 'bytes32' }, { type: 'string' }],
              [sponsoredBindRecording.didPublicKey, sponsoredBindRecording.boundDid]
            );
            break;
          }
          expect(call.from.toLowerCase()).toBe(account.address.toLowerCase());
          expect(call.to.toLowerCase()).toBe(transaction.to);
          expect(BigInt(call.value)).toBe(transaction.value);
          result = '0x';
          break;
        }
        case 'eth_sendRawTransaction':
          if (body.params[0] === sponsoredEvidence.rawTransaction) {
            expect(sponsorBroadcast).toBe(false);
            sponsorWrites.push(sponsoredEvidence.rawTransaction);
            sponsorBroadcast = true;
            sponsorNonce += 1;
            result = sponsoredEvidence.receiptJson.txHash;
            break;
          }
          expect(body.params).toEqual([evidence.rawTransaction]);
          expect(broadcast).toBe(false);
          writes.push(body);
          broadcast = true;
          result = evidence.receiptJson.txHash;
          break;
        case 'pax_sequencerReceipt':
          if (body.params[0] === sponsoredEvidence.receiptJson.txHash) {
            sponsorRequests.push({ path: '/sequencer', body });
            expect(sponsorBroadcast).toBe(true);
            if (sponsorMode === 'sequencer-disconnected') {
              response.destroy();
              return;
            }
            result = sponsoredEvidence.receiptJson;
            break;
          }
          expect(body.params).toEqual([evidence.receiptJson.txHash]);
          expect(broadcast).toBe(true);
          writes.push(body);
          result = servedReceipt();
          break;
        default:
          throw new Error(`unrecorded wallet node method: ${body.method}`);
      }
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
    } catch (error) {
      errors.push(error);
      response.writeHead(500);
      response.end(String(error));
    }
  });
  await new Promise<void>((resolve, reject) => {
    node.once('error', reject);
    node.listen(0, '127.0.0.1', resolve);
  });
  const url = `http://127.0.0.1:${(node.address() as AddressInfo).port}`;
  env.RPC_URLS = [url];
  env.SEQUENCER_RPC_URL = url;
  receiptRpc = sharedRpcPool();
  rpc = new RpcPool({
    urls: [url],
    sequencerUrl: url,
    chainId: transaction.chainId!,
    timeoutMs: 3000,
    healthIntervalMs: 60_000,
    lagThresholdBlocks: 2,
  });
  await rpc.checkHealth();
  const { NonceStore } = await import('../../src/nonce/store.js');
  const nonces = new NonceStore({
    pool: database.getPool(),
    chainId: transaction.chainId!,
    pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
    lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
  });
  const { signRoutes } = await import('../../src/routes/sign.js');
  app = Fastify();
  await app.register(signRoutes, { pool: database.getPool(), rpc, nonces, attestors: null });
  const { walletSponsorRoutes } = await import('../../src/routes/wallet.js');
  await app.register(walletSponsorRoutes, { stationUrl: `${url}/station`, rpc });
  await app.ready();
}, 30_000);

afterAll(async () => {
  await app?.close();
  rpc?.stop();
  receiptRpc?.stop();
  await receiptDatabase?.stop();
  if (node)
    await new Promise<void>((resolve, reject) => {
      node.close((error) => (error ? reject(error) : resolve()));
      node.closeAllConnections();
    });
  await identity?.stop();
});

export async function assertSponsoredSubmitEvidence(): Promise<void> {
  const userId = randomUUID();
  await database.query('delete from wallets where lower(address) = lower($1)', [
    sponsoredBindRecording.user,
  ]);
  await database.query(
    `insert into wallets (user_id, address, encrypted_private_key, key_version, chain_id, kind, migrated_at, attestor_key_id)
     values ($1, $2, null, 1, $3, 'standard', now(), $4)`,
    [userId, sponsoredBindRecording.user, sponsoredBindRecording.chainId, `recorded:${userId}`]
  );
  const token = await identity.mintUserToken(userId);
  for (const scenario of [
    'normal',
    'status-disconnected',
    'submit-disconnected',
    'sequencer-disconnected',
  ] as const) {
    sponsorMode = scenario;
    sponsorBroadcast = false;
    sponsorNonce = sponsoredTransaction.nonce!;
    sponsorWrites.length = 0;
    sponsorRequests.length = 0;
    const request = {
      method: 'POST' as const,
      url: '/v1/wallet/sponsored/submit',
      headers: { authorization: `Bearer ${token}` },
      payload: sponsoredPayload,
    };
    const response = await app.inject(request);
    expect(sponsorWrites).toEqual([sponsoredEvidence.rawTransaction]);
    expect(sponsorRequests.map((entry) => entry.path)).toEqual([
      '/station/submit',
      '/station/status',
      '/sequencer',
    ]);
    expect(sponsorRequests[2]!.body).toMatchObject({
      method: 'pax_sequencerReceipt',
      params: [sponsoredEvidence.receiptJson.txHash],
    });
    if (scenario === 'sequencer-disconnected') {
      expect(response.statusCode, response.body).toBe(503);
      expect(response.json()).toEqual({
        error: 'sequencer_receipt_unavailable',
        status: 'pending',
      });
    } else {
      expect(response.statusCode, response.body).toBe(200);
      expect(response.headers['cache-control']).toBe('no-store');
      const body = response.json<{
        tx_hash: Hex;
        status: string;
        did: string;
        station: Record<string, unknown> | null;
        rung: SequencerReceiptResult['rung'];
        receipt: SequencerReceiptResult;
      }>();
      expect(body.tx_hash).toBe(keccak256(sponsoredEvidence.rawTransaction));
      expect(body.did).toBe(sponsoredBindRecording.boundDid);
      expect(body.status).toBe(scenario === 'status-disconnected' ? 'pending' : 'confirmed');
      if (scenario === 'status-disconnected') expect(body.station).toBeNull();
      else
        expect(body.station).toMatchObject({
          state: 'completed',
          completion: { transactionHash: body.tx_hash },
        });
      expect(body.rung).toBe(sponsoredEvidence.receiptJson.rung);
      expect(body.receipt.rung).toBe(body.rung);
      expect(body.receipt).toEqual(sponsoredEvidence.receiptJson);
      expect(await authenticateSponsoredBindReceipt(body.receipt, sponsoredEvidence)).toEqual(
        body.receipt
      );
      if (body.receipt.rung === 'submitted')
        throw new Error('relayed transaction must be receipted');
      expect(body.receipt.status).toBe('0x1');
      expect(body.receipt.sponsor).toBe(sponsoredBindRecording.sponsor);
      expect(body.receipt.nonce).toBe(toHex(sponsoredTransaction.nonce!));
      expect(BigInt(body.receipt.feeAmount)).toBe(
        BigInt(body.receipt.gasUsed) * BigInt(body.receipt.effectiveGasPrice)
      );
      expect(body.receipt.feeDenom).toBe(sponsoredEvidence.receiptJson.feeDenom);
      if (scenario === 'normal') {
        const nonceBefore = await rpc.getTransactionCount(sponsoredBindRecording.sponsor);
        const repeated = await app.inject(request);
        expect(repeated.statusCode, repeated.body).toBe(200);
        expect(repeated.json()).toEqual(body);
        expect(sponsorWrites).toEqual([sponsoredEvidence.rawTransaction]);
        expect(await rpc.getTransactionCount(sponsoredBindRecording.sponsor)).toBe(nonceBefore);
      }
    }
    expect(errors).toEqual([]);
  }
}

export async function assertWalletSendEvidence(
  receiptVariant: SequencerReceiptResult['rung'] | null
): Promise<void> {
  variant = receiptVariant;
  for (const nonce of [undefined, transaction.nonce!]) {
    await database.query(`truncate wallets, nonce_allocations, nonce_outstanding,
      wallet_approval_recovery, rate_limit_windows, signing_audit cascade`);
    broadcast = false;
    writes.length = 0;
    const userId = randomUUID();
    const { encrypt, loadMasterKey } = await import('../../src/crypto.js');
    const { env } = await import('../../src/env.js');
    const envelope = encrypt(
      Buffer.from(privateKey.slice(2), 'hex'),
      loadMasterKey(env.WALLET_MASTER_KEY),
      env.WALLET_MASTER_KEY_VERSION
    );
    await database.query(
      `insert into wallets
      (user_id, address, encrypted_private_key, key_version, chain_id, kind)
      values ($1, $2, $3, $4, $5, 'standard')`,
      [userId, account.address, envelope.ciphertext, envelope.version, transaction.chainId]
    );
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/send',
      headers: { authorization: `Bearer ${await identity.mintUserToken(userId)}` },
      payload: {
        tx: {
          to: transaction.to,
          value: transaction.value!.toString(),
          gas: transaction.gas!.toString(),
          chainId: transaction.chainId,
          maxFeePerGas: transaction.maxFeePerGas!.toString(),
          maxPriorityFeePerGas: transaction.maxPriorityFeePerGas!.toString(),
          ...(nonce === undefined ? {} : { nonce }),
        },
      },
    });
    expect(response.statusCode, response.body).toBe(200);
    expect(writes.map((entry) => entry.method)).toEqual([
      'eth_sendRawTransaction',
      'pax_sequencerReceipt',
    ]);
    expect(writes[0]!.params).toEqual([evidence.rawTransaction]);
    expect(writes[1]!.params).toEqual([keccak256(evidence.rawTransaction)]);
    const body = response.json<{
      rung: SequencerReceiptResult['rung'];
      receipt: SequencerReceiptResult | null;
    }>();
    expect(body).toEqual({
      tx_hash: evidence.receiptJson.txHash,
      address: account.address,
      chain_id: transaction.chainId,
      rung: variant ?? 'submitted',
      receipt: servedReceipt(),
    });
    if (body.receipt && body.receipt.rung !== 'submitted') {
      expect(await authenticateNodeReceipt(body.receipt, evidence)).toEqual(body.receipt);
      await expect(
        authenticateNodeReceipt({ ...body.receipt, status: '0x0' }, evidence)
      ).rejects.toThrow();
      await expect(
        authenticateNodeReceipt(body.receipt, {
          ...evidence,
          pinnedKey: { ...evidence.pinnedKey, publicKey: `0x${'ff'.repeat(32)}` },
        })
      ).rejects.toThrow();
    }
    expect(errors).toEqual([]);
  }
}
