import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { createHash, randomUUID } from 'node:crypto';
import type { Pool, PoolClient } from 'pg';
import { z } from 'zod';
import { keccak256, recoverAddress, serializeSignature, type Hex, type TransactionSerializableEIP1559, type TypedDataDefinition } from 'viem';
import { requireAuth } from '../middleware/auth.js';
import { CustodyAuthorityError, requireInventoryKey } from '../agent/authority.js';
import { didFromPublicKey, mainAccountId, verifyEd25519 as verifyLxSignature } from '../provision/bind.js';
import {
  archivedWalletGuard,
  getMigrationAwareSigningAccountForRow,
  defaultWalletAttestors,
  usesAttestorCustody,
  loadWalletForSigning,
  logSignature,
  type SignatureLogInput,
  type SigningWallet,
} from '../db/wallets.js';
import { getPool } from '../db/pool.js';
import { evaluate } from '../policy/index.js';
import { env } from '../env.js';
import {
  AttestorError, AttestorSessionError, AttestorQuorumError, constructionDigest, splitSignature, type SignPayload,
  attestorErrorBody,
  attestorErrorStatus,
  type AttestorClient,
  type NodeAudit,
  type SignResult,
  type ActivityDisclosureWire,
  type ActivityApprovalWire,
  lxSigningPreimage,
} from '../attestor/client.js';
import { BroadcastOutcomeUnknownError, NonceConflictError, outcomeUnknownBody, RpcPool, RpcResponseError, sharedRpcPool } from '../rpc/pool.js';
import { NonceBelowLatestError, NonceOutstandingError, NonceStore, NonceUnresolvedError, broadcastReserved, sharedNonceStore } from '../nonce/store.js';
import { BROADCAST_GRACE_MS } from '../chain.js';
import {
  RateLimitedError,
  RateLimiter,
  rateLimitBody,
  recordSigningAudit,
  type AuditKind,
  type AuditPath,
} from '../audit.js';
import { SignTxBody, SendTxBody as SendTxSchema, SignMessageBody, type TxRequest } from '../schemas/tx.js';

const SendTxBody = SendTxSchema;

export interface SignRoutesOptions {
  pool?: Pool;
  attestors?: AttestorClient | null;
  rpc?: RpcPool;
  nonces?: NonceStore;
  limiter?: RateLimiter;
}

const TypedDataField = z.object({ name: z.string().min(1).max(256), type: z.string().min(1).max(256) }).strict();

const TypedDataDocument = z
  .object({
    domain: z.record(z.unknown()),
    types: z.record(z.array(TypedDataField).max(256)),
    primaryType: z.string().min(1).max(256),
    message: z.record(z.unknown()),
  })
  .strict();

export const SignTypedDataBody = z.union([
  z.object({ typedData: TypedDataDocument }).strict(),
  z.object({ typed_data: TypedDataDocument }).strict(),
]);

function hashRequest(payload: unknown): string {
  return createHash('sha256').update(JSON.stringify(payload)).digest('hex');
}

function bearerToken(req: FastifyRequest): string {
  const header = req.headers.authorization ?? '';
  return header.slice('Bearer '.length).trim();
}

function defaultAttestors(): AttestorClient | null {
  return defaultWalletAttestors();
}

interface SignedValue {
  value: Hex;
  attestor: SignResult | null;
}

interface WalletSigner {
  path: AuditPath;
  address: `0x${string}`;
  signTransaction(tx: TransactionSerializableEIP1559): Promise<SignedValue>;
  signMessage(message: string): Promise<SignedValue>;
  signTypedData(td: TypedDataDefinition): Promise<SignedValue>;
}

async function walletSigner(
  sw: SigningWallet,
  attestors: AttestorClient | null,
  token: string,
  connection: PoolClient,
): Promise<WalletSigner> {
  const account = await getMigrationAwareSigningAccountForRow(sw.row, { scheme: 'supabase_jwt', token }, attestors, connection);
  return {
    path: account.path,
    address: account.address,
    async signTransaction(tx) {
      const value = await account.signTransaction(tx);
      return { value, attestor: account.lastAttestorResult() };
    },
    async signMessage(message) {
      const value = await account.signMessage({ message });
      return { value, attestor: account.lastAttestorResult() };
    },
    async signTypedData(td) {
      const value = await account.signTypedData(td);
      return { value, attestor: account.lastAttestorResult() };
    },
  };
}

class RouteRefusal extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly body: Record<string, unknown>,
    readonly headers: Record<string, string> = {},
  ) {
    super(code);
    this.name = 'RouteRefusal';
  }
}

interface SigningContext {
  attestorOnly?: boolean;
  requestId: string;
  subject: string;
  route: string;
  kind: AuditKind;
  requestHash: string;
  to?: string;
}

interface WalletStage {
  client: PoolClient;
  sw: SigningWallet;
  signer: WalletSigner;
}

