import { createContext, createElement, useContext, useSyncExternalStore, type ReactNode } from 'react';
import { useAccount } from 'wagmi';

const serverAccount = {
  address: undefined,
  addresses: undefined,
  chain: undefined,
  chainId: undefined,
  connector: undefined,
  isConnected: false,
  isConnecting: false,
  isDisconnected: true,
  isReconnecting: false,
  status: 'disconnected',
} as const satisfies ReturnType<typeof useAccount>;

const subscribe = () => () => {};
const clientSnapshot = () => true;
const serverSnapshot = () => false;
const WalletHydrationContext = createContext(true);

/** The application boundary keeps server markup stable while the wallet reconnects. */
export function WalletHydrationProvider({ children }: { children: ReactNode }) {
  const hydrated = useSyncExternalStore(subscribe, clientSnapshot, serverSnapshot);
  return createElement(WalletHydrationContext.Provider, { value: hydrated }, children);
}

/** The first client render shares the server's unconnected wallet snapshot. */
export function useHydratedAccount() {
  const account = useAccount();
  const hydrated = useContext(WalletHydrationContext);
  return hydrated ? account : serverAccount;
}
