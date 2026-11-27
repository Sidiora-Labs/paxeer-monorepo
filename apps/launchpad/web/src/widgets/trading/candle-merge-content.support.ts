import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { createServer, request as httpRequest } from 'node:http';
import type { Duplex } from 'node:stream';
import { expect } from 'vitest';
import { browserStreamTransportV2, decodeV2Contract, SdkClient, type V2ContractTypes } from '@kindle/sdk';
import { encodeFunctionData, getAddress, parseAbi } from 'viem';
import {
  canonicalCandleToDisplayBar,
  CanonicalCandleRevisionMerge,
  createReplayUdfDatafeed,
  fetchCanonicalCandlePage,
  registerChartReplayTransport,
  type ChartFreshness,
} from './udf-datafeed';

type Candle = V2ContractTypes['Candle'];

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('Expected candle configuration object');
  return Object.fromEntries(Object.entries(value));
}
function text(value: unknown): string {
  if (typeof value !== 'string' || !value) throw new Error('Expected candle configuration value');
  return value;
}
function origin(value: unknown): string {
  const url = new URL(text(value));
  if (!['http:', 'https:'].includes(url.protocol) || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password) throw new Error('Owned chart authority required');
  return url.origin;
}
async function eventually(predicate: () => boolean | Promise<boolean>) {
  const deadline = Date.now() + 45_000;
  while (!await predicate()) {
    if (Date.now() >= deadline) throw new Error('Canonical candle did not converge');
    await new Promise(resolve => setTimeout(resolve, 50));
  }
}