interface Completed {
  status: number;
  body: Record<string, unknown>;
  decision: 'signed' | 'broadcast' | 'broadcast_failed';
  path: AuditPath;
  account: string;
  walletId: string;
  attestor: SignResult | null;
  txHash: string | null;
  nonce: number | null;
  reasonCode: string | null;
  signatureLog?: SignatureLogInput;
}

export async function signRoutes(app: FastifyInstance, opts: SignRoutesOptions = {}): Promise<void> {
  const pool = opts.pool ?? getPool();
  const rpc = opts.rpc ?? sharedRpcPool();
  const nonces = opts.nonces ?? sharedNonceStore();
  const limiter =
    opts.limiter ??
    new RateLimiter({
      pool,
      clientPerMinute: env.RATE_LIMIT_CLIENT_PER_MINUTE,
      accountPerMinute: env.RATE_LIMIT_ACCOUNT_PER_MINUTE,
    });
  const attestors = opts.attestors !== undefined ? opts.attestors : defaultAttestors();
  await lxApprovalRoutes(app, pool, attestors, limiter);

  async function sequencerReceipt(hash: Hex) {
    const receipt = await rpc.sequencerReceipt(hash);
    return { receipt, rung: receipt === null ? ('submitted' as const) : receipt.rung };
  }

  function requireQuorumWallet(sw:SigningWallet):asserts sw is SigningWallet & {attestorKeyId:string} {
    if(!attestors||!sw.attestorKeyId||!usesAttestorCustody({migratedAt:sw.migratedAt,hasEnvelope:sw.row.encrypted_private_key!==null})) throw new AttestorQuorumError('attestor_custody_required','this operation requires the original wallet attestor key');
  }
  async function signConstruction(sw:SigningWallet,req:FastifyRequest,payload:SignPayload,digest:Hex){
    requireQuorumWallet(sw);
    const result=await attestors!.sign({keyId:sw.attestorKeyId,payload,authorisation:{scheme:'supabase_jwt',token:bearerToken(req)}});
    const signature=serializeSignature(splitSignature(result));
    if(result.signedBytes.toLowerCase()!==digest.toLowerCase()||(await recoverAddress({hash:digest,signature})).toLowerCase()!==sw.row.address.toLowerCase()) throw new AttestorSessionError('signer_mismatch','quorum evidence differs from original owned wallet or construction');
    return {result,signature};
  }
  async function run(
    req: FastifyRequest,
    reply: FastifyReply,
    ctx: SigningContext,
    valueWei: bigint,
    work: (stage: WalletStage) => Promise<Completed>,
  ): Promise<FastifyReply> {
    let account: string | null = null;
    let walletId: string | null = null;
    let path: AuditPath = 'none';
    const refuse = async (
      status: number,
      reasonCode: string,
      body: Record<string, unknown>,
      headers: Record<string, string> = {},
    ): Promise<FastifyReply> => {
      await recordSigningAudit(pool, {
        requestId: ctx.requestId,
        clientSubject: ctx.subject,
        account,
        walletId,
        route: ctx.route,
        kind: ctx.kind,
        path,
        decision: 'refused',
        reasonCode,
        requestHash: ctx.requestHash,
        sessionId: null,
        attestorAudit: [],
        txHash: null,
        nonce: null,
      });
      for (const [k, v] of Object.entries(headers)) void reply.header(k, v);
      return reply.code(status).send({ ...body, request_id: ctx.requestId });
    };

    if (ctx.to) {
      const archived = await archivedWalletGuard({ address: ctx.to });
      if (archived) return refuse(archived.status, 'wallet_archived', archived.body);
    }

    try {
      await limiter.consume('client', ctx.subject);
    } catch (err) {
      if (err instanceof RateLimitedError) {
        return refuse(429, 'rate_limited_client', rateLimitBody(err), {
          'retry-after': String(err.retryAfterSeconds),
        });
      }
      throw err;
    }

    const decision = await evaluate({ user_id: ctx.subject, value_wei: valueWei });
    if (!decision.allow) {
      return refuse(403, `policy_${decision.code.toLowerCase()}`, { error: decision.code, message: decision.message });
    }

    const client = await pool.connect();
    let completed: Completed;
    try {
      await client.query('BEGIN');
      const sw = await loadWalletForSigning(client, ctx.subject);
      if (!sw) {
        const archived = await archivedWalletGuard({ userId: ctx.subject, kind: 'standard' });
        if (archived) throw new RouteRefusal(archived.status, 'wallet_archived', archived.body);
        throw new RouteRefusal(404, 'no_wallet', { error: 'no_wallet', message: 'wallet not provisioned' });
      }
      account = sw.row.address;
      walletId = sw.row.id;
      path = usesAttestorCustody({ migratedAt: sw.migratedAt, hasEnvelope: sw.row.encrypted_private_key !== null }) ? 'attestor' : 'envelope';
      if (sw.row.is_disabled) {
        throw new RouteRefusal(403, 'wallet_disabled', {
          error: 'WALLET_DISABLED',
          message: sw.row.disabled_reason ?? 'wallet disabled',
        });
      }
      try {
        await limiter.consume('account', sw.row.address);
      } catch (err) {
        if (err instanceof RateLimitedError) {
          throw new RouteRefusal(429, 'rate_limited_account', rateLimitBody(err), {
            'retry-after': String(err.retryAfterSeconds),
          });
        }
        throw err;
      }
      if(ctx.attestorOnly) requireQuorumWallet(sw);
      const signer = await walletSigner(sw, attestors, bearerToken(req), client);
      completed = await work({ client, sw, signer });
      await client.query('COMMIT');
    } catch (err) {
      await client.query('ROLLBACK').catch(() => undefined);
      client.release();
      if (err instanceof RouteRefusal) return refuse(err.status, err.code, err.body, err.headers);
      if(err instanceof CustodyAuthorityError) return refuse(503,err.code,{error:err.code,replication_pending:true});
      if (err instanceof NonceUnresolvedError) {
        return refuse(503, 'nonce_unresolved', { error: 'nonce_unresolved', nonce: err.nonce, unreachable: err.unreachable, members: err.memberCount });
      }
      if (err instanceof NonceBelowLatestError) {
        return refuse(409, 'nonce_below_latest', { error: 'nonce_below_latest', nonce: err.nonce, latest: err.latest });
      }
      if (err instanceof NonceOutstandingError) {
        return refuse(409, 'nonce_outstanding', { error: 'nonce_outstanding', nonce: err.nonce });
      }
      if (err instanceof AttestorError) {
        return refuse(attestorErrorStatus(err), `${err.category}:${err.code}`, attestorErrorBody(err));
      }
      if (err instanceof RateLimitedError) {
        return refuse(429, `rate_limited_${err.scope}`, rateLimitBody(err));
      }
      req.log.error({ err, route: ctx.route }, 'signing failed');
      return refuse(500, 'internal', {
        error: ctx.route.endsWith('/send') ? 'send_failed' : 'sign_failed',
        detail: (err as Error).message,
      });
    }
    client.release();

    if (completed.signatureLog) await logSignature(completed.signatureLog);
    await recordSigningAudit(pool, {
      requestId: ctx.requestId,
      clientSubject: ctx.subject,
      account: completed.account,
      walletId: completed.walletId,
      route: ctx.route,
      kind: ctx.kind,
      path: completed.path,
      decision: completed.decision,
      reasonCode: completed.reasonCode,
      requestHash: ctx.requestHash,
      sessionId: completed.attestor?.sessionId ?? null,
      attestorAudit: completed.attestor?.audit ?? ([] as NodeAudit[]),
      txHash: completed.txHash,
      nonce: completed.nonce,
    });
    void reply.header('x-request-id', ctx.requestId);
    return reply.code(completed.status).send(completed.body);
  }

  async function prepareTx(
    signer: WalletSigner,
    tx: TxRequest,
  ): Promise<Omit<TransactionSerializableEIP1559, 'nonce'>> {
    const params = buildTxParams(tx);
    try {
      await rpc.simulate({ from: signer.address, to: params.to, data: params.data, value: params.value });
    } catch (err) {
      if (err instanceof RpcResponseError) {
        throw new RouteRefusal(422, 'simulation_failed', {
          error: 'simulation_failed',
          message: err.message,
        });
      }
      throw err;
    }
    const gas = await rpc.prepareGas({
      from: signer.address,
      to: params.to,
      data: params.data,
      value: params.value,
      gas: params.gas,
      maxFeePerGas: params.maxFeePerGas,
      maxPriorityFeePerGas: params.maxPriorityFeePerGas,
    });
    return {
      type: 'eip1559',
      chainId: tx.chainId ?? env.HYPERPAXEER_CHAIN_ID,
      to: params.to,
      value: params.value,
      data: params.data,
      gas: gas.gas,
      maxFeePerGas: gas.maxFeePerGas,
      maxPriorityFeePerGas: gas.maxPriorityFeePerGas,
    };
  }

  const ipOf = (req: FastifyRequest): string | null => req.ip;

  app.post('/v1/wallet/sign-digest',{preHandler:requireAuth},async(req,reply)=>{
    const parsed=SignDigestBody.safeParse(req.body);if(!parsed.success) return reply.code(400).send({error:'invalid_body',issues:parsed.error.issues});
    const c=parsed.data.construction;
    return run(req,reply,{requestId:randomUUID(),subject:req.user!.id,route:'/v1/wallet/sign-digest',kind:'typed_data',requestHash:hashRequest(parsed.data),attestorOnly:true},c.kind==='sponsored_batch'?c.calls.reduce((sum,call)=>sum+BigInt(call.value),0n):0n,async({sw})=>{
      if(BigInt(c.chainId)!==BigInt(sw.row.chain_id)||sw.row.chain_id!==env.HYPERPAXEER_CHAIN_ID) throw new RouteRefusal(403,'chain_mismatch',{error:'chain_mismatch'});
      if(c.kind==='sponsored_batch'){
        if(c.account.toLowerCase()!==sw.row.address.toLowerCase()||BigInt(c.quote.deadline)<=BigInt(Math.floor(Date.now()/1000))||BigInt(c.quote.tokenAmount)>BigInt(c.quote.maxTokenAmount)) throw new RouteRefusal(403,'consent_mismatch',{error:'consent_mismatch'});
      }else if(BigInt(c.nonce)!==BigInt(await rpc.getTransactionCount(sw.row.address as Hex,'latest'))) throw new RouteRefusal(409,'authorization_nonce_changed',{error:'authorization_nonce_changed'});
      const digest=constructionDigest(c);const signed=await signConstruction(sw,req,{kind:'eth_sign_digest',digest,construction:c},digest);
      if(c.kind==='sponsored_batch'&&BigInt(c.quote.deadline)<=BigInt(Math.floor(Date.now()/1000)))throw new RouteRefusal(403,'consent_expired',{error:'consent_expired'});
      if(c.kind==='eip7702_authorization'&&BigInt(c.nonce)!==BigInt(await rpc.getTransactionCount(sw.row.address as Hex,'latest')))throw new RouteRefusal(409,'authorization_nonce_changed',{error:'authorization_nonce_changed'});
      return {status:200,body:{signature:signed.signature,address:sw.row.address},decision:'signed',path:'attestor',account:sw.row.address,walletId:sw.row.id,attestor:signed.result,txHash:null,nonce:null,reasonCode:null};
    });
  });

  app.post('/v1/wallet/sign', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = SignTxBody.safeParse(req.body);
    if (!parsed.success) {
      return reply.code(400).send({ error: 'invalid_body', issues: parsed.error.issues });
    }
    const userId = req.user!.id;
    const { tx } = parsed.data;
    const valueWei = tx.value ? BigInt(tx.value) : 0n;
    const chainId = tx.chainId ?? env.HYPERPAXEER_CHAIN_ID;
    const requestHash = hashRequest({ tx });
    return run(
      req,
      reply,
      { requestId: randomUUID(), subject: userId, route: '/v1/wallet/sign', kind: 'transaction', requestHash, to: tx.to },
      valueWei,
      async ({ sw, signer }) => {
        const prepared = await prepareTx(signer, tx);
        const broadcastDeadline = new Date(Date.now() + BROADCAST_GRACE_MS).toISOString();
        const { nonce, signed } = await nonces.withLock(sw.row.address, async (lease) => {
          let nonce: number;
          if (tx.nonce !== undefined) {
            await lease.reserveClientNonce(tx.nonce);
            nonce = tx.nonce;
          } else {
            nonce = await lease.next();
          }
          const signed = await signer.signTransaction({ ...prepared, nonce });
          await lease.track(nonce, keccak256(signed.value), null);
          return { nonce, signed };
        });
        const signatureLog: SignatureLogInput = {
          user_id: userId,
          wallet_id: sw.row.id,
          address: sw.row.address,
          kind: 'transaction',
          to_address: tx.to ?? null,
          value_wei: valueWei,
          chain_id: chainId,
          request_hash: requestHash,
          ip: ipOf(req),
          user_agent: req.headers['user-agent'] ?? null,
        };
        return {
          status: 200,
          body: { signed_tx: signed.value, address: sw.row.address, chain_id: chainId, nonce, broadcast_deadline: broadcastDeadline },
          decision: 'signed',
          path: signer.path,
          account: sw.row.address,
          walletId: sw.row.id,
          attestor: signed.attestor,
          txHash: null,
          nonce,
          reasonCode: null,
          signatureLog,
        };
      },
    );
  });

  app.post('/v1/wallet/send', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = SendTxBody.safeParse(req.body);
    if (!parsed.success) {
      return reply.code(400).send({ error: 'invalid_body', issues: parsed.error.issues });
    }
    const userId = req.user!.id;
    const { tx } = parsed.data;
    const valueWei = tx.value ? BigInt(tx.value) : 0n;
    const chainId = tx.chainId ?? env.HYPERPAXEER_CHAIN_ID;
    const requestHash = hashRequest({ tx });
    return run(
      req,
      reply,
      { requestId: randomUUID(), subject: userId, route: '/v1/wallet/send', kind: 'transaction', requestHash, to: tx.to },
      valueWei,
      async ({ sw, signer }) => {
        const prepared = await prepareTx(signer, tx);
        const outcome = await nonces.withLock(sw.row.address, async (lease) => {
          if (tx.nonce !== undefined) {
            try {
              await lease.reserveClientNonce(tx.nonce);
            } catch (err) {
              if (err instanceof NonceBelowLatestError) {
                throw new RouteRefusal(409, 'nonce_below_latest', { error: 'nonce_below_latest', nonce: err.nonce, latest: err.latest });
              }
              throw err;
            }
          }
          const nonce = tx.nonce ?? (await lease.next());
          const signed = await signer.signTransaction({ ...prepared, nonce });
          const sent = await broadcastReserved(lease, nonce, signed.value, (raw) => rpc.sendRawTransactionVia(raw));
          return sent.hash !== null
            ? { ok: true as const, hash: sent.hash, nonce, signed }
            : { ok: false as const, error: sent.error as Error, nonce, signed };
        });
        if (!outcome.ok && outcome.error instanceof NonceConflictError) {
          return {
            status: 409,
            body: { error: 'nonce_conflict', nonce: outcome.nonce, detail: outcome.error.message, retry: 'prepare',
              member: outcome.error.member, members: outcome.error.members },
            decision: 'broadcast_failed',
            path: signer.path,
            account: sw.row.address,
            walletId: sw.row.id,
            attestor: outcome.signed.attestor,
            txHash: null,
            nonce: outcome.nonce,
            reasonCode: 'nonce_conflict',
          };
        }
        if (!outcome.ok && outcome.error instanceof BroadcastOutcomeUnknownError) {
          return {
            status: 202,
            body: outcomeUnknownBody(outcome.error),
            decision: 'broadcast_failed',
            path: signer.path,
            account: sw.row.address,
            walletId: sw.row.id,
            attestor: outcome.signed.attestor,
            txHash: outcome.error.hash,
            nonce: outcome.nonce,
            reasonCode: 'broadcast_outcome_unknown',
          };
        }
        if (!outcome.ok) {
          req.log.error({ err: outcome.error, userId }, 'send_transaction broadcast failed');
          return {
            status: 500,
            body: { error: 'send_failed', detail: outcome.error.message },
            decision: 'broadcast_failed',
            path: signer.path,
            account: sw.row.address,
            walletId: sw.row.id,
            attestor: outcome.signed.attestor,
            txHash: null,
            nonce: outcome.nonce,
            reasonCode: outcome.error instanceof RpcResponseError ? `rpc_${outcome.error.code}` : 'rpc_unavailable',
          };
        }
        const signatureLog: SignatureLogInput = {
          user_id: userId,
          wallet_id: sw.row.id,
          address: sw.row.address,
          kind: 'transaction',
          to_address: tx.to ?? null,
          value_wei: valueWei,
          chain_id: chainId,
          request_hash: requestHash,
          tx_hash: outcome.hash,
          ip: ipOf(req),
          user_agent: req.headers['user-agent'] ?? null,
        };
        return {
          status: 200,
          body: {
            tx_hash: outcome.hash,
            address: sw.row.address,
            chain_id: chainId,
            ...(await sequencerReceipt(outcome.hash)),
          },
          decision: 'broadcast',
          path: signer.path,
          account: sw.row.address,
          walletId: sw.row.id,
          attestor: outcome.signed.attestor,
          txHash: outcome.hash,
          nonce: outcome.nonce,
          reasonCode: null,
          signatureLog,
        };
      },
    );
  });

  app.post('/v1/wallet/sign-message', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = SignMessageBody.safeParse(req.body);
    if (!parsed.success) {
      return reply.code(400).send({ error: 'invalid_body', issues: parsed.error.issues });
    }
    const userId = req.user!.id;
    const requestHash = hashRequest({ message: parsed.data.message });
    return run(
      req,
      reply,
      { requestId: randomUUID(), subject: userId, route: '/v1/wallet/sign-message', kind: 'message', requestHash },
      0n,
      async ({ sw, signer }) => {
        const signed = await signer.signMessage(parsed.data.message);
        const signatureLog: SignatureLogInput = {
          user_id: userId,
          wallet_id: sw.row.id,
          address: sw.row.address,
          kind: 'message',
          request_hash: requestHash,
          ip: ipOf(req),
          user_agent: req.headers['user-agent'] ?? null,
        };
        return {
          status: 200,
          body: { signature: signed.value, address: sw.row.address },
          decision: 'signed',
          path: signer.path,
          account: sw.row.address,
          walletId: sw.row.id,
          attestor: signed.attestor,
          txHash: null,
          nonce: null,
          reasonCode: null,
          signatureLog,
        };
      },
    );
  });

  app.post('/v1/wallet/sign-typed-data', { preHandler: requireAuth }, async (req, reply) => {
    const parsed = SignTypedDataBody.safeParse(req.body);
    if (!parsed.success) {
      return reply.code(400).send({ error: 'invalid_body', issues: parsed.error.issues });
    }
    const userId = req.user!.id;
    const document = 'typedData' in parsed.data ? parsed.data.typedData : parsed.data.typed_data;
    const td = document as unknown as TypedDataDefinition;
    const requestHash = hashRequest({ typed_data: document });
    return run(
      req,
      reply,
      { requestId: randomUUID(), subject: userId, route: '/v1/wallet/sign-typed-data', kind: 'typed_data', requestHash },
      0n,
      async ({ sw, signer }) => {
        const signed = await signer.signTypedData(td);
        const signatureLog: SignatureLogInput = {
          user_id: userId,
          wallet_id: sw.row.id,
          address: sw.row.address,
          kind: 'typed_data',
          request_hash: requestHash,
          ip: ipOf(req),
          user_agent: req.headers['user-agent'] ?? null,
        };
        return {
          status: 200,
          body: { signature: signed.value, address: sw.row.address },
          decision: 'signed',
          path: signer.path,
          account: sw.row.address,
          walletId: sw.row.id,
          attestor: signed.attestor,
          txHash: null,
          nonce: null,
          reasonCode: null,
          signatureLog,
        };
      },
    );
  });
}

