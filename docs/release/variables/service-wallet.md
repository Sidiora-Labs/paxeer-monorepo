# Wallet service variables

The Paxeer wallet system in `services/wallet` contains the wallet gateway, attestor, ceremony and legacy-import tools, browser demo, and deployment templates and cutover scripts. It connects to the merged Paxeer mainnet-beta network with EVM chain ID 125; transaction receipts are final when their blocks commit. `SEQUENCER_RPC_URL` selects a node RPC endpoint used by the gateway. Source paths below are relative to `services/wallet` unless they begin with `docker/`, which is relative to the repository root.

Required entries must be supplied for the named component or command; components use separate environments. Optional entries state their feature dependencies. File settings are named as `file / section / key`; environment-named files are selected by that variable. In policy sections, `*` denotes a map entry and `[]` an array entry. Account rules inherit default rules when omitted. Secret-file paths and public verification material are not secret values; their private file contents must remain protected. Container file inputs are base64 encoded and consumed before the wallet process starts.

The root `.env.example` lists the runtime inputs, including the attestor load generator command. Component examples live in `demo`, `ceremony`, `ceremony/legacy-import`, `deploy`, and `deploy/cutover`; `deploy/railway.env.example`, the five attestor examples, and `deploy/redis.railway.env.example` include their container file inputs. Examples leave required operator values empty and optional unset values commented. Keep those assignments commented to apply source defaults, including the public node RPC default. Set the six required replication fields together before enabling the replication timeout. Public identity-provider browser keys are public values. Supply file keys, pins, addresses and credentials through deployment configuration.

## Required

