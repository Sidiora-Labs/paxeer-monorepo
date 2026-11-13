# Interop deploy contract

The [interop deployment files](../../../services/interop/deploy) configure the service containers, listeners, manifests and reverse proxies. Endpoint and key-file locations are supplied through the settings declared by each service.

## cli-deploy-gateway-render

`--check` and `--self-test` are Boolean switches and take no values. `--check` validates configuration from the environment; `--self-test` runs the renderer’s self-tests. Rendering requires `--network-id`, `--receipt-keys-file` and `--out`; the other value-taking options below are optional.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-gateway-render</code> | <code>cli_command</code> | [services/interop/deploy/gateway/render.py](../../../services/interop/deploy/gateway/render.py) | <code>python3 services/interop/deploy/gateway/render.py --network-id &lt;value&gt; --receipt-keys-file &lt;value&gt; --beta-roots-file &lt;value&gt; --out &lt;value&gt; --migration-profile-out &lt;value&gt; --repo-root &lt;value&gt;</code> |

## cli-deploy-mirror-deploy-ethereum-mirror

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-ethereum-mirror</code> | <code>cli_command</code> | [services/interop/deploy/mirror/deploy-ethereum-mirror.sh](../../../services/interop/deploy/mirror/deploy-ethereum-mirror.sh) | <code>bash services/interop/deploy/mirror/deploy-ethereum-mirror.sh</code> |

## cli-deploy-mirror-deploy-solana-mirror

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-solana-mirror</code> | <code>cli_command</code> | [services/interop/deploy/mirror/deploy-solana-mirror.sh](../../../services/interop/deploy/mirror/deploy-solana-mirror.sh) | <code>bash services/interop/deploy/mirror/deploy-solana-mirror.sh</code> |

## cli-deploy-mirror-render-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-render-config</code> | <code>cli_command</code> | [services/interop/deploy/mirror/render-config.py](../../../services/interop/deploy/mirror/render-config.py) | <code>python3 services/interop/deploy/mirror/render-config.py --output &lt;value&gt; --state-directory &lt;value&gt; --first-height &lt;value&gt; --status-listen &lt;value&gt; --paxeer-endpoint &lt;value&gt; --paxeer-chain-id &lt;value&gt; --paxeer-genesis-hash &lt;value&gt; --receipt-keys-file &lt;value&gt; --ethereum-endpoint &lt;value&gt; --ethereum-chain-id &lt;value&gt; --ethereum-genesis-hash &lt;value&gt; --ethereum-archive-contract &lt;value&gt; --ethereum-archive-code-hash &lt;value&gt; --ethereum-signer-key-handle &lt;value&gt; --ethereum-signer-public-key &lt;value&gt; --ethereum-signer-socket &lt;value&gt; --solana-endpoint &lt;value&gt; --solana-genesis-hash &lt;value&gt; --solana-archive-program &lt;value&gt; --solana-upgradeable-loader &lt;value&gt; --solana-program-data-account &lt;value&gt; --solana-program-code-hash &lt;value&gt; --solana-signer-key-handle &lt;value&gt; --solana-signer-public-key &lt;value&gt; --solana-signer-socket &lt;value&gt;</code> |

## cli-deploy-mirror-tests-render-config-check

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-tests-render-config-check</code> | <code>cli_command</code> | [services/interop/deploy/mirror/tests/render-config-check.sh](../../../services/interop/deploy/mirror/tests/render-config-check.sh) | <code>bash services/interop/deploy/mirror/tests/render-config-check.sh</code> |

## cli-deploy-x-websearch-tests-dry-run-check

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-dry-run-check</code> | <code>cli_command</code> | [services/interop/deploy/x-websearch/tests/dry-run-check.sh](../../../services/interop/deploy/x-websearch/tests/dry-run-check.sh) | <code>bash services/interop/deploy/x-websearch/tests/dry-run-check.sh</code> |

## cli-deploy-x-websearch-tests-packaging-check

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-packaging-check</code> | <code>cli_command</code> | [services/interop/deploy/x-websearch/tests/packaging-check.sh](../../../services/interop/deploy/x-websearch/tests/packaging-check.sh) | <code>bash services/interop/deploy/x-websearch/tests/packaging-check.sh</code> |

