# @pax-js/cosmos

`@pax-js/cosmos` provides TypeScript message types, Protobuf codecs, CosmJS
registries and Cosmos REST query helpers for Paxeer's native modules. The
package exposes the modules present in its checked-in declarations; it does
not cover every capability of the network.

## Provenance

This package was taken from `Paxeer-Network/pax-js` at upstream tag
`v1.0.6-pax.1`. Its current home and repository are
[Sidiora-Labs/paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/sdk/js).
The [package manifest](./package.json) declares its distribution exports; this
checkout does not contain the `dist` artifacts or a `src` build tree. The local
references below describe the package contract without linking to absent files.

## Network and connection configuration

Paxeer mainnet-beta is one BFT network running `paxd`. The kernel is part of
the network and proposes blocks. A transaction receipt is final once its
block is committed.

For EVM clients, the public JSON-RPC endpoints use HTTPS at the root path:
`https://api-mainnet-beta.paxeer.network` and the numbered hosts
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The EVM chain ID is `125`
(`0x7d`). See [Public RPC endpoints](../../docs/site/docs/reference/public-rpc.md)
for the node list and method contracts.

The Cosmos examples below require operator-provided connections to that same
network: a Cosmos REST base URL for `Querier`, or a CometBFT-compatible RPC URL
for `SigningStargateClient`. Public EVM JSON-RPC URLs do not establish that
these Cosmos APIs are available. Obtain the Cosmos chain ID and compatible
wallet signer from your operator; the EVM numeric chain ID is not the Cosmos
signing chain ID. Choose transfer denoms, fee denoms, amounts and gas from the
network configuration and the intended operation.

### Installation

```sh
npm install @pax-js/cosmos
```

### Exported Packages

