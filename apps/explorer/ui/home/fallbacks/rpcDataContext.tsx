import { useQueries } from '@tanstack/react-query';
import React from 'react';

import type { Block } from 'types/api/block';
import type { Transaction } from 'types/api/transaction';

import { publicClient } from 'lib/web3/client';
import formatBlockData from 'lib/web3/rpc/formatBlockData';
import formatTxData from 'lib/web3/rpc/formatTxData';
import { SECOND } from 'toolkit/utils/consts';

export type SubscriptionId = 'latest-blocks' | 'latest-txs' | 'stats-widgets';

interface HomeRpcDataContext {
  blocks: Array<Block>;
  txs: Array<Transaction>;
  totalTxs: number;
  isError: boolean;
  isLoading: boolean;
  isEnabled: boolean;
  enable: (isEnabled: boolean, id: SubscriptionId) => void;
  subscriptions: Array<SubscriptionId>;
}

export const HomeRpcDataContext = React.createContext<HomeRpcDataContext | null>(null);

const ITEMS_LIMIT = 5;

export function HomeRpcDataContextProvider({ children }: { children: React.ReactNode }) {
  const [ feed, setFeed ] = React.useState(() => ({
    blocks: [] as Array<Block>,
    txs: [] as Array<Transaction>,
    observedHashes: new Set<string>(),
  }));
  const [ hasObservedBlock, setHasObservedBlock ] = React.useState(false);
  const [ isError, setIsError ] = React.useState(false);
  const [ subscriptions, setSubscriptions ] = React.useState<Array<SubscriptionId>>([]);
  const isEnabled = Boolean(publicClient) && subscriptions.length > 0;

  React.useEffect(() => {
    if (!publicClient || !isEnabled) {
      return;
    }

    setHasObservedBlock(false);
    setIsError(false);
    let active = true;
    const unwatch = publicClient.watchBlocks({
      onBlock: (block) => {
        if (!active) {
          return;
        }
        try {
          const newTxs = block.transactions.map((tx) => formatTxData(tx, null, null, block)).filter(Boolean);
          const nextBlock = formatBlockData({
            ...block,
            transactions: block.transactions.map((tx) => tx.hash),
          });
          if (!nextBlock) {
            throw new Error('Cannot format an observed RPC block');
          }
          setFeed((prev) => {
            const observedHashes = new Set(prev.observedHashes);
            newTxs.forEach((tx) => observedHashes.add(tx.hash));
            const sampleHashes = new Set<string>();
            const txs = [ ...newTxs, ...prev.txs ].filter((tx) => {
              if (sampleHashes.has(tx.hash)) {
                return false;
              }
              sampleHashes.add(tx.hash);
              return true;
            }).slice(0, ITEMS_LIMIT);

            return {
              blocks: [ nextBlock, ...prev.blocks.filter((item) => item.hash !== nextBlock.hash) ].slice(0, ITEMS_LIMIT),
              txs,
              observedHashes,
            };
          });
          setHasObservedBlock(true);
        } catch (_) {
          setIsError(true);
        }
      },
      onError: () => {
        if (active) {
          setIsError(true);
        }
      },
      pollingInterval: 5 * SECOND,
      includeTransactions: true,
      emitOnBegin: true,
    });
    return () => {
      active = false;
      unwatch();
    };
  }, [ isEnabled ]);

  const receiptQueries = useQueries({
    queries: feed.txs.map((tx) => ({
      queryKey: [ 'RPC', 'tx-receipt', { hash: tx.hash } ],
      queryFn: async() => {
        if (!publicClient) {
          return null;
        }
        return publicClient.getTransactionReceipt({ hash: tx.hash as `0x${ string }` });
      },
      enabled: isEnabled && !isError,
      staleTime: Infinity,
    })),
  });

  const txs = feed.txs.map((tx, index) => {
    const receipt = receiptQueries[index].data;
    if (!receipt) {
      return tx;
    }
    return {
      ...tx,
      status: receipt.status === 'success' ? 'ok' as const : 'error' as const,
    };
  });

  const enable = React.useCallback((enabled: boolean, id: SubscriptionId) => {
    if (!publicClient) {
      setIsError(true);
      return;
    }

    setSubscriptions((prev) => {
      if (enabled) {
        return prev.includes(id) ? prev : [ ...prev, id ];
      }
      return prev.includes(id) ? prev.filter((subscription) => subscription !== id) : prev;
    });
  }, []);

  const value = {
    blocks: feed.blocks,
    txs,
    totalTxs: feed.observedHashes.size,
    isError,
    isLoading: !hasObservedBlock && !isError,
    isEnabled,
    enable,
    subscriptions,
  };

  return (
    <HomeRpcDataContext.Provider value={ value }>
      { children }
    </HomeRpcDataContext.Provider>
  );
}

export function useHomeRpcDataContext() {
  const context = React.useContext(HomeRpcDataContext);
  if (!context) {
    throw new Error('useHomeRpcDataContext must be used within a HomeRpcDataContextProvider');
  }
  return context;
}
