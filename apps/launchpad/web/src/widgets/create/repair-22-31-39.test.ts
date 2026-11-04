import { createHash } from 'node:crypto';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { expect, test } from 'vitest';
import {
 createPublicClient, createWalletClient, decodeFunctionData, encodeAbiParameters,
 encodeEventTopics, encodeFunctionData, encodeFunctionResult, http, keccak256,
 parseAbi, stringToHex, toHex, zeroAddress,
 type Address, type Hex, type PublicClient, type WalletClient,
} from 'viem';
import type { RuntimeAuthority } from '@/core/runtime-authority';
import { runtimeCreationAdmission, type CreationAdmissionStage } from './cap-admission';
import { ChainCreationAdapter, creationFunding, type LaunchDeployment } from './creation-adapters';
import { CreationController } from './creation-controller';
import { address, funding, verifyCost, verifyCreationApproval, type ApprovalContext } from './creation-domain';
import { checkedAttempt, validateCreationDraft, type CreationDraftV2, type CreationStorage, type LaunchAttempt } from './creation-storage';

export const repair223139Record = {
 findings: {
  'native-client-spender': 'nativeClientSpenderCreationAndFirstBuy',
  'native-domain-spender': 'nativeDomainSpenderProofRejectsHostileContexts',
  'approval-admission-cycle': 'approvalAdmissionZeroAllowanceController',
 },
 baseline: 'No unchanged-tree verify was run: the bounded implementation-first gate order prohibits a preliminary verify.',
 wiring: [
  'modules/launchpad/keeper/custody.go — native launchpad domain: qualify zero-allowance creation and first purchase on paxd with this native-spender client; the Node gate exercises the provider/codec/controller boundary using an in-memory ledger peer, not paxd.',
  'apps/launchpad/web/src/core/runtime-authority.ts — web runtime authority domain: transport creationStage in RuntimePreflightRequest when consolidating the scoped, request-hashed creation transport.',
  'sdk/launchpad/shared/config/capabilities/preflight.go — shared configuration domain: include the new approval stage in the owning Go qualification; the declared Node gate does not compile or execute Go.',
 ],
} as const;

