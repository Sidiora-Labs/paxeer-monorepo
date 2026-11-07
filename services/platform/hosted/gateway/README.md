# Hosted gateway

`layerx-platform-gateway` builds the `layerx-gateway` binary in the
[platform Cargo workspace](../../Cargo.toml). It provides hosted APIs and a
JSON-RPC ingress to the single Paxeer network. `paxd` contains the kernel,
executes transactions and proposes blocks; receipt finality is BFT block
commit. Tenant domains are database namespaces in that network.

Public node JSON-RPC is served at the root of
`https://api-mainnet-beta.paxeer.network/` and
`https://api1.mainnet-beta.paxeer.network/` through
`https://api14.mainnet-beta.paxeer.network/`, with EVM chain ID **125** (`0x7d`).
Those nodes expose `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`
namespaces. The gateway's `px_*` methods are a hosted contract; configuring a
public node does not deploy those APIs. Mainnet-beta uses real value; obtain
hosted service coordinates, credentials and any allocation policy from its operator.

## Build and test

From the repository root:

```sh
cargo build --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-gateway
cargo test --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-gateway
```

## Node RPC and hosted RPC are separate contracts

Send node JSON-RPC 2.0 requests to `POST /` with
`Content-Type: application/json`. Check the network before submitting value:

```json
{"jsonrpc":"2.0","id":"network","method":"eth_chainId","params":[]}
```

The expected chain ID is `0x7d`. Native balance reads use `eth_call` against
the bank precompile, without a custody deposit or an anchor join. For example,
`all_balances(address)` for the illustrative address below is:

```json
{"jsonrpc":"2.0","id":"balances","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000001001","data":"0x0c5e2789000000000000000000000000102132435465768798a9bacbdcedfe0f1e2d3c4b"},"latest"]}
```

Decode the result as the bank ABI's coin array; amounts are full-width integers.
This example illustrates the encoding and does not promise a funded account.

The **hosted gateway**, at the operator-provided origin, also accepts `POST /`.
Its dispatcher is [src/rpc.rs](src/rpc.rs), reached through
[src/main.rs](src/main.rs). The gateway-owned [OpenRPC contract](openrpc.json)
declares the hosted `px_*` methods and `pax_sequencerReceipt`; it is not the
node's complete RPC schema. Do not assume a public node implements `px_*`.
Use the checked-in contract; `/rpc/schema` is not a discovery endpoint.
Hosted requests may be single objects or batches of 1..32 entries; notification-only requests
produce HTTP 204. Malformed JSON answers `-32700`, invalid request envelopes
`-32600`, and unknown methods `-32601`.

The old `/rpc`, `/rpc/schema`, `/rpc/ws` and `/rpc/evm/ws` network coordinates
are retired and return HTTP 404 at the gateway. There is no unified WebSocket
execution router; the HTTP relay does not provide subscriptions.
The retired `lx_*` methods answer `-32601` without contacting a backend.
Use native signed transactions and chain reads for network execution.
The removed `/v1/activities` and `/v1/programs` execution routes return 404.
There is no separate LayerX interface or router in the merged network.

## Node relay and receipt rungs

`LAYERX_GATEWAY_PAXEER_RPC_URLS` is a JSON array of two to eight distinct node
URLs. For example:

```json
["https://api1.mainnet-beta.paxeer.network/", "https://api2.mainnet-beta.paxeer.network/"]
```

Invalid arrays refuse startup. The relay tries configured nodes in order,
checks `eth_chainId` equals 125, and requires HTTP 200 and a valid JSON-RPC
response with the matching ID. Transport and HTTP failures move to the next
node and can log `paxeer_endpoint_failed`. After a node answers HTTP 200,
an invalid envelope is refused rather than retried; a JSON-RPC error from that
node remains an error. Missing configuration answers `-32001` with data code
`paxeer_rpc_not_configured`.

The admitted methods are `eth_*`, `net_*`, `web3_*` and
`pax_sequencerReceipt`, subject to the policy in [src/evm.rs](src/evm.rs).
The gateway never signs for the caller. `eth_accounts`, `eth_coinbase`,
`eth_sendTransaction`, `eth_sign`, `eth_signTransaction`, `eth_signTypedData`,
`eth_signTypedData_v4` and `eth_mining` are refused with `-32601`.
`eth_subscribe` and `eth_unsubscribe` require a subscription transport and are
refused by the HTTP relay with `-32004`.

