import Fastify, { type FastifyInstance } from 'fastify';
import {
  createHash,
  createPublicKey,
  generateKeyPairSync,
  randomUUID,
  sign as cryptoSign,
  verify as cryptoVerify,
  X509Certificate,
} from 'node:crypto';
import { mkdtempSync, readFileSync, writeFileSync, rmSync, existsSync } from 'node:fs';
import { createServer as httpServer, type Server as HttpServer } from 'node:http';
import { createServer as httpsServer, type Server as HttpsServer } from 'node:https';
import { createServer as tcpServer, type Server as TcpServer, type Socket } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Pool } from 'pg';
import { SignJWT, exportJWK, generateKeyPair } from 'jose';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  serializeTransaction,
  serializeSignature,
  type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { TestChain } from './support/chain.js';
import { loadProductionWalletSdk } from './support/node-receipts.js';

// Committed loopback TLS credentials; these protect no deployed service.
const TLS_FIXTURE = [
  {
    key: '-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgr3P72Gj2Ohx6wjuq\nQ3+ID4X1DhZGI0s0+XJFwociH/yhRANCAAQNwv8S2/H2OR4yTW6ryCy7Vi+nd2sV\nKERg+iRSFdBKkVQCE0Asq2Vd7YZ3Xl3foQBtDa7feYADO01bYDV1HgdM\n-----END PRIVATE KEY-----\n',
    cert: '-----BEGIN CERTIFICATE-----\nMIIB0zCCAXqgAwIBAgIUeXlkJV8NDsyIs23CMyRrNycWpuAwCgYIKoZIzj0EAwIw\nITEfMB0GA1UEAwwWd2FsbGV0LXJlY292ZXJ5LXBlZXItMDAeFw0yNjExMDQwNDAz\nMDBaFw0zNjExMDEwNDAzMDBaMCExHzAdBgNVBAMMFndhbGxldC1yZWNvdmVyeS1w\nZWVyLTAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQNwv8S2/H2OR4yTW6ryCy7\nVi+nd2sVKERg+iRSFdBKkVQCE0Asq2Vd7YZ3Xl3foQBtDa7feYADO01bYDV1HgdM\no4GPMIGMMB0GA1UdDgQWBBRiCn88GfbIQGhtANgqLMNHgs2J5jAfBgNVHSMEGDAW\ngBRiCn88GfbIQGhtANgqLMNHgs2J5jAPBgNVHRMBAf8EBTADAQH/MBoGA1UdEQQT\nMBGHBH8AAAGCCWxvY2FsaG9zdDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwCgYIKoZIzj0EAwIDRwAwRAIgPbeeIlrWJ4YMnTMehOzXX+N/qzw1XFQv7W7K\nyokPVOECIByNNiM/Id4PmP/9f0nht5uYZyarlP4dPLV9QYn4clem\n-----END CERTIFICATE-----\n',
  },
  {
    key: '-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgKqtI9KAuRswgk6vQ\nwdPGZz5+P5bTp4SR2zrS/VyRO2+hRANCAARkx+4c7pWHpwm8f3uTw1jSYKoq/xiK\n+Y1NSghgwcUlUTgu+FkypgfDl+v5LC7HRl6TAXWwA2zqj1BBzDE+yOkM\n-----END PRIVATE KEY-----\n',
    cert: '-----BEGIN CERTIFICATE-----\nMIIB1DCCAXqgAwIBAgIUQ1lCgRkQPxJMih8aN754+v6L4WgwCgYIKoZIzj0EAwIw\nITEfMB0GA1UEAwwWd2FsbGV0LXJlY292ZXJ5LXBlZXItMTAeFw0yNjExMDQwNDAz\nMDBaFw0zNjExMDEwNDAzMDBaMCExHzAdBgNVBAMMFndhbGxldC1yZWNvdmVyeS1w\nZWVyLTEwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARkx+4c7pWHpwm8f3uTw1jS\nYKoq/xiK+Y1NSghgwcUlUTgu+FkypgfDl+v5LC7HRl6TAXWwA2zqj1BBzDE+yOkM\no4GPMIGMMB0GA1UdDgQWBBQKAs3pxGEDfFlLPLvtbEUuGgzX3jAfBgNVHSMEGDAW\ngBQKAs3pxGEDfFlLPLvtbEUuGgzX3jAPBgNVHRMBAf8EBTADAQH/MBoGA1UdEQQT\nMBGHBH8AAAGCCWxvY2FsaG9zdDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwCgYIKoZIzj0EAwIDSAAwRQIhAPvSCSkPS7DB6jRDL565K7TH7k3IO9JSxYxX\nh8VwA98eAiAIJvI/ZRAcPi4UKXAWgyX0G97q8nxDu9ftbBpZZAHG/w==\n-----END CERTIFICATE-----\n',
  },
  {
    key: '-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgCE5TXXvZp76fFx2C\n5NF7ZfBE4+n9IvXGIme8a6uXnuOhRANCAAQSq1hwU3g6tX6UYdUCp20dRUs0fNol\n5Z1o8X25Wjs5VcWsF3Qu2RKrOvoW7d/iWFMOPtsASleBjDbflY34QUis\n-----END PRIVATE KEY-----\n',
    cert: '-----BEGIN CERTIFICATE-----\nMIIB0zCCAXqgAwIBAgIULoNHgzEQJ6lnhva6x9ElyQAfAq4wCgYIKoZIzj0EAwIw\nITEfMB0GA1UEAwwWd2FsbGV0LXJlY292ZXJ5LXBlZXItMjAeFw0yNjExMDQwNDAz\nMDBaFw0zNjExMDEwNDAzMDBaMCExHzAdBgNVBAMMFndhbGxldC1yZWNvdmVyeS1w\nZWVyLTIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQSq1hwU3g6tX6UYdUCp20d\nRUs0fNol5Z1o8X25Wjs5VcWsF3Qu2RKrOvoW7d/iWFMOPtsASleBjDbflY34QUis\no4GPMIGMMB0GA1UdDgQWBBQFo1y01NSXKJVEiCIvZ7DmcARlnDAfBgNVHSMEGDAW\ngBQFo1y01NSXKJVEiCIvZ7DmcARlnDAPBgNVHRMBAf8EBTADAQH/MBoGA1UdEQQT\nMBGHBH8AAAGCCWxvY2FsaG9zdDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwCgYIKoZIzj0EAwIDRwAwRAIgNlyTnJXluAtPmTMkHsq6wRYaWrfafBUe70mv\n85pn0sMCIBB5n1jgJkkjxLQeJjlj7RGiqNoxb9+XUwwSgPviTAnB\n-----END CERTIFICATE-----\n',
  },
  {
    key: '-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgLFKCrqZDQ7b/fQto\nOE6tBKHl1lpCVcr4oGAjO3wC4X6hRANCAATKbdjKREA/VXEotzu26xO65wcX1ygA\nF8VB8dXaDL0MHcFRHAZgq5Zbx7oUmEo6eGLqk+ntR5zJsaJy755W3XnN\n-----END PRIVATE KEY-----\n',
    cert: '-----BEGIN CERTIFICATE-----\nMIIB1TCCAXqgAwIBAgIUD9Au6C1XHaDlo8LB1OopPGH8bUswCgYIKoZIzj0EAwIw\nITEfMB0GA1UEAwwWd2FsbGV0LXJlY292ZXJ5LXBlZXItMzAeFw0yNjExMDQwNDAz\nMDBaFw0zNjExMDEwNDAzMDBaMCExHzAdBgNVBAMMFndhbGxldC1yZWNvdmVyeS1w\nZWVyLTMwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATKbdjKREA/VXEotzu26xO6\n5wcX1ygAF8VB8dXaDL0MHcFRHAZgq5Zbx7oUmEo6eGLqk+ntR5zJsaJy755W3XnN\no4GPMIGMMB0GA1UdDgQWBBTuZURtSEubD5huejSbwC7PzcvjyTAfBgNVHSMEGDAW\ngBTuZURtSEubD5huejSbwC7PzcvjyTAPBgNVHRMBAf8EBTADAQH/MBoGA1UdEQQT\nMBGHBH8AAAGCCWxvY2FsaG9zdDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwCgYIKoZIzj0EAwIDSQAwRgIhAIh7VOTbbRpynpw2B6krPvXa+KMjcqWJ3tM9\nDEO3OqEOAiEAncCK5kdyDLCH/6sJgz5caV8xL5njHnOtsBE+6DWMvns=\n-----END CERTIFICATE-----\n',
  },
  {
    key: '-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg2eqvscCVUWt8jVrs\na7GAS0j1HXOuK3X4UT21fBi9uiahRANCAARuU0kna0nJNI0th4lm4mP60K3zU6DS\nAYapSLhHPoZdJdmSLcsGRLYE1YQM3ErsDVHoSb2ITMpx9JCCmRyFsGDp\n-----END PRIVATE KEY-----\n',
    cert: '-----BEGIN CERTIFICATE-----\nMIIB1DCCAXqgAwIBAgIUH+gxuk5yiJZEzPLPlx6W2HMvo7gwCgYIKoZIzj0EAwIw\nITEfMB0GA1UEAwwWd2FsbGV0LXJlY292ZXJ5LXBlZXItNDAeFw0yNjExMDQwNDAz\nMDBaFw0zNjExMDEwNDAzMDBaMCExHzAdBgNVBAMMFndhbGxldC1yZWNvdmVyeS1w\nZWVyLTQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARuU0kna0nJNI0th4lm4mP6\n0K3zU6DSAYapSLhHPoZdJdmSLcsGRLYE1YQM3ErsDVHoSb2ITMpx9JCCmRyFsGDp\no4GPMIGMMB0GA1UdDgQWBBROrJXuIlXAqkCLV/U5mzx1/exo0DAfBgNVHSMEGDAW\ngBROrJXuIlXAqkCLV/U5mzx1/exo0DAPBgNVHRMBAf8EBTADAQH/MBoGA1UdEQQT\nMBGHBH8AAAGCCWxvY2FsaG9zdDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwCgYIKoZIzj0EAwIDSAAwRQIhAJ2e672Mbc54mdcscj7ldcQiHHyWHL4PrZlq\nU0TSC4DEAiARwQ5vP6xQ8+6Wj/Rs74m2VR1BmfbGcBH/wN+BvdQybg==\n-----END CERTIFICATE-----\n',
  },
] as const;

