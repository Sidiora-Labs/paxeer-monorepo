import { randomUUID } from 'node:crypto';
import { createServer as httpServer, type Server as HttpServer } from 'node:http';
import { createServer as tcpServer, type Server as TcpServer, type Socket } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';
import Fastify from 'fastify';
import { keccak256, parseTransaction, recoverTransactionAddress, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import type { ActionRow } from '../src/db/actions.js';
import type { WorkerHandle } from '../src/agent/actions/worker.js';

// Task record 22.33.181: action-lease-duration ->
// actionLeaseDurationCoversSigningAndReceiptTimeoutWithCompetingWorkers,
// actionLeaseLossStopsDelayedSigningAndFencesRelease,
// actionLeaseRenewalErrorStopsAdvancement,
// actionLeaseLossDuringReceiptDoesNotPersistStaleOutcome,
// actionLeaseExpiryDrainsPendingRenewal,
// actionOperationDeadlineBoundsDelayedSigning.
// No pre-change verify ran: the explicit implementation-first gate order forbids it.
// Wiring request: wallet domain, services/wallet/gateway/src/index.ts: await
// actionWorker.stop() before closing Fastify/the database pool or exiting.

type Row = Record<string, any>;
type Answer = { rows: Row[]; count?: number };

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

/** An in-memory SQL peer on the real pg wire, following the gateway's committed
 * recovery fixture protocol. Unsupported statements fail loudly. Production
 * claims, renewal, codecs, policy, custody signing, journals and RPC clients run.
 */
class DatabaseFixture {
  row: Row | null = null;
  wallet: Row = {};
  principal: Row = {};
  signatures: Row[] = [];
  accounting: Row[] = [];
  reservation: Row | null = null;
  outstanding: Row[] = [];
  nonce = { next_nonce: '0', latest_seen: '0' };
  claims: Array<{ attempt: number; at: number }> = [];
  renewals: number[] = [];
  releases: number[] = [];
  mutations: Array<{ attempt: number; sql: string }> = [];
  signingEntered = deferred();
  signingGate = deferred();
  signingFinished = deferred();
  renewalEntered = deferred();
  renewalGate: ReturnType<typeof deferred> | null = null;
  renewalError = false;
  errors: string[] = [];
  private server?: TcpServer;
  private sockets = new Set<Socket>();
  private claimOwner: Socket | null = null;
  url = '';

  reset() {
    this.row = null;
    this.signatures = [];
    this.accounting = [];
    this.reservation = null;
    this.outstanding = [];
    this.nonce = { next_nonce: '0', latest_seen: '0' };
    this.claims = [];
    this.renewals = [];
    this.releases = [];
    this.mutations = [];
    this.signingEntered = deferred();
    this.signingGate = deferred();
    this.signingFinished = deferred();
    this.renewalEntered = deferred();
    this.renewalGate = null;
    this.renewalError = false;
    this.errors = [];
  }

  async execute(sql: string, args: Array<string | null>, socket: Socket): Promise<Answer> {
    const q = sql.replace(/\s+/g, ' ').trim().toLowerCase();
    const [a, b, c, d, e, f, g, h, i] = args;
    if (q === 'begin') return { rows: [] };
    if (q === 'commit' || q === 'rollback') {
      if (this.claimOwner === socket) this.claimOwner = null;
      return { rows: [] };
    }
    if (q.startsWith('select pg_advisory_xact_lock') || q.startsWith('select set_config'))
      return { rows: [] };
    if (q.startsWith('insert into agent_actions')) {
      if (this.row) throw new Error('duplicate fixture action');
      const now = new Date().toISOString();
      this.row = {
        id: a,
        did: b,
        wallet_id: c,
        wallet_address: d,
        kind: e,
        idempotency_key: f,
        request: JSON.parse(g!),
        status: 'received',
        phase: 'intake',
        plan: null,
        approval_tx_hash: null,
        approval_nonce: null,
        call_tx_hash: null,
        call_nonce: null,
        reserved_budget_id: null,
        reserved_value_wei: '0',
        error_code: null,
        error: null,
        credit_verified: false,
        credit: null,
        worker_locked_until: null,
        attempts: 0,
        created_at: now,
        updated_at: now,
        terminal_at: null,
      };
      return { rows: [structuredClone(this.row)], count: 1 };
    }
    if (q.startsWith('select id from agent_actions')) {
      const row = this.row;
      if (
        !row ||
        row.terminal_at ||
        row.error_code === 'AGENT_REAUTHORIZATION_REQUIRED' ||
        !row.plan ||
        (row.worker_locked_until && Date.parse(row.worker_locked_until) >= Date.now()) ||
        this.claimOwner !== null
      )
        return { rows: [] };
      this.claimOwner = socket;
      return { rows: [{ id: row.id }] };
    }
    if (q.includes('from agent_actions')) {
      const matches =
        this.row &&
        (q.includes('where did = $1')
          ? this.row.did === a && this.row.idempotency_key === b
          : this.row.id === a);
      return { rows: matches ? [structuredClone(this.row!)] : [] };
    }
    if (q.startsWith('update agent_actions')) {
      const row = this.row;
      if (!row || row.id !== a) return { rows: [], count: 0 };
      if (q.includes('worker_locked_until = clock_timestamp() +')) {
        this.renewalEntered.resolve();
        if (this.renewalGate) await this.renewalGate.promise;
        if (this.renewalError) throw new Error('injected action lease renewal database failure');
        if (
          row.attempts !== Number(b) ||
          !row.worker_locked_until ||
          Date.parse(row.worker_locked_until) <= Date.now()
        )
          return { rows: [], count: 0 };
        row.worker_locked_until = new Date(Date.now() + Number(c)).toISOString();
        this.renewals.push(row.attempts);
        return { rows: [{ id: row.id }], count: 1 };
      }
      if (q.includes('attempts = attempts + 1')) {
        if (this.claimOwner !== socket) throw new Error('claim lacks row lock');
        row.attempts++;
        row.worker_locked_until = new Date(Date.now() + Number(b)).toISOString();
        this.claims.push({ attempt: row.attempts, at: Date.now() });
      } else if (q.includes('worker_locked_until = null')) {
        if (q.includes('attempts = $2') && row.attempts !== Number(b))
          return { rows: [], count: 0 };
        this.releases.push(Number(b ?? row.attempts));
        row.worker_locked_until = null;
      } else if (q.includes("jsonb_set(request, '{_execution}'")) {
        row.request._execution ??= {};
        row.request._execution[b!] = JSON.parse(c!);
        if (q.includes('_tx_hash = $4')) {
          row[`${b}_tx_hash`] = d;
          row[`${b}_nonce`] = Number(e);
          row.status = f;
          row.phase = g;
          row.error_code = null;
          row.error = null;
        }
        this.mutations.push({ attempt: row.attempts, sql: q });
      } else {
        for (const match of q.matchAll(/\b(\w+) = \$(\d+)/g)) {
          const key = match[1]!,
            index = Number(match[2]) - 1;
          if (key === 'id') continue;
          const value = args[index];
          row[key] =
            ['request', 'plan', 'error', 'credit'].includes(key) && value !== null
              ? JSON.parse(value!)
              : key.endsWith('_nonce')
                ? Number(value)
                : value;
        }
        this.mutations.push({ attempt: row.attempts, sql: q });
        if (q.includes('terminal_at = coalesce')) row.terminal_at ??= new Date().toISOString();
      }
      row.updated_at = new Date().toISOString();
      return { rows: q.includes('returning') ? [structuredClone(row)] : [], count: 1 };
    }
    if (q.includes('from agent_principals')) return { rows: [this.principal] };
    if (q.includes('from agent_policies'))
      return {
        rows: [
          {
            did: this.principal.did,
            mode: 'full',
            max_tx_value_wei: '1000000',
            max_daily_value_wei: '1000000',
            rate_limit_per_min: 1000,
            max_approve_wei: '1000000',
            allow_native_transfer: true,
            withdrawal_allowlist_only: false,
            daily_reset_utc_hour: 0,
          },
        ],
      };
    if (q.includes('from agent_policy_rules')) return { rows: [] };
    if (q.includes('from wallets')) {
      if (q.includes('for share')) {
        this.signingEntered.resolve();
        await this.signingGate.promise;
      }
      return { rows: [{ ...this.wallet, has_envelope: true }] };
    }
    if (q.includes('custody_signing_reservations')) {
      if (q.startsWith('insert')) {
        this.reservation ??= {
          address: a,
          chain_id: Number(b),
          action_id: c,
          nonce: d,
          signed_hash: null,
        };
        return { rows: [], count: 1 };
      }
      if (q.startsWith('delete')) {
        if (this.reservation?.action_id === a && this.reservation.nonce === b)
          this.reservation = null;
        return { rows: [], count: 1 };
      }
      if (q.startsWith('update')) {
        if (
          !this.reservation ||
          this.reservation.action_id !== a ||
          this.reservation.nonce !== b ||
          this.reservation.signed_hash !== null
        )
          return { rows: [], count: 0 };
        this.reservation.signed_hash = c;
        return { rows: [], count: 1 };
      }
      return { rows: this.reservation ? [structuredClone(this.reservation)] : [] };
    }
    if (q.startsWith('insert into wallet_signatures')) {
      if (q.includes('select w.user_id')) return { rows: [], count: 1 };
      const row = {
        id: String(this.signatures.length + 1),
        user_id: a,
        wallet_id: b,
        address: c,
        kind: d,
        to_address: e,
        value_wei: f,
        chain_id: Number(g),
        request_hash: h,
        principal_did: i,
        tx_hash: null,
      };
      this.signatures.push(row);
      return { rows: [{ id: row.id }], count: 1 };
    }
    if (q.includes('from wallet_signatures') && q.includes('count(*)'))
      return { rows: [{ count: String(this.signatures.length) }] };
    if (q.startsWith('insert into wallet_signature_reservations')) {
      this.accounting.push({
        wallet_id: a,
        kind: b,
        request_hash: c,
        operation_hash: d,
        signature_id: e,
        signed_evidence: null,
        tx_hash: null,
      });
      return { rows: [], count: 1 };
    }
    if (q.includes('from wallet_signature_reservations r'))
      return {
        rows: this.accounting
          .filter(
            (r) =>
              r.wallet_id === a && r.kind === b && (r.request_hash === c || r.operation_hash === d)
          )
          .map((r) => ({
            ...this.signatures.find((s) => s.id === r.signature_id),
            ...r,
            expired: false,
          })),
      };
    if (q.startsWith('update wallet_signature_reservations')) {
      const row = this.accounting.find((r) => r.signature_id === a);
      if (!row || (row.signed_evidence !== null && row.signed_evidence !== b))
        return { rows: [], count: 0 };
      row.signed_evidence = b;
      row.tx_hash = c;
      return { rows: [{ signature_id: a }], count: 1 };
    }
    if (q.startsWith('update wallet_signatures')) {
      const row = this.signatures.find((r) => r.id === a);
      if (!row) return { rows: [], count: 0 };
      row.tx_hash = b;
      this.signingFinished.resolve();
      return { rows: [], count: 1 };
    }
    if (q.startsWith('insert into nonce_allocations')) return { rows: [], count: 1 };
    if (q.includes('from nonce_allocations')) return { rows: [this.nonce] };
    if (q.startsWith('update nonce_allocations')) {
      if (q.includes('returning latest_seen'))
        this.nonce.latest_seen = String(Math.max(Number(this.nonce.latest_seen), Number(b)));
      return { rows: q.includes('returning') ? [this.nonce] : [], count: 1 };
    }
    if (q.startsWith('insert into nonce_outstanding')) {
      const row = this.outstanding.find((r) => r.address === a && r.nonce === b);
      if (row) {
        row.tx_hash = c;
        row.member = d;
      } else this.outstanding.push({ address: a, nonce: b, tx_hash: c, member: d });
      return { rows: [], count: 1 };
    }
    if (q.startsWith('delete from nonce_outstanding')) {
      this.outstanding = this.outstanding.filter(
        (r) =>
          r.address !== a || (q.includes('nonce <') ? Number(r.nonce) >= Number(b) : r.nonce !== b)
      );
      return { rows: [], count: 1 };
    }
    if (q.includes('from nonce_outstanding'))
      return { rows: q.includes('make_interval') ? [] : this.outstanding };
    throw new Error(`unsupported SQL fixture statement: ${q}`);
  }
  async start() {
    const packet = (tag: string, body = Buffer.alloc(0)) => {
      const size = Buffer.alloc(4);
      size.writeInt32BE(body.length + 4);
      return Buffer.concat([Buffer.from(tag), size, body]);
    };
    const int = (value: number, size = 4) => {
      const b = Buffer.alloc(size);
      size === 2 ? b.writeInt16BE(value) : b.writeInt32BE(value);
      return b;
    };
    const str = (value: string) => Buffer.from(`${value}\0`);
    this.server = tcpServer((socket) => {
      socket.setNoDelay(true);
      this.sockets.add(socket);
      let buffer = Buffer.alloc(0);
      let startup = true;
      let sql = '';
      let args: Array<string | null> = [];
      let pending: Answer = { rows: [] };
      let failed = false;
      let chain = Promise.resolve();
      const output = (answer: Answer, description: boolean) => {
        const fields = Object.keys(answer.rows[0] ?? {});
        if (description && fields.length)
          socket.write(
            packet(
              'T',
              Buffer.concat([
                int(fields.length, 2),
                ...fields.map((name) => {
                  const value = answer.rows.find((r) => r[name] !== null)?.[name];
                  const oid =
                    typeof value === 'object'
                      ? 3802
                      : typeof value === 'boolean'
                        ? 16
                        : typeof value === 'number'
                          ? 23
                          : 25;
                  return Buffer.concat([
                    str(name),
                    int(0),
                    int(0, 2),
                    int(oid),
                    int(-1, 2),
                    int(-1),
                    int(0, 2),
                  ]);
                }),
              ])
            )
          );
        else if (description) socket.write(packet('n'));
        if (!description) {
          for (const row of answer.rows)
            socket.write(
              packet(
                'D',
                Buffer.concat([
                  int(fields.length, 2),
                  ...fields.map((name) => {
                    const value = row[name];
                    if (value === null || value === undefined) return int(-1);
                    const bytes = Buffer.from(
                      typeof value === 'object'
                        ? JSON.stringify(value)
                        : typeof value === 'boolean'
                          ? value
                            ? 't'
                            : 'f'
                          : String(value)
                    );
                    return Buffer.concat([int(bytes.length), bytes]);
                  }),
                ])
              )
            );
          socket.write(packet('C', str(`SELECT ${answer.count ?? answer.rows.length}`)));
        }
      };
      const handle = async (tag: string, body: Buffer) => {
        if (tag === 'startup') {
          socket.write(packet('R', int(0)));
          socket.write(packet('Z', Buffer.from('I')));
          return;
        }
        if (tag === 'X') {
          socket.end();
          return;
        }
        if (tag === 'S') {
          failed = false;
          socket.write(packet('Z', Buffer.from('I')));
          return;
        }
        if (failed) return;
        try {
          if (tag === 'P') {
            let n = body.indexOf(0);
            sql = body.subarray(n + 1, body.indexOf(0, n + 1)).toString();
            socket.write(packet('1'));
          } else if (tag === 'B') {
            let offset = body.indexOf(0) + 1;
            offset = body.indexOf(0, offset) + 1;
            const formats = body.readInt16BE(offset);
            offset += 2 + formats * 2;
            const count = body.readInt16BE(offset);
            offset += 2;
            args = [];
            for (let n = 0; n < count; n++) {
              const length = body.readInt32BE(offset);
              offset += 4;
              args.push(length === -1 ? null : body.subarray(offset, offset + length).toString());
              if (length !== -1) offset += length;
            }
            socket.write(packet('2'));
          } else if (tag === 'D') {
            pending = await this.execute(sql, args, socket);
            output(pending, true);
          } else if (tag === 'E') output(pending, false);
          else if (tag === 'Q') {
            const answer = await this.execute(body.subarray(0, -1).toString(), [], socket);
            output(answer, true);
            output(answer, false);
            socket.write(packet('Z', Buffer.from('I')));
          } else throw new Error(`unsupported PostgreSQL message ${tag}`);
        } catch (error) {
          const message = (error as Error).message;
          if (!message.startsWith('injected')) this.errors.push(message);
          failed = true;
          socket.write(
            packet(
              'E',
              Buffer.concat([str('SERROR'), str('CXX000'), str(`M${message}`), Buffer.from([0])])
            )
          );
          if (tag === 'Q') {
            failed = false;
            socket.write(packet('Z', Buffer.from('I')));
          }
        }
      };
      socket.on('data', (chunk) => {
        buffer = Buffer.concat([buffer, chunk]);
        for (;;) {
          if (startup) {
            if (buffer.length < 4) return;
            const n = buffer.readInt32BE(0);
            if (buffer.length < n) return;
            buffer = buffer.subarray(n);
            startup = false;
            chain = chain.then(() => handle('startup', Buffer.alloc(0)));
          } else {
            if (buffer.length < 5) return;
            const n = buffer.readInt32BE(1);
            if (buffer.length < n + 1) return;
            const tag = buffer.subarray(0, 1).toString(),
              body = buffer.subarray(5, n + 1);
            buffer = buffer.subarray(n + 1);
            chain = chain.then(() => handle(tag, body));
          }
        }
      });
      socket.on('error', () => undefined);
      socket.on('close', () => {
        this.sockets.delete(socket);
        if (this.claimOwner === socket) this.claimOwner = null;
      });
    });
    await new Promise<void>((resolve) => this.server!.listen(0, '127.0.0.1', resolve));
    const address = this.server.address();
    if (!address || typeof address === 'string') throw new Error('database did not bind');
    this.url = `postgres://fixture@127.0.0.1:${address.port}/wallet`;
  }
  async stop() {
    for (const socket of this.sockets) socket.destroy();
    await new Promise<void>((resolve) => this.server!.close(() => resolve()));
  }
}

const key = privateKeyToAccount(`0x${'12'.repeat(32)}`);
const token = '0x1111111111111111111111111111111111111111';
const spender = '0x2222222222222222222222222222222222222222';
const contract = '0x3333333333333333333333333333333333333333';
const db = new DatabaseFixture();
const app = Fastify({ logger: false });
const workers: WorkerHandle[] = [];
const warnings: string[] = [];
const broadcasts: Hex[] = [];
const rpcErrors: string[] = [];
let receiptEntered = deferred();
let rpcServer: HttpServer;
let database: typeof import('../src/db/pool.js');
let actions: typeof import('../src/db/actions.js');
let worker: typeof import('../src/agent/actions/worker.js');
let leases: typeof import('../src/agent/actions/lease.js');
let walletModule: typeof import('../src/db/wallets.js');
let crypto: typeof import('../src/crypto.js');
let env: (typeof import('../src/env.js'))['env'];
const leaseMs = 500;

async function within(promise: Promise<unknown>, ms = 4000): Promise<void> {
  let timer: NodeJS.Timeout | undefined;
  try {
    await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timer = setTimeout(
          () => reject(new Error(`fixture event did not settle within ${ms}ms`)),
          ms
        );
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

beforeAll(async () => {
  await db.start();
  rpcServer = httpServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    try {
      const wire = JSON.parse(Buffer.concat(chunks).toString());
      const answer = (call: { id: number; method: string; params: any[] }) => {
        let result: unknown;
        switch (call.method) {
          case 'eth_chainId':
            result = '0x7d';
            break;
          case 'eth_blockNumber':
            result = '0x64';
            break;
          case 'eth_getTransactionCount':
            result = '0x0';
            break;
          case 'eth_getBlockByNumber':
            result = {
              number: '0x64',
              hash: `0x${'01'.repeat(32)}`,
              parentHash: `0x${'02'.repeat(32)}`,
              timestamp: '0x1',
              baseFeePerGas: '0x1',
              gasLimit: '0x1c9c380',
              gasUsed: '0x0',
              difficulty: '0x0',
              totalDifficulty: '0x0',
              size: '0x1',
              transactions: [],
              uncles: [],
              extraData: '0x',
              miner: contract,
              nonce: '0x0000000000000000',
              sha3Uncles: `0x${'00'.repeat(32)}`,
              receiptsRoot: `0x${'00'.repeat(32)}`,
              stateRoot: `0x${'00'.repeat(32)}`,
              transactionsRoot: `0x${'00'.repeat(32)}`,
              logsBloom: `0x${'00'.repeat(256)}`,
            };
            break;
          case 'eth_maxPriorityFeePerGas':
          case 'eth_gasPrice':
            result = '0x1';
            break;
          case 'eth_estimateGas':
            result = '0x186a0';
            break;
          case 'eth_sendRawTransaction': {
            const raw = call.params[0] as Hex;
            broadcasts.push(raw);
            result = keccak256(raw);
            break;
          }
          case 'eth_getTransactionByHash': {
            const raw = broadcasts.find((tx) => keccak256(tx) === call.params[0]);
            if (!raw) {
              result = null;
              break;
            }
            const tx = parseTransaction(raw);
            result = {
              hash: keccak256(raw),
              blockHash: null,
              blockNumber: null,
              transactionIndex: null,
              from: key.address,
              to: tx.to,
              input: tx.data,
              nonce: `0x${tx.nonce!.toString(16)}`,
              gas: `0x${tx.gas!.toString(16)}`,
              value: '0x0',
              gasPrice: '0x2',
              maxFeePerGas: '0x3',
              maxPriorityFeePerGas: '0x1',
              chainId: '0x7d',
              type: '0x2',
              accessList: [],
              r: tx.r,
              s: tx.s,
              v: '0x0',
              yParity: '0x0',
            };
            break;
          }
          case 'pax_sequencerReceipt':
            receiptEntered.resolve();
            result = null;
            break;
          default:
            throw new Error(`unexpected RPC method ${call.method}`);
        }
        return { jsonrpc: '2.0', id: call.id, result };
      };
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(Array.isArray(wire) ? wire.map(answer) : answer(wire)));
    } catch (err) {
      rpcErrors.push(String(err));
      res.writeHead(500);
      res.end();
    }
  });
  await new Promise<void>((resolve) => rpcServer.listen(0, '127.0.0.1', resolve));
  const address = rpcServer.address();
  if (!address || typeof address === 'string') throw new Error('RPC did not bind');
  const url = `http://127.0.0.1:${address.port}`;
  process.env.DATABASE_URL = db.url;
  process.env.HYPERPAXEER_RPC_URL = url;
  process.env.RPC_URLS = url;
  process.env.SEQUENCER_RPC_URL = url;
  process.env.ACTION_RECEIPT_TIMEOUT_MS = String(leaseMs);
  process.env.ACTION_WORKER_INTERVAL_MS = '20';
  process.env.ACTION_CONFIRMATIONS = '1';
  ({ env } = await import('../src/env.js'));
  database = await import('../src/db/pool.js');
  actions = await import('../src/db/actions.js');
  worker = await import('../src/agent/actions/worker.js');
  leases = await import('../src/agent/actions/lease.js');
  walletModule = await import('../src/db/wallets.js');
  crypto = await import('../src/crypto.js');
});