- [@pax-js/cosmos/types](#pax-jscosmostypes): module-specific TypeScript types
  and enums, imported through `@pax-js/cosmos/types/<module>`.
- [@pax-js/cosmos/encoding](#pax-jscosmosencoding): `Encoder`,
  `paxProtoRegistry` and `aminoConverters`.
- [@pax-js/cosmos/rest](#pax-jscosmosrest): the `Querier` namespace.

These are the public subpaths declared in [package.json](./package.json).
Import registries and converters from `/encoding`; there are no public
`/encoding/amino` or `/encoding/registry` subpaths, aggregate `/types` entry,
or package-root entry.

## Package

This is the maintained package contract for interface-registry consumers.
The [manifest](./package.json) selects these shipped targets; `*` is the
module path, for example `cosmos/bank/v1beta1` or `evm`.

| Export | ESM import | CommonJS require | Type declarations |
| --- | --- | --- | --- |
| `./types/*` | `./dist/esm/types/*/index.js` | `./dist/cjs/types/*/index.js` | `./dist/types/types/*/index.d.ts` |
| `./rest` | `./dist/esm/rest/index.js` | `./dist/cjs/rest/index.js` | `./dist/types/rest/index.d.ts` |
| `./encoding` | `./dist/esm/encoding/index.js` | `./dist/cjs/encoding/index.js` | `./dist/types/encoding/index.d.ts` |

`typesVersions` maps `types/*`, `rest` and `encoding` to the same declaration
files. Runtime imports use the CJS or ESM trees, never the declaration tree.
The retired `main` and `module` fields do not provide a root entry.
`VENDOR.kvx` and its `ref`, `sha` and `url` fields are retired; ownership and
the retained upstream tag are described in [Provenance](#provenance).

## Integration contracts

- [Encoding and signing](#pax-jscosmosencoding) documents message fields,
  Protobuf bytes, type URLs, registries and transaction failures.
- [Cosmos REST](#pax-jscosmosrest) documents the base URL, query route,
  response shape, HTTP failures and cancellation.
- [Bridge interface contracts](../../contracts/bridge/README.md#interface-contracts)
  link deployment, attestation encodings, vectors and operator recovery.
- [Explorer HTTP schema](../../apps/explorer/public/openapi.yaml) describes
  explorer HTTP routes and response/error schemas; the backend also serves
  its schema at `/api/v2/openapi`.
- [Explorer WebSocket payload types](../../apps/explorer/lib/socket/types.ts)
  describe Phoenix channel events and payloads. The
  [socket declarations](../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex)
  define `/socket` and `/socket/v2`; the
  [frontend socket lifecycle](../../apps/explorer/lib/socket/useSocketChannel.tsx)
  handles channel joining and cleanup.

Explorer HTTP and Phoenix WebSocket APIs are separate from Cosmos REST,
Cosmos signing RPC and the node's EVM JSON-RPC. Select the transport for the
operation; an explorer URL is not a signing endpoint.

## `@pax-js/cosmos/types`

Module subpaths provide transaction, query and response types. Use type-only
imports for interfaces. For example, the bank messages use `from_address`,
`to_address` and an array of coins; coin amounts are decimal strings in the
specified denom's base units.

### Example Usage

#### Bank send message

```typescript
import type { MsgSend } from '@pax-js/cosmos/types/cosmos/bank/v1beta1';

function bankSend(
  sender: string,
  recipient: string,
  denom: string,
  amount: string,
): MsgSend {
  return {
    from_address: sender,
    to_address: recipient,
    amount: [{ denom, amount }],
  };
}
```

#### TokenFactory messages

Creating a denom and minting it use separate messages. After creating the
denom successfully, use the returned `new_token_denom` for minting. Factory
denoms have the form `factory/{creator}/{subdenom}`; minting requires the
appropriate authority. These helpers construct messages without submitting
them or checking on-chain permissions.

```typescript
import type { MsgCreateDenom, MsgMint } from '@pax-js/cosmos/types/tokenfactory';

function createDenom(sender: string, subdenom: string): MsgCreateDenom {
  return { sender, subdenom };
}

function mintDenom(sender: string, createdDenom: string, amount: string): MsgMint {
  return { sender, amount: { denom: createdDenom, amount } };
}
```

`MsgCreateDenom` also accepts an optional `allow_list` with an `addresses`
array. See the [TokenFactory package contract](../../contracts/sdk-js.md#encoding-tokenfactory)
for the retained message families.

## `@pax-js/cosmos/encoding`

`Encoder` contains Protobuf codecs organized by module. Each codec exposes
`fromPartial`, `encode`, `decode` and `$type`. Encoding produces bytes; it does
not sign or broadcast a transaction, or provide a gRPC transport. The same
codecs are usable when building transactions for CosmJS clients.

### Importing

```typescript
import {
  Encoder,
  aminoConverters,
  paxProtoRegistry,
} from '@pax-js/cosmos/encoding';
```

### Encoding/Decoding and getting type URLs

The runtime codec supplies `$type`; a TypeScript interface alone has no
runtime value. Prefix `$type` with `/` to obtain the registered type URL.
The bank send type URL is `/cosmos.bank.v1beta1.MsgSend`.

```typescript
import { Encoder } from '@pax-js/cosmos/encoding';
import type { MsgSend } from '@pax-js/cosmos/types/cosmos/bank/v1beta1';

function encodeBankSend(message: MsgSend) {
  const codec = Encoder.cosmos.bank.v1beta1.MsgSend;
  const msgSend = codec.fromPartial(message);
  const encoded = codec.encode(msgSend).finish();
  const decoded = codec.decode(encoded);
  const protoMsgSend = { typeUrl: `/${codec.$type}`, value: encoded };
  return { protoMsgSend, decoded };
}
```

`protoMsgSend` has the byte-valued shape used by a Protobuf `Any` envelope.
For CosmJS `signAndBroadcast`, supply the unencoded message object as `value`,
as shown below. Constructing an envelope alone does not create a signed
transaction.

### Usage with @cosmjs/stargate

`paxProtoRegistry` contains type URL/codec pairs for `Registry`;
`aminoConverters` supplies the converter map for `AminoTypes`. Both exports
come from `/encoding`. Import `AminoTypes` explicitly from `@cosmjs/stargate`.
The package depends on CosmJS `^0.39.0`; use compatible versions in your
application.

This helper takes a wallet-provided `OfflineSigner`, a compatible Cosmos RPC
URL, the operator's Cosmos chain ID and explicit transfer and fee settings.
Enable the intended chain in the wallet before obtaining its signer. It
checks the connected chain before signing and rejects an empty account list
or an unsuccessful transaction result.

```typescript
import { Encoder, aminoConverters, paxProtoRegistry } from '@pax-js/cosmos/encoding';
import { AminoTypes, SigningStargateClient } from '@cosmjs/stargate';
import { Registry } from '@cosmjs/proto-signing';
import type { OfflineSigner } from '@cosmjs/proto-signing';

async function sendBankTokens(
  cosmosRpcUrl: string,
  offlineSigner: OfflineSigner,
  options: {
    chainId: string;
    recipient: string;
    denom: string;
    amount: string;
    feeDenom: string;
    feeAmount: string;
    gas: string;
  },
) {
  const client = await SigningStargateClient.connectWithSigner(
    cosmosRpcUrl,
    offlineSigner,
    {
      aminoTypes: new AminoTypes(aminoConverters),
      registry: new Registry(paxProtoRegistry),
    },
  );

  try {
    if ((await client.getChainId()) !== options.chainId) {
      throw new Error('Connected Cosmos chain ID differs from the configured chain');
    }
    const [account] = await offlineSigner.getAccounts();
    if (!account) {
      throw new Error('The signer has no accounts');
    }

    const codec = Encoder.cosmos.bank.v1beta1.MsgSend;
    const message = {
      typeUrl: `/${codec.$type}`,
      value: codec.fromPartial({
        from_address: account.address,
        to_address: options.recipient,
        amount: [{ denom: options.denom, amount: options.amount }],
      }),
    };
    const result = await client.signAndBroadcast(account.address, [message], {
      amount: [{ denom: options.feeDenom, amount: options.feeAmount }],
      gas: options.gas,
    });
    if (result.code !== 0) {
      throw new Error(`Transaction ${result.transactionHash} failed: ${result.rawLog}`);
    }
    return result;
  } finally {
    client.disconnect();
  }
}
```

### Interoperability with @pax-js/cosmos/types

The encoding declarations use the module interfaces from `/types/*`.
`fromPartial` returns a typed message with defaults for omitted fields, but
those defaults do not guarantee a valid transaction. Supply the required
addresses, amounts and other fields for the operation; the chain validates
transaction semantics and permissions.

### Usage with Ledger

An external Ledger integration must provide a compatible Cosmos
`OfflineAminoSigner` for the intended network and account. Pass that signer
to the signing helper above: `AminoTypes(aminoConverters)` supplies the Amino
conversion, and `Registry(paxProtoRegistry)` supplies the Protobuf codecs.
Device transport, wallet setup and derivation paths belong to the wallet
integration; this package does not implement them.

## `@pax-js/cosmos/rest`

`Querier` exposes typed query helpers for its retained Cosmos REST routes.
For example, `AllBalances` issues a GET to
`/cosmos/bank/v1beta1/balances/{address}`. Configure `pathPrefix` as the
operator's Cosmos REST base URL without a trailing slash; the helper
concatenates it with the route. The runtime must provide `fetch`.

### Example Usage

```typescript
import { Querier } from '@pax-js/cosmos/rest';

async function queryBalances(cosmosRestUrl: string, address: string) {
  const response = await Querier.cosmos.bank.v1beta1.AllBalances(
    { address },
    { pathPrefix: cosmosRestUrl },
  );
  return response;
}
```

This example retrieves the first page, including `balances` and optional
`pagination`. The helpers return parsed JSON without converting wire values
to Protobuf objects, and throw the parsed response body on a non-success
HTTP status. They do not add automatic retries or deadlines; pass a `signal`
in the second argument to control cancellation.

## Local Package Development

Install the locked dependencies from the repository root:

```sh
npm --prefix sdk/js ci
```

The package tests require the distribution artifacts selected by the manifest.
Once those artifacts are available, run:

```sh
npm --prefix sdk/js test
```

The tests exercise the package's exported CommonJS entries, declarations,
codecs, registries and REST routes, and check its provenance. The package has
no build script. Its manifest selects runtime files and declarations under
`dist/cjs`, `dist/esm` and `dist/types`, which are absent from this checkout.

The README example test compiles every TypeScript code block independently
against this package's public subpaths with TypeScript's compiler API. It
does not connect to a node, obtain a signer or broadcast a transaction.
