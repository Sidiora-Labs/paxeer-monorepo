# Launchpad SDK

This directory holds TypeScript clients, Go contract bindings and shared Go libraries for the launchpad applications and services. They connect market creation, trading, indexing and application streams to Paxeer mainnet-beta with EVM chain ID 125. The LayerX kernel inside paxd executes and orders each transaction and builds each block; validators check and commit it. A chain receipt is final when the validators commit its block; launchpad application APIs and streams are served by the application services, while chain JSON-RPC uses the node root path.

## Layout

- [packages/](packages/) — TypeScript HTTP and stream clients, generated API types and schemas, and the contract client: `sdk`, `sdk-v2`, `production-contracts`, `production-contracts-v2`, `sidiora-launchpad-v3`, `candles-sdk`, `indexer-sdk`, `metadata-sdk`, `ranking-algo-sdk`, `stats-sdk` and `users-sdk`.
- [protocol/](protocol/) — independent Go module containing the deployment address book, contract ABIs and bindings, event definitions, ABI catalog and coverage tools.
- [shared/](shared/) — independent Go module providing configuration, chain reads, authentication, accounting, database access, delivery, HTTP helpers, queues, Redis, secrets, storage and telemetry for launchpad services.

## Build and test

The Go modules are included in [services/launchpad/Makefile](../../services/launchpad/Makefile). From the repository root, its targets build and test both SDK modules and the launchpad service modules:

```sh
make -C services/launchpad build
make -C services/launchpad test
```

The TypeScript packages belong to the [launchpad pnpm workspace](../../apps/launchpad/pnpm-workspace.yaml). After installing its dependencies, run the workspace's `build:packages` script from `apps/launchpad`:

```sh
pnpm run build:packages
```

This script builds every package under `sdk/launchpad/packages`. Each package also declares `npm run build` in its own `package.json`. Run the following commands from the relevant package directory:

| Package directory under `packages/` | Build | Test |
| --- | --- | --- |
| `candles-sdk` | `npm run build` | `npm test` |
| `indexer-sdk` | `npm run build` | `npm test` |
| `metadata-sdk` | `npm run build` | `npm test` |
| `production-contracts` | `npm run build` | `npm test` |
| `production-contracts-v2` | `npm run build` | No test script declared. |
| `ranking-algo-sdk` | `npm run build` | `npm test` |
| `sdk` | `npm run build` | `npm test` |
| `sdk-v2` | `npm run build` | No test script declared. |
| `sidiora-launchpad-v3` | `npm run build` | `npm test` |
| `stats-sdk` | `npm run build` | `npm test` |
| `users-sdk` | `npm run build` | `npm test` |

Build `sdk-v2` before `sdk`, which imports its generated types. The `sdk` test script includes connected-service tests that require `SDK_CONNECTED_CONFIG`; see its [package guide](packages/sdk/README.md) for the configuration and contract-bundle requirements. Its `generate`, `generate:operations` and `generate:examples` scripts generate contracts, operations and examples separately from the build.

## Configuration

[shared/config/env.go](shared/config/env.go) reads and validates these environment variables:

| Purpose | Names |
| --- | --- |
| Database and Redis | `DATABASE_URL`, `REDIS_URL`, `REDIS_BULL_URL` |
| Chain connection | `CHAIN_ID`, `RPC_URL`, `RPC_URL_FALLBACK` |
| Contract identities | `EVENT_EMITTER_ADDRESS`, `POOL_REGISTRY_ADDRESS`, `ROUTER_ADDRESS`, `FACTORY_ADDRESS`, `QUOTER_ADDRESS`, `PROTOCOL_CONFIG_ADDRESS`, `FEE_ACCUMULATOR_ADDRESS`, `SIDIORA_NFT_ADDRESS`, `FEES_ROUTER_ADDRESS`, `POOL_BEACON_ADDRESS` |
| Service runtime | `LOG_LEVEL`, `NODE_ENV`, `PORT`, `DEPLOYMENT_REALM` |
| Private fixture selection | `PRIVATE_FIXTURE_MANIFEST_FILE`, `PRIVATE_FIXTURE_MANIFEST_SHA256` |
| Secret provider and refresh | `SECRETS_PROVIDER`, `SECRETS_REFRESH_SECONDS` |

[shared/http/virusscan.go](shared/http/virusscan.go) reads `CLAMAV_HOST` and `CLAMAV_PORT`. The secret provider reads environment entries by the names supplied by its callers. The SDK's connected examples read the configuration file named by `SDK_CONNECTED_CONFIG`; `SDK_RESUME` selects the snapshot-resume example's resumption flow.

Deployment and capability data are embedded from [shared/config/deployment/current.json](shared/config/deployment/current.json), the JSON files in [shared/config/deployment/history/](shared/config/deployment/history/), [shared/config/capabilities/frozen.json](shared/config/capabilities/frozen.json) and [shared/config/capabilities/contract-support.json](shared/config/capabilities/contract-support.json). The protocol embeds [catalog/reviewed.json](protocol/catalog/reviewed.json), [catalog/native_launchpad.json](protocol/catalog/native_launchpad.json) and [coverage/authorities.json](protocol/coverage/authorities.json).

The protocol coverage measurement command reads `L02_MEASUREMENT_RPC_URL`. The shared capability measurement command reads the environment variable named by `urlVariable` in its endpoint input. The contract deployment helper reads `CONTRACT_SOURCE_ROOT`, `CLIENT_DEPLOYMENT_HELPER_OUTPUT` and `CLIENT_SOURCE_ROOT`.

TypeScript clients receive their service origins and deployment information through constructor options. The main SDK's [TransportConfig](packages/sdk/src/transport.ts) supplies `apiOrigin`, `identityOrigin`, `mediaOrigin`, `streamOrigin`, `approvedStreamOrigins`, `production` and optional session settings. TypeScript builds use each package's `package.json` and, where present, `tsconfig.json`.

## Documentation

- [Launch a market and chain](../../docs/launchpad/chain-launch.md)
- [Launchpad web configuration checklist](../../docs/launchpad/production-checklist.md)
- [Launchpad documentation site page](../../docs/site/paxeer-docs/src/app/launchpad/page.tsx)
- [Launchpad site Markdown guide](../../docs/site/paxeer-docs/public/markdown/launchpad.md)
- [Public node RPC reference](../../docs/readme/PUBLIC-RPC.md)
- [Main TypeScript SDK](packages/sdk/README.md), [indexer SDK](packages/indexer-sdk/README.md) and [statistics SDK](packages/stats-sdk/README.md)
