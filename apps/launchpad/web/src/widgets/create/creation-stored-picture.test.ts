import { afterEach, expect, test, vi } from 'vitest';
import { accountEpoch, clearAccountSession, installAccountSession } from '../profile/account-transport';
import { closeStoredPictureService, storedPictureBytes, storedPictureService } from '@/features/metadata-media/stored-picture.test-support';
import { CreationController } from './creation-controller';
import { CreationStorage } from './creation-storage';
import { MemoryCreationStore, creationWireFixture, creationWireDraft, owner, token } from './creation-wire.fixture';

let closeFixture: (() => Promise<void>) | undefined;
afterEach(async () => {
 await closeStoredPictureService();
 await closeFixture?.();
 closeFixture = undefined;
 clearAccountSession();
 vi.useRealTimers();
});

async function launchFlowKeepsTheStoredAssetOfTheSameBytes() {
 const fixture = await creationWireFixture();
 closeFixture = fixture.close;
 const service = await storedPictureService({token,owner,pending:true});
 let reopened: CreationController | undefined;
 try {
  const draft=creationWireDraft();
  draft.description='The coin picture and details.';
  await fixture.controller.reviewDraft(draft);
  await fixture.controller.approve();
  await fixture.controller.reviewDraft(draft);
  const attempt=await fixture.controller.create();
  await fixture.controller.resume(attempt.intentId);
  installAccountSession({sessionId:crypto.randomUUID(),wallet:owner,deviceLabel:'Web browser',scopes:['account'],accessExpiresAt:new Date(Date.now()+900000).toISOString(),refreshExpiresAt:new Date(Date.now()+86400000).toISOString(),revokedAt:null},crypto.randomUUID(),accountEpoch());
  vi.useFakeTimers({toFake:['setTimeout','clearTimeout']});
  const uploading = fixture.controller.uploadLogo(attempt.intentId,new File([storedPictureBytes],'picture.png',{type:'image/png'}));
  await vi.waitFor(()=>expect(service.requests.filter(row=>row.method==='PUT')).toHaveLength(1));
  await vi.advanceTimersByTimeAsync(1500);
  await uploading;
  vi.useRealTimers();
  const uploaded=(await fixture.storage.history())[0];
  expect(uploaded.metadataAssetId).toBe(service.asset.assetId);
  expect(uploaded.metadataUploadId).toBe(service.asset.assetId);
  expect(uploaded.metadataUploadHash).toBe(service.asset.sha256);
  const upload=service.requests.find(row=>row.method==='PUT')!;
  expect(upload.path.endsWith(service.asset.assetId)).toBe(false);
  const reads=service.requests.filter(row=>row.method==='GET'&&row.path.includes('/assets/'));
  expect(reads).toHaveLength(1);
  expect(reads[0].path).toBe(`/v2/tokens/${token}/assets/${service.asset.assetId}`);
  expect(reads[0].idempotency).toBe(service.asset.assetId);
  fixture.controller.dispose();
  const storage: Pick<CreationStorage,keyof CreationStorage>=new MemoryCreationStore(owner,fixture.memoryStorage.database);
  reopened=new CreationController(fixture.adapter,storage as CreationStorage,message=>fixture.wallet.signMessage({account:owner,message}));
  await reopened.publish(attempt.intentId);
  const saved=(await storage.history())[0];
  expect(saved.metadataAssetId).toBe(service.asset.assetId);
  expect(saved.metadataStatus).toBe('published');
  expect(JSON.parse(saved.metadataPatch!)).toMatchObject({description:draft.description,logoAssetId:service.asset.assetId});
  expect(JSON.parse(saved.metadataPublicationBody!)).toMatchObject({metadata:{logoAssetId:service.asset.assetId},uploadRefs:[service.asset.assetId]});
  const publication=service.requests.find(row=>row.method==='POST')!;
  expect(JSON.parse(publication.body.toString())).toEqual(JSON.parse(saved.metadataPublicationBody!));
  expect(service.failures).toEqual([]);
 } finally {
  reopened?.dispose();clearAccountSession();await service.close();await closeFixture?.();closeFixture=undefined;
 }
}

test(launchFlowKeepsTheStoredAssetOfTheSameBytes, launchFlowKeepsTheStoredAssetOfTheSameBytes);
