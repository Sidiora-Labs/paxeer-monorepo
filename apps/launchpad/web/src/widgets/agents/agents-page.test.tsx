// @vitest-environment jsdom
import { createPrivateKey, createPublicKey, sign, webcrypto } from 'node:crypto';
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { erc20Abi, hexToBytes, parseEther, toFunctionSelector } from 'viem';
import { AgentsGatewayFixture } from './agents-gateway.fixture';
import { AgentsWorkspace } from './agents-workspace';
import { AgentsAccess } from './agents-page';
import { agentAmount, agentPresetRules } from './agents-domain';
import { agentClaimDigest, parseAgentClaim, prepareAgentClaim } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk/agent-crypto';
import { LAUNCHPAD_ADDRESS, ROUTER_ADDRESS, USDL_ADDRESS } from '@/core/network/contracts';
import { navItems } from '@/shell/nav-config';
import { routeForPath } from '@/core/app-foundation/navigation';
import { openAuthModal, closeAuthModal, useAuthModal } from '@/shell/auth-modal-store';
import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';

const gateway = new AgentsGatewayFixture();
let host: HTMLDivElement;
let root: Root | null = null;
const previousCrypto = Object.getOwnPropertyDescriptor(globalThis, 'crypto');

beforeAll(async () => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  Object.defineProperty(globalThis, 'crypto', { configurable: true, value: webcrypto });
  await gateway.start();
}, 30000);
beforeEach(async () => { await gateway.reset(); host = document.createElement('div'); document.body.appendChild(host); });
afterEach(async () => { if (root) await act(async () => root!.unmount()); root = null; host.remove(); closeAuthModal(); });
afterAll(async () => { await gateway.stop(); if (previousCrypto) Object.defineProperty(globalThis, 'crypto', previousCrypto); });

async function render(node?: ReactNode) {
  root = createRoot(host);
  await act(async () => root!.render(node ?? <AgentsWorkspace wallet={gateway.wallet} owner={gateway.owner} userId={gateway.session.user.id} accessToken={gateway.session.access_token} reader={gateway.reader} />));
}
async function settle(check: () => void) {
  await expect.poll(async () => { await act(async () => { await new Promise(done => setTimeout(done, 10)); }); check(); return true; }, { timeout: 4000 }).toBe(true);
}
async function ready() { await settle(() => expect(host.querySelector('[aria-label="Manage trader"]')).not.toBeNull()); }
async function input(id: string, value: string) {
  const element = host.querySelector<HTMLInputElement | HTMLTextAreaElement>(`#${id}`)!;
  expect(element, id).not.toBeNull();
  const proto = element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  await act(async () => { Object.getOwnPropertyDescriptor(proto, 'value')!.set!.call(element, value); element.dispatchEvent(new Event('input', { bubbles: true })); });
}
function button(label: string) {
  const result = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(element => element.textContent?.trim() === label);
  expect(result, label).toBeDefined(); return result!;
}
async function click(label: string) { await act(async () => button(label).click()); }
async function select(label: string, value: string) {
  const element = host.querySelector<HTMLSelectElement>(`select[aria-label="${label}"]`)!;
  expect(element, label).not.toBeNull();
  await act(async () => { element.value = value; element.dispatchEvent(new Event('change', { bubbles: true })); });
}

async function agentsPageFundsAnAgentFromTheEmbeddedWallet() {
  await render(); await ready();
  const before = await gateway.reader.getBalance({ address: gateway.agentAddress });
  await input('agent-fund-amount', '1.25');
  await act(async () => { button('Fund agent').click(); button('Fund agent').click(); });
  await settle(() => expect(host.textContent).toContain('1.25 PAX · Confirmed'));
  const native = gateway.requests.filter(request => request.path.endsWith('/fund'));
  expect(native).toHaveLength(1);
  expect(native[0].body).toEqual({ amount: '1250000000000000000', token: null });
  expect(native[0].headers.authorization).toBe(`Bearer ${gateway.session.access_token}`);
  expect(await gateway.reader.getBalance({ address: gateway.agentAddress })).toBe(before + parseEther('1.25'));
  await ready();
  const quoteBefore = await gateway.reader.readContract({ address: USDL_ADDRESS, abi: erc20Abi, functionName: 'balanceOf', args: [gateway.agentAddress] });
  await select('Funding asset', 'quote'); await input('agent-fund-amount', '2.5'); await click('Fund agent');
  await settle(() => expect(host.textContent).toContain('2.5 USDL · Confirmed'));
  const quote = gateway.requests.filter(request => request.path.endsWith('/fund'));
  expect(quote).toHaveLength(2);
  expect(quote[1].body).toEqual({ amount: '2500000', token: USDL_ADDRESS });
  expect(await gateway.reader.readContract({ address: USDL_ADDRESS, abi: erc20Abi, functionName: 'balanceOf', args: [gateway.agentAddress] })).toBe(quoteBefore + 2500000n);
}

