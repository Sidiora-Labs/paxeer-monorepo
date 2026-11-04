// @vitest-environment jsdom
import ERC20Artifact from '@openzeppelin/contracts/build/contracts/ERC20.json';
import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { Server } from 'node:http';
import { createElement } from 'react';
import type { PropsWithChildren } from 'react';
import {
  createPublicClient, createWalletClient, encodeAbiParameters, encodeEventTopics, encodeFunctionData,
  getAddress, http, parseAbiItem, toHex,
} from 'viem';
import type { Address, Hex, PublicClient, RpcLog, RpcTransactionReceipt } from 'viem';
import { foundry } from 'viem/chains';

import type { AddressTokenBalancesResponse } from 'types/api/address';
import type { TokenInfo } from 'types/api/token';
import type { EssentialDappsChainConfig } from 'types/client/marketplace';
import type { AllowanceType } from 'types/client/revoke';

import config from 'configs/app';
import { ZERO_ADDRESS } from 'toolkit/utils/consts';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import type { FetchMock } from 'vitest-fetch-mock';

import { ApprovalScanError } from './approvalScan';
import { getRevokeContractParameters } from './revokeApproval';
import { RevokeSubmission } from './revokeSubmission';
import type { RevokeSubmissionState } from './revokeSubmission';
import useApprovalsQuery, { approvalsQueryOptions } from './useApprovalsQuery';
import useSearchErc20Allowances from './useSearchErc20Allowances';
import useSearchNftAllowances from './useSearchNftAllowances';

// The NFT addresses and receipt envelope are the MyNFT/Ganache transcript used by
// repair-22-31-381. Logs and read payloads pass through viem's production ABI codecs
// and a loopback JSON-RPC transport before reaching the real discovery hooks.
const owner = getAddress('0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1');
const spender = getAddress('0xffcf8fdee72ac11b5c542428b35eef5769c409f0');
const other = getAddress('0x22d491bde2303f2f43325b2108d26f1eaba1e32b');
const token = getAddress('0xe78a0f7e598cc8b0bb87894b0f60dd2a88d6a8ab');
const nft = getAddress('0x5b1869d9a4c187f2eaa108f3062412ecf0526b24');
const hash = toHex(107, { size: 32 });
const word = (value: bigint) => encodeAbiParameters([ { type: 'uint256' } ], [ value ]);
const addressWord = (value: Address) => encodeAbiParameters([ { type: 'address' } ], [ value ]);
const boolWord = (value: boolean) => encodeAbiParameters([ { type: 'bool' } ], [ value ]);
const fetchMocker = globalThis.fetch as FetchMock;
let server: Server;
let endpoint: string;
let chain: EssentialDappsChainConfig;
let publicClient: PublicClient;
let queryClient: QueryClient;
let logs: Array<RpcLog>;
let allowanceReads: Map<string, Hex | { code: number; message: string }>;
let rpcCalls: Array<{ method: string; params: Array<unknown> }>;
let apiCalls: Array<string>;
let metadataMissing: boolean;
let decimals: string | null;
let blockFailure: boolean;
let rewardsFailures: number;
let rewardRequests: Array<unknown>;
let registrationRequests: number;
let receiptReverted: boolean;
let sentTransactions: Array<unknown>;
let unexpectedRequests: Array<string>;
let holdRewards: boolean;
let releaseRewards: (() => void) | undefined;

function approvalLog(address: Address, block: number, position: number, target: Address, tokenId?: bigint): RpcLog {
  const topics = tokenId === undefined ? encodeEventTopics({
    abi: ERC20Artifact.abi, eventName: 'Approval', args: { owner, spender: target },
  }) : encodeEventTopics({
    abi: NftArtifact.abi, eventName: 'Approval', args: { owner, approved: target, tokenId },
  });
  return {
    address, topics: topics as [Hex, ...Array<Hex>], data: tokenId === undefined ? word(BigInt(5)) : '0x',
    blockNumber: toHex(block), blockHash: toHex(block, { size: 32 }), transactionHash: toHex(100 + block * 10 + position, { size: 32 }),
    transactionIndex: '0x0', logIndex: toHex(position), removed: false,
  };
}

