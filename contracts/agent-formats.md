# Agent formats

These formats belong to agents operating on the merged Paxeer network. Tenant
coordinates partition durable state; they do not select another blockchain.
The [API](agent-api.md) describes authentication and evidence semantics;
[configuration](agent-config.md) describes how trusted sources are loaded.
The [Human peer](agent-human-peer.md) and [Rust API](agent-rust-api.md) identify
the owning codecs. Preserve canonical bytes and explicit versions when storing,
replaying or exporting evidence.

## Agent API schema v1

The filename [v1.kvx](../services/agent/schema/agent-api/v1.kvx) is historical:
the current schema declares major **2**, minor **0**, and node interface major 1.
Its includes define identity, write, read, stream, errors, approval and Programs.
Generated SDK output is a projection of these sources. Changing an existing
declaration requires a major-version change; additions within a major must
preserve compatibility.

| Anchor | Source | Contents |
| --- | --- | --- |
| <a id="agent-api-schema-identity"></a>Identity | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) | Registration, sessions, capabilities and budgets |
| <a id="agent-api-schema-write"></a>Write | [write.kvx](../services/agent/schema/agent-api/write.kvx) | Preparation, signing, submission and tracking |
| <a id="agent-api-schema-read"></a>Read | [read.kvx](../services/agent/schema/agent-api/read.kvx) | Reads, projection, proof retrieval and export |
| <a id="agent-api-schema-stream"></a>Stream | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) | Durable subscriptions, delivery and gaps |
| <a id="agent-api-schema-errors"></a>Errors | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) | Error envelope and retry classification |
| <a id="agent-api-schema-approval"></a>Approval | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) | Held decisions and approval lifecycle |
| <a id="agent-api-schema-programs"></a>Programs | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) | Native EIP-1559 lifecycle, calls, simulation and receipt types |

## Agent API request

