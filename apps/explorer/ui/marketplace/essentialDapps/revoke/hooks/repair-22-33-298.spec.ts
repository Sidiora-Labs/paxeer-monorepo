// @vitest-environment jsdom
import ERC20Artifact from '@openzeppelin/contracts/build/contracts/ERC20.json';
import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { Server } from 'node:http';
import { createElement } from 'react';
import type { PropsWithChildren } from 'react';
import { createPublicClient, encodeAbiParameters, encodeEventTopics, encodeFunctionData, getAddress, http, parseAbiItem, toHex } from 'viem';
import type { Address, Hex, PublicClient, RpcLog } from 'viem';
import { foundry } from 'viem/chains';

import type { TokenInfo } from 'types/api/token';
import type { EssentialDappsChainConfig } from 'types/client/marketplace';

import config from 'configs/app';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import type { FetchMock } from 'vitest-fetch-mock';

import { ApprovalScanError } from './approvalScan';
import { approvalsQueryOptions } from './useApprovalsQuery';
import useSearchErc20Allowances from './useSearchErc20Allowances';
import useSearchNftAllowances from './useSearchNftAllowances';

// Replay the ERC721 approval/read shapes committed in repair-22-31-381 through
// the production HTTP transport, ABI codecs, API fetch, hooks and QueryClient.
const owner = getAddress('0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1');
const spender = getAddress('0xffcf8fdee72ac11b5c542428b35eef5769c409f0');
const collection = getAddress('0xe78a0f7e598cc8b0bb87894b0f60dd2a88d6a8ab');
const secondCollection = getAddress('0x5b1869d9a4c187f2eaa108f3062412ecf0526b24');
const erc20 = getAddress('0x22d491bde2303f2f43325b2108d26f1eaba1e32b');
const fetchMocker = globalThis.fetch as FetchMock;
const word = (value: bigint) => encodeAbiParameters([ { type: 'uint256' } ], [ value ]);
const addressWord = (value: Address) => encodeAbiParameters([ { type: 'address' } ], [ value ]);
const boolWord = (value: boolean) => encodeAbiParameters([ { type: 'bool' } ], [ value ]);
const providerFailure = { code: -32000, message: 'Recorded NFT permission read unavailable' };
type ReadResult = Hex | typeof providerFailure;
let server: Server;
let endpoint: string;
let chain: EssentialDappsChainConfig;
let publicClient: PublicClient;
let queryClient: QueryClient;
let logs: Array<RpcLog>;
let reads: Map<string, ReadResult>;
let rpcCalls: Array<{ method: string; params: Array<unknown> }>;
let apiCalls: Array<string>;
let unexpectedRequests: Array<string>;
let missingMetadata: Address | undefined;
let pausedMetadata: Address | undefined;
let releaseMetadata: (() => void) | undefined;

function eventLog(address: Address, kind: 'Approval' | 'ApprovalForAll' | 'Transfer' | 'ERC20', block: number): RpcLog {
  let topics: ReturnType<typeof encodeEventTopics>;
  let data: Hex = '0x';
  if (kind === 'ERC20') {
    topics = encodeEventTopics({ abi: ERC20Artifact.abi, eventName: 'Approval', args: { owner, spender } });
    data = word(BigInt(5));
  } else if (kind === 'Approval') {
    topics = encodeEventTopics({
      abi: NftArtifact.abi, eventName: 'Approval', args: { owner, approved: spender, tokenId: BigInt(17) },
    });
  } else if (kind === 'Transfer') {
    topics = encodeEventTopics({
      abi: NftArtifact.abi, eventName: 'Transfer', args: { from: spender, to: owner, tokenId: BigInt(17) },
    });
  } else {
    topics = encodeEventTopics({ abi: NftArtifact.abi, eventName: 'ApprovalForAll', args: { owner, operator: spender } });
    data = boolWord(true);
  }
  return {
    address, topics: topics as [Hex, ...Array<Hex>],
    data,
    blockNumber: toHex(block), blockHash: toHex(block, { size: 32 }), transactionHash: toHex(100 + block, { size: 32 }),
    transactionIndex: '0x0', logIndex: '0x0', removed: false,
  };
}

function readKey(address: Address, functionName: string, args: Array<unknown>) {
  return `${ address.toLowerCase() }:${ encodeFunctionData({ abi: address === erc20 ? ERC20Artifact.abi : NftArtifact.abi, functionName, args }) }`;
}

function setRead(address: Address, functionName: string, args: Array<unknown>, value: ReadResult) {
  reads.set(readKey(address, functionName, args), value);
}

