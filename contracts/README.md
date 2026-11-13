# Contracts

This directory contains Solidity sources, chain integration tests and interface documentation for Paxeer X Network. Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The kernel is part of the node and proposes blocks; a receipt is final when its block is committed.

See [operations](operations.md) for tool configuration, Docker environment handling and benchmarks, and [public RPC endpoints](../docs/site/docs/reference/public-rpc.md) for node access.

## Project boundaries

The two Foundry configurations at the repository root cover different source trees. Bridge, launchpad and benchmark contracts have independent build boundaries.

| Project | Sources | Tests | Configuration |
| --- | --- | --- | --- |
| Root Solidity project (including names, governance, libraries and security helpers) | `contracts/`, excluding the paths in the configuration's `skip` list | [`tests/solidity`](../tests/solidity), selected by the Make target | [`foundry.toml`](../foundry.toml) |
| Paxeer X chain contracts (pointer contracts, `WPAX.sol`, precompile interfaces under `src/precompiles`, `src/xweb`, test helpers) | [`src/`](src) | [`test/`](test) | [`foundry.paxeer.toml`](../foundry.paxeer.toml) |
| External-chain bridge custody contracts | [`bridge/evm/src`](bridge/evm/src) | [`bridge/evm/test`](bridge/evm/test) | [`bridge/evm/foundry.toml`](bridge/evm/foundry.toml); [bridge guide](bridge/README.md) |
| Launchpad application contracts | [`launchpad/contracts`](launchpad/contracts) | See the [launchpad guide](launchpad/README.md) | [`launchpad/`](launchpad) |
| Benchmark workload contracts | [`tools/benchmark/loadtest/contracts/evm/src`](../tools/benchmark/loadtest/contracts/evm/src) | Owned by the benchmark project | [Benchmark Foundry configuration](../tools/benchmark/loadtest/contracts/evm/foundry.toml) |

The root and Paxeer X chain projects pin Solidity `0.8.27`. The bridge EVM project pins `0.8.30`, enables IR compilation and uses 100,000 optimizer runs; its artifacts and cache are under `contracts/bridge/evm/build/`.

## Kernel contracts

The merged network uses the native bank and committed block state for funding and finality.

Current funding uses native bank balances and derived sub-accounts; see the [bank interface](../precompiles/bank/Bank.sol) and [account mapping](../modules/lxphost/accountmap.go). Block commitment supplies network finality. For bridged assets, outbound movement uses `bridgeOut` in the [bridge precompile interface](../precompiles/layerxbridge/LayerXBridge.sol), subject to registered chains, attestors, caps and pause state. The [external bridge custody boundary](bridge/README.md) has its own deployment and registration requirements.

The remaining root Solidity suite can be run from the repository root:

```bash
make test-contracts
```

The [Make target](../Makefile) runs `scripts/ci/solidity-state-surface.sh`, then runs `forge test --offline --root . --no-dynamic-test-linking -vvv` once with `FOUNDRY_PROFILE=names` and once with `FOUNDRY_PROFILE=governance`. These profiles select `contracts/names` and `contracts/governance` with their corresponding tests under `tests/solidity`. Their output directories are `build/forge-artifacts-names` and `build/forge-artifacts-governance`; their caches are `build/forge-cache-names` and `build/forge-cache-governance`. This suite is separate from the chain integration tests below.

## Paxeer X chain contracts

The chain project's Foundry libraries are not committed. [`bootstrap-libs.sh`](bootstrap-libs.sh) fetches `forge-std` at `v1.9.6` and `openzeppelin-contracts` at `v5.3.0` into `contracts/lib`. It reuses pristine checkouts at those tags and replaces other contents at those library paths. From the repository root:

```bash
bash contracts/bootstrap-libs.sh
FOUNDRY_CONFIG=foundry.paxeer.toml forge build --offline --root .
FOUNDRY_CONFIG=foundry.paxeer.toml forge test --offline --root .
```

Use Solidity `0.8.27`, as pinned by the configuration. Build output goes to `contracts/out/`, which is ignored by Git. This bootstrap does not populate bridge or benchmark libraries. The benchmark uses its own vendored OpenZeppelin and Solmate trees; see [benchmark contracts](operations.md#bench-contracts).

### Hardhat tests against a local chain

1. Prepare a disposable local chain using [`scripts/initialize_local_chain.sh`](../scripts/initialize_local_chain.sh), then leave `paxd` running. Review the script's node-home and genesis setup before running it.
2. Install the npm dependencies and run a test file:

```bash
cd contracts
npm ci
npx hardhat test --network paxlocal test/ERC20toCW20PointerTest.js
```

The `paxlocal` network in [`hardhat.config.js`](hardhat.config.js) reads `PAXEER_LOCAL_EVM_RPC_URL`, defaulting to `http://127.0.0.1:8545`. Hardhat reads `contracts/.env`, compiles `contracts/src` with Solidity `0.8.27`, targets `prague` and enables the optimizer with 1,000 runs. These integration tests require the local chain and the test helpers' native-chain access; the public mainnet endpoint is not a substitute for the local fixture. Invoke Hardhat directly: `npm test` is a placeholder command that exits with an error in this package.

### Updating pointer contracts

The compiled pointer contracts that `paxd` deploys live under [`modules/evm/artifacts/`](../modules/evm/artifacts) (for example `modules/evm/artifacts/cw20/CW20ERC20Pointer.bin`). Regenerate them from `contracts/src` with the steps in [`modules/evm/artifacts/README`](../modules/evm/artifacts/README), then rebuild `paxd`.