// Task record 22.31.370:
// evm-approval-recovery -> evmApprovalRecoveryAcrossEveryBoundary
// lx-approval-recovery -> lxApprovalRecoveryAndDefiniteRefusal
// human-unknown-spend-unaccounted -> humanUnknownSendKeepsDailySpendAfterReopen
// hosted-evm-routes-missing -> hostedEvmSdkRoundTrip
// hosted-sponsored-quote-missing -> hostedSponsoredQuoteAndBudgetRefusal
// No pre-change verify ran: the explicit gate order forbids testing before implementation.
// Wiring request: wallet domain, services/wallet/gateway/src/db/migrate.ts: register APPROVAL_RECOVERY_SQL centrally; the routes currently apply it before admitting requests.
// Wiring request: wallet attestor domain, services/wallet/attestor/internal/server/sign.go: authenticated read-only retrieval of a completed original session after approval expiry; current replay requires a still-valid approval.
// Wiring request: hosted gateway domain, services/platform/hosted/gateway/tests/unified_service_paths.rs: execute the added Rust contract/proxy tests in its Rust gate; this task permits only Node gates.

type Row = Record<string, any>;
type Answer = { rows: Row[]; count?: number };

/** A committed, in-memory SQL peer, exposed over the PostgreSQL wire protocol.
 * Unsupported statements fail instead of pretending that a write succeeded.
 * The production Pool, SQL queries, transactions, codecs and route handlers run unchanged.
 */
