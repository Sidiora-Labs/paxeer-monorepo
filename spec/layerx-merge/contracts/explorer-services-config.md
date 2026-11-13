# Explorer services config contract

This page describes the settings read by the explorer signature provider and smart-contract verifier in the current tree. The settings types define the supported keys.

Paxeer mainnet-beta is one BFT network, with the kernel inside the node participating in execution and block proposal. EVM chain ID is `125` (`0x7d`); receipts are final when their block commits. The explorer services below supply signatures and verify contract source against bytecode. Their HTTP/gRPC listeners and compiler/source-provider URLs are service configuration, not additional chain endpoints. Sub-chains are tenant domains in the database.

For node access, use the root JSON-RPC URL configured for the node. See [public RPC endpoints](../../../docs/site/docs/reference/public-rpc.md) and [explorer configuration](explorer-config.md).

## Dependencies and configuration loading

The server manifests inherit `blockscout-service-launcher` from their respective workspaces. The lockfiles resolve different registry releases:

| Consumer | Workspace declaration | Resolved producer | Evidence |
| --- | --- | --- | --- |
| Signature provider | `0.10.0` | crates.io `blockscout-service-launcher` **0.10.1** | [workspace manifest][sig-workspace], [server manifest][sig-manifest], [lockfile][sig-lock] |
| Smart-contract verifier | `0.17.0` | crates.io `blockscout-service-launcher` **0.17.0** | [workspace manifest][verifier-workspace], [server manifest][verifier-manifest], [lockfile][verifier-lock] |

The repository's [local launcher][local-launcher] is version **0.22.0** and is not the linked producer for either server. Shared settings below cite the versioned registry source, not that local crate. Service-specific settings cite each server's own `src/settings.rs`. The types and supported keys follow the linked producers.

Both executable entry points call `Settings::build()` before starting their server. In both linked launcher versions, `ConfigSettings::build` reads an optional file named by `<SERVICE_NAME>__CONFIG`, adds the service-prefixed environment as an overlay, deserializes settings, then calls `validate`. The loader removes the config-file variable before reading the environment overlay. Environment values override file values; defaults come from the deserialized settings types. Unknown fields are rejected. These settings need no storage, gas, activation or pause rules.

| Service | `SERVICE_NAME` | File selector | Nested-key separator |
| --- | --- | --- | --- |
| Signature provider | `SIG_PROVIDER` | `SIG_PROVIDER__CONFIG` | `__` |
| Smart-contract verifier | `SMART_CONTRACT_VERIFIER` | `SMART_CONTRACT_VERIFIER__CONFIG` | `__` |

For example, `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ADDR` sets `server.http.addr`; underscores within a key, such as `max_body_size`, remain single underscores. Both loaders use `config::File::with_name` for the selected file; the checked-in examples are TOML. The [0.10.1 loader][launcher010] and [0.17.0 loader][launcher017] define this behavior at lines 9–29.

Both linked versions default to HTTP enabled on port `8050` with a `2097152`-byte body limit, gRPC disabled on port `8051`, and metrics disabled on port `6060` with route `/metrics`. Give co-located services distinct listen addresses. Tracing defaults to enabled with format `default` (`json` is the other accepted value); Jaeger defaults to disabled with agent port `6831` on the loopback interface. Checked-in configuration files can override defaults.

The tables below list the settings supported by each linked launcher version.

## cfg-sig-provider

The [signature provider settings][sig-settings] implement `ConfigSettings` with `SERVICE_NAME = "SIG_PROVIDER"`. Shared server/metrics fields come from [launcher 0.10.1][launcher010]; tracing/Jaeger fields come from [its tracing settings][tracing010].

`SourcesSettings` defaults to the 4byte and sigeth URLs shown in the [base configuration][sig-base], with the precompile signature source and eth-bytecode-db source enabled. `sources.precompiles.enabled` is read by `new_service` in [run.rs][sig-run]; it selects a signature lookup source, not a chain deployment.

