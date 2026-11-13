# layerx-agentd

This crate is the non-authoritative agent owner library. The `layerx-agentd`
binary is declared by [`layerx-agentd-host`](../layerx-agentd-host/Cargo.toml)
and built from this crate's [`src/main.rs`](src/main.rs). It provides local
sessions, policy, approvals and verified network access; it does not run
consensus or determine finality. Paxeer's kernel proposes blocks within the
single network, and a committed block makes its receipts final.

See the [workspace README](../../README.md) for the node boundary and test
prerequisites, the [compatibility matrix](../../../../docs/agent/COMPATIBILITY.md) for Agent API
major 2, and the [MCP README](../layerx-mcp/README.md) for session bindings.

## Modes

`LAYERX_AGENT_MODE` selects `full` (the default when unset), `human-owner` or
`inference-market`. Every other value, including an empty value, is refused.
Mode selection is implemented in
[`human_owner_mode.rs`](src/human_owner_mode.rs).

Full mode starts the shared Human owner and verified Programs operations.
In addition to the Human configuration, it requires Programs credentials,
the probe program, deployment journal, replica identity, policy sources and
the protected chain trust root. The Programs reader uses
`LAYERX_AGENT_NODE_ENDPOINT` and `LAYERX_EXPECTED_CHAIN_ID`; these are distinct
settings from the Human node connection described below. Full-mode startup
also obtains export trust through `LAYERX_AGENT_NODE_CONSENSUS_ENDPOINT`.
The exact configuration checks are in `config`, `serve` and
`start_shared_owner` in [`main.rs`](src/main.rs).

Inference-market mode uses the shared owner and Programs configuration and
loads `InferenceConfig` from [`config.rs`](src/config.rs). Its additional
settings include `LAYERX_AGENT_INFERENCE_MODEL_DIGEST`,
`LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST` and `LAYERX_AGENT_INFERENCE_ROOM_PROFILE`.
`LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN` and `LAYERX_AGENT_INFERENCE_SERVE_LISTEN`
select its signaling and serving listeners.

Human-owner mode starts the Human owner and a health listener. It does not
initialize the Programs reader, probe program or deployment journal. It
refuses a non-empty `LAYERX_AGENT_MCP_BINDING_ROOT`, which needs the Programs
operations available in full and inference-market modes.
All modes require the Human peer, authority, protected store, session
key, Unix socket, limit and frame-bound inputs consumed by `start_shared_owner`.
`LAYERX_NATIVE_POLICY_SOURCES` must cover exactly the configured Human tenants.
The runtime clock supervisor binding uses `LAYERX_RUNTIME_CLOCK_SOCKET`,
`LAYERX_RUNTIME_CLOCK_PID` and `LAYERX_RUNTIME_CLOCK_UID`.

All modes require a chain-checked Human node JSON-RPC connection and an
independently authenticated Human authority registry. No LNI connection is
required. Configure the Human node connection with:

| Setting | Meaning |
| --- | --- |
| `LAYERX_AGENT_NODE_RPC_URL` | Node JSON-RPC root URL |
| `LAYERX_AGENT_NODE_CHAIN_ID` | Expected EVM chain ID, `125` or `0x7d` on mainnet-beta |
| `LAYERX_AGENT_HUMAN_NETWORK_ID` | Human network ID; must agree with the node chain ID |
| `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION` | Current canonical wire protocol version, checked by `HumanNodeEnvironment` |

[`NodeRpc::connect`](src/node_rpc.rs) checks `eth_chainId` and refuses a
mismatch. HTTPS uses the system trust store on this environment-based
connection. Node RPC is served at the root path; tenant domains do not need
separate node processes.

Configure the independent authority registry with:

| Setting | Meaning |
| --- | --- |
| `LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT` | HTTPS authority endpoint, independent of the node JSON-RPC URL |
| `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER` | Authority credential of at least 32 bytes, distinct from the health credential |
| `LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER` | Non-empty, readable DER-encoded CA file authenticating the authority server |
| `LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES` | Positive response size bound, at most the Human response limit |
| `LAYERX_AGENT_HUMAN_DEADLINE_MS` | Positive Human transport and authority deadline in milliseconds |
| `LAYERX_AGENT_HUMAN_PEERS` | Peer UID, principal and tenant bindings; startup reads the registry for every peer |

Startup refuses a node chain-ID mismatch or an unavailable or unverified
authority registry. A reachable node alone is insufficient for readiness.

### Human-owner health

`LAYERX_AGENT_PROGRAM_LISTEN` must start with `127.0.0.1:`.
`LAYERX_AGENT_PROGRAM_BEARER_TOKEN` must be at least 32 bytes and distinct from
`LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`. An authenticated `GET /healthz` returns
200 with `{"ready":true}` only while the owner is running and a fresh node
chain-ID check and authority registry read for every configured peer succeed.
Each admitted probe reconnects to the node and queries the registry again;
successful startup does not cache readiness. The probe short-circuits on
node failure before contacting the authority. See `dependencies_ready` and
`serve_health` in [`main.rs`](src/main.rs) and the shared
[`owner_health` handler](src/human_receipt_frame.rs).
Dependency failure returns 503 with `{"ready":false}`; owner termination stops
the process. Other admitted paths return 404. The HTTP reader enforces a
16 KiB header bound and a five-second request deadline.

## Outbound TLS

`LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER` is required in all modes and names the
DER-encoded CA file for the Human authority HTTPS endpoint. Full and
inference-market modes also require `LAYERX_AGENT_AUTHORITY_CA_DER` for Programs
node and authority clients. Empty or unreadable CA files are refused. The Programs listener,
node and authority credentials are separate: `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`
must differ from `LAYERX_AGENT_NODE_BEARER_TOKEN` and
`LAYERX_AGENT_AUTHORITY_BEARER_TOKEN`, and each must be at least 32 bytes.

The private-CA client uses `ureq` with rustls and only the supplied CA;
hostname verification remains enabled. Human authority endpoints require
HTTPS. Programs endpoints allow HTTPS or HTTP on a loopback host. See
[`outbound_tls.rs`](src/outbound_tls.rs) for private and system trust handling.

The registered library TLS test is
`outbound_tls::private_ca_checks_the_actual_server_identity`. It uses the
shared [`tls_boundary.rs`](../../../platform/tests/support/tls_boundary.rs)
harness, which invokes the `openssl` executable and a real
`layerx-paxeer-boundary` binary. `make agent-test` builds that binary; the
harness accepts its location through `LAYERX_PAXEER_BOUNDARY_BIN`. It launches
child probes with `LAYERX_TLS_QUAL_ENDPOINT`, `LAYERX_TLS_QUAL_EXPECT` and
`LAYERX_TLS_QUAL_CA_DER` for trusted, unrelated-root, wrong-hostname,
missing-root, empty-root and CA-as-server cases. The separate source file
[`src/outbound_tls/tests.rs`](src/outbound_tls/tests.rs) is registered as
`outbound_tls::tests`; its in-process TLS tests are described in the
[TLS test guide](src/outbound_tls/README.md).
