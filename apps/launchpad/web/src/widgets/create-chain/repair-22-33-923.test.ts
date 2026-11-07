// @vitest-environment jsdom
import { EventEmitter } from 'node:events';
import { createServer } from 'node:http';
import { act, createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createConfig, WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';
import {
  createPublicClient, createWalletClient, decodeFunctionResult, defineChain, encodeFunctionResult, http, toHex,
  type EIP1193Provider, type Hex,
} from 'viem';
import { expect, test } from 'vitest';
import vectors from '../../../../../../tests/vectors/chains/abi-vectors.json';
import blocks from '../../../../../../services/platform/hosted/indexer/tests/fixtures/chains_blocks.json';
import { chainsAbi } from './chain-adapters';
import { CHAINS_PRECOMPILE, decodeChainCreated, type ChainDraft } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';
import ChainWizard from './chain-wizard';

function calldata(name: string): string {
  const vector = vectors.methods.find(method => method.name === name);
  if (!vector) throw new Error(`Missing producer ABI vector: ${name}`);
  return vector.calldata;
}

/** Replay committed registry bytes; never replace a client action or adapter method. */
async function openChainTranscript() {
  const block = blocks.scope.blocks['42'];
  const transaction = block.transactions[0];
  const recorded = blocks.scope.receipts[transaction.hash as keyof typeof blocks.scope.receipts];
  const params = decodeFunctionResult({ abi: chainsAbi, functionName: 'params', data: vectors.params.returndata as Hex });
  const record = decodeFunctionResult({ abi: chainsAbi, functionName: 'getChain', data: vectors.getChain.returndata as Hex });
  // The indexer fixture stores block/transaction identity beside the receipt.
  // Join that envelope without re-encoding or changing any recorded log bytes.
  const receipt = {
    ...recorded, transactionHash: transaction.hash, blockNumber: block.number,
    transactionIndex: transaction.transactionIndex, from: transaction.from, to: transaction.to,
    contractAddress: null, cumulativeGasUsed: recorded.gasUsed, effectiveGasPrice: toHex(record[9]),
    logs: recorded.logs.map(log => ({
      ...log, blockHash: block.hash, blockNumber: block.number, transactionHash: transaction.hash, removed: false,
    })),
  };
  const created = receipt.logs.filter(log => log.topics[0] === vectors.events.find(event => event.name === 'ChainCreated')?.topic0);
  expect(created).toHaveLength(1);
  expect(decodeChainCreated({ ...created[0], topics: created[0].topics as Hex[], data: created[0].data as Hex })).toEqual({
    id: record[0], name: record[1], operator: record[2].toLowerCase(), chainId: Number(record[4]),
  });

  type RpcRequest = { id: number; method: string; params?: unknown[] };
  type WireTransaction = { from: string; to: string; data: string; value: Hex };
  const submissions: WireTransaction[] = [];
  const receiptReads: unknown[][] = [];
  const registryReads: unknown[][] = [];
  const walletNetworks: unknown[][] = [];
  const unexpected: string[] = [];
  let readbackAvailable = false;
  const server = createServer(async (request, response) => {
    let rpc: RpcRequest | undefined;
    try {
      expect(request.method).toBe('POST');
      let body = '';
      for await (const chunk of request) body += chunk.toString();
      rpc = JSON.parse(body) as RpcRequest;
      const args = rpc.params ?? [];
      let result: unknown;
      switch (rpc.method) {
        case 'eth_chainId': result = blocks.chain_id; break;
        case 'eth_accounts':
        case 'eth_requestAccounts': result = [transaction.from]; break;
        case 'eth_gasPrice': result = toHex(record[9]); break;
        case 'eth_estimateGas':
        case 'eth_sendTransaction': {
          const sent = args[0] as WireTransaction;
          expect(sent.from.toLowerCase()).toBe(record[2].toLowerCase());
          expect(sent.to.toLowerCase()).toBe(CHAINS_PRECOMPILE);
          expect(sent.data).toBe(calldata('createChain'));
          expect(BigInt(sent.value)).toBe(params[1] + params[2]);
          if (rpc.method === 'eth_estimateGas') result = recorded.gasUsed;
          else {
            submissions.push(sent);
            expect(submissions).toHaveLength(1);
            result = transaction.hash;
          }
          break;
        }
        case 'eth_call': {
          const call = args[0] as { to: string; data: string };
          expect(call.to.toLowerCase()).toBe(CHAINS_PRECOMPILE);
          if (call.data === calldata('params')) result = vectors.params.returndata;
          else if (call.data === calldata('chainByName')) {
            result = encodeFunctionResult({ abi: chainsAbi, functionName: 'chainByName', result: submissions.length ? record[0] : 0 });
          } else {
            expect(call.data).toBe(calldata('getChain'));
            expect(args[1]).toBe(block.number);
            registryReads.push(args);
            if (!readbackAvailable) {
              response.setHeader('content-type', 'application/json');
              response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, error: { code: -32000, message: 'Registry readback interrupted' } }));
              return;
            }
            result = vectors.getChain.returndata;
          }
          break;
        }
        case 'eth_getTransactionReceipt':
          expect(args).toEqual([transaction.hash]);
          expect(submissions).toHaveLength(1);
          receiptReads.push(args);
          result = receipt;
          break;
        case 'wallet_addEthereumChain': walletNetworks.push(args); result = null; break;
        case 'wallet_revokePermissions': result = null; break;
        default: throw new Error(`Unrecorded RPC method: ${rpc.method}`);
      }
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }));
    } catch (error) {
      unexpected.push(String(error));
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc?.id ?? null, error: { code: -32603, message: String(error) } }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback RPC did not bind');
  const origin = `http://127.0.0.1:${address.port}`;
  const chain = defineChain({
    id: Number(BigInt(blocks.chain_id)), name: 'Paxeer',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [origin] } },
    blockExplorers: { default: { name: 'Explorer', url: `${origin}/explorer` } },
  });
  const draft: ChainDraft = {
    draftId: 'chain-launch-path-proof', version: 1, creator: record[2].toLowerCase(), chainId: 125,
    name: record[1], tier: 'starter', feeShareBps: record[7], updatedAt: Date.now(),
  };
  return {
    chain, draft, origin, transaction, params, record, submissions, receiptReads, registryReads, walletNetworks, unexpected,
    enableReadback() { readbackAvailable = true; },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    },
  };
}

