# Platform hosted contract

Hosted components under [services/platform/hosted](../../../services/platform/hosted) provide identity, authority, indexing, funding, webhooks and other application services. They operate against one Paxeer network with EVM chain ID `125` (`0x7d`). The kernel executes inside the node, and finality follows block commitment. The declarations below belong to the component named by their source path.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## agent-boundary-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_LISTEN` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_NETWORK_ID` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_TLS_CERT_DER` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_TLS_KEY_DER` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `env_var` | `LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| source | — | [services/platform/hosted/agent-boundary/src/chain_receipt.rs](../../../services/platform/hosted/agent-boundary/src/chain_receipt.rs) |

## agent-boundary-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return Some(refusal(403, "entitlement_denied", None));` | [services/platform/hosted/agent-boundary/src/deployment.rs](../../../services/platform/hosted/agent-boundary/src/deployment.rs) |
| `error_code` | `return Err(refusal(401, "identity_required", None));` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `error_code` | `return refusal(400, "invalid_activity_id", None);` | [services/platform/hosted/agent-boundary/src/deployment.rs](../../../services/platform/hosted/agent-boundary/src/deployment.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `error_code` | `refusal(503, "node_unavailable", Some(5))` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `error_code` | `return Some(refusal(404, "not_found", None));` | [services/platform/hosted/agent-boundary/src/deployment.rs](../../../services/platform/hosted/agent-boundary/src/deployment.rs) |

## agent-boundary-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if let Some(activity) = path.strip_prefix("/internal/v1/receipts/") {` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `http_route` | `if !path.starts_with("/v1/") \|\| query.is_some() {` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `http_route` | `let Some(transaction) = parse_hex32(` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `http_route` | `("GET", target) if target.starts_with("/internal/v1/deployment-proof/") => {` | [services/platform/hosted/agent-boundary/src/deployment.rs](../../../services/platform/hosted/agent-boundary/src/deployment.rs) |
| `http_route` | `if request.method == "GET" && path == "/livez" && query.is_none() {` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |
| `http_route` | `if request.method == "GET" && path == "/readyz" && query.is_none() {` | [services/platform/hosted/agent-boundary/src/main.rs](../../../services/platform/hosted/agent-boundary/src/main.rs) |

## agentd-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `--bearer-file) bearer_file=${2:?probe.sh: --bearer-file needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--ca) ca=${2:?probe.sh: --ca needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--client-cert) client_cert=${2:?probe.sh: --client-cert needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--client-key) client_key=${2:?probe.sh: --client-key needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--credential-file) credential_file=${2:?probe.sh: --credential-file needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--gateway-key-file) gateway_key_file=${2:?probe.sh: --gateway-key-file needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--journey) journey=${2:?probe.sh: --journey needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--rpc-url) rpc_url=${2:?probe.sh: --rpc-url needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |
| `cli_command` | `--url) url=${2:?probe.sh: --url needs a value}; shift 2 ;;` | [services/platform/hosted/agentd/probe.sh](../../../services/platform/hosted/agentd/probe.sh) |

## agentd-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_AGENTD_OWNER_SOCKET, HUMAN_SOCKET` | [services/platform/hosted/agentd/manifest_check.py](../../../services/platform/hosted/agentd/manifest_check.py) |
| source | — | [services/platform/hosted/agentd/live-check.sh](../../../services/platform/hosted/agentd/live-check.sh) |

## ai-artifacts-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `(arg("--data-dir"), arg("--config"), arg("--key-file"))` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |

## ai-artifacts-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub capacity_bytes: u64,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub commit_readers: Vec<String>,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub listener: Listener,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub publishers: Vec<Publisher>,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub retention_mark: Option<PathBuf>,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub revocation_mark: PathBuf,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |
| `config_key` | `pub tenants: Vec<Tenant>,` | [services/platform/hosted/ai-artifacts/src/authority.rs](../../../services/platform/hosted/ai-artifacts/src/authority.rs) |

## ai-artifacts-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `.open(self.dir.join("events.log"))` | [services/platform/hosted/ai-artifacts/src/store.rs](../../../services/platform/hosted/ai-artifacts/src/store.rs) |
| `file_format` | `let path = dir.join("retention.json");` | [services/platform/hosted/ai-artifacts/src/retention.rs](../../../services/platform/hosted/ai-artifacts/src/retention.rs) |
| `file_format` | `let path = self.dir.join("state.json");` | [services/platform/hosted/ai-artifacts/src/store.rs](../../../services/platform/hosted/ai-artifacts/src/store.rs) |

## ai-artifacts-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `"/v1/content" => return self.content(tenant, body),` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/deliveries" => self.delivery_status(tenant, body),` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/finalize" => self.with_id::<FinalizeReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/grants" => self.with_id::<GrantReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/import" => reply(self.import(tenant, body)),` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/locators/check" => reply(parse::<LocatorReq>(body).and_then(\|r\| {` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/prepare" => {` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/publishers/revoke" => self.with_id::<PublisherRevokeReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/binding" => self.with_id::<StateHeadReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/binding-intent" => self.with_id::<RootReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/hold" => self.with_id::<HoldReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/orphan" => self.with_id::<RootReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/projection" => reply(self.projection(body)),` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/reinstate" => self.with_id::<ReinstateReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/release" => self.with_id::<HoldReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/retention/terminal" => self.with_id::<StateHeadReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/revoke" => self.with_id::<RevokeReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/rotate" => self.with_id::<RootReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `"/v1/tombstone" => self.with_id::<RootReq, _>(` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `if req.method == "GET" && path == "/v1/usage" {` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `if req.method != "POST" \|\| req.path != "/v1/resolve" {` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |
| `http_route` | `if let ("PUT", Some(rest)) = (req.method.as_str(), path.strip_prefix("/v1/upload/")) {` | [services/platform/hosted/ai-artifacts/src/main.rs](../../../services/platform/hosted/ai-artifacts/src/main.rs) |

## ai-evaluator-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `access_declaration: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `assessments_dir: PathBuf,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `budget: [u64; 7],` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `capabilities: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `chain: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `evaluator: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `evm_chain_id: u64,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `fee_limit: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `gas_limit: u64,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `near_end_heights: u64,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `owner: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `poll_ms: u64,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `program: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `reader: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `receipt_timeout_ms: u64,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `response_capacity: u32,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `rpc_url: String,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `config_key` | `state_dir: PathBuf,` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |

## ai-evaluator-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_AI_EVALUATOR_CONFIG` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `env_var` | `LAYERX_AI_EVALUATOR_EVM_SENDER` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `env_var` | `LAYERX_AI_EVALUATOR_KEY_FILE` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |

## ai-evaluator-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `error_code` | `_ => refusal(404, "not_found", None),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `error_code` | `("GET", "/readyz") => refusal(503, "not_ready", None),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |

## ai-evaluator-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `("GET", "/healthz") => ok(r#"{"status":"ok"}"#.to_owned()),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `http_route` | `("GET", "/metrics") => ok(metrics.render()),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `http_route` | `("GET", "/readyz") if ready => ok(r#"{"status":"ready"}"#.to_owned()),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |

## ai-evaluator-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `("active", "gauge", count(self.active)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("applied_total", "counter", count(self.applied)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("below_quorum", "gauge", u64::from(self.active < QUORUM)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("epoch", "gauge", self.epoch),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("forfeited_total", "counter", count(self.forfeited)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("quorum", "gauge", count(QUORUM)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("refused_total", "counter", count(self.refused)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `("revealed", "gauge", count(self.revealed)),` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |
| `metric` | `"unrevealed_near_end",` | [services/platform/hosted/ai-evaluator/src/main.rs](../../../services/platform/hosted/ai-evaluator/src/main.rs) |

## authority-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `.any(\|argument\| argument == "--help" \|\| argument == "-h")` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |

## authority-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `principals: Vec<PrincipalPolicy>,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `config_key` | `principal: String,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `config_key` | `tenant: String,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `config_key` | `assets: Vec<CurrencyMetadata>,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `config_key` | `modules: Vec<Module>,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `config_key` | `schema_version: u16,` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |

## authority-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_AUTHORITY_CLIENT_CA_DER` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_CORE_CLOCK_HORIZON` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_HUMAN_AGENT_TENANT` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_IDENTITY_BINDING_GID` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `env_var` | `LAYERX_AUTHORITY_IDENTITY_BINDING_SOCKET` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `env_var` | `LAYERX_AUTHORITY_IDENTITY_BINDING_UID` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `env_var` | `LAYERX_AUTHORITY_LISTEN` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_MODULE_REGISTRY_FILE` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_NETWORK_ID` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_REPLICA_URL` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_STATE_ROOT` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `env_var` | `LAYERX_AUTHORITY_TLS_CERT_DER` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_TLS_KEY_DER` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_TOKEN_FILES` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `env_var` | `LAYERX_AUTHORITY_WIRE_VERSION` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| source | — | [services/platform/hosted/authority/src/chain_receipt.rs](../../../services/platform/hosted/authority/src/chain_receipt.rs) |
| source | — | [services/platform/hosted/authority/src/trust.rs](../../../services/platform/hosted/authority/src/trust.rs) |

## authority-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return Err(refusal(403, "activity_not_bound", None));` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return Err(refusal(404, "budget_not_bound", None));` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(403, "capability_not_bound", None))?;` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `.map_err(\|_\| refusal(403, "child_not_bound", None))?;` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(404, "did_not_bound", None))?;` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return Err(refusal(400, "duplicate_query", None));` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `refusal(502, "evidence_refused", None)` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(401, "identity_required", None))?;` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return Err(refusal(400, "invalid_activity_id", None));` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `let id = hex::decode32(id).map_err(\|_\| refusal(400, "invalid_budget_id", None))?;` | [services/platform/hosted/authority/src/human/budget_state.rs](../../../services/platform/hosted/authority/src/human/budget_state.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(400, "invalid_query", None))?;` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `_ => return Err(refusal(400, "invalid_recovery", None)),` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return refusal(400, "invalid_request", None);` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(405, "method_not_allowed", None);` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(404, "not_found", None))?;` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return Err(refusal(403, "original_activity_authority_mismatch", None));` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `let refused = \|\| refusal(403, "original_activity_invalid", None);` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `return Err(refusal(403, "original_activity_receipt_mismatch", None));` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `return Err(refusal(403, "principal_mismatch", None));` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(404, "principal_not_provisioned", None))` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(400, "query_required", None))?` | [services/platform/hosted/authority/src/human.rs](../../../services/platform/hosted/authority/src/human.rs) |
| `error_code` | `return refusal(503, "receipt_network_mismatch", Some(5));` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(503, "receipt_source_unavailable", Some(5));` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(503, "replica_evidence_unavailable", Some(1));` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(503, "replica_unavailable", Some(5));` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(502, "sequencer_key_mismatch", None);` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `return refusal(503, "state_persistence_unavailable", Some(5));` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `error_code` | `let refused = \|\| refusal(403, "subject_not_bound", None);` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `return Err(refusal(400, "unexpected_registration", None));` | [services/platform/hosted/authority/src/human/dynamic.rs](../../../services/platform/hosted/authority/src/human/dynamic.rs) |
| `error_code` | `ReceiptSource::Unknown => return refusal(404, "unknown_activity", None),` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |

## authority-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `\|\| &states[0].body()[..5] != b"LXGI1"` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `file_format` | `let mut summary = b"LXGS2".to_vec();` | [services/platform/hosted/authority/src/human/session_membership.rs](../../../services/platform/hosted/authority/src/human/session_membership.rs) |

## authority-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `path.strip_prefix("/internal/v1/activities/")` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `http_route` | `if path == "/livez" {` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `http_route` | `if path == "/readyz" {` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `http_route` | `if path.starts_with("/v1/agent/") {` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |
| `http_route` | `path.strip_prefix("/v1/authorized-batches/wait-by-activity/")` | [services/platform/hosted/authority/src/main.rs](../../../services/platform/hosted/authority/src/main.rs) |

## authority-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct AuthorityFacts {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub fn authorized_batch_by_activity(` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub struct BatchEvidence {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub enum BatchIdentityEvidence {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub enum EvidenceRefusal {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub mod hex {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub fn parse_replica_evidence(` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |
| `rust_pub_item` | `pub fn receipt_locator(receipt_bytes: &[u8]) -> Result<ReceiptLocator, EvidenceRefusal> {` | [services/platform/hosted/authority/src/lib.rs](../../../services/platform/hosted/authority/src/lib.rs) |

## core-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_CORE_ADMIN_LISTEN` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_ADMIN_TLS_CERT_DER` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_ADMIN_TLS_KEY_DER` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_ADMIN_TOKEN_FILE` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_CHAIN_RPC_URL` | [services/platform/hosted/core/src/chain_fund.rs](../../../services/platform/hosted/core/src/chain_fund.rs) |
| `env_var` | `LAYERX_CORE_CLIENT_CA_DER` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_FEE_LIMIT` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_FUNDING_SENDER_KEY_FILE` | [services/platform/hosted/core/src/chain_fund.rs](../../../services/platform/hosted/core/src/chain_fund.rs) |
| `env_var` | `LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID` | [services/platform/hosted/core/src/chain_fund.rs](../../../services/platform/hosted/core/src/chain_fund.rs) |
| `env_var` | `LAYERX_CORE_LISTEN` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_NETWORK_ID` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_NODE_URL` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_RECEIPT_DEADLINE_MS` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_STATE_DIR` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_SUPERVISOR_SOCKET` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_TLS_CERT_DER` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_TLS_KEY_DER` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_CORE_WALLET_CAPS_TOKEN_FILE` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `env_var` | `LAYERX_PAY_TIMING` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |

## core-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return refusal(503, "account_evidence_unavailable", Some(5));` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(503, "admin_unavailable", Some(5));` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `Err(_) => refusal(503, "asset_evidence_unavailable", Some(5)),` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `Err(_) => refusal(503, "batch_evidence_unavailable", Some(5)),` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(503, "gateway_readiness_not_configured", None);` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `Ok(Some(_)) => return refusal(409, "idempotency_conflict", None),` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return refusal(400, "idempotency_key_required", None);` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return refusal(400, "invalid_argument", None);` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(400, "invalid_asset_id", None)` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `Self::InvalidSymbol => refusal(502, "invalid_asset_symbol", None),` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(400, "invalid_batch", None);` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(400, "invalid_checkpoint", None);` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(400, "invalid_fee_request", None);` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(400, "invalid_idempotency_key", None);` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return refusal(400, "invalid_proof_selector", None);` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(400, "invalid_request", None);` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return refusal(503, "journal_unavailable", Some(5));` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(503, "node_unavailable", Some(5))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(409, "outcome_unknown", Some(5))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return refusal(503, "proof_evidence_unavailable", Some(5));` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `return refusal(503, "receipt_events_not_configured", None);` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `Err(_) => refusal(503, "receipt_events_unavailable", Some(5)),` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `error_code` | `Err(refusal(503, "receipt_unavailable", Some(5)))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(503, "reset_failed", Some(30))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `return Err(refusal(422, "send_refused", None));` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(422, "send_unbuildable", None)` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(503, "supervisor_unavailable", Some(30))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `error_code` | `refusal(503, "treasury_signer_unavailable", Some(5))` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |

## core-file-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |

## core-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if request.path == "/internal/readyz" {` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `let supported = if let Some(address) = id.strip_prefix("0x") {` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `http_route` | `path == "/v1/accounts"` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `} else if request.path != "/v1/assets"` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `http_route` | `if request.path == "/v1/assets" \|\| request.path.starts_with("/v1/assets/") {` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `http_route` | `if request.path == "/v1/fees/estimate" {` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `http_route` | `if request.path == "/v1/node-info" {` | [services/platform/hosted/core/src/public_reads.rs](../../../services/platform/hosted/core/src/public_reads.rs) |
| `http_route` | `\|\| path.starts_with("/v1/programs/activities/")` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `\|\| path.starts_with("/v1/programs/receipts/by-idempotency/")` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `\|\| path == "/v1/programs/registry"` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `\|\| path.starts_with("/v1/programs/registry/")` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `("GET", "/livez") => json_response(200, &serde_json::json!({ "live": true })),` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `("GET", "/readyz") => readiness(config),` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `("GET", "/v1/sequencer") => sequencer_route(config),` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |
| `http_route` | `"/admin/v1/testnet/fund" \| "/admin/v1/testnet/reset" => {` | [services/platform/hosted/core/src/main.rs](../../../services/platform/hosted/core/src/main.rs) |

## core-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn asset_registry() -> Result<(ModuleRegistry, ActivityType), String> {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn build_asset_send_with_identity_sequence(` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn build_send(seed: &[u8; 32], request: &SendRequest) -> Result<SignedSend, String> {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn domain_hash(domain: Domain, bytes: &[u8]) -> [u8; 32] {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn hex_decode(text: &str) -> Result<Vec<u8>, String> {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn hex_encode(bytes: &[u8]) -> String {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn main_account(did: &str) -> Result<[u8; 32], String> {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn read_custody_asset(` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn read_custody_profile(` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn send_context_hash(` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub struct SendRequest {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub struct SignedSend {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |
| `rust_pub_item` | `pub fn treasury_did(seed: &[u8; 32]) -> String {` | [services/platform/hosted/core/src/lib.rs](../../../services/platform/hosted/core/src/lib.rs) |

## dashboard-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_DASHBOARD_GATEWAY_REDIS_PASSWORD_FILE` | [services/platform/hosted/dashboard/src/gateway.rs](../../../services/platform/hosted/dashboard/src/gateway.rs) |
| `env_var` | `LAYERX_DASHBOARD_GATEWAY_REDIS_URL` | [services/platform/hosted/dashboard/src/gateway.rs](../../../services/platform/hosted/dashboard/src/gateway.rs) |
| `env_var` | `LAYERX_DASHBOARD_GATEWAY_REDIS_USERNAME_FILE` | [services/platform/hosted/dashboard/src/gateway.rs](../../../services/platform/hosted/dashboard/src/gateway.rs) |
| `env_var` | `LAYERX_DASHBOARD_LISTEN` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `env_var` | `LAYERX_DASHBOARD_LISTENER` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `env_var` | `LAYERX_DASHBOARD_REDIS_CA_DER` | [services/platform/hosted/dashboard/src/gateway.rs](../../../services/platform/hosted/dashboard/src/gateway.rs) |
| `env_var` | `LAYERX_DASHBOARD_TLS_CERT_DER` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `env_var` | `LAYERX_DASHBOARD_TLS_KEY_DER` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |

## dashboard-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `\|_\| Reply::refusal(503, "encoding_failed", Some(5)),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `DashboardError::InvalidRequest => Reply::refusal(400, "invalid_request", None),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `return Reply::refusal(404, "not_found", None);` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `DashboardError::Gateway(_) => Reply::refusal(403, "principal_refused", None),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `DashboardError::UnknownReceipt => Reply::refusal(404, "receipt_not_found", None),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `Err(_) => Reply::refusal(401, "session_required", None),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |
| `error_code` | `\| DashboardError::Io(_) => Reply::refusal(503, "state_unavailable", Some(5)),` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |

## dashboard-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if request.method == "GET" && request.path == "/healthz" {` | [services/platform/hosted/dashboard/src/main.rs](../../../services/platform/hosted/dashboard/src/main.rs) |

<a id="env-vars"></a>

## faucet-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_FAUCET_ADDRESS_LIMIT` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_CLAIM_AMOUNT` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_IDEMPOTENCY_SECONDS` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_IDENTITY_LIMIT` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_LISTEN` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_NETWORK_LIMIT` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_NETWORK_REQUEST_LIMIT` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_REDIS_PASSWORD_FILE` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_REDIS_URL` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_REDIS_USERNAME_FILE` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_TLS_CERT_DER` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_TLS_KEY_DER` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_FAUCET_WINDOW_SECONDS` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_INTROSPECTION_URL` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SERVICE_TOKEN_FILE` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_OUTBOUND_CA_DER` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_TESTNET_ADMIN_TOKEN_FILE` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `env_var` | `LAYERX_TESTNET_FUNDING_URL` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |

## faucet-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return Err(refusal(400, "content_type_required", None));` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `_ => refusal(503, "dependency_unavailable", Some(10)),` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `refusal(503, "funding_rejected", Some(10))` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `Reservation::Conflict => refusal(409, "idempotency_conflict", None),` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return Err(refusal(400, "idempotency_key_required", None));` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return Err(refusal(401, "identity_required", None));` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `.map_err(\|_\| refusal(503, "identity_unavailable", Some(10)))?,` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return refusal(400, "invalid_argument", None);` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return Err(refusal(400, "invalid_idempotency_key", None));` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return refusal(404, "not_found", None);` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `Err(_) => return Err(refusal(503, "persistence_unavailable", Some(10))),` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return refusal(401, "service_token_required", None);` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `error_code` | `return Err(refusal(400, "untrusted_identity_header", None));` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |

## faucet-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `let path = if path == "/" {` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `http_route` | `if request.method != "POST" \|\| request.path != "/v1/faucet/claims" {` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/v1/faucet/service-claims" {` | [services/platform/hosted/faucet/src/main.rs](../../../services/platform/hosted/faucet/src/main.rs) |

## feeder-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `[flag] if flag == "--check-config" => true,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `cli_command` | `[flag] if flag == "--version" => {` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |

## feeder-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub api_key_env: String,` | [services/platform/hosted/feeder/src/chainflow.rs](../../../services/platform/hosted/feeder/src/chainflow.rs) |
| `config_key` | `pub base_url: String,` | [services/platform/hosted/feeder/src/chainflow.rs](../../../services/platform/hosted/feeder/src/chainflow.rs) |
| `config_key` | `pub max_quote_age_ms: u64,` | [services/platform/hosted/feeder/src/chainflow.rs](../../../services/platform/hosted/feeder/src/chainflow.rs) |
| `config_key` | `pub timeout_ms: u64,` | [services/platform/hosted/feeder/src/chainflow.rs](../../../services/platform/hosted/feeder/src/chainflow.rs) |
| `config_key` | `pub cadence_ms: u64,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub chain_id: String,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub chainflow: ChainflowConfig,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub feed_account: String,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub gas_limit: u64,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub markets: Vec<MarketConfig>,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub retry_backoff_ms: u64,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub retry_max_attempts: u32,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub validator: String,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub market_id: u64,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `config_key` | `pub symbol: String,` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |

## feeder-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PRICE_SIDECAR_CONFIG` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `env_var` | `PRICE_SIDECAR_CONSENSUS_RPC_URL` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `env_var` | `PRICE_SIDECAR_EVM_RPC_URL` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `env_var` | `PRICE_SIDECAR_FEED_KEY_FILE` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `env_var` | `PRICE_SIDECAR_HEALTH_ADDR` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |
| `env_var` | `PRICE_SIDECAR_STATE_DIR` | [services/platform/hosted/feeder/src/bin/price-sidecar/main.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/main.rs) |

## feeder-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `dir.join("journal.json")` | [services/platform/hosted/feeder/src/lib.rs](../../../services/platform/hosted/feeder/src/lib.rs) |

## feeder-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `let _ = writeln!(out, "# TYPE price_sidecar_fetch_errors_total counter");` | [services/platform/hosted/feeder/src/bin/price-sidecar/health.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/health.rs) |
| `metric` | `let _ = writeln!(out, "# TYPE price_sidecar_last_accepted_height gauge");` | [services/platform/hosted/feeder/src/bin/price-sidecar/health.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/health.rs) |
| `metric` | `let _ = writeln!(out, "# TYPE price_sidecar_market_age_ms gauge");` | [services/platform/hosted/feeder/src/bin/price-sidecar/health.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/health.rs) |
| `metric` | `let _ = writeln!(out, "# TYPE price_sidecar_rounds_total counter");` | [services/platform/hosted/feeder/src/bin/price-sidecar/health.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/health.rs) |
| `metric` | `let _ = writeln!(out, "# TYPE price_sidecar_submissions_total counter");` | [services/platform/hosted/feeder/src/bin/price-sidecar/health.rs](../../../services/platform/hosted/feeder/src/bin/price-sidecar/health.rs) |

## feeder-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn http_agent() -> Result<ureq::Agent, String> {` | [services/platform/hosted/feeder/src/lib.rs](../../../services/platform/hosted/feeder/src/lib.rs) |
| `rust_pub_item` | `pub struct Sequencer {` | [services/platform/hosted/feeder/src/lib.rs](../../../services/platform/hosted/feeder/src/lib.rs) |

## human-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `parser.add_argument('--resume', action='store_true')` | [services/platform/hosted/human/browser_onboarding.py](../../../services/platform/hosted/human/browser_onboarding.py) |
| `cli_command` | `enrollment.add_argument('--epoch', type=int, default=1)` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `enrollment.add_argument('--identity', required=True)` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `assembly.add_argument('--output-dir')` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `enrollment.add_argument('--role', required=True)` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `enrollment.add_argument('--secrets-dir', required=True)` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `enrollment.add_argument('--work-dir', required=True)` | [services/platform/hosted/human/guardians.py](../../../services/platform/hosted/human/guardians.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--assemble':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `if len(sys.argv) == 3 and sys.argv[1] == '--export-registry-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--genesis-binding':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 4 and sys.argv[1] == '--hosted-registry-material-produce':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 7 and sys.argv[1] == '--import-registry-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--import-secret':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) in (6, 7) and sys.argv[1] == '--kernel-registry-material-produce':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 8 and sys.argv[1] == '--kms-prerequisite':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 11 and sys.argv[1] == '--policy-graph-status':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) in (7, 8) and sys.argv[1] == '--publish-authority-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--relocate-bundle':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--seal-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--secret-arguments':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 3 and sys.argv[1] == '--verify-authority-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--verify-bundle':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--verify-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) > 1 and sys.argv[1] == '--verify-projected-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `elif len(sys.argv) == 3 and sys.argv[1] == '--verify-registry-material':` | [services/platform/hosted/human/material.py](../../../services/platform/hosted/human/material.py) |
| `cli_command` | `parser.add_argument('--native-asset', required=True)` | [services/platform/hosted/human/onboarding_bootstrap.py](../../../services/platform/hosted/human/onboarding_bootstrap.py) |
| `cli_command` | `parser.add_argument('--tenant', required=True)` | [services/platform/hosted/human/onboarding_bootstrap.py](../../../services/platform/hosted/human/onboarding_bootstrap.py) |
| `cli_command` | `parser.add_argument('--config-directory', required=True)` | [services/platform/hosted/human/owner_kms.py](../../../services/platform/hosted/human/owner_kms.py) |
| `cli_command` | `parser.add_argument('--executable', required=True)` | [services/platform/hosted/human/owner_kms.py](../../../services/platform/hosted/human/owner_kms.py) |
| `cli_command` | `parser.add_argument('--work-dir', required=True)` | [services/platform/hosted/human/owner_kms.py](../../../services/platform/hosted/human/owner_kms.py) |
| `cli_command` | `mode.add_argument('--account-requests', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--assemble', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--asset')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--catalog', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--chain', type=int)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--journal', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--materialize-journal', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--network', type=int)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--output', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--prepare-owner-admission', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--prepare-owner-request', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--preserve-binding', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--produce-owner-registration', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--producer-manifest', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--qualify-generated-set', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--registry', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--request', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--response', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--secrets-dir', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--sequencer', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--treasury', type=Path)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--validate-evidence-inputs', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--validate-job-input', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--validate-owner-registration', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `mode.add_argument('--validate-owner-result', action='store_true')` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |
| `cli_command` | `parser.add_argument('--work-dir', type=Path, required=True)` | [services/platform/hosted/human/provision.py](../../../services/platform/hosted/human/provision.py) |

## human-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE` | [services/platform/hosted/human/bootstrap.py](../../../services/platform/hosted/human/bootstrap.py) |
| `env_var` | `LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING` | [services/platform/hosted/human/owner_kms.py](../../../services/platform/hosted/human/owner_kms.py) |
| `env_var` | `STORE_ROOT` | [services/platform/hosted/human/onboarding_material.py](../../../services/platform/hosted/human/onboarding_material.py) |
| source | — | [services/platform/hosted/human/material.sh](../../../services/platform/hosted/human/material.sh) |
| source | — | [services/platform/hosted/human/onboarding_provision.sh](../../../services/platform/hosted/human/onboarding_provision.sh) |
| source | — | [services/platform/hosted/human/provision.sh](../../../services/platform/hosted/human/provision.sh) |

## human-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `send(connection, dict(version=1, error='onboarding_request_refused'))` | [services/platform/hosted/human/onboarding_socket.py](../../../services/platform/hosted/human/onboarding_socket.py) |

## identity-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_IDENTITY_CLIENT_CA_DER` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_DISPOSABLE_DOMAINS_FILE` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_HEALTH_ADDR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_LISTEN` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SERVICE_TOKENS_DIR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SESSION_TTL_SECONDS` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SIGNUP_DAILY_CAP` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SIGNUP_PER_EMAIL_PER_HOUR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SIGNUP_PER_IP_PER_HOUR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SIGNUP_TOKEN_TTL_SECONDS` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SMTP_FROM` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_SMTP_URL` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_STATE_DIR` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_STORE_KEY_FILE` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_TLS_CERT_DER` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `env_var` | `LAYERX_IDENTITY_TLS_KEY_DER` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |

## identity-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `(Service::Ramp, _) => return refusal(400, "audience_required", None),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(503, "clock_unavailable", Some(5));` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(400, "content_type_required", None);` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `Self::DisposableDomain => (400, "email_domain_refused", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::EmailRegistered => (409, "email_registered", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `\|_\| refusal(503, "encoding_failed", Some(5)),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(503, "entropy_unavailable", Some(5));` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `Err(_) => return refusal(400, "invalid_argument", None),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(404, "not_found", None);` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(404, "principal_not_found", None);` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `Err(_) => refusal(409, "publication_key_binding_refused", None),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(400, "publication_key_required", None))?;` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `refusal(403, "service_not_permitted", None)` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return Err(refusal(401, "service_token_required", None));` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `refusal(429, "session_bound_reached", Some(60))` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `refusal(429, "session_history_capacity_reached", None)` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(404, "session_not_found", None);` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `Self::DailyCapReached(after) => (429, "signup_daily_cap_reached", Some(after)),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::EmailRateLimited(after) => (429, "signup_email_rate_limited", Some(after)),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::IpRateLimited(after) => (429, "signup_ip_rate_limited", Some(after)),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::AlreadyCredited => (409, "starter_credit_already_claimed", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::GrantMismatch => (409, "starter_credit_grant_mismatch", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `.map_err(\|_\| refusal(503, "store_unavailable", Some(5)))?;` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `refusal(409, "subject_tenant_conflict", None)` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `return refusal(400, "untrusted_identity_header", None);` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `error_code` | `Self::TokenExpired => (410, "verification_expired", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |
| `error_code` | `Self::NotVerified => (403, "verification_required", None),` | [services/platform/hosted/identity/src/signup.rs](../../../services/platform/hosted/identity/src/signup.rs) |

## identity-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const AUDIT_FILE: &str = "audit.log";` | [services/platform/hosted/identity/src/store.rs](../../../services/platform/hosted/identity/src/store.rs) |
| `file_format` | `const JOURNAL_FILE: &str = "journal.log";` | [services/platform/hosted/identity/src/store.rs](../../../services/platform/hosted/identity/src/store.rs) |

## identity-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `\| "/v1/signup"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `\| "/v1/starter-credits/settle"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `Ok(request) if request.method == "GET" && request.path == "/healthz" => readiness(shared),` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `"/internal/v1/principal" \| "/internal/readyz"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `\| "/v1/introspect"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `\| "/v1/principals"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `("POST", "/v1/publication-keys") => {` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `("POST", "/v1/sessions") => {` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `("POST", "/v1/sessions/introspect" \| "/v1/introspect") => {` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `\| "/v1/signup/verify"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |
| `http_route` | `\| "/v1/starter-credits"` | [services/platform/hosted/identity/src/main.rs](../../../services/platform/hosted/identity/src/main.rs) |

## indexer-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `"--cutover-height" => cutover = Some(parse(value()?)?),` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `cli_command` | `"--range-blocks" => range_blocks = parse(value()?)?,` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |

## indexer-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_INDEXER_ABI_DIR` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_ALLOW_REMOTE_PLAINTEXT` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_BACKFILL_RANGE_BLOCKS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_BATCH_INTERVAL_MS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_BLOCK_TIME_MS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_CHALLENGE_WINDOW_SECONDS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_COMET_ATTRIBUTES` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_COMET_URL, LAYERX_INDEXER_COMET_CA_DER` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_DB` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_EVM_URL, LAYERX_INDEXER_EVM_CA_DER` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_EVM_CHAIN_ID` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_EXPLORER_CA_DER` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_EXPLORER_URL` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_FINALITY_DEPTH` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_LISTEN` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_MAX_UNITS_PER_STEP` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_PAXSCAN_CERT_SHA256` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_PAXSCAN_TLS_UNAUTHENTICATED` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_POINTERS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_POLL_MS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_RELAY_CA_DER` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_RELAY_URL` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_STALL_SECS` | [services/platform/hosted/indexer/src/main.rs](../../../services/platform/hosted/indexer/src/main.rs) |
| `env_var` | `LAYERX_INDEXER_START_BATCH` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_START_BLOCK` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_TIMEOUT_MS` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `LAYERX_INDEXER_TLS_CERT_DER` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |
| `env_var` | `PAXSCAN_DATABASE_PUBLIC_URL` | [services/platform/hosted/indexer/src/config.rs](../../../services/platform/hosted/indexer/src/config.rs) |

## indexer-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `Ok(None) => refusal(404, "asset_not_found", None),` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `return Err(refusal(400, "duplicate_query_parameter", None));` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `return refusal(503, "explorer_unavailable", Some(1));` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(400, "invalid_cursor", None))?,` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `return Err(refusal(400, "invalid_kind", None));` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `.ok_or_else(\|\| refusal(400, "invalid_limit", None))?;` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `let decoded = percent_decode(segment, false).map_err(\|_\| refusal(400, "invalid_path", None))?;` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `let name = percent_decode(name, true).map_err(\|_\| refusal(400, "invalid_query", None))?;` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `_ => refusal(400, "invalid_request", None),` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `return refusal(405, "method_not_allowed", None);` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `Ok(_) => return refusal(404, "not_found", None),` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `refusal(503, "store_unavailable", Some(1))` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `error_code` | `_ => return Err(refusal(400, "unknown_query_parameter", None)),` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |

## indexer-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const SCHEMA: &str = "` | [services/platform/hosted/indexer/src/ai_health.rs](../../../services/platform/hosted/indexer/src/ai_health.rs) |
| `file_format` | `const SCHEMA: &str = "` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `file_format` | `const SCHEMA: &str = "` | [services/platform/hosted/indexer/src/ai_reputation.rs](../../../services/platform/hosted/indexer/src/ai_reputation.rs) |
| `file_format` | `const SCHEMA: &str = "` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |

## indexer-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if path == "/healthz" {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `http_route` | `if path == "/readyz" {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `http_route` | `if path == "/v1/assets" {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `http_route` | `if let Some(segment) = path.strip_prefix("/v1/assets/") {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `http_route` | `if let Some(segment) = path.strip_prefix("/v1/history/") {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `http_route` | `if let Some(segment) = path.strip_prefix("/v1/holders/") {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `rust_pub_item` | `pub const STABILITY_SOURCE: &str = "block_commit";` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |

## indexer-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `writeln!(formatter, "# TYPE paxai_health_latency_p95_ms histogram")?;` | [services/platform/hosted/indexer/src/ai_health.rs](../../../services/platform/hosted/indexer/src/ai_health.rs) |
| `metric` | `writeln!(formatter, "# TYPE paxai_health_success_ppm gauge")?;` | [services/platform/hosted/indexer/src/ai_health.rs](../../../services/platform/hosted/indexer/src/ai_health.rs) |
| `metric` | `"# TYPE paxai_health_withheld_observations_total counter"` | [services/platform/hosted/indexer/src/ai_health.rs](../../../services/platform/hosted/indexer/src/ai_health.rs) |

## indexer-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn keccak(bytes: &[u8]) -> [u8; 32] {` | [services/platform/hosted/indexer/src/abi.rs](../../../services/platform/hosted/indexer/src/abi.rs) |
| `rust_pub_item` | `pub const KNOWN_PRECOMPILES: [(&str, u16); 20] = [` | [services/platform/hosted/indexer/src/abi.rs](../../../services/platform/hosted/indexer/src/abi.rs) |
| `rust_pub_item` | `pub const NESTED_PRECOMPILES: [(&str, &str, u16); 6] = [` | [services/platform/hosted/indexer/src/abi.rs](../../../services/platform/hosted/indexer/src/abi.rs) |
| `rust_pub_item` | `pub mod ai_market;` | [services/platform/hosted/indexer/src/lib.rs](../../../services/platform/hosted/indexer/src/lib.rs) |
| `rust_pub_item` | `pub struct CursorKeyring {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub enum EpochStatus {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub enum Freshness {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub struct PageRequest {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub fn parse_epoch_query(market: &str, query: Option<&str>) -> ViewResult<(MarketId, u64, u8)> {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub fn parse_snapshot_query(` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub use layerx_programs_ai_market::types::Presence;` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub enum ProjectionState {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub struct ProjectionStore {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub struct QueryLimiter {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub struct Viewer {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub enum ViewError {` | [services/platform/hosted/indexer/src/ai_market.rs](../../../services/platform/hosted/indexer/src/ai_market.rs) |
| `rust_pub_item` | `pub fn serve(listener: &TcpListener, store: &Arc<Store>, tls: Option<&Arc<ServerConfig>>) {` | [services/platform/hosted/indexer/src/api.rs](../../../services/platform/hosted/indexer/src/api.rs) |
| `rust_pub_item` | `pub fn hex0x(bytes: &[u8]) -> String {` | [services/platform/hosted/indexer/src/codec.rs](../../../services/platform/hosted/indexer/src/codec.rs) |
| `rust_pub_item` | `pub struct AssetRow {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |
| `rust_pub_item` | `pub struct HolderRow {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |
| `rust_pub_item` | `pub struct Position {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |
| `rust_pub_item` | `pub struct Store {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |
| `rust_pub_item` | `pub struct TransferRow {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |
| `rust_pub_item` | `pub struct Unit {` | [services/platform/hosted/indexer/src/store.rs](../../../services/platform/hosted/indexer/src/store.rs) |

## internal-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if arguments.as_slice() == ["--empty-enrollment-mac"] {` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |

## internal-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_EVENTS_BOOTSTRAP_CREDENTIALS_FILE` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `env_var` | `LAYERX_EVENTS_CREDENTIALS_FILE` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_ENROLLMENT_KEY_FILE` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_KIND` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_LISTEN` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_PRODUCERS_FILE` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_STATE_DIR` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_TOKEN_FILE` | [services/platform/hosted/internal/src/bin/layerx-event-source.rs](../../../services/platform/hosted/internal/src/bin/layerx-event-source.rs) |
| `env_var` | `LAYERX_EVENTS_WEBHOOKS` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `env_var` | `LAYERX_KMS_LISTEN` | [services/platform/hosted/internal/src/bin/layerx-kms.rs](../../../services/platform/hosted/internal/src/bin/layerx-kms.rs) |
| `env_var` | `LAYERX_KMS_SEAL_SECRET_FILE` | [services/platform/hosted/internal/src/bin/layerx-kms.rs](../../../services/platform/hosted/internal/src/bin/layerx-kms.rs) |
| `env_var` | `LAYERX_KMS_STATE_DIR` | [services/platform/hosted/internal/src/bin/layerx-kms.rs](../../../services/platform/hosted/internal/src/bin/layerx-kms.rs) |
| `env_var` | `LAYERX_KMS_TOKEN_FILE` | [services/platform/hosted/internal/src/bin/layerx-kms.rs](../../../services/platform/hosted/internal/src/bin/layerx-kms.rs) |
| `env_var` | `LAYERX_TEST_WEBHOOKS_BIN` | [services/platform/hosted/internal/src/producer_test_support.rs](../../../services/platform/hosted/internal/src/producer_test_support.rs) |
| source | — | [services/platform/hosted/internal/env-init.sh](../../../services/platform/hosted/internal/env-init.sh) |

## internal-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return refusal(503, "admission_deadline_exceeded", None);` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return refusal(422, "algorithm_mismatch", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return Some(refusal(401, "client_certificate_required", None));` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return refusal(503, "dependency_unavailable", Some(5));` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return Err((409, "enrollment_credential_reused"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `.ok_or((503, "enrollment_generation_exhausted"))?;` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return Err((409, "enrollment_issuer_conflict"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return refusal(400, "enrollment_malformed", None);` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return Err((503, "enrollment_not_adopted"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return Err((413, "enrollment_oversized"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return Err((403, "enrollment_principal_mismatch"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return Err((409, "enrollment_removed"));` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `.map_err(\|_\| (503, "enrollment_unavailable"))?;` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `.map_err(\|_\| (503, "enrollment_upstream_unavailable"))?;` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `\|\| refusal(404, "event_not_found", None),` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `Ok(Created::Conflict) => refusal(409, "idempotency_conflict", None),` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return refusal(400, "idempotency_key_required", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return refusal(400, "invalid_message", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return refusal(422, "invalid_purpose", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `error_code` | `("LXP_WIRE_VERSION", "3".to_owned()),` | [services/platform/hosted/internal/src/producer_test_support.rs](../../../services/platform/hosted/internal/src/producer_test_support.rs) |
| `error_code` | `return refusal(413, "message_too_large", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `Ok(request) if request.method != "GET" => refusal(405, "method_not_allowed", None),` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `error_code` | `refusal(404, "not_found", None)` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `\|_\| refusal(500, "serialization_failed", None),` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `error_code` | `\|_\| refusal(503, "source_unavailable", Some(5)),` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return refusal(401, "unauthorized", None);` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `error_code` | `return refusal(404, "unknown_key", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `return refusal(422, "unsupported_algorithm", None);` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `error_code` | `refusal(503, "waiting_principals", Some(PRINCIPAL_POLL.as_secs()))` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |

## internal-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `&directory.join("enrollment-snapshot.json"),` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `file_format` | `const JOURNAL_FILE: &str = "journal.log";` | [services/platform/hosted/internal/src/journal.rs](../../../services/platform/hosted/internal/src/journal.rs) |

## internal-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `Ok(request) if request.path != "/healthz" => refusal(404, "not_found", None),` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `http_route` | `"/internal/v1/producer-readiness" \| "/internal/v1/reader-readiness"` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `http_route` | `("GET", path) => path.strip_prefix("/v1/signing-keys/").map_or_else(` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/internal/v1/enrollments" {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `http_route` | `&& request.path == "/internal/v1/observe"` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `http_route` | `("POST", "/v1/signatures") => self.sign(request),` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `http_route` | `("POST", "/v1/signing-keys") => self.create_key(request),` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |

## internal-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub mod base64;` | [services/platform/hosted/internal/src/lib.rs](../../../services/platform/hosted/internal/src/lib.rs) |
| `rust_pub_item` | `pub fn decode(value: &str) -> Option<Vec<u8>> {` | [services/platform/hosted/internal/src/base64.rs](../../../services/platform/hosted/internal/src/base64.rs) |
| `rust_pub_item` | `pub fn encode(bytes: &[u8]) -> String {` | [services/platform/hosted/internal/src/base64.rs](../../../services/platform/hosted/internal/src/base64.rs) |
| `rust_pub_item` | `pub fn enrollment_key(path: &Path) -> Result<Zeroizing<String>, String> {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub fn enrollment_snapshot_mac(` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub const ENROLLMENT_VERSION: u64 = 1;` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub struct Fact {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub enum Kind {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub struct ProducerCredential {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub struct Record {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub struct Service {` | [services/platform/hosted/internal/src/events.rs](../../../services/platform/hosted/internal/src/events.rs) |
| `rust_pub_item` | `pub use layerx_crypto::evm_transaction::{Eip1559Call, Eip7702Authorization, SignedAuthorization};` | [services/platform/hosted/internal/src/evm_sender.rs](../../../services/platform/hosted/internal/src/evm_sender.rs) |
| `rust_pub_item` | `pub struct EvmSender {` | [services/platform/hosted/internal/src/evm_sender.rs](../../../services/platform/hosted/internal/src/evm_sender.rs) |
| `rust_pub_item` | `pub enum EvmSenderError {` | [services/platform/hosted/internal/src/evm_sender.rs](../../../services/platform/hosted/internal/src/evm_sender.rs) |
| `rust_pub_item` | `pub mod gateway_http;` | [services/platform/hosted/internal/src/lib.rs](../../../services/platform/hosted/internal/src/lib.rs) |
| `rust_pub_item` | `pub struct Client {` | [services/platform/hosted/internal/src/gateway_http.rs](../../../services/platform/hosted/internal/src/gateway_http.rs) |
| `rust_pub_item` | `pub struct Endpoint {` | [services/platform/hosted/internal/src/gateway_http.rs](../../../services/platform/hosted/internal/src/gateway_http.rs) |
| `rust_pub_item` | `pub mod http;` | [services/platform/hosted/internal/src/lib.rs](../../../services/platform/hosted/internal/src/lib.rs) |
| `rust_pub_item` | `pub fn health_listen() -> Result<SocketAddr, String> {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub const IO_TIMEOUT: Duration = Duration::from_secs(8);` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn json<T: Serialize>(status: u16, value: &T) -> Response {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn ok(body: String) -> Response {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn parse_client_request(stream: &mut impl Read) -> Result<Request, String> {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn read_http_message(stream: &mut impl Read, maximum: usize) -> Result<Request, String> {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn refusal(status: u16, code: &str, retry_after: Option<u64>) -> Response {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub struct Response {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub fn write_response(stream: &mut impl Write, response: &Response) -> Result<(), String> {` | [services/platform/hosted/internal/src/http.rs](../../../services/platform/hosted/internal/src/http.rs) |
| `rust_pub_item` | `pub struct Journal {` | [services/platform/hosted/internal/src/journal.rs](../../../services/platform/hosted/internal/src/journal.rs) |
| `rust_pub_item` | `pub const LOCK_REFUSAL: &str = "journal lock";` | [services/platform/hosted/internal/src/journal.rs](../../../services/platform/hosted/internal/src/journal.rs) |
| `rust_pub_item` | `pub struct KeyStore {` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `rust_pub_item` | `pub const SEAL_LABEL: &[u8] = b"layerx-kms-ed25519-seed";` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `rust_pub_item` | `pub struct Service {` | [services/platform/hosted/internal/src/kms.rs](../../../services/platform/hosted/internal/src/kms.rs) |
| `rust_pub_item` | `pub struct PrincipalClient {` | [services/platform/hosted/internal/src/principal.rs](../../../services/platform/hosted/internal/src/principal.rs) |
| `rust_pub_item` | `pub fn from_environment(prefix: &str) -> Result<Self, String> {` | [services/platform/hosted/internal/src/principal.rs](../../../services/platform/hosted/internal/src/principal.rs) |
| `rust_pub_item` | `pub struct Client {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub fn from_environment(kinds: &[&str]) -> Result<Self, String> {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub fn from_environment_if_configured(kinds: &[&str]) -> Result<Option<Self>, String> {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub enum DeliveryFailure {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct DeliveryState {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub fn event_id(kind: &str, resource: &str, sequence: u64) -> String {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct Health {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub const MAX_DELIVERY_ATTEMPTS: u16 = 5;` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub const MAX_PENDING: usize = 1024;` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub const MAX_SCHEDULED_PRINCIPALS: usize = 10_000;` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct Observation {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub trait Outbox: Send + Sync + 'static {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct Pending {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct QueueState {` | [services/platform/hosted/internal/src/producer.rs](../../../services/platform/hosted/internal/src/producer.rs) |
| `rust_pub_item` | `pub struct SealKey {` | [services/platform/hosted/internal/src/seal.rs](../../../services/platform/hosted/internal/src/seal.rs) |
| `rust_pub_item` | `pub fn hex(bytes: &[u8]) -> String {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn random_hex(bytes: usize) -> Result<Zeroizing<String>, String> {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn read_secret_file(path: &Path) -> Result<Zeroizing<String>, String> {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn required_env(name: &str) -> Result<String, String> {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn sha256_hex(value: &[u8]) -> String {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn unhex(value: &str) -> Option<Vec<u8>> {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub fn unix_seconds() -> Result<u64, String> {` | [services/platform/hosted/internal/src/secret.rs](../../../services/platform/hosted/internal/src/secret.rs) |
| `rust_pub_item` | `pub mod tls;` | [services/platform/hosted/internal/src/lib.rs](../../../services/platform/hosted/internal/src/lib.rs) |
| `rust_pub_item` | `pub struct Origin {` | [services/platform/hosted/internal/src/tls.rs](../../../services/platform/hosted/internal/src/tls.rs) |
| `rust_pub_item` | `pub fn server_config(prefix: &str) -> Result<Arc<ServerConfig>, String> {` | [services/platform/hosted/internal/src/tls.rs](../../../services/platform/hosted/internal/src/tls.rs) |
| `rust_pub_item` | `pub struct Upstream {` | [services/platform/hosted/internal/src/tls.rs](../../../services/platform/hosted/internal/src/tls.rs) |
| `rust_pub_item` | `pub fn from_environment(prefix: &str) -> Result<Self, String> {` | [services/platform/hosted/internal/src/tls.rs](../../../services/platform/hosted/internal/src/tls.rs) |

## node-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `parser.add_argument('--expected-peer-gid', type=int)` | [services/platform/hosted/node/signer/client.py](../../../services/platform/hosted/node/signer/client.py) |
| `cli_command` | `parser.add_argument('--expected-peer-uid', type=int)` | [services/platform/hosted/node/signer/client.py](../../../services/platform/hosted/node/signer/client.py) |
| `cli_command` | `parser.add_argument('--expected-public-key')` | [services/platform/hosted/node/signer/client.py](../../../services/platform/hosted/node/signer/client.py) |
| `cli_command` | `parser.add_argument('--socket', required=True)` | [services/platform/hosted/node/signer/client.py](../../../services/platform/hosted/node/signer/client.py) |
| `cli_command` | `parser.add_argument('--timeout-seconds', default=10.0, type=float)` | [services/platform/hosted/node/signer/client.py](../../../services/platform/hosted/node/signer/client.py) |
| `cli_command` | `parser.add_argument("--broker-uid", type=int, default=4020)` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `cli_command` | `parser.add_argument("--socket", required=True)` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `cli_command` | `parser.add_argument("--watch-seconds", type=float, default=1)` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `cli_command` | `parser.add_argument('--allowed-gid', type=int, required=True)` | [services/platform/hosted/node/generation_transport.py](../../../services/platform/hosted/node/generation_transport.py) |
| `cli_command` | `parser.add_argument('--allowed-uid', type=int, required=True)` | [services/platform/hosted/node/generation_transport.py](../../../services/platform/hosted/node/generation_transport.py) |
| `cli_command` | `parser.add_argument('--check', action='store_true')` | [services/platform/hosted/node/genesis_fees.py](../../../services/platform/hosted/node/genesis_fees.py) |
| `cli_command` | `parser.add_argument('--module-fees', type=Path)` | [services/platform/hosted/node/genesis_fees.py](../../../services/platform/hosted/node/genesis_fees.py) |
| `cli_command` | `parser.add_argument("--allowed-gid", type=int, default=os.getegid())` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--data-dir", required=True)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--expected-phase")` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--extra", "--extra-json", dest="extra")` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--generation", required=True, type=int)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--initial-generation", type=int, default=1)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--network", type=int, required=True)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--new-phase")` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--reset-id")` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--run-dir", required=True)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--socket")` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument("--state-dir", required=True)` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `cli_command` | `parser.add_argument('--allowed-uid', required=True, type=uid_list)` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--binding-policy', default='')` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--key-file', default='')` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--provider', default='file', choices=('file', 'command'))` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--provider-command', default='')` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--provider-timeout-seconds', default=10, type=positive)` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--public-key-file', default='')` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--request-timeout-seconds', default=5, type=positive)` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--socket', required=True)` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `cli_command` | `parser.add_argument('--socket-group', default=None, type=int)` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| source | — | [services/platform/hosted/node/bootstrap.sh](../../../services/platform/hosted/node/bootstrap.sh) |
| source | — | [services/platform/hosted/node/supervisor.sh](../../../services/platform/hosted/node/supervisor.sh) |

## node-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_CORE_ENV_FILE` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `env_var` | `LAYERX_GENERATION_FD_MODE` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `env_var` | `LAYERX_GENERATION_NUMBER` | [services/platform/hosted/node/generation_client.py](../../../services/platform/hosted/node/generation_client.py) |
| `env_var` | `LAYERX_NODE_RESET_STAGED_GENERATIONS` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| `env_var` | `LAYERX_RESET_OWNER_FD` | [services/platform/hosted/node/reset_state.py](../../../services/platform/hosted/node/reset_state.py) |
| source | — | [services/platform/hosted/node/supervisor.sh](../../../services/platform/hosted/node/supervisor.sh) |
| source | — | [services/platform/hosted/node/bootstrap.sh](../../../services/platform/hosted/node/bootstrap.sh) |

## node-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return {'error': {'code': 'binding_refused', 'retry': 'never'}}` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `error_code` | `return {'error': {'code': 'digest_refused', 'retry': 'never'}}` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `error_code` | `connection.sendall(canonical({'error': {'code': 'generation_unavailable'}}) + b'\n')` | [services/platform/hosted/node/generation_transport.py](../../../services/platform/hosted/node/generation_transport.py) |
| `error_code` | `self.reply({'error': {'code': 'peer_refused', 'retry': 'never'}})` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |
| `error_code` | `return {'error': {'code': 'signature_refused', 'retry': 'never'}}` | [services/platform/hosted/node/signer/signer.py](../../../services/platform/hosted/node/signer/signer.py) |

<a id="paxai-storage-keeper-file-formats"></a>

## paxai-worker-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `let (Some("--config"), Some(path), None) = (args.next().as_deref(), args.next(), args.next())` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |

## paxai-worker-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `certificate_chain: PathBuf,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `chain: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `delegate_seed: PathBuf,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `listen: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `market: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `owner: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `private_key: PathBuf,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `program: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `signed_metadata: PathBuf,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |
| `config_key` | `worker: String,` | [services/platform/hosted/paxai-worker/src/main.rs](../../../services/platform/hosted/paxai-worker/src/main.rs) |

## paxai-worker-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PAXAI_RUNNER_MODELS` | [services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs](../../../services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs) |
| `env_var` | `PAXAI_RUNNER_STATE` | [services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs](../../../services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs) |

## paxai-worker-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `pub const MANIFEST_SCHEMA: u16 = 1;` | [services/platform/hosted/paxai-worker/src/metadata.rs](../../../services/platform/hosted/paxai-worker/src/metadata.rs) |
| `file_format` | `const MAGIC: &[u8; 8] = b"PXMEMB01";` | [services/platform/hosted/paxai-worker/src/membership.rs](../../../services/platform/hosted/paxai-worker/src/membership.rs) |
| `file_format` | `pub const MAGIC: &[u8; 7] = b"PAXRUN1";` | [services/platform/hosted/paxai-worker/src/runner.rs](../../../services/platform/hosted/paxai-worker/src/runner.rs) |
| `file_format` | `pub const SERVICE_MAGIC: &[u8; 7] = b"PAXAIS1";` | [services/platform/hosted/paxai-worker/src/auth.rs](../../../services/platform/hosted/paxai-worker/src/auth.rs) |

## paxai-worker-http-routes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/hosted/paxai-worker/src/discovery.rs](../../../services/platform/hosted/paxai-worker/src/discovery.rs) |

## paxeer-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))` | [services/platform/hosted/paxeer/evm.py](../../../services/platform/hosted/paxeer/evm.py) |
| `cli_command` | `--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--check) CHECK=1; shift ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--env-file) need_value "$@"; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--evm-chain-id) need_value "$@"; EVM_CHAIN_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--fees) need_value "$@"; FEES=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--gas) need_value "$@"; GAS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--module)` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--new-bin) need_value "$@"; NEW_BIN=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--old-bin) need_value "$@"; OLD_BIN=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--port-base) need_value "$@"; PORT_BASE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--source-data) need_value "$@"; SOURCE_DATA=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--source-genesis) need_value "$@"; SOURCE_GENESIS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--upgrade-height-offset) need_value "$@"; OFFSET=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--upgrade-name) need_value "$@"; UPGRADE_NAME=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--voting-seconds) need_value "$@"; VOTING_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--work) need_value "$@"; WORK=$2; shift 2 ;;` | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |
| `cli_command` | `--bin) need_value "$@"; PAXD_BIN=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--bypass-commit-timeout) need_value "$@"; BYPASS_COMMIT=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--deposit) need_value "$@"; DEPOSIT=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--description) need_value "$@"; DESCRIPTION=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--execute) EXECUTE=1; shift ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--expedited) EXPEDITED=1; shift ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--fees) need_value "$@"; FEES=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--gas) need_value "$@"; GAS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--home) need_value "$@"; HOME_DIR=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--key) need_value "$@"; KEY_NAME=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--keyring-backend) need_value "$@"; KEYRING_BACKEND=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--margin-seconds) need_value "$@"; MARGIN_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--now) need_value "$@"; NOW_OVERRIDE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--proposal-file) need_value "$@"; PROPOSAL_FILE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--proposal-id) need_value "$@"; PROPOSAL_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--rpc) need_value "$@"; RPC=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--seconds-per-block) need_value "$@"; SPB_OVERRIDE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--target-utc) need_value "$@"; TARGET_UTC=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--timeout-commit) need_value "$@"; TIMEOUT_COMMIT=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--timeout-vote) need_value "$@"; TIMEOUT_VOTE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--title) need_value "$@"; TITLE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--voter) need_value "$@"; VOTERS+=("$2"); shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-consensus-params.sh](../../../services/platform/hosted/paxeer/gov-consensus-params.sh) |
| `cli_command` | `--anchor) need_value "$@"; ANCHOR=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--bin) need_value "$@"; PAXD_BIN=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--deposit) need_value "$@"; DEPOSIT=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--description) need_value "$@"; DESCRIPTION=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--execute) EXECUTE=1; shift ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--expedited) EXPEDITED=1; shift ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--fees) need_value "$@"; FEES=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--gas) need_value "$@"; GAS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--halt-height) need_value "$@"; HALT_HEIGHT_OVERRIDE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--home) need_value "$@"; HOME_DIR=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--key) need_value "$@"; KEY_NAME=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--keyring-backend) need_value "$@"; KEYRING_BACKEND=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--lookback-blocks) need_value "$@"; LOOKBACK_BLOCKS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--margin-seconds) need_value "$@"; MARGIN_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--measure-seconds) need_value "$@"; MEASURE_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--now) need_value "$@"; NOW_OVERRIDE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--proposal-id) need_value "$@"; PROPOSAL_ID=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--rpc) need_value "$@"; RPC=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--seconds-per-block) need_value "$@"; SPB_OVERRIDE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--target-utc) need_value "$@"; TARGET_UTC=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--title) need_value "$@"; TITLE=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--upgrade-info) need_value "$@"; UPGRADE_INFO=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--upgrade-name) need_value "$@"; UPGRADE_NAME=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--voter) need_value "$@"; VOTERS+=("$2"); shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |
| `cli_command` | `--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;` | [services/platform/hosted/paxeer/gov-upgrade.sh](../../../services/platform/hosted/paxeer/gov-upgrade.sh) |

## paxeer-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_PAXEER_BOUNDARY_LISTEN` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `env_var` | `LAYERX_PAXEER_BOUNDARY_TLS_CERT_DER` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `env_var` | `LAYERX_PAXEER_BOUNDARY_TLS_KEY_DER` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `env_var` | `LAYERX_PAXEER_COMET_URL` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `env_var` | `LAYERX_PAXEER_NODE_URL` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| source | — | [services/platform/hosted/paxeer/gov-lib.sh](../../../services/platform/hosted/paxeer/gov-lib.sh) |
| source | — | [services/platform/hosted/paxeer/init-chain.sh](../../../services/platform/hosted/paxeer/init-chain.sh) |
| source | — | [services/platform/hosted/paxeer/fork-rehearsal.sh](../../../services/platform/hosted/paxeer/fork-rehearsal.sh) |

