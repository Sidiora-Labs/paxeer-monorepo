import { reportBackgroundFailure } from '@/platform/status/background-failures';

export type StorageArea = 'local' | 'session';
export type StorageSensitivity =
  | 'public'
  | 'preferences'
  | 'private-metadata'
  | 'security-relevant';
export type StorageCorruption = 'reset-and-signal' | 'fail-closed';
export type StorageLifecycle =
  | 'reset'
  | 'logout'
  | 'custody-switch'
  | 'account-removal'
  | 'uninstall';

export interface StorageRegistration {
  readonly id: string;
  readonly key: string;
  readonly owner: string;
  readonly schema: string;
  readonly version: number;
  readonly area: StorageArea;
  readonly sensitivity: StorageSensitivity;
  readonly retention: string;
  readonly quotaBytes: number;
  readonly migration: string;
  readonly resetOn: readonly StorageLifecycle[];
  readonly corruption: StorageCorruption;
  readonly prohibitedData: readonly string[];
}

interface StorageEnvelope {
  version: number;
  writtenAt: number;
  value: unknown;
}

export type StorageReader = Pick<Storage, 'getItem' | 'key' | 'length'>;

export interface StorageMigration {
  readonly value: unknown;
  readonly sourceKeys: readonly string[];
}

export interface PersistedStorageValue<T> {
  readonly persisted: true;
  readonly value: T;
}

interface RepositoryDefinition<T> extends StorageRegistration {
  readonly fallback: () => T;
  readonly parse: (input: unknown) => T;
  readonly migrateLegacy?: (storage: StorageReader) => StorageMigration | undefined;
  readonly legacyKeys?: readonly string[];
}

export interface StorageRepository<T> {
  readonly registration: StorageRegistration;
  read(): T;
  write(value: T): PersistedStorageValue<T>;
  update(updater: (current: T) => T): T;
  remove(): void;
}

export class StorageCorruptionError extends Error {
  constructor(readonly storageId: string) {
    super('A security-relevant storage record is corrupt');
    this.name = 'StorageCorruptionError';
  }
}

export class StoragePersistenceError extends Error {
  constructor(
    readonly storageId: string,
    readonly code: 'STORAGE_UNAVAILABLE' | 'STORAGE_QUOTA_EXCEEDED' | 'STORAGE_WRITE_FAILED',
    readonly cause?: unknown
  ) {
    super('Changes could not be saved on this device.');
    this.name = 'StoragePersistenceError';
  }
}

const definitions = new Map<string, RepositoryDefinition<unknown>>();
const namespaces = new Map<string, StorageNamespaceDefinition>();

interface StorageNamespaceDefinition extends StorageRegistration {
  readonly corruption: 'fail-closed';
  readonly maxRecords: number;
  readonly parse: (key: string, value: string) => void;
}

function namespaceKeys(storage: Storage, prefix: string): string[] {
  const keys: string[] = [];
  for (let index = 0; index < storage.length; index += 1) {
    const key = storage.key(index);
    if (key !== null && key.startsWith(prefix)) keys.push(key);
  }
  return keys;
}

