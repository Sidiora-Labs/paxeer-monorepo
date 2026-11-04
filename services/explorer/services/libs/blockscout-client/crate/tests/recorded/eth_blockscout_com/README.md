# Health compatibility and NFT cursor provenance

The client health method set for the merged explorer is:

| Client method | REST contract | Support |
| --- | --- | --- |
| `HealthApi::health()` | `GET /api/health` | Supported; decode `healthy` and `metadata.blocks` with db/cache/node block numbers. |
| `HealthApi::health_v1()` | Historical `GET /api/v1/health` | Refused with `Error::UnsupportedCapability` before any HTTP request. |

`_api_v1_health.json` is the **upstream-v1-2025-02-19** compatibility
recording. Its historical 200 response remains intact for the legacy model
decoder test. It does not establish support by the merged explorer router.
`supported_health_http_and_legacy_refusal` exercises the production Rust client
over loopback HTTP; the pytest gate separately checks the committed local router
and client refusal source. Executing the Phoenix router requires domain 14 wiring.

The NameWrapper instances recording retains its historical response and HTTP
status. Repair 22.33.567 reconstructs only `next_page_params.unique_token` from
the final recorded string ID, following the producer's
`Decimal.to_integer(token_id)` rule. This is a documented reconstruction, not a
fresh producer capture. The exact integer is
`115785665443821794889628784622978168165342723005226866880138161782244014770980`.
All other response-body bytes remain unchanged.

The production recorder at `../../../../ts-src/record-stubs.ts` already reads
`response.text()` and stores `response.status`; it must not parse and stringify
the inner body through JavaScript numbers. A fresh controlled backend capture
remains a wiring request. The Rust cursor test replays the same committed
response for two requests solely to check forwarding at the HTTP boundary; it
does not claim that this transcript is a captured second page.
