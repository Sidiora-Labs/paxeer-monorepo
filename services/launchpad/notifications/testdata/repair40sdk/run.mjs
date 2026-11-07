import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, mkdirSync, cpSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { stripTypeScriptTypes } from 'node:module';
import { execFileSync } from 'node:child_process';

const [root, workspace, endpoint, token, mode, expectedID, expectedWallet, expectedReadAt] = process.argv.slice(2);
const sdk = resolve(root, 'sdk/launchpad/packages/sdk');
const temporarySDK = join(workspace, 'sdk');
mkdirSync(join(temporarySDK, 'src'), {recursive:true});
mkdirSync(join(temporarySDK, 'scripts'), {recursive:true});
cpSync(join(sdk,'src'),join(temporarySDK,'src'),{recursive:true});
cpSync(join(sdk,'scripts/compile-validators.mjs'),join(temporarySDK,'scripts/compile-validators.mjs'));
mkdirSync(join(workspace,'sdk-v2/src'),{recursive:true});
cpSync(resolve(sdk,'../sdk-v2/src/schemas.gen.json'),join(workspace,'sdk-v2/src/schemas.gen.json'));
writeFileSync(join(temporarySDK,'package.json'),'{"type":"module"}');
execFileSync(process.execPath,[join(temporarySDK,'scripts/compile-validators.mjs')],{stdio:['ignore','pipe','pipe']});
for (const name of ['transport','validate','operations','operations.gen','shared-snapshot','served-routes','route-corrections']) {
 const source=readFileSync(join(temporarySDK,'src',name+'.ts'),'utf8');
 writeFileSync(join(temporarySDK,'dist',name+'.js'),stripTypeScriptTypes(source,{mode:'transform',sourceUrl:join(sdk,'src',name+'.ts')}));
}
const {SdkClient,HttpError}=await import(pathToFileURL(join(temporarySDK,'dist/transport.js')));
const {Operations}=await import(pathToFileURL(join(temporarySDK,'dist/operations.js')));
const config={apiOrigin:endpoint,identityOrigin:endpoint,mediaOrigin:endpoint,streamOrigin:'ws://127.0.0.1',approvedStreamOrigins:['ws://127.0.0.1'],production:false,session:{mode:'bearer',token:()=>token}};
const client=new SdkClient(config);
const operations=new Operations(client);
const trace='b2b67d36-02db-4aab-8e02-4891a03b7eaa';
await assert.rejects(()=>client.requestV2('/v2/me/notifications','NotificationsResponse',{authenticated:true,headers:{'X-Actor-Wallet':expectedWallet}}),/Unsupported operation header/);
if(mode==='foreign') {
 await assert.rejects(()=>operations.call('IF_NOTIFICATION_READ',{path:{notificationId:expectedID}}),e=>e instanceof HttpError && e.status===404 && e.code==='NOT_FOUND');
 const page=await operations.call('IF_NOTIFICATIONS',{});
 assert.equal(page.data.items.length,0);
 console.log(JSON.stringify({foreign:true}));
} else {
 const page=await operations.call('IF_NOTIFICATIONS',{}, {requestId:trace});
 assert.equal(page.requestId,trace);
 assert.equal(page.quality,'complete');
 assert.equal(page.data.items.length,1);
 const item=page.data.items[0];
 assert.equal(item.notificationId,expectedID);
 assert.equal(item.wallet,expectedWallet);
 assert.equal(item.readAt,expectedReadAt || null);
 const read=await operations.call('IF_NOTIFICATION_READ',{path:{notificationId:expectedID}}, {requestId:trace});
 assert.equal(read.requestId,trace);
 assert.equal(read.quality,'complete');
 if(expectedReadAt) assert.equal(read.data.readAt,expectedReadAt);
 const unread=await operations.call('IF_NOTIFICATIONS',{query:{unreadOnly:true}});
 assert.equal(unread.data.items.length,0);
 console.log(JSON.stringify({readAt:read.data.readAt,item}));
}
