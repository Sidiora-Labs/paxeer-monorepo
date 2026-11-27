// @vitest-environment jsdom
import {expect,test} from 'vitest';
import {act} from 'react';
import {createRoot} from 'react-dom/client';
import {renderToStaticMarkup} from 'react-dom/server';
import type {MetadataMediaImage,MetadataV2Data} from '@/core/clients/metadata';
import {MetadataView,selectMetadataImage} from './viewer';

const picture:MetadataMediaImage={
 assetId:'11111111-1111-1111-1111-111111111111',
 sha256:'0x'+'a'.repeat(64),
 mime:'image/png',
 width:512,
 height:512,
 variants:{'256':'https://example.com/coin/256.png','512':'https://example.com/coin/512.png','1024':'https://example.com/coin/1024.png'},
};
const details:MetadataV2Data={
 token:{address:'0x'+'1'.repeat(40),name:'Canonical token',symbol:'CAN',decimals:6,decimalsQuality:'verified',totalSupplyRaw:'1000000',supplyQuality:'verified',metadataRevision:'8',logoAssetId:picture.assetId,sourcePosition:{chainId:'125',height:'1',blockHash:'0x'+'a'.repeat(64),transactionHash:'0x'+'b'.repeat(64),transactionIndex:'0',logIndex:'0'}},
 displayName:null,displaySymbol:null,description:'Coin description',tags:[],socialLinks:[],revision:'8',
};

function failedPictureGivesWay(){
 expect(selectMetadataImage(picture,true,picture.variants['512'])).toBeNull();
}

function pictureWithAnotherAddressIsTriedAgain(){
 expect(selectMetadataImage(picture,true,'https://example.com/previous/512.png')).toBe(picture);
}

function noPictureWithoutAPassedTest(){
 expect(selectMetadataImage(picture,false,null)).toBeNull();
 expect(selectMetadataImage(null,true,null)).toBeNull();
 expect(selectMetadataImage(undefined,true,undefined)).toBeNull();
}

function pictureIsDrawnWhileNoLoadFailed(){
 const host=document.createElement('div');
 host.innerHTML=renderToStaticMarkup(<MetadataView tokenAddress={details.token.address} tradePanel={<button>Trade</button>} data={details} pending={false} error={false} image={picture}/>);
 const images=host.querySelectorAll('img');
 expect(images).toHaveLength(1);
 expect(images[0].getAttribute('src')).toBe(picture.variants['512']);
 expect(images[0].getAttribute('alt')).toBe('Coin image');
}

async function failedLoadRemovesPictureAndAnotherAddressCanLoad(){
 const actEnvironment=globalThis as typeof globalThis&{IS_REACT_ACT_ENVIRONMENT?:boolean};
 const previousActEnvironment=actEnvironment.IS_REACT_ACT_ENVIRONMENT;
 actEnvironment.IS_REACT_ACT_ENVIRONMENT=true;
 const host=document.createElement('div');
 document.body.append(host);
 const root=createRoot(host);
 const render=(image:MetadataMediaImage)=>root.render(<MetadataView tokenAddress={details.token.address} tradePanel={<button>Trade</button>} data={details} pending={false} error={false} image={image}/>);
 try{
  await act(async()=>render(picture));
  const image=host.querySelector('img');
  expect(image).not.toBeNull();
  await act(async()=>{image!.dispatchEvent(new Event('error'));});
  expect(host.querySelectorAll('img')).toHaveLength(0);
  expect(host.textContent).toContain('Coin description');
  expect(host.textContent).toContain('Canonical token');
  expect(host.textContent).toContain(details.token.address);
  expect(host.querySelector('button')?.textContent).toBe('Trade');
  await act(async()=>render({...picture}));
  expect(host.querySelectorAll('img')).toHaveLength(0);
  const replacement={...picture,variants:{...picture.variants,'512':'https://example.com/replacement/512.png'}};
  await act(async()=>render(replacement));
  expect(host.querySelectorAll('img')).toHaveLength(1);
  expect(host.querySelector('img')?.getAttribute('src')).toBe(replacement.variants['512']);
  await act(async()=>{host.querySelector('img')!.dispatchEvent(new Event('error'));});
  expect(host.querySelectorAll('img')).toHaveLength(0);
 }finally{
  await act(async()=>root.unmount());
  host.remove();
  actEnvironment.IS_REACT_ACT_ENVIRONMENT=previousActEnvironment;
 }
}

test(failedPictureGivesWay,failedPictureGivesWay);
test(pictureWithAnotherAddressIsTriedAgain,pictureWithAnotherAddressIsTriedAgain);
test(noPictureWithoutAPassedTest,noPictureWithoutAPassedTest);
test(pictureIsDrawnWhileNoLoadFailed,pictureIsDrawnWhileNoLoadFailed);
test(failedLoadRemovesPictureAndAnotherAddressCanLoad,failedLoadRemovesPictureAndAnotherAddressCanLoad);
