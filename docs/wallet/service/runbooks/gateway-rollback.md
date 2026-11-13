# Gateway rollback

The wallet gateway image is built from
[`docker/wallet-gateway/Dockerfile`](../../../../docker/wallet-gateway/Dockerfile)
with the repository root as its build context. Runtime examples are in
[`railway.env.example`](../../../../services/wallet/deploy/railway.env.example)
and [`redis.railway.env.example`](../../../../services/wallet/deploy/redis.railway.env.example).
The deployment platform selects and starts the image with those runtime settings.

The gateway applies SQL files from `services/wallet/gateway/migrations` in
filename order at startup and records them in `_migrations`. It does not revert
migrations. A rollback selects compatible application code while preserving the
current database schema and nonce state.

## Select a compatible image

Compare the target image's migration files with the applied filenames:

```sh
psql "$DATABASE_URL" -Atc 'select filename from _migrations order by filename'
git ls-tree -r --name-only "$TARGET_REVISION" services/wallet/gateway/migrations
```

`TARGET_REVISION` is the source revision of the candidate image. Every applied
migration must be present in that image. Its code must support the current
schema, including attestor-backed wallets and any durable approval or signing
states already stored. Preserve `DATABASE_URL`, the shared nonce store,
authentication settings, attestor configuration and receipt trust material.

## Deploy and check

Select the compatible image using the deployment platform, then replace the
gateway instances. Keep the endpoint proxy pointing to the gateway. Confirm
startup completes with the current migrations and the server listens on `PORT`.

Using the operator-supplied gateway base in `GATEWAY_URL`:

```sh
curl --silent --show-error --max-time 10 "$GATEWAY_URL/healthz"
curl --silent --show-error --max-time 10 "$GATEWAY_URL/readyz"
```

`/healthz` returns `ok: true`. `/readyz` returns 200 with `ready: true` and
`attestors`, `nonce_store`, `rpc_pool` and `identity_provider` states `up`.
Detailed diagnostics require `GET /v1/admin/readiness` with bearer
`GATEWAY_ADMIN_TOKEN`.

Reconcile retained approval and transaction state before accepting another
signing operation. Transactions with an uncertain outcome are polled by their
existing hash; changing images does not authorize another debit. The shared
`nonce_allocations` table retains allocations across application replacements.
