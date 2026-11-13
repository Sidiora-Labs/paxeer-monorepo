'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAccount, usePublicClient, useSwitchChain, useWalletClient } from 'wagmi';
import { formatEther, formatUnits, parseAbi, type PublicClient, type WalletClient } from 'viem';
import { Button } from '@/ui/primitives';
import { poolPath } from '@/core/app-foundation/navigation';
import { PROTOCOL_CONFIG_ADDRESS } from '@/core/network/contracts';
import { useAuthModal } from '@/shell/auth-modal-store';
import type { AccountScope } from '../create/creation-adapters';
import { LaunchProgress, type LaunchStep } from '../create/launch-progress';
import { ChainLaunchAdapter, type ChainLaunchResult, type ChainLaunchStep, type ChainSubmission } from './chain-adapters';
import { SIZE_TIERS, chainCost, validateChainDraft, validateChainName, type ChainDraft, type ChainsParams, type SizeTier } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';
import { StrategyPicker, feeStrategyLabel } from '../create/strategy-picker';
import { NAME_MAX_BYTES, SYMBOL_MAX_BYTES } from '../create/create-form-fields';

const marketFeeAbi = parseAbi(['function creationFee() view returns (uint256)']);

const tierNames: Record<SizeTier, string> = { starter: 'Starter', growth: 'Growth', pro: 'Pro' };
const tiers: SizeTier[] = ['starter', 'growth', 'pro'];
const nameRule = 'Use 3 to 32 lowercase letters, numbers or hyphens. Start and end with a letter or number.';