const a=(n:number):Address=>`0x${n.toString(16).padStart(40,'0')}`;
const h=(n:number):Hex=>`0x${n.toString(16).padStart(64,'0')}`;
const owner=a(90),native=a(0x1017),pool=a(91),token=a(92),treasury=a(93);
const chain={id:125,name:'Creation wire fixture',nativeCurrency:{name:'PAX',symbol:'PAX',decimals:18},rpcUrls:{default:{http:['http://127.0.0.1']}}} as const;
const abi=parseAbi([
 'function decimals() view returns (uint8)',
 'function balanceOf(address) view returns (uint256)',
 'function allowance(address,address) view returns (uint256)',
 'function approve(address,uint256) returns (bool)',
 'function transfer(address,uint256) returns (bool)',
 'function createMarket(string name,string symbol,uint8 feeStrategy,address optical) returns (address tokenAddr,address poolAddr,uint256 nftId)',
 'function buy(address pool,uint256 usdlAmountIn,uint256 minTokensOut,uint256 deadline) returns (uint256)',
 'function factory() view returns (address)',
 'function poolRegistry() view returns (address)',
 'function protocolConfig() view returns (address)',
 'function usdlAddress() view returns (address)',
 'function treasury() view returns (address)',
 'function creationFee() view returns (uint256)',
 'function getPoolMetadata(address pool) view returns ((address creator,address token,address optical,uint256 nftId,uint256 createdAt,uint256 createdBlock))',
 'function getPoolsByCreatorPage(address creator,uint256 offset,uint256 limit) view returns (address[])',
 'function isRegisteredPool(address pool) view returns (bool)',
 'function getPoolByToken(address token) view returns (address)',
 'function tokenAddress() view returns (address)',
 'function opticalAddress() view returns (address)',
 'function quoteExactInput(address pool,uint256 amountIn,bool isBuy) view returns ((uint256 amountOut,uint256 feeAmount,uint256 priceImpactBps))',
 'event MarketCreated(address indexed token,address indexed pool,address indexed creator,uint256 nftId,address optical)',
 'event Swap(bytes32 indexed poolId,address indexed sender,bool isBuy,uint256 amountIn,uint256 amountOut,uint256 fee,uint256 price,uint256 timestamp,uint256 blockNumber)',
]);
const revision:RuntimeAuthority['revision']={schemaVersion:2,deploymentVersion:'native-wire-fixture',manifestSha256:'1'.repeat(64),capabilitySha256:'2'.repeat(64),contractSupportSha256:'3'.repeat(64),serialInterfaceSha256:'4'.repeat(64),revisionSha256:'5'.repeat(64)};
const code='0x60006000' as const;
const trade={revision:revision.revisionSha256,router:a(1),quoter:a(2),registry:a(3),emitter:a(4),usdl:a(5),pins:[a(1),a(2),a(3),a(4)].map(value=>({address:value,runtimeHash:keccak256(code)}))};
const deployment:LaunchDeployment={revision:trade.revision,chainId:125,router:trade.router,usdl:trade.usdl,usdlSpender:native,factory:a(6),protocolConfig:a(7),poolRegistry:trade.registry,quoter:trade.quoter,opticalRegistry:a(8),tradeDeployment:trade};
function exactTopics(values:ReturnType<typeof encodeEventTopics>):Hex[]{return values.map(value=>{if(typeof value!=='string')throw new Error('Event topic must be concrete');return value;});}
function draft():CreationDraftV2 {return {draftId:crypto.randomUUID(),version:2,creator:owner,chainId:125,name:'Native creation',symbol:'NATIVE',description:'',tags:[],socialLinks:[],uploadRefs:[],firstBuyRaw:'2000000',opticalSelection:'standard',updatedAt:Date.now(),feeStrategy:0};}

