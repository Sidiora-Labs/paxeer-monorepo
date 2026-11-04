'use client';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/sonner';
import { Spinner } from '@/components/ui/spinner';
import { BRIDGE_CHAINS, bridgeChainByKey, paxeer, PAXEER_ROUTE_KEY, type BridgeChain } from '@/lib/bridge/chains';
import { importDeposit, importWithdrawal } from '@/lib/bridge/import';
import { solanaRecipientHandle } from '@/lib/bridge/solana';
import { listTransfers, type BridgeTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import type { FormEvent } from 'react';
import { useState } from 'react';
import { isHex } from 'viem';
import type { Address, TransactionReceipt } from 'viem';
import type { Config } from 'wagmi';
import { useAccount } from 'wagmi';
import { getTransactionReceipt } from 'wagmi/actions';

import { resolveWithdrawalVault } from './transfer-actions';
import { saveBridgeTransfer } from './transfer-ledger';

export function importTransferReceipt(chain: BridgeChain | undefined, receipt: TransactionReceipt, address: Address): {
  transfer: BridgeTransfer;
  existing: boolean;
  conflicts: Array<string>;
} {
  const transfer = chain ? importDeposit(chain, receipt) : importWithdrawal(receipt);
  return reconcileImportedTransfer(transfer, address);
}

function reconcileImportedTransfer(transfer: BridgeTransfer, address: Address) {
  if (transfer.account.toLowerCase() !== address.toLowerCase()) {
    throw new Error('Connect the wallet that started this transfer to import it.');
  }
  const existing = listTransfers().find((entry) => entry.id === transfer.id);
  const conflicts: Array<string> = [];
  if (existing) {
    for (const field of [ 'account', 'direction', 'foreignChainId', 'assetId', 'amount', 'recipient', 'bridgeOutNonce', 'logIndex', 'nonce', 'vault' ] as const) {
      const oldValue = existing[field];
      const newValue = transfer[field];
      const sameRecipient = field === 'recipient' && transfer.foreignChainKey === 'solana' &&
        solanaRecipientHandle(existing.recipient)?.toLowerCase() === transfer.recipient.toLowerCase();
      if (oldValue !== undefined && newValue !== undefined && String(oldValue).toLowerCase() !== String(newValue).toLowerCase() && !sameRecipient) conflicts.push(field);
    }
    if (conflicts.includes('account') || conflicts.includes('direction') ||
      (conflicts.length > 0 && (existing.settlementStatus === 'confirmed' || existing.claimedAt !== undefined || existing.releasedAt !== undefined))) {
      throw new Error(`Receipt conflicts with tracked settlement identity: ${ conflicts.join(', ') }. The saved record was retained.`);
    }
  }
  const tracked = saveBridgeTransfer(existing ? {
    ...existing,
    ...transfer,
    vault: transfer.vault ?? existing.vault,
    recipient: transfer.foreignChainKey === 'solana' && solanaRecipientHandle(existing.recipient)?.toLowerCase() === transfer.recipient.toLowerCase() ? existing.recipient : transfer.recipient,
    submittedAt: existing.submittedAt,
  } : transfer);
  return { transfer: tracked, existing: existing !== undefined, conflicts };
}

/** The import UI resolves registry-owned fields before committing any recovery. */
export async function recoverTransferReceipt(chain: BridgeChain | undefined, receipt: TransactionReceipt, address: Address, config?: Config) {
  const transfer = chain ? importDeposit(chain, receipt) : importWithdrawal(receipt);
  if (transfer.account.toLowerCase() !== address.toLowerCase()) throw new Error('Connect the wallet that started this transfer to import it.');
  return reconcileImportedTransfer(await resolveWithdrawalVault(transfer, config), address);
}

export function ImportTransfer({ onImported }: { onImported: () => void }) {
  const { address } = useAccount();
  const [ route, setRoute ] = useState(PAXEER_ROUTE_KEY);
  const [ hash, setHash ] = useState('');
  const [ pending, setPending ] = useState(false);
  const [ error, setError ] = useState<string>();

  const onSubmit = async(event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(undefined);
    const sourceHash = hash.trim();
    if (!isHex(sourceHash) || !/^0x[0-9a-fA-F]{64}$/.test(sourceHash)) {
      setError('Enter a source transaction hash with 64 hexadecimal characters after 0x.');
      return;
    }
    if (!address) {
      setError('Connect the wallet that started the transfer.');
      return;
    }
    setPending(true);
    try {
      const chain = bridgeChainByKey(route);
      const chainId = route === PAXEER_ROUTE_KEY ? paxeer.id : chain?.viemChain?.id;
      if (chainId === undefined) throw new Error('Select a configured source route.');
      const receipt = await getTransactionReceipt(wagmiConfig, { hash: sourceHash, chainId });
      const { existing, conflicts } = await recoverTransferReceipt(chain, receipt, address);
      onImported();
      setHash('');
      if (conflicts.length) toast.info('Receipt repaired saved metadata', { description: conflicts.join(', ') });
      toast.success(existing ? 'Transfer already tracked' : 'Transfer imported');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not import the source transaction.');
    } finally {
      setPending(false);
    }
  };

  return (
    <form onSubmit={ onSubmit } aria-label="Import a bridge transfer" className="flex flex-col gap-3 rounded-xl border border-border p-4">
      <p className="text-sm font-medium">Import a transfer started elsewhere</p>
      <label className="flex flex-col gap-1 text-xs">
        Source route
        <select
          value={ route }
          onChange={ (event) => setRoute(event.target.value) }
          disabled={ pending }
          className="h-10 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value={ PAXEER_ROUTE_KEY }>Paxeer → foreign chain (withdrawal)</option>
          { BRIDGE_CHAINS.filter((chain) => chain.kind === 'evm' && chain.vault && chain.viemChain).map((chain) => (
            <option key={ chain.key } value={ chain.key }>{ chain.label } → Paxeer (deposit)</option>
          )) }
        </select>
      </label>
      <label className="flex flex-col gap-1 text-xs">
        Source transaction hash
        <Input
          value={ hash }
          onChange={ (event) => setHash(event.target.value) }
          placeholder="0x…"
          autoComplete="off"
          spellCheck={ false }
          disabled={ pending }
          required
          aria-invalid={ Boolean(error) }
          aria-describedby={ error ? 'import-transfer-error' : undefined }
        />
      </label>
      { error ? <p id="import-transfer-error" role="alert" className="text-xs text-destructive">{ error }</p> : null }
      <Button type="submit" size="sm" className="self-start" disabled={ pending || !hash.trim() }>
        { pending ? <Spinner/> : null }
        { pending ? 'Importing…' : 'Import transfer' }
      </Button>
    </form>
  );
}
