import { readFileSync } from 'node:fs';
import path from 'node:path';
import type { ChainAccount, KernelAvailabilityState, KernelUnavailableState } from '@paxeer/wallet';
import { expect, expectTypeOf, test } from 'vitest';
import {
  type AsyncState,
  kernelGate,
  kernelReasonText,
  kernelStateGate,
  useAccountDocument,
  useKernelState,
} from './hooks';

function account_hooks_report_the_chain_node_and_its_consensus_reason(): void {
  expectTypeOf(useAccountDocument).returns.toEqualTypeOf<
    AsyncState<ChainAccount> & { readonly reload: () => void }
  >();
  expectTypeOf(useKernelState).returns.toEqualTypeOf<
    AsyncState<KernelAvailabilityState> & { readonly reload: () => void }
  >();

  expect(kernelGate({ status: 'loading' })).toEqual({
    open: false,
    reason: 'Checking the chain node state',
    checking: true,
  });
  expect(kernelGate({ status: 'error', error: new Error('connection refused') })).toEqual({
    open: false,
    reason: 'The endpoint did not report the chain node state: connection refused',
  });
  expect(kernelGate({ status: 'error', error: null })).toEqual({
    open: false,
    reason: 'The endpoint did not report the chain node state: the request failed',
  });

  const available: KernelAvailabilityState = { available: true, reason: 'available' };
  expect(kernelReasonText(available)).toBe('The chain node is available');
  expect(kernelStateGate(available)).toEqual({ open: true });
  expect(kernelGate({ status: 'ready', value: available })).toEqual({ open: true });

  const reasons: readonly [KernelUnavailableState['reason'], string][] = [
    ['not_configured', 'The chain node is not configured on the endpoint'],
    ['unreachable', 'The chain node is unreachable'],
    ['consensus_unavailable', 'The chain node is catching up or unreachable.'],
  ];
  for (const [reason, text] of reasons) {
    for (const backend of [null, 'public_core'] as const) {
      const state: KernelUnavailableState = { available: false, reason, backend };
      const expected =
        backend !== null && reason !== 'consensus_unavailable' ? `${text} (public core)` : text;
      expect(kernelReasonText(state)).toBe(expected);
      expect(kernelStateGate(state)).toEqual({ open: false, reason: expected });
      expect(kernelGate({ status: 'ready', value: state })).toEqual({
        open: false,
        reason: expected,
      });
    }
  }
  expect(readFileSync(path.join(__dirname, 'hooks.ts'), 'utf8')).not.toMatch(/LayerX kernel/i);
}

test(account_hooks_report_the_chain_node_and_its_consensus_reason);