async function chartBrowserProxy(authorities: { api: string; gateway: string; charts: string; applicationOrigin: string }) {
  const sockets = new Set<Duplex>();
  const server = createServer((request, response) => {
    const path = request.url ?? '/';
    const authority = path.startsWith('/v2/auth/') || path.startsWith('/v2/stream/tickets') ? authorities.gateway : path.startsWith('/v2/udf/') ? authorities.charts : authorities.api;
    const target = new URL(path, authority);
    const upstream = httpRequest(target, { method: request.method, headers: { ...request.headers, host: target.host, origin: authorities.applicationOrigin } }, reply => {
      response.writeHead(reply.statusCode ?? 502, reply.headers);
      reply.pipe(response);
    });
    upstream.on('error', () => response.destroy());
    response.on('close', () => upstream.destroy());
    request.pipe(upstream);
  });
  server.on('connection', socket => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
  });
  server.on('upgrade', (request, socket, head) => {
    const target = new URL(request.url ?? '/v2/stream', authorities.api);
    const upstream = httpRequest(target, { headers: { ...request.headers, host: target.host, origin: authorities.applicationOrigin } });
    upstream.on('upgrade', (response, peer, upstreamHead) => {
      sockets.add(peer);
      socket.on('close', () => peer.destroy());
      peer.on('close', () => { sockets.delete(peer); socket.destroy(); });
      socket.on('error', () => peer.destroy());
      peer.on('error', () => socket.destroy());
      socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n${Object.entries(response.headers).map(([key, value]) => `${key}: ${Array.isArray(value) ? value.join(', ') : value}`).join('\r\n')}\r\n\r\n`);
      if (upstreamHead.length) socket.write(upstreamHead);
      if (head.length) peer.write(head);
      socket.pipe(peer);
      peer.pipe(socket);
    });
    upstream.on('response', response => { response.resume(); socket.destroy(); });
    upstream.on('error', () => socket.destroy());
    upstream.end();
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Chart browser transport unavailable');
  return {
    origin: `http://127.0.0.1:${address.port}`,
    close: async () => {
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    },
  };
}

export async function assertHistoryAndTopicKeepFeedDrawingNewerRevisions(crossDay: boolean) {
  const bytes = readFileSync(text(process.env.JOINED_CONFIG_FILE));
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  expect(process.env.JOINED_ALLOW_CHAIN_MUTATION).toBe('1');
  const config = object(JSON.parse(bytes.toString()));
  const chain = object(JSON.parse(readFileSync(text(process.env.JOINED_CHAIN_FILE), 'utf8')));
  expect(chain.realm).toBe('private-anvil125');
  expect(chain.productionActivation).toBe(false);
  const pool = getAddress(text(chain.pool));
  const api = origin(config.api_origin);
  const rpcOrigin = origin(config.rpc_url);
  const applicationOrigin = origin(config.stream_application_origin);
  expect(Array.isArray(config.processes)).toBe(true);
  const processes = (config.processes as unknown[]).map(object);
  const roleOrigin = (role: string) => origin(object(processes.find(item => item.role === role)).ready_url);
  const proxy = await chartBrowserProxy({ api: roleOrigin('api'), gateway: roleOrigin('gateway'), charts: roleOrigin('trading-charts'), applicationOrigin });
  const sockets = proxy.origin.replace(/^http/, 'ws');
  const client = new SdkClient({ apiOrigin: proxy.origin, identityOrigin: proxy.origin, mediaOrigin: proxy.origin, streamOrigin: sockets, approvedStreamOrigins: [sockets], production: false });
  const authorization = new AbortController();
  const release = registerChartReplayTransport({ transport: browserStreamTransportV2(client, { applicationOrigin, snapshotPath: '/v2/snapshots' }, false), signal: authorization.signal });
  const freshness: ChartFreshness[] = [];
  const ticks: ReturnType<typeof canonicalCandleToDisplayBar>[] = [];
  const feed = createReplayUdfDatafeed(pool, 'price', value => freshness.push(value), undefined, { apiOrigin: proxy.origin, valuationOrigin: proxy.origin });
  const rpc = async (method: string, params: unknown[]) => {
    const response = await fetch(rpcOrigin, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }) });
    expect(response.ok).toBe(true);
    const result = object(await response.json());
    expect(result.error).toBeUndefined();
    return result.result;
  };
  try {
    await rpc('anvil_setBlockTimestampInterval', [1]);
    const latestBlock = async () => object(await rpc('eth_getBlockByNumber', ['latest', false]));
    const now = Number(BigInt(text((await latestBlock()).timestamp)));
    const period = { from: now - 3 * 86400, to: now + 60 };
    const router = getAddress(text(object(object(object(chain.manifest).roles).Router).address));
    const buy = async (timestamp: number) => {
      const data = encodeFunctionData({ abi: parseAbi(['function buy(address pool,uint256 usdlAmountIn,uint256 minTokensOut,uint256 deadline) returns (uint256)']), functionName: 'buy', args: [pool, 1000000n, 1n, BigInt(timestamp) + 600n] });
      const call = { from: getAddress(text(chain.account)), to: router, data };
      const gas = '0x' + ((BigInt(text(await rpc('eth_estimateGas', [call]))) * 5n + 3n) / 4n).toString(16);
      await rpc('evm_setNextBlockTimestamp', [timestamp]);
      const transaction = text(await rpc('eth_sendTransaction', [{ ...call, gas }]));
      await eventually(async () => {
        const receipt = await rpc('eth_getTransactionReceipt', [transaction]);
        if (receipt === null) return false;
        expect(object(receipt).status).toBe('0x1');
        const block = object(await rpc('eth_getBlockByHash', [text(object(receipt).blockHash), false]));
        expect(BigInt(text(block.timestamp))).toBe(BigInt(timestamp));
        return true;
      });
      return transaction;
    };
    const readHistory = async () => {
      let page: Awaited<ReturnType<typeof fetchCanonicalCandlePage>> | undefined;
      await eventually(async () => {
        try {
          page = await fetchCanonicalCandlePage(pool, '1D', new Date(period.from * 1000).toISOString(), new Date(period.to * 1000).toISOString(), 50, undefined, undefined, api);
          return page.data.items.length > 0;
        } catch (error) {
          if (error instanceof Error && 'status' in error && error.status === 409) return false;
          throw error;
        }
      });
      if (!page) throw new Error('Canonical candle history required');
      return page;
    };
    const topic = `market.candles.${pool.toLowerCase()}.1d`;
    let held: Candle | undefined;
    let answer: Candle | undefined;
    const readHistoryAndTopic = async (transaction?: string) => {
      await eventually(async () => {
        held = (await readHistory()).data.items.at(-1)!;
        const snapshot = decodeV2Contract('TopicSnapshot', await browserStreamTransportV2(client, { applicationOrigin, snapshotPath: '/v2/snapshots' }, false).snapshot([topic], authorization.signal));
        answer = decodeV2Contract('Candle', snapshot.topics[0].payload);
        return answer.start === held.start && answer.revision === held.revision
          && (!transaction || held.lastPosition.transactionHash.toLowerCase() === transaction.toLowerCase());
      });
    };
    await readHistoryAndTopic();
    if (!held || !answer) throw new Error('Canonical history and topic candles required');
    const initialStart = held.start;
    if (crossDay) {
      const timestamp = Number(BigInt(text((await latestBlock()).timestamp)));
      const midnight = (Math.floor(timestamp / 86400) + 1) * 86400;
      await rpc('evm_setNextBlockTimestamp', [midnight]);
      await rpc('evm_mine', []);
      expect(BigInt(text((await latestBlock()).timestamp))).toBe(BigInt(midnight));
    }
    let nextTimestamp = Number(BigInt(text((await latestBlock()).timestamp))) + 1;
    period.to = nextTimestamp + 60;
    if (new Date(Math.floor(nextTimestamp / 86400) * 86400 * 1000).toISOString() !== held.start) {
      const transaction = await buy(nextTimestamp);
      await readHistoryAndTopic(transaction);
      nextTimestamp = Number(BigInt(text((await latestBlock()).timestamp))) + 1;
    }
    if (!held || !answer) throw new Error('Canonical history and topic candles required');
    if (crossDay) expect(Date.parse(held.start)).toBeGreaterThan(Date.parse(initialStart));
    expect(new Date(Math.floor(nextTimestamp / 86400) * 86400 * 1000).toISOString()).toBe(held.start);
    const bars = await new Promise<ReturnType<typeof canonicalCandleToDisplayBar>[]>((resolve, reject) => feed.getBars({}, '1D', period, resolve, reason => reject(new Error(reason))));
    expect(bars.length).toBeGreaterThan(0);
    expect(answer.start).toBe(held.start);
    expect(answer.revision).toBe(held.revision);
    const merge = new CanonicalCandleRevisionMerge();
    expect(merge.accept(held, pool, '1d')).toBe('apply');
    expect(merge.accept(answer, pool, '1d')).toBe('ignore');
    feed.subscribeBars({}, '1D', bar => ticks.push(bar), 'candle-content');
    await eventually(() => feed.replayStatus()[0]?.replayComplete === true);
    expect(ticks.at(-1)).toEqual(canonicalCandleToDisplayBar(held));
    nextTimestamp = Number(BigInt(text((await latestBlock()).timestamp))) + 1;
    expect(new Date(Math.floor(nextTimestamp / 86400) * 86400 * 1000).toISOString()).toBe(held.start);
    const transaction = await buy(nextTimestamp);
    const snapshotUrl = new URL('/v2/snapshots', proxy.origin);
    snapshotUrl.searchParams.set('topics', topic);
    await eventually(async () => {
      const response = await fetch(snapshotUrl, { headers: { 'X-Schema-Version': '2' } });
      if (response.status !== 200) return false;
      const envelope = decodeV2Contract('TopicSnapshotReadResponse', await response.json());
      expect(envelope.data.snapshot.topics).toHaveLength(1);
      const entry = envelope.data.snapshot.topics[0];
      expect(entry.topic).toBe(topic);
      const candle = decodeV2Contract('Candle', entry.payload);
      return candle.lastPosition.transactionHash.toLowerCase() === transaction.toLowerCase();
    });
    let corrected: Candle | undefined;
    await eventually(async () => {
      const page = await readHistory();
      const candle = page.data.items.at(-1);
      if (candle?.lastPosition.transactionHash.toLowerCase() !== transaction.toLowerCase()) return false;
      corrected = candle;
      return true;
    });
    expect(corrected!.start).toBe(held.start);
    expect(BigInt(corrected!.revision)).toBe(BigInt(held.revision) + 1n);
    await eventually(() => JSON.stringify(ticks.at(-1)) === JSON.stringify(canonicalCandleToDisplayBar(corrected)));
    expect(ticks.at(-1)).toEqual(canonicalCandleToDisplayBar(corrected));
    expect(feed.replayStatus()[0].replayComplete).toBe(true);
    expect(freshness.filter(value => value.connectionStatus === 'resync_required' || value.connectionStatus === 'offline')).toEqual([]);
    expect(freshness.map(value => value.lagReason)).not.toContain('Visible history revision conflict');
  } finally {
    feed.dispose();
    authorization.abort();
    release();
    await proxy.close();
    await rpc('anvil_removeBlockTimestampInterval', []);
  }
}
