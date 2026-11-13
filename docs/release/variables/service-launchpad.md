# Launchpad service variables

The launchpad system in `services/launchpad` contains the API, indexer, PnL tracker, ranking, statistics, trading charts, media gateway, livestream, metadata, social and user services, plus discovery, notifications and security libraries. Its chain-facing services use the merged Paxeer network with EVM chain ID 125 and committed-block finality.

Environment names are case-sensitive. The five chain workers (indexer, PnL, ranking, statistics and charts) also parse `BaseEnv` from `sdk/launchpad/shared/config/env.go`. Reader links below identify the source files; paths in configuration names are relative to `services/launchpad`. Discovery and notifications receive their settings through their callers rather than a separate fixed environment schema.

`Required` lists environment inputs required by a component's loader. `Optional` includes defaults and conditional inputs: production startup also requires the runtime profile, bind interface, zone, instance, database allocation and expected-schema bindings, plus the selected component's runtime document, identity and broker credentials. A conditional input is required when that mode or feature is selected. Gateway database access is required in production. Account, media, livestream and social validators require their corresponding identity, TLS, operation-signing and storage inputs.

For a document with both `*_FILE` and `*_DOCUMENT`, provide exactly one: a regular JSON file or standard-base64 JSON bytes, together with the matching `*_SHA256` digest. Credential and PEM file/inline pairs are alternatives too. JSON rows name the selecting file variable, component, section and key using JSON path notation: `$` is the root, `[]` means each array element and `*` means each map entry. Go fields without a JSON tag retain their declared capitalization. Document fields have no deployment default unless stated; optional sections and private-deployment fields apply only when selected. Numeric Go durations in JSON are nanoseconds.

The root `.env.example` contains shared worker settings and Makefile inputs; each component's `.env.example` contains its complete fixed environment schema. Supply-chain inputs are in `security/.env.example`. Export the selected component's values into the process environment. Empty secret assignments carry no credentials; empty endpoint assignments preserve source defaults where available. Endpoint defaults are described by role below, with host values omitted. Names selected by runtime documents or by `MODERATION_BEARER_KEY` are supplied separately; their concrete names depend on the deployment.

## Required

| Name / configuration file, section and key | What it sets | Source default | Secret | Reader |
| --- | --- | --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string; gateway requires it in production; required for api, indexer, livestream, metadata, pnl-tracker, ranking-algo, social, stats-workers, trading-charts, user | api, indexer, livestream, metadata, pnl-tracker, ranking-algo, social, stats-workers, trading-charts, user: None; gateway: Unset | Yes | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `EVENT_EMITTER_ADDRESS` | Event emitter contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `FACTORY_ADDRESS` | Factory contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `FEES_ROUTER_ADDRESS` | Fees router contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `FEE_ACCUMULATOR_ADDRESS` | Fee accumulator contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `GATEWAY_JWT_SECRET` | Gateway JWT signing secret; required for gateway | None | Yes | [gateway/internal/config/config.go][source-6] |
| `METADATA_UPLOAD_URL` | Metadata upload endpoint; required for gateway | None | No | [gateway/internal/config/config.go][source-6] |
| `POOL_BEACON_ADDRESS` | Pool beacon contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `POOL_REGISTRY_ADDRESS` | Pool registry contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `PROTOCOL_CONFIG_ADDRESS` | Protocol config contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `QUOTER_ADDRESS` | Quoter contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `REDIS_BULL_URL` | Redis connection string for the shared worker queue; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | Yes | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `REDIS_URL` | Redis connection string for cache, rate limits and fan-out; required for api, gateway, indexer, livestream, metadata, pnl-tracker, ranking-algo, social, stats-workers, trading-charts, user | None | Yes | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `ROUTER_ADDRESS` | Router contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `RPC_URL` | Chain JSON-RPC endpoint at its root path; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | gateway, livestream, metadata: Unset; indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts: None | No | [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7] |
| `S3_ACCESS_KEY_ID` | Object storage access credential; required for gateway, metadata, user | gateway, metadata, user: None; trading-charts: Unset | Yes | [gateway/internal/config/config.go][source-6]; [metadata/internal/config/config.go][source-7]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `S3_BUCKET` | Component object storage bucket; required for metadata, user | metadata, user: None; trading-charts: Unset | No | [metadata/internal/config/config.go][source-7]; [trading-charts/cmd/charts-rebuild/main.go][source-16]; [user/internal/config/config.go][source-2] |
| `S3_ENDPOINT` | S3-compatible object storage endpoint; required for gateway, metadata, user | gateway, metadata, user: None; trading-charts: Unset | No | [gateway/internal/config/config.go][source-6]; [metadata/internal/config/config.go][source-7]; [trading-charts/cmd/charts-rebuild/main.go][source-16]; [user/internal/config/config.go][source-2] |
| `S3_SECRET_ACCESS_KEY` | Object storage signing secret; required for gateway, metadata, user | gateway, metadata, user: None; trading-charts: Unset | Yes | [gateway/internal/config/config.go][source-6]; [metadata/internal/config/config.go][source-7]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `SIDIORA_NFT_ADDRESS` | Sidiora nft contract identity in the reviewed chain deployment; required for indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts | None | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `SOCIAL_HTTP_URL` | Social http endpoint; required for gateway | None | No | [gateway/internal/config/config.go][source-6] |
| `SOCIAL_WS_URL` | Social ws endpoint; required for gateway | None | No | [gateway/internal/config/config.go][source-6] |
| `WEBHOOK_HMAC_SECRET` | Webhook authentication secret; required by charts, PnL and stats; indexer requires it or a per-target list when legacy webhooks are configured; required for pnl-tracker, stats-workers, trading-charts | indexer: Unset; pnl-tracker, stats-workers, trading-charts: None | Yes | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |

## Optional