afterEach(async () => {
  db.signingGate.resolve();
  db.renewalGate?.resolve();
  await Promise.all(workers.splice(0).map((handle) => handle.stop()));
  if (db.signatures.length) await within(db.signingFinished.promise);
  expect(db.errors).toEqual([]);
  expect(rpcErrors).toEqual([]);
});

afterAll(async () => {
  await database?.closePool();
  await db.stop();
  rpcServer?.closeAllConnections();
  await new Promise<void>((resolve) => rpcServer.close(() => resolve()));
  await app.close();
});

async function prepareAction(): Promise<ActionRow> {
  db.reset();
  warnings.length = 0;
  broadcasts.length = 0;
  rpcErrors.length = 0;
  receiptEntered = deferred();
  const did = `did:key:z${'1'.repeat(46)}`;
  const { ciphertext } = crypto.encrypt(
    Buffer.from('12'.repeat(32), 'hex'),
    crypto.loadMasterKey(env.WALLET_MASTER_KEY),
    env.WALLET_MASTER_KEY_VERSION
  );
  db.wallet = {
    id: randomUUID(),
    user_id: walletModule.agentWalletUserId(did),
    address: key.address,
    encrypted_private_key: ciphertext,
    key_version: 1,
    chain_id: 125,
    kind: 'agent',
    migrated_at: null,
    attestor_key_id: null,
    is_disabled: false,
    disabled_reason: null,
    archived_at: null,
    created_at: new Date().toISOString(),
    last_used_at: null,
  };
  db.principal = {
    did,
    wallet_id: db.wallet.id,
    owner_user_id: randomUUID(),
    public_key: '11'.repeat(32),
    is_frozen: false,
    label: 'lease fixture',
  };
  const created = await actions.createOrGetAction({
    did,
    walletId: db.wallet.id,
    walletAddress: key.address,
    kind: 'allowance_and_call',
    idempotencyKey: randomUUID(),
    request: {},
  });
  const { buildAllowanceAndCallPlan } = await import('../src/agent/actions/plan.js');
  const plan = buildAllowanceAndCallPlan({
    token,
    spender,
    contract,
    amount: '1',
    method: 'deposit(uint256)',
    args: ['1'],
  });
  return actions.updateAction(created.row.id, {
    status: 'awaiting_approval',
    phase: 'approval_broadcast',
    plan: { ...plan },
  });
}

