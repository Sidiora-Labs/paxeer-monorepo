# Interop gateway contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## err-gw-adapter-declaration

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-adapter-declaration</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:57</code> | <code>GatewayError::Adapter.code() = "adapter-declaration"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-adapter-declaration</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-adapter-duplicate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-adapter-duplicate</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:60</code> | <code>GatewayError::DuplicateAdapter.code() = "adapter-duplicate"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-adapter-duplicate</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-adapter-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-adapter-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:61</code> | <code>GatewayError::UnknownAdapter.code() = "adapter-unknown"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-adapter-unknown</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-audit-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-audit-append</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:59</code> | <code>GatewayError::Audit.code() = "audit-append"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-audit-append</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-idempotency-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:64</code> | <code>GatewayError::IdempotencyConflict.code() = "idempotency-conflict"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-idempotency-conflict</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-principal-scope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-principal-scope</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:58</code> | <code>GatewayError::Store.code() = "principal-scope"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-principal-scope</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-receipt-rejected

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-receipt-rejected</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:68</code> | <code>GatewayError::ReceiptRejected.code() = "receipt-rejected"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-receipt-rejected</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-receipt-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-receipt-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:65</code> | <code>GatewayError::ReceiptRequired.code() = "receipt-required"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-receipt-required</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-record-corrupt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-record-corrupt</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:69</code> | <code>GatewayError::Corrupt.code() = "record-corrupt"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-record-corrupt</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-translation-closed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-translation-closed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:67</code> | <code>GatewayError::TranslationClosed.code() = "translation-closed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-translation-closed</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-translation-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-translation-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:62</code> | <code>GatewayError::InvalidTranslation.code() = "translation-invalid"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-translation-invalid</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-translation-read-only

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-translation-read-only</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:66</code> | <code>GatewayError::NotStateChanging.code() = "translation-read-only"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-translation-read-only</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## err-gw-translation-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-translation-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:63</code> | <code>GatewayError::UnknownTranslation.code() = "translation-unknown"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-translation-unknown</code> | notes | wire code returned by GatewayError::code (services/interop/crates/layerx-interop-gateway/src/error.rs:55) |

## fmt-gw-audit-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-audit-entry</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:15</code> | <code>audit entry: magic "LXGA", version u8 1, AuditEventKind u8 (1 adapter-registered, 2 translation-begun, 3 translation-completed, 4 translation-refused); chain links domain-separated by "LXGG" (genesis) and "LXGL" (link)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-audit-entry</code> | notes | hash chain over layerx_proof merkle leaf_hash |

## fmt-gw-audit-event-kind-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-audit-event-kind-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:32</code> | <code>AuditEventKind::label() -&gt; "adapter-registered" &#124; "translation-begun" &#124; "translation-completed" &#124; "translation-refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-audit-event-kind-labels</code> | notes | stable label vocabulary of AuditEventKind |

## fmt-gw-emission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-emission</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:14</code> | <code>telemetry emission: magic "LXIE", version u8 1, field kinds u8 (1 digest, 2 label, 3 count, 4 duration-ms, 5 trace); schemas adapter-call, audit-append, error-response; names &lt;= 64 bytes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-emission</code> | notes | redacted log line format |

## fmt-gw-evidence-policy-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-evidence-policy-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:158</code> | <code>EvidencePolicy::label() -&gt; "layerx-receipt" &#124; "verified-mandate+layerx-receipt" &#124; "trusted-agent-credential" &#124; "external-settlement+layerx-receipt"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-evidence-policy-labels</code> | notes | stable label vocabulary of EvidencePolicy |

## fmt-gw-external-state-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-external-state-labels</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:181</code> | <code>ExternalState::label() -&gt; "pending" &#124; "reversal-pending" &#124; "refused" &#124; "receipt-verified" &#124; "reversed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-external-state-labels</code> | notes | stable label vocabulary of ExternalState |
| <code>interface.11.fmt-gw-external-state-labels</code> | overridden_by | override.J47.4 |

## fmt-gw-ingress-transport-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-ingress-transport-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:17</code> | <code>IngressTransport::label() -&gt; "http" &#124; "mcp" &#124; "a2a"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-ingress-transport-labels</code> | notes | stable label vocabulary of IngressTransport |

## fmt-gw-ledger-events

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-ledger-events</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:36</code> | <code>layerxledger 0x101A logs: PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128) data assetId+amount (64 bytes); PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-ledger-events</code> | notes | GatewayCore matches settlement legs against these logs at LEDGER_ADDRESS 0x...101a (services/interop/crates/layerx-interop-gateway/src/gateway.rs:33-35); precompile ABI owned by domain 3 |

## fmt-gw-retriability-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-retriability-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:26</code> | <code>Retriability::label() -&gt; "retriable" &#124; "terminal"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-retriability-labels</code> | notes | stable label vocabulary of Retriability |

## fmt-gw-trace-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-trace-header</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:10</code> | <code>HTTP header x-layerx-trace: trc_&lt;id&gt; on inbound adapter requests and every outbound call</code> |

## fmt-gw-translation-kind-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-translation-kind-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:72</code> | <code>TranslationKind::label() -&gt; "read-only" &#124; "state-changing"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-translation-kind-labels</code> | notes | stable label vocabulary of TranslationKind; 32 reference lines in 7 files; callers lists the first reference in each file |

## fmt-gw-translation-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-translation-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:27</code> | <code>translation record: magic "LXIT", version u8 1, TranslationKind u8 (1 read-only, 2 state-changing), TranslationStatus u8 (1 pending, 2 translated, 3 receipt-verified, 4 refused), ...</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-translation-record</code> | notes | row prefix tr-, telemetry prefix tel-, registry chain gateway-registry (services/interop/crates/layerx-interop-gateway/src/gateway.rs:29-31) |

## fmt-gw-translation-status-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gw-translation-status-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:168</code> | <code>TranslationStatus::label() -&gt; "pending" &#124; "translated" &#124; "receipt-verified" &#124; "refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gw-translation-status-labels</code> | notes | stable label vocabulary of TranslationStatus; 70 reference lines in 8 files; callers lists the first reference in each file |

## gateway-sequencer-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gw-receipt-pending-j47</code> | <code>error_code</code> | <code>new</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs</code> (planned) | <code>receipt-pending</code> |
| <code>interface.11.rs-gw-evidence-canonical-receipt-digest-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-interop-gateway/src/evidence.rs</code> (planned) | <code>pub fn canonical_receipt_digest(signed_receipt: &amp;[u8]) -&gt; Result&lt;[u8; 32], GatewayError&gt;</code> |
| <code>interface.11.rs-gw-evidence-sequencer-evidence-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-interop-gateway/src/evidence.rs</code> (planned) | <code>pub struct SequencerEvidence { pub transaction_hash: [u8; 32], pub signed_receipt: Vec&lt;u8&gt;, pub logs: Vec&lt;SequencerLog&gt;, pub rung: SettlementRung }</code> |
| <code>interface.11.rs-gw-evidence-settlement-rung-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-interop-gateway/src/evidence.rs</code> (planned) | <code>pub enum SettlementRung { Submitted, Receipted, Committed { block_number: u64, block_hash: [u8; 32] } }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gw-receipt-pending-j47</code> | notes | GatewayError::ReceiptPending; retriable; created by task 11.1.2. |
| <code>interface.11.rs-gw-evidence-canonical-receipt-digest-j47</code> | notes | Created by task 11.0.1; replaces the private UCP, Visa and portable copies. |
| <code>interface.11.rs-gw-evidence-sequencer-evidence-j47</code> | notes | Created by task 11.0.1; the one evidence shape every adapter settles on. |
| <code>interface.11.rs-gw-evidence-settlement-rung-j47</code> | notes | Created by task 11.0.1; mirrors the three pax_sequencerReceipt answers. |

