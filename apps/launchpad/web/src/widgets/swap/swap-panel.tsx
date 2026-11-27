'use client';
import { launchpadGeometry } from '@/app/presentation-tokens';

import { displayCoinText } from '@/utils/coin-text';
import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useAccount, usePublicClient, useSwitchChain, useWalletClient } from 'wagmi';
import { useOptimisticReceipt } from '@/hooks/tx/use-optimistic-receipt';
import {
  useWriteRouterSwapTokenForToken,
  useWriteErc20Approve,
  useReadErc20Allowance,
  useReadErc20Balance,
  useReadErc20Decimals,
  useReadQuoterQuoteMultihop,
  ROUTER_ADDRESS,
  LAUNCHPAD_ADDRESS,
  erc20ApprovalRequest,
} from '@/core/network/contracts';
import { formatUnits, zeroAddress, erc20Abi, parseAbi, encodeFunctionData, type Address, type Hash, type TransactionReceipt } from 'viem';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { homeChainOfPool, ensureHomeChain, homeChainClient, homeChainMessage, type HomeChain } from '../trade/home-chain';
import { formatNumber, formatCurrency, formatUsdl, safeFixed } from '@/utils/format';
import type { Operations, V2ContractTypes } from '@kindle/sdk';
import { useOperations } from '@/core/market/operations';
import { reportUserFacingError } from '@/core/user-facing-error';
import { fetchStatsBatch } from '@/core/market/use-stats-batch';
import { fetchTokenMetadataBatch } from '@/core/clients/metadata';
import { Coins, SearchX, X, ArrowDownUp } from 'lucide-react';
import { Avatar, Button, EmptyState, ErrorState, IconButton, Skeleton } from '@/ui/primitives';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';
import type { TokenMetadata } from '@/widgets/home/types';
import { SlippageSelector, HighImpactWarning, computeMinOut, DEFAULT_SLIPPAGE_BPS } from '@/widgets/trade/slippage-selector';
import { useDebouncedValue } from '@/hooks/ui/use-debounced-value';
import { parseAmount } from '../trading/transaction-controls/domain';
import { transactionFingerprint } from '../trading/transaction-controls/providers';
import { amountErrorText } from '../trading/trade-panel-parts';
import { plainTradeError } from '../trading/trade-copy';
import { statsSpotPrice, statsMarketCap } from '@/core/market/pool-stats-display';
import { openAuthModal } from '@/shell/auth-modal-store';

const homeSwapAbi = parseAbi(['function swapTokenForToken(address tokenIn,address tokenOut,uint256 amountIn,uint256 minAmountOut,uint256 deadline) returns (uint256 amountOut,uint256 intermediateUsdl)']);

export function swapAllowanceRequest(token: Address, owner: Address) {
  return { token, owner, spender: LAUNCHPAD_ADDRESS } as const;
}

export interface TokenInfo {
  tokenAddress: string;
  poolAddress: string;
  name: string;
  symbol: string;
  logo: string | null;
  price: number | null;
  marketCap: number | null;
}

const SWAP_POOL_LIMIT = 100;
const STATS_BATCH_LIMIT = 50;

/** Raw integer string scaled by `decimals` to a display number (read-only presentation). */
function scaledNumber(raw: string | null, decimals: number): number {
  if (raw === null || !/^(0|[1-9][0-9]*)$/.test(raw)) return 0;
  const padded = raw.padStart(decimals + 1, '0');
  return Number(`${padded.slice(0, -decimals)}.${padded.slice(-decimals)}`);
}

/**
 * Swappable tokens from v2 routes: IF_POOL_LIST `GET /v2/pools?sort=trending`
 * (pools in trending-ranking order with their token address and reserves) and
 * IF_STATS_BATCH `POST /v2/pools/stats:batch` for market caps. The spot price is
 * the verified statistics price in USDL per token.
 */
export async function loadSwapTokens(operations: Operations, signal: AbortSignal): Promise<TokenInfo[]> {
  const pools: V2ContractTypes['Pool'][] = [];
  const cursors = new Set<string>();
  let cursor: string | null = null;
  do {
    const listed: V2ContractTypes['PoolListResponse'] = await operations.call('IF_POOL_LIST', { query: { sort: 'trending', limit: SWAP_POOL_LIMIT, ...(cursor ? { cursor } : {}) } }, { signal });
    if (listed.quality === 'quarantined' || listed.watermark?.integrity === 'quarantined') throw new Error('Pool list integrity quarantined');
    pools.push(...listed.data.items);
    cursor = listed.nextCursor;
    if (cursor && cursors.has(cursor)) throw new Error('Pool list cursor repeated');
    if (cursor) cursors.add(cursor);
  } while (cursor);
  if (pools.length === 0) return [];
  const stats = new Map<string, { price: number | null; marketCap: number | null }>();
  const addresses = pools.map(pool => pool.poolAddress.toLowerCase()).sort();
  for (let start = 0; start < addresses.length; start += STATS_BATCH_LIMIT) {
    const batch = await fetchStatsBatch(operations, addresses.slice(start, start + STATS_BATCH_LIMIT), signal);
    for (const row of batch.data.items) {
      const spot = statsSpotPrice(row).value;
      stats.set(row.poolAddress.toLowerCase(), {
        price: spot === null ? null : Number(spot),
        marketCap: statsMarketCap(row).value === null ? null : scaledNumber(statsMarketCap(row).value, 6),
      });
    }
  }
  const metaByToken = await fetchTokenMetadataBatch(pools.map(pool => pool.tokenAddress.toLowerCase()), signal);
  return pools.map(pool => {
    const poolAddress = pool.poolAddress.toLowerCase();
    const tokenAddress = pool.tokenAddress.toLowerCase();
    const meta: TokenMetadata | null = metaByToken[tokenAddress] ?? null;
    return {
      tokenAddress,
      poolAddress,
      name: displayCoinText(meta?.name, tokenAddress),
      symbol: displayCoinText(meta?.symbol, tokenAddress),
      logo: meta?.images?.logo || null,
      price: stats.get(poolAddress)?.price ?? null,
      marketCap: stats.get(poolAddress)?.marketCap ?? null,
    };
  });
}

