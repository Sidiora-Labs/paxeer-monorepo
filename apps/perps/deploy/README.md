# Hyperpax-UI — Production Deployment

This directory supplies a Compose frontend service and nginx ingress for
`apps/perps`. The [`docker-compose.yml`](docker-compose.yml) builds the
standalone Next.js image from the application directory, passes server-only
configuration to `app`, and connects `app` and `nginx` on one private bridge.
The supported procedure below uses this Compose file throughout.

See the [data API guide](../../../docs/perps/data-api.md) for the application gateway protocol,
[workspace guide](../README.md) for UI preferences, and
[web application](../apps/web) for live widget integration.

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
deployment, and review the remaining public values. Compose forwards public
build arguments and server-only runtime values explicitly. Set
`BUILD_COMMIT_SHA` to the full image revision in `.env.production` before
building. Supply the nginx configuration and TLS assets described below
before starting the stack. All Compose commands below run from `apps/perps`,
use `--env-file .env.production` and explicitly select
`-f deploy/docker-compose.yml`. Shell variables override the env file during
Compose interpolation; clear stale exported values before deploying.

## Architecture

```text
browser → nginx HTTPS ingress → app:3000 (private Compose bridge)
                                             ├─ /api/build-id
                                             └─ /api/candle/* → configured gateway origin
browser → perps data gateway (REST / WebSocket / GraphQL)
app /api/price-feed/* → authenticated price feed
browser / wallet → Paxeer node JSON-RPC at the node root
```

Paxeer mainnet-beta is one chain, EVM chain ID **125 (`0x7d`)**. The LayerX kernel
inside paxd executes and orders each transaction and builds each block;
validators check and commit it. A receipt is final when the validators commit
its block. Public JSON-RPC
uses the node root URLs declared in [`core/rpc.ts`](../packages/paxeer-backend/src/core/rpc.ts). The data gateway origin is a separate
application configuration, not another chain or finality stage. Sub-chains are
tenant domains in the database and do not require a frontend-specific node.

The nginx service publishes ports 80 and 443. It reaches the frontend using
`app:3000` on the shared `internal` bridge; the app publishes no host ports.
The bridge allows outbound access to the data gateway and candle feed.
nginx waits for the app's image health check on initial startup. Set the trusted
proxy peer lists only for ingress peers that overwrite `X-Real-IP`; otherwise
the routes use the socket peer for client identity. After an app
recreation, nginx must resolve its new address. In the operator-supplied TLS
server's `location /`, use the DNS resolver reachable from nginx and a variable
upstream. Read the nameserver from the nginx container's `/etc/resolv.conf` and substitute
it for `<resolver-address>` below:

```nginx
resolver <resolver-address> valid=5s ipv6=off;
set $perps_upstream app:3000;
proxy_pass http://$perps_upstream;
proxy_set_header Host $host;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Real-IP $remote_addr;
```

Include these directives inside the appropriate nginx context; they are not a
complete TLS server configuration. Preserve the request URI with the
`proxy_pass` form above. Configure the HTTP server to redirect to HTTPS and
supply certificates for your hostname. Allow up to five seconds for DNS
refresh after replacement, then repeat the ingress health checks.

## Files

| File                                                     | Current purpose                                                                                                 |
| -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| [`../Dockerfile`](../Dockerfile)                         | Digest-pinned Node and charting stages, pnpm install, standalone build, non-root runtime and image health check |
| [`docker-compose.yml`](docker-compose.yml)               | app build/runtime configuration, shared bridge and health-ordered nginx startup                                 |
| [`.env.production.example`](.env.production.example)     | Public build inputs, server runtime inputs and explicit image revision                                          |
| [`build-args.production.env`](build-args.production.env) | Public build-argument values; Docker does not automatically read an env file as build arguments                 |

The Compose mounts require operator-supplied nginx config, certificates,
ACME webroot and DH parameters. Starting this
Compose file alone is not a production deployment procedure.

## Step-by-step

### 1. Provision the box

