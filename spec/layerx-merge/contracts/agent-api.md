# Agent api contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## agent-http-healthz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agent-rpc-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1334</code> | <code>GET /healthz on the agent RPC listener</code> |

## agent-http-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agent-rpc-post-rpc</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1335</code> | <code>POST /rpc on the agent RPC mutual-TLS listener (agent_rpc_exchange, services/agent/crates/layerx-agentd/src/main.rs:1290): Agent HTTP envelope version 1, body at most 1048576 bytes; refuses 404, 405, 413, 431 and any LayerX-Tenant, LayerX-Agent or Authorization header</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-agent-rpc-post-rpc</code> | notes | Envelope constants at services/agent/crates/layerx-agentd/src/agent_rpc.rs:21-22; the gateway exposes it as POST /v1/agent/rpc (services/agent/schema/agent-api/v1.kvx:102, services/agent/crates/layerx-sdk/src/agent_envelope.rs:30). Per-operation contracts are the [interface.9.http-rpc-*] blocks. |

## agent-register

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-agent-register</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:26</code> | <code>POST /rpc operation=agent.register</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-agent-register</code> | notes | Registration must go through sponsored bindLayerX and complete bootstrap dispatch (stale tasks 6.7.3 and 6.7.4). Operation enum variant AgentRegister at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 122; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.31.1</code> | req | 9031.1 |
| <code>acceptance.9.31.1</code> | scenario | Given an authorised owner credential, when agent.register is posted through handle_rpc, then the response carries a prepared call whose target is 0x101C and whose selector is bindLayerX. |
| <code>acceptance.9.31.1</code> | test | services/agent/crates/layerx-agentd/tests/bootstrap_bind_layerx.rs::agent_register_prepares_bind_layerx (planned) |
| <code>acceptance.9.31.1</code> | qualification | UNRUN |
| <code>req.9031</code> | ac_1 | agent.register answers with a prepared bindLayerX transaction to 0x101C instead of refused_pending_bootstrap_artifact. |
| <code>capability.9.7.3</code> | title | Open sessions and register agents through bindLayerX |
| <code>capability.9.7.3</code> | reqs | 9031.1<br>9031.2 |

## approval-approve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-approval-approve</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/golden/approval.kvx:54</code> | <code>POST /rpc operation=approval.approve</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-approval-approve</code> | notes | Approval must be decided ahead of secp256k1 signing (stale task 6.7.8). Operation enum variant ApprovalApprove at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 123; carried in the POST /rpc envelope field operation. Callers list shows 58 of 83 caller files. |

## approval-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-approval-get</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/schema/agent-api/golden/approval.kvx:49</code> | <code>POST /rpc operation=approval.get</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-approval-get</code> | notes | Operation enum variant ApprovalGet at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 124; carried in the POST /rpc envelope field operation. Callers list shows 60 of 68 caller files. |

## approval-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-approval-list</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/schema/agent-api/golden/approval.kvx:43</code> | <code>POST /rpc operation=approval.list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-approval-list</code> | notes | Operation enum variant ApprovalList at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 125; carried in the POST /rpc envelope field operation. Callers list shows 56 of 79 caller files. |

## approval-reject

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-approval-reject</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/golden/approval.kvx:60</code> | <code>POST /rpc operation=approval.reject</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-approval-reject</code> | notes | Approval must be decided ahead of secp256k1 signing (stale task 6.7.8). Operation enum variant ApprovalReject at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 126; carried in the POST /rpc envelope field operation. Callers list shows 58 of 76 caller files. |

## availability-fetch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-availability-fetch</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:120</code> | <code>POST /rpc operation=availability.fetch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-availability-fetch</code> | notes | agentd serves it from the LNI availability read; schema services/agent/schema/agent-api/read.kvx:125 requires block retrieval by eth_getBlockByNumber (stale task 6.6.5). Operation enum variant AvailabilityFetch at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 127; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.21.1</code> | req | 9021.1 |
| <code>acceptance.9.21.1</code> | scenario | Given a node returning block 30 with two transactions, when availability is read at height 30, then the result is Available with transaction count 2 and both roots equal to the block header's. |
| <code>acceptance.9.21.1</code> | test | services/agent/crates/layerx-agentd/tests/availability_blocks.rs::availability_reports_block_roots_and_transaction_count (planned) |
| <code>acceptance.9.21.1</code> | qualification | UNRUN |
| <code>req.9021</code> | ac_1 | availability returns the transactions root, receipts root and transaction count of the block at the requested height. |
| <code>acceptance.9.21.2</code> | req | 9021.2 |
| <code>acceptance.9.21.2</code> | scenario | Given a node at head 30 that returns null for block 5, when availability is read at 31 and at 5, then the first returns AvailabilityReadError::Ahead and the second returns Pruned. |
| <code>acceptance.9.21.2</code> | test | services/agent/crates/layerx-agentd/tests/availability_blocks.rs::availability_for_a_missing_block_is_pruned_or_ahead (planned) |
| <code>acceptance.9.21.2</code> | qualification | UNRUN |
| <code>req.9021</code> | ac_2 | A height above head is refused with Ahead and a block the node no longer returns is reported as Pruned. |
| <code>capability.9.5.3</code> | title | Serve availability from eth_getBlockByNumber |
| <code>capability.9.5.3</code> | reqs | 9021.1<br>9021.2 |

## budget-create

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-create</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:128</code> | <code>POST /rpc operation=budget.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-create</code> | notes | Budget mutations must move to layerxpay (stale tasks 6.7.5 and 6.7.2). Operation enum variant BudgetCreate at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 128; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.33.1</code> | req | 9033.1 |
| <code>acceptance.9.33.1</code> | scenario | Given a budget request with ceiling 100 and expiry height 500, when create_protocol_budget runs, then the prepared call targets 0x101B with selector createBudget and those arguments. |
| <code>acceptance.9.33.1</code> | test | services/agent/crates/layerx-agentd/tests/budget_mutations_layerxpay.rs::budget_create_prepares_create_budget_calldata (planned) |
| <code>acceptance.9.33.1</code> | qualification | UNRUN |
| <code>req.9033</code> | ac_1 | create_protocol_budget prepares a 0x101B createBudget call with the requested ceiling, asset and expiry. |
| <code>capability.9.9.2</code> | title | Create, fund and revoke budgets on layerxpay |
| <code>capability.9.9.2</code> | reqs | 9033.1<br>9033.2 |

## budget-fund

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-fund</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:136</code> | <code>POST /rpc operation=budget.fund</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-fund</code> | notes | Budget mutations must move to layerxpay (stale tasks 6.7.5 and 6.7.2). Operation enum variant BudgetFund at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 129; carried in the POST /rpc envelope field operation. |

## budget-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-list</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:143</code> | <code>POST /rpc operation=budget.list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-list</code> | notes | Budget handlers must move to layerxauth and layerxpay (stale task 6.7.2). Operation enum variant BudgetList at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 130; carried in the POST /rpc envelope field operation. |

## budget-reconciliation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-reconciliation</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:148</code> | <code>POST /rpc operation=budget.reconciliation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-reconciliation</code> | notes | Reconciliation must read getBudget (stale task 6.7.6). Operation enum variant BudgetReconciliation at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 131; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.32.2</code> | req | 9032.2 |
| <code>acceptance.9.32.2</code> | scenario | Given a daemon limit of 50 and a pending transaction with value 40 and projected fee 15, when consumption_updates runs, then the limit reports 55 consumed and refuses the next spend. |
| <code>acceptance.9.32.2</code> | test | services/agent/crates/layerx-agentd/tests/budget_reconcile_layerxpay.rs::pending_fee_counts_against_the_daemon_limit (planned) |
| <code>acceptance.9.32.2</code> | qualification | UNRUN |
| <code>req.9032</code> | ac_2 | consumption_updates counts the projected fee of a pending transaction against the daemon limit. |

## budget-revoke

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-revoke</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:152</code> | <code>POST /rpc operation=budget.revoke</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-revoke</code> | notes | Budget mutations must move to layerxpay (stale tasks 6.7.5 and 6.7.2). Operation enum variant BudgetRevoke at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 132; carried in the POST /rpc envelope field operation. |

## budget-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-budget-state</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:158</code> | <code>POST /rpc operation=budget.state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-budget-state</code> | notes | Budget state must read getBudget (stale task 6.7.6). Operation enum variant BudgetState at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 133; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.32.1</code> | req | 9032.1 |
| <code>acceptance.9.32.1</code> | scenario | Given a node answering getBudget for budget b-1 with ceiling 100 and spent 30 at height 9, when reconcile reads b-1, then the state has ceiling 100, spent 30 and height 9. |
| <code>acceptance.9.32.1</code> | test | services/agent/crates/layerx-agentd/tests/budget_reconcile_layerxpay.rs::budget_state_is_decoded_from_get_budget (planned) |
| <code>acceptance.9.32.1</code> | qualification | UNRUN |
| <code>req.9032</code> | ac_1 | ProtocolBudgetState is decoded from a getBudget eth_call result at a committed height. |
| <code>acceptance.9.34.1</code> | req | 9034.1 |
| <code>acceptance.9.34.1</code> | scenario | Given a committed budget b-1, when budget.state is posted, then the response carries ceiling, spent and achieved_rung committed. |
| <code>acceptance.9.34.1</code> | test | services/agent/crates/layerx-agentd/tests/identity_ops_chain.rs::budget_state_operation_reports_committed_rung (planned) |
| <code>acceptance.9.34.1</code> | qualification | UNRUN |
| <code>req.9034</code> | ac_1 | budget.state answers with the getBudget result and achieved rung Committed. |
| <code>acceptance.9.6.1</code> | req | 9006.1 |
| <code>acceptance.9.6.1</code> | scenario | Given a decoded protocol budget record, when from_proven is called with Rung::Receipted and then Rung::Committed, then the first returns ContractError::OutOfRange(budget_rung) and the second returns a view with achieved_rung Committed. |
| <code>acceptance.9.6.1</code> | test | services/agent/crates/layerx-agent-api/tests/identity_rungs.rs::budget_view_requires_the_committed_rung (planned) |
| <code>acceptance.9.6.1</code> | qualification | UNRUN |
| <code>req.9006</code> | ac_1 | ProtocolBudgetView::from_proven refuses a record with achieved_rung below Committed, replacing the StateProven check at services/agent/crates/layerx-agent-api/src/identity.rs:1154. |
| <code>capability.9.1.2</code> | title | Re-target agent-api identity, stream and error mirrors to rungs and replay |
| <code>capability.9.1.2</code> | reqs | 9006.1<br>9006.2<br>9006.3 |
| <code>capability.9.9.1</code> | title | Reconcile budgets against layerxpay getBudget |
| <code>capability.9.9.1</code> | reqs | 9032.1<br>9032.2 |
| <code>capability.9.9.3</code> | title | Answer identity, capability, session and budget operations on chain state |
| <code>capability.9.9.3</code> | reqs | 9034.1<br>9034.2 |

## capability-attenuate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-capability-attenuate</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:73</code> | <code>POST /rpc operation=capability.attenuate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-capability-attenuate</code> | notes | Capability handlers must move to layerxauth (stale task 6.7.2). Operation enum variant CapabilityAttenuate at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 134; carried in the POST /rpc envelope field operation. |

## capability-create

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-capability-create</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:62</code> | <code>POST /rpc operation=capability.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-capability-create</code> | notes | Capability handlers must move to layerxauth (stale task 6.7.2). Operation enum variant CapabilityCreate at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 135; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.34.2</code> | req | 9034.2 |
| <code>acceptance.9.34.2</code> | scenario | Given a session granted transfers up to 50, when capability.create asks for 80, then the response reason is capability.exceeds_grant. |
| <code>acceptance.9.34.2</code> | test | services/agent/crates/layerx-agentd/tests/identity_ops_chain.rs::capability_create_cannot_exceed_the_on_chain_grant (planned) |
| <code>acceptance.9.34.2</code> | qualification | UNRUN |
| <code>req.9034</code> | ac_2 | capability.create refuses a capability wider than the session's on-chain grant with reason capability.exceeds_grant. |

## capability-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-capability-list</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:82</code> | <code>POST /rpc operation=capability.list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-capability-list</code> | notes | Capability handlers must move to layerxauth (stale task 6.7.2). Operation enum variant CapabilityList at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 136; carried in the POST /rpc envelope field operation. |

## capability-revoke

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-capability-revoke</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:88</code> | <code>POST /rpc operation=capability.revoke</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-capability-revoke</code> | notes | Capability handlers must move to layerxauth (stale task 6.7.2). Operation enum variant CapabilityRevoke at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 137; carried in the POST /rpc envelope field operation. |

## export-offline

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-export-offline</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:128</code> | <code>POST /rpc operation=export.offline</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-export-offline</code> | notes | agentd builds a certificate-based export (services/agent/crates/layerx-agent-api/src/read.rs:438); schema services/agent/schema/agent-api/read.kvx:141 requires portable v2 bundles (stale task 6.6.4). Operation enum variant ExportOffline at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 138; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.27.1</code> | req | 9027.1 |
| <code>acceptance.9.27.1</code> | scenario | Given an export of two receipts at committed height 50, when the export is verified offline with its bundled trust, then both receipts verify and the export rung is Committed. |
| <code>acceptance.9.27.1</code> | test | services/agent/crates/layerx-agentd/tests/export_v2.rs::offline_export_verifies_with_only_receipt_keys_and_validator_set (planned) |
| <code>acceptance.9.27.1</code> | qualification | UNRUN |
| <code>req.9027</code> | ac_1 | An offline export v2 verifies offline using only the bundled receipt keys and validator set. |
| <code>acceptance.9.27.2</code> | req | 9027.2 |
| <code>acceptance.9.27.2</code> | scenario | Given a produced export, when its serialized JSON is scanned, then no settlement_domain or certificates key exists. |
| <code>acceptance.9.27.2</code> | test | services/agent/crates/layerx-agentd/tests/export_v2.rs::offline_export_names_no_settlement_domain (planned) |
| <code>acceptance.9.27.2</code> | qualification | UNRUN |
| <code>req.9027</code> | ac_2 | The serialized export contains no settlement domain or certificate field. |
| <code>acceptance.9.5.4</code> | req | 9005.4 |
| <code>acceptance.9.5.4</code> | scenario | Given an OfflineExport built from one receipt and one proof at height 7, when it is constructed, then it exposes facts, receipts, proofs and height 7 and the type has no certificates member. |
| <code>acceptance.9.5.4</code> | test | services/agent/crates/layerx-agent-api/tests/read_heights.rs::offline_export_mirror_has_no_certificates (planned) |
| <code>acceptance.9.5.4</code> | qualification | UNRUN |
| <code>req.9005</code> | ac_4 | OfflineExport carries facts, receipts and proofs at one height and no certificates field, replacing services/agent/crates/layerx-agent-api/src/read.rs:438. |
| <code>capability.9.6.1</code> | title | Produce offline exports v2 without settlement certificates |
| <code>capability.9.6.1</code> | reqs | 9027.1<br>9027.2 |

