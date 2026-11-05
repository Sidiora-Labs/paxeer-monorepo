import { readFileSync } from 'node:fs';
import { createServer, request as httpsRequest } from 'node:https';
import type { AddressInfo } from 'node:net';
import path from 'node:path';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { expect } from 'vitest';
import { ethers } from 'ethers';
import { ChainDisconnectedError, type Hex } from '@paxeer/wallet';
import { getActiveRpcUrl, PAXEER_CONFIG } from '@/lib/constants';
import { virtualNetwork } from '@/lib/networks';
import { custodyChoiceRepository, preferencesRepository } from '@/platform/storage/repositories';
import { IdentitySession } from '../identity';
import {
  WalletNetworkProvider,
  WalletProvider,
  useWallet,
  type WalletContextValue,
} from '../WalletProvider';
import {
  ACCESS_TOKEN,
  EMAIL,
  EMAIL_CODE,
  FIXTURE_KEY,
  fixtureBody,
  loadConstructions,
  startGateway,
} from './gateway';

// A local TLS endpoint exercises the same HTTPS URLs accepted by saved-network validation.
const certificate = readFileSync(path.join(__dirname, 'loopback.cert.fixture'));
const key = readFileSync(path.join(__dirname, 'loopback.key.fixture'));

const trustedLoopbackFetch: typeof fetch = async (input, init) => {
  const url = new URL(
    typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
  );
  if (url.protocol !== 'https:') return fetch(input, init);
  return new Promise<Response>((resolve, reject) => {
    const request = httpsRequest(
      url,
      {
        ca: certificate,
        method: init?.method,
        headers: Object.fromEntries(new Headers(init?.headers)),
        signal: init?.signal ?? undefined,
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on('data', (chunk: Buffer) => chunks.push(chunk));
        response.on('error', reject);
        response.on('end', () =>
          resolve(
            new Response(Buffer.concat(chunks).toString('utf8'), {
              status: response.statusCode,
              headers: { 'content-type': 'application/json' },
            })
          )
        );
      }
    );
    request.on('error', reject);
    request.end(init?.body);
  });
};

