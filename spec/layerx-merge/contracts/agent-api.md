# Agent API contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

Agent applications use the merged Paxeer BFT network, EVM chain ID `125` (`0x7d`). The kernel participates in block proposal and execution. Block commit establishes finality. Node JSON-RPC uses the root path; hosted methods are described by the [gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

The agent HTTP envelope is served on a private mutual-TLS listener. Its `POST /rpc` route is not a public node endpoint. The gateway agent route is `POST /v1/agent/rpc`.

## agent-http-healthz

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agent-rpc-get-healthz</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>GET /healthz on the agent RPC listener</code> |

## agent-http-transport

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agent-rpc-post-rpc</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>POST /rpc on the agent RPC mutual-TLS listener (agent_rpc_exchange, services/agent/crates/layerx-agentd/src/main.rs): Agent HTTP envelope version 1, body at most 1048576 bytes; refuses 404, 405, 413, 431 and any LayerX-Tenant, LayerX-Agent or Authorization header</code> (private transport; this is not a public node endpoint) |

## agent-register

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-agent-register</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=agent.register</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9031</code> | agent.register answers with a prepared bindLayerX transaction to the account-binding precompile at 0x1004. |

## approval-approve

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-approval-approve</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/golden/approval.kvx</code> | <code>POST /rpc operation=approval.approve</code> (private transport; this is not a public node endpoint) |

## approval-get

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-approval-get</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/golden/approval.kvx</code> | <code>POST /rpc operation=approval.get</code> (private transport; this is not a public node endpoint) |

## approval-list

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-approval-list</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/golden/approval.kvx</code> | <code>POST /rpc operation=approval.list</code> (private transport; this is not a public node endpoint) |

## approval-reject

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-approval-reject</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/golden/approval.kvx</code> | <code>POST /rpc operation=approval.reject</code> (private transport; this is not a public node endpoint) |

## availability-fetch

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-availability-fetch</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=availability.fetch</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9021</code> | availability returns the transactions root, receipts root and transaction count of the block at the requested height. |
| <code>req.9021</code> | A height above head is refused with Ahead and a block the node no longer returns is reported as Pruned. |

## budget-create

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-create</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.create</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9033</code> | create_protocol_budget prepares a 0x101B createBudget call with the requested ceiling, asset and expiry. |

## budget-fund

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-fund</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.fund</code> (private transport; this is not a public node endpoint) |

## budget-list

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-list</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.list</code> (private transport; this is not a public node endpoint) |

## budget-reconciliation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-reconciliation</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.reconciliation</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9032</code> | consumption_updates counts the projected fee of a pending transaction against the daemon limit. |

## budget-revoke

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-revoke</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.revoke</code> (private transport; this is not a public node endpoint) |

## budget-state

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-budget-state</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=budget.state</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9032</code> | ProtocolBudgetState is decoded from a getBudget eth_call result at a committed height. |
| <code>req.9034</code> | budget.state answers with the getBudget result and achieved rung Committed. |
| <code>req.9006</code> | ProtocolBudgetView::from_proven refuses a record with achieved_rung below Committed in services/agent/crates/layerx-agent-api/src/identity.rs. |

## capability-attenuate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-capability-attenuate</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=capability.attenuate</code> (private transport; this is not a public node endpoint) |

## capability-create

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-capability-create</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=capability.create</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9034</code> | capability.create refuses a capability wider than the session's on-chain grant with reason capability.exceeds_grant. |

## capability-list

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-capability-list</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=capability.list</code> (private transport; this is not a public node endpoint) |

## capability-revoke

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-capability-revoke</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=capability.revoke</code> (private transport; this is not a public node endpoint) |

## export-offline

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-export-offline</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=export.offline</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9027</code> | An offline export v2 verifies offline using only the bundled receipt keys and validator set. |
| <code>req.9027</code> | The serialized export contains no settlement domain or certificate field. |
| <code>req.9005</code> | OfflineExport carries facts, receipts and proofs at one height and no certificates field in services/agent/crates/layerx-agent-api/src/read.rs. |

## faucet-claim

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-faucet-claim</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=faucet.claim</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9009</code> | Posting operation faucet.claim is refused with UnavailableCapability by RETIRED_OPERATIONS (services/agent/crates/layerx-agentd/src/agent_rpc.rs) and no Operation variant's name() (services/agent/crates/layerx-agent-api/src/operation_generated.rs) returns faucet.claim. |

## human-owner-healthz

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agentd-human-owner-get-healthz</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs</code> | <code>GET /healthz in human-owner mode (LAYERX_AGENT_MODE=human-owner); every other path 404 not_found</code> |

## mcp-invoke

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-mcp-invoke</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=mcp.invoke</code> (private transport; this is not a public node endpoint) |

## native-reads

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agentd-serve-get-v1-reads-availability</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs</code> | <code>GET /v1/reads/availability/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-checkpoint</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs</code> | <code>GET /v1/reads/checkpoint/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-history</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs</code> | <code>GET /v1/reads/history/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-proof</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs</code> | <code>GET /v1/reads/proof/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |
| <code>interface.9.http-agentd-serve-get-v1-reads-receipt</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/read/native.rs</code> | <code>GET /v1/reads/receipt/{selector} on the serve listener (dispatched from services/agent/crates/layerx-agentd/src/main.rs) -&gt; verified read JSON, 400 invalid_read, 413 read_too_large, 503 verified_read_unavailable</code> |

## policy-dry-run

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-policy-dry-run</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=policy.dry_run</code> (private transport; this is not a public node endpoint) |

## prepare

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-prepare</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>POST /rpc operation=prepare</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9010</code> | A prepared layerxledger transfer yields an Eip1559Call whose signing_hash equals the golden keccak preimage for chain id, pending nonce, fees, gas, target 0x101A and calldata (services/agent/crates/layerx-crypto/src/evm_transaction.rs). |
| <code>req.9010</code> | prepare returns StaleNonce when the caller's expected nonce differs from eth_getTransactionCount pending at services/agent/crates/layerx-agentd/src/prepare/mod.rs. |
| <code>req.9010</code> | max_fee_per_gas is twice the head base fee plus the priority fee and a request ceiling below that is refused with InvalidDefaults. |
| <code>req.9002</code> | The agent envelope decoder maps known names to Operation values. Unknown names return ProtocolIncompatibility with reason envelope.unknown_operation; the retired faucet.claim name returns UnavailableCapability. |

## program-activity

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-activity</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.activity</code> (private transport; this is not a public node endpoint) |

## program-balances

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agentd-serve-get-v1-programs-balances</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>GET /v1/programs/{program_id}/balances on the serve listener -&gt; balance JSON, 400 invalid_program, 503 program_state_unavailable</code> |

## program-call

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-call</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.call</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9036</code> | program.call answers with the transaction hash and achieved rung of the 0x101D call. |

## program-deploy

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-deploy</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.deploy</code> (private transport; this is not a public node endpoint) |

## program-discover

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-discover</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.discover</code> (private transport; this is not a public node endpoint) |

## program-interface

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-interface</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.interface</code> (private transport; this is not a public node endpoint) |

## program-receipt

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-receipt</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.receipt</code> (private transport; this is not a public node endpoint) |

## program-simulate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-simulate</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.simulate</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9035</code> | ProgramOperations::simulate issues eth_call to 0x101D at the pending tag and returns the decoded result. |
| <code>req.9035</code> | An execution revert from eth_call maps to ProgramOperationError with the revert data preserved. |

## program-upgrade

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-upgrade</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.upgrade</code> (private transport; this is not a public node endpoint) |

## program-wind-down

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-program-wind-down</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>POST /rpc operation=program.wind-down</code> (private transport; this is not a public node endpoint) |

## project

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-project</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=project</code> (private transport; this is not a public node endpoint) |

## read-account

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-account</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.account</code> (private transport; this is not a public node endpoint) |

## read-balance

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-balance</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.balance</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9019</code> | A pending balance read returns the eth_getBalance amount with value_height equal to chain_head plus one and relative_to pending. |
| <code>req.9019</code> | A balance read at a height above chain_head is refused with reason read.height_ahead. |
| <code>req.9024</code> | operation read.balance returns amount, chain_head and value_height and no sequence fields. |
| <code>req.9005</code> | VerifiedRead exposes achieved_rung and a Freshness of chain_head, value_height and relative_to, as declared in services/agent/schema/agent-api/read.kvx and implemented in services/agent/crates/layerx-agent-api/src/read.rs. |

## read-batch

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-batch</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.batch</code> (private transport; this is not a public node endpoint) |

## read-checkpoint

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-checkpoint</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.checkpoint</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9022</code> | A checkpoint read returns the block hash and app hash at the height with a commit carrying more than two thirds of voting power and rung Committed. |
| <code>req.9022</code> | A commit carrying two thirds or less of voting power is refused with CheckpointReadError::Commit. |

## read-history

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-history</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.history</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9020</code> | A history cursor round-trips the (block_height, tx_index, log_index) triple through to_hex and from_hex and refuses another length (services/agent/crates/layerx-agentd/src/read/history.rs). |
| <code>req.9020</code> | Two consecutive history pages joined by next_cursor return every item once and in block order. |
| <code>req.9024</code> | operation read.history returns items and a next_cursor triple. |
| <code>req.9005</code> | HistorySelector validates from_height <= to_height and a nonzero page limit and HistoryValue.next_cursor is the (block_height, tx_index, log_index) triple of services/agent/schema/agent-api/read.kvx. |

## read-module-state

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-module-state</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.module_state</code> (private transport; this is not a public node endpoint) |

## read-proof-bundle

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-read-proof-bundle</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>POST /rpc operation=read.proof_bundle</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9023</code> | A receipt proof bundle v2 carries the receipt, an inclusion path that recomputes the block receipts root and rung Receipted. |
| <code>req.9023</code> | An account proof bundle is built only against a committed block and is refused before commit with reason read.not_committed. |
| <code>req.9024</code> | operation read.proof_bundle answers a receipt target with a layerx-receipt-proof-v2 bundle at rung Receipted. |
| <code>req.9005</code> | A proof bundle record for an Activity or Receipt target requires Rung::Receipted and an Account target requires Rung::Committed in services/agent/crates/layerx-agent-api/src/read.rs. |
| <code>req.9007</code> | operation.read.proof_bundle declares layerx-receipt-proof-v2 with Receipted for activity and receipt targets and Committed for account targets (services/agent/schema/agent-api/read.kvx). |

## record-agenthttpcredential

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-agenthttpcredential</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.AgentHttpCredential] in v1.kvx: fields = ["tenant:utf8","session_id:hex32","token_id:hex32","generation:Sequence"]</code> |

