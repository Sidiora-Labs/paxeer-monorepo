'use client';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/sonner';
import { Spinner } from '@/components/ui/spinner';
import { BRIDGE_CHAINS, bridgeChainByKey, paxeer, PAXEER_ROUTE_KEY, type BridgeChain } from '@/lib/bridge/chains';
import { importDeposit, importWithdrawal } from '@/lib/bridge/import';
import { listTransfers, saveTransfer, type BridgeTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import type { FormEvent } from 'react';
import { useState } from 'react';
import { isHex } from 'viem';
import type { Address, TransactionReceipt } from 'viem';
import { useAccount } from 'wagmi';
import { getTransactionReceipt } from 'wagmi/actions';

export function importTransferReceipt(chain: BridgeChain | undefined, receipt: TransactionReceipt, address: Address): {
  transfer: BridgeTransfer;
  existing: boolean;
} {
  const transfer = chain ? importDeposit(chain, receipt) : importWithdrawal(receipt);
  if (transfer.account.toLowerCase() !== address.toLowerCase()) {
    throw new Error('Connect the wallet that started this transfer to import it.');
  }
  const existing = listTransfers(address).find((entry) => entry.id === transfer.id);
  const tracked = existing ? {
    ...existing,
    vault: existing.vault ?? transfer.vault,
    logIndex: transfer.logIndex ?? existing.logIndex,
    nonce: transfer.nonce ?? existing.nonce,
    bridgeOutNonce: transfer.bridgeOutNonce ?? existing.bridgeOutNonce,
  } : transfer;
  saveTransfer(tracked);
  return { transfer: tracked, existing: existing !== undefined };
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
      const { existing } = importTransferReceipt(chain, receipt, address);
      onImported();
      setHash('');
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
