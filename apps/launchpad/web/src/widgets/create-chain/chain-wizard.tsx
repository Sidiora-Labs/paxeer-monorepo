'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAccount, usePublicClient, useSwitchChain, useWalletClient } from 'wagmi';
import { formatEther, formatUnits, parseAbi, type PublicClient, type WalletClient } from 'viem';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button, FormField } from '@/ui/primitives';
import { poolPath } from '@/core/app-foundation/navigation';
import { PROTOCOL_CONFIG_ADDRESS } from '@/core/network/contracts';
import { useAuthModal } from '@/shell/auth-modal-store';
import type { AccountScope } from '../create/creation-adapters';
import { LaunchProgress, type LaunchStep } from '../create/launch-progress';
import { ChainLaunchAdapter, type ChainLaunchResult, type ChainLaunchStep, type ChainSubmission } from './chain-adapters';
import { SIZE_TIERS, chainCost, validateChainDraft, validateChainName, type ChainDraft, type ChainsParams, type SizeTier } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';
import { ChainFeeAdapter, type ChainFeeApproval, type ChainFeeFunding } from './chain-fees';
import { StrategyPicker, feeStrategyLabel } from '../create/strategy-picker';
import { InlineCounter, NAME_MAX_BYTES, SYMBOL_MAX_BYTES, byteLength } from '../create/create-form-fields';

const marketFeeAbi = parseAbi(['function creationFee() view returns (uint256)']);

const tierNames: Record<SizeTier, string> = { starter: 'Starter', growth: 'Growth', pro: 'Pro' };
const tiers: SizeTier[] = ['starter', 'growth', 'pro'];
const nameRule = 'Use 3 to 32 lowercase letters, numbers or hyphens. Start and end with a letter or number.';

