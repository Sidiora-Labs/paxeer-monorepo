import {afterEach,expect,it,test,vi} from 'vitest';
import {closeStoredPictureService,storedPictureBytes,storedPictureService} from './stored-picture.test-support';
import {fingerprintCreatorImage,createMetadataImageTransport} from './transport';
afterEach(async()=>{await closeStoredPictureService();vi.useRealTimers();});
it('uses the actual byte digest as retry identity',async()=>{
 const a=new File([new Uint8Array([97,98,99])],'first.png',{type:'image/png'});
 const b=new File([new Uint8Array([97,98,99])],'other.png',{type:'image/png'});
 expect(await fingerprintCreatorImage(a)).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');expect(await fingerprintCreatorImage(b)).toBe(await fingerprintCreatorImage(a));
});
it('rejects active image formats before requesting authentication or sending bytes',async()=>{
 await expect(fingerprintCreatorImage(new File(['<svg/>'],'x.png',{type:'image/svg+xml'}))).rejects.toThrow('Choose JPEG, PNG or WebP');
 await expect(fingerprintCreatorImage(new File([new Uint8Array(6291457)],'large.png',{type:'image/png'}))).rejects.toThrow('up to 6 MiB');
});

it('rejects unbounded revision before any image operation begins',()=>{
 const authority={tokenAddress:'0x'+'1'.repeat(40),wallet:'0x'+'2'.repeat(40),audience:'https://metadata.example',revision:'9'.repeat(79),authenticationHeaders:async()=>({}),signMessage:async()=>''};
 expect(()=>createMetadataImageTransport(authority)).toThrow('Invalid creator identity.');
});
it('cancelled upload fails before reading selected bytes or authenticating',async()=>{
 const authority={tokenAddress:'0x'+'1'.repeat(40),wallet:'0x'+'2'.repeat(40),audience:'https://metadata.example',revision:'7',authenticationHeaders:async()=>{throw new Error('Authentication must not run')},signMessage:async()=>{throw new Error('Signing must not run')}};
 const upload=createMetadataImageTransport(authority);const controller=new AbortController();controller.abort();
 await expect(upload(new File(['png'],'image.png',{type:'image/png'}),'11111111-1111-1111-1111-111111111111',controller.signal)).rejects.toMatchObject({name:'AbortError'});
});

async function uploadGoesOnWithTheStoredAssetOfTheSameBytes() {
 const service = await storedPictureService();
 try {
  const operation = crypto.randomUUID();
  expect(operation).not.toBe(service.asset.assetId);
  const upload = createMetadataImageTransport(service.authority);
  const result = await upload(new File([storedPictureBytes], 'picture.png', {type:'image/png'}), operation, new AbortController().signal);
  expect(result).toEqual({operationId:operation,assetId:service.asset.assetId,state:'APPROVED',revision:'8',url:service.asset.publicUrl});
  const request = service.requests.find(row=>row.method==='PATCH');
  expect(request).toBeDefined();
  expect(JSON.parse(request!.body.toString())).toEqual({expectedRevision:'7',logoAssetId:service.asset.assetId});
  expect(service.requests.filter(row=>row.method==='PUT')).toHaveLength(1);
  expect(service.failures).toEqual([]);
 } finally { await service.close(); }
}

test(uploadGoesOnWithTheStoredAssetOfTheSameBytes, uploadGoesOnWithTheStoredAssetOfTheSameBytes);

async function uploadPollsTheStoredAssetOfTheSameBytes() {
 const service = await storedPictureService({pending:true});
 try {
  const operation = crypto.randomUUID();
  vi.useFakeTimers({toFake:['setTimeout','clearTimeout']});
  const uploading = createMetadataImageTransport(service.authority)(new File([storedPictureBytes], 'picture.png', {type:'image/png'}), operation, new AbortController().signal);
  await vi.waitFor(()=>expect(vi.getTimerCount()).toBe(1));
  await vi.advanceTimersByTimeAsync(1000);
  const outcome = await uploading;
  expect(outcome).toMatchObject({operationId:operation,assetId:service.asset.assetId,state:'APPROVED'});
  const reads = service.requests.filter(row=>row.method==='GET'&&row.path.includes('/assets/'));
  expect(reads).toHaveLength(1);
  expect(reads[0].path).toBe(`/v2/tokens/${service.authority.tokenAddress}/assets/${service.asset.assetId}`);
  expect(reads[0].idempotency).toBe(service.asset.assetId);
  expect(service.failures).toEqual([]);
 } finally { await service.close(); }
}

test(uploadPollsTheStoredAssetOfTheSameBytes, uploadPollsTheStoredAssetOfTheSameBytes);

it.each(['hash','owner'] as const)('refuses a stored asset with another %s',async field=>{
 const service = await storedPictureService();
 try {
  if(field==='hash')service.asset.sha256='0x'+'f'.repeat(64);
  else service.asset.ownerWallet='0x'+'3'.repeat(40);
  await expect(createMetadataImageTransport(service.authority)(new File([storedPictureBytes],'picture.png',{type:'image/png'}),crypto.randomUUID(),new AbortController().signal)).rejects.toThrow('Image response did not match this operation.');
  expect(service.requests.map(row=>row.method)).toEqual(['PUT']);
  expect(service.failures).toEqual([]);
 } finally { await service.close(); }
});
