import { createPrivateKey, sign } from 'node:crypto';
import { closeSync, constants, fstatSync, openSync, readFileSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import type { Pool } from 'pg';
import { bytesToHex, getAddress, type Hex } from 'viem';
import { NonceConflictError, RpcPool, TransactionRejectedError } from '../rpc/pool.js';
import { NonceStore, broadcastReserved } from '../nonce/store.js';
import {
  WalletArchivedError,
  agentWalletUserId,
  findArchivedWallet,
  type WalletKind,
} from '../db/wallets.js';
import {
  AttestorDaemonClient,
  bindMessage,
  broadcastOnce,
  didFromPublicKey,
  mainAccountId,
  normalisePublicKey,
  readBindNonce,
  readUnifiedAccount,
  signBindWithAttestors,
  transactionHash,
  verifyEd25519,
  waitForReceipt,
  sponsoredBindRequest,
  provisioningStationConfig,
  type SponsoredBindRequest,
  type SignBody,
} from './bind.js';

import { CustodyAuthorityError, publishCustodyAuthority } from '../agent/authority.js';
import type { AgentOriginalRequest, AgentReauthorization, Authorisation } from '../attestor/client.js';
import { GasStationClient as StationClient, StationAdapterError } from '../sponsor/station.js';

export type GasStationClient = StationClient;

export type ProvisionState =
  | 'new'
  | 'evm_key'
  | 'identity'
  | 'bind_signed'
  | 'funded'
  | 'bind_sent'
  | 'active'
  | 'refused';

export type ProvisionSubject =
  | { kind: 'standard'; userId: string; token: string | null }
  | { kind: 'agent'; did: string; agentSignature?: string | null; custody?: { origin: AgentOriginalRequest; reauthorization?: AgentReauthorization } };

export interface ProvisionDeps {
  pool: Pool;
  rpc: RpcPool;
  attestors: AttestorDaemonClient | null;
  nonces: NonceStore;
  station: GasStationClient | null;
  chainId: number;
  gasCapWei: bigint;
  receiptTimeoutMs: number;
  receiptPollMs: number;
  identityBinding?: { issuer: string; tenant: string; privateKeyFile: string };
}

export interface ProvisionResult {
  state: ProvisionState;
  awaiting: 'owner' | 'agent_signature' | null;
  walletId: string;
  address: `0x${string}`;
  did: string | null;
  mainAccountId: string | null;
  bindMessage: Hex | null;
}

export class ProvisionError extends Error {
  readonly code: string;
  readonly status: number;
  constructor(code: string, status: number, message: string) {
    super(message);
    this.name = 'ProvisionError';
    this.code = code;
    this.status = status;
  }
}

export class ProvisionRefusedError extends ProvisionError {
  readonly boundDid: string | null;
  constructor(reason: string, boundDid: string | null) {
    super('binding_refused', 409, reason);
    this.name = 'ProvisionRefusedError';
    this.boundDid = boundDid;
  }
}

interface ProvisionRow {
  id: string;
  user_id: string;
  kind: WalletKind;
  wallet_id: string | null;
  state: ProvisionState;
  evm_key_generation: number;
  ed_key_generation: number;
  evm_key_id: string | null;
  ed_key_id: string | null;
  ed_public_key: string | null;
  did: string | null;
  main_account_id: string | null;
  bind_nonce: string | null;
  bind_signature: string | null;
  topup_raw_tx: Hex | null;
  topup_tx_hash: Hex | null;
  topup_value_wei: string | null;
  bind_gas: string | null;
  bind_max_fee_wei: string | null;
  bind_raw_tx: Hex | null;
  bind_tx_hash: Hex | null;
  station_quote_nonce: string | null;
  station_batch_nonce: string | null;
  station_request: SponsoredBindRequest | null;
  refusal_reason: string | null;
}

interface WalletRecord {
  id: string;
  user_id: string;
  address: `0x${string}`;
  encrypted_private_key: string | null;
  key_version: number;
  chain_id: number;
  kind: WalletKind;
  created_at: string;
  last_used_at: string | null;
  is_disabled: boolean;
  disabled_reason: string | null;
  migrated_at: string | null;
  attestor_key_id: string | null;
}

const WALLET_COLUMNS = `id, user_id, address, encrypted_private_key, key_version, chain_id, kind,
  created_at, last_used_at, is_disabled, disabled_reason, migrated_at, attestor_key_id`;

async function loadWallet(pool: Pool, userId: string, kind: WalletKind): Promise<WalletRecord | null> {
  const { rows } = await pool.query<WalletRecord>(
    `select ${WALLET_COLUMNS} from wallets where user_id = $1 and kind = $2 and archived_at is null limit 1`,
    [userId, kind],
  );
  return rows[0] ?? null;
}

async function loadWalletById(pool: Pool, id: string): Promise<WalletRecord> {
  const { rows } = await pool.query<WalletRecord>(`select ${WALLET_COLUMNS} from wallets where id = $1`, [id]);
  if (!rows[0]) throw new ProvisionError('wallet_missing', 500, `wallet ${id} vanished during provisioning`);
  return rows[0];
}

async function updateRow(pool: Pool, id: string, fields: Partial<Record<keyof ProvisionRow, unknown>>): Promise<void> {
  const keys = Object.keys(fields);
  const sets = keys.map((k, i) => `${k} = $${i + 2}`).join(', ');
  await pool.query(`update account_provisioning set ${sets}, updated_at = now() where id = $1`, [
    id,
    ...keys.map((k) => fields[k as keyof ProvisionRow]),
  ]);
}

/**
 * Reserve the signer's nonce, persist the signed setup transaction and broadcast it under the
 * signer's wallet lock through the helper the send routes use, so the hash and accepting node are
 * recorded on the nonce. A send no node accepted and no node knows frees the nonce and clears the
 * persisted transaction so the next attempt signs again; a nonce another transaction holds stays held
 * and is refused for a fresh prepare; a send with an unknown outcome keeps the transaction for the
 * next attempt to rebroadcast and is thrown after persist has recorded it.
 */
export async function sendSetupTransaction(
  deps: Pick<ProvisionDeps, 'nonces' | 'rpc'>,
  from: string,
  sign: (nonce: number) => Promise<Hex>,
  persist: (raw: Hex | null) => Promise<void>,
): Promise<{ raw: Hex; hash: Hex; error: unknown }> {
  const outcome = await deps.nonces.withLock(from, async (lease) => {
    const nonce = await lease.next();
    const raw = await sign(nonce);
    const hash = transactionHash(raw);
    await persist(raw);
    const sent = await broadcastReserved(lease, nonce, raw, (r) => deps.rpc.sendRawTransactionVia(r));
    if (sent.error === null) return { kind: 'sent' as const, raw, hash, error: null };
    if (!(sent.error instanceof TransactionRejectedError) && !(sent.error instanceof NonceConflictError)) {
      return { kind: 'kept' as const, raw, hash, error: sent.error };
    }
    let known: boolean;
    try {
      known = await deps.rpc.transactionKnown(hash, null);
    } catch {
      await lease.track(nonce, hash, null);
      return { kind: 'kept' as const, raw, hash, error: sent.error };
    }
    if (known) {
      await lease.track(nonce, hash, null);
      return { kind: 'sent' as const, raw, hash, error: null };
    }
    await persist(null);
    return { kind: 'refused' as const, raw, hash, error: sent.error };
  });
  if (outcome.kind === 'refused') throw setupRefusal(outcome.error);
  return outcome;
}

/** Rebroadcast a persisted setup transaction; one no node accepts or knows is cleared for a fresh sign. */
async function rebroadcastSetup(deps: Pick<ProvisionDeps, 'rpc'>, raw: Hex, clear: () => Promise<void>): Promise<Hex> {
  try {
    return await broadcastOnce(deps.rpc, raw);
  } catch (err) {
    if (!(err instanceof TransactionRejectedError) && !(err instanceof NonceConflictError)) throw err;
    if (await deps.rpc.transactionKnown(transactionHash(raw), null)) return transactionHash(raw);
    await clear();
    throw setupRefusal(err);
  }
}

function setupRefusal(err: unknown): ProvisionError {
  const detail = err instanceof Error ? err.message : String(err);
  return err instanceof NonceConflictError
    ? new ProvisionError('nonce_conflict', 409, `the setup transaction nonce is already used, prepare again: ${detail}`)
    : new ProvisionError('setup_send_rejected', 502, `the node rejected the setup transaction: ${detail}`);
}

async function loadRow(pool: Pool, userId: string, kind: WalletKind): Promise<ProvisionRow> {
  await pool.query(
    `insert into account_provisioning (user_id, kind) values ($1, $2) on conflict (user_id, kind) do nothing`,
    [userId, kind],
  );
  const { rows } = await pool.query<ProvisionRow>(
    `select id::text, user_id, kind, wallet_id, state, evm_key_generation, ed_key_generation, evm_key_id, ed_key_id,
            ed_public_key, did, main_account_id, bind_nonce::text, bind_signature, topup_raw_tx, topup_tx_hash,
            topup_value_wei::text, bind_gas::text, bind_max_fee_wei::text, bind_raw_tx, bind_tx_hash, refusal_reason,
            station_quote_nonce::text, station_batch_nonce::text, station_request
       from account_provisioning where user_id = $1 and kind = $2`,
    [userId, kind],
  );
  return rows[0]!;
}

interface AgentPrincipal {
  did: string;
  public_key: string;
  wallet_id: string | null;
  is_frozen: boolean;
}

async function loadPrincipal(pool: Pool, did: string): Promise<AgentPrincipal> {
  const { rows } = await pool.query<AgentPrincipal>(
    `select did, public_key, wallet_id, is_frozen from agent_principals where did = $1`,
    [did],
  );
  if (!rows[0]) throw new ProvisionError('agent_unknown', 404, 'agent is not registered');
  if (rows[0].is_frozen) throw new ProvisionError('agent_frozen', 403, 'agent is frozen');
  return rows[0];
}

async function recordAudit(
  pool: Pool,
  entry: { walletId: string | null; address: string; event: 'topup' | 'bind' | 'refusal' | 'backfill'; txHash?: string | null; valueWei?: bigint | null; gas?: bigint | null; maxFeeWei?: bigint | null; outcome?: string | null; reason?: string | null },
): Promise<void> {
  await pool.query(
    `insert into account_setup_audit (wallet_id, address, event, tx_hash, value_wei, gas, max_fee_wei, outcome, reason)
     select $1, $2, $3, $4, $5, $6, $7, $8, $9
      where $4::text is null
         or not exists (select 1 from account_setup_audit where event = $3 and tx_hash = $4::text)
     on conflict do nothing`,
    [
      entry.walletId,
      entry.address.toLowerCase(),
      entry.event,
      entry.txHash ?? null,
      entry.valueWei?.toString() ?? null,
      entry.gas?.toString() ?? null,
      entry.maxFeeWei?.toString() ?? null,
      entry.outcome ?? null,
      entry.reason ?? null,
    ],
  );
}

async function lockSubject(pool: Pool, userId: string, kind: WalletKind): Promise<() => Promise<void>> {
  const client = await pool.connect();
  try {
    await client.query(`select pg_advisory_lock(hashtextextended($1, 0))`, [`provision:${userId}:${kind}`]);
  } catch (err) {
    client.release();
    throw err;
  }
  return async () => {
    try {
      await client.query(`select pg_advisory_unlock(hashtextextended($1, 0))`, [`provision:${userId}:${kind}`]);
    } finally {
      client.release();
    }
  };
}

function requireAttestors(deps: ProvisionDeps): AttestorDaemonClient {
  if (!deps.attestors) throw new ProvisionError('attestors_unconfigured', 503, 'no attestor endpoints are configured');
  return deps.attestors;
}

async function fees(rpc: RpcPool): Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint }> {
  const [block, tip] = await Promise.all([
    rpc.request<{ baseFeePerGas?: Hex } | null>('eth_getBlockByNumber', ['latest', false]),
    rpc.request<Hex>('eth_maxPriorityFeePerGas', []),
  ]);
  if (!block?.baseFeePerGas) throw new ProvisionError('chain_unavailable', 503, 'latest block carries no base fee');
  const priority = BigInt(tip);
  return { maxFeePerGas: BigInt(block.baseFeePerGas) + priority, maxPriorityFeePerGas: priority };
}

