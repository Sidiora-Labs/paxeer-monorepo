'use client';
import { displayCoinText } from '@/utils/coin-text';
import { Coins } from 'lucide-react';
import { EmptyState, Skeleton } from '@/ui/primitives';
import {useMemo,useState,type ReactNode} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {useMetadataV2,useMetadataMedia} from '@/hooks/market/use-token-metadata';
import type {MetadataV2Data,MetadataMediaImage} from '@/core/clients/metadata';
import {MetadataImageEditor} from './editor';
import {createMetadataImageTransport,type CreatorImageAuthority} from './transport';

export function selectMetadataImage(image:MetadataMediaImage|null|undefined,safeImage:boolean,failedAddress:string|null|undefined):MetadataMediaImage|null{
 return safeImage&&image&&image.variants['512']!==failedAddress?image:null;
}

export function MetadataView({tokenAddress,tradePanel,data,pending,error,image}:{tokenAddress:string;tradePanel:ReactNode;data:MetadataV2Data|null|undefined;pending:boolean;error:boolean;image?:MetadataMediaImage|null}){
 const [failedAddress,setFailedAddress]=useState<string|null>(null);
 const safeImage=!error&&!!image&&image.assetId===data?.token.logoAssetId;
 const displayedImage=selectMetadataImage(image,safeImage,failedAddress);
 const safeLink=(value:string)=>{try{const url=new URL(value);return url.protocol==='https:'&&!url.username&&!url.password}catch{return false}};
 return <section aria-label="Coin details" className="flex min-w-0 flex-col gap-3 text-sm text-text-muted">
  <div aria-label="Trading" className="order-first min-w-0">{tradePanel}</div>
  <div className="flex min-w-0 flex-col gap-2 rounded-md bg-surface-1 p-3">
   <code className="break-all font-mono text-xs text-text-faint">{tokenAddress || <span aria-label="Loading coin address">…</span>}</code>
   {pending ? <Skeleton variant="rows" rows={3} label="coin details" /> : null}
   {error ? <p role="alert" className="text-xs text-text-muted">Can't load coin details right now, you can still trade</p> : null}
   {!pending && !error && !data ? <EmptyState variant="inline" icon={Coins} title="No coin details yet" body="Coin information appears here when the creator adds it." /> : null}
   {data ? <>
    <h2 className="min-w-0 truncate text-md font-semibold text-text" title={displayCoinText(data.displayName??data.token.name,tokenAddress)}>{displayCoinText(data.displayName??data.token.name,tokenAddress)}</h2>
    <p className="[overflow-wrap:anywhere] text-xs text-text-faint">Onchain name: {displayCoinText(data.token.name,tokenAddress)}</p>
    <p className="break-words">{data.description}</p>
    {data.socialLinks.filter(link=>safeLink(link.url)).map(link=><a key={link.url} href={link.url} rel="noopener noreferrer" target="_blank" className="self-start rounded-sm text-xs text-text-muted underline underline-offset-2">{link.label}</a>)}
    {displayedImage ? <img src={displayedImage.variants['512']} srcSet={`${displayedImage.variants['256']} 256w, ${displayedImage.variants['512']} 512w, ${displayedImage.variants['1024']} 1024w`} sizes="(max-width: 512px) 100vw, 512px" width={displayedImage.width} height={displayedImage.height} className="max-w-full rounded-md" style={{maxWidth:'100%',height:'auto'}} alt="Coin image" onError={()=>setFailedAddress(displayedImage.variants['512'])} /> : null}
   </> : null}
  </div>
 </section>

}
export function MetadataViewer({tokenAddress,tradePanel}:{tokenAddress:string;tradePanel:ReactNode}){
 const query=useMetadataV2(tokenAddress),media=useMetadataMedia(tokenAddress);
 const image=media.data&&query.data&&media.data.metadata.revision===query.data.revision?media.data.image:null;
 return <><MetadataView tokenAddress={tokenAddress} tradePanel={tradePanel} data={query.data} pending={query.isPending} error={query.isError} image={image}/>{media.isError&&query.data?.token.logoAssetId?<p role="alert">Can't load the coin image right now</p>:null}</>;
}
export function CreatorMetadataMedia({authority}:{authority:CreatorImageAuthority}){
 const [revision,setRevision]=useState(authority.revision);const cache=useQueryClient();
 const upload=useMemo(()=>createMetadataImageTransport({...authority,revision}),[authority,revision]);
 return <MetadataImageEditor upload={upload} activeRevision={revision} onApproved={result=>{if(result.revision){setRevision(result.revision);void cache.invalidateQueries({queryKey:['metadata-v2',125,authority.tokenAddress.toLowerCase()]});void cache.invalidateQueries({queryKey:['metadata-media-v2',125,authority.tokenAddress.toLowerCase()]})}}}/>;
}