function startWorker(): WorkerHandle {
  const logger = app.log.child({ fixture: 'action-lease' });
  logger.warn = (entry: any) => {
    warnings.push(String(entry.err));
  };
  const handle = worker.startActionWorker(logger);
  workers.push(handle);
  return handle;
}

async function actionLeaseDurationCoversSigningAndReceiptTimeoutWithCompetingWorkers() {
  await prepareAction();
  const first = startWorker();
  await within(db.signingEntered.promise);
  const second = startWorker();
  await delay(leaseMs + 150);
  expect(db.row!.attempts).toBe(1);
  expect(db.renewals.filter((attempt) => attempt === 1).length).toBeGreaterThanOrEqual(2);
  expect(broadcasts).toEqual([]);
  db.signingGate.resolve();
  await within(receiptEntered.promise);
  let drained = false;
  const stopping = Promise.all([first.stop(), second.stop()]).then(() => {
    drained = true;
  });
  await delay(leaseMs + 150);
  expect(drained).toBe(false);
  expect(db.row!.attempts).toBe(2);
  expect(db.renewals.filter((attempt) => attempt === 2).length).toBeGreaterThanOrEqual(2);
  expect(db.row!.worker_locked_until).not.toBeNull();
  await within(stopping);
  const retained = actions.actionLegExecution(db.row as ActionRow, 'approval').current!;
  expect(broadcasts).toEqual([retained.raw]);
  expect(retained.hash).toBe(keccak256(retained.raw));
  expect(await recoverTransactionAddress({ serializedTransaction: retained.raw })).toBe(
    key.address
  );
  expect(parseTransaction(retained.raw).nonce).toBe(0);
  expect(db.row!.approval_tx_hash).toBe(retained.hash);
  expect(db.row!.error_code).toBe('APPROVAL_PENDING');
  expect(db.row!.error.cause.still_in_mempool).toBe(true);
  expect(db.row!.worker_locked_until).toBeNull();
  expect(warnings).toEqual([]);
  const claims = db.claims.length;
  await delay(60);
  expect(db.claims).toHaveLength(claims);
}