function result(row: ProvisionRow, wallet: WalletRecord, awaiting: ProvisionResult['awaiting'] = null, message: Hex | null = null): ProvisionResult {
  return {
    state: row.state,
    awaiting,
    walletId: wallet.id,
    address: getAddress(wallet.address),
    did: row.did,
    mainAccountId: row.main_account_id,
    bindMessage: message,
  };
}

async function refuse(deps: ProvisionDeps, row: ProvisionRow, wallet: WalletRecord, reason: string, boundDid: string | null): Promise<never> {
  await updateRow(deps.pool, row.id, { state: 'refused', refusal_reason: reason });
  await deps.pool.query(`update wallets set binding_state = 'refused' where id = $1`, [wallet.id]);
  await recordAudit(deps.pool, { walletId: wallet.id, address: wallet.address, event: 'refusal', reason, outcome: boundDid });
  row.state = 'refused';
  row.refusal_reason = reason;
  throw new ProvisionRefusedError(reason, boundDid);
}

async function activate(deps: ProvisionDeps, row: ProvisionRow, wallet: WalletRecord): Promise<void> {
  await updateRow(deps.pool, row.id, { state: 'active' });
  await deps.pool.query(`update wallets set binding_state = 'bound' where id = $1`, [wallet.id]);
  row.state = 'active';
}

