import React from 'react';

import type { FlashblockItem } from 'types/client/flashblocks';

import config from 'configs/app';
import { SECOND } from 'toolkit/utils/consts';

import { formatFlashblockItemMegaEth, formatFlashblockItemOptimism } from './formatFlashblockItem';
import { parseSocketEventDataMegaEth, parseSocketEventDataOptimism } from './parseSocketEventData';

const flashblocksFeature = config.features.flashblocks;

const MAX_FLASHBLOCKS_COUNT = 50;
const QUEUE_TIME_THRESHOLD = 200;
const SUBSCRIPTION_TIMEOUT = 5 * SECOND;

type Status = 'initial' | 'connected' | 'disconnected' | 'error';

export default function useFlashblocksSocketData() {
  const websocketRef = React.useRef<WebSocket | null>(null);
  const isPausedRef = React.useRef(false);
  const subscriptionTimeoutRef = React.useRef(0);
  const flushTimeoutRef = React.useRef(0);
  const lastFlushRef = React.useRef(Date.now());
  const itemsQueueRef = React.useRef<Array<FlashblockItem>>([]);
  const statusRef = React.useRef<Status>('initial');

  const [ items, setItems ] = React.useState<Array<FlashblockItem>>([]);
  const [ itemsNum, setItemsNum ] = React.useState(0);
  const [ newItemsNum, setNewItemsNum ] = React.useState<number | undefined>(undefined);
  const [ txsNum, setTxsNum ] = React.useState(0);
  const [ initialTs, setInitialTs ] = React.useState<number | undefined>(undefined);
  const [ status, setStatus ] = React.useState<Status>('initial');

  const updateStatus = React.useCallback((nextStatus: Status) => {
    statusRef.current = nextStatus;
    setStatus(nextStatus);
  }, []);

  const flush = React.useCallback(() => {
    window.clearTimeout(flushTimeoutRef.current);
    flushTimeoutRef.current = 0;
    const newItems = itemsQueueRef.current;
    if (!newItems.length) {
      return;
    }
    itemsQueueRef.current = [];
    const now = Date.now();
    lastFlushRef.current = now;
    if (isPausedRef.current) {
      setNewItemsNum((prev) => (prev ?? 0) + newItems.length);
    } else {
      setInitialTs((prev) => prev ?? now);
      setItems((prev) => [ ...newItems, ...prev ].slice(0, MAX_FLASHBLOCKS_COUNT));
    }
    setItemsNum((prev) => prev + newItems.length);
    setTxsNum((prev) => prev + newItems.reduce((acc, item) => acc + item.transactions_count, 0));
  }, []);

  const dispose = React.useCallback(() => {
    window.clearTimeout(subscriptionTimeoutRef.current);
    window.clearTimeout(flushTimeoutRef.current);
    subscriptionTimeoutRef.current = 0;
    flushTimeoutRef.current = 0;
    const socket = websocketRef.current;
    websocketRef.current = null;
    if (socket) {
      socket.onopen = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.onclose = null;
      socket.close();
    }
  }, []);

  const connect = React.useCallback(() => {
    if (!flashblocksFeature.isEnabled) {
      return;
    }

    updateStatus('initial');
    let socket: WebSocket;
    try {
      socket = new WebSocket(flashblocksFeature.socketUrl);
    } catch (error) {
      updateStatus('error');
      return;
    }
    websocketRef.current = socket;
    socket.binaryType = 'arraybuffer';
    let subscribed = flashblocksFeature.type === 'optimism';
    let processing = Promise.resolve();

    const fail = () => {
      if (websocketRef.current === socket) {
        window.clearTimeout(subscriptionTimeoutRef.current);
        updateStatus('error');
      }
    };

    subscriptionTimeoutRef.current = window.setTimeout(() => {
      if (websocketRef.current === socket) {
        fail();
        socket.close();
      }
    }, SUBSCRIPTION_TIMEOUT);

    socket.onmessage = (event) => {
      processing = processing.then(async() => {
        if (websocketRef.current !== socket || statusRef.current === 'error') {
          return;
        }
        let newItem: FlashblockItem | undefined;
        if (flashblocksFeature.type === 'optimism') {
          newItem = formatFlashblockItemOptimism(await parseSocketEventDataOptimism(event));
        } else {
          const parsed = parseSocketEventDataMegaEth(event);
          if (!parsed) {
            subscribed = true;
            window.clearTimeout(subscriptionTimeoutRef.current);
            updateStatus('connected');
            return;
          }
          if (!subscribed) {
            throw new Error('Mini-block received before subscription confirmation');
          }
          newItem = formatFlashblockItemMegaEth(parsed);
        }
        if (websocketRef.current !== socket) {
          return;
        }
        itemsQueueRef.current.unshift(newItem);
        const remaining = QUEUE_TIME_THRESHOLD - (Date.now() - lastFlushRef.current);
        if (remaining <= 0) {
          flush();
        } else if (!flushTimeoutRef.current) {
          flushTimeoutRef.current = window.setTimeout(flush, remaining);
        }
      }).catch(fail);
    };

    socket.onopen = () => {
      if (flashblocksFeature.type === 'megaEth') {
        try {
          socket.send(JSON.stringify({ jsonrpc: '2.0', method: 'eth_subscribe', params: [ 'miniBlocks' ], id: 1 }));
        } catch (error) {
          fail();
        }
      } else {
        window.clearTimeout(subscriptionTimeoutRef.current);
        updateStatus('connected');
      }
    };
    socket.onerror = fail;
    socket.onclose = () => {
      if (websocketRef.current === socket) {
        window.clearTimeout(subscriptionTimeoutRef.current);
        flush();
        if (statusRef.current !== 'error') {
          updateStatus('disconnected');
        }
      }
    };
  }, [ flush, updateStatus ]);

  const reconnect = React.useCallback(() => {
    flush();
    dispose();
    connect();
  }, [ connect, dispose, flush ]);

  const pause = React.useCallback(() => {
    isPausedRef.current = true;
    setNewItemsNum(undefined);
  }, []);

  const resume = React.useCallback(() => {
    isPausedRef.current = false;
    setNewItemsNum(undefined);
    if (statusRef.current === 'error' || statusRef.current === 'disconnected') {
      reconnect();
    }
  }, [ reconnect ]);

  React.useEffect(() => {
    connect();
    return () => {
      dispose();
      itemsQueueRef.current = [];
    };
  }, [ connect, dispose ]);

  return React.useMemo(() => ({
    items,
    pause,
    resume,
    reconnect,
    itemsNum,
    newItemsNum,
    initialTs,
    txsNum,
    status,
  }), [ items, pause, resume, reconnect, itemsNum, newItemsNum, initialTs, txsNum, status ]);
}
