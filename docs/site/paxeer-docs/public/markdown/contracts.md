<!-- Source: https://docs.paxeer.app/contracts/ -->

# Contracts

Build Solidity applications on Paxeer X and integrate wrapped PAX, token pointers, and native precompile interfaces.

## Custody and settlement

Paxeer mainnet-beta executes and settles transactions on one BFT chain. Funding uses native bank balances and derived sub-accounts. The `LayerXCustody.sol` ABI is in `precompiles/layerxcustody/LayerXCustody.sol`; `contracts/src/precompiles/ILayerXCustody.sol` is a Solidity interface. Custody calls at `0x1013` are disabled on the merged network. The checkpoint anchor at `0x1014` is also disabled once its store-delete upgrade is active.

Choose the interface for the native module your application uses and verify its authorization requirements. A deployed Solidity contract and a native precompile have distinct call targets and state. A receipt is final when its block is committed; see the [settlement guide](https://docs.paxeer.app/settlement) for related evidence.

### Native application composition

Custody at `0x1013` and anchor at `0x1014` do not provide an application settlement path on the merged network. Native application interfaces include perpetuals at `0x1015`, bridge at `0x1016`, and launchpad at `0x1017`. The verification precompile at `0x1012` is disabled on the merged network. Fee-token selection uses `0x1018`, and XWeb requests use `0x1019`. Import the exact ABI or Solidity interface for each call and preserve integer units, authorization requirements, and event decoding.

Combine [native denominations and token pointers](https://docs.paxeer.app/assets) with [bridged assets](https://docs.paxeer.app/bridge), or use the [XDK](https://docs.paxeer.app/xdk) to prepare calls and consume application events.

### Contract project boundaries

Chain contracts under `contracts/src/` use `foundry.paxeer.toml` (sources `contracts/src`, tests `contracts/test`, artifacts `contracts/out`). The repository-root `foundry.toml` is a separate project (sources `contracts`, artifacts `build/forge-artifacts`) that explicitly skips `contracts/src/**` and `contracts/test/**`; its `tests/solidity/` suite covers governance and names. Select the configuration matching the contract family when compiling or testing.

## Start a New Application

Follow the [Foundry](https://docs.paxeer.app/foundry) or [Hardhat](https://docs.paxeer.app/hardhat) walkthrough to compile, test, and deploy a contract on chain ID 125 (`hyperpax_125-1`). Continue with [Build a Frontend](https://docs.paxeer.app/building-a-frontend) to connect a browser wallet and read deployed state.

The Paxeer X contract library provides Solidity interfaces and implementations for native token operations and Cosmos integration. Its sources live in `contracts/src/`.

## Wrapped PAX and Token Pointers

| Contract | Use |
| --- | --- |
| `WPAX.sol` | Wrap native PAX in an ERC-20-style interface with 18 decimals |
| `NativePaxTokensERC20.sol` | Read and transfer a native Cosmos denomination through an ERC-20 interface |
| `CW20ERC20Pointer.sol` | Interact with a CW20 token through the ERC-20 interface |
| `CW721ERC721Pointer.sol` | Interact with a CW721 collection through the ERC-721 interface |
| `CW1155ERC1155Pointer.sol` | Interact with CW1155 multi-tokens through the ERC-1155 interface |

### Wrap and Unwrap PAX

`WPAX.deposit()` accepts native PAX through `msg.value` and credits the caller’s WPAX balance. `withdraw(amount)` reduces that balance and returns native PAX. The contract also exposes transfers, allowances, and approval events for token integrations.

Use the WPAX deployment address published for your target network. Read its `symbol()`, `decimals()`, and deployed bytecode before configuring an application integration. Keep native PAX available to pay transaction fees.

### Use a Token Pointer

A pointer exposes an underlying Cosmos asset through familiar EVM token methods. The native-token implementation delegates balance, supply, and transfer operations to the bank precompile. CW pointers call the corresponding CosmWasm contract.

Resolve the registered pointer and its version for the underlying denomination or contract. Use the [pointer and pointer-view precompiles](https://docs.paxeer.app/precompiles) for registration and discovery, then read the token’s metadata through its own ABI.

## Native Precompile Interfaces

Import the Solidity interface matching the operation you need. These native precompiles use fixed addresses registered by Paxeer X; the interfaces are in `contracts/src/precompiles/`.

| Interface | Address | Operations |
| --- | --- | --- |
| `IBank.sol` | `0x0000000000000000000000000000000000001001` | Native-token balances, supply, metadata, and transfers |
| `IWasmd.sol` | `0x0000000000000000000000000000000000001002` | Instantiate, execute, and query CosmWasm contracts |
| `IJson.sol` | `0x0000000000000000000000000000000000001003` | Extract bytes, byte arrays, and integers from JSON |
| `IAddr.sol` | `0x0000000000000000000000000000000000001004` | Look up associated EVM and Cosmos addresses |

For example, read the Cosmos address associated with an EVM account:

```
cast call 0x0000000000000000000000000000000000001004 \
  'getPaxAddr(address)(string)' "$ACCOUNT_ADDRESS" \
  --rpc-url "$PAXEER_RPC_URL"
```

Set `ACCOUNT_ADDRESS` to an associated account. If the call reports a missing association, follow [Address Conversion](https://docs.paxeer.app/precompiles) to associate the account first. The reverse lookup is `getEvmAddr(string)`.

## Work with the Contract Library

The repository includes Foundry and Hardhat tooling. Use the configuration for the artifact you are rebuilding and retain its compiler settings alongside the output.

| Tool | Configuration | Project layout |
| --- | --- | --- |
| Foundry | `foundry.paxeer.toml` | Sources in `contracts/src/`, tests in `contracts/test/`, artifacts in `contracts/out/` |
| Hardhat | `contracts/hardhat.config.js` | Hardhat 2 project with its dependencies and lockfile under `contracts/` |

The Hardhat `paxlocal` network reads `PAXEER_LOCAL_EVM_RPC_URL` and defaults to a loopback HTTP listener on port `8545`. Start a [Paxeer development cluster](https://docs.paxeer.app/docker) for integration tests that call native precompiles or CosmWasm modules.

Use the [Foundry](https://docs.paxeer.app/foundry) and [Hardhat](https://docs.paxeer.app/hardhat) guides for a complete new-project setup, including compiler configuration, local tests, and deployment commands.

### Choose the Relevant Tests

- `CW20toERC20PointerTest.js` and `NativePaxTokensERC20Test.t.sol` cover token-pointer behavior.

- `EVMCompatibilityTester.sol`, `TransientStorageTester.sol`, and `SelfDestructTester.sol` exercise EVM behavior.

- `SstoreGasTest.sol` and `SnapshotRevertTester.sol` exercise storage gas and state restoration.

- `MultiSender.sol` and `BatchCallAndSponsor.sol` support batch-call scenarios.

## Maintain Embedded Pointer Artifacts

Chain-level pointer implementations include versioned ABI and bytecode artifacts under `modules/evm/artifacts/`. Go embeds these files into the node binary. When changing a pointer implementation:

1. Compile the contract using the intended compiler and optimizer settings, and run its contract tests.
2. Update the matching ABI and creation bytecode in the `native`, `cw20`, `cw721`, `cw1155`, `erc20`, `erc721`, `erc1155`, or `wpax` artifact package.
3. Update the pointer version and migration logic required by the release. Registry lookups select pointers by type, underlying asset, and version.
4. Rebuild `paxd` to embed the artifacts and exercise registration, transfers, and existing-pointer behavior in a development cluster.
5. Release the binary through the network’s upgrade process, including the migration for any existing deployed pointers.

Application developers can use the registered pointer interface directly. Embedded-artifact updates are part of maintaining the chain implementation.

## Connect a Deployed Contract

Obtain the address from your deployment output or the contract maintainer’s network-specific deployment record. Match your network's chain ID (125 on `hyperpax_125-1`), the contract ABI, and the deployed version before adding the address to your application.

```
cast chain-id --rpc-url "$PAXEER_RPC_URL"
cast receipt "$TX_HASH" --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
```

Retain the creation transaction, source revision, compiler settings, constructor arguments, and ABI. For a proxy contract, also retain the implementation address and upgrade configuration. Continue with [Query the EVM](https://docs.paxeer.app/querying-evm) to read state and events.

**Source:** `contracts/src/`, `contracts/hardhat.config.js`, `foundry.paxeer.toml`, `precompiles/`, and `modules/evm/artifacts/` with `modules/evm/keeper/pointer.go`.