async function actionLeaseLossStopsDelayedSigningAndFencesRelease() {
  await prepareAction();
  const first = startWorker();
  await within(db.signingEntered.promise);
  const stale = await actions.getAction(db.row!.id);
  db.row!.worker_locked_until = new Date(Date.now() - 1).toISOString();
  const competitor = await actions.claimNextAction(leaseMs * 10);
  expect(competitor!.attempts).toBe(stale!.attempts + 1);
  const ownedUntil = db.row!.worker_locked_until;
  const mutations = db.mutations.length;
  await within(first.stop());
  expect(warnings).toContain('action lease renewal lost ownership');
  expect(db.row!.worker_locked_until).toBe(ownedUntil);
  expect(await leases.renewActionLease(stale!, leaseMs)).toBe(false);
  await leases.releaseActionLease(stale!);
  expect(db.row!.worker_locked_until).toBe(ownedUntil);
  db.signingGate.resolve();
  await within(db.signingFinished.promise);
  await delay(60);
  expect(broadcasts).toEqual([]);
  expect(db.mutations).toHaveLength(mutations);
  expect(db.row!.approval_tx_hash).toBeNull();
  expect(db.row!.request._execution.approval.draft.nonce).toBe(0);
  expect(await leases.renewActionLease(competitor!, leaseMs)).toBe(true);
  await leases.releaseActionLease(competitor!);
  expect(db.row!.worker_locked_until).toBeNull();
}

