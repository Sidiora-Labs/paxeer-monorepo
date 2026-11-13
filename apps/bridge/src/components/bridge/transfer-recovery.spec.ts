// @vitest-environment jsdom

import { bridgeChainByKey, paxeer } from '@/lib/bridge/chains';
import { layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from '@/lib/bridge/contracts';
import { releaseNullifier } from '@/lib/bridge/digests';
import { phaseOf, settlementPatch } from '@/lib/bridge/settlement';
import { base58Decode, base58Encode, solanaProgramAddress, solanaRecipientHandle } from '@/lib/bridge/solana';
import { listTransfers, saveTransfer, updateTransfer, type BridgeTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import { createElement, type PropsWithChildren } from 'react';
import {
  decodeFunctionData, encodeAbiParameters, encodeEventTopics, encodeFunctionData, encodeFunctionResult,
  hexToBytes, keccak256, parseTransaction, zeroAddress, zeroHash,
  type Address, type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { createConfig, http, WagmiProvider } from 'wagmi';
import { getTransactionReceipt } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';

import { afterEach, expect, test } from 'vitest';

import { recordBridgeSubmission } from './bridge-card';
import { importTransferReceipt, recoverTransferReceipt } from './import-transfer';
import { readTransferLive, type TransferLive, type TransferReadSettings } from './transfer-actions';
import { listBridgeTransfers } from './transfer-ledger';
import { TransferObservation, TransferRow, transferFinalityEstimate, transferHistoryRecords, transferLiveOptions } from './transfers-panel';

const wallet = privateKeyToAccount(`0x${ '12'.repeat(32) }`);
const vault: Address = '0x2222222222222222222222222222222222222222';
const recipient: Address = '0x4444444444444444444444444444444444444444';
const programId = 'A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9';
const solanaDestination = '59TLtNdRpCZysEHkGDMPFHkHiHXkBqQVqAVxAupzNoQb';
const storageKey = 'paxeer-bridge:transfers:v1';

function route(key: string) {
  const chain = bridgeChainByKey(key);
  if (!chain) throw new Error(`Missing production route ${ key }.`);
  return chain;
}

/** In-process RPC peer: ABI-encoded receipt transcripts and custody account snapshots. */
async function rpcPeer() {
  const receipts = new Map<Hex, unknown>();
  const accounts = new Map<string, { owner: string; executable: boolean; data: [string, string] }>();
  const requests: Array<{ method: string; params: Array<unknown> }> = [];
  const failures: Array<unknown> = [];
  const nullifiers = new Set<string>();
  let failingMethod: string | undefined;
  let failingFunction: string | undefined;
  let head = '0x10';
  let responseId = 1;
  const server = createServer(async(incoming, outgoing) => {
    try {
      let body = '';
      for await (const chunk of incoming) body += chunk.toString();
      const message = JSON.parse(body) as { jsonrpc: string; id: number; method: string; params: Array<unknown> };
      expect(incoming.method).toBe('POST');
      expect(message.jsonrpc).toBe('2.0');
      requests.push(message);
      outgoing.setHeader('Content-Type', 'application/json');
      const error = () => outgoing.end(JSON.stringify({ jsonrpc: '2.0', id: message.id, error: { code: -32000, message: 'fixture RPC unavailable' } }));
      if (message.method === failingMethod) {
        error(); return;
      }
      let result: unknown;
      switch (message.method) {
        case 'eth_getTransactionReceipt': result = receipts.get(message.params[0] as Hex) ?? null; break;
        case 'eth_blockNumber': result = head; break;
        case 'eth_call': {
          const call = message.params[0] as { to: Address; data: Hex };
          const abi = call.to.toLowerCase() === LAYERX_BRIDGE_ADDRESS ? layerxBridgeAbi : paxeerXVaultAbi;
          const decoded = decodeFunctionData({ abi, data: call.data });
          if (decoded.functionName === failingFunction) {
            error(); return;
          }
          let value: unknown;
          switch (decoded.functionName) {
            case 'getChain': value = [ true, vault, 1800n, true ]; break;
            case 'nullified': value = nullifiers.has(String(decoded.args?.[0])); break;
            case 'isNullified': value = nullifiers.has(`${ decoded.args?.[1] }:${ decoded.args?.[2] }`); break;
            default: throw new Error(`Unexpected contract read ${ decoded.functionName }.`);
          }
          result = encodeFunctionResult({ abi, functionName: decoded.functionName, result: value });
          break;
        }
        case 'getAccountInfo': {
          expect(message.params[1]).toEqual({ encoding: 'base64', commitment: 'finalized' });
          result = { context: { slot: 32 }, value: accounts.get(String(message.params[0])) ?? null };
          break;
        }
        default: throw new Error(`Unexpected RPC method ${ message.method }.`);
      }
      outgoing.end(JSON.stringify({ jsonrpc: '2.0', id: message.method === 'getAccountInfo' ? responseId : message.id, result }));
    } catch (error) {
      failures.push(error);
      outgoing.statusCode = 500;
      outgoing.end(String(error));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const listener = server.address();
  if (!listener || typeof listener === 'string') throw new Error('RPC listener did not open.');
  const url = `http://127.0.0.1:${ listener.port }`;
  const config = createConfig({
    chains: wagmiConfig.chains,
    transports: {
      ...Object.fromEntries(wagmiConfig.chains.map((chain) => [ chain.id, http(url, { retryCount: 0 }) ])),
      [paxeer.id]: http(url, { retryCount: 0 }),
    },
    connectors: [ injected() ], ssr: true,
  });
  const settings: TransferReadSettings = { config, solana: { programId, rpcUrl: url } };
  return {
    receipts, accounts, requests, nullifiers, config, settings,
    failMethod(method?: string) {
      failingMethod = method;
    },
    failFunction(name?: string) {
      failingFunction = name;
    },
    setHead(value: string) {
      head = value;
    },
    setResponseId(value: number) {
      responseId = value;
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      expect(failures).toEqual([]);
    },
  };
}

type Peer = Awaited<ReturnType<typeof rpcPeer>>;

/** A signed production bridgeOut call supplies the fields encoded by the contract ABI. */
async function committedBurn(peer: Peer, foreign = route('base')) {
  const asset = foreign.assets[foreign.kind === 'solana' ? 1 : 0];
  const destination = foreign.kind === 'solana' ? solanaDestination : recipient;
  const handle = foreign.kind === 'solana' ? solanaRecipientHandle(destination) : recipient;
  if (!handle) throw new Error('The production Solana recipient codec must derive a handle.');
  const amount = 4200000n;
  const data = encodeFunctionData({ abi: layerxBridgeAbi, functionName: 'bridgeOut', args: [ foreign.chainId, asset.assetId, amount, handle ] });
  const serialized = await wallet.signTransaction({ chainId: paxeer.id, type: 'eip1559', to: LAYERX_BRIDGE_ADDRESS, data, nonce: 0, gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n });
  const transaction = parseTransaction(serialized);
  if (!transaction.data) throw new Error('Signed bridgeOut input is missing.');
  const call = decodeFunctionData({ abi: layerxBridgeAbi, data: transaction.data });
  expect(call.functionName).toBe('bridgeOut');
  expect(call.args).toEqual([ foreign.chainId, asset.assetId, amount, handle ]);
  const hash = keccak256(serialized);
  const nonce = 11n;
  peer.receipts.set(hash, {
    transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash,
    from: wallet.address, to: LAYERX_BRIDGE_ADDRESS, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0', effectiveGasPrice: '0x1',
    contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status: '0x1',
    logs: [ {
      address: LAYERX_BRIDGE_ADDRESS,
      topics: encodeEventTopics({ abi: layerxBridgeAbi, eventName: 'BridgeOut', args: { chain: foreign.chainId, asset: asset.assetId, nonce } }),
      data: encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ amount, handle ]),
      transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash, logIndex: '0x0', removed: false,
    } ],
  });
  return { hash, foreign, asset, account: wallet.address, amount, requiredConfirmations: foreign.finalityDepth, destination };
}

async function committedDeposit(peer: Peer) {
  const foreign = route('base');
  const asset = foreign.assets[0];
  const amount = 4200000n;
  const recipient32 = `0x${ '00'.repeat(12) }${ wallet.address.slice(2) }` as Hex;
  const data = encodeFunctionData({ abi: paxeerXVaultAbi, functionName: 'depositNative', args: [ recipient32 ] });
  const serialized = await wallet.signTransaction({ chainId: Number(foreign.chainId), type: 'eip1559', to: vault, data, value: amount, nonce: 0, gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n });
  const hash = keccak256(serialized);
  peer.receipts.set(hash, {
    transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash,
    from: wallet.address, to: vault, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0', effectiveGasPrice: '0x1',
    contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status: '0x1',
    logs: [ {
      address: vault,
      topics: encodeEventTopics({ abi: paxeerXVaultAbi, eventName: 'BridgeDeposit', args: { asset: zeroAddress, sender: wallet.address, paxeerRecipient: recipient32 } }),
      data: encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ amount, 11n ]),
      transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash, logIndex: '0x0', removed: false,
    } ],
  });
  return { hash, foreign: { ...foreign, vault }, asset, account: wallet.address, amount, requiredConfirmations: foreign.finalityDepth, vault, recipient: wallet.address };
}

function consumerClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
}

function wrapper(client: QueryClient) {
  return function QueryProvider({ children }: PropsWithChildren) {
    return createElement(QueryClientProvider, { client }, children);
  };
}

function ObservationConsumer({ transfer, settings }: { transfer: BridgeTransfer; settings: TransferReadSettings }) {
  const query = useQuery(transferLiveOptions(transfer, settings));
  return createElement('section', null,
    createElement('output', null, `${ query.data?.confirmations ?? 'unknown' }:${ query.data?.credited ?? 'unknown' }:${ query.data?.released ?? 'unknown' }`),
    createElement(TransferObservation, { transfer, live: query.data, error: query.error, retrying: query.isFetching, retry: () => void query.refetch() }),
  );
}

afterEach(() => {
  cleanup(); window.localStorage.clear();
});

async function withdrawalFinalityMatchesSubmitImportAndLegacyRecovery() {
  const peer = await rpcPeer();
  try {
    const input = await committedBurn(peer);
    let notified = false;
    const submitted = recordBridgeSubmission({ direction: 'out', input }, () => {
      notified = true;
      expect(listTransfers(wallet.address)[0].requiredConfirmations).toBe(1);
    });
    expect(notified).toBe(true);
    expect(submitted.requiredConfirmations).toBe(1);
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const before = await readTransferLive(submitted, peer.settings);
    const imported = await recoverTransferReceipt(undefined, receipt, wallet.address, peer.config);
    const after = await readTransferLive(imported.transfer, peer.settings);
    expect(before.confirmations).toBe(1);
    expect(before.requiredConfirmations).toBe(1);
    expect(after).toEqual(before);
    expect(phaseOf(submitted, before, false)).toBe('attesting');
    expect(phaseOf(imported.transfer, after, false)).toBe('attesting');
    expect(transferFinalityEstimate(submitted)).toBe('At the committed Paxeer block');
    expect(transferFinalityEstimate({ ...submitted, direction: 'in', requiredConfirmations: 1800 })).toBe('~60 min');
    window.localStorage.setItem(storageKey, JSON.stringify([ { ...submitted, bridgeSchemaVersion: undefined, requiredConfirmations: 1800 } ]));
    expect(listBridgeTransfers(wallet.address)[0]).toMatchObject({ bridgeSchemaVersion: 2, requiredConfirmations: 1 });
    expect(JSON.parse(window.localStorage.getItem(storageKey)!)[0].requiredConfirmations).toBe(1);
    const deposit = recordBridgeSubmission({ direction: 'in', input: { ...input, vault, recipient } }, () => {});
    expect(deposit.requiredConfirmations).toBe(1800);
  } finally {
    await peer.close();
  }
}

async function reimportRepairsLegacyMetadataAndReopensConfirmedEvidence() {
  const peer = await rpcPeer();
  const client = consumerClient();
  const reopenedClient = consumerClient();
  try {
    const input = await committedBurn(peer);
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const original = importTransferReceipt(undefined, receipt, wallet.address).transfer;
    const releaseHash = keccak256('0x33');
    // This successful destination observation establishes the settlement evidence retained below.
    peer.nullifiers.add(releaseNullifier(input.hash, 11n));
    const verified = await readTransferLive({ ...original, vault }, peer.settings);
    expect(verified.released).toBe(true);
    const evidence = settlementPatch(original, verified, 9000);
    expect(evidence).toMatchObject({ releasedAt: 9000, settlementStatus: 'confirmed' });
    const legacy = { ...original, ...evidence, releaseTxHash: releaseHash, symbol: 'OLD', decimals: 3, requiredConfirmations: 1800, bridgeOutNonce: undefined, vault: undefined, bridgeSchemaVersion: undefined, submittedAt: 1234 };
    const mounted = renderHook(() => useQuery(transferLiveOptions({ ...original, vault }, peer.settings)), { wrapper: wrapper(client) });
    await waitFor(() => expect(mounted.result.current.data?.released).toBe(true));
    mounted.unmount();
    client.clear();
    window.localStorage.setItem(storageKey, JSON.stringify([ legacy ]));
    const result = await recoverTransferReceipt(undefined, receipt, wallet.address, peer.config);
    expect(result.existing).toBe(true);
    expect(result.transfer).toMatchObject({
      vault, bridgeOutNonce: '11', symbol: 'ETH', decimals: 18, requiredConfirmations: 1,
      releaseTxHash: releaseHash, releasedAt: 9000, settlementStatus: 'confirmed', submittedAt: 1234, bridgeSchemaVersion: 2,
    });
    // Reopen the real query consumer with no prior query cache and the persisted row.
    peer.receipts.set(releaseHash, peer.receipts.get(input.hash));
    const reopened = listBridgeTransfers(wallet.address)[0];
    const hook = renderHook(() => useQuery(transferLiveOptions(reopened, peer.settings)), { wrapper: wrapper(reopenedClient) });
    await waitFor(() => expect(hook.result.current.data?.released).toBe(true));
    expect(phaseOf(reopened, hook.result.current.data, false)).toBe('settled');
    expect(listTransfers(wallet.address)).toHaveLength(1);
    const preserved = window.localStorage.getItem(storageKey);
    updateTransfer(reopened.id, { amount: '1' });
    const conflicting = window.localStorage.getItem(storageKey);
    await expect(recoverTransferReceipt(undefined, receipt, wallet.address, peer.config)).rejects.toThrow('Receipt conflicts with tracked settlement identity: amount');
    expect(window.localStorage.getItem(storageKey)).toBe(conflicting);
    window.localStorage.setItem(storageKey, preserved!);
    expect(() => importTransferReceipt(undefined, receipt, recipient)).toThrow('Connect the wallet');
    expect(window.localStorage.getItem(storageKey)).toBe(preserved);
    hook.unmount();
  } finally {
    client.clear(); reopenedClient.clear(); await peer.close();
  }
}

async function reimportReportsAndRepairsUnsettledIdentityConflicts() {
  const peer = await rpcPeer();
  try {
    const input = await committedBurn(peer);
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const original = importTransferReceipt(undefined, receipt, wallet.address).transfer;
    saveTransfer({ ...original, amount: '1', recipient: wallet.address, bridgeOutNonce: '2' });
    const imported = await recoverTransferReceipt(undefined, receipt, wallet.address, peer.config);
    expect(imported.conflicts).toEqual([ 'amount', 'recipient', 'bridgeOutNonce' ]);
    expect(imported.transfer).toMatchObject({ amount: '4200000', recipient, bridgeOutNonce: '11', vault });
    expect(listBridgeTransfers(wallet.address)).toEqual([ imported.transfer ]);
  } finally {
    await peer.close();
  }
}

async function rpcFailuresRetainVerifiedProgressAndOfferWorkingRetry() {
  const peer = await rpcPeer();
  const client = consumerClient();
  try {
    const input = await committedBurn(peer);
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const transfer = (await recoverTransferReceipt(undefined, receipt, wallet.address, peer.config)).transfer;
    const hook = renderHook(() => useQuery(transferLiveOptions(transfer, peer.settings)), { wrapper: wrapper(client) });
    const view = render(createElement(ObservationConsumer, { transfer, settings: peer.settings }), { wrapper: wrapper(client) });
    await waitFor(() => expect(hook.result.current.data?.confirmations).toBe(1));
    const verified = hook.result.current.data;
    for (const method of [ 'eth_getTransactionReceipt', 'eth_blockNumber', 'eth_call' ]) {
      peer.failMethod(method);
      await act(async() => {
        await hook.result.current.refetch();
      });
      expect(hook.result.current.isError).toBe(true);
      expect(hook.result.current.data).toEqual(verified);
      expect(transferHistoryRecords([ transfer ], [ hook.result.current.data ])[0].status).toBe('processing');
      await waitFor(() => expect(view.getByRole('alert').textContent).toContain('RPC unavailable'));
      expect(view.getByRole('status').textContent).toBe('1:false:false');
      peer.failMethod();
      fireEvent.click(view.getByRole('button', { name: 'Retry RPC' }));
      await waitFor(() => expect(hook.result.current.isSuccess).toBe(true));
      await waitFor(() => expect(view.queryByRole('alert')).toBeNull());
    }
    // A verified absent source receipt remains a valid pending result.
    peer.receipts.delete(input.hash);
    expect((await readTransferLive(transfer, peer.settings)).confirmations).toBe(0);
    hook.unmount();
    view.unmount();
  } finally {
    client.clear(); await peer.close();
  }
}

async function creditRpcFailureDoesNotEraseConfirmedCredit() {
  const peer = await rpcPeer();
  const client = consumerClient();
  try {
    const input = await committedDeposit(peer);
    recordBridgeSubmission({ direction: 'in', input }, () => {});
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: Number(input.foreign.chainId) });
    const deposit = importTransferReceipt(input.foreign, receipt, wallet.address).transfer;
    peer.nullifiers.add(`${ input.hash }:0`);
    const hook = renderHook(() => useQuery(transferLiveOptions(deposit, peer.settings)), { wrapper: wrapper(client) });
    await waitFor(() => expect(hook.result.current.data?.credited).toBe(true));
    const evidence = listTransfers(wallet.address)[0];
    expect(evidence.settlementStatus).toBe('confirmed');
    peer.failFunction('isNullified');
    await act(async() => {
      await hook.result.current.refetch();
    });
    expect(hook.result.current.isError).toBe(true);
    expect(hook.result.current.data?.credited).toBe(true);
    expect(listTransfers(wallet.address)[0]).toEqual(evidence);
    expect(phaseOf(deposit, hook.result.current.data, false)).toBe('settled');
    hook.unmount();
  } finally {
    client.clear(); await peer.close();
  }
}