// A committed in-memory peer implements the USDL ledger and JSON-RPC boundary.
// The component under repair is always the production adapter/controller; all
// requests and receipt calldata pass through real viem encoders and decoders.
class QuoteLedger {
 balances=new Map<string,bigint>([[owner,20_000_000n],[treasury,0n]]);
 allowances=new Map<string,bigint>();
 credits=0n;
 allowance(account:string,spender:string){return this.allowances.get(`${address(account)}:${address(spender)}`)??0n;}
 approve(account:string,spender:string,amount:bigint){this.allowances.set(`${address(account)}:${address(spender)}`,amount);}
 pull(account:string,amount:bigint,commit:boolean){
  const allowed=this.allowance(account,native),balance=this.balances.get(address(account))??0n;
  if(allowed<amount||balance<amount)throw new Error('USDL transferFrom: insufficient native spender allowance or balance');
  if(commit){this.approve(account,native,allowed-amount);this.balances.set(address(account),balance-amount);this.credits+=amount;}
 }
}
interface WireTransaction {from:Address;to:Address;data?:Hex;input?:Hex;value?:Hex}
type WireRecord=Record<string,unknown>;
class CreationWirePeer {
 readonly ledger=new QuoteLedger();
 readonly transactions=new Map<Hex,WireRecord>();
 readonly receipts=new Map<Hex,WireRecord>();
 readonly stages:{stage:CreationAdmissionStage;height:bigint;allowance:bigint}[]=[];
 readonly allowanceReads:string[]=[];
 height=1n;
 registered=false;
 allowed=true;
 stale=false;
 fee=1_000_000n;
 readonly blocks=new Map<bigint,WireRecord>();
 constructor(){this.blocks.set(1n,this.block(1n));}
 block(number:bigint):WireRecord{return {number:toHex(number),hash:h(Number(number)),parentHash:h(Number(number-1n)),timestamp:toHex(Math.floor(Date.now()/1000)),nonce:'0x0000000000000000',sha3Uncles:h(0),logsBloom:`0x${'0'.repeat(512)}`,transactionsRoot:h(10),stateRoot:h(11),receiptsRoot:h(12),miner:a(100),difficulty:'0x0',totalDifficulty:'0x0',extraData:'0x',size:'0x100',gasLimit:'0x1c9c380',gasUsed:'0x186a0',baseFeePerGas:'0x1',transactions:[],uncles:[]};}
 execute(tx:WireTransaction,commit=false):Hex {
  const input=tx.data??tx.input;if(!input)throw new Error('Missing transaction input');
  const decoded=decodeFunctionData({abi,data:input});
  const args=decoded.args??[];
  let result:unknown;
  switch(decoded.functionName){
   case 'decimals':result=6;break;
   case 'balanceOf':result=this.ledger.balances.get(address(String(args[0])))??0n;break;
   case 'allowance':this.allowanceReads.push(address(String(args[1])));result=this.ledger.allowance(String(args[0]),String(args[1]));break;
   case 'approve':if(address(tx.to)!==deployment.usdl)throw new Error('Wrong USDL destination');if(commit)this.ledger.approve(tx.from,String(args[0]),BigInt(String(args[1])));result=true;break;
   case 'transfer':{
    const amount=BigInt(String(args[1])),balance=this.ledger.balances.get(address(tx.from))??0n;
    if(balance<amount)throw new Error('Insufficient USDL balance');
    if(commit){this.ledger.balances.set(address(tx.from),balance-amount);this.ledger.balances.set(address(String(args[0])),(this.ledger.balances.get(address(String(args[0])))??0n)+amount);}result=true;break;
   }
   case 'factory':result=deployment.factory;break;
   case 'poolRegistry':result=deployment.poolRegistry;break;
   case 'protocolConfig':result=deployment.protocolConfig;break;
   case 'usdlAddress':result=deployment.usdl;break;
   case 'treasury':result=treasury;break;
   case 'creationFee':result=this.fee;break;
   case 'createMarket':
    if(address(tx.to)!==deployment.router)throw new Error('Wrong Router destination');
    this.ledger.pull(tx.from,this.fee,commit);if(commit)this.registered=true;result=[token,pool,1n];break;
   case 'buy':{
    if(!this.registered||address(String(args[0]))!==pool||address(tx.to)!==deployment.router)throw new Error('Unknown purchase route');
    const amount=BigInt(String(args[1]));if(BigInt(String(args[2]))>amount*10n||BigInt(String(args[3]))<=BigInt(Math.floor(Date.now()/1000)))throw new Error('Unsafe purchase');
    this.ledger.pull(tx.from,amount,commit);result=amount*10n;break;
   }
   case 'isRegisteredPool':result=this.registered&&address(String(args[0]))===pool;break;
   case 'getPoolByToken':if(!this.registered||address(String(args[0]))!==token)throw new Error('Unregistered token');result=pool;break;
   case 'getPoolsByCreatorPage':result=this.registered&&address(String(args[0]))===owner?[pool]:[];break;
   case 'getPoolMetadata':if(!this.registered||address(String(args[0]))!==pool)throw new Error('Unregistered pool');result={creator:owner,token,optical:zeroAddress,nftId:1n,createdAt:1n,createdBlock:1n};break;
   case 'tokenAddress':result=token;break;
   case 'opticalAddress':result=zeroAddress;break;
   case 'quoteExactInput':if(!this.registered)throw new Error('Unregistered pool');result={amountOut:BigInt(String(args[1]))*10n,feeAmount:0n,priceImpactBps:1n};break;
   default:throw new Error('Unsupported ledger operation');
  }
  return encodeFunctionResult({abi,functionName:decoded.functionName,result} as Parameters<typeof encodeFunctionResult>[0]);
 }
 send(tx:WireTransaction):Hex {
  this.execute(tx,true);
  this.height++;const block=this.block(this.height);this.blocks.set(this.height,block);
  const input=tx.data??tx.input!,hash=keccak256(stringToHex(JSON.stringify([tx,this.height.toString()])));
  const index=toHex(this.transactions.size);
  const record={hash,from:tx.from,to:tx.to,input,value:'0x0',chainId:'0x7d',nonce:index,gas:'0x186a0',gasPrice:'0x1',type:'0x0',v:'0x11d',r:h(1),s:h(1),blockHash:block.hash,blockNumber:block.number,transactionIndex:'0x0'};
  this.transactions.set(hash,record);
  const decoded=decodeFunctionData({abi,data:input});
  const logs:WireRecord[]=[];
  const log=(topics:readonly Hex[],data:Hex)=>logs.push({address:decoded.functionName==='buy'?trade.emitter:deployment.factory,topics,data,blockNumber:block.number,blockHash:block.hash,transactionHash:hash,transactionIndex:'0x0',logIndex:'0x0',removed:false});
  if(decoded.functionName==='createMarket')log(exactTopics(encodeEventTopics({abi,eventName:'MarketCreated',args:{token,pool,creator:owner}})),encodeAbiParameters([{type:'uint256'},{type:'address'}],[1n,zeroAddress]));
  if(decoded.functionName==='buy'){
   const amount=decoded.args[1];
   log(exactTopics(encodeEventTopics({abi,eventName:'Swap',args:{poolId:`0x${pool.slice(2).padStart(64,'0')}`,sender:tx.from}})),encodeAbiParameters([{type:'bool'},...Array.from({length:6},()=>({type:'uint256'} as const))],[true,amount,amount*10n,0n,1n,BigInt(Math.floor(Date.now()/1000)),this.height]));
  }
  this.receipts.set(hash,{transactionHash:hash,transactionIndex:'0x0',blockHash:block.hash,blockNumber:block.number,from:tx.from,to:tx.to,cumulativeGasUsed:'0x186a0',gasUsed:'0x186a0',contractAddress:null,logs,logsBloom:`0x${'0'.repeat(512)}`,status:'0x1',effectiveGasPrice:'0x1',type:'0x0'});
  return hash;
 }
 rpc(method:string,params:unknown[]):unknown {
  switch(method){
   case 'eth_chainId':return '0x7d';
   case 'eth_accounts':return [owner];
   case 'eth_blockNumber':return toHex(this.height);
   case 'eth_getBlockByNumber':return this.blocks.get(params[0]==='latest'?this.height:BigInt(String(params[0])));
   case 'eth_getBlockByHash':return [...this.blocks.values()].find(block=>block.hash===params[0]);
   case 'eth_getCode':return code;
   case 'eth_getBalance':return '0xde0b6b3a7640000';
   case 'eth_gasPrice':return '0x1';
   case 'eth_call':return this.execute(params[0] as WireTransaction);
   case 'eth_estimateGas':this.execute(params[0] as WireTransaction);return '0x186a0';
   case 'eth_sendTransaction':return this.send(params[0] as WireTransaction);
   case 'eth_getTransactionReceipt':return this.receipts.get(params[0] as Hex)??null;
   case 'eth_getTransactionByHash':return this.transactions.get(params[0] as Hex)??null;
   default:throw new Error(`Unexpected JSON-RPC method: ${method}`);
  }
 }
 async serve(req:IncomingMessage,res:ServerResponse){
  const chunks:Buffer[]=[];for await(const chunk of req)chunks.push(Buffer.from(chunk));
  const text=Buffer.concat(chunks).toString();res.setHeader('Content-Type','application/json');
  if(req.url==='/rpc'){
   const request=JSON.parse(text) as {id:number;method:string;params:unknown[]};
   try{res.end(JSON.stringify({jsonrpc:'2.0',id:request.id,result:this.rpc(request.method,request.params)}));}
   catch(error){res.end(JSON.stringify({jsonrpc:'2.0',id:request.id,error:{code:-32000,message:error instanceof Error?error.message:String(error)}}));}return;
  }
  if(req.url==='/v2/control/caps/pool_creation'){
   res.end(JSON.stringify({data:{capability:'pool_creation',allowed:this.allowed,stale:this.stale,reason:this.allowed?'available':'operator_disabled',revision:'1',observedAt:new Date().toISOString()}}));return;
  }
  if(req.url==='/v2/preflight/create'){
   const request=JSON.parse(text) as Record<string,unknown>;
   const stage=request.creationStage as CreationAdmissionStage;
   if(request.expectedRevision!==revision.revisionSha256||request.family!=='creation'||request.trader!==owner||request.recipient!==owner||request.blockHash!==h(Number(this.height))||!['creation','approval'].includes(stage))throw new Error('Unbound fixture preflight request');
   this.stages.push({stage,height:this.height,allowance:this.ledger.allowance(owner,native)});
   const data=stage==='approval'?encodeFunctionData({abi,functionName:'approve',args:[native,this.fee]}):encodeFunctionData({abi,functionName:'createMarket',args:[String(request.name),String(request.symbol),Number(request.feeStrategy),zeroAddress]});
   let supported=true;
   try{this.execute({from:owner,to:(stage==='approval'?deployment.usdl:deployment.router) as Address,data});}catch{supported=false;}
   res.end(JSON.stringify({schemaVersion:2,revision,family:'creation',creationStage:stage,blockHash:request.blockHash,blockNumber:Number(this.height),trader:owner,recipient:owner,supported,executionPermitted:supported&&this.allowed,simulationMode:stage==='approval'?'usdl_approval':'create_market',requestSha256:createHash('sha256').update(text).digest('hex'),creationCost:{protocolFeeAsset:'USDL',protocolFeeRaw:this.fee.toString(),gasAsset:'PAX',gasEstimateAvailable:supported,blockHash:request.blockHash,spender:native},...(!supported?{reasonCode:'gas_estimate_unavailable'}:{})}));return;
  }
  res.statusCode=404;res.end('{}');
 }
}

