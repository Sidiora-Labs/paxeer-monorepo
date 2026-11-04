Blockscout client
===

This Rust client consumes the explorer REST API. Its OpenAPI inputs are in
[swaggers](swaggers), its generator templates in
[rust-template-handlebars](rust-template-handlebars), and its Rust crate in
[crate](crate). Generation uses [OpenAPI Generator](https://openapi-generator.tech).
The crate belongs to the `services/explorer/services/libs` Cargo workspace.

The producer is the local Paxeer explorer backend, particularly
[ApiRouter](../../../backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex)
and its controllers and views. REST URLs use the explorer origin; the chain's
root-path JSON-RPC endpoints are described in
[EthereumJSONRPC](../../../backend/apps/ethereum_jsonrpc/README.md). See also
the [account API](../../../backend/apps/block_scout_web/API.md) and the
[database migration runbook](../blockscout-db/migration/README.md).

## Installation

Add the crate by path under `[dependencies]`:

```toml
blockscout-client = { path = "<path to>/services/explorer/services/libs/blockscout-client/crate" }
```

Set `Configuration::builder().base_path(...)` to an explicit explorer REST
origin (scheme, host and optional port), without credentials, query, fragment,
`/api` or `/api/v2`. There is no usable default origin; generated operations
append their full API paths. For example, with a backend listening locally:

```rust
use blockscout_client::{ApiClient, Configuration};

let client = ApiClient::new(
    Configuration::builder().base_path("http://127.0.0.1:4000").build(),
);
```

Supply the root node JSON-RPC origin separately through `node_rpc_origin` if
needed, and validate its `eth_chainId` against the deployment (Paxeer: `0x7d`).
A REST origin does not establish chain identity. The client does not discover
the node origin through `/api/v2/config/json-rpc-url`.

## Generating the client

The authoritative input is `GET /api/v2/openapi` from the target local Paxeer
backend at the revision being integrated, checked against its mounted routes,
controllers and response views. In particular,
[Specs.Public](../../../backend/apps/block_scout_web/lib/block_scout_web/specs/public.ex)
sets a server URL ending in `/api`, while schema paths are relative to that
prefix. The generator expects an origin and full paths such as `/api/health`
and `/api/v2/transactions`.

Use Yarn (the package declares Yarn 1.22.22), Node, Docker and Rust. Run these
commands from this directory against a controlled local backend. `REST_ORIGIN`
is an example shell variable for this procedure, not an environment variable
automatically read by the Rust client or the recorder.

```sh
yarn install --frozen-lockfile
export REST_ORIGIN=http://127.0.0.1:4000
curl --fail --show-error --silent "$REST_ORIGIN/api/v2/openapi" \
  --output swaggers/paxeer-local-openapi.json
node <<'JS'
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { stringify } = require('yaml');
const origin = new URL(process.env.REST_ORIGIN);
assert.equal(origin.href, `${origin.origin}/`);
const spec = JSON.parse(fs.readFileSync('swaggers/paxeer-local-openapi.json', 'utf8'));
assert.ok(spec.openapi && spec.components && spec.paths);
assert.ok(spec.servers.length > 0);
for (const server of spec.servers) {
  assert.equal(new URL(server.url, origin).pathname.replace(/\/$/, ''), '/api');
}
const paths = {};
for (const [path, item] of Object.entries(spec.paths)) {
  assert.ok(path.startsWith('/') && !path.startsWith('/api/'));
  paths[`/api${path}`] = item;
}
assert.ok(paths['/api/health'] && paths['/api/v2/transactions']);
spec.paths = paths;
spec.servers = [{ url: origin.origin }];
fs.writeFileSync('swaggers/blockscout-api-v2.yaml', stringify(spec));
JS
```

Keep the raw schema snapshot with the backend revision as provenance. Before
merging, review every YAML file in `swaggers/overrides` against that producer;
`yarn merge` deep-merges them in sorted order and removes properties marked
`__REMOVE__`. Old overrides can overwrite correct local schemas, reintroduce
unmounted routes or erase native variants. The health override described below
must be aligned before regeneration. Do not assume upstream operation names or
schema names match the local document: review generated API names and callers.

```sh
yarn merge
# Review swaggers/blockscout-api-final.yaml against the raw local schema,
# routes and views before continuing, including native/EVM/null/error variants.
yarn generate
```

`yarn generate` invokes Docker OpenAPI Generator with the final schema,
[openapi-generator-config.json](openapi-generator-config.json) and the local
[rust-template-handlebars](rust-template-handlebars), writing `crate`.
Format changed Rust files with the repository's Rust formatter (`yarn format`
formats the whole workspace). Review the resulting diff and
[crate/.openapi-generator-ignore](crate/.openapi-generator-ignore); configuration,
transport and handwritten codecs are not protected by that ignore file. Preserve
explicit origins, bounded HTTP, strict decoding and native discriminators.
Generation success alone does not prove producer compatibility.

`yarn getV2` downloads the upstream Blockscout Swagger and replaces
`swaggers/blockscout-api-v2.yaml`. `yarn generate-from-remote` chains that
replacement, merge, generation and formatting. Reserve these commands for an
intentional upstream refresh followed by the local-schema procedure above;
neither fetches the Paxeer backend schema.

