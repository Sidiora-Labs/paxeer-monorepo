'use client';

import { useCallback, useEffect } from 'react';
import { useWalletActions, useWalletState } from '@/providers/WalletProvider';
import {
  startNotificationTriggers,
  stopNotificationTriggers,
  handleTxCount,
} from '@/lib/notification-triggers';
import { useTxCountQuery } from '@/lib/queries/txCount';

/**
 * Wires the notification trigger lifecycle to the active wallet session.
 *
 * - Starts triggers when a session becomes active, stops on lock/unmount.
 * - Subscribes to useTxCountQuery (60 s poll via TanStack Query) and
 *   forwards each new count to handleTxCount for comparison + notification.
 */
export function useNotificationLifecycle(): void {
  const { activeAccount } = useWalletState();
  const { getSigner } = useWalletActions();
  const signMessage = useCallback(
    async (text: string) => (await getSigner()).signMessage(text),
    [getSigner]
  );
  const address = activeAccount?.address ? activeAccount.address : undefined;

  const { data: txCount } = useTxCountQuery(address);

  useEffect(() => {
    if (!address) {
      stopNotificationTriggers();
      return;
    }
    startNotificationTriggers(address, signMessage);
    return () => stopNotificationTriggers();
  }, [address, signMessage]);

  useEffect(() => {
    if (txCount !== undefined && address) {
      handleTxCount(txCount);
    }
  }, [txCount, address]);
}