class DatabaseFixture {
  state: {
    wallets: Row[];
    approvals: Record<string, Row>;
    progress: Record<string, Row>;
    signatures: Row[];
    accounting: Row[];
    nonces: Record<string, Row>;
    outstanding: Row[];
    sequence: number;
  };
  failOnce: string | null = null;
  failSignedJournal = false;
  readonly errors: string[] = [];
  private server?: TcpServer;
  private sockets = new Set<Socket>();
  private locks = new Map<string, { owner: Socket; waiters: Array<() => void> }>();
  url = '';
  constructor(private readonly file: string) {
    this.state = existsSync(file)
      ? JSON.parse(readFileSync(file, 'utf8'))
      : {
          wallets: [],
          approvals: {},
          progress: {},
          signatures: [],
          accounting: [],
          nonces: {},
          outstanding: [],
          sequence: 0,
        };
  }
  checkpoint() {
    writeFileSync(this.file, JSON.stringify(this.state));
  }
  async execute(sql: string, args: Array<string | null>, socket: Socket): Promise<Answer> {
    const q = sql.replace(/\s+/g, ' ').trim().toLowerCase();
    const [a, b, c, d, e, f, g, h, i, j] = args;
    if (this.failOnce && q.includes(this.failOnce)) {
      this.failOnce = null;
      throw new Error('injected database interruption before commit');
    }
    if (q.startsWith('begin') || q === 'commit' || q === 'rollback') {
      this.checkpoint();
      return { rows: [] };
    }
    if (q.includes('pg_advisory_unlock')) {
      const lock = this.locks.get(a!);
      if (lock?.owner === socket) {
        this.locks.delete(a!);
        lock.waiters.shift()?.();
      }
      return { rows: [{ pg_advisory_unlock: true }] };
    }
    if (q.includes('pg_advisory_lock(')) {
      while (this.locks.has(a!) && this.locks.get(a!)!.owner !== socket)
        await new Promise<void>((resolve) => this.locks.get(a!)!.waiters.push(resolve));
      this.locks.set(a!, { owner: socket, waiters: [] });
      return { rows: [{ pg_advisory_lock: null }] };
    }
    if (
      q.includes('pg_advisory_xact_lock') ||
      q.includes('set_config(') ||
      q.startsWith('create table')
    )
      return { rows: [] };
    if (q.includes('from wallet_custody_authority'))
      return { rows: [{ sequence: String(this.state.sequence), inventory_sequence: '1' }] };
    if (q.startsWith('update wallet_custody_authority')) {
      if (q.includes('inventory_sequence')) this.state.sequence = Number(a);
      return { rows: [], count: 1 };
    }
    if (
      q.includes('from agent_principals') ||
      q.includes('from custody_budget_reservations') ||
      q.includes('from agent_actions')
    )
      return { rows: [] };
    if (q.startsWith('insert into rate_limit_windows'))
      return { rows: [{ hits: 1, retry_after: 60 }] };
    if (q.startsWith('insert into signing_audit')) return { rows: [], count: 1 };
    if (q.includes('from wallets')) {
      if (q.includes('where archived_at is not null')) return { rows: [] };
      const wallet = this.state.wallets.find((w) =>
        q.includes('where id = $1') ? w.id === a : w.user_id === a
      );
      return {
        rows: wallet ? [{ ...wallet, has_envelope: wallet.encrypted_private_key !== null }] : [],
      };
    }
    if (q.startsWith('update wallets')) return { rows: [], count: 1 };
    if (q.startsWith('insert into wallet_approval_recovery')) {
      if (this.failSignedJournal && d === 'signed') {
        this.failSignedJournal = false;
        throw new Error('injected signature journal interruption');
      }
      const key = `${a}:${b}`;
      const prior = this.state.progress[key];
      if (prior && prior.binding !== c) return { rows: [], count: 0 };
      this.state.progress[key] = {
        domain: a,
        approval_id: b,
        binding: c,
        phase: d,
        evidence: JSON.parse(e!),
      };
      this.checkpoint();
      return { rows: [{ approval_id: b }], count: 1 };
    }
    if (q.includes('join wallet_approval_recovery')) {
      const domain = q.includes('wallet_evm_approvals') ? 'evm' : 'lx';
      return {
        rows: Object.values(this.state.approvals)
          .filter(
            (row) =>
              row.domain === domain &&
              this.state.progress[`${domain}:${row.id}`]?.phase === 'signed' &&
              (domain === 'evm' || row.state === 'signing_unknown')
          )
          .map((row) => ({ id: row.id, principal: row.principal })),
      };
    }
    if (q.includes('from wallet_approval_recovery') && !/wallet_(evm|lx)_approvals/.test(q)) {
      let rows = Object.values(this.state.progress);
      if (q.includes("domain='human'")) rows = rows.filter((r) => r.domain === 'human');
      else rows = rows.filter((r) => r.domain === a && r.approval_id === b);
      if (q.includes("phase='signed'")) rows = rows.filter((r) => r.phase === 'signed');
      if (q.includes('binding=$1')) rows = rows.filter((r) => r.binding === a);
      return { rows: structuredClone(rows) };
    }
    if (/wallet_(evm|lx)_approvals/.test(q)) {
      const domain = q.includes('wallet_evm_approvals') ? 'evm' : 'lx';
      if (q.startsWith('delete')) return { rows: [], count: 0 };
      if (q.startsWith('insert')) {
        const artifact = JSON.parse(e!);
        this.state.approvals[a!] = {
          id: a,
          wallet_id: b,
          principal: c,
          session_id: d,
          domain,
          state: 'reviewed',
          artifact,
          approval: null,
          evidence: null,
        };
        return { rows: [], count: 1 };
      }
      if (q.includes('count(*)'))
        return {
          rows: [
            {
              count: String(
                Object.values(this.state.approvals).filter(
                  (r) =>
                    r.domain === domain &&
                    r.principal === a &&
                    r.state !== 'signed' &&
                    this.state.progress[`${domain}:${r.id}`]?.phase !== 'refused'
                ).length
              ),
            },
          ],
        };
      const row = this.state.approvals[a!];
      if (!row || row.domain !== domain) return { rows: [], count: 0 };
      if (q.startsWith('update')) {
        if (q.includes("state='approved'")) {
          row.state = 'approved';
          row.approval = JSON.parse(b!);
        } else if (q.includes("state='signing_unknown'")) row.state = 'signing_unknown';
        else if (q.includes("state='signed'")) {
          row.state = 'signed';
          row.evidence = JSON.parse(b!);
        } else throw new Error(`unsupported approval mutation: ${q}`);
        return { rows: [], count: 1 };
      }
      return { rows: row.principal === b ? [structuredClone(row)] : [] };
    }
    if (q.startsWith('insert into wallet_signatures')) {
      const row = {
        id: String(this.state.signatures.length + 1),
        user_id: a,
        wallet_id: b,
        address: c,
        kind: d,
        to_address: e,
        value_wei: f,
        chain_id: Number(g),
        request_hash: h,
        principal_did: null,
        tx_hash: null,
        ip: i,
        user_agent: j,
      };
      this.state.signatures.push(row);
      return { rows: [{ id: row.id }], count: 1 };
    }
    if (q.includes('from wallet_signatures') && !q.includes('join wallet_signatures')) {
      const rows = this.state.signatures.filter((r) => r.user_id === a);
      if (q.includes('count(*)')) return { rows: [{ count: String(rows.length) }] };
      if (q.includes('sum(value_wei)'))
        return {
          rows: [{ total: rows.reduce((total, r) => total + BigInt(r.value_wei), 0n).toString() }],
        };
    }
    if (q.startsWith('update wallet_signatures')) {
      const row = this.state.signatures.find((r) => r.id === a);
      if (!row) return { rows: [], count: 0 };
      if (q.includes('tx_hash')) row.tx_hash = b;
      else {
        row.ip = b;
        row.user_agent = c;
      }
      return { rows: [], count: 1 };
    }
    if (q.startsWith('insert into wallet_signature_reservations')) {
      const literal = q.includes("values($1,'transaction'");
      this.state.accounting.push({
        wallet_id: a,
        kind: literal ? 'transaction' : b,
        request_hash: literal ? b : c,
        operation_hash: literal ? c : d,
        signature_id: literal ? d : e,
        signed_evidence: null,
        tx_hash: null,
      });
      return { rows: [], count: 1 };
    }
    if (q.includes('from wallet_signature_reservations r')) {
      const rows = this.state.accounting.filter(
        (r) =>
          r.wallet_id === a &&
          r.kind === b &&
          (r.request_hash === c || r.operation_hash === d || r.tx_hash === d)
      );
      return {
        rows: rows.map((r) => ({
          ...this.state.signatures.find((s) => s.id === r.signature_id),
          ...r,
          expired: false,
        })),
      };
    }
    if (q.startsWith('update wallet_signature_reservations')) {
      const row = this.state.accounting.find((r) => r.signature_id === a);
      if (!row || (row.signed_evidence !== null && row.signed_evidence !== b))
        return { rows: [], count: 0 };
      row.signed_evidence = b;
      row.tx_hash = c;
      return { rows: [{ signature_id: a }], count: 1 };
    }
    if (q.includes('custody_signing_reservations')) return { rows: [] };
    if (q.startsWith('insert into nonce_allocations')) {
      this.state.nonces[a!] ??= { next_nonce: '0', latest_seen: '0' };
      return { rows: [], count: 1 };
    }
    if (q.includes('from nonce_allocations')) return { rows: [this.state.nonces[a!]] };
    if (q.startsWith('update nonce_allocations')) {
      const row = this.state.nonces[a!];
      if (q.includes('returning latest_seen'))
        row.latest_seen = String(Math.max(Number(row.latest_seen), Number(b)));
      return { rows: q.includes('returning') ? [row] : [], count: 1 };
    }
    if (q.startsWith('insert into nonce_outstanding')) {
      const row = this.state.outstanding.find((r) => r.address === a && r.nonce === b);
      if (row) {
        row.tx_hash = c;
        row.member = d;
      } else this.state.outstanding.push({ address: a, nonce: b, tx_hash: c, member: d });
      return { rows: [], count: 1 };
    }
    if (q.startsWith('delete from nonce_outstanding')) {
      this.state.outstanding = this.state.outstanding.filter(
        (r) =>
          r.address !== a || (q.includes('nonce <') ? Number(r.nonce) >= Number(b) : r.nonce !== b)
      );
      return { rows: [], count: 1 };
    }
    if (q.includes('from nonce_outstanding'))
      return {
        rows: q.includes('make_interval')
          ? []
          : this.state.outstanding.filter((r) => r.address === a),
      };
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
        for (const [key, lock] of this.locks)
          if (lock.owner === socket) {
            this.locks.delete(key);
            lock.waiters.shift()?.();
          }
      });
    });
    await new Promise<void>((resolve) => this.server!.listen(0, '127.0.0.1', resolve));
    const address = this.server.address();
    if (!address || typeof address === 'string') throw new Error('database did not bind');
    this.url = `postgres://fixture@127.0.0.1:${address.port}/wallet`;
  }
  async stop() {
    this.checkpoint();
    for (const socket of this.sockets) socket.destroy();
    await new Promise<void>((resolve) => this.server!.close(() => resolve()));
  }
}

const owner = randomUUID();
const recipient = '0x3333333333333333333333333333333333333333' as Hex;
const evmKey = privateKeyToAccount(`0x${'12'.repeat(32)}`);
const sponsorKey = privateKeyToAccount(`0x${'34'.repeat(32)}`);
const evmKeyId = `wallet:${owner}:standard:secp256k1:0`;
const lxKeyId = `wallet:${owner}:standard:ed25519:0`;
const kernelVector = JSON.parse(
  readFileSync(
    new URL('../../attestor/internal/policy/lx/testdata/kernel_activities.json', import.meta.url),
    'utf8'
  )
);
const recordedSend = kernelVector.activities.find(
  (activity: Row) => activity.name === 'native-send'
);
const lxPublicKey: string = kernelVector.public_key;
// Strip the recorded owner's signature to obtain the canonical request to sign.
// The payload and activity hashes are recomputed; the production codec validates it below.
const recordedPayload = Buffer.from(recordedSend.payload, 'hex');
const signatureOffset = 197 + recordedPayload[196]! * 9 + 65;
const lxSignature = recordedPayload.subarray(signatureOffset, signatureOffset + 64).toString('hex');
const unsignedPayload = Buffer.from(recordedPayload);
unsignedPayload.fill(0, signatureOffset, signatureOffset + 64);
const recordedActivity = Buffer.from(recordedSend.unsigned, 'hex');
const payloadOffset = recordedActivity.lastIndexOf(recordedPayload);
const oldPayloadHash = createHash('sha256')
  .update('LXP/v1/payload-hash\0')
  .update(recordedPayload)
  .digest();
