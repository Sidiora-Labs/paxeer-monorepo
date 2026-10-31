import {
  JsonRpcClient,
  JsonRpcTransportError,
  isRecord,
  type JsonRpcClientOptions,
} from './rpc.js';
import type {
  AssetPage,
  Balances,
  Capabilities,
  ChainAccount,
  ChainHistoryItem,
  ChainHistoryPage,
  ConsensusHead,
  HistoryItem,
  HistoryPage,
  HistoryQuery,
  JsonRpcCall,
  JsonRpcOutcome,
  NetworkHead,
  UnifiedAccountDocument,
} from './types.js';

export const DEFAULT_ENDPOINT_URL = 'https://api-mainnet-beta.paxeer.network';

export const ERC20_BALANCE_OF_SELECTOR = '0x70a08231';

const HEX_QUANTITY = /^0x[0-9a-fA-F]{1,64}$/;
const EVM_ADDRESS = /^0x[0-9a-fA-F]{40}$/;
const DECIMAL = /^(0|[1-9][0-9]*)$/;
const HISTORY_CURSOR = /^[0-9A-Za-z]{1,32}$/;

export class EndpointClient {
  readonly rpc: JsonRpcClient;

  constructor(
    options: (Omit<JsonRpcClientOptions, 'url'> & { url?: string }) | JsonRpcClient = {}
  ) {
    this.rpc =
      options instanceof JsonRpcClient
        ? options
        : new JsonRpcClient({ ...options, url: options.url ?? DEFAULT_ENDPOINT_URL });
  }

  batch(calls: readonly JsonRpcCall[]): Promise<JsonRpcOutcome[]> {
    return this.rpc.batch(calls);
  }

  async resolveAccount(account: string): Promise<UnifiedAccountDocument> {
    return decodeAccountDocument(await this.rpc.call('px_resolveAccount', [account]));
  }

  async getChainAccount(address: string): Promise<ChainAccount> {
    return decodeChainAccount(await this.rpc.call('px_getAccount', [address]), address);
  }

  async listBankAssets(): Promise<AssetPage> {
    return decodeBankAssetPage(await this.rpc.call('px_listAssets', []));
  }

  async getRouteCatalogue(): Promise<RouteCatalogue> {
    return decodeRouteCatalogue(await this.rpc.call('px_getRouteCatalogue', []));
  }

  async getCapabilities(): Promise<Capabilities> {
    return decodeCapabilities(await this.rpc.call('px_getCapabilities', []));
  }

  async getNetwork(): Promise<NetworkHead> {
    return decodeNetwork(await this.rpc.call('px_getNetwork', []));
  }

  async getBankBalances(address: string): Promise<Balances> {
    return decodeBankBalances(await this.rpc.call('px_getBalances', [address]));
  }

  async getChainHistory(
    address: string,
    cursor: string | null = null,
    query: Pick<HistoryQuery, 'limit'> = {}
  ): Promise<ChainHistoryPage> {
    if (cursor !== null && !HISTORY_CURSOR.test(cursor))
      throw new RangeError('history cursor is malformed');
    if (
      query.limit !== undefined &&
      (!Number.isInteger(query.limit) || query.limit < 1 || query.limit > 100)
    ) {
      throw new RangeError('history limit must be an integer from 1 to 100');
    }
    return decodeChainHistoryPage(
      await this.rpc.call('px_getUnifiedHistory', [address, cursor, query.limit ?? null, null])
    );
  }

  async *historyPages(
    account: string,
    query: Pick<HistoryQuery, 'limit'> = {}
  ): AsyncGenerator<ChainHistoryPage, void, undefined> {
    let cursor: string | null = null;
    const seen = new Set<string>();
    for (;;) {
      const page: ChainHistoryPage = await this.getChainHistory(account, cursor, query);
      yield page;
      if (page.next_cursor === null) return;
      if (seen.has(page.next_cursor)) throw new JsonRpcTransportError('history cursor repeated');
      seen.add(page.next_cursor);
      cursor = page.next_cursor;
    }
  }

  async *history(
    account: string,
    query: Pick<HistoryQuery, 'limit'> = {}
  ): AsyncGenerator<ChainHistoryItem, void, undefined> {
    for await (const page of this.historyPages(account, query)) {
      yield* page.items;
    }
  }
}

export function balanceOfCalldata(owner: string): `0x${string}` {
  if (!EVM_ADDRESS.test(owner)) throw new RangeError('owner is not an EVM address');
  return `${ERC20_BALANCE_OF_SELECTOR}${owner.slice(2).toLowerCase().padStart(64, '0')}`;
}

function fail(what: string): never {
  throw new JsonRpcTransportError(`malformed ${what}`);
}

function record(value: unknown, what: string): Record<string, unknown> {
  if (!isRecord(value)) fail(what);
  return value;
}