| Interface | Setting and Rust type | Definition |
| --- | --- | --- |
| `interface.14.cfg-sig-provider-jaeger-agent-endpoint` | <code>jaeger.agent_endpoint: String</code> | [launcher 0.10.1 tracing:32][tracing010] |
| `interface.14.cfg-sig-provider-jaeger-enabled` | <code>jaeger.enabled: bool</code> | [launcher 0.10.1 tracing:31][tracing010] |
| `interface.14.cfg-sig-provider-metrics-addr` | <code>metrics.addr: SocketAddr</code> | [launcher 0.10.1 settings:127][launcher010] |
| `interface.14.cfg-sig-provider-metrics-enabled` | <code>metrics.enabled: bool</code> | [launcher 0.10.1 settings:126][launcher010] |
| `interface.14.cfg-sig-provider-metrics-route` | <code>metrics.route: String</code> | [launcher 0.10.1 settings:128][launcher010] |
| `interface.14.cfg-sig-provider-server-grpc-addr` | <code>server.grpc.addr: SocketAddr</code> | [launcher 0.10.1 settings:111][launcher010] |
| `interface.14.cfg-sig-provider-server-grpc-enabled` | <code>server.grpc.enabled: bool</code> | [launcher 0.10.1 settings:110][launcher010] |
| `interface.14.cfg-sig-provider-server-http-addr` | <code>server.http.addr: SocketAddr</code> | [launcher 0.10.1 settings:50][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-allowed-credentials` | <code>server.http.cors.allowed_credentials: bool</code> | [launcher 0.10.1 settings:72][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-allowed-methods` | <code>server.http.cors.allowed_methods: String</code> | [launcher 0.10.1 settings:71][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-allowed-origin` | <code>server.http.cors.allowed_origin: String</code> | [launcher 0.10.1 settings:70][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-block-on-origin-mismatch` | <code>server.http.cors.block_on_origin_mismatch: bool</code> | [launcher 0.10.1 settings:74][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-enabled` | <code>server.http.cors.enabled: bool</code> | [launcher 0.10.1 settings:69][launcher010] |
| `interface.14.cfg-sig-provider-server-http-cors-max-age` | <code>server.http.cors.max_age: usize</code> | [launcher 0.10.1 settings:73][launcher010] |
| `interface.14.cfg-sig-provider-server-http-enabled` | <code>server.http.enabled: bool</code> | [launcher 0.10.1 settings:49][launcher010] |
| `interface.14.cfg-sig-provider-server-http-max-body-size` | <code>server.http.max_body_size: usize</code> | [launcher 0.10.1 settings:51][launcher010] |
| `interface.14.cfg-sig-provider-sources-eth-bytecode-db-enabled` | <code>sources.eth_bytecode_db.enabled: bool</code> | [server settings:62][sig-settings] |
| `interface.14.cfg-sig-provider-sources-eth-bytecode-db-url` | <code>sources.eth_bytecode_db.url: url::Url</code> | [server settings:63][sig-settings] |
| `interface.14.cfg-sig-provider-sources-fourbyte` | <code>sources.fourbyte: url::Url</code> | [server settings:31][sig-settings] |
| `interface.14.cfg-sig-provider-sources-sigeth` | <code>sources.sigeth: url::Url</code> | [server settings:32][sig-settings] |
| `interface.14.cfg-sig-provider-tracing-enabled` | <code>tracing.enabled: bool</code> | [launcher 0.10.1 tracing:15][tracing010] |
| `interface.14.cfg-sig-provider-tracing-format` | <code>tracing.format: TracingFormat</code> | [launcher 0.10.1 tracing:16][tracing010] |
| — (additional implemented setting) | <code>sources.precompiles.enabled: bool</code> | [server settings:50][sig-settings] |

## cfg-verifier

The [verifier settings][verifier-settings] implement `ConfigSettings` with `SERVICE_NAME = "SMART_CONTRACT_VERIFIER"`. Shared server/metrics fields come from [launcher 0.17.0][launcher017]; tracing/Jaeger fields come from [its tracing settings][tracing017].

Solidity, Vyper, Sourcify and Zksync Solidity default to enabled. Compiler directories are under `std::env::temp_dir()`: `solidity-compilers`, `vyper-compilers`, `zksync-solc-compilers` and `zksync-zksolc-compilers`. Refresh schedules default to `0 0 * * * * *` (hourly). `compilers.max_threads` defaults to available parallelism, or 8 when that query fails; the [base TOML][verifier-base] explicitly selects 8. Both that field and `sourcify.verification_attempts` must be nonzero.

`FetcherSettings` is an externally tagged enum: choose one `list` or `s3` table for each fetcher, rather than combining both variants. A list requires `list_url`; S3 requires `bucket` and has optional credential, region and endpoint strings. `Settings::validate` additionally requires at least one of `region` or `endpoint` for the **Solidity** S3 fetcher; it does not apply that check to the other fetchers. Default list URLs come from the constants imported by `settings.rs`.

Sourcify defaults to the URL in `DEFAULT_SOURCIFY_HOST`, `verification_attempts = 3` and `request_timeout = 15`. The timeout is in seconds in [SourcifyApiClient][sourcify-client]; this setting is not an overall compilation timeout. The Zksync compiler settings select verification toolchains, not another Paxeer execution or settlement network. See [verifier HTTP methods](verifier-http.md) for the service API.

| Interface | Setting and Rust type | Definition |
| --- | --- | --- |
| `interface.14.cfg-verifier-compilers-max-threads` | <code>compilers.max_threads: NonZeroUsize</code> | [server settings:157][verifier-settings] |
| `interface.14.cfg-verifier-jaeger-agent-endpoint` | <code>jaeger.agent_endpoint: String</code> | [launcher 0.17.0 tracing:32][tracing017] |
| `interface.14.cfg-verifier-jaeger-enabled` | <code>jaeger.enabled: bool</code> | [launcher 0.17.0 tracing:31][tracing017] |
| `interface.14.cfg-verifier-metrics-addr` | <code>metrics.addr: SocketAddr</code> | [launcher 0.17.0 settings:140][launcher017] |
| `interface.14.cfg-verifier-metrics-enabled` | <code>metrics.enabled: bool</code> | [launcher 0.17.0 settings:139][launcher017] |
| `interface.14.cfg-verifier-metrics-route` | <code>metrics.route: String</code> | [launcher 0.17.0 settings:141][launcher017] |
| `interface.14.cfg-verifier-server-grpc-addr` | <code>server.grpc.addr: SocketAddr</code> | [launcher 0.17.0 settings:124][launcher017] |
| `interface.14.cfg-verifier-server-grpc-enabled` | <code>server.grpc.enabled: bool</code> | [launcher 0.17.0 settings:123][launcher017] |
| `interface.14.cfg-verifier-server-http-addr` | <code>server.http.addr: SocketAddr</code> | [launcher 0.17.0 settings:50][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-allowed-credentials` | <code>server.http.cors.allowed_credentials: bool</code> | [launcher 0.17.0 settings:72][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-allowed-methods` | <code>server.http.cors.allowed_methods: String</code> | [launcher 0.17.0 settings:71][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-allowed-origin` | <code>server.http.cors.allowed_origin: String</code> | [launcher 0.17.0 settings:70][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-block-on-origin-mismatch` | <code>server.http.cors.block_on_origin_mismatch: bool</code> | [launcher 0.17.0 settings:74][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-enabled` | <code>server.http.cors.enabled: bool</code> | [launcher 0.17.0 settings:69][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-max-age` | <code>server.http.cors.max_age: usize</code> | [launcher 0.17.0 settings:73][launcher017] |
| `interface.14.cfg-verifier-server-http-cors-send-wildcard` | <code>server.http.cors.send_wildcard: bool</code> | [launcher 0.17.0 settings:75][launcher017] |
| `interface.14.cfg-verifier-server-http-enabled` | <code>server.http.enabled: bool</code> | [launcher 0.17.0 settings:49][launcher017] |
| `interface.14.cfg-verifier-server-http-max-body-size` | <code>server.http.max_body_size: usize</code> | [launcher 0.17.0 settings:51][launcher017] |
| `interface.14.cfg-verifier-solidity-compilers-dir` | <code>solidity.compilers_dir: PathBuf</code> | [server settings:39][verifier-settings] |
| `interface.14.cfg-verifier-solidity-enabled` | <code>solidity.enabled: bool</code> | [server settings:38][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-list-list-url` | <code>solidity.fetcher.list.list_url: Url</code> | [server settings:88][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-s3-access-key` | <code>solidity.fetcher.s3.access_key: Option&lt;String&gt;</code> | [server settings:94][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-s3-bucket` | <code>solidity.fetcher.s3.bucket: String</code> | [server settings:98][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-s3-endpoint` | <code>solidity.fetcher.s3.endpoint: Option&lt;String&gt;</code> | [server settings:97][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-s3-region` | <code>solidity.fetcher.s3.region: Option&lt;String&gt;</code> | [server settings:96][verifier-settings] |
| `interface.14.cfg-verifier-solidity-fetcher-s3-secret-key` | <code>solidity.fetcher.s3.secret_key: Option&lt;String&gt;</code> | [server settings:95][verifier-settings] |
| `interface.14.cfg-verifier-solidity-refresh-versions-schedule` | <code>solidity.refresh_versions_schedule: Schedule</code> | [server settings:41][verifier-settings] |
| `interface.14.cfg-verifier-sourcify-api-url` | <code>sourcify.api_url: Url</code> | [server settings:105][verifier-settings] |
| `interface.14.cfg-verifier-sourcify-enabled` | <code>sourcify.enabled: bool</code> | [server settings:104][verifier-settings] |
| `interface.14.cfg-verifier-sourcify-request-timeout` | <code>sourcify.request_timeout: u64</code> | [server settings:109][verifier-settings] |
| `interface.14.cfg-verifier-sourcify-verification-attempts` | <code>sourcify.verification_attempts: NonZeroU32</code> | [server settings:108][verifier-settings] |
| `interface.14.cfg-verifier-tracing-enabled` | <code>tracing.enabled: bool</code> | [launcher 0.17.0 tracing:15][tracing017] |
| `interface.14.cfg-verifier-tracing-format` | <code>tracing.format: TracingFormat</code> | [launcher 0.17.0 tracing:16][tracing017] |
| `interface.14.cfg-verifier-vyper-compilers-dir` | <code>vyper.compilers_dir: PathBuf</code> | [server settings:61][verifier-settings] |
| `interface.14.cfg-verifier-vyper-enabled` | <code>vyper.enabled: bool</code> | [server settings:60][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-list-list-url` | <code>vyper.fetcher.list.list_url: Url</code> | [server settings:88][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-s3-access-key` | <code>vyper.fetcher.s3.access_key: Option&lt;String&gt;</code> | [server settings:94][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-s3-bucket` | <code>vyper.fetcher.s3.bucket: String</code> | [server settings:98][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-s3-endpoint` | <code>vyper.fetcher.s3.endpoint: Option&lt;String&gt;</code> | [server settings:97][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-s3-region` | <code>vyper.fetcher.s3.region: Option&lt;String&gt;</code> | [server settings:96][verifier-settings] |
| `interface.14.cfg-verifier-vyper-fetcher-s3-secret-key` | <code>vyper.fetcher.s3.secret_key: Option&lt;String&gt;</code> | [server settings:95][verifier-settings] |
| `interface.14.cfg-verifier-vyper-refresh-versions-schedule` | <code>vyper.refresh_versions_schedule: Schedule</code> | [server settings:63][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-enabled` | <code>zksync_solidity.enabled: bool</code> | [server settings:127][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-list-list-url` | <code>zksync_solidity.era_evm_fetcher.list.list_url: Url</code> | [server settings:88][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-s3-access-key` | <code>zksync_solidity.era_evm_fetcher.s3.access_key: Option&lt;String&gt;</code> | [server settings:94][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-s3-bucket` | <code>zksync_solidity.era_evm_fetcher.s3.bucket: String</code> | [server settings:98][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-s3-endpoint` | <code>zksync_solidity.era_evm_fetcher.s3.endpoint: Option&lt;String&gt;</code> | [server settings:97][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-s3-region` | <code>zksync_solidity.era_evm_fetcher.s3.region: Option&lt;String&gt;</code> | [server settings:96][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-era-evm-fetcher-s3-secret-key` | <code>zksync_solidity.era_evm_fetcher.s3.secret_key: Option&lt;String&gt;</code> | [server settings:95][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-compilers-dir` | <code>zksync_solidity.evm_compilers_dir: PathBuf</code> | [server settings:128][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-list-list-url` | <code>zksync_solidity.evm_fetcher.list.list_url: Url</code> | [server settings:88][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-s3-access-key` | <code>zksync_solidity.evm_fetcher.s3.access_key: Option&lt;String&gt;</code> | [server settings:94][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-s3-bucket` | <code>zksync_solidity.evm_fetcher.s3.bucket: String</code> | [server settings:98][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-s3-endpoint` | <code>zksync_solidity.evm_fetcher.s3.endpoint: Option&lt;String&gt;</code> | [server settings:97][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-s3-region` | <code>zksync_solidity.evm_fetcher.s3.region: Option&lt;String&gt;</code> | [server settings:96][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-fetcher-s3-secret-key` | <code>zksync_solidity.evm_fetcher.s3.secret_key: Option&lt;String&gt;</code> | [server settings:95][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-evm-refresh-versions-schedule` | <code>zksync_solidity.evm_refresh_versions_schedule: Schedule</code> | [server settings:130][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-compilers-dir` | <code>zksync_solidity.zk_compilers_dir: PathBuf</code> | [server settings:133][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-list-list-url` | <code>zksync_solidity.zk_fetcher.list.list_url: Url</code> | [server settings:88][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-s3-access-key` | <code>zksync_solidity.zk_fetcher.s3.access_key: Option&lt;String&gt;</code> | [server settings:94][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-s3-bucket` | <code>zksync_solidity.zk_fetcher.s3.bucket: String</code> | [server settings:98][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-s3-endpoint` | <code>zksync_solidity.zk_fetcher.s3.endpoint: Option&lt;String&gt;</code> | [server settings:97][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-s3-region` | <code>zksync_solidity.zk_fetcher.s3.region: Option&lt;String&gt;</code> | [server settings:96][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-fetcher-s3-secret-key` | <code>zksync_solidity.zk_fetcher.s3.secret_key: Option&lt;String&gt;</code> | [server settings:95][verifier-settings] |
| `interface.14.cfg-verifier-zksync-solidity-zk-refresh-versions-schedule` | <code>zksync_solidity.zk_refresh_versions_schedule: Schedule</code> | [server settings:135][verifier-settings] |

## env-launcher

This row documents the **local launcher test helper**, independently of the registry dependencies above. [TestDbGuard::new][test-database] reads `DATABASE_URL` to initialize a test database and expects it to be set. It is not a field of either production server's `Settings` and does not configure tenant domains.

| Interface | Environment variable | Maps to |
| --- | --- | --- |
| `interface.14.env-launcher-database-url` | `DATABASE_URL` | [Local test database helper:24][test-database] |

## env-sig-provider

These variables use the [0.10.1 loader][launcher010] and map to the supported keys in [cfg-sig-provider](#cfg-sig-provider). `SIG_PROVIDER__CONFIG` selects the configuration file; all other rows are environment overrides. Source definitions and types are in that settings table. See [signature provider HTTP methods](sig-provider-http.md) for its API.

| Interface | Environment variable | Maps to |
| --- | --- | --- |
| `interface.14.env-sig-provider-config` | `SIG_PROVIDER__CONFIG` | Configuration file path |
| `interface.14.env-sig-provider-jaeger-agent-endpoint` | `SIG_PROVIDER__JAEGER__AGENT_ENDPOINT` | `jaeger.agent_endpoint` |
| `interface.14.env-sig-provider-jaeger-enabled` | `SIG_PROVIDER__JAEGER__ENABLED` | `jaeger.enabled` |
| `interface.14.env-sig-provider-metrics-addr` | `SIG_PROVIDER__METRICS__ADDR` | `metrics.addr` |
| `interface.14.env-sig-provider-metrics-enabled` | `SIG_PROVIDER__METRICS__ENABLED` | `metrics.enabled` |
| `interface.14.env-sig-provider-metrics-route` | `SIG_PROVIDER__METRICS__ROUTE` | `metrics.route` |
| `interface.14.env-sig-provider-server-grpc-addr` | `SIG_PROVIDER__SERVER__GRPC__ADDR` | `server.grpc.addr` |
| `interface.14.env-sig-provider-server-grpc-enabled` | `SIG_PROVIDER__SERVER__GRPC__ENABLED` | `server.grpc.enabled` |
| `interface.14.env-sig-provider-server-http-addr` | `SIG_PROVIDER__SERVER__HTTP__ADDR` | `server.http.addr` |
| `interface.14.env-sig-provider-server-http-cors-allowed-credentials` | `SIG_PROVIDER__SERVER__HTTP__CORS__ALLOWED_CREDENTIALS` | `server.http.cors.allowed_credentials` |
| `interface.14.env-sig-provider-server-http-cors-allowed-methods` | `SIG_PROVIDER__SERVER__HTTP__CORS__ALLOWED_METHODS` | `server.http.cors.allowed_methods` |
| `interface.14.env-sig-provider-server-http-cors-allowed-origin` | `SIG_PROVIDER__SERVER__HTTP__CORS__ALLOWED_ORIGIN` | `server.http.cors.allowed_origin` |
| `interface.14.env-sig-provider-server-http-cors-block-on-origin-mismatch` | `SIG_PROVIDER__SERVER__HTTP__CORS__BLOCK_ON_ORIGIN_MISMATCH` | `server.http.cors.block_on_origin_mismatch` |
| `interface.14.env-sig-provider-server-http-cors-enabled` | `SIG_PROVIDER__SERVER__HTTP__CORS__ENABLED` | `server.http.cors.enabled` |
| `interface.14.env-sig-provider-server-http-cors-max-age` | `SIG_PROVIDER__SERVER__HTTP__CORS__MAX_AGE` | `server.http.cors.max_age` |
| `interface.14.env-sig-provider-server-http-enabled` | `SIG_PROVIDER__SERVER__HTTP__ENABLED` | `server.http.enabled` |
| `interface.14.env-sig-provider-server-http-max-body-size` | `SIG_PROVIDER__SERVER__HTTP__MAX_BODY_SIZE` | `server.http.max_body_size` |
| `interface.14.env-sig-provider-sources-eth-bytecode-db-enabled` | `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__ENABLED` | `sources.eth_bytecode_db.enabled` |
| `interface.14.env-sig-provider-sources-eth-bytecode-db-url` | `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__URL` | `sources.eth_bytecode_db.url` |
| `interface.14.env-sig-provider-sources-fourbyte` | `SIG_PROVIDER__SOURCES__FOURBYTE` | `sources.fourbyte` |
| `interface.14.env-sig-provider-sources-sigeth` | `SIG_PROVIDER__SOURCES__SIGETH` | `sources.sigeth` |
| `interface.14.env-sig-provider-tracing-enabled` | `SIG_PROVIDER__TRACING__ENABLED` | `tracing.enabled` |
| `interface.14.env-sig-provider-tracing-format` | `SIG_PROVIDER__TRACING__FORMAT` | `tracing.format` |
| — (additional implemented override) | `SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED` | `sources.precompiles.enabled` |

## env-verifier

These variables use the [0.17.0 loader][launcher017] and map to the supported keys in [cfg-verifier](#cfg-verifier). `SMART_CONTRACT_VERIFIER__CONFIG` selects the configuration file; all other rows are environment overrides. Source definitions, types and fetcher selection constraints are in that settings table.

| Interface | Environment variable | Maps to |
| --- | --- | --- |
| `interface.14.env-smart-contract-verifier-compilers-max-threads` | `SMART_CONTRACT_VERIFIER__COMPILERS__MAX_THREADS` | `compilers.max_threads` |
| `interface.14.env-smart-contract-verifier-config` | `SMART_CONTRACT_VERIFIER__CONFIG` | Configuration file path |
| `interface.14.env-smart-contract-verifier-jaeger-agent-endpoint` | `SMART_CONTRACT_VERIFIER__JAEGER__AGENT_ENDPOINT` | `jaeger.agent_endpoint` |
| `interface.14.env-smart-contract-verifier-jaeger-enabled` | `SMART_CONTRACT_VERIFIER__JAEGER__ENABLED` | `jaeger.enabled` |
| `interface.14.env-smart-contract-verifier-metrics-addr` | `SMART_CONTRACT_VERIFIER__METRICS__ADDR` | `metrics.addr` |
| `interface.14.env-smart-contract-verifier-metrics-enabled` | `SMART_CONTRACT_VERIFIER__METRICS__ENABLED` | `metrics.enabled` |
| `interface.14.env-smart-contract-verifier-metrics-route` | `SMART_CONTRACT_VERIFIER__METRICS__ROUTE` | `metrics.route` |
| `interface.14.env-smart-contract-verifier-server-grpc-addr` | `SMART_CONTRACT_VERIFIER__SERVER__GRPC__ADDR` | `server.grpc.addr` |
| `interface.14.env-smart-contract-verifier-server-grpc-enabled` | `SMART_CONTRACT_VERIFIER__SERVER__GRPC__ENABLED` | `server.grpc.enabled` |
| `interface.14.env-smart-contract-verifier-server-http-addr` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ADDR` | `server.http.addr` |
| `interface.14.env-smart-contract-verifier-server-http-cors-allowed-credentials` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__ALLOWED_CREDENTIALS` | `server.http.cors.allowed_credentials` |
| `interface.14.env-smart-contract-verifier-server-http-cors-allowed-methods` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__ALLOWED_METHODS` | `server.http.cors.allowed_methods` |
| `interface.14.env-smart-contract-verifier-server-http-cors-allowed-origin` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__ALLOWED_ORIGIN` | `server.http.cors.allowed_origin` |
| `interface.14.env-smart-contract-verifier-server-http-cors-block-on-origin-mismatch` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__BLOCK_ON_ORIGIN_MISMATCH` | `server.http.cors.block_on_origin_mismatch` |
| `interface.14.env-smart-contract-verifier-server-http-cors-enabled` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__ENABLED` | `server.http.cors.enabled` |
| `interface.14.env-smart-contract-verifier-server-http-cors-max-age` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__MAX_AGE` | `server.http.cors.max_age` |
| `interface.14.env-smart-contract-verifier-server-http-cors-send-wildcard` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__CORS__SEND_WILDCARD` | `server.http.cors.send_wildcard` |
| `interface.14.env-smart-contract-verifier-server-http-enabled` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ENABLED` | `server.http.enabled` |
| `interface.14.env-smart-contract-verifier-server-http-max-body-size` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__MAX_BODY_SIZE` | `server.http.max_body_size` |
| `interface.14.env-smart-contract-verifier-solidity-compilers-dir` | `SMART_CONTRACT_VERIFIER__SOLIDITY__COMPILERS_DIR` | `solidity.compilers_dir` |
| `interface.14.env-smart-contract-verifier-solidity-enabled` | `SMART_CONTRACT_VERIFIER__SOLIDITY__ENABLED` | `solidity.enabled` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-list-list-url` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__LIST__LIST_URL` | `solidity.fetcher.list.list_url` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-s3-access-key` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__S3__ACCESS_KEY` | `solidity.fetcher.s3.access_key` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-s3-bucket` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__S3__BUCKET` | `solidity.fetcher.s3.bucket` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-s3-endpoint` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__S3__ENDPOINT` | `solidity.fetcher.s3.endpoint` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-s3-region` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__S3__REGION` | `solidity.fetcher.s3.region` |
| `interface.14.env-smart-contract-verifier-solidity-fetcher-s3-secret-key` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__S3__SECRET_KEY` | `solidity.fetcher.s3.secret_key` |
| `interface.14.env-smart-contract-verifier-solidity-refresh-versions-schedule` | `SMART_CONTRACT_VERIFIER__SOLIDITY__REFRESH_VERSIONS_SCHEDULE` | `solidity.refresh_versions_schedule` |
| `interface.14.env-smart-contract-verifier-sourcify-api-url` | `SMART_CONTRACT_VERIFIER__SOURCIFY__API_URL` | `sourcify.api_url` |
| `interface.14.env-smart-contract-verifier-sourcify-enabled` | `SMART_CONTRACT_VERIFIER__SOURCIFY__ENABLED` | `sourcify.enabled` |
| `interface.14.env-smart-contract-verifier-sourcify-request-timeout` | `SMART_CONTRACT_VERIFIER__SOURCIFY__REQUEST_TIMEOUT` | `sourcify.request_timeout` |
| `interface.14.env-smart-contract-verifier-sourcify-verification-attempts` | `SMART_CONTRACT_VERIFIER__SOURCIFY__VERIFICATION_ATTEMPTS` | `sourcify.verification_attempts` |
| `interface.14.env-smart-contract-verifier-tracing-enabled` | `SMART_CONTRACT_VERIFIER__TRACING__ENABLED` | `tracing.enabled` |
| `interface.14.env-smart-contract-verifier-tracing-format` | `SMART_CONTRACT_VERIFIER__TRACING__FORMAT` | `tracing.format` |
| `interface.14.env-smart-contract-verifier-vyper-compilers-dir` | `SMART_CONTRACT_VERIFIER__VYPER__COMPILERS_DIR` | `vyper.compilers_dir` |
| `interface.14.env-smart-contract-verifier-vyper-enabled` | `SMART_CONTRACT_VERIFIER__VYPER__ENABLED` | `vyper.enabled` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-list-list-url` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__LIST__LIST_URL` | `vyper.fetcher.list.list_url` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-s3-access-key` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__ACCESS_KEY` | `vyper.fetcher.s3.access_key` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-s3-bucket` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__BUCKET` | `vyper.fetcher.s3.bucket` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-s3-endpoint` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__ENDPOINT` | `vyper.fetcher.s3.endpoint` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-s3-region` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__REGION` | `vyper.fetcher.s3.region` |
| `interface.14.env-smart-contract-verifier-vyper-fetcher-s3-secret-key` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__SECRET_KEY` | `vyper.fetcher.s3.secret_key` |
| `interface.14.env-smart-contract-verifier-vyper-refresh-versions-schedule` | `SMART_CONTRACT_VERIFIER__VYPER__REFRESH_VERSIONS_SCHEDULE` | `vyper.refresh_versions_schedule` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-enabled` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ENABLED` | `zksync_solidity.enabled` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-list-list-url` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__LIST__LIST_URL` | `zksync_solidity.era_evm_fetcher.list.list_url` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-s3-access-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__S3__ACCESS_KEY` | `zksync_solidity.era_evm_fetcher.s3.access_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-s3-bucket` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__S3__BUCKET` | `zksync_solidity.era_evm_fetcher.s3.bucket` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-s3-endpoint` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__S3__ENDPOINT` | `zksync_solidity.era_evm_fetcher.s3.endpoint` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-s3-region` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__S3__REGION` | `zksync_solidity.era_evm_fetcher.s3.region` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-era-evm-fetcher-s3-secret-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__S3__SECRET_KEY` | `zksync_solidity.era_evm_fetcher.s3.secret_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-compilers-dir` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_COMPILERS_DIR` | `zksync_solidity.evm_compilers_dir` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-list-list-url` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__LIST__LIST_URL` | `zksync_solidity.evm_fetcher.list.list_url` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-s3-access-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__S3__ACCESS_KEY` | `zksync_solidity.evm_fetcher.s3.access_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-s3-bucket` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__S3__BUCKET` | `zksync_solidity.evm_fetcher.s3.bucket` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-s3-endpoint` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__S3__ENDPOINT` | `zksync_solidity.evm_fetcher.s3.endpoint` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-s3-region` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__S3__REGION` | `zksync_solidity.evm_fetcher.s3.region` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-fetcher-s3-secret-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__S3__SECRET_KEY` | `zksync_solidity.evm_fetcher.s3.secret_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-evm-refresh-versions-schedule` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_REFRESH_VERSIONS_SCHEDULE` | `zksync_solidity.evm_refresh_versions_schedule` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-compilers-dir` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_COMPILERS_DIR` | `zksync_solidity.zk_compilers_dir` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-list-list-url` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__LIST__LIST_URL` | `zksync_solidity.zk_fetcher.list.list_url` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-s3-access-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__S3__ACCESS_KEY` | `zksync_solidity.zk_fetcher.s3.access_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-s3-bucket` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__S3__BUCKET` | `zksync_solidity.zk_fetcher.s3.bucket` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-s3-endpoint` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__S3__ENDPOINT` | `zksync_solidity.zk_fetcher.s3.endpoint` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-s3-region` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__S3__REGION` | `zksync_solidity.zk_fetcher.s3.region` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-fetcher-s3-secret-key` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__S3__SECRET_KEY` | `zksync_solidity.zk_fetcher.s3.secret_key` |
| `interface.14.env-smart-contract-verifier-zksync-solidity-zk-refresh-versions-schedule` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_REFRESH_VERSIONS_SCHEDULE` | `zksync_solidity.zk_refresh_versions_schedule` |

[sig-workspace]: ../../../services/explorer/services/sig-provider/Cargo.toml
[sig-manifest]: ../../../services/explorer/services/sig-provider/sig-provider-server/Cargo.toml
[sig-lock]: ../../../services/explorer/services/sig-provider/Cargo.lock
[sig-settings]: ../../../services/explorer/services/sig-provider/sig-provider-server/src/settings.rs
[sig-base]: ../../../services/explorer/services/sig-provider/sig-provider-server/config/base.toml
[sig-run]: ../../../services/explorer/services/sig-provider/sig-provider-server/src/run.rs
[verifier-workspace]: ../../../services/explorer/services/smart-contract-verifier/Cargo.toml
[verifier-manifest]: ../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/Cargo.toml
[verifier-lock]: ../../../services/explorer/services/smart-contract-verifier/Cargo.lock
[verifier-settings]: ../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/settings.rs
[verifier-base]: ../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/config/base.toml
[sourcify-client]: ../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/sourcify/api_client.rs
[local-launcher]: ../../../services/explorer/services/libs/blockscout-service-launcher/Cargo.toml
[test-database]: ../../../services/explorer/services/libs/blockscout-service-launcher/src/test_database.rs
[launcher010]: https://docs.rs/crate/blockscout-service-launcher/0.10.1/source/src/launcher/settings.rs
[tracing010]: https://docs.rs/crate/blockscout-service-launcher/0.10.1/source/src/tracing/settings.rs
[launcher017]: https://docs.rs/crate/blockscout-service-launcher/0.17.0/source/src/launcher/settings.rs
[tracing017]: https://docs.rs/crate/blockscout-service-launcher/0.17.0/source/src/tracing/settings.rs
