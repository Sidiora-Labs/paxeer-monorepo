'use client';

import { useState, useCallback } from 'react';
import Link from 'next/link';
import OrderForm from '@/widgets/meta-ag/order-form';
import OpenOrdersList from '@/widgets/meta-ag/open-orders-list';

export default function MetaAgOrdersPage() {
  // Bumping this counter on a successful place tells OpenOrdersList to
  // refetch its index lists immediately rather than waiting for wagmi's
  // default polling interval. Keeps the placed-→listed loop tight.
  const [refreshCounter, setRefreshCounter] = useState(0);
  const handlePlaced = useCallback(() => {
    setRefreshCounter((n) => n + 1);
  }, []);

  return (
    <main className="mx-auto min-h-dvh max-w-content-narrow-max-width px-4 pb-20 pt-6">
      <div className="mb-5">
        <div className="flex items-baseline gap-3">
          <h1 className="text-lg font-bold text-text">Orders</h1>
        </div>
        <p className="text-xs text-text-faint mt-1">
          Limit and stop orders are not available yet.{' '}
          <Link prefetch={false} href="/terminal">Open the terminal to trade</Link>
        </p>
      </div>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <div>
          <OrderForm onPlaced={handlePlaced} />
          <KeeperHealthHint />
        </div>
        <div>
          <OpenOrdersList refreshSignal={refreshCounter} />
        </div>
      </div>
    </main>
  );
}

function KeeperHealthHint() {
  return (
    <div className="mt-3 px-3 py-2.5 rounded-xl bg-surface-2 text-2xs text-text-faint space-y-1">
      <div className="font-bold uppercase tracking-wider text-text mb-1">How it works</div>
      <p>
        When orders become available, funds will stay locked until the order
        trades, expires or is cancelled. A trigger does not guarantee execution.
      </p>
      <div className="pt-1.5 mt-1.5 flex justify-between">
        <span>Order bot</span>
        <span className="font-mono text-text">Not available yet</span>
      </div>
      <div className="flex justify-between">
        <span>Orders contract</span>
        <span className="font-mono text-text">Not available yet</span>
      </div>
    </div>
  );
}
