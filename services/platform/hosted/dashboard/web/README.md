# Dashboard web image

`DASHBOARD_API_ORIGIN` is a required, non-secret build-time input for the
production dashboard image. Next embeds it in the rewrite from
`/v1/dashboard/:path*` to `${DASHBOARD_API_ORIGIN}/v1/dashboard/:path*` (with
trailing slashes removed from the origin). Changing the origin requires
rebuilding and deploying the image; setting a runtime environment variable
cannot replace the compiled rewrite. Production builds reject missing or empty
origins. The loopback fallback in `next.config.ts` is only for development.

Run the canonical Dockerfile from the repository root with an explicit argument:

```sh
docker build --file docker/platform-dashboard-web/Dockerfile \
  --build-arg DASHBOARD_API_ORIGIN="$DASHBOARD_API_ORIGIN" \
  --tag layerx-dashboard-web:local .
```

The shared image builder reads the same input from its environment and forwards
it as a Docker build argument. It rejects missing, empty or whitespace-containing
values because its consumers split the argument list on whitespace. For example,
from the repository root:

```bash
set -euo pipefail
: "${DASHBOARD_API_ORIGIN:?set the dashboard API origin}"
source services/platform/hosted/tests/beta-images.sh
build_args=$(image_build_args layerx-dashboard-web)
docker build --file docker/platform-dashboard-web/Dockerfile \
  $build_args --tag layerx-dashboard-web:local .
```

For image publication, configure the repository Actions variable
`DASHBOARD_API_ORIGIN`. The planning step in
`.github/workflows/publish-images.yml` forwards `${{ vars.DASHBOARD_API_ORIGIN }}`
through the shared helper to the matrix image's `build-args`. There is no default;
an unset or invalid variable fails planning.

The Fly configuration at `services/platform/hosted/dashboard/web/fly.toml` also
selects `docker/platform-dashboard-web/Dockerfile`. Pass the origin explicitly
when deploying from the repository root, keeping the repository as build context:

```sh
fly deploy . --config services/platform/hosted/dashboard/web/fly.toml \
  --build-arg DASHBOARD_API_ORIGIN="$DASHBOARD_API_ORIGIN"
```

## Styles

The style sheet is built from the dashboard app files and the `src` directory
of the UI package. The build checks the generated sheet for the UI classes.

## Browser sign-in

The page presents sign-in when a dashboard read returns 401. A developer can
create an Ed25519 key in this browser and sign in, use the key this browser
holds, or enter a session token. The key pair lives under one fixed name in
IndexedDB; its private key is non-exportable. The public key has a copy control.
The gateway tenant field takes the tenant configured by the gateway operator;
the registration binding and session digest include that exact tenant.
Browsers without Ed25519 offer the session token path with an explanation.
A successful sign-in reads the dashboard again. The shell's sign-out control
returns to the sign-in view.

The web serves these routes beside the page:

| Route | JSON input and result |
| --- | --- |
| `POST /session/registration` | `signer_public_key` and `registration_signature`; calls `px_register` and returns the confirmed public tenant, subject and signer key |
| `POST /session` | `signer_public_key`, canonical decimal `proof_time` and `session_signature`; calls `px_openSession`. Alternatively, `token` alone checks `GET /v1/dashboard/overview` with Bearer authorization. Success returns 204 with no body. |
| `DELETE /session` | Empty body or `{}`; returns 204 and clears the cookie |

Requests require an Origin matching either the parsed HTTPS origin of the Host
header, including its port, or the origin of the request URL for direct access.
An absent, multi-value or malformed Host header supplies no HTTPS origin.
Missing or non-matching Origins return 403 with `origin_refused` before the
body is read.

Requests require `Content-Type: application/json`. Bodies are bounded at
8192 bytes. Upstream calls have a five-second time limit. Refusals return only a
`reason` name; rejected proofs return 401,
gateway limits return 429 and other upstream failures return 502.

`DASHBOARD_GATEWAY_ORIGIN` is a server runtime variable naming the gateway
origin. Without it, proof sign-in and registration return 503 with
`sign_in_not_configured`; the view explains that key sign-in is not configured.
Token sign-in and sign-out remain available. The API origin is the same
build-time `DASHBOARD_API_ORIGIN` used by the dashboard rewrite.

Successful sign-in sets `__Host-layerx-session` with `Secure`, `HttpOnly`,
`SameSite=Strict`, `Path=/` and `Max-Age`, without a Domain attribute.
The proof session lifetime ends at the gateway's `expires_at`; a token accepted
by the API has a twelve-hour cookie lifetime. Sign-out sets an empty cookie with
the same attributes and `Max-Age=0`. The cookie requires HTTPS. Session and CSRF
tokens stay out of response bodies and error text; the session token is sent to
the browser only in the HttpOnly cookie.