async function createEvmKey(deps: ProvisionDeps, row: ProvisionRow, userId: string, kind: WalletKind, owner = userId): Promise<WalletRecord> {
  const archived = await findArchivedWallet({ userId, kind });
  if (archived) throw new WalletArchivedError(archived.id, archived.address);
  const attestors = requireAttestors(deps);
  const keyId = `wallet:${userId}:${kind}:secp256k1:${row.evm_key_generation}`;
  await updateRow(deps.pool, row.id, { evm_key_id: keyId });
  row.evm_key_id = keyId;
  let key;
  try {
    key = await attestors.generate(keyId, 'secp256k1', owner);
  } catch (err) {
    if (err instanceof CustodyAuthorityError) throw new ProvisionError('custody_inventory_pending', 503, `approved inventory is required for ${keyId}`);
    throw err;
  }
  const { rows } = await deps.pool.query<WalletRecord>(
    `insert into wallets (user_id, address, encrypted_private_key, key_version, chain_id, kind, migrated_at, attestor_key_id, binding_state)
     values ($1, $2, null, 1, $3, $4, now(), $5, 'pending')
     returning ${WALLET_COLUMNS}`,
    [userId, key.address, deps.chainId, kind, keyId],
  );
  const wallet = rows[0]!;
  await updateRow(deps.pool, row.id, { state: 'evm_key', wallet_id: wallet.id, evm_key_id: keyId });
  Object.assign(row, { state: 'evm_key', wallet_id: wallet.id, evm_key_id: keyId });
  return wallet;
}

