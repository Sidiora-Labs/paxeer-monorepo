// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, render, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import { createElement, type ReactNode } from 'react';
import {
  createPublicClient, decodeFunctionData, encodeAbiParameters, encodeErrorResult, encodeEventTopics,
  encodeFunctionResult, getAddress, hexToBytes, http, keccak256, multicall3Abi, numberToHex,
  toFunctionSelector, zeroAddress, type Abi, type Address, type Hex,
} from 'viem';
import { afterAll, afterEach, expect, test, vi } from 'vitest';
import { WagmiProvider } from 'wagmi';

import { readConfig } from './config';
import { bankAbi, BANK_ADDRESS, layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from './contracts';
import { contentSecurityPolicy } from './csp';
import { paxeerRecipient } from './digests';
import depositBundle from './fixtures/deposit-ready.json';
import releaseBundle from './fixtures/release-ready.json';
import {
  base58Decode, base58Encode, recipientRecordAddress, solanaRecipientHandle,
} from './solana';
import { settlementBroadcastPatch, settlementPatch } from './settlement';
import { historyStatusOf, phaseOf } from './phase';
import { depositRecord, withdrawalRecord } from './submitted';
import { listTransfers, saveTransfer, transferIdOf, updateTransfer, type BridgeTransfer } from './transfers';
import './phase.spec';

const account = getAddress('0x2222222222222222222222222222222222222222');
const vault = getAddress(depositBundle.vault);
const programId = base58Encode(Uint8Array.from({ length: 32 }, (_, i) => i + 1));
const destination = 'So11111111111111111111111111111111111111112';
const handle = solanaRecipientHandle(destination)!;
const recipientPda = recipientRecordAddress(programId, handle)!;
const storageKey = 'paxeer-bridge:transfers:v1';
const sourceHash = depositBundle.txHash as Hex;
const burnHash = releaseBundle.paxeerTxHash as Hex;
const attemptHash: Hex = `0x${ '44'.repeat(32) }`;

// These peers use the committed vault/precompile ABI codecs and the RecipientRecord::encode
// layout in contracts/bridge/solana/src/state.rs. Only wire data crosses the loopback boundary.
function depositEvent(nonce: bigint, asset: Address = zeroAddress) {
  return { address: vault, topics: encodeEventTopics({ abi: paxeerXVaultAbi, eventName: 'BridgeDeposit',
    args: { asset, sender: account, paxeerRecipient: paxeerRecipient(account) } }),
  data: encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ 100n, nonce ]) };
}

function burnEvent(nonce: bigint, chain = 8453n, recipient: Address = account, asset: Address = zeroAddress) {
  return { address: LAYERX_BRIDGE_ADDRESS, topics: encodeEventTopics({ abi: layerxBridgeAbi, eventName: 'BridgeOut',
    args: { chain, asset, nonce } }),
  data: encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ 100n, recipient ]) };
}

function wireReceipt(hash: Hex, logs: Array<ReturnType<typeof depositEvent>>, status = '0x1') {
  return { transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: keccak256('0x10'),
    from: account, to: vault, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0', effectiveGasPrice: '0x1',
    contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status,
    logs: logs.map((log, index) => ({ ...log, transactionHash: hash, transactionIndex: '0x0',
      blockHash: keccak256('0x10'), blockNumber: '0x10', logIndex: numberToHex(index + 7), removed: false })) };
}

function recipientAccount() {
  const bytes = new Uint8Array(62);
  bytes.set(new TextEncoder().encode('PXBRREC0'));
  new DataView(bytes.buffer).setUint16(8, 1, false);
  bytes.set(hexToBytes(handle), 10);
  bytes.set(base58Decode(destination)!, 30);
  return { owner: programId, executable: false, data: [ Buffer.from(bytes).toString('base64'), 'base64' ] };
}

const receipts = new Map<Hex, ReturnType<typeof wireReceipt>>();
const urls: string[] = [];
const calls: Array<{ method: string; params: unknown[] }> = [];
const serverErrors: unknown[] = [];
let custodyAccount: unknown = recipientAccount();
let paxeerPaused = false;
let pairEnabled = true;
let vaultPaused = false;
let nullified = false;
let caps: readonly [bigint, bigint] = [ 1000n, 10000n ];
let depositRevert: Hex | undefined;
let holdPaxeer: Promise<void> | undefined;
let holdVault: Promise<void> | undefined;

