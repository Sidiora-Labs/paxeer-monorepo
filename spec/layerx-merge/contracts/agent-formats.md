# Agent formats contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## agent-api-golden

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-golden-version-request-hex</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/schema/agent-api/golden/version-request.hex:1</code> | <code>agent-api version handshake golden version-request.hex</code> |
| <code>interface.9.fmt-agent-api-golden-version-response-hex</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/schema/agent-api/golden/version-response.hex:1</code> | <code>agent-api version handshake golden version-response.hex</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-golden-version-request-hex</code> | notes | Stale task 12.5.103 keeps these goldens byte-exact; no change to the bytes is planned. |
| <code>interface.9.fmt-agent-api-golden-version-response-hex</code> | notes | Stale task 12.5.103 keeps these goldens byte-exact; no change to the bytes is planned. |

## agent-api-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-request-frame</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/v1.kvx:11</code> | <code>agent-api request: contract_major:u16be contract_minor:u16be request_id:u64be operation:utf8 payload:canonical-map</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-request-frame</code> | notes | Schema major 2 minor 0 (services/agent/schema/agent-api/v1.kvx:3-4); stale task 11.5.46 re-targets v1.kvx off the settlement domain; 13.6.3 bootstraps agentd at major 2. |

Contract context <code>contract_context.9.agent-rpc</code>:

| Field | Frozen rule |
|---|---|
| interface | HTTP POST /rpc of layerx-agentd carrying the agent API envelope version 1 and contract major 2 operations |
| storage | agentd store: outbox, idempotency window, receipt store and subscription records |
| errors | error classes of services/agent/schema/agent-api/errors.kvx; envelope.version for any envelope other than 1; unavailable_capability for faucet.claim |
| events | none; subscription records are cursors over eth_subscribe logs |
| gas | none; writes are EIP-1559 transactions paid by the agent or the sponsor |
| activation | contract major 2 is served from wave 2; Level is deleted in wave 6 |
| invariants | every Operation is dispatched, retired or bootstrap gated; rungs are submitted, receipted, committed |
| pause | none |

## agent-api-request-frame

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.2.2</code> | req | 9002.2 |
| <code>acceptance.9.2.2</code> | scenario | Given the version request golden services/agent/schema/agent-api/golden/version-request.hex, when it is exchanged with the daemon and then re-sent with contract_major 1, then the first answer reports 2.0 and the second is refused with envelope.version. |
| <code>acceptance.9.2.2</code> | test | services/agent/crates/layerx-agentd/tests/rpc_surface_major_two.rs::version_exchange_reports_contract_major_two (planned) |
| <code>acceptance.9.2.2</code> | qualification | UNRUN |
| <code>req.9002</code> | ac_2 | The daemon answers a version request with contract_major 2 and contract_minor 0 as declared at services/agent/schema/agent-api/v1.kvx:3-4 and refuses a contract_major 1 request with ProtocolIncompatibility. |

## agent-api-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-response-frame</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/v1.kvx:12</code> | <code>agent-api response: contract_major:u16be contract_minor:u16be request_id:u64be result_code:u32be payload:canonical-map</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-response-frame</code> | notes | Stale task 11.5.46. |

Contract context <code>contract_context.9.agent-rpc</code>:

| Field | Frozen rule |
|---|---|
| interface | HTTP POST /rpc of layerx-agentd carrying the agent API envelope version 1 and contract major 2 operations |
| storage | agentd store: outbox, idempotency window, receipt store and subscription records |
| errors | error classes of services/agent/schema/agent-api/errors.kvx; envelope.version for any envelope other than 1; unavailable_capability for faucet.claim |
| events | none; subscription records are cursors over eth_subscribe logs |
| gas | none; writes are EIP-1559 transactions paid by the agent or the sponsor |
| activation | contract major 2 is served from wave 2; Level is deleted in wave 6 |
| invariants | every Operation is dispatched, retired or bootstrap gated; rungs are submitted, receipted, committed |
| pause | none |

