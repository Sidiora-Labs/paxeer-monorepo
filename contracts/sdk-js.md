# JavaScript native-module SDK contract

`sdk/js` owns the first-party `@pax-js/cosmos` package. It provides native
module types, protobuf codecs and REST query helpers for the same Paxeer
network served by `paxd`. The EVM chain ID is `125` (`0x7d`); the kernel is
part of the network and receipts are final when their block commits.

Use [node RPC](rpc.md) for transport addresses and module query paths. This
package is not an EVM JSON-RPC client. Explorer source verification uses the
[verifier HTTP contract](verifier-http.md), not this package's `Querier`.
Tenant domains do not require another SDK package or public EVM chain ID.

## Package

The authoritative manifest is [sdk/js/package.json](../sdk/js/package.json),
currently version `1.0.6-pax.1`, in
[paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/sdk/js).
It selects runtime artifacts under `dist/cjs` and `dist/esm`, with declarations
under `dist/types`; these directories are absent from this checkout.
There is no `src` build tree or `build` script in this package. Its test
script is `node --test test/`. The upstream tag in the README records
provenance; the old `VENDOR.kvx` pin is removed.

| Public import | Runtime entry | Declaration entry |
| --- | --- | --- |
| `@pax-js/cosmos/encoding` | `dist/{esm,cjs}/encoding/index.js` | `dist/types/encoding/index.d.ts` |
| `@pax-js/cosmos/rest` | `dist/{esm,cjs}/rest/index.js` | `dist/types/rest/index.d.ts` |
| `@pax-js/cosmos/types/<module>` | `dist/{esm,cjs}/types/<module>/index.js` | `dist/types/types/<module>/index.d.ts` |

The export map selects ESM with `import` and CommonJS with `require`. It has
no package-root export, `main` or `module` entry. Do not import from
`@pax-js/cosmos` directly or treat a declaration path as a runtime module.
The wildcard supports nested module directories such as
`types/cosmos/bank/v1beta1`. Internal codec files are not additional public
package subpaths; access codecs through `Encoder`.

Dependencies include `@bufbuild/protobuf` v2 and CosmJS proto-signing and
Stargate v0.32. The package does not supply a wallet, select a network, sign
a message or broadcast automatically.

## encoding-index

