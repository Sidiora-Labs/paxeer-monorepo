# Agent HTTP contract

The agent's serve listener provides health and verified native reads. Source:
[`main.rs`](../../../services/agent/crates/layerx-agentd/src/main.rs) and
[`read/native.rs`](../../../services/agent/crates/layerx-agentd/src/read/native.rs).
The private agent envelope transport is described in [agent-api.md](agent-api.md).

## healthz

`GET /healthz` answers HTTP 503 while readiness checks are pending or fail and
HTTP 200 when the service is ready. Node readiness requires a successful chain-ID
check against the configured network.

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
