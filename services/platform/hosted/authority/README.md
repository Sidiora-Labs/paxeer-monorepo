# Receipt authority

`layerx-platform-authority` builds the `layerx-receipt-authority` binary. It
serves verified authorised-batch facts over TLS. Receipt bytes come from the
node's `pax_sequencerReceipt` JSON-RPC method, identified by transaction hash;
receipt signatures and signed batch evidence are checked against pinned keys
and their authorised height ranges. An independent receipt-authority replica
supplies inclusion evidence. The gateway, registry and webhooks services call
it with their own bearer tokens. An optional, separately
configured group of `/v1/agent/*` routes answers the human agent contract used
by `RemoteHumanAuthority` in
[`services/agent/crates/layerx-agentd/src/human_runtime.rs`](../../../../services/agent/crates/layerx-agentd/src/human_runtime.rs).

Paxeer is one BFT network with EVM chain ID **125** (`0x7d`). Its kernel is part
of `paxd` and proposes blocks. Block commit is finality; this verifier does not
provide a second settlement domain. `submitted` is an acknowledgement,
`receipted` is signed execution evidence, and `committed` carries block
inclusion. Tenant domains are database namespaces within the network. See the
[gateway](../gateway/README.md) for public node coordinates and the
[reference applications](../../examples/README.md) for receipt consumers.

| Source | Responsibility |
| --- | --- |
| [`src/main.rs`](src/main.rs) | TLS listener, routes, startup environment and dependency readiness |
| [`src/chain_receipt.rs`](src/chain_receipt.rs) | `pax_sequencerReceipt` HTTP transport, protected receipt-key list, signature and height-range checks |
| [`src/trust.rs`](src/trust.rs) | Pinned key history and independent signed batch-header verification |
| [`src/committed.rs`](src/committed.rs) | Committed receipt landing, Comet/EVM header binding and native-store ICS23 membership |
| [`../registry/src/committed_head_route.rs`](../registry/src/committed_head_route.rs) | Authenticated committed-head relay mounted by the authority |
| [`src/human.rs`](src/human.rs) | Human agent routes, principal policy and retained evidence |
| [`src/human/dynamic.rs`](src/human/dynamic.rs) | Subject-scoped requests and optional identity binding |
| [`src/human/budget_state.rs`](src/human/budget_state.rs) | Budget state and proof export |
| [`src/protected.rs`](src/protected.rs) | Protected file reads |

## Build and test

The crate is a member of the [`platform`](../../Cargo.toml) Cargo workspace.

```sh
cargo build --locked --manifest-path services/platform/hosted/authority/Cargo.toml --tests
cargo test --locked --manifest-path services/platform/hosted/authority/Cargo.toml --test repair_22_33_1473
```

The focused [service configuration, receipt transport and protected key tests](tests/repair_22_33_1473.rs)
run production receipt lookup, protected key loading and TLS routing in-process
with committed signed receipts and loopback peers. They require no environment variable, credential,
live endpoint, external binary, subprocess toolchain, native artifact or device.

Other integration targets live in [`tests/`](tests):
[`chain_receipts.rs`](tests/chain_receipts.rs) covers node receipt transport;
[`human_tls.rs`](tests/human_tls.rs) drives the real `RemoteHumanAuthority`
client against the binary with a private CA;
[`budget_proof_exports.rs`](tests/budget_proof_exports.rs) covers budget exports;
[`maintenance_identity.rs`](tests/maintenance_identity.rs) covers identity
evidence. The [receipt key rotation, retention and replay tests](tests/repair_22_31_207.rs)
drive native execution and reopened committed state, while the
[mixed receipt height, native proof and budget TLS tests](tests/repair_22_31_208.rs)
cover mixed receipt sequences and block heights, retained-state reopening,
native proof refusals and budget
export across TLS. Recorded inputs live in [`tests/fixtures`](tests/fixtures).

Human routes require configured native identity, capability and key-policy
evidence, retained-state recovery and budget proof export. `/readyz` reports
dependency readiness separately from those per-request proof checks.

## Service routes and configuration

All routes are GET. `/livez` and `/readyz` are unauthenticated; `/readyz`
reports `ready`, `network_id`, `protocol_network_id` and `wire_version`. The
bearer-protected routes are:

| Route | Purpose |
| --- | --- |
| `/v1/authorized-batches/by-activity/{activity_id}` | Verified authorised-batch facts for a transaction |
| `/v1/authorized-batches/wait-by-activity/{activity_id}` | Same, polling receipt and replica evidence within a bounded deadline |
| `/internal/v1/activities/{activity_id}/authority` | Internal authority lookup |
| `/v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}` | Relayed to the replica unchanged |
| `/v1/protocol/account-state/head[?height={height}]` | Independent committed-head relay; optional positive canonical decimal height |