The schema's binary envelope declaration is
`contract_major:u16be contract_minor:u16be request_id:u64be operation:utf8 payload:canonical-map`.
It is distinct from the served Agent HTTP JSON envelope, whose own version is 1.
HTTP request fields are `version`, `request_id`, `operation`, `request`,
`credential` and optional `idempotency_key`; see
[transport](agent-api.md#agent-http-transport) for authority and mutation rules.
JSON consensus integers use canonical decimal strings, without signs, padding
or whitespace. Fixed identifiers are lowercase hexadecimal with exact widths.
Do not convert u64/u128 quantities through floating-point numbers.

Native activity records retain `protocol_version`, `module` and `ordinal`.
Native purpose, signature and local-grant records are closed versioned objects;
their canonical encodings are declared in `v1.kvx` and implemented in
[identity.rs](../services/agent/crates/layerx-agent-api/src/identity.rs).
A structurally valid signature carrier does not establish signing authority:
the runtime must verify the actual signer, current ownership, expiry and session.

## Agent API response

The binary response declaration is
`contract_major:u16be contract_minor:u16be request_id:u64be result_code:u32be payload:canonical-map`.
Agent HTTP success instead contains `request_id`, `value` and
`verification_status`; failure carries the lossless `ApiError` fields. The
HTTP protocol result is an optional signed i32, so retain negative codes exactly.
`pending`, `unverified` and `absent` are real observations, not successful
verification. See [rungs and errors](agent-api.md#verification-and-errors).

## Agent API golden

The [golden directory](../services/agent/schema/agent-api/golden) contains
schema baselines, handshake hex files and Agent HTTP JSON examples. Historical
version examples remain compatibility inputs; use the current schema's declared
major for a new client. HTTP examples include achieved, pending, absent and
refusal responses. A golden shape does not replace owner-side validation.

## Agentd startup config

The reusable loader consumes a protected UTF-8 `key=value` file, bounded to
65,536 bytes with 4,096-byte lines. Values are not TOML or shell expressions.
Exact overrides and the executable/library distinction are described under
[startup](agent-config.md#agentd-startup).

## Agentd store

[`Store`](../services/agent/crates/layerx-agentd/src/store/mod.rs) persists
`store.bin`, with magic `LXAS` and current schema version **3**. Forward migration
supports versions 1 through 3 and refuses newer schemas. Every `TenantKey`
contains tenant, object kind and opaque ID; canonical key bytes use a big-endian
u16 tenant length, UTF-8 tenant, u8 kind, big-endian u32 ID length and ID bytes.
Never recover a record by dropping its tenant coordinate.

## Audit log

[`audit/log.rs`](../services/agent/crates/layerx-agentd/src/audit/log.rs) owns
the append log `LXAUD001` and anchor `LXANC001`. Entries use `LXAE` or committed
marker `LXAC`. Verification checks the linked hashes and anchor; an arbitrary
truncated log is not evidence of a complete audit history. Use the codec and
verification routines instead of concatenating serialized records by hand.

## Audit record

[`audit/record.rs`](../services/agent/crates/layerx-agentd/src/audit/record.rs)
writes `LXAR` version **3**, retaining a reader for version **2**. Entries bind
the event class, actor/session context, policy version, decision, reason and
reconstruction evidence. Version 3 includes transaction hash, nonce and rung
coordinates. The outer log and the inner record have different magic/version
contracts; updating one does not imply that the other changed.

## Budget proof

The current protected-owner export schema is **`layerx.human.budget-proof.v2`**,
decoded by [budget_proof.rs](../services/agent/crates/layerx-agentd/src/budget/budget_proof.rs).
It is closed JSON, at most 1,048,576 bytes, with `schema`, `tenant`, `principal`,
`protocol_version`, `network_id`, `maximum_age_seconds`, `head`, `budget_state`,
`accounts`, `identity_key`, `proofs` and `finality`. `head` binds committed height
and block hash. Amounts such as remaining budget are decimal strings.
The verifier checks configured tenant/principal/owner, network, age, committed
evidence and the real budget state. A caller-provided remaining amount or
verification rank alone cannot authorize spending.

## Human peer frame

A big-endian u32 byte length precedes the bounded message. Ordinary payloads
begin with `LXHAGT01`, then a u8 selector and its exact payload. `LXHAGT02` wraps
a 36-byte trace (`trc_` plus 32 lowercase hex characters) and a length-delimited
ordinary frame. The decoder rejects trailing data. Authentication comes from
the Unix peer UID, not from these bytes. See the
[selector table](agent-human-peer.md#selectors).

## Node JSON-RPC

Node writes submit exact signed EIP-1559 bytes with `eth_sendRawTransaction`.
Read precompile views with `eth_call`, and native state with gRPC queries and
ICS23 proofs pinned to a committed block height. Offline evidence binds chain
ID 125, the committed header and the validator set at that height. See the
[JSON-RPC client](agent-rust-api.md#client) and the current
[operation catalogue](agent-api.md#operation-catalogue). The local
[Human peer frame](#human-peer-frame) has its own codec and authentication.

## MCP binding document

[`binding.rs`](../services/agent/crates/layerx-mcp/src/binding.rs) reads closed
JSON with base keys `mode`, `tenant`, `store`, `audit_root`, `session_id`,
`session_token_file`, `session_generation`, `capability_id`, `core_height`,
`deadline_ms`, `agent` and `limit`. Optional listener/web declarations are
validated separately. `agent` contains `endpoint`, `bearer_file`, `probe_program`
and `readiness`; readiness contains `endpoint`, `gateway_key_file` and
`trust_anchors`. The document is bounded to 65,536 bytes and secret files to
4,096 bytes. Credentials are file references, not public metadata. In particular,
`core_height` is the current key; the old sequence environment names are refused
by the executable. See [MCP configuration](agent-config.md#mcp-binding).

## MCP invocation

[`session_control.rs`](../services/agent/crates/layerx-agentd/src/session_control.rs)
stores `layerx.mcp.invocation.v1` records with tenant, actor, session, generation,
idempotency key, typed request digest, core sequence, phase, response and events;
tool name, native preparation and reconciled receipt associations can also be
retained. Recovery preserves the original invocation and its evidence. Reusing a
key for another request does not create a new invocation.

## Native program policy

[`policy/native_program.rs`](../services/agent/crates/layerx-agentd/src/policy/native_program.rs)
loads `layerx.native-program-policy.v1`: bounded closed JSON, at most 1 MiB,
1,024 rules and 1,000,000 evaluation steps. Rules use explicit Permit/Deny
effects and native Programs activities. Value-source rules distinguish principal
funds from a derived program account, retaining the owning program, seed and
source account. A spend rule retains asset, destination and maximum amount;
an exit route is a separate scope. Evaluation uses genuine disclosed bytes and
effects, rather than a caller's description of a write.

## Native program policy source

The protected wrapper is `layerx.native-program-policy-source.v1`, with exactly
`version`, `tenant` and `policy`. See [source loading](agent-config.md#native-program-policy-source).

## Native effect policy

The same module owns `layerx.native-effect-policy.v1` for supported non-Programs
effects. It is distinct from the Programs policy; supported effect derivation and
explicit rule coverage remain necessary. A local permit cannot supply protocol
signing authority or bypass owner consent.

## Native effect policy source

The protected wrapper is `layerx.native-effect-policy-source.v1`, with exactly
`version`, `tenant` and `policy`. See [source loading](agent-config.md#native-effect-policy-source).

## Program budget denominations

The protected JSON document has numeric `version: 1`, `tenant` and `limits`,
each row containing lowercase hex32 `stable_id`, hex32 `asset` and `source`
(hex32 or explicit `"*"`). The loader records SHA-256 of the exact source bytes
and validates the bindings. See [denomination configuration](agent-config.md#program-budget-denominations).

## Receipt metadata

[`receipt.rs`](../services/agent/crates/layerx-agentd/src/receipt.rs) owns `LXRM`
legacy receipt metadata and transaction receipt metadata. Transaction metadata
retains transaction hash, block height, transaction index, status, gas used,
256-byte logs bloom, 65-byte receipt signature and rung. Stored kernel metadata
also retains exact result classification and its indexes. Read these through
their typed codecs; a legacy level is not automatically a current write rung.

## Receipt evidence

The same module writes transaction evidence as `LXRE` version **2**, embedding
`LXRM` version **2** transaction metadata and the canonical receipt bytes. The
legacy kernel-evidence codec still reads/writes `LXRE` version **1** through
its separate index path; the current transaction-evidence reader refuses that
version instead of upgrading its evidence claim.
Receipt storage verifies evidence before exposing it; absent raw evidence in an
older store is held for recovery rather than treated as verified. Report
`receipt.evidence_version`, `receipt.not_receipted`, `receipt.verification`,
`receipt.store`, `receipt.corrupt` or `receipt.missing` as applicable.

## Offline export

[`export::produce`](../services/agent/crates/layerx-agentd/src/export.rs) builds
a bounded evidence bundle (at most 1,048,576 bytes) from genuine owner facts and
independent export trust. The
[export codec](../services/agent/crates/layerx-proof/src/export_codec.rs) uses
version 1 records: `LXRF` receipt, `LXHD` header, `LXIP` inclusion and `LXAS`
account-state records. Fact selection allows at most 16 references, each at most
135 bytes. The account-state record magic is context-specific and does not mean
the bundle is a daemon store. Offline verification must preserve the requested
facts, canonical evidence and trust anchors.

## Registry source config

The protected JSON configuration is version 1 with exactly `version`, `endpoint`,
`server_ca_der`, `client_certificate_pem`, `client_private_key_pem`,
`request_token_file`, `deadline_ms` and `maximum_response_bytes`. See
[registry source](agent-config.md#registry-source) for bounds and endpoint rules.

## Unknown resolution state

[`outbox/unknown.rs`](../services/agent/crates/layerx-agentd/src/outbox/unknown.rs)
stores **56 bytes**: `LXU2`, transaction hash32, first-observed time u64be,
attempt count u32be and next-attempt time u64be. Times are milliseconds. The
tenant-scoped configuration key includes the submission ID. `LXUR` is retired
and refused as `unknown.state_version`; corrupt/current-hash mismatches are
`unknown.state_corrupt`. Receipt lookup, nonce observations and exact-byte
resend remain distinct observations. Backoff starts at 1 second and caps at
60 seconds, including deterministic jitter. A missing receipt or unavailable
lookup does not prove execution failure.
