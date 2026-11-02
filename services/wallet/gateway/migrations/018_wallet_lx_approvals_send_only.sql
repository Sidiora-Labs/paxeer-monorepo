alter table wallet_lx_approvals
  add constraint wallet_lx_approvals_send_only
  check (state in ('signed','signing_unknown') or artifact->>'kind' = 'lx_send_authorization') not valid;

delete from wallet_lx_approvals
where state in ('reviewed','approved') and artifact->>'kind' = 'lx_activity';

alter table wallet_lx_approvals
  validate constraint wallet_lx_approvals_send_only;
