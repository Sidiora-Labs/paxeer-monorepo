import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import vector from '../../../../tests/vectors/names/indexer_api_v1.json';
import type { PNSDetailedDomain, PNSDomain, PNSDomainEvent } from './pns';

const OWNER = '0x2222222222222222222222222222222222222222';
const ADDRESS = '0x1111111111111111111111111111111111111111';
const NAME = 'alice.pax';
const OWNED_PATH =
  `/api/v1/addresses:lookup?address=${OWNER}&owned_by=true&only_active=true&sort=registration_date&order=DESC` as const;
const DETAIL_PATH = '/api/v1/domains/alice.pax';
const EVENTS_PATH = '/api/v1/domains/alice.pax/events?order=DESC';
const ADDRESS_PATH = `/api/v1/addresses/${ADDRESS}` as const;
const LOOKUP_PATH = '/api/v1/domains:lookup?name=alice.pax&only_active=true';

beforeEach(() => {
  vi.stubEnv('NEXT_PUBLIC_PNS_API_BASE', undefined);
  vi.resetModules();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

async function pns_helpers_parse_the_committed_indexer_vector() {
  const fetchIndexer = vi.fn<typeof fetch>(async (input) => {
    const path = String(input).slice('/wallet/api/pns'.length);
    if (!Object.hasOwn(vector, path)) throw new Error(`Unexpected indexer path: ${path}`);
    return Response.json(vector[path as keyof typeof vector]);
  });
  vi.stubGlobal('fetch', fetchIndexer);
  const {
    fetchOwnedDomains,
    fetchDomainDetail,
    fetchDomainEvents,
    fetchAddressName,
    lookupDomain,
  } = await import('./pns');

  const owned: PNSDomain[] = await fetchOwnedDomains(OWNER);
  const detail: PNSDetailedDomain | null = await fetchDomainDetail(NAME);
  const events: PNSDomainEvent[] = await fetchDomainEvents(NAME);
  const address: PNSDetailedDomain | null = await fetchAddressName(ADDRESS);
  const matches: PNSDomain[] = await lookupDomain(NAME);

  expect(owned).toEqual(vector[OWNED_PATH].items);
  expect(detail).toEqual(vector[DETAIL_PATH]);
  expect(events).toEqual(vector[EVENTS_PATH].items);
  expect(address).toEqual(vector[ADDRESS_PATH].domain);
  expect(matches).toEqual(vector[LOOKUP_PATH].items);
  expect(owned).toHaveLength(1);
  expect(matches).toHaveLength(1);
  for (const domain of [...owned, detail, address, ...matches]) {
    expect(domain?.name).toBe(NAME);
    expect(domain?.owner?.hash).toBe(OWNER);
    expect(domain?.expiry_date).toBe('2036-07-18T13:20:00Z');
    expect(domain?.protocol?.tld_list).toEqual(['pax']);
  }
  expect(detail?.tokens).toEqual(vector[DETAIL_PATH].tokens);
  expect(address?.resolver_address).toEqual(vector[ADDRESS_PATH].domain.resolver_address);
  // The committed producer vector has eight events; the task text still says seven.
  expect(events).toHaveLength(8);
  expect(events.map((event) => event.action)).toEqual([
    'set_text',
    'transfer',
    'renew',
    'set_contenthash',
    'set_addr',
    'register',
    'register',
    'register',
  ]);
  for (let index = 1; index < events.length; index++) {
    expect(Date.parse(events[index - 1].timestamp)).toBeGreaterThanOrEqual(
      Date.parse(events[index].timestamp)
    );
  }
  await expect(fetchAddressName(OWNER)).resolves.toBeNull();
  expect(fetchIndexer.mock.calls.map(([input]) => input)).toEqual([
    `/wallet/api/pns${OWNED_PATH}`,
    `/wallet/api/pns${DETAIL_PATH}`,
    `/wallet/api/pns${EVENTS_PATH}`,
    `/wallet/api/pns${ADDRESS_PATH}`,
    `/wallet/api/pns${LOOKUP_PATH}`,
    `/wallet/api/pns/api/v1/addresses/${OWNER}`,
  ]);
}

async function pns_helpers_use_the_proxy_when_the_base_is_empty() {
  vi.stubEnv('NEXT_PUBLIC_PNS_API_BASE', '');
  const fetchIndexer = vi.fn<typeof fetch>(async () => Response.json(vector[DETAIL_PATH]));
  vi.stubGlobal('fetch', fetchIndexer);
  const { fetchDomainDetail } = await import('./pns');

  await expect(fetchDomainDetail(NAME)).resolves.toEqual(vector[DETAIL_PATH]);
  expect(fetchIndexer).toHaveBeenCalledExactlyOnceWith('/wallet/api/pns/api/v1/domains/alice.pax');
}

async function pns_helpers_return_empty_when_the_proxy_is_not_configured() {
  const fetchIndexer = vi.fn<typeof fetch>(async () =>
    Response.json({ error: { code: 'name_service_not_configured' } }, { status: 503 })
  );
  vi.stubGlobal('fetch', fetchIndexer);
  const {
    fetchOwnedDomains,
    fetchDomainDetail,
    fetchDomainEvents,
    fetchAddressName,
    lookupDomain,
  } = await import('./pns');

  await expect(fetchOwnedDomains(OWNER)).resolves.toEqual([]);
  await expect(fetchDomainDetail(NAME)).resolves.toBeNull();
  await expect(fetchDomainEvents(NAME)).resolves.toEqual([]);
  await expect(fetchAddressName(ADDRESS)).resolves.toBeNull();
  await expect(lookupDomain(NAME)).resolves.toEqual([]);
  expect(fetchIndexer.mock.calls.map(([input]) => input)).toEqual([
    `/wallet/api/pns${OWNED_PATH}`,
    `/wallet/api/pns${DETAIL_PATH}`,
    `/wallet/api/pns${EVENTS_PATH}`,
    `/wallet/api/pns${ADDRESS_PATH}`,
    `/wallet/api/pns${LOOKUP_PATH}`,
  ]);
}

test(pns_helpers_parse_the_committed_indexer_vector);
test(pns_helpers_use_the_proxy_when_the_base_is_empty);
test(pns_helpers_return_empty_when_the_proxy_is_not_configured);