## record-agenthttpenvelope

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-agenthttpenvelope</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.AgentHttpEnvelope] in v1.kvx: fields = ["version:u8","request_id:Sequence","operation:utf8","request:canonical-map","credential:AgentHttpCredential?","idempotency_key:hex32?"]</code> |

## record-agenthttperror

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-agenthttperror</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.AgentHttpError] in v1.kvx: fields = ["class:ErrorClass","protocol_result_code:i32?","retriability:Retriability","request_id:Sequence","reason:utf8"]</code> |

## record-agenthttpsuccess

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-agenthttpsuccess</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.AgentHttpSuccess] in v1.kvx: fields = ["request_id:Sequence","value:canonical-map","verification_status:VerificationStatus"]</code> |

## record-contractversion

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-contractversion</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.ContractVersion] in v1.kvx: fields = ["major:u16","minor:u16"]</code> |

## record-nativeactivityv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeactivityv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeActivityV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeActivity"</code> |

## record-nativeapprovaldecisionv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeapprovaldecisionv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeApprovalDecisionV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalDecisionV1"</code> |

## record-nativeapprovalgetv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalgetv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeApprovalGetV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalGetV1"</code> |

## record-nativeapprovallistresultv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistresultv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeApprovalListResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalListResultV1"</code> |

## record-nativeapprovallistv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeapprovallistv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeApprovalListV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalListV1"</code> |

