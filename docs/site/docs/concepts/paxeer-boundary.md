# Paxeer boundary

Paxeer X Network is one BFT network. The kernel runs inside `paxd` and
participates in transaction execution and block proposal. Kernel state and EVM
state share the committed-block boundary: a receipt is final once its block is
committed. There is no later external checkpoint or settlement deployment
required to make that result final.

The former TLS relay `layerx-paxeer-boundary`, the `paxeer-relay` sidecar and
the custody client `layerx-paxeer-client` are retired integration paths.
They are not prerequisites for connecting an application to the merged network.
There is no separate LayerX interface or router; the standalone `layerxd`
daemon is retired. Sub-chains are tenant domains in the database, launchable
without new infrastructure.

The integrated host and proposal wiring live in `node/app.go`,
`node/sequencer/`, `modules/sequencer/` and `modules/lxphost/`.

## Chain identity and public endpoints

The EVM chain ID is **125** (`0x7d`). Public node JSON-RPC uses HTTPS at the
root path of `api1` through `api14.mainnet-beta.paxeer.network`, or the
canonical alias `api-mainnet-beta.paxeer.network`. See
[Public RPC endpoints](../reference/public-rpc.md) for the full list.

Check the chain identity before signing or broadcasting:

```sh
curl --fail-with-body --silent --show-error --max-time 15 \
  https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected `result` is `"0x7d"`. Check the JSON-RPC envelope and its `error`
field as well as the HTTP status; HTTP success alone does not establish that
an RPC call succeeded. Reject a different chain ID.

Keep multiple nodes configured, use bounded client deadlines and compare
`eth_blockNumber` when selecting a node near the head. The chain ID selects
the transaction signing domain. Authenticate genesis and validator history
separately; a matching chain ID alone does not establish that a node serves
the intended history, is caught up or has committed a particular transaction.

The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`
and `txpool`; method availability depends on the node configuration. The former
unified `/rpc`, `/rpc/ws` and `/rpc/schema` public routes are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.

## Execution receipts and finality

The node's `pax_sequencerReceipt` method takes a transaction hash and distinguishes
these states, defined by `rpc/sequencer_receipt.go`:

| Rung | Meaning |
| --- | --- |
| `submitted` | The node knows the transaction but has no signed execution receipt. |
| `receipted` | A signed execution receipt exists; its sequence batch has not committed. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt. The answer includes `blockNumber` and `blockHash`. |

An unknown hash returns `null`. A node without a local receipt source or a
configured forwarding endpoint returns JSON-RPC `-32000` with
`pax_sequencerReceipt served by sequencer`. These are availability outcomes,
not evidence that execution failed. The proposer role is part of the network;
there is no separate sequencer service to deploy.

A transaction hash or signed receipt alone is insufficient for finality. Wait
for committed block evidence. For independent verification, authenticate the
block header and CometBFT commit, and verify the relevant transaction or state
inclusion at that height. See [Checkpoints](checkpoints.md) for the current
committed-block model and the distinction from historical certificates.

Receipt verification also needs signing authority. The
`SequencerReceiptResult.Verify` implementation checks receipt bytes, execution
output digests and the signature; callers must supply independently trusted
receipt key pins to authenticate the signer. The `signingKey` returned beside
a receipt is not itself a trust pin. The node code declares `px_getReceiptKeys`
for governed key IDs, public keys and height ranges; its availability depends
on the serving node's configuration. Do not assume that a method declared in
the tree is enabled on every public endpoint.

A timeout after submission leaves the outcome unknown. Retain the original
transaction hash and query it before retrying. An EVM receipt's execution
status must also be checked: commitment does not imply successful execution.

## Native interfaces

Applications use node transactions, committed state and native precompiles.
The merge address table is `MergeAddressTable` in `precompiles/setup.go`:

| Construct | EVM address (short form) | Implementation |
| --- | --- | --- |
| Ledger | `0x101A` | `precompiles/layerxledger/` |
| Payments | `0x101B` | `precompiles/layerxpay/` |
| Authority | `0x101C` | `precompiles/layerxauth/` |
| Programs | `0x101D` | `precompiles/layerxprograms/` |
| Tenant chains | `0x101E` | `precompiles/layerxchains/` |

These short forms denote 20-byte addresses padded with leading zeroes.
The table also registers agent functions at `0x0901` through `0x0908`.
Use the ABI and activation rules of the target precompile; these calls operate
inside the network's execution domain.

Hosted gateway `px_` methods have an operator-deployed service contract in
`services/platform/hosted/gateway/openrpc.json`. A gateway method is not
therefore available at the public node root. Obtain the gateway origin and
supported methods from its operator; see
[Hosted gateway JSON-RPC](../platform/gateway-rpc.md).

For a hosted gateway's node connections,
`LAYERX_GATEWAY_PAXEER_RPC_URLS` is a JSON array of two to eight distinct HTTPS
endpoints. For example:

```json
["https://api1.mainnet-beta.paxeer.network","https://api12.mainnet-beta.paxeer.network"]
```

`services/platform/hosted/gateway/src/paxeer.rs` parses that setting, posts to
`/` for root endpoints and checks `eth_chainId` against `125` before forwarding
the requested call. This is gateway upstream configuration, not a new network
boundary or a public endpoint to provision.

## TLS and trust

Use HTTPS with certificate and hostname verification for public node calls.
For private service connections, provision the appropriate trust anchor through
the supported client configuration and keep private keys in secret storage.
The gateway's TLS client in `services/platform/hosted/gateway/src/http.rs`
adds its configured CA certificate, connects using the endpoint hostname and
requires at least TLS 1.2.

TLS authenticates the transport peer. Separately check chain ID, receipt key
authority and committed evidence. The retired relay's certificate variables,
client trust-pin types, method allowlist and `/readyz` response are historical
contracts; they do not define authentication, readiness or errors at today's
public node endpoints.

## Migration from the retired boundary

Replace the retired relay URL and custody-client integration with direct node
JSON-RPC and the native interfaces above. Withdrawals and funding within the
network use native bank or ledger operations; the former external vault,
deposit-root and forced-exit workflow is retired.

The old `deploy-contracts.sh bootstrap` procedure, `GuarantorBond`,
`CheckpointRegistry`, challenge and exit contracts, and Blueprint activation
recipe are retired settlement instructions. Do not run them to initialize an
application or establish finality on the merged network. Guarantor finality is retired;
validators commit the proposed block.

Some compatibility code, fixtures and historical decoders remain in the tree.
Their presence does not make the old topology an operational requirement.
In particular, `services/platform/hosted/paxeer/` contains legacy relay and
bootstrap material. `modules/layerxcustody/` and `modules/layerxanchor/` retain
historical lifecycle handling and explicit retirement gates. Retired precompile
methods return `Error(string)` carrying `LXP_MODULE_DISABLED` under their
activation rules (`precompiles/common/retire.go`); an address remaining
registered is not permission to resume settlement writes. Anchor views have a
separate store-deletion gate, so historical read availability must be evaluated
at the requested height.

Use archived checkpoint encodings only to interpret historical evidence.
Current integrations should track the transaction hash, execution result and
covering committed block, preserving the chain ID and height with any exported
proof.

[Home](../index.md)