export function defineStorageNamespace(definition: StorageNamespaceDefinition): Storage {
  if (
    definitions.has(definition.id) ||
    definitions.has(definition.key) ||
    namespaces.has(definition.id) ||
    [...namespaces.values()].some(
      (entry) => entry.key.startsWith(definition.key) || definition.key.startsWith(entry.key)
    )
  ) {
    throw new TypeError(`Duplicate storage registration: ${definition.id}`);
  }
  namespaces.set(definition.id, definition);

  const refuse = (code: StoragePersistenceError['code'], cause?: unknown): never => {
    reportBackgroundFailure({
      domain: 'storage',
      kind: 'unavailable',
      code,
      message: 'Changes could not be saved on this device.',
      retryable: true,
    });
    throw new StoragePersistenceError(definition.id, code, cause);
  };
  const storage = (): Storage => browserStorage(definition.area) ?? refuse('STORAGE_UNAVAILABLE');
  const checkKey = (key: string): void => {
    if (!key.startsWith(definition.key))
      throw new TypeError('Key is outside the registered namespace');
  };
  return {
    get length() {
      return namespaceKeys(storage(), definition.key).length;
    },
    key(index) {
      return namespaceKeys(storage(), definition.key)[index] ?? null;
    },
    clear() {
      const target = storage();
      for (const key of namespaceKeys(target, definition.key)) target.removeItem(key);
    },
    getItem(key) {
      checkKey(key);
      const raw = storage().getItem(key);
      if (raw === null) return null;
      try {
        if (new TextEncoder().encode(raw).byteLength > definition.quotaBytes) {
          throw new TypeError('Storage record exceeds quota');
        }
        definition.parse(key, raw);
      } catch {
        reportBackgroundFailure({
          domain: 'storage',
          kind: 'corrupt',
          code: 'STORAGE_CORRUPT',
          message: 'Saved application data could not be read.',
          retryable: false,
        });
        throw new StorageCorruptionError(definition.id);
      }
      return raw;
    },
    setItem(key, value) {
      checkKey(key);
      definition.parse(key, value);
      const target = storage();
      const keys = new Set(namespaceKeys(target, definition.key));
      keys.add(key);
      const encoder = new TextEncoder();
      let bytes = 0;
      for (const current of keys)
        bytes += encoder.encode(
          current + (current === key ? value : (target.getItem(current) ?? ''))
        ).byteLength;
      if (keys.size > definition.maxRecords || bytes > definition.quotaBytes)
        refuse('STORAGE_QUOTA_EXCEEDED');
      try {
        target.setItem(key, value);
      } catch (cause) {
        refuse('STORAGE_WRITE_FAILED', cause);
      }
    },
    removeItem(key) {
      checkKey(key);
      storage().removeItem(key);
    },
  };
}

function browserStorage(area: StorageArea): Storage | null {
  try {
    if (area === 'local') return globalThis.localStorage ?? null;
    return globalThis.sessionStorage ?? null;
  } catch {
    return null;
  }
}

function envelope(input: unknown, version: number): unknown {
  if (
    typeof input !== 'object' ||
    input === null ||
    Array.isArray(input) ||
    (input as { version?: unknown }).version !== version ||
    typeof (input as { writtenAt?: unknown }).writtenAt !== 'number' ||
    !Number.isSafeInteger((input as { writtenAt: number }).writtenAt) ||
    (input as { writtenAt: number }).writtenAt < 0 ||
    !('value' in input)
  ) {
    throw new TypeError('Storage envelope is invalid');
  }
  return (input as StorageEnvelope).value;
}

function signal(definition: StorageRegistration, code: string): void {
  reportBackgroundFailure({
    domain: 'storage',
    kind: 'corrupt',
    code,
    message: 'Saved application data was reset because it could not be read.',
    retryable: false,
  });
  if (definition.corruption === 'fail-closed') {
    throw new StorageCorruptionError(definition.id);
  }
}