Incoming requests are limited to 16 KiB. The service takes no arguments and
reads its configuration from the environment; `--help` lists the base service
settings below. Human settings are listed separately in this README.

| Variable | Meaning |
| --- | --- |
| `LAYERX_AUTHORITY_LISTEN` | Optional socket address; defaults to `0.0.0.0:9445` |
| `LAYERX_AUTHORITY_TLS_CERT_DER`, `LAYERX_AUTHORITY_TLS_KEY_DER` | Server certificate and PKCS#8 private key, DER |
| `LAYERX_AUTHORITY_CLIENT_CA_DER` | Optional; a presented client certificate must chain to it |
| `LAYERX_AUTHORITY_TOKEN_FILES` | Colon-separated files, one service bearer token each |
| `LAYERX_AUTHORITY_REPLICA_URL`, `LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE`, `LAYERX_AUTHORITY_REPLICA_ID` | Required loopback HTTP receipt-authority replica at its root URL, bearer-token file and nonzero 64-hex identity |
| `LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE` | Optional protected file containing 32..4096 printable bytes without whitespace; bearer must differ from all general and replica tokens and may read only the receipt-authority relay route |
| `LAYERX_AUTHORITY_CHAIN_RPC_URL` | Required loopback HTTP node endpoint serving receipts and committed-state evidence, normally `http://127.0.0.1:PORT/evm`; native proofs use `/comet` on the same origin |
| `LAYERX_AUTHORITY_RECEIPT_KEYS_FILE` | Required protected JSON array of pinned receipt keys and height ranges |
| `LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID` | Nonzero 32-bit protocol network ID checked in signed evidence |
| `LAYERX_AUTHORITY_NETWORK_ID`, `LAYERX_AUTHORITY_WIRE_VERSION` | Required deployment identifier of 1..64 ASCII letters, digits, `-`, `_` or `.`; optional wire version defaults to the built protocol version (`3`) and must parse to that version |
| `LAYERX_AUTHORITY_SEQUENCER_ID`, `LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY` | Compatibility pins still required by startup parsing: each must be nonzero, 64 hexadecimal characters without `0x` |
| `LAYERX_AUTHORITY_FIRST_BATCH`, `LAYERX_AUTHORITY_LAST_BATCH` | Required compatibility inclusive u64 range; first must be positive and last must be at least first |

The receipt-key file is a nonempty array with `keyId`, `publicKey`,
`firstHeight` and `lastHeight` per entry (snake-case aliases are accepted).
Keys and IDs encode 32 bytes as standard padded base64 (44 characters);
heights are u64 JSON integers. IDs must be unique and nonzero, keys nonzero,
and ranges ordered. Active trust verification also requires the key ID to
match SHA-256 of its public key. This file supplies verification keys; the
compatibility environment pins do not replace it. The list is loaded at startup;
editing it requires a restart to change the active pins. The protected file is
limited to 256 KiB and rejects unknown fields. Protect it as described below.

Receipt HTTP requests preserve the configured path, including `/evm`; the
combined committed-state reader supports the root or `/evm` endpoint and derives
the sibling `/comet` path. Use `http://` with an explicit nonzero port and host
`127.0.0.1`, `localhost` or `[::1]`. This is a local node trust boundary.

The route's `activity_id` is the transaction's 32-byte hexadecimal identifier
without `0x`; the upstream JSON-RPC call adds the prefix. Null or submitted-only
upstream evidence yields 404 `unknown_activity`. Key mismatch yields 502
`sequencer_key_mismatch`; unavailable or malformed receipt-source evidence yields
503 `receipt_source_unavailable`; rejected replica evidence yields 502
`evidence_refused`. A valid signed receipt alone does not establish commitment.

[`src/chain_receipt.rs`](src/chain_receipt.rs) owns receipt transport and key
selection. [`src/trust.rs`](src/trust.rs) verifies receipt-key history and batch
headers. Human state evidence additionally binds receipts to committed blocks
and validates native-store membership through [`src/committed.rs`](src/committed.rs):
it checks chain ID 125, the node's Comet commit and EVM block projection, and
ICS23 membership against the committed app root. A header at H authenticates
state at H-1. The configured loopback node is the committed-header trust boundary;
this is not an independently bootstrapped light client. Receipt sourcing uses the
node JSON-RPC endpoint.