| Name | Sets | Source default | Secret | Reader |
| --- | --- | --- | --- | --- |
| `ATTESTOR_AUTHORITY_ISSUER` | Expected custody producer authority issuer | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE` | Path to pinned custody producer authority public key | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_AUTHORITY_TENANT` | Expected custody producer authority tenant | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_BACKUP_KEY` | Base64 file input materialized at /run/secrets/backup.key | None | Yes | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_CA` | Base64 file input materialized at /run/secrets/attestor_ca.crt | None | No | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_CHAIN_ID` | Required EVM chain identifier | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_CLIENT_CERT` | Base64 file input materialized at /run/secrets/attestor_client.crt | None | No | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_CLIENT_KEY` | Base64 file input materialized at /run/secrets/attestor_client.key | None | Yes | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_DATA_DIR` | Encrypted share-store directory | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_INVENTORY_FILE` | Path to signed approved custody inventory | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_INVENTORY_PUBLIC_KEY_FILE` | Path to approved inventory verification public key | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / modules` | Allowed module names mapped to operation numbers | None | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / root / defaults` | Base rules applied before account overrides | None | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / root / version` | Policy schema version; required when the file is supplied | None (must be 1) | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_LISTEN_ADDR` | API listener socket address | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / root / rules` | Nonempty owner-purpose authorization rule list | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / root / version` | Owner-purpose policy version | None (must be 2) | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / agent_did` | Agent identity | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / capabilities` | Allowed capability identifiers | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / key_id` | Custody key identifier | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / maximum_validity_ms` | Positive maximum purpose validity in milliseconds | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / operations` | Allowed owner-purpose operations | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / owner` | Owner subject | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / owner_public_key` | Owner Ed25519 verification public key | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / rate_per_minute` | Positive maximum signing requests per minute | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NATIVE_POLICY_FILE / rules[] / tenant` | Tenant identifier | None | No | `attestor/internal/policy/native/native.go` |
| `ATTESTOR_NODE_ID` | Participant identifier of this attestor | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_NODE_KEY` | Base64 file input materialized at /run/secrets/node.key | None | Yes | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_NODE_KEY_FILE` | Path to 32-byte share-store encryption key, raw or hex | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_OPERATOR_CA` | Base64 file input materialized at /run/secrets/operator-ca.crt | None | No | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_OPERATOR_CA_FILE` | Path to distinct operator client CA bundle | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_PEERS` | Comma-separated identifier and socket pairs for the other four participants | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_PEER_LISTEN_ADDR` | Peer transport listener socket address | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_PEER_PINS` | Comma-separated identifier and SHA-256 SPKI pin pairs for the other four participants | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_POLICY_FILE / defaults / chain_id` | EVM chain identifier | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / kinds` | Admitted signing kinds | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / rate_per_minute` | Signing rate limit | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / root / defaults` | Base rules applied before account overrides | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / root / version` | Policy schema version; required when the file is supplied | None (must be 1) | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_REPLICA_HOST_KEY` | Base64 file input materialized at /run/secrets/replica-host.pub | None | No | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_REPLICA_KEY` | Base64 file input materialized at /run/secrets/replica.key | None | Yes | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_TLS_CA` | Base64 file input materialized at /run/secrets/ca.crt | None | No | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_TLS_CA_FILE` | Path to peer and gateway client CA bundle | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_TLS_CERT` | Base64 file input materialized at /run/secrets/tls.crt | None | No | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_TLS_CERT_FILE` | Path to API and peer TLS certificate | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_TLS_KEY` | Base64 file input materialized at /run/secrets/tls.key | None | Yes | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `ATTESTOR_TLS_KEY_FILE` | Path to API and peer TLS private key | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `CEREMONY_ARCHIVE_PASSPHRASE` | Archive encryption passphrase, at least 16 bytes; required for archive, delivery and rehearsal | None | Yes | `ceremony/internal/archive/archive.go` |
| `CEREMONY_ARCHIVE_PATH` | Archive output path; required for archive, delivery and rehearsal | None | No | `ceremony/internal/archive/archive.go` |
| `CEREMONY_DATABASE_URL` | PostgreSQL database used by plan, archive, delivery and move commands | None | Yes | `ceremony/internal/migrate/migrate.go` |
| `CEREMONY_GATEWAY_MIGRATIONS_DIR` | Gateway SQL migration directory; required for rehearsal and move | None | No | `ceremony/internal/rehearsal/schema.go`; `ceremony/internal/rehearsal/rehearsal.go`; `ceremony/internal/move/move.go` |
| `CEREMONY_GATEWAY_TLS_CERT_FILE` | Path to separate gateway-role client TLS certificate; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_GATEWAY_TLS_KEY_FILE` | Path to gateway-role TLS private key; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_ID` | Ceremony identifier; required for delivery and rehearsal | None | No | `ceremony/internal/migrate/migrate.go` |
| `CEREMONY_JOURNAL_DIR` | Absolute private journal directory; required for delivery and rehearsal | None | No | `ceremony/internal/migrate/migrate.go` |
| `CEREMONY_JOURNAL_KEY_FILE` | Path to 32-byte journal encryption key in an owned mode-0600 file; required for delivery and rehearsal | None | No | `ceremony/internal/migrate/migrate.go` |
| `CEREMONY_MASTER_KEY` | Base64 32-byte legacy envelope decryption key; required for delivery and rehearsal | None | Yes | `ceremony/internal/envelope/envelope.go` |
| `CEREMONY_NODES` | Exactly five HTTPS node endpoints as identifier and URL pairs; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_NODE_PINS` | Matching node identifier and SHA-256 certificate pin pairs; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_OWNER_TOKENS_FILE` | Path to private owner-subject to JWT JSON map; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_OWNER_TOKENS_FILE / root / *` | Owner-subject names map to bearer JWTs; owned regular mode-0600 JSON file | None | Yes | `ceremony/internal/attestor/client.go` |
| `CEREMONY_REHEARSAL_ADMIN_URL` | Isolated rehearsal PostgreSQL admin URL; required for rehearsal | None | Yes | `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_GATEWAY_TLS_CERT_FILE` | Path to separate gateway-role client TLS certificate; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_GATEWAY_TLS_KEY_FILE` | Path to gateway-role TLS private key; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_JOURNAL_DIR` | Separate private rehearsal journal directory; required for rehearsal | None | No | `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_NODES` | Exactly five HTTPS node endpoints as identifier and URL pairs; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_NODE_PINS` | Matching node identifier and SHA-256 certificate pin pairs; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_OWNER_TOKENS_FILE` | Path to private owner-subject to JWT JSON map; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_OWNER_TOKENS_FILE / root / *` | Owner-subject names map to bearer JWTs; owned regular mode-0600 JSON file | None | Yes | `ceremony/internal/attestor/client.go` |
| `CEREMONY_REHEARSAL_RECEIPT` | Absolute path to rehearsal receipt; required for delivery and rehearsal | None | No | `ceremony/internal/migrate/migrate.go` |
| `CEREMONY_REHEARSAL_RPC_URL` | Isolated rehearsal node RPC endpoint; required for rehearsal | None | No | `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_TLS_CA_FILE` | Path to node TLS CA bundle; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_TLS_CERT_FILE` | Path to operator TLS certificate; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_REHEARSAL_TLS_KEY_FILE` | Path to operator TLS private key; required for rehearsal | None | No | `ceremony/internal/attestor/client.go`; `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_SOURCE_DATABASE_URL` | Source PostgreSQL database for rehearsal and move | None | Yes | `ceremony/internal/rehearsal/source.go`; `ceremony/internal/rehearsal/rehearsal.go`; `ceremony/internal/move/move.go` |
| `CEREMONY_TLS_CA_FILE` | Path to node TLS CA bundle; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_TLS_CERT_FILE` | Path to operator TLS certificate; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CEREMONY_TLS_KEY_FILE` | Path to operator TLS private key; required for delivery; gateway-role certificate, key and token file are configured together | None | No | `ceremony/internal/attestor/client.go` |
| `CORS_ORIGINS` | Comma-separated allowed browser origins; required at gateway startup | None | No | `gateway/src/env.ts` |
| `CUTOVER_CADDYFILE` | Host Caddyfile path, absolute or relative to compose directory; required for apply | None | No | `deploy/cutover/apply.sh` |
| `CUTOVER_CADDYFILE_IN_CONTAINER` | Proxy-container Caddyfile path; required for apply | None | No | `deploy/cutover/apply.sh` |
| `CUTOVER_COMPOSE_DIR` | Existing compose project directory; required for preflight/apply and compose retirement; mutually exclusive with container retirement | None | No | `deploy/cutover/apply.sh`; `deploy/cutover/preflight.sh`; `deploy/cutover/retire.sh`; `deploy/cutover/lib.sh` |
| `CUTOVER_CONTAINER` | Standalone container name; alternative to compose retirement | None | No | `deploy/cutover/retire.sh` |
| `CUTOVER_GATEWAY_HOST` | Bare gateway DNS hostname for preflight and apply | None | No | `deploy/cutover/apply.sh`; `deploy/cutover/preflight.sh` |
| `CUTOVER_PROXY_SERVICE` | Compose proxy service name; required for preflight/apply and compose retirement | None | No | `deploy/cutover/apply.sh`; `deploy/cutover/preflight.sh`; `deploy/cutover/retire.sh` |
| `CUTOVER_PUBLIC_HOST` | Bare public wallet DNS hostname for preflight, apply and retire | None | No | `deploy/cutover/apply.sh`; `deploy/cutover/preflight.sh`; `deploy/cutover/retire.sh` |
| `CUTOVER_RETIRE_SERVICES` | Comma-separated compose service names to retire; required for compose retirement | None | No | `deploy/cutover/retire.sh` |
| `CUTOVER_SITE_LABEL` | Exact wallet site label in the Caddyfile; required for apply | None | No | `deploy/cutover/apply.sh` |
| `CUTOVER_SOAK_SECONDS` | Positive elapsed serving time before retirement, in seconds | None | No | `deploy/cutover/retire.sh` |
| `CUTOVER_STATE_DIR` | Directory for proxy backups and applied state; required for apply and retire | None | No | `deploy/cutover/apply.sh`; `deploy/cutover/retire.sh` |
| `DATABASE_URL` | Gateway PostgreSQL connection string including credentials | None | Yes | `gateway/src/env.ts` |
| `GATEWAY_ADMIN_TOKEN` | Admin-route bearer token, at least 32 characters; required for legacy-import apply; Legacy-import apply bearer token | None | Yes | `gateway/src/env.ts`; `ceremony/legacy-import/src/cli.ts` |
| `GATEWAY_ADMIN_URL` | Gateway base URL; required for legacy-import apply | None | No | `ceremony/legacy-import/src/cli.ts` |
| `HUMAN_EVENTS_APPROVAL_TOKEN` | Deployment file secret mounted at /run/secrets/events-approval-token | None | Yes | `deploy/human.toml [[files]]` |
| `HUMAN_EVENTS_JOURNEY_TOKEN` | Deployment file secret mounted at /run/secrets/events-journey-token | None | Yes | `deploy/human.toml [[files]]` |
| `HUMAN_EVENTS_WEBHOOKS_TOKEN` | Deployment file secret mounted at /run/secrets/events-webhooks-token | None | Yes | `deploy/human.toml [[files]]` |
| `LAYERX_EVENTS_APPROVAL_UPSTREAM_URL` | Required deployment-template event producer upstream URL | None | No | `deploy/human.toml [env]` |
| `LAYERX_EVENTS_JOURNEY_UPSTREAM_URL` | Required deployment-template event producer upstream URL | None | No | `deploy/human.toml [env]` |
| `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_URL` | Required deployment-template event producer upstream URL | None | No | `deploy/human.toml [env]` |
| `LAYERX_HUMAN_WEB_ORIGIN` | Required deployment-template browser origin | None | No | `deploy/human.toml [env]` |
| `LAYERX_INTERNAL_CA_ROOT` | Deployment file secret mounted at /etc/layerx/trust/ca.crt | None | Yes | `deploy/human.toml [[files]]` |
| `LAYERX_ROLE` | Redis role; also selects rows in container secret-file tables | None | No | `docker/redis/entrypoint.sh`; `docker/common/env-files.sh` |
| `LEGACY_DATABASE_URL` | Read-only legacy PostgreSQL connection string for legacy-import | None | Yes | `ceremony/legacy-import/src/cli.ts` |
| `LOADTEST_CA_FILE` | Path to attestor TLS CA bundle; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_CHAIN_ID` | Target EVM chain identifier; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_CLIENT_CERT_FILE` | Path to load generator client TLS certificate; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_CLIENT_KEY_FILE` | Path to load generator client TLS private key; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_CONCURRENCY` | Positive number of concurrent signing sessions; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_DURATION` | Positive run length as a Go duration; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_ENDPOINTS` | Five attestor identifier and socket pairs; command-line settings override environment values | None | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_TOKEN` | Owner bearer JWT used for signing sessions; command-line settings override environment values | None | Yes | `attestor/cmd/loadtest/main.go` |
| `NAME` | Required deployment-template event producer upstream URL | None | No | `deploy/human.toml [env]` |
| `NEXT_PUBLIC_PAXEER_WALLET_API` | Demo wallet API base URL | None | No | `demo/src/lib/paxeer.ts`; `demo/src/components/FundedAccountPanel.tsx` |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Public browser identity-provider key | None | No | `demo/src/lib/paxeer.ts` |
| `NEXT_PUBLIC_SUPABASE_URL` | Demo identity provider base URL | None | No | `demo/src/lib/paxeer.ts` |
| `REDIS_ACL` | Base64 file input materialized at /run/secrets/users.acl | None | Yes | `docker/redis/files.tsv`; `docker/common/env-files.sh` |
| `REDIS_CONF` | Base64 file input materialized at /run/secrets/redis.conf | None | Yes | `docker/redis/files.tsv`; `docker/common/env-files.sh` |
| `REDIS_TLS_CA` | Base64 file input materialized at /run/secrets/ca.crt | None | No | `docker/redis/files.tsv`; `docker/common/env-files.sh` |
| `REDIS_TLS_CERT` | Base64 file input materialized at /run/secrets/redis.crt | None | No | `docker/redis/files.tsv`; `docker/common/env-files.sh` |
| `REDIS_TLS_KEY` | Base64 file input materialized at /run/secrets/redis.key | None | Yes | `docker/redis/files.tsv`; `docker/common/env-files.sh` |
| `SPONSOR_PRIVATE_KEY` | Base64 file input materialized at /run/secrets/sponsor_private_key | None | Yes | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh` |
| `SUPABASE_URL` | Identity provider base URL for JWT issuer and JWKS discovery | None | No | `gateway/src/env.ts` |
| `WALLET_CUSTODY_INVENTORY` | Base64 file input materialized at /run/secrets/wallet_custody_inventory.jwt | None | Yes | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh`; `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `WALLET_CUSTODY_INVENTORY_PUBLIC_KEY` | Base64 file input materialized at /run/secrets/wallet_custody_inventory_authority.pem | None | No | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh`; `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `WALLET_IDENTITY_BINDING_KEY` | Base64 file input materialized at /run/secrets/wallet_identity_binding_key | None | Yes | `docker/wallet-gateway/files.tsv`; `docker/common/env-files.sh` |
| `WALLET_IDENTITY_BINDING_PUBLIC_KEY` | Base64 file input materialized at /run/secrets/wallet_identity_binding_public.pem | None | No | `docker/wallet-attestor/files.tsv`; `docker/common/env-files.sh` |
| `WALLET_MASTER_KEY` | Base64 master encryption key decoding to exactly 32 bytes | None | Yes | `gateway/src/env.ts` |

