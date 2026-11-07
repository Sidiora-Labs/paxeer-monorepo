# Gateway rollback

The wallet gateway image is built from `docker/wallet-gateway/Dockerfile`
with the repository root as its build context. Deployment environment examples
are tracked in `services/wallet/deploy/railway.env.example` and
`services/wallet/deploy/redis.railway.env.example`. For the Fly application
`paxeer-wallet-gateway` used below, the operator supplies its deployment
configuration and deploys the selected image with `--image`. The gateway
applies the SQL files under `services/wallet/gateway/migrations` in filename
order at start and never reverts one. A rollback therefore returns the
gateway's code to an earlier image; it never returns the database schema.

## Trigger

- After a deployment, `curl -sS https://<gateway-base>/readyz` answers 503
  for a component that was `up` before it, or `/healthz` stops answering.
- `scripts/wallet/check-live.sh gateway` fails where it
  passed on the previous release.
- Signing, provisioning or broadcast errors rise after a deployment with the
  attestor network, the identity provider and the RPC pool healthy.

## Preconditions

- The fault follows a gateway deployment. A fault in the attestors, the
  identity provider or the RPC pool is handled by its own runbook; rolling the
  gateway back does not repair it.
- The rollback target image contains every migration file already applied to
  the database, so its code runs on the current schema. Once any wallet
  carries `migrated_at`, the target must contain
  `migrations/006_attestor_signing.sql` and the attestor signing path in
  `src/routes/sign.ts`; an older image cannot sign for a migrated wallet.
- After the endpoint cutover, the proxy on the current host keeps pointing at
  `paxeer-wallet-gateway`; a gateway rollback does not touch the proxy.
- The operator has the gateway deployment configuration used for the running
  release, named `<gateway-config-path>` below, with its environment and
  secrets preserved and its readiness check set to `/readyz`.

## Commands

1. List the releases with their images.

   ```sh
   flyctl releases --app paxeer-wallet-gateway --image
   ```

   Expected: releases newest first, each with version, status and image
   reference. The rollback target `<previous-image>` is the image of the
   newest release before the faulty one whose status is `complete`.

2. Check that the target carries every applied migration.

   ```sh
   psql "$DATABASE_URL" -Atc "select filename from _migrations order by filename"
   git -C <repository-checkout> ls-tree --name-only <target-revision> services/wallet/gateway/migrations/
   ```

   Expected: every name in the first list appears in the second.

3. Deploy the target image.

   ```sh
   flyctl deploy --config <gateway-config-path> --image <previous-image> --app paxeer-wallet-gateway --ha=false -y
   ```

   Expected: every machine replaced with the target image and passing the
   platform check.

4. Watch the log of the rolled-back machines.

   ```sh
   flyctl logs --app paxeer-wallet-gateway --no-tail
   ```

   Expected: migrations report nothing to apply, the server listens on `PORT`,
   and no start-up error.

## Readiness check that proves recovery

```sh
curl -sS https://<gateway-base>/healthz
curl -sS https://<gateway-base>/readyz
```

`/healthz` answers `ok: true`; `/readyz` answers 200 with `"ready":true`
and `attestors`, `nonce_store`, `rpc_pool` and `identity_provider` all `up`;
`scripts/wallet/check-live.sh gateway` passes. Readiness is read from `/readyz`;
`/health/ready` is not served. Record revision, command, exit code and log path
in the operator's change record.

## Rollback

Redeploy the faulty release's image, read from step 1, with the command of
step 3. Nonces allocated by either release sit in the shared nonce store
(`nonce_allocations`), so neither direction reuses a nonce.