class MemoryCreationStore implements Pick<CreationStorage,keyof CreationStorage> {
 private rows=new Map<string,LaunchAttempt>();private saved?:CreationDraftV2;private locked=false;
 constructor(private creator:string){}
 saveDraft(value:CreationDraftV2){if(address(value.creator)!==this.creator||Object.keys(validateCreationDraft(value)).length)throw new Error('Invalid draft');this.saved=structuredClone(value);}
 restoreDraft(){return this.saved?structuredClone(this.saved):null;}
 async history(){return [...this.rows.values()].map(value=>structuredClone(checkedAttempt(value,this.creator)));}
 async put(value:LaunchAttempt){checkedAttempt(value,this.creator);this.rows.set(value.intentId,structuredClone(value));}
 async admit(value:LaunchAttempt){checkedAttempt(value,this.creator,true);if([...this.rows.values()].some(prior=>prior.intentId===value.intentId||prior.draftId===value.draftId&&!prior.deliberateNew))throw new Error('Recover existing intent');await this.put(value);}
 async releaseUnsent(value:LaunchAttempt){const prior=this.rows.get(value.intentId);if(!prior||prior.state!=='signing_locked'||prior.createTxHash)throw new Error('Submitted intent');this.rows.delete(value.intentId);}
 async exclusively<T>(action:()=>Promise<T>){if(this.locked)throw new Error('Concurrent launch');this.locked=true;try{return await action();}finally{this.locked=false;}}
}

