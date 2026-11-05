import { strict as assert } from 'node:assert';
import { createServer } from 'node:http';
import { createPublicClient, encodeFunctionData, getAddress, getSerializedTransactionType, http, keccak256, parseTransaction, recoverTransactionAddress, toHex, type Address, type Hex, type TransactionSerialized, type TransactionSerializedLegacy } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { decodeV2Contract } from '@kindle/sdk';
import FeesRouter from '@/core/network/abis/FeesRouter.json';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { paxeerWalletChain } from '@/core/wallet-sdk/client';
import { RecordedProvider, type RpcRequest } from '../swap/swap-chain.fixture';
import recording from '../swap/swap-chain.recording.json';
import type { RewardClaimAuthority } from './reward-claim-recovery';

function signedLegacy(serialized: Hex): TransactionSerializedLegacy {
  assert.equal(getSerializedTransactionType(serialized), 'legacy');
  return serialized as TransactionSerializedLegacy;
}

// Recorded chain-125 RPC envelopes; transaction identity is emitted by the real signer.
export async function rewardClaimFixture() {
  const signer = privateKeyToAccount(recording.privateKey as Hex);
  const router = getAddress(CORE_DEPLOYMENT.feesRouter);
  const template = recording.swaps[0];
  const request = {
    chainId: '125' as const, from: signer.address, to: router,
    data: encodeFunctionData({ abi: FeesRouter, functionName: 'claimFees', args: [1n] }),
    valueRaw: '0', gasLimitRaw: BigInt(template.gas).toString(),
    maxFeePerGasRaw: null, maxPriorityFeePerGasRaw: null,
  };
  const prepared = decodeV2Contract('RewardClaimResponse', {
    data: { estimatedGasPaxRaw: '100000', transactionRequest: request },
    generatedAt: '2026-01-01T00:00:00.000Z', nextCursor: null, quality: 'complete',
    requestId: 'cbeecf50-c346-48c9-8450-dfdf44bb88eb', revision: '1', watermark: null,
  });
  const ledger = new Map<Hex, {
    transaction: Record<string, unknown>;
    receipt: Record<string, unknown>;
  }>();
  const state = {
    head: { number: BigInt(template.receipt.blockNumber).toString(), hash: template.receipt.blockHash as Hex },
    chainId: '0x7d',
    canonicalHash: template.receipt.blockHash,
    receiptStatus: '0x1',
    loseWalletResponse: false,
    onSend: undefined as (() => void) | undefined,
    walletResponse: undefined as Promise<void> | undefined,
    onReceipt: undefined as (() => void) | undefined,
    receiptResponse: undefined as Promise<void> | undefined,
  };
  let lastHash: Hex | undefined;
  const provider = new RecordedProvider(async ({ method, params = [] }) => {
    switch (method) {
      case 'eth_chainId': return state.chainId;
      case 'eth_accounts': return [signer.address];
      case 'eth_sendTransaction': {
        state.onSend?.();
        const tx = params[0] as { from: Address; to: Address; data: Hex; value?: Hex; gas?: Hex };
        assert.equal(tx.from.toLowerCase(), signer.address.toLowerCase());
        const serialized = await signer.signTransaction({
          type: 'legacy', chainId: 125, nonce: ledger.size, to: tx.to, data: tx.data,
          value: BigInt(tx.value ?? '0x0'), gas: BigInt(tx.gas ?? template.gas), gasPrice: BigInt(template.gasPrice),
        });
        lastHash = await accept(signedLegacy(serialized));
        if (state.walletResponse) await state.walletResponse;
        if (state.loseWalletResponse) throw new Error('Recorded wallet transport closed after broadcast');
        return lastHash;
      }
      case 'eth_getTransactionByHash': return ledger.get(params[0] as Hex)?.transaction ?? null;
      case 'eth_getTransactionReceipt': {
        state.onReceipt?.();
        if (state.receiptResponse) await state.receiptResponse;
        return ledger.get(params[0] as Hex)?.receipt ?? null;
      }
      case 'eth_getBlockByNumber': {
        if (BigInt(String(params[0])) !== BigInt(state.head.number) && BigInt(String(params[0])) !== BigInt(template.receipt.blockNumber)) return null;
        return { number: params[0], hash: state.canonicalHash, transactions: [...ledger.keys()] };
      }
      default: throw new Error(`Unrecorded reward RPC: ${method}`);
    }
  });
  async function accept(serialized: TransactionSerialized) {
    const tx = parseTransaction(serialized);
    const from = await recoverTransactionAddress({ serializedTransaction: serialized });
    const hash = keccak256(serialized);
    ledger.set(hash, {
      transaction: {
        ...template.transaction, hash, chainId: toHex(tx.chainId!), nonce: toHex(tx.nonce!),
        from, to: tx.to, input: tx.data, value: toHex(tx.value ?? 0n), gas: toHex(tx.gas!),
        gasPrice: toHex(tx.gasPrice!), r: tx.r, s: tx.s, v: toHex(tx.v!),
      },
      receipt: { ...template.receipt, transactionHash: hash, from, to: tx.to, logs: [], status: state.receiptStatus },
    });
    return hash;
  }
  const requests: { path: string; body: unknown; authorization?: string }[] = [];
  const errors: unknown[] = [];
  const server = createServer(async (incoming, response) => {
    try {
      let body = '';
      for await (const chunk of incoming) body += String(chunk);
      response.setHeader('content-type', 'application/json');
      if (incoming.url === '/rpc') {
        const rpc = JSON.parse(body) as RpcRequest & { id: number };
        const result = await provider.request(rpc as Parameters<typeof provider.request>[0]);
        response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }));
      } else if (incoming.url === '/head') {
        assert.equal(incoming.method, 'GET');
        response.end(JSON.stringify(state.head));
      } else {
        assert.equal(incoming.method, 'POST');
        assert.equal(incoming.url, '/v2/me/rewards:prepare-claim');
        requests.push({ path: incoming.url, body: JSON.parse(body), authorization: incoming.headers.authorization });
        response.end(JSON.stringify(prepared));
      }
    } catch (error) {
      errors.push(error);
      response.writeHead(500);
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const origin = `http://127.0.0.1:${address.port}`;
  const authority: RewardClaimAuthority = {
    revision: {
      schemaVersion: 2, deploymentVersion: 'recorded-chain-125',
      manifestSha256: CORE_DEPLOYMENT.manifestSha256, capabilitySha256: CORE_DEPLOYMENT.manifestSha256,
      contractSupportSha256: CORE_DEPLOYMENT.manifestSha256, serialInterfaceSha256: CORE_DEPLOYMENT.manifestSha256,
      revisionSha256: CORE_DEPLOYMENT.manifestSha256,
    },
    expiresAt: Date.now() + 60_000,
    network: {
      network: {
        schemaVersion: 1, deploymentVersion: 'recorded-chain-125', chainId: 125,
        manifestHash: CORE_DEPLOYMENT.manifestSha256, sourceManifestSha256: CORE_DEPLOYMENT.manifestSha256,
        publicRPC: origin + '/rpc', explorerBase: origin,
        native: { symbol: 'PAX', decimals: 18 }, settlement: { symbol: 'USDL', decimals: 6 },
        creationFeeRaw: '1000000', addresses: { FeesRouter: router },
      },
      usdlRuntimeHash: CORE_DEPLOYMENT.roleIdentity.usdl.runtimeHash,
      async committedHead(signal) {
        const response = await fetch(origin + '/head', { signal });
        assert.equal(response.status, 200);
        const head = await response.json() as { number: string; hash: Hex };
        return { number: BigInt(head.number), hash: head.hash };
      },
    },
  };
  const client = createPublicClient({ chain: paxeerWalletChain, transport: http(origin + '/rpc', { retryCount: 0 }) });
  return {
    origin, provider, client, authority, prepared, requests, errors, state,
    owner: signer.address,
    get lastHash() { assert.ok(lastHash); return lastHash; },
    get entry() { const entry = ledger.get(this.lastHash); assert.ok(entry); return entry; },
    async otherTransaction(changes: { chainId?: number; to?: Address; data?: Hex; value?: bigint; account?: typeof signer } = {}) {
      return accept(signedLegacy(await (changes.account ?? signer).signTransaction({
        type: 'legacy', chainId: changes.chainId ?? 125, nonce: ledger.size,
        to: changes.to ?? router, data: changes.data ?? request.data, value: changes.value ?? 0n,
        gas: BigInt(template.gas), gasPrice: BigInt(template.gasPrice),
      })));
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
      await provider.disconnect();
    },
  };
}

export type RewardClaimFixture = Awaited<ReturnType<typeof rewardClaimFixture>>;
