// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, waitFor, within } from '@testing-library/react';
import { createServer } from 'node:http';
import { createElement, type ReactElement } from 'react';
import {
  decodeFunctionData, encodeAbiParameters, encodeEventTopics, encodeFunctionData,
  encodeFunctionResult, hexToBytes, keccak256, multicall3Abi, numberToHex, zeroAddress,
  type Abi, type Address, type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { afterAll, afterEach, expect, test } from 'vitest';
import { WagmiProvider } from 'wagmi';

import { base58Decode, base58Encode, bytesToHex, solanaProgramAddress, solanaRecipientHandle } from './solana';
import { bankAbi, BANK_ADDRESS, layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from './contracts';
import { inboundDigest, outboundDigest, paxeerRecipient, releaseNullifier } from './digests';

const wallet = privateKeyToAccount(`0x${ '12'.repeat(32) }`);
const attestor = privateKeyToAccount(`0x${ '34'.repeat(32) }`);
const vault = privateKeyToAccount(`0x${ '56'.repeat(32) }`).address;
const programId = base58Encode(Uint8Array.from({ length: 32 }, (_, i) => i + 1));
const destination = base58Encode(Uint8Array.from({ length: 32 }, (_, i) => i + 40));
const encoder = new TextEncoder();
const configPda = solanaProgramAddress(programId, [ encoder.encode('config') ])!;
const vaultPda = solanaProgramAddress(programId, [ encoder.encode('vault-authority') ])!;
const solanaVault = solanaRecipientHandle(vaultPda.address)!;
const recipientHandle = solanaRecipientHandle(destination)!;
const recipientPda = solanaProgramAddress(programId, [ encoder.encode('recipient'), hexToBytes(recipientHandle) ])!;
const storageKey = 'paxeer-bridge:transfers:v1';
const receipts = new Map<Hex, ReturnType<typeof wireReceipt>>();
const accounts = new Map<string, Uint8Array>();
const nullifiers = new Set<string>();
const attestationBodies = new Map<string, unknown>();
const requests: Array<{ method: string; params: unknown[] }> = [];
const serverErrors: unknown[] = [];
let pairCap = 10n ** 25n;
let registeredVault = vault;
let nextNonce = 0n;

function wireReceipt(hash: Hex, from: Address, to: Address, logs: Array<{ address: Address; data: Hex; topics: readonly Hex[] }>, status: 'success' | 'reverted' = 'success') {
  return {
    transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: keccak256('0x10'),
    from, to, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0', effectiveGasPrice: '0x1',
    contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status: status === 'success' ? '0x1' : '0x0',
    logs: logs.map((log, index) => ({ ...log, transactionHash: hash, transactionIndex: '0x0', blockHash: keccak256('0x10'), blockNumber: '0x10', logIndex: numberToHex(index), removed: false })),
  };
}

function accountRecord(bytes: Uint8Array) {
  return { owner: programId, executable: false, data: [ Buffer.from(bytes).toString('base64'), 'base64' ], lamports: 1, rentEpoch: 0 };
}

// Account snapshots follow Config::encode, Asset::encode and RecipientRecord::encode
// in contracts/bridge/solana/src/state.rs. They cross HTTP and the real decoders.
function seedCustodyAccounts() {
  accounts.clear();
  const config = new Uint8Array(1367);
  config.set(encoder.encode('PXBRCFG0'));
  new DataView(config.buffer).setUint16(8, 1, false);
  config[75] = 1;
  config[76] = 1;
  config[77] = configPda.bump;
  config[78] = vaultPda.bump;
  config.set(hexToBytes(attestor.address), 87);
  accounts.set(configPda.address, config);
  for (const [ mint, decimals ] of [ [ 'So11111111111111111111111111111111111111112', 9 ], [ '5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump', 6 ] ] as const) {
    const bytes = new Uint8Array(89);
    const address = solanaProgramAddress(programId, [ encoder.encode('asset'), base58Decode(mint)! ])!;
    const view = new DataView(bytes.buffer);
    bytes.set(encoder.encode('PXBRAST0'));
    view.setUint16(8, 1, false);
    bytes.set(base58Decode(mint)!, 10);
    bytes.set(hexToBytes(decimals === 6 ? '0x21f7b20a555199fa73A238B1a91FD0f549068fEe' : solanaRecipientHandle(mint)!), 42);
    bytes[62] = decimals;
    bytes[63] = 1;
    bytes[64] = address.bump;
    view.setBigUint64(65, 2n * 10n ** BigInt(decimals), false);
    view.setBigUint64(73, 10n * 10n ** BigInt(decimals), false);
    view.setBigUint64(81, 3n * 10n ** BigInt(decimals), false);
    accounts.set(address.address, bytes);
  }
  const recipient = new Uint8Array(62);
  recipient.set(encoder.encode('PXBRREC0'));
  new DataView(recipient.buffer).setUint16(8, 1, false);
  recipient.set(hexToBytes(recipientHandle), 10);
  recipient.set(base58Decode(destination)!, 30);
  accounts.set(recipientPda.address, recipient);
}
seedCustodyAccounts();

function contractRead(call: { to: Address; data: Hex }): Hex {
  if (call.data.startsWith('0x82ad56cb')) {
    const decoded = decodeFunctionData({ abi: multicall3Abi, data: call.data });
    if (decoded.functionName !== 'aggregate3') throw new Error('Expected the production multicall envelope.');
    return encodeFunctionResult({ abi: multicall3Abi, functionName: 'aggregate3', result: decoded.args[0].map((entry) => ({
      success: true, returnData: contractRead({ to: entry.target, data: entry.callData }),
    })) });
  }
  const abi: Abi = call.to.toLowerCase() === LAYERX_BRIDGE_ADDRESS ? layerxBridgeAbi : call.to.toLowerCase() === BANK_ADDRESS ? bankAbi : paxeerXVaultAbi;
  const { functionName, args = [] } = decodeFunctionData({ abi, data: call.data });
  let result: unknown;
  switch (functionName) {
    case 'getChain': result = [ true, args[0] === 91600046870081n ? solanaVault : registeredVault, 1n, true ]; break;
    case 'getCap': result = [ 'ubridged', 10n ** 26n, pairCap, 0n ]; break;
    case 'getAttestors': result = [ [ attestor.address ], [ 1n ], 1 ]; break;
    case 'isPaused':
    case 'paused': result = false; break;
    case 'isNullified': result = nullifiers.has(`${ args[1] }:${ args[2] }`); break;
    case 'nullified': result = nullifiers.has(String(args[0])); break;
    case 'attestors': result = [ attestor.address ]; break;
    case 'threshold': result = 1n; break;
    case 'balance': result = 10n ** 25n; break;
    case 'caps': result = [ 10n ** 25n, 10n ** 26n ]; break;
    case 'outstanding': result = 10n ** 25n; break;
    case 'releaseDigest': result = outboundDigest({
      chainId: 8453n, vault: call.to, paxeerTxHash: args[0] as Hex, paxeerNonce: args[1] as bigint,
      recipient: args[2] as Address, asset: args[3] as Address, amount: args[4] as bigint,
    }); break;
    default: throw new Error(`Unexpected contract read ${ functionName }`);
  }
  return encodeFunctionResult({ abi, functionName, result });
}

async function rpcResult(method: string, params: unknown[]) {
  requests.push({ method, params });
  switch (method) {
    case 'eth_call': return contractRead(params[0] as { to: Address; data: Hex });
    case 'eth_blockNumber': return '0x20';
    case 'eth_chainId': return '0x7d';
    case 'eth_getBalance': return numberToHex(10n ** 25n);
    case 'eth_getTransactionReceipt': return receipts.get(params[0] as Hex) ?? null;
    case 'eth_getTransactionByHash': return null;
    case 'getAccountInfo': return { context: { slot: 32 }, value: accounts.has(String(params[0])) ? accountRecord(accounts.get(String(params[0]))!) : null };
    case 'getProgramAccounts': {
      const options = params[1] as { filters: [unknown, { memcmp: { bytes: string } }] };
      return [ ...accounts ].filter(([ , bytes ]) => bytes.length === 89 && base58Encode(bytes.subarray(42, 62)) === options.filters[1].memcmp.bytes)
        .map(([ pubkey, bytes ]) => ({ pubkey, account: accountRecord(bytes) }));
    }
    default: throw new Error(`Unexpected RPC method ${ method }`);
  }
}

const server = createServer(async(incoming, outgoing) => {
  try {
    outgoing.setHeader('Content-Type', 'application/json');
    if (incoming.method === 'GET') {
      const body = attestationBodies.get(incoming.url!);
      outgoing.statusCode = body ? 200 : 404;
      outgoing.end(JSON.stringify(body ?? null));
      return;
    }
    let body = '';
    for await (const chunk of incoming) body += chunk.toString();
    const input = JSON.parse(body);
    const dispatch = async(message: { id: number; method: string; params: unknown[] }) => ({ jsonrpc: '2.0', id: message.id, result: await rpcResult(message.method, message.params) });
    outgoing.end(JSON.stringify(Array.isArray(input) ? await Promise.all(input.map(dispatch)) : await dispatch(input)));
  } catch (error) {
    serverErrors.push(error);
    outgoing.statusCode = 500;
    outgoing.end(JSON.stringify({ error: String(error) }));
  }
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
const listener = server.address();
if (!listener || typeof listener === 'string') throw new Error('RPC listener did not open.');
const rpcUrl = `http://127.0.0.1:${ listener.port }`;
const deployment = {
  NEXT_PUBLIC_PAXEER_RPC_URL: rpcUrl,
  NEXT_PUBLIC_BRIDGE_RPC_BASE: rpcUrl,
  NEXT_PUBLIC_BRIDGE_RPC_SOLANA: rpcUrl,
  NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault,
  NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: programId,
  NEXT_PUBLIC_BRIDGE_API_URL: rpcUrl,
};
const previousEnv = Object.fromEntries(Object.keys(deployment).map((key) => [ key, process.env[key] ]));
Object.assign(process.env, deployment);

const { BridgeCard, recordBridgeSubmission } = await import('@/components/bridge/bridge-card');
const { importTransferReceipt } = await import('@/components/bridge/import-transfer');
const { buildVerifiedClaim, buildVerifiedRelease, readTransferLive, releaseCheckOf, settledRefusal } = await import('@/components/bridge/transfer-actions');
const { TransfersPanel, transferListOptions, transferLiveOptions, releaseAttestationOptions } = await import('@/components/bridge/transfers-panel');
const { TooltipProvider } = await import('@/components/ui/tooltip');
const { bridgeChainByKey, paxeer } = await import('./chains');
const { wagmiConfig } = await import('./wagmi');
const { listTransfers, saveTransfer, updateTransfer } = await import('./transfers');
const { settlementBroadcastPatch, phaseOf } = await import('./settlement');
const { getTransactionReceipt } = await import('wagmi/actions');
const { readSolanaCustodyState } = await import('./solana-custody');
const { releaseAttestationPath, depositAttestationPath } = await import('./attestation-api');
const { depositAttestationOptions } = await import('@/components/bridge/transfers-panel');
const originalWalletState = wagmiConfig.state;
const originalStorage = window.localStorage.getItem(storageKey);
const clients: QueryClient[] = [];

function newClient() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  clients.push(client);
  return client;
}

function connectAccount(chainId: (typeof wagmiConfig.chains)[number]['id'] = paxeer.id) {
  const connector = wagmiConfig.connectors[0];
  wagmiConfig.setState({ ...originalWalletState, chainId, current: connector.uid,
    connections: new Map([ [ connector.uid, { accounts: [ wallet.address ], chainId, connector } ] ]), status: 'connected' });
}

async function mount(component: ReactElement, client = newClient(), chainId: (typeof wagmiConfig.chains)[number]['id'] = paxeer.id) {
  const view = render(createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
    createElement(QueryClientProvider, { client }, createElement(TooltipProvider, null, component))));
  // Hydration clears persisted connections before the wallet connects.
  await act(async() => {});
  act(() => connectAccount(chainId));
  return { client, view };
}

async function submitWithdrawal() {
  const foreign = bridgeChainByKey('base')!;
  const asset = foreign.assets[0];
  const amount = 10n ** 18n;
  const data = encodeFunctionData({ abi: layerxBridgeAbi, functionName: 'bridgeOut', args: [ foreign.chainId, asset.assetId, amount, wallet.address ] });
  const serialized = await wallet.signTransaction({ chainId: paxeer.id, type: 'eip1559', nonce: Number(nextNonce), to: LAYERX_BRIDGE_ADDRESS, data, gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n });
  const hash = keccak256(serialized);
  const nonce = nextNonce++;
  const topics = encodeEventTopics({ abi: layerxBridgeAbi, eventName: 'BridgeOut', args: { chain: foreign.chainId, asset: asset.assetId, nonce } });
  receipts.set(hash, wireReceipt(hash, wallet.address, LAYERX_BRIDGE_ADDRESS, [ {
    address: LAYERX_BRIDGE_ADDRESS, topics: topics as Hex[],
    data: encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ amount, wallet.address ]),
  } ]));
  const transfer = recordBridgeSubmission({ direction: 'out', input: { hash, foreign, asset, amount, account: wallet.address, destination: wallet.address, requiredConfirmations: 1 } }, () => {
    expect(listTransfers(wallet.address).some((entry) => entry.sourceTxHash === hash)).toBe(true);
  });
  const digest = outboundDigest({ chainId: foreign.chainId, vault, paxeerTxHash: hash, paxeerNonce: nonce, recipient: wallet.address, asset: asset.assetId, amount });
  attestationBodies.set(releaseAttestationPath({ chainId: foreign.chainId, paxeerTxHash: hash, paxeerNonce: nonce }), {
    status: 'ready', chainId: String(foreign.chainId), vault, paxeerTxHash: hash, paxeerNonce: String(nonce),
    recipient: wallet.address, asset: asset.assetId, amount: String(amount), digest, signatures: [ await attestor.sign({ hash: digest }) ],
  });
  return { transfer, hash, nonce };
}