async function storeIdentity(deps: ProvisionDeps, row: ProvisionRow, wallet: WalletRecord, publicKey: string, edKeyId: string | null): Promise<void> {
  const did = didFromPublicKey(publicKey);
  const main = mainAccountId(did);
  await deps.pool.query(
    `update wallets set did = $2, main_account_id = $3, layerx_key_id = $4, binding_state = 'pending' where id = $1`,
    [wallet.id, did, main, edKeyId],
  );
  await updateRow(deps.pool, row.id, { state: 'identity', ed_key_id: edKeyId, ed_public_key: publicKey, did, main_account_id: main });
  Object.assign(row, { state: 'identity', ed_key_id: edKeyId, ed_public_key: publicKey, did, main_account_id: main });
}

async function signStationPayload(
  deps: ProvisionDeps, subject: ProvisionSubject, wallet: WalletRecord, payload: SignBody,
) {
  if (!wallet.attestor_key_id) throw new ProvisionError('wallet_key_missing', 503, 'sponsored provisioning requires attestor custody');
  let authorisation: Authorisation;
  if (subject.kind === 'standard') {
    if (!subject.token) throw new ProvisionError('owner_required', 401, 'the owner must authorise the sponsored binding');
    authorisation = { scheme: 'supabase_jwt', token: subject.token };
  } else {
    if (!subject.custody || subject.custody.origin.did !== subject.did)
      throw new ProvisionError('agent_request_required', 401, 'binding requires the original signed agent request');
    const principal = await loadPrincipal(deps.pool, subject.did);
    authorisation = { scheme: 'agent_request', publicKey: principal.public_key, ...subject.custody };
  }
  return requireAttestors(deps).signAuthorized(wallet.attestor_key_id, payload, authorisation);
}