## faucet-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-faucet-claim</code> | <code>http_route</code> | <code>retire</code> | <code>services/agent/schema/agent-api/identity.kvx:171</code> | <code>POST /rpc operation=faucet.claim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-faucet-claim</code> | notes | Already refused by RETIRED_OPERATIONS at services/agent/crates/layerx-agentd/src/agent_rpc.rs:24; the schema still declares it at services/agent/schema/agent-api/identity.kvx:171 and the Operation enum still lists FaucetClaim. Operation enum variant FaucetClaim at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 139; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.9.1</code> | req | 9009.1 |
| <code>acceptance.9.9.1</code> | scenario | Given the regenerated operation catalogue, when a well-formed envelope naming faucet.claim is posted to handle_rpc, then the response class is UnavailableCapability and Operation has no FaucetClaim variant. |
| <code>acceptance.9.9.1</code> | test | services/agent/crates/layerx-agentd/tests/faucet_claim_retired.rs::faucet_claim_is_refused_as_retired_and_absent_from_the_catalogue (planned) |
| <code>acceptance.9.9.1</code> | qualification | UNRUN |
| <code>req.9009</code> | ac_1 | Posting operation faucet.claim is refused with UnavailableCapability by RETIRED_OPERATIONS (services/agent/crates/layerx-agentd/src/agent_rpc.rs:24) and no Operation variant's name() (services/agent/crates/layerx-agent-api/src/operation_generated.rs:120) returns faucet.claim after regeneration. |
| <code>capability.9.1.5</code> | title | Retire the faucet.claim operation from the agent-api schema and catalogue |
| <code>capability.9.1.5</code> | reqs | 9009.1 |
| <code>resolution.17.839</code> | title | decision.904 binds: Faucet funding is an MCP tool over px_requestFunds only |
| <code>resolution.17.839</code> | applies_to | decision.904<br>task.9.1.5<br>task.9.15.1<br>task.9.15.3 |
| <code>resolution.17.839</code> | source | decision.904 in fragments/J43/decision.kvx; tasks from affects, task text naming decision.904 |

## human-owner-healthz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agentd-human-owner-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:109</code> | <code>GET /healthz in human-owner mode (LAYERX_AGENT_MODE=human-owner); every other path 404 not_found</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-agentd-human-owner-get-healthz</code> | notes | Bearer from LAYERX_AGENT_HUMAN_AUTHORITY_BEARER per services/agent/crates/layerx-agentd/README.md:23. |

## mcp-invoke

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-mcp-invoke</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:178</code> | <code>POST /rpc operation=mcp.invoke</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-mcp-invoke</code> | notes | Operation enum variant McpInvoke at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 140; carried in the POST /rpc envelope field operation. |

## native-reads

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agentd-serve-get-v1-reads-availability</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs:289</code> | <code>GET /v1/reads/availability/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs:1895) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-checkpoint</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs:288</code> | <code>GET /v1/reads/checkpoint/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs:1895) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-history</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs:290</code> | <code>GET /v1/reads/history/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs:1895) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-proof</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs:287</code> | <code>GET /v1/reads/proof/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs:1895) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-receipt</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs:286</code> | <code>GET /v1/reads/receipt/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs:1895) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-agentd-serve-get-v1-reads-availability</code> | notes | NativeReadRoute reads over the LNI client and sequencer trust history; stale tasks 6.6.5 availability by block retrieval, 6.6.6 and 11.5.19 re-target native.rs. |
| <code>interface.9.http-agentd-serve-get-v1-reads-checkpoint</code> | notes | NativeReadRoute reads over the LNI client and sequencer trust history; stale tasks 6.6.7 checkpoint reads as verified Comet commits, 6.6.6 and 11.5.19 re-target native.rs. |
| <code>interface.9.http-agentd-serve-get-v1-reads-history</code> | notes | NativeReadRoute reads over the LNI client and sequencer trust history; stale tasks 6.6.3 history reads re-backed, 6.6.6 and 11.5.19 re-target native.rs. |
| <code>interface.9.http-agentd-serve-get-v1-reads-proof</code> | notes | NativeReadRoute reads over the LNI client and sequencer trust history; stale tasks 6.6.8 proof bundles from receipt inclusion and account proofs, 6.6.6 and 11.5.19 re-target native.rs. |
| <code>interface.9.http-agentd-serve-get-v1-reads-receipt</code> | notes | NativeReadRoute reads over the LNI client and sequencer trust history; stale tasks 6.6.8 proof bundles from receipt inclusion; 0.36 sequencer receipt checks, 6.6.6 and 11.5.19 re-target native.rs. |

## policy-dry-run

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-policy-dry-run</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:220</code> | <code>POST /rpc operation=policy.dry_run</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-policy-dry-run</code> | notes | Operation enum variant PolicyDryRun at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 141; carried in the POST /rpc envelope field operation. |

## prepare

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-prepare</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:6</code> | <code>POST /rpc operation=prepare</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-prepare</code> | notes | Prepare must build EIP-1559 transactions at the pending nonce (stale task 6.5.2). Operation enum variant Prepare at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 142; carried in the POST /rpc envelope field operation. Callers list shows 57 of 207 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.10.1</code> | req | 9010.1 |
| <code>acceptance.9.10.1</code> | scenario | Given chain id 229, pending nonce 7, base fee 10 gwei and a transfer of 5 units, when prepare_activity runs, then Prepared.call.nonce is 7, Prepared.call.to is 0x...101A and Prepared.signing_hash equals the golden hash recorded in the test. |
| <code>acceptance.9.10.1</code> | test | services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepared_transfer_signing_hash_matches_the_eip1559_golden (planned) |
| <code>acceptance.9.10.1</code> | qualification | UNRUN |
| <code>req.9010</code> | ac_1 | A prepared layerxledger transfer yields an Eip1559Call whose signing_hash equals the golden keccak preimage for chain id, pending nonce, fees, gas, target 0x101A and calldata (services/agent/crates/layerx-crypto/src/evm_transaction.rs:195). |
| <code>acceptance.9.10.2</code> | req | 9010.2 |
| <code>acceptance.9.10.2</code> | scenario | Given a node whose pending nonce for the actor is 8, when prepare is called with expected nonce 7, then it returns PrepareError::StaleNonce { expected: 7, pending: 8 } and no Prepared is produced. |
| <code>acceptance.9.10.2</code> | test | services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepare_refuses_an_expected_nonce_behind_the_pending_nonce (planned) |
| <code>acceptance.9.10.2</code> | qualification | UNRUN |
| <code>req.9010</code> | ac_2 | prepare returns StaleNonce when the caller's expected nonce differs from eth_getTransactionCount pending, replacing StaleSequence at services/agent/crates/layerx-agentd/src/prepare/mod.rs:187. |
| <code>acceptance.9.10.3</code> | req | 9010.3 |
| <code>acceptance.9.10.3</code> | scenario | Given a head block with base fee 10 and priority fee 2, when prepare runs with no ceiling and then with ceiling 15, then the first call has max_fee_per_gas 22 and the second returns PrepareError::InvalidDefaults. |
| <code>acceptance.9.10.3</code> | test | services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepare_derives_fee_caps_from_the_head_base_fee (planned) |
| <code>acceptance.9.10.3</code> | qualification | UNRUN |
| <code>req.9010</code> | ac_3 | max_fee_per_gas is twice the head base fee plus the priority fee and a request ceiling below that is refused with InvalidDefaults. |
| <code>acceptance.9.2.1</code> | req | 9002.1 |
| <code>acceptance.9.2.1</code> | scenario | Given the major-2 operation catalogue, when the test posts one well-formed envelope per Operation variant through handle_rpc (services/agent/crates/layerx-agentd/src/agent_rpc.rs:746) with an authorised credential, then no response carries reason envelope.unknown_operation. |
| <code>acceptance.9.2.1</code> | test | services/agent/crates/layerx-agentd/tests/rpc_surface_major_two.rs::every_schema_operation_is_dispatched_retired_or_bootstrap_gated (planned) |
| <code>acceptance.9.2.1</code> | qualification | UNRUN |
| <code>req.9002</code> | ac_1 | Every Operation variant of services/agent/crates/layerx-agent-api/src/operation_generated.rs:4 is answered by an adapter, refused as retired with unavailable_capability, or refused with refused_pending_bootstrap_artifact, never with envelope.unknown_operation. |
| <code>capability.9.2.1</code> | title | Prepare EIP-1559 transactions at the pending nonce |
| <code>capability.9.2.1</code> | reqs | 9010.1<br>9010.2<br>9010.3 |
| <code>capability.9.20.92</code> | title | Freeze the agent RPC surface at contract major 2 with a total dispatch table |
| <code>capability.9.20.92</code> | reqs | 9002.1<br>9002.2<br>9002.3 |
| <code>resolution.17.836</code> | title | decision.901 binds: The agent RPC envelope version stays 1 across the merge |
| <code>resolution.17.836</code> | applies_to | decision.901<br>task.9.0.2 |
| <code>resolution.17.836</code> | source | decision.901 in fragments/J43/decision.kvx; tasks from affects |

## program-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-activity</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:382</code> | <code>POST /rpc operation=program.activity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-activity</code> | notes | Program reads must be views and resultByIdempotency (stale tasks 6.6.10 and 4.6.4). Operation enum variant ProgramActivity at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 143; carried in the POST /rpc envelope field operation. Callers list shows 59 of 65 caller files. |

## program-balances

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agentd-serve-get-v1-programs-balances</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1912</code> | <code>GET /v1/programs/{program_id}/balances on the serve listener -&gt; balance JSON, 400 invalid_program, 503 program_state_unavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-agentd-serve-get-v1-programs-balances</code> | notes | Served from ProgramBalanceReadRoute over the LNI client; stale task 6.6.10 moves program reads to views and resultByIdempotency. |

## program-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-call</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:363</code> | <code>POST /rpc operation=program.call</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-call</code> | notes | Program calls must go to layerxprograms (stale tasks 6.5.25 and 4.6.4). Operation enum variant ProgramCall at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 144; carried in the POST /rpc envelope field operation. Callers list shows 58 of 114 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.36.1</code> | req | 9036.1 |
| <code>acceptance.9.36.1</code> | scenario | Given a deployed program p-1, when program.call is posted, then the response carries tx_hash and achieved_rung submitted. |
| <code>acceptance.9.36.1</code> | test | services/agent/crates/layerx-agentd/tests/program_adapters_rungs.rs::program_call_returns_tx_hash_and_rung (planned) |
| <code>acceptance.9.36.1</code> | qualification | UNRUN |
| <code>req.9036</code> | ac_1 | program.call answers with the transaction hash and achieved rung of the 0x101D call. |
| <code>capability.9.10.2</code> | title | Answer program operations and pin the programs schema to rungs |
| <code>capability.9.10.2</code> | reqs | 9036.1<br>9036.2 |

## program-deploy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-deploy</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:131</code> | <code>POST /rpc operation=program.deploy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-deploy</code> | notes | Program lifecycle must go to layerxprograms (stale tasks 6.5.25 and 4.6.4). Operation enum variant ProgramDeploy at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 145; carried in the POST /rpc envelope field operation. Callers list shows 56 of 87 caller files. |

## program-discover

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-discover</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:338</code> | <code>POST /rpc operation=program.discover</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-discover</code> | notes | Program reads must be views and resultByIdempotency (stale tasks 6.6.10 and 4.6.4). Operation enum variant ProgramDiscover at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 146; carried in the POST /rpc envelope field operation. Callers list shows 55 of 79 caller files. |

## program-interface

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-interface</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:345</code> | <code>POST /rpc operation=program.interface</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-interface</code> | notes | Program reads must be views and resultByIdempotency (stale tasks 6.6.10 and 4.6.4). Operation enum variant ProgramInterface at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 147; carried in the POST /rpc envelope field operation. Callers list shows 58 of 68 caller files. |

## program-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-receipt</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:374</code> | <code>POST /rpc operation=program.receipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-receipt</code> | notes | Program reads must be views and resultByIdempotency (stale tasks 6.6.10 and 4.6.4). Operation enum variant ProgramReceipt at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 148; carried in the POST /rpc envelope field operation. Callers list shows 57 of 87 caller files. |

## program-simulate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-simulate</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:353</code> | <code>POST /rpc operation=program.simulate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-simulate</code> | notes | Program simulation must go to layerxprograms (stale tasks 6.5.25 and 4.6.4). Operation enum variant ProgramSimulate at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 149; carried in the POST /rpc envelope field operation. Callers list shows 56 of 65 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.35.1</code> | req | 9035.1 |
| <code>acceptance.9.35.1</code> | scenario | Given a node answering eth_call for program p-1 with return data 0x01, when simulate runs for p-1, then the call targets 0x101D at pending and the result is 0x01. |
| <code>acceptance.9.35.1</code> | test | services/agent/crates/layerx-agentd/tests/program_ops_layerxprograms.rs::program_simulation_uses_eth_call_at_pending (planned) |
| <code>acceptance.9.35.1</code> | qualification | UNRUN |
| <code>req.9035</code> | ac_1 | ProgramOperations::simulate issues eth_call to 0x101D at the pending tag and returns the decoded result. |
| <code>acceptance.9.35.2</code> | req | 9035.2 |
| <code>acceptance.9.35.2</code> | scenario | Given a node that reverts eth_call with data 0xdead, when simulate runs, then the result is a ProgramOperationError carrying 0xdead. |
| <code>acceptance.9.35.2</code> | test | services/agent/crates/layerx-agentd/tests/program_ops_layerxprograms.rs::program_revert_maps_to_a_typed_operation_error (planned) |
| <code>acceptance.9.35.2</code> | qualification | UNRUN |
| <code>req.9035</code> | ac_2 | An execution revert from eth_call maps to ProgramOperationError with the revert data preserved. |
| <code>capability.9.10.1</code> | title | Run program operations against layerxprograms |
| <code>capability.9.10.1</code> | reqs | 9035.1<br>9035.2 |

## program-upgrade

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-upgrade</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:143</code> | <code>POST /rpc operation=program.upgrade</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-upgrade</code> | notes | Program lifecycle must go to layerxprograms (stale tasks 6.5.25 and 4.6.4). Operation enum variant ProgramUpgrade at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 150; carried in the POST /rpc envelope field operation. Callers list shows 56 of 80 caller files. |

## program-wind-down

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-program-wind-down</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:155</code> | <code>POST /rpc operation=program.wind-down</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-program-wind-down</code> | notes | Program lifecycle must go to layerxprograms (stale tasks 6.5.25 and 4.6.4). Operation enum variant ProgramWindDown at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 151; carried in the POST /rpc envelope field operation. Callers list shows 55 of 82 caller files. |

## project

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-project</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:148</code> | <code>POST /rpc operation=project</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-project</code> | notes | FeeProjection at services/agent/crates/layerx-agent-api/src/read.rs:535 has snapshot_sequence and no denom or gas_units; schema services/agent/schema/agent-api/read.kvx:193 requires them (stale task 6.7.7). Operation enum variant Project at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 152; carried in the POST /rpc envelope field operation. |