const hashOffset = recordedActivity.indexOf(oldPayloadHash);
const unsignedActivity = Buffer.from(recordedActivity);
unsignedPayload.copy(unsignedActivity, payloadOffset);
createHash('sha256')
  .update('LXP/v1/payload-hash\0')
  .update(unsignedPayload)
  .digest()
  .copy(unsignedActivity, hashOffset);
const lxActivity = unsignedActivity.toString('hex');
// The vector's signature_preimage hashes the activity envelope. The retained
// approval signs Send.AuthorizationMessage: tag, common fields, controller and scope.
const authorizationEnd = 197 + recordedPayload[196]! * 9;
const authorizationMessage = Buffer.concat([
  recordedPayload.subarray(0, 2),
  recordedPayload.subarray(4, authorizationEnd),
  recordedPayload.subarray(authorizationEnd, authorizationEnd + 33),
  recordedPayload.subarray(authorizationEnd + 129),
]);
const lxDigest = createHash('sha256')
  .update('LXP/v1/signature-preimage\0')
  .update(authorizationMessage)
  .digest('hex');

let work: string;
let db: DatabaseFixture;
let database: typeof import('../src/db/pool.js');
let pool: Pool;
let routes: typeof import('../src/routes/evmApprovals.js');
let signing: typeof import('../src/routes/sign.js');
let walletRoutes: typeof import('../src/routes/wallet.js');
let attestorModule: typeof import('../src/attestor/client.js');
let authority: typeof import('../src/agent/authority.js');
let env: (typeof import('../src/env.js'))['env'];
let rpcModule: typeof import('../src/rpc/pool.js');
let attestors: import('../src/attestor/client.js').AttestorClient;
let rpc: import('../src/rpc/pool.js').RpcPool;
let app: FastifyInstance;
let proxy: HttpServer;
let proxyUrl: string;
let idp: HttpServer;
let idpUrl: string;
let token: string;
let identityPublic: CryptoKey;
let mint: (subject: string) => Promise<string>;
let station: HttpServer;
let stationUrl: string;
let budgetExhausted = false;
let lastQuoteBody: Row | null = null;
let chain: TestChain;
let broadcastProxy: HttpServer;
let broadcastUrl: string;
let loseBroadcastReply = false;
let hideReceipts = false;
let loseReceiptReply = false;
let rejectBroadcast = false;
let broadcasts: Hex[] = [];
const peerServers: HttpsServer[] = [];
let peerEndpoints: string[] = [];
const peerRequests: Row[] = [];
let peerMode: 'success' | 'lost-reply' | 'refuse' | 'partial-refusal' = 'success';
let signatureCache: Record<string, Row> = {};
const fixtureErrors: unknown[] = [];
const forwarded: Row[] = [];
const catalogue = JSON.parse(
  readFileSync(new URL('../../../../tools/paxeer-x/route-catalogue.json', import.meta.url), 'utf8')
);

async function listen(server: HttpServer | HttpsServer): Promise<string> {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('fixture did not bind');
  return `${server instanceof Object && peerServers.includes(server as HttpsServer) ? 'https' : 'http'}://127.0.0.1:${address.port}`;
}
async function close(server: HttpServer | HttpsServer) {
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
}
async function body(req: import('node:http').IncomingMessage): Promise<Row> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(Buffer.from(chunk));
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}
function json(res: import('node:http').ServerResponse, status: number, value: unknown) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(value));
}

async function startPeers() {
  signatureCache = existsSync(join(work, 'peer-sessions.json'))
    ? JSON.parse(readFileSync(join(work, 'peer-sessions.json'), 'utf8'))
    : {};
  peerEndpoints = [];
  const { jwtVerify } = await import('jose');
  for (const [index, tls] of TLS_FIXTURE.entries()) {
    const id = `node-${index + 1}`;
    const server = httpsServer(
      {
        key: tls.key,
        cert: tls.cert,
        ca: TLS_FIXTURE.map((peer) => peer.cert).join('\n'),
        requestCert: true,
        rejectUnauthorized: true,
      },
      async (req, res) => {
        try {
          if (req.url === '/health') {
            json(res, 200, {
              node_id: id,
              region: id,
              share_count: 2,
              refresh_epoch: 1,
              audit_sequence: 10,
              audit_head: '0'.repeat(64),
              peers: {},
              reachable_peers: 4,
              ready: true,
            });
            return;
          }
          const wire = await body(req);
          if (req.url === '/v1/authority') {
            const payload = JSON.parse(
              Buffer.from(wire.token.split('.')[1], 'base64url').toString('utf8')
            );
            json(res, 200, { node_id: id, sequence: payload.sequence });
            return;
          }
          const jwt = await jwtVerify((req.headers.authorization ?? '').slice(7), identityPublic, {
            issuer: `${idpUrl}/auth/v1`,
            audience: 'authenticated',
          });
          if (jwt.payload.sub !== owner) {
            json(res, 403, {
              error: { category: 'token', code: 'token_not_owner', message: 'owner required' },
            });
            return;
          }
          if (req.url === '/v1/evm/review') {
            const decoded = parseTransaction(wire.transaction as Hex);
            const unsigned = serializeTransaction({
              ...decoded,
              r: undefined,
              s: undefined,
              v: undefined,
              yParity: undefined,
            });
            json(res, 200, {
              node_id: id,
              key_id: wire.key_id,
              to: decoded.to,
              value: (decoded.value ?? 0n).toString(),
              data: decoded.data ?? '0x',
              method: 'transfer',
              destination: 'evm',
              signing_hash: keccak256(unsigned),
              chain_id: 125,
              audit_sequence: 11,
            });
            return;
          }
          if (req.url !== '/v1/sign') {
            json(res, 404, { error: 'unknown_peer_path' });
            return;
          }
          peerRequests.push({ ...wire, node_id: id });
          if (peerMode === 'refuse' || (peerMode === 'partial-refusal' && index === 0)) {
            json(res, 403, {
              error: {
                category: 'policy',
                code: 'policy_denied',
                message: 'recorded definite pre-sign refusal',
                policy_code: 'VALUE_CAP',
              },
            });
            return;
          }
          const cacheKey = `${id}:${wire.session_id}`;
          let answer = signatureCache[cacheKey];
          if (answer) {
            if (answer.request !== JSON.stringify(wire))
              throw new Error('original session was retargeted');
          } else if (wire.kind === 'evm_tx') {
            const decoded = parseTransaction(`0x${wire.transaction}`);
            const unsigned = {
              ...decoded,
              r: undefined,
              s: undefined,
              v: undefined,
              yParity: undefined,
            };
            const signed = await evmKey.signTransaction(unsigned);
            const parsed = parseTransaction(signed);
            const yParity = parsed.yParity!;
            const signature =
              serializeSignature({ r: parsed.r!, s: parsed.s!, yParity }).slice(2, 130) +
              yParity.toString(16).padStart(2, '0');
            answer = {
              node_id: id,
              key_id: wire.key_id,
              kind: wire.kind,
              signed_bytes: keccak256(serializeTransaction(unsigned)).slice(2),
              signature,
              recovery_id: yParity,
              audit_sequence: 12,
              request: JSON.stringify(wire),
            };
          } else {
            if (
              wire.activity !== lxActivity ||
              attestorModule.lxSigningPreimage(wire.activity, wire.kind) !== lxDigest
            )
              throw new Error('LX signing fixture is not the recorded canonical payment');
            answer = {
              node_id: id,
              key_id: wire.key_id,
              kind: wire.kind,
              signed_bytes: lxDigest,
              signature: lxSignature,
              audit_sequence: 12,
              request: JSON.stringify(wire),
            };
          }
          signatureCache[cacheKey] = answer;
          writeFileSync(join(work, 'peer-sessions.json'), JSON.stringify(signatureCache));
          if (peerMode === 'lost-reply') {
            req.socket.destroy();
            return;
          }
          const { request: original, ...response } = answer;
          json(res, 200, response);
        } catch (error) {
          fixtureErrors.push(error);
          json(res, 500, { error: String(error) });
        }
      }
    );
    peerServers.push(server);
    peerEndpoints.push(await listen(server));
  }
}
async function stopPeers() {
  for (const server of peerServers.splice(0)) await close(server);
}
function newAttestors() {
  return new attestorModule.AttestorClient({
    endpoints: peerEndpoints,
    quorum: 3,
    tls: {
      cert: TLS_FIXTURE[0].cert,
      key: TLS_FIXTURE[0].key,
      ca: TLS_FIXTURE.map((peer) => peer.cert).join('\n'),
    },
    healthIntervalMs: 60_000,
    timeoutMs: 1_000,
  });
}
function limiter() {
  return new (requireRateLimiter())({ pool, clientPerMinute: 1000, accountPerMinute: 1000 });
}
let requireRateLimiter: () => typeof import('../src/audit.js').RateLimiter;
async function mount() {
  const instance = Fastify();
  await routes.evmApprovalRoutes(instance, pool, attestors, limiter(), rpc);
  await signing.signRoutes(instance, {
    pool,
    attestors,
    rpc,
    nonces: new (await import('../src/nonce/store.js')).NonceStore({
      pool,
      chainId: 125,
      pendingCount: async (address) =>
        Number(await rpc.request('eth_getTransactionCount', [address, 'latest'])),
      lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
    }),
    limiter: limiter(),
  });
  await walletRoutes.walletSponsorRoutes(instance, { stationUrl, rpc });
  await instance.listen({ host: '127.0.0.1', port: 0 });
  return instance;
}
function appUrl() {
  const address = app.server.address();
  if (!address || typeof address === 'string') throw new Error('gateway not bound');
  return `http://127.0.0.1:${address.port}`;
}

