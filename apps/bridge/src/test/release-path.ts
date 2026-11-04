/** Loopback peers replay ABI/HTTP fixtures; production consumers are never replaced. */
import { QueryClient } from '@tanstack/react-query';
import { createServer } from 'node:http';
import {
  decodeFunctionData, encodeAbiParameters, encodeEventTopics, encodeFunctionResult,
  getAddress, http, multicall3Abi, recoverAddress, zeroAddress, zeroHash,
  type Address, type Hex,
} from 'viem';
import { expect, vi } from 'vitest';

import depositBody from '../lib/bridge/fixtures/deposit-ready.json';
import releaseBody from '../lib/bridge/fixtures/release-ready.json';
import { layerxBridgeAbi, LAYERX_BRIDGE_ADDRESS, paxeerXVaultAbi } from '../lib/bridge/contracts';
import { outboundDigest } from '../lib/bridge/digests';

export const ledgerKey = 'paxeer-bridge:transfers:v1';

export async function openReleasePath() {
  vi.resetModules();
  const [ actions, panel, ledger, submitted, card, chains, wallet, api, settlement ] = await Promise.all([
    import('@/components/bridge/transfer-actions'), import('@/components/bridge/transfers-panel'),
    import('@/lib/bridge/transfers'), import('@/lib/bridge/submitted'),
    import('@/components/bridge/bridge-card'), import('@/lib/bridge/chains'),
    import('@/lib/bridge/wagmi'), import('@/lib/bridge/attestation-api'), import('@/lib/bridge/settlement'),
  ]);
  const foreign = chains.bridgeChainByKey('ethereum');
  if (!foreign) throw new Error('The production Ethereum route is required.');
  const attestation = api.parseReleaseAttestation({
    chainId: 1n, paxeerTxHash: releaseBody.paxeerTxHash as Hex, paxeerNonce: 3n,
  }, releaseBody);
  const attestor = await recoverAddress({ hash: attestation.digest, signature: attestation.signatures[0] });
  const input = {
    hash: attestation.paxeerTxHash, foreign,
    asset: { ...foreign.assets[0], assetId: attestation.asset },
    amount: BigInt(attestation.amount), account: attestation.recipient,
    destination: attestation.recipient, requiredConfirmations: 1,
  };
  const topics = encodeEventTopics({ abi: layerxBridgeAbi, eventName: 'BridgeOut', args: {
    chain: foreign.chainId, asset: attestation.asset, nonce: 3n,
  } });
  // ABI fixture, not a claim that a chain emitted this receipt. See the wiring record.
  const receipt = {
    transactionHash: input.hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash,
    from: input.account, to: LAYERX_BRIDGE_ADDRESS, cumulativeGasUsed: '0x186a0', gasUsed: '0x186a0',
    effectiveGasPrice: '0x1', contractAddress: null, logsBloom: `0x${ '00'.repeat(256) }`, type: '0x2', status: '0x1',
    logs: [ {
      address: LAYERX_BRIDGE_ADDRESS, topics,
      data: encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ input.amount, input.destination ]),
      transactionHash: input.hash, transactionIndex: '0x0', blockNumber: '0x10', blockHash: zeroHash, logIndex: '0x0', removed: false,
    } ],
  };
  const state = {
    registered: true, enabled: true, registeredVault: attestation.vault, released: false,
    settlementStatus: 'pending' as 'pending' | 'reverted' | 'success',
    settlementHash: `0x${ 'dd'.repeat(32) }` as Hex,
  };
  const errors: unknown[] = [];
  const calls: Array<{ method: string; params: unknown[] }> = [];
  const httpRequests: string[] = [];
  const paths = {
    deposit: api.depositAttestationPath({ chainId: 1n, txHash: depositBody.txHash as Hex, logIndex: 7n }),
    release: api.releaseAttestationPath({ chainId: 1n, paxeerTxHash: input.hash, paxeerNonce: 3n }),
  };
  // Serialize and re-open the committed bundles rather than assembling signed API bodies here.
  const persisted = JSON.stringify({ [paths.deposit]: depositBody, [paths.release]: releaseBody });
  let bodies: Record<string, unknown> = JSON.parse(persisted);
  function contractResult(to: Address, data: Hex): Hex {
    if (data.startsWith('0x82ad56cb')) {
      expect(to.toLowerCase()).toBe('0xca11bde05977b3631167028862be2a173976ca11');
      const decoded = decodeFunctionData({ abi: multicall3Abi, data });
      expect(decoded.functionName).toBe('aggregate3');
      if (decoded.functionName !== 'aggregate3') throw new Error('Expected a Multicall3 aggregate3 request.');
      return encodeFunctionResult({
        abi: multicall3Abi,
        functionName: 'aggregate3',
        result: decoded.args[0].map((call) => ({
          success: true,
          returnData: contractResult(call.target, call.callData),
        })),
      });
    }
    const abi = to.toLowerCase() === LAYERX_BRIDGE_ADDRESS ? layerxBridgeAbi : paxeerXVaultAbi;
    const decoded = decodeFunctionData({ abi, data });
    let result: unknown;
    switch (decoded.functionName) {
      case 'getChain':
        expect(to.toLowerCase()).toBe(LAYERX_BRIDGE_ADDRESS);
        expect(decoded.args).toEqual([ 1n ]);
        result = [ state.registered, state.registeredVault, 12n, state.enabled ];
        break;
      case 'releaseDigest': {
        expect(getAddress(to)).toBe(attestation.vault);
        const [ paxeerTxHash, paxeerNonce, recipient, asset, amount ] = decoded.args as readonly [Hex, bigint, Address, Address, bigint];
        result = outboundDigest({ chainId: 1n, vault: to, paxeerTxHash, paxeerNonce, recipient, asset, amount });
        break;
      }
      case 'attestors': result = [ attestor ]; break;
      case 'threshold': result = 1n; break;
      case 'nullified': result = state.released; break;
      default: throw new Error(`No fixture for contract call ${ decoded.functionName }.`);
    }
    return encodeFunctionResult({ abi, functionName: decoded.functionName, result });
  }
  function rpc(message: { id: number; method: string; params: unknown[] }) {
    calls.push(message);
    let result: unknown;
    switch (message.method) {
      case 'eth_call': {
        const call = message.params[0] as { to: Address; data: Hex };
        result = contractResult(call.to, call.data);
        break;
      }
      case 'eth_blockNumber': result = '0x20'; break;
      case 'eth_getTransactionReceipt':
        if (message.params[0] === input.hash) result = receipt;
        else if (message.params[0] === state.settlementHash) {
          result = state.settlementStatus === 'pending' ? null : {
            ...receipt, transactionHash: state.settlementHash, to: attestation.vault,
            logs: [], status: state.settlementStatus === 'reverted' ? '0x0' : '0x1',
          };
        } else throw new Error('No receipt fixture for the requested hash.');
        break;
      case 'eth_getTransactionByHash':
        expect(message.params).toEqual([ state.settlementHash ]);
        result = null;
        break;
      default: throw new Error(`No fixture for RPC method ${ message.method }.`);
    }
    return { jsonrpc: '2.0', id: message.id, result };
  }
  function listener() {
    return createServer(async(request, response) => {
      try {
        response.setHeader('Content-Type', 'application/json');
        response.setHeader('Connection', 'close');
        if (request.method === 'GET') {
          expect(request.headers.accept).toBe('application/json');
          httpRequests.push(request.url ?? '');
          const body = bodies[request.url ?? ''];
          response.statusCode = body === undefined ? 404 : 200;
          response.end(JSON.stringify(body ?? null));
          return;
        }
        expect(request.method).toBe('POST');
        let body = '';
        for await (const chunk of request) body += chunk.toString();
        const message = JSON.parse(body);
        response.end(JSON.stringify(Array.isArray(message) ? message.map(rpc) : rpc(message)));
      } catch (error) {
        errors.push(error);
        response.statusCode = 500;
        response.end(JSON.stringify({ error: String(error) }));
      }
    });
  }
  let server = listener();
  async function listen(port: number) {
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(port, '127.0.0.1', resolve);
    });
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('The fixture listener did not open.');
    return address.port;
  }
  const port = await listen(0);
  const base = `http://127.0.0.1:${ port }`;
  // Select real HTTP transports before this fresh production config creates clients.
  Object.assign(wallet.wagmiConfig._internal.transports, {
    '125': http(base, { retryCount: 0 }),
    '1': http(base, { retryCount: 0 }),
  });
  const clients: QueryClient[] = [];
  function client() {
    const result = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    clients.push(result);
    return result;
  }
  async function reload() {
    const result = await client().fetchQuery(panel.transferListOptions(input.account, clients.length, 0));
    expect(result).toHaveLength(1);
    const persisted = ledger.listTransfers(input.account);
    expect(persisted).toHaveLength(1);
    expect(persisted[0]).toEqual({
      ...result[0],
      id: ledger.transferIdOf('out', input.hash, result[0].foreignChainId, result[0].bridgeOutNonce),
    });
    return persisted[0];
  }
  async function stop() {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  }
  return {
    actions, panel, ledger, submitted, card, chains, wallet, api, settlement, attestation,
    attestor, input, state, calls, httpRequests, client, reload, base,
    damageAmounts() {
      bodies = Object.fromEntries(Object.entries(bodies).map(([ route, body ]) => [
        route, { ...(body as Record<string, unknown>), amount: '-1' },
      ]));
    },
    async reopen() {
      for (const query of clients.splice(0)) query.clear();
      await stop();
      bodies = JSON.parse(persisted);
      server = listener();
      await listen(port);
    },
    async close() {
      for (const query of clients) query.clear();
      await stop();
      expect(errors).toEqual([]);
    },
  };
}
