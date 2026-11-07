# Paxeer boundary

Paxeer mainnet-beta is one BFT network, running `paxd`, with EVM chain ID **125** (`0x7d`). The kernel is part of the network and proposes blocks. LayerX execution and accounting are integrated into the node; a receipt is final when its block is committed.

The former custody relay and checkpoint-settlement topology is retired. There is no separate LayerX interface, router, or sequencer service to deploy. The retired `layerxd` binary and `layerx-paxeer-client` crate are not prerequisites for connecting to the merged network.

This page describes the native connection and migration path first, then the TLS boundary that remains in the hosted test harness. That adapter forwards requests to a local node; it does not execute transactions or establish finality.

## Chain identity

| Setting | Mainnet-beta value |
| --- | --- |
| EVM chain ID | `125`, returned by `eth_chainId` as `0x7d` |
| Cosmos chain ID | `hyperpax_125-1` |
| Public HTTPS JSON-RPC | `https://api-mainnet-beta.paxeer.network/` |
| Individual node names | `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, using HTTPS at `/` |

`modules/evm/config/config.go` declares the mapping between `hyperpax_125-1` and EVM chain ID `125`. Check `eth_chainId` before signing or submitting a transaction, and refuse an endpoint that reports another chain.

```sh
curl --fail --silent --show-error https://api-mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

A successful response has `result: "0x7d"`. The node reports a Geth client string. `node.hyperpaxeer.com` is the hpx library mirror used by builds, not an RPC endpoint. The old `/rpc`, `/rpc/ws`, and `/rpc/schema` paths are retired.

## Routes

Applications send EVM JSON-RPC to the public node's root path. Sign transactions locally and submit the signed bytes with `eth_sendRawTransaction`; inspect inclusion with `eth_getTransactionReceipt`. Use `eth_call` for native precompile reads. The native HTTP routing and API registration are implemented in `rpc/rpcstack.go` and `rpc/server.go`.

The hosted gateway has its own application methods and authentication contract in `services/platform/hosted/gateway/openrpc.json`. Do not assume that a gateway method is available on every node endpoint. See [Public RPC](PublicRpc.md) for the gateway method contract and [Hosted gateway](HostedGateway.md) for its service integration.

Sub-chains are tenant domains stored in the database, rather than a new node deployment per tenant. The `layerxchains` keeper stores domains and their name indexes in `modules/layerxchains/keeper/keeper.go`. Native HTTP routing resolves `/chain/<domain-id>` to a domain scope and returns HTTP `404` with `{"error":"unknown_chain"}` when resolution fails (`rpc/rpcstack.go`). The mainnet-beta connection above uses the root domain at `/`.

## Finality

The node owns execution, proposal validation, and commitment. `node/app.go` constructs the `lxphost` host and wires the ledger, authority, payment, and other native keepers. The consensus source in `node/sequencer/consensus_source.go` seals the execution layer into a signed `MsgSequenceBatch`; the node's proposal and finalization handlers validate and apply it. `node/abci.go` publishes pending new-head events after a successful commit.

A fast signed receipt can precede commitment. `pax_sequencerReceipt` distinguishes `submitted`, `receipted`, and `committed`; only the last identifies a covering committed block with its number and hash (`rpc/sequencer_receipt.go`). Receipt acceptance or a healthy HTTP listener alone does not prove commitment. There is no additional guarantor finality stage in the merged network; the former checkpoint finality ladder is retired.

## Client contract

Use an EVM JSON-RPC client against a node URL with expected chain ID `125`. Preserve JSON-RPC IDs, handle returned errors, and track the transaction hash through block inclusion. Applications that consume a fast sequencer receipt must also check its committed state before treating it as final.

The merged native entry points include:

| Surface | Address | Declaration |
| --- | --- | --- |
| Ledger | `0x000000000000000000000000000000000000101A` | `precompiles/layerxledger/layerxledger.go` |
| Payments | `0x000000000000000000000000000000000000101B` | `precompiles/layerxpay/layerxpay.go` and its `abi.json` |
| Authority | `0x000000000000000000000000000000000000101C` | `precompiles/layerxauth/layerxauth.go` |

Use each entry point's checked-in ABI and module rules. These addresses are native precompiles registered by the node, so deploying replacement Solidity contracts is not part of connecting a client.

## Settlement suite

The former `deploy-contracts.sh bootstrap` instructions, `GuarantorBond`, `CheckpointRegistry`, vault deployment, and Blueprint activation are retired as prerequisites for merged-network execution or finality. The historical runner remains at `services/platform/hosted/paxeer/deploy-contracts.sh`; its presence does not make its settlement workflow a mainnet-beta installation procedure.

The merge upgrade is implemented in `node/upgrades.go`. It retires the anchor and invokes `LayerXCustodyKeeper.ReleaseRetired`, which releases custody balances to the configured authority and cancels pending claims. The release is restricted to the running merge plan, not a client operation to repeat after migration (`modules/layerxcustody/keeper/release.go`).

