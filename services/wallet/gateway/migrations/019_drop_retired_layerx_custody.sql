alter table agent_actions
  add constraint agent_actions_no_layerx_deposit
  check (kind <> 'layerx_deposit') not valid;

delete from agent_actions
where kind = 'layerx_deposit' and terminal_at is not null;

alter table agent_actions
  validate constraint agent_actions_no_layerx_deposit;

drop table layerx_accounts;
drop table wallet_custody_submissions;
drop table wallet_custody_authorizations;
