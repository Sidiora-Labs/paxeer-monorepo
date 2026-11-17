'use client';

import { AnimatePresence, motion } from 'framer-motion';
import { Copy, Ellipsis, ExternalLink } from 'lucide-react';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import Image from 'next/image';
import { holdingBalance } from './holdingBalance';
import type { HoldingsGridHolding } from './HoldingsGrid';
import { PAX_ICON_URL } from '@/lib/constants';
import { formatUsd } from '@/lib/format';

const CARD_COLORS = [
  'bg-pax-accent-dim',
  'bg-pax-overlay',
  'bg-pax-warning-surface',
  'bg-pax-success-surface',
];

interface WalletItem {
  name: string;
  symbol: string;
  amount: string;
  valueUsd: number | null;
  bgColor: string;
  iconUrl: string | null;
  tokenId: string;
}

export interface PaxCardExpandProps {
  address?: string;
  hidden?: boolean;
  holdings: HoldingsGridHolding[];
  nativeBalance: string;
  nativeValueUsd: number | null;
  onTokenDetail?: (tokenId: string, symbol: string) => void;
}

export const PaxCardExpand = ({
  address,
  hidden = false,
  holdings,
  nativeBalance,
  nativeValueUsd,
  onTokenDetail,
}: PaxCardExpandProps) => {
  const [expandedCard, setExpandedCard] = useState<number | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const walletItems = useMemo((): WalletItem[] => {
    const items: WalletItem[] = [];
    items.push({
      name: 'Paxeer',
      symbol: 'PAX',
      amount: nativeBalance,
      valueUsd: nativeValueUsd,
      bgColor: CARD_COLORS[0],
      iconUrl: PAX_ICON_URL,
      tokenId: 'pax',
    });
    holdings.slice(0, 3).forEach((h, i) => {
      items.push({
        name: h.name || 'Unknown',
        symbol: h.symbol || '???',
        amount: holdingBalance(h),
        valueUsd: h.value_usd != null ? Number(h.value_usd) : null,
        bgColor: CARD_COLORS[(i + 1) % CARD_COLORS.length],
        iconUrl: h.icon_url || null,
        tokenId: h.contract_address ?? '',
      });
    });
    return items.slice(0, 4);
  }, [holdings, nativeBalance, nativeValueUsd]);

  // Handle outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setExpandedCard(null);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  const handleCardClick = (index: number) => {
    setExpandedCard(expandedCard === index ? null : index);
  };

  const renderCard = (item: WalletItem, index: number, isBottomRow = false) => {
    const isExpanded = expandedCard === index;
    const hasThreeBottomCards = expandedCard !== null && getBottomRowCards().length === 3;
    const iconSize = isExpanded
      ? 'size-12'
      : hasThreeBottomCards && isBottomRow
        ? 'size-6'
        : 'size-8';

    return (
      <motion.div
        key={index}
        layoutId={`card-${index}`}

        className={`pax-asset-card relative flex min-w-0 flex-col items-start justify-between gap-4 p-3 text-pax-light ${
          isExpanded
            ? 'min-h-[var(--size-card-expanded)] w-full'
            : hasThreeBottomCards && isBottomRow
              ? 'min-h-[var(--size-card-compact)] flex-1'
              : 'min-h-[var(--size-card-row)] flex-1'
        } rounded-3xl ${item.bgColor}`}
        style={{ transformOrigin: '50% 50% 0px' }}
      >
        <div className="flex w-full flex-wrap items-start justify-between gap-2">
          <motion.div
            layoutId={`icon-${index}`}
            className={`${iconSize} rounded-full bg-pax-overlay overflow-hidden shrink-0 flex items-center justify-center`}
          >
            {item.iconUrl ? (
              <Image
                src={item.iconUrl}
                alt={item.symbol}
                width={48}
                height={48}
                className="w-full h-full object-cover"
              />
            ) : (
              <span className="text-xs font-bold text-pax-light">{item.symbol.slice(0, 3)}</span>
            )}
          </motion.div>

          {!isExpanded && (
            <motion.button
              type="button"
              aria-label={`Expand ${item.symbol}`}
              aria-expanded={false}
              onClick={() => handleCardClick(index)}
              initial={{ opacity: 0, filter: 'blur(2px)' }}
              animate={{ opacity: 1, filter: 'blur(0px)' }}
              exit={{ opacity: 0, filter: 'blur(2px)' }}
              className="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-full bg-pax-overlay p-0.5 transition-colors hover:bg-pax-overlay"
            >
              <Ellipsis className="size-3" />
            </motion.button>
          )}

          <AnimatePresence>
            {isExpanded && (
              <motion.button
                layoutId={`action-top-${index}`}
                onClick={() => {
                  if (item.tokenId)
                    navigator.clipboard
                      ?.writeText(item.tokenId === 'pax' ? address || '' : item.tokenId)
                      .catch(() => {});
                  setExpandedCard(null);
                }}
                className="flex flex-wrap items-center gap-2 font-semibold tracking-tight hover:opacity-80"
              >
                <p className="text-sm">
                  {item.tokenId === 'pax' ? 'Copy Address' : 'Copy Contract'}
                </p>
                <div className="flex size-5 shrink-0 items-center justify-center rounded-full bg-pax-overlay hover:bg-pax-overlay">
                  <Copy className="size-3" />
                </div>
              </motion.button>
            )}
          </AnimatePresence>
          <AnimatePresence>
            {isExpanded && (
              <motion.button
                layoutId={`action-bottom-${index}`}
                onClick={() => {
                  onTokenDetail?.(item.tokenId, item.symbol);
                  setExpandedCard(null);
                }}
                className="flex flex-wrap items-center gap-2 rounded-full bg-pax-overlay px-3 py-1 text-sm font-semibold tracking-tight hover:bg-pax-overlay"
              >
                <ExternalLink className="size-3.5" />
                View Details
              </motion.button>
            )}
          </AnimatePresence>
        </div>
        <div className="flex flex-col items-start justify-center">
          <motion.span
            layoutId={`title-${index}`}
            className={`font-openrunde select-none font-semibold text-pax-light ${
              isExpanded ? 'text-xl' : hasThreeBottomCards && isBottomRow ? 'text-sm' : 'text-base'
            }`}
          >
            {hidden ? item.symbol : item.name}
          </motion.span>
          <motion.span
            layoutId={`desc-${index}`}
            className={`font-openrunde select-none font-semibold text-pax-light ${
              isExpanded ? 'text-lg' : hasThreeBottomCards && isBottomRow ? 'text-xs' : 'text-sm'
            }`}
          >
            {hidden ? '••••' : item.amount}
          </motion.span>
          {isExpanded && item.valueUsd !== null && item.valueUsd > 0 && !hidden && (
            <motion.span className="text-sm text-pax-light mt-0.5">
              {formatUsd(item.valueUsd)}
            </motion.span>
          )}
        </div>
      </motion.div>
    );
  };

  const getTopRowCards = () => {
    if (expandedCard === null) {
      return walletItems.slice(0, 2);
    }
    return walletItems.filter((_, index) => index === expandedCard);
  };

  const getBottomRowCards = () => {
    if (expandedCard === null) {
      return walletItems.slice(2, 4);
    }
    return walletItems.filter((_, index) => index !== expandedCard);
  };

  return (
    <div className="font-sans flex w-full flex-col gap-4">
      <div ref={containerRef} className="flex w-full flex-col gap-4">
        <div className="pax-asset-row grid grid-cols-2 gap-4">
          {getTopRowCards().map((item, index) => {
            const originalIndex = expandedCard !== null ? expandedCard : index;
            return renderCard(item, originalIndex, false);
          })}
        </div>
        <div className="pax-asset-row grid grid-cols-2 gap-4">
          {getBottomRowCards().map((item, index) => {
            const originalIndex =
              expandedCard !== null
                ? walletItems.findIndex((_, i) => i !== expandedCard && walletItems[i] === item)
                : index + 2;
            return renderCard(item, originalIndex, true);
          })}
        </div>
      </div>
    </div>
  );
};
