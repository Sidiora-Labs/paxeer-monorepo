import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { ethers } from 'ethers';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { PaxeerProvider, WalletInterface } from '@paxeer/wallet';
import { WalletSigner } from '../signer';
import { ACCESS_TOKEN, fixtureBody, startGateway, type TestGateway } from './gateway';
import { replay } from '../../../../../sdk/wallet/test/support/replay';

// Recorded with viem privateKeyToAccount.signTransaction, the gateway envelope signing codec.
// These immutable HTTP-boundary vectors attest submission bytes, never mining or policy.
const sendRecordings = [
  {
    request: {
      tx: {
        from: '0xE1952Ec7230F058cd7fd9BCCb15964100e38ea04',
        to: '0x00000000000000000000000000000000000000E2',
        value: '1',
        data: '0x1234',
        gas: '31000',
        maxFeePerGas: '2000000000',
        maxPriorityFeePerGas: '1000000000',
        nonce: 0,
        chainId: 125,
      },
    },
    signed:
      '0x02f86c7d80843b9aca0084773594008279189400000000000000000000000000000000000000e201821234c001a0df098a97f1d9b90d30727def26e5664f3761c7fd3a2588743212515a61e300a0a019803f4c63a95ac2f4c24e6e7e6cef0cdcb640dc3b3e39fbfd010c75d694954c',
    response: {
      tx_hash: '0x3d6a97d011b8345b3c2506a878c359a6b30f4f33a3ed994745ecd2f3c7f1df7b',
      address: '0xE1952Ec7230F058cd7fd9BCCb15964100e38ea04',
      chain_id: 125,
    },
  },
  {
    request: {
      tx: {
        from: '0xE1952Ec7230F058cd7fd9BCCb15964100e38ea04',
        to: '0x0000000000000000000000000000000000000000',
        value: '2',
        data: '0xabcd',
        gas: '31001',
        maxFeePerGas: '2000000001',
        maxPriorityFeePerGas: '1000000001',
        nonce: 1,
        chainId: 125,
      },
    },
    signed:
      '0x02f86c7d01843b9aca0184773594018279199400000000000000000000000000000000000000000282abcdc080a03bd6969d64c658d2fefe4e32b483177a6e721820bd0819756213fa611c2517c9a036caba80f09359627e0cee50cae89df628ed0347e486e0a670a05f2ce4c5d27c',
    response: {
      tx_hash: '0x0d715e05e66e8a1e9632a8c56e1523c17a65ba26c1b376a0f188d9b7230523ce',
      address: '0xE1952Ec7230F058cd7fd9BCCb15964100e38ea04',
      chain_id: 125,
    },
  },
] as const;

const recipient = '0x00000000000000000000000000000000000000e2';
const namespace = 7;
const virtualChainId = 125000000 + namespace;
const requests: Array<{ path: string; method: string; params: unknown[] }> = [];
const accepted = new Map<string, ethers.Transaction>();
const lookupWaiters = new Map<string, () => void>();
const failures: unknown[] = [];
let walletPeer: Awaited<ReturnType<typeof replay>>;
let node: Server;
let nodeUrl: string;
let gateway: TestGateway;
const token = ACCESS_TOKEN;
const address = sendRecordings[0].response.address;
const providers: ethers.JsonRpcProvider[] = [];

function transactionDocument(tx: ethers.Transaction): Record<string, unknown> {
  if (!tx.hash || !tx.from || !tx.signature) throw new Error('unsigned node submission');
  return {
    hash: tx.hash,
    type: ethers.toQuantity(tx.type!),
    from: tx.from,
    to: tx.to,
    nonce: ethers.toQuantity(tx.nonce),
    gas: ethers.toQuantity(tx.gasLimit),
    maxFeePerGas: ethers.toQuantity(tx.maxFeePerGas!),
    maxPriorityFeePerGas: ethers.toQuantity(tx.maxPriorityFeePerGas!),
    gasPrice: ethers.toQuantity(tx.maxFeePerGas!),
    value: ethers.toQuantity(tx.value),
    input: tx.data,
    chainId: ethers.toQuantity(tx.chainId),
    accessList: tx.accessList,
    r: tx.signature.r,
    s: tx.signature.s,
    v: ethers.toQuantity(tx.signature.yParity),
    yParity: ethers.toQuantity(tx.signature.yParity),
    blockHash: null,
    blockNumber: null,
    transactionIndex: null,
  };
}

