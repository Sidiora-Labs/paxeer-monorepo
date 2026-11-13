# Merged network component contract

The merged network runs one `paxd` state machine. The EVM, SDK modules, native kernel host and program runtime share transaction state and block commitment. Mainnet-beta uses EVM chain ID `125` (`0x7d`).

## programme-integrated

[node/app.go](../../../node/app.go) assembles the shared host and native keepers. [node/upgrades.go](../../../node/upgrades.go) imports merge constructors and applies activation gates. The [design](../design.body.md) describes execution, storage and receipt stages.

## programme-surface

The merge address table in [precompiles/setup.go](../../../precompiles/setup.go) contains five constructs at `0x101A`–`0x101E` and eight agent addresses at `0x0901`–`0x0908`. Activation depends on the upgrade and module gate.

## programme-clients

Clients submit signed EVM transactions and use `eth_call` for read-only execution. Receipt verification distinguishes submission, signed execution evidence and committed-block evidence. See [platform SDKs](platform-sdk.md), [examples](platform-examples.md) and [RPC](rpc.md).

## programme-artifacts

The native kernel and program archives link into the host. The [build components](../docs/BUILD-REPORT.md) identify the repository build targets and manifests.

## programme-gate

Transaction admission, authority, activation and pause checks belong to their implementation paths. A code declaration does not imply that a particular node has enabled the corresponding surface.

## wiring-integration

The [host](../../../modules/lxphost/host.go) executes against the transaction's SDK KV context. Keeper caches, EVM logs and bank mirror writes follow the transaction outcome; committed state persists in the node stores.

## decisions

Construct behavior is documented by the linked contract pages and their source ABIs. [The documentation index](../docs/README.md) groups the network, native constructs and application services.

## programme

Hosted application services communicate with the same network. Tenant domains use registry records and scoped views, with shared EVM state and validator security.

<a id="programme-contract"></a>