## paxeer-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return Err(rpc_error(&Value::Null, -32600, "invalid Comet request"));` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `return rpc_error(&id, -32601, "method is not relayed by the boundary");` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `let invalid = \|\| rpc_error(&id, -32602, "invalid Comet parameters");` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `return refusal(400, "batch_not_supported", None);` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `return Err(refusal(413, "body_too_large", None));` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `Ok(_) => refusal(503, "chain_id_mismatch", Some(10)),` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `Err(Failure::Unavailable) => refusal(503, "comet_evidence_unavailable", Some(5)),` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `Err(Failure::ProofUnsupported) => refusal(502, "comet_proof_unsupported", None),` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `Err(Failure::Node(NodeFailure::Invalid)) => refusal(502, "comet_response_invalid", Some(5)),` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `Err(Failure::Node(NodeFailure::Unreachable)) => refusal(503, "comet_unavailable", Some(5)),` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `return refusal(400, "content_type_required", None);` | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |
| `error_code` | `return refusal(400, "invalid_json", None);` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", None),` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `_ => refusal(502, "node_response_invalid", Some(5)),` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `error_code` | `Err(NodeFailure::Unreachable) => refusal(503, "node_unavailable", Some(5)),` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |

## paxeer-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if request.method == "GET" && request.path == "/genesis" {` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/livez" {` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/readyz" {` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/" {` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `http_route` | `if request.method == "POST" && request.path == "/comet" {` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| `jsonrpc_method` | `pax_sequencerReceipt` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |
| source | — | [services/platform/hosted/paxeer/src/comet.rs](../../../services/platform/hosted/paxeer/src/comet.rs) |

