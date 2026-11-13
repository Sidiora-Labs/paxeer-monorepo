import { QueryClient } from '@tanstack/react-query';
import { createServer } from 'node:http';
import {
  decodeFunctionData, encodeAbiParameters, encodeEventTopics, encodeFunctionData,
  encodeFunctionResult, keccak256, multicall3Abi, numberToHex,
  type Address, type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { expect, vi } from 'vitest';

import { layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from '../contracts';
import { inboundDigest, outboundDigest, paxeerRecipient, releaseNullifier } from '../digests';
import type { BridgeTransfer, TransferDirection } from '../transfers';

const storageKey = 'paxeer-bridge:transfers:v1';
const wallet = privateKeyToAccount(`0x${ '12'.repeat(32) }`);
const recipient = privateKeyToAccount(`0x${ '56'.repeat(32) }`).address;
const vault = privateKeyToAccount(`0x${ '78'.repeat(32) }`).address;
const signers = [ '34', '9a' ].map((byte) => privateKeyToAccount(`0x${ byte.repeat(32) }`))
  .sort((a, b) => BigInt(a.address) < BigInt(b.address) ? -1 : 1);

function wireReceipt(hash: Hex, to: Address, logs: Array<{ address: Address; data: Hex; topics: readonly Hex[] }>, status: 'success' | 'reverted') {
  return {
    transactionHash: hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: keccak256('0x10'),
    from: wallet.address, to, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0', effectiveGasPrice: '0x1',
    contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status: status === 'success' ? '0x1' : '0x0',
    logs: logs.map((log, index) => ({ ...log, transactionHash: hash, transactionIndex: '0x0',
      blockHash: keccak256('0x10'), blockNumber: '0x10', logIndex: numberToHex(index), removed: false })),
  };
}

/** Loopback peer snapshots; every request crosses the production HTTP transport and ABI codec. */
export async function openBridgeRepairFixture() {
  const receipts = new Map<Hex, ReturnType<typeof wireReceipt>>();
  const bodies = new Map<string, unknown>();
  const nullifiers = new Set<string>();
  const requests: Array<{ path: string; method: string; params: unknown[] }> = [];
  const errors: unknown[] = [];
  const registry = { registered: true, enabled: true, vault };
  const authority = { attestors: signers.map((signer) => signer.address), threshold: 2n, digestOverride: undefined as Hex | undefined };
  let sourceNonce = 0;

  function contractRead(path: string, call: { to: Address; data: Hex }): Hex {
    if (call.data.startsWith('0x82ad56cb')) {
      expect(path).toBe('/base');
      expect(call.to.toLowerCase()).toBe(foreign.viemChain!.contracts!.multicall3!.address.toLowerCase());
      const decoded = decodeFunctionData({ abi: multicall3Abi, data: call.data });
      if (decoded.functionName !== 'aggregate3') throw new Error('Expected the production multicall envelope.');
      return encodeFunctionResult({ abi: multicall3Abi, functionName: 'aggregate3', result: decoded.args[0].map((entry) => ({
        success: true, returnData: contractRead(path, { to: entry.target, data: entry.callData }),
      })) });
    }
    const precompile = call.to.toLowerCase() === LAYERX_BRIDGE_ADDRESS;
    expect(path).toBe(precompile ? '/paxeer' : '/base');
    if (!precompile) expect(call.to.toLowerCase()).toBe(vault.toLowerCase());
    const abi = precompile ? layerxBridgeAbi : paxeerXVaultAbi;
    const { functionName, args = [] } = decodeFunctionData({ abi, data: call.data });
    let result: unknown;
    switch (functionName) {
      case 'getChain':
        expect(args).toEqual([ 8453n ]);
        result = [ registry.registered, registry.vault, 1n, registry.enabled ];
        break;
      case 'getAttestors': result = [ authority.attestors, authority.attestors.map(() => 1n), Number(authority.threshold) ]; break;
      case 'isNullified': result = nullifiers.has(`${ args[1] }:${ args[2] }`); break;
      case 'nullified': result = nullifiers.has(String(args[0])); break;
      case 'attestors': result = authority.attestors; break;
      case 'threshold': result = authority.threshold; break;
      case 'releaseDigest':
        result = authority.digestOverride ?? outboundDigest({
          chainId: 8453n, vault, paxeerTxHash: args[0] as Hex, paxeerNonce: args[1] as bigint,
          recipient: args[2] as Address, asset: args[3] as Address, amount: args[4] as bigint,
        });
        break;
      default: throw new Error(`Unexpected fixture contract call: ${ functionName }`);
    }
    return encodeFunctionResult({ abi, functionName, result });
  }

  const server = createServer(async(incoming, outgoing) => {
    outgoing.setHeader('Content-Type', 'application/json');
    try {
      if (incoming.method === 'GET') {
        const body = bodies.get(incoming.url!);
        if (!body) throw new Error(`Unexpected attestation request: ${ incoming.url }`);
        requests.push({ path: incoming.url!, method: 'GET', params: [] });
        outgoing.end(JSON.stringify(body));
        return;
      }
      let raw = '';
      for await (const chunk of incoming) raw += chunk.toString();
      const input = JSON.parse(raw);
      const dispatch = (message: { id: number; method: string; params: unknown[] }) => {
        const path = incoming.url!;
        const { method, params } = message;
        requests.push({ path, method, params });
        let result: unknown;
        switch (method) {
          case 'eth_call': result = contractRead(path, params[0] as { to: Address; data: Hex }); break;
          case 'eth_blockNumber': result = '0x20'; break;
          case 'eth_getTransactionReceipt': result = receipts.get(params[0] as Hex) ?? null; break;
          case 'eth_getTransactionByHash': result = null; break;
          default: throw new Error(`Unexpected fixture RPC method: ${ method }`);
        }
        return { jsonrpc: '2.0', id: message.id, result };
      };
      outgoing.end(JSON.stringify(Array.isArray(input) ? input.map(dispatch) : dispatch(input)));
    } catch (error) {
      errors.push(error);
      outgoing.statusCode = 500;
      outgoing.end(JSON.stringify({ error: String(error) }));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const listener = server.address();
  if (!listener || typeof listener === 'string') throw new Error('Bridge fixture listener did not open.');
  const baseUrl = `http://127.0.0.1:${ listener.port }`;
  const deployment = {
    NEXT_PUBLIC_PAXEER_RPC_URL: `${ baseUrl }/paxeer`,
    NEXT_PUBLIC_BRIDGE_RPC_BASE: `${ baseUrl }/base`,
    NEXT_PUBLIC_BRIDGE_VAULT_BASE: vault,
    NEXT_PUBLIC_BRIDGE_API_URL: baseUrl,
  };
  const previousEnv = Object.fromEntries(Object.keys(deployment).map((key) => [ key, process.env[key] ]));
  const previousStorage = window.localStorage.getItem(storageKey);
  window.localStorage.removeItem(storageKey);
  Object.assign(process.env, deployment);
  vi.resetModules();
  const actions = await import('@/components/bridge/transfer-actions');
  const queries = await import('@/components/bridge/transfers-panel');
  const chains = await import('../chains');
  const submitted = await import('../submitted');
  const ledger = await import('../transfers');
  const settlement = await import('../settlement');
  const api = await import('../attestation-api');
  const foreign = chains.bridgeChainByKey('base')!;
  const asset = foreign.assets[0];

  async function reopen(id: string): Promise<BridgeTransfer> {
    const client = new QueryClient();
    try {
      const transfers = await client.fetchQuery(queries.transferListOptions(wallet.address, 0, 0));
      const found = transfers.find((transfer) => transfer.id === id ||
        ledger.transferIdOf(transfer.direction, transfer.sourceTxHash, transfer.foreignChainId) === id);
      if (!found) throw new Error(`Stored transfer did not survive reopening: ${ id }`);
      const stored = ledger.listTransfers(wallet.address).find((transfer) => transfer.sourceTxHash === found.sourceTxHash);
      if (!stored) throw new Error('Enriched transfer did not reach persistent storage.');
      return stored;
    } finally {
      client.clear();
    }
  }

  async function observe(transfer: BridgeTransfer) {
    const client = new QueryClient();
    try {
      return await client.fetchQuery(queries.transferLiveOptions(transfer));
    } finally {
      client.clear();
    }
  }

  async function submit(direction: TransferDirection): Promise<BridgeTransfer> {
    const amount = 9007199254740993n;
    const nonce = BigInt(sourceNonce++);
    const data = direction === 'out' ? encodeFunctionData({ abi: layerxBridgeAbi, functionName: 'bridgeOut',
      args: [ foreign.chainId, asset.assetId, amount, recipient ] }) :
      encodeFunctionData({ abi: paxeerXVaultAbi, functionName: 'depositNative', args: [ paxeerRecipient(recipient) ] });
    const to = direction === 'out' ? LAYERX_BRIDGE_ADDRESS : vault;
    const hash = keccak256(await wallet.signTransaction({ chainId: direction === 'out' ? chains.paxeer.id : 8453,
      type: 'eip1559', nonce: Number(nonce), to, data, gas: 200000n,
      maxFeePerGas: 2n, maxPriorityFeePerGas: 1n, ...(direction === 'in' ? { value: amount } : {}) }));
    const record = direction === 'out' ? submitted.withdrawalRecord({
      hash, foreign, asset, account: wallet.address, amount, destination: recipient, requiredConfirmations: 1,
    }) : submitted.depositRecord({
      hash, foreign, asset, account: wallet.address, amount, recipient, vault, requiredConfirmations: 1,
    });
    const topics = direction === 'out' ? encodeEventTopics({ abi: layerxBridgeAbi, eventName: 'BridgeOut',
      args: { chain: foreign.chainId, asset: asset.assetId, nonce } }) :
      encodeEventTopics({ abi: paxeerXVaultAbi, eventName: 'BridgeDeposit',
        args: { asset: asset.assetId, sender: wallet.address, paxeerRecipient: paxeerRecipient(recipient) } });
    const logData = direction === 'out' ?
      encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ amount, recipient ]) :
      encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ amount, nonce ]);
    receipts.set(hash, wireReceipt(hash, to, [ { address: to, topics: topics as Hex[], data: logData } ], 'success'));
    ledger.saveTransfer(record);
    return record;
  }

  async function releaseAttestation(transfer: BridgeTransfer) {
    const identity = { chainId: foreign.chainId, paxeerTxHash: transfer.sourceTxHash, paxeerNonce: BigInt(transfer.bridgeOutNonce!) };
    const digest = outboundDigest({ ...identity, vault, recipient, asset: transfer.assetId, amount: BigInt(transfer.amount) });
    bodies.set(api.releaseAttestationPath(identity), {
      status: 'ready', chainId: String(foreign.chainId), vault, paxeerTxHash: transfer.sourceTxHash,
      paxeerNonce: transfer.bridgeOutNonce, recipient, asset: transfer.assetId, amount: transfer.amount, digest,
      signatures: await Promise.all(signers.map((signer) => signer.sign({ hash: digest }))),
    });
    const result = await api.fetchReleaseAttestation(identity, baseUrl);
    if (!result) throw new Error('The signed release did not cross HTTP.');
    return result;
  }

  async function signSettlement(transfer: BridgeTransfer, maxFeePerGas = 2n): Promise<Hex> {
    let request;
    if (transfer.direction === 'out') {
      request = (await actions.buildVerifiedRelease(transfer, await releaseAttestation(transfer))).request;
    } else {
      const identity = { chainId: foreign.chainId, txHash: transfer.sourceTxHash, logIndex: BigInt(transfer.logIndex!) };
      const digest = inboundDigest({ ...identity, vault, recipient32: paxeerRecipient(recipient),
        asset: transfer.assetId, amount: BigInt(transfer.amount) });
      bodies.set(api.depositAttestationPath(identity), {
        status: 'ready', chainId: String(foreign.chainId), vault, txHash: transfer.sourceTxHash,
        logIndex: transfer.logIndex, recipient: paxeerRecipient(recipient), asset: transfer.assetId, amount: transfer.amount, digest,
        signatures: await Promise.all(signers.map((signer) => signer.sign({ hash: digest }))),
      });
      const attestation = await api.fetchDepositAttestation(identity, baseUrl);
      if (!attestation) throw new Error('The signed claim did not cross HTTP.');
      request = (await actions.buildVerifiedClaim(transfer, attestation)).request;
    }
    const data = request.functionName === 'bridgeIn' ? encodeFunctionData(request) : encodeFunctionData(request);
    return keccak256(await wallet.signTransaction({ chainId: request.chainId, type: 'eip1559', nonce: 100,
      to: request.address, data, gas: 200000n, maxFeePerGas, maxPriorityFeePerGas: 1n }));
  }

  function nullifierOf(transfer: BridgeTransfer) {
    return transfer.direction === 'in' ? `${ transfer.sourceTxHash }:${ transfer.logIndex }` :
      releaseNullifier(transfer.sourceTxHash, BigInt(transfer.bridgeOutNonce!));
  }

  return {
    wallet, recipient, vault, foreign, signers, registry, authority, actions, ledger, settlement, receipts, requests,
    submit, reopen, observe, releaseAttestation, signSettlement,
    setReceipt(hash: Hex, direction: TransferDirection, status: 'success' | 'reverted') {
      receipts.set(hash, wireReceipt(hash, direction === 'in' ? LAYERX_BRIDGE_ADDRESS : vault, [], status));
    },
    confirm(transfer: BridgeTransfer) {
      nullifiers.add(nullifierOf(transfer));
    },
    clearConfirmation(transfer: BridgeTransfer) {
      nullifiers.delete(nullifierOf(transfer));
    },
    async close() {
      for (const [ key, value ] of Object.entries(previousEnv)) {
        if (value === undefined) delete process.env[key];
        else process.env[key] = value;
      }
      if (previousStorage === null) window.localStorage.removeItem(storageKey);
      else window.localStorage.setItem(storageKey, previousStorage);
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      vi.resetModules();
      expect(errors).toEqual([]);
    },
  };
}
