import { createServer, type IncomingHttpHeaders, type Server } from 'node:http';
import { createServer as reservePort } from 'node:net';
import { spawn, type ChildProcess } from 'node:child_process';
import { createHash, createHmac, createPublicKey, generateKeyPairSync, randomBytes, verify, type KeyObject } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import type { Session } from '@supabase/supabase-js';
import { createPublicClient, createWalletClient, createTestClient, defineChain, encodeAbiParameters, erc20Abi, http, keccak256, toHex, type Address, type Hex } from 'viem';
import { PaxeerWallet, type AgentActivity, type AgentBudget, type AgentDetail, type AgentPolicy, type AgentRule, type PublicWallet } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk';
import { USDL_ADDRESS } from '@/core/network/contracts';
import { verifyAgentSignature } from '../../../../../../services/wallet/gateway/src/auth/did';

interface Principal { did: string; label: string; publicKey: string; owner: string | null; active: boolean; detail: AgentDetail }
export interface GatewayRequest { path: string; method: string; body: Record<string, unknown>; headers: IncomingHttpHeaders }
const ownerId = '00000000-0000-4000-8000-000000000031';
const otherId = '00000000-0000-4000-8000-000000000032';
const defaultPolicy: AgentPolicy = { mode: 'read_only', max_tx_value_wei: '0', max_daily_value_wei: '0', max_approve_wei: '0', rate_limit_per_min: 30, allow_native_transfer: false, withdrawal_allowlist_only: true, daily_reset_utc_hour: 0 };
const prefixed = (value: string) => { const bytes = Buffer.from(value); const size = Buffer.alloc(4); size.writeUInt32BE(bytes.length); return Buffer.concat([size, bytes]); };
const expiryBytes = (value: string) => { const bytes = Buffer.alloc(8); bytes.writeBigUInt64BE(BigInt(value)); return bytes; };
const digest = (...bytes: Buffer[]) => createHash('sha256').update(Buffer.concat(bytes)).digest();
const keyObject = (publicKey: string) => createPublicKey({ key: Buffer.from('302a300506032b6570032100' + publicKey, 'hex'), type: 'spki', format: 'der' });

export class AgentsGatewayFixture {
  origin = '';
  owner!: Address;
  agentAddress!: Address;
  agentDid = '';
  existingKey!: KeyObject;
  wallet!: PaxeerWallet;
  session!: Session;
  requests: GatewayRequest[] = [];
  principals = new Map<string, Principal>();
  events = new Map<string, AgentActivity[]>();
  fail = new Set<string>();
  holdInventory: Promise<void> | null = null;
  private server!: Server;
  private anvil!: ChildProcess;
  private tokenSecret = randomBytes(32);
  private accounts: Address[] = [];
  private challenges = new Map<string, string>();
  private nonces = new Set<string>();
  private nextId = 1;
  private chainUrl = '';
  reader!: ReturnType<typeof createPublicClient>;
  private writer!: ReturnType<typeof createWalletClient>;
  private chainTest!: ReturnType<typeof createTestClient>;

