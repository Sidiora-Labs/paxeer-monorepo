'use client';

import { kernelUnavailableFrom, type BankAsset, type BankBalance } from '@paxeer/wallet';
import { AmountText, ErrorState, Skeleton } from '@/components/ui/primitives';
import { units } from './format';
import { errorMessage, kernelStateGate, useAsync } from './hooks';
import { useAccountClients } from './AccountProvider';
import { KernelNotice } from './KernelNotice';

type AssetBalance = BankBalance & Pick<BankAsset, 'symbol' | 'decimals'>;

function Balances({ balances }: { balances: AssetBalance[] }) {
  return (
    <ul aria-label="Bank balances" className="list-separated">
      {balances.map(({ denom, amount, symbol, decimals }) => (
        <li key={denom} data-denom={denom} className="flex items-start justify-between gap-3 py-3">
          <div>
            <p className="text-sm font-semibold">{symbol}</p>
            <p className="text-[11px] text-pax-muted">{denom}</p>
          </div>
          <div className="text-right text-xs">
            <AmountText
              label={`${symbol} balance`}
              value={units(amount, decimals)}
              symbol={symbol}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}

export function AssetList({ account }: { account: string }) {
  const { endpoint } = useAccountClients();
  const balances = useAsync(async () => {
    const [bankBalances, assetPage] = await Promise.all([
      endpoint.getBankBalances(account),
      endpoint.listBankAssets(),
    ]);
    const assets = new Map(assetPage.assets.map((asset) => [asset.denom, asset]));
    return bankBalances.balances.map((balance): AssetBalance => {
      const asset = assets.get(balance.denom);
      if (!asset) throw new Error(`Asset metadata is unavailable for ${balance.denom}`);
      return { ...balance, symbol: asset.symbol, decimals: asset.decimals };
    });
  }, `balances:${account.toLowerCase()}`);
  const unavailable = balances.status === 'error' ? kernelUnavailableFrom(balances.error) : null;

  return (
    <section aria-label="Assets" className="space-y-3 rounded-[20px] bg-pax-surface p-4">
      <h2 className="text-sm font-bold">Assets</h2>
      {balances.status === 'loading' && <Skeleton className="h-24 w-full" />}
      {balances.status === 'ready' && <Balances balances={balances.value} />}
      {unavailable && <KernelNotice gate={kernelStateGate(unavailable)} />}
      {balances.status === 'error' && !unavailable && (
        <ErrorState
          title="Balances could not be read"
          message={errorMessage(balances.error)}
          onRetry={balances.reload}
        />
      )}
    </section>
  );
}