async function reopen() {
  await app.close();
  attestors.stop();
  await database.closePool();
  await db.stop();
  const old = db;
  expect(old.errors).toEqual([]);
  db = new DatabaseFixture(join(work, 'database.json'));
  await db.start();
  env.DATABASE_URL = db.url;
  pool = database.getPool();
  expect(db.state).toEqual(old.state);
  await stopPeers();
  await startPeers();
  env.ATTESTOR_ENDPOINTS = peerEndpoints;
  attestors = newAttestors();
  app = await mount();
}
async function request(path: string, payload?: Row, auth: string | null = token) {
  return fetch(`${proxyUrl}${path}`, {
    method: payload === undefined ? 'GET' : 'POST',
    headers: {
      ...(auth === null ? {} : { authorization: `Bearer ${auth}` }),
      'content-type': 'application/json',
    },
    ...(payload === undefined ? {} : { body: JSON.stringify(payload) }),
  });
}
function tx(nonce: number, value = 17n) {
  return {
    type: 'eip1559' as const,
    chainId: 125,
    nonce,
    to: recipient,
    value,
    gas: 21_000n,
    maxFeePerGas: 2_000_000_000n,
    maxPriorityFeePerGas: 1_000_000_000n,
  };
}
async function approvedEvm(nonce: number) {
  const review = await request('/v1/wallet/evm/review', {
    transaction: serializeTransaction(tx(nonce)),
  });
  expect(review.status, await review.clone().text()).toBe(200);
  const artifact = await review.json();
  const approved = await request('/v1/wallet/evm/approve', {
    review_id: artifact.id,
    signing_hash: artifact.signing_hash,
    decision: 'approve',
  });
  expect(approved.status, await approved.clone().text()).toBe(200);
  return approved.json();
}
function seedLx() {
  const id = randomUUID(),
    session = randomUUID();
  const artifact = {
    kind: 'lx_send_authorization',
    id,
    activity: lxActivity,
    signing_preimage: lxDigest,
    public_key: lxPublicKey,
    key_id: lxKeyId,
    principal: owner,
    did: `did:layerx:${lxPublicKey}`,
    account_id: recordedSend.from,
    epoch: 1,
    participants: ['node-1', 'node-2', 'node-3'],
    network_id: 125,
    protocol_version: 3,
    session_id: session,
    disclosure: {
      account: recordedSend.from,
      module: 'token',
      operation: 5,
      amounts: [{ asset: recordedSend.asset, amount: recordedSend.amount }],
      destinations: [recordedSend.to],
      sequence: '7',
      not_before: String(recordedSend.not_before),
      not_after: String(recordedSend.not_after),
    },
    expires_at: String(Math.floor(Date.now() / 1000) + 600),
  };
  const approval = {
    version: 1,
    principal: owner,
    key_id: lxKeyId,
    network_id: 125,
    protocol_version: 3,
    session_id: session,
    activity_digest: lxDigest,
    expires_at: artifact.expires_at,
  };
  db.state.approvals[id] = {
    id,
    wallet_id: db.state.wallets[0]!.id,
    principal: owner,
    session_id: session,
    domain: 'lx',
    state: 'approved',
    artifact,
    approval,
    evidence: null,
  };
  db.checkpoint();
  return artifact;
}

