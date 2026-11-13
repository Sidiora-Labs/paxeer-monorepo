# Contracts

This directory contains Solidity sources, external-chain bridge programs, chain integration tests and interface documentation for Paxeer X Network. Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The kernel is part of the node and proposes blocks; a receipt is final when its block is committed. Funding uses native bank balances and derived sub-accounts, while these contracts provide token pointers, wrapped PAX, sponsored account batches, names, governance and application interfaces.

## Layout

| Path | Contents |
| --- | --- |
| [bridge/](bridge) | External-chain EVM custody vault, Solana custody program, deployment configurations and attestation vectors. |
| [config/](config) | Client TOML, checkpoint-settlement JSON and native-state proof fixtures. |
| [governance/](governance) | Sidiora proxy timelock and governance proposal JSON. |
| [launchpad/](launchpad) | Independent Hardhat project for the Sidiora launchpad contracts and Meta-AG/PECOR integration, with tests, scripts and contract documentation. |
| [libraries/](libraries) | Shared arithmetic, constants, safe-call and type definitions. |
| [names/](names) | Registry, registrar, resolver, reverse registrar, availability and deployment contracts. |
| [scripts/](scripts) | Hardhat deployment script for a CW20 token and its ERC-20 pointer. |
| [security/](security) | Governance authority and reentrancy helpers. |
| [src/](src) | Chain token pointers, `WPAX.sol`, `BatchCallAndSponsor.sol`, native precompile interfaces, XWeb consumers and EVM test contracts. |
| [test/](test) | Foundry tests, Hardhat integration tests, staking event tools and proposal fixtures. |
| [wasm/](wasm) | Compiled CosmWasm token and pointer fixtures used by the integration tests. |
| [bootstrap-libs.sh](bootstrap-libs.sh) | Installs the pinned Foundry libraries for the chain contract project. |
| [hardhat.config.js](hardhat.config.js) | Chain contract compiler settings, source path and local test network. |
| [package.json](package.json) | Chain Hardhat dependencies and staking event scripts. |
| [package-lock.json](package-lock.json) | Locked npm dependencies for the chain Hardhat project. |
| [tsconfig.json](tsconfig.json) | TypeScript configuration for the test tools. |
| [.env.example](.env.example) | Environment variable template for the local staking tools. |
| [.gitignore](.gitignore) | Generated contract output and local dependency exclusions. |
| [address-inventory.md](address-inventory.md) | Network address inventory. |
| [agent-api.md](agent-api.md) | Agent API contract. |
| [agent-config.md](agent-config.md) | Agent configuration contract. |
| [agent-formats.md](agent-formats.md) | Agent data formats. |
| [agent-human-peer.md](agent-human-peer.md) | Agent Human peer contract. |
| [agent-rust-api.md](agent-rust-api.md) | Agent Rust API. |
| [bridge-deploy.md](bridge-deploy.md) | Bridge deployment contract. |
| [bridge-vectors.md](bridge-vectors.md) | Bridge attestation vectors. |
| [clients.md](clients.md) | Launchpad client contracts. |
| [consensus-fork.md](consensus-fork.md) | Consensus fork contract. |
| [cosmos-fork.md](cosmos-fork.md) | Cosmos fork contract. |
| [evm-rpc.md](evm-rpc.md) | EVM JSON-RPC contract. |
| [explorer-config.md](explorer-config.md) | Explorer backend configuration. |
| [explorer-frontend-config.md](explorer-frontend-config.md) | Explorer frontend configuration. |
| [explorer-http-api.md](explorer-http-api.md) | Explorer HTTP API. |
| [explorer-metrics.md](explorer-metrics.md) | Explorer metrics. |
| [explorer-services-config.md](explorer-services-config.md) | Explorer Rust service configuration. |
| [explorer-ws.md](explorer-ws.md) | Explorer WebSocket contract. |
| [kernel.md](kernel.md) | Kernel and retained archive formats. |
| [launchpad.md](launchpad.md) | Native launchpad execution and operations contracts. |
| [markets.md](markets.md) | Native market contracts. |
| [operations.md](operations.md) | Tool configuration, Docker environment handling and benchmarks. |
| [perps-client.md](perps-client.md) | Perpetuals client contract. |
| [perps-data.md](perps-data.md) | Perpetuals data service contract. |
| [platform-cli.md](platform-cli.md) | Platform CLI contract. |
| [platform-gateway.md](platform-gateway.md) | Hosted gateway contract. |
| [platform-hosted.md](platform-hosted.md) | Hosted platform services. |
| [platform-middleware.md](platform-middleware.md) | Payment middleware contract. |
| [platform-ramps.md](platform-ramps.md) | Independent ramp contract. |
| [platform-relay-archive.md](platform-relay-archive.md) | Chain history and archive access. |
| [platform-sdk.md](platform-sdk.md) | Platform SDK contract. |
| [programs.md](programs.md) | Programs contract. |
| [proto-api.md](proto-api.md) | Protobuf and REST API contract. |
| [rpc.md](rpc.md) | Node RPC and module query contracts. |
| [sdk-js.md](sdk-js.md) | JavaScript native-module SDK contract. |
| [sequencer.md](sequencer.md) | Node sequencing contract. |
| [services.md](services.md) | Launchpad service contracts. |
| [verifier-http.md](verifier-http.md) | Explorer source verifier transport contract. |

