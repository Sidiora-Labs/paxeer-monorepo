import { readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';
import { GET } from '@/app/api/candle/[...path]/route';
import { SWAP_TOKENS } from '@/lib/swap/constants';
import { ALLOWED_MEDIA_ORIGINS, safeMediaPath } from './media-policy';
import { EXTERNAL_NAVIGATION_ORIGINS, validatedExternalUrl } from './navigation';

const REMOVED_ORIGINS = [
  'https://app.hyperpaxeer.com',
  'https://colosseum.hyperpaxeer.com',
  'https://dao.hyperpaxeer.com',
  'https://sidiora.fun',
  'https://cdn.redixusercontent.ocfstudio.com',
  'https://app.webpoints.app',
  'https://crossverse.app',
];

function removed_origins_are_rejected() {
  for (const origin of REMOVED_ORIGINS) {
    expect(EXTERNAL_NAVIGATION_ORIGINS.has(origin)).toBe(false);
    expect(ALLOWED_MEDIA_ORIGINS.has(origin)).toBe(false);
    expect(validatedExternalUrl(`${origin}/app`)).toBeNull();
    expect(safeMediaPath(`${origin}/icon.png`)).toBe('/wallet/default_icon.webp');
  }
  expect(validatedExternalUrl('https://paxscan.io/tx/0x01')?.origin).toBe('https://paxscan.io');
  expect(
    safeMediaPath('https://raw.githubusercontent.com/Paxeer-Network/brand/main/icon.png')
  ).toBe(
    '/wallet/api/media?url=https%3A%2F%2Fraw.githubusercontent.com%2FPaxeer-Network%2Fbrand%2Fmain%2Ficon.png'
  );
}

function usid_icon_is_a_bundled_public_file() {
  const usid = SWAP_TOKENS.find((token) => token.symbol === 'USID');
  expect(usid).toBeDefined();
  const icon = usid?.iconUrl;
  if (!icon) throw new Error('USID must declare a local icon');
  expect(icon).toMatch(/^\/wallet\/[A-Za-z0-9_/-]+\.webp$/);
  expect(safeMediaPath(icon)).toBe(icon);
  const publicRoot = path.resolve(__dirname, '../../../public');
  const asset = path.resolve(publicRoot, icon.slice('/wallet/'.length));
  expect(asset.startsWith(`${publicRoot}${path.sep}`)).toBe(true);
  expect(statSync(asset).isFile()).toBe(true);
  const bytes = readFileSync(asset);
  expect(bytes.subarray(0, 4).toString()).toBe('RIFF');
  expect(bytes.subarray(8, 12).toString()).toBe('WEBP');
}

async function candle_reports_its_retired_upstream() {
  const response = await GET();
  expect(response.status).toBe(503);
  expect(response.headers.get('cache-control')).toBe('no-store');
  expect(await response.json()).toEqual({
    error: {
      code: 'CANDLE_UPSTREAM_RETIRED',
      message: 'Candle data is unavailable because its upstream has been retired',
    },
  });
}

test(removed_origins_are_rejected.name, removed_origins_are_rejected);
test(usid_icon_is_a_bundled_public_file.name, usid_icon_is_a_bundled_public_file);
test(candle_reports_its_retired_upstream.name, candle_reports_its_retired_upstream);
