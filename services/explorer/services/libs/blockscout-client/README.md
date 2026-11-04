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

Set `Configuration::builder().base_path(...)` to the explorer origin without
an `/api/v2` suffix; generated operations append their full API paths.

## Generating the client

Use Yarn (the package declares Yarn 1.22.22), Docker and Rust. Run generator
commands from this directory. The scripts in [package.json](package.json) form
this pipeline:

1. `yarn getV2` downloads upstream Swagger and patches server URLs and paths.
2. `yarn merge` applies YAML files in `swaggers/overrides` in sorted order and
   removes properties marked `__REMOVE__`, writing
   `swaggers/blockscout-api-final.yaml`.
3. `yarn generate` invokes Docker OpenAPI Generator with that final input,
   [openapi-generator-config.json](openapi-generator-config.json), and the local
   templates, writing `crate`.
4. `yarn format` formats the Rust workspace.

For changes targeting this repository, first compare the input schemas with
the current local producer, then use the local inputs:

```sh
yarn install --frozen-lockfile
yarn merge
yarn generate
yarn format
```

The local backend exposes its public OpenAPI document at `GET /api/v2/openapi`.
Use it together with the mounted routes and response views when reviewing
input changes. Its server URL already includes `/api` and its paths are
relative to that prefix; normalize these to the generator's full paths before
using them as input. A successful upstream download does not establish
agreement with the local backend.

`yarn generate-from-remote` runs all four steps, including the upstream
replacement. Use it only for an intentional upstream refresh followed by a
local contract review. It does not fetch the running Paxeer backend's schema.
Files listed in [crate/.openapi-generator-ignore](crate/.openapi-generator-ignore)
are excluded from generation; inspect that list and the resulting diff before
accepting generated changes.

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
uses the old `metadata.latest_block` shape, lacks the node height and carries
the unmounted v1 operation. Align the inputs with the producer above before
regeneration so it does not discard the current Rust model corrections.

## Tests

The existing [it_works.rs](crate/tests/it_works.rs) suite starts Stubr against
`tests/recorded/eth_blockscout_com`; it includes the obsolete v1 health
recording and old health assertions.
[repair_22_31_273.rs](crate/tests/repair_22_31_273.rs) checks model codecs,
including the blocks envelope and node height.
[repair_22_31_274.rs](crate/tests/repair_22_31_274.rs) exercises the client over
loopback HTTP with recordings and explicit producer-view examples. These are
offline checks, not a live Elixir-backend-to-Rust contract test.

For a local producer contract check, run a backend on a controlled database,
point the real `ApiClient` at its origin, and assert both the HTTP status and
typed body for the routes under review. For health, check
`metadata.blocks.latest_block` with DB, cache and node heights, and verify the
unhealthy HTTP 500 response through `Error::ResponseError`, preserving its raw
body. Include empty, null and error responses from that backend, and use
indexed hashes from that database for resource endpoints. The existing offline
suites do not establish this live contract by themselves.

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