Contract context <code>contract_context.11.gateway-sequencer-settlement</code>:

| Field | Frozen rule |
|---|---|
| interface | GatewayCore settles a translation on SequencerEvidence and records the canonical receipt digest. |
| storage | Translation records and audit chain kept by GatewayCore; no new storage. |
| errors | receipt-pending (retriable), receipt-rejected, receipt-required. |
| events | Audit entry on ReceiptVerified only. |
| gas | None; the gateway reads receipts and logs. |
| activation | Wave 1 (task 11.1.1); the batch path is retired in wave 6. |
| invariants | A submitted rung writes no record; one PaymentExact or PaymentDrawn log per settlement; digest equals receiptDigest of the fixture. |
| pause | Not pausable; refusal is per translation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.1.1</code> | req | 11001.1 |
| <code>acceptance.11.1.1</code> | scenario | Given the three SettlementRung values, when label is called on each, then the strings are submitted, receipted and committed in that order and Committed keeps its block number and hash. |
| <code>acceptance.11.1.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/evidence.rs::settlement_rung_labels_are_submitted_receipted_and_committed (planned) |
| <code>acceptance.11.1.1</code> | qualification | UNRUN |
| <code>req.11001</code> | ac_1 | SettlementRung::label returns submitted, receipted and committed for the three rungs (planned), the labels the x402 settlement response carries at services/interop/crates/layerx-x402/src/facilitator.rs:408. |
| <code>acceptance.11.1.2</code> | req | 11001.2 |
| <code>acceptance.11.1.2</code> | scenario | Given the signedReceipt bytes of case V1-layerxledger-pay-exact, when canonical_receipt_digest runs, then it returns the 32 bytes recorded as receiptDigest, and a receipt shorter than the signature tail is refused with a GatewayError. |
| <code>acceptance.11.1.2</code> | test | services/interop/crates/layerx-interop-gateway/tests/evidence.rs::canonical_receipt_digest_matches_the_recorded_pay_exact_vector (planned) |
| <code>acceptance.11.1.2</code> | qualification | UNRUN |
| <code>req.11001</code> | ac_2 | canonical_receipt_digest of the recorded signedReceipt equals the recorded receiptDigest at sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:150 (planned). |
| <code>acceptance.11.8.1</code> | req | 11008.1 |
| <code>acceptance.11.8.1</code> | scenario | Given GatewayError::ReceiptPending, when code and retriability are read, then the code is receipt-pending and the retriability label is retriable. |
| <code>acceptance.11.8.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/receipt_pending.rs::receipt_pending_is_retriable_with_its_own_code (planned) |
| <code>acceptance.11.8.1</code> | qualification | UNRUN |
| <code>req.11008</code> | ac_1 | GatewayError::ReceiptPending has code receipt-pending and is retriable (planned), unlike the terminal ReceiptRejected of services/interop/crates/layerx-interop-gateway/src/error.rs:48. |
| <code>capability.11.1.1</code> | title | Add the gateway evidence module with settlement rungs and the canonical receipt digest |
| <code>capability.11.1.1</code> | reqs | 11001.1<br>11001.2 |
| <code>capability.11.1.2</code> | title | Add GatewayError::ReceiptPending for a submitted rung that carries no signed receipt |
| <code>capability.11.1.2</code> | reqs | 11008.1 |
| <code>resolution.17.880</code> | title | decision.1103 binds: One canonical receipt digest |
| <code>resolution.17.880</code> | applies_to | decision.1103<br>task.11.0.1<br>task.11.1.1<br>task.11.9.2<br>task.11.17.2 |
| <code>resolution.17.880</code> | source | decision.1103 in fragments/J47/decision.kvx; tasks from affects |
| <code>resolution.17.890</code> | title | decision.1125 binds: layerx-mirror-signer has no task |
| <code>resolution.17.890</code> | applies_to | decision.1125<br>task.11.0.1 |
| <code>resolution.17.890</code> | source | decision.1125 in fragments/J48/decision.kvx; tasks from no affected task: bound to the domain-11 foundation anchor |

## gateway-sequencer-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-evidence-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs</code> (planned) | <code>pub fn settle_with_sequencer_evidence(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], evidence: &amp;SequencerEvidence, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;SequencerSettlementError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-evidence-j47</code> | notes | Created by task 11.1.1. |

Contract context <code>contract_context.11.gateway-sequencer-settlement</code>:

