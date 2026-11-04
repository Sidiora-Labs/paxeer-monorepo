import { createHash } from 'node:crypto';
import type { FastifyInstance, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { env } from '../env.js';
import { query } from '../db/pool.js';
import { safeEqual } from '../crypto.js';

export const LEGACY_IMPORT_PATH = '/v1/admin/legacy-import';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

const LegacyImport = z
  .object({
    legacy_wallet_id: z.string().regex(UUID),
    user_id: z.string().regex(UUID),
    address: z.string().regex(/^0x[0-9a-f]{40}$/),
    kind: z.enum(['standard', 'funded']),
    chain_id: z.number().int().positive(),
    key_version: z.number().int().min(1).max(32_767),
    encrypted_private_key: z.string().refine((value) => value.trim().length > 0),
    attestor_key_id: z.string(),
    identity_key_id: z.string(),
    binding_state: z.literal('unbound'),
    custody: z.enum(['live', 'archived']),
    is_disabled: z.boolean(),
    disabled_reason: z.string().nullable(),
    created_at: z.string().datetime(),
  })
  .strict()
  .refine(
    (r) => r.attestor_key_id === `wallet:${r.legacy_wallet_id}:secp256k1`,
    'attestor_key_id does not match the wallet'
  )
  .refine(
    (r) => r.identity_key_id === `wallet:${r.legacy_wallet_id}:ed25519`,
    'identity_key_id does not match the wallet'
  )
  .refine(
    (r) => (r.kind === 'funded') === (r.custody === 'archived'),
    'funded wallets are archived and only funded wallets are'
  );

function authorized(req: FastifyRequest): boolean {
  const header = req.headers.authorization;
  if (typeof header !== 'string' || !header.startsWith('Bearer ') || !env.GATEWAY_ADMIN_TOKEN)
    return false;
  return safeEqual(header.slice('Bearer '.length).trim(), env.GATEWAY_ADMIN_TOKEN);
}

function importReceipt(r: z.infer<typeof LegacyImport>) {
  const record_sha256 = createHash('sha256')
    .update(
      JSON.stringify([
        r.legacy_wallet_id,
        r.user_id,
        r.address,
        r.kind,
        r.chain_id,
        r.key_version,
        r.encrypted_private_key,
        r.attestor_key_id,
        r.identity_key_id,
        r.binding_state,
        r.custody,
        r.is_disabled,
        r.disabled_reason,
        r.created_at,
      ])
    )
    .digest('hex');
  return { wallet_id: r.legacy_wallet_id, custody: r.custody, record_sha256 };
}

export async function adminRoutes(app: FastifyInstance): Promise<void> {
  app.post(LEGACY_IMPORT_PATH, async (req, reply) => {
    if (!authorized(req)) return reply.code(401).send({ error: 'unauthorized' });
    const parsed = LegacyImport.safeParse(req.body);
    if (!parsed.success)
      return reply
        .code(400)
        .send({ error: 'invalid_body', issues: parsed.error.issues.map((i) => i.message) });
    const r = parsed.data;
    if (r.chain_id !== env.HYPERPAXEER_CHAIN_ID)
      return reply.code(422).send({ error: 'wrong_chain' });
    const { rows } = await query<{ id: string }>(
      `insert into wallets (id, user_id, address, encrypted_private_key, key_version, chain_id, kind,
                            created_at, is_disabled, disabled_reason, attestor_key_id, binding_state, archived_at)
       values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'unbound', case when $12 then now() end)
       on conflict do nothing
       returning id`,
      [
        r.legacy_wallet_id,
        r.user_id,
        r.address,
        r.encrypted_private_key,
        r.key_version,
        r.chain_id,
        r.kind,
        r.created_at,
        r.is_disabled,
        r.disabled_reason,
        r.attestor_key_id,
        r.custody === 'archived',
      ]
    );
    if (rows.length === 0) {
      const existing = await query<{ id: string }>(
        `select id from wallets
          where id = $1 and user_id = $2 and address = $3
            and encrypted_private_key = $4 and key_version = $5 and chain_id = $6 and kind = $7
            and created_at = $8::timestamptz and is_disabled = $9
            and disabled_reason is not distinct from $10::text
            and attestor_key_id = $11 and binding_state = 'unbound'
            and (archived_at is not null) = $12`,
        [
          r.legacy_wallet_id,
          r.user_id,
          r.address,
          r.encrypted_private_key,
          r.key_version,
          r.chain_id,
          r.kind,
          r.created_at,
          r.is_disabled,
          r.disabled_reason,
          r.attestor_key_id,
          r.custody === 'archived',
        ]
      );
      if (existing.rows.length === 0) return reply.code(409).send({ error: 'wallet_conflict' });
      return reply.code(409).send({ error: 'identical_replay', ...importReceipt(r) });
    }
    return reply.code(201).send({ status: 'inserted', ...importReceipt(r) });
  });
}