## read-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-account</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:23</code> | <code>POST /rpc operation=read.account</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-account</code> | notes | agentd serves it over the LNI read path; the design reads at pending through node RPC (stale tasks 6.6.2 and 6.6.1). Operation enum variant ReadAccount at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 153; carried in the POST /rpc envelope field operation. |

## read-balance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-balance</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:17</code> | <code>POST /rpc operation=read.balance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-balance</code> | notes | agentd serves it over the LNI read path; the design reads at pending through node RPC (stale tasks 6.6.2 and 6.6.1). Operation enum variant ReadBalance at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 154; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.19.1</code> | req | 9019.1 |
| <code>acceptance.9.19.1</code> | scenario | Given a node at head 40 holding balance 900 for the actor, when balance is read at the pending tag, then amount is 900, chain_head is 40 and value_height is 41. |
| <code>acceptance.9.19.1</code> | test | services/agent/crates/layerx-agentd/tests/balance_pending.rs::pending_balance_reports_value_height_and_chain_head (planned) |
| <code>acceptance.9.19.1</code> | qualification | UNRUN |
| <code>req.9019</code> | ac_1 | A pending balance read returns the eth_getBalance amount with value_height equal to chain_head plus one and relative_to pending. |
| <code>acceptance.9.19.2</code> | req | 9019.2 |
| <code>acceptance.9.19.2</code> | scenario | Given a node at head 40, when balance is read at height 45, then the result is the read.height_ahead refusal and no amount is returned. |
| <code>acceptance.9.19.2</code> | test | services/agent/crates/layerx-agentd/tests/balance_pending.rs::balance_at_a_height_ahead_of_head_is_refused (planned) |
| <code>acceptance.9.19.2</code> | qualification | UNRUN |
| <code>req.9019</code> | ac_2 | A balance read at a height above chain_head is refused with reason read.height_ahead. |
| <code>acceptance.9.24.1</code> | req | 9024.1 |
| <code>acceptance.9.24.1</code> | scenario | Given a node at head 40, when read.balance is posted through handle_rpc, then the response freshness names chain_head 40 and value_height and contains no latest_sealed_batch key. |
| <code>acceptance.9.24.1</code> | test | services/agent/crates/layerx-agentd/tests/read_ops_rpc.rs::read_balance_operation_returns_height_freshness (planned) |
| <code>acceptance.9.24.1</code> | qualification | UNRUN |
| <code>req.9024</code> | ac_1 | operation read.balance returns amount, chain_head and value_height and no sequence fields. |
| <code>acceptance.9.5.1</code> | req | 9005.1 |
| <code>acceptance.9.5.1</code> | scenario | Given a VerifiedRead&lt;BalanceValue&gt; built with Rung::Receipted and value_height 41 below chain_head 42, when it is constructed and read back, then achieved_rung is Receipted and freshness.value_height is 41 with relative_to Block at height 41. |
| <code>acceptance.9.5.1</code> | test | services/agent/crates/layerx-agent-api/tests/read_heights.rs::verified_read_carries_achieved_rung_and_height_freshness (planned) |
| <code>acceptance.9.5.1</code> | qualification | UNRUN |
| <code>req.9005</code> | ac_1 | VerifiedRead exposes achieved_rung and a Freshness of chain_head, value_height and relative_to, matching services/agent/schema/agent-api/read.kvx:11 instead of the sequence fields at services/agent/crates/layerx-agent-api/src/read.rs:93. |
| <code>capability.9.1.1</code> | title | Carry block heights and rungs in the agent-api read mirror types |
| <code>capability.9.1.1</code> | reqs | 9005.1<br>9005.2<br>9005.3<br>9005.4 |
| <code>capability.9.5.1</code> | title | Read balances at the pending block through eth_getBalance |
| <code>capability.9.5.1</code> | reqs | 9019.1<br>9019.2 |
| <code>capability.9.5.6</code> | title | Answer read operations from the node JSON-RPC reads |
| <code>capability.9.5.6</code> | reqs | 9024.1<br>9024.2<br>9024.3 |

## read-batch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-batch</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:56</code> | <code>POST /rpc operation=read.batch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-batch</code> | notes | agentd serves a kernel batch; schema services/agent/schema/agent-api/read.kvx:61 makes batch a block height served by eth_getBlockByNumber (stale task 6.6.1). Operation enum variant ReadBatch at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 155; carried in the POST /rpc envelope field operation. |

## read-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-checkpoint</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:63</code> | <code>POST /rpc operation=read.checkpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-checkpoint</code> | notes | agentd verifies guarantor certificates; schema services/agent/schema/agent-api/read.kvx:68 requires a CometBFT commit verified against the validator set (stale task 6.6.7). Operation enum variant ReadCheckpoint at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 156; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.22.1</code> | req | 9022.1 |
| <code>acceptance.9.22.1</code> | scenario | Given a four-validator set and block 20 signed by three equal-power validators, when the checkpoint at 20 is read, then the response carries the block hash, the app hash, three votes and rung Committed. |
| <code>acceptance.9.22.1</code> | test | services/agent/crates/layerx-agentd/tests/checkpoint_commit.rs::checkpoint_is_the_committed_block_at_the_height (planned) |
| <code>acceptance.9.22.1</code> | qualification | UNRUN |
| <code>req.9022</code> | ac_1 | A checkpoint read returns the block hash and app hash at the height with a commit carrying more than two thirds of voting power and rung Committed. |
| <code>acceptance.9.22.2</code> | req | 9022.2 |
| <code>acceptance.9.22.2</code> | scenario | Given a four-validator set and block 20 signed by two equal-power validators, when the checkpoint at 20 is read, then the result is CheckpointReadError::Commit. |
| <code>acceptance.9.22.2</code> | test | services/agent/crates/layerx-agentd/tests/checkpoint_commit.rs::checkpoint_with_insufficient_commit_power_is_refused (planned) |
| <code>acceptance.9.22.2</code> | qualification | UNRUN |
| <code>req.9022</code> | ac_2 | A commit carrying two thirds or less of voting power is refused with CheckpointReadError::Commit. |
| <code>capability.9.5.4</code> | title | Serve checkpoints as block commits |
| <code>capability.9.5.4</code> | reqs | 9022.1<br>9022.2 |

## read-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-history</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:35</code> | <code>POST /rpc operation=read.history</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-history</code> | notes | agentd pages by kernel sequence; schema services/agent/schema/agent-api/read.kvx:38 requires the (blockHeight, txIndex, logIndex) cursor (stale task 6.6.3). Operation enum variant ReadHistory at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 157; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.20.1</code> | req | 9020.1 |
| <code>acceptance.9.20.1</code> | scenario | Given Cursor (12, 3, 4), when it is encoded and decoded, then the triple is unchanged; when a 15-byte hex string is decoded, then HistoryReadError::Cursor is returned. |
| <code>acceptance.9.20.1</code> | test | services/agent/crates/layerx-agentd/tests/history_block_cursor.rs::history_cursor_is_the_block_position_triple (planned) |
| <code>acceptance.9.20.1</code> | qualification | UNRUN |
| <code>req.9020</code> | ac_1 | A history cursor round-trips the (block_height, tx_index, log_index) triple through to_hex and from_hex and refuses another length (services/agent/crates/layerx-agentd/src/read/history.rs:7). |
| <code>acceptance.9.20.2</code> | req | 9020.2 |
| <code>acceptance.9.20.2</code> | scenario | Given an account with five history items across blocks 10 to 12, when history is read with limit 3 and then resumed from next_cursor, then the five items appear once each in order and the second page has no next_cursor. |
| <code>acceptance.9.20.2</code> | test | services/agent/crates/layerx-agentd/tests/history_block_cursor.rs::history_pages_resume_from_the_returned_cursor (planned) |
| <code>acceptance.9.20.2</code> | qualification | UNRUN |
| <code>req.9020</code> | ac_2 | Two consecutive history pages joined by next_cursor return every item once and in block order. |
| <code>acceptance.9.24.3</code> | req | 9024.3 |
| <code>acceptance.9.24.3</code> | scenario | Given an account with four history items, when read.history is posted with limit 2, then two items return with a next_cursor holding block_height, tx_index and log_index. |
| <code>acceptance.9.24.3</code> | test | services/agent/crates/layerx-agentd/tests/read_ops_rpc.rs::read_history_operation_returns_block_position_cursor (planned) |
| <code>acceptance.9.24.3</code> | qualification | UNRUN |
| <code>req.9024</code> | ac_3 | operation read.history returns items and a next_cursor triple. |
| <code>acceptance.9.5.2</code> | req | 9005.2 |
| <code>acceptance.9.5.2</code> | scenario | Given a HistorySelector from height 10 to 9, when validate is called, then it returns ContractError::Zero(history_range); given a valid selector and a HistoryValue with next_cursor (10, 2, 3), then the cursor fields round-trip unchanged. |
| <code>acceptance.9.5.2</code> | test | services/agent/crates/layerx-agent-api/tests/read_heights.rs::history_selector_pages_by_block_position_cursor (planned) |
| <code>acceptance.9.5.2</code> | qualification | UNRUN |
| <code>req.9005</code> | ac_2 | HistorySelector validates from_height &lt;= to_height and a nonzero page limit and HistoryValue.next_cursor is the (block_height, tx_index, log_index) triple of services/agent/schema/agent-api/read.kvx:35. |
| <code>capability.9.5.2</code> | title | Page account history by block-position cursor through px_getHistory |
| <code>capability.9.5.2</code> | reqs | 9020.1<br>9020.2 |

## read-module-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-module-state</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:29</code> | <code>POST /rpc operation=read.module_state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-module-state</code> | notes | Module state must be read through node RPC with ICS23 proofs (stale task 6.6.6). Operation enum variant ReadModuleState at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 158; carried in the POST /rpc envelope field operation. |

## read-proof-bundle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-read-proof-bundle</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:70</code> | <code>POST /rpc operation=read.proof_bundle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-read-proof-bundle</code> | notes | agentd bundles kernel proofs; schema services/agent/schema/agent-api/read.kvx:96 requires layerx-receipt-proof-v2 from receipt inclusion and account proofs (stale task 6.6.8). Operation enum variant ReadProofBundle at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 159; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.23.1</code> | req | 9023.1 |
| <code>acceptance.9.23.1</code> | scenario | Given a block with three receipts, when the bundle for the second receipt is built, then recomputing the root from the path equals the header receipts root and the bundle rung is Receipted. |
| <code>acceptance.9.23.1</code> | test | services/agent/crates/layerx-agentd/tests/proof_bundle_v2.rs::receipt_proof_bundle_verifies_against_the_receipts_root (planned) |
| <code>acceptance.9.23.1</code> | qualification | UNRUN |
| <code>req.9023</code> | ac_1 | A receipt proof bundle v2 carries the receipt, an inclusion path that recomputes the block receipts root and rung Receipted. |
| <code>acceptance.9.23.2</code> | req | 9023.2 |
| <code>acceptance.9.23.2</code> | scenario | Given an account and a head block without commit, when an account bundle is requested, then the result is read.not_committed; after the commit arrives, the bundle carries rung Committed. |
| <code>acceptance.9.23.2</code> | test | services/agent/crates/layerx-agentd/tests/proof_bundle_v2.rs::account_proof_bundle_requires_a_committed_state_root (planned) |
| <code>acceptance.9.23.2</code> | qualification | UNRUN |
| <code>req.9023</code> | ac_2 | An account proof bundle is built only against a committed block and is refused before commit with reason read.not_committed. |
| <code>acceptance.9.24.2</code> | req | 9024.2 |
| <code>acceptance.9.24.2</code> | scenario | Given a receipted transaction, when read.proof_bundle is posted for its receipt, then the response format is layerx-receipt-proof-v2 and achieved_rung is receipted. |
| <code>acceptance.9.24.2</code> | test | services/agent/crates/layerx-agentd/tests/read_ops_rpc.rs::read_proof_bundle_operation_returns_receipt_proof_v2 (planned) |
| <code>acceptance.9.24.2</code> | qualification | UNRUN |
| <code>req.9024</code> | ac_2 | operation read.proof_bundle answers a receipt target with a layerx-receipt-proof-v2 bundle at rung Receipted. |
| <code>acceptance.9.5.3</code> | req | 9005.3 |
| <code>acceptance.9.5.3</code> | scenario | Given a receipt-target proof bundle response claiming Rung::Submitted, when the mirror validates it against its request, then it returns ContractError::Mismatch and the same response with Rung::Receipted validates. |
| <code>acceptance.9.5.3</code> | test | services/agent/crates/layerx-agent-api/tests/read_heights.rs::proof_bundle_mirror_requires_receipted_or_committed_rungs (planned) |
| <code>acceptance.9.5.3</code> | qualification | UNRUN |
| <code>req.9005</code> | ac_3 | A proof bundle record for an Activity or Receipt target requires Rung::Receipted and an Account target requires Rung::Committed, replacing the Level checks at services/agent/crates/layerx-agent-api/src/read.rs:327-328. |
| <code>acceptance.9.7.2</code> | req | 9007.2 |
| <code>acceptance.9.7.2</code> | scenario | Given the major-2 schema, when the read.proof_bundle declaration is loaded, then its response names layerx-receipt-proof-v2 and its verification rule names Receipted and Committed. |
| <code>acceptance.9.7.2</code> | test | services/agent/crates/layerx-agent-api/tests/schema_rungs.rs::proof_bundle_operation_declares_receipt_proof_v2 (planned) |
| <code>acceptance.9.7.2</code> | qualification | UNRUN |
| <code>req.9007</code> | ac_2 | operation.read.proof_bundle declares layerx-receipt-proof-v2 with Receipted for activity and receipt targets and Committed for account targets (services/agent/schema/agent-api/read.kvx:95). |
| <code>capability.9.5.5</code> | title | Build receipt proof bundles v2 from eth_getProof |
| <code>capability.9.5.5</code> | reqs | 9023.1<br>9023.2 |