## paxeer-jsonrpc-methods

| Kind | Declaration | Source |
|---|---|---|
| `jsonrpc_method` | `const ALLOWED_METHODS: [&str; 3] = ["net_version", "web3_clientVersion", "pax_sequencerReceipt"];` | [services/platform/hosted/paxeer/src/main.rs](../../../services/platform/hosted/paxeer/src/main.rs) |

<a id="platform-docs"></a>

<a id="platform-workspace"></a>

## registry-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `.any(\|argument\| argument == "--attach-before-exec")` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `.position(\|argument\| argument == "--cgroup-root")` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `if !arguments.iter().any(\|argument\| argument == "--cgroup-v2")` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `cpu_usec: number("--cpu-time-max-usec=")?,` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `io_write: number("--io-write-max=")?,` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `.any(\|argument\| argument == "--kill-tree-on-exit")` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `memory: number("--memory-max=")?,` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `pids: u32::try_from(number("--pids-max=")?)` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `if arguments.first().map(String::as_str) == Some("--stopped-launcher") {` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `wall_ms: number("--wall-time-max-ms=")?,` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |
| `cli_command` | `.position(\|argument\| argument == "--workspace-device-path")` | [services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs](../../../services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs) |

## registry-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_REGISTRY_ATTEMPTS` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_ENTRYPOINT` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_ENVIRONMENT_ROOT` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_IMAGE_DIGEST` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME_DIGEST` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR_DIGEST` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_CLIENT_IDENTITY_PASSWORD_FILE` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_CLIENT_IDENTITY_PKCS12` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_HEALTH_LISTEN` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_HOST_CGROUP_MOUNT` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_IDENTITY` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `env_var` | `LAYERX_REGISTRY_LISTEN` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_MAX_STALENESS_SECONDS` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_NODE_AUTHORIZATION` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_NODE_ENDPOINT` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_OUTBOUND_CA_DER` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_RECEIPT_AUTHORITY_REPLICA_ID` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_SOURCE_MIRROR` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_TEST_DELEGATION_CHILD` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `LAYERX_REGISTRY_VERIFIED` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `env_var` | `UTC, LC_ALL` | [services/platform/hosted/registry/src/builder.rs](../../../services/platform/hosted/registry/src/builder.rs) |
| source | — | [services/platform/hosted/registry/node-provision-build-boundary.sh](../../../services/platform/hosted/registry/node-provision-build-boundary.sh) |
| source | — | [services/platform/hosted/registry/src/programs_chain.rs](../../../services/platform/hosted/registry/src/programs_chain.rs) |