| Name / configuration file, section and key | What it sets | Source default | Secret | Reader |
| --- | --- | --- | --- | --- |
| `--authority JSON (security/supplychain): $.approved_binary_sha256` | Approved application executable digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.approved_database_sha256` | Approved vulnerability database digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.approved_scanner_sha256` | Approved scanner digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.certificate_identity` | Allowed signing certificate identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.certificate_oidc_issuer` | Allowed signing issuer | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.tool.path` | Signing executable path | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.tool.sha256` | Signing executable digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.workflow_commit` | Signing commit identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.cosign.workflow_repository` | Signing repository identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.approved_exception_digests[]` | Approved vulnerability exception digests | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.architecture` | Target architecture | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.binary` | Executable identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.builder` | Build authority identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.commit` | Source commit identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.exception_approvers[]` | Permitted exception approval identities | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.image` | Container image digest identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.job_names.*` | Required workflow job names | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.materials.*` | Expected source material digests | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.ref` | Protected source reference | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.repository` | Repository identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.run_attempt` | Release workflow run attempt | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.run_id` | Release workflow run identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.sbom` | Expected SBOM digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.service` | Service identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.source` | Source identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.workflow` | Expected workflow digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.expected.workflow_path` | Release workflow path | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.max_scan_age_seconds` | Maximum scanner result age in seconds | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.oras.path` | Registry client executable path | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.oras.sha256` | Registry client executable digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.certificate_identity` | Allowed promotion certificate identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.certificate_oidc_issuer` | Allowed promotion signing issuer | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.tool.path` | Promotion signing executable path | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.tool.sha256` | Promotion signing executable digest | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.workflow_commit` | Promotion commit identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.promotion_cosign.workflow_repository` | Promotion repository identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.protected_refs[]` | Protected release references | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.registry.host` | Registry host selection | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.registry.username` | Registry identity | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.response_authority_path` | Vulnerability response authority file | None | No | [security/supplychain/scanner.py][source-60] |
| `--authority JSON (security/supplychain): $.response_authority_sha256` | Vulnerability response authority digest | None | No | [security/supplychain/scanner.py][source-60] |
| `--authority JSON (security/supplychain): $.schema_version` | Authority format version | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.spdx_schema_path` | SPDX schema file | None | No | [security/supplychain/controller.py][source-13] |
| `--authority JSON (security/supplychain): $.spdx_schema_sha256` | SPDX schema digest | None | No | [security/supplychain/controller.py][source-13] |
| `--manifest JSON (security/supplychain): $.schema_version` | Executable manifest version | None | No | [security/supplychain/ci_tools.py][source-15] |
| `--manifest JSON (security/supplychain): $.tools.*.path` | Reviewed executable path | None | No | [security/supplychain/ci_tools.py][source-15] |
| `--manifest JSON (security/supplychain): $.tools.*.sha256` | Reviewed executable digest | None | No | [security/supplychain/ci_tools.py][source-15] |
| `--manifests JSON (trading-charts): $[].blockCheckpoints[].FetchComplete` | Chart archive reconstruction: fetch complete | `false` | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].blockCheckpoints[].Hash` | Chart archive reconstruction: hash | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].blockCheckpoints[].Height` | Chart archive reconstruction: height | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].blockCheckpoints[].RawCount` | Chart archive reconstruction: raw count | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].blockCheckpoints[].RequiredDecodeComplete` | Chart archive reconstruction: required decode complete | `false` | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].bucket` | Chart archive reconstruction: bucket | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].byteLength` | Chart archive reconstruction: byte length | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].chainId` | Chart archive reconstruction: chain id | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].eventCount` | Chart archive reconstruction: event count | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].formatVersion` | Chart archive reconstruction: format version | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].fromHeight` | Chart archive reconstruction: from height | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].objectKey` | Chart archive reconstruction: object key | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].sha256` | Chart archive reconstruction: sha256 | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].sourceStream` | Chart archive reconstruction: source stream | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].toHeight` | Chart archive reconstruction: to height | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `--manifests JSON (trading-charts): $[].typedCount` | Chart archive reconstruction: typed count | None | No | [trading-charts/internal/store/rebuild.go][source-54] |
| `ACCOUNTING_READ_CONFIG_DOCUMENT` | Base64-encoded accounting read config JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1] |
| `ACCOUNTING_READ_CONFIG_FILE` | Path to the accounting read config JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.ChartsConsumerID` | Accounting read model: charts consumer id | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.ChartsDeliverySchema` | Accounting read model: charts delivery schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.ChartsSchema` | Accounting read model: charts schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.ProjectionGeneration` | Accounting read model: projection generation | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.ProjectionVersion` | Accounting read model: projection version | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.SourcePartitionSchema` | Accounting read model: source partition schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.SourceStream` | Accounting read model: source stream | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.StatsConsumerID` | Accounting read model: stats consumer id | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.StatsDeliverySchema` | Accounting read model: stats delivery schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.StatsSchema` | Accounting read model: stats schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_FILE (api): $.StreamSchema` | Accounting read model: stream schema | None | No | [api/internal/store/accounting_readmodel.go][source-17] |
| `ACCOUNTING_READ_CONFIG_SHA256` | SHA-256 digest binding for accounting read config | Unset | No | [api/internal/config/config.go][source-1] |
| `ACCOUNT_ALLOW_METADATA_AVATARS` | Allows metadata-backed avatars | `false` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ASSERTION_AUDIENCE` | Account assertion audience setting | `account` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ASSERTION_HEADER` | Account assertion header setting | `X-Kindle-Service-Assertion` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ASSERTION_ISSUER` | Account assertion issuer setting | `gateway` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ASSERTION_SIGNATURE_HEADER` | Account assertion signature header setting | `X-Kindle-Service-Signature` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_AUDIT_KEY` | Account audit key used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_CHAIN_RPC_URL` | Account chain rpc endpoint | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_CURSOR_KEY` | Account cursor key used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_DELIVERY_SCHEMA` | Account delivery database schema | `users` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_DOWNLOAD_BASE_URL` | Account download base endpoint | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ENABLE_EXTENDED_V3` | Enables extended account operations | `false` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ENDPOINT_FINGERPRINT_KEY` | Account endpoint fingerprint key used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ENDPOINT_KEY` | Account endpoint key used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_ENDPOINT_PROOF_PUBLIC_KEY` | Account endpoint proof public key used to sign, encrypt or authenticate component data | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_EXPORT_BUCKET` | Private export bucket, distinct from the public asset bucket | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_EXPORT_KEY` | Account export key used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_GATEWAY_CERT_SHA256` | SHA-256 digest binding for account gateway cert | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_GATEWAY_PUBLIC_KEY` | Account gateway public key used to sign, encrypt or authenticate component data | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CA_FILE` | Path to account nats ca material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CA_PEM` | Inline account nats ca PEM material; alternative to the corresponding file | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CERT_FILE` | Path to account nats cert material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CERT_PEM` | Inline account nats cert PEM material; alternative to the corresponding file | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CREDENTIALS_FILE` | Account broker credential file; alternative to ACCOUNT_NATS_CREDS | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_CREDS` | Inline account broker credentials; alternative to ACCOUNT_NATS_CREDENTIALS_FILE | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_KEY_FILE` | Path to account nats key material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_KEY_PEM` | Inline account nats key PEM material; alternative to the corresponding file | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_STREAM` | Account nats stream setting | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_NATS_URL` | Account nats endpoint | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_OPERATION_ENVIRONMENT` | Environment bound into signed account operations | `production` | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_SERVICE_SIGNING_SEED` | Account service signing seed used to sign, encrypt or authenticate component data | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_SUBJECTS_DOCUMENT` | Base64-encoded account subjects JSON; alternative to its file input | Unset | May contain secrets | [user/internal/config/config.go][source-2] |
| `ACCOUNT_SUBJECTS_FILE` | Path to the account subjects JSON document; alternative to its inline document | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_SUBJECTS_FILE (user): $.*` | Account event subjects: subject | None | No | [user/internal/app/account_runtime.go][source-18] |
| `ACCOUNT_SUBJECTS_SHA256` | SHA-256 digest binding for account subjects | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_CA_FILE` | Path to account tls ca material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_CA_PEM` | Inline account tls ca PEM material; alternative to the corresponding file | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_CERT_FILE` | Path to account tls cert material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_CERT_PEM` | Inline account tls cert PEM material; alternative to the corresponding file | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_KEY_FILE` | Path to account tls key material | Unset | No | [user/internal/config/config.go][source-2] |
| `ACCOUNT_TLS_KEY_PEM` | Inline account tls key PEM material; alternative to the corresponding file | Unset | Yes | [user/internal/config/config.go][source-2] |
| `ACCOUNT_USE_READ_SWITCH` | Enables the account read-model switch | `false` | No | [user/internal/config/config.go][source-2] |
| `ACTION_AUDIT_KEY` | Action audit key used to sign, encrypt or authenticate component data | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `ACTION_ENVIRONMENT` | Environment bound into signed livestream operations | Unset | No | [livestream/internal/config/config.go][source-3] |
| `ADMIN_API_KEY` | Enables and authenticates privileged API routes when set | Unset | Yes | [api/internal/config/config.go][source-1] |
| `API_ADMISSION_CAPACITY` | Api admission capacity setting | `1000` | No | [api/internal/config/config.go][source-1] |
| `API_ADMISSION_MAX_WAITING` | Api admission max waiting setting | `1000` | No | [api/internal/config/config.go][source-1] |
| `API_MAX_INFLIGHT_REQUESTS` | Api max inflight requests setting | `10000` | No | [api/internal/config/config.go][source-1] |
| `API_RATE_LIMIT_MAX` | Api rate limit max setting | `100` | No | [api/internal/config/config.go][source-1] |
| `API_RATE_LIMIT_WINDOW_SEC` | Api rate limit window sec setting | `60` | No | [api/internal/config/config.go][source-1] |
| `API_ROLE` | Production process role: rest or sockets | Unset | No | [api/internal/config/config.go][source-1] |
| `BACKFILL_BATCH_SIZE` | Backfill batch size setting | `64` | No | [indexer/internal/config/config.go][source-4] |
| `BACKFILL_CONCURRENCY` | Backfill concurrency setting | `20` | No | [indexer/internal/config/config.go][source-4] |
| `BACKFILL_FROM_BLOCK` | Starting height for backfill mode | Unset | No | [indexer/internal/config/config.go][source-4] |
| `BACKFILL_ONLY` | Exits after the requested backfill completes | `false` | No | [indexer/internal/config/config.go][source-4] |
| `BASELINE_TOKEN` | Asset selected for a PnL baseline action | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `BASELINE_WALLET` | Wallet selected for a PnL baseline action | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `BIND_ADDRESS` | Listener bind interface; required for the selected production profile | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `CANONICAL_SCHEMA` | Canonical database schema | `indexer` | No | [metadata/internal/config/config.go][source-7] |
| `CANONICAL_STREAM_ID` | Canonical event stream identity | Unset | No | [metadata/internal/config/config.go][source-7] |
| `CHAIN_ID` | EVM chain identity; the merged network uses 125 | `125` | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `CHART_FEATURES_CONFIG_DOCUMENT` | Base64-encoded chart features config JSON; alternative to its file input | Unset | May contain secrets | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE` | Path to the chart features config JSON document; alternative to its inline document | Unset | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.anchorHash` | Chart features: anchor hash | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.anchorHeight` | Chart features: anchor height | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.archive.bucket` | Chart features: bucket | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.archive.endpoint` | Chart features: endpoint | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.archive.region` | Chart features: region | `auto` | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.canonicalApiOrigin` | Chart features: canonical api origin | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.canonicalSeriesModel` | Chart features: canonical series model | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.deploymentVersion` | Chart features: deployment version | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.displayPrecision` | Chart features: display precision | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.endpointSetVersion` | Chart features: endpoint set version | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.maxHistoryOrigins` | Chart features: max history origins | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.replayGatewayOrigin` | Chart features: replay gateway origin | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.rollupBatch` | Chart features: rollup batch | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.rollupIntervalMillis` | Chart features: rollup interval millis | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_FILE (trading-charts): $.schemaVersion` | Chart features: schema version | None | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_FEATURES_CONFIG_SHA256` | SHA-256 digest binding for chart features config | Unset | No | [trading-charts/internal/config/config.go][source-11] |
| `CHART_OPERATOR_TOKEN` | Chart operator token setting | Unset | Yes | [trading-charts/internal/config/config.go][source-11] |
| `CHAT_ADMIN_API_KEY` | Authenticates administrative chat operations | Unset | Yes | [social/internal/config/config.go][source-9] |
| `CLAMAV_HOST` | Malware scanner host selected by the operator | Unset | No | [metadata/internal/config/config.go][source-7] |
| `CLAMAV_PORT` | Malware scanner port | `3310` | No | [metadata/internal/config/config.go][source-7] |
| `CLIENT_SEND_BUFFER` | Client send buffer setting | `256` | No | [api/internal/config/config.go][source-1] |
| `COALESCE_FLUSH_MS` | Coalesce flush ms setting | `100` | No | [api/internal/config/config.go][source-1] |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allowed browser origins; production validation is component-specific | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `DB_BUDGET_DOCUMENT` | Base64-encoded db budget JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `DB_BUDGET_FILE` | Path to the db budget JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `DB_BUDGET_FILE (all): $.clients[].alias` | Database connection allocation: alias | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].maxConnections` | Database connection allocation: max connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].poolId` | Database connection allocation: pool id | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].pooler` | Database connection allocation: pooler | Unset | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].processes.maximum` | Database connection allocation: maximum | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].processes.replicas` | Database connection allocation: replicas | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].processes.surge` | Database connection allocation: surge | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].processes.terminating` | Database connection allocation: terminating | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].queryClass` | Database connection allocation: query class | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].role` | Database connection allocation: role | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].target` | Database connection allocation: target | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].transport` | Database connection allocation: transport | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.clients[].user` | Database connection allocation: user | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.identity` | Database connection allocation: identity | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.operational[].category` | Database connection allocation: category | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.operational[].connections` | Database connection allocation: connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.operational[].id` | Database connection allocation: id | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.operational[].target` | Database connection allocation: target | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].aliases[].alias` | Database connection allocation: alias | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].aliases[].maxConnections` | Database connection allocation: max connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].configurationSha256` | Database connection allocation: configuration sha256 | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].host` | Database connection allocation: host | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].id` | Database connection allocation: id | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].imageDigest` | Database connection allocation: image digest | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].maxClientConnections` | Database connection allocation: max client connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].mode` | Database connection allocation: mode | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].pairs[].alias` | Database connection allocation: alias | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].pairs[].poolSize` | Database connection allocation: pool size | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].pairs[].reservePoolSize` | Database connection allocation: reserve pool size | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].pairs[].user` | Database connection allocation: user | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].port` | Database connection allocation: port | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].processes.maximum` | Database connection allocation: maximum | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].processes.replicas` | Database connection allocation: replicas | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].processes.surge` | Database connection allocation: surge | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].processes.terminating` | Database connection allocation: terminating | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].target` | Database connection allocation: target | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].users[].maxConnections` | Database connection allocation: max connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].users[].user` | Database connection allocation: user | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.poolers[].version` | Database connection allocation: version | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].explicitUnusedHeadroom` | Database connection allocation: explicit unused headroom | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].host` | Database connection allocation: host | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].id` | Database connection allocation: id | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].maxConnections` | Database connection allocation: max connections | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].member` | Database connection allocation: member | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].port` | Database connection allocation: port | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_FILE (all): $.targets[].protectedSlots` | Database connection allocation: protected slots | None | No | [sdk/launchpad/shared/db/production/allocation.go][source-19] |
| `DB_BUDGET_SHA256` | SHA-256 digest binding for db budget | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `DB_MAX_CONNS` | Db max conns setting | `20` | No | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9] |
| `DB_POOL_ID` | Allocated database pool identity; required in production | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `DELIVERY_CONFIG_DOCUMENT` | Base64-encoded delivery config JSON; alternative to its file input | Unset | May contain secrets | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `DELIVERY_CONFIG_FILE` | Path to the delivery config JSON document; alternative to its inline document | Unset | No | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `DELIVERY_CONFIG_FILE (indexer): $.canonicalStreamId` | Indexer event delivery: canonical stream id | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.assertionHeader` | Indexer event delivery: assertion header | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.audience` | Indexer event delivery: audience | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.issuer` | Indexer event delivery: issuer | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.peerCertSHA256` | Indexer event delivery: peer cert sha256 | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.policyVersion` | Indexer event delivery: policy version | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.publicKey` | Indexer event delivery: public key (base64 bytes) | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.securitySchema` | Indexer event delivery: security schema | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deadLetters.signatureHeader` | Indexer event delivery: signature header | None | No | [indexer/internal/app/render.go][source-21] |
| `DELIVERY_CONFIG_FILE (indexer): $.deploymentId` | Indexer event delivery: deployment id | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.epoch` | Indexer event delivery: epoch | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.generation` | Indexer event delivery: generation | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.inventoryProjectionVersion` | Indexer event delivery: inventory projection version | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.maxDeliveryLagMs` | Indexer event delivery: max delivery lag ms | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.owner` | Indexer event delivery: owner | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.policy.EventTypes.*` | Indexer event delivery: event types | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (indexer): $.policy.Subjects.*` | Indexer event delivery: subjects | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (indexer): $.projectionVersion` | Indexer event delivery: projection version | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.region` | Indexer event delivery: region | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.schema` | Indexer event delivery: schema | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.stream` | Indexer event delivery: stream | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (indexer): $.subjects.*` | Indexer event delivery: subjects | None | No | [indexer/internal/app/app.go][source-20] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.AckWait` | Inventory event delivery: ack wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.Batch` | Inventory event delivery: batch | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.Durable` | Inventory event delivery: durable | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.FetchWait` | Inventory event delivery: fetch wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.MaxAckPending` | Inventory event delivery: max ack pending | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.MaxDeliver` | Inventory event delivery: max deliver | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.MaxWaiting` | Inventory event delivery: max waiting | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.Stream` | Inventory event delivery: stream | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumer.Subject` | Inventory event delivery: subject | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.consumerId` | Inventory event delivery: consumer id | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.epoch` | Inventory event delivery: epoch | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.generation` | Inventory event delivery: generation | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.owner` | Inventory event delivery: owner | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.projectionVersion` | Inventory event delivery: projection version | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.region` | Inventory event delivery: region | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.schema` | Inventory event delivery: schema | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (pnl-tracker): $.sourceStream` | Inventory event delivery: source stream | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumerId` | Statistics event delivery: consumer id | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].AckWait` | Statistics event delivery: ack wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].Batch` | Statistics event delivery: batch | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].Durable` | Statistics event delivery: durable | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].FetchWait` | Statistics event delivery: fetch wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].MaxAckPending` | Statistics event delivery: max ack pending | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].MaxDeliver` | Statistics event delivery: max deliver | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].MaxWaiting` | Statistics event delivery: max waiting | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].Stream` | Statistics event delivery: stream | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.consumers[].Subject` | Statistics event delivery: subject | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.epoch` | Statistics event delivery: epoch | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.generation` | Statistics event delivery: generation | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.maxReplayBlocks` | Statistics event delivery: max replay blocks | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.maxReplayFills` | Statistics event delivery: max replay fills | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.owner` | Statistics event delivery: owner | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.analyticsBatch` | Statistics event delivery: analytics batch | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.analyticsExpirySeconds` | Statistics event delivery: analytics expiry seconds | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.cursorTtlSeconds` | Statistics event delivery: cursor ttl seconds | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.expiryIntervalSeconds` | Statistics event delivery: expiry interval seconds | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.holderIntervalMs` | Statistics event delivery: holder interval ms | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.holderSchema` | Statistics event delivery: holder schema | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.holderTokenBatch` | Statistics event delivery: holder token batch | None | Yes | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.holderTransferBatch` | Statistics event delivery: holder transfer batch | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.outputStream` | Statistics event delivery: output stream | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.publishBatch` | Statistics event delivery: publish batch | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.publishMaxAttempts` | Statistics event delivery: publish max attempts | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.publishTimeoutSeconds` | Statistics event delivery: publish timeout seconds | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.reactionIpPerMinute` | Statistics event delivery: reaction ip per minute | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.reactionNamespace` | Statistics event delivery: reaction namespace | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.reactionPolicyVersion` | Statistics event delivery: reaction policy version | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.reactionWalletPerMinute` | Statistics event delivery: reaction wallet per minute | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumerId` | Statistics event delivery: search consumer id | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.AckWait` | Statistics event delivery: ack wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.Batch` | Statistics event delivery: batch | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.Durable` | Statistics event delivery: durable | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.FetchWait` | Statistics event delivery: fetch wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.MaxAckPending` | Statistics event delivery: max ack pending | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.MaxDeliver` | Statistics event delivery: max deliver | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.MaxWaiting` | Statistics event delivery: max waiting | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.Stream` | Statistics event delivery: stream | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].consumer.Subject` | Statistics event delivery: subject | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].eventType` | Statistics event delivery: event type | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchConsumers[].kind` | Statistics event delivery: kind | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchGeneration` | Statistics event delivery: search generation | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchInvalidationEventType` | Statistics event delivery: search invalidation event type | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchInvalidationSubject` | Statistics event delivery: search invalidation subject | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchMaxFills` | Statistics event delivery: search max fills | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchSources.*.Schema` | Statistics event delivery: schema | None | No | [stats-workers/internal/search/projection.go][source-26] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.searchSources.*.View` | Statistics event delivery: view | None | No | [stats-workers/internal/search/projection.go][source-26] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.statsSubject` | Statistics event delivery: stats subject | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.phase2.windows[]` | Statistics event delivery: windows | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.policy.EventTypes.*` | Statistics event delivery: event types | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.policy.Subjects.*` | Statistics event delivery: subjects | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.projectionVersion` | Statistics event delivery: projection version | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.region` | Statistics event delivery: region | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.schema` | Statistics event delivery: schema | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (stats-workers): $.sourceStream` | Statistics event delivery: source stream | None | No | [stats-workers/internal/app/app.go][source-25] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.chartOutbox.batch` | Chart event delivery: batch | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.chartOutbox.intervalMillis` | Chart event delivery: interval millis | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.chartOutbox.stream` | Chart event delivery: stream | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.chartOutbox.subject` | Chart event delivery: subject | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.AckWait` | Chart event delivery: ack wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.Batch` | Chart event delivery: batch | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.Durable` | Chart event delivery: durable | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.FetchWait` | Chart event delivery: fetch wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.MaxAckPending` | Chart event delivery: max ack pending | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.MaxDeliver` | Chart event delivery: max deliver | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.MaxWaiting` | Chart event delivery: max waiting | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.Stream` | Chart event delivery: stream | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumer.Subject` | Chart event delivery: subject | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.consumerId` | Chart event delivery: consumer id | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.epoch` | Chart event delivery: epoch | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.generation` | Chart event delivery: generation | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.maxReplayBlocks` | Chart event delivery: max replay blocks | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.maxReplayFills` | Chart event delivery: max replay fills | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.owner` | Chart event delivery: owner | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.policy.EventTypes.*` | Chart event delivery: event types | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.policy.Subjects.*` | Chart event delivery: subjects | None | No | [sdk/launchpad/shared/delivery/envelope.go][source-22] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.projectionVersion` | Chart event delivery: projection version | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.region` | Chart event delivery: region | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.schema` | Chart event delivery: schema | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_FILE (trading-charts): $.sourceStream` | Chart event delivery: source stream | None | No | [trading-charts/internal/app/app.go][source-27] |
| `DELIVERY_CONFIG_SHA256` | SHA-256 digest binding for delivery config | Unset | No | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `DEPLOYMENT_REALM` | Deployment validation realm; private_fixture requires its manifest | indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts: `production`; livestream: Unset | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3] |
| `DISCOVERY_RUNTIME_CONFIG_DOCUMENT` | Base64-encoded discovery runtime config JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1] |
| `DISCOVERY_RUNTIME_CONFIG_FILE` | Path to the discovery runtime config JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.chatPeerDNS` | Discovery and notifications: chat peer dns | None | No | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.cursorKeyRef` | Discovery and notifications: cursor key ref; names an environment secret, whose value is supplied separately | None | No (name only) | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.d11Base` | Discovery and notifications: d11 base | None | No | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.d11TokenRef` | Discovery and notifications: d11 token ref; names an environment secret, whose value is supplied separately | None | No (name only) | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.d12Base` | Discovery and notifications: d12 base | None | No | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.d12TokenRef` | Discovery and notifications: d12 token ref; names an environment secret, whose value is supplied separately | None | No (name only) | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.deletePeerDNS` | Discovery and notifications: delete peer dns | None | No | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.internalTokenRef` | Discovery and notifications: internal token ref; names an environment secret, whose value is supplied separately | None | No (name only) | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.APNSKey` | Discovery and notifications: apns key; names an environment secret, whose value is supplied separately | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.APNSKeyID` | Discovery and notifications: apns key id; names an environment secret, whose value is supplied separately | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.APNSSandbox` | Discovery and notifications: apns sandbox | `false` | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.APNSTeamID` | Discovery and notifications: apns team id; names an environment secret, whose value is supplied separately | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.APNSTopic` | Discovery and notifications: apns topic; names an environment secret, whose value is supplied separately | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.DeepLinkOrigin` | Discovery and notifications: deep link origin | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.EnableAPNS` | Discovery and notifications: enable apns | `false` | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.EnableFCM` | Discovery and notifications: enable fcm | `false` | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.EnableWebPush` | Discovery and notifications: enable web push | `false` | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.FCMServiceAccount` | Discovery and notifications: fcm service account; names an environment secret, whose value is supplied separately | None | Yes | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.Owner` | Discovery and notifications: owner | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.Secrets.CacheTTL` | Discovery and notifications: cache ttl in nanoseconds | `0` (credential caching disabled) | No | [sdk/launchpad/shared/secrets/manager.go][source-29] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.Secrets.Provider` | Discovery and notifications: provider | `SECRETS_PROVIDER`, then `env` | No | [sdk/launchpad/shared/secrets/manager.go][source-29] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.VAPIDKey` | Discovery and notifications: vapid key; names an environment secret, whose value is supplied separately | None | Yes | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.push.VAPIDSubject` | Discovery and notifications: vapid subject; names an environment secret, whose value is supplied separately | None | No | [notifications/runtime.go][source-30] |
| `DISCOVERY_RUNTIME_CONFIG_FILE (api): $.quoteTokenAddress` | Discovery and notifications: quote token address | None | Yes | [api/internal/app/app.go][source-28] |
| `DISCOVERY_RUNTIME_CONFIG_SHA256` | SHA-256 digest binding for discovery runtime config | Unset | No | [api/internal/config/config.go][source-1] |
| `EXPECTED_SCHEMA_DOCUMENT` | Base64-encoded expected schema JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `EXPECTED_SCHEMA_FILE` | Path to the expected schema JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `EXPECTED_SCHEMA_FILE (all): $.epoch` | Database readiness: epoch | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.interface_digest` | Database readiness: interface digest | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.migrations.*` | Database readiness: migrations | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].argument_types[]` | Database readiness: argument types | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].columns[]` | Database readiness: columns | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].grant_option` | Database readiness: grant option | `false` | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].object` | Database readiness: object | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].object_kind` | Database readiness: object kind | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].privileges[]` | Database readiness: privileges | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].schema` | Database readiness: schema | None | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.privileges[].view` | Database readiness: view | `false` | No | [sdk/launchpad/shared/db/production/roles.go][source-32] |
| `EXPECTED_SCHEMA_FILE (all): $.reader_version` | Database readiness: reader version | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.region` | Database readiness: region | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.registry_digest` | Database readiness: registry digest | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.runtime_role` | Database readiness: runtime role | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.schemas.*` | Database readiness: schemas | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_FILE (all): $.service` | Database readiness: service | None | No | [sdk/launchpad/shared/db/production/migrations.go][source-31] |
| `EXPECTED_SCHEMA_SHA256` | SHA-256 digest binding for expected schema | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `FIXTURE_COMMIT_ACK_BARRIER` | Private deployment commit acknowledgement barrier | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `FIXTURE_PUBACK_BARRIER` | Private deployment publication acknowledgement barrier | Unset | No | [indexer/internal/config/config.go][source-4] |
| `GATEWAY_APP_DOMAIN` | Domain identifier checked during wallet authentication | `kindlelaunch` | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_ASSERTION_PUBLIC_KEY_HEX` | Gateway assertion public key hex setting | Unset | No | [metadata/internal/config/config.go][source-7] |
| `GATEWAY_JWT_TTL_SECONDS` | Gateway JWT lifetime in seconds | `86400` | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_NONCE_TTL_SECONDS` | Authentication challenge lifetime in seconds | `300` | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_PEER_CERT_SHA256` | SHA-256 digest binding for gateway peer cert | Unset | No | [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9] |
| `GATEWAY_RUNTIME_DOCUMENT` | Base64-encoded gateway runtime JSON; alternative to its file input | Unset | May contain secrets | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE` | Path to the gateway runtime JSON document; alternative to its inline document | Unset | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.approvedBucket` | Gateway runtime: approved bucket | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.assertionKeyReference` | Gateway runtime: assertion key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.deliverySchema` | Gateway runtime: delivery schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.durable` | Gateway runtime: durable | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.eventType` | Gateway runtime: event type | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.keyReference` | Gateway runtime: key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.sinkDeliverySchema` | Gateway runtime: sink delivery schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.sinkSchema` | Gateway runtime: sink schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.stream` | Gateway runtime: stream | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.audit.subject` | Gateway runtime: subject | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.caFile` | Gateway runtime: ca file | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.caSHA256` | Gateway runtime: casha256 | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.clientCertFile` | Gateway runtime: client cert file | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.clientCertSHA256` | Gateway runtime: client cert sha256 | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.clientKeyReference` | Gateway runtime: client key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.consumerGeneration` | Gateway runtime: consumer generation | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.control.authoritySchema` | Gateway runtime: authority schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.control.incidentCredentialReference` | Gateway runtime: incident credential reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.control.revocationEndpoint` | Gateway runtime: revocation endpoint | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.control.revocationTokenReference` | Gateway runtime: revocation token reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.control.schema` | Gateway runtime: schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.durable` | Gateway runtime: durable | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityEndpoint` | Gateway runtime: identity endpoint | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityListenAddress` | Gateway runtime: identity listen address | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityPeers[].issuer` | Gateway runtime: issuer | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityPeers[].peerCertSHA256` | Gateway runtime: peer cert sha256 | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityPeers[].publicKey` | Gateway runtime: public key | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.identityServerPeerCertSHA256` | Gateway runtime: identity server peer cert sha256 | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.mediaSchema` | Gateway runtime: media schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.metadataSchema` | Gateway runtime: metadata schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.natsCredentialsReference` | Gateway runtime: nats credentials reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.natsURL` | Gateway runtime: natsurl | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].bucket` | Gateway runtime: bucket | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].deleteAccessKeyReference` | Gateway runtime: delete access key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].deleteSecretKeyReference` | Gateway runtime: delete secret key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].inventoryAccessKeyReference` | Gateway runtime: inventory access key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].inventoryLimit` | Gateway runtime: inventory limit | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].inventorySecretKeyReference` | Gateway runtime: inventory secret key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].kind` | Gateway runtime: kind | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.objectWorkers[].prefix` | Gateway runtime: prefix | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.pools.controlCredentialReference` | Gateway runtime: control credential reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.pools.controlMaxConnections` | Gateway runtime: control max connections | None | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.pools.controlPoolId` | Gateway runtime: control pool id | None | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.pools.identityMaxConnections` | Gateway runtime: identity max connections | None | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.pools.identityPoolId` | Gateway runtime: identity pool id | None | No | [gateway/internal/config/config.go][source-6] |
| `GATEWAY_RUNTIME_FILE (gateway): $.purgeEndpoint` | Gateway runtime: purge endpoint | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.purgeMode` | Gateway media purge transport | `http` | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.purgeTokenReference` | Gateway runtime: purge token reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.recoveryEpoch` | Gateway runtime: recovery epoch | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.recoveryRegion` | Gateway runtime: recovery region | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.recoverySchema` | Gateway runtime: recovery schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.safetySchema` | Gateway runtime: safety schema | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.serverCertFile` | Gateway runtime: server cert file | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.serverCertSHA256` | Gateway runtime: server cert sha256 | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.serverKeyReference` | Gateway runtime: server key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.stream` | Gateway runtime: stream | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.trustedProxies[]` | Gateway runtime: trusted proxies | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.uploadBytesPerSecond` | Gateway runtime: upload bytes per second | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.uploadConcurrent` | Gateway runtime: upload concurrent | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.uploadNamespace` | Gateway runtime: upload namespace | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].name` | Gateway runtime: name | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].routes[].Authenticate` | Gateway runtime: authenticate | `false` | No | [gateway/internal/proxy/connected.go][source-34] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].routes[].Method` | Gateway runtime: method | None | No | [gateway/internal/proxy/connected.go][source-34] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].routes[].Optional` | Gateway runtime: optional | `false` | No | [gateway/internal/proxy/connected.go][source-34] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].routes[].Template` | Gateway runtime: template | None | No | [gateway/internal/proxy/connected.go][source-34] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].routes[].Upload` | Gateway runtime: upload | `false` | No | [gateway/internal/proxy/connected.go][source-34] |
| `GATEWAY_RUNTIME_FILE (gateway): $.upstreams[].url` | Gateway runtime: url | None | No | [gateway/internal/app/app.go][source-33] |
| `GATEWAY_RUNTIME_SHA256` | SHA-256 digest binding for gateway runtime | Unset | No | [gateway/internal/config/config.go][source-6] |
| `GITHUB_TOKEN` | GitHub API credential; required for registry promotion | Unset | Yes | [security/supplychain/controller.py][source-13] |
| `GO` | Go command used by the launchpad Makefile | `go` | No | [Makefile][source-14] |
| `GOFLAGS` | Additional Go command flags passed by the launchpad Makefile | Empty | No | [Makefile][source-14] |
| `IDENTITY_ASSERTION_AUDIENCE` | Audience identifier in service identity assertions | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_ASSERTION_ISSUER` | Issuer identifier in service identity assertions | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_ASSERTION_SIGNING_KEY` | Signing secret for service identity assertions | Unset | Yes | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_AUDIENCE` | Identity audience setting | Unset | No | [metadata/internal/config/config.go][source-7] |
| `IDENTITY_CA_FILE` | Path to identity ca material | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_CA_PEM` | Inline identity ca PEM material; alternative to the corresponding file | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_CERT_SHA256` | SHA-256 digest binding for identity cert | Unset | No | [user/internal/config/config.go][source-2] |
| `IDENTITY_CLIENT_CERT_FILE` | Path to identity client cert material | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_CLIENT_CERT_PEM` | Inline identity client cert PEM material; alternative to the corresponding file | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_CLIENT_KEY_FILE` | Path to identity client key material | Unset | No | [api/internal/config/config.go][source-1] |
| `IDENTITY_CLIENT_KEY_PEM` | Inline identity client key PEM material; alternative to the corresponding file | Unset | Yes | [api/internal/config/config.go][source-1] |
| `IDENTITY_CONFIG_DOCUMENT` | Base64-encoded identity config JSON; alternative to its file input | Unset | May contain secrets | [gateway/internal/config/config.go][source-6] |
| `IDENTITY_CONFIG_FILE` | Gateway identity domain, URI, origins and signing-key configuration file | Unset | No | [gateway/internal/config/config.go][source-6] |
| `IDENTITY_CONFIG_FILE (gateway): $.activeKey` | Identity signing keys: active key | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_FILE (gateway): $.allowedOrigins` | Identity signing keys: allowed origins | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_FILE (gateway): $.audience` | Identity signing keys: audience | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_FILE (gateway): $.domain` | Identity signing keys: domain | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_FILE (gateway): $.issuer` | Identity signing keys: issuer | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_FILE (gateway): $.keys[].ActiveAt` | Identity signing keys: active at | None | No | [gateway/internal/auth/identity_tokens.go][source-35] |
| `IDENTITY_CONFIG_FILE (gateway): $.keys[].ID` | Identity signing keys: id | None | No | [gateway/internal/auth/identity_tokens.go][source-35] |
| `IDENTITY_CONFIG_FILE (gateway): $.keys[].RetireAt` | Identity signing keys: retire at | None | No | [gateway/internal/auth/identity_tokens.go][source-35] |
| `IDENTITY_CONFIG_FILE (gateway): $.keys[].SecretReference` | Identity signing keys: secret reference; names an environment secret, whose value is supplied separately | None | No (name only) | [gateway/internal/auth/identity_tokens.go][source-35] |
| `IDENTITY_CONFIG_FILE (gateway): $.uri` | Identity signing keys: uri | None | No | [gateway/internal/app/app.go][source-33] |
| `IDENTITY_CONFIG_SHA256` | SHA-256 digest binding for identity config | Unset | No | [gateway/internal/config/config.go][source-6] |
| `IDENTITY_ENDPOINT` | Mutual TLS identity authority endpoint | Unset | No | [metadata/internal/config/config.go][source-7] |
| `IDENTITY_LIMITER_KEY` | Gateway identity rate-limit signing secret | Unset | Yes | [gateway/internal/config/config.go][source-6] |
| `IDENTITY_MTLS_CA_FILE` | Path to identity mtls ca material | Unset | No | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_CA_PEM` | Inline identity mtls ca PEM material; alternative to the corresponding file | Unset | No | [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_CLIENT_CERT_FILE` | Path to identity mtls client cert material | Unset | No | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_CLIENT_CERT_PEM` | Inline identity mtls client cert PEM material; alternative to the corresponding file | Unset | No | [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_CLIENT_KEY_FILE` | Path to identity mtls client key material | Unset | No | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_CLIENT_KEY_PEM` | Inline identity mtls client key PEM material; alternative to the corresponding file | Unset | Yes | [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_MTLS_PEER_CERT_SHA256` | Pinned identity authority certificate digest for mutual TLS | Unset | No | [livestream/internal/config/config.go][source-3]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10] |
| `IDENTITY_PEER_CERT_SHA256` | Pinned identity authority certificate digest | Unset | No | [api/internal/config/config.go][source-1]; [metadata/internal/config/config.go][source-7] |
| `IDENTITY_SERVICE_TOKEN` | Credential for requests to the identity authority | Unset | Yes | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `IDENTITY_SERVICE_URL` | Identity service endpoint; required by production consumers | Unset | No | [api/internal/config/config.go][source-1]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [user/internal/config/config.go][source-2] |
| `IDENTITY_SIGNING_KEY` | Signing secret for identity assertions | Unset | Yes | [api/internal/config/config.go][source-1]; [pnl-tracker/internal/config/config.go][source-5] |
| `IDENTITY_SIGNING_SECRET` | Gateway identity signing secret | Unset | Yes | [gateway/internal/config/config.go][source-6] |
| `INDEXER_BATCH_SIZE` | Indexer batch size setting | `50` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_CONCURRENCY` | Indexer concurrency setting | `25` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_HEAD_SOURCE` | Non-production head source; an empty value follows INDEXER_LOG_SOURCE | `INDEXER_LOG_SOURCE` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_HISTORICAL_ATTEMPT_TIMEOUT_MS` | Indexer historical attempt timeout ms setting | `8000` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_HISTORICAL_CONCURRENCY` | Indexer historical concurrency setting | `48` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_HISTORICAL_READAHEAD` | Indexer historical readahead setting | `256` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_HISTORICAL_RPC_URL` | Historical JSON-RPC endpoint; empty follows RPC_URL | `RPC_URL` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_LIFECYCLE_CONFIG_DOCUMENT` | Base64-encoded indexer lifecycle config JSON; alternative to its file input | Unset | May contain secrets | [indexer/internal/config/config.go][source-4] |
| `INDEXER_LIFECYCLE_CONFIG_FILE` | Path to the indexer lifecycle config JSON document; alternative to its inline document | Unset | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.EventsPerSecond` | Indexer lifecycle: events per second | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.InitialCheckpointHash` | Indexer lifecycle: initial checkpoint hash | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.InitialCheckpointHeight` | Indexer lifecycle: initial checkpoint height | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.WorkBurst` | Indexer lifecycle: work burst | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.WorkConcurrency` | Indexer lifecycle: work concurrency | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.ABIDigest` | Indexer lifecycle: abi digest | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.AccessKeyReference` | Indexer lifecycle: access key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.BatchBlocks` | Indexer lifecycle: batch blocks | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.Bucket` | Indexer lifecycle: bucket | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.CodeDigest` | Indexer lifecycle: code digest | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.DecoderDigest` | Indexer lifecycle: decoder digest | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.Endpoint` | Indexer lifecycle: endpoint | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.FromHeight` | Indexer lifecycle: from height | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.PollMS` | Indexer lifecycle: poll ms | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.Prefix` | Indexer lifecycle: prefix | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.ReadinessObject` | Indexer lifecycle: readiness object | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.ReadinessObjectBytes` | Indexer lifecycle: readiness object bytes | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.Region` | Indexer lifecycle: region | `auto` | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.SecretKeyReference` | Indexer lifecycle: secret key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.archive.enabled` | Indexer lifecycle: enabled | `false` | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.AuthorizationID` | Indexer lifecycle: authorization id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.CheckpointHash` | Indexer lifecycle: checkpoint hash | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.CheckpointHeight` | Indexer lifecycle: checkpoint height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.FromHeight` | Indexer lifecycle: from height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.Generation` | Indexer lifecycle: generation | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.JobID` | Indexer lifecycle: job id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.Mode` | Indexer lifecycle: mode | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.ProjectionRevision` | Indexer lifecycle: projection revision | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.Requester` | Indexer lifecycle: requester | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.SourceStream` | Indexer lifecycle: source stream | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.Target` | Indexer lifecycle: target | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.backfillRequest.ToHeight` | Indexer lifecycle: to height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Anchor.Hash` | Indexer lifecycle: hash | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Anchor.Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.BlockIdentity` | Indexer lifecycle: block identity | `keccak` | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.BulkDeadline` | Indexer lifecycle: bulk deadline in nanoseconds | `15000000000` (15 seconds) | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Endpoints.*` | Indexer lifecycle: endpoints | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].ABIRevision` | Indexer lifecycle: abi revision | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].ActivationHeight` | Indexer lifecycle: activation height | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].Address` | Indexer lifecycle: address | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].CreationTxIndex` | Indexer lifecycle: creation tx index | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].RetirementHeight` | Indexer lifecycle: retirement height | Unset | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.Epochs[].Role` | Indexer lifecycle: role | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.GenesisHash` | Indexer lifecycle: genesis hash | Unset | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.HistoricalEndpoints.*` | Indexer lifecycle: historical endpoints | Unset | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.LiveDeadline` | Indexer lifecycle: live deadline in nanoseconds | `2000000000` (2 seconds) | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.committed.MaxResponseBytes` | Indexer lifecycle: max response bytes | `33554432` | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.endpointSecretReferences.*` | Indexer lifecycle: endpoint secret references; names an environment secret, whose value is supplied separately | None | No (name only) | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].ABIRevision` | Indexer lifecycle: abi revision | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].ActivationHeight` | Indexer lifecycle: activation height | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].Address` | Indexer lifecycle: address | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].CreationTxIndex` | Indexer lifecycle: creation tx index | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].RetirementHeight` | Indexer lifecycle: retirement height | Unset | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.Epochs[].Role` | Indexer lifecycle: role | None | No | [indexer/internal/source/committed.go][source-36] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ExpectedRole` | Indexer lifecycle: expected role | None | No | [indexer/internal/source/explorer_committed.go][source-37] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/source/explorer_committed.go][source-37] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.MaxConnections` | Indexer lifecycle: max connections | None | No | [indexer/internal/source/explorer_committed.go][source-37] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.BlockDifferences[].Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.BlockDifferences[].Kind` | Indexer lifecycle: kind | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.BlockDifferences[].SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Certified` | Indexer lifecycle: certified | `false` | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.CompleteBlockCount` | Indexer lifecycle: complete block count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Count` | Indexer lifecycle: count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Identity.BlockHash` | Indexer lifecycle: block hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Identity.ChainID` | Indexer lifecycle: chain id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Identity.LogIndex` | Indexer lifecycle: log index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Identity.TransactionHash` | Indexer lifecycle: transaction hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].Kind` | Indexer lifecycle: kind | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].LeftPayloadSHA256` | Indexer lifecycle: left payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].RightPayloadSHA256` | Indexer lifecycle: right payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Different[].TransactionIndex` | Indexer lifecycle: transaction index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.DisputesTruncated` | Indexer lifecycle: disputes truncated | `false` | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Count` | Indexer lifecycle: count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Identity.BlockHash` | Indexer lifecycle: block hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Identity.ChainID` | Indexer lifecycle: chain id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Identity.LogIndex` | Indexer lifecycle: log index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Identity.TransactionHash` | Indexer lifecycle: transaction hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.Kind` | Indexer lifecycle: kind | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.LeftPayloadSHA256` | Indexer lifecycle: left payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.RightPayloadSHA256` | Indexer lifecycle: right payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Difference.TransactionIndex` | Indexer lifecycle: transaction index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Left` | Indexer lifecycle: left | Unset | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Disputes[].Right` | Indexer lifecycle: right | Unset | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Count` | Indexer lifecycle: count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Identity.BlockHash` | Indexer lifecycle: block hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Identity.ChainID` | Indexer lifecycle: chain id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Identity.LogIndex` | Indexer lifecycle: log index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Identity.TransactionHash` | Indexer lifecycle: transaction hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].Kind` | Indexer lifecycle: kind | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].LeftPayloadSHA256` | Indexer lifecycle: left payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].RightPayloadSHA256` | Indexer lifecycle: right payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Duplicates[].TransactionIndex` | Indexer lifecycle: transaction index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.ElapsedNanoseconds` | Indexer lifecycle: elapsed nanoseconds | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.EmptyBlockCount` | Indexer lifecycle: empty block count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.FromHeight` | Indexer lifecycle: from height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.HighLogBlockCount` | Indexer lifecycle: high log block count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.LeftBlocksSHA256` | Indexer lifecycle: left blocks sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.LeftCount` | Indexer lifecycle: left count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.LeftRawSHA256` | Indexer lifecycle: left raw sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.LeftSourceID` | Indexer lifecycle: left source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Count` | Indexer lifecycle: count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Height` | Indexer lifecycle: height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Identity.BlockHash` | Indexer lifecycle: block hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Identity.ChainID` | Indexer lifecycle: chain id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Identity.LogIndex` | Indexer lifecycle: log index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Identity.TransactionHash` | Indexer lifecycle: transaction hash | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].Kind` | Indexer lifecycle: kind | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].LeftPayloadSHA256` | Indexer lifecycle: left payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].RightPayloadSHA256` | Indexer lifecycle: right payload sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Missing[].TransactionIndex` | Indexer lifecycle: transaction index | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.ReportID` | Indexer lifecycle: report id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.RightBlocksSHA256` | Indexer lifecycle: right blocks sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.RightCount` | Indexer lifecycle: right count | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.RightRawSHA256` | Indexer lifecycle: right raw sha256 | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.RightSourceID` | Indexer lifecycle: right source id | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.ToHeight` | Indexer lifecycle: to height | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.ParityAdmission.Version` | Indexer lifecycle: version | None | No | [indexer/internal/source/parity.go][source-38] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.SchemaVersion` | Indexer lifecycle: schema version | None | No | [indexer/internal/source/explorer_committed.go][source-37] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.explorer.SourceID` | Indexer lifecycle: source id | None | No | [indexer/internal/source/explorer_committed.go][source-37] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.Action` | Indexer lifecycle: action | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.AuthorizationID` | Indexer lifecycle: authorization id | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.From` | Indexer lifecycle: from | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.Generation` | Indexer lifecycle: generation | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.Operator` | Indexer lifecycle: operator | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.Revision` | Indexer lifecycle: revision | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.Target` | Indexer lifecycle: target | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.operations.*.To` | Indexer lifecycle: to | None | No | [indexer/internal/store/quarantine.go][source-42] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.optionalEmitterTopics.*.ABIRevision` | Indexer lifecycle: abi revision | None | No | [indexer/internal/processor/quarantine.go][source-39] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.optionalEmitterTopics.*.Requirement` | Indexer lifecycle: requirement | None | No | [indexer/internal/processor/quarantine.go][source-39] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].ConsumerID` | Indexer lifecycle: consumer id | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].ExpectedAliasRevision` | Indexer lifecycle: expected alias revision | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].ManifestDigest` | Indexer lifecycle: manifest digest | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Promote` | Indexer lifecycle: promote | `false` | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.AuthorizationID` | Indexer lifecycle: authorization id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.CheckpointHash` | Indexer lifecycle: checkpoint hash | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.CheckpointHeight` | Indexer lifecycle: checkpoint height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.FromHeight` | Indexer lifecycle: from height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.Generation` | Indexer lifecycle: generation | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.JobID` | Indexer lifecycle: job id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.Mode` | Indexer lifecycle: mode | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.ProjectionRevision` | Indexer lifecycle: projection revision | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.Requester` | Indexer lifecycle: requester | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.SourceStream` | Indexer lifecycle: source stream | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.Target` | Indexer lifecycle: target | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.rebuilds[].Request.ToHeight` | Indexer lifecycle: to height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayBatchSize` | Indexer lifecycle: replay batch size | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].AuthorizationID` | Indexer lifecycle: authorization id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].CheckpointHash` | Indexer lifecycle: checkpoint hash | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].CheckpointHeight` | Indexer lifecycle: checkpoint height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].FromHeight` | Indexer lifecycle: from height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].Generation` | Indexer lifecycle: generation | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].JobID` | Indexer lifecycle: job id | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].Mode` | Indexer lifecycle: mode | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].ProjectionRevision` | Indexer lifecycle: projection revision | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].Requester` | Indexer lifecycle: requester | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].SourceStream` | Indexer lifecycle: source stream | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].Target` | Indexer lifecycle: target | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.replayRequests[].ToHeight` | Indexer lifecycle: to height | None | No | [indexer/internal/store/replay.go][source-41] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.sourceSelectionKeys.*` | Indexer lifecycle: source selection keys | Unset | Yes | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.version` | Indexer lifecycle: version | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.CAFile` | Indexer lifecycle: ca file | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.CASHA256` | Indexer lifecycle: casha256 | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.LeaseMS` | Indexer lifecycle: lease ms | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.MaxAttempts` | Indexer lifecycle: max attempts | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.PollMS` | Indexer lifecycle: poll ms | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.RetryMS` | Indexer lifecycle: retry ms | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.TimeoutMS` | Indexer lifecycle: timeout ms | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.generation` | Indexer lifecycle: generation | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.targets[].ID` | Indexer lifecycle: id | None | No | [sdk/launchpad/shared/delivery/webhook.go][source-40] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.targets[].SecretReference` | Indexer lifecycle: secret reference; names an environment secret, whose value is supplied separately | None | No (name only) | [sdk/launchpad/shared/delivery/webhook.go][source-40] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.targets[].Subjects.*` | Indexer lifecycle: subjects | None | No | [sdk/launchpad/shared/delivery/webhook.go][source-40] |
| `INDEXER_LIFECYCLE_CONFIG_FILE (indexer): $.webhooks.targets[].URL` | Indexer lifecycle: url | None | No | [sdk/launchpad/shared/delivery/webhook.go][source-40] |
| `INDEXER_LIFECYCLE_CONFIG_SHA256` | SHA-256 digest binding for indexer lifecycle config | Unset | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_LOG_SOURCE` | Non-production log source: paxscan, rpc or rpc-getlogs | `paxscan` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_MODE` | Indexer live or backfill mode | `live` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_POLL_INTERVAL_MS` | Indexer poll interval in milliseconds | `100` | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_RECEIPT_CONFIG_DOCUMENT` | Base64-encoded indexer receipt config JSON; alternative to its file input | Unset | May contain secrets | [indexer/internal/config/config.go][source-4] |
| `INDEXER_RECEIPT_CONFIG_FILE` | Path to the indexer receipt config JSON document; alternative to its inline document | Unset | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.ActivationHeight` | Receipt authority: activation height | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.CatalogJSON` | Receipt authority: catalog json (base64 bytes) | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.CreationTransactions.*` | Receipt authority: creation transactions | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.DeploymentVersion` | Receipt authority: deployment version | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].Immutables[].Length` | Receipt authority: length | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].Immutables[].Start` | Receipt authority: start | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].Kind` | Receipt authority: kind | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].Role` | Receipt authority: role | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Discovery[].RuntimeHash` | Receipt authority: runtime hash | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Address` | Receipt authority: address | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Enabled` | Receipt authority: enabled | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.EnabledOptionalEvents[]` | Receipt authority: enabled optional events | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.EnumMax.*.*` | Receipt authority: enum max | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.From.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.From.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.From.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Optional` | Receipt authority: optional | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.PaxeerNonzeroBools.*[]` | Receipt authority: paxeer nonzero bools | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Role` | Receipt authority: role | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Until.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Until.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].deployment.Until.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.address` | Receipt authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.beaconAddress` | Receipt authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.implementationAddress` | Receipt authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.implementationHash` | Receipt authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.kind` | Receipt authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.runtimeHash` | Receipt authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.stagedRuntimeHash` | Receipt authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].identity.stagedUpgrade` | Receipt authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Epochs[].sourceActivationHeight` | Receipt authority: source activation height | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.GenesisHash` | Receipt authority: genesis hash | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.address` | Receipt authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.beaconAddress` | Receipt authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.implementationAddress` | Receipt authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.implementationHash` | Receipt authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.kind` | Receipt authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.runtimeHash` | Receipt authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.stagedRuntimeHash` | Receipt authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Roles.*.stagedUpgrade` | Receipt authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.privateFixture.Timeout` | Receipt authority: timeout in nanoseconds | `15000000000` (15 seconds) | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Anchor.Hash` | Receipt authority: hash | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Anchor.Height` | Receipt authority: height | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.BlockIdentity` | Receipt authority: block identity | `keccak` | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.CatalogJSON` | Receipt authority: catalog json (base64 bytes) | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].Immutables[].Length` | Receipt authority: length | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].Immutables[].Start` | Receipt authority: start | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].Kind` | Receipt authority: kind | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].Role` | Receipt authority: role | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Discovery[].RuntimeHash` | Receipt authority: runtime hash | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Address` | Receipt authority: address | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Enabled` | Receipt authority: enabled | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.EnabledOptionalEvents[]` | Receipt authority: enabled optional events | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.EnumMax.*.*` | Receipt authority: enum max | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.From.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.From.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.From.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Optional` | Receipt authority: optional | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.PaxeerNonzeroBools.*[]` | Receipt authority: paxeer nonzero bools | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Role` | Receipt authority: role | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Until.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Until.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].deployment.Until.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.address` | Receipt authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.beaconAddress` | Receipt authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.implementationAddress` | Receipt authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.implementationHash` | Receipt authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.kind` | Receipt authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.runtimeHash` | Receipt authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.stagedRuntimeHash` | Receipt authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].identity.stagedUpgrade` | Receipt authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Epochs[].sourceActivationHeight` | Receipt authority: source activation height | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.GenesisHash` | Receipt authority: genesis hash | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].ActivationHeight` | Receipt authority: activation height | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].Immutables[].Length` | Receipt authority: length | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].Immutables[].Start` | Receipt authority: start | None | No | [indexer/internal/receipt/identity.go][source-46] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].Kind` | Receipt authority: kind | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].Role` | Receipt authority: role | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Discovery[].RuntimeHash` | Receipt authority: runtime hash | None | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.ABIRevision` | Receipt authority: abi revision | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Address` | Receipt authority: address | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Enabled` | Receipt authority: enabled | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.EnabledOptionalEvents[]` | Receipt authority: enabled optional events | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.EnumMax.*.*` | Receipt authority: enum max | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.From.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.From.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.From.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Optional` | Receipt authority: optional | `false` | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.PaxeerNonzeroBools.*[]` | Receipt authority: paxeer nonzero bools | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Role` | Receipt authority: role | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Until.block` | Receipt authority: block | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Until.log` | Receipt authority: log | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].deployment.Until.transaction` | Receipt authority: transaction | None | No | [indexer/internal/decode/registry.go][source-45] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.address` | Receipt authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.beaconAddress` | Receipt authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.implementationAddress` | Receipt authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.implementationHash` | Receipt authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.kind` | Receipt authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.runtimeHash` | Receipt authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.stagedRuntimeHash` | Receipt authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].identity.stagedUpgrade` | Receipt authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.History[].Epochs[].sourceActivationHeight` | Receipt authority: source activation height | Unset | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.activationHeight` | Receipt authority: activation height | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.chainId` | Receipt authority: chain id | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.coreOracleRequired` | Receipt authority: core oracle required | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.creationFeeRaw` | Receipt authority: creation fee raw | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.deploymentVersion` | Receipt authority: deployment version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.governance.expectedBeaconOwner` | Receipt authority: expected beacon owner | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.governance.mode` | Receipt authority: mode | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.governance.timelockDelaySeconds` | Receipt authority: timelock delay seconds | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.graduation` | Receipt authority: graduation | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.marketModel` | Receipt authority: market model | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.native.decimals` | Receipt authority: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.native.symbol` | Receipt authority: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.address` | Receipt authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.beaconAddress` | Receipt authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.implementationAddress` | Receipt authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.implementationHash` | Receipt authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.kind` | Receipt authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.runtimeHash` | Receipt authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.stagedRuntimeHash` | Receipt authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.roles.*.stagedUpgrade` | Receipt authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.schemaVersion` | Receipt authority: schema version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.settlement.decimals` | Receipt authority: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.settlement.symbol` | Receipt authority: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.sourceManifestSha256` | Receipt authority: source manifest sha256 | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Manifest.status` | Receipt authority: status | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.production.Timeout` | Receipt authority: timeout in nanoseconds | `15000000000` (15 seconds) | No | [indexer/internal/receipt/rpc.go][source-44] |
| `INDEXER_RECEIPT_CONFIG_FILE (indexer): $.realm` | Receipt authority: realm | None | No | [indexer/internal/app/app.go][source-20] |
| `INDEXER_RECEIPT_CONFIG_SHA256` | SHA-256 digest binding for indexer receipt config | Unset | No | [indexer/internal/config/config.go][source-4] |
| `INDEXER_START_BLOCK` | Indexer start block setting | `0` | No | [indexer/internal/config/config.go][source-4] |
| `INSTANCE_ID` | Process instance identity; required in production | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `LIVEPEER_API_KEY` | Livepeer provider API credential | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_PLAYBACK_WEBHOOK_ID` | Livepeer playback webhook id setting | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_PLAYBACK_WEBHOOK_SECRET` | Authenticates playback webhook requests | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_PLAYBACK_WEBHOOK_URL` | Livepeer playback webhook endpoint | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_PROVIDER_MODE` | Provider availability mode; connected mode requires available | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_WEBHOOK_SECRET` | Authenticates provider webhook requests | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVEPEER_WEBHOOK_TIMESTAMP_UNIT` | Provider timestamp unit: seconds or milliseconds; required for connected mode | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_BUDGET_ALERT_KEY` | Authenticates budget alert delivery | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_BUDGET_ALERT_URL` | Livestream budget alert endpoint | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_BUDGET_POLICY_DOCUMENT` | Base64-encoded livestream budget policy JSON; alternative to its file input | Unset | May contain secrets | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_BUDGET_POLICY_FILE` | Path to the livestream budget policy JSON document; alternative to its inline document | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.approvalReference` | Stream usage budget: approval reference; names an environment secret, whose value is supplied separately | None | No (name only) | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.broadcastReserveUnits` | Stream usage budget: broadcast reserve units | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.currency` | Stream usage budget: currency | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.deliveryPricePerMinute` | Stream usage budget: delivery price per minute | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.deliveryPricePerTB` | Stream usage budget: delivery price per tb | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.monthlyUnits` | Stream usage budget: monthly units | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.peakDurationSeconds` | Stream usage budget: peak duration seconds | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.storagePricePerMinute` | Stream usage budget: storage price per minute | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.transcodePricePerMinute` | Stream usage budget: transcode price per minute | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_FILE (livestream): $.vendorUserId` | Stream usage budget: vendor user id | None | No | [livestream/internal/streams/usage.go][source-47] |
| `LIVESTREAM_BUDGET_POLICY_SHA256` | SHA-256 digest binding for livestream budget policy | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_CONNECTED_ENABLED` | Enables connected livestream control and accounting | `false` | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_INGEST_ENDPOINT` | Livestream ingest endpoint setting | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_PRIVATE_LISTEN_ADDRESS` | Livestream private listen address setting | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_CERT_FILE` | Path to livestream tls cert material | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_CERT_PEM` | Inline livestream tls cert PEM material; alternative to the corresponding file | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_CLIENT_CA_FILE` | Path to livestream tls client ca material | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_CLIENT_CA_PEM` | Inline livestream tls client ca PEM material; alternative to the corresponding file | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_KEY_FILE` | Path to livestream tls key material | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_TLS_KEY_PEM` | Inline livestream tls key PEM material; alternative to the corresponding file | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_VAULT_TRANSIT_KEY` | Vault transit key name | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_VAULT_TRANSIT_TOKEN` | Vault transit API credential | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_VAULT_TRANSIT_URL` | Vault transit API endpoint | Unset | No | [livestream/internal/config/config.go][source-3] |
| `LIVESTREAM_VIEWER_TOKEN_KEY` | Signing secret for playback viewer tokens | Unset | Yes | [livestream/internal/config/config.go][source-3] |
| `LOG_LEVEL` | Logging level: debug, info, warn or error | `info` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `MAX_AVATAR_SIZE_BYTES` | Max avatar size in bytes | `2097152` | No | [user/internal/config/config.go][source-2] |
| `MAX_BANNER_SIZE_BYTES` | Max banner size in bytes | `5242880` | No | [metadata/internal/config/config.go][source-7]; [user/internal/config/config.go][source-2] |
| `MAX_DM_MSGS_PER_WINDOW` | Max dm msgs per window setting | `5` | No | [social/internal/config/config.go][source-9] |
| `MAX_GLOBAL_STREAMS` | Global concurrent stream cap; required in connected mode | `0` | No | [livestream/internal/config/config.go][source-3] |
| `MAX_LOGO_SIZE_BYTES` | Max logo size in bytes | `2097152` | No | [metadata/internal/config/config.go][source-7] |
| `MAX_MESSAGE_LENGTH` | Max message length setting | `500` | No | [social/internal/config/config.go][source-9] |
| `MAX_POOL_MSGS_PER_WINDOW` | Max pool msgs per window setting | `5` | No | [social/internal/config/config.go][source-9] |
| `MAX_STREAMS_PER_WALLET` | Per-wallet concurrent stream cap | `3` | No | [livestream/internal/config/config.go][source-3] |
| `MAX_UPLOAD_BYTES` | Max upload in bytes | `6291456` | No | [gateway/internal/config/config.go][source-6] |
| `MEDIA_EXPIRY_INTERVAL_SECONDS` | Media expiry interval in seconds | `300` | No | [metadata/internal/config/config.go][source-7] |
| `MEDIA_SCAN_INTERVAL_SECONDS` | Media scan interval in seconds | `5` | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_ASSERTION_PRIVATE_KEY_HEX` | Metadata assertion private key hex setting | Unset | Yes | [metadata/internal/config/config.go][source-7] |
| `METADATA_BUCKET` | Metadata object storage bucket | Unset | No | [gateway/internal/config/config.go][source-6] |
| `METADATA_CERT_SHA256` | SHA-256 digest binding for metadata cert | Unset | No | [user/internal/config/config.go][source-2] |
| `METADATA_INGRESS` | Metadata ingress transport selection | `mtls` | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_AUDIT_KEY` | Metadata publication audit key used to sign, encrypt or authenticate component data | Unset | Yes | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_DOCUMENT` | Base64-encoded metadata publication config JSON; alternative to its file input | Unset | May contain secrets | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_FILE` | Path to the metadata publication config JSON document; alternative to its inline document | Unset | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_FILE (metadata): $.envelopeHeader` | Metadata publication: envelope header | None | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_FILE (metadata): $.environment` | Metadata publication: environment | None | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_FILE (metadata): $.schemaVersion` | Metadata publication: schema version | None | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_FILE (metadata): $.signatureHeader` | Metadata publication: signature header | None | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_PUBLICATION_CONFIG_SHA256` | SHA-256 digest binding for metadata publication config | Unset | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_SCHEMA` | Metadata database schema | `metadata` | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_SERVICE_URL` | Metadata service endpoint | Unset | No | [user/internal/config/config.go][source-2] |
| `METADATA_TLS_CERT_FILE` | Path to metadata tls cert material | Unset | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_TLS_CLIENT_CA_FILE` | Path to metadata tls client ca material | Unset | No | [metadata/internal/config/config.go][source-7] |
| `METADATA_TLS_KEY_FILE` | Path to metadata tls key material | Unset | No | [metadata/internal/config/config.go][source-7] |
| `METRICS_ENVIRONMENT` | Environment label on runtime metrics | `production` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `MODERATION_BEARER_KEY` | Name of the environment secret used to authenticate to the moderation service | Unset | No | [metadata/internal/config/config.go][source-7] |
| `MODERATION_ENDPOINT` | Moderation visibility authority endpoint | Unset | No | [metadata/internal/config/config.go][source-7] |
| `MODERATION_PEER_CERT_SHA256` | SHA-256 digest binding for moderation peer cert | Unset | No | [metadata/internal/config/config.go][source-7] |
| `MODERATION_SCHEMA` | Database visibility schema when MODERATION_SOURCE is database | `chat` | No | [metadata/internal/config/config.go][source-7] |
| `MODERATION_SOURCE` | Visibility authority selection: service or database | `service` | No | [metadata/internal/config/config.go][source-7] |
| `NATS_CREDENTIALS` | Path to the broker credentials file; alternative to NATS_CREDENTIALS_CREDS | Unset | No | [api/internal/config/config.go][source-1]; [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `NATS_CREDENTIALS_CREDS` | Inline broker credentials; alternative to NATS_CREDENTIALS | Unset | Yes | [api/internal/config/config.go][source-1]; [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `NATS_PASSWORD` | Private deployment broker password | Unset | Yes | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `NATS_URL` | Broker endpoint; authenticated TLS is required in production | Unset | No | [api/internal/config/config.go][source-1]; [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `NATS_USERNAME` | Private deployment broker username | Unset | No | [indexer/internal/config/config.go][source-4]; [pnl-tracker/internal/config/config.go][source-5]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11] |
| `NETWORK_AUTHORITY_CACHE_TTL_SEC` | Network authority cache ttl sec setting | `20` | No | [api/internal/config/config.go][source-1] |
| `NETWORK_AUTHORITY_CONFIG_DOCUMENT` | Base64-encoded network authority config JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1] |
| `NETWORK_AUTHORITY_CONFIG_FILE` | Path to the network authority config JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.endpoints.AllowedExplorers[]` | Network authority: allowed explorers | None | No | [sdk/launchpad/shared/config/deployment/public.go][source-49] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.endpoints.AllowedRPCs[]` | Network authority: allowed rp cs | None | No | [sdk/launchpad/shared/config/deployment/public.go][source-49] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.endpoints.Explorer` | Network authority: explorer | None | No | [sdk/launchpad/shared/config/deployment/public.go][source-49] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.endpoints.RPC` | Network authority: rpc | None | No | [sdk/launchpad/shared/config/deployment/public.go][source-49] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.expectedRevisionSha256` | Network authority: expected revision sha256 | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.frontendAddresses.*` | Network authority: frontend addresses | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.goAddresses.*` | Network authority: go addresses | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.activationHeight` | Network authority: activation height | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.chainId` | Network authority: chain id | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.coreOracleRequired` | Network authority: core oracle required | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.creationFeeRaw` | Network authority: creation fee raw | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.deploymentVersion` | Network authority: deployment version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.governance.expectedBeaconOwner` | Network authority: expected beacon owner | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.governance.mode` | Network authority: mode | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.governance.timelockDelaySeconds` | Network authority: timelock delay seconds | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.graduation` | Network authority: graduation | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.marketModel` | Network authority: market model | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.native.decimals` | Network authority: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.native.symbol` | Network authority: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.address` | Network authority: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.beaconAddress` | Network authority: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.implementationAddress` | Network authority: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.implementationHash` | Network authority: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.kind` | Network authority: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.runtimeHash` | Network authority: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.stagedRuntimeHash` | Network authority: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.roles.*.stagedUpgrade` | Network authority: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.schemaVersion` | Network authority: schema version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.settlement.decimals` | Network authority: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.settlement.symbol` | Network authority: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.sourceManifestSha256` | Network authority: source manifest sha256 | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.manifest.status` | Network authority: status | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.reviewedPoolRuntime` | Network authority: reviewed pool runtime | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.schemaVersion` | Network authority: schema version | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.serialInterfaceSha256` | Network authority: serial interface sha256 | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_FILE (api): $.sourceSchema` | Network authority: source schema | None | No | [api/internal/app/authority.go][source-48] |
| `NETWORK_AUTHORITY_CONFIG_SHA256` | SHA-256 digest binding for network authority config | Unset | No | [api/internal/config/config.go][source-1] |
| `NETWORK_RPC_URL` | Network rpc endpoint | Unset | No | [api/internal/config/config.go][source-1] |
| `NODE_ENV` | Execution environment: production, development or test | `production` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `NOTIFICATION_CERT_SHA256` | SHA-256 digest binding for notification cert | Unset | No | [user/internal/config/config.go][source-2] |
| `NOTIFICATION_D11_TOKEN` | Authenticates inbound account notification requests | Unset | Yes | [user/internal/config/config.go][source-2] |
| `NOTIFICATION_SERVICE_TOKEN` | Authenticates requests to the notification service | Unset | Yes | [user/internal/config/config.go][source-2] |
| `NOTIFICATION_SERVICE_URL` | Notification service endpoint | Unset | No | [user/internal/config/config.go][source-2] |
| `OBJECT_CACHE_MAX_BYTES` | Object cache max in bytes | `262144` | No | [gateway/internal/config/config.go][source-6] |
| `OBJECT_CACHE_TTL_SECONDS` | Object cache ttl in seconds | `300` | No | [gateway/internal/config/config.go][source-6] |
| `OG_BUCKET` | Og object storage bucket | Unset | No | [gateway/internal/config/config.go][source-6] |
| `PATH` | Executable search path inherited by supply-chain commands | Inherited process path | No | [security/supplychain/ci_tools.py][source-15] |
| `PAXSCAN_DATABASE_URL` | Explorer PostgreSQL connection; required when a non-production log or head source uses paxscan | Unset | Yes | [indexer/internal/config/config.go][source-4] |
| `PNL_CDN_TOKEN` | Credential for PnL CDN requests | Unset | Yes | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_CURSOR_KEY` | Pnl cursor key used to sign, encrypt or authenticate component data | Unset | Yes | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_FONT_DIR` | Directory containing PnL image fonts | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_JOURNEY_CONFIG_DOCUMENT` | Base64-encoded pnl journey config JSON; alternative to its file input | Unset | May contain secrets | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_JOURNEY_CONFIG_FILE` | Path to the pnl journey config JSON document; alternative to its inline document | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.approvedCommunityWallets[]` | Portfolio and reward operations: approved community wallets | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.brokerStream` | Portfolio and reward operations: broker stream | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.cdnOrigin` | Portfolio and reward operations: cdn origin | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.cdnZone` | Portfolio and reward operations: cdn zone | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.exportsDirectory` | Portfolio and reward operations: exports directory | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.fontBoldSha256` | Portfolio and reward operations: font bold sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.fontDirectory` | Portfolio and reward operations: font directory | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.fontLicenseSha256` | Portfolio and reward operations: font license sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.fontRegularSha256` | Portfolio and reward operations: font regular sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityAudience` | Portfolio and reward operations: identity audience | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityCaFile` | Portfolio and reward operations: identity ca file | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityCaSha256` | Portfolio and reward operations: identity casha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityClientCertFile` | Portfolio and reward operations: identity client cert file | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityClientCertSha256` | Portfolio and reward operations: identity client cert sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityClientKeyFile` | Portfolio and reward operations: identity client key file | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityClientKeySha256` | Portfolio and reward operations: identity client key sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityEndpoint` | Portfolio and reward operations: identity endpoint | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityIssuer` | Portfolio and reward operations: identity issuer | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.identityPeerCertSha256` | Portfolio and reward operations: identity peer cert sha256 | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.interfaceDigest` | Portfolio and reward operations: interface digest | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.liveBatch` | Portfolio and reward operations: live batch | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.nodeIds[]` | Portfolio and reward operations: node i ds | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].Adapter` | Portfolio and reward operations: adapter | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].AdapterCodeHash` | Portfolio and reward operations: adapter code hash | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].Asset` | Portfolio and reward operations: asset | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].AwardCapRaw` | Portfolio and reward operations: award cap raw | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].BeneficiaryCapRaw` | Portfolio and reward operations: beneficiary cap raw | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].CashbackBPS` | Portfolio and reward operations: cashback bps | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].CashbackEndHeight` | Portfolio and reward operations: cashback end height | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].CashbackStartHeight` | Portfolio and reward operations: cashback start height | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].Custody` | Portfolio and reward operations: custody | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].DailyCapRaw` | Portfolio and reward operations: daily cap raw | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].DeploymentID` | Portfolio and reward operations: deployment id | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].ID` | Portfolio and reward operations: id | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].ReferralRaw` | Portfolio and reward operations: referral raw | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].Treasury` | Portfolio and reward operations: treasury | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.programs[].Version` | Portfolio and reward operations: version | None | No | [pnl-tracker/internal/production/l18/authority.go][source-50] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.realm` | Portfolio and reward operations: realm | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.releaseDigest` | Portfolio and reward operations: release digest | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.rewardManifestDigest` | Portfolio and reward operations: reward manifest digest | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.rewardRuntimeHashes.*` | Portfolio and reward operations: reward runtime hashes | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_FILE (pnl-tracker): $.version` | Portfolio and reward operations: version | None | No | [pnl-tracker/internal/app/app.go][source-23] |
| `PNL_JOURNEY_CONFIG_SHA256` | SHA-256 digest binding for pnl journey config | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_OG_BASE_URL` | Open Graph image origin; empty follows PNL_PUBLIC_BASE_URL | `PNL_PUBLIC_BASE_URL` | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_RECONCILE_BATCH_SIZE` | Pnl reconcile batch size setting | `500` | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_RECONCILE_INTERVAL_MS` | Pnl reconcile interval in milliseconds | `30000` | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PNL_REWARD_PER_CONVERSION` | Pnl reward per conversion setting | `1` | No | [pnl-tracker/internal/config/config.go][source-5] |
| `PORT` | Public HTTP listener port; container production requires 8080 | `3000` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `PRIVATE_FIXTURE_MANIFEST_FILE` | Private deployment manifest path; used only in the private_fixture realm | Unset | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.creationTransactions.*` | Private deployment identity: creation transactions | None | No | [sdk/launchpad/shared/config/env.go][source-12] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.activationHeight` | Private deployment identity: activation height | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.chainId` | Private deployment identity: chain id | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.coreOracleRequired` | Private deployment identity: core oracle required | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.creationFeeRaw` | Private deployment identity: creation fee raw | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.deploymentVersion` | Private deployment identity: deployment version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.governance.expectedBeaconOwner` | Private deployment identity: expected beacon owner | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.governance.mode` | Private deployment identity: mode | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.governance.timelockDelaySeconds` | Private deployment identity: timelock delay seconds | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.graduation` | Private deployment identity: graduation | `false` | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.marketModel` | Private deployment identity: market model | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.native.decimals` | Private deployment identity: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.native.symbol` | Private deployment identity: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.address` | Private deployment identity: address | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.beaconAddress` | Private deployment identity: beacon address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.implementationAddress` | Private deployment identity: implementation address | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.implementationHash` | Private deployment identity: implementation hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.kind` | Private deployment identity: kind | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.runtimeHash` | Private deployment identity: runtime hash | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.stagedRuntimeHash` | Private deployment identity: staged runtime hash | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.roles.*.stagedUpgrade` | Private deployment identity: staged upgrade | Unset | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.schemaVersion` | Private deployment identity: schema version | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.settlement.decimals` | Private deployment identity: decimals | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.settlement.symbol` | Private deployment identity: symbol | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.sourceManifestSha256` | Private deployment identity: source manifest sha256 | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.deployment.status` | Private deployment identity: status | None | No | [sdk/launchpad/shared/config/deployment/manifest.go][source-43] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.genesisHash` | Private deployment identity: genesis hash | None | No | [sdk/launchpad/shared/config/env.go][source-12] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.realm` | Private deployment identity: realm | None | No | [sdk/launchpad/shared/config/env.go][source-12] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.rpcUrl` | Private deployment identity: rpcurl | None | No | [sdk/launchpad/shared/config/env.go][source-12] |
| `PRIVATE_FIXTURE_MANIFEST_FILE (shared): $.schemaVersion` | Private deployment identity: schema version | None | No | [sdk/launchpad/shared/config/env.go][source-12] |
| `PRIVATE_FIXTURE_MANIFEST_SHA256` | SHA-256 binding for the private deployment manifest | Unset | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3] |
| `PUBLIC_URL` | Public origin for generated media links; user falls back to the request host | Unset | No | [gateway/internal/config/config.go][source-6]; [metadata/internal/config/config.go][source-7]; [user/internal/config/config.go][source-2] |
| `RANKING_MAX_ENTRIES` | Ranking max entries setting | `200` | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_NEW_TICK_INTERVAL_MS` | Ranking new tick interval in milliseconds | `10000` | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_DOCUMENT` | Base64-encoded ranking runtime JSON; alternative to its file input | Unset | May contain secrets | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE` | Path to the ranking runtime JSON document; alternative to its inline document | Unset | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.caFile` | Ranking runtime: ca file | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.caSha256` | Ranking runtime: casha256 | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.cacheNamespace` | Ranking runtime: cache namespace | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.clientCertFile` | Ranking runtime: client cert file | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.clientCertSha256` | Ranking runtime: client cert sha256 | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.clientKeyReference` | Ranking runtime: client key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.cursorKeyReference` | Ranking runtime: cursor key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.gatewayCertSHA256` | Ranking runtime: gateway cert sha256 | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.gatewayDNS` | Ranking runtime: gateway dns | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.maxAttempts` | Ranking runtime: max attempts | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.maximumReads` | Ranking runtime: maximum reads | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.natsCredentialsReference` | Ranking runtime: nats credentials reference; names an environment secret, whose value is supplied separately | None | No (name only) | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.natsURL` | Ranking runtime: natsurl | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.projectionVersion` | Ranking runtime: projection version | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.rankingSchema` | Ranking runtime: ranking schema | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.rankingStream` | Ranking runtime: ranking stream | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.recoveryEpoch` | Ranking runtime: recovery epoch | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.recoveryRegion` | Ranking runtime: recovery region | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.recoverySchema` | Ranking runtime: recovery schema | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.serverCertFile` | Ranking runtime: server cert file | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.serverCertSha256` | Ranking runtime: server cert sha256 | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.serverKeyReference` | Ranking runtime: server key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.sourceSchema` | Ranking runtime: source schema | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.sourceStream` | Ranking runtime: source stream | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.statsSchema` | Ranking runtime: stats schema | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.version` | Ranking runtime: version | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.visibilityCertSHA256` | Ranking runtime: visibility cert sha256 | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.visibilityDNS` | Ranking runtime: visibility dns | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_FILE (ranking-algo): $.visibilityEndpoint` | Ranking runtime: visibility endpoint | None | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_RUNTIME_SHA256` | SHA-256 digest binding for ranking runtime | Unset | No | [ranking-algo/internal/config/config.go][source-8] |
| `RANKING_TICK_INTERVAL_MS` | Ranking tick interval in milliseconds | `30000` | No | [ranking-algo/internal/config/config.go][source-8] |
| `RASTER_CGROUP_PARENT` | Parent control group for raster worker isolation | Unset | No | [metadata/internal/config/config.go][source-7] |
| `RASTER_EXECUTABLE` | Raster worker executable path | Unset | No | [metadata/internal/config/config.go][source-7] |
| `RASTER_ISOLATION` | Raster worker isolation mode | `cgroup` | No | [metadata/internal/config/config.go][source-7] |
| `RATE_LIMIT_MAX` | Rate limit max setting | `120` | No | [gateway/internal/config/config.go][source-6] |
| `RATE_LIMIT_WINDOW_SECONDS` | Rate limit window in seconds | `60` | No | [gateway/internal/config/config.go][source-6] |
| `RATE_WINDOW_SECONDS` | Rate window in seconds | `10` | No | [social/internal/config/config.go][source-9] |
| `RPC_GETLOGS_HEALTH_INTERVAL_MS` | Rpc getlogs health interval in milliseconds | `30000` | No | [indexer/internal/config/config.go][source-4] |
| `RPC_GETLOGS_RECEIPT_CONCURRENCY` | Rpc getlogs receipt concurrency setting | `10` | No | [indexer/internal/config/config.go][source-4] |
| `RPC_GETLOGS_STALE_THRESHOLD` | Rpc getlogs stale threshold setting | `50` | No | [indexer/internal/config/config.go][source-4] |
| `RPC_GETLOGS_TIMEOUT_MS` | Rpc getlogs timeout ms setting | `5000` | No | [indexer/internal/config/config.go][source-4] |
| `RPC_URLS` | Comma-separated JSON-RPC pool; required for non-production rpc-getlogs mode | Unset | No | [indexer/internal/config/config.go][source-4] |
| `RPC_URL_FALLBACK` | Fallback chain JSON-RPC endpoint | Unset | No | [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts) |
| `RUNTIME_ACTION` | PnL runtime action selector | Unset | No | [pnl-tracker/internal/config/config.go][source-5] |
| `RUNTIME_PROFILE` | Production runtime profile: container or owned_local | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `S3_APPROVED_BUCKET` | S3 approved object storage bucket | Unset | No | [metadata/internal/config/config.go][source-7] |
| `S3_QUARANTINE_BUCKET` | S3 quarantine object storage bucket | Unset | No | [metadata/internal/config/config.go][source-7] |
| `S3_REGION` | Object storage signing region | `auto` | No | [gateway/internal/config/config.go][source-6]; [metadata/internal/config/config.go][source-7]; [trading-charts/cmd/charts-rebuild/main.go][source-16]; [user/internal/config/config.go][source-2] |
| `SECRETS_PROVIDER` | Secret backend; env is the supported production provider | `env` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `SECRETS_REFRESH_SECONDS` | Secret recheck interval in seconds, from 1 through 60 | `30` | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [sdk/launchpad/shared/config/env.go][source-12] (indexer, pnl-tracker, ranking-algo, stats-workers, trading-charts); [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [social/internal/config/config.go][source-9]; [user/internal/config/config.go][source-2] |
| `SOCIAL_AUTHORITY_PEERS` | JSON map of social authority issuers and certificate pins | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_CURSOR_KEY` | Social cursor key used to sign, encrypt or authenticate component data | Unset | Yes | [social/internal/config/config.go][source-9] |
| `SOCIAL_PRIVATE_LISTEN_ADDRESS` | Social private listen address setting | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_DOCUMENT` | Base64-encoded social runtime JSON; alternative to its file input | Unset | May contain secrets | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE` | Path to the social runtime JSON document; alternative to its inline document | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.command.*.Command` | Voice provider capability document: command | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.command.*.ExitCode` | Voice provider capability document: exit code | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.command.*.LogFile` | Voice provider capability document: log file | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.command.*.LogSHA256` | Voice provider capability document: log sha256 | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.endpoint` | Voice provider capability document: endpoint | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.expiresAt` | Voice provider capability document: expires at | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.provider` | Voice provider capability document: provider | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE $.voice.qualificationFile (social): $.version` | Voice provider capability document: version | None | No | [social/internal/app/app.go][source-53] |
| `SOCIAL_RUNTIME_FILE (social): $.accountSchema` | Social runtime: account schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.authorityPeers.*` | Social runtime: authority peers | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.caFile` | Social runtime: ca file | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.caSHA256` | Social runtime: casha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.chatDurable` | Social runtime: chat durable | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.chatStream` | Social runtime: chat stream | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.chatSubject` | Social runtime: chat subject | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.clientCertFile` | Social runtime: client cert file | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.clientCertSHA256` | Social runtime: client cert sha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.clientKeyReference` | Social runtime: client key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.consumerGeneration` | Social runtime: consumer generation | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.cursorKeyReference` | Social runtime: cursor key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.deliverySchema` | Social runtime: delivery schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.erasureDurable` | Social runtime: erasure durable | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.erasureStream` | Social runtime: erasure stream | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.erasureSubject` | Social runtime: erasure subject | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.eventStream` | Social runtime: event stream | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.gatewayCertSHA256` | Social runtime: gateway cert sha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.gatewayDNS` | Social runtime: gateway dns | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.identityEndpoint` | Social runtime: identity endpoint | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.identitySchema` | Social runtime: identity schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.identityServerPeerCertSHA256` | Social runtime: identity server peer cert sha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.identitySigningKeyReference` | Social runtime: identity signing key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.indexerSchema` | Social runtime: indexer schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.interactionSubjects.*` | Social runtime: interaction subjects | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.maxOutboxAgeSeconds` | Social runtime: max outbox age seconds | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.mentionSubject` | Social runtime: mention subject | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.natsCredentialsReference` | Social runtime: nats credentials reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.natsURL` | Social runtime: natsurl | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.notificationEndpoint` | Social runtime: notification endpoint | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.notificationTokenReference` | Social runtime: notification token reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.pendingOutboxLimit` | Social runtime: pending outbox limit | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.policyDurable` | Social runtime: policy durable | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.policyStream` | Social runtime: policy stream | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.policySubject` | Social runtime: policy subject | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.recoveryEpoch` | Social runtime: recovery epoch | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.recoveryRegion` | Social runtime: recovery region | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.recoverySchema` | Social runtime: recovery schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.serverCertFile` | Social runtime: server cert file | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.serverCertSHA256` | Social runtime: server cert sha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.serverKeyReference` | Social runtime: server key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.socialSchema` | Social runtime: social schema | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.version` | Social runtime: version | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.apiKeyReference` | Social runtime: api key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.apiSecretReference` | Social runtime: api secret reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.endpoint` | Social runtime: endpoint | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.policyVersion` | Social runtime: policy version | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.qualificationFile` | Social runtime: qualification file | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.qualificationSHA256` | Social runtime: qualification sha256 | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingAccessKeyReference` | Social runtime: recording access key reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingBucket` | Social runtime: recording bucket | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingEnabled` | Social runtime: recording enabled | `false` | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingEndpoint` | Social runtime: recording endpoint | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingPolicyVersion` | Social runtime: recording policy version | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingPurpose` | Social runtime: recording purpose | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingRegion` | Social runtime: recording region | `auto` | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingRetentionSeconds` | Social runtime: recording retention seconds | None | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_FILE (social): $.voice.recordingSecretReference` | Social runtime: recording secret reference; names an environment secret, whose value is supplied separately | None | No (name only) | [social/internal/config/config.go][source-9] |
| `SOCIAL_RUNTIME_SHA256` | SHA-256 digest binding for social runtime | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_CERT_FILE` | Path to social tls cert material | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_CERT_PEM` | Inline social tls cert PEM material; alternative to the corresponding file | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_CLIENT_CA_FILE` | Path to social tls client ca material | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_CLIENT_CA_PEM` | Inline social tls client ca PEM material; alternative to the corresponding file | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_KEY_FILE` | Path to social tls key material | Unset | No | [social/internal/config/config.go][source-9] |
| `SOCIAL_TLS_KEY_PEM` | Inline social tls key PEM material; alternative to the corresponding file | Unset | Yes | [social/internal/config/config.go][source-9] |
| `SSE_MAX_CONNECTIONS` | Sse max connections setting | `50000` | No | [api/internal/config/config.go][source-1] |
| `SSE_MAX_PER_IP` | Sse max per ip setting | `20` | No | [api/internal/config/config.go][source-1] |
| `STATS_ACTIVE_POOL_WINDOW_HOURS` | Stats active pool window hours setting | `24` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_AUDIT_KEY` | Stats audit key used to sign, encrypt or authenticate component data | Unset | Yes | [stats-workers/internal/config/config.go][source-10] |
| `STATS_CURSOR_KEY` | Stats cursor key used to sign, encrypt or authenticate component data | Unset | Yes | [stats-workers/internal/config/config.go][source-10] |
| `STATS_HOLDER_REFRESH_INTERVAL_MS` | Stats holder refresh interval in milliseconds | `60000` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_LEGACY_BRIDGE_BATCH` | Stats legacy bridge batch setting | `100` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_LEGACY_BRIDGE_INTERVAL_MS` | Stats legacy bridge interval in milliseconds | `1000` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_PUBLIC_STATS_BATCH` | Stats public stats batch setting | `50` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_PUBLIC_STATS_INTERVAL_MS` | Stats public stats interval in milliseconds | `2000` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_PUBLIC_STREAM_SCHEMA` | Stats public stream database schema | `api_stream` | No | [stats-workers/internal/config/config.go][source-10] |
| `STATS_SERVICE_URL` | Stats service endpoint | Unset | No | [api/internal/config/config.go][source-1] |
| `STREAM_CONSUMER_CONFIG_DOCUMENT` | Base64-encoded stream consumer config JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1] |
| `STREAM_CONSUMER_CONFIG_FILE` | Path to the stream consumer config JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].AckWait` | Socket stream consumer: ack wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].Batch` | Socket stream consumer: batch | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].Durable` | Socket stream consumer: durable | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].FetchWait` | Socket stream consumer: fetch wait in nanoseconds | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].MaxAckPending` | Socket stream consumer: max ack pending | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].MaxDeliver` | Socket stream consumer: max deliver | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].MaxWaiting` | Socket stream consumer: max waiting | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].Stream` | Socket stream consumer: stream | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_FILE (api): $[].Subject` | Socket stream consumer: subject | None | No | [sdk/launchpad/shared/delivery/natsdriver/driver.go][source-24] |
| `STREAM_CONSUMER_CONFIG_SHA256` | SHA-256 digest binding for stream consumer config | Unset | No | [api/internal/config/config.go][source-1] |
| `STREAM_RELAY_CONFIG_DOCUMENT` | Base64-encoded stream relay config JSON; alternative to its file input | Unset | May contain secrets | [api/internal/config/config.go][source-1] |
| `STREAM_RELAY_CONFIG_FILE` | Path to the stream relay config JSON document; alternative to its inline document | Unset | No | [api/internal/config/config.go][source-1] |
| `STREAM_RELAY_CONFIG_FILE (api): $.batch` | Rest stream relay: batch | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.burst` | Rest stream relay: burst | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.concurrency` | Rest stream relay: concurrency | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.epoch` | Rest stream relay: epoch | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.eventsPerSecond` | Rest stream relay: events per second | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.idleInterval` | Rest stream relay: idle interval in nanoseconds | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.leaseDuration` | Rest stream relay: lease duration in nanoseconds | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.maxAttempts` | Rest stream relay: max attempts | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.maxLag` | Rest stream relay: max lag in nanoseconds | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.publishTimeout` | Rest stream relay: publish timeout in nanoseconds | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.region` | Rest stream relay: region | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.retryDelay` | Rest stream relay: retry delay in nanoseconds | None | No | [api/internal/app/stream_relay.go][source-52] |
| `STREAM_RELAY_CONFIG_FILE (api): $.routes[].stream` | Rest stream relay: stream | None | No | [sdk/launchpad/shared/delivery/stream.go][source-51] |
| `STREAM_RELAY_CONFIG_FILE (api): $.routes[].subjects[]` | Rest stream relay: subjects | None | No | [sdk/launchpad/shared/delivery/stream.go][source-51] |
| `STREAM_RELAY_CONFIG_SHA256` | SHA-256 digest binding for stream relay config | Unset | No | [api/internal/config/config.go][source-1] |
| `STREAM_SCHEMA` | Stream database schema | `api_stream` | No | [api/internal/config/config.go][source-1] |
| `STREAM_SIGNING_KEY` | Private stream frame signing secret; required in production API roles | Unset | Yes | [api/internal/config/config.go][source-1] |
| `TRUSTED_PROXY_CIDRS` | Comma-separated trusted proxy networks for forwarded client addresses | Unset | No | [api/internal/config/config.go][source-1]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [trading-charts/internal/config/config.go][source-11] |
| `UPSTREAM_TIMEOUT_SECONDS` | Upstream timeout in seconds | `30` | No | [gateway/internal/config/config.go][source-6] |
| `USER_ASSERTION_PUBLIC_KEY_HEX` | User assertion public key hex setting | Unset | No | [metadata/internal/config/config.go][source-7] |
| `USER_BUCKET` | User object storage bucket | Unset | No | [gateway/internal/config/config.go][source-6] |
| `USER_HTTP_URL` | User http endpoint | Unset | No | [gateway/internal/config/config.go][source-6] |
| `USER_PEER_CERT_SHA256` | SHA-256 digest binding for user peer cert | Unset | No | [metadata/internal/config/config.go][source-7] |
| `WEBHOOK_HMAC_SECRETS` | Comma-separated indexer webhook secrets aligned with WEBHOOK_URLS | Unset | Yes | [indexer/internal/config/config.go][source-4] |
| `WEBHOOK_URLS` | Comma-separated legacy indexer webhook destinations | Unset | No | [indexer/internal/config/config.go][source-4] |
| `WS_MAX_CONNECTIONS` | Ws max connections setting | api: `50000`; trading-charts: `10000` | No | [api/internal/config/config.go][source-1]; [trading-charts/internal/config/config.go][source-11] |
| `WS_MAX_PER_IP` | Ws max per ip setting | `20` | No | [api/internal/config/config.go][source-1]; [trading-charts/internal/config/config.go][source-11] |
| `WS_READ_LIMIT_BYTES` | Ws read limit in bytes | `4096` | No | [social/internal/config/config.go][source-9] |
| `WS_SEND_BUFFER` | Ws send buffer setting | `64` | No | [social/internal/config/config.go][source-9] |
| `WS_WRITE_TIMEOUT_SECONDS` | Ws write timeout in seconds | `10` | No | [social/internal/config/config.go][source-9] |
| `ZONE` | Runtime availability zone; required in production | Unset | No | [api/internal/config/config.go][source-1]; [gateway/internal/config/config.go][source-6]; [indexer/internal/config/config.go][source-4]; [livestream/internal/config/config.go][source-3]; [metadata/internal/config/config.go][source-7]; [pnl-tracker/internal/config/config.go][source-5]; [ranking-algo/internal/config/config.go][source-8]; [social/internal/config/config.go][source-9]; [stats-workers/internal/config/config.go][source-10]; [trading-charts/internal/config/config.go][source-11]; [user/internal/config/config.go][source-2] |
| `indexer/sqlc.yaml: $.sql[].engine` | Database engine | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.emit_empty_slices` | Whether empty result collections use allocated slices | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.emit_exact_table_names` | Whether Go types preserve exact table names | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.emit_interface` | Whether to generate the query interface | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.out` | Generated Go output directory | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.overrides[].db_type` | PostgreSQL type to override | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.import` | Import path for the replacement Go type | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.pointer` | Whether the replacement type is a pointer | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.type` | Replacement Go type | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.overrides[].nullable` | Whether the override applies to nullable columns | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.package` | Generated Go package name | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].gen.go.sql_package` | Go SQL driver package | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].queries` | Query source path | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.sql[].schema[]` | Migration and external schema inputs | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `indexer/sqlc.yaml: $.version` | Configuration format version | Explicit in file | No | [indexer/sqlc.yaml][source-55] |
| `livestream/sqlc.yaml: $.sql[].engine` | Database engine | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.emit_empty_slices` | Whether empty result collections use allocated slices | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.emit_exact_table_names` | Whether Go types preserve exact table names | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.emit_interface` | Whether to generate the query interface | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.out` | Generated Go output directory | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.overrides[].db_type` | PostgreSQL type to override | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.pointer` | Whether the replacement type is a pointer | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.type` | Replacement Go type | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.overrides[].nullable` | Whether the override applies to nullable columns | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.package` | Generated Go package name | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].gen.go.sql_package` | Go SQL driver package | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].queries` | Query source path | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.sql[].schema[]` | Migration and external schema inputs | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `livestream/sqlc.yaml: $.version` | Configuration format version | Explicit in file | No | [livestream/sqlc.yaml][source-56] |
| `metadata/sqlc.yaml: $.sql[].engine` | Database engine | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.emit_empty_slices` | Whether empty result collections use allocated slices | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.emit_exact_table_names` | Whether Go types preserve exact table names | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.emit_interface` | Whether to generate the query interface | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.out` | Generated Go output directory | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.overrides[].db_type` | PostgreSQL type to override | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.pointer` | Whether the replacement type is a pointer | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.type` | Replacement Go type | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.overrides[].nullable` | Whether the override applies to nullable columns | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.package` | Generated Go package name | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].gen.go.sql_package` | Go SQL driver package | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].queries` | Query source path | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.sql[].schema[]` | Migration and external schema inputs | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `metadata/sqlc.yaml: $.version` | Configuration format version | Explicit in file | No | [metadata/sqlc.yaml][source-57] |
| `social/sqlc.yaml: $.sql[].engine` | Database engine | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.emit_empty_slices` | Whether empty result collections use allocated slices | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.emit_exact_table_names` | Whether Go types preserve exact table names | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.emit_interface` | Whether to generate the query interface | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.out` | Generated Go output directory | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.overrides[].db_type` | PostgreSQL type to override | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.pointer` | Whether the replacement type is a pointer | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.type` | Replacement Go type | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.overrides[].nullable` | Whether the override applies to nullable columns | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.package` | Generated Go package name | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].gen.go.sql_package` | Go SQL driver package | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].queries` | Query source path | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.sql[].schema[]` | Migration and external schema inputs | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `social/sqlc.yaml: $.version` | Configuration format version | Explicit in file | No | [social/sqlc.yaml][source-58] |
| `user/sqlc.yaml: $.sql[].engine` | Database engine | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.emit_empty_slices` | Whether empty result collections use allocated slices | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.emit_exact_table_names` | Whether Go types preserve exact table names | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.emit_interface` | Whether to generate the query interface | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.out` | Generated Go output directory | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.overrides[].db_type` | PostgreSQL type to override | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.pointer` | Whether the replacement type is a pointer | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.overrides[].go_type.type` | Replacement Go type | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.overrides[].nullable` | Whether the override applies to nullable columns | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.package` | Generated Go package name | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].gen.go.sql_package` | Go SQL driver package | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].queries` | Query source path | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.sql[].schema[]` | Migration and external schema inputs | Explicit in file | No | [user/sqlc.yaml][source-59] |
| `user/sqlc.yaml: $.version` | Configuration format version | Explicit in file | No | [user/sqlc.yaml][source-59] |