export default function ChainWizard() {
  const { address, chainId } = useAccount();
  const client = usePublicClient();
  const { data: wallet } = useWalletClient();
  const { switchChainAsync } = useSwitchChain();
  const { openModal } = useAuthModal();
  const scope = useRef<AccountScope>({ creator: address?.toLowerCase() ?? '', chainId: chainId ?? 0, epoch: 0 });
  if (scope.current.creator !== (address?.toLowerCase() ?? '') || scope.current.chainId !== (chainId ?? 0)) {
    scope.current = { creator: address?.toLowerCase() ?? '', chainId: chainId ?? 0, epoch: scope.current.epoch + 1 };
  }
  const adapter = useMemo(() => client && wallet
    ? new ChainLaunchAdapter(client as PublicClient, wallet as WalletClient, () => scope.current) : null, [client, wallet]);
  const storage = useMemo(() => address ? new ChainDraftStorage(address) : null, [address]);
  const [draft, setDraft] = useState<ChainDraft | null>(null);
  const [name, setName] = useState('');
  const [tokenName, setTokenName] = useState('');
  const [tokenSymbol, setTokenSymbol] = useState('');
  const [feeStrategy, setFeeStrategy] = useState<ChainDraft['feeStrategy']>(0);
  const [tier, setTier] = useState<SizeTier>('starter');
  const [step, setStep] = useState(0);
  const [availability, setAvailability] = useState<{ name: string; state: 'checking' | 'available' | 'taken' | 'error' } | null>(null);
  const [params, setParams] = useState<ChainsParams | null>(null);
  const [marketFee, setMarketFee] = useState<bigint | null>(null);
  const [paramsError, setParamsError] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [submission, setSubmission] = useState<ChainSubmission | null>(null);
  const [validation, setValidation] = useState(0);
  const [recoveryBlocked, setRecoveryBlocked] = useState(false);
  const [phase, setPhase] = useState<ChainLaunchStep | null>(null);
  const [result, setResult] = useState<ChainLaunchResult | null>(null);
  const launchLock = useRef(false);
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  useEffect(() => {
    setStep(0); setResult(null); setPhase(null); setError(''); setNotice('');
    setName(''); setTokenName(''); setTokenSymbol(''); setFeeStrategy(0);
    setTier('starter'); setDraft(null); setSubmission(null); setRecoveryBlocked(false);
    let checkingSubmission = true;
    try {
      const saved = ChainDraftStorage.restoreSubmission();
      checkingSubmission = false;
      if (saved && saved.submission.phase === 'confirm') {
        setDraft(saved.draft); setName(saved.draft.name); setTokenName(saved.draft.tokenName);
        setTokenSymbol(saved.draft.tokenSymbol); setFeeStrategy(saved.draft.feeStrategy); setTier(saved.draft.tier);
        setSubmission(saved.submission); setPhase('confirm'); setStep(2);
        return;
      }
      if (!storage || !address) return;
      const restored = saved?.submission.phase === 'reverted' && saved.submission.creator === address.toLowerCase()
        ? saved.draft : storage.restoreDraft();
      const next: ChainDraft = restored ?? {
        draftId: crypto.randomUUID(), version: 2, creator: address.toLowerCase(), chainId: 125,
        name: '', tokenName: '', tokenSymbol: '', feeStrategy: 0,
        tier: 'starter', feeShareBps: 0, updatedAt: Date.now(),
      };
      setDraft(next); setName(next.name); setTokenName(next.tokenName);
      setTokenSymbol(next.tokenSymbol); setFeeStrategy(next.feeStrategy); setTier(next.tier);
    } catch (cause) {
      setRecoveryBlocked(checkingSubmission);
      setError(cause instanceof Error ? cause.message : 'Your saved draft could not be loaded.');
    }
  }, [storage, address, chainId]);

  useEffect(() => {
    if (!draft || !storage || draft.creator !== address?.toLowerCase() || result || submission || recoveryBlocked) return;
    try {
      storage.saveDraft({ ...draft, name, tokenName, tokenSymbol, feeStrategy, tier, updatedAt: Date.now() });
    } catch {
      setError('Your draft could not be saved. Check that browser storage is enabled.');
    }
  }, [draft, storage, address, name, tokenName, tokenSymbol, feeStrategy, tier, result, submission, recoveryBlocked]);

  useEffect(() => {
    let active = true;
    setParams(null); setMarketFee(null); setParamsError('');
    if (adapter && chainId === 125) {
      Promise.all([adapter.readParams(), adapter.client.readContract({
        address: PROTOCOL_CONFIG_ADDRESS, abi: marketFeeAbi, functionName: 'creationFee',
      })]).then(([value, fee]) => { if (active) { setParams(value); setMarketFee(fee); } })
        .catch(() => { if (active) setParamsError('Chain costs could not be loaded. Reload to try again.'); });
    }
    return () => { active = false; };
  }, [adapter, chainId, validation]);

  useEffect(() => {
    let active = true;
    setAvailability(null);
    if (!adapter || chainId !== 125 || validateChainName(name)) return;
    setAvailability({ name, state: 'checking' });
    adapter.isNameAvailable(name).then(available => {
      if (active) setAvailability({ name, state: available ? 'available' : 'taken' });
    }).catch(() => { if (active) setAvailability({ name, state: 'error' }); });
    return () => { active = false; };
  }, [adapter, chainId, name, validation]);

  const nameAvailable = availability?.name === name && availability.state === 'available';
  const launchFieldsValid = !!draft && Object.keys(validateChainDraft({
    ...draft, name, tokenName, tokenSymbol, feeStrategy, tier, updatedAt: Date.now(),
  }, Date.now())).length === 0;
  const tierAvailable = (value: SizeTier) => {
    if (!params) return false;
    try { chainCost(params, value, '0'); return true; } catch { return false; }
  };
  const connected = !!address && chainId === 125 && !!adapter;
  const submissionMatches = !!submission && submission.creator === address?.toLowerCase() && submission.chainId === chainId;
  const locked = busy || !!submission || phase === 'confirm' || !!result || recoveryBlocked;

  async function launch(retry = false) {
    if (launchLock.current || !adapter || !draft || !connected || recoveryBlocked) return;
    if (retry ? !submissionMatches : !!submission || phase === 'confirm' || !nameAvailable || !launchFieldsValid || !tierAvailable(tier) || marketFee === null) return;
    launchLock.current = true;
    const expectedEpoch = scope.current.epoch;
    const reviewed = retry ? { ...draft } : { ...draft, name, tokenName, tokenSymbol, feeStrategy, tier, updatedAt: Date.now() };
    let sent = retry ? submission : null;
    setBusy(true); setError(''); setPhase(retry ? 'confirm' : 'create');
    try {
      const report = (next: ChainLaunchStep, saved?: ChainSubmission) => {
        if (saved) {
          sent = saved;
          if (mounted.current) {
            setSubmission(saved); setDraft(reviewed); setName(reviewed.name);
            setTokenName(reviewed.tokenName); setTokenSymbol(reviewed.tokenSymbol);
            setFeeStrategy(reviewed.feeStrategy); setTier(reviewed.tier);
            setPhase('confirm'); setStep(2);
          }
          storage?.saveSubmission(reviewed, saved);
        }
        if (mounted.current && scope.current.epoch === expectedEpoch) setPhase(next);
      };
      const outcome = retry && sent
        ? await adapter.confirm(sent.txHash, reviewed, sent, report)
        : await adapter.launch(reviewed, report);
      if ('submission' in outcome) {
        sent = outcome.submission;
        try { storage?.saveSubmission(reviewed, sent); }
        catch { if (mounted.current) setNotice('Your submitted transaction could not be saved. Keep this page open and retain the transaction hash.'); }
        if (!mounted.current || scope.current.epoch !== expectedEpoch) return;
        setError(outcome.error);
        if (sent.phase === 'reverted') {
          setSubmission(null); setPhase(null); setStep(0); setAvailability(null); setParams(null);
          setValidation(value => value + 1);
        } else {
          setSubmission(sent); setPhase('confirm');
        }
        return;
      }
      try { storage?.clearSubmission(outcome.txHash); storage?.clearDraft(); }
      catch { if (mounted.current) setNotice('Your chain is live, but the saved draft could not be cleared.'); }
      if (!mounted.current || scope.current.epoch !== expectedEpoch) return;
      setSubmission(null); setResult(outcome); setPhase('ready');
    } catch (cause) {
      if (mounted.current && scope.current.epoch === expectedEpoch) {
        setPhase(sent ? 'confirm' : null);
        setError(cause instanceof Error ? cause.message : 'The chain could not be launched. Try again.');
      }
    } finally {
      launchLock.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  async function readyAction(action: 'copy' | 'wallet') {
    if (!result) return;
    setNotice('');
    try {
      if (action === 'copy') {
        await navigator.clipboard.writeText(result.rpcUrl);
        setNotice('RPC URL copied.');
      } else {
        if (!wallet) throw new Error('Connect your wallet to add this chain.');
        await wallet.request({ method: 'wallet_addEthereumChain', params: [result.walletNetwork] });
        setNotice('Chain added to your wallet.');
      }
    } catch {
      setNotice(action === 'copy' ? 'Could not copy the RPC URL. Select and copy it below.' : 'The chain could not be added. Check your wallet and try again.');
    }
  }

  const progress: LaunchStep[] = [
    { id: 'create', label: 'Confirm in your wallet', status: phase === 'create' ? 'active' : phase ? 'done' : 'pending' },
    { id: 'confirm', label: 'Waiting for the block', status: phase === 'confirm' ? error ? 'error' : 'active' : phase === 'ready' ? 'done' : 'pending' },
    { id: 'ready', label: 'Your chain and token are live', status: result ? 'done' : 'pending' },
  ];

  return (
    <div className="mx-auto flex w-full max-w-content-narrow-max-width flex-col gap-4 px-4 py-4 lg:px-6">
      <h1 className="text-md font-semibold text-text">Launch a chain and token</h1>
      <p className="text-sm text-text-muted">Name your chain and token, pick a size, and launch them together.</p>
      <ol aria-label="Launch a chain steps" className="flex gap-4 text-sm text-text-muted">
        {['Name', 'Size', 'Review and launch'].map((label, index) => <li key={label} aria-current={step === index ? 'step' : undefined}>{index + 1}. {label}</li>)}
      </ol>
      {!address ? <Button onClick={() => openModal('Connect your wallet to launch a chain')}>Connect wallet</Button>
        : chainId !== 125 ? <Button onClick={() => void switchChainAsync({ chainId: 125 }).catch(() => setError('Switch to Paxeer in your wallet to continue.'))}>Switch to Paxeer</Button> : null}
      <section className="flex flex-col gap-4 rounded-lg bg-surface-1 p-4">
        {step === 0 ? <>
          <h2 className="text-base font-semibold text-text">Name your chain</h2>
          <label htmlFor="chain-name" className="text-sm font-bold text-text">Chain name</label>
          <input id="chain-name" value={name} maxLength={32} disabled={locked} autoCapitalize="none" autoCorrect="off" spellCheck={false}
            aria-describedby="chain-name-rule chain-name-status" aria-invalid={!!name && (!!validateChainName(name) || availability?.state === 'taken') || undefined}
            className="rounded-sm bg-surface-2 p-3 text-text" onChange={event => setName(event.target.value)} />
          <p id="chain-name-rule" className="text-xs text-text-muted">{nameRule}</p>
          <p id="chain-name-status" role="status" className="text-sm text-text-muted">
            {name && validateChainName(name) ? 'Check the name rules above.' : availability?.name === name
              ? { checking: 'Checking availability…', available: 'This name is available.', taken: 'This name is taken. Choose another.', error: 'Name availability could not be checked. Edit the name to try again.' }[availability.state]
              : !connected ? 'Connect your wallet on Paxeer to check availability.' : ''}
          </p>
          <h2 className="text-base font-semibold text-text">Name your token</h2>
          <label htmlFor="chain-token-name" className="text-sm font-bold text-text">Coin name</label>
          <input id="chain-token-name" value={tokenName} maxLength={NAME_MAX_BYTES} disabled={locked}
            className="rounded-sm bg-surface-2 p-3 text-text" onChange={event => setTokenName(event.target.value)} />
          <p className="text-xs text-text-muted">Name must be 1 to {NAME_MAX_BYTES} bytes.</p>
          <label htmlFor="chain-token-symbol" className="text-sm font-bold text-text">Ticker</label>
          <input id="chain-token-symbol" value={tokenSymbol} maxLength={SYMBOL_MAX_BYTES} disabled={locked}
            autoCapitalize="none" autoCorrect="off" spellCheck={false}
            className="rounded-sm bg-surface-2 p-3 text-text" onChange={event => setTokenSymbol(event.target.value)} />
          <p className="text-xs text-text-muted">Ticker must be 1 to {SYMBOL_MAX_BYTES} bytes.</p>
          <StrategyPicker value={feeStrategy} onChange={setFeeStrategy} disabled={locked} />
          <Button variant="primary" disabled={!nameAvailable || !launchFieldsValid || !connected || locked} onClick={() => setStep(1)}>Next</Button>
        </> : step === 1 ? <>
          <h2 className="text-base font-semibold text-text">Pick a size</h2>
          <div role="radiogroup" aria-label="Chain size" className="grid gap-3 sm:grid-cols-3">
            {tiers.map(value => <label key={value} className="flex cursor-pointer flex-col gap-2 rounded-md bg-surface-2 p-3 text-sm text-text">
              <span><input type="radio" name="chain-size" value={value} checked={tier === value} disabled={locked || !tierAvailable(value)} onChange={() => setTier(value)} /> {tierNames[value]}</span>
              <span>Up to {SIZE_TIERS[value].gas.toLocaleString('en-US')} gas and {SIZE_TIERS[value].bytes.toLocaleString('en-US')} bytes per block.</span>
              {params && !tierAvailable(value) ? <span>Currently unavailable</span> : null}
            </label>)}
          </div>
          <Button variant="primary" disabled={!tierAvailable(tier) || locked} onClick={() => setStep(2)}>Next</Button>
        </> : <>
          <h2 className="text-base font-semibold text-text">{result ? 'Your chain and token are live' : 'Review and launch'}</h2>
          <p className="text-sm text-text">{name} · {tierNames[tier]}</p>
          <p className="text-sm text-text">{tokenName} ({tokenSymbol}) · {feeStrategyLabel(feeStrategy)}</p>
          <p className="text-xs text-text-muted">Up to {SIZE_TIERS[tier].gas.toLocaleString('en-US')} gas and {SIZE_TIERS[tier].bytes.toLocaleString('en-US')} bytes per block.</p>
          {!result ? <Button variant="primary" disabled={!connected || !launchFieldsValid || !nameAvailable || !tierAvailable(tier) || marketFee === null || locked} onClick={() => void launch()}>Launch chain and token</Button> : null}
          {submission ? <div className="text-sm text-text" role="region" aria-label="Submitted transaction">
            <p>Transaction: {submission.txHash}</p>
            <p>Creator: {submission.creator} · Root chain: {submission.chainId}</p>
            {!submissionMatches ? <p role="alert">This submitted transaction belongs to a different account or chain. Reconnect its creator on the root chain to confirm it.</p> : null}
            <Button disabled={busy || !connected || !submissionMatches} onClick={() => void launch(true)}>Retry confirmation</Button>
          </div> : null}
          {phase ? <LaunchProgress steps={progress} busy={busy} /> : null}
          {result ? <div className="flex flex-col gap-3 text-sm text-text" role="region" aria-label="Live chain">
            <p>Token: <strong>{result.tokenName} ({result.tokenSymbol})</strong> · {result.token}</p>
            <p>Pool: {result.pool}</p>
            <p>Chain ID: <strong>{result.chainId}</strong></p>
            <p>Registry ID: {result.id}</p>
            <label htmlFor="chain-rpc">Chain node path</label>
            <input id="chain-rpc" readOnly value={result.rpcUrl} className="w-full rounded-sm bg-surface-2 p-2 font-mono text-xs" />
            <div className="flex flex-wrap gap-2">
              <Button onClick={() => void readyAction('copy')}>Copy RPC URL</Button>
              <Button onClick={() => void readyAction('wallet')}>Add to wallet</Button>
              <a href={result.explorerUrl} target="_blank" rel="noopener noreferrer" className="text-accent underline">View explorer<span className="sr-only">, opens in a new tab</span></a>
              <Button variant="link" href={poolPath(125, result.pool)}>Trade token</Button>
            </div>
          </div> : null}
        </>}
        {step > 0 && !locked ? <Button variant="ghost" onClick={() => setStep(previous => previous - 1)}>Back</Button> : null}
        {address && !draft && !busy && !submission && !result && !recoveryBlocked ? <Button onClick={() => {
          setDraft({ draftId: crypto.randomUUID(), version: 2, creator: address.toLowerCase(), chainId: 125,
            name, tokenName, tokenSymbol, feeStrategy, tier, feeShareBps: 0, updatedAt: Date.now() });
          if (recoveryBlocked) return;
          setError('');
        }}>Start a fresh draft</Button> : null}
        {step > 0 && !result ? params ? <div className="text-sm text-text-muted">
          <p>Bond: {formatEther(params[1])} PAX</p>
          <p>Creation fee: {formatEther(params[2])} PAX</p>
          <p>Total: {formatEther(params[1] + params[2])} PAX, plus network gas shown in your wallet.</p>
          {marketFee !== null ? <p>Market creation charge: {formatUnits(marketFee, 6)} USDL from your approved balance.</p> : null}
        </div> : <p role="status">{paramsError || 'Loading chain costs…'}</p> : null}
        {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
        {notice ? <p role="status" className="text-sm text-text-muted">{notice}</p> : null}
      </section>
    </div>
  );
}