function operatorLog(block: number, position: number, approved: boolean): RpcLog {
  return {
    ...approvalLog(nft, block, position, spender),
    topics: encodeEventTopics({ abi: NftArtifact.abi, eventName: 'ApprovalForAll', args: { owner, operator: spender } }) as [Hex, ...Array<Hex>],
    data: boolWord(approved),
  };
}

function readKey(address: Address, functionName: string, args: Array<unknown>): string {
  return `${ address.toLowerCase() }:${ encodeFunctionData({ abi: address === token ? ERC20Artifact.abi : NftArtifact.abi, functionName, args }) }`;
}

function setRead(address: Address, functionName: string, args: Array<unknown>, value: Hex | { code: number; message: string }) {
  allowanceReads.set(readKey(address, functionName, args), value);
}

function tokenMetadata(address: string): TokenInfo {
  return {
    address_hash: address, type: address.toLowerCase() === nft.toLowerCase() ? 'ERC-721' : 'ERC-20',
    decimals, symbol: 'WHOLE', name: 'Whole units', exchange_rate: '2.5', total_supply: '1000',
    icon_url: null, reputation: 'ok', holders_count: '1', circulating_market_cap: null,
  };
}

function recordedReceipt(): RpcTransactionReceipt {
  return {
    transactionHash: hash, transactionIndex: '0x0', blockHash: toHex(7, { size: 32 }), blockNumber: '0x7',
    from: owner, to: token, contractAddress: null, cumulativeGasUsed: '0x10000', gasUsed: '0x10000',
    effectiveGasPrice: '0x3b9aca00', logs: [], logsBloom: `0x${ '00'.repeat(256) }`, status: receiptReverted ? '0x0' : '0x1', type: '0x2',
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
  const failure = (message: string) => ({ jsonrpc: '2.0', id, error: { code: -32000, message } });
  if (method === 'eth_blockNumber') return blockFailure ? failure('Recorded provider unavailable') : success('0x9');
  if (method === 'eth_chainId') return success(toHex(foundry.id));
  if (method === 'eth_accounts') return success([ owner ]);
  if (method === 'eth_getLogs') {
    const filter = params[0] as { topics?: Array<Hex | null> };
    return success(logs.filter((log) => filter.topics?.every((topic, index) => !topic || topic.toLowerCase() === log.topics[index]?.toLowerCase())));
  }
  if (method === 'eth_call') {
    const call = params[0] as { to: Address; data: Hex };
    const result = allowanceReads.get(`${ call.to.toLowerCase() }:${ call.data.toLowerCase() }`);
    if (typeof result === 'string') return success(result);
    if (result) return { jsonrpc: '2.0', id, error: result };
  }
  if (method === 'eth_sendTransaction') {
    sentTransactions.push(params[0]);
    return success(hash);
  }
  if (method === 'eth_getTransactionReceipt') return success(recordedReceipt());
  unexpectedRequests.push(`${ method }:${ JSON.stringify(params) }`);
  return failure('Unrecorded request');
}

beforeAll(async() => {
  fetchMocker.disableMocks();
  server = createServer(async(request, response) => {
    let body = '';
    for await (const chunk of request) body += String(chunk);
    response.setHeader('Content-Type', 'application/json');
    const path = new URL(request.url!, 'http://localhost').pathname;
    if (path === '/rpc') {
      const payload = JSON.parse(body);
      const requests = Array.isArray(payload) ? payload : [ payload ];
      if (!requests.every(isRpcRequest)) throw new Error('Invalid JSON-RPC request received by the transcript server');
      const results = requests.map(rpcResponse);
      response.end(JSON.stringify(Array.isArray(payload) ? results : results[0]));
      return;
    }
    apiCalls.push(path);
    if (path.endsWith('/token-balances')) {
      const balances: AddressTokenBalancesResponse = [ {
        token: tokenMetadata(token.toLowerCase()), value: '7', token_id: null, token_instance: null,
      } ];
      response.end(JSON.stringify(balances));
    } else if (path.includes('/tokens/')) {
      if (metadataMissing) response.statusCode = 503;
      response.end(JSON.stringify(metadataMissing ? { message: 'Metadata unavailable' } : tokenMetadata(path.split('/').at(-1)!)));
    } else if (path.includes('/blocks/')) {
      const block = Number(path.split('/').at(-1));
      response.end(JSON.stringify({ timestamp: new Date(block * 1000).toISOString() }));
    } else if (path === '/rewards/register') {
      registrationRequests++;
      response.end(JSON.stringify({ token: 'recorded-reward-token' }));
    } else if (path === '/rewards/confirm') {
      rewardRequests.push(JSON.parse(body));
      const complete = () => {
        if (rewardsFailures-- > 0) response.statusCode = 503;
        response.end(JSON.stringify({ accepted: response.statusCode === 200 }));
      };
      if (holdRewards) releaseRewards = complete;
      else complete();
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
  allowanceReads = new Map();
  rpcCalls = [];
  apiCalls = [];
  rewardRequests = [];
  registrationRequests = 0;
  sentTransactions = [];
  unexpectedRequests = [];
  holdRewards = false;
  releaseRewards = undefined;
  metadataMissing = false;
  decimals = '0';
  blockFailure = false;
  rewardsFailures = 0;
  receiptReverted = false;
});

afterEach(async() => {
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
  return renderHook(() => ({ erc20: useSearchErc20Allowances(), nft: useSearchNftAllowances() }), { wrapper });
}

async function erc20Events() {
  return publicClient.getLogs({
    event: parseAbiItem('event Approval(address indexed owner, address indexed spender, uint256 value)'),
    args: { owner }, fromBlock: BigInt(0), toBlock: BigInt(9),
  });
}

function permissionReadCount(address: Address, functionName: string, args: Array<unknown>) {
  const key = readKey(address, functionName, args);
  return rpcCalls.filter((call) => {
    if (call.method !== 'eth_call') return false;
    const params = call.params[0] as { to: Address; data: Hex };
    return `${ params.to.toLowerCase() }:${ params.data.toLowerCase() }` === key;
  }).length;
}

async function approvalsFalseEmptyRetainsQueryAndRetry() {
  blockFailure = true;
  const { result } = renderHook(() => {
    const erc20 = useSearchErc20Allowances();
    const nftSearch = useSearchNftAllowances();
    return useQuery(approvalsQueryOptions(chain, owner, publicClient, erc20, nftSearch));
  }, { wrapper });
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(result.current.data).toBeUndefined();
  expect(result.current.error?.message).toContain('Recorded provider unavailable');
  blockFailure = false;
  await act(async() => {
    await result.current.refetch();
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data).toEqual([]);
  expect(queryClient.getQueryCache().getAll().map((query) => query.queryKey)).toEqual([ [ 'revoke:approvals', chain.id, owner ] ]);
}

async function approvalsMissingClientIsError() {
  const { result } = renderHook(() => useApprovalsQuery(undefined, owner), { wrapper });
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(result.current.error?.message).toBe('Public client not found');
  expect(result.current.coverage).toBe('incomplete');
  expect(result.current.data).toBeUndefined();
  expect(result.current.retry).toBe(result.current.refetch);
}

async function erc20ScanErrorsPreserveRowsAndPartialCoverage() {
  logs = [ approvalLog(token, 1, 0, spender), approvalLog(token, 2, 0, other) ];
  metadataMissing = true;
  setRead(token, 'allowance', [ owner, spender ], word(BigInt(5)));
  setRead(token, 'allowance', [ owner, other ], { code: -32000, message: 'Recorded allowance read unavailable' });
  const { result } = renderHook(() => {
    const erc20 = useSearchErc20Allowances();
    const nftSearch = useSearchNftAllowances();
    return useQuery(approvalsQueryOptions(chain, owner, publicClient, erc20, nftSearch));
  }, { wrapper });
  await waitFor(() => expect(result.current.error).toBeInstanceOf(ApprovalScanError));
  const error = result.current.error as ApprovalScanError;
  expect(error.retryable).toBe(true);
  expect(error.coverage).toBe('incomplete');
  expect(error.partialAllowances).toHaveLength(1);
  expect(error.partialAllowances[0]).toMatchObject({ address: token, spender, type: 'ERC-20' });
  expect(error.partialAllowances[0].name).toBeUndefined();
  expect(error.partialAllowances[0].allowance).toBeUndefined();
  expect(error.issues.map((issue) => issue.kind).sort()).toEqual([ 'metadata', 'permission' ]);
  expect(error.issues.find((issue) => issue.kind === 'permission')).toMatchObject({ address: token, spender: other });

  metadataMissing = false;
  setRead(token, 'allowance', [ owner, other ], word(BigInt(0)));
  await act(async() => {
    await result.current.refetch();
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data).toHaveLength(1);
  expect(result.current.data?.[0]).toMatchObject({ name: 'Whole units', allowance: '5' });
}

async function approvalEventRecencyFollowsGrantChangeRevokeGrant() {
  const erc20Sequence = [ 5, 8, 0, 5 ].map((value, index) => ({
    ...approvalLog(token, index + 1, 0, spender), data: word(BigInt(value)),
  }));
  const operatorSequence = [ 1, 2, 3, 4 ].map((block) => operatorLog(block, 0, block !== 3));
  const lastErc20 = approvalLog(token, 4, 3, spender);
  const lastOperator = operatorLog(4, 2, true);
  logs = [ lastOperator, ...erc20Sequence, ...operatorSequence, lastErc20 ];
  setRead(token, 'allowance', [ owner, spender ], word(BigInt(5)));
  setRead(nft, 'isApprovedForAll', [ owner, spender ], boolWord(true));
  const { result } = discovery();
  const events = await erc20Events();
  const erc20 = await result.current.erc20(chain, owner, events.filter((event) => getAddress(event.address) === token), publicClient);
  const nftRows = await result.current.nft(chain, owner, events, publicClient, BigInt(9));
  expect(erc20.issues).toEqual([]);
  expect(erc20.allowances).toHaveLength(1);
  expect(erc20.allowances[0]).toMatchObject({ transactionId: lastErc20.transactionHash, timestamp: 4000 });
  expect(nftRows).toHaveLength(1);
  expect(nftRows[0]).toMatchObject({ transactionId: lastOperator.transactionHash, timestamp: 4000, spender });
  expect(permissionReadCount(token, 'allowance', [ owner, spender ])).toBe(1);
  expect(permissionReadCount(nft, 'isApprovedForAll', [ owner, spender ])).toBe(1);
}

async function erc20BalanceNormalizationAndZeroDecimalsPreserveRisk() {
  logs = [ approvalLog(token, 4, 0, spender) ];
  expect(token).not.toBe(token.toLowerCase());
  setRead(token, 'allowance', [ owner, spender ], word(BigInt(5)));
  const { result } = discovery();
  const scan = await result.current.erc20(chain, owner, await erc20Events(), publicClient);
  expect(scan.issues).toEqual([]);
  expect(scan.allowances).toHaveLength(1);
  expect(scan.allowances[0]).toMatchObject({ address: token, decimals: 0, price: '2.5', allowance: '5', valueAtRiskUsd: 12.5 });
  expect(apiCalls).toContain(`/api/v2/addresses/${ owner }/token-balances`);
}

async function erc20InvalidDecimalsDoNotInventScale() {
  logs = [ approvalLog(token, 1, 0, spender) ];
  setRead(token, 'allowance', [ owner, spender ], word(BigInt(5)));
  const { result } = discovery();
  const events = await erc20Events();
  for (const value of [ null, '', '-1', '1.5', '256' ]) {
    decimals = value;
    const scan = await result.current.erc20(chain, owner, events, publicClient);
    expect(scan.allowances).toHaveLength(1);
    expect(scan.allowances[0].decimals).toBeUndefined();
    expect(scan.allowances[0].allowance).toBeUndefined();
    expect(scan.allowances[0].valueAtRiskUsd).toBeUndefined();
  }
}

async function nftTokenDeduplicationKeepsEveryTokenAndLatestEvent() {
  const latest = approvalLog(nft, 4, 2, spender, BigInt(17));
  logs = [
    approvalLog(nft, 1, 0, spender, BigInt(0)), approvalLog(nft, 1, 1, spender, BigInt(17)),
    latest, approvalLog(nft, 4, 1, other, BigInt(17)), approvalLog(nft, 3, 0, ZERO_ADDRESS, BigInt(17)),
  ];
  for (const tokenId of [ BigInt(0), BigInt(17) ]) {
    setRead(nft, 'ownerOf', [ tokenId ], addressWord(owner));
    setRead(nft, 'getApproved', [ tokenId ], addressWord(spender));
  }
  const { result } = discovery();
  const rows = await result.current.nft(chain, owner, await erc20Events(), publicClient, BigInt(9));
  expect(rows).toHaveLength(2);
  expect(rows.map((row) => row.tokenId).sort()).toEqual([ '0', '17' ]);
  expect(rows.find((row) => row.tokenId === '17')).toMatchObject({ spender, transactionId: latest.transactionHash, timestamp: 4000 });
  expect(permissionReadCount(nft, 'getApproved', [ BigInt(0) ])).toBe(1);
  expect(permissionReadCount(nft, 'getApproved', [ BigInt(17) ])).toBe(1);
}

async function nftCurrentOwnerRejectsTransferredAndReapprovedToken() {
  const newOwnerApproval = {
    ...approvalLog(nft, 4, 0, spender, BigInt(17)),
    topics: encodeEventTopics({
      abi: NftArtifact.abi, eventName: 'Approval', args: { owner: other, approved: spender, tokenId: BigInt(17) },
    }) as [Hex, ...Array<Hex>],
  };
  const transfer = {
    ...approvalLog(nft, 3, 0, other, BigInt(17)),
    topics: encodeEventTopics({
      abi: NftArtifact.abi, eventName: 'Transfer', args: { from: owner, to: other, tokenId: BigInt(17) },
    }) as [Hex, ...Array<Hex>],
  };
  logs = [ approvalLog(nft, 1, 0, spender, BigInt(17)), approvalLog(nft, 2, 0, other, BigInt(0)), transfer, newOwnerApproval ];
  setRead(nft, 'ownerOf', [ BigInt(17) ], addressWord(other));
  setRead(nft, 'getApproved', [ BigInt(17) ], addressWord(spender));
  setRead(nft, 'ownerOf', [ BigInt(0) ], addressWord(owner));
  setRead(nft, 'getApproved', [ BigInt(0) ], addressWord(other));
  const { result } = discovery();
  const rows = await result.current.nft(chain, owner, await erc20Events(), publicClient, BigInt(9));
  expect(rows).toHaveLength(1);
  expect(rows[0]).toMatchObject({ tokenId: '0', spender: other });
  expect(permissionReadCount(nft, 'ownerOf', [ BigInt(17) ])).toBe(1);
  expect(permissionReadCount(nft, 'getApproved', [ BigInt(17) ])).toBe(0);
}

async function scanCancellationAndNftReadErrorsAreNotEmptySuccess() {
  const { result } = discovery();
  const controller = new AbortController();
  controller.abort();
  await expect(result.current.erc20(chain, owner, [], publicClient, controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
  await expect(result.current.nft(chain, owner, [], publicClient, BigInt(9), controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
  logs = [ approvalLog(nft, 1, 0, spender, BigInt(17)) ];
  setRead(nft, 'ownerOf', [ BigInt(17) ], { code: -32000, message: 'Recorded owner read unavailable' });
  await expect(result.current.nft(chain, owner, await erc20Events(), publicClient, BigInt(9))).rejects.toThrow('Recorded owner read unavailable');
}

function submissionFixture() {
  const walletClient = createWalletClient({ chain: foundry, transport: http(`${ endpoint }/rpc`, { retryCount: 0 }) });
  const approval: AllowanceType = {
    type: 'ERC-20', address: token, spender, transactionId: null, tokenReputation: null, timestamp: 0,
  };
  const states: Array<RevokeSubmissionState> = [];
  const submission = new RevokeSubmission({
    submit: () => walletClient.writeContract({ ...getRevokeContractParameters(approval), account: owner, chain: foundry }),
    receipt: (submittedHash) => publicClient.waitForTransactionReceipt({ hash: submittedHash }),
    verify: async(receipt) => {
      expect(receipt.transactionHash).toBe(hash);
      expect(receipt.blockNumber).toBe(BigInt(7));
    },
    registerRewards: async() => {
      const response = await fetch(`${ endpoint }/rewards/register`, { method: 'POST' });
      if (!response.ok) throw new Error('Rewards registration unavailable');
      return response.json() as Promise<{ token: string }>;
    },
    confirmRewards: async(submittedHash, rewardsToken, signal) => {
      const response = await fetch(`${ endpoint }/rewards/confirm`, {
        method: 'POST', signal, headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tx_hash: submittedHash, token: rewardsToken }),
      });
      if (!response.ok) throw new Error('Rewards confirmation unavailable');
      return response.json();
    },
  }, (state) => states.push(state));
  return { submission, states };
}

async function revokeRewardsLifecycleRetainsHashAndRetriesWithoutResubmission() {
  rewardsFailures = 1;
  const { submission, states } = submissionFixture();
  expect(await submission.submit()).toBe(true);
  expect(await submission.rewardsDelivery).toBe(false);
  expect(submission.state).toMatchObject({ hash, status: 'confirmed', rewards: 'failed', rewardsAttempts: 1 });
  expect(states.some((state) => state.hash === hash && state.status === 'pending')).toBe(true);
  expect(submission.state.rewardsError).toBeInstanceOf(Error);
  expect(sentTransactions).toHaveLength(1);
  const transaction = sentTransactions[0] as { from: Address; to: Address; data: Hex };
  expect(getAddress(transaction.from)).toBe(owner);
  expect(getAddress(transaction.to)).toBe(token);
  expect(transaction.data).toBe(encodeFunctionData({ abi: ERC20Artifact.abi, functionName: 'approve', args: [ spender, BigInt(0) ] }));
  expect(await submission.retryRewards()).toBe(true);
  expect(submission.state).toMatchObject({ hash, status: 'confirmed', rewards: 'delivered', rewardsAttempts: 2 });
  expect(await submission.submit()).toBe(true);
  expect(await submission.retryRewards()).toBe(false);
  expect(sentTransactions).toHaveLength(1);
  expect(registrationRequests).toBe(1);
  expect(rewardRequests).toEqual([
    { tx_hash: hash, token: 'recorded-reward-token' }, { tx_hash: hash, token: 'recorded-reward-token' },
  ]);
}

async function revokeRewardsRetriesAreBoundedAndReceiptIsAuthoritative() {
  rewardsFailures = 4;
  receiptReverted = true;
  const { submission } = submissionFixture();
  expect(await submission.submit()).toBe(false);
  expect(await submission.rewardsDelivery).toBe(false);
  expect(await submission.retryRewards()).toBe(false);
  expect(await submission.retryRewards()).toBe(false);
  expect(submission.state).toMatchObject({ hash, status: 'failed', rewards: 'failed', rewardsAttempts: 3 });
  expect(submission.state.error).toBeInstanceOf(Error);
  expect(await submission.retryRewards()).toBe(false);
  expect(rewardRequests).toHaveLength(3);
  expect(sentTransactions).toHaveLength(1);
}

async function revokeReceiptCompletesWhileRewardsPending() {
  holdRewards = true;
  const { submission } = submissionFixture();
  try {
    expect(await submission.submit()).toBe(true);
    await waitFor(() => expect(rewardRequests).toHaveLength(1));
    expect(submission.state).toMatchObject({ hash, status: 'confirmed', rewards: 'pending' });
    expect(sentTransactions).toHaveLength(1);
  } finally {
    releaseRewards?.();
    await submission.rewardsDelivery;
  }
  expect(submission.state.rewards).toBe('delivered');
}

test(approvalsFalseEmptyRetainsQueryAndRetry);
test(approvalsMissingClientIsError);
test(erc20ScanErrorsPreserveRowsAndPartialCoverage);
test(approvalEventRecencyFollowsGrantChangeRevokeGrant);
test(erc20BalanceNormalizationAndZeroDecimalsPreserveRisk);
test(erc20InvalidDecimalsDoNotInventScale);
test(nftTokenDeduplicationKeepsEveryTokenAndLatestEvent);
test(nftCurrentOwnerRejectsTransferredAndReapprovedToken);
test(scanCancellationAndNftReadErrorsAreNotEmptySuccess);
test(revokeRewardsLifecycleRetainsHashAndRetriesWithoutResubmission);
test(revokeRewardsRetriesAreBoundedAndReceiptIsAuthoritative);

test(revokeReceiptCompletesWhileRewardsPending);