async function fixture(){
 const peer=new CreationWirePeer();
 const server=createServer((req,res)=>{void peer.serve(req,res).catch(error=>{res.statusCode=500;res.end(JSON.stringify({error:String(error)}));});});
 server.listen(0,'127.0.0.1');await once(server,'listening');const socket=server.address();if(!socket||typeof socket==='string')throw new Error('No loopback address');
 const origin=`http://127.0.0.1:${socket.port}`;
 const transport=http(`${origin}/rpc`,{retryCount:0});
 const client=createPublicClient({chain,transport,cacheTime:0,pollingInterval:10}),wallet=createWalletClient({account:owner,chain,transport});
 const originalFetch=globalThis.fetch;
 // Node supplies URL resolution here; every request goes through real HTTP.
 globalThis.fetch=(input,init)=>originalFetch(typeof input==='string'&&input.startsWith('/')?new URL(input,origin):input,init);
 const authority:Pick<RuntimeAuthority,'revision'|'expiresAt'|'network'>={revision,expiresAt:Date.now()+60000,network:{usdlRuntimeHash:keccak256(code),network:{schemaVersion:1,deploymentVersion:revision.deploymentVersion,manifestHash:revision.manifestSha256,sourceManifestSha256:revision.manifestSha256,chainId:125,publicRPC:`${origin}/rpc`,explorerBase:origin,native:{symbol:'PAX',decimals:18},settlement:{symbol:'USDL',decimals:6},creationFeeRaw:'1000000',addresses:{Router:deployment.router,SidioraFactory:deployment.factory,USDL:deployment.usdl,ProtocolConfig:deployment.protocolConfig,PoolRegistry:deployment.poolRegistry,LaunchpadPrecompile:native}},async committedHead(signal){signal.throwIfAborted();const block=await client.getBlock();if(!block.hash||block.number===null)throw new Error('Missing committed block');return {number:block.number,hash:block.hash};}}};
 const scope={creator:owner,chainId:125,epoch:1};
 const adapter=new ChainCreationAdapter(client as PublicClient,wallet as WalletClient,deployment,()=>scope,runtimeCreationAdmission(authority,deployment));
 const storage:Pick<CreationStorage,keyof CreationStorage>=new MemoryCreationStore(owner);
 const controller=new CreationController(adapter,storage as CreationStorage,async message=>wallet.signMessage({account:owner,message}));
 return {peer,client,wallet,authority,scope,adapter,storage,controller,async close(){controller.dispose();globalThis.fetch=originalFetch;server.closeAllConnections();await new Promise<void>((resolve,reject)=>server.close(error=>error?reject(error):resolve()));}};
}