## record-agenthttpcredential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpcredential</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:120</code> | <code>[record.AgentHttpCredential] in v1.kvx: fields = ["tenant:utf8","session_id:hex32","token_id:hex32","generation:Sequence"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpcredential</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## record-agenthttpenvelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpenvelope</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:112</code> | <code>[record.AgentHttpEnvelope] in v1.kvx: fields = ["version:u8","request_id:Sequence","operation:utf8","request:canonical-map","credential:AgentHttpCredential?","idempotency_key:hex32?"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpenvelope</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## record-agenthttperror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttperror</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:132</code> | <code>[record.AgentHttpError] in v1.kvx: fields = ["class:ErrorClass","protocol_result_code:i32?","retriability:Retriability","request_id:Sequence","reason:utf8"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttperror</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## record-agenthttpsuccess

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpsuccess</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:128</code> | <code>[record.AgentHttpSuccess] in v1.kvx: fields = ["request_id:Sequence","value:canonical-map","verification_status:VerificationStatus"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-agenthttpsuccess</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-contractversion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-contractversion</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:49</code> | <code>[record.ContractVersion] in v1.kvx: fields = ["major:u16","minor:u16"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-contractversion</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeactivityv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeactivityv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:726</code> | <code>[record.NativeActivityV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeActivity"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeactivityv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeapprovaldecisionv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovaldecisionv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:809</code> | <code>[record.NativeApprovalDecisionV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalDecisionV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovaldecisionv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeapprovalgetv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalgetv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:802</code> | <code>[record.NativeApprovalGetV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalGetV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalgetv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeapprovallistresultv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistresultv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:831</code> | <code>[record.NativeApprovalListResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalListResultV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistresultv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeapprovallistv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:796</code> | <code>[record.NativeApprovalListV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalListV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeapprovalresultv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalresultv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:823</code> | <code>[record.NativeApprovalResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalResultV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalresultv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativedisclosurerequestv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativedisclosurerequestv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:933</code> | <code>[record.NativeDisclosureRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeDisclosureRequestV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativedisclosurerequestv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativedisclosureresultv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativedisclosureresultv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:942</code> | <code>[record.NativeDisclosureResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeDisclosureResultV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativedisclosureresultv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeeffectpreparerequestv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeeffectpreparerequestv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:858</code> | <code>[record.NativeEffectPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeEffectPrepareRequestV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeeffectpreparerequestv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativelocalgrantconsentv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativelocalgrantconsentv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:838</code> | <code>[record.NativeLocalGrantConsentV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeLocalGrantConsentV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativelocalgrantconsentv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativepreparationpurposev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativepreparationpurposev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:737</code> | <code>[record.NativePreparationPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePreparationPurposeV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativepreparationpurposev1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativepreparerequestv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativepreparerequestv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:772</code> | <code>[record.NativePrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePrepareRequestV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativepreparerequestv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeprepareresultv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeprepareresultv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:816</code> | <code>[record.NativePrepareResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePrepareResultV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeprepareresultv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativeregistrationpreparerequestv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeregistrationpreparerequestv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:884</code> | <code>[record.NativeRegistrationPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeRegistrationPrepareRequestV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativeregistrationpreparerequestv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativesendpreparerequestv1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativesendpreparerequestv1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:915</code> | <code>[record.NativeSendPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeSendPrepareRequestV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativesendpreparerequestv1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-nativesendpurposev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativesendpurposev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:900</code> | <code>[record.NativeSendPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeSendPurposeV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-nativesendpurposev1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-signednativepreparationpurposev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-signednativepreparationpurposev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:758</code> | <code>[record.SignedNativePreparationPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::SignedNativePreparationPurposeV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-signednativepreparationpurposev1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-signednativesendpurposev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-signednativesendpurposev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:908</code> | <code>[record.SignedNativeSendPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::SignedNativeSendPurposeV1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-signednativesendpurposev1</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-versionrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-versionrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:52</code> | <code>[record.VersionRequest] in v1.kvx: fields = ["request_id:Sequence","supported:ContractVersion"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-versionrequest</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## record-versionresponse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-versionresponse</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:56</code> | <code>[record.VersionResponse] in v1.kvx: fields = ["request_id:Sequence","contract:ContractVersion","node_interface_major:u16"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-record-versionresponse</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## scalar-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-amount</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:15</code> | <code>[scalar.Amount] in v1.kvx: rust = "u128"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-amount</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. Callers list shows 59 of 66 caller files. |

## scalar-budgetlimit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-budgetlimit</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:29</code> | <code>[scalar.BudgetLimit] in v1.kvx: rust = "u128"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-budgetlimit</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## scalar-sequence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-sequence</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:22</code> | <code>[scalar.Sequence] in v1.kvx: rust = "u64"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-sequence</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## scalar-timestampseconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-timestampseconds</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:36</code> | <code>[scalar.TimestampSeconds] in v1.kvx: rust = "u64"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-scalar-timestampseconds</code> | notes | Module v1.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. Callers list shows 59 of 74 caller files. |

## serve-errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-http-serve-body-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1857</code> | <code>HTTP serve listener error body_too_large</code> |
| <code>interface.9.err-http-serve-headers-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1843</code> | <code>HTTP serve listener error headers_too_large</code> |
| <code>interface.9.err-http-serve-invalid-program</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1921</code> | <code>HTTP serve listener error invalid_program</code> |
| <code>interface.9.err-http-serve-invalid-read</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1904</code> | <code>HTTP serve listener error invalid_read</code> |
| <code>interface.9.err-http-serve-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1846</code> | <code>HTTP serve listener error invalid_request</code> |
| <code>interface.9.err-http-serve-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1897</code> | <code>HTTP serve listener error not_found</code> |
| <code>interface.9.err-http-serve-program-state-unavailable</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1924</code> | <code>HTTP serve listener error program_state_unavailable</code> |
| <code>interface.9.err-http-serve-read-too-large</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1906</code> | <code>HTTP serve listener error read_too_large</code> |
| <code>interface.9.err-http-serve-request-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1824</code> | <code>HTTP serve listener error request_timeout</code> |
| <code>interface.9.err-http-serve-unauthorized</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1874</code> | <code>HTTP serve listener error unauthorized</code> |
| <code>interface.9.err-http-serve-verified-read-unavailable</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1908</code> | <code>HTTP serve listener error verified_read_unavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-http-serve-body-too-large</code> | notes | HTTP error body {"error":"body_too_large"} on the serve listener. |
| <code>interface.9.err-http-serve-headers-too-large</code> | notes | HTTP error body {"error":"headers_too_large"} on the serve listener. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.err-http-serve-invalid-program</code> | notes | HTTP error body {"error":"invalid_program"} on the serve listener. Belongs to the program balance route that stale task 6.6.10 re-targets. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.err-http-serve-invalid-read</code> | notes | HTTP error body {"error":"invalid_read"} on the serve listener. Belongs to the LNI-backed /v1/reads routes that stale tasks 6.6.5 to 6.6.8 re-target. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.err-http-serve-invalid-request</code> | notes | HTTP error body {"error":"invalid_request"} on the serve listener. |
| <code>interface.9.err-http-serve-not-found</code> | notes | HTTP error body {"error":"not_found"} on the serve listener. |
| <code>interface.9.err-http-serve-program-state-unavailable</code> | notes | HTTP error body {"error":"program_state_unavailable"} on the serve listener. Belongs to the program balance route that stale task 6.6.10 re-targets. |
| <code>interface.9.err-http-serve-read-too-large</code> | notes | HTTP error body {"error":"read_too_large"} on the serve listener. Belongs to the LNI-backed /v1/reads routes that stale tasks 6.6.5 to 6.6.8 re-target. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.err-http-serve-request-timeout</code> | notes | HTTP error body {"error":"request_timeout"} on the serve listener. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.err-http-serve-unauthorized</code> | notes | HTTP error body {"error":"unauthorized"} on the serve listener. |
| <code>interface.9.err-http-serve-verified-read-unavailable</code> | notes | HTTP error body {"error":"verified_read_unavailable"} on the serve listener. Belongs to the LNI-backed /v1/reads routes that stale tasks 6.6.5 to 6.6.8 re-target. No in-repo reference outside the defining line at this revision. |

## serve-healthz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-agentd-serve-get-healthz</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1886</code> | <code>GET /healthz on the bearer-authenticated serve listener (route_request, services/agent/crates/layerx-agentd/src/main.rs:1879; Bearer check services/agent/crates/layerx-agentd/src/main.rs:1870; GET only services/agent/crates/layerx-agentd/src/main.rs:1852) -&gt; 200 or 503 {ready,transport_ready,verified_reads_ready,tenant_writes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-agentd-serve-get-healthz</code> | notes | Readiness is a probe-program balance read through ProgramBalanceReadRoute over the LNI client; stale tasks 6.6.10 and 11.5.58 re-target program reads and main.rs off the LNI transport. |

## session-close

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-session-close</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:41</code> | <code>POST /rpc operation=session.close</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-session-close</code> | notes | Session handlers must move to layerxauth grants (stale tasks 6.7.2 and 6.5.9). Operation enum variant SessionClose at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 160; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.30.1</code> | req | 9030.1 |
| <code>acceptance.9.30.1</code> | scenario | Given a session with two pending preparations and a receipted revokeGrant for its grant id, when the revocation is applied, then both preparations are invalidated with reason GrantRevokedOnChain. |
| <code>acceptance.9.30.1</code> | test | services/agent/crates/layerx-agentd/tests/session_grants_layerxauth.rs::revoke_grant_receipt_invalidates_pending_preparations (planned) |
| <code>acceptance.9.30.1</code> | qualification | UNRUN |
| <code>req.9030</code> | ac_1 | A revokeGrant receipt for a session's grant invalidates every pending preparation of that session. |
| <code>capability.9.7.2</code> | title | Grant and revoke session authority through layerxauth |
| <code>capability.9.7.2</code> | reqs | 9030.1<br>9030.2 |

## session-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-session-list</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:46</code> | <code>POST /rpc operation=session.list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-session-list</code> | notes | Session handlers must move to layerxauth grants (stale tasks 6.7.2 and 6.5.9). Operation enum variant SessionList at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 161; carried in the POST /rpc envelope field operation. |

## session-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-session-open</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:31</code> | <code>POST /rpc operation=session.open</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-session-open</code> | notes | Bootstrap dispatch and session keys with their own EVM address (stale tasks 6.7.3 and 6.5.9). Operation enum variant SessionOpen at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 162; carried in the POST /rpc envelope field operation. Callers list shows 56 of 58 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.29.1</code> | req | 9029.1 |
| <code>acceptance.9.29.1</code> | scenario | Given a fresh registry and a prepared seed, when the key is provisioned, then the registry reports the address equal to evm_address of the key's public key. |
| <code>acceptance.9.29.1</code> | test | services/agent/crates/layerx-agentd/tests/session_keys_evm.rs::provisioned_session_key_reports_its_evm_address (planned) |
| <code>acceptance.9.29.1</code> | qualification | UNRUN |
| <code>req.9029</code> | ac_1 | A provisioned session key exposes the 20-byte address derived from its secp256k1 public key. |
| <code>acceptance.9.29.2</code> | req | 9029.2 |
| <code>acceptance.9.29.2</code> | scenario | Given a registry directory with one ed25519 entry, when load runs, then it returns SessionKeyRegistryError::KeyKind and no key is usable. |
| <code>acceptance.9.29.2</code> | test | services/agent/crates/layerx-agentd/tests/session_keys_evm.rs::legacy_ed25519_session_entry_is_refused_at_load (planned) |
| <code>acceptance.9.29.2</code> | qualification | UNRUN |
| <code>req.9029</code> | ac_2 | A registry entry holding an ed25519 seed written at HEAD is refused at load with SessionKeyRegistryError::KeyKind. |
| <code>acceptance.9.31.2</code> | req | 9031.2 |
| <code>acceptance.9.31.2</code> | scenario | Given a provisioned session key with address S, when session.open is posted, then the prepared call targets 0x101C with selector grantSession and argument S. |
| <code>acceptance.9.31.2</code> | test | services/agent/crates/layerx-agentd/tests/bootstrap_bind_layerx.rs::session_open_prepares_grant_session_for_the_session_address (planned) |
| <code>acceptance.9.31.2</code> | qualification | UNRUN |
| <code>req.9031</code> | ac_2 | session.open answers with a prepared grantSession transaction naming the provisioned session address. |
| <code>capability.9.7.1</code> | title | Derive session keys as secp256k1 keys bound to an EVM address |
| <code>capability.9.7.1</code> | reqs | 9029.1<br>9029.2 |

## session-refresh

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-session-refresh</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/identity.kvx:36</code> | <code>POST /rpc operation=session.refresh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-session-refresh</code> | notes | Session handlers must move to layerxauth grants (stale tasks 6.7.2 and 6.5.9). Operation enum variant SessionRefresh at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 163; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.30.2</code> | req | 9030.2 |
| <code>acceptance.9.30.2</code> | scenario | Given a provisioned session key without a grantSession receipt, when session.refresh is requested, then the response reason is session.grant_missing. |
| <code>acceptance.9.30.2</code> | test | services/agent/crates/layerx-agentd/tests/session_grants_layerxauth.rs::grant_session_receipt_is_required_before_refresh (planned) |
| <code>acceptance.9.30.2</code> | qualification | UNRUN |
| <code>req.9030</code> | ac_2 | session.refresh is refused with reason session.grant_missing when no grantSession receipt exists for the session address. |

## sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-sign</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:19</code> | <code>POST /rpc operation=sign</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-sign</code> | notes | Signing must be secp256k1 with the 65-byte encoding already declared in schema write.kvx (stale task 6.5.1). Operation enum variant Sign at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 164; carried in the POST /rpc envelope field operation. Callers list shows 59 of 149 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.11.1</code> | req | 9011.1 |
| <code>acceptance.9.11.1</code> | scenario | Given a provisioned session key and a Prepared transfer, when self_sign runs, then SignedPreparation.signature has 65 bytes and recover_sender on the signed bytes returns SignedPreparation.signer_address. |
| <code>acceptance.9.11.1</code> | test | services/agent/crates/layerx-agentd/tests/evm_signing.rs::self_signed_transaction_recovers_to_the_session_address (planned) |
| <code>acceptance.9.11.1</code> | qualification | UNRUN |
| <code>req.9011</code> | ac_1 | A self-signed preparation carries a 65-byte signature whose recovered sender equals the session key's EVM address (services/agent/crates/layerx-crypto/src/evm_transaction.rs:307). |
| <code>acceptance.9.11.2</code> | req | 9011.2 |
| <code>acceptance.9.11.2</code> | scenario | Given a Prepared transfer for address A and a signature produced by key B, when attach_external_signature is called, then it returns SigningError::SignerMismatch and nothing is queued. |
| <code>acceptance.9.11.2</code> | test | services/agent/crates/layerx-agentd/tests/evm_signing.rs::external_signature_from_another_key_is_refused (planned) |
| <code>acceptance.9.11.2</code> | qualification | UNRUN |
| <code>req.9011</code> | ac_2 | attach_external_signature refuses a 65-byte signature that recovers to an address other than the prepared actor (services/agent/crates/layerx-agentd/src/sign.rs:310). |
| <code>capability.9.2.2</code> | title | Sign prepared transactions with secp256k1 and 65-byte signatures |
| <code>capability.9.2.2</code> | reqs | 9011.1<br>9011.2<br>9011.3 |

## submit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-submit</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:30</code> | <code>POST /rpc operation=submit</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-submit</code> | notes | Submit must use eth_sendRawTransaction and pax_sequencerReceipt instead of the LNI transport (stale tasks 6.5.3 and 6.7.11). Operation enum variant Submit at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 165; carried in the POST /rpc envelope field operation. Callers list shows 54 of 276 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.11.3</code> | req | 9011.3 |
| <code>acceptance.9.11.3</code> | scenario | Given a signed transfer whose calldata byte 4 is flipped after signing, when verify_before_submit runs, then it returns an error and the original bytes verify. |
| <code>acceptance.9.11.3</code> | test | services/agent/crates/layerx-agentd/tests/evm_signing.rs::verify_before_submit_rejects_mutated_calldata (planned) |
| <code>acceptance.9.11.3</code> | qualification | UNRUN |
| <code>req.9011</code> | ac_3 | verify_before_submit refuses signed bytes whose decoded call differs from the prepared call (services/agent/crates/layerx-agentd/src/sign.rs:324). |
| <code>acceptance.9.12.1</code> | req | 9012.1 |
| <code>acceptance.9.12.1</code> | scenario | Given a signed transfer and a node endpoint that accepts it, when submit is posted through handle_rpc, then the response tx_hash equals the keccak hash of the raw bytes and state is Submitted. |
| <code>acceptance.9.12.1</code> | test | services/agent/crates/layerx-agentd/tests/write_path_evm.rs::submit_returns_the_transaction_hash_and_submitted_state (planned) |
| <code>acceptance.9.12.1</code> | qualification | UNRUN |
| <code>req.9012</code> | ac_1 | operation submit answers with the transaction hash returned by eth_sendRawTransaction and SubmissionState Submitted (services/agent/schema/agent-api/write.kvx:30). |
| <code>acceptance.9.14.1</code> | req | 9014.1 |
| <code>acceptance.9.14.1</code> | scenario | Given an empty outbox, when the same signed raw transaction is enqueued twice, then the first returns its transaction hash and the second returns OutboxError::Duplicate. |
| <code>acceptance.9.14.1</code> | test | services/agent/crates/layerx-agentd/tests/outbox_tx_hash.rs::outbox_keys_records_by_transaction_hash (planned) |
| <code>acceptance.9.14.1</code> | qualification | UNRUN |
| <code>req.9014</code> | ac_1 | Outbox::enqueue keys a record by the keccak hash of its raw bytes and refuses a second enqueue of identical bytes (services/agent/crates/layerx-agentd/src/outbox/mod.rs:129). |
| <code>acceptance.9.15.1</code> | req | 9015.1 |
| <code>acceptance.9.15.1</code> | scenario | Given a store with retention_blocks 100 and records at heights 10 and 95, when sweep runs at height 150, then exactly the record at height 10 is removed and sweep returns 1. |
| <code>acceptance.9.15.1</code> | test | services/agent/crates/layerx-agentd/tests/idempotency_height.rs::sweep_expires_records_older_than_the_retention_window_in_blocks (planned) |
| <code>acceptance.9.15.1</code> | qualification | UNRUN |
| <code>req.9015</code> | ac_1 | Store::sweep removes records whose recorded head height is more than retention_blocks below the current height (services/agent/crates/layerx-agentd/src/idempotency.rs:224). |
| <code>acceptance.9.66.1</code> | req | 9066.1 |
| <code>acceptance.9.66.1</code> | scenario | Given the limits suite at services/agent/tests/limits/exactly_once.rs:133 submitting EVM raw transactions, when it runs, then the report has one economic effect, one unique receipt, final state Executed and two attempts. |
| <code>acceptance.9.66.1</code> | test | services/agent/tests/limits/exactly_once.rs::limits_restarts_and_duplicates_converge_on_one_economic_effect (existing) |
| <code>acceptance.9.66.1</code> | qualification | UNRUN |
| <code>req.9066</code> | ac_1 | The limits suite still converges restarts and duplicates on one economic effect, one receipt, final state Executed and two attempts when submissions are EIP-1559 raw transactions. |
| <code>acceptance.9.66.2</code> | req | 9066.2 |
| <code>acceptance.9.66.2</code> | scenario | Given a signed EIP-1559 raw transaction submitted once and then resubmitted after a store restart, when both attempts finish, then the outbox holds one transaction hash and one economic effect. |
| <code>acceptance.9.66.2</code> | test | services/agent/tests/limits/exactly_once.rs::limits_identical_raw_transaction_keeps_one_hash (planned) |
| <code>acceptance.9.66.2</code> | qualification | UNRUN |
| <code>req.9066</code> | ac_2 | Resubmitting the identical signed raw transaction after a restart records one transaction hash in the outbox. |
| <code>capability.9.2.3</code> | title | Route the write adapters through eth_sendRawTransaction |
| <code>capability.9.2.3</code> | reqs | 9012.1<br>9012.2<br>9012.3 |
| <code>capability.9.20.1</code> | title | Re-target the limits exactly-once suite at EVM raw transactions |
| <code>capability.9.20.1</code> | reqs | 9066.1<br>9066.2 |
| <code>capability.9.3.1</code> | title | Key the outbox by transaction hash with a Committed state |
| <code>capability.9.3.1</code> | reqs | 9014.1<br>9014.2<br>9014.3 |
| <code>capability.9.3.2</code> | title | Sweep idempotency records by block height |
| <code>capability.9.3.2</code> | reqs | 9015.1<br>9015.2 |
| <code>resolution.17.838</code> | title | decision.903 binds: The outbox adopts the nine schema submission states |
| <code>resolution.17.838</code> | applies_to | decision.903<br>task.9.3.1<br>task.9.3.2<br>task.9.3.3 |
| <code>resolution.17.838</code> | source | decision.903 in fragments/J43/decision.kvx; tasks from affects |
| <code>resolution.17.843</code> | title | decision.908 binds: Idempotency windows are swept by block height |
| <code>resolution.17.843</code> | applies_to | decision.908<br>task.9.3.2 |
| <code>resolution.17.843</code> | source | decision.908 in fragments/J43/decision.kvx; tasks from affects |

## subscription-acknowledge

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-acknowledge</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:37</code> | <code>POST /rpc operation=subscription.acknowledge</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-acknowledge</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionAcknowledge at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 166; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.37.1</code> | req | 9037.1 |
| <code>acceptance.9.37.1</code> | scenario | Given a subscription acknowledged at block 12, tx 1, log 0, when the store is closed and reopened, then resume_cursor returns (12, 1, 0). |
| <code>acceptance.9.37.1</code> | test | services/agent/crates/layerx-agentd/tests/subscription_cursor_v3.rs::subscription_cursor_triple_survives_restart (planned) |
| <code>acceptance.9.37.1</code> | qualification | UNRUN |
| <code>req.9037</code> | ac_1 | A subscription acknowledged at (12, 1, 0) resumes at that triple after the store is reopened. |
| <code>acceptance.9.39.1</code> | req | 9039.1 |
| <code>acceptance.9.39.1</code> | scenario | Given a subscription that delivered through (10,0,0), when acknowledge is posted with (11,0,0), then the response reason is subscription.cursor_ahead. |
| <code>acceptance.9.39.1</code> | test | services/agent/crates/layerx-agentd/tests/subscription_ops_cursor.rs::subscription_acknowledge_beyond_delivery_is_refused (planned) |
| <code>acceptance.9.39.1</code> | qualification | UNRUN |
| <code>req.9039</code> | ac_1 | subscription.acknowledge refuses a cursor beyond the last delivered position with reason subscription.cursor_ahead. |
| <code>capability.9.11.1</code> | title | Persist subscription cursors as block-position triples |
| <code>capability.9.11.1</code> | reqs | 9037.1<br>9037.2 |
| <code>capability.9.11.3</code> | title | Answer subscription operations with block-position cursors |
| <code>capability.9.11.3</code> | reqs | 9039.1<br>9039.2 |
| <code>resolution.17.844</code> | title | decision.909 binds: Subscription cursors become a block, transaction and log index triple |
| <code>resolution.17.844</code> | applies_to | decision.909<br>task.9.11.1<br>task.9.11.2<br>task.9.11.3 |
| <code>resolution.17.844</code> | source | decision.909 in fragments/J43/decision.kvx; tasks from affects |

## subscription-create

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-create</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:14</code> | <code>POST /rpc operation=subscription.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-create</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionCreate at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 167; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.38.1</code> | req | 9038.1 |
| <code>acceptance.9.38.1</code> | scenario | Given three logs delivered as (11,0,1), (10,2,0), (11,0,0), when the ingestor drains them, then take_next yields (10,2,0), (11,0,0), (11,0,1). |
| <code>acceptance.9.38.1</code> | test | services/agent/crates/layerx-agentd/tests/ingest_eth_subscribe.rs::logs_are_ingested_in_block_position_order (planned) |
| <code>acceptance.9.38.1</code> | qualification | UNRUN |
| <code>req.9038</code> | ac_1 | Logs delivered out of order by eth_subscribe are handed to subscribers in (block_height, tx_index, log_index) order. |
| <code>acceptance.9.6.2</code> | req | 9006.2 |
| <code>acceptance.9.6.2</code> | scenario | Given a ReceiptReference::Verified built with receipt_ref r-1 and Rung::Receipted, when it is matched, then the rung field is Receipted and no verification_level member exists. |
| <code>acceptance.9.6.2</code> | test | services/agent/crates/layerx-agent-api/tests/identity_rungs.rs::stream_receipt_reference_carries_a_rung (planned) |
| <code>acceptance.9.6.2</code> | qualification | UNRUN |
| <code>req.9006</code> | ac_2 | ReceiptReference::Verified carries receipt_ref and rung as services/agent/schema/agent-api/stream.kvx:56 requires. |
| <code>capability.9.11.2</code> | title | Ingest events from eth_subscribe logs with gap backfill |
| <code>capability.9.11.2</code> | reqs | 9038.1<br>9038.2 |

## subscription-delete

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-delete</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:46</code> | <code>POST /rpc operation=subscription.delete</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-delete</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionDelete at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 168; carried in the POST /rpc envelope field operation. |

## subscription-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-health</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:31</code> | <code>POST /rpc operation=subscription.health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-health</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionHealth at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 169; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.37.2</code> | req | 9037.2 |
| <code>acceptance.9.37.2</code> | scenario | Given a durable store holding one LXS2 record written at HEAD, when the store opens, then that subscription reports subscription.cursor_reset_required. |
| <code>acceptance.9.37.2</code> | test | services/agent/crates/layerx-agentd/tests/subscription_cursor_v3.rs::lxs2_subscription_record_requires_cursor_reset (planned) |
| <code>acceptance.9.37.2</code> | qualification | UNRUN |
| <code>req.9037</code> | ac_2 | A persisted LXS2 record is refused with reason subscription.cursor_reset_required. |
| <code>acceptance.9.38.2</code> | req | 9038.2 |
| <code>acceptance.9.38.2</code> | scenario | Given an ingestor at block 10 and a newHeads notification for 13, when gap detection runs, then a gap 11..12 is detected and backfilled items are admitted before block 13 logs. |
| <code>acceptance.9.38.2</code> | test | services/agent/crates/layerx-agentd/tests/ingest_eth_subscribe.rs::missing_block_range_is_backfilled_from_history (planned) |
| <code>acceptance.9.38.2</code> | qualification | UNRUN |
| <code>req.9038</code> | ac_2 | A newHeads jump from 10 to 13 triggers a backfill of blocks 11 and 12 through px_getHistory. |
| <code>acceptance.9.39.2</code> | req | 9039.2 |
| <code>acceptance.9.39.2</code> | scenario | Given a subscription delivered through (10,0,0) and an ingest watermark at (12,1,0), when subscription.health is posted, then the response carries both triples. |
| <code>acceptance.9.39.2</code> | test | services/agent/crates/layerx-agentd/tests/subscription_ops_cursor.rs::subscription_health_reports_delivered_and_watermark_triples (planned) |
| <code>acceptance.9.39.2</code> | qualification | UNRUN |
| <code>req.9039</code> | ac_2 | subscription.health reports both the last delivered triple and the ingest watermark triple. |

## subscription-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-list</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:25</code> | <code>POST /rpc operation=subscription.list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-list</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionList at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 170; carried in the POST /rpc envelope field operation. |

## subscription-pause

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-pause</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:40</code> | <code>POST /rpc operation=subscription.pause</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-pause</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionPause at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 171; carried in the POST /rpc envelope field operation. |

## subscription-resume

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-subscription-resume</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:43</code> | <code>POST /rpc operation=subscription.resume</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-subscription-resume</code> | notes | Subscriptions must run on eth_subscribe instead of the LNI event stream (stale task 6.5.49). Operation enum variant SubscriptionResume at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 172; carried in the POST /rpc envelope field operation. |

## tenant-readiness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-tenant-readiness</code> | <code>http_route</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:9</code> | <code>POST /rpc operation=tenant.readiness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-tenant-readiness</code> | notes | Operation enum variant TenantReadiness at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 173; carried in the POST /rpc envelope field operation. |

## track

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-track</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:37</code> | <code>POST /rpc operation=track</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-track</code> | notes | Track must follow the EVM path and the Committed state (stale tasks 6.5.8 and 6.5.4). Operation enum variant Track at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 174; carried in the POST /rpc envelope field operation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.12.3</code> | req | 9012.3 |
| <code>acceptance.9.12.3</code> | scenario | Given one queued submission, when track is posted with its tx_hash and then with a 31-byte value, then the first returns its state and the second is refused with reason invalid_request. |
| <code>acceptance.9.12.3</code> | test | services/agent/crates/layerx-agentd/tests/write_path_evm.rs::track_reports_state_by_transaction_hash (planned) |
| <code>acceptance.9.12.3</code> | qualification | UNRUN |
| <code>req.9012</code> | ac_3 | operation track looks the submission up by its 32-byte transaction hash and refuses a 31-byte key with invalid_request. |
| <code>acceptance.9.14.2</code> | req | 9014.2 |
| <code>acceptance.9.14.2</code> | scenario | Given a submitted record, when it is moved to Committed, then the transition is refused; when it is moved to Executed then Committed, then both succeed and a later move to Failed is refused. |
| <code>acceptance.9.14.2</code> | test | services/agent/crates/layerx-agentd/tests/outbox_tx_hash.rs::committed_is_reached_only_from_executed (planned) |
| <code>acceptance.9.14.2</code> | qualification | UNRUN |
| <code>req.9014</code> | ac_2 | legal_transition admits Executed to Committed and refuses Submitted to Committed and any transition out of Committed (services/agent/crates/layerx-agentd/src/outbox/mod.rs:366). |
| <code>acceptance.9.14.3</code> | req | 9014.3 |
| <code>acceptance.9.14.3</code> | scenario | Given a submitted record at nonce 4 and a node reporting a different transaction committed at nonce 4, when the outbox reconciles, then the record is Failed with reason submission.nonce_consumed. |
| <code>acceptance.9.14.3</code> | test | services/agent/crates/layerx-agentd/tests/outbox_tx_hash.rs::consumed_nonce_fails_with_nonce_consumed (planned) |
| <code>acceptance.9.14.3</code> | qualification | UNRUN |
| <code>req.9014</code> | ac_3 | A submission whose nonce was consumed by another transaction ends Failed with reason submission.nonce_consumed instead of the retired Superseded state. |
| <code>acceptance.9.16.1</code> | req | 9016.1 |
| <code>acceptance.9.16.1</code> | scenario | Given an Unknown submission and a node returning a signed success receipt for its hash, when resolve_unknown runs, then the observation is ExecutedReceipt and the outbox state is Executed. |
| <code>acceptance.9.16.1</code> | test | services/agent/crates/layerx-agentd/tests/unknown_receipt_lookup.rs::unknown_submission_resolves_to_executed_from_a_signed_receipt (planned) |
| <code>acceptance.9.16.1</code> | qualification | UNRUN |
| <code>req.9016</code> | ac_1 | resolve_unknown moves an Unknown submission to Executed when pax_sequencerReceipt returns a receipt signed by a published receipt key (services/agent/crates/layerx-agentd/src/outbox/unknown.rs:111). |
| <code>acceptance.9.16.2</code> | req | 9016.2 |
| <code>acceptance.9.16.2</code> | scenario | Given an Unknown submission at nonce 3, no receipt and a pending nonce of 5, when resolve_unknown runs, then the record is Failed with reason submission.nonce_consumed and nothing is resent. |
| <code>acceptance.9.16.2</code> | test | services/agent/crates/layerx-agentd/tests/unknown_receipt_lookup.rs::unknown_submission_with_consumed_nonce_fails (planned) |
| <code>acceptance.9.16.2</code> | qualification | UNRUN |
| <code>req.9016</code> | ac_2 | resolve_unknown marks the submission Failed with reason submission.nonce_consumed when no receipt exists and the pending nonce has advanced past it. |
| <code>capability.9.3.3</code> | title | Resolve unknown submissions through pax_sequencerReceipt |
| <code>capability.9.3.3</code> | reqs | 9016.1<br>9016.2<br>9016.3 |

## type-apierror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-apierror</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:6</code> | <code>[type.ApiError] in errors.kvx: required = ["class","protocol_result_code","retriability","request_id","reason"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-apierror</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-approvaldecisionoutcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-approvaldecisionoutcome</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:31</code> | <code>[type.ApprovalDecisionOutcome] in approval.kvx: variants = ["Granted","Rejected","Expired","Defective","AlreadyDecided","Conflict"]</code> |

## type-approvallifecycleevent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-approvallifecycleevent</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:34</code> | <code>[type.ApprovalLifecycleEvent] in approval.kvx: variants = ["Created","Granted","Rejected","Expired","Defective"]</code> |

## type-approvalrecord

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-approvalrecord</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:21</code> | <code>[type.ApprovalRecord] in approval.kvx: required = ["approval_id","tenant","held_activity","canonical_bytes_digest","hold_reason","created_at","expires_at","state"]</code> |

## type-approvalstate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-approvalstate</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:27</code> | <code>[type.ApprovalState] in approval.kvx: variants = ["Held","Granted","Rejected","Expired","Defective"]</code> |

## type-authoritydescription

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-authoritydescription</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:118</code> | <code>[type.AuthorityDescription] in identity.kvx: required = ["tenant","agent_did","authority_ref","protocol_authority"]</code> |

## type-authorityresponse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-authorityresponse</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:163</code> | <code>[type.AuthorityResponse] in identity.kvx: required = ["authority","value"]</code> |

## type-budgetauthorization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-budgetauthorization</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:187</code> | <code>[type.BudgetAuthorization] in identity.kvx: required = ["preparation_ref","signature"]</code> |

## type-budgetenforcement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-budgetenforcement</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:122</code> | <code>[type.BudgetEnforcement] in identity.kvx: variants = ["ProtocolBudget","DaemonLimit"]</code> |

## type-budgetrecord

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-budgetrecord</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:194</code> | <code>[type.BudgetRecord] in identity.kvx: required = ["enforcement","budget_id","head","activity_id"]</code> |

## type-budgetrecords

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-budgetrecords</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:209</code> | <code>[type.BudgetRecords] in identity.kvx: required = ["budgets"]</code> |

## type-budgetstate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-budgetstate</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:213</code> | <code>[type.BudgetState] in identity.kvx: required = ["record","balance","proven_head","activity_id"]</code> |

## type-capabilitydimensions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-capabilitydimensions</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:51</code> | <code>[type.CapabilityDimensions] in identity.kvx: required = ["activity_types","counterparties","assets","amount_ceilings","rate_ceilings","purpose_constraints","expiry"]</code> |

## type-capabilityrecord

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-capabilityrecord</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:97</code> | <code>[type.CapabilityRecord] in identity.kvx: required = ["capability_id","parent_id","tenant","agent_did","dimensions","state","created_at_ms","created_at_sequence","revoked_at_ms","revoked_at_sequence"]</code> |

## type-capabilityrecords

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-capabilityrecords</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:113</code> | <code>[type.CapabilityRecords] in identity.kvx: required = ["capabilities"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-capabilityrecords</code> | notes | No in-repo reference outside the defining line at this revision. |

## type-capabilitystate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-capabilitystate</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:109</code> | <code>[type.CapabilityState] in identity.kvx: variants = ["active","revoked","expired"]</code> |

## type-delivery

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-delivery</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:59</code> | <code>[type.Delivery] in stream.kvx: variants = ["Event","Gap","Truncated"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-delivery</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-disclosure

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-disclosure</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:15</code> | <code>[type.Disclosure] in write.kvx: required = ["canonical_digest","activity_type","actor","authority","counterparties","amounts","asset","fee_limit","expiry","idempotency_key"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-disclosure</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-errorclass

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-errorclass</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:9</code> | <code>[type.ErrorClass] in errors.kvx: variants = ["TransportFailure","Deadline","ProtocolIncompatibility","UnavailableCapability","CoreRejection","VerificationFailure","PolicyRefusal","CapabilityRefusal","BudgetRefusal","RateLimit","IdempotencyConflict","InternalFault"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-errorclass</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-eventdelivery

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-eventdelivery</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:49</code> | <code>[type.EventDelivery] in stream.kvx: required = ["event_identity","event_bytes","deduplication_id","cursor","receipt_reference"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-eventdelivery</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-factref

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-factref</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:175</code> | <code>[type.FactRef] in read.kvx: grammar = ["receipt:&lt;activity_id&gt;","activity:&lt;activity_id&gt;","state:&lt;activity_id&gt;:&lt;account_id&gt;","checkpoint:&lt;activity_id&gt;:&lt;height&gt;"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-factref</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-faucetgrant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-faucetgrant</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:167</code> | <code>[type.FaucetGrant] in identity.kvx: required = ["funded","funding_id","transaction_id","amount","network"]</code> |

## type-feeprojection

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-feeprojection</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:192</code> | <code>[type.FeeProjection] in read.kvx: required = ["request","parameter_version","fee","gas_units","denom","canonical_schedule","snapshot_height","snapshot_state_root"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-feeprojection</code> | notes | Rust mirror services/agent/crates/layerx-agent-api/src/read.rs:535 has snapshot_sequence and no gas_units or denom; this schema requires gas_units, denom and snapshot_height. |

## type-feeprojectionrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-feeprojectionrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:185</code> | <code>[type.FeeProjectionRequest] in read.kvx: required = ["protocol_activity_type","canonical_bytes","execution_units","storage_units"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-feeprojectionrequest</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-freshness</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:11</code> | <code>[type.Freshness] in read.kvx: required = ["chain_head","value_height","relative_to"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-freshness</code> | notes | Rust mirror services/agent/crates/layerx-agent-api/src/read.rs:93 uses kernel sequences (chain_head, latest_sealed_batch, latest_finalised_checkpoint, value_sequence); this schema requires block heights chain_head, value_height, relative_to. |

## type-gapnotice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-gapnotice</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:62</code> | <code>[type.GapNotice] in stream.kvx: required = ["missing_first","missing_last","backfill_cursor","backfill_attempted"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-gapnotice</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-historycursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historycursor</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:46</code> | <code>[type.HistoryCursor] in read.kvx: fields = ["blockHeight","txIndex","logIndex"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historycursor</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-historyitem

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historyitem</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:162</code> | <code>[type.HistoryItem] in read.kvx: required = ["height","txIndex","logIndex","kind","canonical","proof"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historyitem</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-historykind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historykind</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:170</code> | <code>[type.HistoryKind] in read.kvx: variants = ["Activity","Receipt","Event"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-historykind</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-holdreason

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-holdreason</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:17</code> | <code>[type.HoldReason] in approval.kvx: required = ["code","message"]</code> |

## type-idempotentmutation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-idempotentmutation</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:88</code> | <code>[type.IdempotentMutation] in errors.kvx: required = ["request_id","key","body_digest","operation"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-idempotentmutation</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-legacypolicydryrun

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-legacypolicydryrun</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:250</code> | <code>[type.LegacyPolicyDryRun] in identity.kvx: required = ["context","canonical_intent"]</code> |

## type-mcpexternalsignaturev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-mcpexternalsignaturev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:82</code> | <code>[type.McpExternalSignatureV1] in write.kvx: operation = "sign"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-mcpexternalsignaturev1</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-mcpinvokerequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-mcpinvokerequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:184</code> | <code>[type.McpInvokeRequest] in identity.kvx: required = ["tool","arguments"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-mcpinvokerequest</code> | notes | No in-repo reference outside the defining line at this revision. |

## type-mcpnativewritev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-mcpnativewritev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:92</code> | <code>[type.McpNativeWriteV1] in write.kvx: variant = "native_write_v1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-mcpnativewritev1</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-nativedisclosurev1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-nativedisclosurev1</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:75</code> | <code>[type.NativeDisclosureV1] in write.kvx: operation = "prepare"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-nativedisclosurev1</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-nativeprogramcallfields

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallfields</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:241</code> | <code>[type.NativeProgramCallFields] in programs.kvx: required = ["guest_abi","entrypoint","capabilities_hex","access_declaration_hex","response_capacity","resources"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallfields</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-nativeprogramcallrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:251</code> | <code>[type.NativeProgramCallRequest] in programs.kvx: required = ["payload_encoding","program_id","calldata","budget","signed_activity","native_call"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallrequest</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramdeploy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeploy</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:50</code> | <code>[type.NativeProgramDeploy] in programs.kvx: required = ["program_id","guest_abi","policy","authority","new_hash","wasm"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeploy</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramdeployrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeployrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:23</code> | <code>[type.NativeProgramDeployRequest] in programs.kvx: required = ["signed_activity"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeployrequest</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramupgrade

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgrade</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:63</code> | <code>[type.NativeProgramUpgrade] in programs.kvx: required = ["program_id","guest_abi","old_hash","new_hash","migration_hook","clear_interface","wasm"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgrade</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramupgraderequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgraderequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:32</code> | <code>[type.NativeProgramUpgradeRequest] in programs.kvx: required = ["signed_activity"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgraderequest</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramwinddown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddown</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:78</code> | <code>[type.NativeProgramWindDown] in programs.kvx: required = ["program_id","operation"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddown</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeprogramwinddownrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddownrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:41</code> | <code>[type.NativeProgramWindDownRequest] in programs.kvx: required = ["signed_activity"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddownrequest</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-nativeregistrationcapability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-nativeregistrationcapability</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:55</code> | <code>[type.NativeRegistrationCapability] in identity.kvx: activity_types = "Every capability in the native active chain lists Asset module 1 ordinal 1."</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-nativeregistrationcapability</code> | notes | No in-repo reference outside the defining line at this revision. |

## type-policydryrunresult

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-policydryrunresult</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:240</code> | <code>[type.PolicyDryRunResult] in identity.kvx: required = ["outcome","policy_version","matched_rules","deciding_rule","reason","mode","authority_statement"]</code> |

## type-policyintent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-policyintent</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:258</code> | <code>[type.PolicyIntent] in identity.kvx: magic_version = "8 bytes 4c58504400000001"</code> |

## type-preparewithcapability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-preparewithcapability</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:68</code> | <code>[type.PrepareWithCapability] in write.kvx: required = ["request","capability_id"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-preparewithcapability</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-programactivityselector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programactivityselector</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:285</code> | <code>[type.ProgramActivitySelector] in programs.kvx: required = ["activity_id","requested_verification_level"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programactivityselector</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programcallbudget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcallbudget</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:221</code> | <code>[type.ProgramCallBudget] in programs.kvx: required = ["fuel","fee_limit"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcallbudget</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programcallrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcallrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:231</code> | <code>[type.ProgramCallRequest] in programs.kvx: profile = "retained_legacy_call"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcallrequest</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programcapability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcapability</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:226</code> | <code>[type.ProgramCapability] in programs.kvx: variants = ["storage_read","storage_write","transfer","emit_event","compose"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programcapability</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programexecutionevidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionevidence</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:312</code> | <code>[type.ProgramExecutionEvidence] in programs.kvx: required = ["activity_id","receipt","terminal_payload","call_graph","authority"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionevidence</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programexecutionv4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionv4</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:289</code> | <code>[type.ProgramExecutionV4] in programs.kvx: encoding_version = 4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionv4</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programexecutionv5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionv5</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:297</code> | <code>[type.ProgramExecutionV5] in programs.kvx: encoding_version = 5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programexecutionv5</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programfailure

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programfailure</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:334</code> | <code>[type.ProgramFailure] in programs.kvx: variants = ["unknown_program","reentrancy","depth_exceeded","fanout_exceeded","guest_refused","authority","resource","response","fault"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programfailure</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programguestabi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programguestabi</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:16</code> | <code>[type.ProgramGuestAbi] in programs.kvx: variants = ["ABI_V1_VERSION","ABI_V2_VERSION","ABI_V3_VERSION","ABI_V4_VERSION","ABI_V5_VERSION"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programguestabi</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programlifecycleerror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleerror</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:124</code> | <code>[type.ProgramLifecycleError] in programs.kvx: required = ["code","retry"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleerror</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programlifecycleerrorenvelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleerrorenvelope</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:119</code> | <code>[type.ProgramLifecycleErrorEnvelope] in programs.kvx: required = ["error"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleerrorenvelope</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programlifecyclereceipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecyclereceipt</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:278</code> | <code>[type.ProgramLifecycleReceipt] in programs.kvx: required = ["activity_id","receipt"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecyclereceipt</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programlifecyclereceiptenvelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecyclereceiptenvelope</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:274</code> | <code>[type.ProgramLifecycleReceiptEnvelope] in programs.kvx: required = ["result"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecyclereceiptenvelope</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programlifecycleresponse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresponse</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:89</code> | <code>[type.ProgramLifecycleResponse] in programs.kvx: variants = ["ProgramLifecycleResultEnvelope","ProgramLifecycleUnknown","ProgramLifecycleErrorEnvelope"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresponse</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programlifecycleresult

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresult</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:99</code> | <code>[type.ProgramLifecycleResult] in programs.kvx: required = ["state","activity_id","receipt","terminal_payload","call_graph"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresult</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programlifecycleresultenvelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresultenvelope</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:94</code> | <code>[type.ProgramLifecycleResultEnvelope] in programs.kvx: required = ["result"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresultenvelope</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programlifecycleunknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleunknown</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:110</code> | <code>[type.ProgramLifecycleUnknown] in programs.kvx: required = ["state","activity_id","retry","retry_after_seconds"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programlifecycleunknown</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programoutcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programoutcome</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:330</code> | <code>[type.ProgramOutcome] in programs.kvx: variants = ["completed","legacy_completed","refused"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programoutcome</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programreceiptresponse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programreceiptresponse</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:271</code> | <code>[type.ProgramReceiptResponse] in programs.kvx: variants = ["ProgramSubmission","ProgramLifecycleReceiptEnvelope"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programreceiptresponse</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programreceiptselector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programreceiptselector</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:266</code> | <code>[type.ProgramReceiptSelector] in programs.kvx: required = ["idempotency_key","expected_activity_id","requested_verification_level"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programreceiptselector</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programselector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programselector</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:167</code> | <code>[type.ProgramSelector] in programs.kvx: required = ["program_id","requested_verification_level"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programselector</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programsimulation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsimulation</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:319</code> | <code>[type.ProgramSimulation] in programs.kvx: required = ["committed","execution","simulation_evidence"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsimulation</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-programsource

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsource</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:214</code> | <code>[type.ProgramSource] in programs.kvx: required = ["status:string"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsource</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-programsubmission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsubmission</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:324</code> | <code>[type.ProgramSubmission] in programs.kvx: required = ["state","activity_id","idempotency_key"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-programsubmission</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-projectionresult

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-projectionresult</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:203</code> | <code>[type.ProjectionResult] in read.kvx: required = ["projected","rationale","observed_freshness"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-projectionresult</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-proofbundle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-proofbundle</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:95</code> | <code>[type.ProofBundle] in read.kvx: format = "layerx-receipt-proof-v2"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-proofbundle</code> | notes | Rust mirror services/agent/crates/layerx-agent-api/src/read.rs:196 has target and proofs; this schema requires format layerx-receipt-proof-v2 with rawTransaction, signedReceipt, receiptDigest, receiptKey and committed fields. |

## type-proofbundletarget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-proofbundletarget</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:91</code> | <code>[type.ProofBundleTarget] in read.kvx: variants = ["Activity","AccountState","Receipt"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-proofbundletarget</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-receiptkey

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-receiptkey</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/read.kvx:113</code> | <code>[type.ReceiptKey] in read.kvx: required = ["keyId","publicKey","firstHeight","lastHeight"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-receiptkey</code> | notes | Module read.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-receiptreference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-receiptreference</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:54</code> | <code>[type.ReceiptReference] in stream.kvx: variants = ["None","Verified"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-receiptreference</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-retriability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-retriability</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:13</code> | <code>[type.Retriability] in errors.kvx: variants = ["Terminal","Retriable"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-retriability</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-rung

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-rung</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:30</code> | <code>[type.Rung] in errors.kvx: variants = ["Submitted","Receipted","Committed"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-rung</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.253.1</code> | req | 8253.1 |
| <code>acceptance.8.253.1</code> | scenario | Given a prepare answer for nonce 3 with its true signing_hash and a copy whose nonce is 4, when decodeDaemonPrepared runs on each, then the first returns a PreparedActivity and the second throws decode-failure. |
| <code>acceptance.8.253.1</code> | test | services/platform/middleware/agent/test/daemon-evm.test.mjs::decodeDaemonPreparedRecomputesSigningHash (planned) |
| <code>acceptance.8.253.1</code> | qualification | UNRUN |
| <code>req.8253</code> | ac_1 | decodeDaemonPrepared (services/platform/middleware/agent/src/daemon.ts:98) accepts a prepare answer whose signing_hash equals the EIP-1559 signing hash of its fields and refuses one with a single altered nonce. |
| <code>acceptance.8.253.2</code> | req | 8253.2 |
| <code>acceptance.8.253.2</code> | scenario | Given a prepared transaction for sender S, a signature by S and a signature by key T, when signedActivityId runs on each, then the first returns the 0x transaction hash and the second throws verification-failure. |
| <code>acceptance.8.253.2</code> | test | services/platform/middleware/agent/test/daemon-evm.test.mjs::signedTransactionHashRecoversPreparedSender (planned) |
| <code>acceptance.8.253.2</code> | qualification | UNRUN |
| <code>req.8253</code> | ac_2 | signedActivityId (services/platform/middleware/agent/src/daemon.ts:122) returns keccak256 of the assembled raw transaction for a 65-byte signature by the prepared sender and refuses a signature by another key. |
| <code>acceptance.8.253.3</code> | req | 8253.3 |
| <code>acceptance.8.253.3</code> | scenario | Given an executed submission with level Receipted and copies with SequencerSigned, BatchIncluded and SettlementAnchored, when decodeDaemonSubmission runs on each, then only the first decodes. |
| <code>acceptance.8.253.3</code> | test | services/platform/middleware/agent/test/daemon-evm.test.mjs::decodeDaemonSubmissionAcceptsOnlyRungLevels (planned) |
| <code>acceptance.8.253.3</code> | qualification | UNRUN |
| <code>req.8253</code> | ac_3 | decodeDaemonSubmission (services/platform/middleware/agent/src/daemon.ts:130) accepts verification_level Receipted with a signed_receipt and refuses SequencerSigned, BatchIncluded and SettlementAnchored. |
| <code>acceptance.9.1.1</code> | req | 9001.1 |
| <code>acceptance.9.1.1</code> | scenario | Given the agent-api crate at HEAD exports only Level from verify (services/agent/crates/layerx-agent-api/src/lib.rs:130), when the test names verify::Rung and verify::RungStatus::Below with requested Committed and achieved Receipted, then it compiles and Rung::Committed.name() returns "committed". |
| <code>acceptance.9.1.1</code> | test | services/agent/crates/layerx-agent-api/tests/rung.rs::rung_and_rung_status_are_exported_from_the_verify_contract (planned) |
| <code>acceptance.9.1.1</code> | qualification | UNRUN |
| <code>req.9001</code> | ac_1 | layerx_agent_api::verify exports Rung with exactly Submitted, Receipted and Committed and RungStatus with Achieved and Below, matching errors.kvx type.VerificationStatus (services/agent/schema/agent-api/errors.kvx:38). |
| <code>acceptance.9.1.2</code> | req | 9001.2 |
| <code>acceptance.9.1.2</code> | scenario | Given the six Level variants declared at services/agent/crates/layerx-agent-api/src/error.rs:129, when Level::rung is called on each, then the result equals the decision.907 table and SettlementAnchored yields None. |
| <code>acceptance.9.1.2</code> | test | services/agent/crates/layerx-agent-api/tests/rung.rs::legacy_level_maps_onto_rungs_by_decision (planned) |
| <code>acceptance.9.1.2</code> | qualification | UNRUN |
| <code>req.9001</code> | ac_2 | Level::rung maps Unverified and SequencerSigned to Submitted, BatchIncluded and StateProven to Receipted, CheckpointFinalised to Committed and SettlementAnchored to None (services/agent/crates/layerx-agent-api/src/error.rs:129). |
| <code>acceptance.9.8.2</code> | req | 9008.2 |
| <code>acceptance.9.8.2</code> | scenario | Given the agent-api crate after this task, when the test builds VerificationStatus::Unverified with requested Committed and achieved Receipted, then it compiles and matches errors.kvx type.VerificationStatus (services/agent/schema/agent-api/errors.kvx:38). |
| <code>acceptance.9.8.2</code> | test | services/agent/crates/layerx-agent-api/tests/schema_retired.rs::verification_status_is_expressed_in_rungs_only (planned) |
| <code>acceptance.9.8.2</code> | qualification | UNRUN |
| <code>req.9008</code> | ac_2 | layerx_agent_api::verify exposes Rung and a VerificationStatus whose Achieved and Unverified variants carry rungs, and no Level type, replacing services/agent/crates/layerx-agent-api/src/error.rs:129. |
| <code>capability.8.24.2</code> | title | Decode agentd prepared transactions and submissions on EVM and rungs |
| <code>capability.8.24.2</code> | reqs | 8253.1<br>8253.2<br>8253.3 |
| <code>capability.9.20.91</code> | title | Add Rung and RungStatus to the agent-api verify contract beside Level |
| <code>capability.9.20.91</code> | reqs | 9001.1<br>9001.2 |
| <code>resolution.17.820</code> | title | decision.825 binds: Agent daemon verification levels are Submitted, Receipted and Committed |
| <code>resolution.17.820</code> | applies_to | decision.825<br>task.8.24.2 |
| <code>resolution.17.820</code> | source | decision.825 in fragments/J41/decision.kvx; tasks from affects |
| <code>resolution.17.842</code> | title | decision.907 binds: Kernel verification levels map onto the three rungs |
| <code>resolution.17.842</code> | applies_to | decision.907<br>task.9.0.1<br>task.9.1.4<br>task.9.4.1<br>task.9.26.1<br>task.9.27.1 |
| <code>resolution.17.842</code> | source | decision.907 in fragments/J43/decision.kvx; tasks from affects, task text naming decision.907 |

## type-sessioncontext

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-sessioncontext</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:6</code> | <code>[type.SessionContext] in identity.kvx: required = ["tenant","agent_did","authority_ref","permitted_activity_types","expiry","client","policy_version"]</code> |

## type-settlementdomain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-v1-type-settlementdomain</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/agent-api/v1.kvx:43</code> | <code>[type.SettlementDomain] in v1.kvx: variants = ["Paxeer"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-v1-type-settlementdomain</code> | notes | Stale task 6.6.12 retires the settlement domain; stale task 11.5.46 re-targets v1.kvx off it. |

## type-structuredactivitydisclosure

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-approval-type-structuredactivitydisclosure</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:12</code> | <code>[type.StructuredActivityDisclosure] in approval.kvx: required = ["canonical_digest","activity_type","actor","authority","counterparties","amounts","asset","fee_limit","expiry","idempotency_key"]</code> |

## type-submissionstate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-submissionstate</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:44</code> | <code>[type.SubmissionState] in write.kvx: variants = ["Prepared","Signed","Queued","Submitted","Unknown","Executed","Committed","Failed","Expired"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-submissionstate</code> | notes | agentd outbox SubmissionState (services/agent/crates/layerx-agentd/src/outbox/mod.rs:25) has Acknowledged and Superseded and no Committed, unlike this schema and services/agent/crates/layerx-agent-api/src/write.rs:211 (stale task 6.5.4). |

## type-subscriptionfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionfilter</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:9</code> | <code>[type.SubscriptionFilter] in stream.kvx: required = ["agents","accounts","activity_types","modules","assets","counterparties","result_classes"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionfilter</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-subscriptionhealth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealth</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:76</code> | <code>[type.SubscriptionHealth] in stream.kvx: required = ["target","last_acknowledged","last_delivery_at","pending_backfill"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealth</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-subscriptionhealthtarget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealthtarget</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:73</code> | <code>[type.SubscriptionHealthTarget] in stream.kvx: required = ["scope","subscription_id"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealthtarget</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. No in-repo reference outside the defining line at this revision. |

## type-subscriptionrecord

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionrecord</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:69</code> | <code>[type.SubscriptionRecord] in stream.kvx: required = ["subscription_id","scope","filter","start","last_acknowledged","delivery_target","paused"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionrecord</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-subscriptionscope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionscope</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:6</code> | <code>[type.SubscriptionScope] in stream.kvx: required = ["tenant","agent","capability"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-subscriptionscope</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-tenantreadiness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-tenantreadiness</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:19</code> | <code>[type.TenantReadiness] in identity.kvx: required = ["transport_ready:bool","verified_reads_ready:bool","writes_admitted:bool","recovery_reason:optional&lt;TenantRecoveryReason&gt;"]</code> |

## type-tenantreadinessrequest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-tenantreadinessrequest</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:15</code> | <code>[type.TenantReadinessRequest] in identity.kvx: required = []</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-tenantreadinessrequest</code> | notes | No in-repo reference outside the defining line at this revision. |

## type-tenantrecoveryreason

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-identity-type-tenantrecoveryreason</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:23</code> | <code>[type.TenantRecoveryReason] in identity.kvx: variants = ["recovery_pending","store_unavailable","store_refused","budget_state_unverified","receipt_evidence_missing","durable_recovery_failed","spend_unreconciled","transport_unavailable","verified_read_unavailable"]</code> |

## type-transition

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-write-type-transition</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/write.kvx:56</code> | <code>[type.Transition] in write.kvx: required = ["from","to","cause","at"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-write-type-transition</code> | notes | Module write.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-truncationnotice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-truncationnotice</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/stream.kvx:66</code> | <code>[type.TruncationNotice] in stream.kvx: required = ["requested_first","oldest_available","resume_cursor"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-stream-type-truncationnotice</code> | notes | Module stream.kvx is touched by stale pending task 11.5.46 (settlement-domain re-target); this section is not named by it. |

## type-verificationstatus

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-verificationstatus</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:38</code> | <code>[type.VerificationStatus] in errors.kvx: variants = ["Achieved","Unverified"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-errors-type-verificationstatus</code> | notes | Module errors.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-verifiedprogramdiscovery

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-verifiedprogramdiscovery</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:171</code> | <code>[type.VerifiedProgramDiscovery] in programs.kvx: required = ["program_id","lifecycle","version","code_hash","abi_version","receipt_digest","state_root","observed_sequence","observed_at","valid_through","verification","deployment_receipt_digest"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-verifiedprogramdiscovery</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-verifiedprograminterface

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-verifiedprograminterface</code> | <code>proto_message</code> | <code>stable</code> | <code>services/agent/schema/agent-api/programs.kvx:193</code> | <code>[type.VerifiedProgramInterface] in programs.kvx: required = ["program_id","version","code_hash","abi_version","interface","interface_digest","receipt_digest","state_root","observed_sequence","observed_at","valid_through","source","verification"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-programs-type-verifiedprograminterface</code> | notes | Module programs.kvx is touched by stale pending task 11.5.45 (settlement-domain re-target); this section is not named by it. |

## type-verifiedread

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-agent-api-read-type-verifiedread</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:6</code> | <code>[type.VerifiedRead] in read.kvx: required = ["value","achieved_verification_level","freshness"]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-agent-api-read-type-verifiedread</code> | notes | Rust mirror services/agent/crates/layerx-agent-api/src/read.rs:110 carries the agent-api Level as achieved_verification_level; this schema makes it a Rung or null. |

## wait

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.http-rpc-wait</code> | <code>http_route</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:59</code> | <code>POST /rpc operation=wait</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.http-rpc-wait</code> | notes | Wait must resolve on rungs over the EVM path (stale tasks 6.5.8 and 6.5.7). Operation enum variant Wait at services/agent/crates/layerx-agent-api/src/operation_generated.rs line 175; carried in the POST /rpc envelope field operation. Callers list shows 61 of 80 caller files. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.12.2</code> | req | 9012.2 |
| <code>acceptance.9.12.2</code> | scenario | Given a submitted transaction with a sequencer receipt and no block commit, when wait is posted with rung committed and a 50 ms deadline, then the response reports achieved Receipted and status Below. |
| <code>acceptance.9.12.2</code> | test | services/agent/crates/layerx-agentd/tests/write_path_evm.rs::wait_for_committed_reports_the_achieved_rung (planned) |
| <code>acceptance.9.12.2</code> | qualification | UNRUN |
| <code>req.9012</code> | ac_2 | operation wait with requested rung Committed returns achieved rung Receipted with a Below status when the deadline passes before commit. |
| <code>acceptance.9.17.1</code> | req | 9017.1 |
| <code>acceptance.9.17.1</code> | scenario | Given a submitted hash and a receipt signed by an unpublished key, when wait_for_rung asks for Receipted with a 10 ms deadline, then the result is achieved Submitted. |
| <code>acceptance.9.17.1</code> | test | services/agent/crates/layerx-agentd/tests/finality_rungs.rs::receipt_signed_by_unpublished_key_does_not_reach_receipted (planned) |
| <code>acceptance.9.17.1</code> | qualification | UNRUN |
| <code>req.9017</code> | ac_1 | A receipt whose signature recovers to a key absent from px_getReceiptKeys leaves the submission at Submitted. |
| <code>acceptance.9.17.2</code> | req | 9017.2 |
| <code>acceptance.9.17.2</code> | scenario | Given a receipted transaction whose block commit carries exactly two thirds of voting power, when wait_for_rung asks for Committed, then it reports Receipted; with one more validator signature it reports Committed. |
| <code>acceptance.9.17.2</code> | test | services/agent/crates/layerx-agentd/tests/finality_rungs.rs::committed_requires_more_than_two_thirds_voting_power (planned) |
| <code>acceptance.9.17.2</code> | qualification | UNRUN |
| <code>req.9017</code> | ac_2 | Committed is reported only when the block commit carries strictly more than two thirds of voting power. |
| <code>capability.9.4.1</code> | title | Report finality as rungs with receipt keys and block commit |
| <code>capability.9.4.1</code> | reqs | 9017.1<br>9017.2 |

## wiring-integration

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.961.1</code> | req | 9961.1 |
| <code>acceptance.9.961.1</code> | scenario | Given a known private key and an unsigned type-2 transaction, when the test encodes it, signs the signing hash, decodes the raw signed bytes and recovers the sender, then the decoded fields equal the input and the recovered sender equals the key's address. |
| <code>acceptance.9.961.1</code> | test | services/agent/sdk/typescript/test/evm-transaction.test.ts::evm_transaction_roundtrip_recovers_sender (planned) |
| <code>acceptance.9.961.1</code> | qualification | UNRUN |
| <code>req.9961</code> | ac_1 | encodeUnsignedEip1559, eip1559SigningHash and recoverEip1559Sender round-trip a signed type-2 transaction to its sender. |
| <code>acceptance.9.961.2</code> | req | 9961.2 |
| <code>acceptance.9.961.2</code> | scenario | Given the built package, when the test imports the root module, then keccak256 of the empty input equals c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470 and the four helper names are functions. |
| <code>acceptance.9.961.2</code> | test | services/agent/sdk/typescript/test/evm-transaction.test.ts::evm_transaction_exports_from_package_root (planned) |
| <code>acceptance.9.961.2</code> | qualification | UNRUN |
| <code>req.9961</code> | ac_2 | keccak256 and the four EIP-1559 helpers are exported from the package root. |
| <code>acceptance.9.962.1</code> | req | 9962.1 |
| <code>acceptance.9.962.1</code> | scenario | Given a receipt signed by key K valid for heights 1 to 100 at height 50, when verifySequencerReceipt is called with K pinned, with no key pinned, with K valid only for 60 to 100, and with a different raw transaction, then it accepts the first and answers unpinned_key, key_out_of_range and hash_mismatch. |
| <code>acceptance.9.962.1</code> | test | services/agent/sdk/typescript/test/sequencer-receipt-verify.test.ts::verify_sequencer_receipt_pinned_key (planned) |
| <code>acceptance.9.962.1</code> | qualification | UNRUN |
| <code>req.9962</code> | ac_1 | verifySequencerReceipt accepts a receipt signed by a pinned key valid at its height and refuses an unpinned key, an out-of-range key and a wrong raw transaction. |
| <code>acceptance.9.962.2</code> | req | 9962.2 |
| <code>acceptance.9.962.2</code> | scenario | Given a validator set of three equal powers, when verifyCommittedReceipt is called with three signatures, with two signatures, and with a landing height one below the receipt height, then it accepts the first and answers insufficient_power and height_regression. |
| <code>acceptance.9.962.2</code> | test | services/agent/sdk/typescript/test/sequencer-receipt-verify.test.ts::verify_committed_receipt_two_thirds (planned) |
| <code>acceptance.9.962.2</code> | qualification | UNRUN |
| <code>req.9962</code> | ac_2 | verifyCommittedReceipt accepts a commit above 2/3 voting power at a height not below the receipt height and refuses exactly 2/3 or a lower landing height. |
| <code>acceptance.9.963.1</code> | req | 9963.1 |
| <code>acceptance.9.963.1</code> | scenario | Given an agentd dispatch with a fixed chain id and nonce, when prepare is called for a transfer, then the answer holds chain_id, nonce, the two fee fields, gas_limit, to, data and a signing_hash equal to the type-2 signing hash of those fields, and has no unsigned_canonical_bytes key. |
| <code>acceptance.9.963.1</code> | test | services/agent/crates/layerx-agentd/tests/prepare_eip1559.rs::prepare_answers_eip1559_fields (planned) |
| <code>acceptance.9.963.1</code> | qualification | UNRUN |
| <code>req.9963</code> | ac_1 | The prepare answer carries the eight EIP-1559 fields and no unsigned_canonical_bytes. |
| <code>acceptance.9.963.2</code> | req | 9963.2 |
| <code>acceptance.9.963.2</code> | scenario | Given the prepared transaction signed with the test key, when it is submitted against a node fake returning a sequencer receipt, then the answer state is Receipted, tx_hash is keccak256 of the raw bytes and signed_receipt equals the fake receipt bytes. |
| <code>acceptance.9.963.2</code> | test | services/agent/crates/layerx-agentd/tests/prepare_eip1559.rs::submit_answers_receipted_with_tx_hash (planned) |
| <code>acceptance.9.963.2</code> | qualification | UNRUN |
| <code>req.9963</code> | ac_2 | A submission of the signed transaction answers Receipted with signed_receipt and tx_hash. |
| <code>acceptance.9.964.1</code> | req | 9964.1 |
| <code>acceptance.9.964.1</code> | scenario | Given a receipt signed by key K valid for heights 1 to 100 at height 50, when verify_sequencer_receipt is called with K pinned, with no key, and with K valid only for 60 to 100, then it returns the receipt for the first and raises the unpinned and out-of-range refusals. |
| <code>acceptance.9.964.1</code> | test | services/agent/sdk/python/tests/test_sequencer_receipt.py::test_verify_sequencer_receipt_pinned_key (planned) |
| <code>acceptance.9.964.1</code> | qualification | UNRUN |
| <code>req.9964</code> | ac_1 | verify_sequencer_receipt accepts a receipt signed by a pinned key valid at its height and refuses an unpinned or out-of-range key. |
| <code>acceptance.9.964.2</code> | req | 9964.2 |
| <code>acceptance.9.964.2</code> | scenario | Given a valid payment result and a receipt signed by an unpinned key, when verify_rpc_payment is called with receipt_keys holding only K, then it raises the unpinned refusal, and with the receipt signed by K it returns the payment. |
| <code>acceptance.9.964.2</code> | test | services/agent/sdk/python/tests/test_sequencer_receipt.py::test_verify_rpc_payment_uses_receipt_keys (planned) |
| <code>acceptance.9.964.2</code> | qualification | UNRUN |
| <code>req.9964</code> | ac_2 | verify_rpc_payment refuses a payment whose receipt is not signed by a pinned receipt key. |
| <code>acceptance.9.965.1</code> | req | 9965.1 |
| <code>acceptance.9.965.1</code> | scenario | Given the regenerated revert.py, when the test reads LXP_RETRYABLE for LXP_PRICEFEED_DELEGATECALL, LXP_PRICEFEED_STATICCALL, LXP_PRICEFEED_KEEPER_MISSING, LXP_PRICEFEED_BATCH_TOO_LARGE, LXP_AGENT_NOT_ACTIVE, LXP_BANK_STATICCALL, LXP_BANK_DELEGATECALL and LXP_BANK_ZERO_VALUE, then each key is present with value False. |
| <code>acceptance.9.965.1</code> | test | services/agent/sdk/python/tests/test_revert_retryable.py::test_site_codes_present_and_not_retryable (planned) |
| <code>acceptance.9.965.1</code> | qualification | UNRUN |
| <code>req.9965</code> | ac_1 | The Python LXP_RETRYABLE holds the eight site codes of task 6.2.1 with value False. |
| <code>acceptance.9.965.2</code> | req | 9965.2 |
| <code>acceptance.9.965.2</code> | scenario | Given the regenerated revert.ts and the registry table, when the test compares the set with the yes rows, then they are equal and none of the eight site codes is in the set. |
| <code>acceptance.9.965.2</code> | test | services/agent/sdk/typescript/test/revert-retryable.test.ts::retryable_set_matches_registry (planned) |
| <code>acceptance.9.965.2</code> | qualification | UNRUN |
| <code>req.9965</code> | ac_2 | The TypeScript LXP_RETRYABLE set equals the codes whose Retryable column is yes. |
| <code>acceptance.9.966.1</code> | req | 9966.1 |
| <code>acceptance.9.966.1</code> | scenario | Given the edited build.rs, when the test reads its text through include_str and the package is built, then the text contains neither lifecycle.rs nor tests/boundary.rs and the build succeeds. |
| <code>acceptance.9.966.1</code> | test | services/agent/qualification/pay2/tests/build_inputs.rs::pay2_build_reads_no_deleted_sources (planned) |
| <code>acceptance.9.966.1</code> | qualification | UNRUN |
| <code>req.9966</code> | ac_1 | The pay2 build script no longer names lifecycle.rs or tests/boundary.rs and the package builds without them. |
| <code>acceptance.9.967.1</code> | req | 9967.1 |
| <code>acceptance.9.967.1</code> | scenario | Given the regenerated files, when the test runs layerx-platform-sdkgen --write-program-contracts into a temporary directory and compares the four outputs, then each is byte-identical to the committed file. |
| <code>acceptance.9.967.1</code> | test | services/agent/sdk/python/tests/test_receipt_generated_current.py::test_receipt_outputs_match_pipeline_lock (planned) |
| <code>acceptance.9.967.1</code> | qualification | UNRUN |
| <code>req.9967</code> | ac_1 | The four generated receipt files equal the generator output for the resealed pipeline lock. |
| <code>acceptance.9.968.1</code> | req | 9968.1 |
| <code>acceptance.9.968.1</code> | scenario | Given an outbox holding a type-2 transaction signed by the owner whose hash equals the served activity id, when the managed agent restores the action, then it is accepted and no layerx_intents symbol is linked. |
| <code>acceptance.9.968.1</code> | test | services/agent/crates/layerx-agentd/tests/managed_agent_evm_tx.rs::managed_agent_accepts_signed_eip1559 (planned) |
| <code>acceptance.9.968.1</code> | qualification | UNRUN |
| <code>req.9968</code> | ac_1 | A managed agent action whose exact signed bytes are a type-2 transaction from the owner is accepted. |
| <code>acceptance.9.968.2</code> | req | 9968.2 |
| <code>acceptance.9.968.2</code> | scenario | Given the same outbox but a served activity id of another hash, when the action is restored, then it returns HumanOperationError::Refused. |
| <code>acceptance.9.968.2</code> | test | services/agent/crates/layerx-agentd/tests/managed_agent_evm_tx.rs::managed_agent_refuses_hash_mismatch (planned) |
| <code>acceptance.9.968.2</code> | qualification | UNRUN |
| <code>req.9968</code> | ac_2 | A type-2 transaction whose hash differs from the served activity id is refused. |
| <code>acceptance.9.969.1</code> | req | 9969.1 |
| <code>acceptance.9.969.1</code> | scenario | Given programs/tests/vectors/snapshot-binding-v2.hex, when the client decodes it, then block height, block hash, app hash and store proof digest equal the vector fields, and a binding with version byte 1 is refused. |
| <code>acceptance.9.969.1</code> | test | services/agent/crates/layerx-client/tests/snapshot_binding_v2.rs::snapshot_binding_v2_decodes_vector (planned) |
| <code>acceptance.9.969.1</code> | qualification | UNRUN |
| <code>req.9969</code> | ac_1 | SnapshotBinding v2 decodes the shared vector to its four fields and a v1 binding is refused. |
| <code>acceptance.9.970.1</code> | req | 9970.1 |
| <code>acceptance.9.970.1</code> | scenario | Given tasks 9.18.5 and this edit applied, when the test checks the two node fixture paths and greps services/agent/tests for lxp_daemon.h, lxp_guarantor.h and layerxd_lni, then both paths are absent and the grep finds nothing. |
| <code>acceptance.9.970.1</code> | test | services/agent/tests/retired/test_boundary_node_retired.py::test_boundary_node_fixtures_absent (planned) |
| <code>acceptance.9.970.1</code> | qualification | UNRUN |
| <code>req.9970</code> | ac_1 | After wave 6 the LNI test node fixtures are deleted and no boundary source names lxp_daemon.h, lxp_guarantor.h or layerxd_lni. |
| <code>acceptance.9.971.1</code> | req | 9971.1 |
| <code>acceptance.9.971.1</code> | scenario | Given the edited script, when the test reads it and runs bash -n on it, then neither name occurs and bash -n exits 0. |
| <code>acceptance.9.971.1</code> | test | services/agent/tools/test_run_real_node_script.py::test_script_names_no_custody_proof (planned) |
| <code>acceptance.9.971.1</code> | qualification | UNRUN |
| <code>req.9971</code> | ac_1 | run-real-node-tests.sh no longer names layerx-custody-proof or LAYERX_CUSTODY_PROOF_BIN and still parses. |
| <code>acceptance.9.972.1</code> | req | 9972.1 |
| <code>acceptance.9.972.1</code> | scenario | Given the edited native_reads_daemon.rs, when the test reads its text through include_str and checks the availability_daemon.rs path, then the text does not contain program-admission.sh and the path is absent. |
| <code>acceptance.9.972.1</code> | test | services/agent/crates/layerx-agentd/tests/no_admission_daemon.rs::native_reads_tests_name_no_admission_daemon (planned) |
| <code>acceptance.9.972.1</code> | qualification | UNRUN |
| <code>req.9972</code> | ac_1 | The agentd native read tests no longer run tests/daemon/program-admission.sh and the client availability daemon test is gone. |
| <code>acceptance.9.973.1</code> | req | 9973.1 |
| <code>acceptance.9.973.1</code> | scenario | Given an RPC fake answering eth_call on 0x101D with two balances, when the program balance operation runs, then it returns both balances and neither manifest nor the lock names layerx-programs-protocol-adapter. |
| <code>acceptance.9.973.1</code> | test | services/agent/crates/layerx-agentd/tests/program_balances_eth_call.rs::agentd_reads_program_balances_through_eth_call (planned) |
| <code>acceptance.9.973.1</code> | qualification | UNRUN |
| <code>req.9973</code> | ac_1 | agentd reads program balances through eth_call on 0x101D and neither manifest names the protocol adapter. |
