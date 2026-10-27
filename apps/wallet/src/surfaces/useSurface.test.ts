// @vitest-environment jsdom
import { expect, test } from 'vitest';
import * as surface from './useSurface';

function use_surface_exports_no_receipt_polling() {
  expect(surface).not.toHaveProperty('waitForReceipt');
  expect(surface).not.toHaveProperty('RECEIPT_ATTEMPTS');
  expect(surface).not.toHaveProperty('RECEIPT_INTERVAL_MS');
}

test(use_surface_exports_no_receipt_polling);