The [encoding package contract](../sdk/js/README.md#pax-jscosmosencoding) describes
`Encoder`, `aminoConverters` and `paxProtoRegistry`. `Encoder` groups codecs
under namespaces including `cosmos`, `epoch`, `eth`, `evm`, `mint.v1beta1`,
`oracle`, `tokenfactory`, `confio`, `google` and `tendermint`.

```typescript
import { Encoder } from '@pax-js/cosmos/encoding';
import type { MsgSend } from '@pax-js/cosmos/types/cosmos/bank/v1beta1';

// Addresses come from the selected network's signer and recipient.
function encodeBankSend(from: string, to: string) {
  const codec = Encoder.cosmos.bank.v1beta1.MsgSend;
  const message: MsgSend = codec.fromPartial({
    from_address: from,
    to_address: to,
    amount: [{ denom: 'upax', amount: '100' }],
  });
  return { typeUrl: `/${codec.$type}`, value: codec.encode(message).finish() };
}
```

For a CosmJS signing registry, pass the message object as `value`; the
registry performs protobuf encoding. The example returns encoded bytes for
consumers that require a pre-encoded value.

## encoding-common

The internal `common.d.ts` in the distribution's
[encoding export](../sdk/js/package.json) defines
`MessageFns<T, V>`: `$type`, `encode`, `decode`, `fromJSON`, `toJSON`, `create`
and `fromPartial`. `encode` returns a binary writer; call `.finish()` for a
`Uint8Array`. `decode` accepts bytes or a binary reader. `DeepPartial`,
`Exact`, `Builtin` and `KeysOfUnion` support construction and typing; they
are not runtime validators or network clients.

Keep the representation boundaries explicit. In-memory protobuf values use
`Uint8Array` for bytes and often JavaScript `number` for 64-bit fields;
amounts and decimal values declared as strings remain strings. Use the
message codec's JSON conversion for base64 bytes, timestamps and numeric
conversion. Values outside JavaScript's safe integer range cannot be
represented exactly by these `number` declarations. REST helpers do not
call these conversions for you.

## encoding-registry

`paxProtoRegistry` is declared as an array of `[typeUrl, GeneratedType]`
pairs. Entries use slash-prefixed fully qualified protobuf names; codec
`$type` values omit the leading slash. See the
[registry package contract](../sdk/js/README.md#usage-with-cosmjsstargate) for
registration and signing usage. A registered codec is not proof that the node
exposes a query or accepts a transaction; check the corresponding node
module and transport contract.

The current ESM registry implementation spreads
`cosmos_base_store_listening_registry` without importing that binding.
Loading the ESM encoding entry can therefore fail during module evaluation;
the export map alone is not evidence that the ESM aggregate loads. The
CommonJS registry imports its corresponding `listening` module explicitly.

## encoding-amino

`aminoConverters` maps type URLs to `aminoType`, `toAmino` and `fromAmino`
converters for CosmJS `AminoTypes`. It is exported from the encoding entry
point; the manifest does not export an `encoding/amino` subpath. Amino
conversion does not sign or submit a transaction and does not guarantee
that an arbitrary message is supported by a connected wallet.

## Codec and type families

Each section below also supplies the `types-*` anchor used by older registry
rows. Type declarations describe objects; `Encoder` supplies their binary
codecs. Files removed from the package are not restored by a registry link.

<a id="types-confio"></a>

### encoding-confio

`Encoder.confio` contains the ICS23 proof messages from
`dist/types/{encoding,types}/confio/proofs.d.ts`, including commitment,
existence, nonexistence, batch and compressed proofs and proof-operation
enums. Encoding a proof is not verification of a trusted chain commitment.

<a id="types-cosmos"></a>

### encoding-cosmos

`Encoder.cosmos` includes the exported auth, authz, bank, base, capability,
crypto, distribution, evidence, feegrant, genutil, gov, params, slashing,
staking, tx, upgrade and vesting namespaces. The corresponding
`types/cosmos/<module>/<version>` imports provide message interfaces and,
where present, enum values. Coins use string `denom` and string `amount`;
pagination uses the base query messages.

The `accesscontrol`, `accesscontrol_x`, `crisis` and Cosmos `mint.v1beta1`
codecs/types were retired and removed. The REST access-control and Cosmos
mint clients were also removed. Old registry rows naming them are historical
references, not supported imports. The custom Pax mint family below remains
distinct.

<a id="types-epoch"></a>

### encoding-epoch

`Encoder.epoch` and `types/epoch` contain epoch, genesis, params and query
messages. `Epoch` uses optional `Date` timestamps, an optional protobuf
duration, and numeric epoch/height fields. The node's JSON epoch duration
field has a Go JSON tag; use the node's actual gateway representation when
handling raw REST responses. Bindings are in [epoch queries](rpc.md#epoch-queries).

<a id="types-eth"></a>

### encoding-eth

`Encoder.eth` and `types/eth` cover `AccessTuple`, `AssociateTx`, `LegacyTx`,
`AccessListTx`, `DynamicFeeTx`, `BlobTx`, `BlobTxSidecar` and
`ExtensionOptionsEthereumTx`. Fee/value/chain-ID strings and signature/data
byte arrays are part of the
[module type contract](../sdk/js/README.md#pax-jscosmostypes). The node proto also
declares `SetCodeAuthorization` and `SetCodeTx`; this SDK does not currently
export those messages. These native envelopes are not Ethereum raw
transaction RLP or JSON-RPC request objects.

<a id="types-evm"></a>

### encoding-evm

`Encoder.evm` and `types/evm` cover config, enums, genesis, governance, params,
queries, receipts, transactions and EVM types. The six query methods are
listed under [EVM module queries](rpc.md#evm-module-queries). Pointer results
include `exists`; association results include `associated`.

Check enum parity before crossing the native ABI. The SDK's `PointerType`
matches the seven node values, but its `AssetType` declaration still has
`TYPECW20=0`, `TYPECW721=1`, whereas the node proto declares
`TYPEUNKNOWN=0`, `TYPECW20=1`, `TYPECW721=2`, `TYPENATIVE=3`. Those SDK
`AssetType` values must not be used as node enum numbers. Compare
[SDK module types](../sdk/js/README.md#pax-jscosmostypes) with
[node enums](../api/evm/enums.proto).

<a id="types-google"></a>

### encoding-google

`Encoder.google.api` and `Encoder.google.protobuf` provide HTTP annotation,
HTTP body, `Any`, descriptor, duration and timestamp codecs. Corresponding
types are under `types/google/api` and `types/google/protobuf`. Preserve
`Any.type_url` and its encoded value; it is not an arbitrary JSON object.

<a id="types-mint"></a>

### encoding-mint

`Encoder.mint.v1beta1` and `types/mint/v1beta1` contain the custom Pax mint
genesis, governance, mint and query messages. `Minter` query amounts/heights
are declared as numbers in this SDK, while gateway JSON represents uint64
fields as decimal strings. See [mint queries](rpc.md#mint-queries).

<a id="types-oracle"></a>

### encoding-oracle

`Encoder.oracle` and `types/oracle` cover oracle genesis, params, votes,
rates, snapshots, TWAPs and queries. Exchange rates and decimal parameters
are strings; counters and timestamps have their declared numeric types.
Requests use `denom`, `validator_addr` or `lookback_seconds` as applicable;
the exact HTTP prefix is documented in [oracle queries](rpc.md#oracle-queries).

<a id="types-tendermint"></a>

### encoding-tendermint

`Encoder.tendermint` groups `abci`, `crypto`, `libs.bits`, `p2p`, `types` and
`version` messages. Corresponding `types/tendermint/...` modules describe
blocks, validators, proofs, consensus parameters and ABCI requests/responses.
These structures must match the local consensus proto, not an arbitrary
upstream Tendermint release. The old `CheckTxType` enum was removed.

<a id="types-tokenfactory"></a>

### encoding-tokenfactory

`Encoder.tokenfactory` and `types/tokenfactory` provide authority metadata,
genesis, params, queries and the `MsgCreateDenom`, `MsgMint`, `MsgBurn`,
`MsgChangeAdmin`, `MsgSetDenomMetadata` and `MsgUpdateDenom` message families.
Coin amounts remain strings. `QueryDenomsFromCreatorRequest.pagination`
and its response pagination are present in the maintained declarations.

The package is not a complete mirror of the current module: its query client
has no `DenomPaused` method, and authority metadata currently declares
`issuerDid` while the node proto uses `issuer_did`. Use the
[node contract](rpc.md#tokenfactory-queries) for raw REST and newer fields;
do not infer missing functionality from old registry declarations.

## REST

The REST entry exports `Querier`; its methods are static query helpers, not
instances with a stored endpoint. Every call accepts a request and optional
`InitReq`. Configure `pathPrefix` with the operator's Cosmos REST origin,
without a trailing slash. A public EVM JSON-RPC origin is not a substitute.

```typescript
import { Querier } from '@pax-js/cosmos/rest';

async function readTokenfactoryParams(restOrigin: string) {
  return Querier.tokenfactory.Params({}, {
    pathPrefix: restOrigin,
    signal: AbortSignal.timeout(15_000),
  });
}
```

### rest-index

`Querier` groups `cosmos`, `epoch`, `evm`, `mint.v1beta1`, `oracle` and
`tokenfactory`. See the [REST package contract](../sdk/js/README.md#pax-jscosmosrest)
for query usage and transport behavior. No endpoint or chain is selected by default.

### rest-fetch

The internal `InitReq` extends `RequestInit` with `pathPrefix`. `fetchReq`
concatenates that prefix with the generated path and calls global `fetch`.
Generated query methods set `method: "GET"`; headers, credentials and abort
signals can be supplied by the caller. Without a prefix, paths are relative.

The helper calls `response.json()` and returns that body on success. On a
non-OK HTTP status it throws the parsed body, which need not be an `Error`
instance. Invalid JSON rejects during parsing, and fetch/network/abort errors
propagate. It adds no retries or timeout and does not call `fromJSON` or
validate the returned shape. In particular, its declared `Promise<T>` does
not convert base64 strings to `Uint8Array` or uint64 strings to numbers.

`renderURLSearchParams` flattens nested objects with dotted field names,
repeats keys for nonempty primitive arrays, excludes path parameters, and
omits zero numbers, false booleans, empty strings and empty arrays. Raw byte
arrays are not converted to base64 by this serializer. For byte-bearing
queries such as `StaticCall`, construct a properly encoded gateway request
instead of assuming the typed request object is serialized as protobuf JSON.

### rest-cosmos

The live `Querier.cosmos` modules are auth, authz, bank, distribution,
evidence, feegrant, gov, params, slashing, staking and upgrade, each under
`v1beta1`. Removed access-control and Cosmos mint REST clients do not resolve.

### rest-epoch

`Querier.epoch.Epoch` and `.Params` issue the two
[epoch HTTP queries](rpc.md#epoch-queries).

### rest-evm

`Querier.evm` exposes `PaxAddressByEVMAddress`, `EVMAddressByPaxAddress`,
`StaticCall`, `Pointer`, `PointerVersion` and `Pointee`. See the byte encoding
limitation in [rest-fetch](#rest-fetch) before using `StaticCall`.

### rest-mint

`Querier.mint.v1beta1.Params` and `.Minter` use `/paxchain/mint/v1beta1`.
This custom module remains present despite the removal of Cosmos mint clients.

### rest-oracle

`Querier.oracle` supplies the ten methods in
[oracle queries](rpc.md#oracle-queries), including `Twaps` and
`VotePenaltyCounter`. Its prefix contains `pax-chain`.

### rest-tokenfactory

`Querier.tokenfactory` exposes `Params`, `DenomAuthorityMetadata`,
`DenomMetadata`, `DenomsFromCreator` and `DenomAllowList`. Path parameters
are passed through `encodeURIComponent`; other request fields become query
parameters, including dotted pagination fields. If an intermediary rejects
encoded slashes in a denom path, use the node's query-only authority metadata
binding directly. `DenomPaused` requires a direct node query as described
in [tokenfactory queries](rpc.md#tokenfactory-queries).
