# Agent service variables

The agent service in `services/agent` contains the agent daemon, MCP server, client libraries and SDKs. It connects to the merged Paxeer network: mainnet-beta has EVM chain ID 125, node JSON-RPC uses the root path, and receipts become final when their blocks commit. Configure endpoints explicitly for the selected deployment.

The tables list environment inputs and configuration-file keys in name order. Source paths are relative to `services/agent`. For file settings, the name column identifies the file selector, section (or JSON object) and key; file paths are supplied by the operator. “None” means the reader supplies no default. Required settings apply only to the mode, API or enabled feature named in the row. Optional feature selectors may make other settings required. Required file members apply when that file is selected; variant-specific members apply to the selected variant.

The executable reads its environment directly; `.env.example` is a template for the launcher, which must supply the selected values. Blank template entries have no default and need deployment-specific values before use. Commented optional entries remain absent unless enabled; a blank override is not equivalent to an absent setting for every reader. Cargo supplies its own build metadata. Test-only inputs are omitted.

The startup-config library API reads one explicit protected UTF-8 `key=value` file with no sections. Its eight exact environment overrides take precedence over file values; unknown names, blank overrides, duplicate file keys and incomplete tenant maps are rejected. This API is separate from the executable's mode-specific environment reader. Protected JSON sources use absolute, normalized, process-owned regular files with no group or other permissions. Paths to credentials are marked nonsecret; their contents must remain secret.

## Required

