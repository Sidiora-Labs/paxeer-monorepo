# Kindle SDK

`@kindle/sdk` is the private TypeScript client for launchpad HTTP operations and snapshot/replay streams. Its entry point is [src/index.ts](src/index.ts); v1 DTOs are exported there, and versioned v2 DTOs come from `@kindle/sdk-v2/types`.

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). The kernel is part of the network and proposes blocks; a transaction receipt is final when its block commits. Sub-chains are tenant domains in the database. The SDK's API, identity, media and stream origins identify launchpad application services. For chain JSON-RPC, use the root path of the public node hosts, such as `https://api1.mainnet-beta.paxeer.network`, described in the [public RPC reference](../../../../docs/site/docs/reference/public-rpc.md). Do not use a node JSON-RPC origin as the launchpad API origin.

## Contracts and generation

Keep generated DTOs, schemas and operation definitions under generator control. [scripts/generate.py](scripts/generate.py) expects authoritative bundles at `launchpad/spec/production-system/integration/contracts/v1/` and `v2/`, relative to the repository root. Each bundle must contain `codegen-manifest.json`, `schema-definitions.json`, `openapi.json` and `asyncapi.json`. Those bundle directories are absent from this checkout, so reproducibility cannot currently be established by running the generator. Obtain the matching authoritative inputs before regenerating; the checked-in schemas and [release-manifest.json](release-manifest.json) alone do not supply them.

The pinned emitter and validator template live at [tools/launchpad/production/interfaces](../../../../tools/launchpad/production/interfaces/). Generation checks their digests, schema inputs, output inventory and output digests. Frozen v1 reproduction uses `python3 scripts/generate.py --reproduce-v1`; plain `generate` does not select that mode. `generate:operations` selects major 2 and derives the operation catalog from OpenAPI 3.1, checking shared definitions against JSON Schema and AsyncAPI 3.0. `generate:examples` selects the v2 OpenAPI document. Both require the missing v2 bundle. Explicit v2 DTO generation uses `python3 scripts/generate.py --major 2` and writes to the sibling `sdk-v2` package.

[scripts/compatibility.py](scripts/compatibility.py) compares schemas. Breaking changes require a new major and an explicit window with matching `fromMajor`/`toMajor`, ordered timezone-aware `startsAt`/`endsAt`, and both majors in `retainedVersions`. A major increase alone does not establish compatibility. The former phase scripts and v2 evidence-manifest declaration were removed from [package.json](package.json).

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

[src/served-routes.ts](src/served-routes.ts) records route availability: unrouted operations fail before a request, and development-only operations fail in production. Operations requiring a dedicated streaming transport also refuse ordinary calls. `CapabilityUnavailableError` has code `capability_unavailable` and defaults to status 501; the transport also uses it for HTTP 501 or an undocumented 404. `HttpError` retains status, stable code and a validated error envelope's request ID in `envelope.requestId`. Server message/details are sanitized. Redirects and non-JSON success bodies produce `UNEXPECTED_REDIRECT` and `NON_JSON_RESPONSE`; exhausted network interruption produces `StreamTransportError('NETWORK_INTERRUPTED')`.

## Launched chain reads

`Chains` reads the launched chain registry and revenue through the same `SdkClient`. Each response is validated before it is returned. Chain numbers and amounts are decimal strings. The pool detail type includes `pool.homeChain`, with `"0"` for a root network market.

```ts
import { Chains, SdkClient } from '@kindle/sdk';

const chains = new Chains(new SdkClient(transportConfig));
const page = await chains.list({ limit: 20 });
const detail = await chains.get(page.data.items[0].chain);
const revenue = await chains.revenue(detail.data.chain.chain, { holder: walletAddress, limit: 20 });
```

Use `nextCursor` from a page as the next call's `cursor` to continue the list or revenue history.

## Snapshots, streams and amounts

`SnapshotStreamV2` installs a complete snapshot with watermarks before applying buffered replay events at an acknowledged checkpoint. `installSnapshot` and `applyEvent` must commit synchronously; returning a Promise is rejected. Failed store effects do not advance client positions. Sequences and monetary values remain decimal strings on the wire; sequence comparisons and amount formatting use `BigInt`. [Amount helpers](src/amounts.ts) keep `Pax18` and `Usdl6` separate, require explicit display rounding and optionally carry generated price provenance.

`browserStreamTransportV2` uses `GET /v2/snapshots`, WebSocket `/v2/stream` and subprotocol `sidiora.v2`. Authenticated attempts obtain a fresh ticket through identity `POST /v2/stream/tickets` and send it as a WebSocket subprotocol, never a URL credential. Public subscriptions do not request tickets. The adapter pings every 4 seconds after subscribing.