async function nodeResult(path: string, method: string, params: unknown[]): Promise<unknown> {
  const chainId = path === '/' ? 125 : virtualChainId;
  switch (method) {
    case 'eth_chainId':
      return ethers.toQuantity(chainId);
    case 'eth_blockNumber':
      return path === '/' ? '0x10' : '0x2a';
    case 'eth_call':
      return '0x';
    case 'eth_getTransactionCount':
      return ethers.toQuantity(
        [...accepted.values()].filter(
          (tx) =>
            tx.from?.toLowerCase() === String(params[0]).toLowerCase() &&
            tx.chainId === BigInt(chainId)
        ).length
      );
    case 'eth_sendRawTransaction': {
      const raw = String(params[0]);
      const tx = ethers.Transaction.from(raw);
      if (!tx.isSigned() || !tx.from || tx.chainId !== BigInt(chainId))
        throw new Error('invalid signed transaction or namespace');
      if (tx.hash !== ethers.keccak256(raw)) throw new Error('signed bytes/hash disagreement');
      if (tx.from !== ethers.recoverAddress(tx.unsignedHash, tx.signature!))
        throw new Error('invalid sender signature');
      if (!accepted.has(tx.hash!)) {
        const next = [...accepted.values()].filter(
          (previous) => previous.from === tx.from && previous.chainId === tx.chainId
        ).length;
        if (tx.nonce !== next) throw new Error('unexpected transaction nonce');
        accepted.set(tx.hash!, tx);
      }
      return tx.hash;
    }
    case 'eth_getTransactionByHash': {
      const hash = String(params[0]);
      const tx = accepted.get(hash);
      lookupWaiters.get(hash)?.();
      lookupWaiters.delete(hash);
      return tx?.chainId === BigInt(chainId) ? transactionDocument(tx) : null;
    }
    // This peer only acknowledges submission; it cannot establish block finality.
    case 'pax_sequencerReceipt':
      return null;
    default:
      throw new Error(`unrecorded node method ${method}`);
  }
}

beforeAll(async () => {
  node = createServer((req, res) => {
    void (async () => {
      const path = new URL(req.url ?? '/', 'http://127.0.0.1').pathname;
      if (path !== '/' && path !== `/chain/${namespace}`) {
        res.writeHead(404);
        res.end();
        return;
      }
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const call = JSON.parse(Buffer.concat(chunks).toString()) as {
        id: number;
        method: string;
        params: unknown[];
      };
      requests.push({ path, method: call.method, params: call.params });
      const result = await nodeResult(path, call.method, call.params);
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result }));
    })().catch((error: unknown) => {
      failures.push(error);
      res.writeHead(500);
      res.end(JSON.stringify({ error: String(error) }));
    });
  });
  await new Promise<void>((resolve) => node.listen(0, '127.0.0.1', resolve));
  nodeUrl = `http://127.0.0.1:${(node.address() as AddressInfo).port}`;
  walletPeer = await replay([
    {
      path: '/v1/wallet/me',
      method: 'GET',
      authorization: `Bearer ${token}`,
      response: fixtureBody('default', 'GET', '/v1/wallet/me'),
    },
    ...sendRecordings.map((recording) => ({
      path: '/v1/wallet/send',
      method: 'POST',
      authorization: `Bearer ${token}`,
      request: recording.request,
      response: recording.response,
    })),
  ]);
  gateway = await startGateway({ walletUrl: walletPeer.url, nodeUrl });
}, 30_000);

