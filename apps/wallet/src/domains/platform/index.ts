import {
  issue,
  parseBoundedString,
  parseRecord,
  rejectUnknownFields,
  type BoundaryParser,
} from '../shared';

export type StorageOwner =
  | 'contacts'
  | 'custody-choice'
  | 'dapp'
  | 'metadata'
  | 'notifications'
  | 'operations'
  | 'portfolio'
  | 'preferences'
  | 'pwa';

export interface StorageEnvelope<T> {
  readonly version: number;
  readonly writtenAt: number;
  readonly value: T;
}

export function storageEnvelopeParser<T>(
  version: number,
  valueParser: BoundaryParser<T>,
): BoundaryParser<StorageEnvelope<T>> {
  return (input) => {
    const record = parseRecord(input);
    if (!record.ok) return record;
    const fields = rejectUnknownFields(
      record.value,
      new Set(['version', 'writtenAt', 'value']),
    );
    if (!fields.ok) return fields;
    if (record.value.version !== version) {
      return issue('$.version', 'unsupported_value', 'Storage version is unsupported');
    }
    if (
      typeof record.value.writtenAt !== 'number' ||
      !Number.isSafeInteger(record.value.writtenAt) ||
      record.value.writtenAt < 0
    ) {
      return issue('$.writtenAt', 'invalid_format', 'Storage timestamp is invalid');
    }
    const value = valueParser(record.value.value);
    if (!value.ok) return value;
    return {
      ok: true,
      value: {
        version,
        writtenAt: record.value.writtenAt,
        value: value.value,
      },
    };
  };
}

export type NativeRouteEvent =
  | { readonly kind: 'deep-link'; readonly route: string }
  | { readonly kind: 'push-route'; readonly route: string };

const NATIVE_ROUTE_ORIGIN = 'https://wallet.local';
const NATIVE_ROUTE_GRAMMAR =
  /^\/[A-Za-z0-9._~!$&'()*+,;=:@%/-]*(?:\?[A-Za-z0-9._~!$&'()*+,;=:@%/?-]*)?(?:#[A-Za-z0-9._~!$&'()*+,;=:@%/?-]*)?$/;

export const parseNativeRouteEvent: BoundaryParser<NativeRouteEvent> = (input) => {
  const record = parseRecord(input);
  if (!record.ok) return record;
  const fields = rejectUnknownFields(record.value, new Set(['kind', 'route']));
  if (!fields.ok) return fields;
  if (record.value.kind !== 'deep-link' && record.value.kind !== 'push-route') {
    return issue('$.kind', 'unsupported_value', 'Native event kind is unsupported');
  }
  const route = parseBoundedString(record.value.route, {
    path: '$.route',
    minLength: 1,
    maxLength: 1024,
  });
  if (!route.ok) return route;
  if (!NATIVE_ROUTE_GRAMMAR.test(route.value) || route.value.startsWith('//')) {
    return issue('$.route', 'invalid_format', 'Native route must be same-origin');
  }
  let url: URL;
  let decodedPath: string;
  try {
    url = new URL(route.value, NATIVE_ROUTE_ORIGIN);
    decodeURIComponent(route.value);
    decodedPath = decodeURIComponent(url.pathname);
  } catch {
    return issue('$.route', 'invalid_format', 'Native route URL is invalid');
  }
  if (
    url.origin !== NATIVE_ROUTE_ORIGIN ||
    url.pathname !== route.value.split(/[?#]/, 1)[0] ||
    /\/\/|%2f|%5c/i.test(url.pathname) ||
    /[\\\u0000-\u001f\u007f]/.test(decodedPath)
  ) {
    return issue('$.route', 'invalid_format', 'Native route path is ambiguous');
  }
  return {
    ok: true,
    value: { kind: record.value.kind, route: route.value },
  };
};