`/readyz` returns 200 only when both chain and replica probes are ready, otherwise
503 with retry guidance. It does not assert that all optional human-state proofs
or downstream services are available.

## Human agent contract

### Client transport and encoding

`RemoteHumanAuthority` sends every request as GET over HTTPS with
`Authorization: Bearer <token>` and no body. Every route has required `tenant`
and `principal` query parameters. The constructor requires an `https://`
endpoint, a token of at least 32 bytes, a nonzero deadline, a response limit in
`1..=1_048_576` bytes and a private CA certificate in DER; it removes trailing
slashes from the endpoint.

In the tables, `H32` means a JSON string of 64 hexadecimal characters encoding
32 bytes, with no `0x` prefix. `HEX` means a nonempty, even-length hex string of
at most 2,097,152 characters, also bounded by the response limit. `U64`, `U16`,
`U8` and `U32` are unsigned JSON integers. `DEC128` is a JSON string holding an
unsigned base-10 `u128`.

The client treats every non-success HTTP status, including 503, as
`HumanOperationError::Refused`; it does not parse the error body or
`Retry-After`. Transport failure or failure reading the body produces
`HumanOperationError::Unavailable`. Invalid JSON also produces `Refused`.

### Routes

All paths have the prefix `/v1/agent/`.

| Route | Additional query parameters | Response fields |
| --- | --- | --- |
| `registry` | None | `modules`: array of `{module_id: U16, activity_types: [U32]}`; `revision`: H32 |
| `authorized-batch` | `activity_id`: H32 | `batch_id`, `asset`, `previous_state_root`, `resulting_state_root`, `sequencer_public_key`: H32 |
| `balance-context` | None | `account_id`, `asset_id`, `sequencer_id`, `sequencer_public_key`: H32; `currency`, `observed_at`: strings; `age_seconds`, `maximum_age_seconds`, `first_batch_number`, `last_batch_number`: U64 |
| `identity` | `did` | `authorities`: array of `{kind, id: H32}`; `canonical_core_bytes`: HEX; `head_sequence`, `revocation_sequence`: U64; `verification_level`: string; `frozen`: boolean |
| `core-clock` | None | `lower_unix_ms`, `lower_sequence`, `upper_unix_ms`, `upper_sequence`, `observed_head_sequence`: U64; `canonical_attestation`: HEX |
| `capability-scope` | `did`; `authority`, `action_key`, `capability_id`: H32 | `activity_types`: [U16]; `counterparties`, `assets`: [H32]; `amount_ceiling`: DEC128; `expiry_sequence`, `observed_sequence`: U64; `enforceable_dimensions`: [string]; `verification`: U8; `evidence_digest`: H32 |
| `budget-state` | `budget_id`: H32 | `revocation_sequence`, `observed_head_sequence`, `age_sequences`, `maximum_age_sequences`: U64; `verification`: U8; `evidence_digest`, `receipt_digest`, `checkpoint_digest`, `asset`: H32; `remaining`: DEC128 |
| `budget-proof` | `budget_id`: H32 | Budget state proof export |
| `key-policy` | `did`; `recovery`: `true` or `false` | `policy_revision`, `required_delay_seconds`, `maximum_delay_seconds`, `effective_sequence`, `observed_head_sequence`, `age_sequences`, `maximum_age_sequences`: U64; `verification`: U8; `evidence_digest`, `checkpoint_digest`: H32 |

Registry module IDs are the closed set in
[`services/agent/crates/layerx-types/src/payload.rs`](../../../../services/agent/crates/layerx-types/src/payload.rs)
(`Asset` = 1 through `Web` = 11); each module lists at most 64 activity
types (`MAX_MODULE_ACTIVITY_TYPES` in
[`limits.rs`](../../../../services/agent/crates/layerx-types/src/limits.rs)).

Capability installation in the client checks `evidence_digest` against SHA-256
of six length-prefixed parts (four-byte big-endian length each): ASCII
`layerx-human/agent-create/agent-evidence/v1`, byte `05`, `action_key`,
`capability_id`, `observed_sequence` (8 bytes big-endian) and `verification`
(one byte).

### Subject-scoped requests

When a request carries any of `subject_principal`, `owner_did`,
`owner_account`, `asset_id`, `registration` or `signed_activity`, the server
handles it in subject-scoped mode ([`src/human/dynamic.rs`](src/human/dynamic.rs)).
That mode requires all of `tenant`, `principal`, `subject_principal`,
`owner_did`, `owner_account` and `asset_id`, resolves the subject through the
identity binding service, and adds a `subject-context` route. In this mode
`authorized-batch` also requires `signed_activity`, `budget-proof` also requires
`did`, and `registration` is accepted only on `identity`, `key-policy`,
`capability-scope`, `authorized-batch` and `budget-proof`.