beforeAll(async () => {
  work = mkdtempSync(join(tmpdir(), 'wallet-repair-370-'));
  const identity = await generateKeyPair('ES256');
  identityPublic = identity.publicKey;
  const jwk = {
    ...(await exportJWK(identity.publicKey)),
    kid: 'repair-370',
    alg: 'ES256',
    use: 'sig',
  };
  idp = httpServer((req, res) => {
    if (req.url === '/auth/v1/.well-known/jwks.json') json(res, 200, { keys: [jwk] });
    else if (req.url === '/auth/v1/user')
      json(res, 200, {
        id: owner,
        aud: 'authenticated',
        role: 'authenticated',
        email: 'fixture@example.invalid',
        app_metadata: {},
        user_metadata: {},
        created_at: new Date().toISOString(),
      });
    else json(res, 404, { error: 'unknown_identity_path' });
  });
  idpUrl = await listen(idp);
  mint = (subject) =>
    new SignJWT({})
      .setProtectedHeader({ alg: 'ES256', kid: 'repair-370' })
      .setSubject(subject)
      .setIssuer(`${idpUrl}/auth/v1`)
      .setAudience('authenticated')
      .setIssuedAt()
      .setExpirationTime('1h')
      .sign(identity.privateKey);
  token = await mint(owner);
  chain = new TestChain(125);
  await chain.start();
  chain.balances.set(evmKey.address.toLowerCase(), 10n ** 25n);
  broadcastProxy = httpServer(async (req, res) => {
    try {
      const wire = await body(req);
      const handle = async (call: Row): Promise<Row | null> => {
        if (call.method === 'eth_sendRawTransaction') {
          const raw = call.params[0] as Hex;
          broadcasts.push(raw);
          const progress = Object.values(db.state.progress).find(
            (entry) => entry.evidence.raw === raw
          );
          expect(progress, 'signed bytes must commit before the network write').toBeDefined();
          expect(progress!.evidence.tx_hash).toBe(keccak256(raw));
          if (progress!.domain === 'human')
            expect(
              db.state.signatures.find((row) => row.id === progress!.approval_id)?.tx_hash
            ).toBe(keccak256(raw));
          if (rejectBroadcast)
            return {
              jsonrpc: '2.0',
              id: call.id,
              error: { code: -32000, message: 'intrinsic gas too low' },
            };
        }
        if (call.method === 'pax_sequencerReceipt' && loseReceiptReply) return null;
        if (call.method === 'pax_sequencerReceipt' && hideReceipts)
          return { jsonrpc: '2.0', id: call.id, result: null };
        if (call.method === 'eth_call') {
          const { encodeFunctionResult, parseAbi, toFunctionSelector } = await import('viem');
          const abi = parseAbi([
            'function getLayerXDid(address addr) view returns (bytes32 didPublicKey, string did)',
          ]);
          if (call.params[0].data?.startsWith(toFunctionSelector('getLayerXDid(address)')))
            return {
              jsonrpc: '2.0',
              id: call.id,
              result: encodeFunctionResult({
                abi,
                functionName: 'getLayerXDid',
                result: [`0x${lxPublicKey}`, `did:layerx:${lxPublicKey}`],
              }),
            };
        }
        const upstream = await fetch(chain.url, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(call),
        });
        const answer = await upstream.json();
        if (call.method === 'eth_sendRawTransaction' && loseBroadcastReply) return null;
        return answer;
      };
      const answer = Array.isArray(wire) ? await Promise.all(wire.map(handle)) : await handle(wire);
      if (answer === null) {
        req.socket.destroy();
        return;
      }
      json(res, 200, answer);
    } catch (error) {
      fixtureErrors.push(error);
      json(res, 500, { error: String(error) });
    }
  });
  broadcastUrl = await listen(broadcastProxy);
  db = new DatabaseFixture(join(work, 'database.json'));
  await db.start();
  process.env.SUPABASE_URL = idpUrl;
  process.env.DATABASE_URL = db.url;
  process.env.HYPERPAXEER_RPC_URL = broadcastUrl;
  process.env.RPC_URLS = broadcastUrl;
  process.env.SEQUENCER_RPC_URL = broadcastUrl;
  process.env.POLICY_MAX_TX_VALUE_WEI = '100';
  process.env.POLICY_MAX_DAILY_VALUE_WEI = '100';
  const producer = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const privateFile = join(work, 'producer.key'),
    publicFile = join(work, 'producer.pub'),
    inventoryFile = join(work, 'inventory.jwt');
  writeFileSync(privateFile, producer.privateKey.export({ format: 'pem', type: 'pkcs8' }), {
    mode: 0o600,
  });
  writeFileSync(publicFile, producer.publicKey.export({ format: 'pem', type: 'spki' }));
  writeFileSync(join(work, 'client.crt'), TLS_FIXTURE[0].cert);
  writeFileSync(join(work, 'client.key'), TLS_FIXTURE[0].key, { mode: 0o600 });
  writeFileSync(join(work, 'ca.crt'), TLS_FIXTURE.map((peer) => peer.cert).join('\n'));
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = privateFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'repair-370';
  process.env.WALLET_CUSTODY_INVENTORY_FILE = inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = publicFile;
  process.env.ATTESTOR_CLIENT_CERT_FILE = join(work, 'client.crt');
  process.env.ATTESTOR_CLIENT_KEY_FILE = join(work, 'client.key');
  process.env.ATTESTOR_CA_FILE = join(work, 'ca.crt');
  await startPeers();
  process.env.ATTESTOR_ENDPOINTS = peerEndpoints.join(',');
  const now = Math.floor(Date.now() / 1000);
  const inventory = {
    version: 1,
    iss: `${idpUrl}/auth/v1`,
    aud: 'wallet-custody-inventory',
    tenant: 'repair-370',
    sequence: '1',
    iat: now - 1,
    exp: now + 3600,
    protocol: 'wallet',
    threshold: 3,
    members: TLS_FIXTURE.map((tls, index) => ({
      id: `node-${index + 1}`,
      spki_sha256: createHash('sha256')
        .update(new X509Certificate(tls.cert).publicKey.export({ type: 'spki', format: 'der' }))
        .digest('hex'),
    })),
    keys: [
      {
        key_id: evmKeyId,
        owner,
        curve: 'secp256k1',
        epoch: 1,
        public_key: evmKey.publicKey.slice(2),
        operations: ['sign'],
      },
      {
        key_id: lxKeyId,
        owner,
        curve: 'ed25519',
        epoch: 1,
        public_key: lxPublicKey,
        operations: ['sign'],
      },
    ],
  };
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })
  ).toString('base64url');
  const payload = Buffer.from(JSON.stringify(inventory)).toString('base64url');
  const input = `${header}.${payload}`;
  writeFileSync(
    inventoryFile,
    `${input}.${cryptoSign('sha256', Buffer.from(input), { key: producer.privateKey, dsaEncoding: 'ieee-p1363' }).toString('base64url')}`
  );
  ({ env } = await import('../src/env.js'));
  database = await import('../src/db/pool.js');
  pool = database.getPool();
  authority = await import('../src/agent/authority.js');
  attestorModule = await import('../src/attestor/client.js');
  routes = await import('../src/routes/evmApprovals.js');
  signing = await import('../src/routes/sign.js');
  walletRoutes = await import('../src/routes/wallet.js');
  rpcModule = await import('../src/rpc/pool.js');
  const { RateLimiter } = await import('../src/audit.js');
  requireRateLimiter = () => RateLimiter;
  expect(attestorModule.lxSigningPreimage(lxActivity, 'lx_send_authorization')).toBe(lxDigest);
  const lxSpki = Buffer.concat([
    Buffer.from('302a300506032b6570032100', 'hex'),
    Buffer.from(lxPublicKey, 'hex'),
  ]);
  expect(
    cryptoVerify(
      null,
      Buffer.from(lxDigest, 'hex'),
      createPublicKey({ key: lxSpki, format: 'der', type: 'spki' }),
      Buffer.from(lxSignature, 'hex')
    )
  ).toBe(true);
  const { mainAccountId } = await import('../src/provision/bind.js');
  expect(mainAccountId(`did:layerx:${lxPublicKey}`)).toBe(recordedSend.from);
  db.state.wallets.push({
    id: randomUUID(),
    user_id: owner,
    address: evmKey.address,
    chain_id: 125,
    kind: 'standard',
    attestor_key_id: evmKeyId,
    layerx_key_id: lxKeyId,
    did: `did:layerx:${lxPublicKey}`,
    main_account_id: recordedSend.from,
    binding_state: 'bound',
    is_disabled: false,
    archived_at: null,
    migrated_at: 'threshold',
    encrypted_private_key: null,
  });
  db.checkpoint();
  rpc = new rpcModule.RpcPool({
    urls: [broadcastUrl],
    sequencerUrl: broadcastUrl,
    chainId: 125,
    lagThresholdBlocks: 20,
    timeoutMs: 1000,
    healthIntervalMs: 60000,
  });
  attestors = newAttestors();
  station = httpServer(async (req, res) => {
    try {
      lastQuoteBody = await body(req);
      if (budgetExhausted) {
        json(res, 429, { error: 'budget_exhausted' });
        return;
      }
      const { gasQuoteDigest } = await import('../../../agent/sdk/typescript/src/gas-station.js');
      const quote = {
        sponsor: sponsorKey.address,
        token: lastQuoteBody.token,
        decimals: 6,
        maxTokenAmount: BigInt(lastQuoteBody.maxTokenAmount),
        tokenAmount: 1n,
        deadline: BigInt(Math.floor(Date.now() / 1000) + 600),
        quoteNonce: 1n,
        gasCost: BigInt(lastQuoteBody.gasCost),
      };
      const digest = gasQuoteDigest(125n, lastQuoteBody.account, quote);
      if (!digest.ok) throw new Error(JSON.stringify(digest));
      const relayerSignature = await sponsorKey.sign({ hash: digest.value as Hex });
      json(
        res,
        200,
        JSON.parse(
          JSON.stringify({ quote, relayerSignature, feeDenom: 'SID' }, (_k, v) =>
            typeof v === 'bigint' ? v.toString() : v
          )
        )
      );
    } catch (error) {
      fixtureErrors.push(error);
      json(res, 500, { error: String(error) });
    }
  });
  stationUrl = await listen(station);
  app = await mount();
  // The proxy is a committed loopback transport peer. Its routing inventory is the
  // same JSON compiled by the hosted gateway; native Rust execution is recorded separately.
  proxy = httpServer(async (req, res) => {
    try {
      const entry = catalogue.routes.find(
        (row: Row) =>
          row.proxy === true &&
          row.service === 'wallet-gateway' &&
          row.method === req.method &&
          new RegExp('^' + row.path.replace(':id', '[0-9a-f-]{36}') + '$').test(req.url ?? '')
      );
      if (!entry) {
        json(res, 404, { error: 'route_not_found' });
        return;
      }
      expect(entry.upstream).toBe('wallet-gateway');
      expect(entry.authentication).toBe('requireAuth');
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const raw = Buffer.concat(chunks);
      forwarded.push({
        method: req.method,
        path: req.url,
        authorization: req.headers.authorization,
        body: raw.toString(),
      });
      const response = await fetch(`${appUrl()}${req.url}`, {
        method: req.method,
        headers: {
          'content-type': 'application/json',
          ...(req.headers.authorization ? { authorization: req.headers.authorization } : {}),
        },
        ...(req.method === 'GET' ? {} : { body: raw }),
      });
      res.writeHead(response.status, {
        'content-type': response.headers.get('content-type') ?? 'application/json',
        ...(response.headers.get('cache-control')
          ? { 'cache-control': response.headers.get('cache-control')! }
          : {}),
      });
      res.end(await response.text());
    } catch (error) {
      fixtureErrors.push(error);
      json(res, 500, { error: String(error) });
    }
  });
  proxyUrl = await listen(proxy);
}, 30000);

