import { test } from 'vitest';
import { assertSponsoredSubmitEvidence } from './support/wallet-send-evidence.js';

async function sponsoredSubmitReturnsSequencerReceipt(): Promise<void> {
  await assertSponsoredSubmitEvidence();
}

test(sponsoredSubmitReturnsSequencerReceipt, sponsoredSubmitReturnsSequencerReceipt);
