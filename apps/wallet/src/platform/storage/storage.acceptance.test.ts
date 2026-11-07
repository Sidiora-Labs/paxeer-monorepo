// @vitest-environment jsdom

import { beforeEach, describe, expect, it, test, vi } from 'vitest';
import { ACCESS_TOKEN, startGateway } from '@/wallet/test/gateway';
import { getBackgroundFailures } from '@/platform/status/background-failures';
import { resetStorageForLifecycle, storageCatalog, StorageCorruptionError } from './registry';
const previousRpcUrl = process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
process.env.NEXT_PUBLIC_PAXEER_RPC_URL = 'https://rpc.paxeer.example';
const { getActiveRpcUrl, PAXEER_CONFIG } = await import('@/lib/constants');
const { virtualNetwork } = await import('@/lib/networks');
const { WalletNetworkProvider } = await import('@/wallet/WalletProvider');
const {
  contactsRepository,
  custodyChoiceRepository,
  pendingSendRepository,
  preferencesRepository,
  transferObservationsRepository,
} = await import('./repositories');
if (previousRpcUrl === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
else process.env.NEXT_PUBLIC_PAXEER_RPC_URL = previousRpcUrl;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe('application storage registry', () => {
  it('catalogs every record with ownership, lifecycle, quota, and prohibited data', () => {
    const catalog = storageCatalog();
    expect(catalog.length).toBeGreaterThanOrEqual(11);
    expect(new Set(catalog.map((entry) => entry.id)).size).toBe(catalog.length);
    for (const entry of catalog) {
      expect(entry.owner).toBeTruthy();
      expect(entry.schema).toBeTruthy();
      expect(entry.retention).toBeTruthy();
      expect(entry.migration).toBeTruthy();
      expect(entry.quotaBytes).toBeGreaterThan(0);
      expect(entry.resetOn.length).toBeGreaterThan(0);
      expect(entry.prohibitedData).toContain('mnemonic');
      expect(entry.prohibitedData).toContain('private key');
    }
  });

  it('migrates and validates a legacy contact record into a versioned envelope', () => {
    localStorage.setItem(
      'paxeer_contacts',
      JSON.stringify([
        {
          id: 'contact-1',
          name: 'Treasury',
          address: '0x1111111111111111111111111111111111111111',
          createdAt: 1,
          updatedAt: 1,
        },
      ])
    );
    expect(contactsRepository.read()).toHaveLength(1);
    expect(localStorage.getItem('paxeer_contacts')).toBeNull();
    expect(JSON.parse(localStorage.getItem('paxport:v1:contacts') ?? '{}')).toMatchObject({
      version: 1,
    });
  });

  it('signals and resets optional corruption while security state fails closed', () => {
    localStorage.setItem('paxport:v1:contacts', '{"version":1,"value":');
    expect(contactsRepository.read()).toEqual([]);
    expect(getBackgroundFailures()[0]).toMatchObject({
      domain: 'storage',
      code: 'STORAGE_CORRUPT',
    });

    localStorage.setItem(
      'paxport:v1:custody-choice',
      JSON.stringify({ version: 1, writtenAt: 1, value: 'unknown' })
    );
    expect(() => custodyChoiceRepository.read()).toThrow(StorageCorruptionError);
  });

  it('clears only lifecycle-owned records', () => {
    preferencesRepository.write({
      currency: 'EUR',
      language: 'de',
      customRpc: null,
      networks: [],
      activeChainId: null,
      customNonce: null,
      feeMode: 'auto',
      customMaxFeeGwei: null,
      customPriorityFeeGwei: null,
      developerMode: false,
      showHexData: false,
      notifications: {},
    });
    custodyChoiceRepository.write('embedded');
    pendingSendRepository.write({
      symbol: 'PAX',
      amount: '1',
      decimals: 18,
      recipient: '0x1111111111111111111111111111111111111111',
      txHash: '0x1111111111111111111111111111111111111111111111111111111111111111',
      timestamp: 1,
    });

    resetStorageForLifecycle('custody-switch');

    expect(custodyChoiceRepository.read()).toBeNull();
    expect(pendingSendRepository.read()).toBeNull();
    expect(preferencesRepository.read().currency).toBe('EUR');
  });

  it('keeps the newest hundred legacy transfer observations by block, whatever the key order', () => {
    for (let block = 103; block >= 1; block -= 1) {
      localStorage.setItem(
        `paxeer.wallet.transferObservation:125:0x${block.toString(16)}`,
        JSON.stringify({ blockNumber: block })
      );
    }
    localStorage.setItem(
      'paxeer.wallet.transferObservation:125:0xpending-a',
      JSON.stringify({ blockNumber: null })
    );
    localStorage.setItem(
      'paxeer.wallet.transferObservation:125:0xpending-b',
      JSON.stringify({ truth: 'unknown' })
    );
    const kept = transferObservationsRepository.read();
    expect(Object.keys(kept)).toHaveLength(100);
    expect(kept['125:0xpending-a']).toEqual({ blockNumber: null });
    expect(kept['125:0xpending-b']).toEqual({ truth: 'unknown' });
    for (let block = 1; block <= 5; block += 1)
      expect(kept[`125:0x${block.toString(16)}`]).toBeUndefined();
    for (let block = 6; block <= 103; block += 1)
      expect(kept[`125:0x${block.toString(16)}`]).toEqual({ blockNumber: block });
  });

  it('skips a corrupt legacy transfer entry with a warning and migrates the rest', () => {
    const before = getBackgroundFailures().length;
    localStorage.setItem('paxeer.wallet.transferObservation:125:0xbad', '{not json');
    localStorage.setItem(
      'paxeer.wallet.transferObservation:125:0xgood',
      JSON.stringify({ status: 'confirmed' })
    );
    expect(transferObservationsRepository.read()).toEqual({
      '125:0xgood': { status: 'confirmed' },
    });
    const failures = getBackgroundFailures();
    expect(failures.length).toBe(before + 1);
    expect(failures[0]).toMatchObject({ domain: 'storage', code: 'STORAGE_LEGACY_ENTRY_SKIPPED' });
    expect(failures.some((failure) => failure.code === 'STORAGE_MIGRATION_FAILED')).toBe(false);
    expect(localStorage.getItem('paxeer.wallet.transferObservation:125:0xbad')).toBeNull();
    expect(localStorage.getItem('paxeer.wallet.transferObservation:125:0xgood')).toBeNull();
    expect(transferObservationsRepository.read()).toEqual({
      '125:0xgood': { status: 'confirmed' },
    });
    expect(getBackgroundFailures().length).toBe(before + 1);
  });
});

async function preferences_keep_virtual_networks_and_refuse_bad_ones() {
  const rpcFetch = vi.fn<typeof fetch>(async (_input, init) => {
    const request = JSON.parse(String(init?.body));
    expect(request).toMatchObject({ jsonrpc: '2.0', method: 'eth_blockNumber', params: [] });
    return Response.json({ jsonrpc: '2.0', id: request.id, result: '0x2a' });
  });
  const providerConfig = {
    gatewayUrl: 'https://wallet.paxeer.example',
    rpcUrl: PAXEER_CONFIG.rpcUrl,
    chainId: PAXEER_CONFIG.chainId,
    token: () => null,
    fetch: rpcFetch,
  };
  const provider = new WalletNetworkProvider(providerConfig);
  const chainChanged = vi.fn();
  const capsInvalidated = vi.fn();
  provider.on('chainChanged', chainChanged);
  provider.on('message', capsInvalidated);
  const first = virtualNetwork(125000001, 'First virtual chain');
  const chainId = `0x${first.chainId.toString(16)}`;
  preferencesRepository.update((current) => ({
    ...current,
    customRpc: 'https://custom.paxeer.example/rpc',
  }));
  expect(
    await provider.request({
      method: 'wallet_addEthereumChain',
      params: [
        {
          chainId,
          chainName: first.name,
          rpcUrls: [first.rpcUrl],
          blockExplorerUrls: [first.explorerUrl],
        },
      ],
    })
  ).toBeNull();
  expect(preferencesRepository.read().networks).toEqual([first]);
  expect(preferencesRepository.read().activeChainId).toBeNull();
  expect(getActiveRpcUrl()).toBe('https://custom.paxeer.example/rpc');
  expect(
    await provider.request({ method: 'wallet_switchEthereumChain', params: [{ chainId }] })
  ).toBeNull();
  expect(preferencesRepository.read().activeChainId).toBe(first.chainId);
  expect(getActiveRpcUrl()).toBe(`${PAXEER_CONFIG.rpcUrl}/chain/1`);
  expect(await provider.request({ method: 'eth_chainId' })).toBe('0x7735941');
  expect(await provider.request({ method: 'net_version' })).toBe('125000001');
  expect(chainChanged.mock.calls).toEqual([['0x7735941']]);
  expect(capsInvalidated).toHaveBeenCalledWith({ type: 'wallet_caps_invalidated' });
  expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
  expect(rpcFetch).toHaveBeenLastCalledWith(`${PAXEER_CONFIG.rpcUrl}/chain/1`, expect.any(Object));

  const restarted = new WalletNetworkProvider(providerConfig);
  expect(getActiveRpcUrl()).toBe(first.rpcUrl);
  expect(await restarted.request({ method: 'eth_chainId' })).toBe('0x7735941');
  expect(await restarted.request({ method: 'net_version' })).toBe('125000001');
  expect(await restarted.request({ method: 'eth_blockNumber' })).toBe('0x2a');
  expect(rpcFetch).toHaveBeenLastCalledWith(first.rpcUrl, expect.any(Object));
  expect(
    await restarted.request({ method: 'wallet_switchEthereumChain', params: [{ chainId }] })
  ).toBeNull();
  expect(getActiveRpcUrl()).toBe(first.rpcUrl);
  expect(
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: '0x7d' }],
    })
  ).toBeNull();
  expect(await provider.request({ method: 'eth_chainId' })).toBe('0x7d');
  expect(await provider.request({ method: 'net_version' })).toBe('125');
  expect(getActiveRpcUrl()).toBe('https://custom.paxeer.example/rpc');
  expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
  expect(rpcFetch).toHaveBeenLastCalledWith(
    'https://custom.paxeer.example/rpc',
    expect.any(Object)
  );
  expect(chainChanged.mock.calls).toEqual([['0x7735941'], ['0x7d']]);
  preferencesRepository.update((current) => ({ ...current, customRpc: null }));
  expect(
    await provider.request({ method: 'wallet_switchEthereumChain', params: [{ chainId: '0x7d' }] })
  ).toBeNull();
  expect(chainChanged).toHaveBeenCalledTimes(2);
  expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
  expect(await provider.request({ method: 'eth_blockNumber' })).toBe('0x2a');
  expect(rpcFetch).toHaveBeenLastCalledWith(PAXEER_CONFIG.rpcUrl, expect.any(Object));
  preferencesRepository.update((current) => ({
    ...current,
    customRpc: 'https://custom.paxeer.example/rpc',
  }));
  await restarted.request({ method: 'wallet_switchEthereumChain', params: [{ chainId }] });
  expect(
    await restarted.request({
      method: 'wallet_addEthereumChain',
      params: [{ chainId, chainName: 'Renamed chain' }],
    })
  ).toBeNull();
  expect(preferencesRepository.read().networks).toEqual([{ ...first, name: 'Renamed chain' }]);

  const networks = Array.from({ length: 32 }, (_, index) =>
    virtualNetwork(125000001 + index, `Chain ${index + 1}`)
  );
  preferencesRepository.update((current) => ({ ...current, networks }));
  expect(preferencesRepository.read().networks).toEqual(networks);
  const good = preferencesRepository.read();
  expect(() =>
    preferencesRepository.update((current) => ({
      ...current,
      networks: [...networks, virtualNetwork(125000033, 'Thirty-third')],
    }))
  ).toThrow(TypeError);
  expect(() =>
    preferencesRepository.update((current) => ({
      ...current,
      networks: [{ ...first, chainId: PAXEER_CONFIG.chainId }],
    }))
  ).toThrow(TypeError);
  expect(() =>
    preferencesRepository.update((current) => ({
      ...current,
      networks: [{ ...first, kind: 'root' }],
    }))
  ).toThrow(TypeError);
  for (const field of ['rpcUrl', 'explorerUrl'] as const) {
    for (const badUrl of ['http://rpc.example', 'https://user:password@rpc.example', 'https://']) {
      expect(() =>
        preferencesRepository.update((current) => ({
          ...current,
          networks: [{ ...first, [field]: badUrl }],
        }))
      ).toThrow(TypeError);
    }
  }
  expect(() =>
    preferencesRepository.update((current) => ({ ...current, networks: [first, first] }))
  ).toThrow(TypeError);
  expect(() =>
    preferencesRepository.update((current) => ({ ...current, activeChainId: 1.5 }))
  ).toThrow(TypeError);
  expect(preferencesRepository.read()).toEqual(good);
  await expect(
    restarted.request({
      method: 'wallet_addEthereumChain',
      params: [{ chainId: `0x${PAXEER_CONFIG.chainId.toString(16)}`, chainName: 'Root' }],
    })
  ).rejects.toMatchObject({ code: 4200 });
  await expect(
    restarted.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: `0x${(125000100).toString(16)}` }],
    })
  ).rejects.toMatchObject({ code: 4901 });
  await expect(
    restarted.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: '0x1' }],
    })
  ).rejects.toMatchObject({ code: 4901 });
  await expect(
    restarted.request({
      method: 'wallet_addEthereumChain',
      params: [{ chainId: '125000001', chainName: 'Bad id' }],
    })
  ).rejects.toMatchObject({ code: -32602 });
  expect(preferencesRepository.read()).toEqual(good);
  expect(
    await restarted.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: `0x${PAXEER_CONFIG.chainId.toString(16)}` }],
    })
  ).toBeNull();
  expect(preferencesRepository.read().activeChainId).toBeNull();
  expect(getActiveRpcUrl()).toBe('https://custom.paxeer.example/rpc');

  preferencesRepository.update((current) => ({
    ...current,
    activeChainId: first.chainId,
    networks: [],
  }));
  expect(getActiveRpcUrl()).toBe('https://custom.paxeer.example/rpc');
  preferencesRepository.update((current) => ({ ...current, customRpc: null }));
  expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);

  const { networks: _networks, activeChainId: _activeChainId, ...oldPreferences } = good;
  localStorage.setItem(
    preferencesRepository.registration.key,
    JSON.stringify({ version: 1, writtenAt: 1, value: oldPreferences })
  );
  expect(preferencesRepository.read()).toEqual({
    ...oldPreferences,
    networks: [],
    activeChainId: null,
  });
  preferencesRepository.update((current) => ({ ...current, activeChainId: null }));
  expect(
    JSON.parse(localStorage.getItem(preferencesRepository.registration.key) ?? '{}').value
  ).toEqual({
    ...oldPreferences,
    networks: [],
    activeChainId: null,
  });
}

