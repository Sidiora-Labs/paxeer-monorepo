import { randomUUID } from 'node:crypto';
import type { FastifyInstance, FastifyReply } from 'fastify';
import type { Pool, PoolClient } from 'pg';
import { keccak256, parseTransaction, recoverTransactionAddress, serializeTransaction, type Hex } from 'viem';
import { publicKeyToAddress } from 'viem/accounts';
import { z } from 'zod';
import { CustodyAuthorityError, requireInventoryKey } from '../agent/authority.js';
import {
  AttestorError,
  attestorErrorBody,
  attestorErrorStatus,
  splitSignature,
  type AttestorClient,
  type EvmReviewResult,
} from '../attestor/client.js';
import { RateLimitedError, rateLimitBody, type RateLimiter } from '../audit.js';
import { env } from '../env.js';
import { requireAuth } from '../middleware/auth.js';
import { sharedRpcPool, type SequencerReceiptResult } from '../rpc/pool.js';

interface EvmWallet {
  id: string;
  address: string;
  chain_id: number;
  attestor_key_id: string;
  is_disabled: boolean;
  archived_at: Date | null;
}

interface EvmArtifact extends Omit<EvmReviewResult, 'audit'> {
  id: string;
  transaction: Hex;
  principal: string;
  key_id: string;
  public_key: string;
  epoch: number;
  session_id: string;
  expires_at: string;
}

interface EvmApproval {
  version: 1;
  principal: string;
  key_id: string;
  session_id: string;
  signing_hash: string;
  expires_at: string;
}

interface EvmApprovalRow {
  id: string;
  wallet_id: string;
  principal: string;
  session_id: string;
  state: 'reviewed' | 'approved' | 'signing_unknown' | 'signed';
  artifact: EvmArtifact;
  approval: EvmApproval | null;
  evidence: {
    tx_hash: Hex;
    rung: SequencerReceiptResult['rung'];
    receipt: SequencerReceiptResult | null;
    attestor_audit: { node_id: string; audit_sequence: number }[];
  } | null;
}

class EvmApprovalRefusal extends Error {
  constructor(readonly status: number, readonly code: string) {
    super(code);
  }
}

