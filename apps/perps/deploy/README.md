# Hyperpax-UI — Production Deployment

This directory contains environment examples and a **partial nginx Compose
configuration** for the frontend in `apps/perps`. The checked-in
[`docker-compose.yml`](docker-compose.yml) defines only `nginx`: the entire
`app` service is commented out. It cannot build, start, health-check or roll
back a Next.js application.

The application [`Dockerfile`](../Dockerfile) separately builds a standalone
Next.js image. The examples below run that image directly on loopback for an
operator-managed ingress. They do not depend on an absent Compose app service.
See the [data API guide](../data_api.md) for the application gateway protocol,
[settings guide](../src/app/modals/SettingsModal.md) for UI preferences, and
[component guide](../src/components/paxeer/README.md) for live widget integration.

## TL;DR (first-time deploy)

Use a checkout of this monorepo and run application commands from `apps/perps`.
Install Docker with BuildKit on the target build host. Access to the charting
image referenced by digest in the Dockerfile is required.

```bash
cd /path/to/paxeer-monorepo/apps/perps
cp deploy/.env.production.example .env.production
${EDITOR:-vi} .env.production
```

Set at least `NEXT_PUBLIC_PERPS_GATEWAY_URL` and `MARKET_PROXY_ORIGIN` for your
deployment, and review the remaining public values. Build the image with the
public variables explicitly forwarded as build arguments, then run it with a
runtime environment file as shown below. Arrange TLS and ingress separately
before exposing the application to users.

## Architecture

```text
browser → operator-provided HTTPS ingress → frontend container :3000
                                             ├─ /api/build-id
                                             └─ /api/candle/* → external candle feed
browser → perps data gateway (REST / WebSocket / GraphQL)
browser / wallet → Paxeer node JSON-RPC at the node root
```

Paxeer mainnet-beta is one BFT chain, EVM chain ID **125 (`0x7d`)**. The kernel
is part of block proposal; receipts are final at block commit. Public JSON-RPC
uses the roots of `api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`, for example
`https://api1.mainnet-beta.paxeer.network`. The data gateway origin is a separate
application configuration, not another chain or finality stage. Sub-chains are
tenant domains in the database and do not require a frontend-specific node.

The supplied nginx Compose service publishes ports 80 and 443, joins the
`internal` bridge and maps `host.docker.internal` to `host-gateway`. That
mapping alone neither selects an application upstream nor makes a
loopback-only host listener reachable from the container. The example app
container below is not attached to that bridge. Configure ingress networking
and its upstream explicitly for your deployment model.

## Files

| File | Current purpose |
| --- | --- |
| [`../Dockerfile`](../Dockerfile) | Digest-pinned Node and charting stages, pnpm install, standalone build, non-root runtime and image health check |
| [`docker-compose.yml`](docker-compose.yml) | nginx only; app/build/runtime environment block is commented out |
| [`.env.production.example`](.env.production.example) | Environment starting point; some deployment comments describe tooling absent from this tree |
| [`build-args.production.env`](build-args.production.env) | Public build-argument values; Docker does not automatically read an env file as build arguments |
| [`../ecosystem.config.js`](../ecosystem.config.js) | Host PM2 example with fixed `cwd: /root/Hyperpax-UI`; requires path/process review before use in this monorepo |

There is no checked-in `deploy/scripts/deploy.sh`, `server-init.sh`,
`cloudflare-firewall.sh`, `deploy/CLOUDFLARE.md`, or `deploy/nginx/` configuration
in this checkout. The Compose mounts refer to nginx config, certificates,
ACME webroot and DH parameters that an operator must supply. Starting this
Compose file alone is not a production deployment procedure.

## Step-by-step

### 1. Provision the box

Provision Docker/BuildKit, access to required images, storage, logging and an
application service manager through your normal infrastructure process. This
tree does not install packages, create deployment users, configure host
firewalls or provision certificates automatically.

Choose an ingress topology and supply its configuration and TLS certificates.
If using Cloudflare, configure its DNS and TLS mode to match that origin;
certificate paths and access rules must match the actual ingress. No
Cloudflare dashboard walkthrough or firewall installer is included here.

### 2. Configure environment

`.env.production` belongs at the **application root** (`apps/perps`), not the
monorepo root. Docker's build context excludes `.env*`, so copying that file
alone does not configure a Docker build.