test(preferences_keep_virtual_networks_and_refuse_bad_ones);

async function virtual_network_refuses_transactions_and_typed_data_for_another_chain() {
  const gateway = await startGateway();
  try {
    const config = { ...gateway.config, token: () => ACCESS_TOKEN };
    const provider = new WalletNetworkProvider(config);
    const accounts = await provider.request({ method: 'eth_requestAccounts' });
    expect(accounts).toEqual([expect.stringMatching(/^0x[0-9a-fA-F]{40}$/)]);
    const network = virtualNetwork(125000001, 'Selected virtual chain');
    preferencesRepository.update((current) => ({ ...current, networks: [network] }));
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: '0x7735941' }],
    });
    expect(await provider.request({ method: 'eth_requestAccounts' })).toEqual(accounts);
    const restarted = new WalletNetworkProvider(config);
    expect(await restarted.request({ method: 'eth_requestAccounts' })).toEqual(accounts);
    expect(await restarted.request({ method: 'eth_chainId' })).toBe('0x7735941');
    expect(await restarted.request({ method: 'net_version' })).toBe('125000001');
    const requestsBefore = gateway.requests.length;
    for (const wallet of [provider, restarted]) {
      await expect(
        wallet.request({
          method: 'eth_sendTransaction',
          params: [{ chainId: '0x7d' }],
        })
      ).rejects.toMatchObject({
        name: 'ChainDisconnectedError',
        code: 4901,
        requestedChainId: 125,
        connectedChainId: 125000001,
      });
      await expect(
        wallet.request({
          method: 'eth_signTypedData_v4',
          params: [
            (accounts as string[])[0],
            {
              domain: { chainId: 125 },
              types: { EIP712Domain: [{ name: 'chainId', type: 'uint256' }] },
              primaryType: 'EIP712Domain',
              message: {},
            },
          ],
        })
      ).rejects.toMatchObject({
        name: 'ChainDisconnectedError',
        code: 4901,
        requestedChainId: 125,
        connectedChainId: 125000001,
      });
    }
    expect(gateway.requests).toHaveLength(requestsBefore);
  } finally {
    await gateway.close();
  }
}

test(virtual_network_refuses_transactions_and_typed_data_for_another_chain);
