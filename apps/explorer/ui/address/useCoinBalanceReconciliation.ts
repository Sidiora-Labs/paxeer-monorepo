import { useQueryClient } from '@tanstack/react-query';
import React from 'react';

import { getResourceKey } from 'lib/api/useApiQuery';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';

export default function useCoinBalanceReconciliation(addressHash: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const [ degraded, setDegraded ] = React.useState(false);
  const session = React.useRef({ active: false, running: false, dirty: false, interruption: 0 });

  React.useEffect(() => {
    const current = { active: enabled, running: false, dirty: false, interruption: 0 };
    session.current = current;
    setDegraded(false);
    return () => {
      current.active = false;
    };
  }, [ addressHash, enabled ]);

  const reconcile = React.useCallback(async() => {
    const current = session.current;
    if (!current.active) {
      return;
    }
    current.dirty = true;
    if (current.running) {
      return;
    }
    current.running = true;
    setDegraded(true);
    try {
      while (current.active && current.dirty) {
        current.dirty = false;
        const interruption = current.interruption;
        await Promise.all([
          queryClient.refetchQueries({
            queryKey: getResourceKey('general:address_coin_balance', { pathParams: { hash: addressHash } }),
            exact: true,
            type: 'all',
          }, { throwOnError: true, cancelRefetch: false }),
          queryClient.refetchQueries({
            queryKey: getResourceKey('general:address_coin_balance_chart', { pathParams: { hash: addressHash } }),
            exact: true,
            type: 'all',
          }, { throwOnError: true, cancelRefetch: false }),
        ]);
        if (current.active && !current.dirty && interruption === current.interruption) {
          setDegraded(false);
        }
      }
    } catch {
      if (current.active) {
        setDegraded(true);
      }
    } finally {
      current.running = false;
    }
  }, [ addressHash, queryClient ]);

  const onError = React.useCallback(() => {
    session.current.interruption++;
    setDegraded(true);
  }, []);

  const channel = useSocketChannel({
    topic: `addresses:${ addressHash.toLowerCase() }`,
    isDisabled: !enabled || !addressHash,
    onSocketClose: onError,
    onSocketError: onError,
  });
  React.useEffect(() => {
    if (!channel) {
      return;
    }
    // A sibling can own the channel's join push. Observe successful replies on the shared channel
    // so every history consumer reconciles after a rejoin, regardless of who subscribed first.
    const ref = channel.on('phx_reply', (reply: { status: string }) => {
      if (reply.status === 'ok' && channel.state === 'joined') {
        void reconcile();
      }
    });
    if (channel.state === 'joined') {
      void reconcile();
    }
    return () => {
      channel.off('phx_reply', ref);
    };
  }, [ channel, reconcile ]);
  useSocketMessage({ channel, event: 'coin_balance', handler: reconcile });

  return degraded;
}