function nullableString(value: unknown, what: string): string | null {
  if (value === null) return null;
  if (typeof value !== 'string') fail(what);
  return value;
}

function decimalString(value: unknown, what: string): string {
  if (typeof value !== 'string' || !DECIMAL.test(value)) fail(what);
  return value;
}

function chainHeight(value: unknown): string | null {
  if (typeof value === 'number') {
    return Number.isSafeInteger(value) && value >= 0 ? value.toString() : null;
  }
  return typeof value === 'string' && DECIMAL.exec(value)?.[0] === value ? value : null;
}

function hexQuantity(value: unknown, what: string): `0x${string}` {
  if (typeof value !== 'string' || !HEX_QUANTITY.test(value)) fail(what);
  return value as `0x${string}`;
}

function evmAddress(value: unknown, what: string): `0x${string}` {
  if (typeof value !== 'string' || !EVM_ADDRESS.test(value)) fail(what);
  return value as `0x${string}`;
}

function side(value: unknown, what: string): 'layerx' | 'paxeer' {
  if (value !== 'layerx' && value !== 'paxeer') fail(what);
  return value;
}

export function decodeAccountDocument(value: unknown): UnifiedAccountDocument {
  const doc = record(value, 'account document');
  if (typeof doc.bound !== 'boolean') fail('account document bound');
  return {
    evm_address:
      doc.evm_address === null ? null : evmAddress(doc.evm_address, 'account evm_address'),
    pax_address: nullableString(doc.pax_address, 'account pax_address'),
    layerx_did: nullableString(doc.layerx_did, 'account layerx_did'),
    layerx_account: nullableString(doc.layerx_account, 'account layerx_account'),
    bound: doc.bound,
  };
}

function decodeChainAccount(value: unknown, address: string): ChainAccount {
  const doc = record(value, 'chain account');
  const account = decodeAccountDocument(doc.account);
  const half = record(doc.paxeer, 'chain account paxeer');
  const returnedAddress = evmAddress(half.address, 'chain account paxeer address');
  if (returnedAddress.toLowerCase() !== address.toLowerCase())
    fail('chain account paxeer address mismatch');
  const height = chainHeight(half.height);
  if (height === null) fail('chain account paxeer height');
  const text = (key: string): string => {
    const field = half[key];
    if (typeof field !== 'string' || field.length === 0) fail(`chain account paxeer ${key}`);
    return field;
  };
  return {
    account,
    paxeer: {
      address: returnedAddress,
      balance: text('balance'),
      nonce: text('nonce'),
      height,
    },
  };
}

function joinedLimit(value: unknown): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < 1) fail('joined_limit');
  return value;
}

function decodeBankAssetPage(value: unknown): AssetPage {
  const doc = record(value, 'bank asset page');
  if (typeof doc.height !== 'string' || DECIMAL.exec(doc.height)?.[0] !== doc.height)
    fail('bank asset page height');
  if (!Array.isArray(doc.assets)) fail('bank asset page assets');
  const assets = doc.assets.map((entry): AssetPage['assets'][number] => {
    const asset = record(entry, 'bank asset');
    const text = (key: string): string => {
      const field = asset[key];
      if (typeof field !== 'string') fail(`bank asset ${key}`);
      return field;
    };
    if (
      typeof asset.decimals !== 'number' ||
      !Number.isInteger(asset.decimals) ||
      asset.decimals < 0 ||
      asset.decimals > 36
    ) {
      fail('bank asset decimals');
    }
    if (typeof asset.supply !== 'string' || DECIMAL.exec(asset.supply)?.[0] !== asset.supply)
      fail('bank asset supply');
    return {
      asset_id: text('asset_id'),
      denom: text('denom'),
      kind: text('kind'),
      name: text('name'),
      symbol: text('symbol'),
      decimals: asset.decimals,
      supply: asset.supply,
    };
  });
  return { height: doc.height, assets, joined_limit: joinedLimit(doc.joined_limit) };
}

function decodeBankBalances(value: unknown): Balances {
  if (!isRecord(value)) throw new TypeError('malformed bank balances');
  let account: UnifiedAccountDocument;
  try {
    account = decodeAccountDocument(value.account);
  } catch (error) {
    if (!(error instanceof JsonRpcTransportError)) throw error;
    throw new TypeError('malformed bank balances account', { cause: error });
  }
  const height = chainHeight(value.height);
  if (height === null) throw new TypeError('malformed bank balances height');
  if (!Array.isArray(value.balances)) throw new TypeError('malformed bank balances rows');
  const balances = value.balances.map((row) => {
    if (!isRecord(row) || typeof row.denom !== 'string' || row.denom.length === 0) {
      throw new TypeError('malformed bank balance denom');
    }
    if (typeof row.amount !== 'string' || DECIMAL.exec(row.amount)?.[0] !== row.amount) {
      throw new TypeError('malformed bank balance amount');
    }
    return { denom: row.denom, amount: row.amount };
  });
  return { account, height, balances };
}