export function ChainLaunchSummary({
  name, tokenName, tokenSymbol, tier, feeStrategy, params, marketFee,
}: Pick<ChainDraft, 'name' | 'tokenName' | 'tokenSymbol' | 'tier' | 'feeStrategy'> & {
  params: ChainsParams | null;
  marketFee: bigint | null;
}) {
  return (
    <section role="region" aria-labelledby="chain-summary-title" className="flex min-w-0 flex-col gap-3">
      <h2 id="chain-summary-title" className="text-md font-semibold text-text">Launch summary</h2>
      <dl className="flex flex-col gap-4 rounded-lg bg-surface-1 p-4 text-sm text-text [overflow-wrap:anywhere]">
        <div className="flex flex-col gap-1">
          <dt className="text-xs text-text-muted">Chain name</dt>
          <dd>{name || <span className="text-text-muted">Not named yet</span>}</dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-xs text-text-muted">Size</dt>
          <dd>{tierNames[tier]}<span className="block text-xs text-text-muted">Up to {SIZE_TIERS[tier].gas.toLocaleString('en-US')} gas and {SIZE_TIERS[tier].bytes.toLocaleString('en-US')} bytes per block.</span></dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-xs text-text-muted">Coin</dt>
          <dd>{tokenName || <span className="text-text-muted">Coin name</span>} ({tokenSymbol || <span className="text-text-muted">Ticker</span>})</dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-xs text-text-muted">Send creator rewards to</dt>
          <dd>{feeStrategyLabel(feeStrategy)}</dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-xs text-text-muted">Launch cost</dt>
          <dd>{params ? <>{formatEther(params[1] + params[2])} PAX</> : <span className="text-text-muted">Awaiting chain costs</span>}</dd>
          <dd>{marketFee !== null ? <>{formatUnits(marketFee, 6)} USDL</> : <span className="text-text-muted">Awaiting market fee</span>}</dd>
          <dd className="text-xs text-text-muted">Plus network gas shown in your wallet.</dd>
        </div>
      </dl>
    </section>
  );
}

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
  const feeAdapter = useMemo(() => client && wallet
    ? new ChainFeeAdapter(client as PublicClient, wallet as WalletClient, () => scope.current) : null, [client, wallet]);
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
  const [recoveryValidation, setRecoveryValidation] = useState(0);
  const [feeFunding, setFeeFunding] = useState<ChainFeeFunding | null>(null);
  const [feeError, setFeeError] = useState('');
  const [approval, setApproval] = useState<ChainFeeApproval | null>(null);
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
    setTier('starter'); setDraft(null); setSubmission(null); setApproval(null); setRecoveryBlocked(false);
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
      checkingSubmission = true;
      const savedApproval = storage.restoreFeeApproval();
      checkingSubmission = false;
      setApproval(savedApproval);
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
  }, [storage, address, chainId, recoveryValidation]);

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
        .catch(() => { if (active) setParamsError('Chain costs could not be loaded. Try again.'); });
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

  useEffect(() => {
    let active = true;
    setFeeFunding(null); setFeeError('');
    if (!feeAdapter || chainId !== 125 || step !== 2 || submission || result) return;
    feeAdapter.read({ ...scope.current }).then(value => {
      if (active) { setFeeFunding(value); setMarketFee(value.fee); }
    }).catch(cause => {
      if (active) setFeeError(cause instanceof Error ? cause.message : 'Market fee funding could not be loaded.');
    });
    return () => { active = false; };
  }, [feeAdapter, address, chainId, step, submission, result, validation]);

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
  const locked = busy || !!submission || !!approval || phase === 'confirm' || !!result || recoveryBlocked;

  async function approveFee(retry = false) {
    if (launchLock.current || !feeAdapter || !connected || !storage || recoveryBlocked || submission || result) return;
    if (retry ? !approval : !!approval || !feeFunding) return;
    launchLock.current = true;
    const expected = { ...scope.current };
    let sent = approval;
    setBusy(true); setError(''); setNotice('');
    try {
      if (!retry) sent = await feeAdapter.approve(expected, feeFunding!.fee, saved => {
        sent = saved;
        if (mounted.current && scope.current.epoch === expected.epoch) setApproval(saved);
        try { storage.saveFeeApproval(saved); }
        catch { if (mounted.current) setNotice('Your fee approval could not be saved. Keep this page open and retain the transaction hash.'); }
      });
      if (!sent) return;
      const outcome = await feeAdapter.confirm(sent, expected);
      storage.clearFeeApproval(sent.txHash);
      if (!mounted.current || scope.current.epoch !== expected.epoch) return;
      setApproval(null);
      setValidation(value => value + 1);
      if (outcome === 'reverted') setError('Market fee approval reverted. Review the fee and try again.');
      else setNotice('Market fee approved. Review and launch your chain and token.');
    } catch (cause) {
      if (mounted.current && scope.current.epoch === expected.epoch) {
        setError(cause instanceof Error ? cause.message : 'The market fee could not be approved. Try again.');
      }
    } finally {
      launchLock.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  async function launch(retry = false) {
    if (launchLock.current || !adapter || !feeAdapter || !draft || !connected || recoveryBlocked || approval) return;
    if (retry ? !submissionMatches : !!submission || phase === 'confirm' || !nameAvailable || !launchFieldsValid || !tierAvailable(tier) || marketFee === null) return;
    launchLock.current = true;
    const expectedEpoch = scope.current.epoch;
    const reviewed = retry ? { ...draft } : { ...draft, name, tokenName, tokenSymbol, feeStrategy, tier, updatedAt: Date.now() };
    let sent = retry ? submission : null;
    setBusy(true); setError(''); setPhase(retry ? 'confirm' : 'create');
    try {
      if (!retry) {
        const funding = await feeAdapter.read({ ...scope.current });
        if (scope.current.epoch !== expectedEpoch) throw new Error('Wallet changed. Review this launch again.');
        setFeeFunding(funding); setMarketFee(funding.fee);
        if (funding.fee !== marketFee) throw new Error('The market fee changed. Review it again.');
        if (funding.balance < funding.fee) throw new Error('Add USDL to cover the market creation fee.');
        if (funding.allowance < funding.fee) throw new Error('Approve the USDL market fee before launching.');
      }
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
    <div className="@container/create mx-auto w-full max-w-content-narrow-max-width px-4 py-4 lg:px-6">
      <div className="flex flex-col gap-4 @market-two/create:flex-row @market-two/create:items-start @market-two/create:gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-4 @market-two/create:max-w-form-max-width">
          <h1 className="text-md font-semibold text-text">Launch a chain and token</h1>
          <p className="text-sm text-text-muted">Name your chain and token, pick a size, and launch them together.</p>
          <ol aria-label="Launch a chain steps" className="flex flex-wrap gap-x-4 gap-y-2 text-sm text-text-muted">
            {['Name', 'Size', 'Review and launch'].map((label, index) => (
              <li key={label} aria-current={step === index ? 'step' : undefined} data-step-state={index < step ? 'done' : index === step ? 'current' : 'pending'} className="flex min-w-0 items-center gap-2">
                <span className={cn('flex size-7 shrink-0 items-center justify-center rounded-pill text-xs font-bold tabular-nums', index === step ? 'bg-accent text-on-accent' : index < step ? 'bg-accent-soft-strong text-accent' : 'bg-surface-3 text-text-muted')}>{index + 1}</span>
                <span className="sr-only">. </span>
                <span className={cn('min-w-0', index === step && 'font-bold text-text underline underline-offset-[var(--sl-focus-offset)]')}>{label}</span>
                {index < step ? <span className="inline-flex items-center gap-1 text-text-muted"><Check aria-hidden="true" className="size-icon-sm" /><span className="sr-only">, </span>done</span> : <span className="sr-only">{index === step ? ', current step' : ', not started'}</span>}
              </li>
            ))}
          </ol>
          {!address ? <Button type="button" onClick={() => openModal('Connect your wallet to launch a chain')}>Connect wallet</Button>
            : chainId !== 125 ? <Button type="button" onClick={() => void switchChainAsync({ chainId: 125 }).catch(() => setError('Switch to Paxeer in your wallet to continue.'))}>Switch to Paxeer</Button> : null}
          <section className="flex min-w-0 flex-col gap-4 rounded-lg bg-surface-1 p-4 [overflow-wrap:anywhere]">
            {step === 0 ? <>
              <h2 className="text-base font-semibold text-text">Name your chain</h2>
              <FormField
                id="chain-name"
                label="Chain name"
                describedBy="chain-name-rule chain-name-status"
                invalid={!!name && (!!validateChainName(name) || availability?.state === 'taken')}
                inputProps={{ name: 'chainName', autoComplete: 'off', value: name, maxLength: 32, disabled: locked, autoCapitalize: 'none', autoCorrect: 'off', spellCheck: false, placeholder: 'Name your chain', onChange: event => setName(event.target.value) }}
              />
              <p id="chain-name-rule" className="text-xs text-text-muted">{nameRule}</p>
              <p id="chain-name-status" role="status" className="text-sm text-text-muted">
                {name && validateChainName(name) ? 'Check the name rules above.' : availability?.name === name
                  ? { checking: 'Checking availability…', available: 'This name is available.', taken: 'This name is taken. Choose another.', error: 'Name availability could not be checked. Try again.' }[availability.state]
                  : !connected ? 'Connect your wallet on Paxeer to check availability.' : ''}
              </p>
              {availability?.state === 'error' ? <Button disabled={busy} onClick={() => setValidation(value => value + 1)}>Retry name check</Button> : null}
              <h2 className="text-base font-semibold text-text">Name your token</h2>
              <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--sl-form-field-min-width)),1fr))] gap-4">
                <FormField
                  id="chain-token-name"
                  label="Coin name"
                  helper={<>Name must be 1 to {NAME_MAX_BYTES} bytes.</>}
                  describedBy={tokenName ? 'chain-token-name-count' : undefined}
                  trailing={<InlineCounter id="chain-token-name-count" count={byteLength(tokenName)} max={NAME_MAX_BYTES} />}
                  inputProps={{ name: 'tokenName', autoComplete: 'off', value: tokenName, maxLength: NAME_MAX_BYTES, disabled: locked, placeholder: 'Name your coin', onChange: event => setTokenName(event.target.value) }}
                />
                <FormField
                  id="chain-token-symbol"
                  label="Ticker"
                  helper={<>Ticker must be 1 to {SYMBOL_MAX_BYTES} bytes.</>}
                  describedBy={tokenSymbol ? 'chain-token-symbol-count' : undefined}
                  trailing={<InlineCounter id="chain-token-symbol-count" count={byteLength(tokenSymbol)} max={SYMBOL_MAX_BYTES} />}
                  inputProps={{ name: 'tokenSymbol', autoComplete: 'off', value: tokenSymbol, maxLength: SYMBOL_MAX_BYTES, disabled: locked, autoCapitalize: 'none', autoCorrect: 'off', spellCheck: false, placeholder: 'Add a coin ticker (e.g. PAXE)', onChange: event => setTokenSymbol(event.target.value) }}
                />
              </div>
              <StrategyPicker value={feeStrategy} onChange={setFeeStrategy} disabled={locked} />
            </> : step === 1 ? <>
              <h2 className="text-base font-semibold text-text">Pick a size</h2>
              <div role="radiogroup" aria-label="Chain size" className="grid gap-3 sm:grid-cols-3">
                {tiers.map(value => (
                  <label key={value} data-selected={tier === value} className={cn('flex min-w-0 flex-col gap-2 rounded-md p-3 text-sm', tier === value ? 'bg-accent-soft-strong' : 'bg-surface-2', !tierAvailable(value) ? 'cursor-not-allowed text-text-muted' : 'cursor-pointer text-text')}>
                    <span className="flex min-h-touch-target-min flex-wrap items-center gap-2"><input className="size-touch-target-min shrink-0 accent-accent" type="radio" name="chain-size" value={value} checked={tier === value} disabled={locked || !tierAvailable(value)} onChange={() => setTier(value)} /> {tierNames[value]}</span>
                    {tier === value ? <span className="inline-flex items-center gap-1 font-bold"><Check aria-hidden="true" className="size-icon-sm" />Selected</span> : null}
                    <span>Up to {SIZE_TIERS[value].gas.toLocaleString('en-US')} gas and {SIZE_TIERS[value].bytes.toLocaleString('en-US')} bytes per block.</span>
                    {params && !tierAvailable(value) ? <span>Currently unavailable</span> : null}
                  </label>
                ))}
              </div>
            </> : <>
              <h2 className="text-base font-semibold text-text">{result ? 'Your chain and token are live' : 'Review and launch'}</h2>
              <dl className="flex min-w-0 flex-col gap-3 text-sm text-text [overflow-wrap:anywhere]">
                <div><dt className="text-xs text-text-muted">Chain and size</dt><dd>{name} · {tierNames[tier]}</dd></div>
                <div><dt className="text-xs text-text-muted">Coin and creator rewards</dt><dd>{tokenName} ({tokenSymbol}) · {feeStrategyLabel(feeStrategy)}</dd></div>
                <div><dt className="text-xs text-text-muted">Block capacity</dt><dd className="text-xs text-text-muted">Up to {SIZE_TIERS[tier].gas.toLocaleString('en-US')} gas and {SIZE_TIERS[tier].bytes.toLocaleString('en-US')} bytes per block.</dd></div>
              </dl>
              {submission ? <div className="text-sm text-text [overflow-wrap:anywhere]" role="region" aria-label="Submitted transaction">
                <p>Transaction: {submission.txHash}</p>
                <p>Creator: {submission.creator} · Root chain: {submission.chainId}</p>
                {!submissionMatches ? <p role="alert">This submitted transaction belongs to a different account or chain. Reconnect its creator on the root chain to confirm it.</p> : null}
                <Button disabled={busy || !connected || !submissionMatches} onClick={() => void launch(true)}>Retry confirmation</Button>
              </div> : null}
              {phase ? <LaunchProgress steps={progress} busy={busy} /> : null}
              {result ? <div className="flex min-w-0 flex-col gap-3 text-sm text-text [overflow-wrap:anywhere]" role="region" aria-label="Live chain">
                <p>Token: <strong>{result.tokenName} ({result.tokenSymbol})</strong> · {result.token}</p>
                <p>Pool: {result.pool}</p>
                <p>Chain ID: <strong>{result.chainId}</strong></p>
                <p>Registry ID: {result.id}</p>
                <label htmlFor="chain-rpc">Chain node path</label>
                <input id="chain-rpc" name="rpcUrl" autoComplete="off" readOnly value={result.rpcUrl} className="w-full rounded-sm bg-surface-2 p-2 font-mono text-xs" />
                <div className="flex flex-wrap gap-2">
                  <Button onClick={() => void readyAction('copy')}>Copy RPC URL</Button>
                  <Button onClick={() => void readyAction('wallet')}>Add to wallet</Button>
                  <a href={result.explorerUrl} target="_blank" rel="noopener noreferrer" className="text-accent underline">View explorer<span className="sr-only">, opens in a new tab</span></a>
                  <Button variant="link" href={poolPath(125, result.pool)}>Trade token</Button>
                </div>
              </div> : null}
            </>}
            {approval ? <div role="region" aria-label="Submitted market fee approval">
              <p>Market fee approval transaction: {approval.txHash}</p>
              {busy ? <p role="status">Waiting for market fee approval confirmation…</p> : null}
              <Button disabled={busy || !connected} onClick={() => void approveFee(true)}>Retry fee confirmation</Button>
            </div> : null}
            {step === 2 && !submission && !result ? <>
              {feeFunding && feeFunding.balance < feeFunding.fee ? <p role="alert">Add USDL to cover the market creation fee, then refresh the launch details.</p> : null}
              {feeFunding && feeFunding.allowance < feeFunding.fee && !approval ? <Button disabled={busy || !connected || feeFunding.balance < feeFunding.fee} onClick={() => void approveFee()}>Approve USDL market fee</Button> : null}
              {!feeFunding ? <p role="status">{feeError || (!connected ? 'Connect your wallet on Paxeer to read market fee funding.' : 'Loading market fee funding…')}</p> : null}
              <Button disabled={busy || !connected} onClick={() => setValidation(value => value + 1)}>Refresh launch details</Button>
            </> : null}
            {recoveryBlocked ? <>
              <Button disabled={busy} onClick={() => setRecoveryValidation(value => value + 1)}>Retry saved launch</Button>
              <Button variant="link" href="/create">Back to creation</Button>
            </> : null}
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
            </div> : <p role="status">{paramsError || (!connected ? 'Connect your wallet on Paxeer to read chain costs.' : 'Loading chain costs…')}</p> : null}
            {step === 0 && paramsError ? <p role="alert">{paramsError}</p> : null}
            {paramsError && !result ? <Button disabled={busy || !connected} onClick={() => setValidation(value => value + 1)}>Retry chain costs</Button> : null}
            {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
            {notice ? <p role="status" className="text-sm text-text-muted">{notice}</p> : null}
            <div className="flex flex-col gap-2">
              {step === 0 ? <Button variant="primary" size="lg" disabled={!nameAvailable || !launchFieldsValid || !connected || locked} onClick={() => setStep(1)}>Next</Button>
                : step === 1 ? <Button variant="primary" size="lg" disabled={!tierAvailable(tier) || locked} onClick={() => setStep(2)}>Next</Button>
                  : !result ? <Button variant="primary" size="lg" disabled={!connected || !launchFieldsValid || !nameAvailable || !tierAvailable(tier) || marketFee === null || !feeFunding || feeFunding.balance < feeFunding.fee || feeFunding.allowance < feeFunding.fee || locked} onClick={() => void launch()}>Launch chain and token</Button> : null}
              {step > 0 && !locked ? <Button variant="ghost" onClick={() => setStep(previous => previous - 1)}>Back</Button> : null}
            </div>
          </section>
        </div>
        <aside className="min-w-0 shrink-0 @market-two/create:sticky @market-two/create:top-4 @market-two/create:w-56">
          <ChainLaunchSummary name={name} tokenName={tokenName} tokenSymbol={tokenSymbol} tier={tier} feeStrategy={feeStrategy} params={params} marketFee={marketFee} />
        </aside>
      </div>
    </div>
  );
}
