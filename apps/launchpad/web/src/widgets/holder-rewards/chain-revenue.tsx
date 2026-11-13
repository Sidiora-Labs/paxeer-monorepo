'use client';

import { useCallback, useEffect, useState } from 'react';
import { useAccount, usePublicClient, useWalletClient } from 'wagmi';
import { defineChain, formatEther, type Address, type PublicClient } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { VIRTUAL_CHAIN_ID_BASE } from '../create-chain/chain-domain';
import { homeChainClient } from '../trade/home-chain';

export interface ChainRevenueEpoch {
  epoch: bigint;
  amount: bigint;
  claimable: bigint | null;
  openedAt: bigint;
  claimed: boolean;
}

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

export async function readChainRevenue(root: PublicClient, pool: Address, holder: Address | undefined): Promise<{ chain: number; epochs: ChainRevenueEpoch[] }> {
  const chain = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'launchedChainOfPool', args: [pool] });
  if (chain === 0) return { chain: 0, epochs: [] };
  const epochs: ChainRevenueEpoch[] = [];
  for (let epoch = 1n; ; epoch++) {
    let record: readonly [number, bigint, bigint, bigint, bigint, bigint, bigint];
    try {
      record = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'chainRevenueEpoch', args: [chain, epoch] });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (/NotLaunchedChain/i.test(message)) break;
      throw error;
    }
    let isClaimed = false;
    let claimable: bigint | null = null;
    if (holder) {
      try {
        claimable = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'chainRevenueClaimable', args: [chain, epoch, holder] });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (/AlreadyClaimed/i.test(message)) { isClaimed = true; claimable = 0n; }
        else if (/ZeroAmount/i.test(message)) claimable = 0n;
        else throw error;
      }
    }
    epochs.push({ epoch, amount: record[2], claimable, openedAt: record[6], claimed: isClaimed });
  }
  return { chain, epochs };
}

export default function ChainRevenue({ poolAddress }: { poolAddress: string }) {
  const { address, chainId } = useAccount();
  const root = usePublicClient({ chainId: 125 });
  const { data: wallet } = useWalletClient();
  const [data, setData] = useState<{ chain: number; epochs: ChainRevenueEpoch[] } | null>(null);
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const [revision, setRevision] = useState(0);
  const validPool = /^0x[0-9a-fA-F]{40}$/.test(poolAddress);

  useEffect(() => {
    if (!root || !validPool) return;
    let active = true;
    setData(null);
    readChainRevenue(root, poolAddress as Address, address).then(value => {
      if (active) { setData(value); setError(''); }
    }).catch(cause => {
      if (active) setError(cause instanceof Error ? cause.message : 'Revenue epochs could not be loaded.');
    });
    return () => { active = false; };
  }, [root, poolAddress, validPool, address, revision]);

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
      setError(cause instanceof Error ? cause.message : 'Revenue transaction failed.');
    } finally {
      setPending(false);
    }
  }, [wallet, address, chainId, data, pending]);

  if (!validPool || (!data && !error) || data?.chain === 0) return null;
  const open = data?.epochs.filter(row => !row.claimed && row.claimable !== null && row.claimable > 0n) ?? [];
  const latest = data?.epochs.at(-1);
  const mayOpen = !!data && (!latest || Date.now() / 1000 >= Number(latest.openedAt) + 3600);
  return (
    <section aria-labelledby="chain-revenue-title" className="flex flex-col gap-2 rounded-md bg-surface-1 p-3">
      <h2 id="chain-revenue-title" className="text-sm font-bold text-text">Chain revenue</h2>
      {data?.epochs.length === 0 ? <p className="text-xs text-text-muted">No revenue epochs yet.</p> : null}
      {data?.epochs.map(row => <div key={row.epoch.toString()} className="flex items-center justify-between gap-2 text-xs text-text">
        <span>Epoch {row.epoch.toString()} · {formatEther(row.amount)} PAX</span>
        {row.claimed ? <span>Claimed</span> : row.claimable !== null ? <span>{formatEther(row.claimable)} PAX claimable</span> : <span>Connect wallet to see your share</span>}
        {!row.claimed && row.claimable !== null && row.claimable > 0n ? <button disabled={pending} onClick={() => void submit('claim', [row.epoch])}>Claim</button> : null}
      </div>)}
      {open.length > 0 ? <button disabled={pending} onClick={() => void submit('batch', open.map(row => row.epoch))}>Claim all open epochs</button> : null}
      {mayOpen && address ? <button disabled={pending} onClick={() => void submit('open')}>Open next epoch</button> : null}
      {error ? <p role="alert" className="text-xs text-danger">{error}</p> : null}
    </section>
  );
}
