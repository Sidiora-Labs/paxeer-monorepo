# Paxeer X wallet workspace

This directory holds the Paxeer X wallet gateway, threshold-custody attestors, key ceremony tools, API schemas, deployment configuration and reference web app. Its pnpm workspace also includes the wallet SDK and perpetuals app. The gateway authenticates wallet users and agents, applies signing policies and submits transactions to Paxeer mainnet-beta, one BFT network with EVM chain ID 125. The kernel is part of the network and proposes blocks; a transaction receipt is final when its block is committed.

## Layout

| Path | Contents |
| --- | --- |
| `attestor/` | Go module: threshold attestor server in `cmd/attestor`, load generator in `cmd/loadtest` and key dealer in `dealer` |
| `ceremony/` | Go module: the `ceremony` command with `plan`, `deliver`, `archive`, `rehearse` and `move` operations; `legacy-import/` holds the database import CLI |
| `demo/` | `@paxeer/demo`, the reference web app built on the wallet SDK |
| `deploy/` | Attestor, gateway, endpoint and human service configuration, custody deployment material and the environment variable reference `env` |
| `gateway/` | `@paxeer/api`, the HTTP wallet gateway with `/v1/wallet`, `/v1/agent` and `/v1/agents` routes, migrations in `migrations/`, tests in `test/` and a load test in `loadtest/` |
| `schema/` | Attestor API contract in `attestor-api/v1.kvx` and golden examples in `attestor-api/golden/` |
| `package.json` | Workspace scripts, package manager and Node.js requirement |
| `pnpm-workspace.yaml` | Workspace package membership and dependency build settings |
| `pnpm-lock.yaml` | Pinned pnpm dependency resolution |
| `tsconfig.base.json` | Compiler options shared by the wallet SDK, gateway and demo |
| `.gitignore` | Dependency, build, coverage and compiler output exclusions |

The workspace also includes packages outside this directory:

| Path | Package | Contents |
| --- | --- | --- |
| [../../sdk/wallet](../../sdk/wallet) | `@paxeer/wallet` | Browser and React SDK for the wallet gateway, with tests in `test/` |
| [../../apps/perps](../../apps/perps) | `builders-workshop` | Perpetuals web app |

## Build and test

Requires pnpm 10.34.9 and Node.js 22 or newer, as declared in `package.json`.

```sh
cd services/wallet
pnpm install
pnpm build
pnpm test
```

The workspace scripts in `package.json` run `pnpm -r build` and `pnpm -r test` across the member packages. To build the gateway and wallet SDK separately and run the workspace tests:

```sh
pnpm -r --filter ./gateway --filter ../../sdk/wallet build
pnpm -r test
```

The gateway suites load `gateway/test/setup.ts`, which supplies their initial environment configuration. Integration suites configure their service dependencies.

The Go modules use the Go version declared in their respective `go.mod` files. The commands in `scripts/wallet/gate-test.sh` compile and test each module; run `go test ./...` from `services/wallet/attestor` and again from `services/wallet/ceremony`.

The import CLI has its own `ceremony/legacy-import/package.json` and lockfile. From `services/wallet/ceremony/legacy-import`, run `pnpm install`, `pnpm build` and `pnpm test`.

From the repository root, `scripts/wallet/scan-secrets.sh services/wallet` scans the workspace for key-shaped, token-shaped and credentialed connection-string content and exits non-zero on a match.

## Configuration

Runtime configuration is read from environment variables. `deploy/` contains example environment files, including `railway.env.example` and the attestor environment examples. The gateway validates its variables at startup in `gateway/src/env.ts`. The service environment reference is `deploy/env`. Variable names below have no values.

Gateway:

