---
name: deploy-demo
description: Run the explorer frontend image with a configured environment
disable-model-invocation: true
---
# Explorer frontend deployment

The explorer frontend runs as the standalone Next.js container described in the
[application guide](../../../README.md#running-and-configuring-the-app).
The [Explorer / Images workflow](../../../../../.github/workflows/explorer-images.yml)
builds its image from the monorepo root with
[docker/explorer-frontend/Dockerfile](../../../../../docker/explorer-frontend/Dockerfile).
It publishes images on matching pushes to `main` and on manual dispatch.
Pull requests run the workflow's backend tests; image publication is restricted
to events other than pull requests.

## Container configuration

The application uses the `/explorer` base path and port 3000. Supply a deployment
environment file using the variables in the
[environment reference](../../../../../docs/explorer/ENVS.md). The
[Paxeer environment files](../../../configs/envs) define chain ID 125 and branding.
Use `paxeer-x.env` or `paxeer-x-dev.env` as the basis for the file passed to
`--env-file`, with values appropriate to the deployment.

```sh
docker run -p 3000:3000 --env-file <path-to-your-env-file> ghcr.io/sidiora-labs/paxeer-x-explorer-frontend:latest
```

The image entry point prepares runtime assets and configuration before starting
the frontend. Public node JSON-RPC uses a node's root URL; the explorer backend
HTTP API is a separate application service configured by `NEXT_PUBLIC_API_*`.