export function decodeCapabilities(value: unknown): Capabilities {
  const doc = record(value, 'capabilities');
  for (const key of ['exchange', 'bridge', 'launchpad'] as const) {
    if (typeof doc[key] !== 'boolean') fail(`capabilities ${key}`);
  }
  if (typeof doc.probed_at !== 'number' || !Number.isInteger(doc.probed_at) || doc.probed_at < 0)
    fail('capabilities probed_at');
  return {
    exchange: doc.exchange as boolean,
    bridge: doc.bridge as boolean,
    launchpad: doc.launchpad as boolean,
    probed_at: doc.probed_at,
    rpc_height: decimalString(doc.rpc_height, 'capabilities rpc_height'),
  };
}

function decodeConsensusHead(value: unknown): ConsensusHead {
  const doc = record(value, 'consensus head');
  const height = decimalString(doc.latest_height, 'consensus latest_height');
  if (height !== height.trim() || BigInt(height) > 18446744073709551615n)
    fail('consensus latest_height');
  for (const key of ['latest_block_hash', 'latest_app_hash'] as const) {
    const hash = doc[key];
    if (typeof hash !== 'string' || hash.length % 2 !== 0 || /[^0-9a-fA-F]/.test(hash))
      fail(`consensus ${key}`);
  }
  if (typeof doc.catching_up !== 'boolean') fail('consensus catching_up');
  return {
    latest_height: height,
    latest_block_hash: doc.latest_block_hash as string,
    latest_app_hash: doc.latest_app_hash as string,
    catching_up: doc.catching_up,
  };
}

export function decodeNetwork(value: unknown): NetworkHead {
  const doc = record(value, 'network head');
  if (typeof doc.network_id !== 'string') fail('network_id');
  const paxeer = record(doc.paxeer, 'network paxeer');
  const consensus = doc.consensus === null ? null : decodeConsensusHead(doc.consensus);
  if (consensus === null) {
    if (doc.reason !== 'consensus_unavailable') fail('network consensus reason');
  } else if (Object.hasOwn(doc, 'reason')) {
    fail('network consensus reason');
  }
  return {
    network_id: doc.network_id,
    paxeer: {
      chain_id: hexQuantity(paxeer.chain_id, 'network chain_id'),
      latest_block: hexQuantity(paxeer.latest_block, 'network latest_block'),
    },
    consensus,
    consensus_reason: consensus === null ? 'consensus_unavailable' : null,
  };
}

function decodeHistoryItem(value: unknown): HistoryItem {
  const item = record(value, 'history item');
  const text = (key: string): string => {
    if (typeof item[key] !== 'string') fail(`history item ${key}`);
    return item[key] as string;
  };
  if (!/^(?:explorer:)?[1-9][0-9]*$/.test(text('id'))) fail('history item id');
  if (item.direction !== 'in' && item.direction !== 'out') fail('history item direction');
  if (typeof item.final !== 'boolean') fail('history item final');
  if (!('decoded' in item)) fail('history item decoded');
  let metadata: HistoryItem['asset_metadata'] = null;
  if (item.asset_metadata !== null) {
    const meta = record(item.asset_metadata, 'history asset_metadata');
    if (typeof meta.asset !== 'string' || typeof meta.kind !== 'string')
      fail('history asset_metadata');
    const decimals = meta.decimals;
    if (
      decimals !== null &&
      (typeof decimals !== 'number' || !Number.isInteger(decimals) || decimals < 0)
    ) {
      fail('history asset_metadata decimals');
    }
    metadata = {
      asset: meta.asset,
      chain: side(meta.chain, 'history asset_metadata chain'),
      kind: meta.kind,
      address: nullableString(meta.address, 'history asset_metadata address'),
      denom: nullableString(meta.denom, 'history asset_metadata denom'),
      symbol: nullableString(meta.symbol, 'history asset_metadata symbol'),
      decimals,
      native_id: nullableString(meta.native_id, 'history asset_metadata native_id'),
      pointer: nullableString(meta.pointer, 'history asset_metadata pointer'),
      metadata: meta.metadata,
    };
  }
  return {
    id: text('id'),
    height_or_seq: decimalString(item.height_or_seq, 'history item height_or_seq'),
    chain: side(item.chain, 'history item chain'),
    kind: text('kind'),
    direction: item.direction,
    account: text('account'),
    counterparty: nullableString(item.counterparty, 'history item counterparty'),
    asset: text('asset'),
    amount: decimalString(item.amount, 'history item amount'),
    tx_id: text('tx_id'),
    ordinal: decimalString(item.ordinal, 'history item ordinal'),
    final: item.final,
    decoded: item.decoded,
    asset_metadata: metadata,
    side: side(item.side, 'history item side'),
  };
}

