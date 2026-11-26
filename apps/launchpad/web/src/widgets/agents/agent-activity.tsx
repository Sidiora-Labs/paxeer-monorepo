'use client';

import { useEffect, useState } from 'react';
import { formatEther } from 'viem';
import { Activity } from 'lucide-react';
import { Button, EmptyState, ErrorState, Panel, Skeleton } from '@/ui/primitives';
import type { AgentActivity, PaxeerWallet, SigningSession } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk';
import { agentError } from './agents-domain';

export function AgentActivityPanel({ wallet, expected, did, refresh }: { wallet: PaxeerWallet; expected: SigningSession; did: string; refresh: number }) {
  const [events, setEvents] = useState<AgentActivity[] | null>(null);
  const [error, setError] = useState('');
  const [tick, setTick] = useState(0);
  useEffect(() => {
    let active = true;
    setEvents(null); setError('');
    void wallet.agentActivity(did, expected).then(value => {
      if (active) setEvents(value.events);
    }).catch(cause => { if (active) setError(agentError(cause)); });
    return () => { active = false; };
  }, [wallet, expected, did, refresh, tick]);
  return <Panel aria-labelledby="agent-activity-title" className="flex flex-col gap-3">
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 id="agent-activity-title" className="text-md font-semibold">Activity</h3><Button onClick={() => setTick(value => value + 1)}>Refresh activity</Button></div>
    <p className="text-xs text-text-muted">Latest 100 signing events recorded by the gateway. Amounts are native PAX; token amounts and refused requests are not included in this feed. A transaction hash means submitted, not confirmed.</p>
    {!events && !error ? <Skeleton variant="rows" rows={3} label="agent activity" /> : null}
    {error ? <ErrorState message={error} onRetry={() => setTick(value => value + 1)} /> : null}
    {events?.length === 0 ? <EmptyState icon={Activity} title="No activity yet" body="Signing events will appear here when the agent acts." /> : null}
    {events?.map((event, index) => <div key={`${event.created_at}:${index}`} className="sl-hairline grid min-w-0 gap-1 py-3 text-sm sm:grid-cols-4">
      <time dateTime={event.created_at} className="tabular-nums text-text-muted">{new Date(event.created_at).toLocaleString()}</time>
      <span className="break-words">{event.kind}</span>
      <span className="tabular-nums sm:text-right">{formatEther(BigInt(event.value_wei))} PAX</span>
      <div className="min-w-0 sm:text-right"><span>{event.tx_hash ? 'Submitted' : 'Signed'}</span>{event.tx_hash ? <p className="break-all text-xs text-text-muted">{event.tx_hash}</p> : null}</div>
    </div>)}
  </Panel>;
}