## deploy-gateway-template

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-deploy-gateway-beta-conformance-j72</code> | <code>env_var</code> | [services/interop/deploy/gateway/render.py](../../../services/interop/deploy/gateway/render.py) | <code>LAYERX_BETA_INTEROP_CONFORMANCE_&lt;X402&#124;AP2&#124;UCP&#124;VISA_TAP&#124;FIAT&#124;HTTP&#124;MCP&#124;A2A&gt;: optional &lt;suite-identifier&gt;,&lt;vector-count&gt;,&lt;suite-sha256&gt; overriding the first-party conformance suite of one adapter or transport</code> |
| <code>interface.11.env-deploy-gateway-beta-manifest-file-j72</code> | <code>env_var</code> | [services/interop/deploy/gateway/render.py](../../../services/interop/deploy/gateway/render.py) | <code>LAYERX_BETA_INTEROP_MANIFEST_FILE</code> |
| <code>interface.11.env-deploy-gateway-beta-roots-j72</code> | <code>env_var</code> | [services/interop/deploy/gateway/render.py](../../../services/interop/deploy/gateway/render.py) | <code>LAYERX_BETA_INTEROP_&lt;X402_SUPPORTED&#124;AP2_KEYS&#124;AP2_ASSETS&#124;UCP_PAYMENT_HANDLER&#124;VISA_AGENTS&#124;VISA_TARGETS&#124;FIAT_PROVIDERS&gt;: optional JSON trust roots replacing the cluster defaults</code> |

## fmt-deploy-bridge-relayer-relayer-1

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-1</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/relayer-1.toml](../../../services/interop/deploy/bridge-relayer/relayer-1.toml) | <code>TOML sections [build], [env], [[files]], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, env.LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, env.LAYERX_BRIDGE_OPERATOR_INVENTORY, env.LAYERX_BRIDGE_RELAYER_CONFIG, env.LAYERX_BRIDGE_ATTESTOR_INDEX, env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE, env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD, env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE, env.LAYERX_BRIDGE_COSIGN_CA, env.LAYERX_BRIDGE_APPROVED_MEMBERSHIP, files.guest_path, files.secret_name, mounts.source, mounts.destination, mounts.initial_size, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-bridge-relayer-relayer-2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-2</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/relayer-2.toml](../../../services/interop/deploy/bridge-relayer/relayer-2.toml) | <code>TOML sections [build], [env], [[files]], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, env.LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, env.LAYERX_BRIDGE_OPERATOR_INVENTORY, env.LAYERX_BRIDGE_RELAYER_CONFIG, env.LAYERX_BRIDGE_ATTESTOR_INDEX, env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE, env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD, env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE, env.LAYERX_BRIDGE_COSIGN_CA, env.LAYERX_BRIDGE_APPROVED_MEMBERSHIP, files.guest_path, files.secret_name, mounts.source, mounts.destination, mounts.initial_size, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-bridge-relayer-relayer-3

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-3</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/relayer-3.toml](../../../services/interop/deploy/bridge-relayer/relayer-3.toml) | <code>TOML sections [build], [env], [[files]], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, env.LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, env.LAYERX_BRIDGE_OPERATOR_INVENTORY, env.LAYERX_BRIDGE_RELAYER_CONFIG, env.LAYERX_BRIDGE_ATTESTOR_INDEX, env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE, env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD, env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE, env.LAYERX_BRIDGE_COSIGN_CA, env.LAYERX_BRIDGE_APPROVED_MEMBERSHIP, files.guest_path, files.secret_name, mounts.source, mounts.destination, mounts.initial_size, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-bridge-relayer-relayer-4

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-4</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/relayer-4.toml](../../../services/interop/deploy/bridge-relayer/relayer-4.toml) | <code>TOML sections [build], [env], [[files]], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, env.LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, env.LAYERX_BRIDGE_OPERATOR_INVENTORY, env.LAYERX_BRIDGE_RELAYER_CONFIG, env.LAYERX_BRIDGE_ATTESTOR_INDEX, env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE, env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD, env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE, env.LAYERX_BRIDGE_COSIGN_CA, env.LAYERX_BRIDGE_APPROVED_MEMBERSHIP, files.guest_path, files.secret_name, mounts.source, mounts.destination, mounts.initial_size, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-bridge-relayer-relayer-5

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-5</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/relayer-5.toml](../../../services/interop/deploy/bridge-relayer/relayer-5.toml) | <code>TOML sections [build], [env], [[files]], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, env.LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, env.LAYERX_BRIDGE_OPERATOR_INVENTORY, env.LAYERX_BRIDGE_RELAYER_CONFIG, env.LAYERX_BRIDGE_ATTESTOR_INDEX, env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE, env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD, env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE, env.LAYERX_BRIDGE_COSIGN_CA, env.LAYERX_BRIDGE_APPROVED_MEMBERSHIP, files.guest_path, files.secret_name, mounts.source, mounts.destination, mounts.initial_size, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-bridge-relayer-signer-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-signer-keys</code> | <code>file_format</code> | [services/interop/deploy/bridge-relayer/signer-keys.json](../../../services/interop/deploy/bridge-relayer/signer-keys.json) | <code>JSON object with keys keys</code> |

