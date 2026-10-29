// @vitest-environment jsdom
import { expect, test } from 'vitest';
import * as surface from './useSurface';

function use_surface_exports_no_receipt_polling() {
  expect(surface).not.toHaveProperty('waitForReceipt');
  expect(surface).not.toHaveProperty('RECEIPT_ATTEMPTS');
  expect(surface).not.toHaveProperty('RECEIPT_INTERVAL_MS');
}

test(use_surface_exports_no_receipt_polling);

function caps_expiry_ignores_revoked_and_expired_grants() {
  const now = 100n;
  const revoked = { expiration: 101n, revoked: true };
  const expired = { expiration: now, revoked: false };
  const previouslyExpired = { expiration: 99n, revoked: false };
  const later = { expiration: 150n, revoked: false };
  const earliest = { expiration: 125n, revoked: false };

  expect(
    surface.earliestLiveExpiry([revoked, expired, previouslyExpired, later, earliest], now)
  ).toBe(125n);
  expect(surface.earliestLiveExpiry([revoked, expired, previouslyExpired], now)).toBeNull();
  expect(surface.earliestLiveExpiry([], now)).toBeNull();
}

test(caps_expiry_ignores_revoked_and_expired_grants);

function caps_refresh_never_schedules_a_past_expiry() {
  const now = 100_000;

  expect(surface.capsRefreshDelay('99', now)).toBeNull();
  expect(surface.capsRefreshDelay('100', now)).toBeNull();
  expect(surface.capsRefreshDelay('100', now + 1)).toBeNull();
  expect(surface.capsRefreshDelay('101', now)).toBe(1_000);
  expect(surface.capsRefreshDelay('101', now + 1)).toBe(999);
  expect(surface.capsRefreshDelay('160', now)).toBe(60_000);
  expect(surface.capsRefreshDelay('1000', now)).toBe(60_000);
  expect(surface.capsRefreshDelay(((1n << 64n) - 1n).toString(), now)).toBe(60_000);
}

test(caps_refresh_never_schedules_a_past_expiry);