The old custody precompile address, `0x0000000000000000000000000000000000001013`, is retained as a retired entry point. Its executor refuses calls after retirement; the keeper also refuses new deposits with `ErrModuleRetired`, whose registered module code is `29` (`precompiles/layerxcustody/layerxcustody.go`, `modules/layerxcustody/keeper/deposit.go`, and `modules/layerxcustody/types/errors.go`). Do not direct new integrations to that deposit and withdrawal workflow.

To migrate an integration:

1. Replace custody-boundary client configuration with the native node URL and expected chain ID `125`.
2. Replace historical vault and checkpoint calls with the appropriate native module ABI and authorisation flow.
3. Track committed blocks and receipts instead of waiting for a second settlement transaction.
4. Keep tenant-domain selection explicit when accessing a sub-chain; launching a tenant does not require a separate chain process.

## In-cluster chain

`services/platform/hosted/paxeer/deployment.yaml` still defines a local test harness: StatefulSet `paxeer` in namespace `layerx-testnet`, with one replica, a genesis init container, `paxd`, a TLS `boundary` container, and a 40Gi data volume mounted at `/data`. This is a harness configuration, not the public network topology.

The `genesis` init container copies the native material mounted at `/run/native-inputs` (schema `layerx.beta.native-material.v1`, chain `hyperpax_125-1`, EVM chain ID `125`) into `/data/layerx/node`. The `paxd` container runs `kernel_runtime.py --home /data/layerx/node` with container ports `26656` (P2P), `8545` (EVM JSON-RPC), `26657` (consensus) and `9460` (health). The boundary container runs a primary adapter on `9443` and an observer on `9444`; the Services expose both at port `9443`. Those loopback and sidecar choices belong to this manifest; public clients use the node names above.

The harness `init-chain.sh` is a compatibility entry point that refuses the retired `LAYERX_PAXEER_*` and custody-genesis inputs and delegates to `services/platform/hosted/node/bootstrap.sh`. It is not a migration runbook for existing mainnet-beta state. Use the node's merge upgrade path for that state.

## Hosted compatibility transport

