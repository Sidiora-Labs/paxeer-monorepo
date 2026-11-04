import { registerHooks, stripTypeScriptTypes } from 'node:module';
import { readFileSync, existsSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { createHash, generateKeyPairSync } from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';
// Manual recording only: install the gateway dependencies, then run with Node 24.
// The Rust gate only reads the committed JSON and never starts this constructor.
const root = fileURLToPath(new URL('../../../../../../', import.meta.url));
registerHooks({
 resolve(specifier, context, nextResolve) {
  if (specifier.endsWith('.js') && (specifier.startsWith('.') || specifier.startsWith('/'))) {
   const url = new URL(specifier, context.parentURL).href.replace(/\.js$/, '.ts');
   if (existsSync(fileURLToPath(url))) return { url, shortCircuit: true };
  }
  if (!specifier.startsWith('.') && !specifier.startsWith('/') && !specifier.startsWith('node:') && !specifier.startsWith('file:')) {
   return nextResolve(specifier, context.parentURL?.startsWith(pathToFileURL(root).href) && !context.parentURL.includes('/node_modules/') ? {...context,parentURL:pathToFileURL(root+'services/wallet/gateway/package.json').href} : context);
  }
  return nextResolve(specifier, context);
 },
 load(url, context, nextLoad) {
  if (url.endsWith('.ts')) return {format:'module',source:stripTypeScriptTypes(readFileSync(fileURLToPath(url),'utf8'),{mode:'transform',sourceUrl:url}),shortCircuit:true};
  return nextLoad(url,context);
 }
});
process.env.NODE_ENV='test';
process.env.CORS_ORIGINS='http://localhost';
process.env.SUPABASE_URL='https://identity.example.test';
process.env.DATABASE_URL='postgres://fixture@localhost/wallet';
process.env.WALLET_MASTER_KEY=Buffer.alloc(32,42).toString('base64');
const {produceIdentityBinding}=await import(pathToFileURL(root+'services/wallet/gateway/src/provision/state.ts'));
const {RpcPool}=await import(pathToFileURL(root+'services/wallet/gateway/src/rpc/pool.ts'));
const {TestChain}=await import(pathToFileURL(root+'services/wallet/gateway/test/support/chain.ts'));
const {mainAccountId,addrAbi}=await import(pathToFileURL(root+'services/wallet/gateway/src/provision/bind.ts'));
const {encodeFunctionData}=await import('viem');
const directory=mkdtempSync('/tmp/binding-recording-');
const key=generateKeyPairSync('ec',{namedCurve:'prime256v1'});
const keyfile=directory+'/producer.pem';
writeFileSync(keyfile,key.privateKey.export({format:'pem',type:'pkcs8'}),{mode:0o600});
const jwk=key.publicKey.export({format:'jwk'});
const publicHex='04'+Buffer.from(jwk.x,'base64url').toString('hex')+Buffer.from(jwk.y,'base64url').toString('hex');
const publicKey='3f1c0a9e5b7d2468ace013579bdf2468ace013579bdf2468ace013579bdf2468';
const did='did:layerx:'+publicKey;
const address='0x1111111111111111111111111111111111111111';
const user='supabase-user-a';
const chain=new TestChain(125);
chain.bindings.set(address,publicKey);
chain.block=37n;
await chain.start();
try {
 const rpc=new RpcPool({urls:[chain.url],chainId:125,lagThresholdBlocks:2,timeoutMs:3000,healthIntervalMs:60000});
 const sql=[];
 const pool={async query(query,params){
  if(!query.includes('select p.ed_public_key from account_provisioning p join wallets w') || JSON.stringify(params)!==JSON.stringify([user,'wallet-recorded',did,address])) throw new Error('unrecorded query');
  sql.push({query,params});
  return {rows:[{ed_public_key:publicKey}]};
 }};
 const provision={state:'active',awaiting:null,walletId:'wallet-recorded',address,did,mainAccountId:mainAccountId(did),bindMessage:null};
 const deps={pool,rpc,identityBinding:{issuer:'https://identity.example.test/auth/v1',tenant:'human-provider',privateKeyFile:keyfile}};
 const binding=await produceIdentityBinding(deps,user,provision);
 const crossTenant=await produceIdentityBinding({...deps,identityBinding:{...deps.identityBinding,tenant:'other-tenant'}},user,provision);
 const calldata=encodeFunctionData({abi:addrAbi,functionName:'getUnifiedAccount',args:[address]});
 const observations=[];
 for (const height of [37,38]) {
  chain.block=BigInt(height);
  const params=[{to:'0x0000000000000000000000000000000000001004',data:calldata},'0x'+height.toString(16)];
  observations.push({height,params,result:await rpc.request('eth_call',params)});
 }
 const sources={};
 for(const file of ['services/wallet/gateway/src/provision/state.ts','services/wallet/gateway/src/provision/bind.ts','services/wallet/gateway/src/rpc/pool.ts','services/wallet/gateway/test/support/chain.ts']) sources[file]=createHash('sha256').update(readFileSync(root+file)).digest('hex');
 const record={producer:'services/wallet/gateway/src/provision/state.ts:produceIdentityBinding',peer:'committed TestChain and in-memory SQL rows; production issuer, RpcPool and readUnifiedAccount are unchanged',sources,issuer:deps.identityBinding.issuer,subject:user,tenant:deps.identityBinding.tenant,did,address,mainAccountId:provision.mainAccountId,publicKey:publicHex,binding,crossTenant,account:{calldata,observations},sql};
 writeFileSync(root+'services/human/crates/layerx-human-identity-provider/tests/fixtures/wallet_binding_22_33_35.json',JSON.stringify(record,null,2)+'\n');
} finally {await chain.stop(); rmSync(directory, {recursive:true});}