- `NODE_ENV`, `PORT`, `LOG_LEVEL`, `API_WORKERS`, `CORS_ORIGINS`, `LAYERX_TRUSTED_PROXIES`
- `SUPABASE_URL`, `WALLET_IDENTITY_BINDING_TENANT`, `WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE`
- `DATABASE_URL`, `DATABASE_POOL_MAX`
- `WALLET_MASTER_KEY`, `WALLET_MASTER_KEY_VERSION`
- `HYPERPAXEER_CHAIN_ID`, `HYPERPAXEER_RPC_URL`, `HYPERPAXEER_EXPLORER_URL`
- `RPC_URLS`, `SEQUENCER_RPC_URL`, `RPC_LAG_THRESHOLD_BLOCKS`, `RPC_HEALTH_INTERVAL_MS`, `RPC_TIMEOUT_MS`
- `POLICY_MAX_TX_VALUE_WEI`, `POLICY_MAX_DAILY_VALUE_WEI`, `POLICY_RATE_LIMIT_PER_MINUTE`
- `RATE_LIMIT_CLIENT_PER_MINUTE`, `RATE_LIMIT_ACCOUNT_PER_MINUTE`
- `AGENT_JWT_SECRET`, `AGENT_TOKEN_TTL_SECONDS`, `AGENT_CHALLENGE_TTL_SECONDS`, `AGENT_REQUEST_MAX_TTL_SECONDS`, `AGENT_DEFAULT_FROZEN`, `AGENT_DEFAULT_MODE`, `AGENT_DEFAULT_MAX_TX_VALUE_WEI`, `AGENT_DEFAULT_MAX_DAILY_VALUE_WEI`, `AGENT_DEFAULT_RATE_LIMIT_PER_MINUTE`, `AGENT_DEFAULT_MAX_APPROVE_WEI`
- `ACTION_CONFIRMATIONS`, `ACTION_RECEIPT_TIMEOUT_MS`, `ACTION_WORKER_INTERVAL_MS`, `ACTION_RECONCILE_INTERVAL_MS`, `ACTION_FEE_BUMP_PERCENT`
- `WALLET_CUSTODY_INVENTORY_FILE`, `WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE`
- `ATTESTOR_ENDPOINTS`, `ATTESTOR_CLIENT_CERT_FILE`, `ATTESTOR_CLIENT_KEY_FILE`, `ATTESTOR_CA_FILE`, `ATTESTOR_QUORUM`, `ATTESTOR_HEALTH_INTERVAL_MS`, `ATTESTOR_TIMEOUT_MS`
- `WALLET_AUTHORIZATION_DELEGATES`, `WALLET_GAS_STATION_URL`, `ACCOUNT_SETUP_GAS_CAP_WEI`
- `PAXEER_GAS_STATION_PAYMASTER`, `PAXEER_GAS_STATION_GAS_LIMIT`, `PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT`
- `GATEWAY_ADMIN_TOKEN`

Gateway file inputs include the identity binding private key, signed custody inventory and its verification key, and the attestor TLS certificate, key and CA bundle named by the corresponding file variables. Database migrations are loaded from `gateway/migrations/`.

Attestors, read by `attestor/internal/config/config.go` and `attestor/internal/replica/config.go`:

- `ATTESTOR_NODE_ID`, `ATTESTOR_REGION`, `ATTESTOR_LISTEN_ADDR`, `ATTESTOR_PEER_LISTEN_ADDR`, `ATTESTOR_PEERS`, `ATTESTOR_PEER_PINS`
- `ATTESTOR_NODE_KEY_FILE`, `ATTESTOR_DATA_DIR`, `ATTESTOR_CEREMONY`, `ATTESTOR_CHAIN_ID`, `ATTESTOR_RPC_URL`
- `ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE`, `ATTESTOR_AUTHORITY_ISSUER`, `ATTESTOR_AUTHORITY_TENANT`, `ATTESTOR_INVENTORY_PUBLIC_KEY_FILE`, `ATTESTOR_INVENTORY_FILE`
- `ATTESTOR_JWKS_URL`, `ATTESTOR_JWT_ISSUER`, `ATTESTOR_JWT_AUDIENCE`, `ATTESTOR_JWT_MAX_AGE`
- `ATTESTOR_POLICY_FILE`, `ATTESTOR_KERNEL_POLICY_FILE`, `ATTESTOR_NATIVE_POLICY_FILE`, `ATTESTOR_ACTIVITY_TYPES`
- `ATTESTOR_TLS_CERT_FILE`, `ATTESTOR_TLS_KEY_FILE`, `ATTESTOR_TLS_CA_FILE`, `ATTESTOR_OPERATOR_CA_FILE`
- `ATTESTOR_AGENTS_FILE`, `ATTESTOR_AGENT_MAX_EXPIRY`, `ATTESTOR_AGENT_PAYMASTER`
- `ATTESTOR_BACKUP_KEY_FILE`, `ATTESTOR_BACKUP_DIR`, `ATTESTOR_SNAPSHOT_INTERVAL`, `ATTESTOR_SNAPSHOT_RETAIN`
- `ATTESTOR_REPLICA_ADDR`, `ATTESTOR_REPLICA_USER`, `ATTESTOR_REPLICA_KEY_FILE`, `ATTESTOR_REPLICA_HOST_KEY_FILE`, `ATTESTOR_REPLICA_REMOTE_DIR`, `ATTESTOR_REPLICA_INTERVAL`, `ATTESTOR_REPLICA_OPERATION_TIMEOUT`