[source-1]: ../../../services/launchpad/api/internal/config/config.go
[source-2]: ../../../services/launchpad/user/internal/config/config.go
[source-3]: ../../../services/launchpad/livestream/internal/config/config.go
[source-4]: ../../../services/launchpad/indexer/internal/config/config.go
[source-5]: ../../../services/launchpad/pnl-tracker/internal/config/config.go
[source-6]: ../../../services/launchpad/gateway/internal/config/config.go
[source-7]: ../../../services/launchpad/metadata/internal/config/config.go
[source-8]: ../../../services/launchpad/ranking-algo/internal/config/config.go
[source-9]: ../../../services/launchpad/social/internal/config/config.go
[source-10]: ../../../services/launchpad/stats-workers/internal/config/config.go
[source-11]: ../../../services/launchpad/trading-charts/internal/config/config.go
[source-12]: ../../../sdk/launchpad/shared/config/env.go
[source-13]: ../../../services/launchpad/security/supplychain/controller.py
[source-14]: ../../../services/launchpad/Makefile
[source-15]: ../../../services/launchpad/security/supplychain/ci_tools.py
[source-16]: ../../../services/launchpad/trading-charts/cmd/charts-rebuild/main.go
[source-17]: ../../../services/launchpad/api/internal/store/accounting_readmodel.go
[source-18]: ../../../services/launchpad/user/internal/app/account_runtime.go
[source-19]: ../../../sdk/launchpad/shared/db/production/allocation.go
[source-20]: ../../../services/launchpad/indexer/internal/app/app.go
[source-21]: ../../../services/launchpad/indexer/internal/app/render.go
[source-22]: ../../../sdk/launchpad/shared/delivery/envelope.go
[source-23]: ../../../services/launchpad/pnl-tracker/internal/app/app.go
[source-24]: ../../../sdk/launchpad/shared/delivery/natsdriver/driver.go
[source-25]: ../../../services/launchpad/stats-workers/internal/app/app.go
[source-26]: ../../../services/launchpad/stats-workers/internal/search/projection.go
[source-27]: ../../../services/launchpad/trading-charts/internal/app/app.go
[source-28]: ../../../services/launchpad/api/internal/app/app.go
[source-29]: ../../../sdk/launchpad/shared/secrets/manager.go
[source-30]: ../../../services/launchpad/notifications/runtime.go
[source-31]: ../../../sdk/launchpad/shared/db/production/migrations.go
[source-32]: ../../../sdk/launchpad/shared/db/production/roles.go
[source-33]: ../../../services/launchpad/gateway/internal/app/app.go
[source-34]: ../../../services/launchpad/gateway/internal/proxy/connected.go
[source-35]: ../../../services/launchpad/gateway/internal/auth/identity_tokens.go
[source-36]: ../../../services/launchpad/indexer/internal/source/committed.go
[source-37]: ../../../services/launchpad/indexer/internal/source/explorer_committed.go
[source-38]: ../../../services/launchpad/indexer/internal/source/parity.go
[source-39]: ../../../services/launchpad/indexer/internal/processor/quarantine.go
[source-40]: ../../../sdk/launchpad/shared/delivery/webhook.go
[source-41]: ../../../services/launchpad/indexer/internal/store/replay.go
[source-42]: ../../../services/launchpad/indexer/internal/store/quarantine.go
[source-43]: ../../../sdk/launchpad/shared/config/deployment/manifest.go
[source-44]: ../../../services/launchpad/indexer/internal/receipt/rpc.go
[source-45]: ../../../services/launchpad/indexer/internal/decode/registry.go
[source-46]: ../../../services/launchpad/indexer/internal/receipt/identity.go
[source-47]: ../../../services/launchpad/livestream/internal/streams/usage.go
[source-48]: ../../../services/launchpad/api/internal/app/authority.go
[source-49]: ../../../sdk/launchpad/shared/config/deployment/public.go
[source-50]: ../../../services/launchpad/pnl-tracker/internal/production/l18/authority.go
[source-51]: ../../../sdk/launchpad/shared/delivery/stream.go
[source-52]: ../../../services/launchpad/api/internal/app/stream_relay.go
[source-53]: ../../../services/launchpad/social/internal/app/app.go
[source-54]: ../../../services/launchpad/trading-charts/internal/store/rebuild.go
[source-55]: ../../../services/launchpad/indexer/sqlc.yaml
[source-56]: ../../../services/launchpad/livestream/sqlc.yaml
[source-57]: ../../../services/launchpad/metadata/sqlc.yaml
[source-58]: ../../../services/launchpad/social/sqlc.yaml
[source-59]: ../../../services/launchpad/user/sqlc.yaml
[source-60]: ../../../services/launchpad/security/supplychain/scanner.py
