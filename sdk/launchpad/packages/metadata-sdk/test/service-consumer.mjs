import assert from 'node:assert/strict';
import { createMetadataClient, MetadataHttpError } from '../src/index.ts';

const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const context = JSON.parse(Buffer.concat(chunks).toString());
const { token, poolAddress, wallet, timestamp } = context;
const client = createMetadataClient({ baseUrl: context.baseUrl });
const missing = `0x${'c'.repeat(40)}`;
const unknown = {
  token_address: missing,
  pool_address: null,
  name: null,
  symbol: null,
  decimals: 6,
  total_supply: '1000000000',
  creator: null,
  description: null,
  socials: { website: null, twitter: null, telegram: null, discord: null },
  tags: [],
  images: { logo: null, banner: null },
  created_at: null,
  updated_at: null,
};
const input = { wallet, signature: 'legacy', message: 'metadata' };
const options = { accessToken: context.accessToken };
const metadata = {
  name: 'Metadata 🙂', symbol: 'META', decimals: 18, description: 'Token description',
  website: 'https://example.com', twitter: 'https://x.com/example',
  telegram: 'https://t.me/example', discord: 'https://discord.com/example', tags: ['token', '🙂'],
};
const expected = {
  ...unknown, token_address: token, pool_address: poolAddress, creator: wallet,
  name: metadata.name, symbol: metadata.symbol, decimals: metadata.decimals,
  description: metadata.description,
  socials: { website: metadata.website, twitter: metadata.twitter, telegram: metadata.telegram, discord: metadata.discord },
  tags: metadata.tags, created_at: timestamp, updated_at: timestamp,
};
function httpError(status) {
  return error => error instanceof MetadataHttpError && error.status === status && typeof error.body === 'string';
}

async function singleReadsServiceMetadata() {
  assert.deepEqual(await client.getMetadata(missing.toUpperCase().replace('0X', '0x')), unknown);
  assert.deepEqual(await client.getMetadata(token), { ...unknown, token_address: token, pool_address: poolAddress, creator: wallet });
}

async function uploadUsesCreatorSessionAndReadsCommittedMetadata() {
  await assert.rejects(client.uploadMetadata(token, { ...input, metadata }), httpError(401));
  await assert.rejects(client.uploadMetadata(token, { ...input, metadata }, { accessToken: 'invalid' }), httpError(401));
  await assert.rejects(client.uploadMetadata(token, { ...input, metadata }, { accessToken: context.otherToken }), httpError(403));
  await assert.rejects(client.uploadMetadata(missing, { ...input, metadata }, options), httpError(404));
  await assert.rejects(client.uploadMetadata(token, input, options), httpError(400));
  for (const field of ['logo', 'banner']) {
    await assert.rejects(client.uploadMetadata(token, { ...input, [field]: new Blob([new Uint8Array(65)], { type: 'image/png' }) }, options), httpError(413));
    await assert.rejects(client.uploadMetadata(token, { ...input, [field]: new Blob(['invalid image'], { type: 'image/png' }) }, options), httpError(415));
  }
  const result = await client.uploadMetadata(token, { ...input, metadata }, options);
  assert.deepEqual(result, { success: true, metadata_updated: true, logo_url: null, banner_url: null });
  assert.deepEqual(await client.getMetadata(token), expected);
}

async function batchReadsCompleteServiceRecords() {
  assert.deepEqual(await client.getMetadataBatch([]), {});
  assert.deepEqual(await client.getMetadataBatch([token, missing, token.toUpperCase().replace('0X', '0x')]), { [token]: expected, [missing]: unknown });
  const addresses = Array.from({ length: 100 }, (_, index) => `0x${index.toString(16).padStart(40, '0')}`);
  const result = await client.getMetadataBatch(addresses);
  assert.equal(Object.keys(result).length, 100);
  for (const address of addresses) assert.deepEqual(result[address], { ...unknown, token_address: address });
}

async function unavailableAndProductionRoutesPreserveStatus() {
  const unavailable = createMetadataClient({ baseUrl: context.unavailableUrl });
  await assert.rejects(unavailable.uploadMetadata(token, { ...input, metadata }, options), httpError(503));
  const production = createMetadataClient({ baseUrl: context.productionUrl });
  assert.deepEqual(await production.getMetadataBatch([]), {});
  await assert.rejects(production.getMetadata(token), httpError(503));
  await assert.rejects(production.getMetadataBatch([token]), httpError(503));
  await assert.rejects(production.uploadMetadata(token, { ...input, metadata }, options), httpError(405));
}

await singleReadsServiceMetadata();
await uploadUsesCreatorSessionAndReadsCommittedMetadata();
await batchReadsCompleteServiceRecords();
await unavailableAndProductionRoutesPreserveStatus();
