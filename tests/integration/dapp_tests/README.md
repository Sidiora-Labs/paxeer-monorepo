# dApp Tests

These integration tests deploy contracts and exercise EVM, CosmWasm and pointer
interactions on `paxd`, including associated and unassociated accounts. They
fund accounts and submit transactions, so use a disposable test chain. See the
[integration overview](../README.md) for the merged network and public endpoint
conventions; these scripts use local and named test-network settings.

## Setup

Run the script from the repository root:

```bash
./tests/integration/dapp_tests/dapp_tests.sh <paxlocal|devnet|testnet> [uniswap|steak|nft]
```

The script runs `npm ci` in `contracts/` and this directory, compiles this
directory's contracts with Hardhat, exports `DAPP_TEST_ENV` from the first
argument, then runs all three suites or the selected suite. It relies on
[`contracts/test/lib.js`](../../../contracts/test/lib.js), a working `paxd` CLI
and keyring, and the Wasm artifacts referenced by the scenarios.

### Hardhat provider and accounts

[`hardhat.config.js`](hardhat.config.js) defines:

| Network label | Hardhat EVM provider | Accounts |
| ------------- | ------------------- | -------- |
| `paxlocal` | `PAXEER_LOCAL_EVM_RPC_URL`, default `http://127.0.0.1:8545` | Uses the local fixture account configured in Hardhat; `DAPP_TESTS_MNEMONIC` does not override it. |
| `devnet` | `PAXEER_DEVNET_EVM_RPC_URL` | Network is defined only when both URL and `DAPP_TESTS_MNEMONIC` are set. |
| `testnet` | `PAXEER_TESTNET_EVM_RPC_URL` | Network is defined only when both URL and `DAPP_TESTS_MNEMONIC` are set. |

Set `DAPP_TESTS_MNEMONIC` when selecting `devnet` or `testnet`. Uniswap and NFT
setup read `accounts.mnemonic` and `accounts.path` directly from the selected
Hardhat network. The configured HD path is `m/44'/118'/0'/0/0`. On `paxlocal`,
Steak creates its owner through its account helper instead.

On `paxlocal`, prepare the chain with
[`scripts/initialize_local_chain.sh`](../../../scripts/initialize_local_chain.sh)
and a funded `admin` key available to the helpers. Uniswap and NFT fund their
deployer locally. For remote labels, pre-fund the mnemonic's deployer and make
the native CLI configuration consistent with the EVM provider.

### Native and pointer operations use separate settings

The Hardhat URL variables above do **not** configure every operation.
[`constants.js`](constants.js) contains independent `rpcUrls`, `evmRpcUrls`
and `chainIds` maps used by all three suites:

- For `devnet` and `testnet`, setup writes `rpcUrls[DAPP_TEST_ENV]` and
  `chainIds[DAPP_TEST_ENV]` into `paxd config node` and `paxd config chain-id`.
  The stored chain IDs are `arctic-1` and `atlantic-2`,
  respectively. These native chain-ID strings do not select merged mainnet
  and are distinct from its EVM chain ID 125.
- Pointer helpers receive `evmRpcUrls[DAPP_TEST_ENV]` and append it as
  `--evm-rpc` to native CLI commands. These values are hard-coded
  test-network URLs and do not read `PAXEER_DEVNET_EVM_RPC_URL` or
  `PAXEER_TESTNET_EVM_RPC_URL`.
- The maps have no `paxlocal` entries. Local setup leaves the CLI node and
  chain ID as configured; the absent pointer URL causes the helper to omit
  `--evm-rpc` and use the CLI default. Changing only
  `PAXEER_LOCAL_EVM_RPC_URL` does not redirect that default.

Check all three paths (Hardhat provider, native CLI node/chain ID, pointer CLI
EVM URL) before running. The current remote labels are not a fully
environment-configurable route to merged mainnet. The suites also change CLI
keyring configuration; use a dedicated test CLI home.

## Tests

### Uniswap (EVM DEX)

[`uniswap/uniswapTest.js`](uniswap/uniswapTest.js) deploys Uniswap V3 contracts
and tests swaps and pool creation:

- Associated accounts swap ERC20, ERC20–tokenfactory and ERC20–CW20 pairs.
- Unassociated accounts receive ERC20 and ERC20–tokenfactory tokens.
- Unassociated accounts are expected to fail to receive ERC20–CW20 tokens.
- Unassociated accounts deploy pools and supply liquidity.

### Steak (CW Liquid Staking)

[`steak/SteakTests.js`](steak/SteakTests.js) deploys CosmWasm liquid staking
contracts and checks that associated accounts can bond and unbond, and that
unassociated accounts can bond.

### NFT Marketplace (EVM NFT Marketplace)

[`nftMarketplace/nftMarketplaceTests.js`](nftMarketplace/nftMarketplaceTests.js)
deploys a marketplace and checks:

- Associated and unassociated accounts list and buy ERC721 tokens.
- Associated accounts list and buy CW721 tokens through ERC721 pointers.
- Unassociated accounts are expected to fail to list or buy those CW721 pointers.

These are the scenarios asserted by this suite, not an exhaustive compatibility
claim. For RPC behavior, use the [fixture suite](../evm_module/rpc_io_test/RPC_IO_README.md)
or the [geth comparison suite](../rpc_tests/README.md).