  async start() {
    const reserved = reservePort();
    await new Promise<void>(done => reserved.listen(0, '127.0.0.1', done));
    const address = reserved.address();
    if (!address || typeof address === 'string') throw new Error('A chain port is required');
    const port = address.port;
    await new Promise<void>((done, reject) => reserved.close(error => error ? reject(error) : done()));
    this.chainUrl = `http://127.0.0.1:${port}`;
    const chain = defineChain({ id: 125, name: 'Agent wallet local chain', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [this.chainUrl] } } });
    this.anvil = spawn('anvil', ['--host', '127.0.0.1', '--port', String(port), '--chain-id', '125', '--silent'], { stdio: 'ignore' });
    this.reader = createPublicClient({ chain, pollingInterval: 10, transport: http(this.chainUrl, { retryCount: 0, timeout: 500 }) });
    this.writer = createWalletClient({ chain, transport: http(this.chainUrl) });
    this.chainTest = createTestClient({ mode: 'anvil', chain, transport: http(this.chainUrl) });
    let ready = false;
    for (let index = 0; index < 100; index++) {
      if (this.anvil.exitCode !== null) throw new Error('The agent chain exited before readiness');
      try { ready = await this.reader.getChainId() === 125; } catch { ready = false; }
      if (ready) break;
      await new Promise(done => setTimeout(done, 25));
    }
    if (!ready) throw new Error('The agent chain did not become ready');
    this.accounts = await this.writer.getAddresses();
    this.owner = this.accounts[0]; this.agentAddress = this.accounts[1];
    const artifacts = process.env.L25_FIXTURE_ARTIFACTS;
    if (!artifacts) throw new Error('Compiled wallet funding fixtures are required');
    const artifact = JSON.parse(readFileSync(resolve(artifacts, 'FundingToken.sol/FundingToken.json'), 'utf8'));
    const hash = await this.writer.deployContract({ account: this.owner, chain, abi: artifact.abi, bytecode: artifact.bytecode.object, args: [6] });
    const receipt = await this.reader.waitForTransactionReceipt({ hash });
    if (receipt.status !== 'success' || !receipt.contractAddress) throw new Error('The quote token deployment failed');
    const runtime = await this.reader.getCode({ address: receipt.contractAddress });
    if (!runtime) throw new Error('The quote token has no runtime');
    await this.chainTest.setCode({ address: USDL_ADDRESS, bytecode: runtime });
    const slot = keccak256(encodeAbiParameters([{ type: 'address' }, { type: 'uint256' }], [this.owner, 0n]));
    await this.chainTest.setStorageAt({ address: USDL_ADDRESS, index: slot, value: toHex(1000000000000n, { size: 32 }) });
    this.server = createServer((req, res) => void (async () => {
      const send = (status: number, value: unknown) => { res.writeHead(status, { 'content-type': 'application/json' }); res.end(JSON.stringify(value)); };
      try {
        const chunks: Buffer[] = [];
        for await (const chunk of req) chunks.push(Buffer.from(chunk));
        const bytes = Buffer.concat(chunks);
        const body = bytes.length ? JSON.parse(bytes.toString()) as Record<string, unknown> : {};
        const path = new URL(req.url!, 'http://localhost').pathname;
        const method = req.method!;
        this.requests.push({ path, method, body, headers: req.headers });
        if (this.fail.has(path)) { send(503, { error: path === '/v1/agents' ? 'agent_lane_disabled' : 'temporarily_unavailable' }); return; }
        if (path === '/v1/agent/auth/challenge') {
          const did = String(body.did); const nonce = randomBytes(24).toString('base64url');
          this.challenges.set(nonce, did);
          send(200, { did, nonce, expires_in: 60, message: `paxeer-agent-auth:${did}:${nonce}` }); return;
        }
        if (path === '/v1/agent/auth/verify') {
          const input = { did: String(body.did), publicKeyHex: String(body.public_key), nonce: String(body.nonce), signatureHex: String(body.signature) };
          const verified = verifyAgentSignature(input);
          if (!verified.ok || this.challenges.get(input.nonce) !== input.did) { send(401, { error: 'verification_failed' }); return; }
          this.challenges.delete(input.nonce);
          this.addPrincipal(input.did, input.publicKeyHex, null, this.accounts[this.principals.size + 1]);
          send(200, { token: randomBytes(24).toString('base64url'), expires_in: 60, owner_user_id: null, is_frozen: false, wallet_provisioned: false }); return;
        }
        if (path === '/v1/agent/provision') {
          const did = String(req.headers['x-agent-key']); const principal = this.principals.get(did);
          const nonce = String(req.headers['x-agent-nonce']); const expires = String(req.headers['x-agent-expires']);
          const signed = digest(Buffer.from('PXW:AGENT-REQUEST:v1'), prefixed('POST /v1/agent/provision'), prefixed(did), Buffer.from(nonce, 'hex'), expiryBytes(expires), digest(bytes));
          if (!principal || this.nonces.has(nonce) || Number(expires) <= Date.now() / 1000 || !verify(null, signed, keyObject(principal.publicKey), Buffer.from(String(req.headers['x-agent-signature']), 'hex'))) {
            send(401, { error: 'agent_bad_signature' }); return;
          }
          this.nonces.add(nonce);
          const message = Buffer.concat([Buffer.from('LX:PAXEER-BIND:v1'), Buffer.from(toHex(125, { size: 32 }).slice(2), 'hex'), Buffer.from(principal.detail.wallet!.address.slice(2), 'hex'), Buffer.alloc(8)]);
          if (body.bind_signature) {
            if (!verify(null, message, keyObject(principal.publicKey), Buffer.from(String(body.bind_signature), 'hex'))) { send(403, { error: 'agent_signature_invalid' }); return; }
            principal.active = true;
          }
          send(principal.active ? 200 : 202, { wallet: this.publicWallet(principal.detail.wallet!.address), provision: { state: principal.active ? 'active' : 'identity', awaiting: principal.active ? null : 'agent_signature', address: principal.detail.wallet!.address, did, bindMessage: principal.active ? null : toHex(message) } }); return;
        }
        const user = this.authenticate(req.headers.authorization);
        if (path === '/auth/v1/user') { send(200, user.user); return; }
        if (path === '/auth/v1/logout') { res.writeHead(204); res.end(); return; }
        if (path === '/v1/wallet/me') { send(200, { wallet: this.publicWallet(user.user.id === ownerId ? this.owner : this.accounts[9]), chain: { id: 125, rpc_url: this.chainUrl, explorer_url: null } }); return; }
        if (path === '/v1/agents') {
          if (this.holdInventory) await this.holdInventory;
          send(200, { agents: Array.from(this.principals.values()).filter(p => p.owner === user.user.id).map(p => ({ did: p.did, label: p.label, address: p.detail.wallet?.address, mode: p.detail.policy.mode, is_frozen: p.detail.is_frozen, created_at: new Date().toISOString(), last_seen_at: null })) }); return;
        }
        const match = /^\/v1\/agents\/([^/]+)(?:\/(.*))?$/.exec(path);
        if (!match) { send(404, { error: 'not_found' }); return; }
        const did = decodeURIComponent(match[1]); const action = match[2] ?? ''; const principal = this.principals.get(did);
        if (!principal) { send(404, { error: 'not_found' }); return; }
        if (action === 'claim') {
          const claimDigest = digest(Buffer.from('PXW:AGENT-CLAIM:v1'), prefixed(did), prefixed(user.user.id), expiryBytes(String(body.expires)));
          if (principal.owner || !principal.active || Number(body.expires) <= Date.now() / 1000 || !verify(null, claimDigest, keyObject(principal.publicKey), Buffer.from(String(body.signature), 'hex'))) { send(409, { error: 'claim_refused' }); return; }
          principal.owner = user.user.id; principal.detail.owner_user_id = user.user.id;
          send(200, { did, owner_user_id: user.user.id }); return;
        }
        if (principal.owner !== user.user.id) { send(403, { error: 'forbidden' }); return; }
        const detail = principal.detail;
        if (!action) { send(200, detail); return; }
        if (action === 'activity') { send(200, { did, events: this.events.get(did) ?? [] }); return; }
        if (action === 'freeze' || action === 'unfreeze') { detail.is_frozen = action === 'freeze'; send(200, { did, is_frozen: detail.is_frozen }); return; }
        if (action === 'policy') { Object.assign(detail.policy, body); send(200, { did, policy: detail.policy }); return; }
        if (action === 'rules' && method === 'POST') { const rule = { ...body, id: String(this.nextId++) } as unknown as AgentRule; detail.rules.push(rule); send(201, { rule }); return; }
        if (action.startsWith('rules/') && method === 'DELETE') { const count = detail.rules.length; detail.rules = detail.rules.filter(rule => rule.id !== action.split('/')[1]); send(200, { deleted: count !== detail.rules.length }); return; }
        if (action === 'budgets' && method === 'POST') { const budget: AgentBudget = { id: String(this.nextId++), target_contract: null, token: null, cap_wei: String(body.cap_wei), spent_wei: '0', remaining_wei: String(body.cap_wei), expires_at: new Date(Date.now() + Number(body.expires_in_seconds) * 1000).toISOString() }; detail.budgets.push(budget); send(201, { budget }); return; }
        if (action.startsWith('budgets/') && method === 'DELETE') { const count = detail.budgets.length; detail.budgets = detail.budgets.filter(budget => budget.id !== action.split('/')[1]); send(200, { deactivated: count !== detail.budgets.length }); return; }
        if (action === 'fund' || action === 'sweep') {
          const from = action === 'fund' ? this.owner : detail.wallet!.address;
          const to = action === 'fund' ? detail.wallet!.address : this.owner;
          if (body.token !== null && body.token !== undefined && body.token !== USDL_ADDRESS) throw new Error('Only the quote token is accepted');
          const hash = body.token
            ? await this.writer.writeContract({ account: from, chain, address: USDL_ADDRESS, abi: erc20Abi, functionName: 'transfer', args: [to, BigInt(String(body.amount))] })
            : await this.writer.sendTransaction({ account: from, chain, to, value: BigInt(String(body.amount)) });
          const receipt = await this.reader.waitForTransactionReceipt({ hash });
          if (receipt.status !== 'success') throw new Error('The transfer reverted');
          if (action === 'sweep') this.events.set(did, [{ kind: 'transaction', to_address: to, value_wei: body.token ? '0' : String(body.amount), tx_hash: hash, created_at: new Date().toISOString() }, ...(this.events.get(did) ?? [])]);
          send(200, { tx_hash: hash, from, to }); return;
        }
        send(404, { error: 'not_found' });
      } catch (error) { send(500, { error: 'request_failed', message: String(error) }); }
    })());
    await new Promise<void>(done => this.server.listen(0, '127.0.0.1', done));
    const bound = this.server.address();
    if (!bound || typeof bound === 'string') throw new Error('A gateway port is required');
    this.origin = `http://127.0.0.1:${bound.port}`;
    this.wallet = new PaxeerWallet({ apiUrl: this.origin, supabaseUrl: this.origin, supabaseAnonKey: 'agent-wallet-browser' });
    await this.reset();
  }

  private publicWallet(address: Address): PublicWallet { return { id: address, address, chain_id: 125, created_at: new Date().toISOString(), last_used_at: null }; }
  private addPrincipal(did: string, publicKey: string, owner: string | null, address: Address) {
    this.principals.set(did, { did, publicKey, label: did.split(':')[2], owner, active: owner !== null, detail: { did, label: did.split(':')[2], owner_user_id: owner ?? '', is_frozen: false, wallet: { id: address, address, native_wei: '0' }, policy: { ...defaultPolicy }, rules: [], budgets: [] } });
  }
  private mintSession(id: string): Session {
    const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
    const payload = Buffer.from(JSON.stringify({ sub: id, aud: 'authenticated', role: 'authenticated', iss: `${this.origin}/auth/v1`, exp: Math.floor(Date.now() / 1000) + 3600 })).toString('base64url');
    const input = `${header}.${payload}`;
    return { access_token: `${input}.${createHmac('sha256', this.tokenSecret).update(input).digest('base64url')}`, refresh_token: randomBytes(32).toString('hex'), token_type: 'bearer', expires_in: 3600, user: { id, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } };
  }
  private authenticate(header: string | undefined) {
    if (!header?.startsWith('Bearer ')) throw new Error('Owner bearer is required');
    const [a, b, signature] = header.slice(7).split('.');
    if (signature !== createHmac('sha256', this.tokenSecret).update(`${a}.${b}`).digest('base64url')) throw new Error('Owner bearer is invalid');
    const claims = JSON.parse(Buffer.from(b, 'base64url').toString());
    if (claims.exp <= Date.now() / 1000 || ![ownerId, otherId].includes(claims.sub)) throw new Error('Owner session is invalid');
    return this.mintSession(claims.sub);
  }
  async reset() {
    this.requests = []; this.principals.clear(); this.events.clear(); this.fail.clear(); this.challenges.clear(); this.nonces.clear(); this.holdInventory = null;
    const keys = generateKeyPairSync('ed25519'); this.existingKey = keys.privateKey;
    const publicKey = keys.publicKey.export({ type: 'spki', format: 'der' }).subarray(-32).toString('hex');
    this.agentDid = `did:matrix:trader:${publicKey.slice(0, 16)}`;
    this.addPrincipal(this.agentDid, publicKey, ownerId, this.agentAddress);
    this.session = this.mintSession(ownerId);
    const result = await this.wallet.supabase.auth.setSession(this.session);
    if (result.error) throw result.error;
  }
  async switchOwner() { const session = this.mintSession(otherId); const result = await this.wallet.supabase.auth.setSession(session); if (result.error) throw result.error; return session; }
  async stop() {
    if (this.wallet) { await this.wallet.supabase.auth.signOut(); this.wallet.supabase.auth.stopAutoRefresh(); }
    if (this.server) await new Promise<void>((done, reject) => this.server.close(error => error ? reject(error) : done()));
    if (this.anvil && this.anvil.exitCode === null) { this.anvil.kill('SIGTERM'); await new Promise<void>(done => this.anvil.once('exit', () => done())); }
  }
}
