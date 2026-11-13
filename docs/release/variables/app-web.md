# Web application variables

The Human web application in `apps/web` serves the account, activity, explorer, and market pages. Its server reads the environment at runtime; `NEXT_PUBLIC_PAXEER_X_EXPLORER_BASE_URL` is included in the browser bundle at build time. The tables list deployment settings and configuration fields read by the application's dependency policy. Configuration names identify the file, section, and key; `*` denotes each entry in a package map. Defaults are those applied by the reader, rather than values currently stored in a configuration file.

## Required

The Human service and web origins are required for authenticated server requests. Dependency-policy fields describe required build inputs; package metadata requirements apply to the corresponding root, local-package, or installed-package entries.

| Name | What it sets | Source default | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |
| `LAYERX_HUMAN_SERVICE_URL` | HTTPS Human service origin; accepts the root or a trailing `/human` path, which is normalized to the origin. Credentials, queries, and fragments are rejected. | None | No | `apps/web/src/api/human-origin.ts` |
| `LAYERX_HUMAN_WEB_ORIGIN` | HTTPS web application origin for server requests and performance-ingestion origin checks. | None | No | `apps/web/src/api/server.ts`; `apps/web/src/app/api/performance/vitals/route.ts` |
| `apps/web/package-lock.json` · `packages.*` · `dependencies` | Local package dependency metadata, compared with its package manifest. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages.*` · `license` | Package license checked against the approved licenses. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages.*` · `name` | Identity of an approved local package. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages.*` · `resolved` | Target of an approved local package link. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages.*` · `version` | Pinned package version; approved local versions are checked for equality. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages[""]` · `dependencies` | Pinned root runtime dependency map, compared with the manifest. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · `packages[""]` · `devDependencies` | Pinned root development dependency map, compared with the manifest. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package-lock.json` · root · `lockfileVersion` | Lockfile format accepted by the dependency policy. | None; must be `3` | No | `apps/web/policy/check.mjs` |
| `apps/web/package.json` · root · `dependencies` | Direct runtime dependencies, compared with the approved dependency map and lockfile. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/package.json` · root · `devDependencies` | Development dependencies, compared with the lockfile. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/packages/layerx-ui/package.json` · root · `dependencies` | UI package dependency metadata, compared with the lockfile. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/packages/layerx-ui/package.json` · root · `license` | Approved UI package license. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/packages/layerx-ui/package.json` · root · `name` | Approved UI package identity. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/packages/layerx-ui/package.json` · root · `version` | Approved UI package version. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/policy/runtime-dependencies.json` · root · `approved` | Approved direct runtime dependency names and version or local-path specifications. Every map entry is compared with `package.json` · root · `dependencies`. | None | No | `apps/web/policy/check.mjs` |
| `apps/web/policy/runtime-dependencies.json` · root · `maximumDirectRuntimeDependencies` | Positive integer limiting the number of direct runtime dependencies. | None | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `dependencies` | Wallet package dependency metadata, compared with the web lockfile. | None | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `license` | Approved wallet package license. | None | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `name` | Approved wallet package identity. | None | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `private` | Wallet package publication restriction; must be enabled. | None | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `version` | Approved wallet package version. | None | No | `apps/web/policy/check.mjs` |
| `services/agent/sdk/typescript/package.json` · root · `dependencies` | Agent SDK dependency metadata, compared with the web lockfile. | None | No | `apps/web/policy/check.mjs` |
| `services/agent/sdk/typescript/package.json` · root · `license` | Approved agent SDK license. | None | No | `apps/web/policy/check.mjs` |
| `services/agent/sdk/typescript/package.json` · root · `name` | Approved agent SDK identity. | None | No | `apps/web/policy/check.mjs` |
| `services/agent/sdk/typescript/package.json` · root · `version` | Approved agent SDK version. | None | No | `apps/web/policy/check.mjs` |

## Optional

These settings control individual features, execution modes, and optional package metadata. Feature descriptions state when values must be supplied. The quote URL, chain ID, sponsor, and paymaster must be configured together for gas sponsorship. Server and browser explorer origins are configured separately. Network access uses the merged Paxeer network; node endpoint overrides name the node root path.

