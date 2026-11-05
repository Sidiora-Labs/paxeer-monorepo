/**
 * Wallet-level facade functions that aggregate raw Blockscout API data
 * into the shapes the Paxeer wallet app expects.
 *
 * These replace the old Portfolio API + Indexer API + RPC balance calls
 * with a single Blockscout backend as the source of truth.
 */

import { selectedWalletChainId, type WalletReadOptions } from '../../wallet-read-contract';

import {
  getAddress,
  getAddressCounters,
  getAddressTokenBalances,
  getAddressTransactions,
  getAddressTokenTransfers,
  getAddressCoinBalanceHistory,
  getAddressCoinBalanceHistoryByDay,
  getAddressNfts,
  getAddressNftCollections,
  getAddressLogs,
  getAddressTokens,
} from '../api/addresses';
import type {
  AddressTxsParams,
  AddressTokenTransfersParams,
  AddressNftParams,
} from '../api/addresses';
import { getToken, getTokens, getTokenHolders } from '../api/tokens';
import type { TokenListParams } from '../api/tokens';
import {
  getTransaction,
  getTransactionTokenTransfers,
  getTransactionLogs,
} from '../api/transactions';
import { search } from '../api/search';
import { getStats } from '../api/stats';

import type { AddressResponse, AddressCounters, CoinBalanceHistoryByDay } from '../types/address';
import type {
  Token,
  TokenBalance,
  TokenTransfer,
  TokenInstanceInList,
  NFTCollection,
  Log,
} from '../types/token';
import type { Transaction } from '../types/transaction';
import type { PaginatedResponse } from '../types/common';

// ── Portfolio types (mirrors old Portfolio API shapes) ───────────────────────

export interface WalletNativeBalance {
  symbol: string;
  balance_raw: string;
  balance: string;
  price_usd: string | null;
  value_usd: string | null;
}

export interface WalletTokenHolding {
  contract_address: string;
  symbol: string | null;
  name: string | null;
  decimals: number;
  balance_raw: string;
  balance: string;
  price_usd: string | null;
  value_usd: string | null;
  icon_url: string | null;
  token_type: string | null;
}

export interface WalletValuation {
  status: 'partial' | 'unavailable';
  spot_status: 'complete' | 'partial' | 'unavailable';
  priced_assets: number;
  total_assets: number;
  priced_value_usd: string | null;
  spot_value_usd: string | null;
  perps_status: 'unavailable';
  freshness: {
    fetched_at: string;
    price_status: 'unknown';
    prices_updated_at: null;
    native_balance_block: number | null;
  };
}

export interface WalletPortfolio {
  address: string;
  native_balance: WalletNativeBalance;
  token_holdings: WalletTokenHolding[];
  total_value_usd: string | null;
  valuation: WalletValuation;
  token_count: number;
  transaction_count: number;
  transfer_count: number;
  computed_at: string;
}

export interface WalletBalanceSummary {
  address: string;
  native_balance: string;
  native_balance_usd: string | null;
  token_balance_usd: string | null;
  perps_value_usd: null;
  total_balance_usd: string | null;
  valuation: WalletValuation;
  token_count: number;
  daily_pnl_usd: string | null;
  daily_pnl_percent: string | null;
  computed_at: string;
}

// ── Token discovery types (mirrors old PaxscanToken shape) ──────────────────

export interface WalletToken {
  address_hash: string;
  name: string;
  symbol: string;
  decimals: string;
  icon_url: string | null;
  type: string;
  exchange_rate: string | null;
  holders_count: string | null;
  total_supply: string | null;
  circulating_market_cap: string | null;
}

// ── Enriched transaction (mirrors old EnrichedTransaction shape) ─────────────

export type WalletTxType =
  | 'native_transfer'
  | 'token_transfer'
  | 'contract_call'
  | 'contract_deploy'
  | 'approval'
  | 'unknown';

export interface WalletTokenTransferItem {
  tx_hash: string;
  token_address: string;
  token_symbol: string | null;
  token_name: string | null;
  token_decimals: number | null;
  from_address: string;
  to_address: string;
  amount_raw: string;
  amount: string;
  direction: 'in' | 'out';
  token_type: string;
  block_number: number;
  timestamp: string;
  log_index: number;
}

