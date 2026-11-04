# Agent configuration contract

Agent services connect to the merged Paxeer network, with mainnet-beta EVM chain
ID **125** (`0x7d`). Use a node's HTTPS root, for example
`https://api1.mainnet-beta.paxeer.network/`. Numbered public nodes run from
`api1` through `api14`, with `api-mainnet-beta.paxeer.network` also available.
Hosted gateway methods have a distinct [OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is the library mirror, never a node connection setting.
Tenants identify domains in storage on this one network.

See [API](agent-api.md), [formats](agent-formats.md), [Human peer](agent-human-peer.md)
and [Rust API](agent-rust-api.md). Configuration below distinguishes the reusable
library loader from the executable's actual environment readers.

## Agentd startup

[`config::load`](../services/agent/crates/layerx-agentd/src/config.rs) loads an
explicitly named protected UTF-8 file with unquoted `key=value` lines, ignoring
blank lines and `#` comments. The file is at most 65,536 bytes and each line at
most 4,096 bytes. Exact environment keys override file values. All eight settings
are required with no security defaults; duplicates, unknown settings, empty
values and incomplete tenant maps fail closed.

| File key | Environment override | Meaning |
| --- | --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` | Nonzero u32 signing scope |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` | HTTP(S) URL without user info or fragment |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` | Nonzero u64 chain assertion; 125 for mainnet-beta |
| `tenants` | `LAYERX_TENANTS` | Unique comma-separated tenant names |
| `policy_sources` | `LAYERX_POLICY_SOURCES` | Complete `tenant:/absolute/path` map |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` | Complete per-tenant absolute-path map |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` | Complete per-tenant evidence-level map |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` | Literal `px_getReceiptKeys` or pinned absolute file path |

This loader accepts lowercase letters, digits, `-`, `_` and `.` in tenant names.
Its historical evidence-level strings are `sequencer-signed`, `batch-included`,
`state-proven`, `checkpoint-finalised` and `settlement-anchored`; these are parser
values, not assertions that all evidence sources are available. New API write
results use the [three rungs](agent-api.md#verification-and-errors).
`expected_protocol_version` / `LAYERX_EXPECTED_PROTOCOL_VERSION` and
`sequencer_authority_source` / `LAYERX_SEQUENCER_AUTHORITY_SOURCE` are retired
and explicitly refused.

Protected sources must be absolute canonical paths to regular files owned by the
process UID with no group/other permissions, such as mode `0600`. The loader
checks file/path metadata before and after reading and rejects changes or
indirection. Do not store secrets in documentation or diagnostic output.

## Agentd env

The [`layerx-agentd` entry point](../services/agent/crates/layerx-agentd/src/main.rs)
does **not** call `config::load`. It reads the following environment families
directly; setting only the library's file keys does not configure this binary.

| Setting | Executable behavior |
| --- | --- |
| `LAYERX_AGENT_MODE` | Unset or `full` starts full mode; `human-owner` starts the owner and health listener; other values fail |
| `LAYERX_AGENT_PROGRAM_LISTEN` | Loopback `127.0.0.1:` listener |
| `LAYERX_AGENT_PROGRAM_BEARER_TOKEN` | At least 32 bytes; health/read credential |
| `LAYERX_AGENT_NODE_ENDPOINT`, `LAYERX_AGENT_NODE_BEARER_TOKEN` | Full-mode read backend and distinct backend credential |
| `LAYERX_EXPECTED_CHAIN_ID` | Full-mode expected chain ID |
| `LAYERX_AGENT_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_AUTHORITY_BEARER_TOKEN` | Full-mode authority connection |
| `LAYERX_AGENT_AUTHORITY_CA_DER`, `LAYERX_AGENT_AUTHORITY_REPLICA_ID` | Trust certificate file and exact digest |
| `LAYERX_AGENT_CHAIN_TRUST_ROOT` | Absolute protected validator-trust source |
| `LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS` | Nonzero bound for program views |
| `LAYERX_AGENT_DEPLOYMENT_JOURNAL`, `LAYERX_AGENT_PROGRAM_PROBE_ID` | Registry journal and nonzero program digest |
| `LAYERX_POLICY_SOURCES` | Policy path for every configured Human tenant |
| `LAYERX_AGENT_NODE_RPC_URL`, `LAYERX_AGENT_NODE_CHAIN_ID` | Node JSON-RPC URL and chain ID; decimal or `0x` chain ID accepted by `NodeRpcConfig` |
| `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION`, `LAYERX_AGENT_HUMAN_NETWORK_ID` | Current compiled occupancy protocol and network ID; network must equal node chain ID |
| `LAYERX_AGENT_NODE_CONSENSUS_ENDPOINT` | Node consensus connection for committed evidence |

Full mode requires the program bearer to differ from both backend bearers.
Human-owner mode requires its health bearer to differ from its Human authority
bearer. `NodeRpcConfig` supplies a 30-second default deadline and the connection
checks the observed chain ID. Protocol version and chain ID are different values.

The optional operation listener uses `LAYERX_AGENTD_RPC_LISTEN` with
`LAYERX_AGENTD_RPC_TLS_CERT`, `LAYERX_AGENTD_RPC_TLS_KEY`,
`LAYERX_AGENTD_RPC_TLS_CLIENT_CA`, `LAYERX_AGENTD_RPC_PEER`,
`LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT`,
`LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES`,
`LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES` and
`LAYERX_NODE_NETWORK_NAME`. See [transport](agent-api.md#agent-http-transport).

## Human peers

`LAYERX_AGENT_HUMAN_PEERS` is comma-separated, with exact ordered named fields:

```text
uid=4020;tenant=beta;principal=did:layerx:beta:alice,uid=4021;tenant=other;principal=did:key:z123
```

[`human_peer_config.rs`](../services/agent/crates/layerx-agentd/src/human_peer_config.rs)
rejects positional forms, whitespace, control characters, trailing entries,
duplicate UIDs, extra fields and invalid DIDs. UID is decimal u32; tenant is at
most 128 ASCII alphanumeric, `-` or `_` bytes. This parser's tenant grammar is
narrower than the reusable startup loader's grammar.

Owner setup also reads `LAYERX_AGENT_HUMAN_STORE`,
`LAYERX_AGENT_HUMAN_SOCKET`, `LAYERX_AGENT_HUMAN_SOCKET_UID`,
`LAYERX_AGENT_HUMAN_SOCKET_GID`, `LAYERX_AGENT_HUMAN_SOCKET_MODE` (octal),
`LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT`,
`LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE`,
`LAYERX_AGENT_HUMAN_DEADLINE_MS`, `LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES`,
`LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN`, `LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES`,
`LAYERX_AGENT_HUMAN_MAX_CONNECTIONS`, `LAYERX_AGENT_HUMAN_MAX_STREAMS` and
`LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES`. The Human authority connection uses
`LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`,
`LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER` and `LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES`.
The UID mapping establishes transport identity before any frame is decoded;
see [peer authentication](agent-human-peer.md#authentication).

## Native program policy source

`LAYERX_NATIVE_POLICY_SOURCES` maps every configured Human tenant to a protected
JSON file. Its exact fields are `version`, `tenant` and `policy`; version is
`layerx.native-program-policy-source.v1`, tenant must match, and `policy` is the
serialized policy string. The reader caps this source at 2 MiB and rejects
unknown fields. The embedded [program policy](agent-formats.md#native-program-policy)
is separately validated.

## Native effect policy source

`LAYERX_NATIVE_EFFECT_POLICY_SOURCES` uses the same tenant/path grammar for
explicitly opted-in tenants. Its protected source has version
`layerx.native-effect-policy-source.v1`, matching `tenant` and serialized
`policy`. Unknown or duplicate tenants are refused. This policy does not confer
protocol signing authority.

## Program budget denominations

`LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` maps opted-in tenants to protected
JSON documents: `version: 1`, `tenant`, and `limits` rows containing `stable_id`,
`asset` and `source`. Digests are exactly 64 lowercase hexadecimal characters;
`source: "*"` explicitly means any source, otherwise it is a digest. Missing
fields, invalid/duplicate bindings and zero assets are refused. These bindings
preserve the asset/source denomination of each program limit.

## Registry source

`LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` selects an optional protected JSON file.
[`RegistrySourceProvider`](../services/agent/crates/layerx-agentd/src/registry_source.rs)
requires `version: 1`, `endpoint`, `server_ca_der`, `client_certificate_pem`,
`client_private_key_pem`, `request_token_file`, `deadline_ms` (1–60,000) and
`maximum_response_bytes` (1–1,048,576). Endpoint is an HTTPS origin, without
credentials, query, fragment or a path other than an optional trailing slash.
Certificate/key/token files also use protected-source loading. Requests use
mutual TLS and a token, with redirects disabled. Source reproducibility metadata
must bind the verified program version, code hash, interface and state view.

## MCP binding

`LAYERX_AGENT_MCP_BINDING_ROOT` opts into binding publication in full mode;
human-owner mode refuses it. Configure the `LAYERX_AGENT_MCP_` settings
`AUDIT_ROOT`, `MODE` (`full` or `read-only`), `PEER_UID`, `AGENT_DID`,
`SESSION_ID`, `CAPABILITY_ID`, `ACTIVITY_TYPES`, `SCOPES`, `EXPIRY_HEIGHT`,
`CORE_HEIGHT`, `OPENING_CLIENT`, `POLICY_VERSION` and `DEADLINE_MS`.
Limit settings are `LIMIT_ID`, `LIMIT_NAME`, `LIMIT_SCOPE`, `LIMIT_SCOPE_ID`,
`LIMIT_CEILING` and `LIMIT_CONSUMED`; listener settings are `LISTENER_SOCKET`,
`LISTENER_OWNER_UID`, `LISTENER_OWNER_GID`, `LISTENER_MODE` and
`LISTENER_ADMITTED_UIDS`, all with that prefix. Admission must include the peer UID.

The binding contains references to protected credential files, not inline
secrets. See [binding format](agent-formats.md#mcp-binding-document). Optional
web-sidecar configuration starts with `LAYERX_AGENT_MCP_WEB_ENDPOINT`; the
entry point requires the web receipt-key index, network, timeout, pending-attempt
bound and approval threshold when enabled.

## Harness env

Test and qualification harness settings belong to the binaries under
[`services/agent/tests`](../services/agent/tests). They do not override executable
startup requirements or turn an unavailable proof source into trusted evidence.
Use the harness's own entry point for its exact inputs; never substitute test
signing keys, clock data or fixtures for production authority.