afterAll(async () => {
  await app?.close();
  attestors?.stop();
  rpc?.stop();
  rpcModule?.sharedRpcPool().stop();
  await database?.closePool();
  if (db) await db.stop();
  await stopPeers();
  if (proxy) await close(proxy);
  if (broadcastProxy) await close(broadcastProxy);
  if (station) await close(station);
  if (chain) await chain.stop();
  if (idp) await close(idp);
  if (work) rmSync(work, { recursive: true, force: true });
  expect(db?.errors ?? []).toEqual([]);
  expect(fixtureErrors).toEqual([]);
}, 30000);

async function evmApprovalRecoveryAcrossEveryBoundary() {
  // Commit the original session, then lose all replies after the peers retained it.
  let nonce = 0;
  for (const boundary of [
    'before-sign',
    'lost-sign-reply',
    'signature-journal',
    'broadcast-reply',
    'receipt-reply',
    'completion',
  ] as const) {
    peerMode = 'success';
    loseBroadcastReply = false;
    hideReceipts = true;
    const approval = await approvedEvm(nonce++);
    const requestsBefore = peerRequests.length;
    if (boundary === 'before-sign') {
      db.state.approvals[approval.id]!.state = 'signing_unknown';
      db.checkpoint();
    } else {
      if (boundary === 'lost-sign-reply') peerMode = 'lost-reply';
      if (boundary === 'signature-journal') db.failSignedJournal = true;
      if (boundary === 'completion') db.failOnce = "update wallet_evm_approvals set state='signed'";
      if (boundary === 'broadcast-reply') loseBroadcastReply = true;
      if (boundary === 'receipt-reply') loseReceiptReply = true;
      const signed = await request('/v1/wallet/evm/sign', { approval_id: approval.id });
      expect(signed.status, await signed.clone().text()).toBe(
        ['broadcast-reply', 'receipt-reply'].includes(boundary) ? 200 : 202
      );
    }
    const retained = db.state.progress[`evm:${approval.id}`];
    if (['broadcast-reply', 'receipt-reply', 'completion'].includes(boundary)) {
      expect(retained?.evidence.raw).toMatch(/^0x02/);
      expect(retained!.evidence.tx_hash).toBe(keccak256(retained!.evidence.raw));
    }
    peerMode = 'success';
    loseBroadcastReply = false;
    loseReceiptReply = false;
    await reopen();
    const recovered = await request(`/v1/wallet/evm/approvals/${approval.id}`);
    expect(recovered.status, await recovered.clone().text()).toBe(200);
    const result = await recovered.json();
    expect(result.state).toBe('signed');
    expect(result.signing_hash).toBe(approval.signing_hash);
    const progress = db.state.progress[`evm:${approval.id}`]!;
    const raw = progress.evidence.raw as Hex;
    expect(result.tx_hash).toBe(keccak256(raw));
    expect(await recoverTransactionAddress({ serializedTransaction: raw })).toBe(evmKey.address);
    const requests = peerRequests
      .slice(requestsBefore)
      .filter((row) => row.session_id === approval.session_id);
    expect(requests.length).toBeGreaterThanOrEqual(3);
    for (const wire of requests) {
      expect(wire.signers).toEqual(approval.participants);
      expect(wire.transaction).toBe(approval.transaction.slice(2));
    }
    const before = peerRequests.length;
    await request('/v1/wallet/evm/sign', { approval_id: approval.id });
    expect(peerRequests).toHaveLength(before);
    expect(progress.binding).toMatch(/^[0-9a-f]{64}$/);
  }
  // A completed signature, unavailable receipt and lost reply converge after startup.
  hideReceipts = false;
  await reopen();
  const rows = Object.values(db.state.approvals).filter(
    (row) => row.domain === 'evm' && row.state === 'signed'
  );
  for (const row of rows) {
    expect(row.evidence.tx_hash).toMatch(/^0x/);
    expect(row.evidence.rung).not.toBe('submitted');
  }
}

async function lxApprovalRecoveryAndDefiniteRefusal() {
  for (const boundary of [
    'before-sign',
    'lost-reply',
    'evidence-commit',
    'refusal',
    'partial-refusal',
  ] as const) {
    const approval = seedLx();
    peerMode = 'success';
    if (boundary === 'before-sign') {
      db.state.approvals[approval.id]!.state = 'signing_unknown';
      db.checkpoint();
    } else {
      if (boundary === 'lost-reply') peerMode = 'lost-reply';
      if (boundary === 'refusal') peerMode = 'refuse';
      if (boundary === 'partial-refusal') peerMode = 'partial-refusal';
      if (boundary === 'evidence-commit')
        db.failOnce = "update wallet_lx_approvals set state='signed'";
      const first = await app.inject({
        method: 'POST',
        url: '/v1/wallet/lx/sign',
        headers: { authorization: `Bearer ${token}` },
        payload: { approval_id: approval.id },
      });
      expect(first.statusCode, first.body).toBe(boundary === 'refusal' ? 403 : 202);
      if (boundary === 'refusal')
        expect(first.json()).toMatchObject({ state: 'refused', code: 'policy_denied' });
    }
    const before = peerRequests.length;
    peerMode = 'success';
    await reopen();
    const reply = await app.inject({
      method: 'GET',
      url: `/v1/wallet/lx/approvals/${approval.id}`,
      headers: { authorization: `Bearer ${token}` },
    });
    if (boundary === 'refusal') {
      expect(reply.statusCode).toBe(403);
      expect(reply.json()).toMatchObject({ state: 'refused', code: 'policy_denied' });
      expect(peerRequests).toHaveLength(before);
      continue;
    }
    expect(reply.statusCode, reply.body).toBe(200);
    expect(reply.json()).toMatchObject({
      state: 'signed',
      signature: lxSignature,
      signing_preimage: lxDigest,
    });
    expect(db.state.approvals[approval.id]!.evidence).toMatchObject({ signature: lxSignature });
    for (const wire of peerRequests.filter((row) => row.session_id === approval.session_id)) {
      expect(wire.activity).toBe(lxActivity);
      expect(wire.approval).toEqual(db.state.approvals[approval.id]!.approval);
      expect(wire.signers).toEqual(approval.participants);
    }
  }
}

async function evmDefiniteRefusalIsDurable() {
  const approval = await approvedEvm(6);
  peerMode = 'refuse';
  const first = await request('/v1/wallet/evm/sign', { approval_id: approval.id });
  expect(first.status).toBe(403);
  expect(await first.json()).toMatchObject({ state: 'refused', code: 'policy_denied' });
  peerMode = 'success';
  const before = peerRequests.length;
  await reopen();
  const status = await request(`/v1/wallet/evm/approvals/${approval.id}`);
  expect(status.status).toBe(403);
  expect(await status.json()).toMatchObject({ state: 'refused' });
  expect(peerRequests).toHaveLength(before);
}

async function evmConcurrentRetriesShareOneSession() {
  const approval = await approvedEvm(6);
  const before = peerRequests.length;
  const replies = await Promise.all([
    request('/v1/wallet/evm/sign', { approval_id: approval.id }),
    request('/v1/wallet/evm/sign', { approval_id: approval.id }),
  ]);
  const evidence = await Promise.all(
    replies.map(async (response) => {
      expect(response.status, await response.clone().text()).toBe(200);
      return response.json();
    })
  );
  expect(evidence[0].tx_hash).toBe(evidence[1].tx_hash);
  expect(peerRequests.slice(before)).toHaveLength(3);
  expect(chain.sent.filter((row) => row.hash === evidence[0].tx_hash)).toHaveLength(1);
}