function releaseAccount(transfer: BridgeTransfer) {
  // NullifierRecord::encode in contracts/bridge/solana/src/state.rs, using its pinned outbound vector identities.
  const bytes = new Uint8Array(130);
  const view = new DataView(bytes.buffer);
  bytes.set(new TextEncoder().encode('PXBRNUL0'));
  view.setUint16(8, 1, false);
  bytes.set(hexToBytes(transfer.sourceTxHash), 10);
  view.setBigUint64(42, BigInt(transfer.bridgeOutNonce!), false);
  bytes.set(base58Decode(route('solana').assets[1].solanaMint!)!, 50);
  bytes.set(base58Decode(solanaDestination)!, 82);
  view.setBigUint64(114, BigInt(transfer.amount), false);
  view.setBigUint64(122, 25n, false);
  return bytes;
}

async function solanaRelayObservationSurvivesRestartAndShowsPayoutRecovery() {
  const peer = await rpcPeer();
  const client = consumerClient();
  const reopenedClient = consumerClient();
  try {
    const input = await committedBurn(peer, route('solana'));
    recordBridgeSubmission({ direction: 'out', input }, () => {});
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const transfer = (await recoverTransferReceipt(undefined, receipt, wallet.address, peer.config)).transfer;
    expect(transfer.recipient).toBe(solanaDestination);
    const address = solanaProgramAddress(programId, [ new TextEncoder().encode('nullifier'), hexToBytes(releaseNullifier(input.hash, 11n)) ])!;
    const view = render(createElement(WagmiProvider, {
      config: peer.config,
      reconnectOnMount: false,
      children: createElement(TransferRow, { transfer, onChanged: () => {}, readSettings: peer.settings }),
    }), { wrapper: wrapper(client) });
    await waitFor(() => expect(view.getByRole('link').getAttribute('href')).toBe(`https://explorer.solana.com/address/${ address.address }`));
    expect(client.getQueryData<TransferLive>(transferLiveOptions(transfer, peer.settings).queryKey)).toMatchObject({ confirmations: 1, credited: false, released: false });
    expect(view.getByText(/Solana relay pending/)).toBeTruthy();
    expect(view.queryByText(/Switch to Solana/)).toBeNull();
    const bytes = releaseAccount(transfer);
    peer.accounts.set(address.address, { owner: programId, executable: false, data: [ Buffer.from(bytes).toString('base64'), 'base64' ] });
    fireEvent.click(view.getByRole('button', { name: 'Refresh relay status' }));
    await waitFor(() => expect(view.getByText('Solana payout confirmed.')).toBeTruthy());
    expect(view.getByRole('link', { name: 'View Solana payout record' }).getAttribute('href')).toBe(`https://explorer.solana.com/address/${ address.address }`);
    expect(listTransfers(wallet.address)[0]).toMatchObject({ settlementStatus: 'confirmed', releasedAt: expect.any(Number) });
    view.unmount();
    client.clear();
    const reopened = listBridgeTransfers(wallet.address)[0];
    const next = render(createElement(WagmiProvider, {
      config: peer.config,
      reconnectOnMount: false,
      children: createElement(TransferRow, { transfer: reopened, onChanged: () => {}, readSettings: peer.settings }),
    }), { wrapper: wrapper(reopenedClient) });
    await waitFor(() => expect(next.getByText('Solana payout confirmed.')).toBeTruthy());
    expect(next.queryByRole('button', { name: /Switch/ })).toBeNull();
    const observed = await reopenedClient.fetchQuery(transferLiveOptions(reopened, peer.settings));
    expect(observed.released).toBe(true);
    expect(phaseOf(reopened, observed, false)).toBe('settled');
    expect(peer.requests.filter((entry) => entry.method === 'getAccountInfo').length).toBeGreaterThanOrEqual(3);
    next.unmount();
  } finally {
    client.clear(); reopenedClient.clear(); await peer.close();
  }
}

