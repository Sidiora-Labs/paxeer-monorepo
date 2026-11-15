import type { PoolClient } from 'pg';
import { query } from './pool.js';

export interface InventoryProvisioningRow {
  id: string;
  user_id: string;
  kind: string;
  created_at: string;
  evm_key_id: string | null;
  evm_public_key: string | null;
  evm_key_epoch: number | null;
  evm_key_owner: string | null;
  ed_key_id: string | null;
  ed_public_key: string | null;
  ed_key_epoch: number | null;
  ed_key_owner: string | null;
}

export async function loadInventoryProvisioning(): Promise<InventoryProvisioningRow[]> {
  const { rows } = await query<InventoryProvisioningRow>(
    `select id::text, user_id::text, kind, created_at,
            evm_key_id, evm_public_key, evm_key_epoch, evm_key_owner,
            ed_key_id, ed_public_key, ed_key_epoch, ed_key_owner
       from account_provisioning
      where evm_key_id is not null or ed_key_id is not null
      order by created_at asc, id asc`
  );
  return rows;
}

export interface RetainedWalletInventory {
  inventory_sequence: string;
  inventory_token: string | null;
}

export async function loadRetainedWalletInventory(
  client?: PoolClient
): Promise<RetainedWalletInventory> {
  const statement =
    'select inventory_sequence::text, inventory_token from wallet_custody_authority where singleton = true';
  const { rows } = client
    ? await client.query<RetainedWalletInventory>(statement)
    : await query<RetainedWalletInventory>(statement);
  if (!rows[0]) throw new Error('wallet custody inventory state is unavailable');
  return rows[0];
}

export async function retainWalletInventory(
  client: PoolClient,
  token: string,
  sequence: string
): Promise<void> {
  const result = await client.query(
    `update wallet_custody_authority set inventory_sequence = $1, inventory_token = $2, updated_at = now()
       where singleton = true and (inventory_sequence < $1 or
         (inventory_sequence = $1 and (inventory_token is null or inventory_token = $2)))`,
    [sequence, token]
  );
  if (result.rowCount !== 1) throw new Error('wallet custody inventory sequence conflicts');
}