## Build and test

The two Foundry configurations at the repository root cover different source trees. Bridge, launchpad and benchmark contracts have independent build boundaries. Run the root commands below from the repository root; commands with `cd` select a separate project.

### Names and governance

The root [foundry.toml](../foundry.toml) covers `contracts/`, excluding its configured `skip` paths. It pins Solidity `0.8.27`; its `names` and `governance` profiles select the corresponding source directories and tests under [tests/solidity/](../tests/solidity).

```sh
make test-contracts
```

This [Makefile](../Makefile) target runs [scripts/ci/solidity-state-surface.sh](../scripts/ci/solidity-state-surface.sh), then `forge test --offline --root . --no-dynamic-test-linking -vvv` with each profile. Artifacts go to `build/forge-artifacts-names` and `build/forge-artifacts-governance`; caches go to `build/forge-cache-names` and `build/forge-cache-governance`.

### Chain contracts with Foundry

[foundry.paxeer.toml](../foundry.paxeer.toml) selects [src/](src), [test/](test), Solidity `0.8.27` and output under `contracts/out/`. The library bootstrap pins `forge-std` at `v1.9.6` and `openzeppelin-contracts` at `v5.3.0` in `contracts/lib`. It reuses pristine checkouts at those pins and replaces other contents at those library paths.

```sh
bash contracts/bootstrap-libs.sh
FOUNDRY_CONFIG=foundry.paxeer.toml forge build --offline --root .
FOUNDRY_CONFIG=foundry.paxeer.toml forge test --offline --root .
```

The build and test commands are also used by the root `paxeer-manifest-build` and `paxeer-manifest-test` targets. These targets cover other repository projects as well. Chain build output and installed libraries are ignored by Git.

### Hardhat tests against a local chain

The root Makefile provides dependency installation, compiler preparation, compilation and TypeScript checking:

```sh
make paxeer-npm-install
make paxeer-npm-build
make paxeer-npm-static-test
```

These targets also cover the npm integration projects under `tests/integration`. The chain package uses [hardhat.config.js](hardhat.config.js) to compile `src/` with Solidity `0.8.27`, the `prague` EVM target and an optimizer setting of 1,000 runs.

Prepare a disposable local chain using [scripts/initialize_local_chain.sh](../scripts/initialize_local_chain.sh), review its node-home and genesis setup, and leave `paxd` running. Run integration test files with Hardhat directly, for example:

```sh
cd contracts
npm ci
npx hardhat test --network paxlocal test/ERC20toCW20PointerTest.js
```

The `paxlocal` network selects the local EVM endpoint. These tests also use the helpers in [test/lib.js](test/lib.js) for native-chain access and fixture deployment. The staking event listener, transaction trigger and end-to-end event check are exposed by the `listen:staking`, `trigger:staking` and `e2e:staking` scripts in [package.json](package.json).

### Bridge projects

[bridge/evm/foundry.toml](bridge/evm/foundry.toml) pins Solidity `0.8.30`, enables IR compilation and uses 100,000 optimizer runs. Its artifacts and cache are under `contracts/bridge/evm/build/`. The root Makefile provides the library bootstrap and EVM tests, and a separate test target for the Solana program:

