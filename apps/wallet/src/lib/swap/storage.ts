export type SwapStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

export function swapStorage(): SwapStorage {
  if (typeof window === 'undefined') throw new Error('Durable swap storage is unavailable');
  return window.localStorage;
}

export function writeSwapRecord(storage: SwapStorage, key: string, value: unknown): void {
  const encoded = JSON.stringify(value);
  storage.setItem(key, encoded);
  if (storage.getItem(key) !== encoded) throw new Error('Swap record could not be persisted');
}
