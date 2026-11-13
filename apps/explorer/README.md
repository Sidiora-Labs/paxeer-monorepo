# Paxeer X Network Explorer frontend

This is the frontend of the Paxeer X Network block explorer: a fork of the
Blockscout Next.js frontend. Source attribution is in
[`docs/explorer/UPSTREAM.md`](../../docs/explorer/UPSTREAM.md). It reads the API of the
explorer backend in [`services/explorer/backend`](../../services/explorer/backend) and is served under the
`/explorer` base path set in `next.config.js`.

## Running and configuring the app

The app is distributed as a Docker image built from
`docker/explorer-frontend/Dockerfile` at the repository root and published by
`.github/workflows/explorer-images.yml` as
`ghcr.io/sidiora-labs/paxeer-x-explorer-frontend`. It is configured through
environment variables passed when the container starts; see the full list of
ENVs and their description [here](../../docs/explorer/ENVS.md).

```sh
docker run -p 3000:3000 --env-file <path-to-your-env-file> ghcr.io/sidiora-labs/paxeer-x-explorer-frontend:latest
```

To build from source, use the Node version from `.nvmrc`:

```sh
corepack enable
yarn install --frozen-lockfile
yarn build
```

`yarn lint:eslint` and `yarn lint:tsc` run ESLint and the TypeScript check.
`yarn test:vitest` starts Vitest in watch mode; `yarn test:vitest:ci` runs
the unit suite once. Upstream's guide to building a custom image
is kept in [`../../docs/explorer/CUSTOM_BUILD.md`](../../docs/explorer/CUSTOM_BUILD.md).

## Paxeer X

Two run-time ENV presets for the Paxeer X Network explorer are tracked under [`configs/envs`](./configs/envs):

- `paxeer-x.env` — the public deployment (https, wss).
- `paxeer-x-dev.env` — the development deployment (http, ws).

Both describe network `Paxeer X Network` (short name `Paxeer X`, network id `125`, native coin `Paxeer` (`PAX`), 18 decimals) and carry the Paxeer X marks from [`public/static/paxeer-x`](./public/static/paxeer-x). Their navigation links and font resources are declared in the respective presets. `paxeer-x.env` sets a remote footer configuration URL; the local footer data is in [`configs/paxeer-x/footer.json`](./configs/paxeer-x/footer.json). Marketplace and ad providers are disabled in both presets.

Using a preset:

1. Copy it to a dotfile name the container entrypoint understands, e.g. `cp configs/envs/paxeer-x.env configs/envs/.env.paxeer-x`, and build or run with `ENVS_PRESET=paxeer-x`.
2. In a development preset copy, replace `REPLACE_API_HOST` with the Blockscout API host and `REPLACE_APP_HOST` with the explorer host. The entrypoint excludes `NEXT_PUBLIC_APP_PROTOCOL`, `NEXT_PUBLIC_APP_HOST`, `NEXT_PUBLIC_APP_PORT`, `NEXT_PUBLIC_APP_ENV` and `NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL` from preset loading. Supply those values through the container environment, including `wss` for a secure API WebSocket connection or `ws` for a local HTTP deployment.

Validating a preset without a full build:

```sh
cd deploy/tools/envs-validator
yarn install --frozen-lockfile
NEXT_PUBLIC_GIT_COMMIT_SHA=$(git rev-parse --short HEAD) NEXT_PUBLIC_GIT_TAG=$(git describe --tags --always --abbrev=0) ../../scripts/collect_envs.sh ../../../../../docs/explorer/ENVS.md
yarn build
./node_modules/.bin/dotenv -e ../../../configs/envs/paxeer-x.env yarn run validate
```

## Contributing

Upstream's [contribution guide](../../docs/explorer/CONTRIBUTING.md) and [code of conduct](../../docs/explorer/CODE_OF_CONDUCT.md) are kept in `docs/explorer/` at the monorepo root.

## Resources

- [App ENVs list](../../docs/explorer/ENVS.md)
- [Contribution guide](../../docs/explorer/CONTRIBUTING.md)
- [Making a custom build](../../docs/explorer/CUSTOM_BUILD.md)

## License

GNU General Public License v3.0. See [LICENSE](LICENSE).
