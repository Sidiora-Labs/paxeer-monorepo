# Agent Rust API

The agent workspace is
[`services/agent/Cargo.toml`](../services/agent/Cargo.toml). Its crate names retain
the `layerx-` prefix; they implement agents for the merged Paxeer network.
Mainnet-beta uses EVM chain ID 125, and committed blocks establish receipt
finality on the one BFT network. Historical Rust identifiers are not instructions
to provision another execution or finality service.

Read the [API contract](agent-api.md), [configuration](agent-config.md),
[formats](agent-formats.md) and [Human peer](agent-human-peer.md) alongside this
module map. Public node JSON-RPC connects to node HTTPS roots; hosted `px_`
methods follow the gateway [OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

## Types

[`layerx-types`](../services/agent/crates/layerx-types/src/lib.rs) owns identifiers,
amounts, native activities, program lifecycle values, protocol results, clocks and
verification types. Consensus amounts use u128, sequence/time quantities use
their declared u64 types. `ResultCode` preserves numeric codes including unknown
ones; inspect `known()`, `domain()` and `retriability()` rather than discarding
the original number.

[`Rung`](../services/agent/crates/layerx-types/src/rung.rs) orders `Submitted`,
`Receipted`, `Committed`. The older `VerificationLevel` has additional internal
labels; `rung()` is an explicit mapping, and settlement anchoring has no rung.
Constructing a requested level does not construct verified evidence.

## Wire

[`layerx-wire`](../services/agent/crates/layerx-wire/src/lib.rs) owns canonical
encoding/decoding, hashing domains and bounded signed-message codecs. Modules
include `account`, `activity`, `batch_maintenance`, `handover`, `decode`,
`encode`, `hash`, `limits` and `sign`. Preserve exact byte widths, supported
protocol versions and native activity identifiers. Reject truncation, trailing
data and out-of-bound messages through the real decoder.

## Crypto

[`layerx-crypto`](../services/agent/crates/layerx-crypto/src/lib.rs) provides
`SignatureMessage`, `VerifyError`, `ed25519`, `secp256k1`, `evm_transaction`,
`purpose` and `settlement_recipient`. `SignatureMessage::new` validates the
supported protocol and nonzero network scope; `digest()` hashes the selected
domain tag followed by canonical bytes. Key/signature verification failures
retain the exact protocol classification via `VerifyError::result_code`.

The default `custody` feature additionally exposes account derivation, authority
grants, disclosure binding, keystore/local/remote signers, onboarding, payments,
redaction, send and session modules. Disabling default features removes those
custody modules. Secrets belong behind the signer boundary; debug output for
`SignatureMessage` exposes metadata and byte length, not canonical contents.

## Proof

[`layerx-proof`](../services/agent/crates/layerx-proof/src/lib.rs) owns offline
verification in `receipt`, `inclusion`, `sequencer_logs`, `state`,
`state_witness`, `state_range`, `program`, `signed_authority`, `merkle`,
`availability`, `evidence`, `level`, `export` and `export_codec`.
`level::achieved` and `achieved_rung` read the token inseparably carried by
verified evidence. The achieved-token constructor is private; caller-selected
rank integers and RPC-published authority do not establish trust.

Use the exact producer's bytes, pinned receipt keys and protected commit trust.
[Exports](agent-formats.md#offline-export) carry chain identity, committed
headers and validator sets for independent verification. The removed checkpoint
and settlement proof modules are not part of the current crate API.

## Client

[`layerx-client`](../services/agent/crates/layerx-client/src/lib.rs) exposes
`sequencer`, `rpc`, `px`, `account_binding`, `budget`, `chains`, `fee`, `frame`,
`grants`, `paxeer_binding`, `storage_witness`, `subscription`, `ai_market` and
Linux `runtime_clock`. `SequencerEndpoint` is the retained Rust name for the
node HTTP JSON-RPC client. It does not identify a separately deployed service.
`rpc` contains typed receipt/commit, submission, account/module, history,
evidence, stream, payment and withdrawal adapters. `PxEndpoint` serves the hosted
method contract. Keep method refusal, transport failure and verification failure
distinct, and check configured chain ID against the connected node.

## Agent API

[`layerx-agent-api`](../services/agent/crates/layerx-agent-api/src/lib.rs) exposes
the shared contract namespaces `identity`, `capability`, `budget`, `prepare`,
`submit`, `track`, `read`, `policy`, `proof`, `availability`, `export`,
`subscription`, `error`, `idempotency` and `verify`, plus generated `Operation`
and schema/version types. The current contract is 2.0; envelope version 1 is
independent. Public reexports come from the authored contract files and
generated schema projection. Prefer these typed values over parallel structs.

## Agentd

[`layerx-agentd`](../services/agent/crates/layerx-agentd/src/lib.rs) owns the
durable runtime: tenant-scoped store, session control, capabilities, budget
reservations, policy/approval, preparation lifecycle, outbox recovery, verified
reads, receipt storage, audit, subscriptions and authenticated transports.
`SessionControl` coordinates current-generation authorization with durable
effects. `OperationPermit::boundary` is rechecked at effect/release boundaries;
a previously authenticated session is not perpetual permission.

`UnifiedAgentOwner` and `ProductionHumanOperations` bind the
[Human peer](agent-human-peer.md) to real runtime operations. The binary has
full and human-owner modes; see [its actual environment](agent-config.md#agentd-env).
`Store` keys always include `TenantId`. Keep Unknown, pending, failed and
executed states distinct during recovery; absent evidence is not an executed
result.

## Agentd host

[`layerx-agentd-host`](../services/agent/crates/layerx-agentd-host/Cargo.toml)
integrates the executable's host dependencies. Its
[`ProductionWebPayer`](../services/agent/crates/layerx-agentd-host/src/payer.rs)
uses the owner, KMS attestor, native precompile calls and node receipts for web
payments. The payer fixes mainnet-beta chain ID 125 and a 30-second receipt
timeout. A payment boundary refusal, attestor refusal and node refusal remain
distinct typed inputs to its error mapping.

## SDK

[`layerx-sdk`](../services/agent/crates/layerx-sdk/src/lib.rs) exposes typed
`Client`, `Call<T>`, `Deployment`, `SdkError`, `SubmissionOutcome`,
`signer::ExternalSigner`, native capabilities/effects, purpose, registration,
wallet and RPC/program adapters. `Client::daemon` configures the agent endpoint
and contract major; `Client::direct_node` configures a node with explicit trust
anchors. A `Call<T>` describes an operation/request; constructing it does not
perform network I/O. Use the concrete RPC adapters for transport.

`Client::sign_external` passes the exact signing preimage and disclosure to an
external signer, without importing a key. `accept_verified_read` refuses
Submitted-only or below-requested reads. `submission_outcome` preserves Unknown
and rejects an executed result without evidence or a verified rung. Approval
holds are agent-runtime functionality; direct-node access does not emulate them.

## MCP

[`layerx-mcp`](../services/agent/crates/layerx-mcp/src/lib.rs) owns the
catalogue, request validation, binding, stdio/Unix listener and tool boundary.
Read-only and full sessions have explicit deployment modes. A model's tool
arguments never supply tenant/session authority: the authenticated invocation
resolves the named original operation and its permit. Use the daemon-published
[binding](agent-formats.md#mcp-binding-document) and protected credential files.

## Identity binding

[`layerx-identity-binding`](../services/agent/crates/layerx-identity-binding/src/lib.rs)
provides `Config`, `Client`, `Binding` and `subject_namespace`. The client uses a
bounded authenticated Unix connection, configured peer UID/GID and deadline.
The derived namespace hashes the transport tenant and principal with their
lengths under the `layerx-human/subject-tenant/v1` domain. Retain the returned
transport binding and agent tenant; do not accept a request's arbitrary tenant
as the storage authority.

## Commit verification

Acquire node receipts and commits through [client](#client), verify the CometBFT
commit and header at the selected height, and verify per-key native state proofs
against the committed app hash. The retained node commit/header verifier is
[custodyproof.Verify](../custodyproof/verify.go); [proof](#proof) owns the current
Rust evidence API, and [agentd](#agentd) owns runtime trust and receipt storage.