function buildTxParams(tx: TxRequest): {
  to?: `0x${string}`;
  value?: bigint;
  data?: `0x${string}`;
  gas?: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
  nonce?: number;
} {
  const out: ReturnType<typeof buildTxParams> = {};
  if (tx.to) out.to = tx.to as `0x${string}`;
  if (tx.value !== undefined) out.value = BigInt(tx.value);
  if (tx.data) out.data = tx.data as `0x${string}`;
  if (tx.gas !== undefined) out.gas = BigInt(tx.gas);
  if (tx.maxFeePerGas !== undefined) out.maxFeePerGas = BigInt(tx.maxFeePerGas);
  if (tx.maxPriorityFeePerGas !== undefined)
    out.maxPriorityFeePerGas = BigInt(tx.maxPriorityFeePerGas);
  if (tx.nonce !== undefined) out.nonce = tx.nonce;
  return out;
}

const CanonicalUint=z.string().max(78).regex(/^(0|[1-9][0-9]*)$/).refine(v=>BigInt(v)<1n<<256n);
const CanonicalU64=CanonicalUint.refine(v=>BigInt(v)<(1n<<64n)-1n);
const WireAddress=z.string().regex(/^0x[0-9a-fA-F]{40}$/);
const WireBytes=z.string().max(65_538).regex(/^0x(?:[0-9a-fA-F]{2})*$/);
export const SponsoredConstruction=z.object({kind:z.literal('sponsored_batch'),chainId:CanonicalUint,account:WireAddress,nonce:CanonicalUint,calls:z.array(z.object({to:WireAddress,value:CanonicalUint,data:WireBytes}).strict()).min(1).max(128),quote:z.object({sponsor:WireAddress,token:WireAddress,maxTokenAmount:CanonicalUint,tokenAmount:CanonicalUint,deadline:CanonicalU64,quoteNonce:CanonicalUint,gasCost:CanonicalUint}).strict()}).strict();
export const DigestConstructionBody=z.discriminatedUnion('kind',[SponsoredConstruction,z.object({kind:z.literal('eip7702_authorization'),chainId:CanonicalUint,address:WireAddress,nonce:CanonicalU64}).strict()]);
export const SignDigestBody=z.object({construction:DigestConstructionBody}).strict();

