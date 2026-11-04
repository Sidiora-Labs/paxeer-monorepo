'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { exchange, type IntentLeg, type ModuleTransaction } from '@paxeer/wallet';
import { SegmentedControl, TextField } from '@/components/ui/primitives';
import { TOKENS } from '@/lib/constants';
import {
  buildPerpsPosition,
  perpsCollateralApproval,
  perpsCollateralEndpoint,
  readPerpsCollateral,
  requirePerpsCollateral,
  type PerpsCollateral,
} from './perpsCollateral';
import { FeeChoice } from './FeeChoice';
import {
  BuildError,
  SendButton,
  SentResult,
  SurfaceFrame,
  TransactionPreview,
  buildOrError,
} from './Surface';
import {
  errorMessage,
  parseSurfaceAmount,
  parseSurfaceId,
  useFeeSelection,
  useModuleSend,
  useSurfaceWallet,
} from './useSurface';

export type ExchangeAction = 'open' | 'close' | 'cancel' | 'deposit' | 'withdraw';

const ACTIONS: readonly { value: ExchangeAction; label: string }[] = [
  { value: 'open', label: 'Open' },
  { value: 'close', label: 'Close' },
  { value: 'cancel', label: 'Cancel' },
  { value: 'deposit', label: 'Deposit' },
  { value: 'withdraw', label: 'Withdraw' },
];

const SIDES: readonly { value: 'long' | 'short'; label: string }[] = [
  { value: 'long', label: 'Long' },
  { value: 'short', label: 'Short' },
];

const usdlToken = TOKENS.USDL;
if (!usdlToken?.address) throw new Error('the USDL collateral address is not configured');
export const USDL = { address: usdlToken.address, decimals: usdlToken.decimals };

export interface ExchangeViewProps {
  readonly sidRate: string | null;
  readonly collateralEndpoint?: string | null;
  readonly legs?: readonly IntentLeg[];
}

interface Built {
  readonly tx: ModuleTransaction;
  readonly pull: bigint;
}

export function word(id: string): string {
  const value = parseSurfaceId(id);
  return `0x${value.toString(16).padStart(64, '0')}`;
}