// ── Token Selector Modal ────────────────────────────────────────

function TokenSelectorModal({
  open,
  tokens,
  balances,
  onSelect,
  onClose,
  excludeAddress,
  loading,
  error,
  onRetry,
}: {
  open: boolean;
  tokens: TokenInfo[];
  loading: boolean;
  error: string | null;
  onRetry: () => void;
  balances: Record<string, { raw: bigint; decimals: number }>;
  onSelect: (t: TokenInfo) => void;
  onClose: () => void;
  excludeAddress?: string;
}) {
  const [search, setSearch] = useState('');
  useEffect(() => {
    if (open) setSearch('');
  }, [open]);

  if (!open) return null;

  const q = search.toLowerCase();
  const filtered = tokens.filter((t) => {
    if (excludeAddress && t.tokenAddress.toLowerCase() === excludeAddress.toLowerCase()) return false;
    if (!q) return true;
    return (
      t.name.toLowerCase().includes(q) ||
      t.symbol.toLowerCase().includes(q) ||
      t.tokenAddress.toLowerCase().includes(q)
    );
  });

  return (
    <ModalDialog open={open} onClose={onClose} aria-labelledby="swap-selector-title" className="sl-floating sl-sheet max-h-[var(--sl-dialog-max-height)] max-w-modal-width-sm flex-col rounded-md">
      <div className="shrink-0 p-4">
        <div className="flex items-center justify-between mb-3">
          <h2 id="swap-selector-title" className="text-base font-semibold text-text">Select a coin</h2>
          <IconButton label="Close coin selector" icon={<X />} size="sm" onClick={onClose} />
        </div>
        <label htmlFor="swap-coin-search" className="sr-only">Search coins</label><input
          id="swap-coin-search"
          data-autofocus
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name, symbol, or address"
          className="w-full border border-control-edge bg-surface-2 rounded-sm px-[calc(var(--sl-space-3)-var(--sl-edge-width))] py-[calc(var(--sl-space)*2.5-var(--sl-edge-width))] text-xs text-text transition placeholder:text-text-faint"
        />
      </div>

      <div className="shrink-0 py-1">
        {filtered.length === 0 && (
          loading ? (
            <div role="status" className="px-4 py-2 text-text-faint text-xs"><span className="sr-only">Loading coins…</span><Skeleton variant="rows" rows={3} /></div>
          ) : error && tokens.length === 0 ? (
            <ErrorState title="Coins can't load right now" message="Something went wrong, try again" onRetry={onRetry} />
          ) : search.trim() ? (
            <EmptyState icon={SearchX} title={`No coins match "${search.trim()}"`} body="Try a different name, symbol or address." announce />
          ) : (
            <EmptyState icon={Coins} title="No coins to swap yet" body="Coins appear here as soon as they start trading." action={<Button variant="secondary" size="sm" href="/create">Create coin</Button>} announce />
          )
        )}
        {filtered.map((token) => {
          const bal = balances[token.tokenAddress.toLowerCase()];
          const balFmt = bal !== undefined ? Number(formatUnits(bal.raw, bal.decimals)) : null;
          return (
            <div key={token.tokenAddress} className="sl-hairline">
            <button
              type="button"
              onClick={() => { onSelect(token); onClose(); }}
              className="w-full flex items-center gap-3 border border-transparent px-[calc(var(--sl-space-4)-var(--sl-edge-width))] py-[calc(var(--sl-space)*2.5-var(--sl-edge-width))] hover:border-control-edge hover:bg-surface-1 transition text-left"
            >
              <Avatar src={token.logo} name={token.name} size="lg" shape="square" decorative className="size-9" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="min-w-0 truncate text-sm font-semibold text-text" title={token.name}>{token.name}</span>
                  <span className="block min-w-0 truncate text-2xs text-text" title={token.symbol}>{token.symbol}</span>
                </div>
                {token.marketCap !== null && token.marketCap > 0 && (
                  <span className="text-2xs text-text-faint">MC: {formatCurrency(token.marketCap)}</span>
                )}
              </div>
              {balFmt !== null && balFmt > 0 && (
                <span className="text-2xs text-text font-bold tabular-nums flex-shrink-0">
                  {formatNumber(balFmt, 2)}
                </span>
              )}
            </button>
            </div>
          );
        })}
      </div>
    </ModalDialog>
  );
}

// ── Token Input Box ─────────────────────────────────────────────

export function TokenInputBox({
  label,
  token,
  amount,
  onAmountChange,
  balance,
  decimals,
  readOnly,
  invalid,
  onSelectClick,
  onMaxClick,
}: {
  label: string;
  token: TokenInfo | null;
  amount: string;
  onAmountChange?: (v: string) => void;
  balance: bigint | null;
  decimals: number | undefined;
  readOnly?: boolean;
  invalid?: boolean;
  onSelectClick: () => void;
  onMaxClick?: () => void;
}) {
  const balFmt = balance !== null && decimals !== undefined ? Number(formatUnits(balance, decimals)) : null;
  const usdValue = token && token.price !== null && amount && parseFloat(amount) > 0
    ? parseFloat(amount) * token.price
    : 0;

  return (
    <div className={`rounded-sm border bg-surface-2 p-[calc(var(--sl-space-2)-var(--sl-edge-width))] transition-colors duration-base ${invalid ? 'border-danger' : 'border-control-edge'}`}>
      <span className="mb-1.5 block text-2xs font-semibold text-text-muted">{label}</span>
      <div className="flex items-center gap-2">
        <div className="relative min-w-0 flex-1">
          <input
            type="text"
            inputMode="decimal"
            aria-label={`${label} amount`}
            aria-invalid={invalid || undefined}
            value={amount}
            onChange={onAmountChange ? (e) => onAmountChange(e.target.value) : undefined}
            readOnly={readOnly}
            placeholder="0.00"
            className={`w-full min-w-0 rounded-xs bg-transparent text-md font-bold tabular-nums text-text caret-text ${readOnly ? 'cursor-default' : ''}`}
            min="0"
            step="any"
          />
        </div>
        <button
          type="button"
          onClick={onSelectClick}
          className="flex min-h-8 min-w-0 max-w-1/2 shrink-0 items-center gap-2 border border-control-edge bg-surface-3 hover:bg-surface-hover rounded-sm px-[calc(var(--sl-space-3)-var(--sl-edge-width))] py-[calc(var(--sl-space-1)-var(--sl-edge-width))] transition-colors"
        >
          {token ? (
            <>
              <Avatar src={token.logo} name={token.symbol} size="xs" shape="square" decorative />
              <span className="min-w-0 truncate text-sm font-semibold text-text" title={token.symbol}>{token.symbol}</span>
            </>
          ) : <span title="Select coin" className="min-w-0 truncate text-xs font-semibold text-text">Select coin</span>}
          <svg viewBox="0 0 12 12" fill="none" className="size-3 shrink-0 text-text-faint" aria-hidden="true">
            <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth={launchpadGeometry.iconStroke} strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>
      {balFmt !== null || usdValue > 0 ? (
        <div className="mt-1 flex flex-wrap items-center justify-between gap-x-2 text-2xs tabular-nums text-text-muted">
          {balFmt !== null ? <span className="inline-flex items-center gap-1.5">
            Balance: {formatNumber(balFmt, 4)}
            {onMaxClick && !readOnly ? <button type="button" onClick={onMaxClick} className="whitespace-nowrap rounded-xs font-semibold text-text hover:text-text-muted">{amount && parseFloat(amount) > 0 && balFmt > 0 && parseFloat(amount) >= balFmt ? 'Using Max' : 'Use Max'}</button> : null}
          </span> : null}
          {usdValue > 0 ? <span>{formatCurrency(usdValue)}</span> : null}
        </div>
      ) : null}
    </div>
  );
}

