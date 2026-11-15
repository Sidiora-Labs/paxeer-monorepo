'use client';
import { useState, useEffect, useRef } from 'react';
import { useAccount } from 'wagmi';
import { useDmChat } from '@/hooks/chat/use-chat';
import { MessageCircle } from 'lucide-react';
import { formatAddress } from '@/utils/format';
import { Button, EmptyState } from '@/ui/primitives';
export default function DmThread({ conversationId, peerAddress, peerName }: { conversationId: string; peerAddress: string; peerName?: string }) {
 const { address } = useAccount();
 const chat = useDmChat(conversationId,peerAddress);
 const [input,setInput]=useState('');
 const [offset,setOffset]=useState(0);
 const [editing,setEditing]=useState<{id:string;content:string}>();
 const wallet=useRef(address);
 const editor=useRef<HTMLInputElement>(null);
 useEffect(()=>{setInput('');setOffset(0);setEditing(undefined);wallet.current=address;},[address,conversationId]);
 const markRead=chat.markRead;
 const current=wallet.current===address;
 const visible=current ? chat.messages.slice(Math.max(0,chat.messages.length-200-offset),Math.max(0,chat.messages.length-offset)) : [];
 const cancelMarkRead=chat.cancelMarkRead;
 const through=visible.reduce((highest,message)=>message.sender.toLowerCase()!==address?.toLowerCase()&&!message.deleted&&BigInt(message.sequence)>BigInt(highest)?message.sequence:highest,'0');
 const firstVisible=visible.at(0)?.id;
 const lastVisible=visible.at(-1)?.id;
 useEffect(()=>{
  const update=()=>{cancelMarkRead();if(document.visibilityState==='visible'&&through!=='0')markRead(through);};
  update();
  document.addEventListener('visibilitychange',update);
  return()=>{document.removeEventListener('visibilitychange',update);cancelMarkRead();};
 },[through,firstVisible,lastVisible,offset,markRead,cancelMarkRead]);
 const submit=async()=>{const draft=input;if(await chat.sendMessage(draft))setInput(value=>value===draft?'':value);};
 return <div className="flex min-h-0 flex-1 flex-col">
  <header className="flex min-h-control-height-md items-center border-b border-surface-3 bg-surface-1 px-4 py-3"><a href={`/profile/${peerAddress}`} className="font-bold text-text underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-focus">{peerName||formatAddress(peerAddress,6)}</a></header>
  <div role="status" aria-live="polite" className="px-4 py-2 text-2xs text-text-faint">{chat.error||(!chat.isAuthenticated?'Sign in to send messages':'')}</div>
  {chat.wsStatus==='authorization-required'&&<div className="px-4"><Button type="button" variant="secondary" size="sm" onClick={()=>chat.connectChat()}>Try again</Button></div>}
  <div role="log" aria-label="Messages" aria-live="polite" tabIndex={0} className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
   {chat.loading?<p role="status" className="text-sm text-text-faint">Loading...</p>:!chat.error&&chat.messages.length===0?<EmptyState icon={MessageCircle} title="No messages yet" body="Say hi to start the chat" announce/>:null}
   {chat.messages.length>200+offset&&<Button type="button" variant="ghost" size="sm" onClick={()=>setOffset(value=>value+200)}>Earlier messages</Button>}
   {visible.map(message=><article key={message.id} data-message-id={message.id} className={`my-3 max-w-[min(42rem,100%)] rounded-lg p-3 ${message.sender.toLowerCase()===address?.toLowerCase()?'ml-auto bg-accent-soft':'bg-surface-2'}`}>
    {editing?.id===message.id?<div className="flex flex-col gap-2"><label className="sr-only" htmlFor={`edit-${message.id}`}>Edit message</label><textarea id={`edit-${message.id}`} autoComplete="off" value={editing.content} maxLength={2000} onChange={event=>setEditing({id:message.id,content:event.target.value})} className="min-h-20 w-full rounded-sm bg-surface-1 p-2 text-sm text-text focus-visible:outline-2 focus-visible:outline-focus"/><div className="flex flex-wrap gap-2"><Button type="button" size="sm" onClick={async()=>{if(await chat.reviseMessage(message,editing.content))setEditing(undefined);}}>Save</Button><Button type="button" variant="ghost" size="sm" onClick={()=>setEditing(undefined)}>Cancel</Button></div></div>:<p className="whitespace-pre-wrap break-words text-sm text-text">{message.deleted?'This message was removed':message.content}</p>}
    {!message.deleted&&message.sequence!=='0'&&message.sender.toLowerCase()===address?.toLowerCase()&&<div className="mt-2 flex flex-wrap gap-2">{Date.now()/1000-message.createdAt<=900&&<Button type="button" variant="ghost" size="sm" onClick={()=>setEditing({id:message.id,content:message.content})}>Edit</Button>}<Button type="button" variant="ghost" size="sm" onClick={()=>void chat.reviseMessage(message,'',true)}>Delete</Button></div>}
    {message.sendState==='submitting'&&<span role="status" className="text-2xs text-text-faint">Sending…</span>}
    {message.sendState==='persisted'&&<span className="text-2xs text-text-faint">Sent</span>}
    {(message.sendState==='rejected'||message.sendState==='uncertain')&&<div role="status" className="mt-2 text-2xs text-danger">{message.sendState==='uncertain'?'Not sure this sent':'Couldn\'t send'} <Button type="button" variant="ghost" size="sm" onClick={()=>void chat.retryMessage(message.clientMessageId!)}>Try again</Button></div>}
   </article>)}
   {offset>0&&<Button type="button" variant="ghost" size="sm" onClick={()=>setOffset(value=>Math.max(0,value-200))}>Later messages</Button>}
  </div>
  <form className="flex gap-2 border-t border-surface-3 bg-surface-1 p-3" onSubmit={event=>{event.preventDefault();void submit();}}>
   <label className="sr-only" htmlFor="dm-composer">Message</label>
   <input id="dm-composer" name="message" autoComplete="off" ref={editor} value={current?input:''} onChange={event=>setInput(event.target.value)} disabled={!chat.isAuthenticated} maxLength={2000} className="min-w-0 flex-1 rounded-pill bg-surface-2 px-4 py-2 text-sm text-text outline-none focus-visible:outline-2 focus-visible:outline-focus"/>
   <Button type="submit" size="md" disabled={!chat.isAuthenticated||!input.trim()}>Send</Button>
  </form>
 </div>;
}
