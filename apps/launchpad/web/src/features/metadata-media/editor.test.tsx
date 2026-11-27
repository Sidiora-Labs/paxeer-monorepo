// @vitest-environment jsdom
import {afterEach,it,test,expect,vi} from 'vitest';
import {File as NativeFile} from 'node:buffer';
import {closeStoredPictureService,storedPictureBytes,storedPictureService} from './stored-picture.test-support';
import {createMetadataImageTransport} from './transport';
import {act} from 'react';
import {createRoot} from 'react-dom/client';
import {MetadataImageEditor,type ImageOutcome} from './editor';
afterEach(closeStoredPictureService);
it('cancel retires late approval while retry preserves operation',async()=>{Object.assign(globalThis,{IS_REACT_ACT_ENVIRONMENT:true});let resolve!:(v:ImageOutcome)=>void;const upload=vi.fn((_f:File,_id:string,_s:AbortSignal)=>new Promise<ImageOutcome>(r=>{resolve=r}));const host=document.createElement('div');document.body.append(host);const root=createRoot(host);await act(async()=>root.render(<MetadataImageEditor activeRevision="7" upload={upload}/>));const input=host.querySelector('input')!;Object.defineProperty(input,'files',{value:[new File(['png'],'a.png',{type:'image/png'})]});await act(async()=>input.dispatchEvent(new Event('change',{bubbles:true})));const buttons=host.querySelectorAll('button');await act(async()=>buttons[0].click());expect(upload).toHaveBeenCalledTimes(1);const operation=upload.mock.calls[0][1];await act(async()=>buttons[1].click());await act(async()=>resolve({operationId:operation,assetId:'11111111-1111-1111-1111-111111111111',state:'APPROVED',revision:'8',url:`https://cdn.example/approved/${'a'.repeat(64)}/256/${'b'.repeat(64)}.png`}));expect(host.textContent).not.toContain('Image updated');await act(async()=>buttons[0].click());expect(upload.mock.calls[1][1]).toBe(operation);await act(async()=>root.unmount());host.remove()});

async function editorAcceptsTheStoredAssetOfTheSameBytes() {
 Object.assign(globalThis,{IS_REACT_ACT_ENVIRONMENT:true});
 const service=await storedPictureService();
 const host=document.createElement('div');document.body.append(host);
 const root=createRoot(host);
 const approved:ImageOutcome[]=[];
 try {
  await act(async()=>root.render(<MetadataImageEditor activeRevision="7" upload={createMetadataImageTransport(service.authority)} onApproved={outcome=>approved.push(outcome)}/>));
  const input=host.querySelector('input')!;
  Object.defineProperty(input,'files',{value:[new NativeFile([storedPictureBytes],'picture.png',{type:'image/png'})]});
  await act(async()=>input.dispatchEvent(new Event('change',{bubbles:true})));
  await act(async()=>{host.querySelector('button')!.click();});
  const until=Date.now()+5000;
  while(!approved.length&&Date.now()<until)await act(async()=>{await new Promise(resolve=>setTimeout(resolve,20));});
  expect(host.querySelector('[role="status"]')?.textContent).toBe('Image updated');
  expect(host.querySelector('[role="alert"]')?.textContent).toBe('');
  expect(approved).toHaveLength(1);
  expect(approved[0]).toMatchObject({assetId:service.asset.assetId,state:'APPROVED',revision:'8'});
  const requested=service.requests.find(row=>row.method==='PUT')!.path.split('/').pop();
  expect(approved[0].operationId).toBe(requested);
  expect(requested).not.toBe(service.asset.assetId);
  expect(service.failures).toEqual([]);
 } finally {
  await act(async()=>root.unmount());host.remove();await service.close();
 }
}

test(editorAcceptsTheStoredAssetOfTheSameBytes, editorAcceptsTheStoredAssetOfTheSameBytes);