## record-nativeapprovalresultv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeapprovalresultv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeApprovalResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeApprovalResultV1"</code> |

## record-nativedisclosurerequestv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativedisclosurerequestv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeDisclosureRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeDisclosureRequestV1"</code> |

## record-nativedisclosureresultv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativedisclosureresultv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeDisclosureResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeDisclosureResultV1"</code> |

## record-nativeeffectpreparerequestv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeeffectpreparerequestv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeEffectPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeEffectPrepareRequestV1"</code> |

## record-nativelocalgrantconsentv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativelocalgrantconsentv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeLocalGrantConsentV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeLocalGrantConsentV1"</code> |

## record-nativepreparationpurposev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativepreparationpurposev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativePreparationPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePreparationPurposeV1"</code> |

## record-nativepreparerequestv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativepreparerequestv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativePrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePrepareRequestV1"</code> |

## record-nativeprepareresultv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeprepareresultv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativePrepareResultV1] in v1.kvx: rust = "layerx_agent_api::identity::NativePrepareResultV1"</code> |

## record-nativeregistrationpreparerequestv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativeregistrationpreparerequestv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeRegistrationPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeRegistrationPrepareRequestV1"</code> |

## record-nativesendpreparerequestv1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativesendpreparerequestv1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeSendPrepareRequestV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeSendPrepareRequestV1"</code> |

## record-nativesendpurposev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-nativesendpurposev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.NativeSendPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::NativeSendPurposeV1"</code> |

## record-signednativepreparationpurposev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-signednativepreparationpurposev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.SignedNativePreparationPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::SignedNativePreparationPurposeV1"</code> |

## record-signednativesendpurposev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-signednativesendpurposev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.SignedNativeSendPurposeV1] in v1.kvx: rust = "layerx_agent_api::identity::SignedNativeSendPurposeV1"</code> |

## record-versionrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-versionrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.VersionRequest] in v1.kvx: fields = ["request_id:Sequence","supported:ContractVersion"]</code> |

## record-versionresponse

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-record-versionresponse</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[record.VersionResponse] in v1.kvx: fields = ["request_id:Sequence","contract:ContractVersion","node_interface_major:u16"]</code> |

## scalar-amount

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-scalar-amount</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[scalar.Amount] in v1.kvx: rust = "u128"</code> |

## scalar-budgetlimit

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-scalar-budgetlimit</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[scalar.BudgetLimit] in v1.kvx: rust = "u128"</code> |