async function withdrawalVaultSubmissionReloadNonceAndVerifiedRelease() {
  const { transfer, nonce } = await submitWithdrawal();
  expect(transfer).not.toHaveProperty('vault');
  expect(transfer.bridgeOutNonce).toBeUndefined();
  // Re-import the producer's receipt into the existing row: chain identifiers
  // must be recovered rather than keeping the incomplete browser record.
  const receipt = await getTransactionReceipt(wagmiConfig, { hash: transfer.sourceTxHash, chainId: paxeer.id });
  const imported = importTransferReceipt(undefined, receipt, wallet.address);
  expect(imported.existing).toBe(true);
  expect(imported.transfer.bridgeOutNonce).toBe(String(nonce));
  // Restore the actual submission row to also prove reload migration/enrichment.
  saveTransfer(transfer);
  const client = newClient();
  const reloaded = await client.fetchQuery(transferListOptions(wallet.address, 1, 0));
  expect(reloaded).toHaveLength(1);
  const recovered = reloaded[0];
  expect(recovered.vault).toBe(vault);
  expect(recovered.bridgeOutNonce).toBe(String(nonce));
  expect(listTransfers(wallet.address)[0].vault).toBe(vault);
  const attestation = await client.fetchQuery(releaseAttestationOptions(recovered, false));
  expect(attestation?.status).toBe('ready');
  if (!attestation) throw new Error('The signed attestation did not cross HTTP.');
  const verified = await buildVerifiedRelease(recovered, attestation);
  expect(verified.request.address).toBe(vault);
  expect(decodeFunctionData({ abi: paxeerXVaultAbi, data: encodeFunctionData(verified.request) })).toEqual({
    functionName: 'release', args: [ recovered.assetId, BigInt(recovered.amount), wallet.address, transfer.sourceTxHash, nonce, attestation.signatures ],
  });
  // Even an untouched old submitted row can release after nonce enrichment;
  // no fixture injects the missing vault into the submitted format.
  const { enrichRecord } = await import('./enrich');
  expect((await buildVerifiedRelease(enrichRecord(transfer, receipt), attestation)).request.address).toBe(vault);
  expect(releaseCheckOf(recovered)?.args).toEqual([ releaseNullifier(transfer.sourceTxHash, nonce) ]);
  nullifiers.add(releaseNullifier(transfer.sourceTxHash, nonce));
  expect((await readTransferLive(recovered)).released).toBe(true);
  registeredVault = zeroAddress;
  await expect(buildVerifiedRelease(recovered, attestation)).rejects.toThrow('not registered and enabled');
  registeredVault = vault;
  await expect(buildVerifiedRelease(recovered, { ...attestation, vault: wallet.address })).rejects.toThrow('different vault');
}

