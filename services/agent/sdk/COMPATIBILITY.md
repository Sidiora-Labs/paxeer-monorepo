# LayerX SDK compatibility

Version declarations come from
[`schema/agent-api/v1.kvx`](../schema/agent-api/v1.kvx). Despite its filename,
the current schema is contract **2.0**. The executable daemon matrix is
documented in [`docs/agent/COMPATIBILITY.md`](../../../docs/agent/COMPATIBILITY.md).

| SDK version | Contract version | Node interface version | Daemon version | Support |
| --- | --- | --- | --- | --- |
| `0.1.x` | `2.x` | `1.x` | `0.1.x` | supported |

The node column is the client-version compatibility major, not another
network or a separate protocol handshake. Node JSON-RPC uses the public node
names at their root path, with EVM chain ID 125 (`0x7d`); see the
[workspace README](../README.md#network-boundary). Hosted gateway operations
use their own authenticated transport contract.

Within a contract major, schema changes must be additive. A removed or changed
declaration requires a major-version increment. Rust, TypeScript and Python
SDKs refuse unsupported contract majors. Approval operations and lifecycle
events are included in major 2. The schema's `1.1` approval introduction is
historical compatibility information for major 1, not a requirement to
negotiate major 1 with the current SDKs.

## Enforcement guarantees

All three SDKs preserve these distinctions:

| Surface | Enforcement | SDK promise |
| --- | --- | --- |
| Canonical transactions, receipts, protocol budgets and protocol grants | protocol-enforced | Preserve exact signed bytes, protocol result codes and network-produced evidence. |
| Verification rung and freshness | evidence-enforced | Expose only the rung justified by verified evidence, with the observed head and value height. |
| Local policy, rate limits, daemon capabilities and `DaemonLimit` spending controls | daemon-enforced | Restrictions bind while the agent owner is in the path; they do not create protocol authority. |
| Approval holds and decisions | daemon-enforced | Preserve disclosures, hold state, decisions and lifecycle events; approval does not grant protocol authority. |
| Caller idempotency | protocol-and-daemon coordinated | Preserve the caller key across retries and retain `Unknown` until evidence resolves the outcome. |

`submitted` means only the transaction hash is known. `receipted` requires a
receipt verified against an authorized receipt key. `committed` requires a
verified CometBFT commit at that receipt's height. The network's kernel
proposes blocks, and finality is the block commit. Response metadata or a
successful transport exchange alone cannot establish a rung.

The generated [TypeScript guarantees](typescript/src/generated/guarantees.md)
and [Python guarantees](python/layerx_sdk/generated/guarantees.md) contain the
schema-derived restriction rows. The Rust SDK exposes its guarantees through
`layerx_sdk::GUARANTEES`.

## Offline receipt verification

The current `export.offline` value contains `facts`, `bundles` and `height`.
It carries portable-v2 bundles with format `layerx-receipt-proof-v2` and
`verificationLevel` set to `committed`.
Each bundle contains the exact `rawTransaction`, `signedReceipt`,
`receiptDigest` and `receiptKey`, plus `blockHeader`, `receiptInclusion`,
`commit` and `validatorSet` for committed evidence. A state fact additionally
carries `accountProof`. Exports require committed evidence for every requested
fact at one height; missing evidence, mixed snapshots and partial success are
refused. These shapes are implemented in
[`layerx-agent-api/src/read.rs`](../crates/layerx-agent-api/src/read.rs).

An offline reviewer must independently obtain the trusted `ReceiptKeys` list
and validator-set context. Keys supplied inside an export do not establish
their own authority. Verification checks the receipt signature and its key's
height interval, the raw transaction hash, receipt digest and inclusion,
the consensus block header and commit with strictly more than two thirds
voting power, and any required account proof.

The current portable-v2 implementation is
[`layerx_agentd::export`](../crates/layerx-agentd/src/export.rs):
`deserialize_export` decodes the artifact, `ExportTrustSource::new` constructs
the independently supplied key and validator context, and `verify` checks
the complete artifact and returns its achieved rung. These functions run
locally without contacting a daemon or node. They use the receipt-verification
primitives in `layerx-proof`. Structural decoding by an SDK is not proof
verification.

The `layerx-proof` example named `offline_verify` accepts a receipt hex file
and explicit batch context; it is not a portable-v2 export-file verifier.
The Python `offline_receipt_verification.py` example likewise verifies an
individual receipt with supplied context, not the complete export.

## Release regeneration gate

`make agent-test-sdk-compat` runs contract-schema checks, SDK generator tests
and a regeneration diff, then the documentation checker and its tests.
The generator's prose template is
[`compatibility.md.tpl`](../tools/sdk-gen/templates/compatibility.md.tpl);
schema declarations remain authoritative for versions and contract shapes.
The current template still emits the historical offline-export guidance and
must be reconciled with the portable-v2 contract before a release regeneration.
This release gate is separate from verifying an offline artifact or checking
a live node's capabilities.