| Variable | Contract |
| --- | --- |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL` | Required bare HTTP(S) origin. Parsed at module load; missing or malformed values throw. Frontend derives `/api`, `/ws` and `/graphql`. |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Primary root-path node JSON-RPC URL; example template uses `https://api1.mainnet-beta.paxeer.network`. |
| `NEXT_PUBLIC_PAXEER_RPC_URLS` | Optional comma-separated fallback list; see [`core/rpc.ts`](../src/core/rpc.ts) for selection. |
| `NEXT_PUBLIC_MARKET_PROXY_HOST` | Optional browser feed hostname override. |
| `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Embedded-wallet sign-in configuration; when either is blank in the bundle, that sign-in section is hidden. |
| `NEXT_PUBLIC_PAXEER_WALLET_API` | Embedded-wallet API origin. |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` | Optional auth callback and trusted wallet messaging origin configuration. |
| `NEXT_PUBLIC_PAXSCAN_API_URL`, `NEXT_PUBLIC_PAXSCAN_WS_URL`, `NEXT_PUBLIC_PAXSCAN_BASE_URL` | Explorer integration and transaction links. |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN` | `1` opens the spot route/navigation/markets mode. Blank keeps the release gate closed. |
| `MARKET_PROXY_HOST` | Server candle-feed hostname, default `data.feed.chainflowtrading.com`. |
| `MARKET_PROXY_ORIGIN` | Required server upstream Origin header: exact bare HTTP(S) origin, e.g. `https://perps.example.com`, with no trailing slash, path, credentials, query or fragment. |
| `VERCEL_GIT_COMMIT_SHA` or `RAILWAY_GIT_COMMIT_SHA` | Runtime build identity for `/api/build-id`; Railway takes precedence. Unset values report `development`. |

The optional precompile overrides are validated in
[`core/network/precompiles.ts`](../src/core/network/precompiles.ts). Each must
be `0x` followed by exactly 40 hexadecimal digits; blank uses the release
address. These suffixes are shown without their leading zero padding:

| Override | Release address suffix |
| --- | --- |
| `NEXT_PUBLIC_PERPS_PRECOMPILE` | `0x1015` |
| `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` | `0x1017` |
| `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` | `0x1021` |
| `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` | `0x1022` |
| `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE` | `0x1023` |
| `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` | `0x1024` |
| `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` | `0x1025` |

All `NEXT_PUBLIC_*` values are visible in browser code and are baked into the
bundle. Changing them requires a rebuild with the corresponding Dockerfile
build arguments and replacement of the running image. Supabase publishable
keys are client identifiers; private keys, service-role credentials and
signing secrets must never be public build inputs.

Server-only values must reach the actual server process. The Dockerfile does
not embed `MARKET_PROXY_HOST` or `MARKET_PROXY_ORIGIN`. Pass them at container
creation. A plain `docker restart` or `docker compose --env-file ... restart`
retains the existing container environment; it does not load edited values.
For a future configured Compose app service, updating interpolated environment
requires `up -d --force-recreate app`, with that service actually defined.

### 3. Build the frontend image

From `apps/perps`, using Bash and your operator-controlled environment file:

```bash
set -a
. ./.env.production
set +a

perps_release=$(git rev-parse --short HEAD)
perps_build_args=()
for perps_key in ${!NEXT_PUBLIC_@}; do
  perps_build_args+=(--build-arg "$perps_key")
done

docker build "${perps_build_args[@]}" \
  --build-arg "NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA=$perps_release" \
  -t "perps-ui:$perps_release" .
```

Only arguments declared by the Dockerfile affect the build. Consult its
`ARG NEXT_PUBLIC_*` declarations when adding configuration; an environment
variable in a template is not proof that the image forwards it. For example,
`NEXT_PUBLIC_NETWORK_STATUS_WS_URL` is in the template but currently has no
Dockerfile build argument.

The Dockerfile installs with the app's frozen pnpm lockfile, copies the
charting library from its pinned image, checks the standalone charting script,
and runs `pnpm build`. Keep the image digest checks and charting asset check.
The runtime uses UID 1001 and `tini`, defaults to port 3000 and starts
`node server.js`.

### 4. Start the image

```bash
docker run -d --name perps-frontend \
  --restart unless-stopped \
  --env-file .env.production \
  -e "VERCEL_GIT_COMMIT_SHA=$perps_release" \
  -p 127.0.0.1:3000:3000 \
  "perps-ui:$perps_release"
```

