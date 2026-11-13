# Kindle SDK

`@kindle/sdk` is the private TypeScript client for launchpad HTTP operations and snapshot/replay streams. Its entry point is [src/index.ts](src/index.ts); v1 DTOs are exported there, and versioned v2 DTOs come from `@kindle/sdk-v2/types`.

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). The kernel is part of the network and proposes blocks; a transaction receipt is final when its block commits. Sub-chains are tenant domains in the database. The SDK's API, identity, media and stream origins identify launchpad application services. For chain JSON-RPC, use the root path of a public node, described in the [public RPC reference](../../../../docs/site/docs/reference/public-rpc.md). Do not use a node JSON-RPC origin as the launchpad API origin.

## Contracts and generation

Keep generated DTOs, schemas and operation definitions under generator control. [scripts/generate.py](scripts/generate.py) consumes authoritative v1 and v2 contract bundles. Each bundle must contain `codegen-manifest.json`, `schema-definitions.json`, `openapi.json` and `asyncapi.json`. Select the matching bundle with `--contracts <directory>`. The checked-in schemas and [release-manifest.json](release-manifest.json) describe the generated package.

The pinned emitter and validator template live at [tools/launchpad/production/interfaces](../../../../tools/launchpad/production/interfaces/). Generation checks their digests, schema inputs, output inventory and output digests. Frozen v1 reproduction uses `python3 scripts/generate.py --reproduce-v1`; plain `generate` does not select that mode. `generate:operations` selects major 2 and derives the operation catalog from OpenAPI 3.1, checking shared definitions against JSON Schema and AsyncAPI 3.0. `generate:examples` selects the v2 OpenAPI document. Both require the v2 bundle. Explicit v2 DTO generation uses `python3 scripts/generate.py --major 2` and writes to the sibling `sdk-v2` package.

[scripts/compatibility.py](scripts/compatibility.py) compares schemas. Breaking changes require a new major and an explicit window with matching `fromMajor`/`toMajor`, ordered timezone-aware `startsAt`/`endsAt`, and both majors in `retainedVersions`. A major increase alone does not establish compatibility.

## Client and operations

Construct an `SdkClient` with `apiOrigin`, `identityOrigin`, `mediaOrigin`, `streamOrigin`, `approvedStreamOrigins` and `production`. Origins must have no credentials, query, fragment or path beyond `/`. Production requires HTTPS/WSS, rejects local/internal hosts and IP literals, and requires the stream origin in the allowlist. See [src/transport.ts](src/transport.ts) for the exact validation rules.

Bearer sessions use `{ mode: 'bearer', token: () => accessToken }`; authenticated requests send the token in `Authorization`. Cookie sessions use `{ mode: 'cookie', applicationOrigin, csrfToken: () => csrfToken }`. The application and API origins must match, and cookie-authenticated destinations must be same-origin. Authenticated cookie mutations require `X-CSRF-Token`; login and refresh metadata explicitly enable cookie credentials. The deployment front door must route each operation to its actual service.

```ts
import { Operations, SdkClient } from '@kindle/sdk';

// transportConfig is the deployment's TransportConfig; poolAddress is its pool.
const client = new SdkClient(transportConfig);
const operations = new Operations(client);
const trades = await operations.call(
  'IF_TRADES',
  { path: { poolAddress }, query: { limit: 100 } },
  { signal: AbortSignal.timeout(30_000) },
);
```

[Operations](src/operations.ts) validates path/query parameters, declared headers, JSON bodies and responses. Generated metadata selects wire version, service and authentication. Default operation timeouts are at least 8 seconds for reads and 10 seconds for mutations, or the contract budget when larger. A caller's `timeoutMs` overrides these defaults; the transport accepts 1–60,000 milliseconds, and an `AbortSignal` can cancel earlier.

Only GET operations marked as idempotent reads retry network interruption or eligible HTTP 503 responses, at most twice. Mutations are not retried. `Idempotency-Key` is accepted only when the operation declares it. Public GETs share simultaneous in-flight requests within an `Operations` instance; each consumer receives a copy, and one consumer's cancellation leaves other consumers active. Private reads bypass that sharing.

[src/served-routes.ts](src/served-routes.ts) records route availability: unrouted operations fail before a request, and development-only operations fail in production. Operations requiring a dedicated streaming transport also refuse ordinary calls. `CapabilityUnavailableError` has code `capability_unavailable` and defaults to status 501; the transport also uses it for HTTP 501 or an undocumented 404. `HttpError` retains status, stable code and a validated error envelope's request ID in `envelope.requestId`. Server messages and details are sanitized. Redirects and non-JSON success bodies produce `UNEXPECTED_REDIRECT` and `NON_JSON_RESPONSE`; exhausted network interruption produces `StreamTransportError('NETWORK_INTERRUPTED')`.