async function humanUnknownSendKeepsDailySpendAfterReopen() {
  const user = randomUUID();
  const key = privateKeyToAccount(`0x${'56'.repeat(32)}`);
  const crypto = await import('../src/crypto.js');
  const encrypted = crypto.encrypt(
    Buffer.from('56'.repeat(32), 'hex'),
    crypto.loadMasterKey(env.WALLET_MASTER_KEY)
  );
  const wallet = {
    id: randomUUID(),
    user_id: user,
    address: key.address,
    chain_id: 125,
    kind: 'standard',
    is_disabled: false,
    archived_at: null,
    migrated_at: null,
    attestor_key_id: null,
    encrypted_private_key: encrypted.ciphertext,
    key_version: 1,
  };
  db.state.wallets.push(wallet);
  db.checkpoint();
  chain.balances.set(key.address.toLowerCase(), 10n ** 25n);
  const access = await mint(user);
  loseBroadcastReply = true;
  hideReceipts = true;
  const before = broadcasts.length;
  const first = await fetch(`${appUrl()}/v1/wallet/send`, {
    method: 'POST',
    headers: { authorization: `Bearer ${access}`, 'content-type': 'application/json' },
    body: JSON.stringify({
      tx: {
        to: recipient,
        value: '100',
        gas: '21000',
        maxFeePerGas: '2000000000',
        maxPriorityFeePerGas: '1000000000',
      },
    }),
  });
  expect(first.status, await first.clone().text()).toBe(202);
  const answer = await first.json();
  expect(answer).toMatchObject({ error: 'outcome_unknown', must_not_resubmit: true });
  expect(broadcasts).toHaveLength(before + 1);
  const raw = broadcasts[before]!;
  expect(parseTransaction(raw).value).toBe(100n);
  expect(await recoverTransactionAddress({ serializedTransaction: raw })).toBe(key.address);
  const charged = db.state.signatures.filter((row) => row.wallet_id === wallet.id);
  expect(charged).toHaveLength(1);
  expect(charged[0]).toMatchObject({ value_wei: '100', tx_hash: keccak256(raw) });
  expect(chain.sent.some((row) => row.hash === keccak256(raw))).toBe(true);
  loseBroadcastReply = false;
  hideReceipts = false;
  await reopen();
  const progress = db.state.progress[`human:${charged[0]!.id}`];
  expect(progress).toMatchObject({ phase: 'settled', evidence: { raw, tx_hash: keccak256(raw) } });
  for (const route of ['sign', 'send']) {
    const next = await fetch(`${appUrl()}/v1/wallet/${route}`, {
      method: 'POST',
      headers: { authorization: `Bearer ${access}`, 'content-type': 'application/json' },
      body: JSON.stringify({ tx: { to: recipient, value: '1' } }),
    });
    expect(next.status, await next.clone().text()).toBe(403);
    expect(await next.json()).toMatchObject({ error: 'DAILY_VALUE_CAP' });
  }
  expect(db.state.signatures.filter((row) => row.wallet_id === wallet.id)).toHaveLength(1);
}

async function hostedEvmSdkRoundTrip() {
  const { PaxeerWallet } = loadProductionWalletSdk();
  const memory = new Map<string, string>();
  const sdk = new PaxeerWallet({
    apiUrl: proxyUrl,
    supabaseUrl: idpUrl,
    supabaseAnonKey: 'loopback-public-key',
    evmReview: {
      storage: {
        getItem: (key) => memory.get(key) ?? null,
        setItem: (key, value) => {
          memory.set(key, value);
        },
      },
    },
  });
  try {
    const session = await sdk.supabase.auth.setSession({
      access_token: token,
      refresh_token: 'loopback-refresh',
    });
    expect(session.error).toBeNull();
    const review = await sdk.reviewEvmTransaction(tx(7));
    const approval = await sdk.approveEvmTransaction(review.id);
    expect(approval.state).toBe('approved');
    const sign = await request('/v1/wallet/evm/sign', { approval_id: review.id });
    expect(sign.status, await sign.clone().text()).toBe(200);
    const status = await sdk.evmApprovalStatus(review.id);
    expect(status).toMatchObject({
      state: 'signed',
      id: review.id,
      signing_hash: review.signing_hash,
    });
    const paths = forwarded.filter((row) => row.path.startsWith('/v1/wallet/evm/'));
    expect(paths.some((row) => row.method === 'POST' && row.path.endsWith('/review'))).toBe(true);
    expect(paths.some((row) => row.method === 'POST' && row.path.endsWith('/approve'))).toBe(true);
    expect(paths.some((row) => row.method === 'POST' && row.path.endsWith('/sign'))).toBe(true);
    expect(paths.some((row) => row.method === 'GET' && row.path.includes('/approvals/'))).toBe(
      true
    );
    for (const row of paths) expect(row.authorization).toBe(`Bearer ${token}`);
    for (const path of ['/v1/wallet/evm/review', '/v1/wallet/evm/approve', '/v1/wallet/evm/sign']) {
      const refused = await request(path, {}, null);
      expect(refused.status).toBe(401);
      const wrong = await fetch(`${proxyUrl}${path}`);
      expect(wrong.status).toBe(404);
    }
    expect((await request(`/v1/wallet/evm/approvals/${review.id}`, undefined, null)).status).toBe(
      401
    );
    expect((await request(`/v1/wallet/evm/approvals/${review.id}?extra=1`)).status).toBe(404);
  } finally {
    sdk.supabase.auth.stopAutoRefresh();
  }
}

async function hostedSponsoredQuoteAndBudgetRefusal() {
  const { requestGasQuote, SIDIORA_TOKEN, gasQuoteDigest } = await import(
    '../../../agent/sdk/typescript/src/gas-station.js'
  );
  const config = {
    quoteUrl: `${proxyUrl}/v1/wallet/sponsored/quote`,
    chainId: 125n,
    sponsor: sponsorKey.address,
    token: SIDIORA_TOKEN,
    decimals: 6,
    paymaster: recipient,
  };
  const quoteRequest = {
    account: evmKey.address,
    did: `did:layerx:${lxPublicKey}`,
    nonce: 0n,
    calls: [{ to: recipient, value: 0n, data: '0x' }],
    maxTokenAmount: 10n,
    gasCost: 5n,
  };
  const authorizedFetch: typeof fetch = (input, init) =>
    fetch(input, { ...init, headers: { ...init?.headers, authorization: `Bearer ${token}` } });
  budgetExhausted = false;
  const quoted = await requestGasQuote(config, quoteRequest, { fetch: authorizedFetch });
  expect(
    quoted.ok,
    JSON.stringify(quoted, (_k, v) => (typeof v === 'bigint' ? String(v) : v))
  ).toBe(true);
  if (!quoted.ok) throw new Error('quote refused');
  expect(quoted.value.quote.gasCost).toBe(5n);
  expect(lastQuoteBody).toMatchObject({
    account: evmKey.address,
    did: quoteRequest.did,
    gasCost: '5',
  });
  const digest = gasQuoteDigest(125n, evmKey.address, quoted.value.quote);
  expect(digest.ok).toBe(true);
  budgetExhausted = true;
  const exhausted = await requestGasQuote(config, quoteRequest, { fetch: authorizedFetch });
  expect(exhausted).toMatchObject({ ok: false, refusal: { code: 'budget_exhausted' } });
  const raw = await request('/v1/wallet/sponsored/quote', {
    ...quoteRequest,
    chainId: '125',
    nonce: '0',
    calls: [{ to: recipient, value: '0', data: '0x' }],
    maxTokenAmount: '10',
    gasCost: '5',
    token: SIDIORA_TOKEN,
    decimals: 6,
  });
  expect(raw.status).toBe(429);
  expect(raw.headers.get('cache-control')).toBe('no-store');
  expect(await raw.json()).toEqual({ error: 'budget_exhausted', detail: 'budget_exhausted' });
  expect((await request('/v1/wallet/sponsored/quote', {}, null)).status).toBe(401);
  budgetExhausted = false;
  const native = readFileSync(
    new URL('../../../platform/hosted/gateway/src/lib.rs', import.meta.url),
    'utf8'
  );
  expect(native).toContain('"/v1/wallet/sponsored/quote"');
  expect(native).toContain('"/v1/wallet/evm/review"');
  expect(native).toContain('path.strip_prefix("/v1/wallet/evm/approvals/")');
}

test(evmApprovalRecoveryAcrossEveryBoundary.name, evmApprovalRecoveryAcrossEveryBoundary, 30000);
test(lxApprovalRecoveryAndDefiniteRefusal.name, lxApprovalRecoveryAndDefiniteRefusal, 30000);
test(evmDefiniteRefusalIsDurable.name, evmDefiniteRefusalIsDurable, 30000);
test(evmConcurrentRetriesShareOneSession.name, evmConcurrentRetriesShareOneSession, 30000);
test(
  humanUnknownSendKeepsDailySpendAfterReopen.name,
  humanUnknownSendKeepsDailySpendAfterReopen,
  30000
);
test(hostedEvmSdkRoundTrip.name, hostedEvmSdkRoundTrip, 30000);
test(hostedSponsoredQuoteAndBudgetRefusal.name, hostedSponsoredQuoteAndBudgetRefusal, 30000);
