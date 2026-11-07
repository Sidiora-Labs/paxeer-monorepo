import type { Channel, Socket } from 'phoenix';
import { useEffect, useMemo, useRef, useState } from 'react';

import { usePaxscanSocket } from './context';

interface Params {
  topic: string | undefined;
  params?: object;
  isDisabled: boolean;
  onJoin?: (channel: Channel, message: unknown) => void;
  onSocketClose?: () => void;
  onSocketError?: () => void;
  socketName?: string;
}

type JoinValue = null | boolean | number | string | JoinValue[] | { [key: string]: JoinValue };

export function serializeJoinParams(params: object = {}): string {
  const ancestors = new Set<object>();
  function validate(value: unknown): JoinValue {
    if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value !== 'object' || ancestors.has(value)) {
      throw new TypeError('Channel join params must contain finite JSON values without cycles');
    }
    const prototype = Object.getPrototypeOf(value);
    if (!Array.isArray(value) && prototype !== Object.prototype && prototype !== null) {
      throw new TypeError('Channel join params must contain only plain objects and arrays');
    }
    if (Object.getOwnPropertySymbols(value).length > 0) {
      throw new TypeError('Channel join params cannot contain symbol keys');
    }
    ancestors.add(value);
    let result: JoinValue;
    if (Array.isArray(value)) {
      result = Array.from(value, validate);
    } else {
      result = Object.fromEntries(Object.keys(value).sort().map(key => {
        const descriptor = Object.getOwnPropertyDescriptor(value, key)!;
        if (!('value' in descriptor)) throw new TypeError('Channel join params cannot contain accessors');
        return [key, validate(descriptor.value)];
      }));
    }
    ancestors.delete(value);
    return result;
  }
  if (params === null || Array.isArray(params) || typeof params !== 'object') {
    throw new TypeError('Channel join params must be a plain object');
  }
  return JSON.stringify(validate(params));
}

export function usePaxscanSocketChannel({ topic, params, isDisabled, onJoin, onSocketClose, onSocketError, socketName }: Params) {
  const { socket, createSocket, channelRegistry } = usePaxscanSocket(socketName) || {};
  const serializedParams = serializeJoinParams(params);
  const joinParams: object = useMemo(() => JSON.parse(serializedParams), [ serializedParams ]);
  const registryKey = JSON.stringify([ topic, serializedParams ]);
  const [ subscription, setSubscription ] = useState<{ channel: Channel; socket: Socket }>();

  const onJoinRef = useRef(onJoin);
  onJoinRef.current = onJoin;

  useEffect(() => {
    if (isDisabled || !subscription) return;
    const channelSocket = subscription.socket;
    const refs = [
      onSocketClose && channelSocket.onClose(onSocketClose),
      onSocketError && channelSocket.onError(onSocketError),
    ].filter((ref): ref is string => Boolean(ref));
    return () => { channelSocket.off(refs); };
  }, [ onSocketClose, onSocketError, subscription, isDisabled ]);

  useEffect(() => {
    if (!socket || isDisabled || !topic || !channelRegistry || !createSocket) return;

    const currentRegistry = channelRegistry.current;
    let entry = currentRegistry[registryKey];
    if (entry) {
      entry.subscribers++;
      onJoinRef.current?.(entry.channel, '');
    } else {
      // Phoenix leaves any existing channel with the same topic on a socket.
      const topicInUse = Object.values(currentRegistry).some(item => item.channel.topic === topic);
      const channelSocket = topicInUse ? createSocket() : socket;
      if (!channelSocket) return;
      channelSocket.connect();
      const ch = channelSocket.channel(topic, joinParams);
      entry = { channel: ch, socket: channelSocket, subscribers: 1 };
      currentRegistry[registryKey] = entry;
      ch.join()
        .receive('ok', (message) => onJoinRef.current?.(ch, message))
        .receive('error', () => { onSocketError?.(); });
    }

    setSubscription(entry);

    return () => {
      if (currentRegistry[registryKey] === entry) {
        entry.subscribers--;
        if (entry.subscribers === 0) {
          entry.channel.leave();
          delete currentRegistry[registryKey];
          if (!Object.values(currentRegistry).some(item => item.socket === entry.socket)) {
            entry.socket.disconnect();
          }
        }
      }
      setSubscription(undefined);
    };
  }, [ socket, createSocket, topic, registryKey, joinParams, isDisabled, onSocketError, channelRegistry ]);

  return isDisabled ? undefined : subscription?.channel;
}
