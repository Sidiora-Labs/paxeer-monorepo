import { createHash, X509Certificate } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import type { TLSSocket } from 'node:tls';
import type { Pool } from 'pg';
import type { Hex } from 'viem';
import { loadWalletInventory, requireInventoryKey } from '../agent/authority.js';
import {
  AttestorError,
  AttestorQuorumError,
  AttestorSessionError,
  attestorErrorBody,
  attestorErrorStatus,
  parseSignResponse,
  refusalFromWire,
  signRequestBody,
  type AttestorClient,
  type SignPayload,
  type SignResult,
} from '../attestor/client.js';
import { env } from '../env.js';

// Kept with the routes so an upgrade cannot admit signing before its recovery store exists.
export const APPROVAL_RECOVERY_SQL = `create table if not exists wallet_approval_recovery (
  domain text not null check (domain in ('evm','lx','human')),
  approval_id text not null,
  binding text not null,
  phase text not null check (phase in ('attempting','signed','refused','settled','rejected')),
  evidence jsonb not null,
  updated_at timestamptz not null default now(),
  primary key (domain, approval_id)
)`;

export class ApprovalRefused extends Error {
  constructor(
    readonly status: number,
    readonly body: Record<string, unknown>
  ) {
    super(String(body.code ?? body.error));
  }
}

export interface RecoveryEntry {
  domain: 'evm' | 'lx' | 'human';
  approval_id: string;
  binding: string;
  phase: 'attempting' | 'signed' | 'refused' | 'settled' | 'rejected';
  evidence: {
    result?: SignResult;
    raw?: Hex;
    tx_hash?: Hex;
    status?: number;
    body?: Record<string, unknown>;
    signature_id?: string;
  };
}

export class ApprovalRecovery {
  constructor(readonly pool: Pool) {}
  async initialize(): Promise<void> {
    await this.pool.query(APPROVAL_RECOVERY_SQL);
  }
  async read(domain: RecoveryEntry['domain'], id: string): Promise<RecoveryEntry | null> {
    const found = await this.pool.query<RecoveryEntry>(
      'select * from wallet_approval_recovery where domain=$1 and approval_id=$2',
      [domain, id]
    );
    return found.rows[0] ?? null;
  }
  async save(entry: RecoveryEntry): Promise<void> {
    const saved = await this.pool.query(
      `insert into wallet_approval_recovery(domain,approval_id,binding,phase,evidence)
       values($1,$2,$3,$4,$5::jsonb) on conflict(domain,approval_id) do update
       set phase=excluded.phase,evidence=excluded.evidence,updated_at=now()
       where wallet_approval_recovery.binding=excluded.binding returning approval_id`,
      [entry.domain, entry.approval_id, entry.binding, entry.phase, JSON.stringify(entry.evidence)]
    );
    if (saved.rowCount !== 1) throw new Error('approval recovery binding changed');
  }
  async locked<T>(domain: RecoveryEntry['domain'], id: string, work: () => Promise<T>): Promise<T> {
    const client = await this.pool.connect();
    const lock = `wallet-approval:${domain}:${id}`;
    let held = false;
    try {
      await client.query('select pg_advisory_lock(hashtextextended($1,0))', [lock]);
      held = true;
      return await work();
    } finally {
      try {
        if (held) await client.query('select pg_advisory_unlock(hashtextextended($1,0))', [lock]);
      } catch (error) {
        client.release(true);
        throw error;
      }
      client.release();
    }
  }
}