interface OriginalLxWallet {
  id: string; address: string; chain_id: number; did: string; main_account_id: string;
  layerx_key_id: string; is_disabled: boolean; archived_at: unknown; binding_state: string;
}
interface LxArtifact {
  kind: 'lx_send_authorization';
  id: string; activity: string; signing_preimage: string; public_key: string; key_id: string;
  principal: string; did: string; account_id: string; epoch: number; participants: string[];
  network_id: number; protocol_version: number; session_id: string;
  disclosure: ActivityDisclosureWire; expires_at: string;
}
interface LxApprovalRow {
  id: string; wallet_id: string; principal: string; state: 'reviewed'|'approved'|'signing_unknown'|'signed';
  artifact: LxArtifact; approval: ActivityApprovalWire|null;
  evidence: {signature:string;attestor_audit:NodeAudit[]}|null;
}
function stableLx(value: unknown): string {
  if (Array.isArray(value)) return '[' + value.map(stableLx).join(',') + ']';
  if (value !== null && typeof value === 'object') return '{' + Object.entries(value).sort(([a],[b]) => a.localeCompare(b)).map(([key,item]) => JSON.stringify(key) + ':' + stableLx(item)).join(',') + '}';
  const encoded=JSON.stringify(value);if(encoded===undefined)throw new Error('non-JSON LX artifact');return encoded;
}
async function lxApprovalRoutes(app:FastifyInstance,pool:Pool,attestors:AttestorClient|null,limiter:RateLimiter):Promise<void> {
  const idSchema=z.string().uuid();
  const bareBytes=z.string().max(2_097_152).regex(/^(?:[0-9a-f]{2})+$/);
  const reviewSchema=z.object({activity:bareBytes,kind:z.literal('lx_send_authorization').default('lx_send_authorization')}).strict();
  const approveSchema=z.object({review_id:idSchema,activity:bareBytes,signing_preimage:z.string().regex(/^[0-9a-f]{64}$/),
    disclosure:z.unknown(),expires_at:z.string().regex(/^(0|[1-9][0-9]{0,19})$/),decision:z.literal('approve')}).strict();
  const signSchema=z.object({approval_id:idSchema}).strict();
  const response=(row:LxApprovalRow)=>({...row.artifact,state:row.state,...(row.approval?{approval:row.approval}:{}),...(row.evidence??{})});
  const refusal=(status:number,code:string):never=>{throw new RouteRefusal(status,code,{error:code});};
  async function original(client:PoolClient,subject:string):Promise<OriginalLxWallet> {
    const found=await client.query<OriginalLxWallet>(`select id,address,chain_id,did,main_account_id,layerx_key_id,is_disabled,archived_at,binding_state
      from wallets where user_id=$1 and kind='standard' for update`,[subject]);
    const wallet=found.rows[0];
    if(!wallet||wallet.is_disabled||wallet.archived_at!==null||wallet.binding_state!=='bound'||!wallet.layerx_key_id||!wallet.did||!wallet.main_account_id) return refusal(403,'original_lx_identity_unavailable');
    const key=requireInventoryKey(wallet.layerx_key_id,'sign',subject);
    if(key.curve!=='ed25519'||didFromPublicKey(key.public_key)!==wallet.did||mainAccountId(wallet.did)!==wallet.main_account_id) return refusal(403,'original_lx_identity_mismatch');
    return wallet;
  }
  async function retained(client:PoolClient,subject:string,id:string,wallet:OriginalLxWallet):Promise<LxApprovalRow> {
    const found=await client.query<LxApprovalRow>('select * from wallet_lx_approvals where id=$1 and principal=$2 for update',[id,subject]);
    const row=found.rows[0];if(!row||row.wallet_id!==wallet.id)return refusal(404,'lx_approval_not_found');
    const a=row.artifact;const key=requireInventoryKey(wallet.layerx_key_id,'sign',subject);
    if(a.id!==row.id||a.principal!==subject||a.key_id!==wallet.layerx_key_id||a.did!==wallet.did||a.account_id!==wallet.main_account_id||a.public_key!==key.public_key||a.epoch!==key.epoch||a.network_id!==wallet.chain_id) return refusal(409,'lx_approval_identity_changed');
    return row;
  }
  async function transaction<T>(work:(client:PoolClient)=>Promise<T>):Promise<T> {
    const client=await pool.connect();try{await client.query('begin');const result=await work(client);await client.query('commit');return result;}
    catch(error){await client.query('rollback').catch(()=>undefined);throw error;}finally{client.release();}
  }
  async function guarded(reply:FastifyReply,work:()=>Promise<unknown>):Promise<FastifyReply> {
    try{return reply.send(await work());}catch(error){
      if(error instanceof RouteRefusal)return reply.code(error.status).send(error.body);
      if(error instanceof AttestorError)return reply.code(attestorErrorStatus(error)).send(attestorErrorBody(error));
      if(error instanceof CustodyAuthorityError)return reply.code(503).send({error:error.code});
      if(error instanceof RateLimitedError)return reply.code(429).send(rateLimitBody(error));
      return reply.code(503).send({error:'lx_producer_unavailable'});
    }
  }
  app.post('/v1/wallet/lx/review',{preHandler:requireAuth,bodyLimit:2_400_000},async(req,reply)=>{
    const parsed=reviewSchema.safeParse(req.body);if(!parsed.success)return reply.code(400).send({error:'invalid_lx_review'});
    return guarded(reply,async()=>{
      await limiter.consume('client',req.user!.id);
      if(!attestors)return refusal(503,'attestor_custody_required');
      const activity=parsed.data.activity;const kind=parsed.data.kind;const raw=Buffer.from(activity,'hex');
      if(raw.length<5||raw.readUInt16BE(0)<1||raw.readUInt16BE(0)>3)return refusal(409,'lx_protocol_unsupported');
      if(raw.readUInt16BE(2)!==0x1001||raw[4]!==11)return refusal(400,'lx_unsigned_canonical_required');
      return transaction(async client=>{
        const wallet=await original(client,req.user!.id);await limiter.consume('account',wallet.address);
        await client.query("delete from wallet_lx_approvals where principal=$1 and state in ('reviewed','approved') and expires_at<=now()",[req.user!.id]);
        const count=await client.query<{count:string}>("select count(*)::text as count from wallet_lx_approvals where principal=$1 and state in ('reviewed','approved','signing_unknown')",[req.user!.id]);
        if(!count.rows[0]||BigInt(count.rows[0].count)>=256n)return refusal(429,'lx_approval_capacity');
        const id=randomUUID();const session=randomUUID();
        const review=await attestors.reviewLxActivity({keyId:wallet.layerx_key_id,sessionId:session,activity,kind,owner:req.user!.id,token:bearerToken(req)});
        const digest=lxSigningPreimage(activity,kind);
        if(review.signing_preimage!==digest||review.network_id!==wallet.chain_id||review.network_id!==env.HYPERPAXEER_CHAIN_ID||review.disclosure.account!==wallet.main_account_id)return refusal(403,'lx_review_binding_mismatch');
        const now=BigInt(Math.floor(Date.now()/1000));const until=BigInt(review.not_after);const expiry=until<now+600n?until:now+600n;
        if(expiry<=now||BigInt(review.disclosure.not_before)>now)return refusal(403,'lx_activity_outside_validity');
        const artifact:LxArtifact={id,kind,activity,signing_preimage:digest,public_key:review.public_key,key_id:wallet.layerx_key_id,
          principal:req.user!.id,did:wallet.did,account_id:wallet.main_account_id,epoch:review.epoch,participants:review.participants,
          network_id:review.network_id,protocol_version:review.protocol_version,session_id:session,disclosure:review.disclosure,expires_at:expiry.toString()};
        await client.query(`insert into wallet_lx_approvals(id,wallet_id,principal,session_id,state,artifact,expires_at,review_evidence) values($1,$2,$3,$4,'reviewed',$5::jsonb,to_timestamp($6::double precision),$7::jsonb)`,[id,wallet.id,req.user!.id,session,JSON.stringify(artifact),artifact.expires_at,JSON.stringify(review.audit)]);
        return {...artifact,state:'reviewed'};
      });
    });
  });
  app.post('/v1/wallet/lx/approve',{preHandler:requireAuth,bodyLimit:2_400_000},async(req,reply)=>{
    const parsed=approveSchema.safeParse(req.body);if(!parsed.success)return reply.code(400).send({error:'explicit_lx_approval_required'});
    return guarded(reply,()=>transaction(async client=>{
      await limiter.consume('client',req.user!.id);const wallet=await original(client,req.user!.id);const row=await retained(client,req.user!.id,parsed.data.review_id,wallet);const a=row.artifact;
      if(parsed.data.activity!==a.activity||parsed.data.signing_preimage!==a.signing_preimage||parsed.data.expires_at!==a.expires_at||stableLx(parsed.data.disclosure)!==stableLx(a.disclosure))return refusal(409,'lx_review_changed');
      if(row.state!=='reviewed')return response(row);
      if(BigInt(a.expires_at)<=BigInt(Math.floor(Date.now()/1000)))return refusal(403,'lx_approval_expired');
      const approval:ActivityApprovalWire={version:1,principal:a.principal,key_id:a.key_id,network_id:a.network_id,protocol_version:a.protocol_version,
        session_id:a.session_id,activity_digest:a.signing_preimage,expires_at:a.expires_at};
      await client.query("update wallet_lx_approvals set state='approved',approval=$2::jsonb,approved_at=now() where id=$1",[row.id,JSON.stringify(approval)]);
      return response({...row,state:'approved',approval});
    }));
  });
  app.get('/v1/wallet/lx/approvals/:id',{preHandler:requireAuth},async(req,reply)=>{
    const id=idSchema.safeParse((req.params as {id:unknown}).id);if(!id.success)return reply.code(400).send({error:'invalid_lx_approval_id'});
    return guarded(reply,()=>transaction(async client=>response(await retained(client,req.user!.id,id.data,await original(client,req.user!.id)))));
  });
  app.post('/v1/wallet/lx/sign',{preHandler:requireAuth},async(req,reply)=>{
    const parsed=signSchema.safeParse(req.body);if(!parsed.success)return reply.code(400).send({error:'retained_lx_approval_required'});
    return guarded(reply,async()=>{
      await limiter.consume('client',req.user!.id);if(!attestors)return refusal(503,'attestor_custody_required');
      const staged=await transaction(async client=>{
        const wallet=await original(client,req.user!.id);const row=await retained(client,req.user!.id,parsed.data.approval_id,wallet);
        if(row.state==='signed'||row.state==='signing_unknown')return {row,attempt:false};
        if(row.state!=='approved'||!row.approval)return refusal(403,'explicit_lx_approval_required');
        if(BigInt(row.artifact.expires_at)<=BigInt(Math.floor(Date.now()/1000)))return refusal(403,'lx_approval_expired');
        await client.query("update wallet_lx_approvals set state='signing_unknown',attempted_at=now() where id=$1",[row.id]);
        return {row:{...row,state:'signing_unknown' as const},attempt:true};
      });
      if(!staged.attempt){if(staged.row.state==='signing_unknown')reply.code(202);return response(staged.row);}
      const row=staged.row;const a=row.artifact;
      try{
        const signed=await attestors.sign({keyId:a.key_id,participants:a.participants,
          payload:{kind:a.kind,activity:`0x${a.activity}`,disclosure:a.disclosure,approval:row.approval!},authorisation:{scheme:'supabase_jwt',token:bearerToken(req)}});
        const signature=signed.signature.slice(2);
        if(signed.sessionId!==a.session_id||signed.signedBytes!==`0x${a.signing_preimage}`||signed.kind!==a.kind||signed.recoveryId!==null
          ||stableLx(signed.participants)!==stableLx(a.participants)||!verifyLxSignature(a.public_key,Buffer.from(a.signing_preimage,'hex'),signature))return refusal(502,'lx_signature_evidence_mismatch');
        const evidence={signature,attestor_audit:signed.audit};
        return await transaction(async client=>{
          const current=await retained(client,req.user!.id,row.id,await original(client,req.user!.id));
          if(current.state!=='signing_unknown'||stableLx(current.artifact)!==stableLx(a)||stableLx(current.approval)!==stableLx(row.approval))return refusal(409,'lx_signing_state_changed');
          await client.query("update wallet_lx_approvals set state='signed',evidence=$2::jsonb,completed_at=now() where id=$1",[row.id,JSON.stringify(evidence)]);
          return response({...current,state:'signed',evidence});
        });
      }catch{
        reply.code(202);return response(row);
      }
    });
  });
}
