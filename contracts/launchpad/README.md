# KindleLaunch contracts

KindleLaunch is the USDL-paired launchpad AMM implemented by the `Sidiora*` Solidity contracts in
this directory. It holds the core contracts, optical hooks, optional Meta-AG/PECOR integration,
Hardhat tooling and deployment records for Solidity execution on the Paxeer network. The network's
active module and precompile are described in
[native launchpad execution](../launchpad.md#native-execution). The deployment scripts here deploy
the Solidity contracts in this directory.

## Layout

- [contracts/](contracts/) — Solidity core, protocol, data, periphery, optical, Meta-AG, library,
  base, interface and test contracts.
- [deployments/](deployments/) — network deployment manifests, token inventories, transaction
  journals and explorer verification records.
- [docs/](docs/) — Solidity API references grouped by source directory.
- [operations/](operations/) — deployment, governance, oracle, incident response and recovery
  runbook.
- [scripts/](scripts/) — deployment, verification, upgrade, health, integration generation and
  release scripts.
- [storage-layout/](storage-layout/) — core, Meta-AG and production storage-layout baselines.
- [test/](test/) — Hardhat contract, integration, remediation and gas tests.
- [hardhat.config.js](hardhat.config.js) — compiler, network, gas reporter, explorer and
  documentation settings.
- [package.json](package.json) — package scripts, dependencies and runtime requirements.
- [pnpm-lock.yaml](pnpm-lock.yaml) — locked package dependencies.
- [pnpm-workspace.yaml](pnpm-workspace.yaml) — dependency build permissions for this pnpm project.
- [requirements-audit.txt](requirements-audit.txt) — pinned Slither dependency.
- [slither.config.json](slither.config.json) — static analysis path and dependency filters.
- [.solhint.json](.solhint.json) — Solidity lint configuration.
- [.eslintrc.js](.eslintrc.js) — JavaScript lint configuration.
- [.prettierrc](.prettierrc) — source and Markdown formatting settings.
- [.editorconfig](.editorconfig) — editor whitespace settings.
- [.gitignore](.gitignore) — generated output and local-file exclusions.

### Network

The LayerX kernel inside `paxd` executes and orders each transaction and builds each block;
validators check and commit it. EVM chain ID is **125** (`0x7d`). A transaction receipt is final
when its block is committed.

Node JSON-RPC uses the configured [root endpoint](../evm-rpc.md#http-endpoint). The
[Hardhat configuration](hardhat.config.js) selects the endpoint for `paxeer-x-network`. `Router` and
`MetaAGRouter` below are on-chain trading contracts.

### Responsibilities

A launchpad needs market creation, pricing, fee accounting and controls that operators and
integrators can inspect. These contracts separate those responsibilities, but their safety still
depends on deployed implementations, token behavior, attached hooks and the actual authority
configuration. Local tests establish behavior of the checked-out code; they do not certify an
existing deployment.

### Design

The design combines a core/periphery split, optical lifecycle hooks, separate fee accounting and
virtual reserves. Its actual entrypoints and types are defined under [contracts/](contracts/);
contract names, event signatures and deployment records should be taken from that source and the
matching compiled artifacts.

### Pool instances

The core AMM uses a separate beacon proxy for each USDL/token pool. Each pool has its own reserves
and state, while pools sharing a beacon share its upgrade authority and implementation. A shared
implementation defect can affect every pool using it.

USDL pairing does not exclude multi-token trades. The core [Router](contracts/periphery/Router.sol)
implements `swapTokenForToken` as token → USDL → token through two registered pools. The optional
[MetaAGRouter](contracts/meta-ag/router/MetaAGRouter.sol) supports adapter selection and
`swapMultiHop` across connected token paths, re-quoting each hop with the actual intermediate
amount. The optional PECOR layer also has its own custody vault; the per-pool statement applies to
the core AMM.

### The Virtual Reserve Model

[ProtocolConfig](contracts/protocol/ProtocolConfig.sol) initializes the virtual USDL default to
10,000 USDL and the token supply default to one billion six-decimal tokens. Each newly created pool
starts with its configured virtual USDL reserve and zero real USDL. Existing pools retain their
initialized virtual reserve when defaults change.

```text
Effective USDL = virtualUsdlReserve + realUsdlBalance
BUY:  amountOut = tokenReserve × amountInAfterFee / (effectiveUsdl + amountInAfterFee)
SELL: amountOut = effectiveUsdl × amountInAfterFee / (tokenReserve + amountInAfterFee)
```

These are the integer constant-product calculations in
[ReserveLib](contracts/libraries/ReserveLib.sol). The fee is taken from the input asset. Buy fees
are USDL sent to FeeAccumulator; sell fees remain in the pool as tokens. A sell whose output exceeds
real USDL reverts with `VirtualFloorBreached()`. Virtual reserves provide pricing depth, not
redeemable USDL backing.

### Contract groups

The source groups protocol configuration, data, pool logic, optical extensions, periphery and shared
libraries. The sections below describe responsibilities; calls also cross these groups for registry
lookup, configuration, hooks and event emission.

#### Protocol configuration

**ProtocolConfig** stores virtual defaults, fee settings and the creation fee. Fresh deployments
charge **1 USDL per pool creation** (1,000,000 base units), excluding network gas. Direct factory
creation and periphery creation collect the fee once and credit Treasury. Core deployment requires
six-decimal USDL; `ProtocolConfig.initialize` itself does not check decimals, so custom deployment
tooling must do so.

Upgrading an existing deployment does not rerun initialization or reset its fee. A fee change
requires `ProtocolConfig.setCreationFee(1000000)` through its configured governance and Timelock
process. The creation fee is denominated in USDL and does not use an oracle to maintain a dollar
value.

The [creation-fee regressions](test/core/creation-fee.test.js),
[ProtocolConfig tests](test/protocol/ProtocolConfig.test.js) and
[deployment tests](test/remediation/Deployment.test.js) cover local charging, configuration, decimal
validation and recovery behavior. They do not establish a live deployment’s current fee.

**Treasury** accounts for protocol deposits and governed withdrawals. Authorized producers record
deposits; administration and withdrawal authority must be checked on the deployed contract.

**Timelock** is deployed without a proxy and enforces delayed governance operations.
`TIMELOCK_DELAY` defaults to 172800 seconds (48 hours); deployment accepts 3600 through 2592000
seconds. The configured guardian can cancel queued operations.

**GovernanceModule** defaults to `GOVERNANCE_MODE=admin`. Deployment initializes administrator-only
governance without a voting token, then retains `ADMIN_ROLE` for `GOVERNANCE_ADMIN_ADDRESS`, or the
deployment wallet if unset. A multisig is an explicit operator choice. The administrator queues
changes using `adminExecute`; execution still waits for Timelock. Timelock holds proxy
`DEFAULT_ADMIN_ROLE` and proposer authority is handed to GovernanceModule. Administrator-only mode
rejects token proposals and mode deactivation.

`GOVERNANCE_MODE=token` explicitly selects checkpoint voting with `SID_ADDRESS`. That deployment
path disables bootstrap administrator decisions at handoff. It is not an automatic future transition
of an administrator-only deployment. See
[mainnet deployment](operations/README.md#launchpad-mainnet-deployment).

#### Data and fee accounting

**EventEmitter** provides central typed and generic event channels. It is not a complete substitute
for source-contract logs: some callers emit mirrors on a best-effort basis, while other configured
hub calls propagate failure. Indexers must follow the source contracts and deployed wiring as well
as the hub. Core pool events include `poolId`; configuration, transfer, governance and other events
use their own identifiers. For example, `TokenTransfer` has a token address and no `poolId`. Emitted
activity events include block timestamp and number; the administrative `TypedEmitterPermission`
event does not. See the [IEventEmitter reference](docs/interfaces/IEventEmitter.md) for payloads,
indexed fields and permissions.

**PoolRegistry** maps tokens to pools and stores creator, optical and NFT metadata. Factory
registration requires `FACTORY_ROLE`; periphery contracts query it for discovery and validation.

**FeeAccumulator** tracks USDL pool fees, protocol revenue and distribution liabilities. It applies
the selected fee-distribution path and optical fee hooks. Sell-side token fees remain in the pool
instead of becoming USDL fee claims.

#### Core AMM

**SidioraFactory** creates the token with CREATE2 and a pool beacon proxy, authorizes fee recording,
mints the fee-rights NFT, registers metadata, binds the token to the pool and transfers the initial
supply to the pool. It emits creation information locally and through its configured EventEmitter.
These operations occur in one transaction.

**SidioraPool** implements swaps, reserve accounting, dynamic fees and optical swap hooks. It bounds
sells by real USDL, checks prepaid input and output slippage, and checks reserve backing after
callbacks. Its guardian initially is the creator; guardian transfer requires acceptance and is
independent of NFT ownership.

**SidioraERC20** uses the in-house ERC20 base with permit and historical balance/supply checkpoints.
Supply is minted once in the constructor. Its code is not upgradeable, but a bound pool’s optical
can impose transfer policy through `beforeTokenTransfer`; immutable code does not imply unrestricted
transfers.

**SidioraNFT** represents pool fee rights and the selected strategy. Transferring the NFT transfers
control of those rights, including fees still accumulated for that pool. Ownership does not confer
pool guardian authority. The NFT supports strategy changes by approved callers or its
strategy-setter role, with additional owner checks at FeesRouter entrypoints.

#### Optical hooks

The [IOptical interface](contracts/opticals/interfaces/IOptical.sol) exposes `beforeSwap`,
`afterSwap`, `beforeFeeDistribution` and `afterFeeDistribution`, selected by hook flags. The current
token implementation also supports flag bit 4 for `beforeTokenTransfer`.

Core deployment installs five preset contracts:

| Optical                | Behavior                                                                                   |
| ---------------------- | ------------------------------------------------------------------------------------------ |
| **AntiSnipeOptical**   | Limits early buys using a supply percentage and block window                               |
| **MaxWalletOptical**   | Enforces wallet limits through the token transfer hook, with owner-managed exemptions      |
| **BuybackBurnOptical** | Accumulates a fee portion for an explicit slippage-bounded buyback and token burn transfer |
| **TaxOptical**         | Reduces effective swap input; the taxed portion remains in the pool                        |
| **CooldownOptical**    | Enforces a per-wallet interval between swaps                                               |

[LaunchpadOptical](contracts/opticals/presets/LaunchpadOptical.sol) additionally implements
pool-specific launch, vesting and capital-raise behavior; it is not one of those five shared
presets. Presets are deployed without proxies, but some retain mutable owner-controlled state.
Review both bytecode and configuration.

OpticalRegistry provides optional approval signaling tied to observed runtime identity and flags. An
approval can become invalid and must be renewed after review. Deregistration does not detach a
pool’s hooks. See [optical review identity](operations/README.md#optical-review-identity).

#### Periphery

**Router** provides market creation, buy, sell and two-pool token swaps, including permit variants
and multicall. It validates registry membership and transfers input before invoking pools. Direct
factory creation is also available; the periphery is not the only user entrypoint.

**Quoter** provides read-only quotes, fee breakdowns, price impact, pool statistics and discovery.
Off-chain `eth_call` reads do not submit a transaction or pay network gas; on-chain callers still
consume gas. Quotes reflect current state and can change before execution.

**FeesRouter** checks NFT ownership and selected strategy for fee management. Holder airdrop claims
use a separate path without NFT ownership requirements.

#### Libraries and bases

The core Solidity primitives are implemented in this tree: SidioraMath, FeeLib, ReserveLib,
TransferHelper and BitFlag, with in-house ERC20/ERC721, access control, reentrancy, pause and proxy
bases. Build and deployment tooling has package dependencies listed in [package.json](package.json).
In-house implementations still require review and qualification.

### Upgrade authority

Core singleton contracts use UUPS proxies; upgrade authorization requires `DEFAULT_ADMIN_ROLE`.
Finalized deployment assigns that role to Timelock. Pool instances share PoolBeacon, whose ownership
must also be accepted by Timelock through a delayed transaction. A beacon implementation change
affects every attached pool.

User tokens have no implementation upgrade path. Timelock and the deployed optical presets have no
proxy upgrade path. Existing deployment authority must be inspected: temporary deployer privileges
and pending beacon acceptance are real powers until handoff completes. Timelock delays provide
review time; they do not guarantee a safe proposal or a multisig decision maker.

### Fee strategies

Fee strategy values in the NFT and FeesRouter are:

- **CLAIM (0)**: the NFT owner withdraws accumulated USDL fees on demand.
- **BURN (1)**: accumulated USDL fees are transferred to the dead address. This does not burn the
  pool token.
- **AIRDROP (2)**: `executeAirdrop` creates an epoch using balances and total supply from the
  previous block. Pool and dead-address holdings are excluded from eligible supply. Holders claim
  USDL individually through `claimAirdrop` or `claimAirdropEpoch`; there is no holder-iteration
  payout. Claims close after 180 days, and `sweepExpiredAirdrop` returns the remainder to the pool’s
  accumulated fee balance. Tokens without supported historical checkpoints cannot receive new
  snapshot distributions; legacy epochs require explicit migration.
- **LP_REWARDS (3)**: USDL fees move to the pool, then FeesRouter calls `syncReserves()` to include
  them in real liquidity.

Snapshot allocations are fixed for their epoch. Transferring tokens after the snapshot does not
transfer that epoch’s entitlement. See [FeeAccumulator](contracts/data/FeeAccumulator.sol) and
[FeesRouter](contracts/periphery/FeesRouter.sol) for claims and migration entrypoints.

### Dynamic Fees

[SidioraPool](contracts/core/SidioraPool.sol) calculates fees from the configured base, pool age in
seconds and RMS relative price movement across up to eight chronological observations. Snapshot
insertion is bounded by time and volume thresholds; these are not necessarily the last eight trades.

The library has a concentration component, but the current pool supplies a zero holder metric and
ProtocolConfig rejects nonzero concentration weights. Holder concentration does not currently raise
fees. Fresh defaults are a 30 bps base (0.30%), clamped between 10 bps (0.10%) and 300 bps (3.00%).
Age and volatility contributions can raise the actual fee above the base.

### Integration

The core launchpad creates USDL-paired markets with fee-rights NFTs and optional optical policy. Its
two-pool swaps and the optional adapter layer extend trading routes. They do not change the core
pool’s USDL pairing or establish oracle price quality. Integrators should use the matching deployed
ABI, current authority inventory and [operations runbook](operations/README.md) when evaluating a
deployment.

## Build and test

Run from `contracts/launchpad/` with the Node.js range and pnpm version declared in
[package.json](package.json). Install dependencies with `pnpm install --frozen-lockfile` using
[pnpm-lock.yaml](pnpm-lock.yaml).

| Command                     | Purpose                                                                          |
| --------------------------- | -------------------------------------------------------------------------------- |
| `pnpm compile`              | Compile Solidity contracts with Hardhat.                                         |
| `pnpm test`                 | Run the complete Hardhat test suite.                                             |
| `pnpm test:coverage`        | Run Solidity coverage.                                                           |
| `pnpm test:gas`             | Run tests with gas reporting.                                                    |
| `pnpm meta-ag:test`         | Run the Meta-AG test selection.                                                  |
| `pnpm meta-ag:coverage`     | Collect coverage for the Meta-AG test files.                                     |
| `pnpm meta-ag:gas`          | Run the Meta-AG test selection with gas reporting.                               |
| `pnpm meta-ag:layout:check` | Compile and compare tracked storage layouts with core and Meta-AG baselines.     |
| `pnpm lint:sol`             | Lint Solidity sources with Solhint.                                              |
| `pnpm lint:js`              | Lint JavaScript scripts and tests with ESLint.                                   |
| `pnpm slither`              | Run static analysis using `slither.config.json`.                                 |
| `pnpm release:check`        | Check compiler inputs, artifacts, runtime size and production storage baselines. |

These commands are declared in [package.json](package.json). Slither uses the dependency pinned in
[requirements-audit.txt](requirements-audit.txt), installed in an isolated Python environment.
Documentation generation is opt-in through `RUN_DOCGEN` during compilation. Local tests, static
analysis and code review establish properties of the source and build; deployed state is inspected
separately through the [operations runbook](operations/README.md).

## Configuration

[hardhat.config.js](hardhat.config.js) loads `.env` through `dotenv/config` and reads `PRIVATE_KEY`,
`REPORT_GAS` and `RUN_DOCGEN`. It configures Solidity 0.8.27 with viaIR, optimizer settings, storage
layout output, and the `paxeer-x-network` and `hardhat-network` connections. Network connection
settings live in that file.

The scripts and tests read the following environment variables; names are grouped by use:

| Use                                    | Environment variables                                                                                                                                                                                                                                                                    |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Core deployment and governance         | `USDL_ADDRESS`, `SID_ADDRESS`, `GUARDIAN_ADDRESS`, `TIMELOCK_DELAY`, `GOVERNANCE_MODE`, `GOVERNANCE_ADMIN_ADDRESS`, `ADMIN_ADDRESS`, `NETWORK_TYPE`, `RESUME_DEPLOYMENT`                                                                                                                 |
| Meta-AG deployment                     | `WPAX_ADDRESS`, `RELAYER_ADDRESS`, `KEEPER_ADDRESS`, `FEE_COLLECTOR`, `INCIDENT_GUARDIAN_ADDRESS`, `TRANSFER_ADMIN_TO_TIMELOCK`, `VAULT_ADAPTER_FEE_BPS`, `ORACLE_DEVIATION_BPS`, `SIDIORA_MIN_LIQUIDITY_RAW`, `ENABLE_SIDIORA_SPOT_REFERENCE`, `USDL_USD_ORACLE_ADDRESS`, `SEED_PRICES` |
| Upgrade plans and event-emitter wiring | `UPGRADE_PLAN`, `LAUNCHPAD_MANIFEST`, `CONTRACT`, `BEACON`, `PROXY_ADDRESS`, `IMPL_ADDRESS`, `TIMELOCK_ADDRESS`, `ADMIN_WITH_EMITTER_ROLE`, `BUFFER_SECONDS`, `ALLOW_EARLY`, `FORCE_REDEPLOY`, `SKIP_VERIFY`, `SKIP_OPTICAL`, `SKIP_ROUTER`, `SKIP_FACTORY`                              |
| Native launchpad tooling               | `LAUNCHPAD_AUTHORITY`, `LAUNCHPAD_PARAMS_JSON`                                                                                                                                                                                                                                           |
| Integration generation                 | `TEMPLATE_MODE`                                                                                                                                                                                                                                                                          |
| Health and verification                | `RPC_URL`, `VERIFY_DRY_RUN`                                                                                                                                                                                                                                                              |
| Token deployment and liquidity seeding | `INITIAL_MINT`, `EXTRA_RECIPIENTS`, `OUTPUT_FILE`, `DRY_RUN`, `TOP_UP`, `ONLY`, `SKIP`, `FORCE_LEGACY_TX`, `GAS_PRICE`, `TX_WAIT_TIMEOUT_MS`, `TX_POLL_MS`, `PROVIDER_POLL_MS`                                                                                                           |
| Test reports                           | `UPDATE_GAS_SNAPSHOTS`, `PECOR_COVERAGE_OUTPUT`, `LAUNCHPAD_PROOF_NODE_TEST`                                                                                                                                                                                                             |

The generated integration examples use `ROUTER_ADDRESS` and `NEXT_PUBLIC_ROUTER_ADDRESS`. Liquidity
seeding also reads per-symbol `SEED_RAW_${symbol}` and `SEED_${symbol}` names. Oracle registration
reads `ORACLE_HEARTBEAT_${symbol}`, `ORACLE_DEVIATION_BPS_${symbol}`, `ORACLE_MIN_PRICE_${symbol}`,
`ORACLE_MAX_PRICE_${symbol}` and `ORACLE_MAX_STALENESS_${symbol}`. The symbol comes from the token
inventory.

Scripts read network manifests and journals in [deployments/](deployments/), including
[paxeer-x-network-addresses.json](deployments/paxeer-x-network-addresses.json) and
[hardhat-network-addresses.json](deployments/hardhat-network-addresses.json). Meta-AG helpers also
read the network token inventory. Upgrade preflight reads the JSON plan selected by `UPGRADE_PLAN`,
the optional `LAUNCHPAD_MANIFEST`, and the baseline, simulation and token-inventory files named by
the plan. Native parameter updates read the JSON file selected by `LAUNCHPAD_PARAMS_JSON`. Health
inspection accepts a manifest and an optional policy JSON file as command-line arguments. Release
checks read [production storage baselines](storage-layout/production/), including their
[manifest](storage-layout/production/manifest.json).

Development tools use [.solhint.json](.solhint.json), [.eslintrc.js](.eslintrc.js),
[.prettierrc](.prettierrc), [.editorconfig](.editorconfig),
[slither.config.json](slither.config.json) and [pnpm-workspace.yaml](pnpm-workspace.yaml).

### Deployment and recovery

- **Chain**: Paxeer mainnet-beta, EVM chain ID 125 (`0x7d`)
- **Hardhat network**: `paxeer-x-network`, using root-path JSON-RPC
- **Solidity**: 0.8.27 with viaIR; compiler settings are in [hardhat.config.js](hardhat.config.js)
- **Upgrade authority after handoff**: Timelock, with GovernanceModule as proposer
- **Default decisions**: administrator-only governance through `GOVERNANCE_MODE`; configured
  administrator or deployment wallet, with no SID requirement
- **Optional token governance**: token governance through `GOVERNANCE_MODE` with `SID_ADDRESS`
  providing historical voting checkpoints
- **Multisig**: supplied explicitly through `GOVERNANCE_ADMIN_ADDRESS`; not created or guaranteed by
  deployment
- **Addresses**: read the reviewed manifest in [deployments/](deployments/); this overview does not
  establish live contract addresses

### Operator safety and upgrade qualification

The [production operations runbook](operations/README.md) covers limited emergency pause authority,
governed recovery, oracle jump limits and readiness, optical identity renewal, read-only health
inspection, and reproducible release qualification. Existing deployments require explicit role
wiring and the documented migration checks before these capabilities are operational.

Run scripts with the connected Hardhat network; `NETWORK_TYPE`, if supplied, must agree. The
canonical network manifest is in [deployments/](deployments/), with flat `Name_proxy`/`Name_impl`
keys and `_meta.network` as the chain ID. Legacy manifests require an explicit operator migration;
no implicit network fallback is safe.

Core deployment uses temporary deployer authority, hands proxy administration to Timelock, and hands
proposer authority to GovernanceModule. In administrator mode it retains the selected
administrator’s `ADMIN_ROLE` and active administrator decision path. Token mode disables bootstrap
administrator decisions and removes the deployment wallet’s `ADMIN_ROLE`. Core `ADMIN_ADDRESS`
overrides fail before deployment. Beacon ownership acceptance is queued through Timelock and remains
pending until its ETA: the deployment manifest records that outstanding step. A deployment is not
finalized until delayed acceptance succeeds and owner/roles are checked. MetaAG likewise wires under
temporary authority and hands administrative roles to the existing Timelock; relayer, keeper and fee
collector remain explicit operational roles.

Upgrade commands require `UPGRADE_PLAN` pointing to a reviewed JSON plan: `chainId`,
`migrationCalldata`, `simulation: {file, sha256}`, and `contracts` entries containing `contract`,
`proxyKey`, `beacon`, `oldImplementation`, `oldCodeHash`, `baseline`, `baselineSha256`, and
`newArtifactHash` (keccak256 creation bytecode). Preflight checks live implementation identity,
pinned storage baseline, recursive type compatibility, artifact identity and simulation evidence
hash before broadcasting. Evidence hashes establish identity, not quality: reviewers must verify the
fork simulation and migration assertions. Retain baseline layouts and prior artifacts; incompatible
layouts require a separately designed migration, not a refreshed baseline. Storage gaps are
conservatively immutable in the automated checker.

Beacon upgrade plans additionally require `tokenInventory: {file, sha256}` on the pool entry. The
JSON inventory must cover every registered pool and contain `pool`, `token`, `runtimeHash`, and
reviewed `transferHookSimulation` evidence. Preflight verifies token identity, pool binding, and
historical balance/supply APIs. Existing immutable tokens lacking these capabilities require a
separate migration design and cannot be made compatible by upgrading their pool beacon. Legacy
FeeAccumulator upgrades additionally need bounded accounting migration and reviewed immutable legacy
airdrop allocations; legacy NFTs need bounded pool-index migration before new mints. Preserve
historical artifacts and audit the actual migration transaction sequence.

Write helpers retain transaction hash, nonce, target, calldata and value before awaiting
confirmation. They do not automatically resubmit ambiguous writes: reconcile the recorded receipt
and operation-specific postcondition before retrying. Finalized core proxies and beacon are
controlled by Timelock, so direct EOA upgrade scripts cannot bypass governance; prepare and execute
upgrades through the governed proposal lifecycle.

MetaAG deployment defaults the Sidiora spot adapter to disabled. `SIDIORA_MIN_LIQUIDITY_RAW` must be
a positive USDL amount in token base units; default is 1,000 USDL scaled using its live decimals.
Enabling `ENABLE_SIDIORA_SPOT_REFERENCE` additionally requires an independent
`USDL_USD_ORACLE_ADDRESS` implementing a fresh positive `getPrice(USDL)`; this opts into
instantaneous manipulable pool prices, not manipulation resistance. The adapter's disabled default
is not evidence of healthy price availability.

Interrupted deployments can resume with the same environment and `RESUME_DEPLOYMENT` enabled. The
manifest binds configuration, chain, deployer, constructor bytecode, confirmed transaction
identities and runtime hashes. Confirmed implementations, proxies and immutable contracts are
reused; wiring and administrative handoffs reconcile exact on-chain postconditions. Proxy and beacon
implementation drift fails before new broadcasts. New creation intent, predicted address and nonce
are persisted before broadcasting, so an ambiguous RPC response cannot silently cause duplicate
deployment. Ambiguous pending or failed creation receipts require explicit operator reconciliation
rather than automatic resubmission. Keep the original checkpoint; do not replace it with a newly
generated manifest. Expired beacon acceptance requires a reviewed governance cancellation/requeue.

For an existing checkpoint on the configured local `hardhat-network` (chain ID 31337), retain the
original deployment variables and use the core or Meta-AG deployment script with `RESUME_DEPLOYMENT`
enabled. The [operations runbook](operations/README.md) describes the core invocation and delayed
ownership acceptance.

Select the intended connected network explicitly. Core and layered plans occupy separate
`_deployment` and `_metaAgDeployment` sections of the same manifest; layered recovery preserves the
inherited core identity. The layered repair script delegates to the bound deployment plan when
present. An unknown broadcast, missing or failed receipt, configuration mismatch, or implementation
drift stops recovery for review instead of creating replacement contracts.

## Documentation

- [Solidity API reference](docs/) and [IEventEmitter payloads](docs/interfaces/IEventEmitter.md).
- [Production operations](operations/README.md) for deployment, governance, upgrades and recovery.
- [Native launchpad execution](../launchpad.md#native-execution) for the network module and
  precompile.
- [Launch a market and chain](../../docs/launchpad/chain-launch.md) for the native chain-launch
  flow.
- [Launchpad web configuration checklist](../../docs/launchpad/production-checklist.md) for the
  application.
- [Launchpad documentation site page](../../docs/site/paxeer-docs/src/app/launchpad/page.tsx) and
  its [Markdown content](../../docs/site/paxeer-docs/public/markdown/launchpad.md) for the native
  launchpad interface.