async function nativeClientSpenderCreationAndFirstBuy(){
 const f=await fixture();try{
  const d=draft();const read=await f.adapter.readFunding(d,f.scope);
  expect(read.gasStage).toBe('approval');expect(read.cost.approvalSpender).toBe(native);expect(creationFunding(read).state).toBe('approval_required');
  const approval=await f.adapter.approve('1000000',f.scope);await f.adapter.approvalProof(approval,'1000000',f.scope);
  expect(f.peer.ledger.allowance(owner,native)).toBe(1_000_000n);expect(f.peer.ledger.allowance(owner,deployment.router)).toBe(0n);
  const create=await f.adapter.sendCreation(d,f.scope);const resolved=await f.adapter.resolve(create,owner,undefined,d);
  expect(resolved.state).toBe('resolved');expect(f.peer.ledger.credits).toBe(1_000_000n);
  const buyApproval=await f.adapter.approve(d.firstBuyRaw,f.scope);await f.adapter.approvalProof(buyApproval,d.firstBuyRaw,f.scope);
  expect(await f.adapter.purchaseAllowance(owner)).toBe(d.firstBuyRaw);
  const quote=await f.adapter.quotePurchase(pool,d.firstBuyRaw,100,f.scope);
  const buy=await f.adapter.sendPurchase(quote,f.scope);expect((await f.adapter.purchaseReceipt(buy,quote)).status).toBe('success');
  expect(f.peer.ledger.balances.get(owner)).toBe(17_000_000n);expect(f.peer.ledger.credits).toBe(3_000_000n);expect(f.peer.ledger.allowance(owner,native)).toBe(0n);
  expect(f.peer.allowanceReads.length).toBeGreaterThan(4);expect(new Set(f.peer.allowanceReads)).toEqual(new Set([native]));
  for(const tx of [create,buy])expect((await f.client.getTransaction({hash:tx as Hex})).to).toBe(deployment.router);
 }finally{await f.close();}
}

