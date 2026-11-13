# Agent CLI contract

Paths are relative to the repository root. Source links identify command implementations.

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`). The
kernel participates in the network and proposes blocks; a receipt is final when
its block is committed. Agent tools consume node JSON-RPC. Public HTTPS node
endpoints use the root path. Node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods have their own
[OpenRPC contract](../../../services/platform/hosted/gateway/openrpc.json);
they must not be assumed to exist on every node. See the
[network brief](../programme/audit/BRIEF.kvx) and [RPC contract](rpc.md).

There is no separate LayerX interface, router or sequencer service.
The retired `layerxd` binary and LNI transport are not prerequisites of these commands.
Sub-chains are tenant domains in the database; keep tenant identifiers on agent operations.
The running `layerx-agentd` described below is an off-chain agent application,
not a second network executor or a source of chain finality.

## agent-boundary-conformance

Source: [argument dispatch](../../../services/agent/tests/boundary/src/main.rs)
and [production cases](../../../services/agent/tests/boundary/src/production.rs).
All rows are current `cli_command` declarations.

| Interface | Invocation | Behavior |
|---|---|---|
| `interface.9.cli-agent-boundary-conformance` | `agent-boundary-conformance NODE REPOSITORY` | `NODE` is a node JSON-RPC URL; `REPOSITORY` must contain `services/agent/tests/boundary/Cargo.toml`. Sets `LAYERX_QUALIFY_NODE_RPC_URL` and runs admission checks. With no arguments, reads the URL from the environment. |
| `interface.9.cli-agent-boundary-conformance-production-required` | `agent-boundary-conformance --production-required` | Exercises reads, signs and persists a self-transfer, submits it and resolves its receipt. |
| `interface.9.cli-agent-boundary-conformance-production-recovery` | `agent-boundary-conformance --production-recovery` | Reloads retained signed bytes and resolves their receipt without creating a fresh transaction. |
| `interface.9.cli-agent-boundary-conformance-production-duplicate` | `agent-boundary-conformance --production-duplicate` | Resubmits identical retained bytes twice and checks that the hash and original receipt stay identical. |
| `interface.9.cli-agent-boundary-conformance-production-peer-refusal` | `agent-boundary-conformance --production-peer-refusal` | Signs with a different chain ID and requires a JSON-RPC refusal; this mode name does not describe LNI peer credentials. |

Production modes require `LAYERX_QUALIFY_NODE_RPC_URL`. Optional
`LAYERX_QUALIFY_NODE_RPC_CA_DER` names a certificate file. Admission additionally
reads `LAYERX_QUALIFY_EVM_PRIVATE_KEY` (32-byte hex, optional `0x` prefix),
`LAYERX_QUALIFY_MAX_PRIORITY_FEE_PER_GAS` and `LAYERX_QUALIFY_MAX_FEE_PER_GAS`
(decimal wei), and `LAYERX_QUALIFY_TRANSACTION_FILE` (new file for exact signed
bytes, persisted before submission). Recovery and duplicate modes read that
file. Peer refusal also needs the signing key and fee caps. These modes can
submit real transactions; they belong on a controlled qualification node with
an appropriately funded account.

The harness refuses all seven retired variables even before dispatch:
`LAYERX_QUALIFY_LNI_SOCKET`, `LAYERX_QUALIFY_SIGNED_ACTIVITY`,
`LAYERX_QUALIFY_LNI_DAEMON_UID`, `LAYERX_QUALIFY_LNI_CLIENT_GID`,
`LAYERX_QUALIFY_MAX_SIGNED_ACTIVITY`, `LAYERX_QUALIFY_LNI_DEADLINE_MS` and
`LAYERX_QUALIFY_UNKNOWN_SIGNED_ACTIVITY`. Remove them rather than supplying LNI
configuration. Receipts are looked up through `pax_sequencerReceipt`, keyed by
the hash of the retained transaction.

`agent-boundary-conformance --check-report MODE` validates a report on stdin,
with a 65,536-byte bound. `MODE` is `default` or one of the production flags
above; the qualification wrapper uses this strict decoder. Success exits zero;
refusals exit nonzero with `node RPC boundary conformance refused:` on stderr.

## agent-isolation-suite

`interface.9.cli-agent-isolation-suite` declares `agent-isolation-suite` with no
required arguments. Its [main function](../../../services/agent/tests/isolation/src/main.rs)
runs `agent_isolation_suite` and prints counts for checked surfaces, MCP cases,
removed tenant values and retained legal/protocol values. A suite error panics.
This is an isolation test tool, not a node launcher.

## agent-qualification-gates

Source: [gate dispatch](../../../services/agent/tests/qualify/src/main.rs),
[boundary wrapper](../../../services/agent/tests/qualify/boundary.rs) and
[fuzz wrapper](../../../services/agent/tests/qualify/fuzz.rs).

| Interface | Current invocation or retirement | Behavior |
|---|---|---|
| `interface.9.cli-agent-qualification-gates-boundary` | `agent-qualification-gates boundary REPOSITORY NODE HARNESS` | Requires an HTTP(S) node URL and a built harness file, audits boundary coverage, runs all four production modes and validates their output with `--check-report`. |
| `interface.9.cli-agent-qualification-gates-fabrication` | `agent-qualification-gates fabrication REPOSITORY` | Runs the fabrication gate. |
| `interface.9.cli-agent-qualification-gates-faults` | `agent-qualification-gates faults REPOSITORY` | Runs fault injection followed by the exactly-once suite. |
| `interface.9.cli-agent-qualification-gates-fuzz` | `agent-qualification-gates fuzz REPOSITORY MINIMIZED_ROOT` | Audits targets, runs fuzzing/minimisation, checks corpus bounds, then runs sanitizer and purity checks. |
| `interface.9.cli-agent-qualification-gates-wire` | Retired: `agent-qualification-gates wire` | Always exits nonzero with `wire gate is retired`; it does not run a differential. |

Gate failures print `qualification gate failed:` and exit nonzero. The boundary
wrapper supplies `LAYERX_QUALIFY_NODE_RPC_URL` to the harness; the other production
settings in [agent-boundary-conformance](#agent-boundary-conformance) are still
required. `NODE` is never a binary to spawn.

Fuzz declarations live in [fuzz/Cargo.toml](../../../services/agent/fuzz/Cargo.toml).
Run `cargo fuzz run TARGET` from `services/agent/fuzz`. These are current targets:

| Interface | Target | Source under `services/agent/fuzz/` |
|---|---|---|
| `interface.9.cli-agent-fuzz-harness-j44` | `harness` | `fuzz_targets/harness.rs` |
| `interface.9.cli-agent-fuzz-primitive-decode-j44` | `primitive_decode` | `fuzz_targets/primitive_decode.rs` |
| `interface.9.cli-agent-fuzz-envelope-decode-j44` | `envelope_decode` | `fuzz_targets/envelope_decode.rs` |
| `interface.9.cli-agent-fuzz-payload-decode-j44` | `payload_decode` | `fuzz_targets/payload_decode.rs` |
| `interface.9.cli-agent-fuzz-receipt-decode-j44` | `receipt_decode` | `fuzz_targets/receipt_decode.rs` |
| `interface.9.cli-agent-fuzz-proof-decode-j44` | `proof_decode` | `fuzz_targets/proof_decode.rs` |
| `interface.9.cli-agent-fuzz-roundtrip-j44` | `roundtrip` | `fuzz_targets/roundtrip.rs` |
| `interface.9.cli-agent-fuzz-contract-request-j44` | `contract_request` | `fuzz_targets/contract_request.rs` |
| `interface.9.cli-agent-fuzz-policy-loader-j44` | `policy_loader` | `fuzz_targets/policy_loader.rs` |
| `interface.9.cli-agent-fuzz-tenant-key-j44` | `tenant_key` | `fuzz_targets/tenant_key.rs` |

Retired registry reference `interface.9.cli-agent-fuzz-lni-frame-j44` names
`lni_frame`; its target and source have been removed. The qualification wrapper's
`FUZZ_TARGETS` contains the nine current targets other than `harness`.

The current encoding context corresponding to
`contract_context.9.agent-fuzz-targets` is the committed corpus under
`services/agent/fuzz/corpus` and
[codec.rs](../../../services/agent/fuzz/fuzz_targets/codec.rs).
`envelope_decode` decodes signed EIP-1559 transactions and asserts byte-for-byte
reassembly for decoded inputs. `roundtrip` selectors 0 and 1 use that same codec;
selectors 2 through 5 cover receipt, Merkle proof, batch-header and checkpoint
encodings. Those encoding types do not imply a second settlement network.
The [input helper](../../../services/agent/fuzz/fuzz_targets/support.rs) accepts
`hex:` seeds. Round-trip assertion failures abort; this context has no gas,
event or pause interface.

## agent-sdk-doc-check

`interface.9.cli-agent-sdk-doc-check` declares
`agent-sdk-doc-check [REPOSITORY]`; the default is the current directory. The
[implementation](../../../services/agent/tools/doc-check/src/main.rs) checks the
SDK compatibility matrix and guarantee documentation, including offline
verification and example coverage. Extra arguments or failed checks exit
nonzero. Success prints
`SDK compatibility, guarantees, offline path and examples are current`.

## agent-sdk-gen

Source: [sdk-gen/src/main.rs](../../../services/agent/tools/sdk-gen/src/main.rs).

| Interface | Current invocation |
|---|---|
| `interface.9.cli-agent-sdk-gen-check` | `agent-sdk-gen [--check [SCHEMA_ROOT] [OUTPUT_ROOT]]` |
| `interface.9.cli-agent-sdk-gen-write` | `agent-sdk-gen --write [SCHEMA_ROOT] [OUTPUT_ROOT]` |

No arguments selects `--check`. To supply paths, pass an explicit mode first.
Defaults are `services/agent/schema/agent-api` and `services/agent/sdk`. Check
mode compares generated output and the programs golden against their canonical
inputs; write mode updates generated SDK files and `golden/programs.kvx` under
the schema root. An error prints `agent-sdk-gen:` and exits nonzero.

## agent-sdk-parity

`interface.9.cli-agent-sdk-parity` declares `agent-sdk-parity NODE REPOSITORY`.
The [implementation](../../../services/agent/tests/parity/src/main.rs) requires an
HTTP(S) node JSON-RPC URL; it refuses a filesystem executable path with
`expected a node JSON-RPC URL`. It also rejects URL credentials and fragments.
It qualifies the node with chain, block and receipt-key reads, then compares
Rust, TypeScript and Python SDK observations for 21 scenarios through a
provisioned daemon's mutual-TLS operation-envelope transport.

Provide `services/agent/tests/parity/provisioned.json` with a `daemon` object
and `cases` matching all 21 declared scenario names. The daemon configuration
supplies `endpoint`, `tenant`, `session_id`, `token_id`, `generation`,
`client_certificate`, `client_key` and `trust_anchors`. Certificate, key and
trust-anchor paths resolve relative to the plan.
Each case supplies an operation, canonical request ID, request and optional
idempotency key. The TypeScript and Python drivers consume that same file with
`--envelope-plan`.

`agent-sdk-parity --check-scenarios PATH` validates the scenario file without
running the parity suite. The checked-in
[scenarios.kvx](../../../services/agent/tests/parity/scenarios.kvx) uses only
`submitted`, `receipted` and `committed` verification rungs. Invalid rungs fail;
errors print `SDK parity failed:` and exit nonzero.

## agent-secret-check

`interface.9.cli-agent-secret-check` declares `agent-secret-check [ROOT]`.
[Source](../../../services/agent/tools/secret-check/src/main.rs) defaults `ROOT`
to the literal relative directory `agent`; from the repository root pass
`services/agent` explicitly. Violations print `path:line: rule` and exit 1;
success prints `agent secret hygiene passed`.

## agent-wire-differential

The wire differential command is unavailable. The [wire qualification mode](#agent-qualification-gates) refuses with `wire gate is retired`.

## dependency-policy

`interface.9.cli-dependency-policy` declares
`sh services/agent/tools/dependency-policy.sh`.
The [script](../../../services/agent/tools/dependency-policy.sh) uses locked
Cargo metadata, rejects its listed boundary/global-initialisation dependencies
and missing external license declarations, and checks unsafe code against
`services/agent/unsafe-allowlist.toml`. It requires `deny.toml` to be readable;
that file check alone does not mean a separate license-policy tool ran.

## generate-operation

`interface.9.cli-generate-operation` declares
`python3 services/agent/tools/generate-operation.py --schema PATH --output PATH [--check]`.
The [generator](../../../services/agent/tools/generate-operation.py) reads
`transport.agent_http.operation.*` declarations, derives operation names and
mutation classification, and checks the existing output before updating it.
`--output` must already be readable; `--check` compares it without writing.
The generated enum is used in
[layerx-agent-api](../../../services/agent/crates/layerx-agent-api/src/operation_generated.rs).

## layerx-agentd

`interface.9.cli-layerx-agentd` declares the environment-configured
`layerx-agentd` application. Package
[layerx-agentd-host](../../../services/agent/crates/layerx-agentd-host/Cargo.toml)
builds that binary from
[layerx-agentd/src/main.rs](../../../services/agent/crates/layerx-agentd/src/main.rs).
Normal service startup uses no arguments; evidence export is the separate mode
below. Configuration includes `LAYERX_AGENT_*` settings and policy/runtime-clock
settings read by the startup path, not a single universal endpoint setting.

Full mode's program route uses `LAYERX_AGENT_NODE_ENDPOINT` and
`LAYERX_EXPECTED_CHAIN_ID`. The shared human-owner node client uses
`LAYERX_AGENT_NODE_RPC_URL` and `LAYERX_AGENT_NODE_CHAIN_ID`; configure these
consistently for the intended network. See
[node_rpc.rs](../../../services/agent/crates/layerx-agentd/src/node_rpc.rs) and
[agent configuration](agent-config.md) for the broader configuration surface.
Node chain-ID checks bind clients to their configured chain; the mainnet-beta
value is `125`.

Startup calls `refuse_retired_environment` in `node_rpc::environment`; obsolete
human settlement/export trust and MCP sequence settings are rejected. Boot
errors exit 2 with redacted diagnostics. Do not substitute agent availability
or local operator state for a committed chain receipt.

## layerx-agentd-mcp-evidence

`interface.9.cli-layerx-agentd-mcp-evidence` declares
`layerx-agentd --mcp-evidence ROOT TENANT KEY [TOOL]` in
[mcp_evidence_mode](../../../services/agent/crates/layerx-agentd/src/main.rs).
`ROOT` must be an absolute canonical store path. `TENANT` is validated as a
`TenantId`; `KEY` is exactly 64 lowercase hex characters encoding a nonzero
32-byte key. Optional `TOOL` must be non-empty, at most 128 bytes and contain no
NUL. The store opens read-only and the selected evidence prints as JSON.
Refusal exits 2 with `layerx-agentd: MCP evidence refused`.

## layerx-agentd-modes

Source: [mode selection](../../../services/agent/crates/layerx-agentd/src/human_owner_mode.rs) and [startup](../../../services/agent/crates/layerx-agentd/src/main.rs).

| Interface | Current setting | Behavior |
|---|---|---|
| `interface.9.cli-layerx-agentd-mode-full` | `LAYERX_AGENT_MODE=full`, or unset | Starts the full configured agent application. |
| `interface.9.cli-layerx-agentd-mode-human-owner` | `LAYERX_AGENT_MODE=human-owner` | Starts the human owner and exposes only authenticated `GET /healthz` on its HTTP listener. |
| Inference market | `LAYERX_AGENT_MODE=inference-market` | Starts the configured inference-market service. |

Other values, including an empty value, are invalid. Human-owner health uses
`LAYERX_AGENT_PROGRAM_LISTEN` (loopback `127.0.0.1:`) and
`LAYERX_AGENT_PROGRAM_BEARER_TOKEN` (at least 32 bytes, distinct from
`LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`). Health is ready only while the owner,
node connection and human authority are available: HTTP 200 with
`{"ready":true}`, otherwise HTTP 503 with `{"ready":false}`. Other authenticated
paths return 404. `LAYERX_AGENT_MCP_BINDING_ROOT` is refused in human-owner mode;
MCP binding publication needs full mode's program endpoint.

## layerx-audit-verify

`interface.9.cli-layerx-audit-verify` declares `layerx-audit-verify PATH`.
The [main function](../../../services/agent/tools/audit-verify/src/main.rs)
verifies the audit hash chain, printing `entries=... tail_hash=...` on success.
Missing or extra arguments exit 2; verification errors exit 1. This checks agent
audit records and does not confer chain finality.

## layerx-boundary-check

`interface.9.cli-layerx-boundary-check` declares `layerx-boundary-check [ROOT]`.
The [implementation](../../../services/agent/tools/boundary-check/src/main.rs)
defaults to `agent`; use `layerx-boundary-check services/agent` from the repository
root. It walks `ROOT/crates` and requires readable `unsafe-allowlist.toml` and
`nondeterminism-allowlist.toml`. It refuses forbidden dependencies, private C
bindings/layouts, node-private paths, unapproved unsafe code and ambient
nondeterminism outside the clock allowlist. Any `layerx_client::lni` reference
is a retired transport violation, including inside the client crate; it prints
`direct-core-connection` with the file path. No stable-ABI allowlist is read.
Success prints `agent boundary purity passed`; violations exit 1.

## layerx-mcp

`interface.9.cli-layerx-mcp` declares
`layerx-mcp <absolute path to the daemon binding document>`.
The [binary](../../../services/agent/crates/layerx-mcp/src/bin/layerx-mcp.rs)
opens the binding and its agent client before serving MCP JSON-RPC over stdio,
or over the Unix socket configured in the binding's listener section. Exactly
one binding-path argument is required. Startup/serving errors print
`layerx-mcp:` and exit nonzero. This is the agent's MCP surface; node JSON-RPC
remains at the configured node endpoint. See [MCP tools](mcp-tools.md).

## layerx-policy-harness

`interface.9.cli-layerx-policy-harness` declares `layerx-policy-harness` without
required arguments. The [implementation](../../../services/agent/tools/policy-harness/src/main.rs)
evaluates the adversarial corpus, checks deny-by-default and required blocked
constraint coverage, then prints each decision and aggregate case counts.
Its inputs use `EvaluationInput::without_protocol_budget`; do not interpret
success as proof that an unavailable protocol budget permits an operation.
A failed expectation panics.

## layerx-wallet-rpc

Source: [layerx-wallet-rpc.rs](../../../services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs).
These interfaces share one binary: send one JSON request on stdin; it writes
one JSON response on stdout and exits zero for success or nonzero for failure.
Input is bounded to 2,097,152 bytes.

| Interface | Current request |
|---|---|
| `interface.9.cli-layerx-wallet-rpc` | `layerx-wallet-rpc`, with `action` and `configuration` in the input JSON |
| `interface.9.cli-layerx-wallet-rpc-accounts` | `action="accounts"`, `did` |
| `interface.9.cli-layerx-wallet-rpc-balance` | `action="balance"`, `did`, `asset` |
| `interface.9.cli-layerx-wallet-rpc-submit` | `action="submit"`, EVM signing configuration and payload fields; returns `txHash` and `rung="submitted"` inside `result` |
| `interface.9.cli-layerx-wallet-rpc-wait` | `action="wait"`, `txHash`, `rung`, `timeout_ms`; rungs are `submitted`, `receipted`, `committed` |

`configuration.endpoint` selects the endpoint; `configuration.rpc_route` is
`node` or `gateway` (default). The former selects `RpcClient::connect_node`,
the latter `RpcClient::connect`; optional `ca_der` supplies certificate bytes.
Optional `key_id` requires `key_secret`. Node and hosted gateway method
availability differ as described above. The policy decoder also requires
`protocol_version`, `network_id`, `sequencer_id`, `sequencer_key`, `first_batch`
and `last_batch`; `checkpoint_context_digest` is optional. These retained field
names configure receipt verification, not a separately deployed network role.
`accounts` and `balance` obtain a runtime clock from the environment.

Submit reads `configuration.evm_key_ref` as an absolute regular-file path with
no group/other permission bits, derives the account using the decimal-string
`account_index`, and uses `native_asset`. Request fields are `actor`,
`idempotency_key`, `fee_limit`, `not_before`, `not_after`, `ordinal`, `payload`
and `timeout_ms`. Byte fields use unprefixed even-length hex; fixed fields are
length checked. Integer fields decoded by `number` are canonical decimal
strings. Timeout is capped at 300,000 milliseconds. Submit prepares the Asset
module payload and signs an EVM transaction through the SDK wallet.

Wait takes a nonzero 32-byte `0x`-prefixed `txHash`. `submitted` returns that
rung immediately. Higher rungs poll `pax_sequencerReceipt` and verify receipt
signature, transaction hash and protocol version before returning a receipted
result. A `committed` request does not automatically gain verified finality:
the current bridge returns `MissingFinalityTrust` when it observes a committed
receipt without the required finality verification path, surfaced as the
generic bridge error below. The chain's commit semantics and a client's proof
capabilities are distinct. The obsolete `commitment` field is refused on submit
and wait; a submit `rung`, if supplied, must be `submitted`.

Responses use `{"ok":true,"result":...}` or `{"ok":false,"error":...}`.
Remote RPC failures preserve `code`, `message` and `data`. Pending waits report
`kind="pending"`, `txHash` and `rung="submitted"`. Other invalid/unavailable
requests report `kind="wallet_request_refused_or_unavailable"`.

## operator-commands

These are agent command descriptors, not shell subcommands of `layerx-agentd`.
The catalogue is [admin.rs](../../../services/agent/crates/layerx-agentd/src/admin.rs)
(`commands()`); human peer `OPERATOR` requests reach `route_operator_command` in
[human_runtime.rs](../../../services/agent/crates/layerx-agentd/src/human_runtime.rs).
The route takes a store root, tenant, outbox, operator identity and request ID;
it returns the selected action plan and audit count. A descriptor or plan is
not evidence that a protocol operation has completed.

| Interface | Command | Selected route |
|---|---|---|
| `interface.9.cli-agentd-operator-unknown-inspect` | `unknown.inspect` | Read local unknown-submission status. |
| `interface.9.cli-agentd-operator-unknown-resolve` | `unknown.resolve` | Receipt lookup and exact-byte resend plan. |
| `interface.9.cli-agentd-operator-subscription-inspect` | `subscription.inspect` | Inspect stalled delivery. |
| `interface.9.cli-agentd-operator-subscription-resume` | `subscription.resume` | Resume local delivery from its cursor. |
| `interface.9.cli-agentd-operator-budget-inspect` | `budget.inspect` | Inspect reconciliation evidence. |
| `interface.9.cli-agentd-operator-budget-reconcile` | `budget.reconcile` | Reconcile local cache from verified evidence. |
| `interface.9.cli-agentd-operator-verification-inspect` | `verification.inspect` | Inspect verification backlog. |
| `interface.9.cli-agentd-operator-verification-retry` | `verification.retry` | Retry evidence verification. |
| `interface.9.cli-agentd-operator-activity-submit` | `activity.submit` | Ordinary client write path. |
| `interface.9.cli-agentd-operator-budget-create` | `budget.create` | Ordinary client write path. |

All descriptors currently have `protocol_mutating=false`: the operator surface
cannot edit receipts, verification levels, outcomes or protocol values. Write
plans must enter ordinary policy/sign/submit processing; operator action does
not bypass it. Catalogue terms such as “activity” are retained command names,
not instructions to submit a retired kernel envelope.

## run-real-node-tests

Source: [run-real-node-tests.sh](../../../services/agent/tools/run-real-node-tests.sh).

| Interface | Current invocation | Behavior |
|---|---|---|
| `interface.9.cli-run-real-node-tests-test` | `sh services/agent/tools/run-real-node-tests.sh test` | Runs `make agent-test`. |
| `interface.9.cli-run-real-node-tests-sanitizers` | `sh services/agent/tools/run-real-node-tests.sh sanitizers` | Runs `make agent-test-sanitize`. |
| `interface.9.cli-run-real-node-tests-prepare` | `sh services/agent/tools/run-real-node-tests.sh prepare` | Builds `paxd`, exports its paths and prints its SHA-256. |

The script requires exactly one mode and root privileges to arrange distinct
agent user identities. It checks Forge, Cast, Anvil, Go and the listed Python
crypto dependencies. `PAXEER_X_RUNTIME_MANIFEST`, if supplied, provides the chain
ID and RPC port; otherwise the chain ID is 125 and the port comes from
`LAYERX_TEST_PAXEER_RPC_PORT` or defaults to 8545. It exports
`LAYERX_TEST_PAXEER_CHAIN_ID` and `LAYERX_TEST_PAXEER_RPC_PORT` for the selected
path. Prepare requires `CARGO_TARGET_DIR`; `LAYERX_TEST_NATIVE_BUILD_DIR`
selects native output, defaulting to the repository's `build` directory in the
prepare function. The script restores build-output ownership on exit.

## run-sanitizers

`interface.9.cli-run-sanitizers` declares
`sh services/agent/tools/run-sanitizers.sh`.
The [script](../../../services/agent/tools/run-sanitizers.sh) runs the locked agent
workspace tests for `x86_64-unknown-linux-gnu` with AddressSanitizer, followed by
ThreadSanitizer with a rebuilt standard library using `nightly-2025-11-10`.
It requires that toolchain's `rust-src`; missing sources fail immediately.
`LAYERX_AGENT_TSAN_TARGET_DIR` overrides the separate TSan target directory.