## registry-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `BuildRefusal::BuilderFailed { reason } => refusal(422, "build_failed", reason),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `refusal(422, "build_not_reproducible", &refused.to_string())` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "build_queue_full", "the bounded build queue is full");` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `error_code` | `.map_err(\|error\| refusal(503, "deployment_head_unverified", &error))?;` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => return refusal(422, "deployment_proof_refused", &error),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => return refusal(503, "deployment_proof_unavailable", &error),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `JournalRefusal::Corrupt(defect) => refusal(503, "idempotency_record_corrupt", defect),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => return refusal(503, "idempotency_store_corrupt", &error),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "idempotency_store_unavailable", &error)` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => return refusal(503, "interface_proof_unavailable", &error),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => refusal(400, "invalid_argument", &error),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `\|_\| refusal(400, "invalid_request", "request could not be parsed"),` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `error_code` | `return refusal(404, "not_found", "route does not exist");` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "persistence_unavailable", &error);` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "protocol_state_unavailable", &error);` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "read_unverifiable", &error.to_string());` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `"code": "source_mismatch",` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(404, "source_not_mirrored", &refused.to_string())` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(refused) => return refusal(422, "source_unverifiable", &refused.to_string()),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "stale_read", "protocol head freshness overflowed");` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `Err(error) => refusal(502, "unverified_read", &error.to_string()),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `error_code` | `return refusal(503, "verification_pending", &error.to_string());` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |

## registry-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const BUILD_LOG: &str = "layerx-build.log";` | [services/platform/hosted/registry/src/builder.rs](../../../services/platform/hosted/registry/src/builder.rs) |
| `file_format` | `const OWNER_LOCK: &str = "owner.lock";` | [services/platform/hosted/registry/src/verified.rs](../../../services/platform/hosted/registry/src/verified.rs) |

## registry-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `("GET", "/healthz") => Response {` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/metrics" {` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `http_route` | `("GET", "/v1/programs/registry") => self.list(now),` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `http_route` | `("POST", "/__registry/sources") => {` | [services/platform/hosted/registry/src/routes.rs](../../../services/platform/hosted/registry/src/routes.rs) |
| `http_route` | `&& request.path.starts_with("/v1/programs/registry/")` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |

## registry-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `"event_producer_failures": failures,` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `metric` | `"event_producer_overflow": overflow,` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |
| `metric` | `"program_facts_unbound": unbound,` | [services/platform/hosted/registry/src/main.rs](../../../services/platform/hosted/registry/src/main.rs) |

## runtime-clock-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `Some(value) if value == "--runtime-dir" => {` | [services/platform/hosted/runtime-clock/src/main.rs](../../../services/platform/hosted/runtime-clock/src/main.rs) |

## runtime-clock-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_RUNTIME_CLOCK_PID` | [services/platform/hosted/runtime-clock/src/main.rs](../../../services/platform/hosted/runtime-clock/src/main.rs) |
| `env_var` | `LAYERX_RUNTIME_CLOCK_SOCKET` | [services/platform/hosted/runtime-clock/src/main.rs](../../../services/platform/hosted/runtime-clock/src/main.rs) |

