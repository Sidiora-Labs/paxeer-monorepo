export interface AuthCallbackState {
  configured: boolean;
  authenticated: boolean;
  connected: boolean;
  connectorId?: string;
  address?: string;
  walletAddress?: string;
  authError: string | null;
  connectionError: string | null;
  timedOut: boolean;
}

export function authCallbackState(state: AuthCallbackState, embeddedConnectorId: string): 'waiting' | 'ready' | 'failed' {
  if (!state.configured || state.authError || state.connectionError) return 'failed';
  if (state.authenticated && state.connected) {
    return state.connectorId === embeddedConnectorId && !!state.address && state.address.toLowerCase() === state.walletAddress?.toLowerCase() ? 'ready' : 'failed';
  }
  return state.timedOut ? 'failed' : 'waiting';
}