async function contractRead(call: { to: Address; data: Hex }): Promise<Hex> {
  if (call.data.startsWith('0x82ad56cb')) {
    const decoded = decodeFunctionData({ abi: multicall3Abi, data: call.data });
    if (decoded.functionName !== 'aggregate3') throw new Error('Expected aggregate3.');
    const result = await Promise.all(decoded.args[0].map(async(entry) => ({ success: true,
      returnData: await contractRead({ to: entry.target, data: entry.callData }) })));
    return encodeFunctionResult({ abi: multicall3Abi, functionName: 'aggregate3', result });
  }
  const paxeerCall = call.to.toLowerCase() === LAYERX_BRIDGE_ADDRESS;
  if (paxeerCall && holdPaxeer) await holdPaxeer;
  if (!paxeerCall && holdVault) await holdVault;
  const abi: Abi = paxeerCall ? layerxBridgeAbi : call.to.toLowerCase() === BANK_ADDRESS ? bankAbi : paxeerXVaultAbi;
  const { functionName } = decodeFunctionData({ abi, data: call.data });
  const results: Record<string, unknown> = {
    isPaused: paxeerPaused, getChain: [ true, vault, 1n, pairEnabled ],
    getCap: [ 'ubridged', 10000n, 1000n, 0n ], getAttestors: [ [ account ], [ 1n ], 1 ],
    paused: vaultPaused, caps, outstanding: 500n, attestors: [ account ], threshold: 1n,
    isNullified: nullified, nullified, balance: 10000n,
  };
  if (!(functionName in results)) throw new Error(`Unexpected contract read ${ functionName }.`);
  return encodeFunctionResult({ abi, functionName, result: results[functionName] });
}

async function dispatch(input: { id: number; method: string; params: unknown[] }) {
  const { id, method, params } = input;
  calls.push({ method, params });
  let result: unknown;
  switch (method) {
    case 'eth_call': {
      const call = params[0] as { to: Address; data: Hex };
      if (depositRevert && call.to.toLowerCase() === vault.toLowerCase() &&
        call.data.slice(0, 10) === toFunctionSelector('depositNative(bytes32)')) {
        return { jsonrpc: '2.0', id, error: { code: 3, message: 'execution reverted', data: depositRevert } };
      }
      result = await contractRead(call);
      break;
    }
    case 'eth_getTransactionReceipt': result = receipts.get(params[0] as Hex) ?? null; break;
    case 'eth_getTransactionByHash': result = null; break;
    case 'eth_blockNumber': result = '0x20'; break;
    case 'eth_chainId': result = '0x7d'; break;
    case 'getAccountInfo':
      if (params[0] !== recipientPda.address) throw new Error('Recipient lookup used the wrong PDA.');
      result = { context: { slot: 32 }, value: custodyAccount };
      break;
    default: throw new Error(`Unexpected RPC method ${ method }.`);
  }
  return { jsonrpc: '2.0', id, result };
}