## testnet-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_TESTNET_ADMIN_LISTEN` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `env_var` | `LAYERX_TESTNET_PUBLIC_LISTEN` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| source | — | [services/platform/hosted/testnet/run-testnet.sh](../../../services/platform/hosted/testnet/run-testnet.sh) |
| source | — | [services/platform/hosted/testnet/render-status.sh](../../../services/platform/hosted/testnet/render-status.sh) |
| source | — | [services/platform/hosted/testnet/reset-testnet.sh](../../../services/platform/hosted/testnet/reset-testnet.sh) |

## testnet-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `serde_json::json!({ "error": { "code": "content_type_required" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `serde_json::json!({ "error": { "code": "core_unavailable", "retry": "after" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `serde_json::json!({ "error": { "code": "idempotency_key_required" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `serde_json::json!({ "error": { "code": "invalid_argument" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `serde_json::json!({ "error": { "code": "invalid_idempotency_key" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `serde_json::json!({ "error": { "code": "invalid_request" } }),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `"code": "journey_degraded",` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `error_code` | `return json_response(404, serde_json::json!({ "error": { "code": "not_found" } }));` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |

## testnet-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `"/livez" => json_response(200, serde_json::json!({ "status": "live" })),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `http_route` | `"/readyz" => {` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `http_route` | `"/v1/parameters" => json_response(` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `http_route` | `"/v1/status" => json_response(200, status(config).public_status(&config.release)),` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `http_route` | `("POST", "/admin/v1/testnet/fund") => {` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |
| `http_route` | `("POST", "/admin/v1/testnet/reset") => {` | [services/platform/hosted/testnet/src/main.rs](../../../services/platform/hosted/testnet/src/main.rs) |