## scalar-sequence

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-scalar-sequence</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[scalar.Sequence] in v1.kvx: rust = "u64"</code> |

## scalar-timestampseconds

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-v1-scalar-timestampseconds</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>[scalar.TimestampSeconds] in v1.kvx: rust = "u64"</code> |

## serve-errors

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-http-serve-body-too-large</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error body_too_large</code> |
| <code>interface.9.err-http-serve-headers-too-large</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error headers_too_large</code> |
| <code>interface.9.err-http-serve-invalid-program</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error invalid_program</code> |
| <code>interface.9.err-http-serve-invalid-read</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error invalid_read</code> |
| <code>interface.9.err-http-serve-invalid-request</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error invalid_request</code> |
| <code>interface.9.err-http-serve-not-found</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error not_found</code> |
| <code>interface.9.err-http-serve-program-state-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error program_state_unavailable</code> |
| <code>interface.9.err-http-serve-read-too-large</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error read_too_large</code> |
| <code>interface.9.err-http-serve-request-timeout</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error request_timeout</code> |
| <code>interface.9.err-http-serve-unauthorized</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error unauthorized</code> |
| <code>interface.9.err-http-serve-verified-read-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>HTTP serve listener error verified_read_unavailable</code> |

## serve-healthz

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-agentd-serve-get-healthz</code> | <code>http_route</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>GET /healthz on the bearer-authenticated serve listener (route_request, services/agent/crates/layerx-agentd/src/main.rs; Bearer check services/agent/crates/layerx-agentd/src/main.rs; GET only services/agent/crates/layerx-agentd/src/main.rs) -&gt; 200 or 503 {ready,transport_ready,verified_reads_ready,tenant_writes}</code> |

## session-close

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-session-close</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=session.close</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9030</code> | A revokeGrant receipt for a session's grant invalidates every pending preparation of that session. |

## session-list

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-session-list</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=session.list</code> (private transport; this is not a public node endpoint) |

## session-open

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-session-open</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=session.open</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9029</code> | A provisioned session key exposes the 20-byte address derived from its secp256k1 public key. |
| <code>req.9031</code> | session.open answers with a prepared grantSession transaction naming the provisioned session address. |

## session-refresh

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-session-refresh</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=session.refresh</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9030</code> | session.refresh is refused with reason session.grant_missing when no grantSession receipt exists for the session address. |

## sign

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-sign</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>POST /rpc operation=sign</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9011</code> | A self-signed preparation carries a 65-byte signature whose recovered sender equals the session key's EVM address (services/agent/crates/layerx-crypto/src/evm_transaction.rs). |
| <code>req.9011</code> | attach_external_signature refuses a 65-byte signature that recovers to an address other than the prepared actor (services/agent/crates/layerx-agentd/src/sign.rs). |

## submit

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-submit</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>POST /rpc operation=submit</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9011</code> | verify_before_submit refuses signed bytes whose decoded call differs from the prepared call (services/agent/crates/layerx-agentd/src/sign.rs). |
| <code>req.9012</code> | operation submit answers with the transaction hash returned by eth_sendRawTransaction and SubmissionState Submitted (services/agent/schema/agent-api/write.kvx). |
| <code>req.9014</code> | Outbox::enqueue keys a record by the keccak hash of its raw bytes and refuses a second enqueue of identical bytes (services/agent/crates/layerx-agentd/src/outbox/mod.rs). |
| <code>req.9015</code> | Store::sweep removes records whose recorded head height is more than retention_blocks below the current height (services/agent/crates/layerx-agentd/src/idempotency.rs). |
| <code>req.9066</code> | Resubmitting the identical signed raw transaction after a restart records one transaction hash in the outbox. |

## subscription-acknowledge

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-acknowledge</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.acknowledge</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9037</code> | A subscription acknowledged at (12, 1, 0) resumes at that triple after the store is reopened. |
| <code>req.9039</code> | subscription.acknowledge refuses a cursor beyond the last delivered position with reason subscription.cursor_ahead. |

## subscription-create

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-create</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.create</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9038</code> | Logs delivered out of order by eth_subscribe are handed to subscribers in (block_height, tx_index, log_index) order. |
| <code>req.9006</code> | ReceiptReference::Verified carries receipt_ref and rung as services/agent/schema/agent-api/stream.kvx requires. |

## subscription-delete

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-delete</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.delete</code> (private transport; this is not a public node endpoint) |

## subscription-health

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-health</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.health</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9037</code> | A persisted LXS2 record is refused with reason subscription.cursor_reset_required. |
| <code>req.9038</code> | A newHeads jump from 10 to 13 triggers a backfill of blocks 11 and 12 through px_getHistory. |
| <code>req.9039</code> | subscription.health reports both the last delivered triple and the ingest watermark triple. |

## subscription-list

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-list</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.list</code> (private transport; this is not a public node endpoint) |

## subscription-pause

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-pause</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.pause</code> (private transport; this is not a public node endpoint) |

