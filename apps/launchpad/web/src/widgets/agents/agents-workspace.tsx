'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Bot } from 'lucide-react';
import { formatEther, formatUnits, type Address, type Hex } from 'viem';
import { Button, EmptyState, ErrorState, NoticeBar, Panel, Skeleton } from '@/ui/primitives';
import { createOwnedAgent, type CreatedAgentKey } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk/agent-crypto';
import { type PaxeerWallet, type SigningSession, type AgentClaim, PaxeerWalletError } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk';
import { AgentEnrollment, AgentKeyNotice, AgentLimitsForm, AgentTransferForm } from './agent-forms';
import { AgentActivityPanel } from './agent-activity';
import { agentAmount, agentAsset, agentBalanceText, agentError, loadAgentInventory, readAgentBalances, saveAgentLimits, type AgentAsset, type AgentChainReader, type AgentInventory, type AgentPreset, type AgentRow } from './agents-domain';

export interface AgentsWorkspaceProps {
  wallet: PaxeerWallet;
  owner: Address;
  userId: string;
  accessToken: string;
  reader: AgentChainReader;
}
interface TransferStatus { hash: Hex; kind: string; amount: string; symbol: string; result: string; error: boolean }
interface AgentFailure { kind: 'configuration' | 'error'; message: string }

function agentFailure(cause: unknown): AgentFailure {
  return {
    kind: cause instanceof PaxeerWalletError && ['agent_lane_disabled', 'provisioning_unavailable'].includes(cause.code) ? 'configuration' : 'error',
    message: agentError(cause),
  };
}

function AgentFailureState({ failure, title, onRetry }: { failure: AgentFailure; title?: string; onRetry: () => void }) {
  return failure.kind === 'configuration'
    ? <Panel role="status"><EmptyState icon={Bot} title="Agent gateway unavailable" body={failure.message} action={<Button onClick={onRetry}>Try again</Button>} /></Panel>
    : <ErrorState title={title} message={failure.message} onRetry={onRetry} />;
}

