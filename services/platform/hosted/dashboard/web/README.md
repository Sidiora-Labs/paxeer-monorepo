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
  --build-arg DASHBOARD_API_ORIGIN=https://dashboard-api.example.com \
  --tag layerx-dashboard-web:local .
```

The shared image builder reads the same input from its environment and forwards
it as a Docker build argument. It rejects missing, empty or whitespace-containing
values because its consumers split the argument list on whitespace. For example,
from the repository root:

```bash
set -euo pipefail
export DASHBOARD_API_ORIGIN=https://dashboard-api.example.com
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
  --build-arg DASHBOARD_API_ORIGIN=https://dashboard-api.example.com
```
