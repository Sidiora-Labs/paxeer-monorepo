'use client';

import { useState } from 'react';
import { Button, FormField } from '@/ui/primitives';
import type { CreationController } from './creation-controller';

export function PurchaseRecoveryControl({
  controller,
  intentId,
  busy,
  perform,
}: {
  controller: CreationController;
  intentId: string;
  busy: boolean;
  perform: (action: () => Promise<unknown>) => Promise<void>;
}) {
  const [txHash, setTxHash] = useState('');
  return (
    <div
      className="flex flex-col gap-3"
      role="group"
      aria-label="Recover first purchase"
    >
      <p className="text-xs text-text-muted">
        Your first purchase may already have been sent. Open your wallet
        transaction history on Paxeer, find the buy for this pool, and attach
        its transaction hash. Keep this purchase locked until its receipt is
        verified.
      </p>
      <FormField
        id="create-purchase-recovery-hash"
        label="Purchase transaction hash"
        inputProps={{
          value: txHash,
          disabled: busy,
          onChange: (event) => setTxHash(event.target.value),
          autoComplete: 'off',
          spellCheck: false,
        }}
      />
      <Button
        variant="secondary"
        size="sm"
        disabled={busy || !/^0x[0-9a-fA-F]{64}$/.test(txHash)}
        onClick={() =>
          void perform(() =>
            controller.attachRecoveredPurchaseHash(intentId, txHash),
          )
        }
      >
        Verify purchase hash
      </Button>
    </div>
  );
}