## agent-api-schema-approval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-approval</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/schema/agent-api/approval.kvx:1</code> | <code>agent-api schema module approval.kvx (kvx)</code> |

## agent-api-schema-errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-errors</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/errors.kvx:1</code> | <code>agent-api schema module errors.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-errors</code> | notes | Stale task 11.5.45 re-targets errors.kvx off the settlement domain. |

Contract context <code>contract_context.9.agent-rpc</code>:

| Field | Frozen rule |
|---|---|
| interface | HTTP POST /rpc of layerx-agentd carrying the agent API envelope version 1 and contract major 2 operations |
| storage | agentd store: outbox, idempotency window, receipt store and subscription records |
| errors | error classes of services/agent/schema/agent-api/errors.kvx; envelope.version for any envelope other than 1; unavailable_capability for faucet.claim |
| events | none; subscription records are cursors over eth_subscribe logs |
| gas | none; writes are EIP-1559 transactions paid by the agent or the sponsor |
| activation | contract major 2 is served from wave 2; Level is deleted in wave 6 |
| invariants | every Operation is dispatched, retired or bootstrap gated; rungs are submitted, receipted, committed |
| pause | none |

## agent-api-schema-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-identity</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/schema/agent-api/identity.kvx:1</code> | <code>agent-api schema module identity.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-identity</code> | notes | golden/identity.kvx is an older compatibility baseline and differs from the live module. |

## agent-api-schema-programs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-programs</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/programs.kvx:1</code> | <code>agent-api schema module programs.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-programs</code> | notes | Stale task 11.5.45 re-targets programs.kvx off the settlement domain. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.36.2</code> | req | 9036.2 |
| <code>acceptance.9.36.2</code> | scenario | Given services/agent/schema/agent-api/programs.kvx and golden/programs.kvx, when every value is scanned, then none contains protocol3 or sequencer-signed. |
| <code>acceptance.9.36.2</code> | test | services/agent/crates/layerx-agentd/tests/program_adapters_rungs.rs::programs_schema_names_no_kernel_protocol (planned) |
| <code>acceptance.9.36.2</code> | qualification | UNRUN |
| <code>req.9036</code> | ac_2 | programs.kvx and its golden copy name no protocol3 or sequencer-signed verification. |

## agent-api-schema-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-read</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/read.kvx:1</code> | <code>agent-api schema module read.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-read</code> | notes | Stale task 11.5.45 re-targets read.kvx off the settlement domain. golden/read.kvx is an older compatibility baseline and differs from the live module. |

## agent-api-schema-stream

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-stream</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/stream.kvx:1</code> | <code>agent-api schema module stream.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-stream</code> | notes | Stale task 11.5.46 re-targets stream.kvx off the settlement domain. golden/stream.kvx is an older compatibility baseline and differs from the live module. |

