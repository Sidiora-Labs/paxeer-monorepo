# Address inventory

Paxeer mainnet-beta is one BFT network, with EVM chain ID **125 (`0x7d`)**.
The kernel participates in the network and proposes blocks. A transaction's
receipt is final when its block is committed. Sub-chains are tenant domains in
the database, rather than independently deployed networks.

This inventory describes addresses and activation rules in the source tree.
It is not a record of an operator's deployed contracts or an assertion that a
particular node has applied an upgrade. Use [EVM JSON-RPC](evm-rpc.md) to query
the intended node and height. External bridge deployments have their own
[records](bridge-deploy.md#records) and [identity mapping](bridge-vectors.md).

## Activation

The owning declarations are [precompiles/setup.go](../precompiles/setup.go),
[node/upgrades.go](../node/upgrades.go),
[node/precompiles_v6120.go](../node/precompiles_v6120.go), and the EVM keeper's
[custom precompile selection](../modules/evm/keeper/keeper.go).
`GetCustomPrecompilesE` constructs versioned implementations; the keeper selects
them for the queried height using upgrade activation and module state.
An ABI entry or an address constant alone does not prove availability.

Addresses below are hexadecimal suffixes of 20-byte addresses, left-padded with
zeros. For example, `0x1016` means
`0x0000000000000000000000000000000000001016`.

| Address | Name / owner | Declaration and ABI |
| --- | --- | --- |
| `0x1001` | bank | [bank](../precompiles/bank/bank.go), [ABI](../precompiles/bank/abi.json) |
| `0x1002` | wasmd | [wasmd](../precompiles/wasmd/wasmd.go), [ABI](../precompiles/wasmd/abi.json) |
| `0x1003` | json | [json](../precompiles/json/json.go), [ABI](../precompiles/json/abi.json) |
| `0x1004` | addr, including unified account methods | [addr](../precompiles/addr/addr.go), [ABI](../precompiles/addr/abi.json) |
| `0x1005` | staking | [staking](../precompiles/staking/staking.go), [ABI](../precompiles/staking/abi.json) |
| `0x1006` | gov | [gov](../precompiles/gov/gov.go), [ABI](../precompiles/gov/abi.json) |
| `0x1007` | distribution | [distribution](../precompiles/distribution/distribution.go), [ABI](../precompiles/distribution/abi.json) |
| `0x1008` | oracle | [oracle](../precompiles/oracle/oracle.go), [ABI](../precompiles/oracle/abi.json) |
| `0x1009` | ibc | [ibc](../precompiles/ibc/ibc.go), [ABI](../precompiles/ibc/abi.json) |
| `0x100a` | pointerview | [pointerview](../precompiles/pointerview/pointerview.go), [ABI](../precompiles/pointerview/abi.json) |
| `0x100b` | pointer | [pointer](../precompiles/pointer/pointer.go), [ABI](../precompiles/pointer/abi.json) |
| `0x100c` | solo | [solo](../precompiles/solo/solo.go), [ABI](../precompiles/solo/abi.json) |
| `0x1011` | P-256 verification | [p256](../precompiles/p256/p256.go), [ABI](../precompiles/p256/abi.json) |
| `0x1012` | layerxVerify, retired at merge | [verification](../precompiles/layerxverify/layerxverify.go), [ABI](../precompiles/layerxverify/abi.json) |
| `0x1013` | layerxCustody, retired through keeper state | [custody](../precompiles/layerxcustody/layerxcustody.go), [ABI](../precompiles/layerxcustody/abi.json) |
| `0x1014` | layerxAnchor, staged retirement | [anchor](../precompiles/layerxanchor/layerxanchor.go), [ABI](../precompiles/layerxanchor/abi.json) |
| `0x1015` | perpetuals at v6.12.0; historical exchange implementation below that plan | [perps](../precompiles/perps/perps.go), [ABI](../precompiles/perps/abi/abi.json) |
| `0x1016` | layerxBridge | [bridge](../precompiles/layerxbridge/layerxbridge.go), [ABI](../precompiles/layerxbridge/abi.json) |
| `0x1017` | launchpad, versioned across v6.12.0 | [launchpad](../precompiles/launchpad/launchpad.go), [ABI](../precompiles/launchpad/abi.json) |
| `0x1018` | feeToken, introduced by v6.7 | [feetoken](../precompiles/feetoken/feetoken.go), [ABI](../precompiles/feetoken/abi.json) |
| `0x1019` | xweb, introduced by v6.8 | [xweb](../precompiles/xweb/xweb.go), [ABI](../precompiles/xweb/abi.json) |
| `0x1020` | pricefeed, v6.12.0 gate | [pricefeed](../precompiles/pricefeed/pricefeed.go), [ABI](../precompiles/pricefeed/abi/abi.json) |
| `0x1021`–`0x1025` | spot PECOR, orders, stop orders, quoter, vault; v6.12.0 gate | [addresses](../modules/spot/types/keys.go), [ABIs](../precompiles/spot/abi/) |

The v6.9 activation entry gates `0x1012`–`0x1019` against their owning modules.
The v6.12.0 entry additionally gates pricefeed and the five spot addresses.
The spot vault-share and trading routing addresses described in the v6.12.0
selection comments carry no code; they are not additional precompiles.

### Merge address table

`MergeUpgrade` is `v7.0` in
[precompiles/common/precompiles.go](../precompiles/common/precompiles.go).
The table in `setup.go` names these merge addresses:

| Address | Table name |
| --- | --- |
| `0x101a` | layerxledger |
| `0x101b` | layerxpay |
| `0x101c` | layerxauth |
| `0x101d` | layerxprograms |
| `0x101e` | layerxchains |
| `0x0901` | orob |
| `0x0902` | clearing |
| `0x0903` | oracle aggregator |
| `0x0904` | pofq |
| `0x0905` | scheduler |
| `0x0906` | streams |
| `0x0907` | tee attestor |
| `0x0908` | eip712 |

`RegisterMergeConstructor` rejects unknown addresses, nil constructors and
duplicate registrations. `newMergePrecompiles` includes only registered
constructors and checks their returned address and name. Their version maps
carry the merge plan. Inspect the node's imports and registration path as well
as the table before assuming a particular binary serves an entry.

### Retired contracts and methods

The pre-merge checkpoint settlement contracts are retired references, not
contracts to deploy for the merged network. The registry source index below
identifies references whose Solidity source has been removed from this tree.

`layerxVerify` refuses calls from the merge height. The anchor executor refuses
writes at merge and refuses its remaining views at
`v7.0-anchor-store-delete`; its already-retired submission paths also refuse.
The custody executor checks `Keeper.Retired` before dispatch.
Retirement retains an address's refusal path rather than treating it as a new
empty contract. See [retirement helpers](../precompiles/common/retire.go) and
[custody retirement state](../modules/layerxcustody/keeper/retire.go).
These compatibility ABIs do not define the network's current finality rule.

Current funding is a [bank send](../precompiles/bank/Bank.sol) to the derived
[program or account address](../modules/lxphost/accountmap.go). Read asset
metadata through bank `0x1001` and pointerview `0x100a`. Verify finality from the
committed block and CometBFT commit; native state queries use per-key ICS23
proofs at that height, while EVM storage uses `eth_getProof`. Historical ABI
rows below identify compatibility sources, not replacement funding or proof
endpoints.

## Precompile Go API

`GetPrecompileInfoE` resolves ABI metadata; `GetCustomPrecompilesE` constructs
executable implementations. Metadata lookup and execution activation are
different operations. Use the ABI of the implementation selected at the
requested height. Native module messages and state types live under
`modules/`; their protobuf contracts live under `api/`.

## Error registry

[precompiles/common/lxp_errors.go](../precompiles/common/lxp_errors.go) owns
the `LXP_*` revert registry and its mapping to kernel results and module errors.
`LXPRevert` ABI-encodes `Error(string)` with a reason of the form
`LXP_<code>: <message>; <detail>` (the detail suffix is optional).
Unregistered codes become `LXP_UNKNOWN`. Fatal kernel results halt the node
and are not EVM reverts. Retired methods use `LXP_MODULE_DISABLED` with a
`<name> retired` detail. JSON-RPC transport errors are documented
[separately](evm-rpc.md#error-codes).

Bridge refusals include `LXP_BRIDGE_UNKNOWN_CHAIN`, `LXP_BRIDGE_PAUSED`,
`LXP_BRIDGE_NULLIFIED`, `LXP_BRIDGE_CAP_EXCEEDED`,
`LXP_BRIDGE_BAD_SIGNATURE` and `LXP_BRIDGE_BELOW_THRESHOLD`; the executor's
`revertCodes` maps native errors to these strings from the merge height.
Do not interpret a successful ABI metadata lookup as a successful state call.

## Client and compiler configuration

### cfg-client-toml

[config/client.toml](config/client.toml) is a Cosmos CLI configuration:
`chain-id` is empty in the template, `keyring-backend = "os"`,
`output = "text"`, `node = "tcp://localhost:26657"`, and
`broadcast-mode = "sync"`. Its consensus RPC address is distinct from the
EVM JSON-RPC listener. The mainnet mapping in
[modules/evm/config/config.go](../modules/evm/config/config.go) maps
`hyperpax_125-1` to EVM chain ID 125.

### cfg-contracts-hardhat

[hardhat.config.js](hardhat.config.js) reads `contracts/.env`, compiles
`contracts/src` with Solidity 0.8.27, Prague EVM, and the optimizer enabled
with 1,000 runs. Network `paxlocal` reads `PAXEER_LOCAL_EVM_RPC_URL`, defaulting
to `http://127.0.0.1:8545`. It does not configure a public mainnet deployment.

### cfg-foundry-paxeer-toml

[foundry.paxeer.toml](../foundry.paxeer.toml) selects `contracts/src`,
`contracts/test`, `contracts/lib`, and `contracts/out`, pins Solidity 0.8.27,
and disables automatic compiler detection. The remappings point to the
repository's pinned `forge-std` and OpenZeppelin libraries.

### cfg-foundry-toml

[foundry.toml](../foundry.toml) is a separate project rooted at `contracts`,
with artifacts under `build/forge-artifacts`, IR compilation, and 100,000
optimizer runs. It excludes `contracts/src`, `contracts/test`, libraries,
bridge and launchpad trees; it has separate names and governance profiles.
The [bridge EVM project](bridge/evm/foundry.toml) has its own configuration.
[bootstrap-libs.sh](bootstrap-libs.sh) fetches library dependencies;
`node.hyperpaxeer.com` is a library mirror, not a JSON-RPC endpoint.

Explorer consumers should use the
[backend configuration](explorer-config.md) and
[frontend configuration](explorer-frontend-config.md) contracts rather than
hard-coding availability from this table.

## Registry source index

These link targets preserve interface-registry references. Each row points
to the current source owner for its declaration, ABI, message, configuration
family or helper. It does not assert that a method is registered, a feature
is enabled, or a contract is deployed. Rows marked retired identify source
references absent from this tree and must not be used as deployment targets.

| Link targets | Source owner |
| --- | --- |
| <a id="0x1001-bank"></a>`0x1001-bank` | [precompiles/bank/abi.json](../precompiles/bank/abi.json); [precompiles/bank/bank.go](../precompiles/bank/bank.go) |
| <a id="0x1002-wasmd"></a>`0x1002-wasmd` | [precompiles/wasmd/abi.json](../precompiles/wasmd/abi.json) |
| <a id="0x1003-json"></a>`0x1003-json` | [precompiles/json/abi.json](../precompiles/json/abi.json) |
| <a id="0x1004-addr"></a>`0x1004-addr` | [precompiles/addr/abi.json](../precompiles/addr/abi.json); [precompiles/addr/addr.go](../precompiles/addr/addr.go) |
| <a id="0x1005-staking"></a>`0x1005-staking` | [precompiles/staking/abi.json](../precompiles/staking/abi.json) |
| <a id="0x1006-gov"></a>`0x1006-gov` | [precompiles/gov/abi.json](../precompiles/gov/abi.json) |
| <a id="0x1007-distribution"></a>`0x1007-distribution` | [precompiles/distribution/abi.json](../precompiles/distribution/abi.json); [precompiles/distribution/legacy/v620/abi.json](../precompiles/distribution/legacy/v620/abi.json) |
| <a id="0x1009-ibc"></a>`0x1009-ibc` | [precompiles/ibc/abi.json](../precompiles/ibc/abi.json); [precompiles/ibc/legacy/v552/abi.json](../precompiles/ibc/legacy/v552/abi.json) |
| <a id="0x100a-pointerview"></a>`0x100a-pointerview` | [precompiles/pointerview/abi.json](../precompiles/pointerview/abi.json) |
| <a id="0x100b-pointer"></a>`0x100b-pointer` | [precompiles/pointer/abi.json](../precompiles/pointer/abi.json) |
| <a id="0x100c-solo"></a>`0x100c-solo` | [precompiles/solo/abi.json](../precompiles/solo/abi.json) |
| <a id="0x1011-p256"></a>`0x1011-p256` | [precompiles/p256/abi.json](../precompiles/p256/abi.json) |
| <a id="0x1012-layerxverify"></a>`0x1012-layerxverify` | [precompiles/layerxverify/abi.json](../precompiles/layerxverify/abi.json); [precompiles/layerxverify/layerxverify.go](../precompiles/layerxverify/layerxverify.go); [precompiles/layerxverify/setup.go](../precompiles/layerxverify/setup.go) |
| <a id="0x1013-layerxcustody"></a>`0x1013-layerxcustody` | Retired reference: `modules/layerxcustody/client/cli/tx.go` is absent; [modules/layerxcustody/handler.go](../modules/layerxcustody/handler.go); [modules/layerxcustody/keeper/claims.go](../modules/layerxcustody/keeper/claims.go); [modules/layerxcustody/keeper/deposit_root.go](../modules/layerxcustody/keeper/deposit_root.go); [modules/layerxcustody/keeper/invariants.go](../modules/layerxcustody/keeper/invariants.go); [modules/layerxcustody/keeper/keeper.go](../modules/layerxcustody/keeper/keeper.go); [modules/layerxcustody/keeper/msg_server.go](../modules/layerxcustody/keeper/msg_server.go); [modules/layerxcustody/keeper/release.go](../modules/layerxcustody/keeper/release.go); [modules/layerxcustody/keeper/retire.go](../modules/layerxcustody/keeper/retire.go); [modules/layerxcustody/module.go](../modules/layerxcustody/module.go); [modules/layerxcustody/types/anchor.go](../modules/layerxcustody/types/anchor.go); [modules/layerxcustody/types/codec.go](../modules/layerxcustody/types/codec.go); [modules/layerxcustody/types/expected_keepers.go](../modules/layerxcustody/types/expected_keepers.go); [modules/layerxcustody/types/genesis.go](../modules/layerxcustody/types/genesis.go); [modules/layerxcustody/types/governance_activation.go](../modules/layerxcustody/types/governance_activation.go); [modules/layerxcustody/types/hex.go](../modules/layerxcustody/types/hex.go); [modules/layerxcustody/types/ids.go](../modules/layerxcustody/types/ids.go); [modules/layerxcustody/types/keys.go](../modules/layerxcustody/types/keys.go); [modules/layerxcustody/types/msgs.go](../modules/layerxcustody/types/msgs.go); [modules/layerxcustody/types/params.go](../modules/layerxcustody/types/params.go); [modules/layerxcustody/types/proposal.go](../modules/layerxcustody/types/proposal.go); [modules/layerxcustody/types/retire.go](../modules/layerxcustody/types/retire.go); [modules/layerxcustody/types/tx.pb.go](../modules/layerxcustody/types/tx.pb.go); [precompiles/layerxcustody/abi.json](../precompiles/layerxcustody/abi.json); [precompiles/layerxcustody/layerxcustody.go](../precompiles/layerxcustody/layerxcustody.go); [precompiles/layerxcustody/setup.go](../precompiles/layerxcustody/setup.go) |
| <a id="0x1014-layerxanchor"></a>`0x1014-layerxanchor` | [precompiles/layerxanchor/abi.json](../precompiles/layerxanchor/abi.json); [precompiles/layerxanchor/layerxanchor.go](../precompiles/layerxanchor/layerxanchor.go); [precompiles/layerxanchor/setup.go](../precompiles/layerxanchor/setup.go) |
| <a id="0x1016-layerxbridge"></a>`0x1016-layerxbridge` | [modules/layerxbridge/client/cli/tx.go](../modules/layerxbridge/client/cli/tx.go); [modules/layerxbridge/handler.go](../modules/layerxbridge/handler.go); [modules/layerxbridge/keeper/bridge.go](../modules/layerxbridge/keeper/bridge.go); [modules/layerxbridge/keeper/keeper.go](../modules/layerxbridge/keeper/keeper.go); [modules/layerxbridge/keeper/msg_server.go](../modules/layerxbridge/keeper/msg_server.go); [modules/layerxbridge/keeper/sidiora.go](../modules/layerxbridge/keeper/sidiora.go); [modules/layerxbridge/module.go](../modules/layerxbridge/module.go); [modules/layerxbridge/testutil/attestors.go](../modules/layerxbridge/testutil/attestors.go); [modules/layerxbridge/testutil/store.go](../modules/layerxbridge/testutil/store.go); [modules/layerxbridge/types/attestation.go](../modules/layerxbridge/types/attestation.go); [modules/layerxbridge/types/codec.go](../modules/layerxbridge/types/codec.go); [modules/layerxbridge/types/events.go](../modules/layerxbridge/types/events.go); [modules/layerxbridge/types/expected_keepers.go](../modules/layerxbridge/types/expected_keepers.go); [modules/layerxbridge/types/genesis.go](../modules/layerxbridge/types/genesis.go); [modules/layerxbridge/types/keys.go](../modules/layerxbridge/types/keys.go); [modules/layerxbridge/types/msgs.go](../modules/layerxbridge/types/msgs.go); [modules/layerxbridge/types/proposal.go](../modules/layerxbridge/types/proposal.go); [modules/layerxbridge/types/state.go](../modules/layerxbridge/types/state.go); [modules/layerxbridge/types/tx.pb.go](../modules/layerxbridge/types/tx.pb.go); [precompiles/layerxbridge/abi.json](../precompiles/layerxbridge/abi.json); [precompiles/layerxbridge/layerxbridge.go](../precompiles/layerxbridge/layerxbridge.go); [precompiles/layerxbridge/setup.go](../precompiles/layerxbridge/setup.go) |
| <a id="0x1018-feetoken"></a>`0x1018-feetoken` | [precompiles/feetoken/abi.json](../precompiles/feetoken/abi.json); [precompiles/feetoken/feetoken.go](../precompiles/feetoken/feetoken.go) |
| <a id="cli-contracts-bootstrap-libs"></a>`cli-contracts-bootstrap-libs` | [contracts/bootstrap-libs.sh](../contracts/bootstrap-libs.sh) |
| <a id="cli-evm-query-erc1155-payload"></a>`cli-evm-query-erc1155-payload`, <a id="cli-evm-query-erc20"></a>`cli-evm-query-erc20`, <a id="cli-evm-query-erc20-payload"></a>`cli-evm-query-erc20-payload`, <a id="cli-evm-query-erc721-payload"></a>`cli-evm-query-erc721-payload`, <a id="cli-evm-query-evm-addr"></a>`cli-evm-query-evm-addr`, <a id="cli-evm-query-pax-addr"></a>`cli-evm-query-pax-addr`, <a id="cli-evm-query-payload"></a>`cli-evm-query-payload`, <a id="cli-evm-query-pointee"></a>`cli-evm-query-pointee`, <a id="cli-evm-query-pointer"></a>`cli-evm-query-pointer`, <a id="cli-evm-query-pointer-version"></a>`cli-evm-query-pointer-version`, <a id="cli-evm-query-root"></a>`cli-evm-query-root`, <a id="cli-evm-query-tx"></a>`cli-evm-query-tx` | [modules/evm/client/cli/query.go](../modules/evm/client/cli/query.go) |
| <a id="cli-evm-tx-add-erc-native-pointer"></a>`cli-evm-tx-add-erc-native-pointer`, <a id="cli-evm-tx-bind-erc-native-pointer"></a>`cli-evm-tx-bind-erc-native-pointer` | [modules/evm/client/cli/gov_tx.go](../modules/evm/client/cli/gov_tx.go) |
| <a id="cli-evm-tx-associate-address"></a>`cli-evm-tx-associate-address`, <a id="cli-evm-tx-call-contract"></a>`cli-evm-tx-call-contract`, <a id="cli-evm-tx-call-precompile"></a>`cli-evm-tx-call-precompile`, <a id="cli-evm-tx-deploy"></a>`cli-evm-tx-deploy`, <a id="cli-evm-tx-deploy-wpax"></a>`cli-evm-tx-deploy-wpax`, <a id="cli-evm-tx-erc20-send"></a>`cli-evm-tx-erc20-send`, <a id="cli-evm-tx-root"></a>`cli-evm-tx-root`, <a id="cli-evm-tx-send"></a>`cli-evm-tx-send` | [modules/evm/client/cli/tx.go](../modules/evm/client/cli/tx.go) |
| <a id="cli-evm-tx-associate-contract-address"></a>`cli-evm-tx-associate-contract-address`, <a id="cli-evm-tx-native-associate"></a>`cli-evm-tx-native-associate`, <a id="cli-evm-tx-native-send"></a>`cli-evm-tx-native-send`, <a id="cli-evm-tx-print-claim"></a>`cli-evm-tx-print-claim`, <a id="cli-evm-tx-print-claim-by-sender"></a>`cli-evm-tx-print-claim-by-sender`, <a id="cli-evm-tx-print-claim-specific"></a>`cli-evm-tx-print-claim-specific`, <a id="cli-evm-tx-register-cw-pointer"></a>`cli-evm-tx-register-cw-pointer`, <a id="cli-evm-tx-register-evm-pointer"></a>`cli-evm-tx-register-evm-pointer` | [modules/evm/client/cli/native_tx.go](../modules/evm/client/cli/native_tx.go) |
| <a id="cli-layerxbridge-gov-layerxbridge-proposal"></a>`cli-layerxbridge-gov-layerxbridge-proposal` | [modules/layerxbridge/client/cli/tx.go](../modules/layerxbridge/client/cli/tx.go) |
| <a id="cli-layerxcustody-tx-root"></a>`cli-layerxcustody-tx-root`, <a id="cli-layerxcustody-tx-submit-proposal"></a>`cli-layerxcustody-tx-submit-proposal`, <a id="cli-layerxcustody-tx-submit-proposal-custody"></a>`cli-layerxcustody-tx-submit-proposal-custody` | Retired reference: `modules/layerxcustody/client/cli/tx.go` is absent |
| <a id="cli-layerxgov-gov-layerx-proposal"></a>`cli-layerxgov-gov-layerx-proposal` | [modules/layerxgov/client/cli/tx.go](../modules/layerxgov/client/cli/tx.go) |
| <a id="cli-tokenfactory-query-denom-authority-metadata"></a>`cli-tokenfactory-query-denom-authority-metadata`, <a id="cli-tokenfactory-query-denoms-from-creator"></a>`cli-tokenfactory-query-denoms-from-creator`, <a id="cli-tokenfactory-query-params"></a>`cli-tokenfactory-query-params` | [modules/tokenfactory/client/cli/query.go](../modules/tokenfactory/client/cli/query.go) |
| <a id="cli-tokenfactory-tx-burn"></a>`cli-tokenfactory-tx-burn`, <a id="cli-tokenfactory-tx-change-admin"></a>`cli-tokenfactory-tx-change-admin`, <a id="cli-tokenfactory-tx-create-denom"></a>`cli-tokenfactory-tx-create-denom`, <a id="cli-tokenfactory-tx-mint"></a>`cli-tokenfactory-tx-mint`, <a id="cli-tokenfactory-tx-set-denom-metadata"></a>`cli-tokenfactory-tx-set-denom-metadata`, <a id="cli-tokenfactory-tx-update-denom"></a>`cli-tokenfactory-tx-update-denom` | [modules/tokenfactory/client/cli/tx.go](../modules/tokenfactory/client/cli/tx.go) |
| <a id="env-paxeer-local-evm-rpc-url"></a>`env-paxeer-local-evm-rpc-url` | [contracts/hardhat.config.js](../contracts/hardhat.config.js) |
| <a id="evm-errors"></a>`evm-errors` | [modules/evm/types/errors.go](../modules/evm/types/errors.go); [modules/evm/types/layerx_binding.go](../modules/evm/types/layerx_binding.go) |
| <a id="evm-events"></a>`evm-events` | [modules/evm/types/events.go](../modules/evm/types/events.go) |
| <a id="fmt-checkpoint-settlement"></a>`fmt-checkpoint-settlement` | [contracts/config/checkpoint-settlement.json](../contracts/config/checkpoint-settlement.json) |
| <a id="fmt-ed25519-vectors"></a>`fmt-ed25519-vectors` | Retired reference: `contracts/config/ed25519-vectors.json` is absent |
| <a id="fmt-native-state-proofs"></a>`fmt-native-state-proofs` | [contracts/config/native-state-proofs.json](../contracts/config/native-state-proofs.json) |
| <a id="fmt-native-withdrawal-proof"></a>`fmt-native-withdrawal-proof` | Retired reference: `contracts/config/native-withdrawal-proof.json` is absent |
| <a id="fmt-sidioraproxygovernanceproposal"></a>`fmt-sidioraproxygovernanceproposal` | [contracts/governance/SidioraProxyGovernanceProposal.json](../contracts/governance/SidioraProxyGovernanceProposal.json) |
| <a id="layerxanchor-errors"></a>`layerxanchor-errors` | [modules/layerxanchor/types/errors.go](../modules/layerxanchor/types/errors.go) |
| <a id="layerxanchor-events"></a>`layerxanchor-events` | [modules/layerxanchor/types/events.go](../modules/layerxanchor/types/events.go) |
| <a id="layerxbridge-errors"></a>`layerxbridge-errors` | [modules/layerxbridge/types/errors.go](../modules/layerxbridge/types/errors.go) |
| <a id="layerxbridge-events"></a>`layerxbridge-events` | [modules/layerxbridge/types/events.go](../modules/layerxbridge/types/events.go) |
| <a id="layerxcustody-errors"></a>`layerxcustody-errors` | [modules/layerxcustody/types/errors.go](../modules/layerxcustody/types/errors.go) |
| <a id="layerxcustody-events"></a>`layerxcustody-events` | [api/layerxcustody/events.proto](../api/layerxcustody/events.proto); [modules/layerxcustody/keeper/anchor.go](../modules/layerxcustody/keeper/anchor.go); [modules/layerxcustody/keeper/claims.go](../modules/layerxcustody/keeper/claims.go); [modules/layerxcustody/keeper/deposit.go](../modules/layerxcustody/keeper/deposit.go); [modules/layerxcustody/keeper/deposit_root.go](../modules/layerxcustody/keeper/deposit_root.go); [modules/layerxcustody/keeper/keeper.go](../modules/layerxcustody/keeper/keeper.go); [modules/layerxcustody/keeper/release.go](../modules/layerxcustody/keeper/release.go) |
| <a id="layerxgov-errors"></a>`layerxgov-errors` | [modules/layerxgov/types/proposal.go](../modules/layerxgov/types/proposal.go) |
| <a id="msg-evm-associate"></a>`msg-evm-associate`, <a id="msg-evm-associatecontractaddress"></a>`msg-evm-associatecontractaddress`, <a id="msg-evm-bindercnativepointer"></a>`msg-evm-bindercnativepointer`, <a id="msg-evm-claim"></a>`msg-evm-claim`, <a id="msg-evm-claimspecific"></a>`msg-evm-claimspecific`, <a id="msg-evm-evmtransaction"></a>`msg-evm-evmtransaction`, <a id="msg-evm-internalevmcall"></a>`msg-evm-internalevmcall`, <a id="msg-evm-internalevmdelegatecall"></a>`msg-evm-internalevmdelegatecall`, <a id="msg-evm-registerpointer"></a>`msg-evm-registerpointer`, <a id="msg-evm-send"></a>`msg-evm-send`, <a id="proto-evm-asset"></a>`proto-evm-asset`, <a id="proto-evm-msgassociate"></a>`proto-evm-msgassociate`, <a id="proto-evm-msgassociatecontractaddress"></a>`proto-evm-msgassociatecontractaddress`, <a id="proto-evm-msgassociatecontractaddressresponse"></a>`proto-evm-msgassociatecontractaddressresponse`, <a id="proto-evm-msgassociateresponse"></a>`proto-evm-msgassociateresponse`, <a id="proto-evm-msgbindercnativepointer"></a>`proto-evm-msgbindercnativepointer`, <a id="proto-evm-msgbindercnativepointerresponse"></a>`proto-evm-msgbindercnativepointerresponse`, <a id="proto-evm-msgclaim"></a>`proto-evm-msgclaim`, <a id="proto-evm-msgclaimspecific"></a>`proto-evm-msgclaimspecific`, <a id="proto-evm-msgevmtransaction"></a>`proto-evm-msgevmtransaction`, <a id="proto-evm-msgevmtransactionresponse"></a>`proto-evm-msgevmtransactionresponse`, <a id="proto-evm-msginternalevmcall"></a>`proto-evm-msginternalevmcall`, <a id="proto-evm-msginternalevmcallresponse"></a>`proto-evm-msginternalevmcallresponse`, <a id="proto-evm-msginternalevmdelegatecall"></a>`proto-evm-msginternalevmdelegatecall`, <a id="proto-evm-msginternalevmdelegatecallresponse"></a>`proto-evm-msginternalevmdelegatecallresponse`, <a id="proto-evm-msgregisterpointer"></a>`proto-evm-msgregisterpointer`, <a id="proto-evm-msgregisterpointerresponse"></a>`proto-evm-msgregisterpointerresponse`, <a id="proto-evm-msgsend"></a>`proto-evm-msgsend`, <a id="proto-evm-msgsendresponse"></a>`proto-evm-msgsendresponse` | [api/evm/tx.proto](../api/evm/tx.proto) |
| <a id="msg-layerxanchor-updateparams"></a>`msg-layerxanchor-updateparams`, <a id="proto-layerxanchor-msgupdateparams"></a>`proto-layerxanchor-msgupdateparams`, <a id="proto-layerxanchor-msgupdateparamsresponse"></a>`proto-layerxanchor-msgupdateparamsresponse`, <a id="proto-layerxanchor-params"></a>`proto-layerxanchor-params` | [api/layerxanchor/tx.proto](../api/layerxanchor/tx.proto) |
| <a id="msg-layerxbridge-pause"></a>`msg-layerxbridge-pause`, <a id="msg-layerxbridge-registerchain"></a>`msg-layerxbridge-registerchain`, <a id="msg-layerxbridge-registersidiorapair"></a>`msg-layerxbridge-registersidiorapair`, <a id="msg-layerxbridge-setattestors"></a>`msg-layerxbridge-setattestors`, <a id="msg-layerxbridge-setcap"></a>`msg-layerxbridge-setcap`, <a id="msg-layerxbridge-unpause"></a>`msg-layerxbridge-unpause`, <a id="proto-layerxbridge-attestor"></a>`proto-layerxbridge-attestor`, <a id="proto-layerxbridge-attestorset"></a>`proto-layerxbridge-attestorset`, <a id="proto-layerxbridge-chain"></a>`proto-layerxbridge-chain`, <a id="proto-layerxbridge-msgpause"></a>`proto-layerxbridge-msgpause`, <a id="proto-layerxbridge-msgpauseresponse"></a>`proto-layerxbridge-msgpauseresponse`, <a id="proto-layerxbridge-msgregisterchain"></a>`proto-layerxbridge-msgregisterchain`, <a id="proto-layerxbridge-msgregisterchainresponse"></a>`proto-layerxbridge-msgregisterchainresponse`, <a id="proto-layerxbridge-msgregistersidiorapair"></a>`proto-layerxbridge-msgregistersidiorapair`, <a id="proto-layerxbridge-msgregistersidiorapairresponse"></a>`proto-layerxbridge-msgregistersidiorapairresponse`, <a id="proto-layerxbridge-msgsetattestors"></a>`proto-layerxbridge-msgsetattestors`, <a id="proto-layerxbridge-msgsetattestorsresponse"></a>`proto-layerxbridge-msgsetattestorsresponse`, <a id="proto-layerxbridge-msgsetcap"></a>`proto-layerxbridge-msgsetcap`, <a id="proto-layerxbridge-msgsetcapresponse"></a>`proto-layerxbridge-msgsetcapresponse`, <a id="proto-layerxbridge-msgunpause"></a>`proto-layerxbridge-msgunpause`, <a id="proto-layerxbridge-msgunpauseresponse"></a>`proto-layerxbridge-msgunpauseresponse` | [api/layerxbridge/tx.proto](../api/layerxbridge/tx.proto) |
| <a id="msg-layerxcustody-cancelclaim"></a>`msg-layerxcustody-cancelclaim`, <a id="msg-layerxcustody-executeforcedexit"></a>`msg-layerxcustody-executeforcedexit`, <a id="msg-layerxcustody-finalisewithdrawal"></a>`msg-layerxcustody-finalisewithdrawal`, <a id="msg-layerxcustody-registercheckpoint"></a>`msg-layerxcustody-registercheckpoint`, <a id="msg-layerxcustody-requestforcedexit"></a>`msg-layerxcustody-requestforcedexit`, <a id="msg-layerxcustody-requestwithdrawal"></a>`msg-layerxcustody-requestwithdrawal`, <a id="msg-layerxcustody-setasset"></a>`msg-layerxcustody-setasset`, <a id="msg-layerxcustody-setemergency"></a>`msg-layerxcustody-setemergency`, <a id="msg-layerxcustody-updateparams"></a>`msg-layerxcustody-updateparams`, <a id="proto-layerxcustody-msgcancelclaim"></a>`proto-layerxcustody-msgcancelclaim`, <a id="proto-layerxcustody-msgcancelclaimresponse"></a>`proto-layerxcustody-msgcancelclaimresponse`, <a id="proto-layerxcustody-msgexecuteforcedexit"></a>`proto-layerxcustody-msgexecuteforcedexit`, <a id="proto-layerxcustody-msgexecuteforcedexitresponse"></a>`proto-layerxcustody-msgexecuteforcedexitresponse`, <a id="proto-layerxcustody-msgfinalisewithdrawal"></a>`proto-layerxcustody-msgfinalisewithdrawal`, <a id="proto-layerxcustody-msgfinalisewithdrawalresponse"></a>`proto-layerxcustody-msgfinalisewithdrawalresponse`, <a id="proto-layerxcustody-msgregistercheckpoint"></a>`proto-layerxcustody-msgregistercheckpoint`, <a id="proto-layerxcustody-msgregistercheckpointresponse"></a>`proto-layerxcustody-msgregistercheckpointresponse`, <a id="proto-layerxcustody-msgrequestforcedexit"></a>`proto-layerxcustody-msgrequestforcedexit`, <a id="proto-layerxcustody-msgrequestforcedexitresponse"></a>`proto-layerxcustody-msgrequestforcedexitresponse`, <a id="proto-layerxcustody-msgrequestwithdrawal"></a>`proto-layerxcustody-msgrequestwithdrawal`, <a id="proto-layerxcustody-msgrequestwithdrawalresponse"></a>`proto-layerxcustody-msgrequestwithdrawalresponse`, <a id="proto-layerxcustody-msgsetasset"></a>`proto-layerxcustody-msgsetasset`, <a id="proto-layerxcustody-msgsetassetresponse"></a>`proto-layerxcustody-msgsetassetresponse`, <a id="proto-layerxcustody-msgsetemergency"></a>`proto-layerxcustody-msgsetemergency`, <a id="proto-layerxcustody-msgsetemergencyresponse"></a>`proto-layerxcustody-msgsetemergencyresponse`, <a id="proto-layerxcustody-msgupdateparams"></a>`proto-layerxcustody-msgupdateparams`, <a id="proto-layerxcustody-msgupdateparamsresponse"></a>`proto-layerxcustody-msgupdateparamsresponse` | [api/layerxcustody/tx.proto](../api/layerxcustody/tx.proto) |
| <a id="msg-tokenfactory-burn"></a>`msg-tokenfactory-burn`, <a id="msg-tokenfactory-changeadmin"></a>`msg-tokenfactory-changeadmin`, <a id="msg-tokenfactory-createdenom"></a>`msg-tokenfactory-createdenom`, <a id="msg-tokenfactory-mint"></a>`msg-tokenfactory-mint`, <a id="msg-tokenfactory-setdenommetadata"></a>`msg-tokenfactory-setdenommetadata`, <a id="msg-tokenfactory-updatedenom"></a>`msg-tokenfactory-updatedenom`, <a id="proto-tokenfactory-msgburn"></a>`proto-tokenfactory-msgburn`, <a id="proto-tokenfactory-msgburnresponse"></a>`proto-tokenfactory-msgburnresponse`, <a id="proto-tokenfactory-msgchangeadmin"></a>`proto-tokenfactory-msgchangeadmin`, <a id="proto-tokenfactory-msgchangeadminresponse"></a>`proto-tokenfactory-msgchangeadminresponse`, <a id="proto-tokenfactory-msgcreatedenom"></a>`proto-tokenfactory-msgcreatedenom`, <a id="proto-tokenfactory-msgcreatedenomresponse"></a>`proto-tokenfactory-msgcreatedenomresponse`, <a id="proto-tokenfactory-msgmint"></a>`proto-tokenfactory-msgmint`, <a id="proto-tokenfactory-msgmintresponse"></a>`proto-tokenfactory-msgmintresponse`, <a id="proto-tokenfactory-msgsetdenommetadata"></a>`proto-tokenfactory-msgsetdenommetadata`, <a id="proto-tokenfactory-msgsetdenommetadataresponse"></a>`proto-tokenfactory-msgsetdenommetadataresponse`, <a id="proto-tokenfactory-msgsetdenompaused"></a>`proto-tokenfactory-msgsetdenompaused`, <a id="proto-tokenfactory-msgsetdenompausedresponse"></a>`proto-tokenfactory-msgsetdenompausedresponse`, <a id="proto-tokenfactory-msgupdatedenom"></a>`proto-tokenfactory-msgupdatedenom`, <a id="proto-tokenfactory-msgupdatedenomresponse"></a>`proto-tokenfactory-msgupdatedenomresponse` | [api/tokenfactory/tx.proto](../api/tokenfactory/tx.proto) |
| <a id="proto-evm-addcwerc1155pointerproposal"></a>`proto-evm-addcwerc1155pointerproposal`, <a id="proto-evm-addcwerc20pointerproposal"></a>`proto-evm-addcwerc20pointerproposal`, <a id="proto-evm-addcwerc721pointerproposal"></a>`proto-evm-addcwerc721pointerproposal`, <a id="proto-evm-adderccw1155pointerproposal"></a>`proto-evm-adderccw1155pointerproposal`, <a id="proto-evm-adderccw20pointerproposal"></a>`proto-evm-adderccw20pointerproposal`, <a id="proto-evm-adderccw721pointerproposal"></a>`proto-evm-adderccw721pointerproposal`, <a id="proto-evm-addercnativepointerproposal"></a>`proto-evm-addercnativepointerproposal`, <a id="proto-evm-addercnativepointerproposalv2"></a>`proto-evm-addercnativepointerproposalv2`, <a id="proto-evm-pointerbindingproposal"></a>`proto-evm-pointerbindingproposal` | [api/evm/gov.proto](../api/evm/gov.proto) |
| <a id="proto-evm-addressassociation"></a>`proto-evm-addressassociation`, <a id="proto-evm-code"></a>`proto-evm-code`, <a id="proto-evm-contractstate"></a>`proto-evm-contractstate`, <a id="proto-evm-genesisstate"></a>`proto-evm-genesisstate`, <a id="proto-evm-nonce"></a>`proto-evm-nonce`, <a id="proto-evm-serialized"></a>`proto-evm-serialized` | [api/evm/genesis.proto](../api/evm/genesis.proto) |
| <a id="proto-evm-allowedfeedenom"></a>`proto-evm-allowedfeedenom`, <a id="proto-evm-params"></a>`proto-evm-params`, <a id="proto-evm-paramsprev580"></a>`proto-evm-paramsprev580`, <a id="proto-evm-paramsprev600"></a>`proto-evm-paramsprev600`, <a id="proto-evm-paramsprev601"></a>`proto-evm-paramsprev601`, <a id="proto-evm-paramsprev606"></a>`proto-evm-paramsprev606` | [api/evm/params.proto](../api/evm/params.proto) |
| <a id="proto-evm-assettype"></a>`proto-evm-assettype`, <a id="proto-evm-pointertype"></a>`proto-evm-pointertype` | [api/evm/enums.proto](../api/evm/enums.proto) |
| <a id="proto-evm-chainconfig"></a>`proto-evm-chainconfig` | [api/evm/config.proto](../api/evm/config.proto) |
| <a id="proto-evm-deferredinfo"></a>`proto-evm-deferredinfo`, <a id="proto-evm-whitelist"></a>`proto-evm-whitelist` | [api/evm/types.proto](../api/evm/types.proto) |
| <a id="proto-evm-log"></a>`proto-evm-log`, <a id="proto-evm-receipt"></a>`proto-evm-receipt` | [api/evm/receipt.proto](../api/evm/receipt.proto) |
| <a id="proto-evm-queryevmaddressbypaxaddressrequest"></a>`proto-evm-queryevmaddressbypaxaddressrequest`, <a id="proto-evm-queryevmaddressbypaxaddressresponse"></a>`proto-evm-queryevmaddressbypaxaddressresponse`, <a id="proto-evm-querypaxaddressbyevmaddressrequest"></a>`proto-evm-querypaxaddressbyevmaddressrequest`, <a id="proto-evm-querypaxaddressbyevmaddressresponse"></a>`proto-evm-querypaxaddressbyevmaddressresponse`, <a id="proto-evm-querypointeerequest"></a>`proto-evm-querypointeerequest`, <a id="proto-evm-querypointeeresponse"></a>`proto-evm-querypointeeresponse`, <a id="proto-evm-querypointerrequest"></a>`proto-evm-querypointerrequest`, <a id="proto-evm-querypointerresponse"></a>`proto-evm-querypointerresponse`, <a id="proto-evm-querypointerversionrequest"></a>`proto-evm-querypointerversionrequest`, <a id="proto-evm-querypointerversionresponse"></a>`proto-evm-querypointerversionresponse`, <a id="proto-evm-querystaticcallrequest"></a>`proto-evm-querystaticcallrequest`, <a id="proto-evm-querystaticcallresponse"></a>`proto-evm-querystaticcallresponse` | [api/evm/query.proto](../api/evm/query.proto) |
| <a id="proto-layerxbridge-bridgeproposal"></a>`proto-layerxbridge-bridgeproposal` | [api/layerxbridge/proposal.proto](../api/layerxbridge/proposal.proto) |
| <a id="proto-layerxcustody-assetmapping"></a>`proto-layerxcustody-assetmapping`, <a id="proto-layerxcustody-assettotals"></a>`proto-layerxcustody-assettotals`, <a id="proto-layerxcustody-checkpoint"></a>`proto-layerxcustody-checkpoint`, <a id="proto-layerxcustody-claim"></a>`proto-layerxcustody-claim`, <a id="proto-layerxcustody-claimkind"></a>`proto-layerxcustody-claimkind`, <a id="proto-layerxcustody-claimstatus"></a>`proto-layerxcustody-claimstatus`, <a id="proto-layerxcustody-consumedbalance"></a>`proto-layerxcustody-consumedbalance`, <a id="proto-layerxcustody-deposit"></a>`proto-layerxcustody-deposit`, <a id="proto-layerxcustody-depositnonce"></a>`proto-layerxcustody-depositnonce`, <a id="proto-layerxcustody-depositrootregistration"></a>`proto-layerxcustody-depositrootregistration`, <a id="proto-layerxcustody-nullifier"></a>`proto-layerxcustody-nullifier`, <a id="proto-layerxcustody-nullifierstatus"></a>`proto-layerxcustody-nullifierstatus` | [api/layerxcustody/state.proto](../api/layerxcustody/state.proto) |
| <a id="proto-layerxcustody-custodyproposal"></a>`proto-layerxcustody-custodyproposal` | [api/layerxcustody/proposal.proto](../api/layerxcustody/proposal.proto) |
| <a id="proto-layerxcustody-eventcheckpointregistered"></a>`proto-layerxcustody-eventcheckpointregistered`, <a id="proto-layerxcustody-eventclaimcancelled"></a>`proto-layerxcustody-eventclaimcancelled`, <a id="proto-layerxcustody-eventclaimfinalised"></a>`proto-layerxcustody-eventclaimfinalised`, <a id="proto-layerxcustody-eventclaimqueued"></a>`proto-layerxcustody-eventclaimqueued`, <a id="proto-layerxcustody-eventcustodydeposit"></a>`proto-layerxcustody-eventcustodydeposit`, <a id="proto-layerxcustody-eventcustodyrelease"></a>`proto-layerxcustody-eventcustodyrelease`, <a id="proto-layerxcustody-eventdepositrootregistered"></a>`proto-layerxcustody-eventdepositrootregistered`, <a id="proto-layerxcustody-eventemergencyset"></a>`proto-layerxcustody-eventemergencyset`, <a id="proto-layerxcustody-eventforcedexitexecuted"></a>`proto-layerxcustody-eventforcedexitexecuted` | [api/layerxcustody/events.proto](../api/layerxcustody/events.proto) |
| <a id="proto-layerxcustody-genesisstate"></a>`proto-layerxcustody-genesisstate` | [api/layerxcustody/genesis.proto](../api/layerxcustody/genesis.proto) |
| <a id="proto-layerxcustody-params"></a>`proto-layerxcustody-params`, <a id="proto-layerxcustody-sequencerauthorization"></a>`proto-layerxcustody-sequencerauthorization` | [api/layerxcustody/params.proto](../api/layerxcustody/params.proto) |
| <a id="proto-layerxgov-layerxproposal"></a>`proto-layerxgov-layerxproposal` | [api/layerxgov/proposal.proto](../api/layerxgov/proposal.proto) |
| <a id="proto-tokenfactory-denomauthoritymetadata"></a>`proto-tokenfactory-denomauthoritymetadata` | [api/tokenfactory/authority_metadata.proto](../api/tokenfactory/authority_metadata.proto) |
| <a id="proto-tokenfactory-genesisdenom"></a>`proto-tokenfactory-genesisdenom` | [api/tokenfactory/genesis.proto](../api/tokenfactory/genesis.proto) |
| <a id="proto-tokenfactory-params"></a>`proto-tokenfactory-params` | [api/tokenfactory/params.proto](../api/tokenfactory/params.proto) |
| <a id="proto-tokenfactory-querydenomallowlistrequest"></a>`proto-tokenfactory-querydenomallowlistrequest`, <a id="proto-tokenfactory-querydenomallowlistresponse"></a>`proto-tokenfactory-querydenomallowlistresponse`, <a id="proto-tokenfactory-querydenomauthoritymetadatarequest"></a>`proto-tokenfactory-querydenomauthoritymetadatarequest`, <a id="proto-tokenfactory-querydenomauthoritymetadataresponse"></a>`proto-tokenfactory-querydenomauthoritymetadataresponse`, <a id="proto-tokenfactory-querydenommetadatarequest"></a>`proto-tokenfactory-querydenommetadatarequest`, <a id="proto-tokenfactory-querydenommetadataresponse"></a>`proto-tokenfactory-querydenommetadataresponse`, <a id="proto-tokenfactory-querydenomsfromcreatorrequest"></a>`proto-tokenfactory-querydenomsfromcreatorrequest`, <a id="proto-tokenfactory-querydenomsfromcreatorresponse"></a>`proto-tokenfactory-querydenomsfromcreatorresponse`, <a id="proto-tokenfactory-queryparamsrequest"></a>`proto-tokenfactory-queryparamsrequest`, <a id="proto-tokenfactory-queryparamsresponse"></a>`proto-tokenfactory-queryparamsresponse` | [api/tokenfactory/query.proto](../api/tokenfactory/query.proto) |
| <a id="sol-arithmetic"></a>`sol-arithmetic` | [contracts/libraries/Arithmetic.sol](../contracts/libraries/Arithmetic.sol) |
| <a id="sol-assetregistry"></a>`sol-assetregistry` | Retired reference: `contracts/custody/AssetRegistry.sol` is absent |
| <a id="sol-batchcallandsponsor"></a>`sol-batchcallandsponsor` | [contracts/src/BatchCallAndSponsor.sol](../contracts/src/BatchCallAndSponsor.sol) |
| <a id="sol-blockerrors"></a>`sol-blockerrors` | Retired reference: `contracts/manager/BlockErrors.sol` is absent |
| <a id="sol-blueprint"></a>`sol-blueprint`, <a id="sol-imanagersealattestation"></a>`sol-imanagersealattestation` | Retired reference: `contracts/deployment/Blueprint.sol` is absent |
| <a id="sol-box"></a>`sol-box` | [contracts/src/Box.sol](../contracts/src/Box.sol) |
| <a id="sol-boxv2"></a>`sol-boxv2` | [contracts/src/BoxV2.sol](../contracts/src/BoxV2.sol) |
| <a id="sol-burn"></a>`sol-burn` | Retired reference: `contracts/libraries/Burn.sol` is absent |
| <a id="sol-bytes"></a>`sol-bytes` | Retired reference: `contracts/libraries/Bytes.sol` is absent |
| <a id="sol-callbackpool"></a>`sol-callbackpool`, <a id="sol-callbackrouter"></a>`sol-callbackrouter`, <a id="sol-iswapcallback"></a>`sol-iswapcallback`, <a id="sol-proxytoken"></a>`sol-proxytoken`, <a id="sol-simplev2pool"></a>`sol-simplev2pool`, <a id="sol-tokenimplementation"></a>`sol-tokenimplementation` | [contracts/src/ProxySwapTester.sol](../contracts/src/ProxySwapTester.sol) |
| <a id="sol-callerchecker"></a>`sol-callerchecker` | Retired reference: `contracts/libraries/CallerChecker.sol` is absent |
| <a id="sol-canonicalcheckpoint"></a>`sol-canonicalcheckpoint` | Retired reference: `contracts/libraries/CanonicalCheckpoint.sol` is absent |
| <a id="sol-checkpointchallengemanager"></a>`sol-checkpointchallengemanager` | Retired reference: `contracts/challenge/CheckpointChallengeManager.sol` is absent |
| <a id="sol-constants"></a>`sol-constants` | [contracts/libraries/Constants.sol](../contracts/libraries/Constants.sol) |
| <a id="sol-cryptographyprimitives"></a>`sol-cryptographyprimitives`, <a id="sol-ierc1271minimal"></a>`sol-ierc1271minimal` | Retired reference: `contracts/libraries/CryptographyPrimitives.sol` is absent |
| <a id="sol-cw1155erc1155pointer"></a>`sol-cw1155erc1155pointer` | [contracts/src/CW1155ERC1155Pointer.sol](../contracts/src/CW1155ERC1155Pointer.sol) |
| <a id="sol-cw20erc20pointer"></a>`sol-cw20erc20pointer` | [contracts/src/CW20ERC20Pointer.sol](../contracts/src/CW20ERC20Pointer.sol) |
| <a id="sol-cw721erc721pointer"></a>`sol-cw721erc721pointer` | [contracts/src/CW721ERC721Pointer.sol](../contracts/src/CW721ERC721Pointer.sol) |
| <a id="sol-decimalsconverterhelper"></a>`sol-decimalsconverterhelper` | Retired reference: `contracts/libraries/DecimalsConverterHelper.sol` is absent |
| <a id="sol-delegatecallaware"></a>`sol-delegatecallaware` | Retired reference: `contracts/security/DelegateCallAware.sol` is absent |
| <a id="sol-deploypaxeerxvault"></a>`sol-deploypaxeerxvault` | [contracts/bridge/evm/script/DeployPaxeerXVault.s.sol](../contracts/bridge/evm/script/DeployPaxeerXVault.s.sol) |
| <a id="sol-ed25519verifier"></a>`sol-ed25519verifier` | Retired reference: `contracts/crypto/Ed25519.sol` is absent |
| <a id="sol-emergencyexit"></a>`sol-emergencyexit` | Retired reference: `contracts/EmergencyExit.sol` is absent |
| <a id="sol-encoding"></a>`sol-encoding` | Retired reference: `contracts/libraries/Encoding.sol` is absent |
| <a id="sol-eoa"></a>`sol-eoa` | Retired reference: `contracts/libraries/EOA.sol` is absent |
| <a id="sol-erc1155example"></a>`sol-erc1155example` | [contracts/src/ERC1155.sol](../contracts/src/ERC1155.sol) |
| <a id="sol-erc721"></a>`sol-erc721`, <a id="sol-ierc165"></a>`sol-ierc165`, <a id="sol-ierc721"></a>`sol-ierc721`, <a id="sol-ierc721receiver"></a>`sol-ierc721receiver`, <a id="sol-mynft"></a>`sol-mynft` | [contracts/src/ERC721.sol](../contracts/src/ERC721.sol) |
| <a id="sol-error"></a>`sol-error` | Retired reference: `contracts/libraries/Error.sol` is absent |
| <a id="sol-evmcompatibilitytester"></a>`sol-evmcompatibilitytester` | [contracts/src/EVMCompatibilityTester.sol](../contracts/src/EVMCompatibilityTester.sol) |
| <a id="sol-features"></a>`sol-features` | Retired reference: `contracts/config/Features.sol` is absent |
| <a id="sol-governed"></a>`sol-governed` | [contracts/security/Governed.sol](../contracts/security/Governed.sol) |
| <a id="sol-hashing"></a>`sol-hashing` | Retired reference: `contracts/libraries/Hashing.sol` is absent |
| <a id="sol-iaddr"></a>`sol-iaddr` | [contracts/src/precompiles/IAddr.sol](../contracts/src/precompiles/IAddr.sol) |
| <a id="sol-ibank"></a>`sol-ibank` | [contracts/src/precompiles/IBank.sol](../contracts/src/precompiles/IBank.sol) |
| <a id="sol-iblueprintpredictor"></a>`sol-iblueprintpredictor`, <a id="sol-predeploys"></a>`sol-predeploys` | Retired reference: `contracts/deployment/Predeploys.sol` is absent |
| <a id="sol-ierc20minimal"></a>`sol-ierc20minimal` | Retired reference: `contracts/interfaces/IERC20Minimal.sol` is absent |
| <a id="sol-igenesisbond"></a>`sol-igenesisbond`, <a id="sol-igenesischeckpointregistry"></a>`sol-igenesischeckpointregistry`, <a id="sol-igenesisvault"></a>`sol-igenesisvault`, <a id="sol-managercontainer"></a>`sol-managercontainer` | Retired reference: `contracts/manager/ManagerContainer.sol` is absent |
| <a id="sol-iguarantoreligibility"></a>`sol-iguarantoreligibility` | Retired reference: `contracts/interfaces/IGuarantorEligibility.sol` is absent |
| <a id="sol-ijson"></a>`sol-ijson` | [contracts/src/precompiles/IJson.sol](../contracts/src/precompiles/IJson.sol) |
| <a id="sol-ilayerxanchor"></a>`sol-ilayerxanchor` | [contracts/src/precompiles/ILayerXAnchor.sol](../contracts/src/precompiles/ILayerXAnchor.sol) |
| <a id="sol-ilayerxassetregistry"></a>`sol-ilayerxassetregistry` | Retired reference: `contracts/interfaces/ILayerXAssetRegistry.sol` is absent |
| <a id="sol-ilayerxcomponent"></a>`sol-ilayerxcomponent`, <a id="sol-preinstalls"></a>`sol-preinstalls` | Retired reference: `contracts/deployment/Preinstalls.sol` is absent |
| <a id="sol-ilayerxcustody"></a>`sol-ilayerxcustody` | [contracts/src/precompiles/ILayerXCustody.sol](../contracts/src/precompiles/ILayerXCustody.sol) |
| <a id="sol-ilayerxverify"></a>`sol-ilayerxverify` | [contracts/src/precompiles/ILayerXVerify.sol](../contracts/src/precompiles/ILayerXVerify.sol) |
| <a id="sol-isidioraproxyupgrade"></a>`sol-isidioraproxyupgrade`, <a id="sol-sidioraproxytimelock"></a>`sol-sidioraproxytimelock` | [contracts/governance/SidioraProxyTimelock.sol](../contracts/governance/SidioraProxyTimelock.sol) |
| <a id="sol-iwasmd"></a>`sol-iwasmd` | [contracts/src/precompiles/IWasmd.sol](../contracts/src/precompiles/IWasmd.sol) |
| <a id="sol-layerxbetatimelock"></a>`sol-layerxbetatimelock` | Retired reference: `contracts/governance/LayerXBetaTimelock.sol` is absent |
| <a id="sol-layerxcomponent"></a>`sol-layerxcomponent` | Retired reference: `contracts/security/LayerXComponent.sol` is absent |
| <a id="sol-layerxtimelockcore"></a>`sol-layerxtimelockcore` | Retired reference: `contracts/governance/LayerXTimelock.sol` is absent |
| <a id="sol-managermigrator"></a>`sol-managermigrator` | Retired reference: `contracts/manager/ManagerMigrator.sol` is absent |
| <a id="sol-merklelib"></a>`sol-merklelib` | Retired reference: `contracts/libraries/MerkleLib.sol` is absent |
| <a id="sol-messagetypes"></a>`sol-messagetypes` | Retired reference: `contracts/libraries/MessageTypes.sol` is absent |
| <a id="sol-multisender"></a>`sol-multisender` | [contracts/src/MultiSender.sol](../contracts/src/MultiSender.sol) |
| <a id="sol-nativepaxtokenserc20"></a>`sol-nativepaxtokenserc20` | [contracts/src/NativePaxTokensERC20.sol](../contracts/src/NativePaxTokensERC20.sol) |
| <a id="sol-nativepublicationregistry"></a>`sol-nativepublicationregistry`, <a id="sol-nativepublicationverifier"></a>`sol-nativepublicationverifier` | Retired reference: `contracts/libraries/NativePublicationVerifier.sol` is absent |
| <a id="sol-nativestateproof"></a>`sol-nativestateproof` | Retired reference: `contracts/libraries/NativeStateProof.sol` is absent |
| <a id="sol-paxeerbetadeploymentvalidator"></a>`sol-paxeerbetadeploymentvalidator` | Retired reference: `contracts/deployment/PaxeerBetaDeploymentValidator.sol` is absent |
| <a id="sol-paxeerwithdrawalcodec"></a>`sol-paxeerwithdrawalcodec` | Retired reference: `contracts/libraries/PaxeerWithdrawalCodec.sol` is absent |
| <a id="sol-paxeerxvault"></a>`sol-paxeerxvault` | [contracts/bridge/evm/src/PaxeerXVault.sol](../contracts/bridge/evm/src/PaxeerXVault.sol) |
| <a id="sol-reentrancylock"></a>`sol-reentrancylock` | [contracts/security/ReentrancyLock.sol](../contracts/security/ReentrancyLock.sol) |
| <a id="sol-reservereconciler"></a>`sol-reservereconciler` | Retired reference: `contracts/custody/ReserveReconciler.sol` is absent |
| <a id="sol-safecall"></a>`sol-safecall` | [contracts/libraries/SafeCall.sol](../contracts/libraries/SafeCall.sol) |
| <a id="sol-safetransfer"></a>`sol-safetransfer` | Retired reference: `contracts/libraries/SafeTransfer.sol` is absent |
| <a id="sol-selfdestructfactory"></a>`sol-selfdestructfactory`, <a id="sol-selfdestructtester"></a>`sol-selfdestructtester` | [contracts/src/SelfDestructTester.sol](../contracts/src/SelfDestructTester.sol) |
| <a id="sol-semvercomp"></a>`sol-semvercomp` | Retired reference: `contracts/libraries/SemverComp.sol` is absent |
| <a id="sol-sidioranativeerc20"></a>`sol-sidioranativeerc20` | [contracts/src/SidioraNativeERC20.sol](../contracts/src/SidioraNativeERC20.sol) |
| <a id="sol-simplepair"></a>`sol-simplepair`, <a id="sol-simplerouter"></a>`sol-simplerouter`, <a id="sol-simpletoken"></a>`sol-simpletoken` | [contracts/src/MultiHopSwapTester.sol](../contracts/src/MultiHopSwapTester.sol) |
| <a id="sol-snapshotreverttester"></a>`sol-snapshotreverttester` | [contracts/src/SnapshotRevertTester.sol](../contracts/src/SnapshotRevertTester.sol) |
| <a id="sol-sstoregastest"></a>`sol-sstoregastest` | [contracts/src/SstoreGasTest.sol](../contracts/src/SstoreGasTest.sol) |
| <a id="sol-standardvalidatorutils"></a>`sol-standardvalidatorutils` | Retired reference: `contracts/manager/StandardValidatorUtils.sol` is absent |
| <a id="sol-staticconfig"></a>`sol-staticconfig` | Retired reference: `contracts/config/StaticConfig.sol` is absent |
| <a id="sol-storage"></a>`sol-storage` | Retired reference: `contracts/libraries/Storage.sol` is absent |
| <a id="sol-testtoken"></a>`sol-testtoken` | [contracts/src/TestToken.sol](../contracts/src/TestToken.sol) |
| <a id="sol-transientcontext"></a>`sol-transientcontext` | Retired reference: `contracts/libraries/TransientContext.sol` is absent |
| <a id="sol-transientstoragetester"></a>`sol-transientstoragetester` | [contracts/src/TransientStorageTester.sol](../contracts/src/TransientStorageTester.sol) |
| <a id="sol-types"></a>`sol-types` | [contracts/libraries/Types.sol](../contracts/libraries/Types.sol) |
| <a id="sol-uupsnotupgradeable"></a>`sol-uupsnotupgradeable` | Retired reference: `contracts/security/UUPSNotUpgradeable.sol` is absent |
| <a id="sol-withdrawalclaims"></a>`sol-withdrawalclaims` | Retired reference: `contracts/WithdrawalClaims.sol` is absent |
| <a id="sol-withdrawalnullifierregistry"></a>`sol-withdrawalnullifierregistry` | Retired reference: `contracts/storage/WithdrawalNullifierRegistry.sol` is absent |
| <a id="sol-wpax"></a>`sol-wpax` | [contracts/src/WPAX.sol](../contracts/src/WPAX.sol) |
| <a id="staking-events"></a>`staking-events` | [precompiles/common/evm_events.go](../precompiles/common/evm_events.go); [precompiles/common/legacy/v630/evm_events.go](../precompiles/common/legacy/v630/evm_events.go); [precompiles/common/legacy/v640/evm_events.go](../precompiles/common/legacy/v640/evm_events.go); [precompiles/common/legacy/v65/evm_events.go](../precompiles/common/legacy/v65/evm_events.go); [precompiles/common/legacy/v68/evm_events.go](../precompiles/common/legacy/v68/evm_events.go) |
| <a id="tokenfactory-errors"></a>`tokenfactory-errors` | [modules/tokenfactory/types/errors.go](../modules/tokenfactory/types/errors.go) |
| <a id="tokenfactory-events"></a>`tokenfactory-events` | [modules/tokenfactory/types/msgs.go](../modules/tokenfactory/types/msgs.go) |