| Name / file · section · key | What it sets and when it is read | Source default | Secret value? | Source reader |
| --- | --- | --- | --- | --- |
| `CARGO_MANIFEST_DIR` | Crate directory supplied by Cargo to build scripts; build metadata; supplied by Cargo | None | No | `crates/layerx-agent-api/build.rs`; `crates/layerx-types/build.rs` |
| `CARGO_PKG_VERSION` | Package version supplied by Cargo at compile time; build metadata; supplied by Cargo | None | No | `crates/layerx-agentd/src/compat.rs`; `crates/layerx-sdk/src/production.rs`; `crates/layerx-sdk/src/agent_envelope.rs`; `crates/layerx-mcp/src/stdio.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer.nodes[] / `address` | Attestor node socket endpoint | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer.nodes[] / `id` | Attestor node identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `client_certificate_chain` | Paths to client certificate chain material | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `client_private_key_pkcs8` | Path to the private client key; secret file contents | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `evm_address` | EVM signer identity | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `evm_key_id` | EVM key identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `key_id` | Ed25519 key identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `network_id` | Signer network identity | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `nodes` | Attestor node declarations | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `public_key` | Ed25519 public key | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `signers` | Selected attestor node identifiers | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `timeout_seconds` | Attestor timeout in seconds | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / [].signer / `trust_anchors` | Paths to trusted DER certificates | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / each array element / `assertion_source` | Path to the protected payer assertion; secret file contents | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / each array element / `payer_did` | Payer DID | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / each array element / `session_id` | Bound session digest | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / each array element / `signer` | Attestor signer configuration | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` / each array element / `tenant` | Payer tenant | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES` | Daemon sequence retention bound for idempotency; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES` | Protocol sequence retention bound for idempotency; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT` | Absolute path to persistent idempotency records; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_PEER` | Expected TLS client peer identity; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_TLS_CERT` | Absolute path to the RPC TLS certificate; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_TLS_CLIENT_CA` | Absolute path to the RPC TLS client CA; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_TLS_KEY` | Absolute path to the RPC TLS private key; the path itself is not a secret; required when LAYERX_AGENTD_RPC_LISTEN enables RPC | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_AUTHORITY_BEARER_TOKEN` | Authority bearer credential, distinct from the program credential; full and inference-market modes | None | Yes | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_AUTHORITY_CA_DER` | Path to the DER CA used for authority and node connections; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_AUTHORITY_ENDPOINT` | Authority endpoint; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_AUTHORITY_REPLICA_ID` | Authority replica digest; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_CHAIN_TRUST_ROOT` | Absolute path to the protected binary chain trust root; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_DEPLOYMENT_JOURNAL` | Directory holding verified deployment admissions and records; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER` | Human authority bearer credential; all daemon modes | None | Yes | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER` | Path to the human authority DER CA; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT` | Human authority endpoint; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES` | Maximum human authority response size in bytes; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_DEADLINE_MS` | Human transport deadline in milliseconds; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_CEILING` | Budget ceiling as an unsigned integer; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_CONSUMED` | Already consumed budget as an unsigned integer; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_ID` | 16-byte hexadecimal budget limit identifier; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_NAME` | Budget limit name; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_SCOPE` | Budget scope: tenant, agent, session, capability or counterparty; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID` | 32-byte hexadecimal budget scope identifier; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_MAX_CONNECTIONS` | Maximum simultaneous Human connections; all daemon modes | None | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES` | Maximum Human frame size in bytes; all daemon modes | None | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES` | Maximum native activity payload size in bytes; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES` | Maximum queued Human transport bytes; all daemon modes | None | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_MAX_STREAMS` | Maximum simultaneous Human streams; all daemon modes | None | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_NETWORK_ID` | Human network identity; must match the node chain identity; all daemon modes | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_HUMAN_PEERS` | Comma-separated peer entries, each containing uid, tenant and principal separated by semicolons; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION` | Current state-commitment wire protocol version; all daemon modes | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT` | Absolute path to the persistent session-key directory; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE` | Absolute path to the protected operator-secret file; the path itself is not a secret; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_SOCKET` | Absolute path to the Human Unix socket; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_SOCKET_GID` | Human socket owner group ID; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_SOCKET_MODE` | Human socket permissions in octal; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_SOCKET_UID` | Human socket owner user ID; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_STORE` | Absolute path to the persistent agent store; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN` | Accepted activity timestamp span; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_ATTESTATION_FAMILY` | Attestation-family byte; required when host enrolment is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_BANDWIDTH_PROBE` | Bandwidth measurement probe URL; required when host enrolment is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` | Absolute path to protected inference deal and lap terms JSON; inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps.evidence_signers[] / `assertion_source` | Path to the protected evidence assertion; secret file contents | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps.evidence_signers[] / `signer` | Evidence attestor signer configuration | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps / `batch_size` | Settlement batch size from 1 through 64 | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps / `evidence_signers` | One through eight evidence signer configurations | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps / `host_meter_seed_source` | Path to the protected 32-byte host meter seed; secret file contents | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `actor_did` | Actor DID | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `assertion_source` | Path to the protected deal assertion; secret file contents | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `fee_limit` | Activity fee ceiling | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `gas_limit` | EVM gas limit | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `journal` | Absolute path to the deal operation journal | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `laps` | Lap settlement and evidence configuration | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `layer_count` | Model layer count | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `layer_memory_mib` | Memory per model layer in MiB | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `max_fee_per_gas` | EVM maximum gas price | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `max_priority_fee_per_gas` | EVM maximum priority gas price | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `resources` | Seven nonzero resource bounds | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `signer` | Deal attestor signer configuration | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / root / `validity_blocks` | Transaction validity from 1 through 256 blocks | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer.nodes[] / `address` | Attestor node socket endpoint | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer.nodes[] / `id` | Attestor node identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `client_certificate_chain` | Paths to client certificate chain material | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `client_private_key_pkcs8` | Path to the private client key; secret file contents | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `evm_address` | EVM signer identity | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `evm_key_id` | EVM key identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `key_id` | Ed25519 key identifier | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `network_id` | Signer network identity | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `nodes` | Attestor node declarations | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `public_key` | Ed25519 public key | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `signers` | Selected attestor node identifiers | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `timeout_seconds` | Attestor timeout in seconds | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / signer and laps.evidence_signers[].signer / `trust_anchors` | Paths to trusted DER certificates | None | No | `crates/layerx-agentd/src/main.rs`; `../human/crates/layerx-human-kms/src/attestor.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS: laps.canary_table_source` / entries[] / `expected_logits` | Expected canary logits | None | Yes | `crates/layerx-agentd/src/inference_market/canary.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS: laps.canary_table_source` / entries[] / `prompt_tokens` | Nonempty prompt token IDs | None | Yes | `crates/layerx-agentd/src/inference_market/canary.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS: laps.canary_table_source` / root / `entries` | Bounded canary examples | None | No | `crates/layerx-agentd/src/inference_market/canary.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS: laps.canary_table_source` / root / `model_digest` | Canary model digest | None | No | `crates/layerx-agentd/src/inference_market/canary.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `accept` | Signed acceptance transaction | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `enroll` | Signed enrolment transaction | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `expiry` | Document expiry height | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `lock_bond` | Signed bond-lock transaction | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `manifest_v1` | Legacy manifest bytes | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `manifest_v2` | Current manifest bytes | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `post_bid` | Signed bid transaction | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` / root / `publish` | Signed publication transaction | None | No | `crates/layerx-agentd/src/inference_market/enrol.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL` | Path to the persistent enrolment operation journal; required when host enrolment is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_GATEWAY_API_KEY` | Independent hosted AI gateway credential; inference-market mode | None | Yes | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_GATEWAY_CA_FILE` | Path to PEM or DER gateway trust roots; inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_GATEWAY_ENDPOINT` | Canonical HTTPS endpoint for the hosted AI projection; inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_MODEL_DIGEST` | Nonzero model digest; inference-market mode | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_INFERENCE_REGION` | Two-byte region code; required when host enrolment is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` | Path to the JSON room offer; inference-market mode | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / layer_plan[] / `end_layer` | Exclusive end of the assigned layer range | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / layer_plan[] / `first_layer` | First assigned model layer | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / layer_plan[] / `peer_key` | Peer public key | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / root / `layer_plan` | Ordered layer assignments | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / root / `model_digest` | Model digest | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / root / `price` | Canonical decimal room price | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / root / `room_id` | Room digest | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` / root / `runtime_digest` | Runtime digest | None | No | `crates/layerx-agentd/src/inference_market/signal.rs` |
| `LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST` | Nonzero runtime digest; inference-market mode | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_MCP_ACTIVITY_TYPES` | Comma-separated activity type numbers; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_AGENT_DID` | Agent DID; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_AUDIT_ROOT` | Absolute path to the audit directory; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_CAPABILITY_ID` | Capability digest; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_CORE_HEIGHT` | Current committed core height; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_MCP_DEADLINE_MS` | MCP operation deadline in milliseconds; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_EXPIRY_HEIGHT` | Session expiry height; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_MCP_LIMIT_CEILING` | MCP budget ceiling; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LIMIT_CONSUMED` | MCP consumed budget; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LIMIT_ID` | 16-byte hexadecimal MCP budget identifier; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LIMIT_NAME` | MCP budget name; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LIMIT_SCOPE` | MCP budget scope; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LIMIT_SCOPE_ID` | 32-byte hexadecimal MCP scope identifier; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS` | Comma-separated permitted MCP peer user IDs; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LISTENER_MODE` | MCP Unix socket permissions in octal; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LISTENER_OWNER_GID` | MCP socket owner group ID; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LISTENER_OWNER_UID` | MCP socket owner user ID; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_LISTENER_SOCKET` | Absolute path to the MCP Unix socket; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_MODE` | Binding mode: full or read-only; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_OPENING_CLIENT` | Client label recorded at session creation; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_PEER_UID` | Configured Human peer user ID; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_POLICY_VERSION` | Policy version associated with the binding; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_SCOPES` | Comma-separated distinct nonempty scope names; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_SESSION_ID` | Session digest; required when LAYERX_AGENT_MCP_BINDING_ROOT enables MCP enrolment in full or inference-market mode | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD` | Amount above which web payments require approval; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_NETWORK` | Web payment network identifier; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_PAYER_ADDRESS` | EVM payer identity; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS` | Maximum pending-payment attempts; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX` | Index of the receipt authority fetched from the node; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_RECEIVER_ADDRESS` | EVM receiver identity; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_TIMEOUT_MS` | Web payment timeout in milliseconds; required when the MCP web sidecar is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_NODE_BEARER_TOKEN` | Node bearer credential, distinct from the program credential; full and inference-market modes | None | Yes | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_NODE_CHAIN_ID` | Expected node chain identity, decimal or hexadecimal; all daemon modes | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_NODE_COMET_RPC_URL` | Consensus RPC URL for committed native receipts, state exports and policy history; required by those operations | None | No | `crates/layerx-agentd/src/human_runtime.rs` |
| `LAYERX_AGENT_NODE_CONSENSUS_ENDPOINT` | Consensus endpoint for validator evidence used by exports; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_NODE_ENDPOINT` | Node JSON-RPC root endpoint; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_NODE_RPC_URL` | Direct node JSON-RPC root URL; all daemon modes | None | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_PROGRAM_BEARER_TOKEN` | Program HTTP bearer credential; also authenticates human-owner health; all daemon modes | None | Yes | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_PROGRAM_LISTEN` | Program HTTP listener on IPv4 loopback; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS` | Positive maximum age of program evidence in milliseconds; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_PROGRAM_PROBE_ID` | Probe program identifier; full and inference-market modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `client_certificate_pem` | Path to the protected PEM client certificate chain | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `client_private_key_pem` | Path to the protected PEM client private key; secret file contents | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `deadline_ms` | Request deadline from 1 through 60000 milliseconds | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `endpoint` | HTTPS registry authority endpoint | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `maximum_response_bytes` | Response limit from 1 through 1048576 bytes | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `request_token_file` | Path to the protected bearer-token file; secret file contents | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `server_ca_der` | Path to the protected DER server CA | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` / root / `version` | Registry-source configuration schema version; must be 1 | None | No | `crates/layerx-agentd/src/registry_source.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` / root / `selections` | Nonempty bounded list of tenant/account slot selections | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` / root / `version` | State-export schema version; must be 1 | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` / selections[] / `account` | Selected account identifier | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` / selections[] / `slots` | Distinct storage-slot digests | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` / selections[] / `tenant` | Selected tenant identifier | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_EXPECTED_CHAIN_ID` | Expected EVM chain identity; mainnet-beta uses chain ID 125; full and inference-market modes or the startup-config API | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy (embedded JSON) / `network_id` | Policy network identity | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy (embedded JSON) / `rules` | Bounded list of policy rules | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy (embedded JSON) / `schema` | Native policy schema identifier | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy (embedded JSON) / `step_limit` | Evaluation step limit | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy (embedded JSON) / `version` | Policy version | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].activity / `module` | Activity module number | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].activity / `ordinal` | Activity ordinal | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `account` | Issuance, destruction or authorization account | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `asset` | Effect asset; optional for Authorization | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `authorization` | Authorization kind | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `from` | Transfer source | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `kind` | Scope tag: Operation, Transfer, Issuance, Destruction, Authorization or Fee | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `maximum_amount` | Decimal amount ceiling | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `source_account` | Fee source account | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `to` | Transfer destination | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `activity` | Activity module and ordinal | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `effect` | Permit or Deny | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `id` | Unique rule identifier | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `required_approval` | Whether approval is required | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `scope` | Tagged operation or effect restriction | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / root / `policy` | Serialized native policy JSON | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / root / `tenant` | Tenant owning the policy | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / root / `version` | Policy-source envelope version | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` | Comma-separated tenant:absolute-path mappings covering every Human tenant; all daemon modes | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy (embedded JSON) / `network_id` | Policy network identity | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy (embedded JSON) / `rules` | Bounded list of policy rules | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy (embedded JSON) / `schema` | Native policy schema identifier | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy (embedded JSON) / `step_limit` | Evaluation step limit | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy (embedded JSON) / `version` | Policy version | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].activity / `module` | Activity module number | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].activity / `ordinal` | Activity ordinal | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `asset` | Restricted asset | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `destination` | Restricted destination | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `kind` | Scope tag: Operation, Spend or ExitRoute | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `maximum_amount` | Decimal spend ceiling | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `owner_program` | Exit-route owner program | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `seed` | Exit-route derivation seed | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `source` | Spend source declaration | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope (fields required by kind) / `source_account` | Exit-route source account | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope.source (Program fields required by kind) / `kind` | Source tag: Principal or Program | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope.source (Program fields required by kind) / `owner_program` | Owner program | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope.source (Program fields required by kind) / `seed` | Program-account derivation seed | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[].scope.source (Program fields required by kind) / `source_account` | Derived source account | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `activity` | Activity module and ordinal | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `effect` | Permit or Deny | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `id` | Unique rule identifier | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `required_approval` | Whether approval is required | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `scope` | Tagged operation or effect restriction | None | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / root / `policy` | Serialized native policy JSON | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / root / `tenant` | Tenant owning the policy | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / root / `version` | Policy-source envelope version | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NETWORK_ID` | Startup network identity; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NODE_ENDPOINT` | Node JSON-RPC root endpoint; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NODE_NETWORK_NAME` | Network label in agent RPC responses; required when agent RPC is enabled | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_POLICY_SOURCES` | Comma-separated tenant:absolute-path policy mappings; full and inference-market modes or the startup-config API | None | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_POLICY_SOURCES` / root (line-oriented) / `steps` | Evaluation step limit | None | No | `crates/layerx-agentd/src/policy/version.rs` |
| `LAYERX_POLICY_SOURCES` / root (line-oriented) / `version` | Policy version | None | No | `crates/layerx-agentd/src/policy/version.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / limits[] / `asset` | Asset digest | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / limits[] / `source` | Source digest or wildcard | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / limits[] / `stable_id` | Budget limit digest | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / root / `limits` | Budget denomination bindings | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / root / `tenant` | Tenant owning the budget bindings | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` / root / `version` | Denomination schema version; must be 1 | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_RECEIPT_KEY_SOURCE` | Receipt-key source: px_getReceiptKeys or an absolute pinned-file path; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_RUNTIME_CLOCK_PID` | Expected clock supervisor process ID; daemon and wallet RPC runtime clock | None | No | `crates/layerx-client/src/runtime_clock.rs` |
| `LAYERX_RUNTIME_CLOCK_SOCKET` | Absolute path to the private runtime-clock Unix socket; daemon and wallet RPC runtime clock | None | No | `crates/layerx-client/src/runtime_clock.rs` |
| `LAYERX_RUNTIME_CLOCK_UID` | Expected clock supervisor user ID; daemon and wallet RPC runtime clock | None | No | `crates/layerx-client/src/runtime_clock.rs` |
| `LAYERX_SIGNER_CONFIGURATIONS` | Complete tenant:absolute-path signer configuration mappings; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_TENANTS` | Comma-separated distinct tenant identifiers; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_VERIFICATION_DEFAULTS` | Complete tenant:verification-level mappings; committed selects committed evidence; startup-config API; overrides the matching file key | None | No | `crates/layerx-agentd/src/config.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent.readiness / `endpoint` | Readiness gateway endpoint | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent.readiness / `gateway_key_file` | Path to the protected gateway credential; secret file contents | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent / `bearer_file` | Absolute path to the protected bearer file; secret file contents | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent / `endpoint` | Agent HTTP endpoint | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent / `probe_program` | Readiness probe program | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `ceiling` | Budget ceiling | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `consumed` | Consumed budget | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `id` | 16-byte limit identifier | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `name` | Limit name | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `scope` | Limit scope | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / limit / `scope_id` | Scope digest | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / listener / `admitted_uids` | One through 64 distinct admitted user IDs | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / listener / `mode` | Socket permissions in octal | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / listener / `owner_gid` | Socket owner group ID | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / listener / `owner_uid` | Socket owner user ID | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / listener / `socket` | Absolute path to the Unix socket | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `agent` | Agent endpoint configuration | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `audit_root` | Absolute path to the audit directory | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `capability_id` | Capability digest | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `core_height` | Current committed core height | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `deadline_ms` | Positive request deadline in milliseconds | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `limit` | Budget limit configuration | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `mode` | full or read-only deployment mode | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `session_generation` | Session revocation generation | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `session_id` | Session digest | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `session_token_file` | Absolute path to the protected session token; secret file contents | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `store` | Absolute path to the agent store | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `tenant` | Bound tenant | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web.receipt_key_index[] / `firstHeight` | First covered height | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web.receipt_key_index[] / `keyId` | Receipt authority digest | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web.receipt_key_index[] / `lastHeight` | Last covered height | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web.receipt_key_index[] / `publicKey` | Receipt authority public key | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `approval_threshold` | Payment approval threshold | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `endpoint` | Web sidecar endpoint | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `first_batch_number` | First covered receipt height | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `last_batch_number` | Last covered receipt height | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `network` | EIP-155 payment network identifier | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `payer_address` | Payer EVM identity | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `pending_attempts` | Pending-payment attempt limit | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `receipt_key_index` | Receipt-authority entries | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `receiver_address` | Receiver EVM identity | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `sequencer_id` | Receipt authority identifier | None | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `timeout_ms` | Positive timeout up to 60000 milliseconds | None | No | `crates/layerx-mcp/src/binding.rs` |
| `OUT_DIR` | Generated-source output directory supplied by Cargo; build metadata; supplied by Cargo | None | No | `crates/layerx-agent-api/build.rs`; `crates/layerx-types/build.rs`; `crates/layerx-types/src/guest_abi.rs`; `crates/layerx-types/src/programs_activity.rs`; `crates/layerx-types/src/programs_module_abi.rs`; `crates/layerx-types/src/result.rs` |
| `startup file passed to config::load` / root (no sections) / `expected_chain_id` | Expected EVM chain identity; overridden by LAYERX_EXPECTED_CHAIN_ID | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `network_id` | Network identity; overridden by LAYERX_NETWORK_ID | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `node_endpoint` | Node root endpoint; overridden by LAYERX_NODE_ENDPOINT | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `policy_sources` | Complete per-tenant policy paths; overridden by LAYERX_POLICY_SOURCES | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `receipt_key_source` | Node receipt-key method or protected pinned-file path; overridden by LAYERX_RECEIPT_KEY_SOURCE | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `signer_configurations` | Complete per-tenant signer paths; overridden by LAYERX_SIGNER_CONFIGURATIONS | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `tenants` | Tenant identifiers; overridden by LAYERX_TENANTS | None | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `verification_defaults` | Complete per-tenant verification levels; overridden by LAYERX_VERIFICATION_DEFAULTS | None | No | `crates/layerx-agentd/src/config.rs` |

## Optional

| Name / file · section · key | What it sets and when it is read | Source default | Secret value? | Source reader |
| --- | --- | --- | --- | --- |
| `LAYERX_AGENTD_MCP_BINDINGS` | Comma-separated paths to daemon MCP binding documents; agent RPC | Empty binding list | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_MCP_PAYERS` | Path to the protected JSON array of MCP payer configurations; agent RPC | No MCP payers attached | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENTD_RPC_LISTEN` | Enables and binds the mutually authenticated agent RPC listener | RPC listener disabled | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_CONSENSUS_ENDPOINT` | Consensus endpoint for program chain proofs | Uses the configured node endpoint when absent | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_GENESIS_TRUST` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_HANDOVER_FINALITY` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_HUMAN_EXPORT_SET_VERSION` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_RECONNECT_ATTEMPTS` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_RECONNECT_BASE_MS` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_RECONNECT_JITTER_PERCENT` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_HUMAN_RECONNECT_MAX_MS` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/human.rs` |
| `LAYERX_AGENT_INFERENCE_COMPLETION_RECEIPTS` | Path to completion receipt material; inference-market mode | Absent | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_COMPLETION_SOCKET` | Path to the local completion worker socket; inference-market mode | Absent | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps / `canary_answer_timeout_ms` | Positive canary answer timeout in milliseconds | 5000 | No | `crates/layerx-agentd/src/main.rs`; `crates/layerx-agentd/src/inference_market/canary.rs` |
| `LAYERX_AGENT_INFERENCE_DEALS` / laps / `canary_table_source` | Path to the protected model canary table | Absent | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_DRAFT_RUNTIME_KIND` | Draft runtime kind as an unsigned byte; inference-market mode | Absent | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_ENROLMENT` | Path to the signed host enrolment document; enables host enrolment; inference-market mode | Host enrolment disabled | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_INFERENCE_SERVE_LISTEN` | Inference completion listener; inference-market mode | IPv4 loopback, TCP port 8791 | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN` | Inference signal listener; inference-market mode | IPv4 loopback, TCP port 8790 | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_AGENT_MCP_BINDING_ROOT` | Absolute path to the MCP binding directory; enables MCP enrolment; full and inference-market modes | MCP enrolment disabled | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_CORE_SEQUENCE` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_MCP_EXPIRY_SEQUENCE` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_MCP_WEB_ENDPOINT` | Enables the MCP web sidecar and selects its endpoint; MCP enrolment | No web sidecar | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_MODE` | Selects full, human-owner or inference-market | `full` | No | `crates/layerx-agentd/src/human_owner_mode.rs` |
| `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG` | Absolute path to protected registry-source JSON configuration | Registry-source enrichment disabled | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_AGENT_SEQUENCER_TRUST_HISTORY` | Presence is rejected at startup; leave unset | Unset | No | `crates/layerx-agentd/src/node_rpc.rs` |
| `LAYERX_AGENT_STATE_EXPORT_SOURCE` | Absolute path to protected JSON account-slot selections | No configured state-export selections | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_EXPECTED_PROTOCOL_VERSION` | Rejected by the startup-config API; leave unset | Unset | No | `crates/layerx-agentd/src/config.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` | Comma-separated tenant:absolute-path native-effect policy mappings | Empty map | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` / policy.rules[] / `purpose_commitment` | Optional purpose digest | Absent | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_NATIVE_POLICY_SOURCES` / policy.rules[] / `purpose_commitment` | Optional purpose digest | Absent | No | `crates/layerx-agentd/src/policy/native_program.rs` |
| `LAYERX_POLICY_SOURCES` / first line / `schema` | Selects layerx.policy.source.v2 with explicit approval flags | Legacy two-field rules | No | `crates/layerx-agentd/src/policy/version.rs` |
| `LAYERX_POLICY_SOURCES` / root (line-oriented) / `rule` | Repeatable rule: identifier, permit/deny, and an approval flag for schema v2 | Empty rule list | No | `crates/layerx-agentd/src/policy/version.rs` |
| `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` | Comma-separated tenant:absolute-path budget denomination mappings | Empty map | No | `crates/layerx-agentd/src/main.rs` |
| `LAYERX_SEQUENCER_AUTHORITY_SOURCE` | Rejected by the startup-config API; leave unset | Unset | No | `crates/layerx-agentd/src/config.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent.readiness / `trust_anchors` | Path to PEM trust anchors | System trust roots | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / agent / `readiness` | Hosted readiness gateway configuration | Absent | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `core_sequence` | Rejected setting; omit the declaration | Absent | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `listener` | Unix socket listener configuration | Absent; stdio transport when listener is absent | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / root / `web` | Web sidecar configuration | Absent | No | `crates/layerx-mcp/src/binding.rs` |
| `MCP binding JSON passed to layerx-mcp or LAYERX_AGENTD_MCP_BINDINGS` / web / `sequencer_public_key` | Rejected setting; omit the declaration | Absent | No | `crates/layerx-mcp/src/binding.rs` |
| `startup file passed to config::load` / root (no sections) / `expected_protocol_version` | Rejected setting; omit the declaration | Absent | No | `crates/layerx-agentd/src/config.rs` |
| `startup file passed to config::load` / root (no sections) / `sequencer_authority_source` | Rejected setting; omit the declaration | Absent | No | `crates/layerx-agentd/src/config.rs` |
