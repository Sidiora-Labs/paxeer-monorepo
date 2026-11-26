'use client';

import { useEffect, useState } from 'react';
import { formatUnits, type Address } from 'viem';
import { Button, FormField, Panel, SelectPill, NoticeBar } from '@/ui/primitives';
import type { CreatedAgentKey, ClaimInstructions } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk/agent-crypto';
import { parseAgentClaim, prepareAgentClaim, supportsAgentKeyCreation } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk/agent-crypto';
import type { AgentClaim } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk';
import { agentAmount, agentCap, agentAsset, agentPresetMethods, type AgentAsset, type AgentBalances, type AgentPreset } from './agents-domain';
import { LAUNCHPAD_ADDRESS, ROUTER_ADDRESS, USDL_ADDRESS } from '@/core/network/contracts';

export function AgentKeyNotice({ value, onDismiss }: { value: CreatedAgentKey; onDismiss: () => void }) {
  const [copyState, setCopyState] = useState('');
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, []);
  return <Panel aria-labelledby="agent-key-title" className="flex flex-col gap-3">
    <h2 id="agent-key-title" className="text-md font-semibold">Save your agent key</h2>
    <NoticeBar tone="warning" wrap>This key is shown once. Anyone with it can act as this agent. Copy it to a secure place. This application does not save it and cannot recover it after you dismiss this view or leave.</NoticeBar>
    <p className="break-all text-xs text-text-muted">{value.did}</p>
    <FormField id="created-agent-key" label="Private key (Ed25519 seed, hex)" mono inputProps={{ value: value.privateKey, readOnly: true, autoComplete: 'off', spellCheck: false }} />
    <p className="break-all text-xs text-text-muted">Public key: {value.publicKey}</p>
    <div className="flex flex-wrap gap-2">
      <Button onClick={() => { if (!navigator.clipboard?.writeText) { setCopyState('Copy is unavailable. Select the key and copy it manually.'); return; } void navigator.clipboard.writeText(value.privateKey).then(() => setCopyState('Key copied. Clear your clipboard after saving it securely.'), () => setCopyState('Copy is unavailable. Select the key and copy it manually.')); }}>Copy key</Button>
      <label className="flex min-h-11 items-center gap-2 text-sm"><input type="checkbox" checked={saved} onChange={event => setSaved(event.target.checked)} />I have saved the key</label>
      <Button disabled={!saved} onClick={onDismiss}>Dismiss key</Button>
    </div>
    {copyState ? <p role="status" className="text-sm text-text-muted">{copyState}</p> : null}
  </Panel>;
}

