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