### Health contract

Call `GET /api/health` using `client.health_api().health()`. The obsolete
`/api/v1/health` operation remains in the generator override and generated
`health_v1()` method, but is not mounted by the current backend. Do not use it
as a supported health check.

[HealthController](../../../backend/apps/block_scout_web/lib/block_scout_web/controllers/api/health_controller.ex)
returns HTTP 200 when block indexing is healthy and HTTP 500 otherwise. For
the Paxeer indexer/API deployment the healthy body has this structure
(numbers and timestamps here are illustrative):

```json
{
  "healthy": true,
  "metadata": {
    "blocks": {
      "healthy": true,
      "latest_block": {
        "db": {"number": "123", "timestamp": "2026-01-01 00:00:00Z"},
        "cache": {"number": "123", "timestamp": "2026-01-01 00:00:00Z"},
        "node": {"number": "123"}
      }
    }
  },
  "error": null
}
```

Heights and timestamps are strings. Unhealthy responses include
`metadata.blocks.error` and a top-level `error` containing `code` and `message`.
Standalone media worker mode instead returns `{"healthy":true,"data":{}}`.

The current Rust model supports `HealthCheckResponse.metadata.blocks` and
`LatestBlock.node`. However,
[swaggers/overrides/002_health.yaml](swaggers/overrides/002_health.yaml) still
uses the old `metadata.latest_block` shape, spells the cache height `mber`,
lacks the node height and HTTP 500 schema, and carries the unmounted v1
operation. Align the inputs with the producer above before regeneration so it
does not discard the current Rust model corrections.

## Tests

The existing [it_works.rs](crate/tests/it_works.rs) suite starts Stubr against
`tests/recorded/eth_blockscout_com`. These are **upstream samples**, not evidence
that the current Paxeer producer agrees with the client. Its legacy v1 health
case is unsupported by the current backend.
[repair_22_31_273.rs](crate/tests/repair_22_31_273.rs) checks model codecs.
[repair_22_31_274.rs](crate/tests/repair_22_31_274.rs) exercises client transport
with recordings and explicitly labeled producer-view examples.
[repair_22_33_430.rs](../tests/repair_22_33_430.rs) is registered in the crate
manifest and tests the documented origin, healthy/error handling and mixed
native/EVM feed through the production `ApiClient` over loopback HTTP. Its
fixtures distinguish upstream recordings from source-derived examples. Run its
focused offline check from the repository root:

```sh
cargo test --locked --manifest-path services/explorer/services/libs/Cargo.toml --test repair_22_33_430
```

These offline checks need no credentials, live backend or external toolchain
process. They do not execute the Elixir producer or certify a deployment.

For actual-producer validation, start the target backend revision against a
controlled indexed database containing both an EVM transaction and a native
price-feed submission. Point the real `ApiClient` at that backend's explicit
REST origin and compare its raw status/body with both typed consumers
(`try_as_success` and `try_parse_as_success_model`). Use hashes from that
database, not identifiers copied from upstream recordings. Check:

- `GET /api/v2/transactions` and `GET /api/v2/main-page/transactions`: retain
  both EVM transactions and native type `127` items. Assert native `to: null`,
  zero value/fee, `submitPrices`, observation count, order, symbols (including
  null for unknown markets), exact integer price strings and terminal or next
  cursors. Compare with
  [FeedView.prepare_update](../../../backend/apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/feed_view.ex)
  and [TransactionView](../../../backend/apps/block_scout_web/lib/block_scout_web/views/api/v2/transaction_view.ex).
- `GET /api/health`: assert HTTP 200 and
  `metadata.blocks.latest_block` with DB, cache and node heights. In an isolated
  database, exercise unhealthy indexing and assert HTTP 500, the error code and
  message, and the original body through `Error::ResponseError`.
- Exercise empty pages (`items: []`, `next_page_params: null`), nullable fields,
  invalid and missing resource responses with their actual 4xx status/body,
  and pagination without losing filters or native items. A status 200 response
  with an undecodable body must surface `Error::Decode`, not an empty success.

Record the backend revision, database scenario, schema snapshot, request path
and query, actual status/content type/body, and assertions for each case.
Sanitize recordings before committing them; only this actual-producer run can
establish agreement with that backend revision. The offline gate makes no claim
that this operational validation has run.

### Recording responses

[ts-src/record-stubs.ts](ts-src/record-stubs.ts) currently hardcodes an upstream
host, constructs HTTPS URLs and writes every response with status 200. Running
`yarn record-stubs` unchanged therefore records upstream examples and loses
actual error statuses. It has no environment-based local-origin switch.

For a recorder adapted to the local producer, use mounted paths such as:

```ts
const paths = [
  "/api/health",
  "/api/v2/blocks",
];
```

Capture the real local response status, body and relevant content type,
including HTTP 500 health failures and resource errors, rather than rewriting
errors to 200. Keep the exact request path and query parameters. Use sanitized
local recordings as supporting fixtures and retain the live contract check;
remote examples cannot prove the Paxeer producer's response contract.