## webhooks-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub base_delay_seconds: u64,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `config_key` | `pub in_flight_timeout_seconds: u64,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `config_key` | `pub maximum_attempts: u32,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `config_key` | `pub maximum_delay_seconds: u64,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `config_key` | `pub spread_percent: u8,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `config_key` | `pub suspend_after_dead_letters: u32,` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |

## webhooks-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_DASHBOARD_CLIENT_IDENTITY_PASSWORD_FILE` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_DASHBOARD_CLIENT_IDENTITY_PKCS12` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_DASHBOARD_IDENTITY_TOKEN_FILE` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_DASHBOARD_IDENTITY_URL` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_DASHBOARD_INTERNAL_CA_DER` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_BASE_DELAY_SECONDS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_COMPONENT_TOKEN_FILE` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_CURSOR_KEY_FILE` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_DISPATCH_BUDGET` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_DISPATCH_INTERVAL_SECONDS` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_HEALTH_LISTEN` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_IDENTITY_URL` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_INSTANCE_ID` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_INTERNAL_CA_DER` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_KEY_OVERLAP_SECONDS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_KMS_TOKEN_FILE` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_KMS_URL` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_LEASE_SECONDS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_LISTENER` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_MAXIMUM_ATTEMPTS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_MAXIMUM_DELAY_SECONDS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_PUBLIC_CA_DER` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_REDIS_PASSWORD_FILE` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_REDIS_URL` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_REDIS_USERNAME_FILE` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_RETENTION_EVENTS` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_ROLE` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_SPREAD_PERCENT` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_SUSPEND_AFTER_DEAD_LETTERS` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `env_var` | `LAYERX_WEBHOOKS_TLS_CERT_DER` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `env_var` | `LAYERX_WEBHOOKS_TLS_KEY_DER` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| source | — | [services/platform/hosted/webhooks/env-init.sh](../../../services/platform/hosted/webhooks/env-init.sh) |

