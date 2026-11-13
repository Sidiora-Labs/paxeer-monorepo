# Deployment

## Container image

The image is defined in [`docker/wallet-pwa/Dockerfile`](../../../../docker/wallet-pwa/Dockerfile) and is built from the repository root, because it also copies `services/agent/sdk/typescript` and `services/wallet` to build the wallet SDK:

```sh
docker build -f docker/wallet-pwa/Dockerfile \
  --build-arg NEXT_PUBLIC_PAXEER_WALLET_API=... \
  --build-arg NEXT_PUBLIC_PAXEER_RPC_URL=... \
  -t wallet-pwa .
```

The build uses Node 22.22.0 and pnpm 10.34.9. Public build arguments cover
wallet authentication, RPC and service URLs, explorer status, sponsorship,
chain and token configuration, media, push and Sentry. The complete `ARG` and
`ENV` lists are in the linked Dockerfile. Set them before the Next.js build;
public variables are included in the browser bundle.

The runtime stage runs `deployment/start-wallet.sh` under `tini`: it starts the standalone Next.js server on loopback port 3000 and nginx on port 3001 with `deployment/nginx.conf` in front of it, and exits when either process stops. The image health check calls `/wallet/api/health?mode=liveness` through nginx on port 3001.

Run-time server variables (`BLOCKSCOUT_UPSTREAM_BASE`, S3, VAPID, push, chat, Sentry and `TRUSTED_PROXY_SECRET`) are set on the container; the full list is in the [app README](../../../../apps/wallet/README.md#configuration).

## Other descriptors

- `railway.json` builds with the same Dockerfile and restarts on failure, at most three retries.
- `nixpacks.toml` installs with `pnpm install --frozen-lockfile`, builds with `npm run build` and starts with `npm run start`.
- `ecosystem.config.cjs` runs `next start -p 5000` under pm2 from the app directory.

## After a deployment

`GET /wallet/api/health?mode=readiness` reports process, upstream and push configuration checks; `?mode=liveness` reports only that the process answers. Recovery procedures are in [operations/runbooks.md](../operations/runbooks.md).