The crate `layerx-platform-paxeer-boundary` builds the binary `layerx-paxeer-boundary` from `services/platform/hosted/paxeer/src/main.rs`. It serves TLS HTTP/1.1 and forwards a restricted request set to a literal loopback HTTP node URL. Its route contract is:

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/livez` | HTTP `200`, `{"status":"live","service":"paxeer-boundary"}`; does not contact the node |
| `GET` | `/readyz` | Checks the upstream `eth_chainId` against the configured chain ID |
| `GET` | `/genesis` | Comet genesis JSON when the Comet upstream is configured; otherwise `404` `not_found` |
| `POST` | `/` | Restricted EVM JSON-RPC forwarding |
| `POST` | `/comet` | Restricted Comet proof queries when configured; otherwise `404` `not_found` |

Unknown methods or paths return `404` `not_found`. These health, genesis, and Comet routes describe the harness adapter only; they are not promised at the public EVM endpoint.

`POST /` requires exactly `Content-Type: application/json` and one JSON object. It allows `net_version`, `web3_clientVersion`, `pax_sequencerReceipt`, and `eth_` methods except `eth_accounts`, `eth_coinbase`, `eth_sendTransaction`, `eth_sign`, `eth_signTransaction`, `eth_signTypedData`, `eth_signTypedData_v4`, and `eth_mining`. `eth_sendRawTransaction` is permitted. Names must be at most 64 bytes and contain only ASCII letters, digits, or `_`.

The envelope must have `jsonrpc: "2.0"`; `params`, when present, must be an array or object. IDs may be null, a number, or a string of at most 256 bytes. Unknown envelope members are refused. The upstream must return HTTP `200` and a matching single JSON-RPC reply containing exactly one of `result` or `error`. Request bodies are limited to 2 MiB, upstream responses to 8 MiB, and simultaneous connections to 128; excess connections are dropped.

`POST /comet` has a separate 4096-byte request limit and allows only `status`, `commit`, `validators`, and proof-bearing `abci_query` for `/store/evm/key`. Its parameter validation is declared in `services/platform/hosted/paxeer/src/comet.rs`; it is not a general Comet proxy.

## Credentials

Public node clients use HTTPS and sign transactions locally. The native node config keeps `enable_unsafe_keyring_rpc` disabled by default; this controls node-keyring signing and transaction submission methods (`rpc/config/config.go`).

The harness adapter loads a DER server certificate and PKCS#8 DER private key, with no TLS client authentication and no bearer or API-key check in its route handler. The manifest mounts Secret `paxeer-boundary-tls` at `/run/layerx/tls`. Its Kubernetes network policy restricts admitted peers; server TLS alone is not application authentication. These credentials belong to the adapter and do not replace gateway credentials or transaction signatures.

## Typed refusals

The following errors belong to the harness adapter, not to the public node's entire JSON-RPC API. HTTP refusals have an `error` object containing `code` and `retry`, with `retry_after_seconds` for retryable errors (`services/platform/hosted/paxeer/src/main.rs`).

| HTTP status | Code | Meaning |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid HTTP request line or framing |
| `400` | `content_type_required` | Incorrect or missing JSON content type |
| `400` | `invalid_json` | Malformed JSON on the EVM route |
| `400` | `batch_not_supported` | EVM request body is not an object |
| `413` | `body_too_large` | Request exceeds the route's size limit |
| `404` | `not_found` | Unknown route, or unconfigured Comet route |
| `502` | `node_response_invalid` | Invalid upstream EVM reply; retry after 5 seconds |
| `503` | `node_unavailable` | Upstream node I/O failure; retry after 5 seconds |
| `503` | `chain_id_mismatch` | Readiness chain-ID mismatch; retry after 10 seconds |
| `503` | `node_response_invalid` | Invalid upstream chain-ID reply during readiness; retry after 5 seconds |

EVM envelope errors use JSON-RPC `-32600` (`invalid request`); denied or unknown EVM methods use `-32601` (`method is not relayed by the boundary`), both with HTTP `200`. Genesis and Comet forwarding have their own response checks in `src/genesis.rs` and `src/comet.rs` of the same crate. Configuration or listen failure exits the adapter with code `2`.

## Readiness

The adapter's `/readyz` succeeds only when the upstream chain ID matches `LAYERX_PAXEER_CHAIN_ID`. Its ready body is `{"status":"ready","service":"paxeer-boundary","chain_id":125}` for the harness configuration. `/livez` can remain successful when the node is unavailable. Neither probe proves consensus progress, receipt commitment, or tenant-domain availability.

`GET /genesis` returns the upstream genesis bytes with `X-LayerX-Genesis-SHA256`, the SHA-256 of those bytes. That header is a content digest, not an independent consensus trust anchor. The genesis handler supports bounded Comet chunk reassembly (32 chunks and 32 MiB).

## Configuration keys

Native EVM listener settings are in `app.toml` under `[evm]`, declared in `rpc/config/config.go`:

| Key | Role |
| --- | --- |
| `http_enabled`, `http_address`, `http_port` | HTTP listener enablement, bind address, and port; defaults are `true`, `0.0.0.0`, and `8545` |
| `ws_enabled`, `ws_port` | WebSocket enablement and port; defaults are `true` and `8546` |
| `enable_test_api` | Test API switch; the server also refuses to enable test APIs on live chain IDs |
| `enable_unsafe_keyring_rpc` | Unsafe local-keyring methods; disabled by default |
| `deny_list` | Methods the node refuses |

The harness overrides some native defaults as described above. Its TLS adapter uses these separate environment variables:

| Variable | Role |
| --- | --- |
| `LAYERX_PAXEER_BOUNDARY_LISTEN` | TLS bind; default `0.0.0.0:9443` |
| `LAYERX_PAXEER_CHAIN_ID` | Required positive `u64`; harness value `125`, checked by readiness |
| `LAYERX_PAXEER_NODE_URL` | Required literal loopback HTTP endpoint; harness value `http://127.0.0.1:8545` |
| `LAYERX_PAXEER_COMET_URL` | Optional literal loopback HTTP origin at `/`; harness value `http://127.0.0.1:26657` |
| `LAYERX_PAXEER_BOUNDARY_TLS_CERT_DER` | Server certificate file |
| `LAYERX_PAXEER_BOUNDARY_TLS_KEY_DER` | PKCS#8 private-key file |

The adapter accepts any positive configured chain ID; the harness manifest separately sets `125`. An EVM forwarding request does not repeat the readiness chain-ID comparison, so clients still need their own expected-chain check.

## Egress and admitted edges

The harness NetworkPolicy selects `app: paxeer`. It admits TCP `9443` from same-namespace `layerx-testnet-control`, `layerx-node`, and `layerx-gateway` pods, plus pods labelled `layerx-plane: human` in any namespace. Egress permits kube-dns UDP/TCP `53`; the adapter's node and Comet upstreams are loopback (`services/platform/hosted/paxeer/deployment.yaml`). This policy describes that local harness and is not a list of public network participants.

## Tests

The hosted adapter's existing target is `make platform-test-paxeer-boundary`, declared in `services/platform/Makefile.inc`. Its tests live in `services/platform/hosted/paxeer/tests/` and cover the adapter transport and local chain harness. They do not certify a public deployment.

Native execution, proposal validation, commitment, and recovery are covered by the node's tests, including `node/sequenced_proposal_test.go`, `node/sequenced_halt_rehearsal_test.go`, and `node/sequencer/crash_replay_test.go`. The receipt response contract is declared in `rpc/sequencer_receipt.go`. Consult those sources for the merged behavior rather than the retired custody-client tests.

[Home](Home.md)
