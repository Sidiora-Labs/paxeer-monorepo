'use client';
import '@/widgets/rewards/rewards-presentation.css';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useAccount, usePublicClient, useWalletClient } from 'wagmi';
import { defineChain, formatEther, type Address } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { Coins } from 'lucide-react';
import { Button, EmptyState, ErrorState, Skeleton } from '@/ui/primitives';
import { userFacingMessage } from '@/core/user-facing-error';
import { VIRTUAL_CHAIN_ID_BASE } from '../create-chain/chain-domain';
import { homeChainClient } from '../trade/home-chain';
import { readChainRevenue, type ChainRevenueEpoch, type InitialChainRevenue } from './chain-revenue-reader';

export { readChainRevenue } from './chain-revenue-reader';
export type { ChainRevenueEpoch } from './chain-revenue-reader';

export function chainRevenueWriteRequest(chain: number, action: 'claim' | 'batch' | 'open', epochs: readonly bigint[] = []) {
  if (!Number.isInteger(chain) || chain < 1 || chain > 0xffffffff) throw new Error('Invalid launched chain');
  if (action === 'claim') {
    if (epochs.length !== 1) throw new Error('Choose one revenue epoch');
    return { address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'claimChainRevenue' as const, args: [chain, epochs[0]] as const };
  }
  if (action === 'batch') {
    if (epochs.length === 0) throw new Error('Choose revenue epochs');
    return { address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'claimChainRevenueBatch' as const, args: [chain, [...epochs]] as const };
  }
  if (epochs.length !== 0) throw new Error('Opening takes no epoch');
  return { address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'openChainRevenueEpoch' as const, args: [chain] as const };
}

export default function ChainRevenue({ poolAddress, initial }: { poolAddress: string; initial?: InitialChainRevenue }) {
  const { address, chainId } = useAccount();
  const root = usePublicClient({ chainId: 125 });
  const { data: wallet } = useWalletClient();
  const [data, setData] = useState<{ chain: number; epochs: ChainRevenueEpoch[] } | null>(!address && initial?.poolAddress === poolAddress ? initial.data : null);
  const [error, setError] = useState(!address && initial?.poolAddress === poolAddress ? initial.error : '');
  const initialOwner = useRef(initial?.poolAddress === poolAddress ? `${poolAddress}:` : null);
  const [pending, setPending] = useState(false);
  const [revision, setRevision] = useState(0);
  const validPool = /^0x[0-9a-fA-F]{40}$/.test(poolAddress);

  useEffect(() => {
    if (!root || !validPool) return;
    let active = true;
    const owner = `${poolAddress}:${address ?? ''}`;
    if (initialOwner.current !== owner) {
      const publicInitial = !address && initial?.poolAddress === poolAddress ? initial : null;
      setData(publicInitial?.data ?? null);
      setError(publicInitial?.error ?? '');
    }
    initialOwner.current = owner;
    readChainRevenue(root, poolAddress as Address, address).then(value => {
      if (active) { setData(value); setError(''); }
    }).catch(cause => {
      if (active) setError(userFacingMessage(cause, 'Revenue epochs could not be loaded.'));
    });
    return () => { active = false; };
  }, [root, poolAddress, validPool, address, revision, initial]);

  const submit = useCallback(async (action: 'claim' | 'batch' | 'open', epochs: readonly bigint[] = []) => {
    if (!wallet || !address || !chainId || !data?.chain || pending) return;
    setPending(true); setError('');
    try {
      const current = await wallet.getChainId();
      if (current !== chainId) throw new Error('Switch to the chain shown in your wallet to claim revenue.');
      const rpcBase = getPaxeerRpcUrl();
      const rpcUrl = chainId === 125 ? rpcBase : chainId > VIRTUAL_CHAIN_ID_BASE
        ? `${rpcBase}/chain/${chainId - VIRTUAL_CHAIN_ID_BASE}` : null;
      if (!rpcUrl) throw new Error('Switch to Paxeer or a launched chain to claim revenue.');
      const client = homeChainClient({ registryNumber: chainId === 125 ? 0 : chainId - VIRTUAL_CHAIN_ID_BASE, chainId, rpcUrl, walletNetwork: null });
      const chain = defineChain({ id: chainId, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [rpcUrl] } } });
      const request = chainRevenueWriteRequest(data.chain, action, epochs);
      const hash = request.functionName === 'claimChainRevenue'
        ? await wallet.writeContract({ ...request, account: address, chain })
        : request.functionName === 'claimChainRevenueBatch'
          ? await wallet.writeContract({ ...request, account: address, chain })
          : await wallet.writeContract({ ...request, account: address, chain });
      const receipt = await client.waitForTransactionReceipt({ hash });
      if (receipt.status !== 'success') throw new Error('Revenue transaction was not confirmed.');
      if (action !== 'open') {
        setData(currentData => currentData ? { ...currentData, epochs: currentData.epochs.map(row => epochs.includes(row.epoch) ? { ...row, claimed: true, claimable: 0n } : row) } : currentData);
      }
      setRevision(value => value + 1);
    } catch (cause) {
      setError(userFacingMessage(cause, 'Revenue transaction failed.'));
    } finally {
      setPending(false);
    }
  }, [wallet, address, chainId, data, pending]);

  if (!validPool || data?.chain === 0) return null;
  const open = data?.epochs.filter(row => !row.claimed && row.claimable !== null && row.claimable > 0n) ?? [];
  const latest = data?.epochs.at(-1);
  const mayOpen = !!data && (!latest || Date.now() / 1000 >= Number(latest.openedAt) + 3600);
  return (
    <section aria-labelledby="chain-revenue-title" className="rewards-presentation flex min-w-0 flex-col gap-2 rounded-lg bg-surface-1 p-3">
      <h2 id="chain-revenue-title" className="text-sm font-semibold text-text">Chain revenue</h2>
      {!data && !error ? <Skeleton variant="rows" rows={1} label="chain revenue" /> : null}
      {data?.epochs.length === 0 ? <EmptyState icon={Coins} title="No revenue epochs yet." body="Revenue appears here after the first epoch opens" /> : null}
      {data?.epochs.map(row => <div key={row.epoch.toString()} className="sl-hairline flex flex-wrap items-center justify-between gap-2 py-2 text-xs tabular-nums text-text">
        <span>Epoch {row.epoch.toString()} · {formatEther(row.amount)} PAX</span>
        {row.claimed ? <span>Claimed</span> : row.claimable !== null ? <span>{formatEther(row.claimable)} PAX claimable</span> : <span>Connect wallet to see your share</span>}
        {!row.claimed && row.claimable !== null && row.claimable > 0n ? <Button variant="secondary" size="sm" disabled={pending} onClick={() => void submit('claim', [row.epoch])}>Claim</Button> : null}
      </div>)}
      {open.length > 0 ? <Button variant="secondary" size="sm" disabled={pending} onClick={() => void submit('batch', open.map(row => row.epoch))}>Claim all open epochs</Button> : null}
      {mayOpen && address ? <Button variant="secondary" size="sm" disabled={pending} onClick={() => void submit('open')}>Open next epoch</Button> : null}
      {error ? <ErrorState title="Chain revenue" message={error} onRetry={() => setRevision(value => value + 1)} /> : null}
    </section>
  );
}
