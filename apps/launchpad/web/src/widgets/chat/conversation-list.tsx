'use client';

import { useState, useEffect, useSyncExternalStore } from 'react';
import { useAccount } from 'wagmi';
import Link from 'next/link';
import { getDmConversations, type DmConversation } from '@/core/clients/chat-api';
import { MessageCircle } from 'lucide-react';
import { formatAddress } from '@/utils/format';
import { isSignedOutError } from '@/core/market/unsupported';
import { Button, EmptyState, ErrorState, Skeleton } from '@/ui/primitives';
import { SignedOutState } from '@/ui/shared/signed-out-state';
import { accountEpoch, accountSession, publicProfile, registerAccountInvalidator, subscribeAccount } from '@/widgets/profile/account-transport';

const revocations = new Set<() => void>();
registerAccountInvalidator(revocations, () => { for (const revoke of revocations) revoke(); });
const serverEpoch = () => 0;

function relTime(ts: number | null): string {
  if (!ts) return '';
  const diff = Math.floor(Date.now() / 1000) - ts;
  if (diff < 60) return 'now';
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

interface PeerMeta {
  displayName: string;
}

export default function ConversationList() {
  const { address, isConnected } = useAccount();
  const [conversations, setConversations] = useState<DmConversation[]>([]);
  const [peerMeta, setPeerMeta] = useState<Record<string, PeerMeta>>({});
  const [loading, setLoading] = useState(true);
  const sessionEpoch = useSyncExternalStore(subscribeAccount, accountEpoch, serverEpoch);
  const session = accountSession();
  const sessionMatches = isConnected && !!address && !!session && !session.signal.aborted && Date.parse(session.session.accessExpiresAt) > Date.now() && session.session.wallet.toLowerCase() === address.toLowerCase();
  const [scope, setScope] = useState<number>();
  const [error, setError] = useState<{ message: string; signedOut: boolean } | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const reset = () => { setConversations([]); setPeerMeta({}); setScope(undefined); setError(null); setLoading(false); };
    reset();
    if (!sessionMatches || !session) return;
    setLoading(true);
    const controller = new AbortController();
    const signal = AbortSignal.any([controller.signal, session.signal]);
    const current = () => !signal.aborted && accountSession() === session && accountEpoch() === sessionEpoch && Date.parse(session.session.accessExpiresAt) > Date.now();
    const revoke = () => { controller.abort(); reset(); };
    revocations.add(revoke);

    async function load() {
      try {
        const convos = await getDmConversations(signal);
        if (!current()) return;
        setScope(sessionEpoch); setConversations(convos);
        // Peer display names are public; their association with a conversation is session private.
        for (const peer of convos.map(convo => convo.peer).filter(Boolean)) {
          void publicProfile(peer, signal).then(response => {
            if (current()) setPeerMeta(previous => ({ ...previous, [peer.toLowerCase()]: { displayName: response.data.profile.displayName } }));
          }).catch(() => {});
        }
      } catch (e: unknown) {
        if (current()) {
          if (!isSignedOutError(e)) console.error('[chat] conversations failed to load', e);
          setError({ message: 'Check your connection and try again', signedOut: isSignedOutError(e) });
        }
      } finally {
        if (current()) setLoading(false);
      }
    }

    void load();
    return () => { controller.abort(); revocations.delete(revoke); };
  }, [sessionMatches, session, sessionEpoch, attempt]);

  if (!sessionMatches || error?.signedOut) {
    return <SignedOutState icon={MessageCircle} title="Sign in to see your messages" body="Your direct messages with other traders show up here." />;
  }

  if (loading) {
    return <Skeleton variant="rows" rows={4} label="conversations" className="rounded-md bg-surface-1" />;
  }

  if (error) {
    return <ErrorState title="Can't load messages right now" message={error.message} onRetry={() => setAttempt(value => value + 1)} />;
  }

  if (conversations.length === 0) {
    return (
      <EmptyState
        icon={MessageCircle}
        title="No conversations yet"
        body="Open a trader's profile and send them a message"
        action={<Button variant="secondary" size="sm" href="/leaderboard">Find traders</Button>}
        announce
      />
    );
  }

  return (
    <div className="space-y-1">
      {(scope === sessionEpoch ? conversations : []).map(convo => {
        const meta = peerMeta[convo.peer.toLowerCase()];
        return (
          <Link
            key={convo.id}
            href={`/chat/${encodeURIComponent(convo.id)}`}
            className="flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-surface-2 transition"
          >
            <div className="w-10 h-10 rounded-full bg-surface-2 flex-shrink-0 flex items-center justify-center overflow-hidden">
              <span className="text-xs font-bold text-text-faint">
                {convo.peer.slice(2, 4).toUpperCase()}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-bold text-text truncate">
                {meta?.displayName || formatAddress(convo.peer, 6)}
              </div>
              <div className="text-2xs text-text-faint">
                {relTime(convo.lastMessageAt)}{convo.unreadCount > 0 ? ` · ${convo.unreadCount} unread` : ''}
              </div>
            </div>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-text-faint flex-shrink-0">
              <polyline points="9 18 15 12 9 6"/>
            </svg>
          </Link>
        );
      })}
    </div>
  );
}