```sh
make bridge-test-evm
make bridge-test-solana
```

The Solana admin client build is described in the [bridge guide](bridge/README.md):

```sh
cargo build --locked --release --manifest-path contracts/bridge/solana/Cargo.toml -p paxeer-x-bridge-solana-admin
```

### Launchpad contracts

The separate [launchpad/package.json](launchpad/package.json) declares the package manager and Hardhat compile and test scripts:

```sh
cd contracts/launchpad
pnpm install --frozen-lockfile
pnpm compile
pnpm test
```

See the [launchpad guide](launchpad/README.md) for contract groups and the optional Meta-AG/PECOR integration. The benchmark contract project lives under [tools/benchmark/loadtest/contracts/evm/](../tools/benchmark/loadtest/contracts/evm) with its own Foundry configuration and vendored libraries; see [benchmark contracts](operations.md#bench-contracts).

### Updating pointer contracts

The compiled pointer contracts that `paxd` deploys live under [modules/evm/artifacts/](../modules/evm/artifacts), including `modules/evm/artifacts/cw20/CW20ERC20Pointer.bin`. Regenerate them from `contracts/src` using the commands in [modules/evm/artifacts/README](../modules/evm/artifacts/README), then rebuild `paxd`.

## Configuration

Configuration inputs are specific to each project:

| Reader | Configuration files and environment variable names |
| --- | --- |
| Chain Foundry project | [foundry.paxeer.toml](../foundry.paxeer.toml), selected by `FOUNDRY_CONFIG`. |
| Names and governance Foundry profiles | [foundry.toml](../foundry.toml), selected by `FOUNDRY_PROFILE`. |
| Chain Hardhat project | [hardhat.config.js](hardhat.config.js) loads `contracts/.env` and reads `PAXEER_LOCAL_EVM_RPC_URL`; [tsconfig.json](tsconfig.json) configures the TypeScript tools. |
| Native-chain test helper | [test/lib.js](test/lib.js) reads `EXEC_TIMEOUT_MS`. |
| Staking event tools | [test/run-staking-test.ts](test/run-staking-test.ts) loads `.env`; it and [test/staking-event-trigger.ts](test/staking-event-trigger.ts) read `TEST_ADMIN_PRIVATE_KEY`. The variable template is [.env.example](.env.example). |
| Names deployment | [names/DeployNames.s.sol](names/DeployNames.s.sol) reads `NAMES_GOVERNANCE_TIMELOCK`, `NAMES_EMERGENCY_AUTHORITY`, `NAMES_TREASURY`, `NAMES_PRICE_3`, `NAMES_PRICE_4`, `NAMES_PRICE_5` and `NAMES_GRACE_SECONDS`. |
| EVM bridge deployment | [bridge/evm/foundry.toml](bridge/evm/foundry.toml); [DeployPaxeerXVault.s.sol](bridge/evm/script/DeployPaxeerXVault.s.sol) reads `PAXEER_BRIDGE_VAULT_OWNER`, `PAXEER_BRIDGE_VAULT_ATTESTORS`, `PAXEER_BRIDGE_VAULT_THRESHOLD`, `PAXEER_BRIDGE_VAULT_ASSETS`, `PAXEER_BRIDGE_VAULT_PER_TX_CAPS` and `PAXEER_BRIDGE_VAULT_TOTAL_CAPS`. |
| Bridge chain configuration | Chain `config.json` files under [bridge/evm/chains/](bridge/evm/chains) and [bridge/solana/chains/solana/](bridge/solana/chains/solana); the [bridge guide](bridge/README.md#fill-in-the-configuration) describes the deployment inputs. |
| Launchpad Hardhat project | [launchpad/hardhat.config.js](launchpad/hardhat.config.js) loads `.env` and reads `PRIVATE_KEY`, `REPORT_GAS` and `RUN_DOCGEN`. |
| Launchpad deployment and authority | [Deployment scripts](launchpad/scripts) read `GOVERNANCE_MODE`, `GOVERNANCE_ADMIN_ADDRESS`, `GUARDIAN_ADDRESS`, `INCIDENT_GUARDIAN_ADDRESS`, `ADMIN_ADDRESS`, `ADMIN_WITH_EMITTER_ROLE`, `RELAYER_ADDRESS`, `KEEPER_ADDRESS`, `FEE_COLLECTOR`, `TIMELOCK_DELAY`, `TRANSFER_ADMIN_TO_TIMELOCK`, `NETWORK_TYPE`, `RESUME_DEPLOYMENT` and `FORCE_REDEPLOY`. |
| Launchpad assets and oracles | Scripts read `USDL_ADDRESS`, `SID_ADDRESS`, `WPAX_ADDRESS`, `USDL_USD_ORACLE_ADDRESS`, `ENABLE_SIDIORA_SPOT_REFERENCE`, `SIDIORA_MIN_LIQUIDITY_RAW`, `ORACLE_DEVIATION_BPS`, `VAULT_ADAPTER_FEE_BPS`, `SEED_PRICES`, `INITIAL_MINT` and `EXTRA_RECIPIENTS`. |
| Launchpad manifests and upgrades | Scripts read `LAUNCHPAD_MANIFEST`, `LAUNCHPAD_PARAMS_JSON`, `LAUNCHPAD_AUTHORITY`, `UPGRADE_PLAN`, `CONTRACT`, `BEACON`, `PROXY_ADDRESS`, `IMPL_ADDRESS`, `TIMELOCK_ADDRESS`, `ROUTER_ADDRESS`, `NEXT_PUBLIC_ROUTER_ADDRESS`, `BUFFER_SECONDS` and `ALLOW_EARLY`. |
| Launchpad transaction and liquidity tools | Scripts read `RPC_URL`, `GAS_PRICE`, `FORCE_LEGACY_TX`, `PROVIDER_POLL_MS`, `TX_POLL_MS`, `TX_WAIT_TIMEOUT_MS`, `DRY_RUN`, `TOP_UP`, `ONLY`, `SKIP`, `OUTPUT_FILE`, `TEMPLATE_MODE`, `VERIFY_DRY_RUN`, `SKIP_VERIFY`, `SKIP_FACTORY`, `SKIP_OPTICAL` and `SKIP_ROUTER`; liquidity amounts use the symbol-specific `SEED_` and `SEED_RAW_` prefixes. |
| Launchpad test tools | Tests and coverage scripts read `LAUNCHPAD_PROOF_NODE_TEST`, `PECOR_COVERAGE_OUTPUT` and `UPDATE_GAS_SNAPSHOTS`. |

The launchpad scripts read deployment address and operation JSON files under [launchpad/deployments/](launchpad/deployments), the manifest selected by `LAUNCHPAD_MANIFEST`, parameter JSON selected by `LAUNCHPAD_PARAMS_JSON`, and the upgrade plan selected by `UPGRADE_PLAN`. See [launchpad operations](launchpad/operations/README.md) for the input formats.

## Documentation

Funding uses the [native bank interface](../precompiles/bank/Bank.sol) and [derived account mapping](../modules/lxphost/accountmap.go). Outbound bridged assets use `bridgeOut` in the [bridge precompile interface](../precompiles/layerxbridge/LayerXBridge.sol), subject to registered chains, attestors, caps and pause state. The [external bridge custody guide](bridge/README.md) describes its deployment and registration requirements.

- [Monorepo layout and build boundaries](../docs/MONOREPO.md)
- [Network overview](../docs/paxeer-network.md)
- [Bridge guide](../docs/wiki/Bridge.md)
- [Governance guide](../docs/system/governance.md)
- [Launchpad chain execution](../docs/launchpad/chain-launch.md)
- [Launchpad production checklist](../docs/launchpad/production-checklist.md)
- [Contracts documentation site page](../docs/site/paxeer-docs/src/app/contracts/page.tsx) and [Markdown page](../docs/site/paxeer-docs/public/markdown/contracts.md)
- [Foundry documentation site guide](../docs/site/paxeer-docs/public/markdown/foundry.md)
- [Hardhat documentation site guide](../docs/site/paxeer-docs/public/markdown/hardhat.md)
- [Public RPC documentation site reference](../docs/site/paxeer-docs/public/markdown/public-rpc.md)
- [Operations tools and benchmarks](operations.md)
