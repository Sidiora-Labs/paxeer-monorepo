import { EventEmitter } from 'node:events';
import { createServer, type ServerResponse } from 'node:http';
import { createHash } from 'node:crypto';
import { encodeFunctionData, type EIP1193Provider, type Hex } from 'viem';
import { erc20Abi } from 'sidiora-launchpad-v3';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import type { AccountAsset } from '@/widgets/profile/account-transport';
import type { Bounty } from './bounties-contract';
import type { TransactionRequest } from './prepared-transaction';

export const creator = '0x1111111111111111111111111111111111111111';
export const winner = '0x2222222222222222222222222222222222222222';
export const attacker = '0x3333333333333333333333333333333333333333';
export const deployment = { USDL: CORE_DEPLOYMENT.usdl, Router: CORE_DEPLOYMENT.router, FeesRouter: CORE_DEPLOYMENT.feesRouter };
export const bounty: Bounty = { bountyId: 'repair-985', creatorWallet: creator, assetAddress: CORE_DEPLOYMENT.usdl, budgetRaw: '25000000', title: 'Review bounty', description: 'Work', poolAddress: null, state: 'reviewing' };
export const selection = { submissionId: 'submission-985', winnerWallet: winner, awardRaw: '5000000' };
export const broadcastHash = `0x${'ab'.repeat(32)}` as Hex;
export const committedBlockHash = `0x${'cd'.repeat(32)}` as Hex;

export function preparedTokenRequest(overrides: Partial<TransactionRequest> = {}): TransactionRequest {
  // The real deployed ABI produces calldata; JSON carries the prepared DTO.
  return JSON.parse(JSON.stringify({ chainId: '125', from: creator, to: CORE_DEPLOYMENT.usdl,
    data: encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [winner, 5_000_000n] }),
    valueRaw: '0', gasLimitRaw: '65000', maxFeePerGasRaw: '1000000000', maxPriorityFeePerGasRaw: null, ...overrides }));
}

export function transcriptProvider(client: { transport: { request: EIP1193Provider['request'] } }): EIP1193Provider {
  const events = new EventEmitter();
  return { request: client.transport.request, on: events.on.bind(events), removeListener: events.removeListener.bind(events) };
}

/** Committed chain-125 receipt transcript, served through JSON-RPC HTTP. */
export function receiptTranscript(status: 'success' | 'reverted') {
  return { transactionHash: broadcastHash, transactionIndex: '0x0', blockHash: committedBlockHash, blockNumber: '0x7b', from: creator, to: CORE_DEPLOYMENT.usdl, cumulativeGasUsed: '0x5208', gasUsed: '0x5208', effectiveGasPrice: '0x3b9aca00', contractAddress: null, logs: [], logsBloom: `0x${'00'.repeat(256)}`, status: status === 'success' ? '0x1' : '0x0', type: '0x2' };
}

export class BountyRpcTranscript {
  readonly calls: Array<{ method: string; params: unknown[] }> = [];
  receipt: ReturnType<typeof receiptTranscript> | null = null;
  receiptFailure = false;
  private readonly server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk.toString();
    const call = JSON.parse(body) as { id: number; method: string; params: unknown[] };
    this.calls.push(call);
    let result: unknown;
    switch (call.method) {
      case 'eth_chainId': result = '0x7d'; break;
      case 'eth_accounts': result = [creator]; break;
      case 'eth_sendTransaction': result = broadcastHash; break;
      case 'eth_estimateGas': result = '0xfde8'; break;
      case 'eth_gasPrice': result = '0x3b9aca00'; break;
      case 'eth_getTransactionReceipt':
        if (this.receiptFailure) { response.writeHead(200, { 'Content-Type': 'application/json' }); response.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, error: { code: -32000, message: 'Receipt transport timed out' } })); return; }
        result = this.receipt; break;
      default: response.writeHead(500); response.end(`Unexpected RPC ${call.method}`); return;
    }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result }));
  });
  async open() {
    await new Promise<void>(resolve => this.server.listen(0, '127.0.0.1', resolve));
    const address = this.server.address();
    if (!address || typeof address === 'string') throw new Error('Missing loopback address');
    return `http://127.0.0.1:${address.port}`;
  }
  async close() { this.server.closeAllConnections(); await new Promise<void>((resolve, reject) => this.server.close(error => error ? reject(error) : resolve())); }
}

/** In-memory asset service with the same authenticated PUT/GET interface. */
export class BountyAssetGateway {
  readonly assets = new Map<string, AccountAsset>();
  readonly calls: Array<{ method: string; wallet: string; id: string }> = [];
  hold: 'PUT' | 'GET' | null = null;
  private releases: Array<() => void> = [];
  private readonly server = createServer(async (request, response: ServerResponse) => {
    const match = /^\/v2\/accounts\/(0x[0-9a-f]{40})\/assets\/([\w-]+)$/.exec(request.url ?? '');
    if (!match || !request.method) { response.writeHead(404).end(); return; }
    const [, wallet, id] = match;
    if (request.headers.authorization !== `Bearer fixture:${wallet}`) { response.writeHead(403).end(); return; }
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    this.calls.push({ method: request.method, wallet, id });
    if (request.method === 'PUT') {
      const bytes = Buffer.concat(chunks);
      this.assets.set(id, { assetId: id, ownerWallet: wallet, sha256: createHash('sha256').update(bytes).digest('hex'), mediaType: request.headers['content-type'] ?? '', bytes: bytes.length, state: 'quarantined', publicUrl: null });
    }
    if (this.hold === request.method) await new Promise<void>(resolve => this.releases.push(resolve));
    const asset = this.assets.get(id);
    if (!asset || asset.ownerWallet !== wallet) { response.writeHead(404).end(); return; }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(asset));
  });
  async open(origin: string) { const url = new URL(origin); await new Promise<void>(resolve => this.server.listen(Number(url.port), url.hostname, resolve)); }
  approve(id: string) { const asset = this.assets.get(id); if (!asset) throw new Error('Asset not uploaded'); this.assets.set(id, { ...asset, state: 'approved', publicUrl: `https://media.example.test/${asset.ownerWallet}/${id}.png` }); }
  release() { this.hold = null; for (const resolve of this.releases.splice(0)) resolve(); }
  async close() { this.release(); this.server.closeAllConnections(); await new Promise<void>((resolve, reject) => this.server.close(error => error ? reject(error) : resolve())); }
}