async function agentsPageShowsACreatedKeyOnceAndKeepsNoCopy() {
  await render(); await ready();
  await input('agent-label', 'assistant');
  await settle(() => expect(button('Create agent').disabled).toBe(false));
  await click('Create agent');
  await settle(() => expect(host.textContent).toContain('Agent created and claimed.'));
  const secret = host.querySelector<HTMLInputElement>('#created-agent-key')!.value;
  expect(secret).toMatch(/^[0-9a-f]{64}$/);
  const privateKey = createPrivateKey({ key: Buffer.from('302e020100300506032b657004220420' + secret, 'hex'), format: 'der', type: 'pkcs8' });
  const publicKey = createPublicKey(privateKey).export({ format: 'der', type: 'spki' }).subarray(-32).toString('hex');
  const did = `did:matrix:assistant:${publicKey.slice(0, 16)}`;
  expect(gateway.principals.get(did)?.active).toBe(true);
  expect(gateway.principals.get(did)?.owner).toBe(gateway.session.user.id);
  expect(gateway.principals.get(did)?.detail.policy.mode).toBe('read_only');
  const provisioning = gateway.requests.filter(request => request.path === '/v1/agent/provision');
  expect(provisioning).toHaveLength(2);
  expect(provisioning[0].headers.authorization).toBeUndefined();
  expect(provisioning[0].body).toEqual({});
  expect(provisioning[1].body.bind_signature).toMatch(/^[0-9a-f]{128}$/);
  expect(gateway.requests.find(request => request.path.endsWith('/claim'))?.headers.authorization).toBe(`Bearer ${gateway.session.access_token}`);
  expect(button('Copy key').disabled).toBe(false);
  await click('Copy key');
  expect(host.textContent).toContain('Copy is unavailable. Select the key and copy it manually.');
  expect(button('Dismiss key').disabled).toBe(true);
  const checkbox = host.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
  await act(async () => checkbox.click()); await click('Dismiss key');
  expect(host.querySelector('#created-agent-key')).toBeNull();
  expect(host.innerHTML).not.toContain(secret);
  expect(JSON.stringify(gateway.requests)).not.toContain(secret);
  expect(JSON.stringify(Object.values(localStorage))).not.toContain(secret);
  expect(JSON.stringify(Object.values(sessionStorage))).not.toContain(secret);
  expect(JSON.stringify(history.state)).not.toContain(secret);
  await act(async () => root!.unmount()); root = null;
  await render(); await settle(() => expect(host.textContent).toContain('assistant'));
  expect(host.querySelector('#created-agent-key')).toBeNull();
  expect(host.innerHTML).not.toContain(secret);
}

async function agentsPageClaimsAnExistingAgentWithoutBrowserKeySupport() {
  const principal = gateway.principals.get(gateway.agentDid)!; principal.owner = null;
  Object.defineProperty(globalThis, 'crypto', { configurable: true, value: undefined });
  try {
    await render();
    await settle(() => expect(host.textContent).toContain('This browser does not support Ed25519 through Web Crypto.'));
    expect(button('Create agent').disabled).toBe(true);
    await input('agent-did', gateway.agentDid); await click('Prepare claim');
    const instructions = JSON.parse(host.querySelector<HTMLTextAreaElement>('#agent-claim-instructions')!.value);
    expect(instructions.did).toBe(gateway.agentDid);
    expect(instructions.owner_user_id).toBe(gateway.session.user.id);
    expect(instructions.digest).toBe(agentClaimDigest(gateway.agentDid, gateway.session.user.id, instructions.expires));
    const signature = sign(null, Buffer.from(hexToBytes(instructions.digest)), gateway.existingKey).toString('hex');
    await input('agent-claim-response', JSON.stringify({ expires: instructions.expires, signature })); await click('Claim agent');
    await settle(() => expect(host.textContent).toContain('Agent claimed.'));
    expect(principal.owner).toBe(gateway.session.user.id);
    expect(gateway.requests.filter(request => request.path.endsWith('/claim'))).toHaveLength(1);
    expect(gateway.requests.some(request => request.path.startsWith('/v1/agent/'))).toBe(false);
  } finally { Object.defineProperty(globalThis, 'crypto', { configurable: true, value: webcrypto }); }
}