export interface WalletTransaction {
  tx_hash: string;
  block_number: number;
  timestamp: string;
  from_address: string;
  to_address: string | null;
  value_raw: string;
  value: string;
  direction: 'in' | 'out';
  gas_used: string;
  gas_price: string;
  gas_fee: string;
  status: boolean;
  tx_type: WalletTxType;
  method: string | null;
  token_transfers: WalletTokenTransferItem[];
}

export interface WalletTransactionsResponse {
  address: string;
  transactions: WalletTransaction[];
  token_transfers: WalletTokenTransferItem[];
  next_page_params: Record<string, unknown> | null;
  next_transfer_page_params: Record<string, unknown> | null;
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function formatWei(raw: string, decimals: number): string {
  if (!raw || raw === '0') return '0';
  if (decimals === 0) return raw;
  const str = raw.padStart(decimals + 1, '0');
  const intPart = str.slice(0, str.length - decimals) || '0';
  const fracPart = str.slice(str.length - decimals).replace(/0+$/, '');
  return fracPart ? `${intPart}.${fracPart}` : intPart;
}

interface UsdAmount {
  units: bigint;
  scale: number;
}

function computeUsdAmount(
  balanceRaw: string,
  decimals: number,
  exchangeRate: string | null
): UsdAmount | null {
  if (!Number.isSafeInteger(decimals) || decimals < 0 || !/^\d+$/.test(balanceRaw)) return null;
  const match = exchangeRate?.match(/^\+?(?:(\d+)(?:\.(\d*))?|\.(\d+))(?:[eE]([+-]?\d+))?$/);
  if (!match) return null;
  const fraction = match[2] || match[3] || '';
  const exponent = Number(match[4] || '0');
  const scale = decimals + fraction.length - exponent;
  if (!Number.isSafeInteger(scale)) return null;
  return { units: BigInt(balanceRaw) * BigInt((match[1] || '0') + fraction), scale };
}

function formatUsdAmount(amount: UsdAmount | null): string | null {
  if (amount === null) return null;
  const shift = amount.scale - 2;
  const divisor = shift > 0 ? 10n ** BigInt(shift) : 1n;
  const cents =
    shift > 0 ? (amount.units + divisor / 2n) / divisor : amount.units * 10n ** BigInt(-shift);
  return `${cents / 100n}.${(cents % 100n).toString().padStart(2, '0')}`;
}

function tokenUsdAmount(balance: TokenBalance): UsdAmount | null {
  const decimals = balance.token?.decimals;
  if (decimals == null || !/^\d+$/.test(decimals)) return null;
  return computeUsdAmount(balance.value, Number(decimals), balance.token?.exchange_rate ?? null);
}

function sumUsdAmounts(amounts: UsdAmount[]): UsdAmount {
  const scale = Math.max(0, ...amounts.map((amount) => amount.scale));
  return {
    scale,
    units: amounts.reduce(
      (sum, amount) => sum + amount.units * 10n ** BigInt(scale - amount.scale),
      0n
    ),
  };
}

function completeUsdTotal(amounts: (UsdAmount | null)[]): string | null {
  return amounts.every((amount): amount is UsdAmount => amount !== null)
    ? formatUsdAmount(sumUsdAmounts(amounts))
    : null;
}

function walletValuation(
  amounts: (UsdAmount | null)[],
  fetchedAt: string,
  nativeBalanceBlock: number | null
): WalletValuation {
  const priced = amounts.filter((amount): amount is UsdAmount => amount !== null);
  return {
    status: priced.length > 0 ? 'partial' : 'unavailable',
    spot_status:
      priced.length === amounts.length ? 'complete' : priced.length > 0 ? 'partial' : 'unavailable',
    priced_assets: priced.length,
    total_assets: amounts.length,
    priced_value_usd: priced.length > 0 ? formatUsdAmount(sumUsdAmounts(priced)) : null,
    spot_value_usd: completeUsdTotal(amounts),
    perps_status: 'unavailable',
    freshness: {
      fetched_at: fetchedAt,
      price_status: 'unknown',
      prices_updated_at: null,
      native_balance_block: nativeBalanceBlock ?? null,
    },
  };
}

function inferTxType(tx: Transaction): WalletTxType {
  const types = tx.transaction_types || [];
  if (types.includes('contract_creation')) return 'contract_deploy';
  if (types.includes('token_transfer')) return 'token_transfer';
  if (types.includes('coin_transfer')) return 'native_transfer';
  if (types.includes('contract_call')) return 'contract_call';
  if (tx.method === 'approve') return 'approval';
  return 'unknown';
}

function mapTokenTransfer(tt: TokenTransfer, walletAddress: string): WalletTokenTransferItem {
  const decimals = tt.token?.decimals ? parseInt(tt.token.decimals, 10) : 18;
  const rawAmount = tt.total && 'value' in tt.total && tt.total.value ? tt.total.value || '0' : '0';
  return {
    tx_hash: tt.transaction_hash,
    token_address: tt.token?.address_hash || '',
    token_symbol: tt.token?.symbol || null,
    token_name: tt.token?.name || null,
    token_decimals: decimals,
    from_address: tt.from?.hash || '',
    to_address: tt.to?.hash || '',
    amount_raw: rawAmount,
    amount: formatWei(rawAmount, decimals),
    direction: tt.to?.hash?.toLowerCase() === walletAddress.toLowerCase() ? 'in' : 'out',
    token_type: tt.token?.type || 'ERC-20',
    block_number: tt.block_number,
    timestamp: tt.timestamp || '',
    log_index: tt.log_index,
  };
}

function mapTransaction(tx: Transaction, walletAddress: string): WalletTransaction {
  const gasUsed = tx.gas_used || '0';
  const gasPrice = tx.gas_price || '0';
  const gasFee = tx.fee?.value || '0';
  const tokenTransfers = (tx.token_transfers || []).map((tt) =>
    mapTokenTransfer(tt, walletAddress)
  );

  return {
    tx_hash: tx.hash,
    block_number: tx.block_number || 0,
    timestamp: tx.timestamp || '',
    from_address: tx.from?.hash || '',
    to_address: tx.to?.hash || null,
    value_raw: tx.value,
    value: formatWei(tx.value, 18),
    direction: tx.to?.hash?.toLowerCase() === walletAddress.toLowerCase() ? 'in' : 'out',
    gas_used: gasUsed,
    gas_price: gasPrice,
    gas_fee: gasFee,
    status: tx.status === 'ok',
    tx_type: inferTxType(tx),
    method: tx.method || null,
    token_transfers: tokenTransfers,
  };
}

// ── Facade: Portfolio ───────────────────────────────────────────────────────

export async function getWalletPortfolio(
  addressHash: string,
  options: WalletReadOptions = {}
): Promise<WalletPortfolio> {
  options = { ...options, chainId: options.chainId ?? selectedWalletChainId() };
  const [addrInfo, tokenBalances, counters] = await Promise.all([
    getAddress(addressHash, options),
    getAddressTokenBalances(addressHash, options),
    getAddressCounters(addressHash, options),
  ]);

  const nativeRaw = addrInfo.coin_balance || '0';
  const nativeRate = addrInfo.exchange_rate;
  const nativeAmount =
    addrInfo.coin_balance === null ? null : computeUsdAmount(nativeRaw, 18, nativeRate);
  const nativeValueUsd = formatUsdAmount(nativeAmount);

  const holdings: WalletTokenHolding[] = tokenBalances.map((tb) => {
    const decimals = tb.token?.decimals ? parseInt(tb.token.decimals, 10) : 18;
    return {
      contract_address: tb.token?.address_hash || '',
      symbol: tb.token?.symbol || null,
      name: tb.token?.name || null,
      decimals,
      balance_raw: tb.value,
      balance: formatWei(tb.value, decimals),
      price_usd: tb.token?.exchange_rate || null,
      value_usd: formatUsdAmount(tokenUsdAmount(tb)),
      icon_url: tb.token?.icon_url || null,
      token_type: tb.token?.type || null,
    };
  });

  const computedAt = new Date().toISOString();
  const valuation = walletValuation(
    [nativeAmount, ...tokenBalances.map(tokenUsdAmount)],
    computedAt,
    addrInfo.block_number_balance_updated_at
  );

  return {
    address: addressHash,
    native_balance: {
      symbol: 'PAX',
      balance_raw: nativeRaw,
      balance: formatWei(nativeRaw, 18),
      price_usd: nativeRate,
      value_usd: nativeValueUsd,
    },
    token_holdings: holdings,
    total_value_usd: null,
    valuation,
    token_count: tokenBalances.length,
    transaction_count: parseInt(counters.transactions_count || '0', 10),
    transfer_count: parseInt(counters.token_transfers_count || '0', 10),
    computed_at: computedAt,
  };
}

// ── Facade: Holdings ────────────────────────────────────────────────────────

export async function getWalletHoldings(
  addressHash: string,
  options: WalletReadOptions = {}
): Promise<WalletTokenHolding[]> {
  options = { ...options, chainId: options.chainId ?? selectedWalletChainId() };
  const tokenBalances = await getAddressTokenBalances(addressHash, options);
  return tokenBalances.map((tb) => {
    const decimals = tb.token?.decimals ? parseInt(tb.token.decimals, 10) : 18;
    return {
      contract_address: tb.token?.address_hash || '',
      symbol: tb.token?.symbol || null,
      name: tb.token?.name || null,
      decimals,
      balance_raw: tb.value,
      balance: formatWei(tb.value, decimals),
      price_usd: tb.token?.exchange_rate || null,
      value_usd: formatUsdAmount(tokenUsdAmount(tb)),
      icon_url: tb.token?.icon_url || null,
      token_type: tb.token?.type || null,
    };
  });
}

// ── Facade: Balance summary ─────────────────────────────────────────────────

export async function getWalletBalance(
  addressHash: string,
  options: WalletReadOptions = {}
): Promise<WalletBalanceSummary> {
  options = { ...options, chainId: options.chainId ?? selectedWalletChainId() };
  const [addrInfo, tokenBalances] = await Promise.all([
    getAddress(addressHash, options),
    getAddressTokenBalances(addressHash, options),
  ]);

  const nativeRaw = addrInfo.coin_balance || '0';
  const nativeRate = addrInfo.exchange_rate;
  const nativeAmount =
    addrInfo.coin_balance === null ? null : computeUsdAmount(nativeRaw, 18, nativeRate);
  const tokenAmounts = tokenBalances.map(tokenUsdAmount);
  const computedAt = new Date().toISOString();
  const valuation = walletValuation(
    [nativeAmount, ...tokenAmounts],
    computedAt,
    addrInfo.block_number_balance_updated_at
  );

  return {
    address: addressHash,
    native_balance: formatWei(nativeRaw, 18),
    native_balance_usd: formatUsdAmount(nativeAmount),
    token_balance_usd: completeUsdTotal(tokenAmounts),
    perps_value_usd: null,
    total_balance_usd: null,
    valuation,
    token_count: tokenBalances.length,
    daily_pnl_usd: null,
    daily_pnl_percent: null,
    computed_at: computedAt,
  };
}

// ── Facade: Transactions ────────────────────────────────────────────────────

export async function getWalletTransactions(
  addressHash: string,
  params?: AddressTxsParams,
  transferParams?: AddressTokenTransfersParams,
  options: WalletReadOptions = {}
): Promise<WalletTransactionsResponse> {
  options = { ...options, chainId: options.chainId ?? selectedWalletChainId() };
  const [txResult, transferResult] = await Promise.all([
    getAddressTransactions(addressHash, params, options),
    getAddressTokenTransfers(addressHash, transferParams, options),
  ]);
  const mappedTxs = txResult.items.map((tx) => mapTransaction(tx, addressHash));
  const mappedTransfers: WalletTokenTransferItem[] = transferResult.items.map((tt) =>
    mapTokenTransfer(tt, addressHash)
  );
  return {
    address: addressHash,
    transactions: mappedTxs,
    token_transfers: mappedTransfers,
    next_page_params: txResult.next_page_params,
    next_transfer_page_params: transferResult.next_page_params,
  };
}

// ── Facade: Token discovery (replaces fetchTopTokens / searchTokens) ────────

export async function getWalletTopTokens(limit = 20): Promise<WalletToken[]> {
  try {
    const result = await getTokens({ items_count: limit > 50 ? 50 : limit });
    return result.items.map(tokenToWalletToken);
  } catch {
    return [];
  }
}

export async function searchWalletTokens(query: string): Promise<WalletToken[]> {
  if (!query.trim()) return [];
  try {
    const result = await search(query);
    return result.items
      .filter((item) => item.type === 'token' && item.address)
      .map((item) => ({
        address_hash: item.address!,
        name: item.name || 'Unknown',
        symbol: item.symbol || '???',
        decimals: String(item.token_type === 'ERC-20' ? (item.decimals ?? 18) : 18),
        icon_url: item.icon_url || null,
        type: item.token_type || 'ERC-20',
        exchange_rate: item.exchange_rate || null,
        holders_count: item.holder_count != null ? String(item.holder_count) : null,
        total_supply: null,
        circulating_market_cap: null,
      }));
  } catch {
    return [];
  }
}

function tokenToWalletToken(t: Token): WalletToken {
  return {
    address_hash: t.address_hash,
    name: t.name,
    symbol: t.symbol,
    decimals: t.decimals || '18',
    icon_url: t.icon_url,
    type: t.type || 'ERC-20',
    exchange_rate: t.exchange_rate,
    holders_count: t.holders_count,
    total_supply: t.total_supply,
    circulating_market_cap: t.circulating_market_cap,
  };
}

// ── Facade: Token info (replaces fetchTokenInfo / fetchTokenMetadata) ───────

export interface WalletTokenMetadata {
  address: string;
  name: string | null;
  symbol: string | null;
  decimals: number | null;
  token_type: string | null;
  total_supply: string | null;
  holder_count: number | null;
  icon_url: string | null;
  exchange_rate: string | null;
  circulating_market_cap: string | null;
  volume_24h: string | null;
}

export async function getWalletTokenMetadata(tokenAddress: string): Promise<WalletTokenMetadata> {
  const t = await getToken(tokenAddress);
  return {
    address: t.address_hash,
    name: t.name,
    symbol: t.symbol,
    decimals: t.decimals ? parseInt(t.decimals, 10) : null,
    token_type: t.type,
    total_supply: t.total_supply,
    holder_count: t.holders_count ? parseInt(t.holders_count, 10) : null,
    icon_url: t.icon_url,
    exchange_rate: t.exchange_rate,
    circulating_market_cap: t.circulating_market_cap,
    volume_24h: t.volume_24h,
  };
}

// ── Facade: Tx detail (replaces fetchTxDetail / fetchTxTokenTransfers / fetchTxLogs)

export async function getWalletTxDetail(txHash: string) {
  return getTransaction(txHash);
}

export async function getWalletTxTokenTransfers(txHash: string) {
  return getTransactionTokenTransfers(txHash);
}

export async function getWalletTxLogs(txHash: string) {
  return getTransactionLogs(txHash);
}

// ── Facade: Address token balances (replaces fetchAddressTokenBalances) ─────

export async function getWalletAddressTokenBalances(
  addressHash: string,
  options?: WalletReadOptions
) {
  return getAddressTokenBalances(addressHash, options);
}

// ── Facade: Token holders (replaces fetchTokenHolders) ──────────────────────

export async function getWalletTokenHolders(tokenAddress: string) {
  return getTokenHolders(tokenAddress);
}

// ── Facade: Address counters (replaces fetchAddressCounters) ────────────────

export async function getWalletAddressCounters(addressHash: string) {
  return getAddressCounters(addressHash);
}

// ── Facade: Coin balance history ────────────────────────────────────────────

export async function getWalletCoinBalanceHistory(addressHash: string) {
  return getAddressCoinBalanceHistory(addressHash);
}

export async function getWalletCoinBalanceHistoryByDay(addressHash: string) {
  return getAddressCoinBalanceHistoryByDay(addressHash);
}

// ── Facade: NFTs ────────────────────────────────────────────────────────────

export async function getWalletNfts(addressHash: string, params?: AddressNftParams) {
  return getAddressNfts(addressHash, params);
}

export async function getWalletNftCollections(addressHash: string, params?: AddressNftParams) {
  return getAddressNftCollections(addressHash, params);
}

// ── Facade: Token transfers ─────────────────────────────────────────────────

export async function getWalletTokenTransfers(
  addressHash: string,
  params?: AddressTokenTransfersParams
) {
  return getAddressTokenTransfers(addressHash, params);
}

// ── Facade: Logs ────────────────────────────────────────────────────────────

export async function getWalletLogs(addressHash: string) {
  return getAddressLogs(addressHash);
}

// ── Facade: Chain stats ─────────────────────────────────────────────────────

export async function getWalletChainStats() {
  return getStats();
}

// ── Re-exports for convenience ──────────────────────────────────────────────

export type { AddressTxsParams, AddressTokenTransfersParams, AddressNftParams, TokenListParams };