async function settlementBroadcastRestartsReconcilesAndRetries() {
  const { transfer, nonce } = await submitWithdrawal();
  const client = newClient();
  const [ recovered ] = await client.fetchQuery(transferListOptions(wallet.address, 0, 0));
  const attestation = await client.fetchQuery(releaseAttestationOptions(recovered, false));
  if (!attestation) throw new Error('Missing release attestation.');
  const { request } = await buildVerifiedRelease(recovered, attestation);
  const signed = await wallet.signTransaction({ chainId: 8453, type: 'eip1559', nonce: 0, to: request.address, data: encodeFunctionData(request), gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n });
  const broadcast = keccak256(signed);
  updateTransfer(transfer.id, settlementBroadcastPatch(recovered, broadcast, Date.now()));
  let stored = listTransfers(wallet.address)[0];
  expect(stored.releasedAt).toBeUndefined();
  expect(phaseOf(stored, undefined, true)).toBe('settling');
  let first = await mount(createElement(TransfersPanel, { refreshKey: 0 }), newClient(), 8453);
  await waitFor(() => expect(first.view.getByRole('button', { name: 'Retry settlement' })).toBeTruthy());
  expect(first.view.getByText(/Settlement broadcast/)).toBeTruthy();
  first.view.unmount();
  first.client.clear();
  // Reopen with a fresh React Query cache: a pending hash must still be read.
  const before = requests.filter((entry) => entry.method === 'eth_getTransactionReceipt').length;
  first = await mount(createElement(TransfersPanel, { refreshKey: 1 }), newClient(), 8453);
  await waitFor(() => expect(first.view.getByRole('button', { name: 'Retry settlement' })).toBeTruthy());
  expect(requests.filter((entry) => entry.method === 'eth_getTransactionReceipt').length).toBeGreaterThan(before);
  expect(listTransfers(wallet.address)[0].releasedAt).toBeUndefined();
  first.view.unmount();
  first.client.clear();
  // An absent broadcast beyond propagation time is recoverable and retryable.
  updateTransfer(transfer.id, settlementBroadcastPatch(stored, broadcast, Date.now() - 130_000));
  stored = listTransfers(wallet.address)[0];
  const dropped = await newClient().fetchQuery(transferLiveOptions(stored));
  expect(dropped.settlementStatus).toBe('dropped');
  expect(phaseOf(stored, dropped, true)).toBe('ready');
  expect(settledRefusal(stored, dropped)).toBeUndefined();
  // A committed revert must not become a completed settlement after restart.
  receipts.set(broadcast, wireReceipt(broadcast, wallet.address, vault, [], 'reverted'));
  const reverted = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(reverted.settlementStatus).toBe('reverted');
  expect(listTransfers(wallet.address)[0].releasedAt).toBeUndefined();
  const reopened = await mount(createElement(TransfersPanel, { refreshKey: 2 }), newClient(), 8453);
  await waitFor(() => {
    expect(reopened.view.getByText(/settlement attempt reverted/)).toBeTruthy();
    expect(reopened.view.getByRole('button', { name: 'Retry settlement' })).toBeTruthy();
  });
  reopened.view.unmount();
  reopened.client.clear();
  // The replacement's successful receipt alone is insufficient without a nullifier.
  const replacement = keccak256(await wallet.signTransaction({ chainId: 8453, type: 'eip1559', nonce: 0, to: vault, data: encodeFunctionData(request), gas: 200000n, maxFeePerGas: 4n, maxPriorityFeePerGas: 2n }));
  updateTransfer(transfer.id, settlementBroadcastPatch(stored, replacement, Date.now()));
  receipts.set(replacement, wireReceipt(replacement, wallet.address, vault, []));
  const receiptOnly = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(receiptOnly.released).toBe(false);
  expect(phaseOf(listTransfers(wallet.address)[0], receiptOnly, true)).toBe('settling');
  expect(listTransfers(wallet.address)[0].releasedAt).toBeUndefined();
  nullifiers.add(releaseNullifier(transfer.sourceTxHash, nonce));
  const confirmed = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(confirmed.settlementStatus).toBe('confirmed');
  expect(phaseOf(listTransfers(wallet.address)[0], confirmed, true)).toBe('settled');
  expect(listTransfers(wallet.address)[0].releasedAt).toEqual(expect.any(Number));
  // A wallet replacement can settle while the originally recorded hash disappears.
  updateTransfer(transfer.id, { releaseTxHash: broadcast });
  receipts.delete(broadcast);
  expect((await readTransferLive(listTransfers(wallet.address)[0])).settlementStatus).toBe('confirmed');
  // Legacy optimistic completion timestamps never suppress authoritative checks.
  nullifiers.clear();
  updateTransfer(transfer.id, { releasedAt: 1, settlementStatus: undefined });
  const legacy = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(phaseOf(listTransfers(wallet.address)[0], legacy, true)).not.toBe('settled');
  expect(listTransfers(wallet.address)[0].releasedAt).toBeUndefined();
}

