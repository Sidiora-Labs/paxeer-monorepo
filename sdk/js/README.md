# @pax-js/cosmos

`sdk/js` holds the manifest and tests for the first-party `@pax-js/cosmos`
package, maintained in this repository and aligned to its protobuf definitions.
The package declares native-module types, protobuf encoding and REST query
subpaths for the Paxeer network running `paxd`. Runtime use requires compiled
files and declarations at the distribution targets below.

## Layout

- [README.md](README.md) — Package layout, usage, commands and configuration.
- [package.json](package.json) — Package metadata, dependencies, exports and test script.
- [package-lock.json](package-lock.json) — npm dependency resolution and integrity metadata.
- [test/](test/) — Node.js tests for distribution entry points, declarations, codecs, registries, REST routes and README examples.

### Package exports

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

### Message types

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

## Build and test

This directory holds package metadata and tests. The manifest provides a test
script and no build script, and the directory has no Makefile or source build
tree. Runtime imports and package tests use the declared distribution files.

From the repository root, install the locked dependencies:

```sh
npm --prefix sdk/js ci
```

With the declared distribution files available, run the manifest's test script:

```sh
npm --prefix sdk/js test
```

The script runs `node --test test/`. The
[README example test](test/readme-examples.test.cjs) compiles each TypeScript
block independently against the public package subpaths. The
[Node.js CI job](../../.github/workflows/ci.yml) includes this directory in
its package test matrix.

## Configuration

The checked-in tests read [package.json](package.json) for the package export
map and use TypeScript compiler options declared in the test files. They read
no environment variables or separate application configuration files.

Paxeer mainnet-beta is one BFT network running `paxd`. The kernel is part of
the network and proposes blocks. A transaction receipt is final once its
block is committed. The EVM chain ID is `125` (`0x7d`), and public EVM
JSON-RPC uses HTTPS at the node's root path.

Cosmos REST and Cosmos signing clients require compatible connections to
that same network and an operator-supplied Cosmos signing chain ID. The
numeric EVM chain ID does not identify a Cosmos signing chain. A public
EVM endpoint alone does not establish a Cosmos REST or signing transport.

REST query calls accept transport options such as `pathPrefix` and `signal`,
as shown in the [REST tests](test/rest-namespace.test.cjs). Applications supply
the Cosmos REST origin through `pathPrefix`.

## Documentation

- [SDK surfaces](../../docs/system/sdk.md) — Package entry points and related SDKs.
- [SDK documentation site page](../../docs/site/paxeer-docs/src/app/sdk/page.tsx) — The in-tree SDK and its JavaScript package.
- [Public RPC reference](../../docs/site/paxeer-docs/src/app/public-rpc/page.tsx) — Node JSON-RPC connections and network identifiers.

Related interface contracts are available for
[SDK packaging](../../contracts/sdk-js.md),
[bridge interfaces](../../contracts/bridge/README.md#interface-contracts),
[explorer HTTP](../../apps/explorer/public/openapi.yaml) and
[explorer WebSocket payloads](../../apps/explorer/lib/socket/types.ts).
