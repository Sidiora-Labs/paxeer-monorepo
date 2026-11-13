# Agent config contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

Agent applications use the merged Paxeer BFT network, EVM chain ID `125` (`0x7d`). The kernel participates in block proposal and execution. Block commit establishes finality. Node JSON-RPC uses the root path; hosted methods are described by the [gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

## agentd-env

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.env-layerx-agent-authority-bearer-token</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_AUTHORITY_BEARER_TOKEN</code> |
| <code>interface.9.env-layerx-agent-authority-ca-der</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_AUTHORITY_CA_DER</code> |
| <code>interface.9.env-layerx-agent-authority-endpoint</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_AUTHORITY_ENDPOINT</code> |
| <code>interface.9.env-layerx-agent-authority-replica-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_AUTHORITY_REPLICA_ID</code> |
| <code>interface.9.env-layerx-agent-deployment-journal</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_DEPLOYMENT_JOURNAL</code> |
| <code>interface.9.env-layerx-agent-human-authority-ca-der</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER</code> |
| <code>interface.9.env-layerx-agent-human-authority-endpoint</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT</code> |
| <code>interface.9.env-layerx-agent-human-authority-max-bytes</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES</code> |
| <code>interface.9.env-layerx-agent-human-limit-ceiling</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_CEILING</code> |
| <code>interface.9.env-layerx-agent-human-limit-consumed</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_CONSUMED</code> |
| <code>interface.9.env-layerx-agent-human-limit-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_ID</code> |
| <code>interface.9.env-layerx-agent-human-limit-name</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_NAME</code> |
| <code>interface.9.env-layerx-agent-human-limit-scope</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_SCOPE</code> |
| <code>interface.9.env-layerx-agent-human-limit-scope-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID</code> |
| <code>interface.9.env-layerx-agent-human-max-payload-bytes</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES</code> |
| <code>interface.9.env-layerx-agent-human-network-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_NETWORK_ID</code> |
| <code>interface.9.env-layerx-agent-human-peers</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_PEERS</code> |
| <code>interface.9.env-layerx-agent-human-session-key-root</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT</code> |
| <code>interface.9.env-layerx-agent-human-session-operator-secret-file</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE</code> |
| <code>interface.9.env-layerx-agent-human-socket</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SOCKET</code> |
| <code>interface.9.env-layerx-agent-human-socket-gid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SOCKET_GID</code> |
| <code>interface.9.env-layerx-agent-human-socket-mode</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SOCKET_MODE</code> |
| <code>interface.9.env-layerx-agent-human-socket-uid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_SOCKET_UID</code> |
| <code>interface.9.env-layerx-agent-human-store</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_STORE</code> |
| <code>interface.9.env-layerx-agent-human-timestamp-span</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN</code> |
| <code>interface.9.env-layerx-agent-mcp-activity-types</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_ACTIVITY_TYPES</code> |
| <code>interface.9.env-layerx-agent-mcp-agent-did</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_AGENT_DID</code> |
| <code>interface.9.env-layerx-agent-mcp-audit-root</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_AUDIT_ROOT</code> |
| <code>interface.9.env-layerx-agent-mcp-capability-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_CAPABILITY_ID</code> |
| <code>interface.9.env-layerx-agent-mcp-deadline-ms</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_DEADLINE_MS</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-ceiling</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_CEILING</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-consumed</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_CONSUMED</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_ID</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-name</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_NAME</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-scope</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_SCOPE</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-scope-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LIMIT_SCOPE_ID</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-admitted-uids</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-mode</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LISTENER_MODE</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-owner-gid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LISTENER_OWNER_GID</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-owner-uid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LISTENER_OWNER_UID</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-socket</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_LISTENER_SOCKET</code> |
| <code>interface.9.env-layerx-agent-mcp-mode</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_MODE</code> |
| <code>interface.9.env-layerx-agent-mcp-opening-client</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_OPENING_CLIENT</code> |
| <code>interface.9.env-layerx-agent-mcp-peer-uid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_PEER_UID</code> |
| <code>interface.9.env-layerx-agent-mcp-policy-version</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_POLICY_VERSION</code> |
| <code>interface.9.env-layerx-agent-mcp-scopes</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_SCOPES</code> |
| <code>interface.9.env-layerx-agent-mcp-session-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_SESSION_ID</code> |
| <code>interface.9.env-layerx-agent-mcp-web-approval-threshold</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD</code> |
| <code>interface.9.env-layerx-agent-mcp-web-endpoint</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_WEB_ENDPOINT</code> |
| <code>interface.9.env-layerx-agent-mcp-web-network</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_WEB_NETWORK</code> |
| <code>interface.9.env-layerx-agent-mcp-web-pending-attempts</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS</code> |
| <code>interface.9.env-layerx-agent-mcp-web-timeout-ms</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_WEB_TIMEOUT_MS</code> |
| <code>interface.9.env-layerx-agent-mode</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs</code> | <code>LAYERX_AGENT_MODE</code> |
| <code>interface.9.env-layerx-agent-node-bearer-token</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_NODE_BEARER_TOKEN</code> |
| <code>interface.9.env-layerx-agent-node-chain-id-j43</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/node_rpc.rs</code> | <code>LAYERX_AGENT_NODE_CHAIN_ID</code> |
| <code>interface.9.env-layerx-agent-node-endpoint</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_NODE_ENDPOINT</code> |
| <code>interface.9.env-layerx-agent-node-rpc-url-j43</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/node_rpc.rs</code> | <code>LAYERX_AGENT_NODE_RPC_URL</code> |
| <code>interface.9.env-layerx-agent-program-max-staleness-ms</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS</code> |
| <code>interface.9.env-layerx-agent-program-probe-id</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_PROGRAM_PROBE_ID</code> |
| <code>interface.9.env-layerx-agent-registry-source-config</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_REGISTRY_SOURCE_CONFIG</code> |
| <code>interface.9.env-layerx-agentd-mcp-bindings</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_MCP_BINDINGS</code> |
| <code>interface.9.env-layerx-agentd-mcp-payers</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_MCP_PAYERS</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-daemon-sequences</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-protocol-sequences</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-root</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT</code> |
| <code>interface.9.env-layerx-agentd-rpc-listen</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_LISTEN</code> |
| <code>interface.9.env-layerx-agentd-rpc-peer</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_PEER</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-cert</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_TLS_CERT</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-client-ca</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_TLS_CLIENT_CA</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-key</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENTD_RPC_TLS_KEY</code> |
| <code>interface.9.env-layerx-native-effect-policy-sources</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_NATIVE_EFFECT_POLICY_SOURCES</code> |
| <code>interface.9.env-layerx-native-policy-sources</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_NATIVE_POLICY_SOURCES</code> |
| <code>interface.9.env-layerx-node-network-name</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_NODE_NETWORK_NAME</code> |
| <code>interface.9.env-layerx-policy-sources</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_POLICY_SOURCES</code> |
| <code>interface.9.env-layerx-program-budget-denomination-sources</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES</code> |
| <code>interface.9.env-layerx-tls-qual-ca-der</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/outbound_tls.rs</code> | <code>LAYERX_TLS_QUAL_CA_DER</code> |
| <code>interface.9.env-native-send-daemon-canonical-output</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_wire.rs</code> | <code>NATIVE_SEND_DAEMON_CANONICAL_OUTPUT</code> |
| <code>interface.9.env-layerx-agent-human-authority-bearer</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_AUTHORITY_BEARER</code> |
| <code>interface.9.env-layerx-agent-human-deadline-ms</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_HUMAN_DEADLINE_MS</code> |
| <code>interface.9.env-layerx-agent-program-bearer-token</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_PROGRAM_BEARER_TOKEN</code> |
| <code>interface.9.env-layerx-agent-program-listen</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_PROGRAM_LISTEN</code> |
| <code>interface.9.env-layerx-agent-mcp-binding-root</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>LAYERX_AGENT_MCP_BINDING_ROOT</code> |
| <code>interface.9.env-layerx-agent-human-max-connections</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>LAYERX_AGENT_HUMAN_MAX_CONNECTIONS</code> |
| <code>interface.9.env-layerx-agent-human-max-frame-bytes</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES</code> |
| <code>interface.9.env-layerx-agent-human-max-queued-bytes</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES</code> |
| <code>interface.9.env-layerx-agent-human-max-streams</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>LAYERX_AGENT_HUMAN_MAX_STREAMS</code> |
| <code>interface.9.env-layerx-agent-human-protocol-version</code> | <code>env_var</code> | <code>services/agent/crates/layerx-agentd/src/node_rpc.rs</code> | <code>LAYERX_AGENT_HUMAN_PROTOCOL_VERSION</code> |

| Field | Rule |
|---|---|
| interface | the agent daemon's node JSON-RPC client over LAYERX_AGENT_NODE_RPC_URL |
| storage | none beyond the agentd store |
| errors | `NodeRpcError` distinguishes MissingVariable, InvalidVariable, InvalidDeadline, ChainIdMismatch, Unavailable and Sequencer |
| events | the synchronous `NodeRpc` client performs request/response reads |
| gas | none |
| invariants | eth_chainId equals LAYERX_AGENT_NODE_CHAIN_ID before serving |
| pause | none |

| Requirement | Behavior |
| --- | --- |
| <code>req.9003</code> | Starting layerx-agentd in human-owner mode with LAYERX_AGENT_HUMAN_NODE_LNI set exits nonzero naming that variable as retired.|
| <code>req.9040</code> | NodeRpcConfig::from_environment refuses an environment missing LAYERX_AGENT_NODE_RPC_URL or LAYERX_AGENT_NODE_CHAIN_ID by name. |
| <code>req.9044</code> | Human-owner mode reports LAYERX_AGENT_NODE_RPC_URL and LAYERX_AGENT_NODE_CHAIN_ID as its node variables and no LNI variable (services/agent/crates/layerx-agentd/src/main.rs). |
| <code>req.9045</code> | Starting the binary with LAYERX_AGENT_GENESIS_TRUST, LAYERX_AGENT_HANDOVER_FINALITY, LAYERX_AGENT_SEQUENCER_TRUST_HISTORY or LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN set exits nonzero naming the variable. |


## agentd-startup

Startup requires these eight file keys or their corresponding environment
variables. Exact environment values override the explicitly named file.

| File key | Environment variable |
| --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` |
| `tenants` | `LAYERX_TENANTS` |
| `policy_sources` | `LAYERX_POLICY_SOURCES` |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` |

`receipt_key_source` accepts `px_getReceiptKeys` for node-provided keys or a
pinned-file path. `expected_protocol_version` and
`sequencer_authority_source`, including their environment forms listed below,
are retired inputs and are rejected.

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-startup-expected-protocol-version</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>expected_protocol_version (file key) / LAYERX_EXPECTED_PROTOCOL_VERSION (environment) (retired; rejected)</code> |
| <code>interface.9.cfg-agentd-startup-network-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>network_id (file key) / LAYERX_NETWORK_ID (environment)</code> |
| <code>interface.9.cfg-agentd-startup-node-endpoint</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>node_endpoint (file key) / LAYERX_NODE_ENDPOINT (environment)</code> |
| <code>interface.9.cfg-agentd-startup-policy-sources</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>policy_sources (file key) / LAYERX_POLICY_SOURCES (environment)</code> |
| <code>interface.9.cfg-agentd-startup-sequencer-authority-source</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>sequencer_authority_source (file key) / LAYERX_SEQUENCER_AUTHORITY_SOURCE (environment) (retired; rejected)</code> |
| <code>interface.9.cfg-agentd-startup-signer-configurations</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>signer_configurations (file key) / LAYERX_SIGNER_CONFIGURATIONS (environment)</code> |
| <code>interface.9.cfg-agentd-startup-tenants</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>tenants (file key) / LAYERX_TENANTS (environment)</code> |
| <code>interface.9.cfg-agentd-startup-verification-defaults</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>verification_defaults (file key) / LAYERX_VERIFICATION_DEFAULTS (environment)</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9003</code> | With LAYERX_AGENT_NODE_RPC_URL naming a closed loopback port the binary exits nonzero with `node JSON-RPC is unavailable` rather than the LNI error at services/agent/crates/layerx-agentd/src/main.rs. |
| <code>req.9040</code> | NodeRpc::connect against a closed port returns Unavailable within the configured deadline. |
| <code>req.9041</code> | The readiness gate stays NotReady when px_getReceiptKeys returns an empty set. |
| <code>req.9041</code> | compat::verify accepts contract version 2 and refuses 1 (services/agent/crates/layerx-agentd/src/compat.rs). |
| <code>req.9042</code> | validate refuses a node_endpoint that is a filesystem path and accepts an https URL. |
| <code>req.9042</code> | A configuration naming expected_protocol_version is refused with RejectionReason::Retired naming that key. |

## harness-env

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.env-anvil-bin</code> | <code>env_var</code> | <code>services/agent/sdk/typescript/test/gas-station.test.ts</code> | <code>ANVIL_BIN</code> |
| <code>interface.9.env-build-dir</code> | <code>env_var</code> | <code>services/agent/tools/run-real-node-tests.sh</code> | <code>LAYERX_TEST_NATIVE_BUILD_DIR</code> |
| <code>interface.9.env-cargo-target-dir</code> | <code>env_var</code> | <code>services/agent/tools/run-real-node-tests.sh</code> | <code>CARGO_TARGET_DIR</code> |
| <code>interface.9.env-goflags</code> | <code>env_var</code> | <code>services/agent/tools/run-real-node-tests.sh</code> | <code>GOMAXPROCS</code> |
| <code>interface.9.env-layerx-agent-tsan-target-dir</code> | <code>env_var</code> | <code>services/agent/tools/run-sanitizers.sh</code> | <code>LAYERX_AGENT_TSAN_TARGET_DIR</code> |
| <code>interface.9.env-layerx-qualify-node-rpc-url-j43</code> | <code>env_var</code> | <code>services/agent/tests/boundary/src/production.rs</code> | <code>LAYERX_QUALIFY_NODE_RPC_URL</code> |
| <code>interface.9.env-layerx-runtime-clock-pid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs</code> | <code>LAYERX_RUNTIME_CLOCK_PID</code> |
| <code>interface.9.env-layerx-runtime-clock-socket</code> | <code>env_var</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs</code> | <code>LAYERX_RUNTIME_CLOCK_SOCKET</code> |
| <code>interface.9.env-layerx-runtime-clock-uid</code> | <code>env_var</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs</code> | <code>LAYERX_RUNTIME_CLOCK_UID</code> |
| <code>interface.9.env-layerx-test-native-build-dir</code> | <code>env_var</code> | <code>services/agent/tools/run-real-node-tests.sh</code> | <code>LAYERX_TEST_NATIVE_BUILD_DIR</code> |
| <code>interface.9.env-layerx-test-sanitizer</code> | <code>env_var</code> | <code>services/agent/tests/support/rustc.rs</code> | <code>LAYERX_TEST_SANITIZER</code> |
| <code>interface.9.env-native-effect-cross-language-request</code> | <code>env_var</code> | <code>services/agent/sdk/typescript/test/native-effect.test.ts</code> | <code>NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST</code> |
| <code>interface.9.env-native-effect-rust-canonical-output</code> | <code>env_var</code> | <code>services/agent/crates/layerx-sdk/src/native_effect.rs</code> | <code>NATIVE_EFFECT_RUST_CANONICAL_OUTPUT</code> |
| <code>interface.9.env-native-effect-typescript-canonical-output</code> | <code>env_var</code> | <code>services/agent/sdk/typescript/test/native-effect.test.ts</code> | <code>NATIVE_EFFECT_TYPESCRIPT_CANONICAL_OUTPUT</code> |
| <code>interface.9.env-native-send-rust-canonical-output</code> | <code>env_var</code> | <code>services/agent/crates/layerx-sdk/src/native_effect.rs</code> | <code>NATIVE_SEND_RUST_CANONICAL_OUTPUT</code> |
| <code>interface.9.env-paxeer-x-agent-envelope-case</code> | <code>env_var</code> | <code>services/agent/sdk/typescript/test/agent-operation-envelope.test.ts</code> | <code>PAXEER_X_AGENT_ENVELOPE_CASE</code> |
| <code>interface.9.env-paxeer-x-program-terminal-v5-corpus</code> | <code>env_var</code> | <code>services/agent/crates/layerx-sdk/src/program_http.rs</code> | <code>PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS</code> |

## human-peers

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-human-peers-principal</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field principal= (entries comma separated, fields semicolon separated)</code> |
| <code>interface.9.cfg-agentd-human-peers-tenant</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field tenant= (entries comma separated, fields semicolon separated)</code> |
| <code>interface.9.cfg-agentd-human-peers-uid</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field uid= (entries comma separated, fields semicolon separated)</code> |

## mcp-binding

Bindings use `binding.core_height`; `binding.core_sequence` is rejected with
`mcp.binding_retired_key`. Web configuration uses `binding.web.receipt_key_index`
and also accepts `payer_address`, `receiver_address`, `sequencer_id`,
`first_batch_number` and `last_batch_number` within `binding.web`.
`binding.web.sequencer_public_key` is retired and rejected.

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-mcp-binding-agent</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent</code> |
| <code>interface.9.cfg-mcp-binding-agent-bearer-file</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.bearer_file</code> |
| <code>interface.9.cfg-mcp-binding-agent-endpoint</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-agent-probe-program</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.probe_program</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.readiness</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-endpoint</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.readiness.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-gateway-key-file</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.readiness.gateway_key_file</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-trust-anchors</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.agent.readiness.trust_anchors</code> |
| <code>interface.9.cfg-mcp-binding-audit-root</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.audit_root</code> |
| <code>interface.9.cfg-mcp-binding-capability-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.capability_id</code> |
| <code>interface.9.cfg-mcp-binding-core-sequence</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.core_sequence (retired; rejected)</code> |
| <code>interface.9.cfg-mcp-binding-deadline-ms</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.deadline_ms</code> |
| <code>interface.9.cfg-mcp-binding-limit</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit</code> |
| <code>interface.9.cfg-mcp-binding-limit-ceiling</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.ceiling</code> |
| <code>interface.9.cfg-mcp-binding-limit-consumed</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.consumed</code> |
| <code>interface.9.cfg-mcp-binding-limit-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.id</code> |
| <code>interface.9.cfg-mcp-binding-limit-name</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.name</code> |
| <code>interface.9.cfg-mcp-binding-limit-scope</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.scope</code> |
| <code>interface.9.cfg-mcp-binding-limit-scope-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.limit.scope_id</code> |
| <code>interface.9.cfg-mcp-binding-listener</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener</code> |
| <code>interface.9.cfg-mcp-binding-listener-admitted-uids</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener.admitted_uids</code> |
| <code>interface.9.cfg-mcp-binding-listener-mode</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener.mode</code> |
| <code>interface.9.cfg-mcp-binding-listener-owner-gid</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener.owner_gid</code> |
| <code>interface.9.cfg-mcp-binding-listener-owner-uid</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener.owner_uid</code> |
| <code>interface.9.cfg-mcp-binding-listener-socket</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.listener.socket</code> |
| <code>interface.9.cfg-mcp-binding-mode</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.mode</code> |
| <code>interface.9.cfg-mcp-binding-session-generation</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.session_generation</code> |
| <code>interface.9.cfg-mcp-binding-session-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.session_id</code> |
| <code>interface.9.cfg-mcp-binding-session-token-file</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.session_token_file</code> |
| <code>interface.9.cfg-mcp-binding-store</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.store</code> |
| <code>interface.9.cfg-mcp-binding-tenant</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.tenant</code> |
| <code>interface.9.cfg-mcp-binding-web</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web</code> |
| <code>interface.9.cfg-mcp-binding-web-approval-threshold</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.approval_threshold</code> |
| <code>interface.9.cfg-mcp-binding-web-endpoint</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-web-network</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.network</code> |
| <code>interface.9.cfg-mcp-binding-web-pending-attempts</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.pending_attempts</code> |
| <code>interface.9.cfg-mcp-binding-web-sequencer-public-key</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.sequencer_public_key (retired; rejected)</code> |
| <code>interface.9.cfg-mcp-binding-web-timeout-ms</code> | <code>config_key</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>binding.web.timeout_ms</code> |

## native-effect-policy-source

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-native-effect-policy-source-policy</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-effect-policy-source.v1 source: policy</code> |
| <code>interface.9.cfg-agentd-native-effect-policy-source-tenant</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-effect-policy-source.v1 source: tenant</code> |
| <code>interface.9.cfg-agentd-native-effect-policy-source-version</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-effect-policy-source.v1 source: version</code> |

## native-program-policy-source

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-native-program-policy-source-policy</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-program-policy-source.v1 source: policy</code> |
| <code>interface.9.cfg-agentd-native-program-policy-source-tenant</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-program-policy-source.v1 source: tenant</code> |
| <code>interface.9.cfg-agentd-native-program-policy-source-version</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>layerx.native-program-policy-source.v1 source: version</code> |

## program-budget-denominations

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-program-budget-denominations-asset</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: asset</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-limits</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: limits</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-source</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: source</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-stable-id</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: stable_id</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-tenant</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: tenant</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-version</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/config.rs</code> | <code>program budget denominations source: version</code> |

## registry-source

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.cfg-agentd-registry-source-client-certificate-pem</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: client_certificate_pem</code> |
| <code>interface.9.cfg-agentd-registry-source-client-private-key-pem</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: client_private_key_pem</code> |
| <code>interface.9.cfg-agentd-registry-source-deadline-ms</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: deadline_ms</code> |
| <code>interface.9.cfg-agentd-registry-source-endpoint</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: endpoint</code> |
| <code>interface.9.cfg-agentd-registry-source-maximum-response-bytes</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: maximum_response_bytes</code> |
| <code>interface.9.cfg-agentd-registry-source-request-token-file</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: request_token_file</code> |
| <code>interface.9.cfg-agentd-registry-source-server-ca-der</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: server_ca_der</code> |
| <code>interface.9.cfg-agentd-registry-source-version</code> | <code>config_key</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs</code> | <code>registry source configuration: version</code> |