async function settlementClaimBroadcastRecoversAfterReload() {
  const foreign = bridgeChainByKey('base')!;
  const asset = foreign.assets[0];
  const amount = 10n ** 18n;
  const recipient32 = paxeerRecipient(wallet.address);
  const signed = await wallet.signTransaction({ chainId: 8453, type: 'eip1559', nonce: Number(nextNonce++), to: vault,
    data: encodeFunctionData({ abi: paxeerXVaultAbi, functionName: 'depositNative', args: [ recipient32 ] }),
    value: amount, gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n });
  const hash = keccak256(signed);
  const topics = encodeEventTopics({ abi: paxeerXVaultAbi, eventName: 'BridgeDeposit', args: { asset: asset.assetId, sender: wallet.address, paxeerRecipient: recipient32 } });
  receipts.set(hash, wireReceipt(hash, wallet.address, vault, [ { address: vault, topics: topics as Hex[],
    data: encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ amount, 1n ]) } ]));
  recordBridgeSubmission({ direction: 'in', input: { hash, foreign, asset, amount, vault, account: wallet.address, recipient: wallet.address, requiredConfirmations: 1 } }, () => {});
  const [ transfer ] = await newClient().fetchQuery(transferListOptions(wallet.address, 0, 0));
  expect(transfer.logIndex).toBe('0');
  const digest = inboundDigest({ chainId: foreign.chainId, vault, txHash: hash, logIndex: 0n, recipient32, asset: asset.assetId, amount });
  attestationBodies.set(depositAttestationPath({ chainId: foreign.chainId, txHash: hash, logIndex: 0n }), {
    status: 'ready', chainId: String(foreign.chainId), vault, txHash: hash, logIndex: '0', recipient: recipient32,
    asset: asset.assetId, amount: String(amount), digest, signatures: [ await attestor.sign({ hash: digest }) ],
  });
  const attestation = await newClient().fetchQuery(depositAttestationOptions(transfer, false));
  if (!attestation) throw new Error('The deposit attestation did not cross HTTP.');
  const { request } = await buildVerifiedClaim(transfer, attestation);
  const claimHash = keccak256(await wallet.signTransaction({ chainId: paxeer.id, type: 'eip1559', nonce: 99,
    to: request.address, data: encodeFunctionData(request), gas: 200000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n }));
  updateTransfer(transfer.id, settlementBroadcastPatch(transfer, claimHash, Date.now()));
  const first = await mount(createElement(TransfersPanel, { refreshKey: 0 }));
  await waitFor(() => expect(first.view.getByRole('button', { name: 'Retry settlement' })).toBeTruthy());
  expect(listTransfers(wallet.address)[0].claimedAt).toBeUndefined();
  first.view.unmount();
  first.client.clear();
  receipts.set(claimHash, wireReceipt(claimHash, wallet.address, LAYERX_BRIDGE_ADDRESS, [], 'reverted'));
  const reopened = await mount(createElement(TransfersPanel, { refreshKey: 1 }));
  await waitFor(() => {
    expect(reopened.view.getByText(/settlement attempt reverted/)).toBeTruthy();
    expect(reopened.view.getByRole('button', { name: 'Retry settlement' })).toBeTruthy();
  });
  expect(listTransfers(wallet.address)[0].claimedAt).toBeUndefined();
  reopened.view.unmount();
  reopened.client.clear();
  receipts.set(claimHash, wireReceipt(claimHash, wallet.address, LAYERX_BRIDGE_ADDRESS, []));
  const receiptOnly = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(receiptOnly.credited).toBe(false);
  expect(listTransfers(wallet.address)[0].claimedAt).toBeUndefined();
  nullifiers.add(`${ hash }:0`);
  const confirmed = await newClient().fetchQuery(transferLiveOptions(listTransfers(wallet.address)[0]));
  expect(confirmed.credited).toBe(true);
  expect(confirmed.settlementStatus).toBe('confirmed');
  expect(listTransfers(wallet.address)[0].claimedAt).toEqual(expect.any(Number));
  expect(settledRefusal(listTransfers(wallet.address)[0], confirmed)).toBe('This deposit is already credited on Paxeer.');
}