function permissionReadCount(address: Address, functionName: string, args: Array<unknown>) {
  return rpcCalls.filter((call) => {
    if (call.method !== 'eth_call') return false;
    const params = call.params[0] as { to: Address; data: Hex };
    return `${ params.to.toLowerCase() }:${ params.data.toLowerCase() }` === readKey(address, functionName, args);
  }).length;
}

function tokenMetadata(address: string): TokenInfo {
  return {
    address_hash: address, type: address.toLowerCase() === erc20.toLowerCase() ? 'ERC-20' : 'ERC-721',
    decimals: '0', symbol: 'NFT', name: 'Recorded collection', exchange_rate: null, total_supply: '1000',
    icon_url: null, reputation: 'ok', holders_count: '1', circulating_market_cap: null,
  };
}

interface RpcRequest {
  id: number;
  method: string;
  params?: Array<unknown>;
}

function isRpcRequest(value: unknown): value is RpcRequest {
  return typeof value === 'object' && value !== null &&
    'id' in value && typeof value.id === 'number' &&
    'method' in value && typeof value.method === 'string' &&
    (!('params' in value) || Array.isArray(value.params));
}

function rpcResponse(request: RpcRequest) {
  const { id, method, params = [] } = request;
  rpcCalls.push({ method, params });
  const success = (result: unknown) => ({ jsonrpc: '2.0', id, result });
  if (method === 'eth_blockNumber') return success('0x9');
  if (method === 'eth_getLogs') {
    const filter = params[0] as { topics?: Array<Hex | null> };
    return success(logs.filter((log) => filter.topics?.every((topic, index) => !topic || topic.toLowerCase() === log.topics[index]?.toLowerCase())));
  }
  if (method === 'eth_call') {
    const call = params[0] as { to: Address; data: Hex };
    const value = reads.get(`${ call.to.toLowerCase() }:${ call.data.toLowerCase() }`);
    if (typeof value === 'string') return success(value);
    if (value) return { jsonrpc: '2.0', id, error: value };
  }
  unexpectedRequests.push(`${ method }:${ JSON.stringify(params) }`);
  return { jsonrpc: '2.0', id, error: { code: -32000, message: 'Unrecorded request' } };
}