[ReconnectingStream](src/reconnect.ts) keeps cursors in memory and retries with full jitter from zero up to an exponentially increasing ceiling, initially 500 milliseconds and capped at 30 seconds. The attempt counter resets after 30 seconds live. Reconnect may reuse an acknowledged cursor within the same authorization scope for up to 72 hours; visibility restoration always fetches a fresh snapshot. Authentication failures stop automatic retries. Exhausting the default 15 attempts opens the circuit; `retry()` explicitly resumes it. Logout clears cursor state.

The shared browser helpers are application code: [acquireCanonicalDataStream](../../../../apps/launchpad/web/src/core/realtime/data-stream.ts) and [useCanonicalDataStream](../../../../apps/launchpad/web/src/core/realtime/use-data-stream.ts). They share a connection per client, stream scope and authorization scope, with at most 20 topics in the union. Empty subscriptions are rejected; exceeding 20 topics raises `TopicConnectionPolicyError`. Immutable fills are delivered individually; replaceable price, statistics, candle and portfolio state can be coalesced per animation frame. Consumer errors are isolated, and cleanup releases its own subscription. Authorization changes clear subscriptions and cursor state. The SDK exports the v2 browser adapter; its v1 browser adapter was removed.

## Package commands and proof requirements

The package belongs to the pnpm workspace at [apps/launchpad](../../../../apps/launchpad/pnpm-workspace.yaml). Run these commands from that workspace after installing its dependencies:

```sh
pnpm --filter @kindle/sdk-v2 build
pnpm --filter @kindle/sdk build
```

`build` compiles standalone validators from the checked-in v1/v2 schemas and operation definitions, then runs TypeScript. It does not regenerate contracts. `test` runs that build and `node --test test/*.test.mjs`, including connected-service tests. It requires the real fixture below; it does not run the Python generation/compatibility checks or the TSX browser lifecycle file. A passing isolated primitive test is not proof of durable service behavior.

For connected tests and examples, set `SDK_CONNECTED_CONFIG` to an absolute path to the fixture owner's real-service JSON configuration. [test/connected-services.test.mjs](test/connected-services.test.mjs) requires `realm: "private_fixture"`, a 64-character lowercase hexadecimal `sourceSha256`, `applicationOrigin` using HTTPS, `transport` matching `TransportConfig`, actual `accessToken`, `authenticated`, and canonical topic/pool identities. Its fields include `topics`, `publicTopics`, `privateTopics`, `tokenATopics`, `tokenBTopics`, `tokenAPool`, `tokenBPool` and `wrongOrigin`. Examples additionally use `poolAddress` for pagination; the browser lifecycle proof uses `authorizationScope`, `tradeTopics` and `candleTopics`.

The fixture's `commands` map must supply each provider requested by the proof. Entries have executable `argv`, `executableSha256` and optional `env`; the tests hash the actual executable and parse its JSON output. Providers cover snapshot/ack barriers and fill insertion, delayed reads/snapshots, gateway restart and subsequent fills, retained-history expiry/session revocation, and `emit100Fills` for the browser proof. Use the named test file as the exact provider/result contract. Missing inputs fail rather than returning empty success.

After building, run the connected proof directly from this SDK directory with `node --test test/connected-services.test.mjs`, or run the package's complete `test` script with the same fixture. [test/browser-lifecycle.test.tsx](test/browser-lifecycle.test.tsx) exercises React StrictMode and consumer ownership against that fixture. It requires a Vitest setup with React, a DOM environment and explicit inclusion of this SDK test path; the web application's default `src/**/*.test.*` include does not discover it. There is no package script wrapping that browser proof. Python generation and compatibility checks remain separate files under [test](test/); generation still requires the authoritative bundles described above.

The checked-in examples consume compiled `dist` output. From this SDK directory, with the fixture configured:

```sh
node examples/pagination.mjs
node examples/snapshot-resume.mjs
SDK_RESUME=1 node examples/snapshot-resume.mjs
```

[pagination.mjs](examples/pagination.mjs) checks advancing cursors and duplicate trade identities. [snapshot-resume.mjs](examples/snapshot-resume.mjs) supports public subscriptions or fresh authenticated tickets; `SDK_RESUME=1` additionally invokes `restartGateway` and `fillAfterGatewayRestart` and checks the resumed payload. Running these examples or the SDK proofs is separate from qualifying a documentation-only change.
