'use client';

import { motion } from 'framer-motion';
import { Clock, AlertTriangle, ExternalLink, Fuel } from 'lucide-react';
import { useWalletState } from '@/providers/WalletProvider';
import { shortenAddress } from '@/lib/format';
import { PAX_ICON_URL } from '@/lib/constants';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { useTxDetailQuery } from '@/lib/queries/txDetail';
import { ethers } from 'ethers';
import Image from 'next/image';

interface TxDetailWidgetProps {
  txHash: string;
  onBack?: () => void;
  onPaxscan?: (path?: string) => void;
}

interface DetailTransfer {
  from?: { hash?: string } | null;
  to?: { hash?: string } | null;
  token?: { symbol?: string; decimals?: unknown; icon_url?: string | null } | null;
  total?: { value?: string | null } | null;
}

function tokenPrecision(value: unknown): number | null {
  if (value == null) return null;
  if (typeof value !== 'number' && (typeof value !== 'string' || !/^\d+$/.test(value))) {
    return null;
  }
  const precision = Number(value);
  // ethers.formatUnits supports integer precisions up to 80.
  return Number.isInteger(precision) && precision >= 0 && precision <= 80 ? precision : null;
}

function transferDirection(from: string | undefined, to: string | undefined, address: string) {
  if (!address) return 'unrelated';
  const sent = from?.toLowerCase() === address;
  const received = to?.toLowerCase() === address;
  return sent && received ? 'self' : sent ? 'sent' : received ? 'received' : 'unrelated';
}

function transferAmount(raw: string | null | undefined, precision: number | null): string {
  if (precision === null) return `Amount unavailable (precision unknown; raw: ${raw ?? 'unknown'})`;
  if (raw == null || !/^\d+$/.test(raw)) return 'Amount unavailable (invalid raw amount)';
  try {
    const value = ethers.formatUnits(raw, precision);
    const n = Number(value);
    if (n === 0) return '0';
    if (n < 0.0001) return '< 0.0001';
    if (n >= 1000000) return n.toLocaleString('en-US', { maximumFractionDigits: 2 });
    if (n >= 1) return n.toLocaleString('en-US', { maximumFractionDigits: 4 });
    return n.toFixed(5);
  } catch {
    return 'Amount unavailable (invalid raw amount)';
  }
}

function fmtUnits(value: string, decimals = 18) {
  try {
    return ethers.formatUnits(value || '0', decimals);
  } catch {
    return value;
  }
}

function fmtDate(ts: string) {
  try {
    const d = new Date(ts);
    return (
      d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) +
      ' at ' +
      d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
    );
  } catch {
    return ts;
  }
}