Attestors load policy JSON, signed authority and inventory documents, agent configuration, keys and TLS material from the configured files. `deploy/attestor-policy.json` and `deploy/attestor-kernel-policy.json` provide policy documents; `deploy/attestor-policy.md` describes signing policy.

Ceremony tools, read by the loaders under `ceremony/internal/`:

- `CEREMONY_SOURCE_DATABASE_URL`, `CEREMONY_DATABASE_URL`, `CEREMONY_GATEWAY_MIGRATIONS_DIR`
- `CEREMONY_ARCHIVE_PATH`, `CEREMONY_ARCHIVE_PASSPHRASE`, `CEREMONY_MASTER_KEY`
- `CEREMONY_ID`, `CEREMONY_JOURNAL_DIR`, `CEREMONY_JOURNAL_KEY_FILE`, `CEREMONY_REHEARSAL_RECEIPT`, `CEREMONY_RPC_URL`
- `CEREMONY_NODES`, `CEREMONY_NODE_PINS`, `CEREMONY_TLS_CERT_FILE`, `CEREMONY_TLS_KEY_FILE`, `CEREMONY_TLS_CA_FILE`
- `CEREMONY_GATEWAY_TLS_CERT_FILE`, `CEREMONY_GATEWAY_TLS_KEY_FILE`, `CEREMONY_OWNER_TOKENS_FILE`
- `CEREMONY_REHEARSAL_ADMIN_URL`, `CEREMONY_ATTESTOR_BIN`, `CEREMONY_REHEARSAL_JOURNAL_DIR`, `CEREMONY_REHEARSAL_RPC_URL`

Rehearsal reads a separate set of attestor client settings by replacing the `CEREMONY_` prefix with `CEREMONY_REHEARSAL_` for the node, pin, TLS and owner-token variables. File inputs include the journal key, rehearsal receipt, archive, owner tokens and TLS material. The import CLI reads `LEGACY_DATABASE_URL`, `GATEWAY_ADMIN_URL` and `GATEWAY_ADMIN_TOKEN` in `ceremony/legacy-import/src/cli.ts`.

Demo:

- `NEXT_PUBLIC_PAXEER_WALLET_API`
- `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`
- `NEXT_PUBLIC_AUTH_REDIRECT_URL`
- `NEXT_PUBLIC_SITE_URL`

The demo accepts environment configuration through Next.js, including `demo/.env.local`; see its [configuration guide](demo/README.md).

Load generators:

- `attestor/cmd/loadtest` reads `LOADTEST_ENDPOINTS`, `LOADTEST_CLIENT_CERT_FILE`, `LOADTEST_CLIENT_KEY_FILE`, `LOADTEST_CA_FILE`, `LOADTEST_CHAIN_ID`, `LOADTEST_TOKEN`, `LOADTEST_DURATION`, `LOADTEST_CONCURRENCY`, `LOADTEST_SESSION_TIMEOUT` and `LOADTEST_KEYGEN_TIMEOUT`.
- `gateway/loadtest/wallet.js` reads `API_BASE` and `PAXEER_JWT`.

## Documentation

- [Custody cutover and ceremony commands](../../docs/wallet/service/cutover.md)
- [Attestor security boundaries](../../docs/wallet/service/security-review.md)
- [Backup and restore](../../docs/wallet/service/runbooks/backup-restore.md)
- [Node replacement](../../docs/wallet/service/runbooks/node-replacement.md)
- [Share refresh](../../docs/wallet/service/runbooks/refresh-cadence.md)
- [Gateway rollback](../../docs/wallet/service/runbooks/gateway-rollback.md)
- [Identity provider outage](../../docs/wallet/service/runbooks/identity-provider-outage.md)
- [RPC pool operations](../../docs/wallet/service/runbooks/rpc-pool-failure.md)
- [Documentation site: account funding and key custody](../../docs/site/paxeer-docs/public/markdown/custody.md)
- [Documentation site: public JSON-RPC](../../docs/site/paxeer-docs/public/markdown/public-rpc.md)
- [Reference demo](demo/README.md)
- [Gateway load test](gateway/loadtest/README.md)