## fmt-deploy-gas-station-hosted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gas-station-hosted</code> | <code>file_format</code> | [services/interop/deploy/gas-station/railway.env.example](../../../services/interop/deploy/gas-station/railway.env.example) | <code>Environment file with keys PORT, GAS_STATION_LISTEN, GAS_STATION_CHAIN_ID, GAS_STATION_ENDPOINTS, GAS_STATION_TOKEN, GAS_STATION_DECIMALS, GAS_STATION_SPREAD_BPS, GAS_STATION_RELAYER_KEY_ENV, GAS_STATION_RATE_OWNER_KEY_ENV, GAS_STATION_RATE_CADENCE_SECONDS, GAS_STATION_RATE_CONFIRMATION_RETRY_SECONDS, GAS_STATION_RATE_MAX_FEE_PER_GAS, GAS_STATION_PAYMASTER, GAS_STATION_MAX_RATE_AGE, GAS_STATION_MARGIN_BPS, GAS_STATION_PER_QUOTE_LIMIT, GAS_STATION_PER_ACCOUNT_LIMIT, GAS_STATION_PER_INTERVAL_LIMIT, GAS_STATION_INTERVAL_SECONDS, GAS_STATION_BALANCE_FLOOR, GAS_STATION_GAS_LIMIT, GAS_STATION_MAX_PRIORITY_FEE_PER_GAS, GAS_STATION_DID_GAS_LIMIT, GAS_STATION_RATE_GAS_BUDGET_PER_DAY, GAS_STATION_RATE_BALANCE_FLOOR, GAS_STATION_BROWSER_ORIGINS, GAS_STATION_RELAYER_KEY, GAS_STATION_RATE_OWNER_KEY</code> |

## fmt-deploy-gateway-config-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-config-example</code> | <code>file_format</code> | [services/interop/deploy/gateway/config.example.json](../../../services/interop/deploy/gateway/config.example.json) | <code>JSON object with keys adapters, ap2_assets, ap2_keys, fiat_providers, transports, ucp_payment_handler, visa_agents, visa_targets, x402_supported</code> |