## Optional

| Name | Sets | Source default | Secret | Reader |
| --- | --- | --- | --- | --- |
| `ACCOUNT_SETUP_GAS_CAP_WEI` | Maximum account setup gas cost in wei | 10000000000000000 | No | `gateway/src/env.ts` |
| `ACTION_CONFIRMATIONS` | Receipt confirmations required for durable actions | 1 | No | `gateway/src/env.ts` |
| `ACTION_FEE_BUMP_PERCENT` | Replacement transaction fee bump percentage | 12 | No | `gateway/src/env.ts` |
| `ACTION_RECEIPT_TIMEOUT_MS` | Receipt wait timeout in milliseconds | 90000 | No | `gateway/src/env.ts` |
| `ACTION_RECONCILE_INTERVAL_MS` | Durable-action reconciliation interval in milliseconds | 30000 | No | `gateway/src/env.ts` |
| `ACTION_WORKER_INTERVAL_MS` | Durable-action worker interval in milliseconds | 3000 | No | `gateway/src/env.ts` |
| `AGENT_CHALLENGE_TTL_SECONDS` | Agent authentication challenge lifetime in seconds | 120 | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_FROZEN` | Initial freeze flag for a new agent | true | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_MAX_APPROVE_WEI` | Default per-agent token approval cap in base units | 0 | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_MAX_DAILY_VALUE_WEI` | Default per-agent daily value cap in wei | 0 | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_MAX_TX_VALUE_WEI` | Default per-agent transaction value cap in wei | 0 | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_MODE` | Initial agent policy mode | read_only | No | `gateway/src/env.ts` |
| `AGENT_DEFAULT_RATE_LIMIT_PER_MINUTE` | Default per-agent signing requests per minute | 30 | No | `gateway/src/env.ts` |
| `AGENT_JWT_SECRET` | HS256 agent-token signing secret, at least 32 characters; setting it enables agent authentication | None | Yes | `gateway/src/env.ts` |
| `AGENT_REQUEST_MAX_TTL_SECONDS` | Maximum signed agent-request lifetime in seconds | 300 | No | `gateway/src/env.ts` |
| `AGENT_TOKEN_TTL_SECONDS` | Agent bearer-token lifetime in seconds | 900 | No | `gateway/src/env.ts` |
| `API_WORKERS` | Gateway worker count, from 1 to 64; each has its own database pool | 1 | No | `gateway/src/env.ts` |
| `ATTESTOR_ACTIVITY_TYPES` | Comma-separated admitted activity type numbers; asset-send type is supported | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_AGENTS_FILE` | Principal-file configuration path | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_AGENT_MAX_EXPIRY` | Maximum signed agent-request lifetime as a Go duration | 5m | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_AGENT_PAYMASTER` | Agent sponsored-transaction paymaster contract | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_BACKUP_DIR` | Encrypted snapshot directory; required for replication | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_BACKUP_KEY_FILE` | Path to snapshot encryption key; required with backup directory and for restore | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_CA_FILE` | Path to attestor TLS CA bundle; required with attestor endpoints | None | No | `gateway/src/env.ts` |
| `ATTESTOR_CEREMONY` | Enable operator key imports during a ceremony window | false | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_CLIENT_CERT_FILE` | Path to gateway client TLS certificate; required with attestor endpoints | None | No | `gateway/src/env.ts` |
| `ATTESTOR_CLIENT_KEY_FILE` | Path to gateway client TLS private key; required with attestor endpoints | None | No | `gateway/src/env.ts` |
| `ATTESTOR_ENDPOINTS` | Comma-separated HTTPS custody endpoints; when set, exactly five endpoints and quorum three are required | Empty list | No | `gateway/src/env.ts` |
| `ATTESTOR_HEALTH_INTERVAL_MS` | Attestor health polling interval in milliseconds | 5000 | No | `gateway/src/env.ts` |
| `ATTESTOR_JWKS_URL` | JWT verifier JWKS endpoint; setting it enables user-token verification | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_JWT_AUDIENCE` | Expected JWT audience; required when JWKS verification is enabled | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_JWT_ISSUER` | Expected JWT issuer; required when JWKS verification is enabled | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_JWT_MAX_AGE` | Maximum age of accepted user JWTs as a Go duration | 1h | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_KERNEL_POLICY_FILE` | Path to native activity and grant policy JSON | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / caps` | Per-asset activity spend caps; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / caps.*.daily` | Daily activity spend cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / caps.*.per_operation` | Per-operation asset spend cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / destinations_allow` | Activity destination allowlist; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / grants` | Per-asset grant caps; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / grants.*.daily` | Daily grant cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / grants.*.per_grant` | Individual grant cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / accounts.* / modules` | Allowed module names mapped to operation numbers; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / caps` | Per-asset activity spend caps | Empty | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / caps.*.daily` | Daily activity spend cap | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / caps.*.per_operation` | Per-operation asset spend cap | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / destinations_allow` | Activity destination allowlist | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / grants` | Per-asset grant caps | Empty | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / grants.*.daily` | Daily grant cap | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / defaults / grants.*.per_grant` | Individual grant cap | Unset | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_KERNEL_POLICY_FILE / root / accounts` | Map of account-specific rule overrides | Empty map | No | `attestor/internal/policy/lx/lx.go` |
| `ATTESTOR_NATIVE_POLICY_FILE` | Path to owner-purpose policy JSON | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_POLICY_FILE` | Path to EVM signing policy JSON | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / authorization_delegates` | Allowed EIP-7702 delegation targets; omitted account fields inherit defaults | Empty; refuses delegations | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / caps` | Asset spend caps; omitted account fields inherit defaults | Empty; assets need explicit caps | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / caps.*.daily` | Daily asset amount cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / caps.*.per_transaction` | Per-transaction asset amount cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / chain_id` | EVM chain identifier; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / destinations_allow` | Destination allowlist; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / destinations_deny` | Destination denylist; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / fee_caps` | Sponsored fee asset caps; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / fee_caps.*.daily` | Daily sponsored fee cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / fee_caps.*.per_transaction` | Per-transaction sponsored fee cap; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / fee_sponsors` | Sponsors whose fee leg uses fee caps; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / kinds` | Admitted signing kinds; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / program_calls` | Program invocation rules; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / program_calls[].entrypoints` | Allowed program entrypoints; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / program_calls[].fee_limit` | Program-call fee ceiling; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / program_calls[].program_id` | Allowed program identifier; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / rate_per_minute` | Signing rate limit; omitted account fields inherit defaults | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / selectors` | Per-contract function selector allowlists; omitted account fields inherit defaults | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / accounts.* / trusted_spenders` | Token approval spender allowlist; omitted account fields inherit defaults | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / authorization_delegates` | Allowed EIP-7702 delegation targets | Empty; refuses delegations | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / caps` | Asset spend caps | Empty; assets need explicit caps | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / caps.*.daily` | Daily asset amount cap | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / caps.*.per_transaction` | Per-transaction asset amount cap | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / destinations_allow` | Destination allowlist | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / destinations_deny` | Destination denylist | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / fee_caps` | Sponsored fee asset caps | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / fee_caps.*.daily` | Daily sponsored fee cap | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / fee_caps.*.per_transaction` | Per-transaction sponsored fee cap | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / fee_sponsors` | Sponsors whose fee leg uses fee caps | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / program_calls` | Program invocation rules | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / program_calls[].entrypoints` | Allowed program entrypoints | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / program_calls[].fee_limit` | Program-call fee ceiling | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / program_calls[].program_id` | Allowed program identifier | None | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / selectors` | Per-contract function selector allowlists | Unset | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / defaults / trusted_spenders` | Token approval spender allowlist | Empty | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_POLICY_FILE / root / accounts` | Map of account-specific rule overrides | Empty map | No | `attestor/internal/policy/policy.go` |
| `ATTESTOR_QUORUM` | Attestor signing threshold | 3 | No | `gateway/src/env.ts` |
| `ATTESTOR_REGION` | Region label reported by health | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_REPLICA_ADDR` | Replica SSH socket address; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_HOST_KEY_FILE` | Path to pinned replica SSH host public key; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_INTERVAL` | Shipping interval as a Go duration, at least one second; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_KEY_FILE` | Path to replica SSH private key; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_OPERATION_TIMEOUT` | SSH/SFTP operation timeout as a positive Go duration; applies when replication is configured | 2m | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_REMOTE_DIR` | Absolute per-node directory on the replica; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_REPLICA_USER` | Replica SSH login name; required with any replica setting | None | No | `attestor/internal/replica/config.go` |
| `ATTESTOR_RPC_URL` | Node RPC endpoint; required with kernel policy | None | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_SNAPSHOT_INTERVAL` | Snapshot interval as a Go duration, at least one second | 1h | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_SNAPSHOT_RETAIN` | Number of newest snapshots to retain | 48 | No | `attestor/internal/config/config.go`; `attestor/cmd/attestor/main.go` |
| `ATTESTOR_TIMEOUT_MS` | Attestor request timeout in milliseconds | 15000 | No | `gateway/src/env.ts` |
| `CEREMONY_ATTESTOR_BIN` | Attestor binary path supplied to rehearsal options | None | No | `ceremony/internal/rehearsal/rehearsal.go` |
| `CEREMONY_RPC_URL` | Node RPC endpoint for ceremony binding operations | None | No | `ceremony/internal/migrate/migrate.go` |
| `DATABASE_POOL_MAX` | Maximum PostgreSQL connections per worker | 20 | No | `gateway/src/env.ts` |
| `HYPERPAXEER_CHAIN_ID` | EVM chain identifier | 125 | No | `gateway/src/env.ts` |
| `HYPERPAXEER_EXPLORER_URL` | Explorer URL returned to wallet clients | None | No | `gateway/src/env.ts` |
| `HYPERPAXEER_RPC_URL` | Chain RPC URL | Public mainnet-beta node RPC | No | `gateway/src/env.ts` |
| `LAYERX_FILES_OWNER` | Owner applied to materialized files when running as root | Owner configured by the container image; otherwise unset | No | `docker/common/env-files.sh`; `docker/wallet-gateway/Dockerfile` |
| `LAYERX_FILES_OWNER_ATTESTOR_BACKUP_KEY` | Ownership override for the materialized ATTESTOR_BACKUP_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_CA` | Ownership override for the materialized ATTESTOR_CA file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_CLIENT_CERT` | Ownership override for the materialized ATTESTOR_CLIENT_CERT file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_CLIENT_KEY` | Ownership override for the materialized ATTESTOR_CLIENT_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_NODE_KEY` | Ownership override for the materialized ATTESTOR_NODE_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_OPERATOR_CA` | Ownership override for the materialized ATTESTOR_OPERATOR_CA file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_REPLICA_HOST_KEY` | Ownership override for the materialized ATTESTOR_REPLICA_HOST_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_REPLICA_KEY` | Ownership override for the materialized ATTESTOR_REPLICA_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_TLS_CA` | Ownership override for the materialized ATTESTOR_TLS_CA file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_TLS_CERT` | Ownership override for the materialized ATTESTOR_TLS_CERT file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_ATTESTOR_TLS_KEY` | Ownership override for the materialized ATTESTOR_TLS_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_REDIS_ACL` | Ownership override for the materialized REDIS_ACL file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_REDIS_CONF` | Ownership override for the materialized REDIS_CONF file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_REDIS_TLS_CA` | Ownership override for the materialized REDIS_TLS_CA file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_REDIS_TLS_CERT` | Ownership override for the materialized REDIS_TLS_CERT file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_REDIS_TLS_KEY` | Ownership override for the materialized REDIS_TLS_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_SPONSOR_PRIVATE_KEY` | Ownership override for the materialized SPONSOR_PRIVATE_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_WALLET_CUSTODY_INVENTORY` | Ownership override for the materialized WALLET_CUSTODY_INVENTORY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_WALLET_CUSTODY_INVENTORY_PUBLIC_KEY` | Ownership override for the materialized WALLET_CUSTODY_INVENTORY_PUBLIC_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_WALLET_IDENTITY_BINDING_KEY` | Ownership override for the materialized WALLET_IDENTITY_BINDING_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_WALLET_IDENTITY_BINDING_PUBLIC_KEY` | Ownership override for the materialized WALLET_IDENTITY_BINDING_PUBLIC_KEY file | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_TABLE` | Container secret-file mapping table path | /etc/layerx/files.tsv | No | `docker/common/env-files.sh` |
| `LAYERX_TRUSTED_PROXIES` | Comma-separated proxy IP addresses or CIDRs trusted for client attribution | Empty | No | `gateway/src/env.ts` |
| `LOADTEST_KEYGEN_TIMEOUT` | Key generation timeout as a positive Go duration; command-line settings override environment values | 10m | No | `attestor/cmd/loadtest/main.go` |
| `LOADTEST_SESSION_TIMEOUT` | Signing session timeout as a positive Go duration; command-line settings override environment values | 2m | No | `attestor/cmd/loadtest/main.go` |
| `LOG_LEVEL` | Gateway logging level | info | No | `gateway/src/env.ts` |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL` | OAuth callback URL override | Current browser origin plus /auth/callback | No | `demo/src/lib/paxeer.ts` |
| `NEXT_PUBLIC_SITE_URL` | Metadata base URL for demo page links | Unset | No | `demo/src/app/layout.tsx` |
| `NODE_ENV` | Gateway runtime mode | development | No | `gateway/src/env.ts` |
| `PAXEER_GAS_STATION_GAS_LIMIT` | Positive gas limit for gas-station provisioning | None | No | `gateway/src/provision/bind.ts` |
| `PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT` | Positive maximum sponsored token charge in base units | None | No | `gateway/src/provision/bind.ts` |
| `PAXEER_GAS_STATION_PAYMASTER` | Paymaster contract; required for gas-station provisioning | None | No | `gateway/src/provision/bind.ts` |
| `POLICY_MAX_DAILY_VALUE_WEI` | Maximum human-wallet daily value in wei | 10000000000000000000 | No | `gateway/src/env.ts` |
| `POLICY_MAX_TX_VALUE_WEI` | Maximum human-wallet value per transaction in wei | 1000000000000000000 | No | `gateway/src/env.ts` |
| `POLICY_RATE_LIMIT_PER_MINUTE` | Human-wallet signing requests per minute | 60 | No | `gateway/src/env.ts` |
| `PORT` | Gateway HTTP listening port | 8787 | No | `gateway/src/env.ts` |
| `RATE_LIMIT_ACCOUNT_PER_MINUTE` | HTTP requests per account per minute | 60 | No | `gateway/src/env.ts` |
| `RATE_LIMIT_CLIENT_PER_MINUTE` | HTTP requests per client per minute | 120 | No | `gateway/src/env.ts` |
| `REDIS_MAXMEMORY` | Redis maximum memory size | 0 | No | `docker/redis/entrypoint.sh` |
| `RPC_HEALTH_INTERVAL_MS` | RPC member health polling interval in milliseconds | 5000 | No | `gateway/src/env.ts` |
| `RPC_LAG_THRESHOLD_BLOCKS` | Maximum RPC member lag in blocks | 20 | No | `gateway/src/env.ts` |
| `RPC_TIMEOUT_MS` | RPC request timeout in milliseconds | 8000 | No | `gateway/src/env.ts` |
| `RPC_URLS` | Comma-separated HTTP(S) node RPC pool | Public mainnet-beta node RPC | No | `gateway/src/env.ts` |
| `SEQUENCER_RPC_URL` | Node RPC endpoint for transaction submission and receipts | First entry of RPC_URLS | No | `gateway/src/env.ts` |
| `WALLET_AUTHORIZATION_DELEGATES` | Comma-separated EIP-7702 delegation allowlist | Empty | No | `gateway/src/env.ts` |
| `WALLET_CUSTODY_INVENTORY_FILE` | Path to approved signed custody inventory; required with attestor endpoints | None | No | `gateway/src/env.ts` |
| `WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE` | Path to custody-inventory verification public key; required with attestor endpoints | None | No | `gateway/src/env.ts` |
| `WALLET_GAS_STATION_URL` | Gas-station service URL for sponsored account setup | None | No | `gateway/src/env.ts` |
| `WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE` | Path to identity-binding signing key; configure with the tenant; required by the gateway container and with attestor endpoints | None | No | `gateway/src/env.ts` |
| `WALLET_IDENTITY_BINDING_TENANT` | Identity-binding tenant; configure with the private-key file; required by the gateway container and with attestor endpoints | None | No | `gateway/src/env.ts` |
| `WALLET_MASTER_KEY_VERSION` | Encryption key version recorded with wallet envelopes | 1 | No | `gateway/src/env.ts` |
| `deploy/human.toml / build / dockerfile` | Container build recipe | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / deploy / strategy` | Deployment strategy | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_APPROVAL_UPSTREAM_CA_DER` | Human-service environment setting LAYERX_EVENTS_APPROVAL_UPSTREAM_CA_DER | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE` | Human-service environment setting LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PKCS12` | Human-service environment setting LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PKCS12 | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_APPROVAL_UPSTREAM_TOKEN_FILE` | Human-service environment setting LAYERX_EVENTS_APPROVAL_UPSTREAM_TOKEN_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_APPROVAL_UPSTREAM_URL` | Human-service environment setting LAYERX_EVENTS_APPROVAL_UPSTREAM_URL | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_JOURNEY_UPSTREAM_CA_DER` | Human-service environment setting LAYERX_EVENTS_JOURNEY_UPSTREAM_CA_DER | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE` | Human-service environment setting LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PKCS12` | Human-service environment setting LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PKCS12 | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_JOURNEY_UPSTREAM_TOKEN_FILE` | Human-service environment setting LAYERX_EVENTS_JOURNEY_UPSTREAM_TOKEN_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_JOURNEY_UPSTREAM_URL` | Human-service environment setting LAYERX_EVENTS_JOURNEY_UPSTREAM_URL | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CA_DER` | Human-service environment setting LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CA_DER | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE` | Human-service environment setting LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PKCS12` | Human-service environment setting LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PKCS12 | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_WEBHOOKS_UPSTREAM_TOKEN_FILE` | Human-service environment setting LAYERX_EVENTS_WEBHOOKS_UPSTREAM_TOKEN_FILE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_EVENTS_WEBHOOKS_UPSTREAM_URL` | Human-service environment setting LAYERX_EVENTS_WEBHOOKS_UPSTREAM_URL | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_BIND` | Human-service environment setting LAYERX_HUMAN_BIND | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_COMPONENT_SOCKET` | Human-service environment setting LAYERX_HUMAN_COMPONENT_SOCKET | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_IO_DEADLINE_SECONDS` | Human-service environment setting LAYERX_HUMAN_IO_DEADLINE_SECONDS | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_LISTENER` | Human-service environment setting LAYERX_HUMAN_LISTENER | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_MAX_BODY_BYTES` | Human-service environment setting LAYERX_HUMAN_MAX_BODY_BYTES | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_MAX_CONNECTIONS` | Human-service environment setting LAYERX_HUMAN_MAX_CONNECTIONS | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_MAX_HEADER_BYTES` | Human-service environment setting LAYERX_HUMAN_MAX_HEADER_BYTES | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_MAX_PRINCIPALS` | Human-service environment setting LAYERX_HUMAN_MAX_PRINCIPALS | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_REQUESTS_PER_MINUTE` | Human-service environment setting LAYERX_HUMAN_REQUESTS_PER_MINUTE | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_RP_ID` | Human-service environment setting LAYERX_HUMAN_RP_ID | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_HUMAN_WEB_ORIGIN` | Human-service environment setting LAYERX_HUMAN_WEB_ORIGIN | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_KERNEL_PAXEER_RPC_NAMES` | Human-service environment setting LAYERX_KERNEL_PAXEER_RPC_NAMES | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / env / LAYERX_NODE_NETWORK_ID` | Human-service environment setting LAYERX_NODE_NETWORK_ID | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / files[] / guest_path` | Mounted secret file path | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / files[] / secret_name` | Platform file-secret name | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service / auto_start_machines` | Automatic machine starting | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service / auto_stop_machines` | Automatic machine stopping | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service / force_https` | Redirect to HTTPS | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service / internal_port` | Container service port | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service / min_machines_running` | Minimum running machines | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / grace_period` | Health check startup grace period | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / interval` | Health check interval | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / method` | HTTP health check method | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / path` | HTTP health check route | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / protocol` | Service or health check transport | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.checks[] / timeout` | Health check timeout | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.concurrency / hard_limit` | Concurrency hard limit | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.concurrency / soft_limit` | Concurrency soft limit | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / http_service.concurrency / type` | Concurrency accounting mode | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / mounts[] / destination` | Volume mount directory | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / mounts[] / initial_size` | Initial volume capacity | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / mounts[] / scheduled_snapshots` | Enable scheduled snapshots | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / mounts[] / snapshot_retention` | Snapshot retention count | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / mounts[] / source` | Persistent volume name | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / root / app` | Deployment application name | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / root / kill_signal` | Shutdown signal | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / root / kill_timeout` | Shutdown grace interval | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / root / primary_region` | Primary deployment region | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[] / auto_start_machines` | Automatic machine starting | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[] / auto_stop_machines` | Automatic machine stopping | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[] / internal_port` | Container service port | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[] / min_machines_running` | Minimum running machines | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[] / protocol` | Service or health check transport | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[].ports[] / port` | Published service port | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[].tcp_checks[] / grace_period` | Health check startup grace period | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[].tcp_checks[] / interval` | Health check interval | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / services[].tcp_checks[] / timeout` | Health check timeout | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / vm[] / cpu_kind` | Machine CPU class | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / vm[] / cpus` | Machine CPU count | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / vm[] / memory` | Machine memory | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
| `deploy/human.toml / vm[] / size` | Machine size | None (set in template) | No | `Deployment platform reads deploy/human.toml` |
