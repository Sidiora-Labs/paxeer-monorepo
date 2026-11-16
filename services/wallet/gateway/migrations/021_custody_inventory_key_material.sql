alter table account_provisioning
  add column if not exists evm_public_key text,
  add column if not exists evm_key_epoch integer,
  add column if not exists evm_key_owner text,
  add column if not exists ed_key_epoch integer,
  add column if not exists ed_key_owner text;

alter table account_provisioning
  drop constraint if exists account_provisioning_key_epoch_chk;

alter table account_provisioning
  add constraint account_provisioning_key_epoch_chk
  check ((evm_key_epoch is null or evm_key_epoch >= 0)
     and (ed_key_epoch is null or ed_key_epoch >= 0));