export function AgentEnrollment({ ownerId, busy, hasKey, onCreate, onClaim }: {
  ownerId: string; busy: boolean; hasKey: boolean;
  onCreate: (label: string) => void;
  onClaim: (did: string, claim: AgentClaim) => void;
}) {
  const [supported, setSupported] = useState<boolean | null>(null);
  const [label, setLabel] = useState('');
  const [did, setDid] = useState('');
  const [instructions, setInstructions] = useState<ClaimInstructions | null>(null);
  const [pasted, setPasted] = useState('');
  const [error, setError] = useState('');
  useEffect(() => { let current = true; void supportsAgentKeyCreation().then(value => { if (current) setSupported(value); }); return () => { current = false; }; }, []);
  return <div className="grid min-w-0 gap-3 lg:grid-cols-2">
    <Panel aria-labelledby="agent-create-title" className="flex flex-col gap-3">
      <h2 id="agent-create-title" className="text-md font-semibold">Create an agent</h2>
      <p className="text-sm text-text-muted">Make an Ed25519 key in this browser, provision its wallet and claim it with your embedded wallet account. New agents start in read only mode.</p>
      {supported === false ? <NoticeBar tone="warning" wrap>This browser does not support Ed25519 through Web Crypto. Use a supported browser to create a key, or claim an existing agent below.</NoticeBar> : null}
      <form className="flex flex-col gap-3" onSubmit={event => { event.preventDefault(); if (!busy && supported && !hasKey) onCreate(label); }}>
        <FormField id="agent-label" label="Agent name" helper="Letters, numbers, underscores and hyphens; at most 128 characters." inputProps={{ value: label, onChange: event => setLabel(event.target.value), required: true, maxLength: 128, pattern: '[A-Za-z0-9_-]{1,128}', autoComplete: 'off' }} />
        <Button type="submit" variant="primary" disabled={busy || supported !== true || hasKey || !/^[A-Za-z0-9_-]{1,128}$/.test(label)} loading={busy}>Create agent</Button>
        {hasKey ? <p className="text-xs text-text-muted">Save and dismiss the displayed key before creating another agent.</p> : null}
      </form>
    </Panel>
    <Panel aria-labelledby="agent-claim-title" className="flex flex-col gap-3">
      <h2 id="agent-claim-title" className="text-md font-semibold">Claim an existing agent</h2>
      <p className="text-sm text-text-muted">The agent keeps its own key. It must authenticate and provision its wallet at the gateway first, then sign the request below. Never paste its private key here.</p>
      <FormField id="agent-did" label="Agent DID" mono inputProps={{ value: did, onChange: event => { setDid(event.target.value); setInstructions(null); setPasted(''); setError(''); }, autoComplete: 'off', maxLength: 256 }} />
      <Button disabled={busy} onClick={() => { try { setInstructions(prepareAgentClaim(did.trim(), ownerId)); setPasted(''); setError(''); } catch { setError('Enter a DID in the form did:matrix:name: followed by 16 lowercase hex characters.'); } }}>Prepare claim</Button>
      {instructions ? <>
        <p className="text-sm text-text-muted">Ask the agent to sign the 32 digest bytes with Ed25519. The SHA-256 digest binds its DID, your owner account and the expiry. Return JSON containing only expires and signature (128 hex characters without 0x).</p>
        <FormField id="agent-claim-instructions" label="Agent signing request" multiline mono textareaProps={{ value: JSON.stringify(instructions, null, 2), readOnly: true, rows: 7, spellCheck: false }} />
        <p className="text-xs tabular-nums text-text-muted">Expires at {new Date(Number(instructions.expires) * 1000).toLocaleString()}. Prepare another request if it expires.</p>
        <FormField id="agent-claim-response" label="Signed claim JSON" multiline mono textareaProps={{ value: pasted, onChange: event => setPasted(event.target.value), maxLength: 2048, rows: 3, autoComplete: 'off', spellCheck: false }} />
        <Button disabled={busy || !pasted} onClick={() => { try { const claim = parseAgentClaim(pasted, instructions); setError(''); onClaim(instructions.did, claim); setPasted(''); } catch (cause) { setError(cause instanceof Error ? cause.message : 'The claim could not be read.'); } }}>Claim agent</Button>
      </> : null}
      {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
    </Panel>
  </div>;
}

export function AgentTransferForm({ kind, balances, owner, busy, onSend }: {
  kind: 'fund' | 'sweep'; balances: AgentBalances | null; owner: Address; busy: boolean;
  onSend: (asset: AgentAsset, amount: string) => void;
}) {
  const [asset, setAsset] = useState<AgentAsset>('native');
  const [amount, setAmount] = useState('');
  const selected = balances ? agentAsset(asset, balances) : null;
  let error = '';
  if (selected && amount) { try { agentAmount(amount, selected.decimals, selected.balance); } catch (cause) { error = cause instanceof Error ? cause.message : 'Check the amount.'; } }
  const title = kind === 'fund' ? 'Fund from embedded wallet' : 'Sweep to your wallet';
  return <Panel aria-labelledby={`agent-${kind}-title`} className="flex flex-col gap-3">
    <h3 id={`agent-${kind}-title`} className="text-md font-semibold">{title}</h3>
    <p className="break-all text-xs text-text-muted">{kind === 'fund' ? 'From' : 'Back to'} your embedded wallet: {owner}</p>
    <form className="flex flex-col gap-3" onSubmit={event => { event.preventDefault(); if (selected && amount && !error && !busy) onSend(asset, amount); }}>
      <SelectPill<AgentAsset> label={`${kind === 'fund' ? 'Funding' : 'Sweep'} asset`} value={asset} onChange={value => { setAsset(value); setAmount(''); }} options={[{ value: 'native', label: 'PAX' }, { value: 'quote', label: 'USDL' }]} size="md" disabled={busy} />
      <FormField id={`agent-${kind}-amount`} label={`${kind === 'fund' ? 'Funding' : 'Sweep'} amount`} error={error} helper={selected ? `Available: ${formatUnits(selected.balance, selected.decimals)} ${selected.symbol}. PAX is also needed for network fees.` : 'Balances are unavailable. Refresh before sending.'} inputProps={{ value: amount, onChange: event => setAmount(event.target.value), inputMode: 'decimal', autoComplete: 'off', maxLength: 80, disabled: busy || !selected }} />
      <Button type="submit" variant={kind === 'fund' ? 'primary' : 'secondary'} disabled={busy || !selected || !amount || !!error}>{kind === 'fund' ? 'Fund agent' : 'Sweep amount'}</Button>
    </form>
  </Panel>;
}

export function AgentLimitsForm({ quoteDecimals, busy, onSave }: {
  quoteDecimals: number | null; busy: boolean;
  onSave: (preset: AgentPreset, transaction: string, day: string, approval: string) => void;
}) {
  const [preset, setPreset] = useState<AgentPreset>('read');
  const [transaction, setTransaction] = useState('');
  const [day, setDay] = useState('');
  const [approval, setApproval] = useState('');
  let error = '';
  let valid = false;
  if (transaction && day && approval && quoteDecimals !== null) {
    try {
      const tx = agentCap(transaction, 18);
      const daily = agentCap(day, 18);
      agentCap(approval, quoteDecimals);
      if (tx > daily) throw new Error('The daily PAX cap must be at least the transaction cap.');
      valid = true;
    } catch (cause) { error = cause instanceof Error ? cause.message : 'Check the caps.'; }
  }
  return <Panel aria-labelledby="agent-limits-title" className="flex flex-col gap-3">
    <h3 id="agent-limits-title" className="text-md font-semibold">Limits</h3>
    <SelectPill<AgentPreset> label="Agent preset" value={preset} onChange={setPreset} options={[{ value: 'read', label: 'Read only' }, { value: 'trade', label: 'Trade' }, { value: 'create', label: 'Trade and create' }]} size="md" disabled={busy} />
    <form className="flex flex-col gap-3" onSubmit={event => { event.preventDefault(); if (valid && !busy) onSave(preset, transaction, day, approval); }}>
      <div className="grid gap-3 sm:grid-cols-3">
        <FormField id="agent-transaction-cap" label="PAX per transaction" inputProps={{ value: transaction, onChange: event => setTransaction(event.target.value), inputMode: 'decimal', maxLength: 80 }} />
        <FormField id="agent-daily-cap" label="PAX per day" inputProps={{ value: day, onChange: event => setDay(event.target.value), inputMode: 'decimal', maxLength: 80 }} />
        <FormField id="agent-approval-cap" label="USDL per approval" inputProps={{ value: approval, onChange: event => setApproval(event.target.value), inputMode: 'decimal', maxLength: 80 }} />
      </div>
      <div className="flex flex-col gap-2 text-sm text-text-muted" aria-live="polite">
        <p>{preset === 'read' ? 'The agent can read balances and activity. Signing is disabled.' : preset === 'trade' ? 'The agent can call the Router and approve USDL for that Router.' : 'The agent can trade, create a market through the Router, and launch a chain through the Launchpad.'}</p>
        {preset !== 'read' ? <>
          <p className="break-all">Router: {ROUTER_ADDRESS}. USDL: {USDL_ADDRESS}.{preset === 'create' ? ` Launchpad: ${LAUNCHPAD_ADDRESS}.` : ''}</p>
          <p className="break-words">Published methods: {[...agentPresetMethods.trade, ...(preset === 'create' ? agentPresetMethods.create : [])].map(value => value.split('(')[0]).join(', ')}. Other published write methods are blocked. Only USDL approvals are included; selling other tokens requires their existing allowance.</p>
        </> : null}
        <p>At most {transaction || '…'} PAX in one transaction and {day || '…'} PAX per day, resetting at midnight UTC. PAX caps cover native value, not the value of a token trade. No extra budget overrides these caps.</p>
        <p>{preset === 'read' ? 'Approvals are disabled.' : `USDL approvals are limited to ${approval || '…'} USDL each. The gateway does not enforce a daily USDL spending cap.`} Native transfers are disabled. Owner sweeps remain available.</p>
        <p>Saving freezes the agent and replaces its policy, rules and budgets. It stays frozen until you review the saved limits and unfreeze it.</p>
      </div>
      {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
      <Button type="submit" disabled={busy || !valid}>Save limits and freeze</Button>
    </form>
  </Panel>;
}
