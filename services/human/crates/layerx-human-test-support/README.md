# Human service test support

Development helpers shared by the [Human service](../layerx-human-service/README.md)
integration suites. This is a library, not a production listener. It provides
principal and tenant store setup, retention helpers, a deterministic `StepClock`,
signed receipt/state evidence builders, shared receive vectors and an HTTP
JSON-RPC fixture for durable admission with Submitted-only receipt replies and
journal recovery. Callers keep their independent Cargo integration target names.

## HTTP admission and readiness

[`evidence_node::EvidenceNode`](src/evidence_node.rs) serves a bounded number of
HTTP JSON-RPC requests over a `TcpListener`, through the boundary used by
`layerx-agentd::node_rpc::NodeRpc`. Supported methods are `eth_chainId`,
`px_getReceiptKeys`, `eth_sendRawTransaction` and `pax_sequencerReceipt`.
`serve_readiness_node` and `connect_readiness_node` in [lib.rs](src/lib.rs) start
this same producer for readiness requests and provide its client configuration.

Admission decodes the signed EIP-1559 transaction, recovers its sender, checks
chain id 125 and requires the sender to have been registered with
`register_sender`. It hashes and durably records the transaction before returning
the hash. Repeating the same signed transaction does not append a duplicate.
Malformed transactions, wrong networks, invalid signatures and unregistered
senders are refused through JSON-RPC errors. `served()` records method names for
callers to inspect.

`pax_sequencerReceipt` returns `rung: "submitted"`, the transaction hash and nonce
for an admitted transaction, or null for an unknown hash. It does not manufacture
an execution receipt or consensus commitment. `px_getReceiptKeys` is a fixture
method here; it does not establish that public node endpoints expose hosted
`px_*` methods.

## Journal recovery and evidence limits

`AdmissionJournal` writes `admission.journal` under a private directory. The
superblock binds its format and configured fixture network id; each record
contains transaction hash, raw signed bytes and a SHA-256 record digest. Writes
are synced before admission succeeds. Reopen replays the valid prefix, truncates
a partial or invalid tail, syncs the recovered file and restores the admitted
hash set. `records()`, `contains()` and `recover()` let tests inspect that state.

This journal demonstrates durable admission and retry behavior. Its configured
fixture network id is distinct from its EVM chain-id check; constants such as
`EVIDENCE_NETWORK_ID = 42` are test data, not mainnet configuration. Local signing
keys, evidence builders and `StepClock` are test utilities, not production trust
or clock authorities. Tests requiring `Committed` evidence must use actual chain
headers, commits and inclusion proofs through the production verifier. A stored
admission record or a synthetically signed receipt is insufficient.

The separate real-node proof is
[`proof_human_test_support_real_committed_receipt_recovers_after_human_restart`](tests/proof_human_test_support.rs),
using [the committed-evidence helper](tests/support/committed_evidence.rs) to pass
a recorded `paxd` receipt through `NodeRpc`, verify its chain headers, consensus
commit and inclusion proofs, and reopen the human evidence store. Its committed
recording is independent of `EvidenceNode`; this HTTP admission fixture cannot
produce `Committed` evidence. The [crate-owned recording](tests/fixtures/committed/README.md)
and its derived vector are checked together by source SHA-256 and receipt
identity, and the proof runs offline against those bytes.

See the [security provider](../layerx-human-security-provider/README.md) for the
recovery-evidence decoder and the [workspace guide](../../README.md) for Cargo
and runtime-clock development commands.
