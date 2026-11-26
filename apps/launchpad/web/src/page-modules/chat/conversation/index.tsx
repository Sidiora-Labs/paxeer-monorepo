'use client';
import { useParams } from 'next/navigation';
import { useEffect, useState, useSyncExternalStore } from 'react';
import { useAccount } from 'wagmi';
import Link from 'next/link';
import { MessageCircle, SearchX } from 'lucide-react';
import DmThread from '@/widgets/chat/dm-thread';
import { getDmConversations, type DmConversation } from '@/core/clients/chat-api';
import { isSignedOutError } from '@/core/market/unsupported';
import { Button, EmptyState, ErrorState, Skeleton } from '@/ui/primitives';
import { SignedOutState } from '@/ui/shared/signed-out-state';

import { accountEpoch, accountSession, subscribeAccount } from '@/widgets/profile/account-transport';

const serverEpoch = () => 0;
type Failure = { kind: 'signed-out' } | { kind: 'not-found' } | { kind: 'error'; message: string };

export default function ConversationModule(){
 const params=useParams();const {address}=useAccount();
 const sessionEpoch=useSyncExternalStore(subscribeAccount,accountEpoch,serverEpoch);
 const session=accountSession();
 const active=!!address&&!!session&&!session.signal.aborted&&session.session.wallet.toLowerCase()===address.toLowerCase();
 const id=typeof params.conversationId==='string'?params.conversationId:'';
 const [value,setValue]=useState<{wallet:string;epoch:number;conversation:DmConversation}>();const [failure,setFailure]=useState<Failure>();const [attempt,setAttempt]=useState(0);
 useEffect(()=>{const controller=new AbortController();setValue(undefined);setFailure(undefined);const signal=session?AbortSignal.any([controller.signal,session.signal]):controller.signal;if(active)void getDmConversations(signal).then(rows=>{if(signal.aborted||accountSession()!==session)return;const conversation=rows.find(row=>row.id===id);if(!conversation){setFailure({kind:'not-found'});return;}setValue({wallet:address!,epoch:sessionEpoch,conversation});}).catch(cause=>{if(signal.aborted||accountSession()!==session)return;setFailure(isSignedOutError(cause)?{kind:'signed-out'}:{kind:'error',message:'Check your connection and try again'});});return()=>controller.abort();},[address,id,attempt,session,sessionEpoch,active]);
 const body=!active||failure?.kind==='signed-out'
  ?<SignedOutState icon={MessageCircle} title="Sign in to read this conversation" body="Direct messages are only visible to the people in them." className="chat-state px-8"/>
  :failure?.kind==='not-found'
   ?<EmptyState icon={SearchX} title="Conversation not found" body="It may have been deleted, or it belongs to another wallet" action={<Button variant="secondary" size="sm" href="/chat">Back to messages</Button>}/>
   :failure?.kind==='error'
    ?<ErrorState title="Can't load this chat right now" message={failure.message} onRetry={()=>setAttempt(value=>value+1)}/>
    :value&&value.wallet===address&&value.epoch===sessionEpoch?<DmThread key={`${address}:${id}:${sessionEpoch}`} conversationId={id} peerAddress={value.conversation.peer}/>:<Skeleton variant="rows" rows={3} label="conversation" />;
 return <div className="mx-auto flex min-h-[calc(100vh-var(--sl-space)*15)] max-w-2xl flex-col text-text"><Link prefetch={false} href="/chat" className="inline-flex items-center text-md text-text-muted underline-offset-2 hover:text-text hover:underline focus-visible:outline-2 focus-visible:outline-focus">Back to messages</Link>{body}</div>;
}