export async function provisionAccount(deps: ProvisionDeps, subject: ProvisionSubject): Promise<ProvisionResult> {
  if (!deps.station) {
    throw new ProvisionError('gas_station_unconfigured', 503, 'no gas station is configured');
  }
  try {
    deps.station.requireConfiguredUrl();
  } catch (err) {
    if (err instanceof StationAdapterError) throw new ProvisionError(err.code, err.status, err.message);
    throw err;
  }
  let userId: string;
  let kind: WalletKind;
  let principal: AgentPrincipal | null = null;
  if (subject.kind === 'standard') {
    userId = subject.userId;
    kind = 'standard';
  } else {
    principal = await loadPrincipal(deps.pool, subject.did);
    userId = agentWalletUserId(subject.did);
    kind = 'agent';
  }
  const unlock = await lockSubject(deps.pool, userId, kind);
  try {
    const row = await loadRow(deps.pool, userId, kind);
    if (row.state === 'refused') {
      throw new ProvisionRefusedError(row.refusal_reason ?? 'binding refused', null);
    }
    let wallet: WalletRecord | null = row.wallet_id ? await loadWalletById(deps.pool, row.wallet_id) : null;

    if (row.state === 'new') {
      const existing = wallet ?? (await loadWallet(deps.pool, userId, kind));
      if (existing) {
        wallet = existing;
        await updateRow(deps.pool, row.id, { state: 'evm_key', wallet_id: existing.id, evm_key_id: existing.attestor_key_id });
        Object.assign(row, { state: 'evm_key', wallet_id: existing.id, evm_key_id: existing.attestor_key_id });
      } else {
        if (subject.kind === 'agent' && !subject.custody) {
          throw new ProvisionError('agent_request_required', 401, 'provisioning requires the original signed agent request');
        }
        wallet = await createEvmKey(deps, row, userId, kind, principal ? `agent:${normalisePublicKey(principal.public_key)}` : userId);
      }
    }
    if (principal && wallet) {
      await deps.pool.query('update agent_principals set wallet_id = $2 where did = $1', [subject.kind === 'agent' ? subject.did : '', wallet.id]);
      await publishCustodyAuthority();
    }
    if (!wallet) throw new ProvisionError('wallet_missing', 500, 'provisioning row has no wallet');
    if (wallet.is_disabled) throw new ProvisionError('wallet_disabled', 403, wallet.disabled_reason ?? 'wallet is disabled');
    const address = getAddress(wallet.address);

    if (row.state === 'evm_key') {
      if (principal) {
        await storeIdentity(deps, row, wallet, normalisePublicKey(principal.public_key), null);
      } else {
        const attestors = requireAttestors(deps);
        const keyId = `account:${userId}:${kind}:ed25519:${row.ed_key_generation}`;
        let key;
        try {
          key = await attestors.generate(keyId, 'ed25519', userId, address);
        } catch (err) {
          if (err instanceof CustodyAuthorityError) throw new ProvisionError('custody_inventory_pending', 503, `approved inventory is required for ${keyId}`);
          throw err;
        }
        await storeIdentity(deps, row, wallet, key.publicKey, keyId);
      }
    }

    if (row.state === 'identity') {
      const bound = await readUnifiedAccount(deps.rpc, address);
      if (bound.didPublicKey) {
        if (bound.didPublicKey === row.ed_public_key) {
          await activate(deps, row, wallet);
          return result(row, wallet);
        }
        return await refuse(deps, row, wallet, 'address is already bound to a different DID', didFromPublicKey(bound.didPublicKey));
      }
      const nonce = await readBindNonce(deps.rpc, address);
      const message = bindMessage(deps.chainId, address, nonce);
      let signature: string;
      if (principal) {
        const offered = subject.kind === 'agent' ? subject.agentSignature : null;
        if (!offered) return result(row, wallet, 'agent_signature', bytesToHex(message));
        if (!verifyEd25519(row.ed_public_key!, message, offered)) {
          throw new ProvisionError('agent_signature_invalid', 403, 'binding signature does not verify under the registered agent key');
        }
        signature = offered.replace(/^0x/, '').toLowerCase();
      } else {
        if (subject.kind !== 'standard' || !subject.token) return result(row, wallet, 'owner', bytesToHex(message));
        signature = await signBindWithAttestors(requireAttestors(deps), row.ed_key_id!, row.ed_public_key!, message, subject.token);
      }
      await updateRow(deps.pool, row.id, { state: 'bind_signed', bind_nonce: nonce.toString(), bind_signature: signature });
      Object.assign(row, { state: 'bind_signed', bind_nonce: nonce.toString(), bind_signature: signature });
    }

    if (row.state === 'bind_signed' || row.state === 'funded') {
      if (subject.kind === 'standard' && !subject.token) return result(row, wallet, 'owner');
      let requoted = false;
      for (;;) {
        if (!row.station_request) {
          const config = provisioningStationConfig();
          const f = await fees(deps.rpc);
          const gasCost = config.gasLimit * f.maxFeePerGas;
          if (gasCost > deps.gasCapWei)
            throw new ProvisionError('setup_gas_over_cap', 503, `binding needs ${gasCost} wei, above the setup gas cap ${deps.gasCapWei}`);
          const signerWallet = wallet;
          const request = await sponsoredBindRequest({
            station: deps.station, rpc: deps.rpc, chainId: deps.chainId, address,
            publicKey: row.ed_public_key!, signature: row.bind_signature!, gasCost,
            paymaster: config.paymaster, maxTokenAmount: config.maxTokenAmount,
            sign: payload => signStationPayload(deps, subject, signerWallet, payload),
          });
          const fields = { station_request: request, station_quote_nonce: request.batch.quote.quoteNonce,
            station_batch_nonce: request.batch.nonce, bind_gas: config.gasLimit.toString(), bind_max_fee_wei: f.maxFeePerGas.toString() };
          await updateRow(deps.pool, row.id, fields);
          Object.assign(row, fields);
        }
        let submitted: Record<string, unknown>;
        try {
          submitted = await deps.station.submit(row.station_request);
        } catch (err) {
          if (!(err instanceof StationAdapterError) || err.code !== 'expired_quote') throw err;
          const bound = await readUnifiedAccount(deps.rpc, address);
          if (bound.didPublicKey === row.ed_public_key) {
            await activate(deps, row, wallet);
            return result(row, wallet);
          }
          if (bound.didPublicKey) {
            return await refuse(deps, row, wallet, 'address is already bound to a different DID', didFromPublicKey(bound.didPublicKey));
          }
          if (requoted) throw err;
          const fields = { station_request: null, station_quote_nonce: null, station_batch_nonce: null,
            bind_gas: null, bind_max_fee_wei: null };
          await updateRow(deps.pool, row.id, fields);
          Object.assign(row, fields);
          requoted = true;
          continue;
        }
        if (typeof submitted.transactionHash !== 'string' || !/^0x[0-9a-fA-F]{64}$/.test(submitted.transactionHash))
          throw new StationAdapterError(502, 'gas_station_invalid_response');
        const fields = { state: 'bind_sent' as const, bind_tx_hash: submitted.transactionHash as Hex };
        await updateRow(deps.pool, row.id, fields);
        Object.assign(row, fields);
        break;
      }
    }

    if (row.state === 'bind_sent') {
      if (!row.bind_tx_hash) throw new ProvisionError('bind_transaction_missing', 503, 'binding submission has no transaction hash');
      const hash = row.bind_tx_hash;
      const receipt = await waitForReceipt(deps.rpc, hash, deps.receiptTimeoutMs, deps.receiptPollMs);
      await recordAudit(deps.pool, {
        walletId: wallet.id,
        address,
        event: 'bind',
        txHash: hash,
        gas: receipt.gasUsed,
        maxFeeWei: BigInt(row.bind_max_fee_wei!),
        outcome: receipt.status,
      });
      const bound = await readUnifiedAccount(deps.rpc, address);
      if (bound.didPublicKey === row.ed_public_key) {
        await activate(deps, row, wallet);
      } else if (bound.didPublicKey) {
        return await refuse(deps, row, wallet, 'address is already bound to a different DID', didFromPublicKey(bound.didPublicKey));
      } else {
        return await refuse(deps, row, wallet, `binding transaction ${receipt.status} without binding the address`, null);
      }
    }

    return result(row, wallet);
  } catch (err) {
    if (err instanceof StationAdapterError) throw new ProvisionError(err.code, err.status, err.message);
    throw err;
  } finally {
    await unlock();
  }
}

