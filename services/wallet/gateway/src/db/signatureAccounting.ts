import {
  hashMessage,
  hashTypedData,
  keccak256,
  serializeTransaction,
  type Hex,
  type TransactionSerializable,
  type TypedDataDefinition,
} from 'viem';
import type { PoolClient } from 'pg';
import { query, withTransaction } from './pool.js';
import {
  agentWalletUserId,
  type SignatureLogInput,
  type SigningAccount,
  type WalletRow,
} from './wallets.js';

export const SIGNATURE_ACCOUNTING_MIGRATION = {
  filename: 'db/001_signature_reservations.sql',
  sql: `
    create table if not exists wallet_signature_reservations (
      wallet_id uuid not null references wallets(id),
      kind text not null check (kind in ('transaction', 'message', 'typed_data')),
      request_hash text not null,
      operation_hash text not null,
      signature_id bigint not null unique references wallet_signatures(id),
      signed_evidence text,
      tx_hash text,
      primary key (wallet_id, kind, request_hash),
      unique (wallet_id, tx_hash)
    );
    drop trigger if exists wallet_signatures_touch_wallet on wallet_signatures;
    create trigger wallet_signatures_touch_wallet
      after insert on wallet_signatures for each row
      when (current_setting('wallet.accounting_reservation', true) is distinct from 'on')
      execute function touch_wallet_last_used();
  `,
};

export interface SignatureAccountingContext {
  requestHash: string;
  subject: string;
  /** A session client, with no caller-owned transaction open. */
  connection?: PoolClient;
}

interface ReservationRow {
  signature_id: string;
  operation_hash: string;
  signed_evidence: Hex | null;
  tx_hash: string | null;
  user_id: string;
  address: string;
  to_address: string | null;
  value_wei: string;
  chain_id: number | null;
  principal_did: string | null;
  expired: boolean;
}

async function lockAccounting(client: PoolClient, userId: string): Promise<void> {
  await client.query('select pg_advisory_xact_lock(hashtextextended($1, 0))', [
    `wallet-signatures:${userId}`,
  ]);
}

function matchesAccounting(row: ReservationRow, input: SignatureLogInput): boolean {
  return (
    row.user_id === input.user_id &&
    row.address.toLowerCase() === input.address.toLowerCase() &&
    (row.to_address?.toLowerCase() ?? null) === (input.to_address?.toLowerCase() ?? null) &&
    BigInt(row.value_wei) === (input.value_wei ?? 0n) &&
    row.chain_id === (input.chain_id ?? null) &&
    row.principal_did === (input.principal_did ?? null)
  );
}

const RESERVATION_PROJECTION = `r.signature_id::text, r.operation_hash, r.signed_evidence, r.tx_hash,
  s.user_id, s.address, s.to_address, s.value_wei::text, s.chain_id, s.principal_did,
  s.created_at < now() - interval '24 hours' as expired`;

async function reserveSignature(
  input: SignatureLogInput,
  operationHash: string,
  connection?: PoolClient
): Promise<ReservationRow> {
  return withTransaction(async (client) => {
    await lockAccounting(client, input.user_id);
    const existing = await client.query<ReservationRow>(
      `select ${RESERVATION_PROJECTION}
      from wallet_signature_reservations r join wallet_signatures s on s.id = r.signature_id
      where r.wallet_id = $1 and r.kind = $2
        and (r.request_hash = $3 or ($2 = 'transaction' and r.operation_hash = $4))
      for update of r`,
      [input.wallet_id, input.kind, input.request_hash, operationHash]
    );
    if (existing.rows.length > 1)
      throw new Error('signature accounting request conflicts with another transaction');
    const row = existing.rows[0];
    if (row) {
      if (row.operation_hash !== operationHash || !matchesAccounting(row, input)) {
        throw new Error('signature accounting reservation cannot be retargeted');
      }
      if (row.expired && row.signed_evidence === null)
        throw new Error('unresolved signature accounting reservation expired');
      return row;
    }
    // Signing callers can hold a SHARE lock on wallets. A reservation must commit
    // independently without the last-used trigger trying to upgrade that lock.
    await client.query("select set_config('wallet.accounting_reservation', 'on', true)");
    const inserted = await client.query<{ id: string }>(
      `insert into wallet_signatures
      (user_id, wallet_id, address, kind, to_address, value_wei, chain_id, request_hash, principal_did)
      values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id::text`,
      [
        input.user_id,
        input.wallet_id,
        input.address,
        input.kind,
        input.to_address ?? null,
        (input.value_wei ?? 0n).toString(),
        input.chain_id ?? null,
        input.request_hash,
        input.principal_did ?? null,
      ]
    );
    const id = inserted.rows[0]?.id;
    if (!id) throw new Error('signature accounting reservation returned no row');
    await client.query(
      `insert into wallet_signature_reservations
      (wallet_id, kind, request_hash, operation_hash, signature_id) values ($1, $2, $3, $4, $5)`,
      [input.wallet_id, input.kind, input.request_hash, operationHash, id]
    );
    return {
      signature_id: id,
      operation_hash: operationHash,
      signed_evidence: null,
      tx_hash: null,
      user_id: input.user_id,
      address: input.address,
      to_address: input.to_address ?? null,
      value_wei: (input.value_wei ?? 0n).toString(),
      chain_id: input.chain_id ?? null,
      principal_did: input.principal_did ?? null,
      expired: false,
    };
  }, connection);
}

