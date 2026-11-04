import type { Channel } from 'phoenix';
import { useEffect, useRef, useState } from 'react';

import type { ChannelSubscription, RegisteredChannel } from './context';
import { useSocket } from './context';

interface Params {
  topic: string | undefined;
  params?: object;
  isDisabled: boolean;
  onJoin?: (channel: Channel, message: unknown) => void;
  onSocketClose?: () => void;
  onSocketError?: () => void;
  socketName?: string;
}

export default function useSocketChannel({ topic, params, isDisabled, onJoin, onSocketClose, onSocketError, socketName }: Params) {
  const { socket, channelRegistry } = useSocket(socketName) || {};
  const [ channel, setChannel ] = useState<Channel>();
  const onCloseRef = useRef<string>(undefined);
  const onErrorRef = useRef<string>(undefined);

  const onJoinRef = useRef(onJoin);
  onJoinRef.current = onJoin;
  const onSocketErrorRef = useRef(onSocketError);
  onSocketErrorRef.current = onSocketError;

  useEffect(() => {
    const cleanUpRefs = () => {
      const refs = [ onCloseRef.current, onErrorRef.current ].filter(Boolean);
      refs.length > 0 && socket?.off(refs);
    };

    if (!isDisabled) {
      onCloseRef.current = onSocketClose && socket?.onClose(onSocketClose);
      onErrorRef.current = onSocketError && socket?.onError(onSocketError);
    } else {
      cleanUpRefs();
    }

    return cleanUpRefs;
  }, [ onSocketClose, onSocketError, socket, isDisabled ]);

  useEffect(() => {
    if (!socket || isDisabled || !topic || !channelRegistry) {
      return;
    }

    const currentRegistry = channelRegistry.current;
    const existing = currentRegistry[topic];
    const entry: RegisteredChannel = existing?.socket === socket ? existing : {
      socket,
      channel: socket.channel(topic),
      subscribers: 0,
      listeners: new Set(),
    };
    const ch = entry.channel;
    const listener: ChannelSubscription = {
      onJoin: (message) => onJoinRef.current?.(ch, message),
      onError: () => onSocketErrorRef.current?.(),
    };

    entry.subscribers++;
    entry.listeners.add(listener);

    if (entry !== existing) {
      currentRegistry[topic] = entry;
      ch.join()
        .receive('ok', (message) => {
          entry.listeners.forEach((subscriber) => subscriber.onJoin(message));
        })
        .receive('error', () => {
          entry.listeners.forEach((subscriber) => subscriber.onError());
        });
    } else if (ch.state === 'joined') {
      listener.onJoin('');
    }

    setChannel(ch);

    return () => {
      entry.listeners.delete(listener);
      entry.subscribers--;

      if (entry.subscribers === 0) {
        ch.leave();
        if (currentRegistry[topic] === entry) {
          delete currentRegistry[topic];
        }
      }

      setChannel(undefined);
    };
  }, [ socket, topic, params, isDisabled, channelRegistry ]);

  return isDisabled ? undefined : channel;
}
