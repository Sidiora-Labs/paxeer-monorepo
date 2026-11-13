# indexer-sdk

Configure `httpClient` with the stats-workers service base URL before calling
`getRecentTrades(poolAddress)`. An independently configured `HttpClient` exposes
the same operation as `recentTrades(poolAddress)`.

Both methods GET `/stats/{poolAddress}/transactions` and adapt the service's
`transactions` array to `{ pool: poolAddress, trades }`. The service's default
page contains up to 50 transactions, newest first; an empty page returns
`trades: []`. Amounts, prices and fees remain decimal strings. Invalid transaction
responses are rejected, and HTTP errors and abort signals propagate to callers.

Requests use a ten-second default deadline, configurable through `timeoutMs`
from 1 to 120000 integer milliseconds. Successful response bodies are limited to
1 MiB and error bodies to 16 KiB; larger bodies raise `ResponseTooLargeError`.

The [source entry point](src/index.ts) exports `HttpClient`, `httpClient`,
`getRecentTrades`, `HttpError`, `ResponseTooLargeError` and the response types.
From the launchpad workspace, build with `pnpm --filter indexer-sdk build`.
`pnpm --filter indexer-sdk test` builds the package before running its tests.