Provision Docker/BuildKit, access to required images, storage, logging and
Docker daemon restart management through your infrastructure process. This
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

| Variable                                                                                    | Contract                                                                                                                               |
| ------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL`                                                             | Required bare HTTP(S) origin. Parsed at module load; missing or malformed values throw. Frontend derives `/api`, `/ws` and `/graphql`. |
| `NEXT_PUBLIC_PAXEER_RPC_URL`                                                                | Primary root-path node JSON-RPC URL.                                                                                                   |
| `NEXT_PUBLIC_PAXEER_RPC_URLS`                                                               | Optional comma-separated fallback list; see [`core/rpc.ts`](../packages/paxeer-backend/src/core/rpc.ts) for selection.                 |
| `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS`                                                     | Comma-separated browser origins allowed to use the public price feed directly.                                                         |
| `NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER`                                                   | `1` disables live streams for render checks. Blank keeps live streams enabled.                                                         |
| `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`                          | Embedded-wallet sign-in configuration; when either is blank in the bundle, that sign-in section is hidden.                             |
| `NEXT_PUBLIC_PAXEER_WALLET_API`                                                             | Embedded-wallet API origin.                                                                                                            |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`                         | Optional auth callback and trusted wallet messaging origin configuration.                                                              |
| `NEXT_PUBLIC_PAXSCAN_API_URL`, `NEXT_PUBLIC_PAXSCAN_WS_URL`, `NEXT_PUBLIC_PAXSCAN_BASE_URL` | Explorer integration and transaction links.                                                                                            |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN`                                                             | `1` opens the spot route/navigation/markets mode. Blank keeps the release gate closed.                                                 |
| `NEXT_PUBLIC_RELEASE`                                                                       | Optional client release identifier included in error reports.                                                                          |
| `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`                                          | Optional browser error reporting destination and environment.                                                                          |
| `PRICE_FEED_ORIGIN`                                                                         | Server origin used by `/api/price-feed`; the SDK default is used when blank.                                                           |
| `PRICE_FEED_REST_API_KEY`, `PRICE_FEED_WS_API_KEY`                                          | Server-only price-feed REST and WebSocket token credentials.                                                                           |
| `CANDLE_TRUSTED_PROXY_IPS`, `PRESENCE_TRUSTED_PROXY_IPS`                                    | Comma-separated socket peer IPs trusted to supply `X-Real-IP`; configure ingress to overwrite this header.                             |
| `NEXT_PUBLIC_LOG_LEVEL`                                                                     | Browser logger minimum level.                                                                                                          |
| `MARKET_PROXY_ORIGIN`                                                                       | Required server upstream Origin header: exact bare HTTP(S) origin, with no trailing slash, path, credentials, query or fragment.       |
| `VERCEL_GIT_COMMIT_SHA` or `RAILWAY_GIT_COMMIT_SHA`                                         | Runtime build identity for `/api/build-id`; Railway takes precedence. Unset values report `development`.                               |

The candle route forwards allowlisted requests to the gateway origin from the
public build settings. The price-feed route uses its server origin and credentials.

The optional precompile overrides are validated in
[`core/network/precompiles.ts`](../packages/paxeer-backend/src/core/network/precompiles.ts). Each must
be `0x` followed by exactly 40 hexadecimal digits; blank uses the release
address. These suffixes are shown without their leading zero padding:

| Override                              | Release address suffix |
| ------------------------------------- | ---------------------- |
| `NEXT_PUBLIC_PERPS_PRECOMPILE`        | `0x1015`               |
| `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE`    | `0x1017`               |
| `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` | `0x1021`               |
| `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` | `0x1022`               |
| `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE`   | `0x1023`               |
| `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` | `0x1024`               |
| `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE`  | `0x1025`               |

All `NEXT_PUBLIC_*` values are visible in browser code and are baked into the
bundle. Changing them requires a rebuild with the corresponding Dockerfile
build arguments and replacement of the running image. Supabase publishable
keys are client identifiers; private keys, service-role credentials and
signing secrets must never be public build inputs.

Server-only values must reach the actual server process. The Dockerfile does
not embed `PRICE_FEED_ORIGIN` or `MARKET_PROXY_ORIGIN`. Pass them at container
creation. A plain `docker restart` or `docker compose --env-file ... restart`
retains the existing container environment; it does not load edited values.
Apply edited server values to the configured `app` service using
`up -d --no-build --pull never --force-recreate --wait app` with the same image
revision, as shown under routine operations. Compose refuses an empty
`MARKET_PROXY_ORIGIN` before starting the service. This validation does not
replace the route's bare-origin validation or the candle health check.

### 3. Build the frontend image

From `apps/perps`, set `BUILD_COMMIT_SHA` in `.env.production` to the
revision being built (`git rev-parse HEAD`). Use a new revision for a new
bundle; retain the previous image and matching environment file for rollback.

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml build app
```