export function approvalBinding(artifact: unknown, approval: unknown): string {
  const stable = (value: unknown): string => {
    if (Array.isArray(value)) return `[${value.map(stable).join(',')}]`;
    if (value !== null && typeof value === 'object')
      return `{${Object.entries(value)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`)
        .join(',')}}`;
    const result = JSON.stringify(value);
    if (result === undefined) throw new Error('invalid approval binding');
    return result;
  };
  return createHash('sha256').update(stable({ artifact, approval })).digest('hex');
}

/** Replays the exact reviewed session and quorum; it never allocates a new session. */
export async function signApprovalSession(
  client: AttestorClient,
  input: {
    sessionId: string;
    keyId: string;
    participants: string[];
    payload: SignPayload;
    token: string;
  }
): Promise<SignResult> {
  const key = requireInventoryKey(input.keyId, 'sign');
  if (input.participants.length !== 3 || new Set(input.participants).size !== 3) {
    throw new AttestorSessionError(
      'participant_mismatch',
      'the reviewed quorum must have three distinct members'
    );
  }
  const inventory = loadWalletInventory();
  const health = await client.refreshHealth();
  const members = input.participants.map((id) => {
    const node = health.find((candidate) => candidate.nodeId === id && candidate.healthy);
    if (!node)
      throw new AttestorQuorumError(
        'participant_unavailable',
        'an original approval participant is unavailable'
      );
    return { id, endpoint: node.endpoint };
  });
  if (!env.ATTESTOR_CLIENT_CERT_FILE || !env.ATTESTOR_CLIENT_KEY_FILE || !env.ATTESTOR_CA_FILE) {
    throw new AttestorQuorumError(
      'attestor_unconfigured',
      'approval recovery requires authenticated TLS'
    );
  }
  const agent = new Agent({
    cert: readFileSync(env.ATTESTOR_CLIENT_CERT_FILE),
    key: readFileSync(env.ATTESTOR_CLIENT_KEY_FILE),
    ca: readFileSync(env.ATTESTOR_CA_FILE),
    rejectUnauthorized: true,
    minVersion: 'TLSv1.3',
  });
  const body = JSON.stringify(
    signRequestBody(input.sessionId, input.keyId, input.participants, input.payload)
  );
  try {
    const replies = await Promise.allSettled(
      members.map(
        (member) =>
          new Promise<ReturnType<typeof parseSignResponse>>((resolve, reject) => {
            const req = request(
              new URL('/v1/sign', member.endpoint),
              {
                method: 'POST',
                agent,
                timeout: env.ATTESTOR_TIMEOUT_MS,
                headers: {
                  authorization: `Bearer ${input.token}`,
                  'content-type': 'application/json',
                  'content-length': Buffer.byteLength(body),
                },
              },
              (res) => {
                try {
                  const cert = (res.socket as TLSSocket).getPeerCertificate();
                  const pin = createHash('sha256')
                    .update(
                      new X509Certificate(cert.raw).publicKey.export({
                        type: 'spki',
                        format: 'der',
                      })
                    )
                    .digest('hex');
                  if (inventory.members.find((peer) => peer.id === member.id)?.spki_sha256 !== pin)
                    throw new Error('approval participant TLS identity changed');
                } catch (error) {
                  res.destroy();
                  reject(error);
                  return;
                }
                const chunks: Buffer[] = [];
                let length = 0;
                res.on('data', (chunk: Buffer) => {
                  length += chunk.length;
                  if (length > 65_536)
                    res.destroy(new Error('approval signature reply exceeds bound'));
                  else chunks.push(chunk);
                });
                res.on('error', reject);
                res.on('end', () => {
                  try {
                    const value: unknown = JSON.parse(Buffer.concat(chunks).toString('utf8'));
                    if (res.statusCode !== 200) throw refusalFromWire(value, member.id);
                    const answer = parseSignResponse(value, member.id, key.curve);
                    if (answer.key_id !== input.keyId || answer.kind !== input.payload.kind)
                      throw new AttestorSessionError(
                        'request_mismatch',
                        'approval signature reply names another request'
                      );
                    resolve(answer);
                  } catch (error) {
                    reject(error);
                  }
                });
              }
            );
            req.on('error', reject);
            req.on('timeout', () => req.destroy(new Error('approval session timed out')));
            req.end(body);
          })
      )
    );
    const refused = replies.filter((r): r is PromiseRejectedResult => r.status === 'rejected');
    // All original participants must definitely refuse before this is a pre-sign refusal.
    if (
      refused.length === 3 &&
      refused.every(
        (r) =>
          r.reason instanceof AttestorError &&
          ['token', 'policy', 'key'].includes(r.reason.category)
      )
    ) {
      const refusal = refused[0]!.reason as AttestorError;
      throw new ApprovalRefused(attestorErrorStatus(refusal), attestorErrorBody(refusal));
    }
    if (refused.length > 0)
      throw new AttestorQuorumError(
        'approval_outcome_unknown',
        'the original signing session must be reconciled'
      );
    const answers = replies.map(
      (r) => (r as PromiseFulfilledResult<ReturnType<typeof parseSignResponse>>).value
    );
    const first = answers[0]!;
    if (
      answers.some(
        (a) =>
          a.signature !== first.signature ||
          a.signed_bytes !== first.signed_bytes ||
          a.recovery_id !== first.recovery_id
      )
    ) {
      throw new AttestorSessionError(
        'signature_disagreement',
        'the original quorum returned different evidence'
      );
    }
    return {
      sessionId: input.sessionId,
      kind: input.payload.kind,
      signature: `0x${first.signature}`,
      recoveryId: first.recovery_id ?? null,
      signedBytes: `0x${first.signed_bytes}`,
      participants: input.participants,
      audit: answers.map((a) => ({ node_id: a.node_id, audit_sequence: a.audit_sequence })),
    };
  } finally {
    agent.destroy();
  }
}

export async function retainedSignature(
  store: ApprovalRecovery,
  domain: 'evm' | 'lx',
  id: string,
  binding: string,
  sign: () => Promise<SignResult>,
  verify: (result: SignResult) => Promise<Hex | undefined>
): Promise<RecoveryEntry> {
  const prior = await store.read(domain, id);
  if (prior && prior.binding !== binding) throw new Error('approval recovery binding changed');
  if (prior?.phase === 'refused')
    throw new ApprovalRefused(prior.evidence.status!, prior.evidence.body!);
  if (prior?.phase === 'signed' || prior?.phase === 'settled' || prior?.phase === 'rejected')
    return prior;
  await store.save({ domain, approval_id: id, binding, phase: 'attempting', evidence: {} });
  try {
    const result = await sign();
    const raw = await verify(result);
    const entry: RecoveryEntry = {
      domain,
      approval_id: id,
      binding,
      phase: 'signed',
      evidence: { result, ...(raw ? { raw } : {}) },
    };
    await store.save(entry);
    return entry;
  } catch (error) {
    if (error instanceof ApprovalRefused && prior === null) {
      await store.save({
        domain,
        approval_id: id,
        binding,
        phase: 'refused',
        evidence: { status: error.status, body: error.body },
      });
      throw error;
    }
    // A refusal following a lost reply cannot prove that the earlier attempt never signed.
    if (error instanceof ApprovalRefused)
      throw new AttestorQuorumError(
        'approval_outcome_unknown',
        'an earlier signing attempt remains unresolved'
      );
    throw error;
  }
}