// ── Main Swap Panel ─────────────────────────────────────────────

export function readSwapQuote(raw: unknown) {
  if (!raw || typeof raw !== 'object') return null;
  const fields = ['amountOut', 'intermediateUsdl', 'sellFeeAmount', 'buyFeeAmount', 'sellPriceImpactBps', 'buyPriceImpactBps', 'combinedPriceImpactBps', 'poolA', 'poolB'] as const;
  const values = Array.isArray(raw) ? raw : fields.map(field => (raw as Record<string, unknown>)[field]);
  const [amountOut, intermediateUsdl, sellFeeAmount, buyFeeAmount, sellPriceImpactBps, buyPriceImpactBps, combinedPriceImpactBps, poolA, poolB] = values as unknown[];
  if (typeof amountOut !== 'bigint' || amountOut <= 0n || typeof intermediateUsdl !== 'bigint' || intermediateUsdl <= 0n || typeof sellFeeAmount !== 'bigint' || sellFeeAmount < 0n || typeof buyFeeAmount !== 'bigint' || buyFeeAmount < 0n || typeof sellPriceImpactBps !== 'bigint' || typeof buyPriceImpactBps !== 'bigint' || typeof combinedPriceImpactBps !== 'bigint' || typeof poolA !== 'string' || typeof poolB !== 'string' || !/^0x[0-9a-fA-F]{40}$/.test(poolA) || !/^0x[0-9a-fA-F]{40}$/.test(poolB) || poolA === zeroAddress || poolB === zeroAddress) return null;
  if ([sellPriceImpactBps, buyPriceImpactBps, combinedPriceImpactBps].some(value => value < 0n || value > 10000n)) return null;
  return { amountOut, intermediateUsdl, sellFeeAmount, buyFeeAmount, sellPriceImpactBps: Number(sellPriceImpactBps), buyPriceImpactBps: Number(buyPriceImpactBps), combinedPriceImpactBps: Number(combinedPriceImpactBps), poolA, poolB };
}