async function nativeDomainSpenderProofRejectsHostileContexts(){
 const f=await fixture();try{
  const tx=await f.adapter.approve('1000000',f.scope),proof=await f.adapter.approvalProof(tx,'1000000',f.scope);
  const cost=verifyCost(deployment,{revision:deployment.revision,decimals:6,feeRaw:'1000000',gasRaw:'100000'});
  expect(funding(cost,deployment,{usdl:'2000000',pax:'1000000',allowance:proof.allowanceRaw}).approvalSpender).toBe(native);
  const expected:ApprovalContext={chainId:125,token:deployment.usdl,spender:cost.approvalSpender,account:owner,manifestRevision:deployment.revision,accountEpoch:f.scope.epoch,targetRaw:'1000000',transactionHash:tx,blockHeight:String(proof.receipt.blockNumber),blockHash:proof.receipt.blockHash};
  const receipt={...expected,status:proof.receipt.status},allowance={...expected,allowanceRaw:proof.allowanceRaw,observedAt:Date.now()};
  expect(verifyCreationApproval(expected,receipt,allowance,deployment,Date.now(),f.scope.epoch)).toBe(true);
  for(const patch of [{spender:deployment.router},{spender:a(99)},{account:a(99)},{manifestRevision:'changed'},{accountEpoch:2},{chainId:1},{targetRaw:'2000000'},{transactionHash:h(99)},{blockHash:h(99)},{blockHeight:'99'}])expect(()=>verifyCreationApproval(expected,{...receipt,...patch},allowance,deployment,Date.now(),f.scope.epoch)).toThrow();
  for(const spender of [deployment.router,a(99)])expect(()=>verifyCreationApproval({...expected,spender},{...receipt,spender},{...allowance,spender},deployment,Date.now(),f.scope.epoch)).toThrow('approval_context_invalid');
  expect(()=>verifyCreationApproval(expected,receipt,{...allowance,allowanceRaw:'1000001'},deployment,Date.now(),f.scope.epoch)).toThrow('approval_not_verified');
  expect(()=>verifyCreationApproval(expected,receipt,{...allowance,observedAt:Date.now()-10001},deployment,Date.now(),f.scope.epoch)).toThrow('stale_approval_proof');
 }finally{await f.close();}
}

async function approvalAdmissionZeroAllowanceController(){
 const f=await fixture();try{
  const d=draft();await expect(f.adapter.admitCreation(d,f.scope)).rejects.toThrow('Creation is unavailable');expect(f.peer.transactions.size).toBe(0);
  const reviewed=await f.controller.reviewDraft(d);expect(reviewed.read.gasStage).toBe('approval');
  await f.controller.approve();expect(f.controller.state.reviewed).toBeUndefined();expect(f.controller.state.phase).toBe('draft');
  expect(f.peer.transactions.size).toBe(1);expect(f.peer.stages.filter(value=>value.stage==='approval').every(value=>value.allowance===0n)).toBe(true);
  await expect(f.controller.create()).rejects.toThrow('Review this launch');expect(f.peer.transactions.size).toBe(1);
  const fresh=await f.controller.reviewDraft(d);expect(fresh.read.gasStage).toBe('creation');
  const attempt=await f.controller.create();expect(attempt.state).toBe('submitted');expect(f.peer.transactions.size).toBe(2);expect(f.peer.ledger.credits).toBe(1_000_000n);
  const creations=f.peer.stages.filter(value=>value.stage==='creation');expect(creations.length).toBeGreaterThan(1);expect(creations.slice(1).every(value=>value.allowance===1_000_000n&&value.height>1n)).toBe(true);
  expect((await f.storage.history())[0].createTxHash).toBe(attempt.createTxHash);
 }finally{await f.close();}
}