async function solanaCustodyLimitsReachTheActualCard() {
  const { client, view } = await mount(createElement(BridgeCard, { onTransferSubmitted: () => {} }));
  fireEvent.click(view.getByRole('button', { name: 'To: Paxeer. Change' }));
  const dialog = await view.findByRole('dialog');
  fireEvent.click(within(dialog).getByRole('button', { name: 'SOLASolana' }));
  await waitFor(() => expect(view.getByRole('button', { name: 'To: Solana. Change' })).toBeTruthy());
  const destinationInput = view.container.querySelector<HTMLInputElement>('#bridge-destination')!;
  const amountInput = view.container.querySelector<HTMLInputElement>('#bridge-amount')!;
  fireEvent.change(destinationInput, { target: { value: destination } });
  fireEvent.change(amountInput, { target: { value: '1' } });
  const ready = () => view.getByRole('button', { name: 'Bridge to Solana' }) as HTMLButtonElement;
  await waitFor(() => expect(ready().disabled).toBe(false));
  expect(view.queryByText(/more than the vault's outstanding/)).toBeNull();
  expect(requests.some((entry) => entry.method === 'getProgramAccounts')).toBe(true);
  expect(requests.some((entry) => entry.method === 'getAccountInfo' && entry.params[0] === recipientPda.address)).toBe(true);
  const foreign = bridgeChainByKey('solana')!;
  for (const asset of foreign.assets) {
    if (asset.symbol === 'SID') {
      fireEvent.click(view.getByRole('button', { name: 'Asset: SOL' }));
      fireEvent.click(await view.findByRole('option', { name: 'SID SID' }));
      fireEvent.keyDown(document.activeElement!, { key: 'Escape' });
      await waitFor(() => expect(ready().disabled).toBe(false));
    }
    const unit = 10n ** BigInt(asset.decimals);
    const state = await readSolanaCustodyState(asset, { programId, rpcUrl });
    expect(state.outstanding).toBe(3n * unit);
    expect(state.perTx).toBe(2n * unit);
    fireEvent.change(amountInput, { target: { value: '2.5' } });
    await waitFor(() => expect(view.getByText('Above the vault per-transaction cap.')).toBeTruthy());
    expect(ready().disabled).toBe(true);
    const entry = [ ...accounts.values() ].find((bytes) => bytes.length === 89 && bytesToHex(bytes.subarray(42, 62)).toLowerCase() === asset.assetId.toLowerCase())!;
    const data = new DataView(entry.buffer);
    data.setBigUint64(65, 5n * unit, false);
    await act(async() => {
      await client.invalidateQueries({ queryKey: [ 'solana-custody' ] });
    });
    fireEvent.change(amountInput, { target: { value: '4' } });
    await waitFor(() => expect(view.getByText('That is more than the vault\'s outstanding balance for this asset.')).toBeTruthy());
    expect(ready().disabled).toBe(true);
    fireEvent.change(amountInput, { target: { value: '1' } });
    await waitFor(() => expect(ready().disabled).toBe(false));
    pairCap = unit / 2n;
    await act(async() => {
      await client.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'readContract' });
    });
    await waitFor(() => expect(view.getByText(/Above the Paxeer per-transaction cap/)).toBeTruthy());
    expect(ready().disabled).toBe(true);
    pairCap = 10n ** 25n;
    await act(async() => {
      await client.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'readContract' });
    });
    entry[63] = 0;
    await act(async() => {
      await client.invalidateQueries({ queryKey: [ 'solana-custody' ] });
    });
    await waitFor(() => expect(view.getByText('The Solana custody asset is disabled.')).toBeTruthy());
    entry[63] = 1;
    data.setBigUint64(65, 0n, false);
    await act(async() => {
      await client.invalidateQueries({ queryKey: [ 'solana-custody' ] });
    });
    await waitFor(() => expect(view.getByText('Above the vault per-transaction cap.')).toBeTruthy());
    expect(ready().disabled).toBe(true);
    data.setBigUint64(65, 2n * unit, false);
    await act(async() => {
      await client.invalidateQueries({ queryKey: [ 'solana-custody' ] });
    });
    await waitFor(() => expect(ready().disabled).toBe(false));
  }
  const config = accounts.get(configPda.address)!;
  config[74] = 1;
  await act(async() => {
    await client.invalidateQueries({ queryKey: [ 'solana-custody' ] });
  });
  await waitFor(() => expect(view.getByText('The vault on the foreign chain is paused.')).toBeTruthy());
  expect(view.getByRole('button', { name: 'Withdrawal unavailable' })).toBeTruthy();
  view.unmount();
}