## agent-api-schema-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-v1</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/v1.kvx:1</code> | <code>agent-api schema module v1.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-v1</code> | notes | Stale task 11.5.46 re-targets v1.kvx off the settlement domain. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.7.1</code> | req | 9007.1 |
| <code>acceptance.9.7.1</code> | scenario | Given the parsed v1.kvx declarations, when every operation.* value is scanned through agent_api_schema_v1, then none names a kernel level, PB1 or protocol3 and program.discover names requested_rung=receipted. |
| <code>acceptance.9.7.1</code> | test | services/agent/crates/layerx-agent-api/tests/schema_rungs.rs::v1_schema_declares_no_kernel_verification_level (planned) |
| <code>acceptance.9.7.1</code> | qualification | UNRUN |
| <code>req.9007</code> | ac_1 | No operation declaration in services/agent/schema/agent-api/v1.kvx names sequencer-signed, BatchIncluded, StateProven, PB1 or protocol3 after the edits at services/agent/schema/agent-api/v1.kvx:391, :493-495 and :905. |
| <code>acceptance.9.8.1</code> | req | 9008.1 |
| <code>acceptance.9.8.1</code> | scenario | Given the major-2 schema loaded through agent_api_schema_v1, when every declaration key is listed, then none ends in settlement_domain and no type.SettlementDomain section exists. |
| <code>acceptance.9.8.1</code> | test | services/agent/crates/layerx-agent-api/tests/schema_retired.rs::settlement_domain_vocabulary_is_absent (planned) |
| <code>acceptance.9.8.1</code> | qualification | UNRUN |
| <code>req.9008</code> | ac_1 | No schema file under services/agent/schema/agent-api declares type.SettlementDomain or any settlement_domain key after the deletions at services/agent/schema/agent-api/v1.kvx:43, services/agent/schema/agent-api/read.kvx:133, services/agent/schema/agent-api/stream.kvx:57 and services/agent/schema/agent-api/write.kvx:50. |
| <code>capability.9.1.3</code> | title | Re-target v1.kvx operation declarations from kernel levels to rungs |
| <code>capability.9.1.3</code> | reqs | 9007.1<br>9007.2 |
| <code>capability.9.1.4</code> | title | Retire Level and the settlement-domain vocabulary from agent-api |
| <code>capability.9.1.4</code> | reqs | 9008.1<br>9008.2 |
| <code>resolution.17.837</code> | title | decision.902 binds: The agent API schema is already at contract major 2 with rungs |
| <code>resolution.17.837</code> | applies_to | decision.902<br>task.9.1.3<br>task.9.1.4<br>task.9.18.4 |
| <code>resolution.17.837</code> | source | decision.902 in fragments/J43/decision.kvx; tasks from affects |

## agent-api-schema-write

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agent-api-schema-write</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/schema/agent-api/write.kvx:1</code> | <code>agent-api schema module write.kvx (kvx)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agent-api-schema-write</code> | notes | Stale task 11.5.46 re-targets write.kvx off the settlement domain. |

## agentd-startup-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agentd-startup-config</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:146</code> | <code>agentd startup configuration file (key=value lines, at most 65536 bytes, lines at most 4096 bytes)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-agentd-startup-config</code> | notes | Carries node_endpoint and expected_protocol_version for the LNI node; stale task 11.5.11 re-targets config.rs. |

## agentd-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-agentd-store</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/store/mod.rs:20</code> | <code>agentd durable store: magic LXAS, schema version 3 (oldest supported 1, services/agent/crates/layerx-agentd/src/store/migrate.rs:13)</code> |

## audit-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-audit-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/audit/log.rs:10</code> | <code>audit log file: magic LXAUD001, anchor LXANC001, entry LXAE, hash domain layerx-agent-audit-v1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-audit-log</code> | notes | Verified by layerx-audit-verify; audit/ is touched by stale tasks 0.58 and 11.5.7. |

## audit-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-audit-record</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/audit/record.rs:16</code> | <code>audit record entry: magic LXAR version 2 (legacy version 1 readable)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-audit-record</code> | notes | Stale task 11.5.7 re-targets record.rs off the settlement domain and export trust. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.28.1</code> | req | 9028.1 |
| <code>acceptance.9.28.1</code> | scenario | Given a submitted transaction, when an audit entry is recorded and read back, then the version is 3 and tx_hash, nonce and rung equal the recorded values. |
| <code>acceptance.9.28.1</code> | test | services/agent/crates/layerx-agentd/tests/audit_record_v3.rs::audit_entry_v3_records_transaction_hash_and_rung (planned) |
| <code>acceptance.9.28.1</code> | qualification | UNRUN |
| <code>req.9028</code> | ac_1 | A version 3 audit entry round-trips its transaction hash, nonce and rung. |
| <code>acceptance.9.28.2</code> | req | 9028.2 |
| <code>acceptance.9.28.2</code> | scenario | Given a log with one version 2 entry and one version 1 entry, when read_entries runs, then the version 2 entry is returned and the version 1 entry yields RecordError::Version. |
| <code>acceptance.9.28.2</code> | test | services/agent/crates/layerx-agentd/tests/audit_record_v3.rs::version_two_audit_entries_stay_readable (planned) |
| <code>acceptance.9.28.2</code> | qualification | UNRUN |
| <code>req.9028</code> | ac_2 | read_entries reads version 2 entries written at HEAD and refuses version 1. |
| <code>capability.9.6.2</code> | title | Record audit entries version 3 with rungs and transaction hashes |
| <code>capability.9.6.2</code> | reqs | 9028.1<br>9028.2 |

