import { expect, test } from 'vitest';
import { parseNativeRouteEvent, type NativeRouteEvent } from './index';
import { parseRouteUrl, serializeRoute, type ShellRoute } from '../shell';

const EVENT_KINDS = ['deep-link', 'push-route'] as const;

function native_route_origin_rejects_external_authorities() {
  for (const kind of EVENT_KINDS) {
    for (const route of [
      '//evil.example/send',
      '///evil.example/send',
      '/\\evil.example/send',
      '/\\\\evil.example/send',
      '//wallet.local@evil.example/send',
      '//wallet.local/send',
      'https://evil.example/send',
      'https://wallet.local/send',
      'javascript:alert(1)',
      'send',
    ]) {
      expect(parseNativeRouteEvent({ kind, route }), route).toMatchObject({
        ok: false,
        issues: [{ path: '$.route', code: 'invalid_format' }],
      });
    }
  }
  expect(new URL('//evil.example/send', 'https://wallet.local').origin).toBe(
    'https://evil.example'
  );
  expect(new URL('/\\evil.example/send', 'https://wallet.local').origin).toBe(
    'https://evil.example'
  );
}

function native_route_origin_rejects_ambiguous_path_grammar() {
  for (const kind of EVENT_KINDS) {
    for (const route of [
      '/\t/evil.example/send',
      '/\n/evil.example/send',
      '/\r/evil.example/send',
      '/send\u0000',
      '/send\u007f',
      '/wallet//send',
      '/wallet/\\send',
      '/%2fevil.example/send',
      '/%2Fevil.example/send',
      '/%5cevil.example/send',
      '/%5Cevil.example/send',
      '/wallet/%2f/send',
      '/wallet/%00send',
      '/wallet/%0asend',
      '/wallet/%7fsend',
      '/wallet/../send',
      '/wallet/./send',
      '/wallet/%2e%2e/send',
      '/wallet/%2E/send',
      '/send with spaces',
      '/send?to=bad value',
      '/send?to=bad\\value',
      '/send?to=<script>',
      '/send#bad#fragment',
      '/send%',
      '/send?to=%zz',
      '/send#%FF',
    ]) {
      expect(parseNativeRouteEvent({ kind, route }), route).toMatchObject({
        ok: false,
        issues: [{ path: '$.route', code: 'invalid_format' }],
      });
    }
  }
}

function native_route_origin_preserves_real_shell_route_roundtrips() {
  const address = `0x${'1'.repeat(40)}`;
  const routes: readonly ShellRoute[] = [
    { name: 'portfolio' },
    { name: 'send' },
    { name: 'send', token: 'pax', to: address },
    { name: 'send', to: address },
    { name: 'receive' },
    { name: 'transactions' },
    { name: 'swap' },
    { name: 'discover' },
    { name: 'pns' },
    { name: 'settings' },
    { name: 'contacts' },
    { name: 'token-detail', token: address, symbol: 'A+B' },
    { name: 'tx-detail', hash: `0x${'2'.repeat(64)}` },
  ];
  for (const kind of EVENT_KINDS) {
    for (const route of routes) {
      const event: NativeRouteEvent = { kind, route: serializeRoute(route) };
      const parsed = parseNativeRouteEvent(JSON.parse(JSON.stringify(event)));
      expect(parsed).toEqual({ ok: true, value: event });
      if (!parsed.ok) throw new Error('Native route was rejected');
      const destination = new URL(parsed.value.route, 'https://wallet.local');
      expect(destination.origin).toBe('https://wallet.local');
      expect(parseRouteUrl(destination)).toEqual({ ok: true, value: route });
    }
    for (const route of [
      '/',
      '/send',
      '/wallet/',
      '/wallet/?screen=send#details',
      '/send?next=https%3A%2F%2Fexample.com%2Fpath',
      '/send?label=Hello%20world&symbol=%E2%82%AC',
      '/send?next=/wallet/?screen=send',
      `/${'a'.repeat(1023)}`,
    ]) {
      expect(parseNativeRouteEvent({ kind, route })).toEqual({
        ok: true,
        value: { kind, route },
      });
    }
  }
}

function native_route_origin_preserves_event_boundary_checks() {
  for (const input of [null, [], 'send', {}, { kind: 'unknown', route: '/send' }]) {
    expect(parseNativeRouteEvent(input).ok).toBe(false);
  }
  expect(parseNativeRouteEvent({ kind: 'deep-link', route: '/send', extra: true })).toMatchObject({
    ok: false,
    issues: [{ path: '$.extra', code: 'unknown_field' }],
  });
  expect(parseNativeRouteEvent({ kind: 'push-route', route: 1 })).toMatchObject({
    ok: false,
    issues: [{ path: '$.route', code: 'invalid_type' }],
  });
  for (const route of ['', `/${'a'.repeat(1024)}`]) {
    expect(parseNativeRouteEvent({ kind: 'deep-link', route })).toMatchObject({
      ok: false,
      issues: [{ path: '$.route', code: 'out_of_bounds' }],
    });
  }
}

test(
  native_route_origin_rejects_external_authorities.name,
  native_route_origin_rejects_external_authorities
);
test(
  native_route_origin_rejects_ambiguous_path_grammar.name,
  native_route_origin_rejects_ambiguous_path_grammar
);
test(
  native_route_origin_preserves_real_shell_route_roundtrips.name,
  native_route_origin_preserves_real_shell_route_roundtrips
);
test(
  native_route_origin_preserves_event_boundary_checks.name,
  native_route_origin_preserves_event_boundary_checks
);