export async function evmApprovalRoutes(
  app: FastifyInstance,
  pool: Pool,
  attestors: AttestorClient | null,
  limiter: RateLimiter,
): Promise<void> {
  const idSchema = z.string().uuid();
  const reviewSchema = z.object({
    transaction: z.string().max(2_097_154).regex(/^0x02(?:[0-9a-fA-F]{2})+$/),
  }).strict();
  const approveSchema = z.object({
    review_id: idSchema,
    signing_hash: z.string().regex(/^0x[0-9a-fA-F]{64}$/),
    decision: z.literal('approve'),
  }).strict();
  const signSchema = z.object({ approval_id: idSchema }).strict();
  const response = (row: EvmApprovalRow) => ({
    ...row.artifact,
    state: row.state,
    ...(row.approval ? { approval: row.approval } : {}),
    ...(row.evidence ?? {}),
  });
  const refuse = (status: number, code: string): never => {
    throw new EvmApprovalRefusal(status, code);
  };

  async function original(client: PoolClient, subject: string): Promise<EvmWallet> {
    const found = await client.query<EvmWallet>(
      `select id,address,chain_id,attestor_key_id,is_disabled,archived_at
       from wallets where user_id=$1 and kind='standard' for update`,
      [subject],
    );
    const wallet = found.rows[0];
    if (!wallet || wallet.is_disabled || wallet.archived_at !== null || !wallet.attestor_key_id) {
      return refuse(403, 'original_evm_identity_unavailable');
    }
    const key = requireInventoryKey(wallet.attestor_key_id, 'sign', subject);
    if (key.curve !== 'secp256k1'
      || publicKeyToAddress(`0x${key.public_key}`).toLowerCase() !== wallet.address.toLowerCase()) {
      return refuse(403, 'original_evm_identity_mismatch');
    }
    return wallet;
  }

  async function retained(client: PoolClient, subject: string, id: string): Promise<EvmApprovalRow> {
    const found = await client.query<EvmApprovalRow>(
      'select * from wallet_evm_approvals where id=$1 and principal=$2 for update', [id, subject],
    );
    const row = found.rows[0];
    if (!row) return refuse(404, 'evm_approval_not_found');
    const wallet = await original(client, subject);
    if (row.wallet_id !== wallet.id) return refuse(404, 'evm_approval_not_found');
    const a = row.artifact;
    const key = requireInventoryKey(wallet.attestor_key_id, 'sign', subject);
    if (a.id !== row.id || a.principal !== subject || a.key_id !== wallet.attestor_key_id
      || a.public_key !== key.public_key || a.epoch !== key.epoch
      || a.chain_id !== wallet.chain_id || a.session_id !== row.session_id) {
      return refuse(409, 'evm_approval_identity_changed');
    }
    return row;
  }

  async function transaction<T>(work: (client: PoolClient) => Promise<T>): Promise<T> {
    const client = await pool.connect();
    try {
      await client.query('begin');
      const result = await work(client);
      await client.query('commit');
      return result;
    } catch (error) {
      await client.query('rollback').catch(() => undefined);
      throw error;
    } finally {
      client.release();
    }
  }

  async function guarded(reply: FastifyReply, work: () => Promise<unknown>): Promise<FastifyReply> {
    try {
      return reply.send(await work());
    } catch (error) {
      if (error instanceof EvmApprovalRefusal) return reply.code(error.status).send({ error: error.code });
      if (error instanceof AttestorError) return reply.code(attestorErrorStatus(error)).send(attestorErrorBody(error));
      if (error instanceof CustodyAuthorityError) return reply.code(503).send({ error: error.code });
      if (error instanceof RateLimitedError) return reply.code(429).send(rateLimitBody(error));
      app.log.error({ err: error }, 'EVM approval request failed');
      return reply.code(503).send({ error: 'evm_producer_unavailable' });
    }
  }

  app.post('/v1/wallet/evm/review', { preHandler: requireAuth, bodyLimit: 2_400_000 }, async (req, reply) => {
    const parsed = reviewSchema.safeParse(req.body);
    if (!parsed.success) return reply.code(400).send({ error: 'invalid_evm_review' });
    return guarded(reply, async () => {
      await limiter.consume('client', req.user!.id);
      if (!attestors) return refuse(503, 'attestor_custody_required');
      let unsigned: Hex;
      let canonical: Hex;
      let decoded: ReturnType<typeof parseTransaction>;
      try {
        decoded = parseTransaction(parsed.data.transaction as Hex);
        if (decoded.type !== 'eip1559' || !decoded.to
          || (decoded.r !== undefined && BigInt(decoded.r) !== 0n)
          || (decoded.s !== undefined && BigInt(decoded.s) !== 0n)
          || (decoded.yParity !== undefined && decoded.yParity !== 0)) {
          return refuse(400, 'evm_unsigned_canonical_required');
        }
        const tx = { ...decoded, r: undefined, s: undefined, v: undefined, yParity: undefined };
        unsigned = serializeTransaction(tx);
        canonical = serializeTransaction(tx, { r: '0x0', s: '0x0', yParity: 0 });
        if (![unsigned, canonical].includes(parsed.data.transaction.toLowerCase() as Hex)) {
          return refuse(400, 'evm_unsigned_canonical_required');
        }
      } catch {
        return refuse(400, 'evm_unsigned_canonical_required');
      }
      return transaction(async (client) => {
        const wallet = await original(client, req.user!.id);
        await limiter.consume('account', wallet.address);
        await client.query(
          "delete from wallet_evm_approvals where principal=$1 and state in ('reviewed','approved') and expires_at<=now()",
          [req.user!.id],
        );
        const count = await client.query<{ count: string }>(
          "select count(*)::text as count from wallet_evm_approvals where principal=$1 and state in ('reviewed','approved','signing_unknown')",
          [req.user!.id],
        );
        if (!count.rows[0] || BigInt(count.rows[0].count) >= 256n) return refuse(429, 'evm_approval_capacity');
        const id = randomUUID();
        const session = randomUUID();
        const review = await attestors.reviewEvmTransaction({
          keyId: wallet.attestor_key_id, sessionId: session, transaction: canonical,
          owner: req.user!.id, token: req.headers.authorization!.slice('Bearer '.length).trim(),
        });
        if (review.signing_hash !== keccak256(unsigned) || review.chain_id !== wallet.chain_id
          || review.chain_id !== env.HYPERPAXEER_CHAIN_ID
          || review.to.toLowerCase() !== decoded.to!.toLowerCase()
          || review.value !== (decoded.value ?? 0n).toString()
          || review.data.toLowerCase() !== (decoded.data ?? '0x').toLowerCase()) {
          return refuse(403, 'evm_review_binding_mismatch');
        }
        const key = requireInventoryKey(wallet.attestor_key_id, 'sign', req.user!.id);
        const { audit, ...disclosure } = review;
        const artifact: EvmArtifact = {
          ...disclosure, id, transaction: canonical, principal: req.user!.id,
          key_id: wallet.attestor_key_id, public_key: key.public_key, epoch: key.epoch,
          session_id: session, expires_at: String(Math.floor(Date.now() / 1000) + 600),
        };
        await client.query(
          `insert into wallet_evm_approvals(id,wallet_id,principal,session_id,state,artifact,expires_at,review_evidence)
           values($1,$2,$3,$4,'reviewed',$5::jsonb,to_timestamp($6::double precision),$7::jsonb)`,
          [id, wallet.id, req.user!.id, session, JSON.stringify(artifact), artifact.expires_at, JSON.stringify(audit)],
        );
        return { ...artifact, state: 'reviewed' };
      });
    });
  });

  app.post('/v1/wallet/evm/approve', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = approveSchema.safeParse(req.body);
    if (!parsed.success) return reply.code(400).send({ error: 'explicit_evm_approval_required' });
    return guarded(reply, async () => {
      await limiter.consume('client', req.user!.id);
      return transaction(async (client) => {
        const row = await retained(client, req.user!.id, parsed.data.review_id);
        const a = row.artifact;
        if (parsed.data.signing_hash !== a.signing_hash) return refuse(409, 'evm_review_changed');
        if (row.state !== 'reviewed') return response(row);
        if (BigInt(a.expires_at) <= BigInt(Math.floor(Date.now() / 1000))) return refuse(403, 'evm_approval_expired');
        const approval: EvmApproval = {
          version: 1, principal: a.principal, key_id: a.key_id, session_id: a.session_id,
          signing_hash: a.signing_hash, expires_at: a.expires_at,
        };
        await client.query(
          "update wallet_evm_approvals set state='approved',approval=$2::jsonb,approved_at=now() where id=$1",
          [row.id, JSON.stringify(approval)],
        );
        return response({ ...row, state: 'approved', approval });
      });
    });
  });

  app.post('/v1/wallet/evm/sign', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = signSchema.safeParse(req.body);
    if (!parsed.success) return reply.code(400).send({ error: 'explicit_evm_approval_required' });
    return guarded(reply, async () => {
      await limiter.consume('client', req.user!.id);
      if (!attestors) return refuse(503, 'attestor_custody_required');
      const staged = await transaction(async (client) => {
        const row = await retained(client, req.user!.id, parsed.data.approval_id);
        if (row.state === 'signed' || row.state === 'signing_unknown') return { row, attempt: false };
        if (row.state !== 'approved' || !row.approval) return refuse(403, 'explicit_evm_approval_required');
        const a = row.artifact;
        const approval = row.approval;
        if (BigInt(a.expires_at) <= BigInt(Math.floor(Date.now() / 1000))) return refuse(403, 'evm_approval_expired');
        if (approval.version !== 1 || approval.principal !== a.principal || approval.key_id !== a.key_id
          || approval.session_id !== a.session_id || approval.signing_hash !== a.signing_hash
          || approval.expires_at !== a.expires_at) return refuse(409, 'evm_review_changed');
        await limiter.consume('account', publicKeyToAddress(`0x${a.public_key}`));
        await client.query("update wallet_evm_approvals set state='signing_unknown',attempted_at=now() where id=$1", [row.id]);
        return { row: { ...row, state: 'signing_unknown' as const }, attempt: true };
      });
      if (!staged.attempt) {
        if (staged.row.state === 'signing_unknown') reply.code(202);
        return response(staged.row);
      }
      const row = staged.row;
      const a = row.artifact;
      const signed = await attestors.sign({
        keyId: a.key_id, payload: { kind: 'evm_tx', transaction: a.transaction },
        authorisation: { scheme: 'supabase_jwt', token: req.headers.authorization!.slice('Bearer '.length).trim() },
      });
      if (signed.kind !== 'evm_tx' || signed.signedBytes !== a.signing_hash
        || (signed.recoveryId !== 0 && signed.recoveryId !== 1)) {
        return refuse(502, 'evm_signature_evidence_mismatch');
      }
      const decoded = parseTransaction(a.transaction);
      const unsigned = { ...decoded, r: undefined, s: undefined, v: undefined, yParity: undefined };
      if (keccak256(serializeTransaction(unsigned)) !== a.signing_hash) return refuse(502, 'evm_signature_evidence_mismatch');
      const raw = serializeTransaction(unsigned, splitSignature(signed));
      if ((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()
        !== publicKeyToAddress(`0x${a.public_key}`).toLowerCase()) return refuse(502, 'evm_signature_evidence_mismatch');
      const rpc = sharedRpcPool();
      const { hash } = await rpc.sendRawTransactionVia(raw);
      const receipt = await rpc.sequencerReceipt(hash);
      const evidence: NonNullable<EvmApprovalRow['evidence']> = {
        tx_hash: hash, rung: receipt === null ? 'submitted' : receipt.rung,
        receipt, attestor_audit: signed.audit,
      };
      return transaction(async (client) => {
        const current = await retained(client, req.user!.id, row.id);
        if (current.state !== 'signing_unknown' || JSON.stringify(current.artifact) !== JSON.stringify(a)
          || JSON.stringify(current.approval) !== JSON.stringify(row.approval)) return refuse(409, 'evm_signing_state_changed');
        await client.query(
          "update wallet_evm_approvals set state='signed',evidence=$2::jsonb,completed_at=now() where id=$1",
          [row.id, JSON.stringify(evidence)],
        );
        return response({ ...current, state: 'signed', evidence });
      });
    });
  });

  app.get('/v1/wallet/evm/approvals/:id', { preHandler: requireAuth }, async (req, reply) => {
    const id = idSchema.safeParse((req.params as { id: unknown }).id);
    if (!id.success) return reply.code(400).send({ error: 'invalid_evm_approval_id' });
    return guarded(reply, () => transaction(async (client) => response(await retained(client, req.user!.id, id.data))));
  });
}