export function useSwapQuoteExecution({ tokenIn, tokenOut, poolIn, poolOut, amountIn, enabled, slippageBps }: {
  tokenIn: `0x${string}`;
  tokenOut: `0x${string}`;
  poolIn?: `0x${string}`;
  poolOut?: `0x${string}`;
  amountIn: bigint;
  enabled: boolean;
  slippageBps: number;
}) {
  const { address, chainId, isConnected } = useAccount();
  const { switchChainAsync } = useSwitchChain();
  const root = usePublicClient({ chainId: 125 });
  const { data: wallet } = useWalletClient();
  const [homeError, setHomeError] = useState<string | null>(null);
  const [homePending, setHomePending] = useState(false);
  const [home, setHome] = useState<HomeChain | null>(null);
  const [homeLoading, setHomeLoading] = useState(false);
  const [homeAttempt, setHomeAttempt] = useState(0);
  const [homeTxHash, setHomeTxHash] = useState<Hash>();
  const [homeReceipt, setHomeReceipt] = useState<TransactionReceipt>();
  const [homeApprovalReceipt, setHomeApprovalReceipt] = useState<TransactionReceipt>();
  type HomeAttempt = { home: HomeChain; hash: Hash; sender: Address; target: Address; data: `0x${string}`; action: 'approve' | 'swap' };
  const [homeRecovery, setHomeRecovery] = useState<HomeAttempt | null>(null);
  const homeChecking = useRef(false);

  const [clock, setClock] = useState(Date.now());
  const submitting = useRef(false);
  useEffect(() => {
    if (!enabled) return;
    const timer = setInterval(() => setClock(Date.now()), 500);
    return () => clearInterval(timer);
  }, [enabled]);
  const debouncedAmountIn = useDebouncedValue(amountIn, 250);
  const { data: quoteRaw, error: readQuoteError, refetch: refreshQuote, isSuccess: quoteSuccess, isFetching: quoteFetching, dataUpdatedAt: quoteUpdatedAt } = useReadQuoterQuoteMultihop(
    { tokenIn, tokenOut, amountIn: debouncedAmountIn },
    enabled && debouncedAmountIn > 0n,
  );
  const quote = useMemo(() => {
    if (!enabled || !isConnected ||
        amountIn !== debouncedAmountIn || !quoteSuccess || quoteFetching || !quoteRaw || Math.max(clock, Date.now()) - quoteUpdatedAt >= 30000) return null;
    const result = readSwapQuote(quoteRaw);
    if (!result || (poolIn && result.poolA.toLowerCase() !== poolIn.toLowerCase()) || (poolOut && result.poolB.toLowerCase() !== poolOut.toLowerCase())) return null;
    return { identity: { chainId, tokenIn, tokenOut, amountIn: debouncedAmountIn }, ...result };
  }, [quoteRaw, quoteSuccess, quoteFetching, quoteUpdatedAt, enabled, isConnected, chainId, tokenIn, tokenOut, poolIn, poolOut, amountIn, debouncedAmountIn, clock >= quoteUpdatedAt + 30000]);

  const minAmountOut = quote ? computeMinOut(quote.amountOut, slippageBps) : 0n;
  const approval = useMemo(() => quote && minAmountOut > 0n ? { quote, minAmountOut, address, slippageBps } : null,
    [quote, minAmountOut, slippageBps, address]);
  const [review, setReview] = useState<typeof approval>(null);
  const { writeContractAsync, error: writeError, reset: resetSwap, isPending, data: txHash } = useWriteRouterSwapTokenForToken();
  const latest = useRef({ approval, isConnected, isPending: isPending || homePending || !!homeRecovery, address, tokenIn, tokenOut, amountIn });
  latest.current = { approval, isConnected, isPending: isPending || homePending || !!homeRecovery, address, tokenIn, tokenOut, amountIn };

  useEffect(() => {
    if (review && review !== approval) setReview(null);
  }, [review, approval]);

  const resolveHome = async (): Promise<HomeChain | null> => {
    if (!poolIn || !poolOut) return null;
    if (!root) throw new Error('The market chain could not be read. Try again.');
    const rpc = getPaxeerRpcUrl();
    const explorer = 'https://paxscan.paxeer.app';
    const [left, right] = await Promise.all([
      homeChainOfPool(root, poolIn, rpc, explorer),
      homeChainOfPool(root, poolOut, rpc, explorer),
    ]);
    if (left.chainId !== right.chainId) throw new Error('These coins trade on different home chains.');
    return left;
  };

  useEffect(() => {
    let active = true;
    setHome(null);
    setHomeError(null);
    if (!poolIn || !poolOut) { setHomeLoading(false); return; }
    setHomeLoading(true);
    void resolveHome().then(value => { if (active) setHome(value); })
      .catch(error => { if (active) setHomeError(homeChainMessage(error)); })
      .finally(() => { if (active) setHomeLoading(false); });
    return () => { active = false; };
  }, [poolIn, poolOut, root, homeAttempt]);

  const switchToHome = async () => {
    if (submitting.current || !home) return;
    setHomeError(null);
    setHomePending(true);
    try {
      if (home.registryNumber === 0) await switchChainAsync({ chainId: 125 });
      else {
        if (!wallet) throw new Error('Connect your wallet to trade.');
        await ensureHomeChain(wallet, home);
      }
    } catch (error) { setHomeError(homeChainMessage(error)); }
    finally { setHomePending(false); }
  };

  const checkHomeReceipt = async (attempt: HomeAttempt) => {
    if (homeChecking.current) return;
    homeChecking.current = true;
    setHomeError(null);
    try {
      const client = homeChainClient(attempt.home);
      const receipt = await client.waitForTransactionReceipt({ hash: attempt.hash });
      const transaction = await client.getTransaction({ hash: receipt.transactionHash });
      const block = await client.getBlock({ blockNumber: receipt.blockNumber });
      if (await client.getChainId() !== attempt.home.chainId || block.hash !== receipt.blockHash || transaction.from.toLowerCase() !== attempt.sender.toLowerCase()) throw new Error('Transaction confirmation does not match this wallet or chain.');
      if (transactionFingerprint(transaction.to ?? transaction.from, transaction.input, transaction.value) !== transactionFingerprint(attempt.target, attempt.data)) {
        setHomeRecovery(null);
        throw new Error('Transaction cancelled or changed in your wallet. Review the trade again.');
      }
      if (attempt.action === 'approve') setHomeApprovalReceipt(receipt);
      else setHomeReceipt(receipt);
      setHomeRecovery(null);
      if (receipt.status !== 'success') throw new Error(attempt.action === 'approve' ? 'Approval was not confirmed.' : 'Swap was not confirmed.');
    } catch (error) { setHomeError(homeChainMessage(error)); }
    finally { homeChecking.current = false; }
  };

  const approveHome = async (amount: bigint) => {
    if (!wallet || !address || submitting.current || latest.current.isPending) return true;
    submitting.current = true;
    setHomePending(true);
    setHomeError(null);
    setHomeApprovalReceipt(undefined);
    try {
      const home = await resolveHome();
      if (latest.current.address !== address || latest.current.tokenIn !== tokenIn || latest.current.amountIn !== amountIn || !latest.current.isConnected) throw new Error('Wallet, coin or amount changed. Review again.');
      if (!home || home.registryNumber === 0) return false;
      const approvedAddress = address;
      const approvedToken = tokenIn;
      setHomeApprovalReceipt(undefined);
      await ensureHomeChain(wallet, home);
      if (!(await wallet.getAddresses()).some(value => value.toLowerCase() === approvedAddress.toLowerCase()) || latest.current.tokenIn !== approvedToken || latest.current.address !== approvedAddress || latest.current.amountIn !== amountIn) throw new Error('Wallet or coin changed. Review again.');
      const client = homeChainClient(home);
      const hash = await wallet.writeContract({ chain: client.chain, account: address, address: tokenIn, abi: erc20Abi, functionName: 'approve', args: [LAUNCHPAD_ADDRESS, amount] });
      const attempt: HomeAttempt = { home, hash, sender: address, target: tokenIn, data: encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [LAUNCHPAD_ADDRESS, amount] }), action: 'approve' };
      setHomeRecovery(attempt);
      await checkHomeReceipt(attempt);
      return true;
    } catch (error) {
      setHomeError(homeChainMessage(error));
      return true;
    } finally { submitting.current = false; setHomePending(false); }
  };

  const executeSwap = async (approved: NonNullable<typeof approval>) => {
    const current = latest.current;
    if (!current.isConnected || current.isPending || submitting.current || current.approval !== approved || Date.now() - quoteUpdatedAt >= 30000) return;
    submitting.current = true;
    setHomePending(true);
    resetSwap();
    setHomeReceipt(undefined);
    setHomeTxHash(undefined);
    const { identity } = approved.quote;
    const request = {
      tokenIn: identity.tokenIn,
      tokenOut: identity.tokenOut,
      amountIn: identity.amountIn,
      minAmountOut: approved.minAmountOut,
      deadline: BigInt(Math.floor(Date.now() / 1000) + 300),
    };
    setHomeError(null);
    try {
      const home = await resolveHome();
      if (latest.current.approval !== approved || !latest.current.isConnected || Date.now() - quoteUpdatedAt >= 30000) throw new Error('Trade quote changed. Review again.');
      if ((!home || home.registryNumber === 0) && chainId !== 125) throw new Error('Switch to Paxeer in your wallet to trade.');
      if (!home || home.registryNumber === 0) {
        await writeContractAsync({ chainId: 125, address: ROUTER_ADDRESS, abi: homeSwapAbi, functionName: 'swapTokenForToken', args: [request.tokenIn, request.tokenOut, request.amountIn, request.minAmountOut, request.deadline] });
        return;
      }
      if (!wallet || !address) throw new Error('Connect your wallet to trade.');
      setHomeReceipt(undefined);
      setHomeTxHash(undefined);
      await ensureHomeChain(wallet, home);
      if (latest.current.approval !== approved || !latest.current.isConnected || Date.now() - quoteUpdatedAt >= 30000) throw new Error('Trade quote changed. Review again.');
      const client = homeChainClient(home);
      const hash = await wallet.writeContract({
        account: address, chain: client.chain, address: ROUTER_ADDRESS,
        abi: homeSwapAbi, functionName: 'swapTokenForToken',
        args: [request.tokenIn, request.tokenOut, request.amountIn, request.minAmountOut, request.deadline],
      });
      setHomeTxHash(hash);
      const attempt: HomeAttempt = { home, hash, sender: address, target: ROUTER_ADDRESS, data: encodeFunctionData({ abi: homeSwapAbi, functionName: 'swapTokenForToken', args: [request.tokenIn, request.tokenOut, request.amountIn, request.minAmountOut, request.deadline] }), action: 'swap' };
      setHomeRecovery(attempt);
      await checkHomeReceipt(attempt);
    } catch (error) { setHomeError(homeChainMessage(error)); }
    finally { submitting.current = false; setHomePending(false); }
  };

  const requestSwap = () => {
    const approved = latest.current.approval;
    if (!approved || latest.current.isPending || submitting.current) return;
    if (approved.quote.combinedPriceImpactBps > 500 || approved.slippageBps > 500) setReview(approved);
    else void executeSwap(approved);
  };
  const confirmSwap = () => {
    setReview(null);
    if (review) void executeSwap(review);
  };

  return {
    quote, minAmountOut, home, homeLoading, switchToHome, retryHome: () => setHomeAttempt(value => value + 1),
    quoteLoading: enabled && !quote && (quoteFetching || amountIn !== debouncedAmountIn),
    quoteError: enabled && quote && minAmountOut === 0n ? 'This amount is too small to protect against price changes. Increase it and try again.' : enabled && !quote && !quoteFetching && amountIn === debouncedAmountIn ? (readQuoteError ? plainTradeError(readQuoteError.message) : quoteSuccess ? 'The price expired or could not be read. Refresh and try again.' : null) : null,
    refreshQuote: () => { setHomeError(null); resetSwap(); void refreshQuote(); },
    review: review === approval ? review : null,
    cancelReview: () => setReview(null),
    requestSwap, confirmSwap, approveHome, homeTxHash, homeReceipt, homeApprovalReceipt, homeError: homeError ?? (writeError ? plainTradeError(writeError.message) : null),
    homeRecovery, retryHomeReceipt: () => { if (homeRecovery) void checkHomeReceipt(homeRecovery); },
    isPending: isPending || homePending || !!homeRecovery, txHash: homeTxHash ?? txHash,
  };
}

