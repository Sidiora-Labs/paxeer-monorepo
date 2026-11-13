# Health and NFT cursor fixtures

The current client health methods are:

| Client method | REST contract | Support |
| --- | --- | --- |
| `HealthApi::health()` | `GET /api/health` | Decodes `healthy` and `metadata.blocks` with DB, cache and node block numbers. |
| `HealthApi::health_v1()` | None | Returns `Error::UnsupportedCapability` before any HTTP request. |

The JSON files in this directory are static API fixtures. `_api_v1_health.json`
is used for compatibility decoding; current health requests use `/api/health`.

The NameWrapper fixture represents the final token ID as a decimal string and
`next_page_params.unique_token` as the corresponding exact integer. Forward that
cursor unchanged when requesting the next page. Its value is
`115785665443821794889628784622978168165342723005226866880138161782244014770980`.

The [response recorder](../../../../ts-src/record-stubs.ts) stores
`response.text()` and `response.status`, preserving the response body without
round-tripping its integers through JavaScript numbers. See the
[client guide](../../../../README.md#recording-responses) for recorder arguments.
