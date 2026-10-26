# Agent config contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## agentd-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.env-layerx-agent-authority-bearer-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1622</code> | <code>let authority_bearer = required("LAYERX_AGENT_AUTHORITY_BEARER_TOKEN")?;</code> |
| <code>interface.9.env-layerx-agent-authority-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1656</code> | <code>authority_ca_der: read_ca("LAYERX_AGENT_AUTHORITY_CA_DER")?,</code> |
| <code>interface.9.env-layerx-agent-authority-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1654</code> | <code>authority_endpoint: required("LAYERX_AGENT_AUTHORITY_ENDPOINT")?,</code> |
| <code>interface.9.env-layerx-agent-authority-replica-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1657</code> | <code>authority_replica_id: parse_digest("LAYERX_AGENT_AUTHORITY_REPLICA_ID")?,</code> |
| <code>interface.9.env-layerx-agent-deployment-journal</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1660</code> | <code>deployment_journal: required("LAYERX_AGENT_DEPLOYMENT_JOURNAL")?,</code> |
| <code>interface.9.env-layerx-agent-execution-prestate</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:677</code> | <code>let execution_prestate = match env::var("LAYERX_AGENT_EXECUTION_PRESTATE") {</code> |
| <code>interface.9.env-layerx-agent-genesis-trust</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1933</code> | <code>let genesis_trust = match std::env::var("LAYERX_AGENT_GENESIS_TRUST") {</code> |
| <code>interface.9.env-layerx-agent-handover-finality</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1940</code> | <code>let handover_finality = match std::env::var("LAYERX_AGENT_HANDOVER_FINALITY") {</code> |
| <code>interface.9.env-layerx-agent-human-authority-bearer</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:75</code> | <code>if bearer == required("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER")? {</code> |
| <code>interface.9.env-layerx-agent-human-authority-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:760</code> | <code>&amp;read_ca("LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER")?,</code> |
| <code>interface.9.env-layerx-agent-human-authority-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:754</code> | <code>&amp;required("LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT")?,</code> |
| <code>interface.9.env-layerx-agent-human-authority-max-bytes</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:757</code> | <code>required("LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES")?</code> |
| <code>interface.9.env-layerx-agent-human-deadline-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:43</code> | <code>let deadline = Duration::from_millis(parse_u64("LAYERX_AGENT_HUMAN_DEADLINE_MS")?);</code> |
| <code>interface.9.env-layerx-agent-human-export-set-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:788</code> | <code>parse_u64("LAYERX_AGENT_HUMAN_EXPORT_SET_VERSION")?,</code> |
| <code>interface.9.env-layerx-agent-human-export-settlement-domain</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:782</code> | <code>"LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN",</code> |
| <code>interface.9.env-layerx-agent-human-limit-ceiling</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:341</code> | <code>ceiling: required("LAYERX_AGENT_HUMAN_LIMIT_CEILING")?</code> |
| <code>interface.9.env-layerx-agent-human-limit-consumed</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:344</code> | <code>consumed: required("LAYERX_AGENT_HUMAN_LIMIT_CONSUMED")?</code> |
| <code>interface.9.env-layerx-agent-human-limit-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:338</code> | <code>id: LimitId(parse_hex("LAYERX_AGENT_HUMAN_LIMIT_ID")?),</code> |
| <code>interface.9.env-layerx-agent-human-limit-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:339</code> | <code>name: required("LAYERX_AGENT_HUMAN_LIMIT_NAME")?,</code> |
| <code>interface.9.env-layerx-agent-human-limit-scope</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:329</code> | <code>let scope = match required("LAYERX_AGENT_HUMAN_LIMIT_SCOPE")?.as_str() {</code> |
| <code>interface.9.env-layerx-agent-human-limit-scope-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:328</code> | <code>let scope_bytes = parse_hex("LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID")?;</code> |
| <code>interface.9.env-layerx-agent-human-max-connections</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:661</code> | <code>maximum_connections: required("LAYERX_AGENT_HUMAN_MAX_CONNECTIONS")?</code> |
| <code>interface.9.env-layerx-agent-human-max-frame-bytes</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:658</code> | <code>maximum_frame_bytes: required("LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES")?</code> |
| <code>interface.9.env-layerx-agent-human-max-payload-bytes</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:894</code> | <code>required("LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES")?</code> |
| <code>interface.9.env-layerx-agent-human-max-queued-bytes</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:667</code> | <code>maximum_queued_bytes: required("LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES")?</code> |
| <code>interface.9.env-layerx-agent-human-max-streams</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:664</code> | <code>maximum_streams: required("LAYERX_AGENT_HUMAN_MAX_STREAMS")?</code> |
| <code>interface.9.env-layerx-agent-human-network-id</code> | <code>env_var</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:698</code> | <code>expected_network_id: required("LAYERX_AGENT_HUMAN_NETWORK_ID")?</code> |
| <code>interface.9.env-layerx-agent-human-node-lni</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:52</code> | <code>PathBuf::from(required("LAYERX_AGENT_HUMAN_NODE_LNI")?),</code> |
| <code>interface.9.env-layerx-agent-human-peers</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:323</code> | <code>human_peer_config::parse(&amp;required("LAYERX_AGENT_HUMAN_PEERS")?)</code> |
| <code>interface.9.env-layerx-agent-human-protocol-version</code> | <code>env_var</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:683</code> | <code>let human_protocol_version = required("LAYERX_AGENT_HUMAN_PROTOCOL_VERSION")?</code> |
| <code>interface.9.env-layerx-agent-human-reconnect-attempts</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:704</code> | <code>maximum_attempts: required("LAYERX_AGENT_HUMAN_RECONNECT_ATTEMPTS")?</code> |
| <code>interface.9.env-layerx-agent-human-reconnect-base-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:707</code> | <code>base_delay: Duration::from_millis(parse_u64("LAYERX_AGENT_HUMAN_RECONNECT_BASE_MS")?),</code> |
| <code>interface.9.env-layerx-agent-human-reconnect-jitter-percent</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:709</code> | <code>jitter_percent: required("LAYERX_AGENT_HUMAN_RECONNECT_JITTER_PERCENT")?</code> |
| <code>interface.9.env-layerx-agent-human-reconnect-max-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:708</code> | <code>maximum_delay: Duration::from_millis(parse_u64("LAYERX_AGENT_HUMAN_RECONNECT_MAX_MS")?),</code> |
| <code>interface.9.env-layerx-agent-human-session-key-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:862</code> | <code>let session_key_root = PathBuf::from(required("LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT")?);</code> |
| <code>interface.9.env-layerx-agent-human-session-operator-secret-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:864</code> | <code>PathBuf::from(required("LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE")?);</code> |
| <code>interface.9.env-layerx-agent-human-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:861</code> | <code>let socket_path = PathBuf::from(required("LAYERX_AGENT_HUMAN_SOCKET")?);</code> |
| <code>interface.9.env-layerx-agent-human-socket-gid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:956</code> | <code>owner_gid: required("LAYERX_AGENT_HUMAN_SOCKET_GID")?</code> |
| <code>interface.9.env-layerx-agent-human-socket-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:959</code> | <code>mode: u32::from_str_radix(&amp;required("LAYERX_AGENT_HUMAN_SOCKET_MODE")?, 8)</code> |
| <code>interface.9.env-layerx-agent-human-socket-uid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:922</code> | <code>let socket_uid = required("LAYERX_AGENT_HUMAN_SOCKET_UID")?</code> |
| <code>interface.9.env-layerx-agent-human-store</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:860</code> | <code>let store_path = PathBuf::from(required("LAYERX_AGENT_HUMAN_STORE")?);</code> |
| <code>interface.9.env-layerx-agent-human-timestamp-span</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:897</code> | <code>parse_u64("LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-activity-types</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:385</code> | <code>for entry in required("LAYERX_AGENT_MCP_ACTIVITY_TYPES")?.split(',') {</code> |
| <code>interface.9.env-layerx-agent-mcp-agent-did</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:515</code> | <code>did: Did::new(required("LAYERX_AGENT_MCP_AGENT_DID")?.as_bytes())</code> |
| <code>interface.9.env-layerx-agent-mcp-audit-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:513</code> | <code>audit_root: absolute_path("LAYERX_AGENT_MCP_AUDIT_ROOT")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-binding-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:78</code> | <code>if optional("LAYERX_AGENT_MCP_BINDING_ROOT").is_some() {</code> |
| <code>interface.9.env-layerx-agent-mcp-capability-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:519</code> | <code>capability_id: CapabilityId(parse_digest("LAYERX_AGENT_MCP_CAPABILITY_ID")?),</code> |
| <code>interface.9.env-layerx-agent-mcp-core-sequence</code> | <code>env_var</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:525</code> | <code>core_sequence: parse_u64("LAYERX_AGENT_MCP_CORE_SEQUENCE")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-deadline-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:528</code> | <code>deadline: Duration::from_millis(parse_u64("LAYERX_AGENT_MCP_DEADLINE_MS")?),</code> |
| <code>interface.9.env-layerx-agent-mcp-expiry-sequence</code> | <code>env_var</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:522</code> | <code>expiry_sequence: parse_u64("LAYERX_AGENT_MCP_EXPIRY_SEQUENCE")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-ceiling</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:425</code> | <code>ceiling: required("LAYERX_AGENT_MCP_LIMIT_CEILING")?</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-consumed</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:428</code> | <code>consumed: required("LAYERX_AGENT_MCP_LIMIT_CONSUMED")?</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:422</code> | <code>id: LimitId(parse_hex("LAYERX_AGENT_MCP_LIMIT_ID")?),</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:423</code> | <code>name: required("LAYERX_AGENT_MCP_LIMIT_NAME")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-scope</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:413</code> | <code>let scope = match required("LAYERX_AGENT_MCP_LIMIT_SCOPE")?.as_str() {</code> |
| <code>interface.9.env-layerx-agent-mcp-limit-scope-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:412</code> | <code>let scope_bytes = parse_hex("LAYERX_AGENT_MCP_LIMIT_SCOPE_ID")?;</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-admitted-uids</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:436</code> | <code>for entry in required("LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS")?.split(',') {</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:467</code> | <code>u32::from_str_radix(&amp;required("LAYERX_AGENT_MCP_LISTENER_MODE")?, 8)</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-owner-gid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:464</code> | <code>required("LAYERX_AGENT_MCP_LISTENER_OWNER_GID")?</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-owner-uid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:461</code> | <code>required("LAYERX_AGENT_MCP_LISTENER_OWNER_UID")?</code> |
| <code>interface.9.env-layerx-agent-mcp-listener-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:460</code> | <code>absolute_path("LAYERX_AGENT_MCP_LISTENER_SOCKET")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:503</code> | <code>let mode = match required("LAYERX_AGENT_MCP_MODE")?.as_str() {</code> |
| <code>interface.9.env-layerx-agent-mcp-opening-client</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:523</code> | <code>opening_client: required("LAYERX_AGENT_MCP_OPENING_CLIENT")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-peer-uid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:508</code> | <code>let peer_uid = required("LAYERX_AGENT_MCP_PEER_UID")?</code> |
| <code>interface.9.env-layerx-agent-mcp-policy-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:524</code> | <code>policy_version: required("LAYERX_AGENT_MCP_POLICY_VERSION")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-scopes</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:399</code> | <code>for entry in required("LAYERX_AGENT_MCP_SCOPES")?.split(',') {</code> |
| <code>interface.9.env-layerx-agent-mcp-session-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:518</code> | <code>session_id: SessionId(parse_digest("LAYERX_AGENT_MCP_SESSION_ID")?),</code> |
| <code>interface.9.env-layerx-agent-mcp-web-approval-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:488</code> | <code>required("LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD")?</code> |
| <code>interface.9.env-layerx-agent-mcp-web-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:477</code> | <code>let Some(endpoint) = optional("LAYERX_AGENT_MCP_WEB_ENDPOINT") else {</code> |
| <code>interface.9.env-layerx-agent-mcp-web-network</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:482</code> | <code>required("LAYERX_AGENT_MCP_WEB_NETWORK")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-web-pending-attempts</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:485</code> | <code>required("LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS")?</code> |
| <code>interface.9.env-layerx-agent-mcp-web-sequencer-public-key</code> | <code>env_var</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:483</code> | <code>parse_digest("LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY")?,</code> |
| <code>interface.9.env-layerx-agent-mcp-web-timeout-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:484</code> | <code>parse_u64("LAYERX_AGENT_MCP_WEB_TIMEOUT_MS")?,</code> |
| <code>interface.9.env-layerx-agent-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:31</code> | <code>let mode = match env::var("LAYERX_AGENT_MODE") {</code> |
| <code>interface.9.env-layerx-agent-node-bearer-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1621</code> | <code>let node_bearer = required("LAYERX_AGENT_NODE_BEARER_TOKEN")?;</code> |
| <code>interface.9.env-layerx-agent-node-chain-id-j43</code> | <code>env_var</code> | <code>new</code> | <code>services/agent/crates/layerx-agentd/src/node_rpc.rs</code> (planned) | <code>LAYERX_AGENT_NODE_CHAIN_ID</code> |
| <code>interface.9.env-layerx-agent-node-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1652</code> | <code>node_endpoint: required("LAYERX_AGENT_NODE_ENDPOINT")?,</code> |
| <code>interface.9.env-layerx-agent-node-rpc-url-j43</code> | <code>env_var</code> | <code>new</code> | <code>services/agent/crates/layerx-agentd/src/node_rpc.rs</code> (planned) | <code>LAYERX_AGENT_NODE_RPC_URL</code> |
| <code>interface.9.env-layerx-agent-program-bearer-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:71</code> | <code>let bearer = required("LAYERX_AGENT_PROGRAM_BEARER_TOKEN")?;</code> |
| <code>interface.9.env-layerx-agent-program-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:70</code> | <code>let listen = required("LAYERX_AGENT_PROGRAM_LISTEN")?;</code> |
| <code>interface.9.env-layerx-agent-program-max-staleness-ms</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1634</code> | <code>let staleness_ms = parse_u64("LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS")?;</code> |
| <code>interface.9.env-layerx-agent-program-probe-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1661</code> | <code>probe_program: ProgramId::new(parse_digest("LAYERX_AGENT_PROGRAM_PROBE_ID")?)</code> |
| <code>interface.9.env-layerx-agent-registry-source-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:735</code> | <code>let path = match env::var_os("LAYERX_AGENT_REGISTRY_SOURCE_CONFIG") {</code> |
| <code>interface.9.env-layerx-agent-sequencer-trust-history</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1658</code> | <code>sequencer_trust_history: required("LAYERX_AGENT_SEQUENCER_TRUST_HISTORY")?,</code> |
| <code>interface.9.env-layerx-agentd-mcp-bindings</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1015</code> | <code>let mut mcp_bindings = optional("LAYERX_AGENTD_MCP_BINDINGS")</code> |
| <code>interface.9.env-layerx-agentd-mcp-payers</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1092</code> | <code>let Some(source) = optional("LAYERX_AGENTD_MCP_PAYERS") else {</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-daemon-sequences</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:994</code> | <code>parse_u64("LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-protocol-sequences</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:995</code> | <code>parse_u64("LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-idempotency-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:993</code> | <code>absolute_path("LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:988</code> | <code>let Some(listen) = optional("LAYERX_AGENTD_RPC_LISTEN") else {</code> |
| <code>interface.9.env-layerx-agentd-rpc-peer</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1002</code> | <code>peer: required("LAYERX_AGENTD_RPC_PEER")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-cert</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:999</code> | <code>cert: absolute_path("LAYERX_AGENTD_RPC_TLS_CERT")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-client-ca</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1001</code> | <code>client_ca: absolute_path("LAYERX_AGENTD_RPC_TLS_CLIENT_CA")?,</code> |
| <code>interface.9.env-layerx-agentd-rpc-tls-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1000</code> | <code>key: absolute_path("LAYERX_AGENTD_RPC_TLS_KEY")?,</code> |
| <code>interface.9.env-layerx-native-effect-policy-sources</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1573</code> | <code>const SETTING: &amp;str = "LAYERX_NATIVE_EFFECT_POLICY_SOURCES";</code> |
| <code>interface.9.env-layerx-native-policy-sources</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1566</code> | <code>layerx_agentd::config::parse_policy_sources(&amp;required("LAYERX_NATIVE_POLICY_SOURCES")?, tenants)</code> |
| <code>interface.9.env-layerx-node-network-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1005</code> | <code>let network = required("LAYERX_NODE_NETWORK_NAME")?;</code> |
| <code>interface.9.env-layerx-policy-sources</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1640</code> | <code>layerx_agentd::config::parse_policy_sources(&amp;required("LAYERX_POLICY_SOURCES")?, &amp;tenants)</code> |
| <code>interface.9.env-layerx-program-budget-denomination-sources</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1597</code> | <code>const SETTING: &amp;str = "LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES";</code> |
| <code>interface.9.env-layerx-test-owner-authority-asset</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_runtime.rs:19960</code> | <code>let asset = std::env::var("LAYERX_TEST_OWNER_AUTHORITY_ASSET")</code> |
| <code>interface.9.env-layerx-test-owner-authority-did</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_runtime.rs:19947</code> | <code>let actor = std::env::var("LAYERX_TEST_OWNER_AUTHORITY_DID")</code> |
| <code>interface.9.env-layerx-test-owner-authority-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_runtime.rs:19952</code> | <code>let key_file = std::env::var("LAYERX_TEST_OWNER_AUTHORITY_KEY_FILE")</code> |
| <code>interface.9.env-layerx-test-owner-authority-public</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_runtime.rs:19945</code> | <code>let public = std::env::var("LAYERX_TEST_OWNER_AUTHORITY_PUBLIC")</code> |
| <code>interface.9.env-layerx-test-owner-authority-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_runtime.rs:19942</code> | <code>let Ok(socket) = std::env::var("LAYERX_TEST_OWNER_AUTHORITY_SOCKET") else {</code> |
| <code>interface.9.env-layerx-tls-qual-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/outbound_tls.rs:52</code> | <code>std::env::var("LAYERX_TLS_QUAL_CA_DER").map_err(&#124;error&#124; error.to_string())?,</code> |
| <code>interface.9.env-native-send-daemon-canonical-output</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_wire.rs:3189</code> | <code>if let Ok(path) = std::env::var("NATIVE_SEND_DAEMON_CANONICAL_OUTPUT") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.env-layerx-agent-genesis-trust</code> | notes | Feeds native handover genesis trust; the design deletes handover finality (stale task 6.6.11). |
| <code>interface.9.env-layerx-agent-handover-finality</code> | notes | Feeds the handover finality policy; the design deletes handover finality (stale task 6.6.11). |
| <code>interface.9.env-layerx-agent-human-authority-bearer</code> | notes | Also read at services/agent/crates/layerx-agentd/src/main.rs:755. |
| <code>interface.9.env-layerx-agent-human-deadline-ms</code> | notes | Also read at services/agent/crates/layerx-agentd/src/main.rs:851. |
| <code>interface.9.env-layerx-agent-human-export-set-version</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-export-settlement-domain</code> | notes | Selects the export settlement domain; the design deletes the settlement domain (stale task 6.6.12). No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-max-connections</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-max-frame-bytes</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-max-payload-bytes</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-max-queued-bytes</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-max-streams</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-network-id</code> | notes | Kernel network id for the LNI link; stale task 11.5.15 re-targets main.rs off the LNI transport. Also read at services/agent/crates/layerx-agentd/src/main.rs:930. |
| <code>interface.9.env-layerx-agent-human-node-lni</code> | notes | Names the node LNI socket; the design deletes the LNI transport (stale tasks 6.7.11 and 11.7.29). Also read at services/agent/crates/layerx-agentd/src/main.rs:859, services/agent/crates/layerx-agentd/src/main.rs:2047. |
| <code>interface.9.env-layerx-agent-human-protocol-version</code> | notes | Kernel LNI protocol version for the human peer link; stale task 11.5.15 re-targets main.rs off the LNI transport. |
| <code>interface.9.env-layerx-agent-human-reconnect-attempts</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-reconnect-base-ms</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-reconnect-jitter-percent</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-reconnect-max-ms</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-socket-gid</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-socket-mode</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-human-timestamp-span</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-mcp-binding-root</code> | notes | Also read at services/agent/crates/layerx-agentd/src/main.rs:500, services/agent/crates/layerx-agentd/src/main.rs:512. |
| <code>interface.9.env-layerx-agent-mcp-core-sequence</code> | notes | Carries a kernel core sequence; the design uses block heights (services/agent/schema/agent-api/read.kvx:13). |
| <code>interface.9.env-layerx-agent-mcp-expiry-sequence</code> | notes | Carries a kernel sequence expiry; the design uses block heights (services/agent/schema/agent-api/read.kvx:13). |
| <code>interface.9.env-layerx-agent-mcp-limit-name</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-mcp-limit-scope-id</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-agent-mcp-web-sequencer-public-key</code> | notes | Single pinned sequencer key; the design verifies receipts against the ReceiptKeys list (services/agent/schema/agent-api/read.kvx:85). |
| <code>interface.9.env-layerx-agent-node-chain-id-j43</code> | notes | planned by task 9.12.1; decimal EIP-155 chain id checked against eth_chainId |
| <code>interface.9.env-layerx-agent-node-rpc-url-j43</code> | notes | planned by task 9.12.1; http or https URL of the node JSON-RPC endpoint |
| <code>interface.9.env-layerx-agent-program-bearer-token</code> | notes | Also read at services/agent/crates/layerx-agentd/src/main.rs:1620. |
| <code>interface.9.env-layerx-agent-program-listen</code> | notes | Also read at services/agent/crates/layerx-agentd/src/main.rs:1619. |
| <code>interface.9.env-layerx-agent-sequencer-trust-history</code> | notes | Points at sequencer trust history; the design deletes handover finality and sequencer trust history (stale task 6.6.11). |
| <code>interface.9.env-layerx-native-effect-policy-sources</code> | notes | No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-test-owner-authority-asset</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-test-owner-authority-did</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-test-owner-authority-key-file</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-test-owner-authority-public</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-test-owner-authority-socket</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-tls-qual-ca-der</code> | notes | Also read at services/agent/crates/layerx-sdk/src/rpc.rs:67, services/agent/crates/layerx-sdk/src/rpc.rs:91. |
| <code>interface.9.env-native-send-daemon-canonical-output</code> | notes | Test or qualification harness input, not a daemon setting. No in-repo setter outside the reading code. |

Contract context <code>contract_context.9.node-jsonrpc-client</code>:

| Field | Frozen rule |
|---|---|
| interface | the agent daemon's node JSON-RPC client over LAYERX_AGENT_NODE_RPC_URL |
| storage | none beyond the agentd store |
| errors | JSON-RPC error codes are surfaced as NodeUnavailable or ProtocolIncompatibility |
| events | eth_subscribe logs |
| gas | none |
| activation | replaces the LNI connection in wave 3 |
| invariants | eth_chainId equals LAYERX_AGENT_NODE_CHAIN_ID before serving |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.3.1</code> | req | 9003.1 |
| <code>acceptance.9.3.1</code> | scenario | Given the integrated binary and an environment holding LAYERX_AGENT_HUMAN_NODE_LNI, when layerx-agentd starts with LAYERX_AGENT_MODE=human-owner, then it exits nonzero and stderr names LAYERX_AGENT_HUMAN_NODE_LNI as retired. |
| <code>acceptance.9.3.1</code> | test | services/agent/crates/layerx-agentd/host-tests/node_integration.rs::startup_refuses_the_retired_lni_variable (planned) |
| <code>acceptance.9.3.1</code> | qualification | UNRUN |
| <code>req.9003</code> | ac_1 | Starting layerx-agentd in human-owner mode with LAYERX_AGENT_HUMAN_NODE_LNI set exits nonzero naming that variable as retired, where HEAD reads it at services/agent/crates/layerx-agentd/src/human_owner_mode.rs:52. |
| <code>acceptance.9.40.3</code> | req | 9040.3 |
| <code>acceptance.9.40.3</code> | scenario | Given an environment with only LAYERX_AGENT_NODE_RPC_URL set, when from_environment runs, then the error names LAYERX_AGENT_NODE_CHAIN_ID. |
| <code>acceptance.9.40.3</code> | test | services/agent/crates/layerx-agentd/tests/node_rpc.rs::node_rpc_config_requires_both_variables (planned) |
| <code>acceptance.9.40.3</code> | qualification | UNRUN |
| <code>req.9040</code> | ac_3 | NodeRpcConfig::from_environment refuses an environment missing LAYERX_AGENT_NODE_RPC_URL or LAYERX_AGENT_NODE_CHAIN_ID by name. |
| <code>acceptance.9.44.1</code> | req | 9044.1 |
| <code>acceptance.9.44.1</code> | scenario | Given the binary started in human-owner mode with an empty environment, when it exits, then stderr names LAYERX_AGENT_NODE_RPC_URL as missing and does not mention LAYERX_AGENT_HUMAN_NODE_LNI. |
| <code>acceptance.9.44.1</code> | test | services/agent/crates/layerx-agentd/host-tests/runtime_mode.rs::human_owner_mode_lists_node_rpc_variables (planned) |
| <code>acceptance.9.44.1</code> | qualification | UNRUN |
| <code>req.9044</code> | ac_1 | Human-owner mode reports LAYERX_AGENT_NODE_RPC_URL and LAYERX_AGENT_NODE_CHAIN_ID as its node variables and no LNI variable (services/agent/crates/layerx-agentd/src/human_owner_mode.rs:52). |
| <code>acceptance.9.45.1</code> | req | 9045.1 |
| <code>acceptance.9.45.1</code> | scenario | Given the binary and each retired variable set in turn, when it starts, then it exits nonzero and stderr names that variable as retired. |
| <code>acceptance.9.45.1</code> | test | services/agent/crates/layerx-agentd/host-tests/retired_env.rs::retired_trust_variables_are_refused_by_name (planned) |
| <code>acceptance.9.45.1</code> | qualification | UNRUN |
| <code>req.9045</code> | ac_1 | Starting the binary with LAYERX_AGENT_GENESIS_TRUST, LAYERX_AGENT_HANDOVER_FINALITY, LAYERX_AGENT_SEQUENCER_TRUST_HISTORY or LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN set exits nonzero naming the variable. |
| <code>capability.9.12.5</code> | title | Remove the LNI connection from agentd startup |
| <code>capability.9.12.5</code> | reqs | 9044.1 |
| <code>capability.9.12.6</code> | title | Retire the settlement, trust-history, handover and genesis startup variables |
| <code>capability.9.12.6</code> | reqs | 9045.1 |
| <code>capability.9.20.93</code> | title | Integrate agentd startup with the node JSON-RPC endpoint end to end |
| <code>capability.9.20.93</code> | reqs | 9003.1<br>9003.2<br>9003.3 |

## agentd-startup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-startup-expected-protocol-version</code> | <code>config_key</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:39</code> | <code>expected_protocol_version (file key) / LAYERX_EXPECTED_PROTOCOL_VERSION (environment)</code> |
| <code>interface.9.cfg-agentd-startup-network-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:31</code> | <code>network_id (file key) / LAYERX_NETWORK_ID (environment)</code> |
| <code>interface.9.cfg-agentd-startup-node-endpoint</code> | <code>config_key</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:35</code> | <code>node_endpoint (file key) / LAYERX_NODE_ENDPOINT (environment)</code> |
| <code>interface.9.cfg-agentd-startup-policy-sources</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:47</code> | <code>policy_sources (file key) / LAYERX_POLICY_SOURCES (environment)</code> |
| <code>interface.9.cfg-agentd-startup-sequencer-authority-source</code> | <code>config_key</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:59</code> | <code>sequencer_authority_source (file key) / LAYERX_SEQUENCER_AUTHORITY_SOURCE (environment)</code> |
| <code>interface.9.cfg-agentd-startup-signer-configurations</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:51</code> | <code>signer_configurations (file key) / LAYERX_SIGNER_CONFIGURATIONS (environment)</code> |
| <code>interface.9.cfg-agentd-startup-tenants</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:43</code> | <code>tenants (file key) / LAYERX_TENANTS (environment)</code> |
| <code>interface.9.cfg-agentd-startup-verification-defaults</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:55</code> | <code>verification_defaults (file key) / LAYERX_VERIFICATION_DEFAULTS (environment)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-agentd-startup-expected-protocol-version</code> | notes | Startup configuration file key; the exact environment value LAYERX_EXPECTED_PROTOCOL_VERSION overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. Names the LNI node socket and protocol handshake; stale task 11.5.11 re-targets config.rs off the LNI transport. |
| <code>interface.9.cfg-agentd-startup-network-id</code> | notes | Startup configuration file key; the exact environment value LAYERX_NETWORK_ID overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. Callers list shows 52 of 346 caller files. |
| <code>interface.9.cfg-agentd-startup-node-endpoint</code> | notes | Startup configuration file key; the exact environment value LAYERX_NODE_ENDPOINT overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. Names the LNI node socket and protocol handshake; stale task 11.5.11 re-targets config.rs off the LNI transport. |
| <code>interface.9.cfg-agentd-startup-policy-sources</code> | notes | Startup configuration file key; the exact environment value LAYERX_POLICY_SOURCES overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. |
| <code>interface.9.cfg-agentd-startup-sequencer-authority-source</code> | notes | Startup configuration file key; the exact environment value LAYERX_SEQUENCER_AUTHORITY_SOURCE overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. Names the LNI node socket and protocol handshake; stale task 11.5.11 re-targets config.rs off the LNI transport. |
| <code>interface.9.cfg-agentd-startup-signer-configurations</code> | notes | Startup configuration file key; the exact environment value LAYERX_SIGNER_CONFIGURATIONS overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. |
| <code>interface.9.cfg-agentd-startup-tenants</code> | notes | Startup configuration file key; the exact environment value LAYERX_TENANTS overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. |
| <code>interface.9.cfg-agentd-startup-verification-defaults</code> | notes | Startup configuration file key; the exact environment value LAYERX_VERIFICATION_DEFAULTS overrides it (config.rs PRECEDENCE). layerx-agentd main does not call config::load at this revision; StartupConfig is built by tests and services/human test support. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.3.2</code> | req | 9003.2 |
| <code>acceptance.9.3.2</code> | scenario | Given a loopback port with no listener, when layerx-agentd starts with LAYERX_AGENT_NODE_RPC_URL pointing at it, then it exits nonzero within the startup deadline and stderr contains `node JSON-RPC is unavailable`. |
| <code>acceptance.9.3.2</code> | test | services/agent/crates/layerx-agentd/host-tests/node_integration.rs::startup_reports_node_json_rpc_unavailable_for_a_closed_endpoint (planned) |
| <code>acceptance.9.3.2</code> | qualification | UNRUN |
| <code>req.9003</code> | ac_2 | With LAYERX_AGENT_NODE_RPC_URL naming a closed loopback port the binary exits nonzero with `node JSON-RPC is unavailable` rather than the LNI error at services/agent/crates/layerx-agentd/src/main.rs:714. |
| <code>acceptance.9.3.3</code> | req | 9003.3 |
| <code>acceptance.9.3.3</code> | scenario | Given the integrated devnet node serving eth_chainId, when layerx-agentd starts with LAYERX_AGENT_NODE_CHAIN_ID set to a different value, then it exits nonzero with `node chain id mismatch` and opens no listener. |
| <code>acceptance.9.3.3</code> | test | services/agent/crates/layerx-agentd/host-tests/node_integration.rs::startup_refuses_a_chain_id_mismatch (planned) |
| <code>acceptance.9.3.3</code> | qualification | UNRUN |
| <code>req.9003</code> | ac_3 | A node whose eth_chainId differs from LAYERX_AGENT_NODE_CHAIN_ID is refused at startup with `node chain id mismatch` (planned). |
| <code>acceptance.9.40.1</code> | req | 9040.1 |
| <code>acceptance.9.40.1</code> | scenario | Given a loopback listener answering eth_chainId with 0xe5, when NodeRpc::connect runs with chain id 1, then it returns NodeRpcError::ChainIdMismatch { expected: 1, observed: 229 }. |
| <code>acceptance.9.40.1</code> | test | services/agent/crates/layerx-agentd/tests/node_rpc.rs::node_rpc_refuses_a_chain_id_mismatch (planned) |
| <code>acceptance.9.40.1</code> | qualification | UNRUN |
| <code>req.9040</code> | ac_1 | NodeRpc::connect refuses an endpoint whose eth_chainId differs from the configured chain id with ChainIdMismatch (planned). |
| <code>acceptance.9.40.2</code> | req | 9040.2 |
| <code>acceptance.9.40.2</code> | scenario | Given a loopback port with no listener and a 200 ms deadline, when NodeRpc::connect runs, then it returns NodeRpcError::Unavailable before 400 ms elapse. |
| <code>acceptance.9.40.2</code> | test | services/agent/crates/layerx-agentd/tests/node_rpc.rs::node_rpc_reports_unavailable_for_a_closed_port (planned) |
| <code>acceptance.9.40.2</code> | qualification | UNRUN |
| <code>req.9040</code> | ac_2 | NodeRpc::connect against a closed port returns Unavailable within the configured deadline. |
| <code>acceptance.9.41.1</code> | req | 9041.1 |
| <code>acceptance.9.41.1</code> | scenario | Given a node answering eth_chainId correctly and px_getReceiptKeys with an empty list, when readiness_gate runs, then the gate status is NotReady with reason node.receipt_keys_empty. |
| <code>acceptance.9.41.1</code> | test | services/agent/crates/layerx-agentd/tests/readiness_node_rpc.rs::daemon_is_not_ready_without_receipt_keys (planned) |
| <code>acceptance.9.41.1</code> | qualification | UNRUN |
| <code>req.9041</code> | ac_1 | The readiness gate stays NotReady when px_getReceiptKeys returns an empty set. |
| <code>acceptance.9.41.2</code> | req | 9041.2 |
| <code>acceptance.9.41.2</code> | scenario | Given the published matrix, when verify runs with contract version 2 and then 1, then the first passes and the second returns CompatibilityError. |
| <code>acceptance.9.41.2</code> | test | services/agent/crates/layerx-agentd/tests/readiness_node_rpc.rs::compatibility_matrix_names_contract_major_two (planned) |
| <code>acceptance.9.41.2</code> | qualification | UNRUN |
| <code>req.9041</code> | ac_2 | compat::verify accepts contract version 2 and refuses 1 (services/agent/crates/layerx-agentd/src/compat.rs:6). |
| <code>acceptance.9.42.1</code> | req | 9042.1 |
| <code>acceptance.9.42.1</code> | scenario | Given settings with node_endpoint /run/layerx.sock and then https://node.invalid/rpc, when validate runs, then the first is refused with RejectionReason::Invalid and the second yields a StartupConfig. |
| <code>acceptance.9.42.1</code> | test | services/agent/crates/layerx-agentd/tests/config_node_rpc.rs::node_endpoint_must_be_a_json_rpc_url (planned) |
| <code>acceptance.9.42.1</code> | qualification | UNRUN |
| <code>req.9042</code> | ac_1 | validate refuses a node_endpoint that is a filesystem path and accepts an https URL. |
| <code>acceptance.9.42.2</code> | req | 9042.2 |
| <code>acceptance.9.42.2</code> | scenario | Given a configuration file holding expected_protocol_version = 3, when load runs, then the error names expected_protocol_version as retired. |
| <code>acceptance.9.42.2</code> | test | services/agent/crates/layerx-agentd/tests/config_node_rpc.rs::expected_protocol_version_is_retired (planned) |
| <code>acceptance.9.42.2</code> | qualification | UNRUN |
| <code>req.9042</code> | ac_2 | A configuration naming expected_protocol_version is refused with RejectionReason::Retired naming that key. |
| <code>capability.9.12.1</code> | title | Add the agentd node JSON-RPC connection with a chain id check |
| <code>capability.9.12.1</code> | reqs | 9040.1<br>9040.2<br>9040.3 |
| <code>capability.9.12.2</code> | title | Gate daemon readiness on the node JSON-RPC connection |
| <code>capability.9.12.2</code> | reqs | 9041.1<br>9041.2 |
| <code>capability.9.12.3</code> | title | Replace the startup node settings with JSON-RPC URL and chain id |
| <code>capability.9.12.3</code> | reqs | 9042.1<br>9042.2 |

## harness-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.env-anvil-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/sdk/typescript/test/gas-station.test.ts:678</code> | <code>process.env.ANVIL_BIN ?? "anvil",</code> |
| <code>interface.9.env-build-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tools/run-real-node-tests.sh:19</code> | <code>native_build_output=${LAYERX_TEST_NATIVE_BUILD_DIR:-${BUILD_DIR:-$repo_root/build}}</code> |
| <code>interface.9.env-cargo-target-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tools/run-real-node-tests.sh:20</code> | <code>cargo_build_output=${CARGO_TARGET_DIR:-$repo_root/.lane-target}</code> |
| <code>interface.9.env-goflags</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tools/run-real-node-tests.sh:65</code> | <code>GOMAXPROCS=4 GOFLAGS="${GOFLAGS:-} -p=4" make paxeer-build</code> |
| <code>interface.9.env-layerx-agent-tsan-target-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tools/run-sanitizers.sh:18</code> | <code>tsan_target=${LAYERX_AGENT_TSAN_TARGET_DIR:-${CARGO_TARGET_DIR:-$repo_root/services/agent/target}/thread-sanitizer}</code> |
| <code>interface.9.env-layerx-c-reference</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tools/wire-differential/src/main.rs:358</code> | <code>let Some(c_reference) = std::env::var_os("LAYERX_C_REFERENCE").map(PathBuf::from) else {</code> |
| <code>interface.9.env-layerx-qualify-lni-client-gid</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:76</code> | <code>let client_gid: u32 = std::env::var("LAYERX_QUALIFY_LNI_CLIENT_GID")</code> |
| <code>interface.9.env-layerx-qualify-lni-daemon-uid</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:72</code> | <code>let daemon_uid: u32 = std::env::var("LAYERX_QUALIFY_LNI_DAEMON_UID")</code> |
| <code>interface.9.env-layerx-qualify-lni-deadline-ms</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:220</code> | <code>let deadline_ms: u64 = std::env::var("LAYERX_QUALIFY_LNI_DEADLINE_MS")</code> |
| <code>interface.9.env-layerx-qualify-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:63</code> | <code>let Some(socket) = std::env::var_os("LAYERX_QUALIFY_LNI_SOCKET") else {</code> |
| <code>interface.9.env-layerx-qualify-max-signed-activity</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:203</code> | <code>let maximum_activity_path = std::env::var_os("LAYERX_QUALIFY_MAX_SIGNED_ACTIVITY")</code> |
| <code>interface.9.env-layerx-qualify-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tests/boundary/src/production.rs:107</code> | <code>expected_network_id: std::env::var("LAYERX_QUALIFY_NETWORK_ID")</code> |
| <code>interface.9.env-layerx-qualify-node-rpc-url-j43</code> | <code>env_var</code> | <code>new</code> | <code>services/agent/tests/boundary/src/production.rs</code> (planned) | <code>LAYERX_QUALIFY_NODE_RPC_URL</code> |
| <code>interface.9.env-layerx-qualify-protocol-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tests/boundary/src/production.rs:103</code> | <code>expected_protocol_version: std::env::var("LAYERX_QUALIFY_PROTOCOL_VERSION")</code> |
| <code>interface.9.env-layerx-qualify-signed-activity</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:66</code> | <code>let activity_path = std::env::var_os("LAYERX_QUALIFY_SIGNED_ACTIVITY")</code> |
| <code>interface.9.env-layerx-qualify-unknown-signed-activity</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tests/boundary/src/production.rs:266</code> | <code>let path = std::env::var_os("LAYERX_QUALIFY_UNKNOWN_SIGNED_ACTIVITY").ok_or_else(&#124;&#124; {</code> |
| <code>interface.9.env-layerx-repository-root</code> | <code>env_var</code> | <code>retire</code> | <code>services/agent/tools/wire-differential/src/main.rs:356</code> | <code>let repository_root = std::env::var_os("LAYERX_REPOSITORY_ROOT")</code> |
| <code>interface.9.env-layerx-runtime-clock-pid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs:29</code> | <code>process: number("LAYERX_RUNTIME_CLOCK_PID")?,</code> |
| <code>interface.9.env-layerx-runtime-clock-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs:26</code> | <code>socket: std::env::var_os("LAYERX_RUNTIME_CLOCK_SOCKET")</code> |
| <code>interface.9.env-layerx-runtime-clock-uid</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-client/src/runtime_clock.rs:30</code> | <code>user: number("LAYERX_RUNTIME_CLOCK_UID")?,</code> |
| <code>interface.9.env-layerx-test-native-build-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tools/run-real-node-tests.sh:19</code> | <code>native_build_output=${LAYERX_TEST_NATIVE_BUILD_DIR:-${BUILD_DIR:-$repo_root/build}}</code> |
| <code>interface.9.env-layerx-test-sanitizer</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/tests/support/rustc.rs:6</code> | <code>if std::env::var("LAYERX_TEST_SANITIZER").as_deref() == Ok("thread") {</code> |
| <code>interface.9.env-native-effect-cross-language-request</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/sdk/typescript/test/native-effect.test.ts:100</code> | <code>const crossPath = process.env.NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST;</code> |
| <code>interface.9.env-native-effect-rust-canonical-output</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-sdk/src/native_effect.rs:297</code> | <code>if let Ok(path) = std::env::var("NATIVE_EFFECT_RUST_CANONICAL_OUTPUT") {</code> |
| <code>interface.9.env-native-effect-typescript-canonical-output</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/sdk/typescript/test/native-effect.test.ts:111</code> | <code>const output = process.env.NATIVE_EFFECT_TYPESCRIPT_CANONICAL_OUTPUT;</code> |
| <code>interface.9.env-native-send-rust-canonical-output</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-sdk/src/native_effect.rs:555</code> | <code>if let Ok(path) = std::env::var("NATIVE_SEND_RUST_CANONICAL_OUTPUT") {</code> |
| <code>interface.9.env-paxeer-x-agent-envelope-case</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/sdk/typescript/test/agent-operation-envelope.test.ts:67</code> | <code>const probePath = process.env.PAXEER_X_AGENT_ENVELOPE_CASE;</code> |
| <code>interface.9.env-paxeer-x-program-terminal-v5-corpus</code> | <code>env_var</code> | <code>stable</code> | <code>services/agent/crates/layerx-sdk/src/program_http.rs:2443</code> | <code>let path = std::env::var_os("PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.env-anvil-bin</code> | notes | Test or qualification harness input, not a daemon setting. No in-repo setter outside the reading code. |
| <code>interface.9.env-build-dir</code> | notes | Test or qualification harness input, not a daemon setting. Callers list shows 221 of 851 caller files. |
| <code>interface.9.env-cargo-target-dir</code> | notes | Test or qualification harness input, not a daemon setting. Also read at services/agent/tools/run-real-node-tests.sh:52, services/agent/tools/run-real-node-tests.sh:56, services/agent/tools/run-real-node-tests.sh:57, services/agent/tools/run-sanitizers.sh:18. |
| <code>interface.9.env-goflags</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-agent-tsan-target-dir</code> | notes | Test or qualification harness input, not a daemon setting. No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-c-reference</code> | notes | Input of the wire C differential; the design deletes it (stale task 11.7.33). Also read at services/agent/tools/wire-differential/tests/parity.rs:6. |
| <code>interface.9.env-layerx-qualify-lni-client-gid</code> | notes | Production boundary gate over the LNI socket; the design deletes the LNI transport (stale task 11.7.29). Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-qualify-lni-daemon-uid</code> | notes | Production boundary gate over the LNI socket; the design deletes the LNI transport (stale task 11.7.29). Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-qualify-lni-deadline-ms</code> | notes | Production boundary gate over the LNI socket; the design deletes the LNI transport (stale task 11.7.29). Test or qualification harness input, not a daemon setting. Also read at services/agent/tests/boundary/src/production.rs:379. |
| <code>interface.9.env-layerx-qualify-lni-socket</code> | notes | Production boundary gate over the LNI socket; the design deletes the LNI transport (stale task 11.7.29). Test or qualification harness input, not a daemon setting. Also read at services/agent/tests/boundary/src/production.rs:247. |
| <code>interface.9.env-layerx-qualify-max-signed-activity</code> | notes | Supplies a signed kernel activity; the design deletes the activity wire codec (stale task 11.7.33). Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-layerx-qualify-network-id</code> | notes | Test or qualification harness input, not a daemon setting. Also read at services/agent/tests/boundary/src/production.rs:258. |
| <code>interface.9.env-layerx-qualify-node-rpc-url-j43</code> | notes | planned by task 9.18.1 |
| <code>interface.9.env-layerx-qualify-protocol-version</code> | notes | Test or qualification harness input, not a daemon setting. Also read at services/agent/tests/boundary/src/production.rs:254. |
| <code>interface.9.env-layerx-qualify-signed-activity</code> | notes | Supplies a signed kernel activity; the design deletes the activity wire codec (stale task 11.7.33). Test or qualification harness input, not a daemon setting. Also read at services/agent/tests/boundary/src/production.rs:490. |
| <code>interface.9.env-layerx-qualify-unknown-signed-activity</code> | notes | Supplies a signed kernel activity; the design deletes the activity wire codec (stale task 11.7.33). Test or qualification harness input, not a daemon setting. No in-repo setter outside the reading code. |
| <code>interface.9.env-layerx-repository-root</code> | notes | Input of the wire C differential; the design deletes it (stale task 11.7.33). |
| <code>interface.9.env-layerx-test-native-build-dir</code> | notes | Test or qualification harness input, not a daemon setting. Also read at services/agent/tools/run-real-node-tests.sh:53, services/agent/tools/run-real-node-tests.sh:54, services/agent/tools/run-real-node-tests.sh:58, services/agent/tools/run-real-node-tests.sh:59, services/agent/tools/run-real-node-tests.sh:60, services/agent/tools/run-real-node-tests.sh:61. |
| <code>interface.9.env-layerx-test-sanitizer</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-native-effect-cross-language-request</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-native-effect-rust-canonical-output</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-native-effect-typescript-canonical-output</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-native-send-rust-canonical-output</code> | notes | Test or qualification harness input, not a daemon setting. No in-repo setter outside the reading code. |
| <code>interface.9.env-paxeer-x-agent-envelope-case</code> | notes | Test or qualification harness input, not a daemon setting. |
| <code>interface.9.env-paxeer-x-program-terminal-v5-corpus</code> | notes | Test or qualification harness input, not a daemon setting. |

## human-peers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-human-peers-principal</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs:48</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field principal= (entries comma separated, fields semicolon separated)</code> |
| <code>interface.9.cfg-agentd-human-peers-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs:45</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field tenant= (entries comma separated, fields semicolon separated)</code> |
| <code>interface.9.cfg-agentd-human-peers-uid</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_peer_config.rs:42</code> | <code>LAYERX_AGENT_HUMAN_PEERS entry field uid= (entries comma separated, fields semicolon separated)</code> |

## mcp-binding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-mcp-binding-agent</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:51</code> | <code>binding.agent</code> |
| <code>interface.9.cfg-mcp-binding-agent-bearer-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:54</code> | <code>binding.agent.bearer_file</code> |
| <code>interface.9.cfg-mcp-binding-agent-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:54</code> | <code>binding.agent.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-agent-probe-program</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:54</code> | <code>binding.agent.probe_program</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:54</code> | <code>binding.agent.readiness</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:55</code> | <code>binding.agent.readiness.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-gateway-key-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:55</code> | <code>binding.agent.readiness.gateway_key_file</code> |
| <code>interface.9.cfg-mcp-binding-agent-readiness-trust-anchors</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:55</code> | <code>binding.agent.readiness.trust_anchors</code> |
| <code>interface.9.cfg-mcp-binding-audit-root</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:44</code> | <code>binding.audit_root</code> |
| <code>interface.9.cfg-mcp-binding-capability-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:48</code> | <code>binding.capability_id</code> |
| <code>interface.9.cfg-mcp-binding-core-sequence</code> | <code>config_key</code> | <code>change</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:49</code> | <code>binding.core_sequence</code> |
| <code>interface.9.cfg-mcp-binding-deadline-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:50</code> | <code>binding.deadline_ms</code> |
| <code>interface.9.cfg-mcp-binding-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:52</code> | <code>binding.limit</code> |
| <code>interface.9.cfg-mcp-binding-limit-ceiling</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.ceiling</code> |
| <code>interface.9.cfg-mcp-binding-limit-consumed</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.consumed</code> |
| <code>interface.9.cfg-mcp-binding-limit-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.id</code> |
| <code>interface.9.cfg-mcp-binding-limit-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.name</code> |
| <code>interface.9.cfg-mcp-binding-limit-scope</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.scope</code> |
| <code>interface.9.cfg-mcp-binding-limit-scope-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:56</code> | <code>binding.limit.scope_id</code> |
| <code>interface.9.cfg-mcp-binding-listener</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:547</code> | <code>binding.listener</code> |
| <code>interface.9.cfg-mcp-binding-listener-admitted-uids</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:57</code> | <code>binding.listener.admitted_uids</code> |
| <code>interface.9.cfg-mcp-binding-listener-mode</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:57</code> | <code>binding.listener.mode</code> |
| <code>interface.9.cfg-mcp-binding-listener-owner-gid</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:57</code> | <code>binding.listener.owner_gid</code> |
| <code>interface.9.cfg-mcp-binding-listener-owner-uid</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:57</code> | <code>binding.listener.owner_uid</code> |
| <code>interface.9.cfg-mcp-binding-listener-socket</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:57</code> | <code>binding.listener.socket</code> |
| <code>interface.9.cfg-mcp-binding-mode</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:41</code> | <code>binding.mode</code> |
| <code>interface.9.cfg-mcp-binding-session-generation</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:47</code> | <code>binding.session_generation</code> |
| <code>interface.9.cfg-mcp-binding-session-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:45</code> | <code>binding.session_id</code> |
| <code>interface.9.cfg-mcp-binding-session-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:46</code> | <code>binding.session_token_file</code> |
| <code>interface.9.cfg-mcp-binding-store</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:43</code> | <code>binding.store</code> |
| <code>interface.9.cfg-mcp-binding-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:42</code> | <code>binding.tenant</code> |
| <code>interface.9.cfg-mcp-binding-web</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:548</code> | <code>binding.web</code> |
| <code>interface.9.cfg-mcp-binding-web-approval-threshold</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:64</code> | <code>binding.web.approval_threshold</code> |
| <code>interface.9.cfg-mcp-binding-web-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:59</code> | <code>binding.web.endpoint</code> |
| <code>interface.9.cfg-mcp-binding-web-network</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:60</code> | <code>binding.web.network</code> |
| <code>interface.9.cfg-mcp-binding-web-pending-attempts</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:63</code> | <code>binding.web.pending_attempts</code> |
| <code>interface.9.cfg-mcp-binding-web-sequencer-public-key</code> | <code>config_key</code> | <code>change</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:61</code> | <code>binding.web.sequencer_public_key</code> |
| <code>interface.9.cfg-mcp-binding-web-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:62</code> | <code>binding.web.timeout_ms</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-mcp-binding-agent</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-bearer-file</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-endpoint</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-probe-program</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-readiness</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-readiness-endpoint</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-readiness-gateway-key-file</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-agent-readiness-trust-anchors</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.cfg-mcp-binding-audit-root</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-capability-id</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-core-sequence</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. Fed from LAYERX_AGENT_MCP_CORE_SEQUENCE, a kernel sequence the EVM design replaces (stale task 6.5.16). |
| <code>interface.9.cfg-mcp-binding-deadline-ms</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-ceiling</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-consumed</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-id</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-name</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-scope</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-limit-scope-id</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener-admitted-uids</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener-mode</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener-owner-gid</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener-owner-uid</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-listener-socket</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-mode</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-session-generation</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-session-id</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-session-token-file</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-store</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-tenant</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web-approval-threshold</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web-endpoint</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web-network</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web-pending-attempts</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |
| <code>interface.9.cfg-mcp-binding-web-sequencer-public-key</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. Fed from LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY; stale tasks 6.5.17 and 5.5.2 re-point the web payer. |
| <code>interface.9.cfg-mcp-binding-web-timeout-ms</code> | notes | Closed-key JSON binding document read by layerx-mcp (Binding::load); unknown keys are refused. |

## native-effect-policy-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-native-effect-policy-source-policy</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:716</code> | <code>layerx.native-effect-policy-source.v1 source: policy</code> |
| <code>interface.9.cfg-agentd-native-effect-policy-source-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:715</code> | <code>layerx.native-effect-policy-source.v1 source: tenant</code> |
| <code>interface.9.cfg-agentd-native-effect-policy-source-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:714</code> | <code>layerx.native-effect-policy-source.v1 source: version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-agentd-native-effect-policy-source-policy</code> | notes | Protected JSON source with deny_unknown_fields. |
| <code>interface.9.cfg-agentd-native-effect-policy-source-tenant</code> | notes | Protected JSON source with deny_unknown_fields. |
| <code>interface.9.cfg-agentd-native-effect-policy-source-version</code> | notes | Protected JSON source with deny_unknown_fields. |

## native-program-policy-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-native-program-policy-source-policy</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:671</code> | <code>layerx.native-program-policy-source.v1 source: policy</code> |
| <code>interface.9.cfg-agentd-native-program-policy-source-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:670</code> | <code>layerx.native-program-policy-source.v1 source: tenant</code> |
| <code>interface.9.cfg-agentd-native-program-policy-source-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:669</code> | <code>layerx.native-program-policy-source.v1 source: version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-agentd-native-program-policy-source-policy</code> | notes | Protected JSON source with deny_unknown_fields. |
| <code>interface.9.cfg-agentd-native-program-policy-source-tenant</code> | notes | Protected JSON source with deny_unknown_fields. |
| <code>interface.9.cfg-agentd-native-program-policy-source-version</code> | notes | Protected JSON source with deny_unknown_fields. |

## program-budget-denominations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-program-budget-denominations-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:541</code> | <code>program budget denominations source: asset</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-limits</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:535</code> | <code>program budget denominations source: limits</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-source</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:542</code> | <code>program budget denominations source: source</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-stable-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:540</code> | <code>program budget denominations source: stable_id</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:534</code> | <code>program budget denominations source: tenant</code> |
| <code>interface.9.cfg-agentd-program-budget-denominations-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/config.rs:533</code> | <code>program budget denominations source: version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-agentd-program-budget-denominations-asset</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |
| <code>interface.9.cfg-agentd-program-budget-denominations-limits</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |
| <code>interface.9.cfg-agentd-program-budget-denominations-source</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |
| <code>interface.9.cfg-agentd-program-budget-denominations-stable-id</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |
| <code>interface.9.cfg-agentd-program-budget-denominations-tenant</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |
| <code>interface.9.cfg-agentd-program-budget-denominations-version</code> | notes | Protected JSON source with deny_unknown_fields, loaded by load_program_budget_denominations. |

## registry-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cfg-agentd-registry-source-client-certificate-pem</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:29</code> | <code>registry source configuration: client_certificate_pem</code> |
| <code>interface.9.cfg-agentd-registry-source-client-private-key-pem</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:30</code> | <code>registry source configuration: client_private_key_pem</code> |
| <code>interface.9.cfg-agentd-registry-source-deadline-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:32</code> | <code>registry source configuration: deadline_ms</code> |
| <code>interface.9.cfg-agentd-registry-source-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:27</code> | <code>registry source configuration: endpoint</code> |
| <code>interface.9.cfg-agentd-registry-source-maximum-response-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:33</code> | <code>registry source configuration: maximum_response_bytes</code> |
| <code>interface.9.cfg-agentd-registry-source-request-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:31</code> | <code>registry source configuration: request_token_file</code> |
| <code>interface.9.cfg-agentd-registry-source-server-ca-der</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:28</code> | <code>registry source configuration: server_ca_der</code> |
| <code>interface.9.cfg-agentd-registry-source-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/registry_source.rs:26</code> | <code>registry source configuration: version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cfg-agentd-registry-source-client-certificate-pem</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-client-private-key-pem</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-deadline-ms</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-endpoint</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-maximum-response-bytes</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-request-token-file</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-server-ca-der</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
| <code>interface.9.cfg-agentd-registry-source-version</code> | notes | Protected JSON file named by LAYERX_AGENT_REGISTRY_SOURCE_CONFIG; deny_unknown_fields; endpoint must be https. |