async function actionLeaseLossDuringReceiptDoesNotPersistStaleOutcome() {
  await prepareAction();
  db.signingGate.resolve();
  const handle = startWorker();
  await within(receiptEntered.promise);
  db.row!.worker_locked_until = new Date(Date.now() - 1).toISOString();
  const competitor = await actions.claimNextAction(leaseMs * 10);
  expect(competitor).not.toBeNull();
  const ownedUntil = db.row!.worker_locked_until;
  const mutations = db.mutations.length;
  await within(handle.stop());
  expect(warnings).toContain('action lease renewal lost ownership');
  await delay(1600);
  expect(db.mutations).toHaveLength(mutations);
  expect(db.row!.error_code).toBeNull();
  expect(db.row!.worker_locked_until).toBe(ownedUntil);
  expect(broadcasts).toHaveLength(1);
  await leases.releaseActionLease(competitor!);
}

async function actionLeaseRenewalErrorStopsAdvancement() {
  await prepareAction();
  const handle = startWorker();
  await within(db.signingEntered.promise);
  db.renewalError = true;
  const mutations = db.mutations.length;
  await within(handle.stop());
  expect(warnings).toContain('injected action lease renewal database failure');
  db.signingGate.resolve();
  await within(db.signingFinished.promise);
  await delay(60);
  expect(broadcasts).toEqual([]);
  expect(db.mutations).toHaveLength(mutations);
  expect(db.row!.worker_locked_until).toBeNull();
}

