# Agent formats contract

This inventory describes the code and schemas in the merged tree, reviewed against revision `4edaf4fff3f943e154c8337c0bcc16fe4dd9dd3c`. Source links identify the implementations; the former pinned inventory and its planned-task statuses are superseded. Existing section anchors are retained for specification links. Test references describe coverage in the tree, not a claim that those tests ran for this documentation change.

Paxeer mainnet-beta is one BFT network with EVM chain id **125** (`0x7d`). Its kernel participates in the network and proposes blocks. A receipt is final when its block is committed. Public node JSON-RPC uses HTTPS at the root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, and `api-mainnet-beta.paxeer.network`. Agent operation envelopes and local durable records below are application formats, not additional public node endpoints. Tenant domains are database namespaces, without a new chain deployment for each tenant. See the [network audit brief](../programme/audit/BRIEF.kvx) and [public RPC reference](../../../docs/site/docs/reference/public-rpc.md).

## agent-api-golden

Interfaces: `interface.9.fmt-agent-api-golden-version-request-hex` and `interface.9.fmt-agent-api-golden-version-response-hex`.

The [request golden](../../../services/agent/schema/agent-api/golden/version-request.hex) is `00020000000000000000000700000000020000`; the [response golden](../../../services/agent/schema/agent-api/golden/version-response.hex) is `0002000000000000000000070000000000000200000001`. They describe request id 7, contract 2.0 and node-interface major 1. The latter is a handshake compatibility field, not another network endpoint.

## agent-api-request

Interface: `interface.9.fmt-agent-api-request-frame`.

The `[encoding]` declaration in [v1.kvx](../../../services/agent/schema/agent-api/v1.kvx) is `contract_major:u16be contract_minor:u16be request_id:u64be operation:utf8 payload:canonical-map`. The filename is retained; its top-level contract version is **2.0**.

The HTTP operation representation implemented by [agent_rpc.rs](../../../services/agent/crates/layerx-agentd/src/agent_rpc.rs) is a JSON object with `version`, `request_id`, `operation`, `request`, `credential`, and an optional `idempotency_key`. Envelope version is **1**, independent of contract major 2. Request ids and credential generations use canonical decimal strings. Credential coordinates are `tenant`, `session_id`, `token_id`, `generation`. The body bound is **1,048,576 bytes**. Unknown fields, incompatible envelope versions and malformed coordinates are refused before dispatch; session authorization controls access to owner operations.

## agent-api-request-frame

`agent_rpc::version_exchange` implements the binary version handshake. A major-2 request receives contract 2.0; other majors produce `ProtocolIncompatibility` with reason `envelope.version`. Malformed or trailing bytes produce `envelope.malformed`. This preserves the useful criterion `acceptance.9.2.2` / `req.9002.2`.

Coverage: [rpc_surface_major_two.rs](../../../services/agent/crates/layerx-agentd/tests/rpc_surface_major_two.rs), function `version_exchange_reports_contract_major_two`. Its envelope tests separately cover JSON version 1 and the HTTP goldens.

## agent-api-response

Interface: `interface.9.fmt-agent-api-response-frame`.

The schema's binary declaration is `contract_major:u16be contract_minor:u16be request_id:u64be result_code:u32be payload:canonical-map`. HTTP responses are rendered by [agent_rpc.rs](../../../services/agent/crates/layerx-agentd/src/agent_rpc.rs). Success carries `request_id`, `value`, and `verification_status`; errors use the [error schema](../../../services/agent/schema/agent-api/errors.kvx). Envelope-version refusal is `envelope.version`; the retired operation `faucet.claim` produces `UnavailableCapability` with reason `unavailable_capability.faucet.claim`.

Transaction rungs are `submitted`, `receipted`, `committed`. A submission or transport timeout alone is not execution evidence. A verified status-0 receipt records a revert. Committed evidence binds the landing block and its verified CometBFT commit; it does not require another settlement network.

## agent-api-schema-approval

Interface: `interface.9.fmt-agent-api-schema-approval`.

[approval.kvx](../../../services/agent/schema/agent-api/approval.kvx) declares approval records, holds and approval operations. Its module-local `contract_major = 1` is retained in the source; the including top-level schema advertises contract 2.0. Module header values must not be substituted for handshake negotiation.

## agent-api-schema-errors

Interface: `interface.9.fmt-agent-api-schema-errors`.

[errors.kvx](../../../services/agent/schema/agent-api/errors.kvx) declares `ApiError` fields `class`, `protocol_result_code`, `retriability`, `request_id`, `reason`, together with transaction rungs and success verification status. The [Rust error contract](../../../services/agent/crates/layerx-agent-api/src/error.rs) supplies the corresponding types. Error reasons are machine-readable identifiers, separate from numeric protocol results.