## subscription-resume

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-subscription-resume</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>POST /rpc operation=subscription.resume</code> (private transport; this is not a public node endpoint) |

## tenant-readiness

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-tenant-readiness</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>POST /rpc operation=tenant.readiness</code> (private transport; this is not a public node endpoint) |

## track

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-track</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>POST /rpc operation=track</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9012</code> | operation track looks the submission up by its 32-byte transaction hash and refuses a 31-byte key with invalid_request. |
| <code>req.9014</code> | legal_transition admits Executed to Committed and refuses Submitted to Committed and any transition out of Committed (services/agent/crates/layerx-agentd/src/outbox/mod.rs). |
| <code>req.9014</code> | A submission whose nonce was consumed by another transaction ends Failed with reason submission.nonce_consumed. |
| <code>req.9016</code> | resolve_unknown moves an Unknown submission to Executed when pax_sequencerReceipt returns a receipt signed by a published receipt key (services/agent/crates/layerx-agentd/src/outbox/unknown.rs). |
| <code>req.9016</code> | resolve_unknown marks the submission Failed with reason submission.nonce_consumed when no receipt exists and the pending nonce has advanced past it. |

## type-apierror

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-apierror</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.ApiError] in errors.kvx: required = ["class","protocol_result_code","retriability","request_id","reason"]</code> |

## type-approvaldecisionoutcome

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-approvaldecisionoutcome</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.ApprovalDecisionOutcome] in approval.kvx: variants = ["Granted","Rejected","Expired","Defective","AlreadyDecided","Conflict"]</code> |

## type-approvallifecycleevent

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-approvallifecycleevent</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.ApprovalLifecycleEvent] in approval.kvx: variants = ["Created","Granted","Rejected","Expired","Defective"]</code> |

## type-approvalrecord

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-approvalrecord</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.ApprovalRecord] in approval.kvx: required = ["approval_id","tenant","held_activity","canonical_bytes_digest","hold_reason","created_at","expires_at","state"]</code> |

## type-approvalstate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-approvalstate</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.ApprovalState] in approval.kvx: variants = ["Held","Granted","Rejected","Expired","Defective"]</code> |

## type-authoritydescription

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-authoritydescription</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.AuthorityDescription] in identity.kvx: required = ["tenant","agent_did","authority_ref","protocol_authority"]</code> |

## type-authorityresponse

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-authorityresponse</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.AuthorityResponse] in identity.kvx: required = ["authority","value"]</code> |

## type-budgetauthorization

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-budgetauthorization</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.BudgetAuthorization] in identity.kvx: required = ["preparation_ref","signature"]</code> |

## type-budgetenforcement

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-budgetenforcement</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.BudgetEnforcement] in identity.kvx: variants = ["ProtocolBudget","DaemonLimit"]</code> |

## type-budgetrecord

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-budgetrecord</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.BudgetRecord] in identity.kvx: required = ["enforcement","budget_id","head","activity_id"]</code> |

## type-budgetrecords

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-budgetrecords</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.BudgetRecords] in identity.kvx: required = ["budgets"]</code> |

## type-budgetstate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-budgetstate</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.BudgetState] in identity.kvx: required = ["record","balance","proven_head","activity_id"]</code> |

## type-capabilitydimensions

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-capabilitydimensions</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.CapabilityDimensions] in identity.kvx: required = ["activity_types","counterparties","assets","amount_ceilings","rate_ceilings","purpose_constraints","expiry"]</code> |

## type-capabilityrecord

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-capabilityrecord</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.CapabilityRecord] in identity.kvx: required = ["capability_id","parent_id","tenant","agent_did","dimensions","state","created_at_ms","created_at_sequence","revoked_at_ms","revoked_at_sequence"]</code> |

## type-capabilityrecords

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-capabilityrecords</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.CapabilityRecords] in identity.kvx: required = ["capabilities"]</code> |

## type-capabilitystate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-capabilitystate</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.CapabilityState] in identity.kvx: variants = ["active","revoked","expired"]</code> |

## type-delivery

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-delivery</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.Delivery] in stream.kvx: variants = ["Event","Gap","Truncated"]</code> |

## type-disclosure

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-disclosure</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.Disclosure] in write.kvx: required = ["canonical_digest","activity_type","actor","authority","counterparties","amounts","asset","fee_limit","expiry","idempotency_key"]</code> |

## type-errorclass

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-errorclass</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.ErrorClass] in errors.kvx: variants = ["TransportFailure","Deadline","ProtocolIncompatibility","UnavailableCapability","CoreRejection","VerificationFailure","PolicyRefusal","CapabilityRefusal","BudgetRefusal","RateLimit","IdempotencyConflict","InternalFault"]</code> |

## type-eventdelivery

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-eventdelivery</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.EventDelivery] in stream.kvx: required = ["event_identity","event_bytes","deduplication_id","cursor","receipt_reference"]</code> |

