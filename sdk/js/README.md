# @pax-js/cosmos

`sdk/js` is the first-party `@pax-js/cosmos` package, edited in place and
aligned to the protos of this repository. Its [manifest](package.json)
declares module type, encoding and REST subpaths. Runtime use requires the
compiled files and declarations at the distribution targets below.

## Package exports

The package version is `0.1.0`. Its published file selection is `dist`.
The wildcard `*` represents a module path.

| Export | ESM import | CommonJS require | Type declarations |
| --- | --- | --- | --- |
| `./types/*` | `./dist/esm/types/*/index.js` | `./dist/cjs/types/*/index.js` | `./dist/types/types/*/index.d.ts` |
| `./rest` | `./dist/esm/rest/index.js` | `./dist/cjs/rest/index.js` | `./dist/types/rest/index.d.ts` |
| `./encoding` | `./dist/esm/encoding/index.js` | `./dist/cjs/encoding/index.js` | `./dist/types/encoding/index.d.ts` |

`typesVersions` maps `types/*`, `rest` and `encoding` to the same declaration
files. The manifest has no package-root export or aggregate `./types` export.
Its dependencies are `@bufbuild/protobuf` `^2.1.0`,
`@cosmjs/proto-signing` `^0.39.0` and `@cosmjs/stargate` `^0.39.0`.

## Message types

The bank send message is defined in
[`tx.proto`](../proto/cosmos/bank/v1beta1/tx.proto) with `from_address`,
`to_address` and repeated `amount` coins. Each coin has a denom and a
decimal-string amount in base units. The corresponding module import uses
the `types/*` export:

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

Constructing a message does not sign or broadcast it. Choose addresses,
denoms, amounts, fees and gas for the intended operation and network.

## Network configuration

Paxeer mainnet-beta is one BFT network running `paxd`. The kernel is part of
the network and proposes blocks. A transaction receipt is final once its
block is committed. The EVM chain ID is `125` (`0x7d`), and public EVM
JSON-RPC uses HTTPS at the node's root path. See the
[public RPC reference](../../docs/site/docs/reference/public-rpc.md).

Cosmos REST and Cosmos signing clients require compatible connections to
that same network and an operator-supplied Cosmos signing chain ID. The
numeric EVM chain ID does not identify a Cosmos signing chain. A public
EVM endpoint alone does not establish a Cosmos REST or signing transport.

Related interface contracts are available for
[SDK packaging](../../contracts/sdk-js.md),
[bridge interfaces](../../contracts/bridge/README.md#interface-contracts),
[explorer HTTP](../../apps/explorer/public/openapi.yaml) and
[explorer WebSocket payloads](../../apps/explorer/lib/socket/types.ts).

## Package commands

From the repository root, install the locked dependencies:

```sh
npm --prefix sdk/js ci
```

The manifest provides a test script and no build script. With the declared
distribution files available, run:

```sh
npm --prefix sdk/js test
```

The [package tests](test/) cover distribution entry points, declarations,
codecs, registries and REST routes. The
[README example test](test/readme-examples.test.cjs) compiles each TypeScript
block independently against the public package subpaths.