beforeAll(async() => {
  fetchMocker.disableMocks();
  server = createServer(async(request, response) => {
    response.setHeader('Content-Type', 'application/json');
    const path = new URL(request.url!, 'http://localhost').pathname;
    if (path === '/rpc') {
      let body = '';
      for await (const chunk of request) body += String(chunk);
      const payload = JSON.parse(body);
      const requests = Array.isArray(payload) ? payload : [ payload ];
      if (!requests.every(isRpcRequest)) throw new Error('Invalid JSON-RPC request');
      const results = requests.map(rpcResponse);
      response.end(JSON.stringify(Array.isArray(payload) ? results : results[0]));
      return;
    }
    apiCalls.push(path);
    if (path.includes('/tokens/')) {
      const address = getAddress(path.split('/').at(-1)!);
      const complete = () => {
        if (address === missingMetadata) response.statusCode = 503;
        response.end(JSON.stringify(address === missingMetadata ? { message: 'Recorded metadata unavailable' } : tokenMetadata(address)));
      };
      if (address === pausedMetadata) {
        releaseMetadata = () => {
          releaseMetadata = undefined;
          complete();
        };
      } else complete();
    } else if (path.endsWith('/token-balances')) {
      response.end(JSON.stringify([]));
    } else if (path.includes('/blocks/')) {
      response.end(JSON.stringify({ timestamp: new Date(Number(path.split('/').at(-1)) * 1000).toISOString() }));
    } else {
      unexpectedRequests.push(path);
      response.statusCode = 404;
      response.end(JSON.stringify({ message: 'Unrecorded request' }));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('No loopback address');
  endpoint = `http://127.0.0.1:${ address.port }`;
  const general = config.apis.general;
  if (!general) throw new Error('Missing committed Vitest API configuration');
  chain = {
    id: String(foundry.id), name: 'Recorded chain',
    app_config: { app: config.app, chain: config.chain, apis: { general: { ...general, endpoint, basePath: '' } } },
  };
  publicClient = createPublicClient({ chain: foundry, transport: http(`${ endpoint }/rpc`, { retryCount: 0 }), cacheTime: 0 });
});

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  logs = [];
  reads = new Map();
  rpcCalls = [];
  apiCalls = [];
  unexpectedRequests = [];
  missingMetadata = undefined;
  pausedMetadata = undefined;
  releaseMetadata = undefined;
});

afterEach(async() => {
  releaseMetadata?.();
  cleanup();
  await queryClient.cancelQueries();
  queryClient.clear();
  expect(unexpectedRequests).toEqual([]);
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMocker.enableMocks();
});

function wrapper({ children }: PropsWithChildren) {
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

function discovery() {
  return renderHook(() => useSearchNftAllowances(), { wrapper });
}

function queryDiscovery() {
  return renderHook(() => {
    const searchErc20 = useSearchErc20Allowances();
    const searchNft = useSearchNftAllowances();
    return useQuery(approvalsQueryOptions(chain, owner, publicClient, searchErc20, searchNft));
  }, { wrapper });
}

async function approvalEvents() {
  return publicClient.getLogs({
    event: parseAbiItem('event Approval(address indexed owner, address indexed spender, uint256 value)'),
    args: { owner }, fromBlock: BigInt(0), toBlock: BigInt(9),
  });
}

test('nftScanErrorsRetainSuccessfulContractsAndRetry', nftScanErrorsRetainSuccessfulContractsAndRetry);
test('nftScanWaitsForSuccessfulContractsAfterAnotherFails', nftScanWaitsForSuccessfulContractsAfterAnotherFails);
test('nftTerminalPermissionErrorsRemainObservable', nftTerminalPermissionErrorsRemainObservable);
test('nftCancellationBeforeAndDuringScanPropagates', nftCancellationBeforeAndDuringScanPropagates);
test('nftOperatorOnlyCollectionIsDiscoveredOnceAcrossEventKinds', nftOperatorOnlyCollectionIsDiscoveredOnceAcrossEventKinds);
test('nftContractDiscoveryIgnoresInterleavedErc20Indexes', nftContractDiscoveryIgnoresInterleavedErc20Indexes);
test('nftConfirmedRevocationIsCompleteEmptySuccess', nftConfirmedRevocationIsCompleteEmptySuccess);

async function nftScanErrorsRetainSuccessfulContractsAndRetry() {
  logs = [ eventLog(collection, 'ApprovalForAll', 2), eventLog(secondCollection, 'Approval', 4) ];
  setRead(collection, 'isApprovedForAll', [ owner, spender ], boolWord(true));
  setRead(secondCollection, 'ownerOf', [ BigInt(17) ], providerFailure);
  const { result } = queryDiscovery();
  await waitFor(() => expect(result.current.error).toBeInstanceOf(ApprovalScanError));
  const error = result.current.error as ApprovalScanError;
  expect(result.current.isError).toBe(true);
  expect(result.current.data).toBeUndefined();
  expect(error.coverage).toBe('incomplete');
  expect(error.retryable).toBe(true);
  expect(error.partialAllowances).toHaveLength(1);
  expect(error.partialAllowances[0]).toMatchObject({ address: collection, spender, allowance: 'Unlimited', timestamp: 2000 });
  expect(error.issues).toHaveLength(1);
  expect(error.issues[0]).toMatchObject({ address: secondCollection, kind: 'scan' });
  expect(error.issues[0].cause).toBeInstanceOf(Error);
  expect((error.issues[0].cause as Error).message).toContain(providerFailure.message);
  setRead(secondCollection, 'ownerOf', [ BigInt(17) ], addressWord(owner));
  setRead(secondCollection, 'getApproved', [ BigInt(17) ], addressWord(spender));
  await act(async() => {
    await result.current.refetch();
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data).toHaveLength(2);
  expect(result.current.data?.map((row) => row.address)).toEqual([ secondCollection, collection ]);
  expect(queryClient.getQueryCache().getAll().map((query) => query.queryKey)).toEqual([ [ 'revoke:approvals', chain.id, owner ] ]);
}

async function nftScanWaitsForSuccessfulContractsAfterAnotherFails() {
  logs = [ eventLog(collection, 'ApprovalForAll', 2), eventLog(secondCollection, 'ApprovalForAll', 4) ];
  missingMetadata = secondCollection;
  pausedMetadata = collection;
  setRead(collection, 'isApprovedForAll', [ owner, spender ], boolWord(true));
  const { result } = queryDiscovery();
  try {
    await waitFor(() => expect(releaseMetadata).toBeTypeOf('function'));
    await waitFor(() => expect(apiCalls).toContain(`/api/v2/tokens/${ secondCollection }`));
    expect(result.current.isPending || result.current.isPlaceholderData).toBe(true);
  } finally {
    releaseMetadata?.();
  }
  await waitFor(() => expect(result.current.error).toBeInstanceOf(ApprovalScanError));
  const error = result.current.error as ApprovalScanError;
  expect(error.partialAllowances).toHaveLength(1);
  expect(error.partialAllowances[0]).toMatchObject({ address: collection, spender });
  expect(error.issues).toHaveLength(1);
  expect(error.issues[0].address).toBe(secondCollection);
}

async function nftTerminalPermissionErrorsRemainObservable() {
  logs = [ eventLog(collection, 'ApprovalForAll', 2) ];
  setRead(collection, 'isApprovedForAll', [ owner, spender ], providerFailure);
  const { result } = discovery();
  await expect(result.current(chain, owner, [], publicClient, BigInt(9))).rejects.toMatchObject({
    name: 'ApprovalScanError', coverage: 'incomplete', retryable: true,
    partialAllowances: [], issues: [ { address: collection, cause: expect.any(Error) } ],
  });
  await expect(result.current(chain, owner, [], publicClient, BigInt(9))).rejects.toThrow(providerFailure.message);
}

async function nftCancellationBeforeAndDuringScanPropagates() {
  const { result } = discovery();
  const before = new AbortController();
  before.abort();
  await expect(result.current(chain, owner, [], publicClient, BigInt(9), before.signal)).rejects.toMatchObject({ name: 'AbortError' });
  expect(rpcCalls).toEqual([]);
  logs = [ eventLog(collection, 'ApprovalForAll', 2) ];
  pausedMetadata = collection;
  const during = new AbortController();
  const scan = result.current(chain, owner, [], publicClient, BigInt(9), during.signal);
  const outcome = scan.then((value) => ({ value }), (error: unknown) => ({ error }));
  try {
    await waitFor(() => expect(releaseMetadata).toBeTypeOf('function'));
    during.abort();
  } finally {
    releaseMetadata?.();
  }
  await expect(outcome).resolves.toMatchObject({ error: { name: 'AbortError' } });
  expect(rpcCalls.filter((call) => call.method === 'eth_call')).toEqual([]);
}

async function nftOperatorOnlyCollectionIsDiscoveredOnceAcrossEventKinds() {
  logs = [
    eventLog(collection.toLowerCase() as Address, 'ApprovalForAll', 2),
    eventLog(collection, 'Transfer', 3), eventLog(collection, 'ApprovalForAll', 4),
  ];
  setRead(collection, 'isApprovedForAll', [ owner, spender ], boolWord(true));
  const { result } = discovery();
  const events = await approvalEvents();
  expect(events).toEqual([]);
  const rows = await result.current(chain, owner, events, publicClient, BigInt(9));
  expect(rows).toHaveLength(1);
  expect(rows[0]).toMatchObject({ address: collection, spender, timestamp: 4000, tokenId: undefined });
  expect(permissionReadCount(collection, 'isApprovedForAll', [ owner, spender ])).toBe(1);
  expect(apiCalls.filter((path) => path === `/api/v2/tokens/${ collection }`)).toHaveLength(1);
}

async function nftContractDiscoveryIgnoresInterleavedErc20Indexes() {
  logs = [
    eventLog(erc20, 'ERC20', 1), eventLog(collection, 'Approval', 2),
    eventLog(erc20, 'ERC20', 3), eventLog(secondCollection, 'ApprovalForAll', 4),
    eventLog(collection.toLowerCase() as Address, 'Transfer', 5),
  ];
  setRead(collection, 'ownerOf', [ BigInt(17) ], addressWord(owner));
  setRead(collection, 'getApproved', [ BigInt(17) ], addressWord(spender));
  setRead(secondCollection, 'isApprovedForAll', [ owner, spender ], boolWord(true));
  const { result } = discovery();
  const events = await approvalEvents();
  expect(events.map((event) => event.topics.length)).toEqual([ 3, 4, 3 ]);
  const rows = await result.current(chain, owner, events, publicClient, BigInt(9));
  expect(rows).toHaveLength(2);
  expect(rows.find((row) => row.address === collection)).toMatchObject({ tokenId: '17', spender });
  expect(rows.find((row) => row.address === secondCollection)).toMatchObject({ tokenId: undefined, spender });
  expect(permissionReadCount(collection, 'getApproved', [ BigInt(17) ])).toBe(1);
  expect(permissionReadCount(secondCollection, 'isApprovedForAll', [ owner, spender ])).toBe(1);
  expect(apiCalls.filter((path) => path.includes('/tokens/')).sort()).toEqual([
    `/api/v2/tokens/${ collection }`, `/api/v2/tokens/${ secondCollection }`,
  ].sort());
}

async function nftConfirmedRevocationIsCompleteEmptySuccess() {
  logs = [ eventLog(collection, 'ApprovalForAll', 2) ];
  setRead(collection, 'isApprovedForAll', [ owner, spender ], boolWord(false));
  const { result } = queryDiscovery();
  await waitFor(() => expect(result.current.isSuccess && !result.current.isPlaceholderData).toBe(true));
  expect(result.current.error).toBeNull();
  expect(result.current.data).toEqual([]);
  expect(permissionReadCount(collection, 'isApprovedForAll', [ owner, spender ])).toBe(1);
}