Compose's build context is `..` relative to the file in `deploy`, so it uses
`apps/perps/Dockerfile`, the application source and its `.dockerignore`.
The image tag and runtime build identity both use `BUILD_COMMIT_SHA`. The env file configures interpolation; it is not copied
into the image or injected wholesale at runtime.

Public settings are declared once each as sorted Docker build arguments and
forwarded by Compose. `build-args.production.env` supplies public values to
the image publishing script. The production build takes
`NEXT_PUBLIC_PERPS_GATEWAY_URL=https://perps-api.paxeer.app` from that file.
Another deployment sets its own gateway origin; `.env.production.example`
leaves this value empty. Optional blank arguments use application defaults.

The Dockerfile installs the whole workspace with its frozen pnpm lockfile and
manifest-pinned package manager. It copies the charting library and datafeeds
from the digest-pinned charting image, requires its standalone script and
builds the web application into `apps/web/.next-production`. The runner uses
UID 1001 and `tini`, listens on port 3000 and starts `node apps/web/server.js`.
It contains the workspace standalone output, web static output and public assets.
Railway uses `railway.json` at the workspace root and probes `/api/build-id`;
its web project variables supply the same server-only settings as Compose.
The Vercel builder targets `apps/web/package.json` and the production output.

### 4. Start the stack

With the nginx mounts and TLS assets provisioned:

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml up -d --no-build --pull never --wait
```

The app receives the explicit Compose `environment` values at container
creation; client-visible variables still come from the built bundle.
The image health check requests `/api/build-id` every 15 seconds with a
5-second timeout, 20-second start period and three retries. A healthy response
proves the Next.js endpoint is available; check the candle proxy separately.
`--no-build --pull never` ensures startup uses the locally built image.

### 5. Verify before routing traffic

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml ps
docker compose --env-file .env.production -f deploy/docker-compose.yml logs --tail 100 app
docker compose --env-file .env.production -f deploy/docker-compose.yml exec -T app wget -qO- http://127.0.0.1:3000/api/build-id
```

`/api/build-id` returns `{ "buildId": "<configured revision>" }`; compare it
with the intended image revision. To verify the proxy origin in the running
container and exercise the actual candle route, replace the example origin
below with the exact intended `MARKET_PROXY_ORIGIN` from your runtime file:

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml exec -T app node -e '
if (process.env.MARKET_PROXY_ORIGIN !== process.argv[1]) {
  throw new Error("Running app has the wrong MARKET_PROXY_ORIGIN");
}
fetch("http://127.0.0.1:3000/api/candle/config").then(async response => {
  if (!response.ok) throw new Error(`Candle proxy HTTP ${response.status}`);
  const config = await response.json();
  if (!Array.isArray(config.supported_resolutions) || !config.supported_resolutions.length) {
    throw new Error("Invalid upstream UDF config");
  }
  console.log("Candle proxy origin and upstream config verified");
}).catch(error => { console.error(error); process.exit(1); });
' '<deployment-origin>'
```

The route reads the same runtime variable and sends it as the upstream
`Origin` header. This check must succeed after startup, environment updates
and rollback; build identity alone does not exercise that header. Verify both
endpoints through the actual HTTPS ingress as well (substitute your hostname):

```bash
curl --fail-with-body "<deployment-origin>/api/build-id"
curl --fail-with-body "<deployment-origin>/api/candle/config"
```

Check the ingress UDF config and intended build identity before routing users.

## Routine operations

### Deploy a new version

Retain the previous image tag and its matching environment file outside
version control. Update `.env.production` for the new revision and public
inputs, build `app` as above, then recreate it:

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml up -d --no-build --pull never --force-recreate --wait app
```

