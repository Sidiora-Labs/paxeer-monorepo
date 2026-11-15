alter table wallet_custody_authority
  add column inventory_token text check (octet_length(inventory_token) <= 2097152);
