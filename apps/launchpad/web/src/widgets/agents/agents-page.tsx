'use client';

import type { ReactNode } from 'react';
import { useAccount, usePublicClient } from 'wagmi';
import { Bot, TriangleAlert } from 'lucide-react';
import { usePaxeerOptional, PAXEER_CONNECTOR_ID } from '@/core/wallet-sdk/embedded_sdk/embedded';
import { openAuthModal } from '@/shell/auth-modal-store';
import { PageWithFooter } from '@/shell/platform-metrics';
import { Button, EmptyState, ErrorState, Panel, Skeleton } from '@/ui/primitives';
import { AgentsWorkspace } from './agents-workspace';
import type { AgentConnection } from './agent-connection';

export type AgentsAccessState = 'loading' | 'unconfigured' | 'signed-out' | 'external-wallet' | 'wrong-chain' | 'wallet-error' | 'ready';
export function AgentsAccess({ state, onSignIn, onRetry, children }: { state: AgentsAccessState; onSignIn: () => void; onRetry: () => void; children?: ReactNode }) {
  return <div data-agents-route className="mx-auto flex w-full min-w-0 max-w-5xl flex-col gap-4 p-4 sm:p-6">
    <header className="flex flex-col gap-2">
      <h1 className="text-lg font-bold text-text">Agents</h1>
      <p className="max-w-2xl text-sm text-text-muted">An agent wallet is a separate wallet for software that acts for you. Fund it from your embedded wallet, set its limits and freeze it whenever you need.</p>
    </header>
    {state === 'loading' ? <Skeleton variant="rows" rows={4} label="agent wallets" /> : null}
    {state === 'unconfigured' ? <Panel><EmptyState icon={Bot} title="Agent gateway unavailable" body="The embedded wallet gateway is not configured here. Agent management will be available when it is connected." /></Panel> : null}
    {state === 'signed-out' ? <Panel><EmptyState icon={Bot} title="Sign in to manage agents" body="Use the embedded wallet sign-in to create or claim an agent and manage its funds." /><Button variant="inverse" onClick={onSignIn}>Sign in with embedded wallet</Button></Panel> : null}
    {state === 'external-wallet' ? <Panel><EmptyState icon={Bot} title="Agents need the embedded wallet" body="Your connected wallet is a browser or external wallet. Switch to the embedded wallet through sign-in to manage agents." /><Button variant="inverse" onClick={onSignIn}>Open embedded wallet sign-in</Button></Panel> : null}
    {state === 'wrong-chain' ? <ErrorState title="Switch to Paxeer" message="Agent wallets use Paxeer chain 125. Switch your connected embedded wallet to Paxeer." onRetry={onRetry} /> : null}
    {state === 'wallet-error' ? <Panel role="alert"><EmptyState icon={TriangleAlert} title="Embedded wallet unavailable" body="Your embedded wallet could not be loaded or does not match the connected account. Refresh it to continue." action={<Button onClick={onRetry}>Try again</Button>} /></Panel> : null}
    {state === 'ready' ? children : null}
  </div>;
}

export default function AgentsPage({ connection }: { connection: AgentConnection }) {
  const embedded = usePaxeerOptional();
  const account = useAccount();
  const reader = usePublicClient({ chainId: 125 });
  let state: AgentsAccessState;
  if (embedded?.isLoading || account.status === 'reconnecting' || account.status === 'connecting') state = 'loading';
  else if (!embedded?.wallet) state = 'unconfigured';
  else if (account.isConnected && account.connector?.id !== PAXEER_CONNECTOR_ID) state = 'external-wallet';
  else if (!embedded.session || !account.isConnected) state = 'signed-out';
  else if (account.chainId !== 125) state = 'wrong-chain';
  else if (!embedded.publicWallet || !account.address || !reader || embedded.publicWallet.address.toLowerCase() !== account.address.toLowerCase()) state = 'wallet-error';
  else state = 'ready';
  return <PageWithFooter><AgentsAccess state={state} onSignIn={() => openAuthModal('Sign in with your embedded wallet to manage agents')} onRetry={() => embedded?.refresh()}>
    {state === 'ready' && embedded?.wallet && embedded.session && account.address && reader ? <AgentsWorkspace
      key={`${embedded.session.user.id}:${embedded.session.access_token}:${account.address}`}
      wallet={embedded.wallet} owner={account.address} userId={embedded.session.user.id} accessToken={embedded.session.access_token} reader={reader}
      connection={connection}
    /> : null}
  </AgentsAccess></PageWithFooter>;
}