Repeat all build-identity, proxy-origin and HTTPS checks above. This
single-instance replacement has downtime; there is no automatic rolling
update or rollback trigger. nginx's Docker DNS resolver follows the new app
address without requiring an nginx restart.

### Roll back

Restore the retained environment file to `.env.production`, including its
`BUILD_COMMIT_SHA`, `MARKET_PROXY_ORIGIN`, `PRICE_FEED_ORIGIN` and public build
inputs. Retain that revision-tagged image locally; do not rebuild it or prune
it before the rollback window closes. Then run:

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml up -d --no-build --pull never --force-recreate --wait app
```

Repeat all build-identity, proxy-origin and HTTPS checks above. Restoring only
the runtime env cannot undo browser configuration embedded in a newer image.

### Tail logs

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml logs -f --tail 100 app
```

Both Compose services use `json-file` rotation at 50 MB × 5 files.

### Apply server-only environment changes

Edit `MARKET_PROXY_ORIGIN` or `PRICE_FEED_ORIGIN` in `.env.production`, keep
`BUILD_COMMIT_SHA` unchanged to use the **same image**, and recreate the app:

```bash
docker compose --env-file .env.production -f deploy/docker-compose.yml up -d --no-build --pull never --force-recreate --wait app
```

Repeat the running-container proxy-origin check with the new expected origin,
the build-identity check and both HTTPS checks above. A plain `restart`
retains the existing container environment. Public `NEXT_PUBLIC_*` bundle
changes always require a rebuild and replacement with a new image revision.

## Troubleshooting

### Compose says there is no `app` service

Run from `apps/perps` and explicitly select `-f deploy/docker-compose.yml`.
The checked-in file defines `app` and `nginx`; another Compose file may not.

### nginx fails to start or returns `502 Bad Gateway`

Check that every mounted nginx file, certificate and DH parameter file exists
and that the TLS server proxies to `app:3000` using the Docker DNS resolver
shown above. Both services must join `internal`. Check `app` health and logs
using the same Compose file and environment file. After app recreation,
allow the resolver refresh period before checking again.

### Candle proxy returns 503 or fails to load

With an unset `MARKET_PROXY_ORIGIN`, the route returns HTTP 503 with
`{ "error": "MARKET_PROXY_ORIGIN is not configured" }`. A nonempty malformed
origin can throw when the route module loads. Fix the origin format and
replace the container with the corrected environment. HTTP 504 means the
upstream timed out; HTTP 502 means it was unreachable. See the
[API guide](../../../docs/perps/data-api.md) for the chart/data-gateway distinction.

### Embedded-wallet sign-in is missing

Check that both Supabase public values were passed to the build. Docker's
`.dockerignore` excludes `.env.production`, and runtime `--env-file` cannot
repair a previously built client bundle. Rebuild with the correct public
arguments and replace the image. Docker's `build --no-cache` option is available
when needed.

### Build identity reports `development`

Set `BUILD_COMMIT_SHA` in the Compose env file for the image being launched;
Compose forwards it as runtime `VERCEL_GIT_COMMIT_SHA`.
`RAILWAY_GIT_COMMIT_SHA` takes precedence when Railway supplies it.

## Deployment responsibilities

TLS, ingress, firewall rules, secret/environment backups, image retention,
health monitoring and service-manager configuration belong to the deployment.
The repository assets describe parts of those systems and do not provision a
complete production stack. Back up `apps/perps/.env.production` and external
ingress certificates in your approved secret storage; do not commit them.