## type-factref

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-factref</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.FactRef] in read.kvx: grammar = ["receipt:&lt;activity_id&gt;","activity:&lt;activity_id&gt;","state:&lt;activity_id&gt;:&lt;account_id&gt;","checkpoint:&lt;activity_id&gt;:&lt;height&gt;"]</code> |

<a id="type-faucetgrant"></a>

## type-feeprojection

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-feeprojection</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.FeeProjection] in read.kvx: required = ["request","parameter_version","fee","gas_units","denom","canonical_schedule","snapshot_height","snapshot_state_root"]</code> |

## type-feeprojectionrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-feeprojectionrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.FeeProjectionRequest] in read.kvx: required = ["protocol_activity_type","canonical_bytes","execution_units","storage_units"]</code> |

## type-freshness

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-freshness</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.Freshness] in read.kvx: required = ["chain_head","value_height","relative_to"]</code> |

## type-gapnotice

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-gapnotice</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.GapNotice] in stream.kvx: required = ["missing_first","missing_last","backfill_cursor","backfill_attempted"]</code> |

## type-historycursor

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-historycursor</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.HistoryCursor] in read.kvx: fields = ["blockHeight","txIndex","logIndex"]</code> |

## type-historyitem

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-historyitem</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.HistoryItem] in read.kvx: required = ["height","txIndex","logIndex","kind","canonical","proof"]</code> |

## type-historykind

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-historykind</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.HistoryKind] in read.kvx: variants = ["Activity","Receipt","Event"]</code> |

## type-holdreason

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-holdreason</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.HoldReason] in approval.kvx: required = ["code","message"]</code> |

## type-idempotentmutation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-idempotentmutation</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.IdempotentMutation] in errors.kvx: required = ["request_id","key","body_digest","operation"]</code> |

## type-legacypolicydryrun

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-legacypolicydryrun</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.LegacyPolicyDryRun] in identity.kvx: required = ["context","canonical_intent"]</code> |

## type-mcpexternalsignaturev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-mcpexternalsignaturev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.McpExternalSignatureV1] in write.kvx: operation = "sign"</code> |

## type-mcpinvokerequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-mcpinvokerequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.McpInvokeRequest] in identity.kvx: required = ["tool","arguments"]</code> |

## type-mcpnativewritev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-mcpnativewritev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.McpNativeWriteV1] in write.kvx: variant = "native_write_v1"</code> |

## type-nativedisclosurev1

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-nativedisclosurev1</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.NativeDisclosureV1] in write.kvx: operation = "prepare"</code> |

## type-nativeprogramcallfields

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallfields</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramCallFields] in programs.kvx: required = ["guest_abi","entrypoint","capabilities_hex","access_declaration_hex","response_capacity","resources"]</code> |

## type-nativeprogramcallrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramcallrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramCallRequest] in programs.kvx: required = ["payload_encoding","program_id","calldata","budget","signed_activity","native_call"]</code> |

## type-nativeprogramdeploy

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeploy</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramDeploy] in programs.kvx: required = ["program_id","guest_abi","policy","authority","new_hash","wasm"]</code> |

## type-nativeprogramdeployrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramdeployrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramDeployRequest] in programs.kvx: required = ["signed_activity"]</code> |

## type-nativeprogramupgrade

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgrade</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramUpgrade] in programs.kvx: required = ["program_id","guest_abi","old_hash","new_hash","migration_hook","clear_interface","wasm"]</code> |

## type-nativeprogramupgraderequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramupgraderequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramUpgradeRequest] in programs.kvx: required = ["signed_activity"]</code> |

## type-nativeprogramwinddown

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddown</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramWindDown] in programs.kvx: required = ["program_id","operation"]</code> |

## type-nativeprogramwinddownrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-nativeprogramwinddownrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.NativeProgramWindDownRequest] in programs.kvx: required = ["signed_activity"]</code> |

## type-nativeregistrationcapability

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-nativeregistrationcapability</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.NativeRegistrationCapability] in identity.kvx: activity_types = "Every capability in the native active chain lists Asset module 1 ordinal 1."</code> |

## type-policydryrunresult

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-policydryrunresult</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.PolicyDryRunResult] in identity.kvx: required = ["outcome","policy_version","matched_rules","deciding_rule","reason","mode","authority_statement"]</code> |

## type-policyintent

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-policyintent</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.PolicyIntent] in identity.kvx: magic_version = "8 bytes 4c58504400000001"</code> |

## type-preparewithcapability

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-preparewithcapability</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.PrepareWithCapability] in write.kvx: required = ["request","capability_id"]</code> |

## type-programactivityselector

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programactivityselector</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramActivitySelector] in programs.kvx: required = ["activity_id","requested_rung"]</code> |

## type-programcallbudget

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programcallbudget</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramCallBudget] in programs.kvx: required = ["fuel","fee_limit"]</code> |