## agent-api-schema-identity

Interface: `interface.9.fmt-agent-api-schema-identity`.

[identity.kvx](../../../services/agent/schema/agent-api/identity.kvx) declares tenant readiness, sessions, capabilities, budgets and local policy operations. Its module-local major remains 1. The `golden/` files are compatibility snapshots; they must not be assumed to duplicate every live module declaration.

## agent-api-schema-programs

Interface: `interface.9.fmt-agent-api-schema-programs`.

[programs.kvx](../../../services/agent/schema/agent-api/programs.kvx) declares program operations using ABI calls, EIP-1559 transactions and the three transaction rungs. Its module-local major remains 1. Program operation results must be interpreted using the current operation declaration and its receipt evidence, rather than historical verification-level terminology.

## agent-api-schema-read

Interface: `interface.9.fmt-agent-api-schema-read`.

[read.kvx](../../../services/agent/schema/agent-api/read.kvx) declares read freshness, proof bundles, availability, projections and offline export. [read.rs](../../../services/agent/crates/layerx-agent-api/src/read.rs) defines the Rust types and JSON codecs, re-exported through namespaces in [lib.rs](../../../services/agent/crates/layerx-agent-api/src/lib.rs).

The live offline-export response contains `facts`, `bundles`, `height`. [golden/read.kvx](../../../services/agent/schema/agent-api/golden/read.kvx) still contains the earlier `facts`, `receipts`, `proofs`, `certificates`, `headers` declaration. It is a compatibility baseline, not the current export shape. See [offline-export](#offline-export).

## agent-api-schema-stream

Interface: `interface.9.fmt-agent-api-schema-stream`.

[stream.kvx](../../../services/agent/schema/agent-api/stream.kvx) declares tenant and capability restrictions, subscription records, cursor acknowledgement and delivery. Creation persists a definition and acknowledged cursor for restoration after restart. Its module-local major remains 1; consult the live module for current fields instead of treating the golden snapshot as an operational inventory.

## agent-api-schema-v1

Interface: `interface.9.fmt-agent-api-schema-v1`.

[v1.kvx](../../../services/agent/schema/agent-api/v1.kvx) includes `identity.kvx`, `write.kvx`, `read.kvx`, `stream.kvx`, `errors.kvx`, `approval.kvx`, and `programs.kvx`. The API crate's `agent_api_schema_v1` retains its historical name while reporting contract 2.0. Consensus integers use fixed-width Rust types and decimal strings at dynamic-language boundaries.

Historical migration criteria `req.9007` and `req.9008` describe the move to rungs and removal of settlement-domain vocabulary. They are not evidence that every legacy compatibility file has been rewritten. The current producer and consumer determine the format documented here.

## agent-api-schema-write

Interface: `interface.9.fmt-agent-api-schema-write`.

[write.kvx](../../../services/agent/schema/agent-api/write.kvx) declares preparation, signing, submission, tracking and waiting. Submission responses carry `state`, `evidence`, `rung`, `transitions`. Waiting names `requested_rung`; a deadline reports the actual reached rung rather than inventing a stronger result. Unknown submission outcomes remain submitted until authenticated evidence resolves them.

## agentd-startup-config

Interface: `interface.9.fmt-agentd-startup-config`.

[config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) loads a protected UTF-8 `key=value` file, bounded to **65,536 bytes** and **4,096 bytes per line**. Exact environment keys override file values:

| File key | Environment key |
| --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` |
| `tenants` | `LAYERX_TENANTS` |
| `policy_sources` | `LAYERX_POLICY_SOURCES` |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` |

`node_endpoint` is an HTTP(S) node URL; mainnet uses the root endpoints above and expected chain id 125. `expected_protocol_version` and its environment spelling are retired and explicitly rejected. Unknown or duplicate settings and incomplete tenant maps are errors.

## agentd-store

Interface: `interface.9.fmt-agentd-store`.

[store/mod.rs](../../../services/agent/crates/layerx-agentd/src/store/mod.rs) persists `store.bin` with magic `LXAS` and schema version **3**. [store/migrate.rs](../../../services/agent/crates/layerx-agentd/src/store/migrate.rs) supports migration from version 1. `TenantKey` combines tenant, object kind and object id; receipt and retained-export keys below remain tenant-scoped.

## audit-log

Interface: `interface.9.fmt-audit-log`.

[audit/log.rs](../../../services/agent/crates/layerx-agentd/src/audit/log.rs) defines `audit.chain` (`LXAUD001`) and `audit.anchor` (`LXANC001`), entry markers `LXAE` and `LXAC`, and hash domain `layerx-agent-audit-v1`. The log checks sequence, previous hash, entry hash and trusted tail anchor. This local audit chain is distinct from network block commitment.

## audit-record

Interface: `interface.9.fmt-audit-record`.

[audit/record.rs](../../../services/agent/crates/layerx-agentd/src/audit/record.rs) writes `LXAR` version **3** and retains version **2** reading. Version 1 is refused. The useful v3 criterion is round-trip preservation of transaction hash, nonce and rung (`req.9028.1`); version-2 compatibility is `req.9028.2`. Coverage is in [audit_record_v3.rs](../../../services/agent/crates/layerx-agentd/tests/audit_record_v3.rs): `audit_entry_v3_records_transaction_hash_and_rung` and `version_two_audit_entries_stay_readable`.

## budget-proof

Interface: `interface.9.fmt-budget-proof`.

[budget/budget_proof.rs](../../../services/agent/crates/layerx-agentd/src/budget/budget_proof.rs) accepts schema **`layerx.human.budget-proof.v2`**. The bounded JSON export binds tenant, principal, protocol version, network id, a positive `head.committed_height`, and block hash to the supplied checkpoint. Verification checks the committed budget state, including the `getBudget` result; schema v1 is refused with `BudgetProofError::Refused`. The maximum proof document size is **1,048,576 bytes**. This retains the useful v2 criterion `req.9033.2` without advertising v1 as current.

## human-peer-frame

Interface: `interface.9.fmt-human-peer-frame`.

[human.rs](../../../services/agent/crates/layerx-agentd/src/human.rs) retains magic `LXHAGT01`; traced envelopes use `LXHAGT02`. [human_frame.rs](../../../services/agent/crates/layerx-agentd/src/human_frame.rs) re-exports the shared client [frame codec](../../../services/agent/crates/layerx-client/src/frame/framing.rs) and transport as `HumanFrameTransport`. Frames have a four-byte unsigned big-endian length prefix, checked against the configured maximum before allocating the body. This is local peer framing, not the retired LNI protocol.

`PREPARE` remains opcode 1, `HEAD` 7, and `EVIDENCE` 8. Preparation checks the pending nonce and returns `prepare.stale_nonce` on mismatch. Transaction evidence carries transaction and receipt hashes, observed height and rung; receipted is wire rank 1 and committed rank 2. Useful framing and rung criteria remain `req.9046` and `req.9047`; the opcode table in `human.rs` is authoritative rather than the old inventory's opcode count.

## lni-capabilities

Interface: `interface.9.fmt-lni-v1-capabilities` is retired. The former `services/agent/schema/lni/v1.kvx` is absent from this tree. Its capability list is not startup or node negotiation configuration for the merged network.

## lni-frame

Interface: `interface.9.fmt-lni-v1-frame` is retired. Do not construct the former LNI frame for node calls. Local human-peer framing is described [above](#human-peer-frame); node requests use the configured JSON-RPC endpoint.

## mcp-binding-document

Interface: `interface.9.fmt-mcp-binding-document`.

[layerx-mcp/src/binding.rs](../../../services/agent/crates/layerx-mcp/src/binding.rs) parses a closed-key JSON document of at most **65,536 bytes**. Top-level keys are `mode`, `tenant`, `store`, `audit_root`, `session_id`, `session_token_file`, `session_generation`, `capability_id`, `core_height`, `deadline_ms`, `agent`, `limit`, and optional `listener` and `web` objects. Modes are `full` and `read-only`.

The former `core_sequence` key is retired and refused as `mcp.binding_retired_key`. The former `web.sequencer_public_key` is retired; the parser requires the `receipt_key_index` form instead. Internal field names retained for compatibility do not authorize the retired binding keys.

## mcp-invocation

Interface: `interface.9.fmt-mcp-invocation`.

[session_control.rs](../../../services/agent/crates/layerx-agentd/src/session_control.rs) persists `layerx.mcp.invocation.v1` records with tenant, actor, session, generation, idempotency key, request digest, phase and response. Repeated invocations match the durable identity; a settled record returns its retained response, while an unresolved record remains unknown. This record version is independent of the binding document's `core_height` key.

## native-effect-policy

Interface: `interface.9.fmt-native-effect-policy`.

[policy/native_program.rs](../../../services/agent/crates/layerx-agentd/src/policy/native_program.rs) defines `NativeEffectPolicy::SCHEMA` as `layerx.native-effect-policy.v1`. The policy evaluates disclosed native effects bound to the prepared activity. Its document version is independent of API contract version.

## native-effect-policy-source

Interface: `interface.9.fmt-native-effect-policy-source`.

`config::load_native_effect_policy` in [config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) reads a protected JSON wrapper with exactly `version`, `tenant`, `policy`. Version must be `layerx.native-effect-policy-source.v1`, tenant must match the requested tenant, and `policy` is a string containing the policy document. The wrapper read bound is **2,097,152 bytes**.

## native-program-policy

Interface: `interface.9.fmt-native-program-policy`.

[policy/native_program.rs](../../../services/agent/crates/layerx-agentd/src/policy/native_program.rs) loads `layerx.native-program-policy.v1` with a **1,048,576-byte** source bound. It validates native program activity and disclosure bindings before evaluating policy.

## native-program-policy-source

Interface: `interface.9.fmt-native-program-policy-source`.

`config::load_native_program_policy` reads the protected wrapper `version`, `tenant`, `policy`, with version `layerx.native-program-policy-source.v1`. Tenant must match, unknown fields are rejected, and `policy` is the serialized policy string. The wrapper bound is **2,097,152 bytes**; the policy loader applies its own bound.

## offline-export

Interface: `interface.9.fmt-offline-export`.

The current [Rust `OfflineExport`](../../../services/agent/crates/layerx-agent-api/src/read.rs) and live [read schema](../../../services/agent/schema/agent-api/read.kvx) use the following value inside `VerifiedRead<OfflineExport>`:

| Field | Current representation |
| --- | --- |
| `facts` | Requested references in request order; 1 through 16 unique references |
| `bundles` | Corresponding portable `layerx-receipt-proof-v2` proof bundles, all at committed rung |
| `height` | Common committed height, encoded as a canonical decimal u64 string in JSON |

The shared [fact parser](../../../services/agent/crates/layerx-proof/src/export_codec.rs) accepts `receipt:<transaction_hash>` and `state:<transaction_hash>:<account_id>`. Identifiers are nonzero, 32-byte lowercase hex without a prefix; a reference is at most 135 bytes. Empty, duplicate, malformed and oversized fact sets are refused. A state fact must match the owner's bound account.

[export.rs](../../../services/agent/crates/layerx-agentd/src/export.rs) implements `retain_bundle`, `produce_retained`, `produce`, `build`, `verify`, `serialize_export`, and `deserialize_export`. Retention stores a verified single-fact JSON artifact under the tenant's Configuration key `export-proof-v2:<canonical fact>`. Production reads those retained records, preserves request order, rejects missing ownership or mixed heights, and verifies the assembled artifact. Identical retention is idempotent; conflicting records are refused.

Each bundle includes `format`, `verificationLevel`, `rawTransaction`, `signedReceipt`, `receiptDigest`, `receiptKey`, and committed evidence `blockHeader`, `receiptInclusion`, `commit`, `validatorSet`; state evidence additionally uses `accountProof`. The proof-bundle validator bounds binary evidence to **262,144 bytes** before hex encoding. Export serialization and the complete transport response must fit **1,048,576 bytes** (`MAX_EXPORT_BYTES = agent_rpc::MAX_BODY_BYTES`); oversized artifacts are refused, never truncated.

The verifier binds raw transaction hash, signed receipt, receipt key and interval, receipt inclusion, logs, header and CometBFT commit to one block. It checks account proof when the fact requires it. Receipt keys and validator-set context are independently supplied through `ExportTrustSource`; a returned key or claimed rung alone is not trust. The response reports committed rung and block-relative freshness for the verified height.

The older certificate/header vector layout is superseded in the live Rust mirror. The [golden read baseline](../../../services/agent/schema/agent-api/golden/read.kvx) still preserves that earlier declaration; do not send it as the current export. The portable proof format version is also distinct from the LXRE durable receipt-evidence version [below](#receipt-evidence).

Coverage references: [export_v2.rs](../../../services/agent/crates/layerx-agentd/tests/export_v2.rs), `offline_export_verifies_with_only_receipt_keys_and_validator_set`; [export_offline_bounds.rs](../../../services/agent/crates/layerx-agentd/tests/export_offline_bounds.rs), `export_frame_is_bounded_at_exactly_one_mebibyte` and `export_response_must_answer_every_requested_fact_at_the_requested_rung`. These are source references, not qualification results for this rewrite.

## program-budget-denominations

Interface: `interface.9.fmt-program-budget-denominations`.

`config::load_program_budget_denominations` reads protected JSON version **1** with `tenant` and `limits` rows containing `stable_id`, `asset`, `source`. Identifiers use 64 lowercase hex characters; `source` may also be `*` for an unbound source. The requested tenant must match the document. See [config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs).

## receipt-evidence

Interface: `interface.9.fmt-receipt-evidence`.

[receipt.rs](../../../services/agent/crates/layerx-agentd/src/receipt.rs) defines **LXRE version 2** (`EVIDENCE_VERSION = 2`) for `TransactionReceiptEvidence`. The durable byte layout is:

| Field | Encoding |
| --- | --- |
| Evidence prefix | `LXRE`, version byte `2` |
| Metadata prefix | `LXRM`, version byte `2` |
| Transaction hash | 32 bytes |
| Block height, transaction index | Two big-endian u64 values |
| Status | One byte, 0 or 1 |
| Gas used | Big-endian u64 |
| Logs bloom | 256 bytes |
| Receipt signature | 65 bytes |
| Rung | One byte: 1 receipted, 2 committed |
| Canonical receipt | Big-endian u32 byte length, then exact nonempty receipt bytes |

`store_verified_if_absent` with `TransactionReceiptEvidence` persists receipt bytes under `ObjectKind::Receipt`, object id `receipt:transaction:` followed by the raw transaction hash. Evidence uses `ObjectKind::Configuration`, object id `receipt-evidence:` followed by the same hash. Both keys include the tenant. This storage boundary checks hash, byte equality and minimum rung; it is not itself a cryptographic receipt verifier.

`serve_evidence` returns the record by tenant and transaction hash, checks the metadata hash and exact stored receipt bytes, and returns `None` for an absent evidence key. Identical writes are idempotent; conflicting metadata is refused. A stronger rung can replace a weaker rung for the same observation, while a weaker repeat preserves the stronger record. Submitted evidence is refused as `receipt.not_receipted`. Bad encodings or mismatched stored bytes yield `receipt.corrupt`.

LXRE version 1 is retired for this transaction-evidence reader and produces `receipt.evidence_version`; it is not silently upgraded or replaced. Legacy kernel evidence helpers remain in `receipt.rs` for recovery, but their record and indexes must not be confused with the transaction-hash v2 path.

The useful criteria remain:

| Criterion | Existing coverage and assertion |
| --- | --- |
| `acceptance.9.18.1` / `req.9018.1` | [receipt_store_v2.rs](../../../services/agent/crates/layerx-agentd/tests/receipt_store_v2.rs), `receipt_evidence_round_trips_by_transaction_hash`: height 12 and index 1 round-trip with all fields and exact bytes, including after reopening; the stored prefixes are `LXRE` v2 and `LXRM` v2 |
| `acceptance.9.18.2` / `req.9018.2` | Same file, `version_one_receipt_evidence_is_refused`: persisted v1 evidence produces `receipt.evidence_version` and remains unchanged |

## receipt-metadata

Interface: `interface.9.fmt-receipt-metadata`.

Current `TransactionReceiptMetadata` is embedded as `LXRM` v2 inside [receipt evidence](#receipt-evidence). It contains transaction hash, block height, transaction index, status, gas used, bloom, signature and rung. The separate 81-byte `LXRM` metadata codec and activity/idempotency/global-sequence indexes still present in `receipt.rs` belong to legacy recovery. They do not describe the transaction-evidence record above.

## registry-source-config

Interface: `interface.9.fmt-registry-source-config`.

[registry_source.rs](../../../services/agent/crates/layerx-agentd/src/registry_source.rs) defines closed-key JSON configuration with `version`, `endpoint`, `server_ca_der`, `client_certificate_pem`, `client_private_key_pem`, `request_token_file`, `deadline_ms`, `maximum_response_bytes`. The provider loads protected credentials, requires an HTTPS endpoint, and bounds its response. These are registry-source settings, not node JSON-RPC settings.

## unknown-resolution-state

Interface: `interface.9.fmt-unknown-resolution-state`.

[outbox/unknown.rs](../../../services/agent/crates/layerx-agentd/src/outbox/unknown.rs) writes magic **`LXU2`**, followed by transaction hash (32 bytes), first-observed time (u64), attempt count (u32), and next-attempt time (u64), for a total of **56 bytes**. Integers are big-endian. Loading validates the hash and time coordinate.

The former `LXUR` state is retired and refused with `unknown.state_version`, preserving criterion `req.9016.3`; see [unknown_receipt_lookup.rs](../../../services/agent/crates/layerx-agentd/tests/unknown_receipt_lookup.rs), `legacy_lxur_resolution_state_is_refused`. Resolution uses authenticated receipt lookup for the retained signed transaction. It does not infer success from a missing response or create a new transaction merely to resolve an unknown outcome.