## webhooks-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return Reply::refusal(503, "admission_deadline_exceeded", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(400, "body_not_allowed", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `Ok(_) => Reply::refusal(503, "delivery_state_unavailable", None),` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(400, "idempotency_key_required", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(400, "invalid_admission_deadline", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(404, "not_found", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(401, "operator_authentication_required", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(403, "operator_role_required", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(403, "peer_role_refused", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(403, "producer_role_required", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `Err(_) => Reply::refusal(401, "session_required", None),` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `error_code` | `return Reply::refusal(401, "source_authentication_required", None);` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |

## webhooks-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const LOCK_FILE: &str = "state.lock";` | [services/platform/hosted/webhooks/src/ai_market.rs](../../../services/platform/hosted/webhooks/src/ai_market.rs) |

## webhooks-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if !request.path.starts_with("/internal/") {` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `http_route` | `if !request.path.starts_with("/v1/webhooks/") {` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/healthz" {` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |
| `http_route` | `if request.method == "GET" && request.path == "/v1/webhooks/scheme" {` | [services/platform/hosted/webhooks/src/main.rs](../../../services/platform/hosted/webhooks/src/main.rs) |

## webhooks-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct DeliveryRecord {` | [services/platform/hosted/webhooks/src/deliveries.rs](../../../services/platform/hosted/webhooks/src/deliveries.rs) |
| `rust_pub_item` | `pub fn hex_encode(bytes: &[u8]) -> String {` | [services/platform/hosted/webhooks/src/encoding.rs](../../../services/platform/hosted/webhooks/src/encoding.rs) |
| `rust_pub_item` | `pub struct EndpointHealth {` | [services/platform/hosted/webhooks/src/endpoints.rs](../../../services/platform/hosted/webhooks/src/endpoints.rs) |
| `rust_pub_item` | `pub enum WebhookError {` | [services/platform/hosted/webhooks/src/error.rs](../../../services/platform/hosted/webhooks/src/error.rs) |
| `rust_pub_item` | `pub struct EndpointId(String);` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub enum EventKind {` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub struct Principal(String);` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub struct ProtocolEvent {` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub struct ProtocolFact {` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub enum Verification {` | [services/platform/hosted/webhooks/src/events.rs](../../../services/platform/hosted/webhooks/src/events.rs) |
| `rust_pub_item` | `pub struct HostedReader {` | [services/platform/hosted/webhooks/src/hosted.rs](../../../services/platform/hosted/webhooks/src/hosted.rs) |
| `rust_pub_item` | `pub mod http;` | [services/platform/hosted/webhooks/src/lib.rs](../../../services/platform/hosted/webhooks/src/lib.rs) |
| `rust_pub_item` | `pub struct Reply {` | [services/platform/hosted/webhooks/src/http.rs](../../../services/platform/hosted/webhooks/src/http.rs) |
| `rust_pub_item` | `pub struct Request {` | [services/platform/hosted/webhooks/src/http.rs](../../../services/platform/hosted/webhooks/src/http.rs) |
| `rust_pub_item` | `pub struct DeveloperIdentity {` | [services/platform/hosted/webhooks/src/trusted.rs](../../../services/platform/hosted/webhooks/src/trusted.rs) |
