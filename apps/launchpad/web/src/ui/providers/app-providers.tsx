'use client';

// Installs the postMessage EIP-1193 bridge when this app runs inside the
// PaxPort wallet iframe; it must evaluate before the wagmi config so the
// injected connector sees the bridged provider as window.ethereum.
import '@/core/iframe-bridge-init';
import { useMemo, type ReactNode } from 'react';
import { cookieStorage, createConfig, createStorage, http, WagmiProvider, type State } from 'wagmi';
import { injected } from 'wagmi/connectors';
import QueryProvider, { PrincipalCacheBarrier } from './query-provider';
import Web3Provider from './web3-provider';
import ToastProvider from '@/ui/shared/toast-provider';
import { WalletRuntimeProvider } from '@/core/wallet-sdk/runtime';
import { resolvePrivateFixtureAuthorityProfileV1, RuntimeAuthorityProvider, useRuntimeAuthority, type PrivateFixtureAuthorityProfileV1 } from '@/core/runtime-authority';
import { TradeExecutionProvider } from '@/widgets/trading/transaction-controls/providers';
import { paxeer } from '@/core/wagmi-config';
import { PRIVATE_FIXTURE_WAGMI_STORAGE_KEY } from '@/core/network/deployment';

export function createPrivateFixtureWagmiStorage() {
  return createStorage({ key: PRIVATE_FIXTURE_WAGMI_STORAGE_KEY, storage: cookieStorage });
}

export function createPrivateFixtureWagmiConfig(profile: PrivateFixtureAuthorityProfileV1) {
  return createConfig({
    chains: [{ ...paxeer, rpcUrls: { default: { http: [profile.publicRPC] } }, blockExplorers: { default: { name: 'Paxscan', url: profile.explorerBase } } }],
    connectors: [injected()],
    storage: createPrivateFixtureWagmiStorage(),
    transports: { 125: http(profile.publicRPC, { batch: { batchSize: 1024, wait: 16 }, retryCount: 0 }) },
    ssr: true,
  });
}

function PrivateFixtureWeb3Provider({ profile, initialState, children }: { profile: PrivateFixtureAuthorityProfileV1; initialState?: State; children: ReactNode }) {
  const config = useMemo(() => createPrivateFixtureWagmiConfig(profile), [profile]);
  return <WagmiProvider config={config} initialState={initialState}>{children}</WagmiProvider>;
}

function RuntimeProviders({ children }: { children: ReactNode }) {
  const { authority } = useRuntimeAuthority();

  return (
    <WalletRuntimeProvider authority={authority?.network}>
      <TradeExecutionProvider deployment={authority?.tradeDeployment}>
        <PrincipalCacheBarrier>
          <ToastProvider />
          {children}
        </PrincipalCacheBarrier>
      </TradeExecutionProvider>
    </WalletRuntimeProvider>
  );
}

export default function AppProviders({
  children,
  wagmiInitialState,
  privateFixtureProfile,
  privateFixtureProfileInput,
}: {
  children: ReactNode;
  wagmiInitialState?: State;
  privateFixtureProfile?: PrivateFixtureAuthorityProfileV1;
  privateFixtureProfileInput?: string;
}) {
  const configured = useMemo(() => {
    try {
      const profile = resolvePrivateFixtureAuthorityProfileV1({ privateFixtureProfile, privateFixtureProfileInput });
      if (profile === undefined) return {};
      if (typeof window !== 'undefined' && window.location.origin !== profile.applicationOrigin) throw new Error('Approved local network origin is unavailable');
      return { profile };
    } catch (error) { return { error: error instanceof Error ? error.message : 'Approved network input is unavailable' }; }
  }, [privateFixtureProfile, privateFixtureProfileInput]);
  if (configured.error) {
    console.error('[app-providers]', configured.error);
    return <p role="status" className="p-4 text-sm text-white/60">Something went wrong, reload the page</p>;
  }
  const runtime = (
    <RuntimeAuthorityProvider key={configured.profile ? `${configured.profile.revision.revisionSha256}:${configured.profile.networkSha256}` : 'production'} privateFixtureProfile={configured.profile}>
      <RuntimeProviders>{children}</RuntimeProviders>
    </RuntimeAuthorityProvider>
  );
  return (
    <QueryProvider>
      {configured.profile ? (
        <PrivateFixtureWeb3Provider profile={configured.profile} initialState={wagmiInitialState}>{runtime}</PrivateFixtureWeb3Provider>
      ) : (
        <Web3Provider initialState={wagmiInitialState}>{runtime}</Web3Provider>
      )}
    </QueryProvider>
  );
}
