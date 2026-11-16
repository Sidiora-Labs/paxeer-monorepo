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

## Local dashboard

Start the dashboard from the repository root after installing its dependencies:

```sh
npm --prefix services/platform/hosted/dashboard/web run dev
```

For a production build, set `DASHBOARD_API_ORIGIN` and run the dashboard's
`build` script, then its `start` script. The production build clears generated
development output before generating production route types.

## Styles

The style sheet is built from the dashboard app files and the `src` directory
of the UI package. The build checks the generated sheet for the UI classes. The dashboard uses
shared colour, spacing, type and motion tokens, follows the system colour
scheme and respects reduced motion and browser text scaling. Section tabs
scroll within their track and support arrow keys, Home and End. Log rows wrap
identifiers and adapt to their card's width. Sign-in forms use native constraints
and show field errors after interaction.

## Browser sign-in

The page presents sign-in when a dashboard read returns 401. A developer can
create an Ed25519 key in this browser and sign in, use the key this browser
holds, or enter a session token. The key pair lives under one fixed name in
IndexedDB; its private key is non-exportable. The public key has a copy control.
When key sign-in is configured, the page shows one primary action: it creates a
key and signs in when this browser has no key, or signs in with the held key.
Browsers without Ed25519 offer the session token path with an explanation. If
the gateway tenant or origin is absent, the page says that key sign-in is not
configured. The session token field and action remain available in every state.
A successful sign-in reads the dashboard again. The shell's sign-out control
returns to the sign-in view.

The web serves these routes beside the page:

| Route                        | JSON input and result                                                                                                                                                                                                              |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /session/registration` | `signer_public_key` and `registration_signature`; calls `px_register` and returns the confirmed public tenant, subject and signer key                                                                                              |
| `POST /session`              | `signer_public_key`, canonical decimal `proof_time` and `session_signature`; calls `px_openSession`. Alternatively, `token` alone checks `GET /v1/dashboard/overview` with Bearer authorization. Success returns 204 with no body. |
| `DELETE /session`            | Empty body or `{}`; returns 204 and clears the cookie                                                                                                                                                                              |
| `POST /keys`                 | `signer_public_key`, sorted non-empty `scopes`, `quota_requests` (1–1,000,000) and `quota_window_seconds` (1–2,592,000), plus `Idempotency-Key`; returns the gateway key and one-time secret with `Cache-Control: no-store` |
| `POST /keys/{id}/rotate`     | Empty JSON body plus `Idempotency-Key`; returns the replacement gateway key and one-time secret with `Cache-Control: no-store` |
| `DELETE /keys/{id}`          | Empty JSON body; revokes the owned key and returns its gateway result |

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
origin. `DASHBOARD_GATEWAY_TENANT` is also read by the server at request time;
a value is accepted only when it has 1 to 95 characters from `a-z`, `0-9`, `.`,
`_` and `-`. Invalid or absent values are treated as unset. The tenant is not
compiled into the build, so a process started with a new value serves that value
without rebuilding. Registration bindings and session digests include that runtime
tenant. Without either runtime setting, proof sign-in and
registration return 503 with `sign_in_not_configured`; the view explains that
key sign-in is not configured. Token sign-in and sign-out remain available. The
API origin is the same build-time `DASHBOARD_API_ORIGIN` used by the dashboard
rewrite.

Successful sign-in sets `__Host-layerx-session` with `Secure`, `HttpOnly`,
`SameSite=Strict`, `Path=/` and `Max-Age`, without a Domain attribute.
The proof session lifetime ends at the gateway's `expires_at`; a token accepted
by the API has a twelve-hour cookie lifetime. Sign-out sets an empty cookie with
the same attributes and `Max-Age=0`. The cookie requires HTTPS. Session and CSRF
tokens stay out of response bodies and error text; the session token is sent to
the browser only in the HttpOnly cookie.

The Keys section signs an issue request with the Ed25519 public key held by the
browser key of the current session, offers all six labelled gateway scopes and
bounded quota fields, and provides rotate and confirmation-protected revoke
controls for active keys. When this browser has no developer key, the view
explains that it cannot sign issuance and keeps the Issue key control disabled.
Creating a key during sign-in updates the view immediately, without a page
reload. Refusals are shown in words from the gateway reason so the developer
can correct the request or try again.
Each write uses a fresh idempotency key and refreshes the dashboard after
success. An issued or rotated secret is held only in page memory and shown once
in a dialog with a copy control; closing the dialog removes it from the page.