| Field | Frozen rule |
|---|---|
| interface | GatewayCore settles a translation on SequencerEvidence and records the canonical receipt digest. |
| storage | Translation records and audit chain kept by GatewayCore; no new storage. |
| errors | receipt-pending (retriable), receipt-rejected, receipt-required. |
| events | Audit entry on ReceiptVerified only. |
| gas | None; the gateway reads receipts and logs. |
| activation | Wave 1 (task 11.1.1); the batch path is retired in wave 6. |
| invariants | A submitted rung writes no record; one PaymentExact or PaymentDrawn log per settlement; digest equals receiptDigest of the fixture. |
| pause | Not pausable; refusal is per translation. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.15.1</code> | req | 11015.1 |
| <code>acceptance.11.15.1</code> | scenario | Given the route table, when the test reads the x402, AP2, UCP, Visa and fiat entries, then each path and method equals the recorded value. |
| <code>acceptance.11.15.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/route_freeze.rs::settling_routes_keep_their_paths_and_methods (planned) |
| <code>acceptance.11.15.1</code> | qualification | UNRUN |
| <code>req.11015</code> | ac_1 | interop_gateway_routes lists the nine settling routes of services/interop/crates/layerx-interop-gateway/src/server.rs:229-264 unchanged (planned). |
| <code>acceptance.11.15.2</code> | req | 11015.2 |
| <code>acceptance.11.15.2</code> | scenario | Given a protocol responder whose settlement returns GatewayError::ReceiptPending, when the answer is rendered, then the status is 202 and the state label is submitted. |
| <code>acceptance.11.15.2</code> | test | services/interop/crates/layerx-interop-gateway/tests/route_freeze.rs::receipt_pending_answers_submitted_with_202 (planned) |
| <code>acceptance.11.15.2</code> | qualification | UNRUN |
| <code>req.11015</code> | ac_2 | A ReceiptPending settlement answers state submitted with HTTP 202 (planned) (services/interop/crates/layerx-interop-gateway/src/server.rs:181). |
| <code>acceptance.11.55.1</code> | req | 11055.1 |
| <code>acceptance.11.55.1</code> | scenario | Given a pending translation, when it is settled on a foreign-key receipt, a tampered log and a submitted rung, then each is refused and the translation stays pending. |
| <code>acceptance.11.55.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/gateway.rs::a_state_changing_translation_refuses_every_unproved_sequencer_termination (planned) |
| <code>acceptance.11.55.1</code> | qualification | UNRUN |
| <code>req.11055</code> | ac_1 | A state-changing translation is refused for every receipt the pinned keys do not verify (planned) (services/interop/crates/layerx-interop-gateway/tests/gateway.rs:212). |
| <code>acceptance.11.9.1</code> | req | 11009.1 |
| <code>acceptance.11.9.1</code> | scenario | Given a pending state-changing translation keyed by the paymentReference of case V1-layerxledger-pay-exact, when its receipted evidence is settled with the pinned key, then the status is ReceiptVerified with the recorded receiptDigest. |
| <code>acceptance.11.9.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/sequencer_evidence.rs::receipted_evidence_records_the_canonical_receipt_digest (planned) |
| <code>acceptance.11.9.1</code> | qualification | UNRUN |
| <code>req.11009</code> | ac_1 | settle_with_sequencer_evidence records receiptDigest of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:150 as the ReceiptVerified digest (planned). |
| <code>acceptance.11.9.2</code> | req | 11009.2 |
| <code>acceptance.11.9.2</code> | scenario | Given the same pending translation, when evidence at the submitted rung is settled, then the call returns GatewayError::ReceiptPending and translation() still reports pending. |
| <code>acceptance.11.9.2</code> | test | services/interop/crates/layerx-interop-gateway/tests/sequencer_evidence.rs::submitted_evidence_leaves_the_translation_pending (planned) |
| <code>acceptance.11.9.2</code> | qualification | UNRUN |
| <code>req.11009</code> | ac_2 | A submitted rung returns ReceiptPending and leaves the translation pending (planned) (services/interop/crates/layerx-interop-gateway/src/gateway.rs:576). |
| <code>acceptance.11.9.3</code> | req | 11009.3 |
| <code>acceptance.11.9.3</code> | scenario | Given two translations settled on the same recorded receipt, one through each method, when both statuses are read, then both carry the recorded receiptDigest. |
| <code>acceptance.11.9.3</code> | test | services/interop/crates/layerx-interop-gateway/tests/sequencer_evidence.rs::receipt_and_evidence_paths_agree_on_the_digest (planned) |
| <code>acceptance.11.9.3</code> | qualification | UNRUN |
| <code>req.11009</code> | ac_3 | settle_with_sequencer_receipt and settle_with_sequencer_evidence record the same digest for one receipt (planned) (services/interop/crates/layerx-interop-gateway/src/gateway.rs:540). |
| <code>capability.11.1.3</code> | title | Settle a translation on SequencerEvidence and record the canonical receipt digest |
| <code>capability.11.1.3</code> | reqs | 11009.1<br>11009.2<br>11009.3 |
| <code>capability.11.1.4</code> | title | Report the submitted rung through ExternalState and freeze the protocol routes |
| <code>capability.11.1.4</code> | reqs | 11015.1<br>11015.2 |
| <code>capability.11.18.1</code> | title | Retire GatewayCore::settle_with_receipt and its AuthorizedBatch tests |
| <code>capability.11.18.1</code> | reqs | 11055.1 |

## gw-adapter-call-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-call-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:19</code> | <code>pub const ADAPTER_CALL_SCHEMA: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-call-schema</code> | notes | const layerx_interop_gateway::redaction::ADAPTER_CALL_SCHEMA |

## gw-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:211</code> | <code>pub struct AdapterDescriptor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor</code> | notes | struct layerx_interop_gateway::adapter::AdapterDescriptor |

## gw-adapter-descriptor-conformance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-conformance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:242</code> | <code>pub const fn conformance(&amp;self) -&gt; &amp;ConformanceSuite</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-conformance</code> | notes | method layerx_interop_gateway::adapter::AdapterDescriptor::conformance |

## gw-adapter-descriptor-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:230</code> | <code>pub const fn id(&amp;self) -&gt; &amp;AdapterId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-id</code> | notes | method layerx_interop_gateway::adapter::AdapterDescriptor::id; no reference outside the defining file found by name match |

## gw-adapter-descriptor-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:220</code> | <code>pub const fn new(id: AdapterId, spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-new</code> | notes | method layerx_interop_gateway::adapter::AdapterDescriptor::new |

## gw-adapter-descriptor-spec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-spec</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:236</code> | <code>pub const fn spec(&amp;self) -&gt; &amp;PinnedSpec</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-spec</code> | notes | method layerx_interop_gateway::adapter::AdapterDescriptor::spec |

## gw-adapter-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:249</code> | <code>pub enum AdapterError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-error</code> | notes | enum layerx_interop_gateway::adapter::AdapterError |

## gw-adapter-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:24</code> | <code>pub struct AdapterId(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-id</code> | notes | struct layerx_interop_gateway::adapter::AdapterId; 57 reference lines in 30 files; callers lists the first reference in each file |

## gw-adapter-id-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:43</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-id-as-str</code> | notes | method layerx_interop_gateway::adapter::AdapterId::as_str; no reference outside the defining file found by name match |

## gw-adapter-id-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:32</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-adapter-id-new</code> | notes | method layerx_interop_gateway::adapter::AdapterId::new; 44 reference lines in 29 files; callers lists the first reference in each file |

## gw-audit-append-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-append-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:21</code> | <code>pub const AUDIT_APPEND_SCHEMA: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-append-schema</code> | notes | const layerx_interop_gateway::redaction::AUDIT_APPEND_SCHEMA; no reference outside the defining file found by name match |

## gw-audit-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:226</code> | <code>pub struct AuditChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain</code> | notes | struct layerx_interop_gateway::audit::AuditChain |

## gw-audit-chain-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-append</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:253</code> | <code>pub fn append(&amp;mut self, recorded_at: u64, trace: &amp;TraceId, kind: AuditEventKind, adapter: &amp;str, subject: [u8; 32]) -&gt; Result&lt;&amp;ChainEntry, AuditError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-append</code> | notes | method layerx_interop_gateway::audit::AuditChain::append |

## gw-audit-chain-entries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-entries</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:297</code> | <code>pub fn entries(&amp;self) -&gt; &amp;[ChainEntry]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-entries</code> | notes | method layerx_interop_gateway::audit::AuditChain::entries; no reference outside the defining file found by name match |

## gw-audit-chain-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-export</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:317</code> | <code>pub fn export(&amp;self) -&gt; Vec&lt;Vec&lt;u8&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-export</code> | notes | method layerx_interop_gateway::audit::AuditChain::export; no reference outside the defining file found by name match |

## gw-audit-chain-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-head</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:306</code> | <code>pub fn head(&amp;self) -&gt; Result&lt;ChainHead, AuditError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-head</code> | notes | method layerx_interop_gateway::audit::AuditChain::head; no reference outside the defining file found by name match |

## gw-audit-chain-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-name</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:243</code> | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-name</code> | notes | method layerx_interop_gateway::audit::AuditChain::name; no reference outside the defining file found by name match |

## gw-audit-chain-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:234</code> | <code>pub fn new(name: impl Into&lt;String&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-chain-new</code> | notes | method layerx_interop_gateway::audit::AuditChain::new |

