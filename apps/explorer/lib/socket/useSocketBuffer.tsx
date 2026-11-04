import React from 'react';

// The chain seals a block every 150-200ms, so a live list that applied every socket message to React
// state would re-render itself several times a second and leave nothing on the page clickable. Every
// live list collects what it receives in a ref instead and hands the collection over on this cadence.
export const SOCKET_FLUSH_INTERVAL_MS = 2000;
export const SOCKET_BUFFER_LIMIT = 1000;

export interface SocketBufferFlush {
  overflowed: boolean;
}

export interface SocketBufferParams<T> {
  onFlush: (items: Array<T>, status: SocketBufferFlush) => void;
  onOverflow?: () => void;
  coalesce?: (previous: T, incoming: T) => T;
  isPaused?: boolean;
  limit?: number;
  intervalMs?: number;
}

export interface SocketBufferHoverProps {
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}

export interface SocketBuffer<T> {
  push: (item: T) => void;
  hoverProps: SocketBufferHoverProps;
  isHeld: boolean;
  readonly size: number;
  readonly overflowed: boolean;
}

export function useIsDocumentVisible() {
  const [ isVisible, setIsVisible ] = React.useState(true);

  React.useEffect(() => {
    const handleVisibilityChange = () => {
      setIsVisible(document.visibilityState !== 'hidden');
    };

    handleVisibilityChange();
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);

  return isVisible;
}

export default function useSocketBuffer<T>({
  onFlush,
  onOverflow,
  coalesce,
  isPaused,
  limit = SOCKET_BUFFER_LIMIT,
  intervalMs = SOCKET_FLUSH_INTERVAL_MS,
}: SocketBufferParams<T>): SocketBuffer<T> {
  if (!Number.isSafeInteger(limit) || limit < 1) {
    throw new RangeError('Socket buffer limit must be a positive safe integer');
  }

  const bufferRef = React.useRef<Array<T>>([]);
  const overflowRef = React.useRef(false);
  const onFlushRef = React.useRef(onFlush);
  onFlushRef.current = onFlush;
  const onOverflowRef = React.useRef(onOverflow);
  onOverflowRef.current = onOverflow;
  const coalesceRef = React.useRef(coalesce);
  coalesceRef.current = coalesce;

  const [ isHovered, setIsHovered ] = React.useState(false);
  const isDocumentVisible = useIsDocumentVisible();
  const isHeld = isHovered || Boolean(isPaused) || !isDocumentVisible;

  const push = React.useCallback((item: T) => {
    const buffer = bufferRef.current;

    if (coalesceRef.current && buffer.length > 0) {
      buffer[0] = coalesceRef.current(buffer[0], item);
      return;
    }

    if (buffer.length >= limit) {
      buffer.splice(0, buffer.length - limit + 1);
      overflowRef.current = true;
    }

    buffer.push(item);
  }, [ limit ]);

  const hoverProps = React.useMemo(() => ({
    onMouseEnter: () => setIsHovered(true),
    onMouseLeave: () => setIsHovered(false),
  }), []);

  React.useEffect(() => {
    if (bufferRef.current.length > limit) {
      bufferRef.current.splice(0, bufferRef.current.length - limit);
      overflowRef.current = true;
    }
  }, [ limit ]);

  React.useEffect(() => {
    if (isHeld) {
      return;
    }

    const intervalId = window.setInterval(() => {
      if (bufferRef.current.length === 0) {
        return;
      }

      const items = bufferRef.current;
      const overflowed = overflowRef.current;
      bufferRef.current = [];
      overflowRef.current = false;

      if (overflowed && onOverflowRef.current) {
        onOverflowRef.current();
      } else {
        onFlushRef.current(items, { overflowed });
      }
    }, intervalMs);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [ isHeld, intervalMs ]);

  return React.useMemo(() => ({
    push,
    hoverProps,
    isHeld,
    get size() {
      return bufferRef.current.length;
    },
    get overflowed() {
      return overflowRef.current;
    },
  }), [ push, hoverProps, isHeld ]);
}

function addSocketCounts(previous: number, incoming: number) {
  const total = previous + incoming;
  if (!Number.isSafeInteger(total)) {
    throw new RangeError('Socket count exceeds the safe integer range');
  }
  return total;
}

export function useSocketCountBuffer({
  onFlush,
  ...params
}: Omit<SocketBufferParams<number>, 'onFlush' | 'limit' | 'coalesce' | 'onOverflow'> & { onFlush: (count: number) => void }): SocketBuffer<number> {
  const buffer = useSocketBuffer<number>({
    ...params,
    limit: 1,
    coalesce: addSocketCounts,
    onFlush: (items) => onFlush(items[0]),
  });
  const pushItem = buffer.push;
  const push = React.useCallback((count: number) => {
    if (!Number.isSafeInteger(count) || count < 0) {
      throw new RangeError('Socket count must be a nonnegative safe integer');
    }
    pushItem(count);
  }, [ pushItem ]);

  return React.useMemo(() => ({
    push,
    hoverProps: buffer.hoverProps,
    isHeld: buffer.isHeld,
    get size() {
      return buffer.size;
    },
    get overflowed() {
      return buffer.overflowed;
    },
  }), [ buffer, push ]);
}