export function defineStorageRepository<T>(
  definition: RepositoryDefinition<T>
): StorageRepository<T> {
  if (
    definitions.has(definition.id) ||
    definitions.has(definition.key) ||
    namespaces.has(definition.id) ||
    [...namespaces.values()].some((entry) => definition.key.startsWith(entry.key))
  ) {
    throw new TypeError(`Duplicate storage registration: ${definition.id}`);
  }
  definitions.set(definition.id, definition as RepositoryDefinition<unknown>);
  definitions.set(definition.key, definition as RepositoryDefinition<unknown>);

  const removeLegacy = (storage: Storage, keys = definition.legacyKeys ?? []) => {
    for (const key of keys) storage.removeItem(key);
  };

  const refuse = (code: StoragePersistenceError['code'], cause?: unknown): never => {
    reportBackgroundFailure({
      domain: 'storage',
      kind: 'unavailable',
      code,
      message: 'Changes could not be saved on this device.',
      retryable: true,
    });
    throw new StoragePersistenceError(definition.id, code, cause);
  };

  const writeTo = (storage: Storage, value: T): PersistedStorageValue<T> => {
    const parsed = definition.parse(value);
    const serialized = JSON.stringify({
      version: definition.version,
      writtenAt: Date.now(),
      value: parsed,
    });
    if (new TextEncoder().encode(serialized).byteLength > definition.quotaBytes) {
      return refuse('STORAGE_QUOTA_EXCEEDED');
    }
    try {
      storage.setItem(definition.key, serialized);
    } catch (cause) {
      return refuse('STORAGE_WRITE_FAILED', cause);
    }
    return { persisted: true, value: parsed };
  };

  return {
    registration: definition,
    read(): T {
      const storage = browserStorage(definition.area);
      if (!storage) return definition.fallback();
      const raw = storage.getItem(definition.key);
      if (raw !== null) {
        try {
          if (new TextEncoder().encode(raw).byteLength > definition.quotaBytes) {
            throw new TypeError('Storage record exceeds quota');
          }
          return definition.parse(envelope(JSON.parse(raw) as unknown, definition.version));
        } catch {
          storage.removeItem(definition.key);
          signal(definition, 'STORAGE_CORRUPT');
          return definition.fallback();
        }
      }
      if (!definition.migrateLegacy) return definition.fallback();
      let migration: StorageMigration;
      let parsed: T;
      try {
        const legacy = definition.migrateLegacy(storage);
        if (legacy === undefined) return definition.fallback();
        migration = legacy;
        parsed = definition.parse(legacy.value);
      } catch {
        signal(definition, 'STORAGE_MIGRATION_FAILED');
        return definition.fallback();
      }
      const persisted = writeTo(storage, parsed);
      try {
        removeLegacy(storage, migration.sourceKeys);
      } catch {
        reportBackgroundFailure({
          domain: 'storage',
          kind: 'unavailable',
          code: 'STORAGE_LEGACY_CLEANUP_FAILED',
          message: 'Saved application data could not be removed from its previous location.',
          retryable: true,
        });
      }
      return persisted.value;
    },
    write(value: T): PersistedStorageValue<T> {
      const storage = browserStorage(definition.area);
      if (!storage) return refuse('STORAGE_UNAVAILABLE');
      return writeTo(storage, value);
    },
    update(updater: (current: T) => T): T {
      const next = definition.parse(updater(this.read()));
      return this.write(next).value;
    },
    remove(): void {
      const storage = browserStorage(definition.area);
      if (!storage) return;
      storage.removeItem(definition.key);
      removeLegacy(storage);
    },
  };
}

export function storageCatalog(): readonly StorageRegistration[] {
  return [
    ...new Map([...definitions.values()].map((definition) => [definition.id, definition])).values(),
    ...namespaces.values(),
  ].map((definition) => ({
    id: definition.id,
    key: definition.key,
    owner: definition.owner,
    schema: definition.schema,
    version: definition.version,
    area: definition.area,
    sensitivity: definition.sensitivity,
    retention: definition.retention,
    quotaBytes: definition.quotaBytes,
    migration: definition.migration,
    resetOn: definition.resetOn,
    corruption: definition.corruption,
    prohibitedData: definition.prohibitedData,
  }));
}

export function resetStorageForLifecycle(lifecycle: StorageLifecycle): void {
  for (const definition of new Map(
    [...definitions.values()].map((item) => [item.id, item])
  ).values()) {
    if (!definition.resetOn.includes(lifecycle)) continue;
    const storage = browserStorage(definition.area);
    storage?.removeItem(definition.key);
    for (const legacyKey of definition.legacyKeys ?? []) {
      storage?.removeItem(legacyKey);
    }
  }
  for (const definition of namespaces.values()) {
    if (!definition.resetOn.includes(lifecycle)) continue;
    const storage = browserStorage(definition.area);
    if (storage) for (const key of namespaceKeys(storage, definition.key)) storage.removeItem(key);
  }
}