### Server configuration

| Variable | Required configuration |
| --- | --- |
| `LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE` | Protected file holding 32..4096 bytes in `0x21..0x7e`; must differ from every service token |
| `LAYERX_AUTHORITY_HUMAN_AGENT_TENANT`, `LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL` | Nonempty tenant and principal bound to the token |
| `LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE` | Protected principal policy, schema below |
| `LAYERX_AUTHORITY_MODULE_REGISTRY_FILE` | Protected module registry file, also read by the gateway |
| `LAYERX_AUTHORITY_CORE_CLOCK_HORIZON` | Positive u64 sequence horizon |
| `LAYERX_AUTHORITY_STATE_ROOT` | Existing absolute canonical directory owned by the process UID with no group or other permissions, holding retained receipt records |
| `LAYERX_AUTHORITY_IDENTITY_BINDING_SOCKET`, `LAYERX_AUTHORITY_IDENTITY_BINDING_UID`, `LAYERX_AUTHORITY_IDENTITY_BINDING_GID` | Optional as a set; identity binding socket and its expected peer UID and GID, used by subject-scoped requests |

When the token file variable is absent and none of the others is set, the
service runs without the human routes and they return 503
`human_authority_unconfigured`. Setting any of the others without the token file
prevents startup. The bearer is compared in constant time; a wrong or missing
bearer gets 401 `identity_required`, a mismatched `tenant` or `principal` gets
403 `principal_mismatch`, an unknown route gets 404, and unexpected or missing
query keys get 400 `invalid_query`.

Protected files are read through [`src/protected.rs`](src/protected.rs): the
path must be absolute and canonical, and the file must be a regular file owned
by the process UID with no group or other permissions. The policy is parsed and
pinned by SHA-256 at startup; every authenticated request re-reads it and
compares the digest, and a missing, invalid or changed policy returns 503
`policy_unavailable`, so changing the policy requires a restart. Verified
activity lookups are retained as JSON records under the state root.

Server-side 503 refusal codes include
`budget_revocation_and_checkpoint_evidence_unavailable` (missing authenticated
budget evidence), `identity_state_proof_unavailable`,
`capability_state_proof_unavailable`,
`key_policy_checkpoint_evidence_unavailable`, `budget_state_proof_unavailable`
and `budget_proof_export_unavailable`.

Budget state and export use native `getBudget` and `budgetRemaining` calls at
the pay precompile `0x000000000000000000000000000000000000101b`, bound to a
stable committed block and authenticated account state. The export schema is
`layerx.human.budget-proof.v2` and includes tenant/principal bindings, committed
height/hash, calls, account join and finality facts. The identity binding socket,
when enabled, resolves subject ownership; it is not a receipt-source socket.

### Principal policy schema

All fields are required and unknown fields are rejected. H32 fields are 64-digit
hexadecimal identifiers. The file is an object with `principals`, an array of:

```text
{
  tenant: string,
  principal: string,
  account_id: H32,
  asset_id: H32,
  activities: [H32],
  budgets: [H32],
  maximum_age_seconds: U64,
  maximum_age_sequences: U64,
  identities: [{
    did: string,
    authorities: [{kind: primary_key|session_key|capability_grant, id: H32}],
    revocation_sequence: U64,
    frozen: boolean,
    evidence: {activity_id: H32, receipt_digest: H32},
    capabilities: [{
      authority: H32,
      action_key: H32,
      capability_id: H32,
      activity_types: [U16],
      counterparties: [H32],
      assets: [H32],
      amount_ceiling: unsigned decimal string,
      expiry_sequence: U64,
      enforceable_dimensions: [activity_type|counterparty|asset|amount|rate|purpose|expiry],
      evidence: {activity_id: H32, receipt_digest: H32}
    }],
    rotation: KeyPolicy,
    recovery: KeyPolicy
  }]
}
KeyPolicy = {
  policy_revision: U64,
  required_delay_seconds: U64,
  maximum_delay_seconds: U64,
  effective_sequence: U64,
  evidence: {activity_id: H32, receipt_digest: H32}
}
```

`authorized-batch` refuses an activity that is not in the principal's
`activities` list (403 `activity_not_bound`), and the budget routes refuse a
budget that is not in `budgets` (404 `budget_not_bound`).
