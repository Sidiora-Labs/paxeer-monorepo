import { describe, expect, it } from 'vitest';
import { CapabilityUnavailableError, HttpError, type V2ContractTypes } from '@kindle/sdk';
import { metadataApiUrl } from '@/core/sdk-config';
import recording from '@/widgets/my-token/creator-metadata.recording.json';
import { USER_SEARCH_OPERATION, checkedSearchMatches, checkedSearchMetadata, normalizedSearch, resolveSearchEntities, userSearchFailure } from './search-client';

describe('normalizedSearch', () => {
  it('lowercases a complete address and trims surrounding whitespace', () => {
    expect(normalizedSearch('  0xABCDEFabcdef0123456789ABCDEFabcdef012345 ')).toBe('0xabcdefabcdef0123456789abcdefabcdef012345');
  });
  it('keeps free text and rejects queries over 80 characters', () => {
    expect(normalizedSearch(' wired ')).toBe('wired');
    expect(() => normalizedSearch('x'.repeat(81))).toThrow(TypeError);
  });
});

describe('userSearchFailure', () => {
  it('maps the stats-workers 412 for type=profile onto an unavailable marker for the Users group', () => {
    const marker = userSearchFailure(new HttpError(412));
    expect(marker).toEqual({ state: 'unsupported', operation: USER_SEARCH_OPERATION, status: 412, notServed: true, message: 'HTTP 412' });
  });
  it('maps a missing route (404 as CapabilityUnavailableError) the same way', () => {
    const marker = userSearchFailure(new CapabilityUnavailableError('Capability unavailable', 404));
    expect(marker.state).toBe('unsupported');
    expect(marker.status).toBe(404);
    expect(marker.operation).toBe(USER_SEARCH_OPERATION);
  });
  it('rethrows every other failure so genuine errors never masquerade as unavailable', () => {
    expect(() => userSearchFailure(new HttpError(500))).toThrow(HttpError);
    expect(() => userSearchFailure(new TypeError('Failed to fetch'))).toThrow(TypeError);
  });
});

describe('checkedSearchMetadata', () => {
  const token = '0x0000000000000000000000000000000000000001';
  it('accepts a null entry and a well-formed entry keyed by the requested token', () => {
    const pool = '0x0000000000000000000000000000000000000002';
    const result = checkedSearchMetadata({ [token]: { token_address: token, pool_address: pool, name: 'Wired', symbol: 'WIRED' } }, [token]);
    expect(result[token]).toEqual({ token_address: token, pool_address: pool, name: 'Wired', symbol: 'WIRED' });
    expect(checkedSearchMetadata({ [token]: null }, [token])[token]).toBeNull();
  });
  it('rejects identities that were not requested or do not match', () => {
    const other = '0x0000000000000000000000000000000000000003';
    expect(() => checkedSearchMetadata({ [other]: null }, [token])).toThrow(TypeError);
    expect(() => checkedSearchMetadata({ [token]: { token_address: other } }, [token])).toThrow(TypeError);
  });
});

const alpha = recording[0].body;
const beta = recording[1].body;
if (!alpha.token_address || !alpha.pool_address || !beta.token_address || !beta.pool_address) {
  throw new TypeError('Coin metadata requires token and pool addresses');
}
const alphaToken = alpha.token_address;
const alphaPool = alpha.pool_address;
const betaToken = beta.token_address;
const betaPool = beta.pool_address;

function searchMatch(kind: 'pool' | 'token', token: string | null, pool: string | null): V2ContractTypes['SearchResult'] {
  return { id: (kind === 'pool' ? pool : token)!, kind, label: 'Chain label', tokenAddress: token, poolAddress: pool, wallet: null, score: '1' };
}

function search_pool_and_token_form_one_coin() {
  const logo = metadataApiUrl(`/logo/${alphaToken}.png`);
  const metadata = checkedSearchMetadata({ [alphaToken]: { ...alpha, images: { logo } } }, [alphaToken]);
  const coins = resolveSearchEntities([
    searchMatch('pool', null, alphaPool.toUpperCase().replace('0X', '0x')),
    searchMatch('token', alphaToken, null),
  ], metadata);
  expect(coins).toEqual([{
    identity: `125:pool:${alphaPool}`, chainId: 125, kind: 'pool',
    token_address: alphaToken, pool_address: alphaPool, name: alpha.name, symbol: alpha.symbol,
    images: { logo },
  }]);
}

function search_coins_keep_first_appearance_order() {
  const metadata = checkedSearchMetadata({ [alphaToken]: alpha, [betaToken]: beta }, [alphaToken, betaToken]);
  for (const matches of [
    [searchMatch('pool', null, betaPool), searchMatch('pool', null, alphaPool), searchMatch('token', alphaToken, null), searchMatch('token', betaToken, null)],
    [searchMatch('token', betaToken, null), searchMatch('token', alphaToken, null), searchMatch('pool', null, alphaPool), searchMatch('pool', null, betaPool)],
  ]) {
    expect(resolveSearchEntities(matches, metadata)).toEqual([
      { identity: `125:pool:${betaPool}`, chainId: 125, kind: 'pool', token_address: betaToken, pool_address: betaPool, name: beta.name, symbol: beta.symbol, images: { logo: null } },
      { identity: `125:pool:${alphaPool}`, chainId: 125, kind: 'pool', token_address: alphaToken, pool_address: alphaPool, name: alpha.name, symbol: alpha.symbol, images: { logo: null } },
    ]);
  }
}

