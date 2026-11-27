import { createHash, randomUUID } from 'node:crypto';
import { decodeV2Contract } from '@kindle/sdk';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type { MetadataV2Data } from '@/core/clients/metadata';
import type { FrozenAsset } from './frozen-adapter';

export const storedPictureBytes = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1foAAAAASUVORK5CYII=', 'base64');
const installed = Symbol.for('metadata.storedPictureService');
let closeInstalled: (() => Promise<void>) | undefined;

export async function closeStoredPictureService() {
 await closeInstalled?.();
}

export async function storedPictureService(options: { token?: string; owner?: string; pending?: boolean } = {}) {
 if (installed in globalThis.fetch) throw new Error('A stored picture service is already installed.');
 const signer = privateKeyToAccount(generatePrivateKey());
 const token = options.token ?? '0x' + '1'.repeat(40);
 const owner = options.owner ?? signer.address.toLowerCase();
 const digest = createHash('sha256').update(storedPictureBytes).digest('hex');
 const asset: FrozenAsset = {
  assetId: randomUUID(), ownerWallet: owner, sha256: '0x' + digest,
  mediaType: 'image/png', bytes: storedPictureBytes.length, state: 'approved',
  publicUrl: `https://images.example/approved/${digest}/512/${'b'.repeat(64)}.png`,
 };
 let metadata: MetadataV2Data = {
  token: { address: token, name: 'Stored picture', symbol: 'PIC', decimals: 6, decimalsQuality: 'verified', totalSupplyRaw: '1000000', supplyQuality: 'verified', metadataRevision: '7', logoAssetId: null,
   sourcePosition: { chainId: '125', height: '1', blockHash: '0x'+'a'.repeat(64), transactionHash: '0x'+'b'.repeat(64), transactionIndex: '0', logIndex: '0' } },
  revision: '7', displayName: null, displaySymbol: null, description: '', tags: [], socialLinks: [],
 };
 const requests: { method: string; path: string; query: string; body: Buffer; idempotency: string | null }[] = [];
 const failures: Error[] = [];
 let publication: Record<string, unknown> | undefined;
 const envelope = (data: unknown) => ({ data, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: randomUUID(), revision: null, watermark: null });
 const origin = 'https://metadata.example';
 const previousFetch = globalThis.fetch;
 const fetchPicture: typeof fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, origin);
  if (!url.pathname.startsWith('/v2/tokens/') && !url.pathname.startsWith('/v3/')) return previousFetch(input, init);
  const request = new Request(url, input instanceof Request ? input : undefined);
  const effective = new Request(request, init);
  effective.signal.throwIfAborted();
  const send = (status: number, data: unknown) => Response.json(envelope(data), { status });
  try {
   const body = Buffer.from(await effective.arrayBuffer()), path = url.pathname;
   requests.push({ method: effective.method, path, query: url.search, body, idempotency: effective.headers.get('Idempotency-Key') });
   const assetsPath = `/v2/tokens/${token}/assets/`;
   if (effective.method === 'PUT' && path.startsWith(assetsPath)) {
    if (!body.equals(storedPictureBytes)) throw new Error('Upload bytes differ from the stored picture.');
    const data = { asset: options.pending ? { ...asset, state: 'quarantined', publicUrl: null } : asset };
    decodeV2Contract('AssetGetResponse', envelope(data));
    return send(options.pending ? 202 : 200, data);
   } else if (effective.method === 'GET' && path === assetsPath + asset.assetId) {
    decodeV2Contract('AssetGetResponse', envelope({ asset }));
    return send(200, { asset });
   } else if (effective.method === 'GET' && path === `/v2/tokens/${token}/metadata`) {
    decodeV2Contract('MetadataGetResponse', envelope(metadata));
    return send(200, metadata);
   } else if (effective.method === 'PATCH' && path === `/v2/tokens/${token}/metadata`) {
    const patch = JSON.parse(body.toString());
    if (patch.logoAssetId !== asset.assetId || patch.expectedRevision !== metadata.revision) throw new Error('Metadata must name the stored picture and current revision.');
    const revision = String(BigInt(metadata.revision) + 1n);
    metadata = { ...metadata, revision, token: { ...metadata.token, logoAssetId: asset.assetId, metadataRevision: revision } };
    decodeV2Contract('MetadataGetResponse', envelope(metadata));
    return send(200, metadata);
   } else if (effective.method === 'GET' && path === '/v3/metadata-publication-authority') {
    return send(200, { schemaVersion: 3, action: 'metadata.publish', environment: 'test', envelopeHeader: 'X-Operation-Envelope', signatureHeader: 'X-Operation-Signature', expiresAt: new Date(Date.now() + 60000).toISOString() });
   } else if (effective.method === 'POST' && path === `/v3/tokens/${token}/metadata-publications`) {
    const value = JSON.parse(body.toString());
    if (value.metadata.logoAssetId !== asset.assetId || JSON.stringify(value.uploadRefs) !== JSON.stringify([asset.assetId])) throw new Error('Publication must keep the stored picture.');
    const accepted = Date.now();
    publication = { schemaVersion: 3, operationId: value.operationId, draftId: value.draftId, tokenAddress: token, creator: owner,
     contentHash: createHash('sha256').update(JSON.stringify(value.metadata)).digest('hex'), uploadRefs: value.uploadRefs,
     status: 'published', revision: '8', attemptCount: 1, lastErrorCode: '', acceptedAt: new Date(accepted).toISOString(), expiresAt: new Date(accepted + 1800000).toISOString() };
    return send(200, publication);
   } else if (effective.method === 'GET' && path.startsWith(`/v3/tokens/${token}/metadata-publications/`)) {
    return send(publication ? 200 : 404, publication ?? null);
   } else throw new Error(`Unexpected picture request: ${effective.method} ${path}`);
  } catch (error) {
   failures.push(error instanceof Error ? error : new Error(String(error)));
   return send(500, null);
  }
 };
 Object.assign(fetchPicture, { [installed]: true });
 globalThis.fetch = fetchPicture;
 let closed = false;
 const close = async () => {
  if (closed) return;
  closed = true;
  globalThis.fetch = previousFetch;
  closeInstalled = undefined;
 };
 closeInstalled = close;
 return {
  asset, requests, failures,
  authority: { tokenAddress: token, wallet: owner, audience: origin, revision: '7', authenticationHeaders: async () => ({}), signMessage: (message: string) => signer.signMessage({ message }) },
  close,
 };
}
