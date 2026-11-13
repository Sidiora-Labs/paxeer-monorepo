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
currently version `0.1.0`, in
[paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/sdk/js).
Its distribution contract selects runtime artifacts under `dist/cjs` and
`dist/esm`, with declarations under `dist/types`. The checkout contains the
manifest, README and package tests; consumers need the distribution artifacts
to load the declared subpaths.
There is no `src` build tree or `build` script in this package. Its test
script is `node --test test/`.

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
Stargate `^0.39.0`. The package does not supply a wallet, select a network, sign
a message or broadcast automatically.

## encoding-index

The [encoding package contract](../sdk/js/README.md#pax-jscosmosencoding) describes
`Encoder`, `aminoConverters` and `paxProtoRegistry`. The package tests specify
codecs for bank sends, EVM messages, custom mint messages, TokenFactory and
consensus messages.

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

The [encoding guide](../sdk/js/README.md#pax-jscosmosencoding) specifies
`$type`, `fromPartial`, `encode` and `decode`. Call `.finish()` on an encoded
message to obtain bytes. `fromPartial` fills omitted fields with defaults;
the chain still validates transaction semantics and permissions.

The [TokenFactory codec cases](../sdk/js/test/tokenfactory-encoding.test.cjs)
specify `Uint8Array` pagination keys in message objects and base64 keys in
codec JSON. REST helpers return parsed wire JSON without codec conversion.

## encoding-registry

`paxProtoRegistry` contains type URL/codec pairs for CosmJS `Registry`. Entries use slash-prefixed fully qualified protobuf names; codec
`$type` values omit the leading slash. See the
[registry package contract](../sdk/js/README.md#usage-with-cosmjsstargate) for
registration and signing usage. A registered codec is not proof that the node
exposes a query or accepts a transaction; check the corresponding node
module and transport contract.

## encoding-amino

`aminoConverters` supplies the converter map for CosmJS `AminoTypes`. It is exported from the encoding entry
point; the manifest does not export an `encoding/amino` subpath. Amino
conversion does not sign or submit a transaction and does not guarantee
that an arbitrary message is supported by a connected wallet.

## Codec and type families

The package tests and [package guide](../sdk/js/README.md) describe the
distribution interface. Native message fields and wire numbers come from the
linked protobuf sources. Type declarations describe objects; codecs encode
and decode those objects.

<a id="types-confio"></a>

### encoding-confio

Native ICS23 commitment and existence proof messages are declared in
[proofs.proto](../interchain/third_party/proto/proofs.proto). Encoding a proof
alone does not verify a trusted chain commitment.

<a id="types-cosmos"></a>

### encoding-cosmos

The package guide specifies bank `MsgSend` through
`Encoder.cosmos.bank.v1beta1` and the corresponding
`types/cosmos/bank/v1beta1` import. Coins use string `denom` and string
`amount`; [coin.proto](../sdk/proto/cosmos/base/v1beta1/coin.proto) defines the
native wire fields. The custom Pax mint family below is separate.

<a id="types-epoch"></a>

### encoding-epoch

The native [epoch declaration](../api/epoch/epoch.proto) defines timestamp,
duration, epoch and height fields. Its duration has the Go JSON tag
`duration,omitempty`. See [epoch queries](rpc.md#epoch-queries) for node
query bindings.

<a id="types-eth"></a>

### encoding-eth

The native [Ethereum transaction declarations](../api/eth/tx.proto) define
access lists, association signatures and transaction envelopes, including
`SetCodeAuthorization` and `SetCodeTx`. These protobuf envelopes are distinct
from Ethereum raw transaction RLP and JSON-RPC request objects.

<a id="types-evm"></a>

### encoding-evm

The [EVM codec cases](../sdk/js/test/evm-encoding-declarations.test.cjs)
specify snake_case codec fields such as `pax_sstore_set_gas_eip2200` and
`allowed_fee_denoms`. The [module declaration cases](../sdk/js/test/types-declarations-app.test.cjs)
use camelCase fields such as `paxSstoreSetGasEip2200` and `allowedFeeDenoms`
in the module interfaces. Use the field names of the selected interface.

The [node enums](../api/evm/enums.proto) define `AssetType` as
`TYPEUNKNOWN=0`, `TYPECW20=1`, `TYPECW721=2`, `TYPENATIVE=3`.
Use those wire values for native requests. The declaration cases also specify
EVM asset `denom`, TokenFactory `issuerDid` and pagination fields.
See [EVM module queries](rpc.md#evm-module-queries) for node query bindings.

<a id="types-google"></a>

### encoding-google

Native protobuf [Any](../sdk/third_party/proto/google/protobuf/any.proto)
contains `type_url` and encoded `value` bytes. An arbitrary JSON object does
not supply that protobuf envelope.

<a id="types-mint"></a>

### encoding-mint

The [namespace cases](../sdk/js/test/encoding-namespace.test.cjs) specify
`Encoder.mint.v1beta1`, including `Minter`, `Params`, scheduled releases and
their version-two counterparts. They exclude `Encoder.cosmos.mint`.
See [mint queries](rpc.md#mint-queries) for the custom module's node bindings.

<a id="types-oracle"></a>

### encoding-oracle

Native oracle messages are declared under [api/oracle](../api/oracle).
Requests use `denom`, `validator_addr` or `lookback_seconds` as applicable;
see [oracle queries](rpc.md#oracle-queries) for HTTP paths and query methods.

<a id="types-tendermint"></a>

### encoding-tendermint

The [consensus codec cases](../sdk/js/test/tendermint-encoding.test.cjs) specify
`Encoder.tendermint.types.Proposal` and `Encoder.tendermint.version.Consensus`.
The [module declaration cases](../sdk/js/test/types-declarations-tendermint.test.cjs)
specify proposal, header, commit, consensus parameter and validator types.
The [ABCI distribution contract](../sdk/js/test/abci-encoding-responses.test.cjs)
selects `ResponseInfo`, `ResponseQuery`, `ResponseCheckTx`, `ResponseDeliverTx`,
`ResponseCommit`, `ResponseFinalizeBlock`, `Event`, `EventAttribute`,
`ExecTxResult`, `TxResult` and `ValidatorUpdate`. Its
[request contract](../sdk/js/test/abci-encoding-requests.test.cjs) excludes
ABCI request messages. These types follow the local consensus proto.

<a id="types-tokenfactory"></a>

### encoding-tokenfactory

The [package guide](../sdk/js/README.md#tokenfactory-messages) specifies
`MsgCreateDenom` and `MsgMint` through `types/tokenfactory`. Coin amounts
are strings. The [codec cases](../sdk/js/test/tokenfactory-encoding.test.cjs)
specify authority metadata `issuerDid` and optional request/response
pagination for `DenomsFromCreator`. The native
[authority metadata](../api/tokenfactory/authority_metadata.proto) uses
`issuer_did`.

The [node query contract](rpc.md#tokenfactory-queries) lists raw REST bindings,
including `DenomPaused`. Select a client operation from the package's
available distribution exports.

## REST

The REST entry exports `Querier`. Query calls accept a request and transport
options. Configure `pathPrefix` with the operator's Cosmos REST origin,
without a trailing slash. A public EVM JSON-RPC origin is not a substitute.

```typescript
import { Querier } from '@pax-js/cosmos/rest';

async function readBalances(restOrigin: string, address: string) {
  return Querier.cosmos.bank.v1beta1.AllBalances({ address }, {
    pathPrefix: restOrigin,
    signal: AbortSignal.timeout(15_000),
  });
}
```

### rest-index

The [REST package guide](../sdk/js/README.md#pax-jscosmosrest) specifies
`Querier`, including `Querier.cosmos.bank.v1beta1.AllBalances`. Configure the
Cosmos REST base URL explicitly.

### rest-fetch

The package guide specifies `pathPrefix` concatenation and a runtime-provided
`fetch`. Query helpers return parsed JSON and throw the parsed response body
on a non-success HTTP status. They add no automatic retries or deadlines;
pass an abort `signal` in the second argument for cancellation. Parsed JSON
retains wire representations such as base64 bytes and decimal-string uint64
values.

### rest-cosmos

The guide's `AllBalances` example queries
`/cosmos/bank/v1beta1/balances/{address}`. The
[REST namespace cases](../sdk/js/test/rest-namespace.test.cjs) specify the
bank query namespace and exclude Cosmos mint and access-control clients.

### rest-epoch

Use [epoch queries](rpc.md#epoch-queries) for the native node HTTP bindings.

### rest-evm

Use [EVM module queries](rpc.md#evm-module-queries) for native query methods
and byte-bearing requests such as `StaticCall`. REST JSON retains its wire
representations.

### rest-mint

The [REST namespace cases](../sdk/js/test/rest-namespace.test.cjs) specify
`Querier.mint.v1beta1.Params` as a GET to `/paxchain/mint/v1beta1/params`.
See [mint queries](rpc.md#mint-queries) for the native module bindings.

### rest-oracle

Use [oracle queries](rpc.md#oracle-queries) for the native query methods.
Their HTTP prefix contains `pax-chain`.

### rest-tokenfactory

The [REST cases](../sdk/js/test/tokenfactory-rest.test.cjs) specify
`Querier.tokenfactory.DenomAuthorityMetadata` and `DenomsFromCreator`.
Path parameters are escaped; pagination uses dotted query names such as
`pagination.limit` and `pagination.key`. The
[node query contract](rpc.md#tokenfactory-queries) lists the raw REST paths
and query-only authority metadata binding.