## fmt-deploy-gateway-hosted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-hosted</code> | <code>file_format</code> | [services/interop/deploy/gateway/railway.env.example](../../../services/interop/deploy/gateway/railway.env.example) | <code>Environment file with keys LAYERX_INTEROP_LISTENER, PORT, LAYERX_INTEROP_LISTEN, LAYERX_INTEROP_CHAIN_ID, LAYERX_INTEROP_PROTOCOL_NETWORK_ID, LAYERX_INTEROP_WIRE_VERSION, LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS, LAYERX_INTEROP_IDEMPOTENCY_SECONDS, LAYERX_INTEROP_NETWORK_ID, LAYERX_INTEROP_READINESS_MAX_AGE_MS, LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK, LAYERX_INTEROP_HOSTED_GATEWAY_URL, LAYERX_INTEROP_REDIS_URL, LAYERX_INTEROP_CONFIG, LAYERX_INTEROP_MODULE_REGISTRY_FILE, LAYERX_INTEROP_OUTBOUND_CA_DER, LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12, LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE, LAYERX_INTEROP_RECEIPT_KEYS_FILE, LAYERX_INTEROP_REDIS_USERNAME_FILE, LAYERX_INTEROP_REDIS_PASSWORD_FILE, LAYERX_INTEROP_TLS_CERT_DER, LAYERX_INTEROP_TLS_KEY_DER, LAYERX_INTEROP_MIGRATION_V2_CONFIG, INTEROP_CONFIG, INTEROP_MODULE_REGISTRY, INTEROP_CLIENT_CA_DER, INTEROP_CLIENT_P12, INTEROP_CLIENT_PASSWORD, INTEROP_RECEIPT_KEYS, INTEROP_REDIS_USERNAME, INTEROP_REDIS_PASSWORD</code> |

## fmt-deploy-gateway-module-registry-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-module-registry-example</code> | <code>file_format</code> | [services/interop/deploy/gateway/module-registry.example.json](../../../services/interop/deploy/gateway/module-registry.example.json) | <code>JSON object with keys assets, modules, schema_version</code> |

## fmt-deploy-gateway-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-readme</code> | <code>file_format</code> | [services/interop/deploy/gateway/README.md](../../../services/interop/deploy/gateway/README.md) | <code>Markdown document: Interop gateway runtime additions</code> |

## fmt-deploy-gateway-runtime

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-runtime</code> | <code>file_format</code> | [services/interop/deploy/gateway/runtime.env.example](../../../services/interop/deploy/gateway/runtime.env.example) | <code>Environment file with keys LAYERX_INTEROP_PROTOCOL_NETWORK_ID, LAYERX_INTEROP_MODULE_REGISTRY_FILE, LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS</code> |

## fmt-deploy-gateway-visa-trust-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-visa-trust-example</code> | <code>file_format</code> | [services/interop/deploy/gateway/visa-trust.example.json](../../../services/interop/deploy/gateway/visa-trust.example.json) | <code>JSON object with keys visa_agents, visa_targets</code> |

## fmt-deploy-mirror-config-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-config-example</code> | <code>file_format</code> | [services/interop/deploy/mirror/config.example.json](../../../services/interop/deploy/mirror/config.example.json) | <code>JSON object with keys ethereum, first_height, freshness_budget_blocks, paxeer, poll_interval_ms, solana, state_directory, status_listen</code> |

## fmt-deploy-mirror-ethereum-deployment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-ethereum-deployment</code> | <code>file_format</code> | [services/interop/deploy/mirror/ethereum-deployment.json](../../../services/interop/deploy/mirror/ethereum-deployment.json) | <code>JSON object with keys constructor_arguments, contract, required_post_deploy_records, source</code> |

## fmt-deploy-mirror-ethereum-testnets-dated

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-ethereum-testnets-dated</code> | <code>file_format</code> | [services/interop/deploy/mirror/ethereum-testnets-2026-08-31.json](../../../services/interop/deploy/mirror/ethereum-testnets-2026-08-31.json) | <code>JSON object with keys compiler, contract, deployments, evm_version, optimizer, publisher, runtime_code_keccak256, source, source_commit</code> |

## fmt-deploy-mirror-signer-protocol

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-signer-protocol</code> | <code>file_format</code> | [services/interop/deploy/mirror/signer-protocol.md](../../../services/interop/deploy/mirror/signer-protocol.md) | <code>Markdown document: Mirror remote signer protocol</code> |

## fmt-deploy-mirror-solana-deployment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-solana-deployment</code> | <code>file_format</code> | [services/interop/deploy/mirror/solana-deployment.json](../../../services/interop/deploy/mirror/solana-deployment.json) | <code>JSON object with keys archive_namespace, program, required_post_deploy_records, source, upgrade_policy</code> |