## type-programcallrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programcallrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramCallRequest] in programs.kvx: profile = "retained_legacy_call"</code> |

## type-programcapability

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programcapability</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramCapability] in programs.kvx: variants = ["storage_read","storage_write","transfer","emit_event","compose"]</code> |

<a id="type-programexecutionevidence"></a>

<a id="type-programexecutionv4"></a>

<a id="type-programexecutionv5"></a>

<a id="type-programfailure"></a>

## type-programguestabi

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programguestabi</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramGuestAbi] in programs.kvx: variants = ["ABI_V1_VERSION","ABI_V2_VERSION","ABI_V3_VERSION","ABI_V4_VERSION","ABI_V5_VERSION"]</code> |

<a id="type-programlifecycleerror"></a>

<a id="type-programlifecycleerrorenvelope"></a>

<a id="type-programlifecyclereceipt"></a>

<a id="type-programlifecyclereceiptenvelope"></a>

## type-programlifecycleresponse

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programlifecycleresponse</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramLifecycleResponse] in programs.kvx: variants = ["ProgramSubmission"]</code> |

<a id="type-programlifecycleresult"></a>

<a id="type-programlifecycleresultenvelope"></a>

<a id="type-programlifecycleunknown"></a>

<a id="type-programoutcome"></a>

## type-programreceiptresponse

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programreceiptresponse</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramReceiptResponse] in programs.kvx: variants = ["ProgramSubmission"]</code> |

## type-programreceiptselector

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programreceiptselector</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramReceiptSelector] in programs.kvx: required = ["idempotency_key","expected_activity_id","requested_rung"]</code> |

## type-programselector

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programselector</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramSelector] in programs.kvx: required = ["program_id","requested_rung"]</code> |

## type-programsimulation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programsimulation</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramSimulation] in programs.kvx: required = ["committed","program_id","terminal_result","terminal_kind","response","block","rung"]</code> |

<a id="type-programsource"></a>

## type-programsubmission

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-programsubmission</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.ProgramSubmission] in programs.kvx: required = ["tx_hash","submission_ref","state","rung"]</code> |

## type-projectionresult

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-projectionresult</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.ProjectionResult] in read.kvx: required = ["projected","rationale","observed_freshness"]</code> |

## type-proofbundle

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-proofbundle</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.ProofBundle] in read.kvx: format = "layerx-receipt-proof-v2"</code> |

## type-proofbundletarget

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-proofbundletarget</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.ProofBundleTarget] in read.kvx: variants = ["Activity","AccountState","Receipt"]</code> |

## type-receiptkey

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-receiptkey</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.ReceiptKey] in read.kvx: required = ["keyId","publicKey","firstHeight","lastHeight"]</code> |

## type-receiptreference

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-receiptreference</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.ReceiptReference] in stream.kvx: variants = ["None","Verified"]</code> |

## type-retriability

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-retriability</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.Retriability] in errors.kvx: variants = ["Terminal","Retriable"]</code> |

## type-rung

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-rung</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.Rung] in errors.kvx: variants = ["Submitted","Receipted","Committed"]</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.8253</code> | decodeDaemonSubmission validates the transaction hash, submission reference, state, rung, evidence and transitions. Receipt evidence must match the submission rung and transaction. |
| <code>req.9001</code> | layerx_agent_api::verify exports Rung with Submitted, Receipted and Committed and RungStatus with Achieved and Below. The wire schema in services/agent/schema/agent-api/errors.kvx declares VerificationStatus with Achieved and Unverified. |
| <code>req.9008</code> | VerificationStatus aliases RungStatus in services/agent/crates/layerx-agent-api/src/error.rs. Achieved carries a rung; Below carries requested and achieved rungs and a reason. The JSON response serializes Below with state unverified and requested, achieved and reason fields. |

## type-sessioncontext

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-sessioncontext</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.SessionContext] in identity.kvx: required = ["tenant","agent_did","authority_ref","permitted_activity_types","expiry","client","policy_version"]</code> |

<a id="type-settlementdomain"></a>

## type-structuredactivitydisclosure

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-approval-type-structuredactivitydisclosure</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/approval.kvx</code> | <code>[type.StructuredActivityDisclosure] in approval.kvx: required = ["canonical_digest","activity_type","actor","authority","counterparties","amounts","asset","fee_limit","expiry","idempotency_key"]</code> |

## type-submissionstate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-submissionstate</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.SubmissionState] in write.kvx: variants = ["Prepared","Signed","Queued","Submitted","Unknown","Executed","Committed","Failed","Expired"]</code> |

## type-subscriptionfilter

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-subscriptionfilter</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.SubscriptionFilter] in stream.kvx: required = ["agents","accounts","activity_types","modules","assets","counterparties","result_classes"]</code> |

## type-subscriptionhealth

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealth</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.SubscriptionHealth] in stream.kvx: required = ["target","last_acknowledged","last_delivery_at","pending_backfill"]</code> |