| Name | What it sets | Source default | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |
| `LAYERX_GATEWAY_URL` | Endpoint for hosted gateway operations and EVM calls selected by the gateway client. Accepts HTTPS, or loopback HTTP, without credentials, query, or fragment. | None; required for those operations | No | `apps/web/src/api/gateway.ts` |
| `LAYERX_NODE_ENDPOINT` | Root endpoint for the node-method set selected by the gateway client. Accepts HTTPS, or loopback HTTP, without credentials, query, fragment, or route suffix. | `DEFAULT_NODE_ENDPOINT` in the reader, the built-in mainnet-beta node root | No | `apps/web/src/api/gateway.ts` |
| `LAYERX_RUM_DEVELOPMENT_FALLBACK` | Enables in-memory performance observations only when exactly `1`, `NODE_ENV` is `development`, and no storage directory is configured. | None; disabled when unset | No | `apps/web/src/api/performance-sink.ts` |
| `LAYERX_RUM_STORAGE_DIRECTORY` | Durable performance-observation directory; must be an absolute, normalized path other than the filesystem root. Required for durable collection. | None; collection is unconfigured unless the development fallback applies | No | `apps/web/src/api/performance-sink.ts` |
| `LAYERX_SUPPORT_REPORT_BEARER_TOKEN` | Bearer credential for support-report retrieval; at least 32 characters after trimming. | None; required for retrieval | Yes | `apps/web/src/server/support-reports.ts` |
| `LAYERX_SUPPORT_REPORT_DIR` | Absolute support-report storage directory other than the filesystem root. | None; required for saving or retrieving reports | No | `apps/web/src/server/support-reports.ts` |
| `LAYERX_SUPPORT_REPORT_RETENTION` | Maximum retained reports per principal; integer from `1` through `10000`. | `1000` when unset or blank | No | `apps/web/src/server/support-reports.ts` |
| `NEXT_PUBLIC_PAXEER_X_EXPLORER_BASE_URL` | Browser-visible explorer origin for activity evidence links. Accepts HTTPS, or loopback HTTP, with no credentials, path, query, or fragment. | None; explorer evidence links require an origin | No; public build-time setting | `apps/web/src/journeys/activity/detail.tsx` |
| `NODE_ENV` | Enables development CSP behavior and, with its opt-in setting, in-memory performance storage when equal to `development`. | No application default; supplied by Next.js | No | `apps/web/src/proxy.ts`; `apps/web/src/api/performance-sink.ts` |
| `PATH` | Inherited command search path used by the UI build script after prepending the web application's local binary directory. | Inherited process value | No | `apps/web/package.json` · `scripts` · `build:ui` |
| `PAXEER_GAS_STATION_CHAIN_ID` | Positive decimal chain ID for gas quotes and sponsored batches. | None; sponsorship is unavailable without valid station configuration | No | `apps/web/src/api/gas-station.ts` |
| `PAXEER_GAS_STATION_PAYMASTER` | Paymaster contract identifier for gas sponsorship; a 20-byte hexadecimal identifier. | None | No | `apps/web/src/api/gas-station.ts` |
| `PAXEER_GAS_STATION_QUOTE_URL` | HTTPS gas-quote endpoint without credentials or a fragment. | None | No | `apps/web/src/api/gas-station.ts` |
| `PAXEER_GAS_STATION_SPONSOR` | Sponsor identifier for gas sponsorship; a 20-byte hexadecimal identifier. | None | No | `apps/web/src/api/gas-station.ts` |
| `PAXEER_GAS_STATION_SUBMIT_URL` | HTTPS endpoint for submitting sponsored batches, without credentials or a fragment. | None; required for station submission | No | `apps/web/src/api/gas-station.ts` |
| `PAXEER_X_EXPLORER_BASE_URL` | Server-side explorer origin for native API reads and explorer links. Accepts HTTPS, or loopback HTTP, with no credentials, path, query, or fragment. | None; required for explorer reads and links | No | `apps/web/src/explorer/client.ts` |
| `apps/web/package-lock.json` · `packages.*` · `link` | Whether a package entry is a local link; approved local link entries must set it to `true`. | None; absence is treated as an ordinary package entry | No | `apps/web/policy/check.mjs` |
| `apps/web/packages/layerx-ui/package.json` · root · `link` | UI package metadata must describe the package itself; `true` is rejected. | None; absence is accepted | No | `apps/web/policy/check.mjs` |
| `sdk/wallet/package.json` · root · `link` | Wallet package metadata must describe the package itself; `true` is rejected. | None; absence is accepted | No | `apps/web/policy/check.mjs` |
| `services/agent/sdk/typescript/package.json` · root · `link` | Agent SDK metadata must describe the package itself; `true` is rejected. | None; absence is accepted | No | `apps/web/policy/check.mjs` |