const server = createServer(async(incoming, outgoing) => {
  try {
    outgoing.setHeader('Content-Type', 'application/json');
    if (incoming.method === 'GET') {
      urls.push(incoming.url!);
      outgoing.end(JSON.stringify(incoming.url!.includes('/v1/deposit/') ? depositBundle : releaseBundle));
      return;
    }
    let body = '';
    for await (const chunk of incoming) body += chunk.toString();
    const input = JSON.parse(body);
    outgoing.end(JSON.stringify(Array.isArray(input) ? await Promise.all(input.map(dispatch)) : await dispatch(input)));
  } catch (error) {
    serverErrors.push(error);
    outgoing.statusCode = 500;
    outgoing.end(JSON.stringify({ error: String(error) }));
  }
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
const listener = server.address();
if (!listener || typeof listener === 'string') throw new Error('The loopback listener did not open.');
const rpcUrl = `http://127.0.0.1:${ listener.port }`;
const deployment = {
  NEXT_PUBLIC_PAXEER_RPC_URL: rpcUrl, NEXT_PUBLIC_BRIDGE_RPC_BASE: rpcUrl,
  NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: rpcUrl, NEXT_PUBLIC_BRIDGE_RPC_SOLANA: rpcUrl,
  NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault, NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: vault,
  NEXT_PUBLIC_BRIDGE_API_URL: rpcUrl, NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: programId,
};
const previousEnv = Object.fromEntries(Object.keys(deployment).map((key) => [ key, process.env[key] ]));
Object.assign(process.env, deployment);
// config is a deployment-time singleton; load consumers after providing this self-contained deployment.
vi.resetModules();
const { bridgeChainByKey, buildBridgeChains, paxeer } = await import('./chains');
const { createBridgeWagmiConfig, wagmiConfig } = await import('./wagmi');
const { importDeposit, importWithdrawal, importWithdrawalResolved, matchDepositLog, nonceFromBridgeOutLogs } = await import('./import');
const { enrichRecord } = await import('./enrich');
const { fetchDepositAttestation, fetchReleaseAttestation } = await import('./attestation-api');
const { usePaxeerPairState, useVaultState } = await import('./use-bridge-state');
const { cardState } = await import('./card-state');
const { TransfersPanel, transferHistoryRecords } = await import('@/components/bridge/transfers-panel');
const { readTransferLive } = await import('@/components/bridge/transfer-actions');
const { TooltipProvider } = await import('@/components/ui/tooltip');
const publicClient = createPublicClient({ transport: http(rpcUrl, { retryCount: 0 }) });
const foreign = bridgeChainByKey('base')!;
const asset = foreign.assets[0];
const originalWalletState = wagmiConfig.state;
const clients: QueryClient[] = [];

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  clients.push(client);
  return createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
    createElement(QueryClientProvider, { client }, createElement(TooltipProvider, null, children)));
}

function routeCard(paxeerState: ReturnType<typeof usePaxeerPairState>, vaultState: ReturnType<typeof useVaultState>) {
  return cardState({ direction: 'in', foreign, asset, amount: '0.0000000000000001', balance: 1000n,
    gasReserve: 0n, paxeerState, vaultState, paxeerConfigured: true, attestationApiConfigured: true,
    allowanceError: false, isConnected: true, onSourceChain: true, destinationValid: true,
    depositPlanAvailable: true, busy: false });
}

function useRouteCard() {
  const paxeerState = usePaxeerPairState(foreign, asset.assetId);
  const vaultState = useVaultState(foreign, asset.address);
  return { paxeerState, vaultState, card: routeCard(paxeerState, vaultState) };
}

