# Platform gateway contract

The [hosted gateway](../../../services/platform/hosted/gateway) serves configured product routes and root-path JSON-RPC. Its `px_` methods are described by [openrpc.json](../../../services/platform/hosted/gateway/openrpc.json). The node JSON-RPC namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`; gateway-specific method availability depends on the configured gateway.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## gateway-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `schema: String,` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `config_key` | `trusted_proxies: Vec<String>,` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |

## gateway-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_FILES_OWNER_ENDPOINT_UI_BINDINGS` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `env_var` | `LAYERX_GATEWAY_AUTHORITY_TOKEN_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_AUTHORITY_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS` | [services/platform/hosted/gateway/src/capabilities.rs](../../../services/platform/hosted/gateway/src/capabilities.rs) |
| `env_var` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_COMPONENT_TOKEN_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_COMPONENT_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_CONSENSUS_RPC_URL` | [services/platform/hosted/gateway/src/chain_reads.rs](../../../services/platform/hosted/gateway/src/chain_reads.rs) |
| `env_var` | `LAYERX_GATEWAY_EXPLORER_BACKEND_URL` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `env_var` | `LAYERX_GATEWAY_EXPLORER_FRONTEND_URL` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `env_var` | `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE` | [services/platform/hosted/gateway/src/rpc_faucet.rs](../../../services/platform/hosted/gateway/src/rpc_faucet.rs) |
| `env_var` | `LAYERX_GATEWAY_FAUCET_URL` | [services/platform/hosted/gateway/src/rpc_faucet.rs](../../../services/platform/hosted/gateway/src/rpc_faucet.rs) |
| `env_var` | `LAYERX_GATEWAY_HPX_INGRESS_FILE` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `env_var` | `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | [services/platform/hosted/gateway/src/rpc_register.rs](../../../services/platform/hosted/gateway/src/rpc_register.rs) |
| `env_var` | `LAYERX_GATEWAY_IDENTITY_TOKEN_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_IDENTITY_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_INDEXER_URL` | [services/platform/hosted/gateway/src/history.rs](../../../services/platform/hosted/gateway/src/history.rs) |
| `env_var` | `LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_LISTEN` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_LISTENER` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_LXP_WIRE_VERSION` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_MODULE_REGISTRY_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_NETWORK_ID` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_OUTBOUND_CA_DER` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_PAXEER_RPC_URLS` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `env_var` | `LAYERX_GATEWAY_PAXEER_WS_URL` | [services/platform/hosted/gateway/src/ws.rs](../../../services/platform/hosted/gateway/src/ws.rs) |
| `env_var` | `LAYERX_GATEWAY_PROGRAM_REGISTRY_TOKEN_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_PROGRAM_REGISTRY_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_PUBLIC_CORE_URL` | [services/platform/hosted/gateway/src/public_reads.rs](../../../services/platform/hosted/gateway/src/public_reads.rs) |
| `env_var` | `LAYERX_GATEWAY_PUBLIC_HOST` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `env_var` | `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_REDIS_PASSWORD_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_REDIS_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_REDIS_USERNAME_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `env_var` | `LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_TLS_CERT_DER` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_TLS_KEY_DER` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_TRUSTED_PROXIES` | [services/platform/hosted/gateway/src/rpc_register.rs](../../../services/platform/hosted/gateway/src/rpc_register.rs) |
| `env_var` | `LAYERX_GATEWAY_UI_BINDINGS_FILE` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_BINDING_SOCKET` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_CHAIN_ID` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_IDENTITY_GID` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_IDENTITY_UID` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_IDENTITY_URL` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_ISSUER_SOCKET` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_GATEWAY_WALLET_TENANT` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `LAYERX_PAY_TIMING` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `env_var` | `PAXEER_X_GATEWAY_READINESS_CASE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `PAXEER_X_ROUTER_READINESS_CASE` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `env_var` | `PORT` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |

