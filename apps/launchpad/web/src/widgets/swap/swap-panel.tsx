'use client';

import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useAccount, usePublicClient, useWalletClient } from 'wagmi';
import { useOptimisticReceipt } from '@/hooks/tx/use-optimistic-receipt';
import {
  useWriteRouterSwapTokenForToken,
  useWriteErc20Approve,
  useReadErc20Allowance,
  useReadErc20Balance,
  useReadErc20Decimals,
  useReadQuoterQuoteMultihop,
  ROUTER_ADDRESS,
} from '@/core/network/contracts';
import { parseUnits, formatUnits, zeroAddress, erc20Abi, parseAbi, type Address, type Hash, type TransactionReceipt } from 'viem';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { homeChainOfPool, ensureHomeChain, homeChainClient, homeChainMessage, type HomeChain } from '../trade/home-chain';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { formatNumber, formatCurrency, safeFixed } from '@/utils/format';
import type { Operations } from '@kindle/sdk';
import { useOperations } from '@/core/market/operations';
import { reportUserFacingError } from '@/core/user-facing-error';
import { fetchStatsBatch } from '@/core/market/use-stats-batch';
import { fetchTokenMetadataBatch } from '@/core/clients/metadata';
import { Coins, SearchX } from 'lucide-react';
import TokenImage from '@/ui/shared/token-image';
import { Button, EmptyState, ErrorState } from '@/ui/primitives';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';
import type { TokenMetadata } from '@/widgets/home/types';
import { SlippageSelector, HighImpactWarning, computeMinOut, DEFAULT_SLIPPAGE_BPS } from '@/widgets/trade/slippage-selector';
import { useDebouncedValue } from '@/hooks/ui/use-debounced-value';

const LAUNCHPAD_DEFAULT_DECIMALS = 6; // Safe fallback for Sidiora launchpad tokens only
const homeSwapAbi = parseAbi(['function swapTokenForToken(address tokenIn,address tokenOut,uint256 amountIn,uint256 minAmountOut,uint256 deadline) returns (uint256 amountOut,uint256 intermediateUsdl)']);