export async function runProviderSwitchRestartCoverage(): Promise<void> {
  const gateway = await startGateway();
  const originalConfig = { rpcUrl: PAXEER_CONFIG.rpcUrl, chainId: PAXEER_CONFIG.chainId };
  const sent: ethers.Transaction[] = [];
  const routed: string[] = [];
  const sendBodies: unknown[] = [];
  const failures: unknown[] = [];
  const signingWallet = new ethers.Wallet(FIXTURE_KEY);
  const server = createServer({ cert: certificate, key }, (request, response) => {
    void (async () => {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = chunks.length ? JSON.parse(Buffer.concat(chunks).toString()) : null;
      const route = request.url ?? '/';
      response.setHeader('content-type', 'application/json');
      if (route === '/v1/wallet/me') {
        expect(request.headers.authorization).toBe(`Bearer ${ACCESS_TOKEN}`);
        response.end(JSON.stringify(fixtureBody('default', 'GET', route)));
        return;
      }
      if (route === '/v1/wallet/send') {
        expect(request.headers.authorization).toBe(`Bearer ${ACCESS_TOKEN}`);
        sendBodies.push(body);
        const tx = body.tx;
        const raw = await signingWallet.signTransaction({
          type: 2,
          to: tx.to,
          data: tx.data ?? '0x',
          value: BigInt(tx.value),
          gasLimit: BigInt(tx.gas ?? '21000'),
          maxFeePerGas: BigInt(tx.maxFeePerGas ?? '2000000000'),
          maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas ?? '1000000000'),
          nonce: tx.nonce ?? sent.length,
          chainId: tx.chainId,
        });
        const parsed = ethers.Transaction.from(raw);
        sent.push(parsed);
        response.end(
          JSON.stringify({
            tx_hash: parsed.hash,
            address: parsed.from,
            chain_id: Number(parsed.chainId),
          })
        );
        return;
      }
      expect(['/', '/chain/1']).toContain(route);
      routed.push(route);
      expect(body.method).toBe('eth_blockNumber');
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          result: route === '/chain/1' ? '0x2a' : '0x10',
        })
      );
    })().catch((error: unknown) => {
      failures.push(error);
      response.statusCode = 500;
      response.end(JSON.stringify({ error: 'loopback_fixture_failure' }));
    });
  });
  const container = document.createElement('div');
  document.body.appendChild(container);
  let root: Root | null = null;
  let current: WalletContextValue | null = null;
  const identities: IdentitySession[] = [];
  function context(): WalletContextValue {
    if (!current) throw new Error('the connected network probe did not render');
    return current;
  }
  function Probe() {
    const value = useWallet();
    useLayoutEffect(() => {
      current = value;
    });
    return null;
  }
  async function unmount() {
    await act(async () => root?.unmount());
    root = null;
    current = null;
  }
  try {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const origin = `https://127.0.0.1:${(server.address() as AddressInfo).port}`;
    Object.assign(PAXEER_CONFIG, { rpcUrl: origin, chainId: 125 });
    const config = { ...gateway.config, gatewayUrl: origin, rpcUrl: origin };
    const target = new EventTarget();
    async function mount() {
      const identity = IdentitySession.create(config, { detectSessionInUrl: false });
      identities.push(identity);
      await act(async () => {
        root = createRoot(container);
        root.render(
          createElement(WalletProvider, {
            config,
            identity,
            eventTarget: target,
            fetch: trustedLoopbackFetch,
            children: createElement(Probe),
          })
        );
      });
    }
    async function until(status: WalletContextValue['status']) {
      for (let attempt = 0; attempt < 200; attempt += 1) {
        if (context().status === status) return;
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 10));
        });
      }
      throw new Error(`expected ${status}, got ${context().status}: ${context().error}`);
    }
    await mount();
    await until('signed-out');
    await act(async () => {
      await context().verifyEmailCode(EMAIL, EMAIL_CODE);
    });
    const original = context();
    const wallet = original.wallet;
    if (!(wallet?.provider instanceof WalletNetworkProvider))
      throw new Error('no network provider');
    const provider = wallet.provider;
    const network = virtualNetwork(125000001, 'Loopback virtual network');
    const recipient = loadConstructions().transaction.to as Hex;
    expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x10');
    expect(routed).toEqual(['/']);
    await act(async () => {
      await provider.request({
        method: 'wallet_addEthereumChain',
        params: [
          {
            chainId: ethers.toQuantity(network.chainId),
            chainName: network.name,
            rpcUrls: [network.rpcUrl],
            blockExplorerUrls: [network.explorerUrl],
          },
        ],
      });
      await provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: ethers.toQuantity(network.chainId) }],
      });
    });
    expect(context().status).toBe('ready');
    expect(context().chainId).toBe(network.chainId);
    expect(context().wallet).toBe(wallet);
    expect(context().identity).toBe(original.identity);
    expect(context().address).toBe(original.address);
    expect(provider.isConnected()).toBe(true);
    expect(await wallet.chainId()).toBe(network.chainId);
    expect(getActiveRpcUrl()).toBe(network.rpcUrl);
    expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
    expect(routed).toEqual(['/', '/chain/1']);
    await act(async () => {
      const hash = await context().send({ to: recipient, value: '0.001' });
      expect(hash).toBe(sent[0]?.hash);
    });
    expect(sent).toHaveLength(1);
    expect(sent[0].chainId).toBe(BigInt(network.chainId));
    expect(sent[0].from).toBe(signingWallet.address);
    expect(sent[0].to).toBe(ethers.getAddress(recipient));
    expect(sent[0].value).toBe(ethers.parseEther('0.001'));
    expect(sendBodies).toEqual([
      {
        tx: {
          from: signingWallet.address,
          to: ethers.getAddress(recipient),
          value: '1000000000000000',
          chainId: network.chainId,
        },
      },
    ]);
    await expect(
      context().sendTransaction({ to: recipient, value: 1n, chainId: 125 })
    ).rejects.toBeInstanceOf(ChainDisconnectedError);
    await expect(
      wallet.sendTransaction({ to: recipient, value: 1n, chainId: 125 })
    ).rejects.toBeInstanceOf(ChainDisconnectedError);
    expect(sendBodies).toHaveLength(1);
    const saved = preferencesRepository.read();
    expect(saved.networks).toEqual([network]);
    expect(saved.activeChainId).toBe(network.chainId);
    expect(custodyChoiceRepository.read()).toBe('embedded');
    await unmount();
    await identities[0].client.auth.stopAutoRefresh();
    await mount();
    await until('ready');
    expect(context().wallet).not.toBe(wallet);
    expect(context().wallet?.provider).not.toBe(provider);
    expect(context().status).toBe('ready');
    expect(context().chainId).toBe(network.chainId);
    expect(context().address).toBe(original.address);
    expect(context().identity).toEqual(original.identity);
    expect(preferencesRepository.read()).toEqual(saved);
    expect(getActiveRpcUrl()).toBe(network.rpcUrl);
    const restored = context().wallet;
    if (!restored) throw new Error('no restored wallet');
    expect(await restored.chainId()).toBe(network.chainId);
    expect(await restored.provider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
    expect(routed).toEqual(['/', '/chain/1', '/chain/1']);
    await act(async () => {
      const hash = await context().sendTransaction({ to: recipient, value: 2n });
      expect(hash).toBe(sent[1]?.hash);
      await context().refresh();
    });
    expect(sent).toHaveLength(2);
    expect(sent[1].chainId).toBe(BigInt(network.chainId));
    expect(sent[1].value).toBe(2n);
    expect(context().status).toBe('ready');
    expect(gateway.requests.filter((entry) => entry.path === '/auth/v1/verify')).toHaveLength(1);
    expect(gateway.requests.filter((entry) => entry.path === '/auth/v1/logout')).toEqual([]);
    expect(failures).toEqual([]);
  } finally {
    await unmount();
    for (const identity of identities) await identity.client.auth.stopAutoRefresh();
    container.remove();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
    await gateway.close();
    Object.assign(PAXEER_CONFIG, originalConfig);
  }
}