Submit the caller's signed EIP-1559 bytes with `eth_sendRawTransaction`.
Reconcile the exact `0x`-prefixed 32-byte transaction hash with
`pax_sequencerReceipt`:

- `submitted`: acknowledgement, with no signed execution receipt yet.
- `receipted`: signed execution evidence, requiring independent key and
  transaction verification.
- `committed`: inclusion in a committed block, the network's finality boundary.

The relay returns node evidence; it does not itself turn a receipt into a
verified finality claim. Verify the receipt signature and transaction binding
against governed receipt keys valid at its height. For `committed`, authenticate
the block header and validator commit against a trusted consensus context and
verify the receipt's sequence-batch inclusion and coverage under that header.
A transaction hash, a `receipted` response or an unverified `px_getCommit`
response alone is not block-commit evidence. No separate sequencer trust anchor,
custody credit, periodic checkpoint or guarantor join establishes finality.
See the [receipt authority](../authority/README.md)
and [reference applications](../../examples/README.md) for consumers and their
verification limits.

## Hosted reads

[src/paxeer.rs](src/paxeer.rs) dispatches `px_resolveAccount`, `px_getAccount`,
`px_getBalances`, `px_listAssets`, `px_getNetwork` and `px_getCapabilities`.
[src/history.rs](src/history.rs) owns `px_getHistory` and
`px_getUnifiedHistory`. `px_getRouteCatalogue` returns an authenticated hosted
route catalogue; it is not a second network execution path.

Account, balance and asset reads use the native bank (`0x1001`), address binding
(`0x1004`) and ledger (`0x101a`) precompiles. Native payment, authority and
Programs constructs are at `0x101b`, `0x101c` and `0x101d`, respectively; these
are 20-byte addresses padded with leading zeros. The old custody/anchor join
is retired. ABI selectors and decoders live in [src/evm.rs](src/evm.rs), with
vectors in [tests/fixtures/paxeer-abi-vectors.json](tests/fixtures/paxeer-abi-vectors.json).

Successful `px_listAssets` ledger rows include `paused` (boolean) and
`verification` (`"authenticated_committed_snapshot"`). Both are optional in the
closed result schema and absent from bank, token-factory and failed-read rows.

[src/chain_reads.rs](src/chain_reads.rs) implements `px_getCommit`,
`px_getValidators`, `px_getReceiptKeys` and `px_getSequenceBatch`. Configure
consensus and receipt-key inputs for those reads; missing evidence produces
an explicit unavailable response. Registration, faucet and protected service
routes have their own authentication and backend requirements. Public node
availability alone does not make them ready.

## Configuration and readiness

Read [src/main.rs](src/main.rs) for environment parsing and
[deployment.yaml](deployment.yaml) for a deployment template. Configure node
URLs, network identity, persistent key/idempotency storage, outbound trust and
service credentials for the routes being enabled. The listener supports TLS
and a plain mode for a deployment that terminates TLS at its edge.

`GET /livez` checks process liveness. `GET /readyz` reports dependency readiness;
inspect individual backend results before enabling a feature. Optional
hosted backends can be unavailable while chain reads remain usable.
Legacy deployment names in environment templates do not establish public
endpoints or new network components.

## Merged receipt-key deployment