## budget-proof

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-budget-proof</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/budget/budget_proof.rs:217</code> | <code>human budget proof document layerx.human.budget-proof.v1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-budget-proof</code> | notes | Stale task 11.5.8 re-targets budget_proof.rs off the LNI transport. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.33.2</code> | req | 9033.2 |
| <code>acceptance.9.33.2</code> | scenario | Given a v2 budget proof at committed height 12 and a v1 proof, when verify_budget_proof runs on each, then the first verifies and the second returns BudgetProofError::Refused. |
| <code>acceptance.9.33.2</code> | test | services/agent/crates/layerx-agentd/tests/budget_mutations_layerxpay.rs::budget_proof_v2_carries_the_committed_height (planned) |
| <code>acceptance.9.33.2</code> | qualification | UNRUN |
| <code>req.9033</code> | ac_2 | verify_budget_proof accepts schema v2 with a getBudget result at a committed height and refuses schema v1. |

## human-peer-frame

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-human-peer-frame</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:17</code> | <code>human peer frame: magic LXHAGT01 (traced LXHAGT02 at services/agent/crates/layerx-agentd/src/human.rs:18) + opcode u8 + payload over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-human-peer-frame</code> | notes | Served on LAYERX_AGENT_HUMAN_SOCKET; framing comes from the agentd-local HumanFrameTransport with a 4-byte big-endian length prefix in services/agent/crates/layerx-agentd/src/human_frame.rs:19-49, declared as human::frame at services/agent/crates/layerx-agentd/src/human.rs:3-6; the frame no longer depends on layerx_client::lni. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.46.1</code> | req | 9046.1 |
| <code>acceptance.9.46.1</code> | scenario | Given a socket pair and a maximum of 64 bytes, when a 10-byte payload is written and read, then the bytes match; when a 65-byte frame is read, then the result is a frame-too-large error. |
| <code>acceptance.9.46.1</code> | test | services/agent/crates/layerx-agentd/tests/human_frame.rs::human_frame_round_trips_with_a_length_prefix (planned) |
| <code>acceptance.9.46.1</code> | qualification | UNRUN |
| <code>req.9046</code> | ac_1 | write_frame and read_frame round-trip a payload with a 4-byte big-endian length prefix and refuse a frame above the maximum. |
| <code>acceptance.9.46.2</code> | req | 9046.2 |
| <code>acceptance.9.46.2</code> | scenario | Given a HumanPeer and a HEAD request framed with LXHAGT01, when serve_one handles it, then the response begins with LXHAGT01. |
| <code>acceptance.9.46.2</code> | test | services/agent/crates/layerx-agentd/tests/human_frame.rs::human_peer_still_answers_lxhagt01_frames (planned) |
| <code>acceptance.9.46.2</code> | qualification | UNRUN |
| <code>req.9046</code> | ac_2 | serve_one answers a request framed with magic LXHAGT01 over the agentd-local transport. |
| <code>acceptance.9.47.1</code> | req | 9047.1 |
| <code>acceptance.9.47.1</code> | scenario | Given a human peer whose node reports pending nonce 3, when a PREPARE frame with nonce 2 is served, then the response is the stale-nonce refusal naming pending 3. |
| <code>acceptance.9.47.1</code> | test | services/agent/crates/layerx-agentd/tests/human_peer_rungs.rs::human_prepare_carries_the_pending_nonce (planned) |
| <code>acceptance.9.47.1</code> | qualification | UNRUN |
| <code>req.9047</code> | ac_1 | A PREPARE request decodes a nonce field and answers StaleNonce when it differs from the pending nonce. |
| <code>acceptance.9.47.2</code> | req | 9047.2 |
| <code>acceptance.9.47.2</code> | scenario | Given a receipted transaction, when an EVIDENCE frame is served for its hash, then the response decodes to rung byte 1 and the receipt's block height. |
| <code>acceptance.9.47.2</code> | test | services/agent/crates/layerx-agentd/tests/human_peer_rungs.rs::human_evidence_reports_rung_bytes (planned) |
| <code>acceptance.9.47.2</code> | qualification | UNRUN |
| <code>req.9047</code> | ac_2 | An EVIDENCE answer carries tx_hash, observed height and rung byte 1 for a receipted transaction. |
| <code>acceptance.9.47.3</code> | req | 9047.3 |
| <code>acceptance.9.47.3</code> | scenario | Given the opcode table, when each opcode constant is compared with its HEAD value recorded in the test, then all 61 match. |
| <code>acceptance.9.47.3</code> | test | services/agent/crates/layerx-agentd/tests/human_peer_rungs.rs::human_opcode_numbers_are_unchanged (planned) |
| <code>acceptance.9.47.3</code> | qualification | UNRUN |
| <code>req.9047</code> | ac_3 | Every human peer opcode number at services/agent/crates/layerx-agentd/src/human.rs:57-119 is unchanged. |
| <code>capability.9.13.1</code> | title | Move the human peer frame codec into agentd |
| <code>capability.9.13.1</code> | reqs | 9046.1<br>9046.2 |
| <code>capability.9.13.2</code> | title | Carry nonces, heights and rungs in the human peer operations |
| <code>capability.9.13.2</code> | reqs | 9047.1<br>9047.2<br>9047.3 |
| <code>resolution.17.841</code> | title | decision.906 binds: The human peer frame LXHAGT01 is kept with its codec moved into agentd |
| <code>resolution.17.841</code> | applies_to | decision.906<br>task.9.13.1<br>task.9.13.2<br>task.9.13.3 |
| <code>resolution.17.841</code> | source | decision.906 in fragments/J43/decision.kvx; tasks from affects |

