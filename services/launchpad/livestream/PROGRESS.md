# Livestream service

The service entry point is [cmd/livestreamd/main.go](cmd/livestreamd/main.go).
The service provides persistent lifecycle handling, v2 HTTP adapters and
provider workers.

## Current manifests and storage

[go.mod](go.mod) declares Go 1.25.0 and pins goose v3.24.3. The
supply-chain tools pin Go 1.26.4, Node 24.16.0 and pnpm 9.15.0; see the
[supply-chain notes](../security/supplychain/README.md). A module's `go` directive
is distinct from that build-tool pin.

- [../go.work](../go.work) includes this module and the shared SDK. This module's
  [go.mod](go.mod) replaces the shared SDK with
  [sdk/launchpad/shared](../../../sdk/launchpad/shared) in the monorepo.
- [migrations](migrations) embed the initial stream schema and the lifecycle read
  store. [internal/db/sqlcdb](internal/db/sqlcdb) contains generated query bindings;
  lifecycle operations also use PostgreSQL directly.
- [internal/app/app.go](internal/app/app.go) applies embedded migrations outside
  production. Production verifies the migration coordinator's expected schema
  and opens its budgeted runtime pool without migrating at startup.
- PostgreSQL persists operations, provider bindings, worker attempts, key
  rotations and usage. Redis backs viewer counting and rate limits. Provider
  credentials use Vault transit through
  [internal/streams/credentials.go](internal/streams/credentials.go).

## Runtime modes and configuration

[internal/config/config.go](internal/config/config.go) is the configuration
authority. Common inputs include `DATABASE_URL`, `REDIS_URL`, `PORT` (default
3000), `LOG_LEVEL` (default `info`) and `MAX_STREAMS_PER_WALLET` (default 3).

The production `RUNTIME_PROFILE=platform` mode requires
`LIVEPEER_PROVIDER_MODE=unavailable` and rejects `LIVESTREAM_CONNECTED_ENABLED`.
It serves lifecycle reads, viewer sessions and heartbeats over real stores.
Provider-bound routes answer HTTP 409 with `BROADCAST_NOT_PROVISIONED`.
`LIVESTREAM_PRIVATE_LISTEN_ADDRESS` selects the private mTLS listener,
with the gateway certificate pinned by `GATEWAY_PEER_CERT_SHA256`; its certificate,
key and client CA use the `LIVESTREAM_TLS_*` settings. The plain listener serves
health and metrics. Non-production mode also exposes reads without broadcast
authority.

Broadcasting uses the production connected runtime with
`LIVESTREAM_CONNECTED_ENABLED=true` and a `container` or `owned_local` profile.
It requires independently configured Livepeer, identity, Vault, action-signing,
playback, budget and chain authorities. `LIVEPEER_API_URL` must match the Studio
API endpoint enforced by `ValidateConnected`; `LIVEPEER_PROVIDER_MODE=available`,
`MAX_GLOBAL_STREAMS`, `LIVESTREAM_VIEWER_TOKEN_KEY`, `LIVESTREAM_VAULT_TRANSIT_*`,
`IDENTITY_SERVICE_URL`, identity mTLS settings and `RPC_URL` are among the required
inputs. Startup requires `LIVEPEER_WEBHOOK_TIMESTAMP_UNIT=milliseconds` for the
Studio webhook decoder. Read `ValidateConnected` and `NewConnectedRuntime` for
the complete validation contract.

Paxeer is one BFT network with EVM chain ID 125 (`0x7d`); its kernel participates
in the network and proposes blocks. Receipts are final when their blocks commit.
The connected runtime checks chain ID 125. Public node JSON-RPC uses the root
path; the
[public RPC reference](../../../docs/site/docs/reference/public-rpc.md) lists the
node endpoints. Tenant sub-chains are database domains within this network.

## Current HTTP and lifecycle contract

[RegisterRoutes](internal/streams/handlers.go) mounts the v2 surface:

- `GET /v2/streams`, `/v2/streams/live`, `/v2/streams/pool/{poolAddress}` and
  `/v2/streams/{id}` read the lifecycle store.
- `POST /v2/streams` creates a durable operation; clients observe it through
  `GET /v2/streams/operations/{id}`. The v2 create response carries an operation,
  rather than the old synchronous stream key and playback fields.
- `POST /v2/streams/{id}:start` and `:end` use `expectedRevision` through the
  frozen adapters. The registered `/go-live` and `/end` forms also remain in v2.
- Title edits, restore, key rotation, ingest-key access and private playback
  have dedicated v2 handlers. Signed mutations bind the operation ledger,
  identity, creator authority and persisted revision.
- Viewer sessions and `/v2/streams/{id}/viewers:heartbeat` use viewer authority
  and Redis. Provider callbacks include `/webhooks/livepeer`,
  `/webhooks/livepeer/playback-access` and `/v2/provider-webhooks/livepeer`.
  Webhook authentication checks raw bytes before accepting state observations.

The unversioned `/streams` handlers return dependency-unavailable errors; they
are not the current creation or playback contract. The v2 adapters supply typed
response/error envelopes. Provisioning, reconciliation and termination workers
preserve operation identity across retries and restart; an HTTP acceptance does
not by itself prove provider activation.

## Integration prerequisites

Integration tests require dedicated PostgreSQL and Redis stores, a chain-125
RPC, TLS Vault transit, admitted Livepeer operations and a registered playback
webhook, and a media measurement manifest with its SHA-256. The lifecycle
fixtures also start PostgreSQL and Redis executables directly. Receipt and
provider observations remain separate: HTTP acceptance does not establish
broadcast activation.
