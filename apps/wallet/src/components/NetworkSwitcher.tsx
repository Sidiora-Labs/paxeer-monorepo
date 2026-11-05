'use client';

import { useEffect, useState, type FormEvent } from 'react';
import { announceRpcChanged, PAXEER_CONFIG, RPC_CHANGED_EVENT } from '@/lib/constants';
import {
  networkForChainId,
  rootNetwork,
  virtualNetwork,
  VIRTUAL_CHAIN_ID_BASE,
} from '@/lib/networks';
import { preferencesRepository } from '@/platform/storage/repositories';

function useNetworkPreferences() {
  const [preferences, setPreferences] = useState(() => preferencesRepository.read());
  useEffect(() => {
    const refresh = () => setPreferences(preferencesRepository.read());
    window.addEventListener(RPC_CHANGED_EVENT, refresh);
    window.addEventListener('storage', refresh);
    return () => {
      window.removeEventListener(RPC_CHANGED_EVENT, refresh);
      window.removeEventListener('storage', refresh);
    };
  }, []);
  return preferences;
}

export function useActiveNetwork() {
  const preferences = useNetworkPreferences();
  return networkForChainId(preferences.activeChainId, preferences.networks);
}

function parseVirtualId(input: string): number {
  let value = input.trim();
  if (!/^\d+$/.test(value)) {
    let url: URL;
    try {
      url = new URL(value);
    } catch {
      throw new Error('Enter a virtual chain ID or an explorer chain URL.');
    }
    const explorer = new URL(PAXEER_CONFIG.explorerBase);
    const prefix = `${explorer.pathname.replace(/\/+$/, '')}/chains/`;
    if (
      url.protocol !== 'https:' ||
      url.username ||
      url.password ||
      url.origin !== explorer.origin ||
      !url.pathname.startsWith(prefix)
    ) {
      throw new Error('Use a chain URL from the configured explorer.');
    }
    value = url.pathname.slice(prefix.length).replace(/\/$/, '');
  }
  const id = Number(value);
  if (!/^\d+$/.test(value) || !Number.isSafeInteger(id) || id < 1 || id > 4294967295) {
    throw new Error('Virtual chain ID must be an integer from 1 to 4294967295.');
  }
  return id;
}

export function NetworkSwitcher() {
  const preferences = useNetworkPreferences();
  const active = networkForChainId(preferences.activeChainId, preferences.networks);
  const [input, setInput] = useState('');
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState('');
  const [status, setStatus] = useState('');

  const switchNetwork = (chainId: number) => {
    try {
      preferencesRepository.update((current) => ({ ...current, activeChainId: chainId }));
      if (preferencesRepository.read().activeChainId !== chainId) {
        throw new Error('The active network could not be saved on this device.');
      }
      announceRpcChanged();
      setError('');
      setStatus('Network switched.');
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Could not switch networks.');
    }
  };

  const removeNetwork = (chainId: number) => {
    try {
      preferencesRepository.update((current) => ({
        ...current,
        networks: current.networks.filter((network) => network.chainId !== chainId),
        activeChainId:
          current.activeChainId === chainId ? rootNetwork().chainId : current.activeChainId,
      }));
      if (preferencesRepository.read().networks.some((network) => network.chainId === chainId)) {
        throw new Error('The network could not be removed on this device.');
      }
      announceRpcChanged();
      setError('');
      setStatus('Network removed.');
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Could not remove the network.');
    }
  };

  const addNetwork = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (adding) return;
    setAdding(true);
    setError('');
    setStatus('');
    try {
      const id = parseVirtualId(input);
      const network = virtualNetwork(VIRTUAL_CHAIN_ID_BASE + id, `Virtual chain ${id}`);
      const response = await fetch(network.rpcUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_chainId', params: [] }),
        signal: AbortSignal.timeout(10_000),
        cache: 'no-store',
      });
      if (!response.ok) throw new Error(`RPC returned HTTP ${response.status}.`);
      const payload = (await response.json()) as {
        jsonrpc?: unknown;
        id?: unknown;
        result?: unknown;
        error?: unknown;
      };
      if (
        payload.jsonrpc !== '2.0' ||
        payload.id !== 1 ||
        payload.error ||
        typeof payload.result !== 'string' ||
        !/^0x[0-9a-f]+$/i.test(payload.result)
      ) {
        throw new Error('RPC returned an invalid eth_chainId response.');
      }
      const reportedChainId = Number.parseInt(payload.result, 16);
      if (reportedChainId !== network.chainId) {
        throw new Error(`RPC reports chain ${reportedChainId}; expected chain ${network.chainId}.`);
      }
      preferencesRepository.update((current) => {
        if (current.networks.some((saved) => saved.chainId === network.chainId)) {
          throw new Error('This network is already saved.');
        }
        if (current.networks.length >= 32)
          throw new Error('Remove a network before adding another.');
        return { ...current, networks: [...current.networks, network] };
      });
      if (
        !preferencesRepository.read().networks.some((saved) => saved.chainId === network.chainId)
      ) {
        throw new Error('The network could not be saved on this device.');
      }
      announceRpcChanged();
      setInput('');
      setStatus(`${network.name} added. Select it to switch networks.`);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Could not add the network.');
    } finally {
      setAdding(false);
    }
  };

  return (
    <section aria-label="Wallet networks" className="bg-pax-surface rounded-2xl p-4 space-y-3">
      <h3 className="text-sm font-semibold">Networks</h3>
      <ul className="space-y-2">
        {[rootNetwork(), ...preferences.networks].map((network) => (
          <li
            key={network.chainId}
            data-chain-id={network.chainId}
            className="flex items-center gap-2"
          >
            <button
              type="button"
              aria-pressed={active.chainId === network.chainId}
              onClick={() => switchNetwork(network.chainId)}
              className="flex-1 rounded-xl bg-white/[0.06] p-3 text-left text-sm press-scale"
            >
              <span>
                {network.name}
                {active.chainId === network.chainId ? ' · Active' : ''}
              </span>
              <span className="block text-xs text-pax-muted">Chain ID {network.chainId}</span>
            </button>
            {network.kind === 'virtual' && (
              <button
                type="button"
                aria-label={`Remove ${network.name}`}
                onClick={() => removeNetwork(network.chainId)}
                className="p-2 text-xs press-scale"
              >
                Remove
              </button>
            )}
          </li>
        ))}
      </ul>
      <form onSubmit={addNetwork} className="space-y-2">
        <label htmlFor="virtual-chain-id" className="block text-xs text-pax-muted">
          Virtual chain ID or explorer chain URL
        </label>
        <input
          id="virtual-chain-id"
          value={input}
          onChange={(event) => setInput(event.target.value)}
          placeholder="1 or https://…/chains/1"
          required
          maxLength={512}
          className="w-full rounded-xl bg-white/[0.06] px-3 py-2.5 text-sm"
        />
        <button
          type="submit"
          disabled={adding || !input.trim()}
          className="w-full rounded-xl bg-pax-accent py-2.5 text-sm font-semibold text-black press-scale disabled:opacity-30"
        >
          {adding ? 'Checking network…' : 'Add network'}
        </button>
      </form>
      {error && (
        <p role="alert" className="text-xs text-red-400">
          {error}
        </p>
      )}
      {status && (
        <p role="status" className="text-xs text-pax-muted">
          {status}
        </p>
      )}
    </section>
  );
}