export function AgentsWorkspace({ wallet, owner, userId, accessToken, reader }: AgentsWorkspaceProps) {
  const current = useRef(true);
  const lock = useRef(false);
  const [invalid, setInvalid] = useState(false);
  const [inventory, setInventory] = useState<AgentInventory | null>(null);
  const [loadError, setLoadError] = useState<AgentFailure | null>(null);
  const [actionError, setActionError] = useState<AgentFailure | null>(null);
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [tick, setTick] = useState(0);
  const [selected, setSelected] = useState('');
  const [key, setKey] = useState<CreatedAgentKey | null>(null);
  const [transaction, setTransaction] = useState<TransferStatus | null>(null);
  const expected = useMemo<SigningSession>(() => ({
    userId, accessToken,
    assertCurrent() { if (!current.current) throw new PaxeerWalletError('The wallet session changed.', 'SESSION_CHANGED', 401); },
  }), [userId, accessToken]);

  useEffect(() => {
    current.current = true;
    const unsubscribe = wallet.onAuthStateChange((_event, session) => {
      if (session?.user.id !== userId || session.access_token !== accessToken) {
        current.current = false;
        setKey(null); setInventory(null); setTransaction(null); setInvalid(true);
      }
    });
    return () => { current.current = false; unsubscribe(); };
  }, [wallet, userId, accessToken]);

  const reload = useCallback(() => { setTick(value => value + 1); }, []);
  useEffect(() => {
    let active = true;
    setInventory(null); setLoadError(null);
    if (invalid) return;
    void loadAgentInventory(wallet, expected, owner, reader).then(value => {
      if (!active || !current.current) return;
      setInventory(value);
      setSelected(previous => value.rows.some(row => row.summary.did === previous) ? previous : value.rows[0]?.summary.did ?? '');
    }).catch(cause => { if (active && current.current) setLoadError(agentFailure(cause)); });
    return () => { active = false; };
  }, [wallet, expected, owner, reader, tick, invalid]);

  async function run(action: () => Promise<void>, success: string) {
    if (lock.current || !current.current) return;
    lock.current = true; setBusy(true); setActionError(null); setNotice('');
    try {
      expected.assertCurrent();
      await action();
      expected.assertCurrent();
      setNotice(success);
    } catch (cause) {
      if (current.current) setActionError(agentFailure(cause));
    } finally {
      lock.current = false;
      if (current.current) { setBusy(false); reload(); }
    }
  }

  async function checkReceipt(value: TransferStatus) {
    try {
      const receipt = await reader.getTransactionReceipt({ hash: value.hash });
      expected.assertCurrent();
      setTransaction({ ...value, result: receipt.status === 'success' ? 'Confirmed' : 'Reverted', error: false });
    } catch (cause) {
      if (!current.current) return;
      const pending = cause instanceof Error && cause.name === 'TransactionReceiptNotFoundError';
      setTransaction({ ...value, result: pending ? 'Submitted; awaiting confirmation' : 'Confirmation could not be loaded', error: !pending });
    }
  }

  async function transfer(row: AgentRow, kind: 'fund' | 'sweep', asset: AgentAsset, input: string) {
    await run(async () => {
      const record = await wallet.getWallet(expected);
      if (record.wallet.address.toLowerCase() !== owner.toLowerCase() || record.wallet.chain_id !== 125)
        throw new Error('The embedded wallet changed.');
      const source = kind === 'fund' ? owner : row.detail.wallet?.address;
      if (!source) throw new Error('The agent wallet is not provisioned.');
      const balances = await readAgentBalances(reader, source);
      expected.assertCurrent();
      const selectedAsset = agentAsset(asset, balances);
      const amount = agentAmount(input, selectedAsset.decimals, selectedAsset.balance).toString();
      const body = { amount, token: selectedAsset.token };
      const response = kind === 'fund' ? await wallet.fundAgent(row.summary.did, body, expected) : await wallet.sweepAgent(row.summary.did, body, expected);
      expected.assertCurrent();
      if (!/^0x[0-9a-fA-F]{64}$/.test(response.tx_hash)) throw new Error('The transaction hash is invalid.');
      const status = { hash: response.tx_hash, kind: kind === 'fund' ? 'Funding' : 'Sweep', amount: input, symbol: selectedAsset.symbol, result: 'Submitted; awaiting confirmation', error: false };
      setTransaction(status);
      await checkReceipt(status);
    }, 'Transfer submitted. Its result is shown below.');
  }

  if (invalid) return <NoticeBar wrap>Your embedded wallet session changed. Sign in again to manage agents.</NoticeBar>;
  const row = inventory?.rows.find(item => item.summary.did === selected);
  return <div className="flex min-w-0 flex-col gap-4" aria-busy={busy || undefined}>
    {key ? <AgentKeyNotice value={key} onDismiss={() => setKey(null)} /> : null}
    <AgentEnrollment ownerId={userId} busy={busy} hasKey={key !== null} onCreate={label => void run(async () => {
      const did = await createOwnedAgent(wallet, expected, label, value => { expected.assertCurrent(); setKey(value); });
      setSelected(did);
    }, 'Agent created and claimed. Save its key before leaving this page.')} onClaim={(did: string, claim: AgentClaim) => void run(async () => {
      const result = await wallet.claimAgent(did, claim, expected);
      if (result.did !== did || result.owner_user_id !== userId) throw new Error('The claimed agent does not match this owner.');
      setSelected(did);
    }, 'Agent claimed. You can now manage its wallet.')} />
    {actionError ? <AgentFailureState title="Action could not finish" failure={actionError} onRetry={() => { setActionError(null); reload(); }} /> : null}
    {notice ? <p role="status" className="text-sm text-text-muted">{notice}</p> : null}
    {transaction ? <Panel aria-labelledby="agent-transfer-result" className="flex flex-col gap-2">
      <h2 id="agent-transfer-result" className="text-md font-semibold">{transaction.kind} transaction</h2>
      <p role="status" className="text-sm tabular-nums">{transaction.amount} {transaction.symbol} · {transaction.result}</p>
      <p className="break-all text-xs text-text-muted">{transaction.hash}</p>
      <Button disabled={busy} onClick={() => void checkReceipt(transaction)}>{transaction.error ? 'Retry confirmation' : 'Check confirmation'}</Button>
    </Panel> : null}
    <Panel aria-labelledby="agent-list-title" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2"><h2 id="agent-list-title" className="text-md font-semibold">Your agents</h2><Button disabled={busy} onClick={reload}>Refresh agents</Button></div>
      {!inventory && !loadError ? <Skeleton variant="rows" rows={3} label="your agents" /> : null}
      {loadError ? <AgentFailureState failure={loadError} onRetry={reload} /> : null}
      {inventory?.rows.length === 0 ? <EmptyState icon={Bot} title="No agents yet" body="Create an agent or claim one that holds its own key. Its wallet and activity will appear here." /> : null}
      {inventory?.rows.map(item => <button key={item.summary.did} type="button" aria-pressed={selected === item.summary.did} onClick={() => setSelected(item.summary.did)} disabled={busy}
        className={`sl-hairline flex min-h-11 w-full min-w-0 flex-col gap-1 rounded-sm px-2 py-3 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ${selected === item.summary.did ? 'bg-surface-2' : 'hover:bg-surface-hover'}`}>
        <span className="flex w-full min-w-0 flex-wrap justify-between gap-2"><span className="min-w-0 break-all text-md font-semibold">{item.summary.label}</span><span className="text-sm text-text-muted">{item.detail.policy.mode.replaceAll('_', ' ')} · {item.detail.is_frozen ? 'Frozen' : 'Active'}</span></span>
        <span className="break-all text-xs text-text-muted">{item.detail.wallet?.address ?? 'Wallet not provisioned'}</span>
        <span className="break-all text-xs text-text-muted">{item.summary.did}</span>
        <span className="w-full text-right text-md font-bold tabular-nums">{item.balances ? agentBalanceText(item.balances) : item.detail.wallet ? 'Balances unavailable' : 'No wallet balances'}</span>
      </button>)}
    </Panel>
    {inventory?.balanceError ? <ErrorState message={inventory.balanceError} onRetry={reload} /> : null}
    {row ? <section aria-label={`Manage ${row.summary.label}`} className="flex min-w-0 flex-col gap-3">
      <Panel aria-labelledby="agent-management-title" className="flex flex-col gap-3">
        <h2 id="agent-management-title" className="break-all text-md font-semibold">Manage {row.summary.label}</h2>
        <p className="text-sm text-text-muted">{row.detail.is_frozen ? 'Frozen: agent requests are blocked. Owner funding and sweeps remain available.' : 'Active: the agent may act within its saved policy.'}</p>
        <Button disabled={busy} variant={row.detail.is_frozen ? 'secondary' : 'danger-text'} onClick={() => void run(async () => { await wallet.setAgentFrozen(row.summary.did, !row.detail.is_frozen, expected); }, row.detail.is_frozen ? 'Agent unfrozen.' : 'Agent frozen.')}>{row.detail.is_frozen ? 'Unfreeze agent' : 'Freeze agent'}</Button>
        <p className="text-sm tabular-nums text-text-muted">Saved caps: {formatEther(BigInt(row.detail.policy.max_tx_value_wei))} PAX per transaction; {formatEther(BigInt(row.detail.policy.max_daily_value_wei))} PAX per day. USDL per approval: {row.balances ? formatUnits(BigInt(row.detail.policy.max_approve_wei), row.balances.quoteDecimals) : 'unavailable'}.</p>
        <details className="min-w-0"><summary className="min-h-11 cursor-pointer py-2 text-sm">Saved rules and budgets</summary>
          <div className="flex min-w-0 flex-col gap-2 text-xs text-text-muted">
            {row.detail.rules.length === 0 ? <p>No custom rules.</p> : row.detail.rules.map(rule => <p key={rule.id} className="break-all">{rule.effect} {rule.subject}: {rule.value}{rule.max_value_wei !== undefined && rule.max_value_wei !== null ? `; cap ${rule.max_value_wei} base units` : ''}</p>)}
            {row.detail.budgets.length === 0 ? <p>No extra budgets.</p> : row.detail.budgets.map(budget => <p key={budget.id} className="break-all">Extra budget: {budget.remaining_wei} base units remaining of {budget.cap_wei}; {budget.token ?? 'native PAX'}; {budget.target_contract ?? 'all contracts'}; expires {new Date(budget.expires_at).toLocaleString()}.</p>)}
          </div>
        </details>
      </Panel>
      {row.balanceError ? <ErrorState message={row.balanceError} onRetry={reload} /> : null}
      {row.detail.wallet ? <div className="grid min-w-0 gap-3 lg:grid-cols-2">
        <AgentTransferForm key={`fund:${row.summary.did}`} kind="fund" owner={owner} busy={busy} balances={inventory?.ownerBalances ?? null} onSend={(asset, amount) => void transfer(row, 'fund', asset, amount)} />
        <AgentTransferForm key={`sweep:${row.summary.did}`} kind="sweep" owner={owner} busy={busy} balances={row.balances} onSend={(asset, amount) => void transfer(row, 'sweep', asset, amount)} />
      </div> : <NoticeBar wrap>This agent has not provisioned a wallet. Ask the agent to finish provisioning before funding or sweeping.</NoticeBar>}
      <AgentLimitsForm key={`limits:${row.summary.did}`} quoteDecimals={row.balances?.quoteDecimals ?? null} busy={busy} onSave={(preset: AgentPreset, transaction, day, approval) => void run(() => saveAgentLimits(wallet, expected, row, owner, preset, transaction, day, approval), 'Limits saved. The agent remains frozen; review them before unfreezing.')} />
      <AgentActivityPanel key={`activity:${row.summary.did}`} wallet={wallet} expected={expected} did={row.summary.did} refresh={tick} />
    </section> : null}
  </div>;
}