async function approvalAdmissionRetainsAuthorityChecks(){
 const f=await fixture();try{
  const d=draft();
  for(const change of ['expired','capability','stale','epoch','revision','spender'] as const){
   await f.controller.reviewDraft(d);
   const oldExpiry=f.authority.expiresAt;
   if(change==='expired')f.authority.expiresAt=Date.now()-1;
   if(change==='capability')f.peer.allowed=false;
   if(change==='stale')f.peer.stale=true;
   if(change==='epoch')f.scope.epoch++;
   if(change==='revision')f.authority.revision={...revision,revisionSha256:'6'.repeat(64)};
   if(change==='spender')f.authority.network={...f.authority.network,network:{...f.authority.network.network,addresses:{...f.authority.network.network.addresses,LaunchpadPrecompile:a(99)}}};
   await expect(f.controller.approve()).rejects.toThrow();expect(f.peer.transactions.size).toBe(0);
   f.authority.expiresAt=oldExpiry;f.peer.allowed=true;f.peer.stale=false;f.authority.revision=revision;
   f.authority.network={...f.authority.network,network:{...f.authority.network.network,addresses:{...f.authority.network.network.addresses,LaunchpadPrecompile:native}}};
  }
 }finally{await f.close();}
}

async function canonicalApprovalProofRejectsWrongAmountWalletAndSpender(){
 const f=await fixture();try{
  for(const args of [[trade.router,1_000_000n],[native,2_000_000n]] as const){
   const tx=await f.wallet.writeContract({address:deployment.usdl as Address,abi,functionName:'approve',args});
   await expect(f.adapter.approvalProof(tx,'1000000',f.scope)).rejects.toThrow('reviewed amount and spender');
  }
  const tx=await f.adapter.approve('1000000',f.scope);
  await expect(f.adapter.approvalProof(tx,'1000000',{...f.scope,creator:a(99)})).rejects.toThrow('Wallet changed');
  f.scope.epoch++;await expect(f.adapter.approvalProof(tx,'1000000',{...f.scope,epoch:1})).rejects.toThrow('Wallet changed');
  expect(f.peer.ledger.credits).toBe(0n);
 }finally{await f.close();}
}

async function creationRequiresFreshFundedAdmissionAfterApproval(){
 const f=await fixture();try{
  const d=draft();await f.controller.reviewDraft(d);await f.controller.approve();await f.controller.reviewDraft(d);
  f.peer.allowed=false;
  await expect(f.controller.create()).rejects.toThrow('disabled');expect(f.peer.transactions.size).toBe(1);expect((await f.storage.history()).length).toBe(0);
  f.peer.allowed=true;await f.controller.reviewDraft(d);
  await f.wallet.writeContract({address:deployment.usdl as Address,abi,functionName:'transfer',args:[a(99),19_000_001n]});
  await expect(f.controller.create()).rejects.toThrow('insufficient_usdl');expect(f.peer.ledger.credits).toBe(0n);
 }finally{await f.close();}
}

test(nativeClientSpenderCreationAndFirstBuy,{timeout:30000},nativeClientSpenderCreationAndFirstBuy);
test(nativeDomainSpenderProofRejectsHostileContexts,{timeout:30000},nativeDomainSpenderProofRejectsHostileContexts);
test(approvalAdmissionZeroAllowanceController,{timeout:30000},approvalAdmissionZeroAllowanceController);
test(approvalAdmissionRetainsAuthorityChecks,{timeout:30000},approvalAdmissionRetainsAuthorityChecks);
test(canonicalApprovalProofRejectsWrongAmountWalletAndSpender,{timeout:30000},canonicalApprovalProofRejectsWrongAmountWalletAndSpender);
test(creationRequiresFreshFundedAdmissionAfterApproval,{timeout:30000},creationRequiresFreshFundedAdmissionAfterApproval);