## lni-capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-lni-v1-capabilities</code> | <code>file_format</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:12</code> | <code>LNI v1 capability list (22 values)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-lni-v1-capabilities</code> | notes | Stale task 11.7.29. |

## lni-frame

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-lni-v1-frame</code> | <code>file_format</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:5</code> | <code>LNI v1 frame: major:u16be minor:u16be message_tag:u16be correlation_id:u64be canonical_payload:bytes32 proof_material:bytes32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-lni-v1-frame</code> | notes | Stale tasks 11.7.29 (delete the LNI transport) and 11.5.47. |

## mcp-binding-document

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-mcp-binding-document</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:40</code> | <code>layerx-mcp daemon binding document (closed-key JSON: mode, tenant, store, audit_root, session_*, capability_id, core_sequence, deadline_ms, agent, limit, listener, web)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-mcp-binding-document</code> | notes | core_sequence and web.sequencer_public_key carry kernel-era inputs (stale tasks 6.5.14, 6.5.16, 6.5.17). |

## mcp-invocation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-mcp-invocation</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/session_control.rs:1203</code> | <code>MCP invocation record layerx.mcp.invocation.v1</code> |

## native-effect-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-native-effect-policy</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/native_program.rs:472</code> | <code>native effect policy layerx.native-effect-policy.v1</code> |

## native-effect-policy-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-native-effect-policy-source</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:722</code> | <code>protected native effect policy source layerx.native-effect-policy-source.v1 (JSON: version, tenant, policy)</code> |

## native-program-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-native-program-policy</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/native_program.rs:11</code> | <code>native program policy layerx.native-program-policy.v1</code> |

## native-program-policy-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-native-program-policy-source</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:677</code> | <code>protected native program policy source layerx.native-program-policy-source.v1 (JSON: version, tenant, policy)</code> |