function search_unresolved_token_remains_a_coin() {
  const metadata = checkedSearchMetadata({ [alphaToken]: null, [betaToken]: { ...beta, pool_address: null } }, [alphaToken, betaToken]);
  expect(resolveSearchEntities([
    searchMatch('pool', null, alphaPool),
    searchMatch('token', betaToken, null),
  ], metadata)).toEqual([
    { identity: `125:pool:${alphaPool}`, chainId: 125, kind: 'pool', token_address: undefined, pool_address: alphaPool, name: 'Chain label', symbol: '', images: { logo: null } },
    { identity: `125:token:${betaToken}`, chainId: 125, kind: 'token', token_address: betaToken, pool_address: undefined, name: beta.name, symbol: beta.symbol, images: { logo: null } },
  ]);
}

function search_token_address_returns_one_coin() {
  const q = normalizedSearch(` ${alphaToken.toUpperCase().replace('0X', '0x')} `);
  const page = (items: V2ContractTypes['SearchResult'][]): V2ContractTypes['SearchResponse'] => ({
    requestId: '11111111-1111-4111-8111-111111111111', quality: 'complete',
    generatedAt: '2026-10-13T09:00:00.000Z', revision: null, watermark: null, nextCursor: null,
    data: { items },
  });
  const pools = checkedSearchMatches(q, 'pool', page([searchMatch('pool', alphaToken, alphaPool)]));
  const tokens = checkedSearchMatches(q, 'token', page([
    searchMatch('token', alphaToken, null), searchMatch('token', alphaToken, alphaPool), searchMatch('token', betaToken, betaPool),
  ]));
  expect(pools).toEqual([]);
  expect(tokens).toHaveLength(2);
  const metadata = checkedSearchMetadata({ [alphaToken]: alpha }, [alphaToken]);
  expect(resolveSearchEntities([...pools, ...tokens], metadata)).toEqual([{
    identity: `125:token:${alphaToken}`, chainId: 125, kind: 'token', token_address: alphaToken,
    pool_address: alphaPool, name: alpha.name, symbol: alpha.symbol, images: { logo: null },
  }]);
}

function search_shared_token_keeps_pool_metadata() {
  const metadata = checkedSearchMetadata({ [alphaToken]: { ...alpha, pool_address: null } }, [alphaToken]);
  expect(resolveSearchEntities([
    searchMatch('token', alphaToken, null), searchMatch('pool', alphaToken, alphaPool),
  ], metadata)).toEqual([{
    identity: `125:pool:${alphaPool}`, chainId: 125, kind: 'pool', token_address: alphaToken,
    pool_address: alphaPool, name: alpha.name, symbol: alpha.symbol, images: { logo: null },
  }]);
  const ownMetadata = checkedSearchMetadata({ [alphaToken]: alpha, [betaToken]: { ...beta, pool_address: alphaPool } }, [alphaToken, betaToken]);
  expect(resolveSearchEntities([
    searchMatch('pool', alphaToken, alphaPool), searchMatch('token', betaToken, null),
  ], ownMetadata)).toEqual([{
    identity: `125:pool:${alphaPool}`, chainId: 125, kind: 'pool', token_address: alphaToken,
    pool_address: alphaPool, name: alpha.name, symbol: alpha.symbol, images: { logo: null },
  }]);
}

function search_linked_matches_merge_at_first_position() {
  const metadata = checkedSearchMetadata({ [alphaToken]: null, [betaToken]: beta }, [alphaToken, betaToken]);
  expect(resolveSearchEntities([
    searchMatch('token', alphaToken, null), searchMatch('pool', null, betaPool),
    searchMatch('pool', alphaToken, betaPool), searchMatch('token', betaToken, null),
  ], metadata)).toEqual([{
    identity: `125:pool:${betaPool}`, chainId: 125, kind: 'pool', token_address: alphaToken,
    pool_address: betaPool, name: beta.name, symbol: beta.symbol, images: { logo: null },
  }]);
}

describe('searchEntities coin projection', () => {
  it('keeps one pool entity with both addresses and resolved token metadata', search_pool_and_token_form_one_coin);
  it('orders two coins by their first match while preferring pool entities', search_coins_keep_first_appearance_order);
  it('keeps a token without a pool as its own entity', search_unresolved_token_remains_a_coin);
  it('filters a full token address and returns one token entity', search_token_address_returns_one_coin);
  it('matches by token address and preserves metadata held by a pool', search_shared_token_keeps_pool_metadata);
  it('merges linked pool and token matches at the earliest position', search_linked_matches_merge_at_first_position);
});
