'use client';

import { useCallback, useMemo, useState } from 'react';
import { parseUnits } from 'viem';
import { EndpointClient, exchange, type IntentLeg, type ModuleTransaction } from '@paxeer/wallet';
import { SegmentedControl, TextField } from '@/components/ui/primitives';
import { TOKENS } from '@/lib/constants';
import { useAsync } from '@/account/hooks';
import { FeeChoice } from './FeeChoice';
import {
  BuildError,
  SendButton,
  SentResult,
  SurfaceFrame,
  TransactionPreview,
  buildOrError,
} from './Surface';
import { useFeeSelection, useModuleSend, useSurfaceWallet } from './useSurface';

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
  readonly legs?: readonly IntentLeg[];
}

interface Built {
  readonly tx: ModuleTransaction;
  readonly pull: bigint;
}

function word(id: string): string {
  if (!/^(0|[1-9][0-9]*)$/.test(id)) throw new Error('enter a decimal id');
  const value = BigInt(id);
  if (value >= 1n << 256n) throw new Error('the id exceeds 32 bytes');
  return `0x${value.toString(16).padStart(64, '0')}`;
}

export function ExchangeView({ sidRate, legs }: ExchangeViewProps) {
  const { provider, address } = useSurfaceWallet();
  const surfaceModule = useMemo(() => (provider ? exchange(provider) : null), [provider]);
  const fee = useFeeSelection(provider, address);
  const endpoint = useMemo(
    () => new EndpointClient({ url: process.env.NEXT_PUBLIC_PAXEER_RPC_URL }),
    []
  );
  const document = useAsync(
    () => (address ? endpoint.resolveAccount(address) : Promise.resolve(null)),
    `exchange-account:${address ?? ''}`
  );
  const account =
    document.status === 'ready' &&
    document.value !== null &&
    address !== null &&
    document.value.evm_address?.toLowerCase() === address?.toLowerCase()
      ? document.value.layerx_account
      : null;
  const accountId = account && /^[0-9a-fA-F]{64}$/.test(account) ? `0x${account}` : null;
  const [action, setAction] = useState<ExchangeAction>('open');
  const [marketId, setMarketId] = useState('');
  const [side, setSide] = useState<'long' | 'short'>('long');
  const [size, setSize] = useState('');
  const [leverage, setLeverage] = useState('');
  const [id, setId] = useState('');
  const [amount, setAmount] = useState('');

  const built = useMemo(() => {
    if (!surfaceModule) return { value: null, error: null };
    return buildOrError<Built | null>(() => {
      switch (action) {
        case 'open': {
          if (!marketId || !size || !leverage) return null;
          if (!accountId) throw new Error('the account has no resolved LayerX main account');
          const tx = surfaceModule.openPosition({
            marketId: word(marketId),
            accountId,
            side: side === 'long' ? 1 : 2,
            size: parseUnits(size, 18),
            leverage: parseUnits(leverage, 18),
          });
          return { tx, pull: 0n };
        }
        case 'close':
          return id ? { tx: surfaceModule.closePosition(BigInt(id)), pull: 0n } : null;
        case 'cancel': {
          if (!id) return null;
          if (!accountId) throw new Error('the account has no resolved LayerX main account');
          return { tx: surfaceModule.cancelOrder(word(id), accountId), pull: 0n };
        }
        case 'deposit': {
          if (!amount) return null;
          const pull = parseUnits(amount, USDL.decimals);
          return { tx: surfaceModule.deposit(USDL.address, pull), pull };
        }
        case 'withdraw':
          return amount
            ? {
                tx: surfaceModule.withdraw(USDL.address, parseUnits(amount, USDL.decimals)),
                pull: 0n,
              }
            : null;
      }
    });
  }, [surfaceModule, action, marketId, side, size, leverage, id, amount, accountId]);
  const transaction = built.value?.tx ?? null;
  const pull = built.value?.pull ?? 0n;
  const approval = useCallback(
    async (owner: string) =>
      surfaceModule && pull > 0n ? surfaceModule.approval(owner, USDL.address, pull) : undefined,
    [surfaceModule, pull]
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
          <TextField
            label="Size"
            name="size"
            value={size}
            onChange={(e) => setSize(e.target.value)}
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
          label="Amount (USDL)"
          name="amount"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
        />
      )}
      {pull > 0n && (
        <p data-role="approve-step" className="text-xs text-pax-muted">
          {state.approving
            ? 'Approving the exact USDL amount for the perps venue'
            : 'If the perps venue allowance is short, one approve for the exact USDL amount is sent first'}
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