## fmt-deploy-mirror-solana-devnet-dated

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-solana-devnet-dated</code> | <code>file_format</code> | [services/interop/deploy/mirror/solana-devnet-2026-08-31.json](../../../services/interop/deploy/mirror/solana-devnet-2026-08-31.json) | <code>JSON object with keys deployment_fee_lamports, deployment_finalized_slot, deployment_signature, finalization_fee_lamports, finalization_finalized_slot, finalization_signature, genesis_hash, network, on_chain_elf_verified, program, program_account_rent_lamports, program_data_account, program_data_rent_lamports, program_elf_bytes, program_elf_sha256, program_id, publisher_ed25519_public_key, source, source_commit, toolchain, upgrade_authority, upgradeable_loader_id</code> |

## fmt-deploy-mirror-verify-config-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-verify-config-example</code> | <code>file_format</code> | [services/interop/deploy/mirror/verify-config.example.json](../../../services/interop/deploy/mirror/verify-config.example.json) | <code>JSON object with keys layerx_network_id, receipt_keys, sources</code> |

## fmt-deploy-search-front-hosted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-search-front-hosted</code> | <code>file_format</code> | [services/interop/deploy/search-front/railway.env.example](../../../services/interop/deploy/search-front/railway.env.example) | <code>Environment file with keys PORT, LAYERX_TRUSTED_PROXIES, SEARCH_FRONT_UPSTREAMS</code> |

## fmt-deploy-search-front-nginx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-search-front-nginx</code> | <code>file_format</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>nginx configuration: location = /healthz, location = /readyz, location = /_search_ready, location @not_ready, location = /xweb/health, location ~ ^/(search&#124;fetch&#124;content/), location /</code> |

## fmt-deploy-x-websearch-attestor-1

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-1</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/attestor-1.toml](../../../services/interop/deploy/x-websearch/attestor-1.toml) | <code>TOML sections [build], [deploy], [env], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, build.build-target, deploy.strategy, env.X_WEBSEARCH_ATTESTOR_KEY_FILE, env.X_WEBSEARCH_SUBMITTER_KEY_FILE, env.X_WEBSEARCH_RECEIVER_KEY_FILE, env.XWEB_HOPS, mounts.source, mounts.destination, mounts.initial_size, mounts.scheduled_snapshots, mounts.snapshot_retention, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-2</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/attestor-2.toml](../../../services/interop/deploy/x-websearch/attestor-2.toml) | <code>TOML sections [build], [deploy], [env], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, build.build-target, deploy.strategy, env.X_WEBSEARCH_ATTESTOR_KEY_FILE, env.X_WEBSEARCH_SUBMITTER_KEY_FILE, env.X_WEBSEARCH_RECEIVER_KEY_FILE, env.XWEB_HOPS, mounts.source, mounts.destination, mounts.initial_size, mounts.scheduled_snapshots, mounts.snapshot_retention, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-3

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-3</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/attestor-3.toml](../../../services/interop/deploy/x-websearch/attestor-3.toml) | <code>TOML sections [build], [deploy], [env], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, build.build-target, deploy.strategy, env.X_WEBSEARCH_ATTESTOR_KEY_FILE, env.X_WEBSEARCH_SUBMITTER_KEY_FILE, env.X_WEBSEARCH_RECEIVER_KEY_FILE, env.XWEB_HOPS, mounts.source, mounts.destination, mounts.initial_size, mounts.scheduled_snapshots, mounts.snapshot_retention, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-4

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-4</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/attestor-4.toml](../../../services/interop/deploy/x-websearch/attestor-4.toml) | <code>TOML sections [build], [deploy], [env], [[mounts]], [[vm]]; keys app, primary_region, kill_signal, kill_timeout, build.dockerfile, build.build-target, deploy.strategy, env.X_WEBSEARCH_ATTESTOR_KEY_FILE, env.X_WEBSEARCH_SUBMITTER_KEY_FILE, env.X_WEBSEARCH_RECEIVER_KEY_FILE, env.XWEB_HOPS, mounts.source, mounts.destination, mounts.initial_size, mounts.scheduled_snapshots, mounts.snapshot_retention, vm.size, vm.memory, vm.cpus, vm.cpu_kind</code> |