async function agentsPageValidatesAmountsBeforeSending() {
  await render(); await ready();
  const before = gateway.requests.filter(request => request.path.endsWith('/fund')).length;
  for (const invalid of ['0', '-1', '1e2', '1.0000000000000000001', '999999999']) {
    await input('agent-fund-amount', invalid);
    expect(button('Fund agent').disabled, invalid).toBe(true);
  }
  await select('Funding asset', 'quote'); await input('agent-fund-amount', '0.0000001');
  expect(button('Fund agent').disabled).toBe(true);
  expect(gateway.requests.filter(request => request.path.endsWith('/fund'))).toHaveLength(before);
  expect(() => agentAmount('1.0000001', 6)).toThrow('at most 6');
  expect(() => agentAmount('2', 6, 1000000n)).toThrow('exceeds');
}

async function agentsPageSavesLimitsThroughPolicyRulesAndBudgets() {
  const detail = gateway.principals.get(gateway.agentDid)!.detail;
  detail.rules.push({ id: '701', effect: 'allow', subject: 'contract', value: gateway.owner });
  detail.budgets.push({ id: '702', target_contract: null, token: null, cap_wei: '100', spent_wei: '0', remaining_wei: '100', expires_at: new Date(Date.now() + 86400000).toISOString() });
  await render(); await ready();
  await select('Agent preset', 'create');
  await input('agent-transaction-cap', '1.5'); await input('agent-daily-cap', '4'); await input('agent-approval-cap', '25');
  expect(host.textContent).toContain('At most 1.5 PAX in one transaction and 4 PAX per day');
  expect(host.textContent).toContain('does not enforce a daily USDL spending cap');
  expect(host.textContent).toContain('buy, sell, swapTokenForToken, approve, createMarket, launchChain');
  await click('Save limits and freeze');
  await settle(() => expect(host.textContent).toContain('Limits saved.'));
  expect(detail.is_frozen).toBe(true);
  expect(detail.policy).toMatchObject({ mode: 'trade_only', max_tx_value_wei: '1500000000000000000', max_daily_value_wei: '4000000000000000000', max_approve_wei: '25000000', allow_native_transfer: false, withdrawal_allowlist_only: true, daily_reset_utc_hour: 0 });
  expect(detail.rules).not.toContainEqual(expect.objectContaining({ id: '701' }));
  for (const value of [ROUTER_ADDRESS, USDL_ADDRESS, LAUNCHPAD_ADDRESS]) expect(detail.rules).toContainEqual(expect.objectContaining({ effect: 'allow', subject: 'contract', value }));
  expect(detail.rules).toContainEqual(expect.objectContaining({ effect: 'allow', subject: 'token', value: USDL_ADDRESS, max_value_wei: '25000000' }));
  expect(detail.rules).toContainEqual(expect.objectContaining({ effect: 'deny', subject: 'selector', value: toFunctionSelector('transfer(address,uint256)') }));
  expect(detail.rules).toContainEqual(expect.objectContaining({ effect: 'deny', subject: 'selector', value: toFunctionSelector('multicall(bytes[])') }));
  expect(detail.budgets).toEqual([expect.objectContaining({ cap_wei: '0', remaining_wei: '0' })]);
  const writes = gateway.requests.filter(request => request.method !== 'GET' && request.path.startsWith('/v1/agents/'));
  expect(writes[0].path).toMatch(/\/freeze$/);
  expect(writes[1].body).toEqual({ mode: 'read_only' });
  expect(writes.some(request => request.path.endsWith('/unfreeze'))).toBe(false);
  expect(agentPresetRules('read', gateway.owner, '0')).toEqual([]);
  expect(agentPresetRules('trade', gateway.owner, '1')).not.toContainEqual(expect.objectContaining({ subject: 'contract', value: LAUNCHPAD_ADDRESS }));
}