afterAll(async () => {
  for (const provider of providers) provider.destroy();
  await gateway?.close();
  await walletPeer?.close();
  if (node) {
    node.closeAllConnections();
    await new Promise<void>((resolve) => node.close(() => resolve()));
  }
  expect(walletPeer?.errors).toEqual([]);
  expect(failures).toEqual([]);
}, 30_000);

function readProvider(url: string, chainId: number): ethers.JsonRpcProvider {
  const provider = new ethers.JsonRpcProvider(url, chainId, {
    staticNetwork: true,
    batchMaxCount: 1,
  });
  providers.push(provider);
  return provider;
}

async function synthetic_chain_response_binds_producer_bytes_to_signer_lookup(): Promise<void> {
  const wallet = new WalletInterface(
    new PaxeerProvider({
      gatewayUrl: gateway.config.gatewayUrl,
      rpcUrl: gateway.config.rpcUrl,
      chainId: 125,
      token: () => token,
    })
  );
  const signer = new WalletSigner(wallet, readProvider(gateway.config.rpcUrl, 125));
  expect(await signer.getAddress()).toBe(address);
  const publisher = readProvider(nodeUrl, 125);
  const hashes: string[] = [];
  for (const nonce of [0, 1]) {
    const recording = sendRecordings[nonce];
    const request = {
      from: address,
      to: nonce === 0 ? recipient : ethers.ZeroAddress,
      value: BigInt(nonce + 1),
      data: nonce === 0 ? '0x1234' : '0xabcd',
      gasLimit: 31_000n + BigInt(nonce),
      maxFeePerGas: 2_000_000_000n + BigInt(nonce),
      maxPriorityFeePerGas: 1_000_000_000n + BigInt(nonce),
      nonce,
      chainId: 125,
    };
    const lookupStarted = new Promise<void>((resolve) => {
      lookupWaiters.set(recording.response.tx_hash, resolve);
    });
    const pendingResponse = signer.sendTransaction(request);
    await Promise.race([
      lookupStarted,
      pendingResponse.then(() => {
        throw new Error('signer returned a transaction before its bytes reached the node');
      }),
    ]);
    expect(accepted.has(recording.response.tx_hash)).toBe(false);
    expect(walletPeer.seen).toContainEqual({
      path: '/v1/wallet/send',
      method: 'POST',
      body: recording.request,
      authorization: `Bearer ${token}`,
    });
    // The recorded HTTP acknowledgement cannot manufacture a lookup result.
    // Only submitting its original signed bytes makes this transaction visible.
    expect(await publisher.send('eth_sendRawTransaction', [recording.signed])).toBe(
      recording.response.tx_hash
    );
    const response = await pendingResponse;
    const raw = requests.filter((entry) => entry.method === 'eth_sendRawTransaction')[nonce]
      ?.params[0];
    expect(typeof raw).toBe('string');
    expect(raw).toBe(sendRecordings[nonce].signed);
    const produced = ethers.Transaction.from(String(raw));
    expect(response.hash).toBe(ethers.keccak256(String(raw)));
    expect(response.hash).toBe(produced.hash);
    expect(response.hash).toBe(sendRecordings[nonce].response.tx_hash);
    expect(response.from).toBe(address);
    expect(response.to).toBe(ethers.getAddress(request.to));
    expect(response.value).toBe(request.value);
    expect(response.data).toBe(request.data);
    expect(response.gasLimit).toBe(request.gasLimit);
    expect(response.maxFeePerGas).toBe(request.maxFeePerGas);
    expect(response.maxPriorityFeePerGas).toBe(request.maxPriorityFeePerGas);
    expect(response.nonce).toBe(nonce);
    expect(response.chainId).toBe(125n);
    expect(response.signature).toEqual(produced.signature);
    expect(ethers.recoverAddress(produced.unsignedHash, response.signature!)).toBe(address);
    const reconstructed = ethers.Transaction.from({
      type: response.type,
      to: response.to,
      nonce: response.nonce,
      gasLimit: response.gasLimit,
      maxFeePerGas: response.maxFeePerGas,
      maxPriorityFeePerGas: response.maxPriorityFeePerGas,
      value: response.value,
      data: response.data,
      chainId: response.chainId,
      accessList: response.accessList,
      signature: response.signature,
    });
    expect(reconstructed.serialized).toBe(raw);
    expect(response.blockNumber).toBeNull();
    expect(response.isMined()).toBe(false);
    expect(requests).toContainEqual({
      path: '/',
      method: 'eth_getTransactionByHash',
      params: [response.hash],
    });
    hashes.push(response.hash);
  }
  expect(hashes[0]).not.toBe(hashes[1]);
  expect(accepted.size).toBe(2);
  expect(
    gateway.requests.filter((entry) => entry.path === '/v1/wallet/send').map((entry) => entry.body)
  ).toEqual(
    [0, 1].map((nonce) => ({
      tx: {
        from: address,
        to: nonce === 0 ? ethers.getAddress(recipient) : ethers.ZeroAddress,
        value: String(nonce + 1),
        data: nonce === 0 ? '0x1234' : '0xabcd',
        gas: String(31_000 + nonce),
        maxFeePerGas: String(2_000_000_000 + nonce),
        maxPriorityFeePerGas: String(1_000_000_000 + nonce),
        nonce,
        chainId: 125,
      },
    }))
  );
}