async function solanaInvalidCustodyAccountsAreRefused() {
  const asset = bridgeChainByKey('solana')!.assets[1];
  const entry = [ ...accounts.values() ].find((bytes) => bytes.length === 89 && bytes[62] === asset.decimals)!;
  const config = accounts.get(configPda.address)!;
  for (const index of [ 10, 62, 64 ]) {
    entry[index] ^= 1;
    await expect(readSolanaCustodyState(asset, { programId, rpcUrl })).rejects.toThrow('does not match its mint, PDA or configured identity');
    entry[index] ^= 1;
  }
  config[77] ^= 1;
  await expect(readSolanaCustodyState(asset, { programId, rpcUrl })).rejects.toThrow('invalid flags, threshold or PDA bumps');
  config[77] ^= 1;
  config[8] = 1;
  await expect(readSolanaCustodyState(asset, { programId, rpcUrl })).rejects.toThrow('invalid layout');
}

afterEach(() => {
  cleanup();
  for (const client of clients.splice(0)) client.clear();
  window.localStorage.removeItem(storageKey);
  nullifiers.clear();
  receipts.clear();
  registeredVault = vault;
  pairCap = 10n ** 25n;
  seedCustodyAccounts();
  wagmiConfig.setState(originalWalletState);
  expect(serverErrors).toEqual([]);
});

afterAll(async() => {
  for (const [ key, value ] of Object.entries(previousEnv)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  if (originalStorage === null) window.localStorage.removeItem(storageKey);
  else window.localStorage.setItem(storageKey, originalStorage);
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

test(withdrawalVaultSubmissionReloadNonceAndVerifiedRelease.name, withdrawalVaultSubmissionReloadNonceAndVerifiedRelease);
test(settlementBroadcastRestartsReconcilesAndRetries.name, settlementBroadcastRestartsReconcilesAndRetries);
test(settlementClaimBroadcastRecoversAfterReload.name, settlementClaimBroadcastRecoversAfterReload);
test(solanaCustodyLimitsReachTheActualCard.name, solanaCustodyLimitsReachTheActualCard);
test(solanaInvalidCustodyAccountsAreRefused.name, solanaInvalidCustodyAccountsAreRefused);
