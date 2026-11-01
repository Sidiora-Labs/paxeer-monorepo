import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { activityRoutes } from '../../src/runtime/routes/activity.ts';

const hash = `0x${'Ab'.repeat(32)}`;
const wallet = `0x${'Cd'.repeat(20)}`;
const time = '2026-01-01T00:00:00.000Z';
const envelope = {
  generatedAt: time,
  nextCursor: null,
  quality: 'complete',
  requestId: '123e4567-e89b-42d3-a456-426614174000',
  revision: '1',
  watermark: {
    blockHash: hash,
    chainId: '125',
    height: '9007199254740993',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '1',
  },
} as const;

async function withLoopback(
  responseBody: () =>
    V2ContractTypes['StreamListResponse'] | V2ContractTypes['TxStatusResponse'],
  run: (
    sdk: SdkClient,
    observed: { method: string | undefined; path: string | undefined }[],
  ) => Promise<void>,
) {
  const observed: { method: string | undefined; path: string | undefined }[] =
    [];
  const server = createServer((request, response) => {
    observed.push({ method: request.method, path: request.url });
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(responseBody()));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = `ws://127.0.0.1:${bound.port}`;
    await run(
      new SdkClient({
        apiOrigin: origin,
        identityOrigin: origin,
        mediaOrigin: origin,
        streamOrigin,
        approvedStreamOrigins: [streamOrigin],
        production: false,
      }),
      observed,
    );
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

async function streamListUsesV2Route() {
  const stream = {
    creatorWallet: wallet,
    playbackUrl: 'https://media.example.com/live/playlist.m3u8',
    poolAddress: wallet,
    status: 'live',
    streamId: '123e4567-e89b-42d3-a456-426614174001',
    title: 'Live pool stream',
    viewerCount: null,
    viewerQuality: 'unavailable',
  } satisfies V2ContractTypes['Stream'];
  const response = {
    ...envelope,
    data: { items: [stream] },
  } satisfies V2ContractTypes['StreamListResponse'];
  await withLoopback(
    () => response,
    async (sdk, observed) => {
      const result = await activityRoutes.streams(
        sdk,
        new AbortController().signal,
      );
      expect(observed).toEqual([
        { method: 'GET', path: '/v2/streams?status=live&limit=50' },
      ]);
      expect(result).toEqual(response);
      expect(
        result.data.items.map(({ streamId, title, status, playbackUrl }) => ({
          streamId,
          title,
          status,
          playbackUrl,
        })),
      ).toEqual([
        {
          streamId: stream.streamId,
          title: stream.title,
          status: 'live',
          playbackUrl: stream.playbackUrl,
        },
      ]);
      await expect(
        activityRoutes.streams(sdk, AbortSignal.abort()),
      ).rejects.toThrow();
      expect(observed).toHaveLength(1);
    },
  );
}

async function receiptUsesV2TransactionStatus() {
  const receipt = {
    blockHash: hash,
    effectiveGasPriceRaw: '1000000000',
    from: wallet,
    gasUsedRaw: '21000',
    height: envelope.watermark.height,
    position: {
      blockHash: hash,
      chainId: '125',
      height: envelope.watermark.height,
      logIndex: '0',
      transactionHash: hash,
      transactionIndex: '1',
    },
    success: true,
    to: wallet,
    transactionHash: hash,
  } satisfies V2ContractTypes['Receipt'];
  let response: V2ContractTypes['TxStatusResponse'] = {
    ...envelope,
    data: { status: 'included_success', receipt },
  };
  await withLoopback(
    () => response,
    async (sdk, observed) => {
      const signal = new AbortController().signal;
      for (const invalid of [
        '0x123',
        'Ab'.repeat(32),
        `0X${'Ab'.repeat(32)}`,
        `0x${'a'.repeat(63)}`,
        `0x${'a'.repeat(65)}`,
        `0x${'g'.repeat(64)}`,
        `${hash}/../other`,
      ]) {
        expect(() => activityRoutes.receipt(sdk, invalid, signal)).toThrow(
          'Invalid transaction hash',
        );
      }
      expect(observed).toEqual([]);
      expect(await activityRoutes.receipt(sdk, hash, signal)).toEqual(response);
      response = {
        ...envelope,
        data: {
          status: 'included_reverted',
          receipt: { ...receipt, success: false },
        },
      };
      expect(await activityRoutes.receipt(sdk, hash, signal)).toEqual(response);
      for (const status of ['submitted', 'unknown'] as const) {
        response = { ...envelope, data: { status, receipt: null } };
        expect(await activityRoutes.receipt(sdk, hash, signal)).toEqual(
          response,
        );
      }
      expect(observed).toEqual(
        Array.from({ length: 4 }, () => ({
          method: 'GET',
          path: `/v2/transactions/${hash}`,
        })),
      );
      await expect(
        activityRoutes.receipt(sdk, hash, AbortSignal.abort()),
      ).rejects.toThrow();
      expect(observed).toHaveLength(4);
    },
  );
}

test(streamListUsesV2Route, streamListUsesV2Route);
test(receiptUsesV2TransactionStatus, receiptUsesV2TransactionStatus);