## type-subscriptionhealthtarget

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-subscriptionhealthtarget</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.SubscriptionHealthTarget] in stream.kvx: required = ["scope","subscription_id"]</code> |

## type-subscriptionrecord

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-subscriptionrecord</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.SubscriptionRecord] in stream.kvx: required = ["subscription_id","scope","filter","start","last_acknowledged","delivery_target","paused"]</code> |

## type-subscriptionscope

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-subscriptionscope</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.SubscriptionScope] in stream.kvx: required = ["tenant","agent","capability"]</code> |

## type-tenantreadiness

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-tenantreadiness</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.TenantReadiness] in identity.kvx: required = ["transport_ready:bool","verified_reads_ready:bool","writes_admitted:bool","recovery_reason:optional&lt;TenantRecoveryReason&gt;"]</code> |

## type-tenantreadinessrequest

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-tenantreadinessrequest</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.TenantReadinessRequest] in identity.kvx: required = []</code> |

## type-tenantrecoveryreason

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-identity-type-tenantrecoveryreason</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/identity.kvx</code> | <code>[type.TenantRecoveryReason] in identity.kvx: variants = ["recovery_pending","store_unavailable","store_refused","budget_state_unverified","receipt_evidence_missing","durable_recovery_failed","spend_unreconciled","transport_unavailable","verified_read_unavailable"]</code> |

## type-transition

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-write-type-transition</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>[type.Transition] in write.kvx: required = ["from","to","cause","at"]</code> |

## type-truncationnotice

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-stream-type-truncationnotice</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/stream.kvx</code> | <code>[type.TruncationNotice] in stream.kvx: required = ["requested_first","oldest_available","resume_cursor"]</code> |

## type-verificationstatus

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-errors-type-verificationstatus</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>[type.VerificationStatus] in errors.kvx: variants = ["Achieved","Unverified"]</code> |

## type-verifiedprogramdiscovery

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-verifiedprogramdiscovery</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.VerifiedProgramDiscovery] in programs.kvx: required = ["value","achieved_verification_level","freshness"]</code> |

## type-verifiedprograminterface

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-programs-type-verifiedprograminterface</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/programs.kvx</code> | <code>[type.VerifiedProgramInterface] in programs.kvx: required = ["value","achieved_verification_level","freshness"]</code> |

## type-verifiedread

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.proto-agent-api-read-type-verifiedread</code> | <code>proto_message</code> | <code>services/agent/schema/agent-api/read.kvx</code> | <code>[type.VerifiedRead] in read.kvx: required = ["value","achieved_verification_level","freshness"]</code> |

## wait

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.http-rpc-wait</code> | <code>http_route</code> | <code>services/agent/schema/agent-api/write.kvx</code> | <code>POST /rpc operation=wait</code> (private transport; this is not a public node endpoint) |

| Requirement | Behavior |
| --- | --- |
| <code>req.9012</code> | operation wait with requested rung Committed returns achieved rung Receipted with a Below status when the deadline passes before commit. |
| <code>req.9017</code> | A receipt whose signature recovers to a key absent from px_getReceiptKeys leaves the submission at Submitted. |
| <code>req.9017</code> | Committed is reported only when the block commit carries strictly more than two thirds of voting power. |

## wiring-integration

| Requirement | Behavior |
| --- | --- |
| <code>req.9961</code> | encodeUnsignedEip1559, eip1559SigningHash and recoverEip1559Sender round-trip a signed type-2 transaction to its sender. |
| <code>req.9961</code> | keccak256 and the four EIP-1559 helpers are exported from the package root. |
| <code>req.9962</code> | verifySequencerReceipt accepts a receipt signed by a pinned key valid at its height and refuses an unpinned key, an out-of-range key and a wrong raw transaction. |
| <code>req.9962</code> | verifyCommittedReceipt accepts a commit above 2/3 voting power at a height not below the receipt height and refuses exactly 2/3 or a lower landing height. |
| <code>req.9963</code> | The prepare answer carries the eight EIP-1559 fields and no unsigned_canonical_bytes. |
| <code>req.9963</code> | A submission of the signed transaction answers Receipted with signed_receipt and tx_hash. |
| <code>req.9964</code> | verify_sequencer_receipt accepts a receipt signed by a pinned key valid at its height and refuses an unpinned or out-of-range key. |
| <code>req.9964</code> | verify_rpc_payment refuses a payment whose receipt is not signed by a pinned receipt key. |
| <code>req.9965</code> | The TypeScript LXP_RETRYABLE set equals the codes whose Retryable column is yes. |
| <code>req.9968</code> | A managed agent action whose exact signed bytes are a type-2 transaction from the owner is accepted. |
| <code>req.9968</code> | A type-2 transaction whose hash differs from the served activity id is refused. |
| <code>req.9969</code> | SnapshotBinding v2 decodes the shared vector to its four fields and a v1 binding is refused. |