export async function produceIdentityBinding(
  deps: ProvisionDeps,
  userId: string,
  provision: ProvisionResult,
): Promise<string | null> {
  const config = deps.identityBinding;
  if (!config || provision.state !== 'active') return null;
  if (!provision.did || !/^did:layerx:[0-9a-f]{64}$/.test(provision.did)
    || provision.did === `did:layerx:${'0'.repeat(64)}`) {
    throw new ProvisionError('identity_binding_unavailable', 503, 'active wallet has no valid LayerX identity');
  }
  const { rows } = await deps.pool.query<{ ed_public_key: string }>(
    `select p.ed_public_key from account_provisioning p join wallets w on w.id = p.wallet_id
      where p.user_id = $1 and p.kind = 'standard' and p.wallet_id = $2
        and p.state = 'active' and p.did = $3 and w.user_id = $1 and w.kind = 'standard'
        and w.did = p.did and w.binding_state = 'bound' and w.archived_at is null
        and w.is_disabled = false and lower(w.address) = lower($4)`,
    [userId, provision.walletId, provision.did, provision.address],
  );
  const publicKey = rows[0]?.ed_public_key;
  if (rows.length !== 1 || !publicKey || didFromPublicKey(publicKey) !== provision.did) {
    throw new ProvisionError('identity_binding_refused', 409, 'wallet identity is not active for this user');
  }
  const bound = await readUnifiedAccount(deps.rpc, provision.address);
  if (bound.didPublicKey !== publicKey) {
    throw new ProvisionError('identity_binding_refused', 409, 'wallet identity differs from its on-chain binding');
  }
  let descriptor: number | undefined;
  try {
    if (!isAbsolute(config.privateKeyFile)) throw new Error('invalid key path');
    descriptor = openSync(config.privateKeyFile, constants.O_RDONLY | constants.O_NOFOLLOW);
    const metadata = fstatSync(descriptor);
    if (!metadata.isFile() || metadata.nlink !== 1 || (metadata.mode & 0o777) !== 0o600
      || metadata.uid !== process.getuid?.() || metadata.size < 1 || metadata.size > 16_384) {
      throw new Error('invalid key file');
    }
    const key = createPrivateKey(readFileSync(descriptor));
    if (key.asymmetricKeyType !== 'ec' || key.asymmetricKeyDetails?.namedCurve !== 'prime256v1') {
      throw new Error('invalid producer key');
    }
    const header = Buffer.from(JSON.stringify({ alg: 'ES256', typ: 'layerx-wallet-binding+jwt' })).toString('base64url');
    const payload = Buffer.from(JSON.stringify({
      iss: config.issuer,
      sub: userId,
      did: provision.did,
      tenant: config.tenant,
    })).toString('base64url');
    const input = `${header}.${payload}`;
    if (input.length + 87 > 4096) throw new Error('identity binding exceeds protocol bound');
    const signature = sign('sha256', Buffer.from(input, 'ascii'), { key, dsaEncoding: 'ieee-p1363' });
    if (signature.length !== 64) throw new Error('invalid producer signature');
    return `${input}.${signature.toString('base64url')}`;
  } catch {
    throw new ProvisionError('identity_binding_unavailable', 503, 'wallet identity binding signer is unavailable');
  } finally {
    if (descriptor !== undefined) closeSync(descriptor);
  }
}