async function solanaReleaseRejectsForeignOrMismatchedEvidence() {
  const peer = await rpcPeer();
  try {
    const input = await committedBurn(peer, route('solana'));
    const receipt = await getTransactionReceipt(peer.config, { hash: input.hash, chainId: paxeer.id });
    const transfer = importTransferReceipt(undefined, receipt, wallet.address).transfer;
    const address = solanaProgramAddress(programId, [ new TextEncoder().encode('nullifier'), hexToBytes(releaseNullifier(input.hash, 11n)) ])!;
    const bytes = releaseAccount(transfer);
    const account = { owner: programId, executable: false, data: [ Buffer.from(bytes).toString('base64'), 'base64' ] as [string, string] };
    for (const offset of [ 0, 9, 10, 42, 50, 82, 114, 122 ]) {
      const malformed = bytes.slice();
      malformed[offset] ^= 1;
      peer.accounts.set(address.address, { ...account, data: [ Buffer.from(malformed).toString('base64'), 'base64' ] });
      await expect(readTransferLive(transfer, peer.settings)).rejects.toThrow(/Solana release/);
    }
    peer.accounts.set(address.address, { ...account, owner: base58Encode(new Uint8Array(32)) });
    await expect(readTransferLive(transfer, peer.settings)).rejects.toThrow('account owner');
    peer.accounts.set(address.address, account);
    peer.setResponseId(2);
    await expect(readTransferLive(transfer, peer.settings)).rejects.toThrow('Invalid Solana release RPC response');
    peer.setResponseId(1);
    peer.failMethod('getAccountInfo');
    await expect(readTransferLive(transfer, peer.settings)).rejects.toThrow('Invalid Solana release RPC response');
    peer.failMethod();
    expect((await readTransferLive(transfer, peer.settings)).released).toBe(true);
    await expect(readTransferLive(transfer, { config: peer.config, solana: {} })).rejects.toThrow('requires the custody program, RPC and burn nonce');
  } finally {
    await peer.close();
  }
}

test(withdrawalFinalityMatchesSubmitImportAndLegacyRecovery);
test(reimportRepairsLegacyMetadataAndReopensConfirmedEvidence);
test(reimportReportsAndRepairsUnsettledIdentityConflicts);
test(rpcFailuresRetainVerifiedProgressAndOfferWorkingRetry);
test(creditRpcFailureDoesNotEraseConfirmedCredit);
test(solanaRelayObservationSurvivesRestartAndShowsPayoutRecovery);
test(solanaReleaseRejectsForeignOrMismatchedEvidence);
