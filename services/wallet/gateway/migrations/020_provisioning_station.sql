alter table account_provisioning
  add column if not exists station_quote_nonce numeric(78, 0),
  add column if not exists station_batch_nonce numeric(78, 0),
  add column if not exists station_request jsonb;