export function TxDetailWidget({ txHash, onBack, onPaxscan }: TxDetailWidgetProps) {
  const { activeAccount } = useWalletState();
  const { data, isLoading, error } = useTxDetailQuery(txHash);

  const tx = data?.tx as any;
  const tokenTransfers = (data?.tokenTransfers ?? []) as DetailTransfer[];

  const addr = activeAccount?.address?.toLowerCase() || '';
  const relevantTransfers = tokenTransfers.filter(
    (transfer) => transferDirection(transfer.from?.hash, transfer.to?.hash, addr) !== 'unrelated'
  );
  const primaryTransfer = relevantTransfers.length === 1 ? relevantTransfers[0] : undefined;
  const multipleTransfers = relevantTransfers.length > 1;
  const direction = primaryTransfer
    ? transferDirection(primaryTransfer.from?.hash, primaryTransfer.to?.hash, addr)
    : transferDirection(tx?.from?.hash, tx?.to?.hash, addr);
  const isSent = direction === 'sent';
  const symbol = primaryTransfer ? primaryTransfer.token?.symbol || '???' : 'PAX';
  const decimals = primaryTransfer ? tokenPrecision(primaryTransfer.token?.decimals) : 18;
  const rawAmount = primaryTransfer ? primaryTransfer.total?.value : tx?.value;
  const iconUrl = primaryTransfer
    ? primaryTransfer.token?.icon_url || '/wallet/default_icon.webp'
    : PAX_ICON_URL;
  const amountLabel = multipleTransfers
    ? `${relevantTransfers.length} token transfers`
    : direction === 'unrelated'
      ? 'No wallet transfer'
      : `${direction === 'self' ? '' : isSent ? '-' : '+'}${transferAmount(rawAmount, decimals)} ${symbol}`;

  const fmtFee = () => {
    if (!tx?.fee?.value) return '0 PAX';
    const n = Number(fmtUnits(tx.fee.value, 18));
    if (n === 0) return '0 PAX';
    if (n < 0.00001) return '< 0.00001 PAX';
    return `-${n.toFixed(6)} PAX`;
  };

  const statusLabel =
    tx?.status === 'ok' ? 'Succeeded' : tx?.status === 'error' ? 'Failed' : 'Pending';
  const statusColor =
    tx?.status === 'ok'
      ? 'text-pax-success'
      : tx?.status === 'error'
        ? 'text-pax-error'
        : 'text-pax-warning';
  const directionLabel = multipleTransfers
    ? 'Token transfers'
    : direction === 'self'
      ? 'Self transfer'
      : direction === 'unrelated'
        ? 'Transaction'
        : isSent
          ? 'Sent'
          : 'Received';
  const directionColor =
    multipleTransfers || direction === 'self' || direction === 'unrelated'
      ? 'text-pax-muted'
      : isSent
        ? 'text-pax-error'
        : 'text-pax-success';
  const counterparty = primaryTransfer
    ? shortenAddress(isSent ? primaryTransfer.to?.hash || '' : primaryTransfer.from?.hash || '')
    : shortenAddress(isSent ? tx?.to?.hash || '' : tx?.from?.hash || '');
  const counterpartyLabel = isSent ? 'To' : 'From';

  const isPending = tx?.status === 'pending';
  const isFailed = tx?.status === 'error';
  const revertReason =
    tx?.revert_reason ??
    tx?.decode?.parameters?.find((p: any) => p.name === 'revert_reason')?.value;

  const fmtGasUsed = () => {
    if (!tx?.gas_used) return null;
    const used = Number(tx.gas_used);
    const limit = Number(tx.gas_limit || tx.gas || 0);
    const utilPct = limit > 0 ? Math.round((used / limit) * 100) : null;
    return {
      used: used.toLocaleString(),
      limit: limit > 0 ? limit.toLocaleString() : null,
      utilPct,
    };
  };

  const fmtGasPrice = () => {
    if (!tx?.gas_price) return null;
    try {
      const gwei = Number(ethers.formatUnits(tx.gas_price, 'gwei'));
      return gwei < 1 ? `${(gwei * 1000).toFixed(2)} mGwei` : `${gwei.toFixed(2)} Gwei`;
    } catch {
      return null;
    }
  };

  const gasInfo = fmtGasUsed();
  const gasPriceLabel = fmtGasPrice();

  return (
    <div className="flex flex-col min-h-[100dvh]">
      <div className="flex-1 px-4 pt-6 pb-4">
        {isLoading ? (
          <div className="space-y-4 pt-12">
            <div className="h-8 w-24 mx-auto shimmer rounded-lg" />
            <div className="h-16 w-16 mx-auto shimmer rounded-full" />
            <div className="h-12 w-48 mx-auto shimmer rounded-lg" />
            <div className="h-48 shimmer rounded-2xl mt-8" />
          </div>
        ) : error ? (
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            className="flex flex-col items-center justify-center text-center py-16 px-4 gap-4 mt-4"
          >
            <div className="w-16 h-16 rounded-2xl bg-pax-error-muted flex items-center justify-center">
              <AlertTriangle className="w-7 h-7 text-pax-error" />
            </div>
            <div className="space-y-1">
              <p className="text-base font-semibold text-pax-light">Could not load transaction</p>
              <p className="text-sm text-pax-muted max-w-[260px] leading-relaxed">
                {(error as Error).message}
              </p>
            </div>
            {onPaxscan && (
              <button
                type="button"
                onClick={() => onPaxscan(`/tx/${txHash}`)}
                className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-pax-overlay text-sm font-medium press-scale"
              >
                <ExternalLink className="w-4 h-4" />
                Try PaxScan
              </button>
            )}
          </motion.div>
        ) : tx ? (
          <>
            <div className="text-center pt-4 pb-6">
              <p className={`text-lg font-semibold mb-4 ${directionColor}`}>{directionLabel}</p>
              <div className="relative w-16 h-16 mx-auto mb-5">
                <Image
                  src={iconUrl}
                  alt={symbol}
                  className="w-16 h-16 rounded-full bg-pax-overlay object-cover"
                  onError={(e) => {
                    (e.target as HTMLImageElement).src = '/wallet/default_icon.webp';
                  }}
                  width={64}
                  height={64}
                />
                <div
                  className={`absolute -bottom-1 -right-1 w-7 h-7 rounded-full flex items-center justify-center ${isSent ? 'bg-pax-error' : 'bg-pax-success'}`}
                >
                  <SvgIcon
                    name={isSent ? 'arrow-up-right' : 'arrow-down-left'}
                    className="w-3.5 h-3.5"
                  />
                </div>
              </div>
              <p className={`text-3xl font-bold tracking-tight ${directionColor}`}>{amountLabel}</p>
            </div>

            {/* Pending banner */}
            {isPending && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                className="flex items-center gap-3 mb-4 px-4 py-3 rounded-xl bg-pax-warning-muted"
              >
                <Clock className="w-4 h-4 text-pax-warning shrink-0" />
                <div className="flex-1">
                  <p className="text-xs font-semibold text-pax-warning">Transaction Pending</p>
                  <p className="text-xs text-pax-warning/80 mt-0.5">
                    Waiting for confirmation on the network
                  </p>
                </div>
              </motion.div>
            )}

            {/* Failed revert reason */}
            {isFailed && revertReason && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                className="mb-4 px-4 py-3 rounded-xl bg-pax-error-muted  "
              >
                <p className="text-xs font-semibold text-pax-error mb-1">Revert reason</p>
                <p className="text-xs text-pax-error font-mono break-all leading-relaxed">
                  {revertReason}
                </p>
              </motion.div>
            )}

            <div className="glass-card  divide-pax-line overflow-hidden">
              <DetailRow label="Date" value={tx.timestamp ? fmtDate(tx.timestamp) : '—'} />
              <DetailRow label="Status" value={statusLabel} valueClass={statusColor} />
              {!multipleTransfers && direction !== 'unrelated' && (
                <DetailRow label={counterpartyLabel} value={counterparty} mono />
              )}
              <DetailRow label="Network" value="Paxeer" />
              <DetailRow label="Network Fee" value={fmtFee()} />
              <button
                type="button"
                onClick={() => onPaxscan?.(`/tx/${tx.hash}`)}
                className="w-full py-3 text-center text-sm font-medium text-pax-accent hover:bg-pax-control transition-colors"
              >
                View on PaxScan
              </button>
            </div>

            {/* Gas breakdown */}
            {(gasInfo || gasPriceLabel) && (
              <div className="glass-card mt-4 overflow-hidden">
                <div className="flex items-center gap-2 px-4 py-3  ">
                  <Fuel className="w-3.5 h-3.5 text-pax-muted" />
                  <p className="text-xs font-semibold text-pax-muted">Gas Breakdown</p>
                </div>
                {gasInfo && (
                  <div className="px-4 py-3 space-y-2">
                    <div className="flex justify-between">
                      <span className="text-xs text-pax-muted">Gas used</span>
                      <span className="text-xs font-medium">
                        {gasInfo.used}
                        {gasInfo.limit ? ` / ${gasInfo.limit}` : ''}
                        {gasInfo.utilPct !== null && (
                          <span className="ml-1 text-pax-muted">({gasInfo.utilPct}%)</span>
                        )}
                      </span>
                    </div>
                    {gasInfo.utilPct !== null && (
                      <div className="h-1.5 rounded-full bg-pax-control overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all ${
                            gasInfo.utilPct > 90
                              ? 'bg-pax-error'
                              : gasInfo.utilPct > 70
                                ? 'bg-pax-warning'
                                : 'bg-pax-accent'
                          }`}
                          style={{ width: `${Math.min(gasInfo.utilPct, 100)}%` }}
                        />
                      </div>
                    )}
                  </div>
                )}
                {gasPriceLabel && (
                  <div className="flex justify-between px-4 py-3  ">
                    <span className="text-xs text-pax-muted">Gas price</span>
                    <span className="text-xs font-medium">{gasPriceLabel}</span>
                  </div>
                )}
              </div>
            )}

            {multipleTransfers && (
              <div
                aria-label="Token transfers"
                className="glass-card mt-4  divide-pax-line overflow-hidden"
              >
                <div className="px-4 py-3">
                  <p className="text-xs font-semibold text-pax-muted">
                    Token Transfers ({relevantTransfers.length})
                  </p>
                </div>
                {relevantTransfers.map((tt, i) => {
                  const ttDirection = transferDirection(tt.from?.hash, tt.to?.hash, addr);
                  const ttSent = ttDirection === 'sent';
                  const ttSelf = ttDirection === 'self';
                  const ttSym = tt.token?.symbol || '???';
                  const ttDec = tokenPrecision(tt.token?.decimals);
                  const ttAmt = transferAmount(tt.total?.value, ttDec);
                  return (
                    <div key={i} className="flex items-center gap-3 px-4 py-3">
                      <div
                        className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 ${ttSent ? 'bg-pax-error-muted' : 'bg-pax-success-muted'}`}
                      >
                        <SvgIcon
                          name={ttSent ? 'arrow-up-right' : 'arrow-down-left'}
                          className="w-3.5 h-3.5"
                        />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium">
                          {ttSelf ? 'Self transfer' : ttSent ? 'Sent' : 'Received'} {ttSym}
                        </p>
                        <p className="text-xs text-pax-muted truncate">
                          {ttSent
                            ? `To ${shortenAddress(tt.to?.hash || '')}`
                            : `From ${shortenAddress(tt.from?.hash || '')}`}
                        </p>
                      </div>
                      <p
                        className={`text-sm font-medium shrink-0 ${ttSent ? 'text-pax-error' : 'text-pax-success'}`}
                      >
                        {ttSelf ? '' : ttSent ? '-' : '+'}
                        {ttAmt} {ttSym}
                      </p>
                    </div>
                  );
                })}
              </div>
            )}
          </>
        ) : null}
      </div>

      <div className="px-4 pb-6 pt-2">
        <button
          type="button"
          onClick={onBack}
          className="min-h-[var(--touch-target)] w-full py-3 rounded-2xl bg-pax-overlay text-pax-light font-semibold text-sm press-scale hover:bg-pax-overlay transition-colors"
        >
          Close
        </button>
      </div>
    </div>
  );
}

function DetailRow({
  label,
  value,
  valueClass = '',
  mono = false,
}: {
  label: string;
  value: string;
  valueClass?: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between px-4 py-3">
      <span className="text-sm text-pax-muted">{label}</span>
      <span className={`text-sm font-medium text-right ${mono ? 'font-mono' : ''} ${valueClass}`}>
        {value}
      </span>
    </div>
  );
}
