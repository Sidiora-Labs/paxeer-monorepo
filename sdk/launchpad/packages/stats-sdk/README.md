# stats-sdk

TypeScript client for pool statistics from
[`services/launchpad/stats-workers`](../../../../services/launchpad/stats-workers/internal/httpapi/pool_stats.go).
The [entry point](src/index.ts) exports `StatsHttpClient`, the shared `httpClient`,
response parsers, error classes and `PoolStats` types.

Configure `httpClient.configure({ baseUrl })` with the service base URL.
`getPoolStats(poolAddress)` requests `GET /stats/{poolAddress}`;
`getPoolStatsBatch(pools)` requests `GET /stats/batch?pools=...` and returns
statistics keyed by pool address. A batch must contain at least one valid
20-byte EVM address. Missing pools follow the server's response.

Monetary and decimal fields remain strings. HTTP failures retain status and
response body; malformed JSON or incompatible DTOs reject. Requests use a
ten-second default timeout, configurable from 1 to 120000 milliseconds, and
accept a caller `AbortSignal`. Response body limits are 64 KiB for one pool,
1 MiB for batches and 16 KiB for errors. Larger bodies raise
`StatsResponseTooLargeError`.

The package has no runtime dependencies and uses TypeScript 5.9.3 to emit
JavaScript and declarations into `dist`. From the
[launchpad workspace](../../../../apps/launchpad/pnpm-workspace.yaml):

```sh
pnpm --filter stats-sdk build
pnpm --filter stats-sdk test
```

Tests use Node's built-in test runner against loopback HTTP servers and the
built SDK. Build before running them.