async function accountedSignature(
  input: SignatureLogInput,
  operationHash: string,
  sign: () => Promise<Hex>,
  connection?: PoolClient
): Promise<Hex> {
  // This transaction commits before invoking either custody backend. A failure
  // after this point leaves a counted reservation, including across a restart.
  const reservation = await reserveSignature(input, operationHash, connection);
  if (reservation.signed_evidence !== null) return reservation.signed_evidence;
  const evidence = await sign();
  const txHash = input.kind === 'transaction' ? keccak256(evidence) : null;
  await withTransaction(async (client) => {
    const result = await client.query(
      `update wallet_signature_reservations
      set signed_evidence = $2, tx_hash = $3 where signature_id = $1
      and (signed_evidence is null or signed_evidence = $2) returning signature_id`,
      [reservation.signature_id, evidence, txHash]
    );
    if (result.rowCount !== 1)
      throw new Error('signature accounting completion differs from retained evidence');
    await client.query('update wallet_signatures set tx_hash = $2 where id = $1', [
      reservation.signature_id,
      txHash,
    ]);
  }, connection);
  return evidence;
}

export function withSignatureAccounting(
  wallet: WalletRow,
  account: SigningAccount,
  context?: SignatureAccountingContext
): SigningAccount {
  const input = async (kind: SignatureLogInput['kind']): Promise<SignatureLogInput> => {
    if (!context?.requestHash || !context.subject)
      throw new Error('durable signature accounting requires request context');
    let principalDid: string | null = null;
    if (wallet.kind === 'agent') {
      const read = context.connection ? context.connection.query.bind(context.connection) : query;
      const { rows } = await read<{ did: string; owner_user_id: string | null }>(
        'select did, owner_user_id from agent_principals where wallet_id = $1',
        [wallet.id]
      );
      const principal = rows[0];
      if (
        rows.length !== 1 ||
        !principal ||
        agentWalletUserId(principal.did) !== wallet.user_id ||
        (context.subject !== principal.did && context.subject !== principal.owner_user_id)
      ) {
        throw new Error('signature accounting subject differs from wallet owner');
      }
      principalDid = principal.did;
    } else if (context.subject !== wallet.user_id) {
      throw new Error('signature accounting subject differs from wallet owner');
    }
    return {
      user_id: wallet.user_id,
      wallet_id: wallet.id,
      address: wallet.address,
      kind,
      request_hash: context.requestHash,
      principal_did: principalDid,
    };
  };
  return {
    address: account.address,
    signTransaction: (async (transaction: TransactionSerializable) =>
      accountedSignature(
        {
          ...(await input('transaction')),
          to_address: transaction.to ?? null,
          value_wei: transaction.value ?? 0n,
          chain_id: transaction.chainId ?? wallet.chain_id,
        },
        keccak256(serializeTransaction(transaction)),
        () => account.signTransaction(transaction),
        context?.connection
      )) as SigningAccount['signTransaction'],
    signMessage: async (args) =>
      accountedSignature(
        await input('message'),
        hashMessage(args.message),
        () => account.signMessage(args),
        context?.connection
      ),
    signTypedData: (async (typedData: TypedDataDefinition) =>
      accountedSignature(
        await input('typed_data'),
        hashTypedData(typedData),
        () => account.signTypedData(typedData),
        context?.connection
      )) as SigningAccount['signTypedData'],
  };
}

export async function reconcileSignature(input: SignatureLogInput): Promise<void> {
  await withTransaction(async (client) => {
    await lockAccounting(client, input.user_id);
    const result = await client.query<ReservationRow>(
      `select ${RESERVATION_PROJECTION}
      from wallet_signature_reservations r join wallet_signatures s on s.id = r.signature_id
      where r.wallet_id = $1 and r.kind = $2
        and (r.request_hash = $3 or ($4::text is not null and r.tx_hash = $4)) for update of r`,
      [input.wallet_id, input.kind, input.request_hash, input.tx_hash?.toLowerCase() ?? null]
    );
    const row = result.rows[0];
    if (
      result.rows.length !== 1 ||
      !row ||
      row.signed_evidence === null ||
      !matchesAccounting(row, input) ||
      (input.tx_hash != null && row.tx_hash !== input.tx_hash.toLowerCase())
    ) {
      throw new Error('signature accounting completion has no matching durable signature');
    }
    await client.query('update wallet_signatures set ip = $2, user_agent = $3 where id = $1', [
      row.signature_id,
      input.ip ?? null,
      input.user_agent ?? null,
    ]);
    await client.query('update wallets set last_used_at = now() where id = $1', [input.wallet_id]);
  });
}
