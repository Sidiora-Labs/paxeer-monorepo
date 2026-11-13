# Platform ramps contract

The [ramp reference service and toolkit](../../../services/platform/ramps) encode orders, payment calls and receipt verification. Customer off-ramp payments use `budgetSpend` at `0x101B` with typed budget authorization. The toolkit binds the signed calldata and authenticated `BudgetSpent` event to the expected recipient, budget and amount. Codec availability alone does not grant spending authority.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## ramp-container-environment

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_RAMP_EXPECTED_REVISION` | [services/platform/ramps/sandbox-journey.sh](../../../services/platform/ramps/sandbox-journey.sh) |
| `env_var` | `LAYERX_RAMP_EXPECTED_SOURCE_DIGEST` | [services/platform/ramps/sandbox-journey.sh](../../../services/platform/ramps/sandbox-journey.sh) |
| `env_var` | `LAYERX_RAMP_RECEIPT_VERIFIER_BINARY` | [services/platform/ramps/sandbox-journey.sh](../../../services/platform/ramps/sandbox-journey.sh) |
| source | — | [services/platform/ramps/env-init.sh](../../../services/platform/ramps/env-init.sh) |

<a id="ramp-dependencies"></a>

## ramp-layerx-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub chain_id: u64` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub gas_limit_cap: u64` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub max_fee_per_gas: u128` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub max_priority_fee_per_gas: u128` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub operator_address: String` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub receipt_keys: Vec<ReceiptKeyConfig>` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub rpc_endpoint: String` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub rpc_trust_anchor_der: PathBuf` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| source | — | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |

## ramp-reference-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if std::env::args_os().nth(1).as_deref() == Some(std::ffi::OsStr::new("--verify-receipt")) {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-reference-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `endpoint: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `public_key: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `service_token_file: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `client_tls: ClientTls,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `compliance: ComplianceConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `identity: IdentityConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `journal_path: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `layerx: LayerxConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `lease_seconds: u64,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `listen: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `listener: Option<String>,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `operator: OperatorIdentity,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `operator_control_token_file: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `paxeer: PaxeerConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `provider: ProviderConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `provider_callback_public_key: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `quotes: Vec<QuoteTerms>,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `reconcile_seconds: u64,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `server_identity_password_file: Option<PathBuf>,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `server_identity_pkcs12: Option<PathBuf>,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `source_settlement: Option<SourceIntakeConfig>,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `worker_id: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `audience: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `broadcast_path: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `custody_credential_file: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `custody_endpoint: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `operator_account: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `poll_cadence_seconds: u64,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `signer_key_handle: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `status_path: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `vault_id: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `wallet_address: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `credential_file: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `settlement_path: String,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `migration_service_token_file: PathBuf,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `config_key` | `verification: SourceSettlementConfig,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-reference-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_RAMP_BUILD_REVISION` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `env_var` | `LAYERX_RAMP_BUILD_SOURCE_DIGEST` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-reference-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `.map_err(\|_\| error(401, "authentication_refused"))` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(401, "authentication_required"))?,` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(400, "body_too_large"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `serde_json::from_slice(&request.body).map_err(\|_\| error(400, "callback_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `RampError::Compliance => error(503, "compliance_unavailable"),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(400, "content_length_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(401, "customer_authentication_refused"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(401, "customer_authentication_required"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(400, "customer_binding_invalid"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(400, "customer_binding_required"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(400, "duplicate_header"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `std::str::from_utf8(&bytes[..boundary]).map_err(\|_\| error(400, "headers_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(400, "headers_too_large"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `parse_hex32(idempotency).map_err(\|_\| error(400, "idempotency_key_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(503, "journal_busy"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(503, "journal_recovery_required"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(503, "journal_unavailable"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `error(400, "layerx_binding_refused")` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `RampError::Layerx \| RampError::Receipt(_) => error(503, "layerx_unavailable"),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(403, "migration_authentication_refused"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(401, "migration_authentication_required"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(404, "not_found"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `error(409, "operation_conflict")` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(403, "operator_authentication_refused"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(401, "operator_authentication_required"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `let digest = parse_hex32(digest).map_err(\|_\| error(400, "order_digest_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(409, "order_id_conflict"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `serde_json::from_slice(&request.body).map_err(\|_\| error(400, "order_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(404, "order_not_found"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `RampError::Paxeer => error(503, "paxeer_unavailable"),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `RampError::Provider => error(503, "provider_unavailable"),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(409, "quote_binding_conflict"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(409, "quote_expired"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(404, "quote_not_found"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(404, "rebalance_not_found"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(400, "request_incomplete"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `let request_line = lines.next().ok_or_else(\|\| error(400, "request_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(400, "request_read_failed"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `error(400, "request_refused")` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `RampError::Journal \| RampError::Configuration => error(503, "service_unavailable"),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(409, "source_claim_conflict"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(400, "source_settlement_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `return Err(error(409, "source_settlement_not_admitted"));` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| error(503, "source_settlement_unconfigured"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `_ => return Err(error(409, "source_state_conflict")),` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `.map_err(\|_\| error(400, "transaction_hash_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `error_code` | `serde_json::from_slice(&request.body).map_err(\|_\| error(400, "work_invalid"))?;` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-reference-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if let Some(digest) = request.path.strip_prefix("/v1/orders/") {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.path == "/internal/v1/journal" && request.method == "GET" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.path == "/internal/v1/journal/recover" && request.method == "POST" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/internal/v1/rebalances" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if let Some(idempotency) = request.path.strip_prefix("/internal/v1/rebalances/") {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/internal/v1/work" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/internal/v2/source-settlements" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/v1/orders" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/v1/provider-callbacks" {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-reference-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub const fn platform_reference_ramp() -> &'static str {` | [services/platform/ramps/reference/src/main.rs](../../../services/platform/ramps/reference/src/main.rs) |

## ramp-toolkit-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if args.len() != 8 \|\| args[0] != "--config-file" \|\| args[2] != "--order-file"` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |

## ramp-toolkit-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `client_tls: TlsConfig,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `config_key` | `layerx: LayerxConfig,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `config_key` | `pub actor_did: String,` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub signer_endpoint: String,` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub signer_token_file: PathBuf,` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `config_key` | `pub ethereum: Option<EthereumConfig>,` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `config_key` | `pub quotes: Vec<SourceQuoteBinding>,` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `config_key` | `pub solana: Option<SolanaConfig>,` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `config_key` | `ca_pem: PathBuf,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `config_key` | `identity_password_file: PathBuf,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `config_key` | `identity_pkcs12: PathBuf,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `config_key` | `timeout_seconds: u64,` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |

## ramp-toolkit-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_RAMP_BUILD_REVISION` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |
| `env_var` | `LAYERX_RAMP_BUILD_SOURCE_DIGEST` | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |

## ramp-toolkit-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const JOURNAL_DOMAIN: &[u8] = b"LXP/market-maker-ramp/journal/v2\0";` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |

## ramp-toolkit-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn callback_evidence_digest(callback: &ProviderCallback) -> Result<[u8; 32], RampError> {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct ComplianceDecision {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub enum ComplianceOutcome {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub fn configured_sequencer(` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub fn decode_hex(value: &str, maximum: usize) -> Result<Vec<u8>, RampError> {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct HttpRequest<'a> {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct HttpResponse {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct MaintainedBatchDocument {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct PaxeerSubmission {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct PreparedLayerx {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct ProviderResult {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub enum ProviderState {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct CallbackIdentity {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub enum Event {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct JournalHealth {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct OrderSnapshot {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct PaxeerObservation<'a> {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct Projection {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct ProviderCallbackWrite<'a> {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct StagedCallback {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct TornTail {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct TransitionEvidence {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub enum WriteFault {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub enum WriteStep {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct AuthenticatedPrincipal {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct ComplianceClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct Endpoint {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub fn hex(bytes: &[u8]) -> String {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct IdentityClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct LayerxClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct LayerxConfig {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub enum LayerxSubmission {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct MutualTlsClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct MutualTlsFiles {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub fn parse_hex32(value: &str) -> Result<[u8; 32], RampError> {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct PaxeerCustodyClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct ProviderCallback {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct ProviderClient {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub struct SecretFile {` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub fn verify_recovery_settlement(` | [services/platform/ramps/toolkit/src/clients.rs](../../../services/platform/ramps/toolkit/src/clients.rs) |
| `rust_pub_item` | `pub const COMPLIANCE_CONTRACT_VERSION: &str = "layerx-ramp-compliance-v1";` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct CreateOrder {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct InventoryRebalancer<'a> {` | [services/platform/ramps/toolkit/src/engine.rs](../../../services/platform/ramps/toolkit/src/engine.rs) |
| `rust_pub_item` | `pub struct RampEngine<'a> {` | [services/platform/ramps/toolkit/src/engine.rs](../../../services/platform/ramps/toolkit/src/engine.rs) |
| `rust_pub_item` | `pub const EXTERNAL_CUSTODY_LABEL: &str =` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct Journal {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub struct PaxeerSnapshot {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub enum WorkflowStage {` | [services/platform/ramps/toolkit/src/journal.rs](../../../services/platform/ramps/toolkit/src/journal.rs) |
| `rust_pub_item` | `pub const fn source_error_status(error: SourceVerificationError) -> u16 {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub const SOURCE_SETTLEMENT_VERSION: &str = "layerx-migration-source-v2";` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub fn source_trace(header: Option<&str>, order_digest: [u8; 32]) -> TraceId {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub enum SourceChainKind {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct SourceSettlementConfig {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct SourceSettlementRequest {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct SourceSettlementResult {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct SourceSettlementService {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub enum SourceSettlementState {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub use layerx_migrate::MigrationError as SourceVerificationError;` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct OperatorIdentity {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub const PAXEER_CONTRACT_VERSION: &str = "layerx-ramp-paxeer-v1";` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub const fn platform_ramp_toolkit() -> &'static str {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub const PROVIDER_CONTRACT_VERSION: &str = "layerx-ramp-provider-v1";` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct QuoteTerms {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub enum RampDirection {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub enum RampError {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct RampOrder {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub enum AggregateStatus {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub fn compile_operator_send(` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub fn compile_payer_grant_draw(` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub const ORDER_DIGEST_DOMAIN: &[u8] = b"LXP/market-maker-ramp/order/v1\0";` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct RampPresentation {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct ReceiptEvidence {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct VerifiedLayerxLeg {` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub fn verify_order_receipt(` | [services/platform/ramps/toolkit/src/lib.rs](../../../services/platform/ramps/toolkit/src/lib.rs) |
| `rust_pub_item` | `pub struct SourceQuoteBinding {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct SourceSettlementRecordV2 {` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |
| `rust_pub_item` | `pub struct VerifiedSourceSettlement(SourceSettlementRecordV2);` | [services/platform/ramps/toolkit/src/migration.rs](../../../services/platform/ramps/toolkit/src/migration.rs) |

## ramp-verify-receipt

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs](../../../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) |

## ramps-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"client_tls": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"ca_pem": "/run/secrets/outbound-ca.pem",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"identity_password_file": "/run/secrets/outbound-identity-password",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"identity_pkcs12": "/run/secrets/outbound-identity.p12",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"timeout_seconds": 8` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"compliance": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"public_key": "REPLACE_64_HEX_ED25519_PUBLIC_KEY"` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"service_token_file": "/run/secrets/identity-token",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"identity": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"audience": "layerx-ramp"` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"journal_path": "/var/lib/layerx-ramp/journal.jsonl",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"layerx": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"signer_token_file": "/run/secrets/kms-token",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"lease_seconds": 60,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"operator": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"account": "agent:did:layerx:market-maker-eu-1:main",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"operator_control_token_file": "/run/secrets/operator-control-token"` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"principal_id": "market-maker-eu-1",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"paxeer": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"broadcast_path": "/layerx-paxeer-v1/rebalances",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"custody_credential_file": "/run/secrets/paxeer-custody-token",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"operator_account": "agent:did:layerx:market-maker-eu-1:main",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"rpc_trust_anchor_der": "/run/secrets/paxeer-rpc-ca.der",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"status_path": "/layerx-ramp-v1/settlements"` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"vault_id": "REPLACE_OPERATOR_VAULT",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"wallet_address": "REPLACE_OPERATOR_WALLET",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"provider": {` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"provider_callback_public_key": "REPLACE_64_HEX_ED25519_PUBLIC_KEY",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"credential_file": "/run/secrets/provider-token",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"settlement_path": "/layerx-ramp-v1/settlements",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"quotes": [` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"context": [` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"direction": "on_ramp",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"expires_at": 2000000000` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"external_amount_minor": 100,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"external_currency": "EUR",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"fee_minor": 2,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"layerx_amount": 1000000,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"layerx_asset": [` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"maximum_slippage_bps": 25,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"payout_token": "REPLACE_TOKENIZED_BENEFICIARY",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"provider_token": "REPLACE_PROVIDER_PRODUCT_TOKEN",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"quote_id": "REPLACE_ON_RAMP_QUOTE",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"rate_denominator": 100,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"rate_numerator": 1000000,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"reconcile_seconds": 5,` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"server_identity_password_file": "/run/secrets/server-identity-password",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"server_identity_pkcs12": "/run/secrets/server-identity.p12",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |
| `config_key` | `"worker_id": "ramp-worker-eu-1",` | [services/platform/ramps/config.example.json](../../../services/platform/ramps/config.example.json) |

## ramps-env-vars

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/ramps/sandbox-journey.sh](../../../services/platform/ramps/sandbox-journey.sh) |

<a id="toolkit-configuration-and-order-presentation-v2"></a>

See the implementation sources above.