export function ExchangeView({ sidRate, legs, collateralEndpoint }: ExchangeViewProps) {
  const { provider, address } = useSurfaceWallet();
  const surfaceModule = useMemo(() => (provider ? exchange(provider) : null), [provider]);
  const fee = useFeeSelection(provider, address);
  const [action, setAction] = useState<ExchangeAction>('open');
  const [marketId, setMarketId] = useState('');
  const [side, setSide] = useState<'long' | 'short'>('long');
  const endpoint = useMemo(
    () =>
      buildOrError(() =>
        collateralEndpoint === undefined
          ? perpsCollateralEndpoint(process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL)
          : collateralEndpoint
      ),
    [collateralEndpoint]
  );
  const [configuration, setConfiguration] = useState<{
    provider: typeof provider;
    endpoint: string | null;
    tokens: readonly PerpsCollateral[];
    error: string | null;
  } | null>(null);
  const [collateralToken, setCollateralToken] = useState('');
  useEffect(() => {
    let alive = true;
    setConfiguration(null);
    setCollateralToken('');
    if (!provider) return;
    readPerpsCollateral(endpoint.value).then(
      (tokens) => {
        if (alive) {
          setConfiguration({ provider, endpoint: endpoint.value, tokens, error: null });
          setCollateralToken(tokens[0]!.address);
        }
      },
      (cause: unknown) => {
        if (alive)
          setConfiguration({
            provider,
            endpoint: endpoint.value,
            tokens: [],
            error: errorMessage(cause),
          });
      }
    );
    return () => {
      alive = false;
    };
  }, [provider, endpoint.value]);
  const tokens =
    configuration?.provider === provider && configuration?.endpoint === endpoint.value
      ? configuration.tokens
      : null;
  const collateral = tokens?.find((token) => token.address === collateralToken) ?? null;
  const collateralError =
    endpoint.error ??
    configuration?.error ??
    (tokens === null ? 'Reading perps collateral configuration.' : null);
  const [collateralAmount, setCollateralAmount] = useState('');
  const [leverage, setLeverage] = useState('');
  const [id, setId] = useState('');
  const [amount, setAmount] = useState('');

  const built = useMemo(() => {
    if (!surfaceModule) return { value: null, error: null };
    return buildOrError<Built | null>(() => {
      switch (action) {
        case 'open': {
          if (!marketId || !collateralAmount || !leverage) return null;
          if (!tokens)
            throw new Error(collateralError ?? 'Perps collateral configuration is unavailable.');
          return buildPerpsPosition(surfaceModule, tokens, {
            marketId,
            collateralToken,
            collateralAmount,
            leverage,
            isLong: side === 'long',
          });
        }
        case 'close':
          return id ? { tx: surfaceModule.closePosition(parseSurfaceId(id)), pull: 0n } : null;
        case 'cancel':
          return id ? { tx: surfaceModule.cancelOrder(parseSurfaceId(id)), pull: 0n } : null;
        case 'deposit': {
          if (!amount) return null;
          if (!tokens)
            throw new Error(collateralError ?? 'Perps collateral configuration is unavailable.');
          const asset = requirePerpsCollateral(tokens, collateralToken);
          const pull = parseSurfaceAmount(amount, asset.decimals);
          return { tx: surfaceModule.deposit(asset.address, pull), pull };
        }
        case 'withdraw': {
          if (!amount) return null;
          if (!tokens)
            throw new Error(collateralError ?? 'Perps collateral configuration is unavailable.');
          const asset = requirePerpsCollateral(tokens, collateralToken);
          return {
            tx: surfaceModule.withdraw(asset.address, parseSurfaceAmount(amount, asset.decimals)),
            pull: 0n,
          };
        }
      }
    });
  }, [
    surfaceModule,
    tokens,
    collateralError,
    action,
    marketId,
    side,
    collateralToken,
    collateralAmount,
    leverage,
    id,
    amount,
  ]);
  const transaction = built.value?.tx ?? null;
  const pull = built.value?.pull ?? 0n;
  const approval = useCallback(
    async (owner: string, tx: ModuleTransaction) => {
      if (action === 'close' || action === 'cancel') return undefined;
      if (!surfaceModule || !provider || !collateral)
        throw new Error('Perps collateral configuration is unavailable.');
      return perpsCollateralApproval(
        surfaceModule,
        provider,
        endpoint.value,
        owner,
        tx,
        collateral
      );
    },
    [surfaceModule, provider, collateral, endpoint.value, action]
  );
  const state = useModuleSend(surfaceModule, provider, address, fee, approval);

  return (
    <SurfaceFrame title="Exchange" connected={surfaceModule !== null}>
      <SegmentedControl
        label="Exchange action"
        value={action}
        options={ACTIONS}
        onChange={setAction}
      />
      {action === 'open' && (
        <>
          <TextField
            label="Market id"
            name="marketId"
            value={marketId}
            onChange={(e) => setMarketId(e.target.value)}
          />
          <SegmentedControl label="Side" value={side} options={SIDES} onChange={setSide} />
          <label className="block text-sm text-pax-light">
            Collateral asset
            <select
              name="collateralToken"
              aria-label="Collateral asset"
              value={collateralToken}
              disabled={!tokens?.length}
              onChange={(event) => setCollateralToken(event.target.value)}
            >
              {(tokens ?? []).map((token) => (
                <option key={token.address} value={token.address}>
                  {token.symbol} · {token.address}
                </option>
              ))}
            </select>
          </label>
          <TextField
            label={`Collateral amount (${collateral?.symbol ?? 'asset unavailable'})`}
            name="collateralAmount"
            value={collateralAmount}
            onChange={(e) => setCollateralAmount(e.target.value)}
          />
          <TextField
            label="Leverage (x)"
            name="leverage"
            value={leverage}
            onChange={(e) => setLeverage(e.target.value)}
          />
        </>
      )}
      {(action === 'close' || action === 'cancel') && (
        <TextField
          label={action === 'close' ? 'Position id' : 'Order id'}
          name="id"
          value={id}
          onChange={(e) => setId(e.target.value)}
        />
      )}
      {(action === 'deposit' || action === 'withdraw') && (
        <TextField
          label={`Amount (${collateral?.symbol ?? 'asset unavailable'})`}
          name="amount"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
        />
      )}
      {pull > 0n && (
        <p data-role="approve-step" className="text-xs text-pax-muted">
          {state.approving
            ? `Approving the exact ${collateral?.symbol} amount for the perps venue`
            : `If the perps venue allowance is short, one approve for the exact ${collateral?.symbol} amount is sent first`}
        </p>
      )}
      {collateralError && (
        <p role="status" data-role="collateral-status">
          {collateralError}
        </p>
      )}
      <BuildError error={built.error} />
      <TransactionPreview transaction={transaction} />
      <FeeChoice
        provider={provider}
        address={address}
        transaction={transaction}
        selection={fee}
        sidRate={sidRate}
        legs={legs}
      />
      <SendButton
        label="Send to the perps venue"
        transaction={transaction}
        blocked={fee.blocked}
        state={state}
      />
      <SentResult state={state} />
    </SurfaceFrame>
  );
}