async function agentsPageSavesReadOnlyAndTradePresets() {
  await render(); await ready();
  for (const preset of ['read', 'trade'] as const) {
    await select('Agent preset', preset);
    await input('agent-transaction-cap', '0'); await input('agent-daily-cap', '0'); await input('agent-approval-cap', preset === 'read' ? '0' : '2');
    await click('Save limits and freeze');
    await settle(() => expect(gateway.principals.get(gateway.agentDid)!.detail.policy.mode).toBe(preset === 'read' ? 'read_only' : 'trade_only'));
    await settle(() => expect(host.textContent).toContain('Limits saved.')); await ready();
    const detail = gateway.principals.get(gateway.agentDid)!.detail;
    expect(detail.policy.max_tx_value_wei).toBe('0'); expect(detail.policy.max_daily_value_wei).toBe('0');
    expect(detail.is_frozen).toBe(true);
    expect(detail.rules.filter(rule => rule.subject === 'contract').map(rule => rule.value)).toEqual(preset === 'read' ? [] : [ROUTER_ADDRESS, USDL_ADDRESS]);
    expect(detail.policy.max_approve_wei).toBe(preset === 'read' ? '0' : '2000000');
  }
}

async function agentsPageKeepsAnAgentFrozenWhenSavingLimitsFails() {
  await render(); await ready();
  gateway.fail.add(`/v1/agents/${encodeURIComponent(gateway.agentDid)}/rules`);
  await select('Agent preset', 'trade'); await input('agent-transaction-cap', '1'); await input('agent-daily-cap', '2'); await input('agent-approval-cap', '3');
  await click('Save limits and freeze');
  await settle(() => expect(host.textContent).toContain('Action could not finish'));
  expect(gateway.principals.get(gateway.agentDid)!.detail.is_frozen).toBe(true);
  expect(gateway.principals.get(gateway.agentDid)!.detail.policy.mode).toBe('read_only');
  expect(gateway.requests.some(request => request.path.endsWith('/unfreeze'))).toBe(false);
}

async function agentsPageFreezesUnfreezesSweepsAndShowsActivity() {
  await render(); await ready();
  await click('Freeze agent'); await settle(() => expect(host.textContent).toContain('Agent frozen.')); await ready();
  expect(gateway.principals.get(gateway.agentDid)!.detail.is_frozen).toBe(true);
  await click('Unfreeze agent'); await settle(() => expect(host.textContent).toContain('Agent unfrozen.')); await ready();
  expect(gateway.principals.get(gateway.agentDid)!.detail.is_frozen).toBe(false);
  const before = await gateway.reader.getBalance({ address: gateway.owner });
  await input('agent-sweep-amount', '0.5'); await click('Sweep amount');
  await settle(() => expect(host.textContent).toContain('0.5 PAX · Confirmed'));
  expect(await gateway.reader.getBalance({ address: gateway.owner })).toBe(before + parseEther('0.5'));
  const swept = gateway.requests.filter(request => request.path.endsWith('/sweep'));
  expect(swept).toHaveLength(1); expect(swept[0].body).toEqual({ amount: '500000000000000000', token: null });
  await ready();
  await settle(() => expect(host.querySelector('[aria-labelledby="agent-activity-title"] time')).not.toBeNull());
  const activity = host.querySelector('[aria-labelledby="agent-activity-title"]')!;
  expect(activity.textContent).toContain('transaction'); expect(activity.textContent).toContain('0.5 PAX'); expect(activity.textContent).toContain('Submitted');
  expect(activity.querySelector('time')!.getAttribute('datetime')).toBe(gateway.events.get(gateway.agentDid)![0].created_at);
}

async function agentsPageHasLoadingEmptyRetryAndUnavailableViews() {
  let release!: () => void;
  gateway.holdInventory = new Promise<void>(done => { release = done; });
  await render();
  expect(host.textContent).toContain('Loading your agents');
  expect(host.querySelector('[aria-busy="true"] .sl-skeleton')).not.toBeNull();
  await act(async () => { release(); gateway.holdInventory = null; }); await ready();
  gateway.principals.clear(); await click('Refresh agents');
  await settle(() => expect(host.textContent).toContain('No agents yet'));
  gateway.fail.add('/v1/agents'); await click('Refresh agents');
  await settle(() => expect(host.textContent).toContain('The agent gateway is not configured.'));
  expect(button('Try again')).toBeDefined();
  gateway.fail.clear(); await click('Try again');
  await settle(() => expect(host.textContent).toContain('No agents yet'));
}

