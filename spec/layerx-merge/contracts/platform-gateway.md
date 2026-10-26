# Platform gateway contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## gateway-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-gateway-policy-schema</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1098</code> | <code>schema: String,</code> |
| <code>interface.8.cfg-gateway-policy-trusted-proxies</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1099</code> | <code>trusted_proxies: Vec&lt;String&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-gateway-policy-schema</code> | notes | 2031 callers total |
| <code>interface.8.cfg-gateway-policy-trusted-proxies</code> | notes | no in-repo config file sets the key outside the defining file |

## gateway-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-gateway-layerx-files-owner-endpoint-ui-bindings-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1609</code> | <code>.env("LAYERX_FILES_OWNER_ENDPOINT_UI_BINDINGS", owner)</code> |
| <code>interface.8.env-gateway-layerx-gateway-authority-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:109</code> | <code>"LAYERX_GATEWAY_AUTHORITY_TOKEN_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-authority-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1045</code> | <code>&amp;env::var("LAYERX_GATEWAY_AUTHORITY_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-capabilities-ttl-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/capabilities.rs:189</code> | <code>let seconds = std::env::var("LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS")</code> |
| <code>interface.8.env-gateway-layerx-gateway-client-identity-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:115</code> | <code>"LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-client-identity-pkcs12</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1014</code> | <code>env::var("LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12")</code> |
| <code>interface.8.env-gateway-layerx-gateway-component-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:106</code> | <code>"LAYERX_GATEWAY_COMPONENT_TOKEN_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-component-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1000</code> | <code>if env::var_os("LAYERX_GATEWAY_COMPONENT_URL").is_none() {</code> |
| <code>interface.8.env-gateway-layerx-gateway-consensus-rpc-url-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/chain_reads.rs</code> (planned) | <code>LAYERX_GATEWAY_CONSENSUS_RPC_URL</code> |
| <code>interface.8.env-gateway-layerx-gateway-explorer-backend-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:115</code> | <code>std::env::var("LAYERX_GATEWAY_EXPLORER_BACKEND_URL").ok(),</code> |
| <code>interface.8.env-gateway-layerx-gateway-explorer-frontend-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:114</code> | <code>std::env::var("LAYERX_GATEWAY_EXPLORER_FRONTEND_URL").ok(),</code> |
| <code>interface.8.env-gateway-layerx-gateway-faucet-service-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_faucet.rs:9</code> | <code>const TOKEN_VARIABLE: &amp;str = "LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE";</code> |
| <code>interface.8.env-gateway-layerx-gateway-faucet-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_faucet.rs:8</code> | <code>const URL_VARIABLE: &amp;str = "LAYERX_GATEWAY_FAUCET_URL";</code> |
| <code>interface.8.env-gateway-layerx-gateway-hpx-ingress-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:109</code> | <code>let hpx_ingress = match std::env::var_os("LAYERX_GATEWAY_HPX_INGRESS_FILE") {</code> |
| <code>interface.8.env-gateway-layerx-gateway-idempotency-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1112</code> | <code>let idempotency_seconds = env::var("LAYERX_GATEWAY_IDEMPOTENCY_SECONDS")</code> |
| <code>interface.8.env-gateway-layerx-gateway-identity-provisioning-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_register.rs:11</code> | <code>const TOKEN_VARIABLE: &amp;str = "LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE";</code> |
| <code>interface.8.env-gateway-layerx-gateway-identity-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:111</code> | <code>"LAYERX_GATEWAY_IDENTITY_TOKEN_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-identity-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1050</code> | <code>&amp;env::var("LAYERX_GATEWAY_IDENTITY_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-indexer-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:67</code> | <code>std::env::var("LAYERX_GATEWAY_INDEXER_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-key-provisioning-key-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:117</code> | <code>"LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1123</code> | <code>listen: env::var("LAYERX_GATEWAY_LISTEN")</code> |
| <code>interface.8.env-gateway-layerx-gateway-listener</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:930</code> | <code>match env::var("LAYERX_GATEWAY_LISTENER") {</code> |
| <code>interface.8.env-gateway-layerx-gateway-lxp-wire-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:976</code> | <code>let wire_version = env::var("LAYERX_GATEWAY_LXP_WIRE_VERSION")</code> |
| <code>interface.8.env-gateway-layerx-gateway-module-registry-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4303</code> | <code>env::var("LAYERX_GATEWAY_MODULE_REGISTRY_FILE")</code> |
| <code>interface.8.env-gateway-layerx-gateway-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:974</code> | <code>let network_id = env::var("LAYERX_GATEWAY_NETWORK_ID")</code> |
| <code>interface.8.env-gateway-layerx-gateway-outbound-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1097</code> | <code>env::var("LAYERX_GATEWAY_OUTBOUND_CA_DER")</code> |
| <code>interface.8.env-gateway-layerx-gateway-paxeer-rpc-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:23</code> | <code>let Ok(value) = std::env::var("LAYERX_GATEWAY_PAXEER_RPC_URLS") else {</code> |
| <code>interface.8.env-gateway-layerx-gateway-paxeer-ws-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ws.rs:340</code> | <code>let endpoint = std::env::var("LAYERX_GATEWAY_PAXEER_WS_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-program-registry-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:113</code> | <code>"LAYERX_GATEWAY_PROGRAM_REGISTRY_TOKEN_FILE",</code> |
| <code>interface.8.env-gateway-layerx-gateway-program-registry-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1055</code> | <code>&amp;env::var("LAYERX_GATEWAY_PROGRAM_REGISTRY_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-protocol-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:987</code> | <code>let protocol_network_id = env::var("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID")</code> |
| <code>interface.8.env-gateway-layerx-gateway-public-core-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/public_reads.rs:71</code> | <code>std::env::var("LAYERX_GATEWAY_PUBLIC_CORE_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-public-host</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:113</code> | <code>std::env::var("LAYERX_GATEWAY_PUBLIC_HOST").ok(),</code> |
| <code>interface.8.env-gateway-layerx-gateway-receipt-keys-file</code> | <code>env_var</code> | <code>change</code> | <code>services/platform/hosted/gateway/src/main.rs:1022</code> | <code>env::var("LAYERX_GATEWAY_RECEIPT_KEYS_FILE")</code> |
| <code>interface.8.env-gateway-layerx-gateway-redis-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1146</code> | <code>read_secret("LAYERX_GATEWAY_REDIS_PASSWORD_FILE")?,</code> |
| <code>interface.8.env-gateway-layerx-gateway-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1141</code> | <code>&amp;env::var("LAYERX_GATEWAY_REDIS_URL")</code> |
| <code>interface.8.env-gateway-layerx-gateway-redis-username-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1145</code> | <code>read_secret("LAYERX_GATEWAY_REDIS_USERNAME_FILE")?,</code> |
| <code>interface.8.env-gateway-layerx-gateway-route-bindings-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:115</code> | <code>let Some(path) = std::env::var_os("LAYERX_GATEWAY_ROUTE_BINDINGS_FILE") else {</code> |
| <code>interface.8.env-gateway-layerx-gateway-service-client-identity-password-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1068</code> | <code>const PASSWORD: &amp;str = "LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE";</code> |
| <code>interface.8.env-gateway-layerx-gateway-service-client-identity-pkcs12</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1067</code> | <code>const IDENTITY: &amp;str = "LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12";</code> |
| <code>interface.8.env-gateway-layerx-gateway-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:948</code> | <code>env::var("LAYERX_GATEWAY_TLS_CERT_DER")</code> |
| <code>interface.8.env-gateway-layerx-gateway-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:955</code> | <code>env::var("LAYERX_GATEWAY_TLS_KEY_DER").map_err(&#124;_&#124; "gateway TLS key is required")?,</code> |
| <code>interface.8.env-gateway-layerx-gateway-trusted-proxies</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_register.rs:58</code> | <code>std::env::var("LAYERX_GATEWAY_TRUSTED_PROXIES")</code> |
| <code>interface.8.env-gateway-layerx-gateway-ui-bindings-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:121</code> | <code>if let Some(path) = std::env::var_os("LAYERX_GATEWAY_UI_BINDINGS_FILE") {</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-binding-socket-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7086</code> | <code>"LAYERX_GATEWAY_WALLET_BINDING_SOCKET",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-chain-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7090</code> | <code>"LAYERX_GATEWAY_WALLET_CHAIN_ID",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-gid-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7088</code> | <code>"LAYERX_GATEWAY_WALLET_IDENTITY_GID",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-uid-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7087</code> | <code>"LAYERX_GATEWAY_WALLET_IDENTITY_UID",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7084</code> | <code>"LAYERX_GATEWAY_WALLET_IDENTITY_URL",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-issuer-socket-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7085</code> | <code>"LAYERX_GATEWAY_WALLET_ISSUER_SOCKET",</code> |
| <code>interface.8.env-gateway-layerx-gateway-wallet-tenant-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7089</code> | <code>"LAYERX_GATEWAY_WALLET_TENANT",</code> |
| <code>interface.8.env-gateway-layerx-pay-timing</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:40</code> | <code>if std::env::var_os("LAYERX_PAY_TIMING").is_some() {</code> |
| <code>interface.8.env-gateway-paxeer-x-authority-contract-case-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:6895</code> | <code>env::var("PAXEER_X_AUTHORITY_CONTRACT_CASE").expect("real authority case required");</code> |
| <code>interface.8.env-gateway-paxeer-x-gateway-readiness-case-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7421</code> | <code>let input = env::var("PAXEER_X_GATEWAY_READINESS_CASE")</code> |
| <code>interface.8.env-gateway-paxeer-x-router-readiness-case-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7481</code> | <code>let input = env::var("PAXEER_X_ROUTER_READINESS_CASE").expect("real service case required");</code> |
| <code>interface.8.env-gateway-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1124</code> | <code>.or_else(&#124;_&#124; env::var("PORT").map(&#124;port&#124; format!("&lt;addr&gt;:{port}")))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-gateway-layerx-files-owner-endpoint-ui-bindings-j69</code> | notes | read at services/platform/hosted/gateway/src/ui_proxy.rs:1609 |
| <code>interface.8.env-gateway-layerx-gateway-authority-token-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:109; 14 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-client-identity-password-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:115; 20 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-component-token-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:106; 17 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-consensus-rpc-url-j40</code> | notes | planned consensus JSON-RPC endpoint for px_getCommit, px_getValidators and px_getSequenceBatch; task 8.1.2 |
| <code>interface.8.env-gateway-layerx-gateway-explorer-backend-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-gateway-layerx-gateway-explorer-frontend-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-gateway-layerx-gateway-identity-token-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:111; 17 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-key-provisioning-key-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:117; 13 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-program-registry-token-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:113; 19 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-public-host</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-gateway-layerx-gateway-receipt-keys-file</code> | overridden_by | override.J40.1 |
| <code>interface.8.env-gateway-layerx-gateway-redis-password-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:1146; 16 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-redis-username-file-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:1145; 16 in-repo references, first 8 listed |
| <code>interface.8.env-gateway-layerx-gateway-trusted-proxies</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-gateway-layerx-gateway-wallet-binding-socket-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7086 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-chain-id-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7090 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-gid-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7088 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-uid-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7087 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-identity-url-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7084 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-issuer-socket-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7085 |
| <code>interface.8.env-gateway-layerx-gateway-wallet-tenant-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7089 |
| <code>interface.8.env-gateway-paxeer-x-authority-contract-case-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:6895 |
| <code>interface.8.env-gateway-paxeer-x-gateway-readiness-case-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7421 |
| <code>interface.8.env-gateway-paxeer-x-router-readiness-case-j69</code> | notes | read at services/platform/hosted/gateway/src/main.rs:7481 |
| <code>interface.8.env-gateway-port</code> | notes | system variable; callers not enumerated |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.2.1</code> | req | 8002.1 |
| <code>acceptance.8.2.1</code> | scenario | Given a chain-only gateway environment that sets LAYERX_GATEWAY_RECEIPT_KEYS_FILE and no component URL, when the gateway binary starts, then it accepts the configuration and GET /livez answers 200 with status live |
| <code>acceptance.8.2.1</code> | test | services/platform/hosted/gateway/tests/degraded.rs::chain_only_gateway_pins_receipt_keys_without_the_component_url (planned) |
| <code>acceptance.8.2.1</code> | qualification | UNRUN |
| <code>req.8002</code> | ac_1 | A gateway started with LAYERX_GATEWAY_RECEIPT_KEYS_FILE and without LAYERX_GATEWAY_COMPONENT_URL starts and answers /livez 200, where services/platform/hosted/gateway/src/main.rs:1001 refuses it at HEAD. |
| <code>capability.8.1.1</code> | title | Load the receipt key list without the kernel backend set |
| <code>capability.8.1.1</code> | reqs | 8002.1 |
| <code>resolution.17.796</code> | title | decision.801 binds: px_getReceiptKeys answers the gateway's pinned receipt key list |
| <code>resolution.17.796</code> | applies_to | decision.801<br>task.8.1.1<br>task.8.1.2 |
| <code>resolution.17.796</code> | source | decision.801 in fragments/J40/decision.kvx; tasks from affects |

## gateway-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-gateway-32001</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:582</code> | <code>return refusal(id, -32001, "History unavailable", json!({"code": code}))</code> |
| <code>interface.8.err-gateway-32002</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1064</code> | <code>return super::rpc::error(id, -32002, "Authentication required");</code> |
| <code>interface.8.err-gateway-32003</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ws.rs:613</code> | <code>&amp;rpc::error(&amp;value["id"], -32003, "Sequencer unavailable"),</code> |
| <code>interface.8.err-gateway-32004</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:175</code> | <code>return super::rpc::error(id, -32004, "Subscriptions are not relayed");</code> |
| <code>interface.8.err-gateway-32005</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:457</code> | <code>429 =&gt; (-32005, "Read unavailable"),</code> |
| <code>interface.8.err-gateway-32600</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:332</code> | <code>Some(error(&amp;Value::Null, -32600, "Invalid Request"))</code> |
| <code>interface.8.err-gateway-32601</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:177</code> | <code>return super::rpc::error(id, -32601, "Method not relayed");</code> |
| <code>interface.8.err-gateway-32602</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/capabilities.rs:282</code> | <code>return super::rpc::error(id, -32602, "Invalid params");</code> |
| <code>interface.8.err-gateway-32603</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:120</code> | <code>&#124;&#124; error(id, -32603, "Invalid upstream response"),</code> |
| <code>interface.8.err-gateway-32700</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:727</code> | <code>return json_response(200, &amp;error(&amp;Value::Null, -32700, "Parse error"));</code> |
| <code>interface.8.err-gateway-activity-authorization-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2035</code> | <code>return response(403, "activity_authorization_refused", None);</code> |
| <code>interface.8.err-gateway-activity-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2496</code> | <code>return Err(response(415, "activity_content_type_required", None));</code> |
| <code>interface.8.err-gateway-activity-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:5065</code> | <code>"code": "activity_refused",</code> |
| <code>interface.8.err-gateway-api-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ai_views.rs:98</code> | <code>&amp;json!({"ok": false, "error": {"code": "api_key_required"}}),</code> |
| <code>interface.8.err-gateway-authority-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1835</code> | <code>.map_err(&#124;_&#124; response(503, "authority_invalid", Some(5)))?;</code> |
| <code>interface.8.err-gateway-authority-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1841</code> | <code>return Err(response(503, "authority_mismatch", Some(5)));</code> |
| <code>interface.8.err-gateway-authority-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1832</code> | <code>return Err(response(503, "authority_unavailable", Some(5)));</code> |
| <code>interface.8.err-gateway-caps-account-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7371</code> | <code>return response(403, "caps_account_refused", None);</code> |
| <code>interface.8.err-gateway-caps-assertion-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7362</code> | <code>Err(_) =&gt; return response(403, "caps_assertion_refused", None),</code> |
| <code>interface.8.err-gateway-caps-binding-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7357</code> | <code>return response(403, "caps_binding_refused", None);</code> |
| <code>interface.8.err-gateway-caps-binding-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7334</code> | <code>return response(503, "caps_binding_unavailable", Some(5));</code> |
| <code>interface.8.err-gateway-caps-clock-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7349</code> | <code>return response(503, "caps_clock_unavailable", Some(5));</code> |
| <code>interface.8.err-gateway-caps-evidence-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7380</code> | <code>return response(502, "caps_evidence_refused", None);</code> |
| <code>interface.8.err-gateway-caps-identity-evidence</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7328</code> | <code>return response(502, "caps_identity_evidence", None);</code> |
| <code>interface.8.err-gateway-caps-identity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7319</code> | <code>return response(503, "caps_identity_unavailable", Some(5));</code> |
| <code>interface.8.err-gateway-caps-issuer-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7361</code> | <code>Err(503) =&gt; return response(503, "caps_issuer_unavailable", Some(5)),</code> |
| <code>interface.8.err-gateway-caps-network-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7306</code> | <code>return response(403, "caps_network_refused", None);</code> |
| <code>interface.8.err-gateway-caps-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7283</code> | <code>return response(503, "caps_not_configured", Some(5));</code> |
| <code>interface.8.err-gateway-caps-principal-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7365</code> | <code>return response(403, "caps_principal_refused", None);</code> |
| <code>interface.8.err-gateway-caps-session-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7404</code> | <code>return response(403, "caps_session_expired", None);</code> |
| <code>interface.8.err-gateway-caps-session-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7322</code> | <code>return response(403, "caps_session_refused", None);</code> |
| <code>interface.8.err-gateway-caps-session-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7291</code> | <code>return response(401, "caps_session_required", None);</code> |
| <code>interface.8.err-gateway-client-identity-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:831</code> | <code>return response(503, "client_identity_required", None);</code> |
| <code>interface.8.err-gateway-component-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1898</code> | <code>parse_hex32(activity_id).map_err(&#124;_&#124; response(503, "component_invalid", Some(5)))?;</code> |
| <code>interface.8.err-gateway-component-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1588</code> | <code>.map_err(&#124;_&#124; response(503, "component_unavailable", Some(5)))</code> |
| <code>interface.8.err-gateway-explorer-authority-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:643</code> | <code>Err(_) =&gt; return Some(response(503, "explorer_authority_unavailable", None)),</code> |
| <code>interface.8.err-gateway-explorer-cookie-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:315</code> | <code>Err(_) =&gt; return response(502, "explorer_cookie_refused", None),</code> |
| <code>interface.8.err-gateway-explorer-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:274</code> | <code>return response(503, "explorer_not_configured", None);</code> |
| <code>interface.8.err-gateway-explorer-redirect-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:320</code> | <code>Err(_) =&gt; return response(502, "explorer_redirect_refused", None),</code> |
| <code>interface.8.err-gateway-explorer-target-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:249</code> | <code>return response(400, "explorer_target_refused", None);</code> |
| <code>interface.8.err-gateway-explorer-upstream-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:299</code> | <code>return response(502, "explorer_upstream_unavailable", None);</code> |
| <code>interface.8.err-gateway-explorer-websocket-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ws.rs:1950</code> | <code>&amp;super::response(502, "explorer_websocket_unavailable", None),</code> |
| <code>interface.8.err-gateway-faucet-claim-pending</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_faucet.rs:121</code> | <code>"code": "faucet_claim_pending", "state": "pending", "upstream": document</code> |
| <code>interface.8.err-gateway-hpx-discovery-body-forbidden</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:700</code> | <code>return Some(response(400, "hpx_discovery_body_forbidden", None));</code> |
| <code>interface.8.err-gateway-hpx-ingress-provenance-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:687</code> | <code>Err(_) =&gt; return Some(response(400, "hpx_ingress_provenance_refused", None)),</code> |
| <code>interface.8.err-gateway-hpx-ingress-provenance-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:680</code> | <code>return Some(response(503, "hpx_ingress_provenance_unavailable", None));</code> |
| <code>interface.8.err-gateway-hpx-registration-body-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:697</code> | <code>return Some(response(413, "hpx_registration_body_too_large", None));</code> |
| <code>interface.8.err-gateway-hpx-registration-token-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:694</code> | <code>return Some(response(400, "hpx_registration_token_invalid", None));</code> |
| <code>interface.8.err-gateway-hpx-upstream-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:733</code> | <code>Err(_) =&gt; response(503, "hpx_upstream_unavailable", None),</code> |
| <code>interface.8.err-gateway-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2573</code> | <code>return Err(response(409, "idempotency_conflict", None));</code> |
| <code>interface.8.err-gateway-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2371</code> | <code>_ =&gt; return response(400, "idempotency_key_required", None),</code> |
| <code>interface.8.err-gateway-identity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1603</code> | <code>.map_err(&#124;_&#124; response(503, "identity_unavailable", Some(5)))?,</code> |
| <code>interface.8.err-gateway-indexer-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:566</code> | <code>json!({"code": "indexer_not_configured"}),</code> |
| <code>interface.8.err-gateway-insufficient-scope</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3694</code> | <code>let result = response(403, "insufficient_scope", None);</code> |
| <code>interface.8.err-gateway-invalid-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2513</code> | <code>Err(_) =&gt; return Err(response(400, "invalid_activity", None)),</code> |
| <code>interface.8.err-gateway-invalid-caps-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:7297</code> | <code>return response(400, "invalid_caps_request", None);</code> |
| <code>interface.8.err-gateway-invalid-core-response</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/public_reads.rs:62</code> | <code>return response(502, "invalid_core_response", None);</code> |
| <code>interface.8.err-gateway-invalid-expected-state-root</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2014</code> | <code>_ =&gt; return response(400, "invalid_expected_state_root", None),</code> |
| <code>interface.8.err-gateway-invalid-explorer-profile-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:566</code> | <code>return Some(response(400, "invalid_explorer_profile_request", None));</code> |
| <code>interface.8.err-gateway-invalid-http-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3775</code> | <code>&amp;response(400, "invalid_http_request", None),</code> |
| <code>interface.8.err-gateway-invalid-indexer-readiness</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:592</code> | <code>json!({"code": "invalid_indexer_readiness"}),</code> |
| <code>interface.8.err-gateway-invalid-key-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3865</code> | <code>Err(_) =&gt; return response(400, "invalid_key_request", None),</code> |
| <code>interface.8.err-gateway-invalid-mcp-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1332</code> | <code>Err(_) =&gt; return response(400, "invalid_mcp_request", None),</code> |
| <code>interface.8.err-gateway-invalid-mcp-tool</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1360</code> | <code>None =&gt; return response(400, "invalid_mcp_tool", None),</code> |
| <code>interface.8.err-gateway-invalid-minimum-sequence</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2007</code> | <code>_ =&gt; return response(400, "invalid_minimum_sequence", None),</code> |
| <code>interface.8.err-gateway-invalid-preflight</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:449</code> | <code>return Some(response(400, "invalid_preflight", None));</code> |
| <code>interface.8.err-gateway-invalid-program-activity-selector</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3413</code> | <code>return response(400, "invalid_program_activity_selector", None);</code> |
| <code>interface.8.err-gateway-invalid-program-call</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2023</code> | <code>return response(400, "invalid_program_call", None);</code> |
| <code>interface.8.err-gateway-invalid-program-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4557</code> | <code>return response(400, "invalid_program_id", None);</code> |
| <code>interface.8.err-gateway-invalid-program-lifecycle</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2500</code> | <code>return Err(response(400, "invalid_program_lifecycle", None));</code> |
| <code>interface.8.err-gateway-invalid-program-receipt-selector</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3408</code> | <code>Err(()) =&gt; return response(400, "invalid_program_receipt_selector", None),</code> |
| <code>interface.8.err-gateway-invalid-program-selector</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3401</code> | <code>return response(400, "invalid_program_selector", None);</code> |
| <code>interface.8.err-gateway-invalid-program-success</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1388</code> | <code>"reason": "invalid_program_success",</code> |
| <code>interface.8.err-gateway-invalid-program-u64</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1405</code> | <code>"reason": "invalid_program_u64",</code> |
| <code>interface.8.err-gateway-invalid-quota</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3875</code> | <code>return response(400, "invalid_quota", None);</code> |
| <code>interface.8.err-gateway-invalid-read-selector</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/public_reads.rs:87</code> | <code>Err(()) =&gt; Some(response(400, "invalid_read_selector", None)),</code> |
| <code>interface.8.err-gateway-invalid-route-target</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:676</code> | <code>None =&gt; return Some(response(400, "invalid_route_target", None)),</code> |
| <code>interface.8.err-gateway-invalid-scopes</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3878</code> | <code>return response(400, "invalid_scopes", None);</code> |
| <code>interface.8.err-gateway-invalid-source-publication</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4673</code> | <code>.map_err(&#124;_&#124; response(400, "invalid_source_publication", None))?;</code> |
| <code>interface.8.err-gateway-invalid-upstream</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:638</code> | <code>Err(_) =&gt; return Some(response(503, "invalid_upstream", None)),</code> |
| <code>interface.8.err-gateway-invalid-upstream-content-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:869</code> | <code>Ok(_) =&gt; response(502, "invalid_upstream_content_type", None),</code> |
| <code>interface.8.err-gateway-invalid-websocket-upgrade</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:266</code> | <code>return response(400, "invalid_websocket_upgrade", None);</code> |
| <code>interface.8.err-gateway-json-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:724</code> | <code>return response(415, "json_content_type_required", None);</code> |
| <code>interface.8.err-gateway-kernel-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:176</code> | <code>"code": "kernel_unavailable",</code> |
| <code>interface.8.err-gateway-key-limit-reached</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3913</code> | <code>response(409, "key_limit_reached", None)</code> |
| <code>interface.8.err-gateway-lifecycle-binding-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2765</code> | <code>return Err(response(502, "lifecycle_binding_invalid", None));</code> |
| <code>interface.8.err-gateway-lxp-err</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1233</code> | <code>} else if code.starts_with("LXP_ERR_") {</code> |
| <code>interface.8.err-gateway-mcp-capacity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1380</code> | <code>return response(503, "mcp_capacity_unavailable", None);</code> |
| <code>interface.8.err-gateway-mcp-deadline-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1399</code> | <code>return response(503, "mcp_deadline_exceeded", None);</code> |
| <code>interface.8.err-gateway-mcp-identity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1377</code> | <code>return response(503, "mcp_identity_unavailable", None);</code> |
| <code>interface.8.err-gateway-mcp-owner-binding-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1369</code> | <code>return response(403, "mcp_owner_binding_required", None);</code> |
| <code>interface.8.err-gateway-mcp-owner-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1414</code> | <code>&amp;json!({"ok":false,"error":{"code":"mcp_owner_unavailable","reason":reason,"automatic_retry":false}}),</code> |
| <code>interface.8.err-gateway-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ai_views.rs:69</code> | <code>&amp;json!({"ok": false, "error": {"code": "method_not_allowed"}}),</code> |
| <code>interface.8.err-gateway-migration-history-query-forbidden</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:589</code> | <code>return Some(response(400, "migration_history_query_forbidden", None));</code> |
| <code>interface.8.err-gateway-migration-history-request-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:592</code> | <code>return Some(response(413, "migration_history_request_too_large", None));</code> |
| <code>interface.8.err-gateway-migration-internal-headers-forbidden</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:620</code> | <code>return Some(response(400, "migration_internal_headers_forbidden", None));</code> |
| <code>interface.8.err-gateway-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:163</code> | <code>reason: "not_configured",</code> |
| <code>interface.8.err-gateway-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:252</code> | <code>return response(404, "not_found", None);</code> |
| <code>interface.8.err-gateway-operation-state-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2607</code> | <code>return Err(response(503, "operation_state_unknown", Some(5)));</code> |
| <code>interface.8.err-gateway-origin-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:460</code> | <code>response(403, "origin_not_allowed", None)</code> |
| <code>interface.8.err-gateway-paxeer-rpc-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:66</code> | <code>refusal["error"]["data"] = json!({"code": "paxeer_rpc_not_configured"});</code> |
| <code>interface.8.err-gateway-persistence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ai_views.rs:102</code> | <code>&amp;json!({"ok": false, "error": {"code": "persistence_unavailable"}}),</code> |
| <code>interface.8.err-gateway-private-service</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:491</code> | <code>return Some(response(403, "private_service", None));</code> |
| <code>interface.8.err-gateway-program-activity-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1941</code> | <code>.map_err(&#124;_&#124; response(503, "program_activity_invalid", Some(5)))?;</code> |
| <code>interface.8.err-gateway-program-activity-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4887</code> | <code>Ok(None) =&gt; return response(404, "program_activity_not_found", None),</code> |
| <code>interface.8.err-gateway-program-call-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2964</code> | <code>"code": "program_call_refused", "protocol_result_code": result_code,</code> |
| <code>interface.8.err-gateway-program-interface-absent</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4815</code> | <code>return response(404, "program_interface_absent", None);</code> |
| <code>interface.8.err-gateway-program-not-active</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2042</code> | <code>return response(409, "program_not_active", None);</code> |
| <code>interface.8.err-gateway-program-read-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2124</code> | <code>return response(503, "program_read_unverified", Some(5));</code> |
| <code>interface.8.err-gateway-program-receipt-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4847</code> | <code>Ok(Some(_) &#124; None) =&gt; return response(404, "program_receipt_not_found", None),</code> |
| <code>interface.8.err-gateway-program-receipt-selector-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4854</code> | <code>return response(409, "program_receipt_selector_mismatch", None);</code> |
| <code>interface.8.err-gateway-program-receipt-verification-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1971</code> | <code>response(503, "program_receipt_verification_failed", Some(5)),</code> |
| <code>interface.8.err-gateway-program-registry-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:873</code> | <code>return Err(response(503, "program_registry_invalid", Some(5)));</code> |
| <code>interface.8.err-gateway-program-registry-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:860</code> | <code>.map_err(&#124;_&#124; response(503, "program_registry_unavailable", Some(5)))?;</code> |
| <code>interface.8.err-gateway-program-registry-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4179</code> | <code>return Err(response(503, "program_registry_unverified", Some(5)));</code> |
| <code>interface.8.err-gateway-program-simulation-head-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2047</code> | <code>return response(503, "program_simulation_head_unavailable", Some(5));</code> |
| <code>interface.8.err-gateway-program-simulation-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2230</code> | <code>response(503, "program_simulation_unverified", Some(5)),</code> |
| <code>interface.8.err-gateway-program-source-absent</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4780</code> | <code>404 =&gt; response(404, "program_source_absent", None),</code> |
| <code>interface.8.err-gateway-program-source-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4782</code> | <code>422 =&gt; response(422, "program_source_unverified", None),</code> |
| <code>interface.8.err-gateway-program-state-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4239</code> | <code>now_millis().map_err(&#124;_&#124; response(503, "program_state_unverified", Some(5)))?;</code> |
| <code>interface.8.err-gateway-protocol-idempotency-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2409</code> | <code>return response(409, "protocol_idempotency_mismatch", None);</code> |
| <code>interface.8.err-gateway-public-read-rate-limit</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:574</code> | <code>json!({"code": "public_read_rate_limit"}),</code> |
| <code>interface.8.err-gateway-publication-authorization-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4779</code> | <code>403 =&gt; response(403, "publication_authorization_refused", None),</code> |
| <code>interface.8.err-gateway-query-string-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:788</code> | <code>return response(400, "query_string_not_allowed", None);</code> |
| <code>interface.8.err-gateway-quota-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2064</code> | <code>Ok(Some(retry)) =&gt; return response(429, "quota_exceeded", Some(retry)),</code> |
| <code>interface.8.err-gateway-readiness-clock-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4065</code> | <code>return response(503, "readiness_clock_unavailable", None);</code> |
| <code>interface.8.err-gateway-receipt-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2922</code> | <code>return response(503, "receipt_encoding_failed", Some(5));</code> |
| <code>interface.8.err-gateway-receipt-key-unauthorized</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1159</code> | <code>response(502, "receipt_key_unauthorized", None)</code> |
| <code>interface.8.err-gateway-receipt-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4381</code> | <code>Ok(None) =&gt; return response(404, "receipt_not_found", None),</code> |
| <code>interface.8.err-gateway-receipt-pending</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1253</code> | <code>"reason": "receipt_pending",</code> |
| <code>interface.8.err-gateway-receipt-verification-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1911</code> | <code>unauthorized_or(error, response(503, "receipt_verification_failed", Some(5)))</code> |
| <code>interface.8.err-gateway-request-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:576</code> | <code>return Some(response(413, "request_too_large", None));</code> |
| <code>interface.8.err-gateway-rotation-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:5174</code> | <code>response(409, "rotation_conflict", None)</code> |
| <code>interface.8.err-gateway-route-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:627</code> | <code>&amp;json!({"error":{"code":"route_unavailable","service":id,"reason":"not_configured"}}),</code> |
| <code>interface.8.err-gateway-server-side-receipt-verification-only</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1280</code> | <code>"reason": "server_side_receipt_verification_only",</code> |
| <code>interface.8.err-gateway-session-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1600</code> | <code>.ok_or_else(&#124;&#124; response(401, "session_required", None))?;</code> |
| <code>interface.8.err-gateway-settlement-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4494</code> | <code>return response(415, "settlement_content_type_required", None);</code> |
| <code>interface.8.err-gateway-settlement-inclusion-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:4480</code> | <code>.map_err(&#124;_&#124; response(503, "settlement_inclusion_unavailable", Some(5)))?;</code> |
| <code>interface.8.err-gateway-signer-not-owned</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3872</code> | <code>return response(403, "signer_not_owned", None);</code> |
| <code>interface.8.err-gateway-snapshot-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2107</code> | <code>Some("snapshot_mismatch") =&gt; return response(409, "snapshot_mismatch", None),</code> |
| <code>interface.8.err-gateway-snapshot-stale</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:2106</code> | <code>Some("snapshot_stale") =&gt; return response(409, "snapshot_stale", Some(1)),</code> |
| <code>interface.8.err-gateway-state-document-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/state.rs:155</code> | <code>Err(_) =&gt; return response(502, "state_document_invalid", None),</code> |
| <code>interface.8.err-gateway-state-proof-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/state.rs:177</code> | <code>return response(502, "state_proof_unverified", None);</code> |
| <code>interface.8.err-gateway-subscription-limit</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ws.rs:469</code> | <code>return http::write_response(stream, &amp;super::response(429, "subscription_limit", Some(1)));</code> |
| <code>interface.8.err-gateway-surface-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/capabilities.rs:294</code> | <code>"code": "surface_unavailable",</code> |
| <code>interface.8.err-gateway-unknown-program</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:862</code> | <code>return Err(response(404, "unknown_program", None));</code> |
| <code>interface.8.err-gateway-unsupported-media-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:825</code> | <code>return response(415, "unsupported_media_type", None);</code> |
| <code>interface.8.err-gateway-untrusted-identity-header</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3619</code> | <code>let result = response(400, "untrusted_identity_header", None);</code> |
| <code>interface.8.err-gateway-upstream-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:768</code> | <code>Err(_) =&gt; response(503, "upstream_unavailable", None),</code> |
| <code>interface.8.err-gateway-withdrawal-verification-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3041</code> | <code>return response(502, "withdrawal_verification_failed", None);</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-gateway-32001</code> | notes | 176 callers total |
| <code>interface.8.err-gateway-32002</code> | notes | 59 callers total |
| <code>interface.8.err-gateway-32005</code> | notes | 105 callers total |
| <code>interface.8.err-gateway-32600</code> | notes | 39 callers total |
| <code>interface.8.err-gateway-32601</code> | notes | 116 callers total |
| <code>interface.8.err-gateway-32602</code> | notes | 444 callers total |
| <code>interface.8.err-gateway-32603</code> | notes | 65 callers total |
| <code>interface.8.err-gateway-activity-authorization-refused</code> | notes | HTTP 403 |
| <code>interface.8.err-gateway-activity-content-type-required</code> | notes | HTTP 415 |
| <code>interface.8.err-gateway-authority-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-authority-mismatch</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-authority-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-caps-account-refused</code> | notes | HTTP 403 |
| <code>interface.8.err-gateway-caps-assertion-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-binding-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-binding-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-clock-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-evidence-refused</code> | notes | HTTP 502 |
| <code>interface.8.err-gateway-caps-identity-evidence</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-identity-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-issuer-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-network-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-not-configured</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-caps-principal-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-session-expired</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-session-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-caps-session-required</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-client-identity-required</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-component-invalid</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-component-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-authority-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-cookie-refused</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-not-configured</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-redirect-refused</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-target-refused</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-upstream-unavailable</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-explorer-websocket-unavailable</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-discovery-body-forbidden</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-ingress-provenance-refused</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-ingress-provenance-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-registration-body-too-large</code> | notes | HTTP 413; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-registration-token-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-hpx-upstream-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-idempotency-conflict</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-identity-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-indexer-not-configured</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-insufficient-scope</code> | notes | HTTP 403 |
| <code>interface.8.err-gateway-invalid-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-caps-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-core-response</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-expected-state-root</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-explorer-profile-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-http-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-indexer-readiness</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-key-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-mcp-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-mcp-tool</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-minimum-sequence</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-preflight</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-program-activity-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-program-id</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-program-lifecycle</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-program-receipt-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-program-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-invalid-quota</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-read-selector</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-route-target</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-scopes</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-source-publication</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-upstream</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-invalid-upstream-content-type</code> | notes | HTTP 502 |
| <code>interface.8.err-gateway-invalid-websocket-upgrade</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-json-content-type-required</code> | notes | HTTP 415; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-kernel-unavailable</code> | notes | 48 callers total |
| <code>interface.8.err-gateway-key-limit-reached</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-lifecycle-binding-invalid</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-mcp-capacity-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-mcp-deadline-exceeded</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-mcp-identity-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-mcp-owner-binding-required</code> | notes | HTTP 403 |
| <code>interface.8.err-gateway-mcp-owner-unavailable</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-method-not-allowed</code> | notes | 48 callers total |
| <code>interface.8.err-gateway-migration-history-query-forbidden</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-migration-history-request-too-large</code> | notes | HTTP 413; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-migration-internal-headers-forbidden</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-not-configured</code> | notes | 81 callers total |
| <code>interface.8.err-gateway-not-found</code> | notes | HTTP 404; 136 callers total |
| <code>interface.8.err-gateway-operation-state-unknown</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-origin-not-allowed</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-paxeer-rpc-not-configured</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-persistence-unavailable</code> | notes | 68 callers total |
| <code>interface.8.err-gateway-private-service</code> | notes | HTTP 403 |
| <code>interface.8.err-gateway-program-activity-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-activity-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-gateway-program-call-refused</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-interface-absent</code> | notes | HTTP 404 |
| <code>interface.8.err-gateway-program-not-active</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-program-read-unverified</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-receipt-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-gateway-program-receipt-selector-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-program-receipt-verification-failed</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-program-registry-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-registry-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-registry-unverified</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-simulation-head-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-simulation-unverified</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-program-source-absent</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-source-unverified</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-program-state-unverified</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-protocol-idempotency-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-publication-authorization-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-query-string-not-allowed</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-quota-exceeded</code> | notes | HTTP 429 |
| <code>interface.8.err-gateway-readiness-clock-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-receipt-encoding-failed</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-receipt-key-unauthorized</code> | notes | HTTP 502 |
| <code>interface.8.err-gateway-receipt-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-gateway-receipt-verification-failed</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-request-too-large</code> | notes | HTTP 413 |
| <code>interface.8.err-gateway-rotation-conflict</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-session-required</code> | notes | HTTP 401 |
| <code>interface.8.err-gateway-settlement-content-type-required</code> | notes | HTTP 415; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-settlement-inclusion-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-signer-not-owned</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-snapshot-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-snapshot-stale</code> | notes | HTTP 409 |
| <code>interface.8.err-gateway-state-document-invalid</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-state-proof-unverified</code> | notes | HTTP 502 |
| <code>interface.8.err-gateway-subscription-limit</code> | notes | HTTP 429; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-surface-unavailable</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-gateway-unknown-program</code> | notes | HTTP 404 |
| <code>interface.8.err-gateway-unsupported-media-type</code> | notes | HTTP 415 |
| <code>interface.8.err-gateway-untrusted-identity-header</code> | notes | HTTP 400 |
| <code>interface.8.err-gateway-upstream-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-gateway-withdrawal-verification-failed</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |

## gateway-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-gateway-any-explorer</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:128</code> | <code>rest == "/explorer" &#124;&#124; rest.starts_with("/explorer/") &#124;&#124; rest.starts_with("/explorer?")</code> |
| <code>interface.8.http-gateway-any-explorer-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:128</code> | <code>rest == "/explorer" &#124;&#124; rest.starts_with("/explorer/") &#124;&#124; rest.starts_with("/explorer?")</code> |
| <code>interface.8.http-gateway-any-explorer-backend</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:617</code> | <code>.is_some_and(&#124;p&#124; p == "/explorer/backend" &#124;&#124; p.starts_with("/explorer/backend/"))</code> |
| <code>interface.8.http-gateway-any-explorer-backend-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:617</code> | <code>.is_some_and(&#124;p&#124; p == "/explorer/backend" &#124;&#124; p.starts_with("/explorer/backend/"))</code> |
| <code>interface.8.http-gateway-any-explorer-next-data</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:272</code> | <code>let Some(tail) = path.strip_prefix("/explorer/_next/data/") else {</code> |
| <code>interface.8.http-gateway-any-gas-station-quote</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:33</code> | <code>"/gas-station/quote" &#124; "/gas-station/submit" &#124; "/gas-station/status" &#124; "/gas-station/retry"</code> |
| <code>interface.8.http-gateway-any-gas-station-retry</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:33</code> | <code>"/gas-station/quote" &#124; "/gas-station/submit" &#124; "/gas-station/status" &#124; "/gas-station/retry"</code> |
| <code>interface.8.http-gateway-any-gas-station-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:33</code> | <code>"/gas-station/quote" &#124; "/gas-station/submit" &#124; "/gas-station/status" &#124; "/gas-station/retry"</code> |
| <code>interface.8.http-gateway-any-gas-station-submit</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:33</code> | <code>"/gas-station/quote" &#124; "/gas-station/submit" &#124; "/gas-station/status" &#124; "/gas-station/retry"</code> |
| <code>interface.8.http-gateway-any-hpx-api-myip</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1050</code> | <code>if matches!(path, "/hpx/api/register" &#124; "/hpx/api/myip") &amp;&amp; query.is_some() {</code> |
| <code>interface.8.http-gateway-any-hpx-api-register</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1050</code> | <code>if matches!(path, "/hpx/api/register" &#124; "/hpx/api/myip") &amp;&amp; query.is_some() {</code> |
| <code>interface.8.http-gateway-any-human-ui</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:429</code> | <code>} else if path.starts_with("/human-ui/") {</code> |
| <code>interface.8.http-gateway-any-mcp</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:1303</code> | <code>if request.path != "/mcp" {</code> |
| <code>interface.8.http-gateway-any-rpc</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/public_reads.rs:78</code> | <code>if matches!(request.path.as_str(), "/rpc" &#124; "/rpc/schema") {</code> |
| <code>interface.8.http-gateway-any-rpc-schema</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/public_reads.rs:78</code> | <code>if matches!(request.path.as_str(), "/rpc" &#124; "/rpc/schema") {</code> |
| <code>interface.8.http-gateway-any-v1-accounts-accountid-unified</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:533</code> | <code>"/v1/accounts/{accountId}/unified" =&gt; {</code> |
| <code>interface.8.http-gateway-any-v1-programs</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:886</code> | <code>let remainder = path.strip_prefix("/v1/programs/")?;</code> |
| <code>interface.8.http-gateway-any-v1-programs-program-id</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/routes.rs:536</code> | <code>"/v1/programs/{program_id}" =&gt; matches!(</code> |
| <code>interface.8.http-gateway-any-v1-programs-program-id-reads-resolve</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/routes.rs:540</code> | <code>"/v1/programs/{program_id}/reads/resolve" =&gt; matches!(</code> |
| <code>interface.8.http-gateway-any-v1-routes</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:481</code> | <code>if request.path == "/v1/routes" {</code> |
| <code>interface.8.http-gateway-any-v1-sync-readiness</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1053</code> | <code>if wallet_contract == Some(false) &#124;&#124; path == "/v1/sync/readiness" &amp;&amp; query.is_some() {</code> |
| <code>interface.8.http-gateway-any-v1-wallet-lx-approvals</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1033</code> | <code>_ if path.starts_with("/v1/wallet/lx/approvals/") =&gt; {</code> |
| <code>interface.8.http-gateway-any-v1-wallet-lx-approve</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1023</code> | <code>&#124; "/v1/wallet/lx/approve"</code> |
| <code>interface.8.http-gateway-any-v1-wallet-lx-review</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1022</code> | <code>&#124; "/v1/wallet/lx/review"</code> |
| <code>interface.8.http-gateway-any-v1-wallet-sign-custody</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1021</code> | <code>&#124; "/v1/wallet/sign-custody"</code> |
| <code>interface.8.http-gateway-any-v1-wallet-sign-digest</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1020</code> | <code>&#124; "/v1/wallet/sign-digest"</code> |
| <code>interface.8.http-gateway-any-v1-wallet-sponsored-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1019</code> | <code>&#124; "/v1/wallet/sponsored/status"</code> |
| <code>interface.8.http-gateway-any-v2-migration-assets</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:599</code> | <code>&#124; "/v2/migration/assets"</code> |
| <code>interface.8.http-gateway-any-v2-migration-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:585</code> | <code>Some("/v2/migration/history" &#124; "/v2/migration/history/read")</code> |
| <code>interface.8.http-gateway-any-v2-migration-history-read</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/routes.rs:585</code> | <code>Some("/v2/migration/history" &#124; "/v2/migration/history/read")</code> |
| <code>interface.8.http-gateway-any-wallet</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:426</code> | <code>let wallet = path == "/wallet" &#124;&#124; path.starts_with("/wallet/");</code> |
| <code>interface.8.http-gateway-any-wallet-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:426</code> | <code>let wallet = path == "/wallet" &#124;&#124; path.starts_with("/wallet/");</code> |
| <code>interface.8.http-gateway-delete-wallet-api-push-subscribe</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1315</code> | <code>("DELETE", "/wallet/api/push/subscribe"),</code> |
| <code>interface.8.http-gateway-get-api-account-auth-logout</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:159</code> | <code>&#124;&#124; route.path == "/api/account/auth/logout"</code> |
| <code>interface.8.http-gateway-get-api-account-v2-authenticate-via-dynamic</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:161</code> | <code>&#124;&#124; route.path == "/api/account/v2/authenticate_via_dynamic";</code> |
| <code>interface.8.http-gateway-get-api-account-v2-email-resend</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:160</code> | <code>&#124;&#124; route.path == "/api/account/v2/email/resend"</code> |
| <code>interface.8.http-gateway-get-api-myip</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:778</code> | <code>("POST", "/api/register") &#124; ("GET", "/api/myip")</code> |
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-anchors-j40</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs:962</code> | <code>GET /explorer/api/v2/paxeer-x/anchors</code> |
| <code>interface.8.http-gateway-get-explorer-api-config</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1316</code> | <code>("GET", "/explorer/api/config"),</code> |
| <code>interface.8.http-gateway-get-explorer-api-csrf</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1317</code> | <code>("GET", "/explorer/api/csrf"),</code> |
| <code>interface.8.http-gateway-get-explorer-api-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1318</code> | <code>("GET", "/explorer/api/healthz"),</code> |
| <code>interface.8.http-gateway-get-explorer-api-log</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1319</code> | <code>("GET", "/explorer/api/log"),</code> |
| <code>interface.8.http-gateway-get-explorer-api-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1320</code> | <code>("GET", "/explorer/api/metrics"),</code> |
| <code>interface.8.http-gateway-get-explorer-node-api-config</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1325</code> | <code>("GET", "/explorer/node-api/config"),</code> |
| <code>interface.8.http-gateway-get-explorer-node-api-csrf</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1326</code> | <code>("GET", "/explorer/node-api/csrf"),</code> |
| <code>interface.8.http-gateway-get-explorer-node-api-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1327</code> | <code>("GET", "/explorer/node-api/healthz"),</code> |
| <code>interface.8.http-gateway-get-explorer-node-api-log</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1328</code> | <code>("GET", "/explorer/node-api/log"),</code> |
| <code>interface.8.http-gateway-get-explorer-node-api-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1329</code> | <code>("GET", "/explorer/node-api/metrics"),</code> |
| <code>interface.8.http-gateway-get-explorer-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:1942</code> | <code>&amp;&amp; (path == "/explorer/verify" &#124;&#124; next_static_path(path, "/human-ui/_next/static/")))</code> |
| <code>interface.8.http-gateway-get-human-ui-next-static</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:1942</code> | <code>&amp;&amp; (path == "/explorer/verify" &#124;&#124; next_static_path(path, "/human-ui/_next/static/")))</code> |
| <code>interface.8.http-gateway-get-internal-v1-principal</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3635</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/internal/v1/principal" {</code> |
| <code>interface.8.http-gateway-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3638</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-gateway-get-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3654</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/metrics" {</code> |
| <code>interface.8.http-gateway-get-next-static</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:2072</code> | <code>&amp;&amp; (path == "/explorer/verify" &#124;&#124; next_static_path(path, "/_next/static/"))</code> |
| <code>interface.8.http-gateway-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3648</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-gateway-get-readyz-core</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3651</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz/core" {</code> |
| <code>interface.8.http-gateway-get-v1-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1733</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/v1/keys" {</code> |
| <code>interface.8.http-gateway-get-v1-keys-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1736</code> | <code>let Some(suffix) = request.path.strip_prefix("/v1/keys/") else {</code> |
| <code>interface.8.http-gateway-get-v1-programs-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:937</code> | <code>("GET", path) if path.starts_with("/v1/programs/activities/") =&gt; {</code> |
| <code>interface.8.http-gateway-get-v1-programs-receipts-by-idempotency</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:951</code> | <code>("GET", path) if path.starts_with("/v1/programs/receipts/by-idempotency/") =&gt; {</code> |
| <code>interface.8.http-gateway-get-v1-programs-registry</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:966</code> | <code>("GET", path) if path.starts_with("/v1/programs/registry/") =&gt; {</code> |
| <code>interface.8.http-gateway-get-v1-programs-registry-2</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:965</code> | <code>("GET", "/v1/programs/registry") =&gt; Ok(ProductionRoute::ProgramCatalog),</code> |
| <code>interface.8.http-gateway-get-v1-state</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:936</code> | <code>("GET", "/v1/state") =&gt; Ok(ProductionRoute::State),</code> |
| <code>interface.8.http-gateway-get-v1-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3661</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/v1/status" {</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-bnb-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1334</code> | <code>("GET", "/wallet/api/candle/cv/bnb/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-bnb-price</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1335</code> | <code>("GET", "/wallet/api/candle/cv/bnb/price/"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-eth-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1336</code> | <code>("GET", "/wallet/api/candle/cv/eth/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-eth-price</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1337</code> | <code>("GET", "/wallet/api/candle/cv/eth/price/"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-pax-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1338</code> | <code>("GET", "/wallet/api/candle/cv/pax/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-pax-price</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1339</code> | <code>("GET", "/wallet/api/candle/cv/pax/price/"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-sol-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1340</code> | <code>("GET", "/wallet/api/candle/cv/sol/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-sol-price</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1341</code> | <code>("GET", "/wallet/api/candle/cv/sol/price/"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-pax-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1342</code> | <code>("GET", "/wallet/api/candle/pax/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-candle-sid-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1343</code> | <code>("GET", "/wallet/api/candle/sid/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-fx-latest-usd</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1344</code> | <code>("GET", "/wallet/api/fx/latest/USD"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1345</code> | <code>("GET", "/wallet/api/health"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-media</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1346</code> | <code>("GET", "/wallet/api/media"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-pns-api-v1-addresses-lookup</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1348</code> | <code>("GET", "/wallet/api/pns/api/v1/addresses:lookup"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-pns-api-v1-domains-lookup</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1351</code> | <code>("GET", "/wallet/api/pns/api/v1/domains:lookup"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-candles-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1353</code> | <code>("GET", "/wallet/api/sdk/candles/history"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-breakout</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1557</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/breakout"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-movers</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1561</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/movers"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1558</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/new"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-top-volume</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1559</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/top_volume"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-trending</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1556</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/trending"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-unusual</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1560</code> | <code>("GET", "/wallet/api/sdk/ranking/rankings/unusual"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sdk-stats-stats-batch</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1363</code> | <code>("GET", "/wallet/api/sdk/stats/stats/batch"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-sidiora-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1365</code> | <code>("GET", "/wallet/api/sidiora/metadata"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-token-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1367</code> | <code>("GET", "/wallet/api/token-metadata"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:436</code> | <code>if path.starts_with("/wallet/api/wallet/") &amp;&amp; !matches!(request.method.as_str(), "GET" &#124; "HEAD")</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v1-trending</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1390</code> | <code>("GET", "/wallet/api/wallet/api/v1/trending"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-main-page-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1436</code> | <code>("GET", "/wallet/api/wallet/api/v2/main-page/blocks"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-main-page-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1437</code> | <code>("GET", "/wallet/api/wallet/api/v2/main-page/transactions"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1438</code> | <code>("GET", "/wallet/api/wallet/api/v2/search"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-stats</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1439</code> | <code>("GET", "/wallet/api/wallet/api/v2/stats"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1440</code> | <code>("GET", "/wallet/api/wallet/api/v2/tokens"),</code> |
| <code>interface.8.http-gateway-get-wallet-api-wallet-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1455</code> | <code>("GET", "/wallet/api/wallet/health"),</code> |
| <code>interface.8.http-gateway-head-explorer-api-config</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1456</code> | <code>("HEAD", "/explorer/api/config"),</code> |
| <code>interface.8.http-gateway-head-explorer-api-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1457</code> | <code>("HEAD", "/explorer/api/healthz"),</code> |
| <code>interface.8.http-gateway-head-explorer-api-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1458</code> | <code>("HEAD", "/explorer/api/metrics"),</code> |
| <code>interface.8.http-gateway-head-explorer-node-api-config</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1459</code> | <code>("HEAD", "/explorer/node-api/config"),</code> |
| <code>interface.8.http-gateway-head-explorer-node-api-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1460</code> | <code>("HEAD", "/explorer/node-api/healthz"),</code> |
| <code>interface.8.http-gateway-head-explorer-node-api-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1461</code> | <code>("HEAD", "/explorer/node-api/metrics"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v1-trending</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1484</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/trending"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-main-page-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1530</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/main-page/blocks"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-main-page-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1531</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/main-page/transactions"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1532</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/search"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-stats</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1533</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/stats"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1534</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/tokens"),</code> |
| <code>interface.8.http-gateway-head-wallet-api-wallet-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1549</code> | <code>("HEAD", "/wallet/api/wallet/health"),</code> |
| <code>interface.8.http-gateway-options-quote</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:822</code> | <code>&amp;&amp; matches!(request.path, "/quote" &#124; "/submit" &#124; "/status" &#124; "/retry")</code> |
| <code>interface.8.http-gateway-options-retry</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:822</code> | <code>&amp;&amp; matches!(request.path, "/quote" &#124; "/submit" &#124; "/status" &#124; "/retry")</code> |
| <code>interface.8.http-gateway-options-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:822</code> | <code>&amp;&amp; matches!(request.path, "/quote" &#124; "/submit" &#124; "/status" &#124; "/retry")</code> |
| <code>interface.8.http-gateway-options-submit</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:822</code> | <code>&amp;&amp; matches!(request.path, "/quote" &#124; "/submit" &#124; "/status" &#124; "/retry")</code> |
| <code>interface.8.http-gateway-post</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3629</code> | <code>if request.path == "/" &amp;&amp; request.method == "POST" {</code> |
| <code>interface.8.http-gateway-post-api-explorer-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:1943</code> | <code>&#124;&#124; (method == "POST" &amp;&amp; path == "/api/explorer/verify")</code> |
| <code>interface.8.http-gateway-post-api-register</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:778</code> | <code>("POST", "/api/register") &#124; ("GET", "/api/myip")</code> |
| <code>interface.8.http-gateway-post-explorer-api-log</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1550</code> | <code>("POST", "/explorer/api/log"),</code> |
| <code>interface.8.http-gateway-post-explorer-api-monitoring-invalid-api-schema</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1551</code> | <code>("POST", "/explorer/api/monitoring/invalid-api-schema"),</code> |
| <code>interface.8.http-gateway-post-explorer-node-api-log</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1552</code> | <code>("POST", "/explorer/node-api/log"),</code> |
| <code>interface.8.http-gateway-post-explorer-node-api-monitoring-invalid-api-schema</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1553</code> | <code>("POST", "/explorer/node-api/monitoring/invalid-api-schema"),</code> |
| <code>interface.8.http-gateway-post-internal-v2-source-settlements</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:800</code> | <code>&#124;&#124; request.path != "/internal/v2/source-settlements"</code> |
| <code>interface.8.http-gateway-post-v1-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:917</code> | <code>("POST", "/v1/activities") =&gt; Ok(ProductionRoute::Activity),</code> |
| <code>interface.8.http-gateway-post-v1-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:1730</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/v1/keys" {</code> |
| <code>interface.8.http-gateway-post-v1-programs-call</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:921</code> | <code>("POST", "/v1/programs/call") =&gt; Ok(ProductionRoute::ProgramCall),</code> |
| <code>interface.8.http-gateway-post-v1-programs-deploy</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:922</code> | <code>("POST", "/v1/programs/deploy") =&gt; Ok(ProductionRoute::ProgramDeploy),</code> |
| <code>interface.8.http-gateway-post-v1-programs-read</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:920</code> | <code>("POST", "/v1/programs/read") =&gt; Ok(ProductionRoute::ProgramRead),</code> |
| <code>interface.8.http-gateway-post-v1-programs-registry</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:925</code> | <code>("POST", path) if path.starts_with("/v1/programs/registry/") =&gt; {</code> |
| <code>interface.8.http-gateway-post-v1-programs-simulate</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:919</code> | <code>("POST", "/v1/programs/simulate") =&gt; Ok(ProductionRoute::ProgramSimulation),</code> |
| <code>interface.8.http-gateway-post-v1-programs-upgrade</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:923</code> | <code>("POST", "/v1/programs/upgrade") =&gt; Ok(ProductionRoute::ProgramUpgrade),</code> |
| <code>interface.8.http-gateway-post-v1-programs-wind-down</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/lib.rs:924</code> | <code>("POST", "/v1/programs/wind-down") =&gt; Ok(ProductionRoute::ProgramWindDown),</code> |
| <code>interface.8.http-gateway-post-v1-settle</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:918</code> | <code>("POST", "/v1/settle") =&gt; Ok(ProductionRoute::Settle),</code> |
| <code>interface.8.http-gateway-post-v1-wallet-custody</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1025</code> | <code>_ if path.starts_with("/v1/wallet/custody/") =&gt; {</code> |
| <code>interface.8.http-gateway-post-v1-wallet-lx-sign</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1024</code> | <code>&#124; "/v1/wallet/lx/sign" =&gt; Some(method == "POST" &amp;&amp; query.is_none()),</code> |
| <code>interface.8.http-gateway-post-wallet-api-chat</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:2180</code> | <code>&amp;&amp; path == "/wallet/api/chat"</code> |
| <code>interface.8.http-gateway-post-wallet-api-push-subscribe</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1555</code> | <code>("POST", "/wallet/api/push/subscribe"),</code> |
| <code>interface.8.http-gateway-ui-get-explorer-api-tokens-hash-instances-id-media-type-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1323</code> | <code>("GET", "/explorer/api/tokens/[hash]/instances/[id]/media-type"),</code> |
| <code>interface.8.http-gateway-ui-get-explorer-node-api-tokens-hash-instances-id-media-type-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1332</code> | <code>("GET", "/explorer/node-api/tokens/[hash]/instances/[id]/media-type"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-addresses-address-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1347</code> | <code>("GET", "/wallet/api/pns/api/v1/addresses/[address]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-domains-name-events-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1350</code> | <code>("GET", "/wallet/api/pns/api/v1/domains/[name]/events"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-domains-name-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1349</code> | <code>("GET", "/wallet/api/pns/api/v1/domains/[name]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-points-balance-address-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1352</code> | <code>("GET", "/wallet/api/points/balance/[address]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-metadata-metadata-tokenaddress-json-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1356</code> | <code>("GET", "/wallet/api/sdk/metadata/metadata/[tokenAddress].json"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-stats-stats-pooladdress-holders-distribution-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1361</code> | <code>("GET", "/wallet/api/sdk/stats/stats/[poolAddress]/holders/distribution"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-stats-stats-pooladdress-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1358</code> | <code>("GET", "/wallet/api/sdk/stats/stats/[poolAddress]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-sidiora-logo-address-png-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1364</code> | <code>("GET", "/wallet/api/sidiora/logo/[address].png"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-token-icon-address-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1366</code> | <code>("GET", "/wallet/api/token-icon/[address]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-dex-history-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1368</code> | <code>("GET", "/wallet/api/wallet/api/v1/[address]/dex-history"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-performance-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1369</code> | <code>("GET", "/wallet/api/wallet/api/v1/[address]/performance"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-profile-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1370</code> | <code>("GET", "/wallet/api/wallet/api/v1/[address]/profile"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-rank-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1371</code> | <code>("GET", "/wallet/api/wallet/api/v1/[address]/rank"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-charts-symbol-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1372</code> | <code>("GET", "/wallet/api/wallet/api/v1/charts/[symbol]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-holdings-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1375</code> | <code>("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/holdings"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-pnl-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1379</code> | <code>("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/pnl"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-tx-volume-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1383</code> | <code>("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/tx-volume"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-value-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1387</code> | <code>("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/value"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-pnl-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1389</code> | <code>("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/pnl"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-by-day-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1398</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history-by-day"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1394</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-counters-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1402</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/counters"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1391</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-logs-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1406</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/logs"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-nft-collections-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1414</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft/collections"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-nft-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1410</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-tabs-counters-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1418</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tabs-counters"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-token-balances-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1422</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-balances"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-token-transfers-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1426</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-transfers"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-tokens-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1430</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tokens"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-transactions-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1434</code> | <code>("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/transactions"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-tokens-tokenhash-holders-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1444</code> | <code>("GET", "/wallet/api/wallet/api/v2/tokens/[tokenHash]/holders"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-tokens-tokenhash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1441</code> | <code>("GET", "/wallet/api/wallet/api/v2/tokens/[tokenHash]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1446</code> | <code>("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-logs-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1449</code> | <code>("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]/logs"),</code> |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-token-transfers-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1453</code> | <code>("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]/token-transfers"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-dex-history-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1462</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/[address]/dex-history"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-performance-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1463</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/[address]/performance"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-profile-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1464</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/[address]/profile"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-rank-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1465</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/[address]/rank"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-charts-symbol-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1466</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/charts/[symbol]"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-holdings-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1469</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/holdings"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-pnl-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1473</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/pnl"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-tx-volume-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1477</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/tx-volume"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-value-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1481</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/value"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-pnl-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1483</code> | <code>("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/pnl"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-by-day-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1492</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history-by-day"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1488</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-counters-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1496</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/counters"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1485</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-logs-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1500</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/logs"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-nft-collections-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1508</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft/collections"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-nft-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1504</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-tabs-counters-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1512</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tabs-counters"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-token-balances-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1516</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-balances"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-token-transfers-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1520</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-transfers"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-tokens-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1524</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tokens"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-transactions-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1528</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/transactions"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-tokens-tokenhash-holders-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1538</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/tokens/[tokenHash]/holders"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-tokens-tokenhash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1535</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/tokens/[tokenHash]"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1540</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-logs-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1543</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]/logs"),</code> |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-token-transfers-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1547</code> | <code>("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]/token-transfers"),</code> |
| <code>interface.8.http-gateway-ui-post-wallet-api-chat-j69</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ui_proxy.rs:1554</code> | <code>("POST", "/wallet/api/chat"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-gateway-any-explorer</code> | notes | 170 callers total |
| <code>interface.8.http-gateway-any-explorer-2</code> | notes | 748 callers total; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-any-explorer-backend-2</code> | notes | 146 callers total |
| <code>interface.8.http-gateway-any-mcp</code> | notes | 28 callers total |
| <code>interface.8.http-gateway-any-rpc</code> | notes | 612 callers total |
| <code>interface.8.http-gateway-any-v1-programs</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 592 callers total |
| <code>interface.8.http-gateway-any-v1-programs-program-id</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 350 callers total |
| <code>interface.8.http-gateway-any-v1-programs-program-id-reads-resolve</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567) |
| <code>interface.8.http-gateway-any-v1-routes</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-any-wallet</code> | notes | 180 callers total |
| <code>interface.8.http-gateway-any-wallet-2</code> | notes | 1238 callers total; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-delete-wallet-api-push-subscribe</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-anchors-j40</code> | notes | explorer anchor checkpoint view admitted by the gateway proxy; retired with the anchor construct by task 8.6.1 |
| <code>interface.8.http-gateway-get-explorer-api-config</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-api-csrf</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-api-healthz</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-api-log</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-api-metrics</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-node-api-config</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-node-api-healthz</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-node-api-log</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-explorer-node-api-metrics</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-livez</code> | notes | 119 callers total |
| <code>interface.8.http-gateway-get-metrics</code> | notes | 323 callers total |
| <code>interface.8.http-gateway-get-readyz</code> | notes | 320 callers total |
| <code>interface.8.http-gateway-get-v1-keys-2</code> | notes | 40 callers total |
| <code>interface.8.http-gateway-get-v1-programs-activities</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 51 callers total |
| <code>interface.8.http-gateway-get-v1-programs-receipts-by-idempotency</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 41 callers total |
| <code>interface.8.http-gateway-get-v1-programs-registry</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 84 callers total |
| <code>interface.8.http-gateway-get-v1-programs-registry-2</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567) |
| <code>interface.8.http-gateway-get-v1-state</code> | notes | 37 callers total |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-bnb-history</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-bnb-price</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-eth-history</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-eth-price</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-pax-history</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-pax-price</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-sol-history</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-candle-cv-sol-price</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-candles-history</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-breakout</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-movers</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-new</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-top-volume</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-trending</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-ranking-rankings-unusual</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-sdk-stats-stats-batch</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-token-metadata</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v1-trending</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-main-page-blocks</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-main-page-transactions</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-search</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-stats</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-api-v2-tokens</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-get-wallet-api-wallet-health</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-api-config</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-api-healthz</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-api-metrics</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-node-api-config</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-node-api-healthz</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-explorer-node-api-metrics</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v1-trending</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-main-page-blocks</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-main-page-transactions</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-search</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-stats</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-api-v2-tokens</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-head-wallet-api-wallet-health</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-options-quote</code> | notes | 138 callers total |
| <code>interface.8.http-gateway-options-status</code> | notes | 410 callers total |
| <code>interface.8.http-gateway-options-submit</code> | notes | 100 callers total |
| <code>interface.8.http-gateway-post</code> | notes | path is too generic to attribute callers |
| <code>interface.8.http-gateway-post-explorer-api-log</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-post-explorer-api-monitoring-invalid-api-schema</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-post-explorer-node-api-log</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-post-v1-activities</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 104 callers total |
| <code>interface.8.http-gateway-post-v1-programs-call</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 80 callers total |
| <code>interface.8.http-gateway-post-v1-programs-deploy</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 60 callers total |
| <code>interface.8.http-gateway-post-v1-programs-read</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567) |
| <code>interface.8.http-gateway-post-v1-programs-registry</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 84 callers total |
| <code>interface.8.http-gateway-post-v1-programs-simulate</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 45 callers total |
| <code>interface.8.http-gateway-post-v1-programs-upgrade</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 50 callers total |
| <code>interface.8.http-gateway-post-v1-programs-wind-down</code> | notes | gateway programs ingress; stale task 4.7.5 (spec/layerx-merge/spec.kvx:16652) removes the program routes (spec/layerx-merge/spec.kvx:12567); 55 callers total |
| <code>interface.8.http-gateway-post-wallet-api-push-subscribe</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-gateway-ui-get-explorer-api-tokens-hash-instances-id-media-type-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-explorer-node-api-tokens-hash-instances-id-media-type-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-addresses-address-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-domains-name-events-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-pns-api-v1-domains-name-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-points-balance-address-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-metadata-metadata-tokenaddress-json-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-stats-stats-pooladdress-holders-distribution-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-sdk-stats-stats-pooladdress-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-sidiora-logo-address-png-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-token-icon-address-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-dex-history-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-performance-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-profile-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-address-rank-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-charts-symbol-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-holdings-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-pnl-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-tx-volume-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-charts-value-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v1-portfolio-address-pnl-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-by-day-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-counters-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-logs-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-nft-collections-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-nft-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-tabs-counters-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-token-balances-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-token-transfers-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-tokens-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-addresses-addresshash-transactions-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-tokens-tokenhash-holders-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-tokens-tokenhash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-logs-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-get-wallet-api-wallet-api-v2-transactions-txhash-token-transfers-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-dex-history-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-performance-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-profile-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-address-rank-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-charts-symbol-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-holdings-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-pnl-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-tx-volume-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-charts-value-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v1-portfolio-address-pnl-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-by-day-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-coin-balance-history-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-counters-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-logs-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-nft-collections-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-nft-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-tabs-counters-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-token-balances-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-token-transfers-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-tokens-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-addresses-addresshash-transactions-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-tokens-tokenhash-holders-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-tokens-tokenhash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-logs-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-head-wallet-api-wallet-api-v2-transactions-txhash-token-transfers-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |
| <code>interface.8.http-gateway-ui-post-wallet-api-chat-j69</code> | notes | entry of the const API allowlist at services/platform/hosted/gateway/src/ui_proxy.rs:1314; the gateway forwards it to the wallet or explorer UI upstream; reached by browser requests of the proxied UI, so callers is empty |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.44.1</code> | req | 8044.1 |
| <code>acceptance.8.44.1</code> | scenario | Given the explorer route allowlist, when selected is asked for GET /api/v2/paxeer-x/anchors and GET /api/v2/paxeer-x/receipts, then the first is refused and the second is admitted |
| <code>acceptance.8.44.1</code> | test | services/platform/hosted/gateway/src/explorer_proxy.rs::explorer_refuses_the_retired_anchor_view (planned) |
| <code>acceptance.8.44.1</code> | qualification | UNRUN |
| <code>req.8044</code> | ac_1 | GET /api/v2/paxeer-x/anchors is not an admitted explorer route while GET /api/v2/paxeer-x/receipts still is. |
| <code>acceptance.8.45.1</code> | req | 8045.1 |
| <code>acceptance.8.45.1</code> | scenario | Given a claim body carrying a receipt with a trailing byte and another whose leaf hash differs, when claim parses each, then the first is refused with Refusal::Receipt and the second with Refusal::Evidence |
| <code>acceptance.8.45.1</code> | test | services/platform/hosted/gateway/src/settlement.rs::settlement_claim_refuses_a_receipt_that_does_not_reencode (planned) |
| <code>acceptance.8.45.1</code> | qualification | UNRUN |
| <code>req.8045</code> | ac_1 | settlement::claim (services/platform/hosted/gateway/src/settlement.rs:124) refuses a receipt whose decode and re-encode differ with the receipt reason and a leaf mismatch with the evidence reason. |
| <code>acceptance.8.45.2</code> | req | 8045.2 |
| <code>acceptance.8.45.2</code> | scenario | Given a canonical receipt and a transaction inclusion whose log topics differ from it, when transaction_committed checks them, then the answer is not Ok(true) |
| <code>acceptance.8.45.2</code> | test | services/platform/hosted/gateway/src/settlement.rs::settlement_refuses_an_inclusion_whose_logs_differ (planned) |
| <code>acceptance.8.45.2</code> | qualification | UNRUN |
| <code>req.8045</code> | ac_2 | transaction_committed (services/platform/hosted/gateway/src/settlement.rs:271) does not answer committed for an inclusion whose logs differ from the receipt. |
| <code>acceptance.8.73.1</code> | req | 8073.1 |
| <code>acceptance.8.73.1</code> | scenario | Given a gateway whose consensus listener answers status with catching_up false and then true, when /readyz is read after each answer, then the first document is ready with consensus ready and the second is 503 with consensus not ready |
| <code>acceptance.8.73.1</code> | test | services/platform/hosted/gateway/tests/chain_reads.rs::readyz_reports_the_consensus_endpoint (planned) |
| <code>acceptance.8.73.1</code> | qualification | UNRUN |
| <code>req.8073</code> | ac_1 | GET /readyz of a gateway with a consensus endpoint reports consensus ready while status answers catching_up false and not ready, with the gateway not ready, once it answers catching_up true. |
| <code>acceptance.8.74.1</code> | req | 8074.1 |
| <code>acceptance.8.74.1</code> | scenario | Given a gateway with an explorer upstream listener, when a client gets both kernel program paths for a canonical program id, then both answer 404 not_found and the listener saw no request |
| <code>acceptance.8.74.1</code> | test | services/platform/hosted/gateway/tests/unified_service_paths.rs::explorer_program_paths_are_not_proxied (planned) |
| <code>acceptance.8.74.1</code> | qualification | UNRUN |
| <code>req.8074</code> | ac_1 | GET /v1/programs/&lt;64 hex&gt; and GET /v1/programs/&lt;64 hex&gt;/reads/resolve answer 404 not_found from the gateway and reach no explorer upstream. |
| <code>acceptance.8.75.1</code> | req | 8075.1 |
| <code>acceptance.8.75.1</code> | scenario | Given the twelve retired program routes and POST /v1/activities, when each is parsed by production_route, then each answers InvalidRoute and the three kept routes answer Settle, State and Receipt |
| <code>acceptance.8.75.1</code> | test | services/platform/hosted/gateway/src/lib.rs::production_route_refuses_every_program_and_activity_route (planned) |
| <code>acceptance.8.75.1</code> | qualification | UNRUN |
| <code>req.8075</code> | ac_1 | production_route refuses POST /v1/activities and every /v1/programs path with InvalidRoute while POST /v1/settle, GET /v1/state and GET /v1/receipts/&lt;id&gt; still parse. |
| <code>acceptance.8.76.1</code> | req | 8076.1 |
| <code>acceptance.8.76.1</code> | scenario | Given the gateway tests directory without its local harness, when it is listed and every file is searched for /v1/activities, then the four scripts are absent and nothing is found |
| <code>acceptance.8.76.1</code> | test | services/platform/hosted/gateway/tests/test_retired_activity_scripts.py::test_gateway_tests_ship_no_activity_script (planned) |
| <code>acceptance.8.76.1</code> | qualification | UNRUN |
| <code>req.8076</code> | ac_1 | hosted-boundary.sh, run-load.sh, load.js and lifecycle-boundary.sh are absent from the gateway tests and no other gateway test script outside the local harness posts to /v1/activities. |
| <code>capability.8.6.1</code> | title | Stop proxying the explorer anchor view |
| <code>capability.8.6.1</code> | reqs | 8044.1 |
| <code>capability.8.6.2</code> | title | Unit-test the settlement claim parser and sequencer commitment checks |
| <code>capability.8.6.2</code> | reqs | 8045.1<br>8045.2 |
| <code>capability.8.7.1</code> | title | Report the consensus endpoint in gateway readiness |
| <code>capability.8.7.1</code> | reqs | 8073.1 |
| <code>capability.8.7.2</code> | title | Stop proxying the kernel program explorer paths through the gateway |
| <code>capability.8.7.2</code> | reqs | 8074.1 |
| <code>capability.8.7.3</code> | title | Retire the kernel activity and program production routes of the gateway |
| <code>capability.8.7.3</code> | reqs | 8075.1 |
| <code>capability.8.7.4</code> | title | Retire the kernel activity scripts of the gateway tests |
| <code>capability.8.7.4</code> | reqs | 8076.1 |
| <code>resolution.17.803</code> | title | decision.808 binds: Kernel program surfaces of the hosted services retire rather than translate |
| <code>resolution.17.803</code> | applies_to | decision.808<br>task.8.7.2<br>task.8.7.3<br>task.8.8.2<br>task.8.8.3 |
| <code>resolution.17.803</code> | source | decision.808 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.835</code> | title | decision.891 binds: Retirement of the gateway receipt views wins over the explorer receipt consumer |
| <code>resolution.17.835</code> | applies_to | decision.891<br>task.8.6.1<br>task.14.23.3<br>task.14.32.3<br>task.14.32.4 |
| <code>resolution.17.835</code> | source | decision.891 in fragments/J79/decision.kvx; tasks from affects |

## gateway-jsonrpc-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rpc-gateway-eth-accounts-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:18</code> | <code>"eth_accounts",</code> |
| <code>interface.8.rpc-gateway-eth-coinbase-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:19</code> | <code>"eth_coinbase",</code> |
| <code>interface.8.rpc-gateway-eth-mining-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:25</code> | <code>"eth_mining",</code> |
| <code>interface.8.rpc-gateway-eth-net-web3-relay</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:12</code> | <code>pub const EVM_NAMESPACES: [&amp;str; 3] = ["eth_", "net_", "web3_"];</code> |
| <code>interface.8.rpc-gateway-eth-sendrawtransaction</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/capabilities.rs:26</code> | <code>["eth_sendRawTransaction", "eth_sendRawTransactionSync"];</code> |
| <code>interface.8.rpc-gateway-eth-sendrawtransactionsync</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/capabilities.rs:26</code> | <code>["eth_sendRawTransaction", "eth_sendRawTransactionSync"];</code> |
| <code>interface.8.rpc-gateway-eth-sendtransaction-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:20</code> | <code>"eth_sendTransaction",</code> |
| <code>interface.8.rpc-gateway-eth-sign-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:21</code> | <code>"eth_sign",</code> |
| <code>interface.8.rpc-gateway-eth-signtransaction-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:22</code> | <code>"eth_signTransaction",</code> |
| <code>interface.8.rpc-gateway-eth-signtypeddata-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:23</code> | <code>"eth_signTypedData",</code> |
| <code>interface.8.rpc-gateway-eth-signtypeddata-v4-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:24</code> | <code>"eth_signTypedData_v4",</code> |
| <code>interface.8.rpc-gateway-eth-subscribe</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:30</code> | <code>pub const SUBSCRIPTION_METHODS: [&amp;str; 2] = ["eth_subscribe", "eth_unsubscribe"];</code> |
| <code>interface.8.rpc-gateway-eth-subscription</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/ws.rs:136</code> | <code>if value["method"] != "eth_subscription"</code> |
| <code>interface.8.rpc-gateway-eth-unsubscribe</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/evm.rs:30</code> | <code>pub const SUBSCRIPTION_METHODS: [&amp;str; 2] = ["eth_subscribe", "eth_unsubscribe"];</code> |
| <code>interface.8.rpc-gateway-lx-estimatefee</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:346</code> | <code>&#124; "lx_estimateFee"</code> |
| <code>interface.8.rpc-gateway-lx-getaccount</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:286</code> | <code>"lx_getAccount" &#124; "lx_getBalance" &#124; "lx_getSequence" =&gt; "/v1/accounts/",</code> |
| <code>interface.8.rpc-gateway-lx-getactivitystatus</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:349</code> | <code>&#124; "lx_getActivityStatus"</code> |
| <code>interface.8.rpc-gateway-lx-getasset</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:354</code> | <code>&#124; "lx_getAsset"</code> |
| <code>interface.8.rpc-gateway-lx-getbalance</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:318</code> | <code>"lx_getBalance" &#124; "lx_getSequence" =&gt; "/balance",</code> |
| <code>interface.8.rpc-gateway-lx-getbalances</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:306</code> | <code>} else if method == "lx_getBalances" {</code> |
| <code>interface.8.rpc-gateway-lx-getbatchheader</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:297</code> | <code>if method == "lx_getBatchHeader" {</code> |
| <code>interface.8.rpc-gateway-lx-getcheckpoint</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:351</code> | <code>&#124; "lx_getCheckpoint"</code> |
| <code>interface.8.rpc-gateway-lx-getnodeinfo</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:278</code> | <code>if method == "lx_getNodeInfo" {</code> |
| <code>interface.8.rpc-gateway-lx-getprogramevents</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:347</code> | <code>&#124; "lx_getProgramEvents"</code> |
| <code>interface.8.rpc-gateway-lx-getproof</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:250</code> | <code>if method == "lx_getProof" {</code> |
| <code>interface.8.rpc-gateway-lx-getreceipt</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:348</code> | <code>&#124; "lx_getReceipt"</code> |
| <code>interface.8.rpc-gateway-lx-getsequence</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:237</code> | <code>if method == "lx_getSequence" {</code> |
| <code>interface.8.rpc-gateway-lx-getwalletcaps</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:368</code> | <code>if method == "lx_getWalletCaps" {</code> |
| <code>interface.8.rpc-gateway-lx-listassets</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:274</code> | <code>if method == "lx_listAssets" {</code> |
| <code>interface.8.rpc-gateway-lx-requestfunds</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:340</code> | <code>"lx_requestFunds" =&gt; Some(KernelBackend::Identity),</code> |
| <code>interface.8.rpc-gateway-lx-sendactivity</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:341</code> | <code>"lx_sendActivity" =&gt; Some(KernelBackend::Component),</code> |
| <code>interface.8.rpc-gateway-lx-subscribe</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:404</code> | <code>if matches!(method, "lx_subscribe" &#124; "lx_unsubscribe") {</code> |
| <code>interface.8.rpc-gateway-lx-subscription</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/ws.rs:593</code> | <code>"lx_subscribe" &#124; "lx_unsubscribe" &#124; "lx_subscription"</code> |
| <code>interface.8.rpc-gateway-lx-unsubscribe</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/hosted/gateway/src/rpc.rs:404</code> | <code>if matches!(method, "lx_subscribe" &#124; "lx_unsubscribe") {</code> |
| <code>interface.8.rpc-gateway-pax-sequencerreceipt</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>pax_sequencerReceipt</code> |
| <code>interface.8.rpc-gateway-px-getaccount</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:809</code> | <code>"px_getAccount" =&gt; get_account(config, id, params),</code> |
| <code>interface.8.rpc-gateway-px-getbalances</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:810</code> | <code>"px_getBalances" =&gt; get_balances(config, id, params),</code> |
| <code>interface.8.rpc-gateway-px-getcapabilities</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:813</code> | <code>"px_getCapabilities" =&gt; super::capabilities::get(config, id, params),</code> |
| <code>interface.8.rpc-gateway-px-getcommit</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>px_getCommit</code> |
| <code>interface.8.rpc-gateway-px-gethistory</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:546</code> | <code>"px_getHistory" =&gt; {</code> |
| <code>interface.8.rpc-gateway-px-getnetwork</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:812</code> | <code>"px_getNetwork" =&gt; get_network(config, id, params),</code> |
| <code>interface.8.rpc-gateway-px-getreceiptkeys</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>px_getReceiptKeys</code> |
| <code>interface.8.rpc-gateway-px-getroutecatalogue</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc.rs:375</code> | <code>if method == "px_getRouteCatalogue" {</code> |
| <code>interface.8.rpc-gateway-px-getsequencebatch</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>px_getSequenceBatch</code> |
| <code>interface.8.rpc-gateway-px-getunifiedhistory</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:577</code> | <code>let unified = method == "px_getUnifiedHistory";</code> |
| <code>interface.8.rpc-gateway-px-getvalidators</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>px_getValidators</code> |
| <code>interface.8.rpc-gateway-px-listassets</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:811</code> | <code>"px_listAssets" =&gt; list_assets(config, id, params),</code> |
| <code>interface.8.rpc-gateway-px-listholderrecords</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/history.rs:769</code> | <code>"px_listHolderRecords" =&gt; Some(answer_holders(config, id, params)),</code> |
| <code>interface.8.rpc-gateway-px-register</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/rpc_register.rs:404</code> | <code>method == "px_register"</code> |
| <code>interface.8.rpc-gateway-px-requestfunds</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/paxeer.rs</code> (planned) | <code>px_requestFunds</code> |
| <code>interface.8.rpc-gateway-px-resolveaccount</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/paxeer.rs:808</code> | <code>"px_resolveAccount" =&gt; resolve_account(config, id, params),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rpc-gateway-eth-accounts-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it; 83 callers total |
| <code>interface.8.rpc-gateway-eth-coinbase-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it |
| <code>interface.8.rpc-gateway-eth-mining-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it |
| <code>interface.8.rpc-gateway-eth-net-web3-relay</code> | notes | namespace relay to the node through the dispatch at services/platform/hosted/gateway/src/paxeer.rs:814; namespace relay; callers are every EVM client |
| <code>interface.8.rpc-gateway-eth-sendrawtransaction</code> | notes | 161 callers total |
| <code>interface.8.rpc-gateway-eth-sendrawtransactionsync</code> | notes | no in-repo caller outside the defining file |
| <code>interface.8.rpc-gateway-eth-sendtransaction-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it; 101 callers total |
| <code>interface.8.rpc-gateway-eth-sign-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it; 40 callers total |
| <code>interface.8.rpc-gateway-eth-signtransaction-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it |
| <code>interface.8.rpc-gateway-eth-signtypeddata-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it |
| <code>interface.8.rpc-gateway-eth-signtypeddata-v4-refused</code> | notes | NODE_SIGNING_METHODS: the public endpoint never relays it |
| <code>interface.8.rpc-gateway-lx-estimatefee</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getaccount</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 27 callers total |
| <code>interface.8.rpc-gateway-lx-getactivitystatus</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getasset</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getbalance</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getbalances</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 27 callers total |
| <code>interface.8.rpc-gateway-lx-getbatchheader</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getcheckpoint</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getnodeinfo</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getprogramevents</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); stale task asserts its absence (spec/layerx-merge/spec.kvx:20620); 34 callers total |
| <code>interface.8.rpc-gateway-lx-getproof</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-getreceipt</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 42 callers total |
| <code>interface.8.rpc-gateway-lx-getsequence</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 57 callers total |
| <code>interface.8.rpc-gateway-lx-getwalletcaps</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); stale task asserts its absence (spec/layerx-merge/spec.kvx:20620) |
| <code>interface.8.rpc-gateway-lx-listassets</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 35 callers total |
| <code>interface.8.rpc-gateway-lx-requestfunds</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); replaced by px_requestFunds (spec/layerx-merge/spec.kvx:20889) |
| <code>interface.8.rpc-gateway-lx-sendactivity</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); 66 callers total |
| <code>interface.8.rpc-gateway-lx-subscribe</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); answers -32004 at HEAD instead of -32601; 36 callers total |
| <code>interface.8.rpc-gateway-lx-subscription</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575) |
| <code>interface.8.rpc-gateway-lx-unsubscribe</code> | notes | served at HEAD; stale design switches lx_* ingress off so it answers -32601 at activation (spec/layerx-merge/spec.kvx:109, spec/layerx-merge/spec.kvx:575); answers -32004 at HEAD instead of -32601 |
| <code>interface.8.rpc-gateway-pax-sequencerreceipt</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:668; 58 callers total |
| <code>interface.8.rpc-gateway-px-getbalances</code> | notes | 35 callers total |
| <code>interface.8.rpc-gateway-px-getcommit</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:27718 |
| <code>interface.8.rpc-gateway-px-gethistory</code> | notes | 27 callers total |
| <code>interface.8.rpc-gateway-px-getnetwork</code> | notes | 42 callers total |
| <code>interface.8.rpc-gateway-px-getreceiptkeys</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:27718; 29 callers total |
| <code>interface.8.rpc-gateway-px-getsequencebatch</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:553; planned; no callers at HEAD |
| <code>interface.8.rpc-gateway-px-getunifiedhistory</code> | notes | 31 callers total |
| <code>interface.8.rpc-gateway-px-getvalidators</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:27718 |
| <code>interface.8.rpc-gateway-px-listassets</code> | notes | 27 callers total |
| <code>interface.8.rpc-gateway-px-requestfunds</code> | notes | planned router method; absent at HEAD; stale design spec/layerx-merge/spec.kvx:20889; planned; no callers at HEAD |
| <code>interface.8.rpc-gateway-px-resolveaccount</code> | notes | 37 callers total |

Contract context <code>contract_context.8.router-jsonrpc</code>:

| Field | Frozen rule |
|---|---|
| interface | JSON-RPC 2.0 over POST /rpc and WebSocket /rpc/ws of the platform gateway; px_ router methods, the relayed eth_, net_, web3_ namespaces and pax_sequencerReceipt |
| storage | none; the router holds no chain state, receipt keys load from LAYERX_GATEWAY_RECEIPT_KEYS_FILE |
| errors | -32600 invalid request, -32601 method not found (every lx_ name after task 8.3.1), -32602 invalid params, -32003 upstream unavailable, -32004 WebSocket required for HTTP eth_subscribe |
| events | none |
| gas | none; reads are relayed, sends are eth_sendRawTransaction relays |
| activation | lx_ switch-off lands in wave 6 after clients are repointed in wave 5 |
| invariants | batches hold 1 to 32 requests; every relayed answer keeps the caller id; no lx_ name reaches a kernel backend after task 8.3.1 |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.1.1</code> | req | 8001.1 |
| <code>acceptance.8.1.1</code> | scenario | Given the router method table of the gateway library, when the library test reads ROUTER_METHODS, then it holds 17 names in strictly increasing bytewise order and every name starts with px_ or pax_ |
| <code>acceptance.8.1.1</code> | test | services/platform/hosted/gateway/src/lib.rs::router_methods_are_sorted_unique_and_namespaced (planned) |
| <code>acceptance.8.1.1</code> | qualification | UNRUN |
| <code>req.8001</code> | ac_1 | ROUTER_METHODS (planned in services/platform/hosted/gateway/src/lib.rs) holds 17 bytewise-sorted unique names, each starting px_ or pax_. |
| <code>acceptance.8.1.2</code> | req | 8001.2 |
| <code>acceptance.8.1.2</code> | scenario | Given the router method predicate, when it is asked about the 22 lx_ names, eth_chainId and px_getBalances, then it answers false for each lx_ name and true for eth_chainId and px_getBalances |
| <code>acceptance.8.1.2</code> | test | services/platform/hosted/gateway/src/lib.rs::router_method_refuses_every_lx_name (planned) |
| <code>acceptance.8.1.2</code> | qualification | UNRUN |
| <code>req.8001</code> | ac_2 | router_method (planned) answers false for every lx_ name served at services/platform/hosted/gateway/src/rpc.rs:338-359 and true for eth_chainId and px_getBalances. |
| <code>acceptance.8.152.1</code> | req | 8152.1 |
| <code>acceptance.8.152.1</code> | scenario | Given a gateway with chain, consensus and indexer listeners and no kernel backend, when every router method is posted with its declared params, then no answer is method not found |
| <code>acceptance.8.152.1</code> | test | services/platform/hosted/gateway/tests/router_surface.rs::router_answers_every_listed_method (planned) |
| <code>acceptance.8.152.1</code> | qualification | UNRUN |
| <code>req.8152</code> | ac_1 | Each ROUTER_METHODS name posted to a chain-only gateway answers a JSON-RPC result or a named refusal and none answers -32601. |
| <code>acceptance.8.153.1</code> | req | 8153.1 |
| <code>acceptance.8.153.1</code> | scenario | Given a gateway started without any kernel variable and with chain, consensus and indexer listeners, when the four methods and /readyz are read, then each answers a result and readiness is 200 |
| <code>acceptance.8.153.1</code> | test | services/platform/hosted/gateway/tests/router_surface.rs::chain_only_router_serves_receipts_balances_and_assets (planned) |
| <code>acceptance.8.153.1</code> | qualification | UNRUN |
| <code>req.8153</code> | ac_1 | With every kernel-side variable unset, the gateway answers px_getBalances, px_listAssets, px_getCapabilities and pax_sequencerReceipt from the chain and indexer listeners and reports ready. |
| <code>acceptance.8.3.1</code> | req | 8003.1 |
| <code>acceptance.8.3.1</code> | scenario | Given a gateway whose consensus endpoint is a local listener answering commit for height 5, when a client posts px_getCommit with ["5"] and then ["0"], then the first answer carries height 5 and the listener's signedHeader unchanged and the second answers -32602 |
| <code>acceptance.8.3.1</code> | test | services/platform/hosted/gateway/tests/chain_reads.rs::px_get_commit_answers_the_signed_header_of_a_height (planned) |
| <code>acceptance.8.3.1</code> | qualification | UNRUN |
| <code>req.8003</code> | ac_1 | px_getCommit ["5"] answers the signedHeader the consensus commit route (consensus/internal/rpc/core/routes.go:49) returns for height 5, and ["0"] answers -32602. |
| <code>acceptance.8.3.2</code> | req | 8003.2 |
| <code>acceptance.8.3.2</code> | scenario | Given a gateway whose consensus endpoint answers validators for height 5 with two validators, when a client posts px_getValidators with ["5"] and later the listener is stopped, then the first answer lists both validators with total 2 and the second answers -32003 consensus_unavailable |
| <code>acceptance.8.3.2</code> | test | services/platform/hosted/gateway/tests/chain_reads.rs::px_get_validators_answers_the_set_at_a_height (planned) |
| <code>acceptance.8.3.2</code> | qualification | UNRUN |
| <code>req.8003</code> | ac_2 | px_getValidators ["5"] answers the validator addresses, public keys and voting powers the consensus validators route (consensus/internal/rpc/core/routes.go:54) returns, and an unreachable endpoint answers -32003. |
| <code>acceptance.8.3.3</code> | req | 8003.3 |
| <code>acceptance.8.3.3</code> | scenario | Given a gateway started with a two-entry receipt key file, when a client posts px_getReceiptKeys with [], then the answer lists both entries in file order with their key ids, public keys and height ranges |
| <code>acceptance.8.3.3</code> | test | services/platform/hosted/gateway/tests/receipt_keys.rs::receipt_keys_method_answers_the_pinned_list (planned) |
| <code>acceptance.8.3.3</code> | qualification | UNRUN |
| <code>req.8003</code> | ac_3 | px_getReceiptKeys answers the keyId, publicKey, firstHeight and lastHeight of every entry of the pinned LAYERX_GATEWAY_RECEIPT_KEYS_FILE list (services/platform/hosted/gateway/src/lib.rs:1311). |
| <code>acceptance.8.38.1</code> | req | 8038.1 |
| <code>acceptance.8.38.1</code> | scenario | Given a probe whose node answers one word for 0x101B and nine words for 0x101D, when the capabilities are probed, then pay and programs are live with their addresses and a one-word answer from 0x101D marks programs absent |
| <code>acceptance.8.38.1</code> | test | services/platform/hosted/gateway/src/capabilities.rs::pay_and_programs_surfaces_probe_their_served_views (planned) |
| <code>acceptance.8.38.1</code> | qualification | UNRUN |
| <code>req.8038</code> | ac_1 | The capability probe sends budgetRemaining(bytes32) to 0x101B and getCurrentFeeSchedule() to 0x101D and reports pay and programs live only when the answers decode to 1 and 9 words. |
| <code>acceptance.8.38.2</code> | req | 8038.2 |
| <code>acceptance.8.38.2</code> | scenario | Given the served ABI file of every capability surface, when the test looks up each probe view signature, then each is a view function of that ABI with the declared output |
| <code>acceptance.8.38.2</code> | test | services/platform/hosted/gateway/src/capabilities.rs::capabilities_probe_views_exist_in_the_served_abi_of_each_surface (existing) |
| <code>acceptance.8.38.2</code> | qualification | UNRUN |
| <code>req.8038</code> | ac_2 | Every surface of Surface::ALL, including pay and programs, names a probe view that exists in its served ABI file with the declared output type. |
| <code>acceptance.8.39.1</code> | req | 8039.1 |
| <code>acceptance.8.39.1</code> | scenario | Given a probe whose node answers the declared words for 0x101A and reverts for 0x101C, when the capabilities are probed, then ledger is live and auth is absent while the other surfaces keep their answers |
| <code>acceptance.8.39.1</code> | test | services/platform/hosted/gateway/src/capabilities.rs::ledger_and_auth_surfaces_probe_their_served_views (planned) |
| <code>acceptance.8.39.1</code> | qualification | UNRUN |
| <code>req.8039</code> | ac_1 | The capability probe reports ledger (0x101A) and auth (0x101C) live only when their probe views answer the word count of their served ABI files. |
| <code>acceptance.8.4.1</code> | req | 8004.1 |
| <code>acceptance.8.4.1</code> | scenario | Given a local consensus listener serving a block whose transaction 0 is a MsgSequenceBatch, when a client posts px_getSequenceBatch for that height, then the answer's tx equals transaction 0 and its proof recomputes the header data_hash |
| <code>acceptance.8.4.1</code> | test | services/platform/hosted/gateway/tests/chain_reads.rs::px_get_sequence_batch_proves_tx_zero_under_the_data_hash (planned) |
| <code>acceptance.8.4.1</code> | qualification | UNRUN |
| <code>req.8004</code> | ac_1 | px_getSequenceBatch [height] answers transaction 0 of the block at that height with a proof whose root equals the header data_hash returned by the consensus block method (consensus/internal/rpc/core/routes.go:46). |
| <code>acceptance.8.4.2</code> | req | 8004.2 |
| <code>acceptance.8.4.2</code> | scenario | Given a local consensus listener serving a block whose transaction 0 is a bank send, when a client posts px_getSequenceBatch for that height, then the answer is error -32602 with data.code no_sequence_batch |
| <code>acceptance.8.4.2</code> | test | services/platform/hosted/gateway/tests/chain_reads.rs::px_get_sequence_batch_refuses_a_block_without_a_batch (planned) |
| <code>acceptance.8.4.2</code> | qualification | UNRUN |
| <code>req.8004</code> | ac_2 | px_getSequenceBatch for a block whose transaction 0 is not a MsgSequenceBatch answers -32602 with data.code no_sequence_batch (planned). |
| <code>acceptance.8.40.1</code> | req | 8040.1 |
| <code>acceptance.8.40.1</code> | scenario | Given a chain listener answering eth_blockNumber 0x10 and all_balances with two coins for the bound address, when a client posts px_getBalances for the DID, then the answer has height 16 and both coins and every eth_call the listener saw names block 0x10 |
| <code>acceptance.8.40.1</code> | test | services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_balances_reads_bank_all_balances_at_one_height (planned) |
| <code>acceptance.8.40.1</code> | qualification | UNRUN |
| <code>req.8040</code> | ac_1 | px_getBalances answers the coin list of bank all_balances (precompiles/bank/abi.json) for the bound EVM address with every eth_call pinned to the height eth_blockNumber returned. |
| <code>acceptance.8.40.2</code> | req | 8040.2 |
| <code>acceptance.8.40.2</code> | scenario | Given a chain-only gateway with no public core URL, when a client posts px_getAccount for a bound DID, then the answer holds the account and paxeer members and no layerx member |
| <code>acceptance.8.40.2</code> | test | services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_account_reads_no_kernel_backend (planned) |
| <code>acceptance.8.40.2</code> | qualification | UNRUN |
| <code>req.8040</code> | ac_2 | px_getAccount answers from the chain alone with no kernel public core configured and its answer carries no layerx member. |
| <code>acceptance.8.41.1</code> | req | 8041.1 |
| <code>acceptance.8.41.1</code> | scenario | Given an indexer listing one bank denom and one ledger asset and a chain listener answering bank and ledger views, when a client posts px_listAssets, then both assets carry their metadata at one height and the listener saw no call to 0x1013 |
| <code>acceptance.8.41.1</code> | test | services/platform/hosted/gateway/tests/one_source_reads.rs::px_list_assets_joins_indexer_assets_with_chain_metadata (planned) |
| <code>acceptance.8.41.1</code> | qualification | UNRUN |
| <code>req.8041</code> | ac_1 | px_listAssets lists the indexer assets with bank metadata for bank denoms and ledger getAsset metadata for ledger assets, all read at one height, and never calls 0x1013. |
| <code>acceptance.8.42.1</code> | req | 8042.1 |
| <code>acceptance.8.42.1</code> | scenario | Given a chain listener and a consensus listener answering status at height 9, when a client posts px_getNetwork, then consensus.latest_height is 9 with the listener's hashes and the answer has no anchor and no kernel member |
| <code>acceptance.8.42.1</code> | test | services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_network_reports_the_consensus_head (planned) |
| <code>acceptance.8.42.1</code> | qualification | UNRUN |
| <code>req.8042</code> | ac_1 | px_getNetwork answers the consensus latest height, block hash and app hash from the status method and carries no anchor or kernel member. |
| <code>acceptance.8.5.1</code> | req | 8005.1 |
| <code>acceptance.8.5.1</code> | scenario | Given the EVM relay predicate, when it is asked about pax_sequencerReceipt, pax_other, pax_ and eth_sign, then only pax_sequencerReceipt is relayable |
| <code>acceptance.8.5.1</code> | test | services/platform/hosted/gateway/src/evm.rs::sequencer_receipt_is_the_only_relayed_pax_name (planned) |
| <code>acceptance.8.5.1</code> | qualification | UNRUN |
| <code>req.8005</code> | ac_1 | relayable_method (services/platform/hosted/gateway/src/evm.rs:95) answers true for pax_sequencerReceipt and false for pax_other, pax_ and eth_sign. |
| <code>acceptance.8.5.2</code> | req | 8005.2 |
| <code>acceptance.8.5.2</code> | scenario | Given a gateway relaying to a local chain listener that answers pax_sequencerReceipt with a fixed receipt document, when a client posts pax_sequencerReceipt with a transaction hash, then the client receives the listener's result unchanged with its own request id |
| <code>acceptance.8.5.2</code> | test | services/platform/hosted/gateway/tests/paxeer_failover.rs::pax_sequencer_receipt_relays_the_node_answer (planned) |
| <code>acceptance.8.5.2</code> | qualification | UNRUN |
| <code>req.8005</code> | ac_2 | A pax_sequencerReceipt call posted to the gateway reaches the chain listener and its answer returns unchanged under the caller id, where paxeer::dispatch (services/platform/hosted/gateway/src/paxeer.rs:801) returns None for it at HEAD. |
| <code>acceptance.8.6.1</code> | req | 8006.1 |
| <code>acceptance.8.6.1</code> | scenario | Given the OpenRPC document bundled into the gateway, when the test parses its methods array, then every ROUTER_METHODS name occurs exactly once |
| <code>acceptance.8.6.1</code> | test | services/platform/hosted/gateway/tests/openrpc_methods.rs::openrpc_lists_every_router_method_once (planned) |
| <code>acceptance.8.6.1</code> | qualification | UNRUN |
| <code>req.8006</code> | ac_1 | services/platform/hosted/gateway/openrpc.json has exactly one method entry for each of the 17 ROUTER_METHODS names (planned), including px_getCommit, px_getValidators, px_getReceiptKeys, px_getSequenceBatch, px_requestFunds and pax_sequencerReceipt. |
| <code>acceptance.8.7.1</code> | req | 8007.1 |
| <code>acceptance.8.7.1</code> | scenario | Given the faucet dispatcher, when it receives px_requestFunds with an empty params array and px_unknown with valid params, then the first answers -32602 Invalid params and the second is not handled |
| <code>acceptance.8.7.1</code> | test | services/platform/hosted/gateway/src/rpc_faucet.rs::px_request_funds_uses_the_claim_path (planned) |
| <code>acceptance.8.7.1</code> | qualification | UNRUN |
| <code>req.8007</code> | ac_1 | rpc_faucet::dispatch (services/platform/hosted/gateway/src/rpc_faucet.rs:366) answers px_requestFunds through request_funds and refuses malformed params with -32602. |
| <code>acceptance.8.8.1</code> | req | 8008.1 |
| <code>acceptance.8.8.1</code> | scenario | Given a history source whose indexer answers /v1/assets with two assets and no public core configured, when px_getHistory resolves asset metadata, then the registry holds the two indexer assets |
| <code>acceptance.8.8.1</code> | test | services/platform/hosted/gateway/src/history.rs::history_registry_reads_the_indexer_assets (planned) |
| <code>acceptance.8.8.1</code> | qualification | UNRUN |
| <code>req.8008</code> | ac_1 | The history asset registry (services/platform/hosted/gateway/src/history.rs:503) is read from the indexer /v1/assets route and no request reaches the kernel public core. |
| <code>acceptance.8.9.1</code> | req | 8009.1 |
| <code>acceptance.8.9.1</code> | scenario | Given the router dispatcher with and without kernel backends configured, when each of the 22 lx_ names is dispatched with valid params, then every answer is error -32601 Method not found |
| <code>acceptance.8.9.1</code> | test | services/platform/hosted/gateway/src/rpc.rs::lx_names_answer_method_not_found (planned) |
| <code>acceptance.8.9.1</code> | qualification | UNRUN |
| <code>req.8009</code> | ac_1 | dispatch (services/platform/hosted/gateway/src/rpc.rs:362) answers -32601 for every lx_ name, including lx_sendActivity, lx_subscribe and lx_getWalletCaps, with no backend configured and with every backend configured. |
| <code>acceptance.8.9.2</code> | req | 8009.2 |
| <code>acceptance.8.9.2</code> | scenario | Given the gateway binary started by the unified service harness, when it receives each lx_ read and lx_requestFunds and then px_getHistory, then every lx_ answer is -32601 and px_getHistory is not refused with -32601 or -32602 |
| <code>acceptance.8.9.2</code> | test | services/platform/hosted/gateway/tests/unified_service_paths.rs::lx_reads_switched_off (existing) |
| <code>acceptance.8.9.2</code> | qualification | UNRUN |
| <code>req.8009</code> | ac_2 | A running gateway answers -32601 for the 15 lx_ reads and lx_requestFunds listed at services/platform/hosted/gateway/tests/unified_service_paths.rs:426-441 while px_getHistory and px_getUnifiedHistory are still served. |
| <code>acceptance.8.9.3</code> | req | 8009.3 |
| <code>acceptance.8.9.3</code> | scenario | Given a chain-only gateway with no kernel backend, when a client posts eth_chainId and lx_getAccount, then eth_chainId answers 0x7d and lx_getAccount answers -32601 |
| <code>acceptance.8.9.3</code> | test | services/platform/hosted/gateway/tests/degraded.rs::degraded_endpoint_serves_the_chain_and_refuses_kernel_methods_without_the_kernel (existing) |
| <code>acceptance.8.9.3</code> | qualification | UNRUN |
| <code>req.8009</code> | ac_3 | The chain-only gateway relays eth_chainId and answers lx_getAccount with -32601 instead of the kernel_unavailable refusal asserted at services/platform/hosted/gateway/tests/degraded.rs:560. |
| <code>acceptance.8.999.1</code> | req | 8999.1 |
| <code>acceptance.8.999.1</code> | scenario | Given the candidate after every domain-8 wave-6 task, when platform_completion_retired_surfaces_absent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.8.999.1</code> | test | services/platform/hosted/gateway/tests/completion_gate.rs::platform_completion_retired_surfaces_absent (planned) |
| <code>acceptance.8.999.1</code> | qualification | UNRUN |
| <code>req.8999</code> | ac_1 | platform_completion_retired_surfaces_absent passes on the merged candidate: no retired domain-8 surface remains and every planned domain-8 wave-6 path exists. |
| <code>capability.8.0.9</code> | title | Domain 8 completion gate |
| <code>capability.8.0.9</code> | reqs | 8999.1 |
| <code>capability.8.1.2</code> | title | Serve px_getCommit, px_getValidators and px_getReceiptKeys from the router |
| <code>capability.8.1.2</code> | reqs | 8003.1<br>8003.2<br>8003.3 |
| <code>capability.8.1.3</code> | title | Serve px_getSequenceBatch with the transaction proof of a committed height |
| <code>capability.8.1.3</code> | reqs | 8004.1<br>8004.2 |
| <code>capability.8.1.4</code> | title | Relay pax_sequencerReceipt through the EVM relay namespace |
| <code>capability.8.1.4</code> | reqs | 8005.1<br>8005.2 |
| <code>capability.8.1.91</code> | title | Router public method table |
| <code>capability.8.1.91</code> | reqs | 8001.1<br>8001.2 |
| <code>capability.8.1.92</code> | title | Router surface frozen in OpenRPC |
| <code>capability.8.1.92</code> | reqs | 8006.1 |
| <code>capability.8.15.93</code> | title | Router surface integration |
| <code>capability.8.15.93</code> | reqs | 8152.1 |
| <code>capability.8.15.94</code> | title | Hosted chain cutover gate |
| <code>capability.8.15.94</code> | reqs | 8153.1 |
| <code>capability.8.2.1</code> | title | Serve px_requestFunds over the faucet claim path |
| <code>capability.8.2.1</code> | reqs | 8007.1 |
| <code>capability.8.2.2</code> | title | Read the history asset registry from the indexer |
| <code>capability.8.2.2</code> | reqs | 8008.1 |
| <code>capability.8.3.1</code> | title | Answer -32601 for every lx_ JSON-RPC name on HTTP and WebSocket |
| <code>capability.8.3.1</code> | reqs | 8009.1<br>8009.2<br>8009.3 |
| <code>capability.8.4.2</code> | title | Probe the payments and programs surfaces in px_getCapabilities |
| <code>capability.8.4.2</code> | reqs | 8038.1<br>8038.2 |
| <code>capability.8.4.3</code> | title | Probe the ledger and authority surfaces in px_getCapabilities |
| <code>capability.8.4.3</code> | reqs | 8039.1 |
| <code>capability.8.5.1</code> | title | Answer px_getAccount and px_getBalances from chain state alone |
| <code>capability.8.5.1</code> | reqs | 8040.1<br>8040.2 |
| <code>capability.8.5.2</code> | title | Answer px_listAssets from the indexer asset list and chain metadata |
| <code>capability.8.5.2</code> | reqs | 8041.1 |
| <code>capability.8.5.3</code> | title | Answer px_getNetwork with the consensus head instead of the anchor checkpoint |
| <code>capability.8.5.3</code> | reqs | 8042.1 |
| <code>resolution.17.797</code> | title | decision.802 binds: Faucet funding stays on the faucet, testnet-control and core boundary chain |
| <code>resolution.17.797</code> | applies_to | decision.802<br>task.8.2.1<br>task.8.10.1 |
| <code>resolution.17.797</code> | source | decision.802 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.798</code> | title | decision.803 binds: lx_ switch-off is one change across HTTP, WebSocket and OpenRPC |
| <code>resolution.17.798</code> | applies_to | decision.803<br>task.8.3.1 |
| <code>resolution.17.798</code> | source | decision.803 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.799</code> | title | decision.804 binds: px_getAccount and px_getBalances drop the kernel half |
| <code>resolution.17.799</code> | applies_to | decision.804<br>task.8.5.1 |
| <code>resolution.17.799</code> | source | decision.804 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.801</code> | title | decision.806 binds: Capability probes cover every merged construct with one served view each |
| <code>resolution.17.801</code> | applies_to | decision.806<br>task.8.4.2<br>task.8.4.3 |
| <code>resolution.17.801</code> | source | decision.806 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.810</code> | title | decision.815 binds: Router anchors are integration tests over the real gateway binary |
| <code>resolution.17.810</code> | applies_to | decision.815<br>task.8.0.3<br>task.8.0.4 |
| <code>resolution.17.810</code> | source | decision.815 in fragments/J40/decision.kvx; tasks from affects |
| <code>service.platform-gateway</code> | purpose | Public router of the platform: JSON-RPC px_ methods, the relayed EVM namespaces, WebSocket subscriptions, HTTP product routes and UI proxies |
| <code>service.platform-gateway</code> | route | POST /rpc, GET /rpc/schema, /rpc/ws, /rpc/evm/ws, /livez, /readyz, /v1/settle, /v1/keys, UI proxy prefixes |
| <code>service.platform-gateway</code> | inputs | client JSON-RPC and HTTP requests, LAYERX_GATEWAY_PAXEER_RPC_URLS, LAYERX_GATEWAY_PAXEER_WS_URL, LAYERX_GATEWAY_INDEXER_URL, LAYERX_GATEWAY_RECEIPT_KEYS_FILE |
| <code>service.platform-gateway</code> | outputs | JSON-RPC answers, relayed node answers, readiness documents, settlement receipts |
| <code>service.platform-gateway</code> | state | Redis durable store for keys, idempotency and event production |
| <code>service.platform-gateway</code> | dependencies | Paxeer chain node JSON-RPC and WebSocket, consensus RPC, indexer, identity service, receipt authority |
| <code>service.platform-gateway</code> | failure_policy | an unavailable upstream answers its named refusal (-32003 or 503) and is never replaced by fabricated data; readiness reports each dependency |
| <code>service.platform-gateway</code> | completion_task | 8.0.9 |
| <code>service.platform-gateway</code> | operation_task | 8.3.1 |
| <code>service.platform-gateway</code> | domain | 8 |

## gateway-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.met-gateway-event-producer-failures</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3658</code> | <code>&amp;serde_json::json!({"event_producer_failures": failures, "event_producer_overflow": overflow}),</code> |
| <code>interface.8.met-gateway-event-producer-overflow</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3658</code> | <code>&amp;serde_json::json!({"event_producer_failures": failures, "event_producer_overflow": overflow}),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.met-gateway-event-producer-failures</code> | notes | JSON body of GET /metrics |
| <code>interface.8.met-gateway-event-producer-overflow</code> | notes | JSON body of GET /metrics |

## gateway-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-gateway-evm-merged-constructs-j40</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/evm.rs</code> (planned) | <code>pub const LEDGER_PRECOMPILE, PAY_PRECOMPILE, AUTH_PRECOMPILE, PROGRAMS_PRECOMPILE: [u8; 20]; pub fn decode_coins</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-accesserror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:53</code> | <code>pub enum AccessError {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-activitytype</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:22</code> | <code>pub use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-authenticate-gateway-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:65</code> | <code>pub fn authenticate_gateway_key(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-authority-evidence-maintainedbatchdocument</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/authority_evidence.rs:5</code> | <code>pub struct MaintainedBatchDocument {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-authority-evidence-present-maintained</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/authority_evidence.rs:179</code> | <code>pub fn present_maintained&lt;'de, D: serde::Deserializer&lt;'de&gt;&gt;(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-authorityfacts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:279</code> | <code>pub struct AuthorityFacts {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-configured-sequencer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1564</code> | <code>pub fn configured_sequencer(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-gateway-audit-event</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:124</code> | <code>pub fn gateway_audit_event(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-gateway-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:111</code> | <code>pub fn gateway_digest(parts: &amp;[&amp;[u8]]) -&gt; String {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-gateway-principal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:100</code> | <code>pub fn gateway_principal(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-gatewayerror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:1258</code> | <code>pub enum GatewayError {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:10</code> | <code>pub mod http;</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:187</code> | <code>pub struct Client {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:117</code> | <code>pub struct Endpoint {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http-incomingrequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:960</code> | <code>pub struct IncomingRequest {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http-outboundrequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:236</code> | <code>pub struct OutboundRequest&lt;'a&gt; {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-http-outgoingresponse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/http.rs:976</code> | <code>pub struct OutgoingResponse {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-principalid</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:150</code> | <code>pub struct PrincipalId(String);</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-sequencerauthorization</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:3</code> | <code>pub use layerx_proof::inclusion::SequencerAuthorization;</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-completion</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:2</code> | <code>pub struct Completion&lt;'a&gt; {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-keyrecord</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:93</code> | <code>pub struct KeyRecord {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-redisendpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:49</code> | <code>pub struct RedisEndpoint {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-redisstore</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:82</code> | <code>pub struct RedisStore {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-reservation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:137</code> | <code>pub enum Reservation {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-reservationrequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:14</code> | <code>pub struct ReservationRequest&lt;'a&gt; {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-tapcredentialrecord</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:168</code> | <code>pub struct TapCredentialRecord {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-store-tapnonceconsumption</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/store.rs:186</code> | <code>pub enum TapNonceConsumption {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verifiedoperation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:338</code> | <code>pub struct VerifiedOperation {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verifiedoperation-verification-level</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:541</code> | <code>pub const fn verification_level(&amp;self) -&gt; &amp;'static str {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verifiedsubmission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:351</code> | <code>pub struct VerifiedSubmission {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verifiedtransfer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:360</code> | <code>pub struct VerifiedTransfer {</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verify-activity-operation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:562</code> | <code>pub fn verify_activity_operation(</code> |
| <code>interface.8.rs-gateway-layerx-platform-gateway-verify-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/lib.rs:473</code> | <code>pub fn verify_submission(</code> |
| <code>interface.8.rs-gateway-router-methods-j40</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/lib.rs</code> (planned) | <code>pub const ROUTER_METHODS: [&amp;str; 17]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-gateway-evm-merged-constructs-j40</code> | notes | planned merged construct addresses 0x101A to 0x101D and the bank coin-list decoder; task 8.4.1 |
| <code>interface.8.rs-gateway-router-methods-j40</code> | notes | planned frozen list of router JSON-RPC names; task 8.0.1 |

Contract context <code>contract_context.8.router-one-source-reads</code>:

| Field | Frozen rule |
|---|---|
| interface | px_getAccount, px_getBalances, px_listAssets, px_getNetwork and px_getCapabilities of the platform gateway |
| storage | none; every answer is read from chain state at one height |
| errors | -32602 invalid params, -32003 upstream unavailable with data.code, -32001 unconfigured chain |
| events | none |
| gas | eth_call reads only |
| activation | one-source answers land in waves 3 to 5; custody and anchor reads are removed in wave 6 |
| invariants | every eth_call of one answer names the same block height; no answer reads 0x1013, 0x1014 or a kernel route after wave 6 |
| pause | a paused construct is reported by its capability probe and is not hidden |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.37.1</code> | req | 8037.1 |
| <code>acceptance.8.37.1</code> | scenario | Given the merged construct constants of the EVM table, when each is rendered with address_hex, then the four addresses are 0x000000000000000000000000000000000000101a through 0x000000000000000000000000000000000000101d in order |
| <code>acceptance.8.37.1</code> | test | services/platform/hosted/gateway/src/evm.rs::merged_construct_addresses_are_exact (planned) |
| <code>acceptance.8.37.1</code> | qualification | UNRUN |
| <code>req.8037</code> | ac_1 | LEDGER_PRECOMPILE, PAY_PRECOMPILE, AUTH_PRECOMPILE and PROGRAMS_PRECOMPILE (planned in services/platform/hosted/gateway/src/evm.rs) render as 0x...101a, 0x...101b, 0x...101c and 0x...101d through address_hex. |
| <code>acceptance.8.37.2</code> | req | 8037.2 |
| <code>acceptance.8.37.2</code> | scenario | Given the bank.all_balances vector of the ABI fixture, when decode_coins reads the answer and then the answer cut by one byte, then the first yields both coins with exact amounts and the second is refused |
| <code>acceptance.8.37.2</code> | test | services/platform/hosted/gateway/src/evm.rs::all_balances_answer_decodes_exactly (planned) |
| <code>acceptance.8.37.2</code> | qualification | UNRUN |
| <code>req.8037</code> | ac_2 | decode_coins (planned) decodes the bank.all_balances fixture answer into its two (denom, amount) pairs and refuses a truncated answer. |
| <code>acceptance.8.43.1</code> | req | 8043.1 |
| <code>acceptance.8.43.1</code> | scenario | Given the trimmed EVM table and ABI fixture, when the vector test compares every remaining selector and address, then all match and no case names custody or anchor |
| <code>acceptance.8.43.1</code> | test | services/platform/hosted/gateway/src/evm.rs::precompile_addresses_and_selectors_match_the_generated_vectors (existing) |
| <code>acceptance.8.43.1</code> | qualification | UNRUN |
| <code>req.8043</code> | ac_1 | Every address and selector left in services/platform/hosted/gateway/src/evm.rs matches the ABI fixture and the fixture holds no custody or anchor case. |
| <code>capability.8.4.1</code> | title | Pin the merged construct addresses and the bank coin-list ABI in the EVM table |
| <code>capability.8.4.1</code> | reqs | 8037.1<br>8037.2 |
| <code>capability.8.4.4</code> | title | Remove the custody and anchor constructs from the EVM table |
| <code>capability.8.4.4</code> | reqs | 8043.1 |
| <code>resolution.17.800</code> | title | decision.805 binds: px_getNetwork reports the consensus head, not an anchor checkpoint |
| <code>resolution.17.800</code> | applies_to | decision.805<br>task.8.4.4<br>task.8.5.3 |
| <code>resolution.17.800</code> | source | decision.805 in fragments/J40/decision.kvx; tasks from affects |

## gateway-ws-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ws-gateway-any-rpc-evm-ws</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3787</code> | <code>if request.path == "/rpc/evm/ws" {</code> |
| <code>interface.8.ws-gateway-any-rpc-ws</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/hosted/gateway/src/main.rs:3790</code> | <code>if request.path == "/rpc/ws" {</code> |