async function chain_launch_path_proof_recovers_the_submitted_journal_with_one_submission() {
  const transcript = await openChainTranscript();
  const actEnvironment = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
  const previousActEnvironment = actEnvironment.IS_REACT_ACT_ENVIRONMENT;
  actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
  let root: Root | undefined;
  let container = document.createElement('div');
  let queryClient: QueryClient | undefined;
  let config: ReturnType<typeof createConfig> | undefined;
  let provider: (EIP1193Provider & EventEmitter) | undefined;

  async function openWizard() {
    // A new provider, connector, query cache and component on every open: only
    // browser storage and the server's submitted transaction survive the reload.
    const rpc = createWalletClient({ chain: transcript.chain, transport: http(transcript.origin, { retryCount: 0 }) });
    provider = Object.assign(new EventEmitter(), { request: rpc.request });
    const mountedConfig = createConfig({
      chains: [transcript.chain], storage: null, multiInjectedProviderDiscovery: false,
      connectors: [injected({ shimDisconnect: false, target: { id: 'chains-transcript', name: 'Chains transcript', provider } })],
      transports: { [transcript.chain.id]: http(transcript.origin, { retryCount: 0 }) },
    });
    config = mountedConfig;
    await connect(mountedConfig, { connector: mountedConfig.connectors[0] });
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => root!.render(createElement(WagmiProvider, { config: mountedConfig, reconnectOnMount: false },
      createElement(QueryClientProvider, { client: queryClient! }, createElement(ChainWizard)))));
  }

  async function closeWizard() {
    await act(async () => root?.unmount());
    root = undefined;
    container.remove();
    queryClient?.clear();
    if (config) await disconnect(config);
    config = undefined;
    provider?.removeAllListeners();
    provider = undefined;
  }

  function button(label: string): HTMLButtonElement {
    const element = Array.from(container.querySelectorAll('button')).find(node => node.textContent === label);
    expect(element, `Button ${label}`).toBeDefined();
    return element!;
  }

  async function click(label: string) {
    expect(button(label).disabled).toBe(false);
    await act(async () => button(label).click());
  }

  async function rendered(text: string) {
    await expect.poll(async () => {
      await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
      return container.textContent;
    }, { timeout: 5000 }).toContain(text);
  }

  try {
    localStorage.clear();
    new ChainDraftStorage(transcript.draft.creator).saveDraft(transcript.draft);
    await openWizard();
    await rendered('This name is available.');
    expect(container.querySelector<HTMLInputElement>('#chain-name')?.value).toBe(transcript.record[1]);
    await click('Next');
    await rendered('Total: 101 PAX');
    await click('Next');
    expect(transcript.submissions).toHaveLength(0);
    await click('Launch');
    await rendered('Registry readback interrupted');
    expect(transcript.unexpected).toEqual([]);
    expect(transcript.submissions).toHaveLength(1);
    expect(transcript.receiptReads).toHaveLength(1);
    expect(transcript.registryReads.length).toBeGreaterThan(0);
    expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
    expect(button('Launch').disabled).toBe(true);
    const saved = ChainDraftStorage.restoreSubmission();
    expect(saved).toEqual({
      draft: { ...transcript.draft, updatedAt: expect.any(Number) },
      submission: {
        creator: transcript.draft.creator, chainId: 125, txHash: transcript.transaction.hash,
        phase: 'confirm', bondRaw: transcript.params[1].toString(),
      },
    });
    expect(saved!.draft.updatedAt).toBeGreaterThanOrEqual(transcript.draft.updatedAt);
    expect(saved!.draft.updatedAt).toBeLessThanOrEqual(Date.now());
    const journal = localStorage.getItem('chain-submission-v1');
    expect(journal).not.toBeNull();
    await closeWizard();
    localStorage.clear();
    localStorage.setItem('chain-submission-v1', journal!);
    transcript.enableReadback();
    await openWizard();
    await rendered('Total: 101 PAX');
    const registry = createPublicClient({ chain: transcript.chain, transport: http(transcript.origin, { retryCount: 0 }) });
    expect(await registry.readContract({
      address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'chainByName', args: [transcript.draft.name],
    })).toBe(transcript.record[0]);
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('3. Review and launch');
    expect(container.querySelector('[aria-label="Submitted transaction"]')?.textContent).toContain(transcript.transaction.hash);
    expect(ChainDraftStorage.restoreSubmission()).toEqual(saved);
    expect(button('Launch').disabled).toBe(true);
    expect(transcript.submissions).toHaveLength(1);
    await click('Retry confirmation');
    await rendered(`Chain ID: ${transcript.record[4]}`);
    expect(container.querySelector('[aria-label="Live chain"]')?.textContent).toContain(`Chain ID: ${transcript.record[4]}`);
    expect(container.querySelector('[aria-label="Live chain"]')?.textContent).toContain(`Registry ID: ${transcript.record[0]}`);
    expect(container.querySelector<HTMLInputElement>('#chain-rpc')?.value).toBe(`${transcript.origin}/chain/${transcript.record[0]}`);
    expect(container.querySelector('a')?.getAttribute('href')).toBe(`${transcript.origin}/explorer/chains/${transcript.record[0]}`);
    expect(ChainDraftStorage.restoreSubmission()).toBeNull();
    expect(new ChainDraftStorage(transcript.draft.creator).restoreDraft()).toBeNull();
    await click('Add to wallet');
    await rendered('Chain added to your wallet.');
    expect(transcript.walletNetworks).toEqual([[{
      chainId: toHex(transcript.record[4]), chainName: `${transcript.record[1]} on Paxeer`,
      rpcUrls: [`${transcript.origin}/chain/${transcript.record[0]}`],
      nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
      blockExplorerUrls: [`${transcript.origin}/explorer/chains/${transcript.record[0]}`],
    }]]);
    expect(transcript.receiptReads).toEqual([[transcript.transaction.hash], [transcript.transaction.hash]]);
    expect(transcript.registryReads.every(args => args[1] === '0x2a')).toBe(true);
    expect(transcript.submissions).toHaveLength(1);
    expect(transcript.unexpected).toEqual([]);
  } finally {
    await closeWizard();
    localStorage.clear();
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = previousActEnvironment;
    await transcript.close();
  }
}

test(chain_launch_path_proof_recovers_the_submitted_journal_with_one_submission.name,
  { timeout: 20000 }, chain_launch_path_proof_recovers_the_submitted_journal_with_one_submission);