async function agentsPageDiscardsDataAndKeysWhenTheOwnerChanges() {
  await render(); await ready(); await input('agent-label', 'isolated');
  await settle(() => expect(button('Create agent').disabled).toBe(false)); await click('Create agent');
  await settle(() => expect(host.querySelector('#created-agent-key')).not.toBeNull());
  await settle(() => expect(host.textContent).toContain('Agent created and claimed.'));
  const secret = host.querySelector<HTMLInputElement>('#created-agent-key')!.value;
  await act(async () => { await gateway.switchOwner(); });
  expect(host.querySelector('#created-agent-key')).toBeNull(); expect(host.innerHTML).not.toContain(secret);
  expect(host.textContent).toContain('Your embedded wallet session changed.');
  expect(host.querySelector('[aria-label="Manage trader"]')).toBeNull();
  const expected = { userId: gateway.session.user.id, accessToken: gateway.session.access_token, assertCurrent() {} };
  await expect(gateway.wallet.fundAgent(gateway.agentDid, { amount: '1' }, expected)).rejects.toMatchObject({ code: 'SESSION_CHANGED' });
  expect(gateway.requests.some(request => request.path.endsWith('/fund'))).toBe(false);
}

async function agentsPageRegistersNavigationAndAccessibleAccessStates() {
  expect(routeForPath('/agents')).toMatchObject({ id: 'agents', permission: 'wallet' });
  expect(navItems.find(item => item.id === 'agents')).toMatchObject({ href: '/agents', requires: 'public' });
  function AuthState() { const auth = useAuthModal(); return <span>{auth.open ? 'Sign-in opened' : 'Sign-in closed'}</span>; }
  await render(<><AgentsAccess state="signed-out" onSignIn={() => openAuthModal('Sign in with your embedded wallet to manage agents')} onRetry={() => gateway.wallet.getSession()} /><AuthState /></>);
  expect(host.querySelectorAll('h1')).toHaveLength(1); expect(host.textContent).toContain('separate wallet for software');
  await click('Sign in with embedded wallet'); expect(host.textContent).toContain('Sign-in opened');
  for (const [state, copy] of [['external-wallet', 'Agents need the embedded wallet'], ['unconfigured', 'gateway is not configured here'], ['wallet-error', 'Embedded wallet unavailable'], ['wrong-chain', 'Switch to Paxeer']] as const) {
    await act(async () => root!.render(<AgentsAccess state={state} onSignIn={() => openAuthModal()} onRetry={() => gateway.wallet.getSession()} />));
    expect(host.textContent).toContain(copy); assertControlEdges(host.innerHTML);
  }
  expect(gateway.requests.filter(request => request.path.startsWith('/v1/agents'))).toHaveLength(0);
}

function agentsPageRejectsExpiredMalformedAndMismatchedClaims() {
  const instructions = prepareAgentClaim(gateway.agentDid, gateway.session.user.id);
  expect(() => parseAgentClaim('not json', instructions)).toThrow('JSON');
  expect(() => parseAgentClaim(JSON.stringify({ expires: '1', signature: 'a'.repeat(128) }), instructions)).toThrow('this expiry');
  expect(() => parseAgentClaim(JSON.stringify({ expires: instructions.expires, signature: 'a'.repeat(128), private_key: 'extra' }), instructions)).toThrow('must contain');
  const expired = { ...instructions, expires: '1' };
  expect(() => parseAgentClaim(JSON.stringify({ expires: '1', signature: 'a'.repeat(128) }), expired)).toThrow('expired');
}

for (const check of [agentsPageFundsAnAgentFromTheEmbeddedWallet, agentsPageShowsACreatedKeyOnceAndKeepsNoCopy,
  agentsPageClaimsAnExistingAgentWithoutBrowserKeySupport, agentsPageValidatesAmountsBeforeSending,
  agentsPageSavesLimitsThroughPolicyRulesAndBudgets, agentsPageSavesReadOnlyAndTradePresets, agentsPageKeepsAnAgentFrozenWhenSavingLimitsFails,
  agentsPageFreezesUnfreezesSweepsAndShowsActivity, agentsPageHasLoadingEmptyRetryAndUnavailableViews,
  agentsPageDiscardsDataAndKeysWhenTheOwnerChanges, agentsPageRegistersNavigationAndAccessibleAccessStates,
  agentsPageRejectsExpiredMalformedAndMismatchedClaims]) test(check.name, check);