async function test_retired_rpc_path_uses_root_and_selected_namespace(): Promise<void> {
  expect(new URL(gateway.config.rpcUrl).pathname).toBe('/');
  const selected = await startGateway({ nodeUrl, namespace });
  const unit = await startGateway();
  try {
    expect(new URL(unit.config.rpcUrl).pathname).toBe('/');
    expect(await readProvider(unit.config.rpcUrl, 125).send('eth_chainId', [])).toBe('0x7d');
    expect(unit.requests[unit.requests.length - 1]?.path).toBe('/');
    expect(new URL(selected.config.rpcUrl).pathname).toBe(`/chain/${namespace}`);
    expect(selected.config.chainId).toBe(virtualChainId);
    const provider = new PaxeerProvider({
      gatewayUrl: gateway.url,
      rpcUrl: gateway.config.rpcUrl,
      chainId: 125,
      token: () => token,
    });
    const start = requests.length;
    expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x10');
    const selectedProvider = new PaxeerProvider({
      gatewayUrl: gateway.url,
      rpcUrl: selected.config.rpcUrl,
      chainId: virtualChainId,
      token: () => token,
    });
    expect(await selectedProvider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
    expect(await readProvider(selected.config.rpcUrl, virtualChainId).send('eth_chainId', [])).toBe(
      ethers.toQuantity(virtualChainId)
    );
    expect(
      requests
        .slice(start)
        .filter((entry) => entry.method === 'eth_blockNumber')
        .map((entry) => entry.path)
    ).toEqual(['/', `/chain/${namespace}`]);
    expect(requests.slice(start)).toContainEqual({
      path: `/chain/${namespace}`,
      method: 'eth_chainId',
      params: [],
    });
    for (const path of ['/rpc', `/rpc/chain/${namespace}`]) {
      const refused = await fetch(`${nodeUrl}${path}`, {
        method: 'POST',
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_chainId', params: [] }),
      });
      expect(refused.status).toBe(404);
      const fixtureRefused = await fetch(`${unit.url}${path}`, { method: 'POST', body: '{}' });
      expect(fixtureRefused.status).toBe(401);
    }
    expect(requests.some((entry) => entry.path.startsWith('/rpc'))).toBe(false);
  } finally {
    await selected.close();
    await unit.close();
  }
}

test(
  synthetic_chain_response_binds_producer_bytes_to_signer_lookup.name,
  synthetic_chain_response_binds_producer_bytes_to_signer_lookup,
  30_000
);
test(
  test_retired_rpc_path_uses_root_and_selected_namespace.name,
  test_retired_rpc_path_uses_root_and_selected_namespace,
  30_000
);