## Launched chain reads

`Chains` reads the launched chain registry and revenue through the same `SdkClient`. Each response is validated before it is returned. Chain numbers and amounts are decimal strings. The pool detail type includes `pool.homeChain`, with `"0"` for a root network market.

```ts
import { Chains, SdkClient } from '@kindle/sdk';

const chains = new Chains(new SdkClient(transportConfig));
const page = await chains.list({ limit: 20 });
const first = page.data.items[0];
if (!first) throw new Error('No launched chains returned');
const detail = await chains.get(first.chain);
const revenue = await chains.revenue(detail.data.chain.chain, { holder: walletAddress, limit: 20 });
```

Use `nextCursor` from a page as the next call's `cursor` to continue the list or revenue history.

## Snapshots, streams and amounts

`SnapshotStreamV2` installs a complete snapshot with watermarks before applying buffered replay events at an acknowledged checkpoint. `installSnapshot` and `applyEvent` must commit synchronously; returning a Promise is rejected. Failed store effects do not advance client positions. Sequences and monetary values remain decimal strings on the wire; sequence comparisons and amount formatting use `BigInt`. [Amount helpers](src/amounts.ts) keep `Pax18` and `Usdl6` separate, require explicit display rounding and optionally carry generated price provenance.

`browserStreamTransportV2` uses `GET /v2/snapshots`, WebSocket `/v2/stream` and subprotocol `sidiora.v2`. Authenticated attempts obtain a fresh ticket through identity `POST /v2/stream/tickets` and send it as a WebSocket subprotocol, never a URL credential. Public subscriptions do not request tickets. The adapter pings every 4 seconds after subscribing.

[ReconnectingStream](src/reconnect.ts) keeps cursors in memory and retries with full jitter from zero up to an exponentially increasing ceiling, initially 500 milliseconds and capped at 30 seconds. The attempt counter resets after 30 seconds live. Reconnect may reuse an acknowledged cursor within the same authorization scope for up to 72 hours; visibility restoration always fetches a fresh snapshot. Authentication failures stop automatic retries. Exhausting the default 15 attempts opens the circuit; `retry()` explicitly resumes it. Logout clears cursor state.

The shared browser helpers are application code: [acquireCanonicalDataStream](../../../../apps/launchpad/web/src/core/realtime/data-stream.ts) and [useCanonicalDataStream](../../../../apps/launchpad/web/src/core/realtime/use-data-stream.ts). They share a connection per client, stream scope and authorization scope, with at most 20 topics in the union. Empty subscriptions are rejected; exceeding 20 topics raises `TopicConnectionPolicyError`. Immutable fills are delivered individually; replaceable price, statistics, candle and portfolio state can be coalesced per animation frame. Consumer errors are isolated, and cleanup releases its own subscription. Authorization changes clear subscriptions and cursor state. The SDK exports the v2 browser adapter.

## Package commands and examples

The package belongs to the pnpm workspace at [apps/launchpad](../../../../apps/launchpad/pnpm-workspace.yaml).
After installing dependencies there, build the v2 dependency and this package:

```sh
pnpm --filter @kindle/sdk-v2 build
pnpm --filter @kindle/sdk build
```

`build` compiles standalone validators from the checked-in v1/v2 schemas and
operation definitions, then runs TypeScript. Contract generation is a separate
command. The `test` script builds the package and runs `node --test test/*.test.mjs`.
Connected-service tests require `SDK_CONNECTED_CONFIG` to name the deployment's
service configuration; see [connected-services.test.mjs](test/connected-services.test.mjs)
for its required fields. Generation tests require the contract bundles described
above.

The examples consume compiled `dist` output. From this SDK directory, with
`SDK_CONNECTED_CONFIG` configured:

```sh
node examples/pagination.mjs
node examples/snapshot-resume.mjs
SDK_RESUME=1 node examples/snapshot-resume.mjs
```

[pagination.mjs](examples/pagination.mjs) follows trade pages and checks cursor
advancement and duplicate identities. [snapshot-resume.mjs](examples/snapshot-resume.mjs)
supports public subscriptions and authenticated stream tickets. `SDK_RESUME=1`
selects the example's gateway restart and stream-resumption flow.