function SwapReceiptRecovery({ recovery }: { recovery: ReturnType<typeof useOptimisticReceipt> }) {
  const [replacementHash, setReplacementHash] = useState('');
  const [error, setError] = useState<string | null>(null);
  if (recovery.state === 'cancelled' || recovery.state === 'changed_action') return <p role="alert">{recovery.state === 'cancelled' ? 'Transaction cancelled in your wallet. You can try again.' : 'Transaction replaced with a different action. Review the trade again.'}</p>;
  if (recovery.state !== 'delayed' && recovery.state !== 'quarantined' && !recovery.error) return null;
  return <div>
    <ErrorState variant="chip" wrap message="This transaction is taking longer than usual. Check its status before trading again." onRetry={recovery.retry} />
    <label>Replacement transaction ID<input value={replacementHash} onChange={event => setReplacementHash(event.target.value)} /></label>
    <Button size="sm" variant="secondary" onClick={() => { setError(null); void recovery.inspectReplacement(replacementHash as Hash).catch(caught => setError(plainTradeError(homeChainMessage(caught)))); }}>Check replacement</Button>
    {error ? <p role="alert">{error}</p> : null}
  </div>;
}

export default function SwapPanel() {
  const { address, isConnected, chainId } = useAccount();
  const [actionError, setActionError] = useState<string | null>(null);

  const [tokens, setTokens] = useState<TokenInfo[]>([]);
  const [tokensLoading, setTokensLoading] = useState(true);
  const [tokensError, setTokensError] = useState<string | null>(null);
  const [tokensAttempt, setTokensAttempt] = useState(0);
  const [tokenIn, setTokenIn] = useState<TokenInfo | null>(null);
  const [tokenOut, setTokenOut] = useState<TokenInfo | null>(null);
  const [amountIn, setAmountIn] = useState('');
  const [selectorFor, setSelectorFor] = useState<'in' | 'out' | null>(null);
  const [slippageBps, setSlippageBps] = useState(DEFAULT_SLIPPAGE_BPS);

  const operations = useOperations();
  // Swappable tokens from the v2 pool list (trending order) + stats batch.
  useEffect(() => {
    const controller = new AbortController();
    setTokensLoading(true);
    setTokensError(null);
    loadSwapTokens(operations, controller.signal)
      .then(list => { if (!controller.signal.aborted) setTokens(list); })
      .catch(err => {
        if (controller.signal.aborted) return;
        setTokensError(reportUserFacingError(err, "Couldn't load coins, try again", 'swap tokens'));
      })
      .finally(() => { if (!controller.signal.aborted) setTokensLoading(false); });
    return () => controller.abort();
  }, [operations, tokensAttempt]);

  // ── Balances ──────────────────────────────────────────────────
  const tokenInAddr = (tokenIn?.tokenAddress || zeroAddress) as `0x${string}`;
  const tokenOutAddr = (tokenOut?.tokenAddress || zeroAddress) as `0x${string}`;
  const userAddr = (address || zeroAddress) as `0x${string}`;

  // Amounts use the decimals the selected token reports on chain.
  const { data: tokenInDecRaw, error: tokenInDecError, refetch: refetchInDecimals } = useReadErc20Decimals({ token: tokenInAddr });
  const { data: tokenOutDecRaw, error: tokenOutDecError, refetch: refetchOutDecimals } = useReadErc20Decimals({ token: tokenOutAddr });
  const tokenInDec = tokenInDecRaw !== undefined && tokenInDecRaw !== null
    ? Number(tokenInDecRaw) : undefined;
  const tokenOutDec = tokenOutDecRaw !== undefined && tokenOutDecRaw !== null
    ? Number(tokenOutDecRaw) : undefined;

  const { data: balInRaw, error: balInError, refetch: refetchBalIn } = useReadErc20Balance({ token: tokenInAddr, account: userAddr });
  const { data: balOutRaw, error: balOutError, refetch: refetchBalOut } = useReadErc20Balance({ token: tokenOutAddr, account: userAddr });

  const balIn = balInRaw !== undefined && balInRaw !== null ? BigInt(String(balInRaw)) : null;
  const balOut = balOutRaw !== undefined && balOutRaw !== null ? BigInt(String(balOutRaw)) : null;

  // Build balances map for the selector (lazy — just what we know)
  const balances = useMemo(() => {
    const map: Record<string, { raw: bigint; decimals: number }> = {};
    if (tokenIn && balIn !== null && tokenInDec !== undefined) map[tokenIn.tokenAddress.toLowerCase()] = { raw: balIn, decimals: tokenInDec };
    if (tokenOut && balOut !== null && tokenOutDec !== undefined) map[tokenOut.tokenAddress.toLowerCase()] = { raw: balOut, decimals: tokenOutDec };
    return map;
  }, [tokenIn, tokenOut, balIn, balOut, tokenInDec, tokenOutDec]);

  // ── Allowance ─────────────────────────────────────────────────
  const { data: allowanceRaw, error: allowanceError, refetch: refetchAllowance } = useReadErc20Allowance(swapAllowanceRequest(tokenInAddr, userAddr));
  const allowance = allowanceRaw !== undefined && allowanceRaw !== null ? BigInt(String(allowanceRaw)) : null;

  // ── Quote ─────────────────────────────────────────────────────
  const amount = parseAmount(amountIn, tokenInDec, balIn ?? undefined);
  const parsedAmountIn = amount.parsedRaw ?? 0n;
  const hasAmount = !amount.errorCode && parsedAmountIn > 0n;
  const canQuote = isConnected && !!tokenIn && !!tokenOut && hasAmount && tokenOutDec !== undefined && tokenIn.tokenAddress !== tokenOut.tokenAddress;
  const { quote, minAmountOut, quoteLoading, quoteError, refreshQuote, review, cancelReview, requestSwap, confirmSwap,
    home, homeLoading, switchToHome, retryHome, approveHome, homeTxHash, homeReceipt, homeApprovalReceipt, homeError, homeRecovery, retryHomeReceipt, isPending: swapPending, txHash: swapTxHash } = useSwapQuoteExecution({
    tokenIn: tokenInAddr, tokenOut: tokenOutAddr, amountIn: parsedAmountIn,
    poolIn: tokenIn?.poolAddress as Address | undefined, poolOut: tokenOut?.poolAddress as Address | undefined,
    enabled: canQuote, slippageBps,
  });

  const estOut = quote && tokenOutDec !== undefined ? Number(formatUnits(quote.amountOut, tokenOutDec)) : null;
  const intermediateUsdl = quote ? parseFloat(formatUnits(quote.intermediateUsdl, 6)) : null; // USDL is always 6
  const totalFee = quote
    ? parseFloat(formatUnits(quote.sellFeeAmount + quote.buyFeeAmount, 6)) // fees in USDL
    : null;
  const combinedImpact = quote ? quote.combinedPriceImpactBps / 100 : null;

  // ── Approval state ────────────────────────────────────────────
  const needsApproval = hasAmount && allowance !== null && allowance < parsedAmountIn;
  const { writeContractAsync: writeApprove, error: approveError, reset: resetApproval, isPending: approvePending, data: approveTxHash } = useWriteErc20Approve();
  const approvalRecovery = useOptimisticReceipt(approveTxHash);
  const { receipt: rootApproveReceipt } = approvalRecovery;
  const approveReceipt = homeApprovalReceipt ?? rootApproveReceipt;
  const approveConfirming = approvePending || approvalRecovery.isPending;

  useEffect(() => {
    if (approveReceipt?.status === 'success') refetchAllowance();
  }, [approveReceipt, refetchAllowance]);

  // ── Swap execution ────────────────────────────────────────────
  // The swap trades on both legs' pools; their market.trades topics wake the receipt check.
  const swapRecovery = useOptimisticReceipt(homeTxHash ? undefined : swapTxHash, { poolAddresses: [tokenIn?.poolAddress, tokenOut?.poolAddress] });
  const { receipt: rootSwapReceipt } = swapRecovery;
  const swapReceipt = homeReceipt ?? rootSwapReceipt;

  const completedSwap = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (swapReceipt?.status === 'success' && completedSwap.current !== swapReceipt.transactionHash) {
      completedSwap.current = swapReceipt.transactionHash;
      setAmountIn('');
      refetchBalIn();
      refetchBalOut();
      refetchAllowance();
    }
  }, [swapReceipt, refetchBalIn, refetchBalOut, refetchAllowance]);

  const insufficientBalance = amount.errorCode === 'insufficient_balance';
  const isPending = swapPending || swapRecovery.isPending || approveConfirming;

  const quoteUnavailable = canQuote && (!quote || minAmountOut === 0n);

  const actionInFlight = useRef(false);
  const handleSwap = async () => {
    if (!isConnected) { openAuthModal('trade'); return; }
    if (actionInFlight.current) return;
    actionInFlight.current = true;
    setActionError(null);
    try {
      if (!home || homeLoading) return;
      if (chainId !== home.chainId) { await switchToHome(); return; }
      if (!canQuote || isPending || insufficientBalance || allowance === null || balIn === null || allowanceError || balInError || tokenInDecError || tokenOutDecError) return;
      if (needsApproval) {
        resetApproval();
        if (await approveHome(parsedAmountIn)) return;
        await writeApprove(erc20ApprovalRequest({ ...swapAllowanceRequest(tokenInAddr, userAddr), amount: parsedAmountIn }));
        return;
      }
      requestSwap();
    } catch (error) { setActionError(plainTradeError(homeChainMessage(error))); }
    finally { actionInFlight.current = false; }
  };

  // ── Side switcher ─────────────────────────────────────────────
  const handleFlip = useCallback(() => {
    setTokenIn(tokenOut);
    setTokenOut(tokenIn);
    setAmountIn('');
  }, [tokenIn, tokenOut]);

  const handleMaxIn = () => {
    if (balIn !== null && tokenInDec !== undefined) setAmountIn(formatUnits(balIn, tokenInDec));
  };

  // ── Button label ──────────────────────────────────────────────
  const buttonLabel = (() => {
    if (!isConnected) return 'Connect wallet';
    if (homeLoading) return 'Reading market chain...';
    if (home && chainId !== home.chainId) return swapPending ? 'Switching...' : 'Switch to market chain';
    if (!tokenIn) return 'Select a coin to pay with';
    if (!tokenOut) return 'Select a coin to receive';
    if (tokenIn.tokenAddress === tokenOut.tokenAddress) return 'Pick two different coins';
    if (insufficientBalance) return `Not enough ${tokenIn.symbol}`;
    if (amount.errorCode) return amountErrorText(amount.errorCode, tokenInDec ?? 0);
    if (!hasAmount) return 'Enter an amount';
    if (approveConfirming) return `Approving...`;
    if (swapPending || swapRecovery.isPending) return 'Confirming...';
    if (allowance === null || balIn === null) return 'Loading balance and allowance...';
    if (needsApproval) return `Approve ${tokenIn.symbol}`;
    if (quoteLoading) return 'Getting price...';
    return 'Swap';
  })();

  const buttonDisabled = isConnected && (!home || homeLoading || (chainId !== home.chainId ? swapPending :
    !canQuote || !hasAmount || isPending || insufficientBalance || allowance === null || balIn === null || !!allowanceError || !!balInError || !!tokenInDecError || !!tokenOutDecError || (quoteUnavailable && !needsApproval)));

  const readError = (tokenIn && (tokenInDecError || balInError || allowanceError)) || (tokenOut && (tokenOutDecError || balOutError));
  const retryReads = () => {
    void refetchInDecimals(); void refetchOutDecimals();
    void refetchBalIn(); void refetchBalOut(); void refetchAllowance();
  };

  const effectiveRate =
    estOut !== null && parseFloat(amountIn) > 0
      ? estOut / parseFloat(amountIn)
      : null;

  return (
    <div className="relative mx-auto w-full max-w-[calc(var(--sl-space)*115)]">
      <div className="rounded-md bg-surface-1 overflow-hidden">
        {/* Header */}
        <div className="px-4 pt-4 pb-3 flex items-center justify-between gap-3">
          <h1 className="shrink-0 text-md font-bold text-text">Swap</h1>
          <div className="flex min-w-0 items-center gap-2">
            <SlippageSelector bps={slippageBps} onChange={setSlippageBps} />
          <div className="hidden items-center gap-1.5 text-2xs text-text-faint sm:flex">
            <img src="/icons/ArrowsLeftRight.svg" alt="" width={launchpadGeometry.smallBadge} height={launchpadGeometry.smallBadge} className="opacity-60" />
            Token to Token
          </div>
          </div>
        </div>

        <div className="px-4 pb-4 space-y-1 relative">
          {/* Token In */}
          <TokenInputBox
            label="You pay"
            token={tokenIn}
            amount={amountIn}
            onAmountChange={setAmountIn}
            balance={isConnected ? balIn : null}
            decimals={tokenInDec}
            onSelectClick={() => setSelectorFor('in')}
            invalid={!!amount.errorCode}
            onMaxClick={handleMaxIn}
          />

          {/* Flip button */}
          <div className="flex justify-center -my-3 relative z-10">
            <IconButton label="Switch tokens" icon={<ArrowDownUp />} variant="raised" size="md" onClick={handleFlip} className="size-10" />
          </div>

          {/* Token Out */}
          <TokenInputBox
            label="You receive"
            token={tokenOut}
            amount={quote && tokenOutDec !== undefined ? formatUnits(quote.amountOut, tokenOutDec) : ''}
            balance={isConnected ? balOut : null}
            decimals={tokenOutDec}
            readOnly
            onSelectClick={() => setSelectorFor('out')}
          />

          {/* Quote details */}
          {canQuote && hasAmount && (
            <div className="p-3.5 mt-3 space-y-2 tabular-nums [&>div]:gap-2 [&>div>span:last-child]:text-right">
              {quoteLoading && !quote && (
                <div className="text-2xs text-text-faint text-center animate-pulse py-2">Getting price...</div>
              )}
              {quote && (
                <>
                  {/* Rate */}
                  {effectiveRate !== null && (
                    <div className="flex justify-between text-2xs">
                      <span className="text-text-faint">Rate</span>
                      <span className="min-w-0 truncate text-text font-bold" title={`1 ${tokenIn?.symbol} = ${formatNumber(effectiveRate, 4)} ${tokenOut?.symbol}`}>
                        1 {tokenIn?.symbol} = {formatNumber(effectiveRate, 4)} {tokenOut?.symbol}
                      </span>
                    </div>
                  )}

                  {/* Intermediate USDL */}
                  {intermediateUsdl !== null && intermediateUsdl > 0 && (
                    <div className="flex justify-between text-2xs">
                      <span className="text-text-faint">Route via USDL</span>
                      <span className="text-text">{quote ? formatUsdl(quote.intermediateUsdl, { fractionDigits: 4, compact: true, compactDigits: 4, trimCompact: false }) : '...'}</span>
                    </div>
                  )}

                  {/* Price impact */}
                  <div className="flex justify-between text-2xs">
                    <span className="text-text-faint">Price impact</span>
                    <span className={`font-bold ${
                      combinedImpact !== null && combinedImpact > 5 ? 'text-danger' :
                      combinedImpact !== null && combinedImpact > 1 ? 'text-warning' : 'text-accent'
                    }`}>
                      {combinedImpact !== null ? `${safeFixed(combinedImpact, 2)}%` : '...'}
                    </span>
                  </div>

                  {/* Per-leg impacts */}
                  <div className="flex justify-between text-2xs">
                    <span className="text-text-faint pl-2">Sell side</span>
                    <span className="text-text-faint">{safeFixed(Number(quote.sellPriceImpactBps) / 100, 2)}%</span>
                  </div>
                  <div className="flex justify-between text-2xs">
                    <span className="text-text-faint pl-2">Buy side</span>
                    <span className="text-text-faint">{safeFixed(Number(quote.buyPriceImpactBps) / 100, 2)}%</span>
                  </div>

                  {/* Total fees */}
                  <div className="flex justify-between text-2xs pt-1">
                    <span className="text-text-faint">Total fees</span>
                    <span className="text-text-faint">
                      {totalFee !== null ? formatUsdl(quote!.sellFeeAmount + quote!.buyFeeAmount, { fractionDigits: 4, compact: true, compactDigits: 4, trimCompact: false }) : '...'}
                    </span>
                  </div>

                  {/* Min received (with slippage) */}
                  <div className="flex justify-between text-2xs">
                    <span className="text-text-faint">Minimum received ({safeFixed(Number(slippageBps) / 100, 1)}% slippage)</span>
                    <span className="min-w-0 truncate text-text font-bold" title={tokenOut?.symbol}>
                      {quote && tokenOutDec !== undefined ? `${formatNumber(Number(formatUnits(minAmountOut, tokenOutDec)), 4)} ${tokenOut?.symbol}` : '...'}
                    </span>
                  </div>
                </>
              )}
            </div>
          )}

          {/* Insufficient balance warning */}
          {insufficientBalance && (
            <div className="text-danger text-2xs text-center py-1.5">
              Not enough {tokenIn?.symbol}
            </div>
          )}

          {amount.errorCode && amount.errorCode !== 'insufficient_balance' ? <p role="alert" className="text-xs text-danger">{amountErrorText(amount.errorCode, tokenInDec ?? 0)}</p> : null}
          {readError ? <ErrorState variant="chip" wrap message="Coin balances or details can't load right now." onRetry={retryReads} /> : null}
          {quoteError ? <ErrorState variant="chip" wrap message={quoteError} onRetry={refreshQuote} /> : null}
          {homeError && homeRecovery ? <ErrorState variant="chip" wrap message={plainTradeError(homeError)} onRetry={retryHomeReceipt} /> : homeError && !home ? <ErrorState variant="chip" wrap message={plainTradeError(homeError)} onRetry={retryHome} /> : homeError || actionError || approveError ? <p role="alert" className="text-xs text-danger">{homeError ? plainTradeError(homeError) : actionError ?? plainTradeError(approveError?.message)}</p> : null}
          {[approvalRecovery, swapRecovery].map((recovery, index) => <SwapReceiptRecovery key={index} recovery={recovery} />)}
          {/* Swap / Approve button */}
          <button
            type="button"
            onClick={() => void handleSwap()}
            disabled={buttonDisabled}
            className={`w-full py-3.5 rounded-sm text-base font-bold transition-colors mt-2 disabled:opacity-40 disabled:cursor-not-allowed ${
              !isConnected
                ? 'bg-surface-inverse text-canvas hover:opacity-90'
                : 'bg-accent text-on-accent hover:bg-accent-hover'
            }`}
          >
            {buttonLabel}
          </button>

          {/* Swap success */}
          {(approveReceipt?.status === 'reverted' || swapReceipt?.status === 'reverted') && (
            <p role="alert" className="text-danger text-2xs text-center">{approveReceipt?.status === 'reverted' ? "Approval didn't go through. Try again" : "Swap didn't go through. Try again"}</p>
          )}
          {swapReceipt?.status === 'success' && (
            <div className="text-accent text-2xs text-center py-1">
              Swapped!
            </div>
          )}
        </div>
      </div>

      {/* Loading state */}
      {tokensLoading && (
        <div role="status" className="absolute inset-x-0 top-full text-center text-text-faint text-2xs mt-4">
          Loading tokens...
        </div>
      )}

      {/* High price impact confirmation */}
      {review && (
        <HighImpactWarning
          priceImpactBps={review.quote.combinedPriceImpactBps}
          slippageBps={slippageBps}
          onConfirm={confirmSwap}
          onCancel={cancelReview}
        />
      )}

      {/* Token Selector Modal */}
      <TokenSelectorModal
        open={selectorFor !== null}
        tokens={tokens}
        balances={balances}
        onSelect={(t) => {
          if (selectorFor === 'in') setTokenIn(t);
          else setTokenOut(t);
          setSelectorFor(null);
        }}
        onClose={() => setSelectorFor(null)}
        excludeAddress={selectorFor === 'in' ? tokenOut?.tokenAddress : tokenIn?.tokenAddress}
        loading={tokensLoading}
        error={tokensError}
        onRetry={() => setTokensAttempt(value => value + 1)}
      />
    </div>
  );
}