## offline-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-offline-export</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/export.rs:201</code> | <code>offline export bundle produced by export::produce (at most MAX_EXPORT_BYTES = 1048576)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-offline-export</code> | notes | Stale task 6.6.4 moves offline export to portable v2; the Rust OfflineExport (services/agent/crates/layerx-agent-api/src/read.rs:438) carries certificates and headers where services/agent/schema/agent-api/read.kvx:131 requires facts and portable v2 bundles. |

## program-budget-denominations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-program-budget-denominations</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:512</code> | <code>protected program budget denominations source (JSON version 1: tenant, limits[stable_id, asset, source])</code> |

## receipt-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-receipt-evidence</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/receipt.rs:17</code> | <code>receipt evidence record: magic LXRE version 1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-receipt-evidence</code> | notes | Stale task 0.36. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.18.1</code> | req | 9018.1 |
| <code>acceptance.9.18.1</code> | scenario | Given a receipted transaction with block height 12 and tx index 1, when its evidence is persisted and served by hash, then every field round-trips and the record version is 2. |
| <code>acceptance.9.18.1</code> | test | services/agent/crates/layerx-agentd/tests/receipt_store_v2.rs::receipt_evidence_round_trips_by_transaction_hash (planned) |
| <code>acceptance.9.18.1</code> | qualification | UNRUN |
| <code>req.9018</code> | ac_1 | A version 2 receipt evidence record stored under a transaction hash is served back byte for byte. |
| <code>acceptance.9.18.2</code> | req | 9018.2 |
| <code>acceptance.9.18.2</code> | scenario | Given a version 1 evidence record written at HEAD, when serve_evidence reads it, then the result is the receipt.evidence_version refusal. |
| <code>acceptance.9.18.2</code> | test | services/agent/crates/layerx-agentd/tests/receipt_store_v2.rs::version_one_receipt_evidence_is_refused (planned) |
| <code>acceptance.9.18.2</code> | qualification | UNRUN |
| <code>req.9018</code> | ac_2 | serve_evidence refuses a stored LXRE version 1 record with reason receipt.evidence_version (services/agent/crates/layerx-agentd/src/receipt.rs:389). |
| <code>capability.9.4.2</code> | title | Store receipt metadata and evidence version 2 by transaction hash |
| <code>capability.9.4.2</code> | reqs | 9018.1<br>9018.2 |

## receipt-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-receipt-metadata</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/receipt.rs:16</code> | <code>receipt metadata record: magic LXRM</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-receipt-metadata</code> | notes | Stale task 0.36 moves receipts to the sequencer receipt with pinned receipt keys. No in-repo reference outside the defining line at this revision. |

## registry-source-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-registry-source-config</code> | <code>file_format</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:25</code> | <code>registry source configuration (JSON: version, endpoint, server_ca_der, client_certificate_pem, client_private_key_pem, request_token_file, deadline_ms, maximum_response_bytes)</code> |

## unknown-resolution-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.fmt-unknown-resolution-state</code> | <code>file_format</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/outbox/unknown.rs:10</code> | <code>unknown-outcome resolution state: magic LXUR</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.fmt-unknown-resolution-state</code> | notes | Stale task 6.5.5 resolves unknown outcomes by transaction hash then resultByIdempotency. No in-repo reference outside the defining line at this revision. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.16.3</code> | req | 9016.3 |
| <code>acceptance.9.16.3</code> | scenario | Given a persisted LXUR resolution record, when the daemon loads unknown state, then it returns the unknown.state_version refusal and no record is resolved. |
| <code>acceptance.9.16.3</code> | test | services/agent/crates/layerx-agentd/tests/unknown_receipt_lookup.rs::legacy_lxur_resolution_state_is_refused (planned) |
| <code>acceptance.9.16.3</code> | qualification | UNRUN |
| <code>req.9016</code> | ac_3 | Loading a resolution state written with magic LXUR is refused with reason unknown.state_version (services/agent/crates/layerx-agentd/src/outbox/unknown.rs:10). |