## gateway-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return refusal(id, -32001, "History unavailable", json!({"code": code}))` | [services/platform/hosted/gateway/src/history.rs](../../../services/platform/hosted/gateway/src/history.rs) |
| `error_code` | `return super::rpc::error(id, -32002, "Authentication required");` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return super::rpc::error(id, -32004, "Subscriptions are not relayed");` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `error_code` | `Some(error(&Value::Null, -32600, "Invalid Request"))` | [services/platform/hosted/gateway/src/rpc.rs](../../../services/platform/hosted/gateway/src/rpc.rs) |
| `error_code` | `return super::rpc::error(id, -32601, "Method not relayed");` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `error_code` | `return super::rpc::error(id, -32602, "Invalid params");` | [services/platform/hosted/gateway/src/capabilities.rs](../../../services/platform/hosted/gateway/src/capabilities.rs) |
| `error_code` | `return json_response(200, &error(&Value::Null, -32700, "Parse error"));` | [services/platform/hosted/gateway/src/rpc.rs](../../../services/platform/hosted/gateway/src/rpc.rs) |
| `error_code` | `.map_err(\|_\| response(503, "authority_invalid", Some(5)))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return Err(response(503, "authority_mismatch", Some(5)));` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return Err(response(503, "authority_unavailable", Some(5)));` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(503, "client_identity_required", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `parse_hex32(activity_id).map_err(\|_\| response(503, "component_invalid", Some(5)))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `.map_err(\|_\| response(503, "component_unavailable", Some(5)))` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Err(_) => return Some(response(503, "explorer_authority_unavailable", None)),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `Err(_) => return response(502, "explorer_cookie_refused", None),` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `return response(503, "explorer_not_configured", None);` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `Err(_) => return response(502, "explorer_redirect_refused", None),` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `return response(400, "explorer_target_refused", None);` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `return response(502, "explorer_upstream_unavailable", None);` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `"code": "faucet_claim_pending", "state": "pending", "upstream": document` | [services/platform/hosted/gateway/src/rpc_faucet.rs](../../../services/platform/hosted/gateway/src/rpc_faucet.rs) |
| `error_code` | `return Some(response(400, "hpx_discovery_body_forbidden", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return Some(response(503, "hpx_ingress_provenance_unavailable", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return Some(response(413, "hpx_registration_body_too_large", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return Some(response(400, "hpx_registration_token_invalid", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `Err(_) => response(503, "hpx_upstream_unavailable", None),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `_ => return response(400, "idempotency_key_required", None),` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `let result = match facts.batch_evidence {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(502, "invalid_core_response", None);` | [services/platform/hosted/gateway/src/public_reads.rs](../../../services/platform/hosted/gateway/src/public_reads.rs) |
| `error_code` | `&response(400, "invalid_http_request", None),` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Err(_) => return response(400, "invalid_key_request", None),` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Err(_) => return response(400, "invalid_mcp_request", None),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `None => return response(400, "invalid_mcp_tool", None),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return Some(response(400, "invalid_preflight", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `"reason": "invalid_program_success",` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `"reason": "invalid_program_u64",` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(400, "invalid_quota", None);` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Err(()) => Some(response(400, "invalid_read_selector", None)),` | [services/platform/hosted/gateway/src/public_reads.rs](../../../services/platform/hosted/gateway/src/public_reads.rs) |
| `error_code` | `return response(400, "invalid_scopes", None);` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `.map_err(\|_\| response(400, "invalid_source_publication", None))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Ok(_) => response(502, "invalid_upstream_content_type", None),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return response(400, "invalid_websocket_upgrade", None);` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `return response(415, "json_content_type_required", None);` | [services/platform/hosted/gateway/src/rpc.rs](../../../services/platform/hosted/gateway/src/rpc.rs) |
| `error_code` | `"code": "kernel_unavailable",` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `response(409, "key_limit_reached", None)` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `} else if code.starts_with("LXP_ERR_") {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(503, "mcp_capacity_unavailable", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return response(503, "mcp_deadline_exceeded", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return response(503, "mcp_identity_unavailable", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return response(403, "mcp_owner_binding_required", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return response(404, "not_found", None);` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `error_code` | `response(403, "origin_not_allowed", None)` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `refusal["error"]["data"] = json!({"code": "paxeer_rpc_not_configured"});` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `error_code` | `return Some(response(403, "private_service", None));` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `return Err(response(503, "program_registry_unverified", Some(5)));` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `now_millis().map_err(\|_\| response(503, "program_state_unverified", Some(5)))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(400, "query_string_not_allowed", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `Ok(Some(retry)) => return response(429, "quota_exceeded", Some(retry)),` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(503, "readiness_clock_unavailable", None);` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `response(502, "receipt_key_unauthorized", None)` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Ok(None) => return response(404, "receipt_not_found", None),` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `"reason": "receipt_pending",` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `unauthorized_or(error, response(503, "receipt_verification_failed", Some(5)))` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `response(409, "rotation_conflict", None)` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `"reason": "server_side_receipt_verification_only",` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| response(401, "session_required", None))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(415, "settlement_content_type_required", None);` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `.map_err(\|_\| response(503, "settlement_inclusion_unavailable", Some(5)))?;` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `return response(403, "signer_not_owned", None);` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `error_code` | `Err(_) => return response(502, "state_document_invalid", None),` | [services/platform/hosted/gateway/src/state.rs](../../../services/platform/hosted/gateway/src/state.rs) |
| `error_code` | `return response(502, "state_proof_unverified", None);` | [services/platform/hosted/gateway/src/state.rs](../../../services/platform/hosted/gateway/src/state.rs) |
| `error_code` | `return http::write_response(stream, &super::response(429, "subscription_limit", Some(1)));` | [services/platform/hosted/gateway/src/ws.rs](../../../services/platform/hosted/gateway/src/ws.rs) |
| `error_code` | `"code": "surface_unavailable",` | [services/platform/hosted/gateway/src/capabilities.rs](../../../services/platform/hosted/gateway/src/capabilities.rs) |
| `error_code` | `return response(415, "unsupported_media_type", None);` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `error_code` | `Err(_) => response(503, "upstream_unavailable", None),` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| source | — | [services/platform/hosted/gateway/src/ai_views.rs](../../../services/platform/hosted/gateway/src/ai_views.rs) |

## gateway-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `rest == "/explorer" \|\| rest.starts_with("/explorer/") \|\| rest.starts_with("/explorer?")` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `http_route` | `.is_some_and(\|p\| p == "/explorer/backend" \|\| p.starts_with("/explorer/backend/"))` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `let Some(tail) = path.strip_prefix("/explorer/_next/data/") else {` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `"/gas-station/quote" \| "/gas-station/submit" \| "/gas-station/status" \| "/gas-station/retry"` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `if matches!(path, "/hpx/api/register" \| "/hpx/api/myip") && query.is_some() {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `} else if path.starts_with("/human-ui/") {` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `if request.path != "/mcp" {` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `http_route` | `if request.path == "/v1/routes" {` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `http_route` | `if wallet_contract == Some(false) \|\| path == "/v1/sync/readiness" && query.is_some() {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `\| "/v1/wallet/lx/approve"` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `\| "/v1/wallet/lx/review"` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `\| "/v1/wallet/sign-digest"` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `\| "/v1/wallet/sponsored/status"` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `\| "/v2/migration/assets"` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `http_route` | `Some("/v2/migration/history" \| "/v2/migration/history/read")` | [services/platform/hosted/gateway/src/routes.rs](../../../services/platform/hosted/gateway/src/routes.rs) |
| `http_route` | `let wallet = path == "/wallet" \|\| path.starts_with("/wallet/");` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("DELETE", "/wallet/api/push/subscribe"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `\|\| route.path == "/api/account/auth/logout"` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `http_route` | `\|\| route.path == "/api/account/v2/authenticate_via_dynamic";` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `http_route` | `\|\| route.path == "/api/account/v2/email/resend"` | [services/platform/hosted/gateway/src/explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) |
| `http_route` | `("POST", "/api/register") \| ("GET", "/api/myip")` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `("GET", "/explorer/api/config"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/api/csrf"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/api/healthz"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/api/log"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/api/metrics"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/config"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/csrf"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/healthz"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/log"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/metrics"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `&& (path == "/explorer/verify" \|\| next_static_path(path, "/human-ui/_next/static/")))` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/internal/v1/principal" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/metrics" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `&& (path == "/explorer/verify" \|\| next_static_path(path, "/_next/static/"))` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz/core" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/v1/keys" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `let Some(suffix) = request.path.strip_prefix("/v1/keys/") else {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `("GET", "/v1/state") => Ok(ProductionRoute::State),` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/v1/status" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/bnb/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/bnb/price/"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/eth/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/eth/price/"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/pax/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/pax/price/"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/sol/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/cv/sol/price/"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/pax/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/candle/sid/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/fx/latest/USD"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/health"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/media"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/pns/api/v1/addresses:lookup"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/pns/api/v1/domains:lookup"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/candles/history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/breakout"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/movers"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/new"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/top_volume"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/trending"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/ranking/rankings/unusual"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/stats/stats/batch"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sidiora/metadata"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/token-metadata"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `if path.starts_with("/wallet/api/wallet/") && !matches!(request.method.as_str(), "GET" \| "HEAD")` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/trending"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/main-page/blocks"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/main-page/transactions"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/search"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/stats"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/tokens"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/health"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/api/config"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/api/healthz"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/api/metrics"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/node-api/config"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/node-api/healthz"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/explorer/node-api/metrics"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/trending"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/main-page/blocks"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/main-page/transactions"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/search"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/stats"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/tokens"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/health"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `&& matches!(request.path, "/quote" \| "/submit" \| "/status" \| "/retry")` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `\|\| (method == "POST" && path == "/api/explorer/verify")` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `("POST", "/explorer/api/log"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("POST", "/explorer/api/monitoring/invalid-api-schema"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("POST", "/explorer/node-api/log"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("POST", "/explorer/node-api/monitoring/invalid-api-schema"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `\|\| request.path != "/internal/v2/source-settlements"` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/v1/keys" {` | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
| `http_route` | `("POST", "/v1/settle") => Ok(ProductionRoute::Settle),` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `_ if path.starts_with("/v1/wallet/custody/") => {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `http_route` | `&& path == "/wallet/api/chat"` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `http_route` | `("POST", "/wallet/api/push/subscribe"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/api/tokens/[hash]/instances/[id]/media-type"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/explorer/node-api/tokens/[hash]/instances/[id]/media-type"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/pns/api/v1/addresses/[address]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/pns/api/v1/domains/[name]/events"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/pns/api/v1/domains/[name]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/points/balance/[address]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/metadata/metadata/[tokenAddress].json"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/stats/stats/[poolAddress]/holders/distribution"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sdk/stats/stats/[poolAddress]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/sidiora/logo/[address].png"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/token-icon/[address]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/[address]/dex-history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/[address]/performance"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/[address]/profile"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/[address]/rank"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/charts/[symbol]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/holdings"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/pnl"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/tx-volume"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/value"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v1/portfolio/[address]/pnl"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history-by-day"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/counters"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/logs"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft/collections"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tabs-counters"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-balances"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-transfers"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tokens"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/addresses/[addressHash]/transactions"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/tokens/[tokenHash]/holders"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/tokens/[tokenHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]/logs"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("GET", "/wallet/api/wallet/api/v2/transactions/[txHash]/token-transfers"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/[address]/dex-history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/[address]/performance"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/[address]/profile"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/[address]/rank"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/charts/[symbol]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/holdings"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/pnl"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/tx-volume"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/charts/value"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v1/portfolio/[address]/pnl"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history-by-day"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/coin-balance-history"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/counters"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/logs"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft/collections"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/nft"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tabs-counters"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-balances"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/token-transfers"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/tokens"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/addresses/[addressHash]/transactions"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/tokens/[tokenHash]/holders"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/tokens/[tokenHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]/logs"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("HEAD", "/wallet/api/wallet/api/v2/transactions/[txHash]/token-transfers"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| `http_route` | `("POST", "/wallet/api/chat"),` | [services/platform/hosted/gateway/src/ui_proxy.rs](../../../services/platform/hosted/gateway/src/ui_proxy.rs) |
| source | — | [services/platform/hosted/gateway/src/public_reads.rs](../../../services/platform/hosted/gateway/src/public_reads.rs) |

## gateway-jsonrpc-methods

| Kind | Declaration | Source |
|---|---|---|
| `jsonrpc_method` | `"eth_accounts",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_coinbase",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_mining",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `pub const EVM_NAMESPACES: [&str; 3] = ["eth_", "net_", "web3_"];` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `["eth_sendRawTransaction", "eth_sendRawTransactionSync"];` | [services/platform/hosted/gateway/src/capabilities.rs](../../../services/platform/hosted/gateway/src/capabilities.rs) |
| `jsonrpc_method` | `"eth_sendTransaction",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_sign",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_signTransaction",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_signTypedData",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `"eth_signTypedData_v4",` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `pub const SUBSCRIPTION_METHODS: [&str; 2] = ["eth_subscribe", "eth_unsubscribe"];` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `jsonrpc_method` | `if value["method"] != "eth_subscription"` | [services/platform/hosted/gateway/src/ws.rs](../../../services/platform/hosted/gateway/src/ws.rs) |
| `jsonrpc_method` | `pax_sequencerReceipt` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `"px_getAccount" => get_account(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `"px_getBalances" => get_balances(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `"px_getCapabilities" => super::capabilities::get(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `"px_getHistory" => {` | [services/platform/hosted/gateway/src/history.rs](../../../services/platform/hosted/gateway/src/history.rs) |
| `jsonrpc_method` | `"px_getNetwork" => get_network(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `if method == "px_getRouteCatalogue" {` | [services/platform/hosted/gateway/src/rpc.rs](../../../services/platform/hosted/gateway/src/rpc.rs) |
| `jsonrpc_method` | `let unified = method == "px_getUnifiedHistory";` | [services/platform/hosted/gateway/src/history.rs](../../../services/platform/hosted/gateway/src/history.rs) |
| `jsonrpc_method` | `"px_listAssets" => list_assets(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |
| `jsonrpc_method` | `"px_listHolderRecords" => Some(answer_holders(config, id, params)),` | [services/platform/hosted/gateway/src/history.rs](../../../services/platform/hosted/gateway/src/history.rs) |
| `jsonrpc_method` | `method == "px_register"` | [services/platform/hosted/gateway/src/rpc_register.rs](../../../services/platform/hosted/gateway/src/rpc_register.rs) |
| `jsonrpc_method` | `"px_resolveAccount" => resolve_account(config, id, params),` | [services/platform/hosted/gateway/src/paxeer.rs](../../../services/platform/hosted/gateway/src/paxeer.rs) |

## gateway-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |

## gateway-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn decode_coins(answer: &[u8]) -> Result<Vec<(String, [u8; 32])>, AbiError> {` | [services/platform/hosted/gateway/src/evm.rs](../../../services/platform/hosted/gateway/src/evm.rs) |
| `rust_pub_item` | `pub enum AccessError {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn authenticate_gateway_key(` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub struct MaintainedBatchDocument {` | [services/platform/hosted/gateway/src/authority_evidence.rs](../../../services/platform/hosted/gateway/src/authority_evidence.rs) |
| `rust_pub_item` | `pub fn present_maintained<'de, D: serde::Deserializer<'de>>(` | [services/platform/hosted/gateway/src/authority_evidence.rs](../../../services/platform/hosted/gateway/src/authority_evidence.rs) |
| `rust_pub_item` | `pub struct AuthorityFacts {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn configured_sequencer(` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn gateway_audit_event(` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn gateway_digest(parts: &[&[u8]]) -> String {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn gateway_principal(` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub enum GatewayError {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub mod http;` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub struct Client {` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `rust_pub_item` | `pub struct Endpoint {` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `rust_pub_item` | `pub struct IncomingRequest {` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `rust_pub_item` | `pub struct OutboundRequest<'a> {` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `rust_pub_item` | `pub struct OutgoingResponse {` | [services/platform/hosted/gateway/src/http.rs](../../../services/platform/hosted/gateway/src/http.rs) |
| `rust_pub_item` | `pub struct PrincipalId(String);` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub use layerx_proof::inclusion::SequencerAuthorization;` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub struct Completion<'a> {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct KeyRecord {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct RedisEndpoint {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct RedisStore {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub enum Reservation {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct ReservationRequest<'a> {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct TapCredentialRecord {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub enum TapNonceConsumption {` | [services/platform/hosted/gateway/src/store.rs](../../../services/platform/hosted/gateway/src/store.rs) |
| `rust_pub_item` | `pub struct VerifiedOperation {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub const fn verification_level(&self) -> &'static str {` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub fn verify_activity_operation(` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |
| `rust_pub_item` | `pub const ROUTER_METHODS: [&str; 17]` | [services/platform/hosted/gateway/src/lib.rs](../../../services/platform/hosted/gateway/src/lib.rs) |

## gateway-ws-routes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/hosted/gateway/src/main.rs](../../../services/platform/hosted/gateway/src/main.rs) |
