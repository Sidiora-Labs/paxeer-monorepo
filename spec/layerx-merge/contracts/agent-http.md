# Agent HTTP contract

The agent's program read listener provides health, program balances and verified
native reads. [`main.rs`](../../../services/agent/crates/layerx-agentd/src/main.rs)
loads `serve_read_routes` from
[`proof_native_reads.rs`](../../../services/agent/crates/layerx-agentd/tests/proof_native_reads.rs)
through [`proof_serve_errors.rs`](../../../services/agent/crates/layerx-agentd/tests/proof_serve_errors.rs).
[`read/native.rs`](../../../services/agent/crates/layerx-agentd/src/read/native.rs)
implements native read dispatch.
The private agent envelope transport is described in [agent-api.md](agent-api.md).

## Authentication

Every request to the program read listener, including health, requires exactly
one `Authorization: Bearer` header matching `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`.
Missing, duplicate or incorrect credentials return HTTP 401 `unauthorized`.
Requests must use HTTP/1.1 GET with an empty body.

Native read routes are available when the MCP binding installs a native reader;
otherwise they return HTTP 404 `not_found`. The Human owner mode serves a separate
health-only listener using `LAYERX_AGENT_PROGRAM_BEARER_TOKEN` and does not serve
program balances or native reads.

## healthz

`GET /healthz` answers HTTP 503 while readiness checks are pending or fail and
HTTP 200 when the service is ready. Node readiness requires a successful chain-ID
check against the configured network and the `readiness_gate` checks. Its JSON
body reports `ready`, `transport_ready`, `verified_reads_ready` and
`tenant_writes: "unknown"`.

## programs-balances

`GET /v1/programs/{program}/balances` validates the program identifier and reads
its accounts through `eth_call` to layerxprograms at a committed height.
The balance reader binds every call to that height and block hash, and refuses
an unavailable or changing block. The response includes the program, lifecycle,
accounts and freshness coordinates. A malformed program identifier returns
HTTP 400 `invalid_program`.

The reader is implemented in
[`read/program_node.rs`](../../../services/agent/crates/layerx-agentd/src/read/program_node.rs)
and exported through
[`read/program_balances.rs`](../../../services/agent/crates/layerx-agentd/src/read/program_balances.rs).

## reads-receipt

`GET /v1/reads/receipt/{hash}` looks up `pax_sequencerReceipt` for a transaction
hash. An absent lookup is unavailable. A submitted lookup returns the transaction
hash, nonce, null receipt and key index, and rung `submitted`. A verified signed
receipt returns canonical receipt bytes, the pinned key and its index, height,
transaction index, freshness and rung `receipted`.

A signed execution receipt alone does not establish block commitment. The proof
read acquires and verifies committed-block evidence separately. The native read
route also dispatches `proof`, `checkpoint`, `availability` and `history`; unknown
kinds are refused with `NativeReadError::InvalidRequest`.