async function actionLeaseExpiryDrainsPendingRenewal() {
  await prepareAction();
  db.renewalGate = deferred();
  const handle = startWorker();
  await within(db.signingEntered.promise);
  await within(db.renewalEntered.promise);
  let stopped = false;
  const stopping = handle.stop().then(() => {
    stopped = true;
  });
  const mutations = db.mutations.length;
  await delay(leaseMs + 100);
  expect(stopped).toBe(false);
  expect(db.releases).toEqual([]);
  db.signingGate.resolve();
  await within(db.signingFinished.promise);
  expect(broadcasts).toEqual([]);
  expect(db.mutations).toHaveLength(mutations);
  db.renewalGate.resolve();
  await within(stopping);
  expect(db.renewals).toEqual([]);
  expect(warnings).toContain('action lease expired');
  expect(db.row!.worker_locked_until).toBeNull();
}

async function actionOperationDeadlineBoundsDelayedSigning() {
  await prepareAction();
  const handle = startWorker();
  await within(db.signingEntered.promise);
  const mutations = db.mutations.length;
  const started = Date.now();
  await within(handle.stop(), leaseMs * 4 + 1000);
  expect(Date.now() - started).toBeLessThan(leaseMs * 4 + 500);
  expect(warnings).toContain('action operation deadline exceeded');
  expect(db.renewals.length).toBeGreaterThanOrEqual(4);
  expect(db.row!.worker_locked_until).toBeNull();
  db.signingGate.resolve();
  await within(db.signingFinished.promise);
  await delay(60);
  expect(broadcasts).toEqual([]);
  expect(db.mutations).toHaveLength(mutations);
}

test(actionLeaseDurationCoversSigningAndReceiptTimeoutWithCompetingWorkers, 10000);
test(actionLeaseLossStopsDelayedSigningAndFencesRelease, 10000);
test(actionLeaseRenewalErrorStopsAdvancement, 10000);
test(actionLeaseLossDuringReceiptDoesNotPersistStaleOutcome, 10000);
test(actionLeaseExpiryDrainsPendingRenewal, 10000);
test(actionOperationDeadlineBoundsDelayedSigning, 10000);
