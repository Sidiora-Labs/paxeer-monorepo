# RPC pool availability

The [gateway RPC pool](../../../../services/wallet/gateway/src/rpc/pool.ts)
separates node reads from submission. `RPC_URLS` is a comma-separated list of
read endpoints; its default is the canonical public mainnet-beta node alias.
`RPC_LAG_THRESHOLD_BLOCKS` defaults to `20`, `RPC_HEALTH_INTERVAL_MS` to `5000`
and `RPC_TIMEOUT_MS` to `8000`.

Health probes check chain ID and head. An endpoint that trails the best head
beyond the threshold is `lagging`; an endpoint that cannot answer is `down`.
Healthy endpoints are selected by latency, with transport failover for reads.
Unknown endpoints remain eligible before their first probe. A JSON-RPC error
is returned to the caller rather than retried on another endpoint.

`eth_sendRawTransaction` and `pax_sequencerReceipt` go to `SEQUENCER_RPC_URL`,
which defaults to the first `RPC_URLS` entry. That setting names the node
submission endpoint within the merged network. Submission sends once to that
endpoint; read-pool failover does not rebroadcast a transaction. Balance and
chain reads in `src/chainReads.ts` and `src/chain.ts` use
`HYPERPAXEER_RPC_URL`; attestor binding reads use `ATTESTOR_RPC_URL`.

## Diagnose

Public `/readyz` reports component states and reasons. The RPC component is
`up` when at least one configured read endpoint is healthy. Detailed endpoint
states and counts require the authenticated admin route. `GATEWAY_URL` below
is the operator-supplied gateway base:

```sh
curl --silent --show-error --max-time 10 \
  -H "Authorization: Bearer $GATEWAY_ADMIN_TOKEN" \
  "$GATEWAY_URL/v1/admin/readiness"
```

Inspect `components.rpc_pool.healthy` and `endpoints`. Endpoint errors are
redacted as `rpc_endpoint_unavailable`. A healthy read pool alone does not
establish that the submission endpoint or the separately configured read
clients can answer.

For each candidate node, set the operator variable `RPC_CANDIDATE` and check:

```sh
curl --silent --show-error --max-time 10 "$RPC_CANDIDATE" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --silent --show-error --max-time 10 "$RPC_CANDIDATE" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
```

Require chain ID `0x7d` and compare the head with current nodes on the same
chain. For submission, also check the configured node's receipt support.

## Recover

Update `RPC_URLS` in the gateway runtime configuration to verified nodes and
restart the gateway with its database and nonce store preserved. Set
`SEQUENCER_RPC_URL` explicitly when submission should use a different node.
Update `HYPERPAXEER_RPC_URL` or `ATTESTOR_RPC_URL` separately when those clients
need a different endpoint; roll attestors one at a time while preserving quorum.

If a broadcast returns `outcome_unknown`, retain its hash and nonce and follow
`retry: poll_receipt`. Query `eth_getTransactionReceipt` for that hash. Do not
prepare or sign another transaction to resolve an uncertain broadcast.

Confirm admin diagnostics show a healthy pool and public `/readyz` answers
200 with `ready: true`. Verify reads and a consented transaction separately.
Restoring a prior endpoint configuration does not reverse a submitted payment
or release its allocated nonce.