## gw-audit-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:363</code> | <code>pub enum AuditError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-error</code> | notes | enum layerx_interop_gateway::audit::AuditError |

## gw-audit-event-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:22</code> | <code>pub enum AuditEventKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind</code> | notes | enum layerx_interop_gateway::audit::AuditEventKind |

## gw-audit-event-kind-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:32</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind-label</code> | notes | method layerx_interop_gateway::audit::AuditEventKind::label; no reference outside the defining file found by name match |

## gw-banned-field-names

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-banned-field-names</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:27</code> | <code>pub const BANNED_FIELD_NAMES: &amp;[&amp;str]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-banned-field-names</code> | notes | const layerx_interop_gateway::redaction::BANNED_FIELD_NAMES; no reference outside the defining file found by name match |

## gw-chain-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:66</code> | <code>pub struct ChainEntry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry</code> | notes | struct layerx_interop_gateway::audit::ChainEntry; no reference outside the defining file found by name match |

## gw-chain-entry-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:105</code> | <code>pub fn adapter(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-adapter</code> | notes | method layerx_interop_gateway::audit::ChainEntry::adapter; no reference outside the defining file found by name match |

## gw-chain-entry-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:129</code> | <code>pub fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-bytes</code> | notes | method layerx_interop_gateway::audit::ChainEntry::bytes; no reference outside the defining file found by name match |

## gw-chain-entry-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:99</code> | <code>pub const fn kind(&amp;self) -&gt; AuditEventKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-kind</code> | notes | method layerx_interop_gateway::audit::ChainEntry::kind; no reference outside the defining file found by name match |

## gw-chain-entry-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:123</code> | <code>pub const fn link(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-link</code> | notes | method layerx_interop_gateway::audit::ChainEntry::link; no reference outside the defining file found by name match |

## gw-chain-entry-prev-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-prev-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:117</code> | <code>pub const fn prev_link(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-prev-link</code> | notes | method layerx_interop_gateway::audit::ChainEntry::prev_link; no reference outside the defining file found by name match |