The environment file is supplied again for runtime server configuration;
client-visible variables still come from the built bundle. Use a matching
runtime identity and image tag. The image health check requests
`/api/build-id` every 15 seconds with a 5-second timeout, 20-second start
period and three retries. A healthy response proves the Next.js endpoint is
available, not that the gateway, wallet or candle feed works.

### 5. Verify before routing traffic

```bash
docker inspect --format '{{json .State.Health}}' perps-frontend
docker logs --tail 100 perps-frontend
curl --fail-with-body http://127.0.0.1:3000/api/build-id
curl --fail-with-body http://127.0.0.1:3000/api/candle/config
```

`/api/build-id` returns `{ "buildId": "<configured revision>" }`. Check it
against the intended image, then make the same requests through your ingress.
The candle request must receive a valid upstream UDF config; build identity
alone does not exercise the proxy Origin header.

## Routine operations

### Deploy a new version

Build a new revision-tagged image with the required public inputs. Retain the
previous image tag and its matching environment file outside version control.
During a maintenance window, stop/remove the old container and run the new
image with the current environment:

```bash
docker stop perps-frontend
docker rm perps-frontend
# Repeat the docker run command above with the new perps_release.
```

This direct-container procedure has downtime. There is no checked-in automatic
image retagging, rolling update, health-wait deploy script or rollback trigger.
An operator-managed multi-instance ingress can provide a different lifecycle.

### Roll back

Replace the container using the retained image tag and its matching runtime
environment, then check build identity and candle proxy again. Restoring only
the runtime env cannot undo browser configuration embedded in a newer image.

### Tail logs

```bash
docker logs -f --tail 100 perps-frontend
```

The direct-container example uses the Docker daemon's logging defaults; set
rotation in your infrastructure. The supplied nginx Compose service explicitly
uses `json-file` rotation at 50 MB × 5 files.

### Apply server-only environment changes

Edit the runtime file, stop/remove the container, and repeat `docker run` with
the **same image** and the updated `--env-file`. Check the candle endpoint
again. For a host-managed process, supply the updated environment through its
service manager and restart that process; the fixed-path PM2 example requires
adaptation before use. Public bundle changes always require a new build.

## Troubleshooting

### Compose says there is no `app` service

That matches the current file: its app definition is comments. Commands such
as `docker compose ... build app`, `logs app`, or inspection of an assumed
`hyperpax-app` container cannot operate the frontend in this checkout.

### nginx fails to start or returns `502 Bad Gateway`

Check that every mounted nginx file, certificate and DH parameter file exists
and that the configured upstream matches a running, reachable frontend. An
`app:3000` upstream cannot resolve to a Compose app service absent from the
file. Container-to-host connectivity also requires a listener reachable from
the bridge; the loopback example above is for host ingress.

### Candle proxy returns 503 or fails to load

With an unset `MARKET_PROXY_ORIGIN`, the route returns HTTP 503 with
`{ "error": "MARKET_PROXY_ORIGIN is not configured" }`. A nonempty malformed
origin can throw when the route module loads. Fix the origin format and
replace the container with the corrected environment. HTTP 504 means the
upstream timed out; HTTP 502 means it was unreachable. See the
[API guide](../data_api.md) for the chart/data-gateway distinction.

### Embedded-wallet sign-in is missing

Check that both Supabase public values were passed to the build. Docker's
`.dockerignore` excludes `.env.production`, and runtime `--env-file` cannot
repair a previously built client bundle. Rebuild with the correct public
arguments and replace the image. There is no `deploy.sh --no-cache` command in
this checkout; Docker's own `build --no-cache` option is available when needed.

### Build identity reports `development`

Set the runtime `VERCEL_GIT_COMMIT_SHA` or `RAILWAY_GIT_COMMIT_SHA` for the image
being launched. `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` is a separate public build
argument and does not by itself set the server-only variable this endpoint
reads. An arbitrary configured identity is not independent image attestation.

## Deployment responsibilities

TLS, ingress, firewall rules, secret/environment backups, image retention,
health monitoring and service-manager configuration belong to the deployment.
The repository assets describe parts of those systems and do not provision a
complete production stack. Back up `apps/perps/.env.production` and external
ingress certificates in your approved secret storage; do not commit them.