The Kubernetes template runs the gateway with its optional kernel backend group
unset. It requires no receipt-authority, pending-core, authorized-batch files,
component, identity or program-registry client secrets. Native receipt evidence
comes from the chain through `pax_sequencerReceipt`. Hosted
`px_getReceiptKeys` serves the operator-pinned snapshot; it does not fetch or
trust a key supplied by an incoming receipt. Kernel-dependent routes keep their
existing explicit unavailable responses. The Kubernetes client identity is for
independent services such as the faucet, through
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` and its password file.

Before deployment, the platform operator must supply an authenticated export
of the chain's governed ReceiptKeys to the existing
[sequencer-pins.py](../tests/sequencer-pins.py) provisioning path. The input is a
nonempty JSON array with `keyId` and `publicKey` as canonical base64-encoded
32-byte values, and integer `firstHeight` and `lastHeight` as inclusive uint64
chain heights. The node's `px_getReceiptKeys` result instead wraps the entries
in `{"keys": [...]}` and represents the bytes as hex. Exporting that result for
provisioning must preserve the exact IDs, public keys and height bounds while
converting the bytes to base64; copying the raw RPC result is invalid.

Authenticate this export against the operator's chain/governance trust anchor
for chain 125 before provisioning. A chain-ID label alone is not authentication.
These are public verification keys, never validator or receipt signing keys.
There is no bundled default key or automatic network fetch. Missing, empty,
malformed, duplicate or reversed-range material fails provisioning. The Rust
consumer additionally checks Ed25519 key validity.

The existing producer validates the export and atomically publishes a protected
`receipt-keys.json`. Its `gateway_secret` function serializes that file as the
same-namespace Secret `layerx-gateway-receipt-keys`. The template projects only
`receipt-keys.json`, with mode 0440, at
`LAYERX_GATEWAY_RECEIPT_KEYS_FILE`; the gateway loads it through
`ReceiptKeyList::from_config`. Its projection matches the existing
`gateway_manifest` renderer exactly. A gateway container restart reopens the
retained projection; pod replacement uses the retained Secret. The provisioning
path revalidates retained material without receipt-authority or batch files.
Key rotation requires an authenticated export update and a pod rollout: the
gateway loads the list at startup. Keep historical keys and their exact height
windows for retained receipts; do not reset Redis keys, quotas, audit or
idempotency state. Redis retains its existing PVC and AOF configuration.

Identifier meanings in this template and the Railway example are:

| Identifier | Meaning |
| --- | --- |
| EVM chain ID `125` (`0x7d`) | The single Paxeer chain, checked by the RPC relay. |
| `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID=125` | Numeric native protocol network field for this deployment; separate from a tenant identifier. |
| `LAYERX_GATEWAY_NETWORK_ID=paxeer-mainnet-beta` | Hosted discovery and route-binding identity label; it must match operator route bindings. |
| Kubernetes namespace `layerx-testnet` | Retained installation and DNS namespace; its name does not select a chain. |
| `LAYERX_GATEWAY_LXP_WIRE_VERSION=3` | Receipt/state-commitment wire version, not a network or chain ID. |
| `keyId` | Governed 32-byte receipt-key identifier, independent of service credentials. |
| `firstHeight` / `lastHeight` | Inclusive chain-height authorization bounds, never an authority-issued batch allocation. |

The Railway example keeps receipt keys optional until its external file-secret
mapping is provisioned with the converted base64 array. Gateway-wide service
mTLS also remains unset there: enabling it requires the Docker entrypoint to
stage the two `LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_*` files before dropping
privileges. Existing event-producer mTLS uses its separate supported file
variables. The repair record assigns both Docker changes to their owning task.

Qualification for this repair executes the existing production provisioning,
Secret serialization and manifest renderer, and checks source wiring into the
Rust loader and key reader. It does not launch the Rust binary or certify a live cluster. The existing Kubernetes RPC list contains one
controlled boundary URL while the current relay requires two to eight distinct
URLs; reconciling the boundary topology and its prior one-path qualification is
an external wiring request recorded in the repair ledger. Fresh and retained
**material provisioning** are proven here; full deployment readiness also needs
that RPC reconciliation and the operator's existing TLS/Redis/service secrets.

## Canonical module registry

`LAYERX_GATEWAY_MODULE_REGISTRY_FILE` names a read-only JSON registry shared
with the [receipt authority](../authority/README.md):

```json
{
  "schema_version": 2,
  "assets": [{
    "asset": "0202020202020202020202020202020202020202020202020202020202020202",
    "currency": "USD",
    "decimals": 6,
    "symbol": "$"
  }],
  "modules": [
    {"module": 9, "ordinals": [1, 2, 7]}
  ]
}
```

The asset is illustrative. Version must be 2, with 1..256 unique nonzero
lowercase 64-hex asset identifiers. Currency and symbol are 1..32 bytes without
control characters; decimals is at most 38. Module IDs must be known and unique;
Programs ordinals 1, 2 and 7 are added by the gateway. This registry describes
admitted protocol data; it does not restore retired activity submission APIs.
A fuller example is [module-registry.example.json](../../../interop/deploy/gateway/module-registry.example.json).

Public node details: [public-rpc.md](../../../../docs/site/docs/reference/public-rpc.md).
Method and evidence references: [PublicRpc](../../../../docs/wiki/PublicRpc.md),
[CommitmentLevels](../../../../docs/wiki/CommitmentLevels.md) and
[PublicAPI](../../../../docs/wiki/PublicAPI.md).
