// @vitest-environment jsdom
import { createServer } from 'node:http';
import { once } from 'node:events';
import { SdkClient, type V2ContractTypes } from '@kindle/sdk';
import { expect, test } from 'vitest';
import { metadataApiUrl } from '@/core/sdk-config';
import recording from '@/widgets/my-token/creator-metadata.recording.json';
import { checkedSearchMetadata, searchEntities } from './search-client';
import './use-dialog-focus.test';

const metadata = recording[0].body;
const token = metadata.token_address!;
const pool = metadata.pool_address!;
const immutableLogo = `https://metadata.example/approved/${'a'.repeat(64)}/512/${'b'.repeat(64)}.png`;

async function search_logo_projection() {
  const origin = new URL(metadataApiUrl(''));
  const requests: string[] = [];
  let logo: unknown = metadataApiUrl(`/logo/${token}.png`);
  const server = createServer((req, res) => {
    const url = new URL(req.url!, origin);
    requests.push(url.pathname + url.search);
    res.setHeader('Content-Type', 'application/json');
    if (url.pathname === '/v2/search') {
      const kind = url.searchParams.get('type');
      if (url.searchParams.get('q') !== 'Creator' || url.searchParams.get('limit') !== '10' || (kind !== 'pool' && kind !== 'token')) {
        res.writeHead(400).end();
        return;
      }
      const page: V2ContractTypes['SearchResponse'] = {
        requestId: '11111111-1111-4111-8111-111111111111', quality: 'complete',
        generatedAt: '2026-10-13T09:00:00.000Z', revision: null, watermark: null, nextCursor: null,
        data: { items: [{ id: kind === 'pool' ? pool : token, kind, label: 'Chain label',
          poolAddress: pool, tokenAddress: token, wallet: null, score: '1' }] },
      };
      res.end(JSON.stringify(page));
      return;
    }
    if (url.pathname === '/metadata/batch' && url.searchParams.get('addresses') === token) {
      res.end(JSON.stringify({ [token]: { ...metadata, images: { logo, banner: null } } }));
      return;
    }
    res.writeHead(404).end();
  });
  server.listen(Number(origin.port), origin.hostname);
  await once(server, 'listening');
  const streamOrigin = origin.origin.replace(/^http/, 'ws');
  const client = new SdkClient({ apiOrigin: origin.origin, identityOrigin: origin.origin, mediaOrigin: origin.origin,
    streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  const search = () => searchEntities('Creator', new AbortController().signal, client);
  try {
    for (const approved of [logo, immutableLogo]) {
      logo = approved;
      expect(await search()).toEqual(['pool', 'token'].map(kind => ({
        identity: `125:${kind}:${kind === 'pool' ? pool : token}`, chainId: 125, kind,
        token_address: token, pool_address: pool, name: metadata.name, symbol: metadata.symbol,
        images: { logo: approved },
      })));
    }
    // A moderated/unpublished record has no image; it must not reuse an earlier approval.
    logo = null;
    expect((await search()).map(item => item.images.logo)).toEqual([null, null]);
    for (const unapproved of [
      `${origin.origin}/quarantine/asset.png`, `${origin.origin}/logo/${pool}.png`,
      'https://untrusted.example/logo/' + token + '.png',
      'javascript:alert(1)', 'data:image/svg+xml,<svg/>',
      immutableLogo.replace('/512/', '/256/'), immutableLogo + '?download=1',
      immutableLogo + '#fragment', immutableLogo.replace('https://', 'http://'),
      immutableLogo.replace('https://', 'https://user:pass@'), 42,
    ]) {
      logo = unapproved;
      await expect(search()).rejects.toThrow(TypeError);
    }
    expect(requests).toHaveLength(42);
    for (let i = 0; i < requests.length; i += 3) {
      expect(requests.slice(i, i + 3)).toEqual([
        '/v2/search?q=Creator&type=pool&limit=10', '/v2/search?q=Creator&type=token&limit=10',
        '/metadata/batch?addresses=' + token,
      ]);
    }
  } finally {
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}
test('search_logo_projection: batch transport preserves approved logos and rejects unapproved images', search_logo_projection);

function search_logo_shape_validation() {
  for (const images of [null, 'logo', [], { logo: {} }]) {
    expect(() => checkedSearchMetadata({ [token]: { ...metadata, images } }, [token])).toThrow(TypeError);
  }
  expect(checkedSearchMetadata({ [token]: { ...metadata, images: { logo: null } } }, [token])[token]?.images?.logo).toBeNull();
}
test('search_logo_projection: malformed images are rejected by the production decoder', search_logo_shape_validation);