export function decodeHistoryPage(value: unknown): HistoryPage {
  const doc = record(value, 'history page');
  if (!Array.isArray(doc.accounts) || doc.accounts.length > 16) fail('history accounts');
  if (!Array.isArray(doc.items) || doc.items.length > 100) fail('history items');
  const accounts = doc.accounts.map((entry) => {
    const account = record(entry, 'history account');
    if (typeof account.account !== 'string') fail('history account key');
    return { side: side(account.side, 'history account side'), account: account.account };
  });
  if (!('next_cursor' in doc)) fail('history next_cursor');
  const next = nullableString(doc.next_cursor, 'history next_cursor');
  if (next !== null && !HISTORY_CURSOR.test(next)) fail('history next_cursor');
  return {
    account: decodeAccountDocument(doc.account),
    accounts,
    items: doc.items.map(decodeHistoryItem),
    next_cursor: next,
  };
}

export function decodeChainHistoryPage(value: unknown): ChainHistoryPage {
  const page = decodeHistoryPage(value);
  if (page.account.evm_address === null) fail('chain history account');
  return {
    account: page.account.evm_address,
    items: page.items
      .filter((item) => item.chain === 'paxeer')
      .map(({ chain, side, ...item }) => item),
    next_cursor: page.next_cursor,
  };
}

export interface RouteCatalogue {
  schema: 'paxeer-x.routes.v1';
  chain_id: 125;
  canonical_origin: string;
  services: ReadonlyArray<Readonly<Record<string, unknown>>>;
  routes: ReadonlyArray<Readonly<Record<string, unknown>>>;
  readiness: {
    complete: boolean;
    services: Readonly<Record<string, { ready: boolean; reason: string }>>;
  };
}

export function decodeRouteCatalogue(value: unknown): RouteCatalogue {
  const doc = record(value, 'route catalogue');
  if (
    doc.schema !== 'paxeer-x.routes.v1' ||
    doc.chain_id !== 125 ||
    typeof doc.canonical_origin !== 'string'
  )
    fail('route catalogue identity');
  const origin = new URL(doc.canonical_origin);
  if (
    origin.protocol !== 'https:' ||
    origin.origin !== doc.canonical_origin ||
    origin.username ||
    origin.password
  )
    fail('route catalogue origin');
  if (
    !Array.isArray(doc.services) ||
    doc.services.length !== 31 ||
    !Array.isArray(doc.routes) ||
    doc.routes.length > 4096
  )
    fail('route catalogue inventory');
  const ids = new Set<string>();
  const services = doc.services.map((entry) => {
    const service = record(entry, 'route service');
    if (
      typeof service.id !== 'string' ||
      ids.has(service.id) ||
      (service.exposure !== 'product' && service.exposure !== 'private')
    )
      fail('route service identity');
    ids.add(service.id);
    for (const field of [
      'discovery_path',
      'transport',
      'authentication',
      'upstream',
      'health_predicate',
      'version',
    ]) {
      if (typeof service[field] !== 'string' || !service[field]) fail('route service contract');
    }
    return Object.freeze({ ...service });
  });
  const routeIds = new Set<string>();
  const routes = doc.routes.map((entry) => {
    const route = record(entry, 'product route');
    if (
      typeof route.id !== 'string' ||
      routeIds.has(route.id) ||
      typeof route.service !== 'string' ||
      !ids.has(route.service)
    )
      fail('product route identity');
    routeIds.add(route.id);
    for (const field of [
      'method',
      'path',
      'transport',
      'authentication',
      'upstream',
      'health_predicate',
      'version',
    ]) {
      if (typeof route[field] !== 'string' || !route[field]) fail('product route contract');
    }
    if (
      typeof route.timeout_seconds !== 'number' ||
      route.timeout_seconds <= 0 ||
      route.retries !== 0
    )
      fail('product route bounds');
    return Object.freeze({ ...route });
  });
  const ready = record(doc.readiness, 'route readiness');
  const states = record(ready.services, 'service readiness');
  if (typeof ready.complete !== 'boolean' || Object.keys(states).length !== ids.size)
    fail('service readiness inventory');
  const readiness: Record<string, { ready: boolean; reason: string }> = {};
  for (const id of ids) {
    const state = record(states[id], 'service readiness state');
    if (typeof state.ready !== 'boolean' || typeof state.reason !== 'string' || !state.reason)
      fail('service readiness state');
    readiness[id] = { ready: state.ready, reason: state.reason };
  }
  if (ready.complete !== Object.values(readiness).every((state) => state.ready))
    fail('aggregate readiness');
  return {
    schema: doc.schema,
    chain_id: doc.chain_id,
    canonical_origin: doc.canonical_origin,
    services,
    routes,
    readiness: { complete: ready.complete, services: readiness },
  };
}
