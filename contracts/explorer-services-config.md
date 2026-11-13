# Explorer Rust service configuration

The shared [blockscout-service-launcher](../services/explorer/services/libs/blockscout-service-launcher/src/lib.rs) configures explorer support applications, including signature lookup and source verification. These applications serve the explorer for one Paxeer BFT network; they are not additional execution or finality authorities. Mainnet-beta has EVM chain ID **125**. Sub-chains are database tenant domains, not one new service stack per domain.

See [HTTP contracts](explorer-http-api.md), [sockets](explorer-ws.md), and [metrics](explorer-metrics.md#launcher). This configuration is distinct from Elixir backend environment configuration and the [platform CLI profiles](platform-cli.md#cli-config-keys).

## Loading and precedence

[ConfigSettings::build](../services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs) reads an optional `<SERVICE>__CONFIG` file, removes that selector from the process environment, then overlays environment variables with the service prefix and `__` separator. It deserializes into that service's settings and calls `validate`. Invalid files, types, unknown fields in structs marked `deny_unknown_fields`, or service validation errors fail startup.

Use `SIG_PROVIDER` or `SMART_CONTRACT_VERIFIER` as `<SERVICE>`. For a nested key, uppercase each component and separate components with `__`, preserving underscores inside components. For example, `server.http.max_body_size` becomes `SIG_PROVIDER__SERVER__HTTP__MAX_BODY_SIZE`. Environment values override file values. Defaults apply to fields with Serde defaults; they do not make required fetcher fields optional.

## Shared server settings

The following fields are shared by both services. Addresses are IP socket addresses, not URLs. Sizes are bytes.

| Key | Type | Default |
| --- | --- | --- |
| `server.http.enabled` | bool | `true` |
| `server.http.addr` | SocketAddr | All IPv4 interfaces, port 8050 |
| `server.http.max_body_size` | usize | `2097152` |
| `server.http.base_path` | optional BasePath | absent |
| `server.grpc.enabled` | bool | `false` |
| `server.grpc.addr` | SocketAddr | All IPv4 interfaces, port 8051 |
| `metrics.enabled` | bool | `false` |
| `metrics.addr` | SocketAddr | All IPv4 interfaces, port 6060 |
| `metrics.route` | string | `/metrics` |
| `tracing.enabled` | bool | `true` |
| `tracing.format` | enum | `default` (also `json`) |
| `jaeger.enabled` | bool | `false` |
| `jaeger.agent_endpoint` | string | Loopback, UDP port 6831 |

`base_path` must begin with `/` and must not end with `/`; `/` itself is invalid. Leave it absent for an unprefixed application. [configure_router](../services/explorer/services/libs/blockscout-service-launcher/src/launcher/router.rs) scopes the service's HTTP routes under that prefix. It does not prefix the independent metrics listener or gRPC address. `max_body_size` sets Actix's JSON extractor limit.

### CORS

| Key under `server.http.cors` | Type | Default |
| --- | --- | --- |
| `enabled` | bool | `false` |
| `allowed_origin` | string | empty |
| `allowed_methods` | string | `PUT, GET, POST, OPTIONS, DELETE, PATCH` |
| `allowed_credentials` | bool | `true` |
| `max_age` | usize | `3600` seconds |
| `block_on_origin_mismatch` | bool | `false` |
| `send_wildcard` | bool | `false` |

Enabled CORS splits origins and methods on commas and trims whitespace. Origin `*` allows any origin. The launcher installs CORS middleware only when enabled. CORS is browser request policy, not user authentication; invalid Actix combinations can still fail startup.

## Cfg sig provider

[Signature-provider Settings](../services/explorer/services/sig-provider/sig-provider-server/src/settings.rs) uses all shared settings and these source fields:

| Key | Type | Default |
| --- | --- | --- |
| `sources.precompiles.enabled` | bool | `true` |
| `sources.fourbyte` | URL | Built-in 4byte source in `SourcesSettings::default` |
| `sources.sigeth` | URL | Built-in signature source in `SourcesSettings::default` |
| `sources.eth_bytecode_db.enabled` | bool | `true` |
| `sources.eth_bytecode_db.url` | URL | Built-in bytecode source in `EthBytecodeDbSettings::default` |

These upstreams resolve signatures/bytecode metadata. They are not Paxeer node RPC endpoints, nor authoritative proofs of contract behavior.

## Env sig provider

`SIG_PROVIDER__CONFIG` selects the configuration file. Every shared key maps with the prefix described above, including `SIG_PROVIDER__METRICS__ENABLED`, `SIG_PROVIDER__METRICS__ADDR`, and `SIG_PROVIDER__METRICS__ROUTE`.

Service-specific variables are `SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED`, `SIG_PROVIDER__SOURCES__FOURBYTE`, `SIG_PROVIDER__SOURCES__SIGETH`, `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__ENABLED`, and `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__URL`.

For example, this TOML uses only signature-provider settings declared in the source:

```toml
[server.http]
max_body_size = 2097152

[metrics]
enabled = true
route = "/metrics"

[tracing]
format = "json"
```

## Cfg verifier

[Verifier Settings](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/settings.rs) adds compiler and Sourcify sections to the same server, metrics, tracing, and Jaeger fields:

| Section/key | Type and behavior |
| --- | --- |
| `solidity.enabled`, `vyper.enabled`, `zksync_solidity.enabled` | bool, all default true |
| `solidity.compilers_dir`, `vyper.compilers_dir` | paths under the OS temporary directory by default (`solidity-compilers`, `vyper-compilers`) |
| `solidity.refresh_versions_schedule`, `vyper.refresh_versions_schedule` | cron schedule strings, default `0 0 * * * * *` |
| `solidity.fetcher`, `vyper.fetcher` | externally tagged `list` or `s3` fetcher; list defaults reference the library's compiler-list constants |
| `zksync_solidity.evm_compilers_dir`, `zksync_solidity.zk_compilers_dir` | temporary-directory defaults `zksync-solc-compilers`, `zksync-zksolc-compilers` |
| `zksync_solidity.evm_refresh_versions_schedule`, `zksync_solidity.zk_refresh_versions_schedule` | same hourly cron default |
| `zksync_solidity.evm_fetcher`, `era_evm_fetcher`, `zk_fetcher` | three fetchers nested under `zksync_solidity`; default list sources |
| `sourcify.enabled` | bool, default true |
| `sourcify.api_url` | URL, default `DEFAULT_SOURCIFY_HOST` from the verifier library |
| `sourcify.verification_attempts` | nonzero u32, default 3 |
| `sourcify.request_timeout` | u64, default 15 seconds |
| `compilers.max_threads` | nonzero usize, default available parallelism, or 8 if it cannot be determined |

For each fetcher, choose one variant: `fetcher.list.list_url` (required URL), or `fetcher.s3` with required `bucket` and optional `access_key`, `secret_key`, `region`, `endpoint` strings. The same variant shape applies to each zkSync fetcher. Do not configure both variants in one fetcher. The explicit service validator checks that a Solidity S3 fetcher has at least one of `region` or `endpoint`; that extra check is not applied to every other fetcher by this validator.

The zkSync compiler options are supported shared verifier features, not a statement that Paxeer settles to another chain. Compiler cache locations, download access, and source-verification availability are operational dependencies.

## Env verifier

`SMART_CONTRACT_VERIFIER__CONFIG` selects the file. Shared settings use this prefix, for example `SMART_CONTRACT_VERIFIER__SERVER__HTTP__BASE_PATH` and `SMART_CONTRACT_VERIFIER__JAEGER__AGENT_ENDPOINT`.

Compiler examples follow the actual nested types:

| Key | Environment variable |
| --- | --- |
| `compilers.max_threads` | `SMART_CONTRACT_VERIFIER__COMPILERS__MAX_THREADS` |
| `solidity.compilers_dir` | `SMART_CONTRACT_VERIFIER__SOLIDITY__COMPILERS_DIR` |
| `solidity.refresh_versions_schedule` | `SMART_CONTRACT_VERIFIER__SOLIDITY__REFRESH_VERSIONS_SCHEDULE` |
| `solidity.fetcher.list.list_url` | `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__LIST__LIST_URL` |
| `vyper.fetcher.s3.bucket` | `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__S3__BUCKET` |
| `zksync_solidity.era_evm_fetcher.list.list_url` | `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__LIST__LIST_URL` |
| `sourcify.verification_attempts` | `SMART_CONTRACT_VERIFIER__SOURCIFY__VERIFICATION_ATTEMPTS` |

Apply the same conversion for every field in the tables, including S3 credential keys. Credentials belong in private runtime configuration; they are not public API metadata.

## Env launcher

The launcher's `DATABASE_URL` lookup in [test_database.rs](../services/explorer/services/libs/blockscout-service-launcher/src/test_database.rs) is used by its test-database support. It is not an implicit database requirement of signature lookup or source verification. Database-backed services must define and consume their own database settings.

## Startup and shutdown

[launch](../services/explorer/services/libs/blockscout-service-launcher/src/launcher/launch.rs) starts only the enabled HTTP, gRPC, and metrics listeners. Enabling metrics both starts its listener and instruments application HTTP traffic. An HTTP base path affects only application routing. Tracing initialization can be disabled independently; Jaeger is opt-in and depends on tracing initialization.

Bind failures fail startup. When a launched future finishes, the launcher cancels sibling tasks and waits up to ten seconds for local shutdown before aborting remaining tasks. These settings do not add durable storage or replay; recovery depends on the owning application's persisted state.
