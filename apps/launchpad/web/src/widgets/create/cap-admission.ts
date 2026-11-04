import type { RuntimeAuthority } from '@/core/runtime-authority';
import type { AccountScope, LaunchDeployment } from './creation-adapters';
import type { CreationDraftV2 } from './creation-storage';
import { address } from './creation-domain';
import { readFreshCapability, CapabilityAdmissionError } from '../../../../../../services/launchpad/security/control/cap-admission';

export class KnownCapabilityAdmissionError extends Error {
 constructor(message:string,readonly cause?:unknown){super(message);this.name='KnownCapabilityAdmissionError';}
}
type CreationAuthority=Pick<RuntimeAuthority,'revision'|'expiresAt'|'network'>;
export type CreationAdmissionStage = 'approval' | 'creation';
export type CreationCapabilityAdmission=(draft:CreationDraftV2,scope:AccountScope,stage:CreationAdmissionStage)=>Promise<void>;

// Keep the approval stage in the hashed wire request: the shared runtime's
// older creation request codec does not transport this field.
async function creationPreflight(authority:CreationAuthority,draft:CreationDraftV2,scope:AccountScope,stage:CreationAdmissionStage,head:{hash:`0x${string}`;number:bigint},signal:AbortSignal) {
 const zero='0x0000000000000000000000000000000000000000';
 const request={family:'creation',expectedRevision:authority.revision.revisionSha256,blockHash:head.hash.toLowerCase(),trader:address(scope.creator),recipient:address(scope.creator),pool:zero,amountIn:'',isBuy:false,creationPath:'Router',creationStage:stage,name:draft.name,symbol:draft.symbol,feeStrategy:draft.feeStrategy,optical:zero,nftId:''};
 const body=JSON.stringify(request).replace(/[<>&\u2028\u2029]/g,value=>`\\u${value.charCodeAt(0).toString(16).padStart(4,'0')}`);
 const requestHash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(body)))].map(byte=>byte.toString(16).padStart(2,'0')).join('');
 const response=await fetch('/v2/preflight/create',{method:'POST',credentials:'same-origin',cache:'no-store',redirect:'error',signal,headers:{Accept:'application/json','Content-Type':'application/json'},body});
 if(!response.ok||!response.headers.get('content-type')?.toLowerCase().startsWith('application/json')||!response.body)throw new Error('Creation preflight is unavailable.');
 const reader=response.body.getReader();const chunks:Uint8Array[]=[];let size=0;
 try{for(;;){const next=await reader.read();if(next.done)break;size+=next.value.length;if(size>65536)throw new Error('Creation preflight exceeded its limit.');chunks.push(next.value);}}finally{await reader.cancel();reader.releaseLock();}
 const bytes=new Uint8Array(size);let offset=0;for(const chunk of chunks){bytes.set(chunk,offset);offset+=chunk.length;}
 const proof=JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(bytes)) as Record<string,unknown>;
 const revision=proof.revision as RuntimeAuthority['revision']|undefined;
 if(!revision||Object.entries(authority.revision).some(([key,value])=>revision[key as keyof typeof revision]!==value)||proof.schemaVersion!==2||proof.family!=='creation'||proof.creationStage!==stage||proof.requestSha256!==requestHash||proof.blockHash!==request.blockHash||!Number.isSafeInteger(proof.blockNumber)||BigInt(String(proof.blockNumber))!==head.number||address(String(proof.trader))!==request.trader||address(String(proof.recipient))!==request.recipient)throw new Error('Creation preflight differs from the reviewed context.');
 return proof;
}
export function runtimeCreationAdmission(authority:CreationAuthority,deployment:LaunchDeployment):CreationCapabilityAdmission {
 return async(draft,scope,stage='creation')=>{
  try{
   if(!Number.isFinite(authority.expiresAt)||Date.now()>=authority.expiresAt||authority.revision.revisionSha256!==deployment.revision||scope.chainId!==125||!Number.isSafeInteger(scope.epoch)||scope.epoch<0||address(draft.creator)!==address(scope.creator)||draft.chainId!==125)throw new Error('Committed creation authority expired or the reviewed deployment changed.');
   for(const [role,expected] of [['Router',deployment.router],['SidioraFactory',deployment.factory],['USDL',deployment.usdl],['ProtocolConfig',deployment.protocolConfig],['PoolRegistry',deployment.poolRegistry]] as const)if(address(authority.network.network.addresses[role]!)!==address(expected))throw new Error('Creation authority differs from the reviewed deployment.');
   const native=authority.network.network.addresses.LaunchpadPrecompile;
   if(address(deployment.usdlSpender)!==address(native!))throw new Error('Creation USDL spender differs from the reviewed deployment generation.');
   if(draft.opticalSelection!=='standard')throw new Error('Selected optical schema has not been admitted.');
   const signal=AbortSignal.timeout(15000);
   const capability=await readFreshCapability('pool_creation',signal);
   if(!capability.allowed||capability.stale)throw new CapabilityAdmissionError(capability);
   const head=await authority.network.committedHead(signal);
   const proof=await creationPreflight(authority,draft,scope,stage,head,signal);
   const cost=proof.creationCost as Record<string,unknown>|undefined;
   if(proof.supported!==true||proof.executionPermitted!==true||proof.simulationMode!==(stage==='approval'?'usdl_approval':'create_market')||!cost||cost.protocolFeeAsset!=='USDL'||cost.protocolFeeRaw!=='1000000'||cost.gasAsset!=='PAX'||cost.gasEstimateAvailable!==true||cost.blockHash!==head.hash||address(String(cost.spender))!==address(deployment.usdlSpender))throw new Error('Creation is unavailable under the committed deployment capability.');
   if(Date.now()>=authority.expiresAt||Date.now()-Date.parse(capability.observedAt)>60000)throw new Error('Creation authority expired before admission.');
  }catch(error){if(error instanceof KnownCapabilityAdmissionError)throw error;throw new KnownCapabilityAdmissionError(error instanceof Error?error.message:'Creation authority is unavailable.',error);}
 };
}
