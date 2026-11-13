# JavaScript Cosmos package contract

[sdk/js/package.json](../../../sdk/js/package.json) declares `@pax-js/cosmos` version `0.1.0`, a TypeScript package for Pax protobuf types, encoding and REST clients. The package manifest declares `dist` as its published file set.

## package

| Dependency | Manifest range |
|---|---|
| `@bufbuild/protobuf` | `^2.1.0` |
| `@cosmjs/proto-signing` | `^0.39.0` |
| `@cosmjs/stargate` | `^0.39.0` |
| TypeScript (development) | `^5.6.3` |

The manifest's test script is `node --test test/`. It declares no build script.

## dist

The export map distinguishes ESM (`dist/esm`), CommonJS (`dist/cjs`) and declarations (`dist/types`). These are package artifact destinations declared by the manifest. The checked-in package contains its manifest, README and tests; it does not contain the `dist` implementation tree.

| Export | ESM target | CommonJS target | Type target |
|---|---|---|---|
| `./types/*` | `./dist/esm/types/*/index.js` | `./dist/cjs/types/*/index.js` | `./dist/types/types/*/index.d.ts` |
| `./rest` | `./dist/esm/rest/index.js` | `./dist/cjs/rest/index.js` | `./dist/types/rest/index.d.ts` |
| `./encoding` | `./dist/esm/encoding/index.js` | `./dist/cjs/encoding/index.js` | `./dist/types/encoding/index.d.ts` |

## encoding-index

Use the encoding export supplied by the installed package artifact. The repository's [protobuf schemas](../../../api) define native module message shapes; this manifest alone does not declare individual generated TypeScript exports.

## rest-index

REST client availability follows the installed package artifact and the node's enabled API listener. REST gateway routes are separate from root-path EVM JSON-RPC. See [proto APIs](proto-api.md) and [RPC](rpc.md).

<a id="sdk-js-contract"></a>

<a id="encoding-amino"></a>

<a id="encoding-common"></a>

<a id="encoding-confio"></a>

<a id="encoding-cosmos"></a>

<a id="encoding-epoch"></a>

<a id="encoding-eth"></a>

<a id="encoding-evm"></a>

<a id="encoding-google"></a>

<a id="encoding-mint"></a>

<a id="encoding-oracle"></a>

<a id="encoding-registry"></a>

<a id="encoding-tendermint"></a>

<a id="encoding-tokenfactory"></a>

<a id="rest"></a>

<a id="rest-cosmos"></a>

<a id="rest-epoch"></a>

<a id="rest-evm"></a>

<a id="rest-fetch"></a>

<a id="rest-mint"></a>

<a id="rest-oracle"></a>

<a id="rest-tokenfactory"></a>

<a id="types-confio"></a>

<a id="types-cosmos"></a>

<a id="types-epoch"></a>

<a id="types-eth"></a>

<a id="types-evm"></a>

<a id="types-google"></a>

<a id="types-mint"></a>

<a id="types-oracle"></a>

<a id="types-tendermint"></a>

<a id="types-tokenfactory"></a>