interface TokenInfo {
  tokenAddress: string;
  poolAddress: string;
  name: string;
  symbol: string;
  logo: string | null;
  price: number;
  marketCap: number;
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
 * the pool's pricing reserve ratio (USDL per token, both 6-decimal raw); a
 * failed stats batch only hides market caps.
 */
async function loadSwapTokens(operations: Operations, signal: AbortSignal): Promise<TokenInfo[]> {
  const listed = await operations.call('IF_POOL_LIST', { query: { sort: 'trending', limit: SWAP_POOL_LIMIT } }, { signal });
  if (listed.quality === 'quarantined') throw new Error('Pool list integrity quarantined');
  const pools = listed.data.items;
  if (pools.length === 0) return [];
  const caps = new Map<string, number>();
  const addresses = pools.map(pool => pool.poolAddress.toLowerCase());
  for (let start = 0; start < addresses.length; start += STATS_BATCH_LIMIT) {
    try {
      const batch = await fetchStatsBatch(operations, addresses.slice(start, start + STATS_BATCH_LIMIT), signal);
      for (const row of batch.data.items) caps.set(row.poolAddress.toLowerCase(), scaledNumber(row.totalSupplyValuationUsdlRaw, 6));
    } catch (error) {
      if (signal.aborted) throw error;
    }
  }
  const metaByToken = await fetchTokenMetadataBatch(pools.map(pool => pool.tokenAddress.toLowerCase()));
  return pools.map(pool => {
    const poolAddress = pool.poolAddress.toLowerCase();
    const tokenAddress = pool.tokenAddress.toLowerCase();
    const meta: TokenMetadata | null = metaByToken[tokenAddress] ?? null;
    const tokenReserve = scaledNumber(pool.tokenReserveRaw, LAUNCHPAD_DEFAULT_DECIMALS);
    return {
      tokenAddress,
      poolAddress,
      name: meta?.name || `Token ${poolAddress.slice(0, 6)}`,
      symbol: meta?.symbol || '???',
      logo: meta?.images?.logo || null,
      price: tokenReserve > 0 ? scaledNumber(pool.effectivePricingUsdlReserveRaw, pool.quoteAssetDecimals) / tokenReserve : 0,
      marketCap: caps.get(poolAddress) ?? 0,
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
  balances: Record<string, bigint>;
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
    <ModalDialog open={open} onClose={onClose} aria-labelledby="swap-selector-title" className="max-h-[var(--sl-dialog-max-height)] max-w-modal-width-sm flex-col rounded-xl bg-surface-1">
      <div className="shrink-0 p-4">
        <div className="flex items-center justify-between mb-3">
          <h2 id="swap-selector-title" className="text-base font-bold text-text">Select a coin</h2>
          <button type="button" onClick={onClose} aria-label="Close coin selector" className="min-h-touch-target-min min-w-touch-target-min p-1 text-text-faint transition hover:text-text">
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M4.5 4.5L13.5 13.5M13.5 4.5L4.5 13.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>
        <label htmlFor="swap-coin-search" className="sr-only">Search coins</label><input
          id="swap-coin-search"
          data-autofocus
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name, symbol, or address"
          className="w-full bg-surface-2 rounded-xl px-3 py-2.5 text-xs text-text transition placeholder:text-text-faint"
        />
      </div>

      <div className="shrink-0 py-1">
        {filtered.length === 0 && (
          loading ? (
            <div role="status" className="px-4 py-8 text-center text-text-faint text-xs animate-pulse">Loading coins…</div>
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
          const balFmt = bal !== undefined ? parseFloat(formatUnits(bal, LAUNCHPAD_DEFAULT_DECIMALS)) : 0;
          return (
            <button
              key={token.tokenAddress}
              type="button"
              onClick={() => { onSelect(token); onClose(); }}
              className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-surface-3 transition text-left"
            >
              <div className="relative w-9 h-9 rounded-full bg-surface-2 overflow-hidden flex-shrink-0">
                <TokenImage
                  fill
                  src={token.logo}
                  alt={token.symbol}
                  sizes="36px"
                  className="object-cover"
                />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-sm font-bold text-text truncate">{token.name}</span>
                  <span className="text-2xs text-text-faint uppercase">{token.symbol}</span>
                </div>
                {token.marketCap > 0 && (
                  <span className="text-2xs text-text-faint">MC: {formatCurrency(token.marketCap)}</span>
                )}
              </div>
              {balFmt > 0 && (
                <span className="text-2xs text-text font-bold flex-shrink-0">
                  {formatNumber(balFmt, 2)}
                </span>
              )}
            </button>
          );
        })}
      </div>
    </ModalDialog>
  );
}

// ── Token Input Box ─────────────────────────────────────────────

function TokenInputBox({
  label,
  token,
  amount,
  onAmountChange,
  balance,
  readOnly,
  onSelectClick,
  onMaxClick,
}: {
  label: string;
  token: TokenInfo | null;
  amount: string;
  onAmountChange?: (v: string) => void;
  balance: bigint | null;
  readOnly?: boolean;
  onSelectClick: () => void;
  onMaxClick?: () => void;
}) {
  const balFmt = balance !== null ? parseFloat(formatUnits(balance, LAUNCHPAD_DEFAULT_DECIMALS)) : 0;
  const usdValue = token && amount && parseFloat(amount) > 0
    ? parseFloat(amount) * token.price
    : 0;

  return (
    <div className="bg-surface-2 rounded-xl p-3.5 transition-all duration-base">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-1.5">
        <span className="text-2xs text-text-faint uppercase tracking-wide">{label}</span>
        {balance !== null && (
          <div className="flex items-center gap-1.5">
            <span className="text-2xs text-text-faint">Balance: {formatNumber(balFmt, 4)}</span>
            {onMaxClick && !readOnly && (
              <button
                type="button"
                onClick={onMaxClick}
                className="text-2xs text-info hover:text-text font-bold transition"
              >
                {amount && parseFloat(amount) > 0 && balFmt > 0 && parseFloat(amount) >= balFmt ? (
                  <span key="using">Using Max</span>
                ) : (
                  <span key="use">Use Max</span>
                )}
              </button>
            )}
          </div>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-24 flex-1">
          <input
            type="number"
            aria-label={`${label} amount`}
            value={amount}
            onChange={onAmountChange ? (e) => onAmountChange(e.target.value) : undefined}
            readOnly={readOnly}
            placeholder="0.00"
            className={`w-full bg-transparent text-md font-bold text-text caret-text min-w-0 ${
              readOnly ? 'cursor-default opacity-70' : ''
            }`}
            min="0"
            step="any"
          />
        </div>
        <button
          type="button"
          onClick={onSelectClick}
          className="flex items-center gap-2 bg-surface-3 hover:bg-surface-hover rounded-xl px-3 py-2 transition max-w-full flex-shrink-0"
        >
          {token ? (
            <>
              <div className="relative w-6 h-6 rounded-full bg-surface-2 overflow-hidden flex-shrink-0"
                style={{ filter: 'url(#SkiperSquiCircleFilterLayout)' }}>
                <TokenImage
                  fill
                  src={token.logo}
                  alt={token.symbol}
                  sizes="24px"
                  className="object-cover"
                />
              </div>
              <span className="text-sm font-bold text-text">{token.symbol}</span>
            </>
          ) : (
            <span className="text-xs font-bold text-text">Select coin</span>
          )}
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className="text-text-faint">
            <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>
      {usdValue > 0 ? (
        <div key="usd-value" className="mt-1">
          <span className="text-2xs text-text-faint">{formatCurrency(usdValue)}</span>
        </div>
      ) : null}
    </div>
  );
}

// ── Main Swap Panel ─────────────────────────────────────────────

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
  const root = usePublicClient({ chainId: 125 });
  const { data: wallet } = useWalletClient();
  const [homeError, setHomeError] = useState<string | null>(null);
  const [homePending, setHomePending] = useState(false);
  const [homeTxHash, setHomeTxHash] = useState<Hash>();
  const [homeReceipt, setHomeReceipt] = useState<TransactionReceipt>();
  const [homeApprovalReceipt, setHomeApprovalReceipt] = useState<TransactionReceipt>();
  const debouncedAmountIn = useDebouncedValue(amountIn, 250);
  const { data: quoteRaw, isSuccess: quoteSuccess, isFetching: quoteFetching, dataUpdatedAt: quoteUpdatedAt } = useReadQuoterQuoteMultihop(
    { tokenIn, tokenOut, amountIn: debouncedAmountIn },
    enabled && debouncedAmountIn > 0n,
  );
  const quote = useMemo(() => {
    if (!enabled || !isConnected ||
        amountIn !== debouncedAmountIn || !quoteSuccess || quoteFetching || !quoteRaw) return null;
    const q = quoteRaw as any;
    try {
      // MultihopQuoteResult tuple or struct
      if (Array.isArray(q)) {
        return {
          identity: { chainId, tokenIn, tokenOut, amountIn: debouncedAmountIn },
          amountOut: BigInt(q[0]),
          intermediateUsdl: BigInt(q[1]),
          sellFeeAmount: BigInt(q[2]),
          buyFeeAmount: BigInt(q[3]),
          sellPriceImpactBps: Number(q[4]),
          buyPriceImpactBps: Number(q[5]),
          combinedPriceImpactBps: Number(q[6]),
          poolA: String(q[7]),
          poolB: String(q[8]),
        };
      }
      return {
        identity: { chainId, tokenIn, tokenOut, amountIn: debouncedAmountIn },
        amountOut: BigInt(q.amountOut ?? 0),
        intermediateUsdl: BigInt(q.intermediateUsdl ?? 0),
        sellFeeAmount: BigInt(q.sellFeeAmount ?? 0),
        buyFeeAmount: BigInt(q.buyFeeAmount ?? 0),
        sellPriceImpactBps: Number(q.sellPriceImpactBps ?? 0),
        buyPriceImpactBps: Number(q.buyPriceImpactBps ?? 0),
        combinedPriceImpactBps: Number(q.combinedPriceImpactBps ?? 0),
        poolA: String(q.poolA ?? ''),
        poolB: String(q.poolB ?? ''),
      };
    } catch {
      return null;
    }
  }, [quoteRaw, quoteSuccess, quoteFetching, quoteUpdatedAt, enabled, isConnected, chainId, tokenIn, tokenOut, amountIn, debouncedAmountIn]);

  const minAmountOut = quote ? computeMinOut(quote.amountOut, slippageBps) : 0n;
  const approval = useMemo(() => quote ? { quote, minAmountOut, address } : null,
    [quote, minAmountOut, slippageBps, address]);
  const [review, setReview] = useState<typeof approval>(null);
  const { write, isPending, data: txHash } = useWriteRouterSwapTokenForToken();
  const latest = useRef({ approval, isConnected, isPending });
  latest.current = { approval, isConnected, isPending };

  useEffect(() => {
    if (review && review !== approval) setReview(null);
  }, [review, approval]);

  const resolveHome = async (): Promise<HomeChain | null> => {
    if (!poolIn || !poolOut || !root) return null;
    const rpc = getPaxeerRpcUrl();
    const explorer = 'https://paxscan.paxeer.app';
    const [left, right] = await Promise.all([
      homeChainOfPool(root, poolIn, rpc, explorer),
      homeChainOfPool(root, poolOut, rpc, explorer),
    ]);
    if (left.chainId !== right.chainId) throw new Error('These coins trade on different home chains.');
    return left;
  };

  const approveHome = async (amount: bigint) => {
    if (!wallet || !address) return;
    setHomeError(null);
    try {
      const home = await resolveHome();
      if (!home || home.registryNumber === 0) return false;
      setHomePending(true);
      setHomeApprovalReceipt(undefined);
      await ensureHomeChain(wallet, home);
      const client = homeChainClient(home);
      const hash = await wallet.writeContract({ chain: client.chain, account: address, address: tokenIn, abi: erc20Abi, functionName: 'approve', args: [ROUTER_ADDRESS, amount] });
      const receipt = await client.waitForTransactionReceipt({ hash });
      if (receipt.status !== 'success') throw new Error('Approval was not confirmed.');
      setHomeApprovalReceipt(receipt);
      return true;
    } catch (error) {
      setHomeError(homeChainMessage(error));
      return true;
    } finally { setHomePending(false); }
  };

  const executeSwap = async (approved: NonNullable<typeof approval>) => {
    const current = latest.current;
    if (!current.isConnected || current.isPending || current.approval !== approved) return;
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
      if (!home || home.registryNumber === 0) { write(request); return; }
      if (!wallet || !address) throw new Error('Connect your wallet to trade.');
      setHomePending(true);
      setHomeReceipt(undefined);
      await ensureHomeChain(wallet, home);
      const client = homeChainClient(home);
      const hash = await wallet.writeContract({
        account: address, chain: client.chain, address: ROUTER_ADDRESS,
        abi: homeSwapAbi, functionName: 'swapTokenForToken',
        args: [request.tokenIn, request.tokenOut, request.amountIn, request.minAmountOut, request.deadline],
      });
      setHomeTxHash(hash);
      const receipt = await client.waitForTransactionReceipt({ hash });
      if (receipt.status !== 'success') throw new Error('Swap was not confirmed.');
      setHomeReceipt(receipt);
    } catch (error) { setHomeError(homeChainMessage(error)); }
    finally { setHomePending(false); }
  };

  const requestSwap = () => {
    const approved = latest.current.approval;
    if (!approved || latest.current.isPending) return;
    if (approved.quote.combinedPriceImpactBps > 500) setReview(approved);
    else void executeSwap(approved);
  };
  const confirmSwap = () => {
    setReview(null);
    if (review) void executeSwap(review);
  };

  return {
    quote, minAmountOut,
    quoteLoading: enabled && !quote && (quoteFetching || amountIn !== debouncedAmountIn),
    review: review === approval ? review : null,
    cancelReview: () => setReview(null),
    requestSwap, confirmSwap, approveHome, homeTxHash, homeReceipt, homeApprovalReceipt, homeError,
    isPending: isPending || homePending, txHash: homeTxHash ?? txHash,
  };
}

export default function SwapPanel() {
  const { address, isConnected } = useAccount();

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

  // On-chain decimals reads (fallback to launchpad default)
  const { data: tokenInDecRaw } = useReadErc20Decimals({ token: tokenInAddr });
  const { data: tokenOutDecRaw } = useReadErc20Decimals({ token: tokenOutAddr });
  const tokenInDec = tokenInDecRaw !== undefined && tokenInDecRaw !== null
    ? Number(tokenInDecRaw) : LAUNCHPAD_DEFAULT_DECIMALS;
  const tokenOutDec = tokenOutDecRaw !== undefined && tokenOutDecRaw !== null
    ? Number(tokenOutDecRaw) : LAUNCHPAD_DEFAULT_DECIMALS;

  const { data: balInRaw, refetch: refetchBalIn } = useReadErc20Balance({ token: tokenInAddr, account: userAddr });
  const { data: balOutRaw, refetch: refetchBalOut } = useReadErc20Balance({ token: tokenOutAddr, account: userAddr });

  const balIn = balInRaw !== undefined && balInRaw !== null ? BigInt(String(balInRaw)) : null;
  const balOut = balOutRaw !== undefined && balOutRaw !== null ? BigInt(String(balOutRaw)) : null;

  // Build balances map for the selector (lazy — just what we know)
  const balances = useMemo(() => {
    const map: Record<string, bigint> = {};
    if (tokenIn && balIn !== null) map[tokenIn.tokenAddress.toLowerCase()] = balIn;
    if (tokenOut && balOut !== null) map[tokenOut.tokenAddress.toLowerCase()] = balOut;
    return map;
  }, [tokenIn, tokenOut, balIn, balOut]);

  // ── Allowance ─────────────────────────────────────────────────
  const { data: allowanceRaw, refetch: refetchAllowance } = useReadErc20Allowance({
    token: tokenInAddr,
    owner: userAddr,
    spender: ROUTER_ADDRESS,
  });
  const allowance = allowanceRaw !== undefined && allowanceRaw !== null ? BigInt(String(allowanceRaw)) : null;

  // ── Quote ─────────────────────────────────────────────────────
  const hasAmount = amountIn !== '' && parseFloat(amountIn) > 0;
  const parsedAmountIn = hasAmount ? parseUnits(amountIn, tokenInDec) : 0n;
  const canQuote = !!tokenIn && !!tokenOut && hasAmount && tokenIn.tokenAddress !== tokenOut.tokenAddress;
  const { quote, minAmountOut, quoteLoading, review, cancelReview, requestSwap, confirmSwap,
    approveHome, homeTxHash, homeReceipt, homeApprovalReceipt, homeError, isPending: swapPending, txHash: swapTxHash } = useSwapQuoteExecution({
    tokenIn: tokenInAddr, tokenOut: tokenOutAddr, amountIn: parsedAmountIn,
    poolIn: tokenIn?.poolAddress as Address | undefined, poolOut: tokenOut?.poolAddress as Address | undefined,
    enabled: canQuote, slippageBps,
  });

  const estOut = quote ? parseFloat(formatUnits(quote.amountOut, tokenOutDec)) : null;
  const intermediateUsdl = quote ? parseFloat(formatUnits(quote.intermediateUsdl, 6)) : null; // USDL is always 6
  const totalFee = quote
    ? parseFloat(formatUnits(quote.sellFeeAmount + quote.buyFeeAmount, 6)) // fees in USDL
    : null;
  const combinedImpact = quote ? quote.combinedPriceImpactBps / 100 : null;

  // ── Approval state ────────────────────────────────────────────
  const needsApproval = hasAmount && allowance !== null && allowance < parsedAmountIn;
  const { write: writeApprove, isPending: approvePending, data: approveTxHash } = useWriteErc20Approve();
  const { receipt: rootApproveReceipt } = useOptimisticReceipt(approveTxHash);
  const approveReceipt = homeApprovalReceipt ?? rootApproveReceipt;
  const approveConfirming = approvePending || (!!approveTxHash && !approveReceipt);

  useEffect(() => {
    if (approveReceipt?.status === 'success') refetchAllowance();
  }, [approveReceipt, refetchAllowance]);

  // ── Swap execution ────────────────────────────────────────────
  // The swap trades on both legs' pools; their market.trades topics wake the receipt check.
  const { receipt: rootSwapReceipt } = useOptimisticReceipt(homeTxHash ? undefined : swapTxHash, { poolAddresses: [tokenIn?.poolAddress, tokenOut?.poolAddress] });
  const swapReceipt = homeReceipt ?? rootSwapReceipt;

  useEffect(() => {
    if (swapReceipt?.status === 'success') {
      setAmountIn('');
      refetchBalIn();
      refetchBalOut();
      refetchAllowance();
    }
  }, [swapReceipt, refetchBalIn, refetchBalOut, refetchAllowance]);

  const insufficientBalance = hasAmount && balIn !== null && balIn < parsedAmountIn;
  const isPending = swapPending || (!!swapTxHash && !swapReceipt) || approveConfirming;

  const quoteUnavailable = canQuote && !quote;

  const handleSwap = async () => {
    if (!canQuote || !isConnected || isPending || insufficientBalance) return;
    if (needsApproval) {
      if (await approveHome(parsedAmountIn)) return;
      writeApprove({ token: tokenInAddr, spender: ROUTER_ADDRESS, amount: parsedAmountIn });
      return;
    }
    requestSwap();
  };

  // ── Side switcher ─────────────────────────────────────────────
  const handleFlip = useCallback(() => {
    setTokenIn(tokenOut);
    setTokenOut(tokenIn);
    setAmountIn('');
  }, [tokenIn, tokenOut]);

  const handleMaxIn = () => {
    if (balIn !== null) setAmountIn(formatUnits(balIn, tokenInDec));
  };

  // ── Button label ──────────────────────────────────────────────
  const buttonLabel = (() => {
    if (!isConnected) return 'Connect wallet';
    if (!tokenIn) return 'Select a coin to pay with';
    if (!tokenOut) return 'Select a coin to receive';
    if (tokenIn.tokenAddress === tokenOut.tokenAddress) return 'Pick two different coins';
    if (!hasAmount) return 'Enter an amount';
    if (insufficientBalance) return `Not enough ${tokenIn.symbol}`;
    if (approveConfirming) return `Approving...`;
    if (swapPending) return 'Confirming...';
    if (needsApproval) return `Approve ${tokenIn.symbol}`;
    return 'Swap';
  })();

  const buttonDisabled =
    !isConnected || !canQuote || !hasAmount || isPending || insufficientBalance || quoteUnavailable;

  const effectiveRate =
    estOut !== null && parseFloat(amountIn) > 0
      ? estOut / parseFloat(amountIn)
      : null;

  return (
    <div className="mx-auto w-full max-w-md">
      <div className="rounded-2xl bg-surface-1 overflow-hidden">
        {/* Header */}
        <div className="px-5 pt-5 pb-3 flex flex-wrap items-center justify-between gap-3">
          <h2 className="shrink-0 text-md font-bold text-text">Swap</h2>
          <div className="flex max-w-full flex-wrap items-center gap-2">
            <SlippageSelector bps={slippageBps} onChange={setSlippageBps} />
          <div className="flex items-center gap-1.5 text-2xs text-text-faint">
            <img src="/icons/ArrowsLeftRight.svg" alt="" width={14} height={14} className="opacity-60" />
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
            onSelectClick={() => setSelectorFor('in')}
            onMaxClick={handleMaxIn}
          />

          {/* Flip button */}
          <div className="flex justify-center -my-3 relative z-10">
            <button
              onClick={handleFlip}
              className="w-10 h-10 rounded-xl bg-surface-3 hover:bg-surface-hover transition flex items-center justify-center group"
              title="Switch tokens"
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 18 18"
                fill="none"
                className="text-text-muted group-hover:text-text transition group-hover:rotate-180 duration-slow"
              >
                <path d="M9 3V15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                <path d="M5 11L9 15L13 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          </div>

          {/* Token Out */}
          <TokenInputBox
            label="You receive"
            token={tokenOut}
            amount={estOut !== null ? safeFixed(estOut, 4) : ''}
            balance={isConnected ? balOut : null}
            readOnly
            onSelectClick={() => setSelectorFor('out')}
          />

          {/* Quote details */}
          {canQuote && hasAmount && (
            <div className="rounded-xl p-3.5 mt-3 space-y-2 bg-surface-2">
              {quoteLoading && !quote && (
                <div className="text-2xs text-text-faint text-center animate-pulse py-2">Getting price...</div>
              )}
              {quote && (
                <>
                  {/* Rate */}
                  {effectiveRate !== null && (
                    <div className="flex justify-between text-2xs">
                      <span className="text-text-faint">Rate</span>
                      <span className="text-text font-bold">
                        1 {tokenIn?.symbol} = {formatNumber(effectiveRate, 4)} {tokenOut?.symbol}
                      </span>
                    </div>
                  )}

                  {/* Intermediate USDL */}
                  {intermediateUsdl !== null && intermediateUsdl > 0 && (
                    <div className="flex justify-between text-2xs">
                      <span className="text-text-faint">Route via USDL</span>
                      <span className="text-text">{formatNumber(intermediateUsdl, 4)} USDL</span>
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
                      {totalFee !== null ? `${formatNumber(totalFee, 4)} USDL` : '...'}
                    </span>
                  </div>

                  {/* Min received (with slippage) */}
                  <div className="flex justify-between text-2xs">
                    <span className="text-text-faint">Minimum received ({safeFixed(Number(slippageBps) / 100, 1)}% slippage)</span>
                    <span className="text-text font-bold">
                      {quote ? `${formatNumber(parseFloat(formatUnits(minAmountOut, tokenOutDec)), 4)} ${tokenOut?.symbol}` : '...'}
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

          {homeError ? <p role="alert" className="text-xs text-danger">{homeError}</p> : null}
          {/* Swap / Approve button */}
          <button
            type="button"
            onClick={() => void handleSwap()}
            disabled={buttonDisabled}
            className={`w-full py-3.5 rounded-xl text-base font-bold transition mt-2 disabled:opacity-40 disabled:cursor-not-allowed ${
              insufficientBalance
                ? 'bg-danger-soft text-danger'
                : needsApproval
                  ? 'bg-info-soft text-info hover:bg-surface-hover'
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
        <div className="text-center text-text-faint text-2xs mt-4 animate-pulse">
          Loading tokens...
        </div>
      )}

      {/* High price impact confirmation */}
      {review && (
        <HighImpactWarning
          priceImpactBps={review.quote.combinedPriceImpactBps}
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