afterEach(() => {
  cleanup();
  for (const client of clients) client.clear();
  clients.length = 0;
  wagmiConfig.setState(originalWalletState);
  window.localStorage.clear();
  receipts.clear();
  urls.length = 0;
  calls.length = 0;
  custodyAccount = recipientAccount();
  paxeerPaused = false;
  pairEnabled = true;
  vaultPaused = false;
  nullified = false;
  caps = [ 1000n, 10000n ];
  depositRevert = undefined;
  holdPaxeer = undefined;
  holdVault = undefined;
});
afterAll(async() => {
  for (const [ key, value ] of Object.entries(previousEnv)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  expect(serverErrors).toEqual([]);
});

async function apiBaseUrlRoutesPreserveQueryAndExcludeFragment() {
  for (const suffix of [ '?key=a%2Fb&key=c', '#deployment', '?key=a%2Fb#deployment' ]) {
    const config = readConfig({ NEXT_PUBLIC_BRIDGE_API_URL: `${ rpcUrl }/backend/${ suffix }` });
    expect(config.attestationApiUrl).toBeDefined();
    const deposit = await fetchDepositAttestation({ chainId: 1n, txHash: sourceHash, logIndex: 7n }, config.attestationApiUrl);
    const release = await fetchReleaseAttestation({ chainId: 1n, paxeerTxHash: burnHash, paxeerNonce: 3n }, config.attestationApiUrl);
    expect(deposit?.digest).toBe(depositBundle.digest);
    expect(release?.digest).toBe(releaseBundle.digest);
    for (const [ offset, route ] of [ [ -2, `deposit/1/${ sourceHash }/7` ], [ -1, `release/1/${ burnHash }/3` ] ] as const) {
      const actual = new URL(urls.at(offset)!, rpcUrl);
      expect(actual.pathname).toBe(`/backend/v1/${ route }`);
      expect(actual.search).toBe(new URL(`${ rpcUrl }/${ suffix }`).search);
      expect(actual.hash).toBe('');
    }
  }
}

function cspAllowsTheEffectiveDefaultRpcForAVaultRoute() {
  const config = readConfig({ NEXT_PUBLIC_PAXEER_RPC_URL: rpcUrl, NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault });
  const route = buildBridgeChains(config).find((chain) => chain.key === 'base')!;
  expect(route.state).toBe('configured');
  expect(config.foreignRpcUrls.base).toBeUndefined();
  const actualClient = createBridgeWagmiConfig(config).getClient({ chainId: 8453 });
  expect(actualClient.transport.url).toBe(route.viemChain!.rpcUrls.default.http[0]);
  const meta = document.createElement('meta');
  meta.httpEquiv = 'Content-Security-Policy';
  meta.content = contentSecurityPolicy(config);
  document.head.append(meta);
  const connect = meta.content.split('; ').find((entry) => entry.startsWith('connect-src '))!.split(' ').slice(1);
  expect(connect).toContain(new URL(actualClient.transport.url as string).origin);
  expect(connect).not.toContain('*');
  expect(connect).not.toContain('https:');
  expect(connect).not.toContain('https://unconfigured.invalid');
  const explicit = readConfig({ NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault, NEXT_PUBLIC_BRIDGE_RPC_BASE: rpcUrl });
  const overriddenClient = createBridgeWagmiConfig(explicit).getClient({ chainId: 8453 });
  expect(overriddenClient.transport.url).toBe(rpcUrl);
  expect(contentSecurityPolicy(explicit)).toContain(new URL(rpcUrl).origin);
  expect(contentSecurityPolicy(explicit).split('; ').find((entry) => entry.startsWith('connect-src ')))
    .not.toContain(new URL(route.viemChain!.rpcUrls.default.http[0]).origin);
  meta.remove();
}

async function registryDepositShapeFeedsTheRealEnrichmentConsumer() {
  receipts.set(sourceHash, wireReceipt(sourceHash, [ depositEvent(9n) ]));
  const receipt = await publicClient.getTransactionReceipt({ hash: sourceHash });
  const submitted = depositRecord({ hash: sourceHash, foreign, asset, account, recipient: account,
    amount: 100n, vault, requiredConfirmations: 1 });
  const parsed = matchDepositLog(submitted, receipt.logs);
  expect(parsed).toEqual({ ...receipt.logs[0], eventName: 'BridgeDeposit', args: {
    asset: zeroAddress, amount: 100n, sender: account, paxeerRecipient: paxeerRecipient(account), nonce: 9n,
  } });
  expect(parsed).not.toHaveProperty('nonce');
  expect(typeof parsed!.logIndex).toBe('number');
  const enriched = enrichRecord(submitted, receipt);
  expect(enriched).toEqual({ ...submitted, logIndex: '7', nonce: '9' });
  saveTransfer(enriched);
  expect(listTransfers(account)[0]).toMatchObject({ logIndex: '7', nonce: '9' });
}

async function receiptMultipleEventsAndLegacyKeysSurviveRestart() {
  receipts.set(sourceHash, wireReceipt(sourceHash, [ depositEvent(9n), depositEvent(10n) ]));
  const receipt = await publicClient.getTransactionReceipt({ hash: sourceHash });
  expect(() => importDeposit(foreign, receipt)).toThrow('Select a BridgeDeposit log index');
  const first = importDeposit(foreign, receipt, 7);
  const second = importDeposit(foreign, receipt, 8);
  expect(first.id).not.toBe(second.id);
  expect(matchDepositLog(second, receipt.logs)?.args.nonce).toBe(10n);
  const provisional = depositRecord({ hash: sourceHash, foreign, asset, account, recipient: account,
    amount: 100n, vault, requiredConfirmations: 1 });
  expect(enrichRecord(provisional, receipt)).toBe(provisional);
  window.localStorage.setItem(storageKey, JSON.stringify([ { ...first,
    id: transferIdOf('in', sourceHash), claimedAt: 17 } ]));
  saveTransfer(second);
  const otherChain = importDeposit({ ...foreign, chainId: 1n, key: 'ethereum' }, receipt, 7);
  saveTransfer(otherChain);
  receipts.set(burnHash, wireReceipt(burnHash, [ burnEvent(42n), burnEvent(43n) ]));
  const burned = await publicClient.getTransactionReceipt({ hash: burnHash });
  expect(() => importWithdrawal(burned)).toThrow('Select a BridgeOut nonce');
  expect(nonceFromBridgeOutLogs(burned.logs)).toBeUndefined();
  expect(nonceFromBridgeOutLogs(burned.logs, 43n)).toBe(43n);
  saveTransfer(importWithdrawal(burned, 42n));
  saveTransfer(importWithdrawal(burned, 43n));
  const mounted = renderHook(() => listTransfers(account));
  expect(mounted.result.current).toHaveLength(5);
  mounted.unmount();
  vi.resetModules();
  const reopened = await import('./transfers');
  const recovered = renderHook(() => reopened.listTransfers(account));
  expect(new Set(recovered.result.current.map((entry) => entry.id)).size).toBe(5);
  expect(recovered.result.current.find((entry) => entry.id === first.id)).toMatchObject({ nonce: '9', claimedAt: 17 });
  expect(recovered.result.current.find((entry) => entry.id === second.id)).toMatchObject({ nonce: '10', logIndex: '8' });
  expect(recovered.result.current.filter((entry) => entry.direction === 'out').map((entry) => entry.bridgeOutNonce).sort())
    .toEqual([ '42', '43' ]);
  reopened.updateTransfer(second.id, { claimedAt: 23 });
  expect(reopened.listTransfers(account).find((entry) => entry.id === first.id)?.claimedAt).toBe(17);
  expect(reopened.listTransfers(account).find((entry) => entry.id === second.id)?.claimedAt).toBe(23);
  reopened.updateTransfer(transferIdOf('in', sourceHash), { claimedAt: 999 });
  expect(reopened.listTransfers(account).some((entry) => entry.claimedAt === 999)).toBe(false);
  expect(JSON.parse(window.localStorage.getItem(storageKey)!).map((entry: BridgeTransfer) => entry.id))
    .toEqual(reopened.listTransfers(account).map((entry) => entry.id));
}

async function solanaBurnRecoveryResolvesAndValidatesTheCustodyRecipient() {
  const solana = bridgeChainByKey('solana')!;
  const solanaAsset = solana.assets[0];
  receipts.set(burnHash, wireReceipt(burnHash, [ burnEvent(3n, solana.chainId, handle, solanaAsset.assetId) ]));
  const receipt = await publicClient.getTransactionReceipt({ hash: burnHash });
  expect(() => importWithdrawal(receipt)).toThrow('custody recipient lookup');
  const config = { programId, rpcUrl };
  const imported = await importWithdrawalResolved(receipt, config);
  const submitted = withdrawalRecord({ hash: burnHash, foreign: solana, asset: solanaAsset,
    account, amount: 100n, destination, requiredConfirmations: 1 });
  expect(imported.recipient).toBe(submitted.recipient);
  expect(imported.recipientHandle?.toLowerCase()).toBe(submitted.recipientHandle?.toLowerCase());
  saveTransfer(imported);
  const mounted = renderHook(() => listTransfers(account));
  expect(mounted.result.current[0].recipient).toBe(destination);
  mounted.unmount();
  vi.resetModules();
  const reopened = await import('./transfers');
  expect(reopened.listTransfers(account)[0]).toMatchObject({ recipient: destination, recipientHandle: getAddress(handle) });
  expect(calls.find((call) => call.method === 'getAccountInfo')?.params).toEqual([
    recipientPda.address, { encoding: 'base64', commitment: 'finalized' },
  ]);
  const valid = recipientAccount();
  const corrupted = (offset: number) => {
    const bytes = Buffer.from(valid.data[0], 'base64');
    bytes[offset] ^= 1;
    return { ...valid, data: [ bytes.toString('base64'), 'base64' ] };
  };
  for (const invalid of [ null, { ...valid, owner: destination }, { ...valid, executable: true },
    corrupted(0), corrupted(9), corrupted(10), corrupted(30),
    { ...valid, data: [ Buffer.alloc(61).toString('base64'), 'base64' ] } ]) {
    custodyAccount = invalid;
    await expect(importWithdrawalResolved(receipt, config).then(reopened.saveTransfer)).rejects.toThrow();
    expect(reopened.listTransfers(account)).toEqual([ imported ]);
  }
}

async function zeroVaultCapsReachTheCardWithPaxeerXVaultSemantics() {
  // Paired caps/admission responses exercise the exact PaxeerXVault._admitDeposit guards.
  const cases = [
    { caps: [ 0n, 10000n ] as const, error: 'PerTxCapExceeded' as const, args: [ 100n, 0n ] as const,
      message: 'Above the vault per-transaction cap.' },
    { caps: [ 1000n, 0n ] as const, error: 'AssetNotEnabled' as const, args: [ zeroAddress ] as const,
      message: 'The vault asset is disabled (total capacity is zero).' },
  ];
  for (const fixture of cases) {
    caps = fixture.caps;
    depositRevert = encodeErrorResult({ abi: paxeerXVaultAbi, errorName: fixture.error, args: fixture.args });
    const mounted = renderHook(useRouteCard, { wrapper });
    await waitFor(() => expect(mounted.result.current.vaultState.loaded).toBe(true));
    expect(mounted.result.current.vaultState.perTx).toBe(fixture.caps[0]);
    expect(mounted.result.current.vaultState.total).toBe(fixture.caps[1]);
    expect(mounted.result.current.card.inputError).toBe(fixture.message);
    expect(mounted.result.current.card.ready).toBe(false);
    await expect(publicClient.simulateContract({ address: vault, abi: paxeerXVaultAbi,
      functionName: 'depositNative', args: [ paxeerRecipient(account) ], value: 100n, account }))
      .rejects.toThrow(fixture.error);
    mounted.unmount();
  }
}

async function unloadedRoutesNeverBecomeReadyBeforePausedOrDisabledReads() {
  for (const disabled of [ false, true ]) {
    let releasePaxeer!: () => void;
    let releaseVault!: () => void;
    holdPaxeer = new Promise<void>((resolve) => {
      releasePaxeer = resolve;
    });
    holdVault = new Promise<void>((resolve) => {
      releaseVault = resolve;
    });
    paxeerPaused = !disabled;
    vaultPaused = !disabled;
    pairEnabled = !disabled;
    const readyStates: boolean[] = [];
    const mounted = renderHook(() => {
      const state = useRouteCard();
      readyStates.push(state.card.ready);
      return state;
    }, { wrapper });
    expect(mounted.result.current.card.ready).toBe(false);
    expect(mounted.result.current.card.blockers).toContain('The Paxeer bridge state is loading. Wait for this route to be verified.');
    expect(mounted.result.current.card.blockers).toContain('The Base vault state is loading. Wait for custody to be verified.');
    await act(async() => {
      releasePaxeer();
    });
    await waitFor(() => expect(mounted.result.current.paxeerState.loaded).toBe(true));
    expect(mounted.result.current.vaultState.loaded).toBe(false);
    expect(mounted.result.current.card.ready).toBe(false);
    await act(async() => {
      releaseVault();
    });
    await waitFor(() => expect(mounted.result.current.vaultState.loaded).toBe(true));
    expect(mounted.result.current.card.blockers).toContain(disabled ?
      'This chain is registered but currently disabled on Paxeer.' :
      'The bridge is paused on Paxeer. No deposits or withdrawals will be accepted until it reopens.');
    expect(readyStates.every((ready) => !ready)).toBe(true);
    mounted.unmount();
  }
  holdPaxeer = undefined;
  holdVault = undefined;
  paxeerPaused = false;
  vaultPaused = false;
  pairEnabled = true;
  const verified = renderHook(useRouteCard, { wrapper });
  await waitFor(() => expect(verified.result.current.card.ready).toBe(true));
}

async function broadcastAndRevertedRecoveryKeepTheRealRetryAction() {
  const ethereum = bridgeChainByKey('ethereum')!;
  receipts.set(sourceHash, wireReceipt(sourceHash, [ depositEvent(9n, depositBundle.asset as Address) ]));
  const receipt = await publicClient.getTransactionReceipt({ hash: sourceHash });
  const chain = { ...ethereum, assets: [ { ...ethereum.assets[0], address: depositBundle.asset as Address,
    assetId: depositBundle.asset as Address } ] };
  const transfer = importDeposit(chain, receipt);
  saveTransfer(transfer);
  updateTransfer(transfer.id, settlementBroadcastPatch(transfer, attemptHash, 1001));
  const [ pending ] = listTransfers(account);
  expect(phaseOf(pending, undefined, true)).toBe('settling');
  expect(transferHistoryRecords([ pending ], [ undefined ])[0].status).toBe('processing');
  const before = renderHook(() => listTransfers(account));
  before.unmount();
  vi.resetModules();
  const reopened = await import('./transfers');
  const [ recovered ] = reopened.listTransfers(account);
  expect(phaseOf(recovered, undefined, true)).toBe('settling');
  receipts.set(attemptHash, wireReceipt(attemptHash, [], '0x0'));
  const live = await readTransferLive(recovered);
  expect(live.settlementStatus).toBe('reverted');
  const patch = settlementPatch(recovered, live, 1002);
  expect(patch).toMatchObject({ settlementStatus: 'reverted' });
  reopened.updateTransfer(recovered.id, patch!);
  expect(phaseOf(reopened.listTransfers(account)[0], undefined, true)).toBe('ready');
  const view = render(createElement(TransfersPanel, { refreshKey: 0 }), { wrapper });
  await act(async() => {});
  act(() => {
    const connector = wagmiConfig.connectors[0];
    wagmiConfig.setState({ ...originalWalletState, chainId: paxeer.id, current: connector.uid,
      connections: new Map([ [ connector.uid, { accounts: [ account ], chainId: paxeer.id, connector } ] ]), status: 'connected' });
  });
  await waitFor(() => {
    expect(view.getByText('The settlement attempt reverted. Retry to submit another attempt.')).toBeTruthy();
    expect(view.getByRole('button', { name: 'Retry settlement' }).hasAttribute('disabled')).toBe(false);
  });
  nullified = true;
  const confirmed = await readTransferLive(recovered);
  expect(phaseOf(recovered, confirmed, true)).toBe('settled');
  expect(historyStatusOf(phaseOf(recovered, confirmed, true))).toBe('completed');
}

test(apiBaseUrlRoutesPreserveQueryAndExcludeFragment.name, apiBaseUrlRoutesPreserveQueryAndExcludeFragment);
test(cspAllowsTheEffectiveDefaultRpcForAVaultRoute.name, cspAllowsTheEffectiveDefaultRpcForAVaultRoute);
test(registryDepositShapeFeedsTheRealEnrichmentConsumer.name, registryDepositShapeFeedsTheRealEnrichmentConsumer);
test(receiptMultipleEventsAndLegacyKeysSurviveRestart.name, receiptMultipleEventsAndLegacyKeysSurviveRestart);
test(solanaBurnRecoveryResolvesAndValidatesTheCustodyRecipient.name, solanaBurnRecoveryResolvesAndValidatesTheCustodyRecipient);
test(zeroVaultCapsReachTheCardWithPaxeerXVaultSemantics.name, zeroVaultCapsReachTheCardWithPaxeerXVaultSemantics);
test(unloadedRoutesNeverBecomeReadyBeforePausedOrDisabledReads.name, unloadedRoutesNeverBecomeReadyBeforePausedOrDisabledReads);
test(broadcastAndRevertedRecoveryKeepTheRealRetryAction.name, broadcastAndRevertedRecoveryKeepTheRealRetryAction);
