'use client';
import { useEffect, useRef, useState } from 'react';
import { encodeFunctionData } from 'viem';
import type { Order, TransactionRequest } from '../../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';
import { assertAccount, type AccountSession } from '@/widgets/profile/account-transport';
import { advancedOrdersAbi, type AdvancedProvider, type ConnectedOrder } from '@/widgets/meta-ag/advanced-provider';
import { Button, NoticeBar } from '@/ui/primitives';
import { userFacingMessage } from '@/core/user-facing-error';

export class OrderCancellationUnavailableError extends Error {}

/** Bind the API preparation to the reviewed provider's owned on-chain order. */
export async function admitOrderCancellation(request: TransactionRequest, order: Order, scope: AccountSession, provider?: AdvancedProvider): Promise<ConnectedOrder> {
  assertAccount(scope);
  if (!provider?.manifest.orders || provider.blockers.has('orders')) throw new OrderCancellationUnavailableError('Wallet cancellation is not available yet');
  if (request.chainId !== '125' || request.from.toLowerCase() !== scope.session.wallet.toLowerCase() || order.wallet.toLowerCase() !== scope.session.wallet.toLowerCase()) throw new Error('Cancellation signer changed');
  if (request.to.toLowerCase() !== provider.manifest.orders.address.toLowerCase() || request.valueRaw !== '0') throw new Error('Cancellation contract or value differs from the reviewed order');
  if (!order.chainOrderId || !/^[1-9][0-9]*$/.test(order.chainOrderId) || order.kind === 'scheduled') throw new OrderCancellationUnavailableError('This order has no supported wallet cancellation');
  let cursor: string | null = null;
  const cursors = new Set<string>();
  do {
    const page = await provider.orderPage(cursor);
    assertAccount(scope);
    const item = page.items.find(item => item.orderId === order.chainOrderId && item.owner.toLowerCase() === order.wallet.toLowerCase() && (order.kind === 'limit' ? item.type === 'limit-buy' || item.type === 'limit-sell' : item.type === 'stop-loss' || item.type === 'stop-limit-buy' || item.type === 'stop-limit-sell'));
    if (item) {
      if (item.amountRaw.toString() !== order.inputRaw || !['active', 'triggered'].includes(item.state)) throw new Error('Order changed before cancellation review');
      const data = encodeFunctionData({ abi: advancedOrdersAbi, functionName: item.family === 'limit' ? 'cancelLimitOrder' : 'cancelStopLimitOrder', args: [BigInt(item.orderId)] });
      if (request.data.toLowerCase() !== data.toLowerCase()) throw new Error('Prepared cancellation does not match the reviewed order');
      return item;
    }
    cursor = page.nextCursor;
    if (cursor !== null && cursors.has(cursor)) throw new Error('Order pagination did not advance');
    if (cursor !== null) cursors.add(cursor);
  } while (cursor !== null);
  throw new Error('Prepared cancellation order was not found in the connected wallet');
}

export function OrderCancellationReview({ request, order, scope, provider, onConfirmed, onDismiss }: {
  request: TransactionRequest;
  order: Order;
  scope: AccountSession;
  provider?: AdvancedProvider;
  onConfirmed(outcome: 'cancelled' | 'filled_race'): void;
  onDismiss(): void;
}) {
  const [review, setReview] = useState<ConnectedOrder | null>(null);
  const [status, setStatus] = useState('Checking the wallet cancellation…');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [hash, setHash] = useState<string | null>(null);
  const active = useRef(false);
  const invoked = useRef(false);
  const confirmed = useRef(false);
  useEffect(() => {
    let live = true;
    setReview(null); setError(null);
    void admitOrderCancellation(request, order, scope, provider).then(item => {
      if (live) { setReview(item); setStatus('Review this cancellation before continuing in your wallet. Your order may trade before cancellation confirms.'); }
    }).catch(error => {
      if (live) { setStatus(''); setError(error instanceof OrderCancellationUnavailableError ? error.message : userFacingMessage(error)); }
    });
    return () => { live = false; };
  }, [request, order, scope, provider]);

  const submit = async () => {
    if (!review || !provider || active.current || confirmed.current) return;
    active.current = true; setBusy(true); setError(null);
    try {
      assertAccount(scope);
      // The reviewed provider simulates and rechecks the exact admitted ABI call,
      // retains its recovery hash and refuses another dispatch while unresolved.
      const pending = await provider.pendingCancel(review);
      assertAccount(scope);
      setStatus(pending ? 'Checking your cancellation…' : 'Confirm the cancellation in your wallet.');
      invoked.current = true;
      const attempt = pending ?? await provider.cancel(review);
      setHash(attempt.txHash);
      assertAccount(scope);
      setStatus('Cancellation sent. Checking the confirmed outcome…');
      const result = await provider.recoverCancel(review);
      assertAccount(scope);
      if (result.outcome.state === 'cancelled' || result.outcome.state === 'filled_race') {
        confirmed.current = true;
        onConfirmed(result.outcome.state);
      } else {
        setStatus(result.outcome.state === 'checking' ? 'Your cancellation is still being checked.' : 'The cancellation failed. Your order has not been cancelled.');
        if (result.outcome.state === 'cancel_failed') { invoked.current = false; setReview(null); }
      }
    } catch (error) {
      setStatus('');
      setError(invoked.current ? 'The cancellation outcome is unresolved. Check its status before trying another cancellation.' : userFacingMessage(error));
    } finally { active.current = false; setBusy(false); }
  };

  return (
    <section aria-label="Wallet cancellation review" className="flex flex-col gap-3">
      <dl className="break-all text-xs text-text-muted">
        <dt>Wallet</dt><dd>{request.from}</dd>
        <dt>Orders contract</dt><dd>{request.to}</dd>
        <dt>Network</dt><dd>Paxeer (125)</dd>
        <dt>Order</dt><dd>{order.orderId}</dd>
      </dl>
      {status ? <p role="status">{status}</p> : null}
      {hash ? <p className="break-all text-xs">Transaction {hash}</p> : null}
      {error ? <NoticeBar tone="warning" announce>{error}</NoticeBar> : null}
      <div className="flex justify-end gap-2">
        {!invoked.current ? <Button variant="secondary" size="sm" disabled={busy} onClick={onDismiss}>Keep order</Button> : null}
        {review ? <Button variant="danger" size="sm" loading={busy} onClick={() => { void submit(); }}>{invoked.current ? 'Check cancellation status' : 'Continue in wallet'}</Button> : null}
      </div>
    </section>
  );
}