## gw-chain-entry-recorded-at

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-recorded-at</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:87</code> | <code>pub const fn recorded_at(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-recorded-at</code> | notes | method layerx_interop_gateway::audit::ChainEntry::recorded_at; no reference outside the defining file found by name match |

## gw-chain-entry-sequence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-sequence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:81</code> | <code>pub const fn sequence(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-sequence</code> | notes | method layerx_interop_gateway::audit::ChainEntry::sequence; no reference outside the defining file found by name match |

## gw-chain-entry-subject

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-subject</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:111</code> | <code>pub const fn subject(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-subject</code> | notes | method layerx_interop_gateway::audit::ChainEntry::subject; no reference outside the defining file found by name match |

## gw-chain-entry-trace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-trace</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:93</code> | <code>pub const fn trace(&amp;self) -&gt; &amp;TraceId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-entry-trace</code> | notes | method layerx_interop_gateway::audit::ChainEntry::trace; no reference outside the defining file found by name match |

## gw-chain-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-head</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:137</code> | <code>pub struct ChainHead</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-head</code> | notes | struct layerx_interop_gateway::audit::ChainHead; no reference outside the defining file found by name match |

## gw-chain-head-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-head-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:145</code> | <code>pub const fn length(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-head-length</code> | notes | method layerx_interop_gateway::audit::ChainHead::length; no reference outside the defining file found by name match |

## gw-chain-head-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-chain-head-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:151</code> | <code>pub const fn link(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-chain-head-link</code> | notes | method layerx_interop_gateway::audit::ChainHead::link; no reference outside the defining file found by name match |

## gw-conformance-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:157</code> | <code>pub struct ConformanceSuite</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-conformance-suite</code> | notes | struct layerx_interop_gateway::adapter::ConformanceSuite |

## gw-conformance-suite-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:170</code> | <code>pub fn new(suite: AdapterId, vector_count: u64, suite_digest: [u8; 32]) -&gt; Result&lt;Self, AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-new</code> | notes | method layerx_interop_gateway::adapter::ConformanceSuite::new |

## gw-conformance-suite-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:190</code> | <code>pub const fn suite(&amp;self) -&gt; &amp;AdapterId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite</code> | notes | method layerx_interop_gateway::adapter::ConformanceSuite::suite |

## gw-conformance-suite-suite-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:202</code> | <code>pub const fn suite_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite-digest</code> | notes | method layerx_interop_gateway::adapter::ConformanceSuite::suite_digest |

## gw-conformance-suite-vector-count

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-vector-count</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:196</code> | <code>pub const fn vector_count(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-vector-count</code> | notes | method layerx_interop_gateway::adapter::ConformanceSuite::vector_count |

## gw-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:327</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Result&lt;Emission, RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-decode</code> | notes | fn layerx_interop_gateway::redaction::decode |

## gw-emission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-emission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:302</code> | <code>pub struct Emission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-emission</code> | notes | struct layerx_interop_gateway::redaction::Emission; no reference outside the defining file found by name match |

## gw-emission-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-emission-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:201</code> | <code>pub struct EmissionSchema</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-emission-schema</code> | notes | struct layerx_interop_gateway::redaction::EmissionSchema; no reference outside the defining file found by name match |

## gw-emission-values

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-emission-values</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:316</code> | <code>pub fn values(&amp;self) -&gt; &amp;[FieldValue]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-emission-values</code> | notes | method layerx_interop_gateway::redaction::Emission::values; no reference outside the defining file found by name match |

## gw-emit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-emit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:267</code> | <code>pub fn emit(schema_name: &amp;str, values: &amp;[FieldValue]) -&gt; Result&lt;Vec&lt;u8&gt;, RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-emit</code> | notes | fn layerx_interop_gateway::redaction::emit |

## gw-emitted-schemas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-emitted-schemas</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:34</code> | <code>pub const EMITTED_SCHEMAS: &amp;[EmissionSchema]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-emitted-schemas</code> | notes | const layerx_interop_gateway::redaction::EMITTED_SCHEMAS; no reference outside the defining file found by name match |

## gw-error-emission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-error-emission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:160</code> | <code>pub fn error_emission(traced: &amp;Traced&lt;GatewayError&gt;) -&gt; Result&lt;Vec&lt;u8&gt;, RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-error-emission</code> | notes | fn layerx_interop_gateway::error::error_emission |

## gw-error-response-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-error-response-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:23</code> | <code>pub const ERROR_RESPONSE_SCHEMA: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-error-response-schema</code> | notes | const layerx_interop_gateway::redaction::ERROR_RESPONSE_SCHEMA |

## gw-evidence-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:135</code> | <code>pub enum EvidencePolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-evidence-policy</code> | notes | enum layerx_interop_gateway::server::EvidencePolicy |

## gw-evidence-policy-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:158</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-label</code> | notes | method layerx_interop_gateway::server::EvidencePolicy::label; no reference outside the defining file found by name match |

## gw-evidence-policy-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:146</code> | <code>pub fn parse(value: &amp;str) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-parse</code> | notes | method layerx_interop_gateway::server::EvidencePolicy::parse |

## gw-external-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-external-state</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:170</code> | <code>pub enum ExternalState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-external-state</code> | notes | enum layerx_interop_gateway::server::ExternalState |
| <code>interface.11.rs-gw-external-state</code> | overridden_by | override.J47.2 |

## gw-external-state-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-external-state-label</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:181</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-external-state-label</code> | notes | method layerx_interop_gateway::server::ExternalState::label; no reference outside the defining file found by name match |
| <code>interface.11.rs-gw-external-state-label</code> | overridden_by | override.J47.3 |

## gw-field-value

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-field-value</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:168</code> | <code>pub enum FieldValue</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-field-value</code> | notes | enum layerx_interop_gateway::redaction::FieldValue |

## gw-field-value-class

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-field-value-class</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:179</code> | <code>pub const fn class(&amp;self) -&gt; SafeClass</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-field-value-class</code> | notes | method layerx_interop_gateway::redaction::FieldValue::class; no reference outside the defining file found by name match |

## gw-gateway-core

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:346</code> | <code>pub struct GatewayCore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core</code> | notes | struct layerx_interop_gateway::gateway::GatewayCore; 48 reference lines in 18 files; callers lists the first reference in each file; re-exported at services/interop/crates/layerx-interop-gateway/src/lib.rs:13 |

## gw-gateway-core-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:401</code> | <code>pub fn adapter(&amp;self, id: &amp;AdapterId) -&gt; Option&lt;&amp;AdapterDescriptor&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::adapter |

## gw-gateway-core-adapter-ids

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter-ids</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:407</code> | <code>pub fn adapter_ids(&amp;self) -&gt; Vec&lt;AdapterId&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter-ids</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::adapter_ids |

## gw-gateway-core-begin-translation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-begin-translation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:446</code> | <code>pub fn begin_translation(&amp;mut self, principal: &amp;PrincipalId, request: &amp;TranslationRequest, trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-begin-translation</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::begin_translation |

## gw-gateway-core-complete-read-only

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-complete-read-only</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:640</code> | <code>pub fn complete_read_only(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-complete-read-only</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::complete_read_only |

## gw-gateway-core-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:361</code> | <code>pub fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-new</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::new |

## gw-gateway-core-principal-audit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-principal-audit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:419</code> | <code>pub fn principal_audit(&amp;self, principal: &amp;PrincipalId) -&gt; Option&lt;&amp;AuditChain&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-principal-audit</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::principal_audit |

## gw-gateway-core-refuse-translation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-refuse-translation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:692</code> | <code>pub fn refuse_translation(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-refuse-translation</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::refuse_translation |

## gw-gateway-core-register-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-register-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:377</code> | <code>pub fn register_adapter(&amp;mut self, descriptor: AdapterDescriptor, trace: &amp;TraceId, now: u64) -&gt; Result&lt;(), Traced&lt;GatewayError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-register-adapter</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::register_adapter |

## gw-gateway-core-registry-audit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-registry-audit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:413</code> | <code>pub const fn registry_audit(&amp;self) -&gt; &amp;AuditChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-registry-audit</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::registry_audit |

## gw-gateway-core-settle-with-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-receipt</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:506</code> | <code>pub fn settle_with_receipt(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], canonical_receipt: &amp;[u8], authorised: &amp;AuthorizedBatch, trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-receipt</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::settle_with_receipt |
| <code>interface.11.rs-gw-gateway-core-settle-with-receipt</code> | overridden_by | override.J47.16 |

## gw-gateway-core-settle-with-sequencer-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:540</code> | <code>pub fn settle_with_sequencer_receipt(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], signed_receipt: &amp;[u8], logs: Vec&lt;SequencerLog&gt;, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;SequencerSettlementError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-receipt</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::settle_with_sequencer_receipt |
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-receipt</code> | overridden_by | override.J47.1 |

## gw-gateway-core-translation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-translation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:426</code> | <code>pub fn translation(&amp;self, principal: &amp;PrincipalId, idempotency_key: [u8; 32]) -&gt; Option&lt;TranslationStatus&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-core-translation</code> | notes | method layerx_interop_gateway::gateway::GatewayCore::translation |

## gw-gateway-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:36</code> | <code>pub enum GatewayError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-error</code> | notes | enum layerx_interop_gateway::error::GatewayError; 81 reference lines in 9 files; callers lists the first reference in each file |

## gw-gateway-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:55</code> | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-error-code</code> | notes | method layerx_interop_gateway::error::GatewayError::code; no reference outside the defining file found by name match |

## gw-gateway-error-retriability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error-retriability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:75</code> | <code>pub const fn retriability(&amp;self) -&gt; Retriability</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-error-retriability</code> | notes | method layerx_interop_gateway::error::GatewayError::retriability |

## gw-gateway-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:120</code> | <code>pub struct GatewayStore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-store</code> | notes | struct layerx_interop_gateway::principal::GatewayStore |

## gw-gateway-store-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:127</code> | <code>pub fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-store-new</code> | notes | method layerx_interop_gateway::principal::GatewayStore::new |

## gw-gateway-store-principal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:134</code> | <code>pub fn principal(&amp;mut self, id: &amp;PrincipalId) -&gt; PrincipalScope&lt;'_&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal</code> | notes | method layerx_interop_gateway::principal::GatewayStore::principal |

## gw-gateway-store-principal-audit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal-audit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:161</code> | <code>pub fn principal_audit(&amp;self, id: &amp;PrincipalId) -&gt; Option&lt;&amp;AuditChain&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal-audit</code> | notes | method layerx_interop_gateway::principal::GatewayStore::principal_audit |

## gw-gateway-store-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:151</code> | <code>pub fn read(&amp;self, id: &amp;PrincipalId, table: Table, key: &amp;RowKey) -&gt; Option&lt;StoredRow&lt;'_&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-gateway-store-read</code> | notes | method layerx_interop_gateway::principal::GatewayStore::read; no reference outside the defining file found by name match |

## gw-hosted-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:28</code> | <code>pub enum HostedAdapter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter</code> | notes | enum layerx_interop_gateway::server::HostedAdapter |

## gw-hosted-adapter-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:42</code> | <code>pub const fn id(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-id</code> | notes | method layerx_interop_gateway::server::HostedAdapter::id; no reference outside the defining file found by name match |

## gw-hosted-adapter-surface

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-surface</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:56</code> | <code>pub const fn surface(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-surface</code> | notes | method layerx_interop_gateway::server::HostedAdapter::surface |

## gw-ingress-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-ingress-transport</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:8</code> | <code>pub enum IngressTransport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-ingress-transport</code> | notes | enum layerx_interop_gateway::server::IngressTransport |

## gw-ingress-transport-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-ingress-transport-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:17</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-ingress-transport-label</code> | notes | method layerx_interop_gateway::server::IngressTransport::label; no reference outside the defining file found by name match |

## gw-interop-gateway-core

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-core</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/lib.rs:38</code> | <code>pub fn interop_gateway_core() -&gt; GatewayCore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-core</code> | notes | fn layerx_interop_gateway::interop_gateway_core |

## gw-interop-gateway-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-routes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:196</code> | <code>pub fn interop_gateway_routes&lt;'a&gt;(method: &amp;str, path: &amp;'a str) -&gt; Result&lt;InteropRoute&lt;'a&gt;, RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-routes</code> | notes | fn layerx_interop_gateway::server::interop_gateway_routes |

## gw-interop-route

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-route</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:74</code> | <code>pub enum InteropRoute&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-route</code> | notes | enum layerx_interop_gateway::server::InteropRoute |

## gw-interop-route-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-route-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:99</code> | <code>pub const fn adapter(&amp;self) -&gt; Option&lt;HostedAdapter&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-route-adapter</code> | notes | method layerx_interop_gateway::server::InteropRoute::adapter |

## gw-interop-route-state-changing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-route-state-changing</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:118</code> | <code>pub const fn state_changing(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-route-state-changing</code> | notes | method layerx_interop_gateway::server::InteropRoute::state_changing; no reference outside the defining file found by name match |

## gw-interop-workspace-manifest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-interop-workspace-manifest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/lib.rs:19</code> | <code>pub struct InteropWorkspaceManifest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-interop-workspace-manifest</code> | notes | struct layerx_interop_gateway::InteropWorkspaceManifest; no reference outside the defining file found by name match |

## gw-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:141</code> | <code>pub struct Label(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-label</code> | notes | struct layerx_interop_gateway::redaction::Label |

## gw-label-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-label-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:160</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-label-as-str</code> | notes | method layerx_interop_gateway::redaction::Label::as_str; no reference outside the defining file found by name match |

## gw-label-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-label-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:149</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-label-new</code> | notes | method layerx_interop_gateway::redaction::Label::new |

## gw-pinned-spec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:90</code> | <code>pub struct PinnedSpec</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec</code> | notes | struct layerx_interop_gateway::adapter::PinnedSpec |

## gw-pinned-spec-document-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-document-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:131</code> | <code>pub const fn document_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-document-digest</code> | notes | method layerx_interop_gateway::adapter::PinnedSpec::document_digest |

## gw-pinned-spec-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:102</code> | <code>pub fn new(protocol: AdapterId, version: SpecVersion, document_digest: [u8; 32]) -&gt; Result&lt;Self, AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-new</code> | notes | method layerx_interop_gateway::adapter::PinnedSpec::new |

## gw-pinned-spec-protocol

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-protocol</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:119</code> | <code>pub const fn protocol(&amp;self) -&gt; &amp;AdapterId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-protocol</code> | notes | method layerx_interop_gateway::adapter::PinnedSpec::protocol |

## gw-pinned-spec-verify-document

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-verify-document</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:141</code> | <code>pub fn verify_document(&amp;self, document: &amp;[u8]) -&gt; Result&lt;(), AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-verify-document</code> | notes | method layerx_interop_gateway::adapter::PinnedSpec::verify_document |

## gw-pinned-spec-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:125</code> | <code>pub const fn version(&amp;self) -&gt; &amp;SpecVersion</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-version</code> | notes | method layerx_interop_gateway::adapter::PinnedSpec::version; no reference outside the defining file found by name match |

## gw-principal-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:24</code> | <code>pub struct PrincipalId(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-id</code> | notes | struct layerx_interop_gateway::principal::PrincipalId; 104 reference lines in 24 files; callers lists the first reference in each file |

## gw-principal-id-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-id-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:44</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-id-as-str</code> | notes | method layerx_interop_gateway::principal::PrincipalId::as_str; no reference outside the defining file found by name match |

## gw-principal-id-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-id-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:33</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-id-new</code> | notes | method layerx_interop_gateway::principal::PrincipalId::new |

## gw-principal-scope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:169</code> | <code>pub struct PrincipalScope&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope</code> | notes | struct layerx_interop_gateway::principal::PrincipalScope; no reference outside the defining file found by name match |

## gw-principal-scope-audit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:211</code> | <code>pub fn audit(&amp;mut self) -&gt; &amp;mut AuditChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::audit; no reference outside the defining file found by name match |

## gw-principal-scope-audit-view

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit-view</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:217</code> | <code>pub fn audit_view(&amp;self) -&gt; &amp;AuditChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit-view</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::audit_view; no reference outside the defining file found by name match |

## gw-principal-scope-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:183</code> | <code>pub fn get(&amp;self, table: Table, key: &amp;RowKey) -&gt; Option&lt;StoredRow&lt;'_&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-get</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::get; no reference outside the defining file found by name match |

## gw-principal-scope-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-keys</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:193</code> | <code>pub fn keys(&amp;self, table: Table) -&gt; Vec&lt;RowKey&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-keys</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::keys; no reference outside the defining file found by name match |

## gw-principal-scope-principal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-principal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:177</code> | <code>pub const fn principal(&amp;self) -&gt; &amp;PrincipalId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-principal</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::principal; no reference outside the defining file found by name match |

## gw-principal-scope-put

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-put</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:201</code> | <code>pub fn put(&amp;mut self, table: Table, key: RowKey, written_at: u64, bytes: Vec&lt;u8&gt;)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-principal-scope-put</code> | notes | method layerx_interop_gateway::principal::PrincipalScope::put; no reference outside the defining file found by name match |

## gw-redaction-emission-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-redaction-emission-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:310</code> | <code>pub const fn schema(&amp;self) -&gt; &amp;'static EmissionSchema</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-redaction-emission-schema</code> | notes | method layerx_interop_gateway::redaction::Emission::schema; no reference outside the defining file found by name match |

## gw-redaction-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-redaction-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:371</code> | <code>pub enum RedactionError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-redaction-error</code> | notes | enum layerx_interop_gateway::redaction::RedactionError |

## gw-retriability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-retriability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:18</code> | <code>pub enum Retriability</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-retriability</code> | notes | enum layerx_interop_gateway::error::Retriability |

## gw-retriability-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-retriability-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/error.rs:26</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-retriability-label</code> | notes | method layerx_interop_gateway::error::Retriability::label; no reference outside the defining file found by name match |

## gw-root-interop-workspace-manifest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-root-interop-workspace-manifest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/lib.rs:28</code> | <code>pub const fn interop_workspace_manifest() -&gt; InteropWorkspaceManifest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-root-interop-workspace-manifest</code> | notes | fn layerx_interop_gateway::interop_workspace_manifest; no reference outside the defining file found by name match |

## gw-route-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-route-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:285</code> | <code>pub enum RouteError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-route-error</code> | notes | enum layerx_interop_gateway::server::RouteError |

## gw-row-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-row-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:52</code> | <code>pub struct RowKey(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-row-key</code> | notes | struct layerx_interop_gateway::principal::RowKey |

## gw-row-key-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-row-key-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:71</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-row-key-as-str</code> | notes | method layerx_interop_gateway::principal::RowKey::as_str; no reference outside the defining file found by name match |

## gw-row-key-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-row-key-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:60</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-row-key-new</code> | notes | method layerx_interop_gateway::principal::RowKey::new |

## gw-safe-class

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-safe-class</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:107</code> | <code>pub enum SafeClass</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-safe-class</code> | notes | enum layerx_interop_gateway::redaction::SafeClass; no reference outside the defining file found by name match |

## gw-schema-field

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-schema-field</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:192</code> | <code>pub struct SchemaField</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-schema-field</code> | notes | struct layerx_interop_gateway::redaction::SchemaField; no reference outside the defining file found by name match |

## gw-sequencer-settlement-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-sequencer-settlement-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:280</code> | <code>pub enum SequencerSettlementError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-sequencer-settlement-error</code> | notes | enum layerx_interop_gateway::gateway::SequencerSettlementError |

## gw-spec-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-spec-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:52</code> | <code>pub struct SpecVersion(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-spec-version</code> | notes | struct layerx_interop_gateway::adapter::SpecVersion |

## gw-spec-version-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-spec-version-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:75</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-spec-version-as-str</code> | notes | method layerx_interop_gateway::adapter::SpecVersion::as_str; no reference outside the defining file found by name match |

## gw-spec-version-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-spec-version-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/adapter.rs:61</code> | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-spec-version-parse</code> | notes | method layerx_interop_gateway::adapter::SpecVersion::parse |

## gw-store-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-store-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:224</code> | <code>pub enum StoreError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-store-error</code> | notes | enum layerx_interop_gateway::principal::StoreError |

## gw-stored-row

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-stored-row</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:85</code> | <code>pub struct StoredRow&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-stored-row</code> | notes | struct layerx_interop_gateway::principal::StoredRow; no reference outside the defining file found by name match |

## gw-stored-row-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-stored-row-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:99</code> | <code>pub const fn bytes(&amp;self) -&gt; &amp;'a [u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-stored-row-bytes</code> | notes | method layerx_interop_gateway::principal::StoredRow::bytes; no reference outside the defining file found by name match |

## gw-stored-row-written-at

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-stored-row-written-at</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:93</code> | <code>pub const fn written_at(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-stored-row-written-at</code> | notes | method layerx_interop_gateway::principal::StoredRow::written_at; no reference outside the defining file found by name match |

## gw-table

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-table</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/principal.rs:78</code> | <code>pub enum Table</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-table</code> | notes | enum layerx_interop_gateway::principal::Table |

## gw-trace-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:101</code> | <code>pub enum TraceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-error</code> | notes | enum layerx_interop_gateway::trace::TraceError; no reference outside the defining file found by name match |

## gw-trace-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:10</code> | <code>pub const TRACE_HEADER: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-header</code> | notes | const layerx_interop_gateway::trace::TRACE_HEADER; no reference outside the defining file found by name match |

## gw-trace-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:29</code> | <code>pub struct TraceId(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id</code> | notes | struct layerx_interop_gateway::trace::TraceId; 229 reference lines in 36 files; callers lists the first reference in each file |

## gw-trace-id-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:72</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-as-str</code> | notes | method layerx_interop_gateway::trace::TraceId::as_str; no reference outside the defining file found by name match |

## gw-trace-id-from-inbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-from-inbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:64</code> | <code>pub fn from_inbound(header: Option&lt;&amp;str&gt;, entropy: [u8; 16]) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-from-inbound</code> | notes | method layerx_interop_gateway::trace::TraceId::from_inbound |

## gw-trace-id-mint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-mint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:50</code> | <code>pub fn mint(entropy: [u8; 16]) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-mint</code> | notes | method layerx_interop_gateway::trace::TraceId::mint; 85 reference lines in 23 files; callers lists the first reference in each file |

## gw-trace-id-outbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-outbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:79</code> | <code>pub fn outbound(&amp;self) -&gt; (&amp;'static str, &amp;str)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-outbound</code> | notes | method layerx_interop_gateway::trace::TraceId::outbound; no reference outside the defining file found by name match |

## gw-trace-id-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:38</code> | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, TraceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-parse</code> | notes | method layerx_interop_gateway::trace::TraceId::parse |

## gw-trace-id-wrap

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-wrap</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:85</code> | <code>pub fn wrap&lt;E&gt;(&amp;self, error: E) -&gt; Traced&lt;E&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-trace-id-wrap</code> | notes | method layerx_interop_gateway::trace::TraceId::wrap; 45 reference lines in 9 files; callers lists the first reference in each file |

## gw-traced

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-traced</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:118</code> | <code>pub struct Traced&lt;E&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-traced</code> | notes | struct layerx_interop_gateway::trace::Traced |

## gw-traced-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-traced-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:138</code> | <code>pub const fn error(&amp;self) -&gt; &amp;E</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-traced-error</code> | notes | method layerx_interop_gateway::trace::Traced::error; no reference outside the defining file found by name match |

## gw-traced-into-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-traced-into-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:144</code> | <code>pub fn into_error(self) -&gt; E</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-traced-into-error</code> | notes | method layerx_interop_gateway::trace::Traced::into_error |

## gw-traced-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-traced-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:126</code> | <code>pub fn new(trace: TraceId, error: E) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-traced-new</code> | notes | method layerx_interop_gateway::trace::Traced::new; no reference outside the defining file found by name match |

## gw-traced-trace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-traced-trace</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/trace.rs:132</code> | <code>pub const fn trace(&amp;self) -&gt; &amp;TraceId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-traced-trace</code> | notes | method layerx_interop_gateway::trace::Traced::trace |

## gw-translation-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:64</code> | <code>pub enum TranslationKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-kind</code> | notes | enum layerx_interop_gateway::gateway::TranslationKind |

## gw-translation-kind-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-kind-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:72</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-kind-label</code> | notes | method layerx_interop_gateway::gateway::TranslationKind::label; no reference outside the defining file found by name match |

## gw-translation-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:99</code> | <code>pub struct TranslationRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request</code> | notes | struct layerx_interop_gateway::gateway::TranslationRequest |

## gw-translation-request-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:131</code> | <code>pub const fn adapter(&amp;self) -&gt; &amp;AdapterId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request-adapter</code> | notes | method layerx_interop_gateway::gateway::TranslationRequest::adapter |

## gw-translation-request-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:143</code> | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request-idempotency-key</code> | notes | method layerx_interop_gateway::gateway::TranslationRequest::idempotency_key; no reference outside the defining file found by name match |

## gw-translation-request-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:137</code> | <code>pub const fn kind(&amp;self) -&gt; TranslationKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request-kind</code> | notes | method layerx_interop_gateway::gateway::TranslationRequest::kind; no reference outside the defining file found by name match |

## gw-translation-request-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:112</code> | <code>pub fn new(adapter: AdapterId, kind: TranslationKind, idempotency_key: [u8; 32], request_digest: [u8; 32]) -&gt; Result&lt;Self, GatewayError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request-new</code> | notes | method layerx_interop_gateway::gateway::TranslationRequest::new |

## gw-translation-request-request-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-request-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:149</code> | <code>pub const fn request_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-request-request-digest</code> | notes | method layerx_interop_gateway::gateway::TranslationRequest::request_digest; no reference outside the defining file found by name match |

## gw-translation-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:158</code> | <code>pub enum TranslationStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-status</code> | notes | enum layerx_interop_gateway::gateway::TranslationStatus; 62 reference lines in 8 files; callers lists the first reference in each file |

## gw-translation-status-label

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-translation-status-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/gateway.rs:168</code> | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-translation-status-label</code> | notes | method layerx_interop_gateway::gateway::TranslationStatus::label; no reference outside the defining file found by name match |

## gw-verify-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-verify-export</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/audit.rs:332</code> | <code>pub fn verify_export(name: &amp;str, exported: &amp;[Vec&lt;u8&gt;], head: &amp;ChainHead) -&gt; Result&lt;Vec&lt;ChainEntry&gt;, AuditError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-verify-export</code> | notes | fn layerx_interop_gateway::audit::verify_export |

## gw-verify-registry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-verify-registry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:242</code> | <code>pub fn verify_registry() -&gt; Result&lt;(), RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-verify-registry</code> | notes | fn layerx_interop_gateway::redaction::verify_registry; no reference outside the defining file found by name match |

## gw-verify-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gw-verify-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/redaction.rs:214</code> | <code>pub fn verify_schema(schema: &amp;EmissionSchema) -&gt; Result&lt;(), RedactionError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gw-verify-schema</code> | notes | fn layerx_interop_gateway::redaction::verify_schema; no reference outside the defining file found by name match |

## http-gw-adapters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-adapters</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:203</code> | <code>GET /v1/adapters</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-adapters</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler metadata at services/interop/crates/layerx-interop-service/src/server.rs:2846; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-ap2-execute

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-ap2-execute</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:244</code> | <code>POST /v1/http/ap2/execute</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-ap2-execute</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler ap2_execute at services/interop/crates/layerx-interop-service/src/server.rs:1451; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt; spec/layerx-merge/spec.kvx task 7.5.18 verifies the AP2 settlement on sequencer receipts |

## http-gw-ap2-mandates-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-ap2-mandates-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:241</code> | <code>POST /v1/http/ap2/mandates/verify</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-ap2-mandates-verify</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler ap2 at services/interop/crates/layerx-interop-service/src/server.rs:1361; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-fiat-bank-callbacks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-fiat-bank-callbacks</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:259</code> | <code>POST /v1/http/fiat/bank/callbacks</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-fiat-bank-callbacks</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler fiat at services/interop/crates/layerx-interop-service/src/server.rs:2253; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt; spec/layerx-merge/spec.kvx task 7.5.21 |

## http-gw-fiat-card-callbacks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-fiat-card-callbacks</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:254</code> | <code>POST /v1/http/fiat/card/callbacks</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-fiat-card-callbacks</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler fiat at services/interop/crates/layerx-interop-service/src/server.rs:2253; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt; spec/layerx-merge/spec.kvx task 7.5.21 settles fiat on GatewayCore::settle_with_sequencer_receipt |

## http-gw-fiat-rtp-callbacks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-fiat-rtp-callbacks</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:264</code> | <code>POST /v1/http/fiat/rtp/callbacks</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-fiat-rtp-callbacks</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler fiat at services/interop/crates/layerx-interop-service/src/server.rs:2253; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt; spec/layerx-merge/spec.kvx task 7.5.21 |

## http-gw-livez

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:201</code> | <code>GET /livez</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-livez</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler live at services/interop/crates/layerx-interop-service/src/server.rs:2743; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-migration-accounts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-migration-accounts</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:204</code> | <code>POST /v2/migration/accounts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-migration-accounts</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler migration_account_v2 at services/interop/crates/layerx-interop-service/src/server.rs:705; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; account mapping reads addr 0x1004 getEvmAddr/getPaxAddr through layerx-migrate PaxeerBindingVerifierV2 by eth_call at one finalized block |

## http-gw-migration-assets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-migration-assets</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:205</code> | <code>POST /v2/migration/assets</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-migration-assets</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler migration_asset_v2 at services/interop/crates/layerx-interop-service/src/server.rs:598; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; posts the verified source settlement to the ramp at POST /internal/v2/source-settlements (services/interop/crates/layerx-interop-service/src/server.rs:646) |

## http-gw-migration-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-migration-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:206</code> | <code>POST /v2/migration/history</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-migration-history</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler migration_history_import_v2 at services/interop/crates/layerx-interop-service/src/server.rs:484; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-migration-history-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-migration-history-read</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:207</code> | <code>POST /v2/migration/history/read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-migration-history-read</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler migration_history_read_v2 at services/interop/crates/layerx-interop-service/src/server.rs:537; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-operations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-operations</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:211</code> | <code>GET /v1/operations/{operation: 64 hex}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-operations</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler resume_operation at services/interop/crates/layerx-interop-service/src/server.rs:280; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; resumes a stored idempotent operation; a non-64-hex key is refused as RouteError::InvalidOperation |

## http-gw-readyz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:202</code> | <code>GET /readyz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-readyz</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler ready at services/interop/crates/layerx-interop-service/src/server.rs:2825; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-ucp-checkouts-complete

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-ucp-checkouts-complete</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:245</code> | <code>POST /v1/http/ucp/checkouts/complete</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-ucp-checkouts-complete</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler ucp at services/interop/crates/layerx-interop-service/src/server.rs:1724; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt |

## http-gw-visa-tap-intents-execute

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-execute</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:251</code> | <code>POST /v1/http/visa-tap/intents/execute</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-execute</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler visa_execute at services/interop/crates/layerx-interop-service/src/server.rs:1992; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt |

## http-gw-visa-tap-intents-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:248</code> | <code>POST /v1/http/visa-tap/intents/verify</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-verify</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler visa at services/interop/crates/layerx-interop-service/src/server.rs:1881; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-x402-buyer-build

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-x402-buyer-build</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:229</code> | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/buyer/build</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-x402-buyer-build</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler x402_buyer at services/interop/crates/layerx-interop-service/src/server.rs:913; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 0.101 replaces the buyer AuthorizedBatch verification with the signed sequencer receipt |

## http-gw-x402-seller-offer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-x402-seller-offer</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:232</code> | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/seller/offer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-x402-seller-offer</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler x402_seller at services/interop/crates/layerx-interop-service/src/server.rs:979; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-x402-settle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-x402-settle</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:238</code> | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/settle, /seller/settle and /facilitator/settle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-x402-settle</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler x402_settle at services/interop/crates/layerx-interop-service/src/server.rs:1260; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 do_3 matches PaymentExact by payment reference or PaymentDrawn by idempotency key in settle (task cites services/interop/crates/layerx-interop-service/src/server.rs:1109; x402_settle is at :1260 at HEAD) |

## http-gw-x402-supported

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-x402-supported</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:226</code> | <code>GET /v1/{http&#124;mcp&#124;a2a}/x402/supported and /v1/{transport}/x402/facilitator/supported</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-x402-supported</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler x402_supported at services/interop/crates/layerx-interop-service/src/server.rs:895; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes |

## http-gw-x402-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gw-x402-verify</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-interop-gateway/src/server.rs:235</code> | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/verify and /v1/{transport}/x402/facilitator/verify</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gw-x402-verify</code> | notes | parsed by interop_gateway_routes; served by layerx-interop-service dispatch (services/interop/crates/layerx-interop-service/src/server.rs:411) handler x402_verify at services/interop/crates/layerx-interop-service/src/server.rs:1118; body JSON {ok, operation, result&#124;error.code, trace}; Authorization bearer API key, Idempotency-Key on state-changing routes; spec/layerx-merge/spec.kvx task 7.5.14 removes the hosted-gateway activity submission (services/interop/crates/layerx-interop-service/src/server.rs:2526 inside submit at :2517) this route executes through; payer-signed raw transactions relay by eth_sendRawTransaction and receipts come from pax_sequencerReceipt |