## fmt-deploy-x-websearch-config-example

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-config-example</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/config.example.json](../../../services/interop/deploy/x-websearch/config.example.json) | <code>JSON object with keys assets, crawl, data_dir, evm, fetch, gateway, listen, note, peers, seeds</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## fmt-deploy-x-websearch-nginx-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-nginx-search</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/nginx-search.conf](../../../services/interop/deploy/x-websearch/nginx-search.conf) | <code>nginx configuration: location ~ ^/(search&#124;fetch&#124;content/), location = /xweb/health</code> |

## fmt-deploy-x-websearch-tests-fixtures-dry-run

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-dry-run</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/tests/fixtures/dry-run.json](../../../services/interop/deploy/x-websearch/tests/fixtures/dry-run.json) | <code>JSON object with keys attestors, callback_gas, calls, chain_id, consumer, credential_header, credential_scan, legs, pages, precompile, submitter, threshold, version, views</code> |

## fmt-deploy-x-websearch-tests-fixtures-site-guide

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-guide</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/tests/fixtures/site/guide.html](../../../services/interop/deploy/x-websearch/tests/fixtures/site/guide.html) | <code>HTML fixture page</code> |

## fmt-deploy-x-websearch-tests-fixtures-site-index

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-index</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/tests/fixtures/site/index.html](../../../services/interop/deploy/x-websearch/tests/fixtures/site/index.html) | <code>HTML fixture page</code> |

## fmt-deploy-x-websearch-tests-fixtures-site-notes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-notes</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/tests/fixtures/site/notes.txt](../../../services/interop/deploy/x-websearch/tests/fixtures/site/notes.txt) | <code>plain text fixture</code> |

## fmt-deploy-x-websearch-tests-fixtures-site-robots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-robots</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/tests/fixtures/site/robots.txt](../../../services/interop/deploy/x-websearch/tests/fixtures/site/robots.txt) | <code>plain text fixture</code> |

## fmt-deploy-x-websearch-x-websearch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-x-websearch</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/x-websearch.service](../../../services/interop/deploy/x-websearch/x-websearch.service) | <code>systemd unit; User=...; LoadCredential=...; LoadCredential=...; LoadCredential=...; ExecStart=/usr/local/bin/x-websearch; StateDirectory=...</code> |

## fmt-deploy-x-websearch-x-websearch-serve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-x-websearch-serve</code> | <code>file_format</code> | [services/interop/deploy/x-websearch/x-websearch-serve.service](../../../services/interop/deploy/x-websearch/x-websearch-serve.service) | <code>systemd unit; User=...; LoadCredential=...; ExecStart=/usr/local/bin/x-websearch; StateDirectory=...</code> |

## http-search-front-default

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-default</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>any other path</code> |

## http-search-front-healthz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-healthz</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>GET /healthz</code> |

## http-search-front-not-ready

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-not-ready</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>named location @not_ready</code> |

## http-search-front-readyz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-readyz</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>GET /readyz</code> |

## http-search-front-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-search</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>GET /search &#124; /fetch &#124; /content/&lt;digest&gt; (regex ^/(search&#124;fetch&#124;content/))</code> |

## http-search-front-search-ready

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-search-ready</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>internal auth_request /_search_ready</code> |

## http-search-front-xweb-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-search-front-xweb-health</code> | <code>http_route</code> | [services/interop/deploy/search-front/nginx.conf](../../../services/interop/deploy/search-front/nginx.conf) | <code>GET /xweb/health</code> |

## http-xws-nginx-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-nginx-search</code> | <code>http_route</code> | [services/interop/deploy/x-websearch/nginx-search.conf](../../../services/interop/deploy/x-websearch/nginx-search.conf) | <code>GET /search &#124; /fetch &#124; /content/&lt;digest&gt; (include for a sidecar host)</code> |

## http-xws-nginx-xweb-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-nginx-xweb-health</code> | <code>http_route</code> | [services/interop/deploy/x-websearch/nginx-search.conf](../../../services/interop/deploy/x-websearch/nginx-search.conf) | <code>GET /xweb/health (include for a sidecar host)</code> |
