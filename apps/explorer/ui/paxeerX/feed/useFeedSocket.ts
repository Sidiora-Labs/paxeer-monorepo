import type { Channel, Push } from 'phoenix';
import React from 'react';

import useSocketChannel from 'lib/socket/useSocketChannel';
import { SECOND } from 'toolkit/utils/consts';

// While the channel is joined the pages take every submission from the socket; until it joins, or once
// the socket closes or fails, they fall back to reading the endpoint on this cadence.
export const FEED_FALLBACK_REFETCH_INTERVAL = 5 * SECOND;
export const FEED_OVERVIEW_BUFFER_LIMIT = 200;
export const FEED_HISTORY_BUFFER_LIMIT = 50;

interface Params {
  topic: string;
  isDisabled: boolean;
  onJoin?: () => void;
}

interface FeedChannel extends Channel {
  topic: string;
  joinPush: Push;
}

export default function useFeedSocket({ topic, isDisabled, onJoin }: Params) {
  const [ joinedChannel, setJoinedChannel ] = React.useState<Channel>();

  const handleJoin = React.useCallback((joined: Channel) => {
    if (!isDisabled && (joined as FeedChannel).topic === topic && joined.state === 'joined') {
      setJoinedChannel(joined);
      onJoin?.();
    }
  }, [ isDisabled, onJoin, topic ]);

  const handleSocketIssue = React.useCallback(() => {
    setJoinedChannel(undefined);
  }, []);

  const channel = useSocketChannel({
    topic,
    isDisabled,
    onJoin: handleJoin,
    onSocketClose: handleSocketIssue,
    onSocketError: handleSocketIssue,
  }) as FeedChannel | undefined;

  React.useEffect(() => {
    setJoinedChannel(undefined);
    if (!channel || isDisabled || channel.topic !== topic) {
      return;
    }

    let isActive = true;
    const handleCurrentJoin = () => {
      if (isActive && channel.state === 'joined') {
        setJoinedChannel(channel);
      }
    };
    const handleCurrentIssue = () => {
      if (isActive) {
        setJoinedChannel((joined) => joined === channel ? undefined : joined);
      }
    };
    const errorRef = channel.on('phx_error', handleCurrentIssue);
    const closeRef = channel.on('phx_close', handleCurrentIssue);
    channel.joinPush
      .receive('ok', handleCurrentJoin)
      .receive('error', handleCurrentIssue)
      .receive('timeout', handleCurrentIssue);
    handleCurrentJoin();

    return () => {
      isActive = false;
      channel.off('phx_error', errorRef);
      channel.off('phx_close', closeRef);
      setJoinedChannel((joined) => joined === channel ? undefined : joined);
    };
  }, [ channel, isDisabled, topic ]);

  const isJoined = channel !== undefined && channel === joinedChannel && channel.topic === topic && channel.state === 'joined';
  const refetchInterval: number | false = isDisabled || isJoined ? false : FEED_FALLBACK_REFETCH_INTERVAL;

  return { channel, refetchInterval, isSocketDown: !isDisabled && !isJoined };
}
