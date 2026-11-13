# Launchpad execution and operations contracts

Launchpad executes on the single Paxeer mainnet-beta network, chain ID **125** (`0x7d`). The kernel is part of `paxd` and block proposal; BFT block commit makes execution final. Indexers and HTTP services project that committed state for [clients](clients.md); they do not provide a second settlement layer. Sub-chains are tenant domains in the database.

Node JSON-RPC uses the configured [HTTPS root endpoint](evm-rpc.md#http-endpoint).
Hosted service calls follow the [gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

## Native execution

The active Go module is [`modules/launchpad`](../modules/launchpad), with module/store name `launchpad` and treasury account `launchpad_treasury`. [keys.go](../modules/launchpad/types/keys.go) declares the launchpad precompile at **`0x0000000000000000000000000000000000001017`** and launched-token decimals as 6.

[`precompiles/launchpad`](../precompiles/launchpad) exposes the module through [the direct ABI](../precompiles/launchpad/abi.json) and [role ABIs](../precompiles/launchpad/abi). Direct selectors resolve in the role order documented in [launchpad.go](../precompiles/launchpad/launchpad.go): `Router`, `SidioraFactory`, `Quoter`, `PoolRegistry`, `FeesRouter`, `FeeAccumulator`, `ProtocolConfig`, followed by the chain launch methods. Registered role and pool surfaces forward calls with the original caller appended; [surface.go](../precompiles/launchpad/surface.go) installs the runtime owned by the keeper. A role named `Router` here is an on-chain launchpad ABI role. There is no network RPC router.

Use the reviewed manifest to select role addresses. Native surfaces are not Solidity implementation proxies to upgrade with UUPS. [The keeper's governance message server](../modules/launchpad/keeper/msg_server.go) implements `MsgUpdateParams`, role grants/revokes and treasury operations under the module governance authority. The [retired-address runtime](../precompiles/launchpad/retired.go) refuses calls with `Retired(address replacement)` instead of redirecting a write to another deployment.

The [swap keeper](../modules/launchpad/keeper/swap.go) computes fee and output amounts with integer arithmetic, maintains real and virtual reserves, and enforces liquidity, slippage and optical constraints. Quotes do not reserve execution. Clients must preserve the quoted units, minimum output and deadline when signing; HTTP success or a preview is not a committed swap.

## Launchpad errors

The current [error registry](../modules/launchpad/types/errors.go) uses codespace `launchpad`, numeric codes 2–81 and `LXP_LAUNCHPAD_*` descriptions. Examples:

| Code | Description | Contract custom error |
| --- | --- | --- |
| 3 | `LXP_LAUNCHPAD_OVERFLOW` | `Overflow()` |
| 6 | `LXP_LAUNCHPAD_INSUFFICIENT_LIQUIDITY` | `InsufficientLiquidity()` |
| 8 | `LXP_LAUNCHPAD_SLIPPAGE_EXCEEDED` | `SlippageExceeded()` |
| 9 | `LXP_LAUNCHPAD_DEADLINE_EXPIRED` | `DeadlineExpired()` |
| 12 | `LXP_LAUNCHPAD_UNKNOWN_MARKET` | `PoolNotFound()` |
| 16 | `LXP_LAUNCHPAD_NOT_FEE_RIGHTS_HOLDER` | `NotNftOwner()` |
| 70 | `LXP_LAUNCHPAD_RETIRED` | `Retired(address)` |
| 77 | `LXP_LAUNCHPAD_WRONG_CHAIN` | `WrongChain()` |
| 78 | `LXP_LAUNCHPAD_NOT_LAUNCHED_CHAIN` | `NotLaunchedChain()` |

The registry's `ContractErrorOf` table and the precompile's
[errors.go](../precompiles/launchpad/errors.go) define custom error signatures and
arguments. The precompile additionally maps codes 77–81 to `WrongChain()`,
`NotLaunchedChain()`, `NotFeeRightHolder()`, `EpochTooEarly()`, and
`NothingToDistribute()`. It preserves nested revert bytes; keeper-only failures
use an ABI `Error(string)` reason. Do not assume every failure is a string or
that module names map mechanically to Solidity error names.
[run.go](../precompiles/launchpad/run.go) also handles unknown selectors with
empty revert data. The `legacy/v610` registries describe historical replay, not
the current error inventory.

## Launchpad events

There are separate Cosmos event names and EVM ABI logs. [types/events.go](../modules/launchpad/types/events.go) declares Cosmos events such as `launchpad_market_created`, `launchpad_swap`, `launchpad_fee_recorded`, `launchpad_fee_strategy_changed`, `launchpad_params_updated` and `launchpad_pool_state_updated`. Attributes include `denom`, `pointer`, `trader`, `is_buy`, `amount_in`, `amount_out`, `fee` and `strategy`; use the emitting keeper to determine which attributes belong to an event.

EVM logs follow [precompile event encoding](../precompiles/launchpad/events.go), the role ABIs and the keeper's log emission. For example, `MsgUpdateParams` emits `ConfigUpdated(bytes32,uint256,uint256)` and the five-argument EventEmitter form at `0x1017`; changed role membership emits `RoleGranted` or `RoleRevoked` at the role surface. A failed update writes and emits nothing from that cached update.

The retained Solidity [EventEmitter](launchpad/contracts/data/EventEmitter.sol) is an optional mirror; native source logs remain authoritative. Its [interface](launchpad/contracts/interfaces/IEventEmitter.sol) includes typed and generic event families. Do not require a `poolId` on every event: `TokenTransfer`, for example, has a different identity shape. `reinitializeV2` is once-only and repeated calls revert `Unauthorized()`; reconcile confirmed state before retrying an upgrade transaction.

The [indexer](services.md#launchpad-indexer) validates signatures and emitter identities before projection. Consumers must retain block hash, transaction hash, emitter and log position, and decode the ABI belonging to that role and revision. A mirror event is not an additional trade or a separate finality proof.

## Launchpad contracts tooling

The retained Solidity project is [contracts/launchpad](launchpad), with its own [package manifest](launchpad/package.json) and [Hardhat configuration](launchpad/hardhat.config.js). Its package manager is pnpm 10.34.9, Node requirement is `>=22 <25`, and Solidity compiler is 0.8.27 with optimizer and IR enabled. `PECOROrders.sol` has a separate optimizer-runs override. These sources support contract deployments, compatibility and regression work alongside the native module.

| Package script | Declared action |
| --- | --- |
| `compile` | Hardhat compilation |
| `test` | Hardhat tests |
| `deploy:paxeer` | `scripts/deploy.js` on `paxeer-x-network` |
| `verify:paxeer` | `scripts/verify.js` without compilation on that network |
| `health` | Read-only `scripts/production-health.js` |
| `release:check` | Compiled artifact/layout checks |
| `release:qualify` | The retained Solidity project's release qualification |

The configured `paxeer-x-network` uses chain ID 125 and a root-path HTTPS node endpoint. `hardhat-network` uses local port 8545 and chain ID 31337. Select the actual network name from the configuration; do not use an invented alias. `PRIVATE_KEY` supplies deployment signers; it is not required by the standalone read-only health inspector. `RUN_DOCGEN=true` enables the documentation plugin; `REPORT_GAS=true` enables gas reporting.

[deploy.js](launchpad/scripts/deploy.js) defaults `GOVERNANCE_MODE` to `admin` and calls `initializeAdmin`. `GOVERNANCE_ADMIN_ADDRESS` overrides the deployment wallet as decision authority; choosing a multisig requires supplying that address. Token voting is an explicit `GOVERNANCE_MODE=token` choice requiring `SID_ADDRESS`. A deployment can remain `pending-beacon-acceptance`; a written address file does not prove the handoff completed.

[operations.js](launchpad/scripts/lib/operations.js) binds `NETWORK_TYPE`, if supplied, to Hardhat's actual network name and defaults manifests to `deployments/<network>-addresses.json`. `validateManifest` checks chain ID, address syntax and code presence. Runtime/implementation identity and governance acceptance require their own readback; code presence alone is insufficient. Deployment and transaction journals support reconciliation across interrupted runs. The [operations guide](launchpad/operations/README.md) describes the retained contract project's authority and inspection workflow.

## Launchpad contracts operations

[`native-surface.js`](launchpad/scripts/lib/native-surface.js) recognizes one exact runtime byte string. Its recognition must not be treated as a general proof for every native surface version; compare against the runtime installed by the [current keeper](../modules/launchpad/keeper) and the selected deployment.

[`planCreationFee`](launchpad/scripts/set-creation-fee.js) returns a governance message for the runtime it recognizes as native. For a recognized native surface, `LAUNCHPAD_AUTHORITY` is required and `LAUNCHPAD_PARAMS_JSON` names a file containing the full params object. The emitted message has type `/paxprotocol.paxchain.launchpad.MsgUpdateParams`, replaces `creation_fee` with `1000000`, and includes empty `role_grants` and `role_revokes`. It prints the message rather than submitting it. Its contract branch calls `setCreationFee` and checks readback. `LAUNCHPAD_MANIFEST` can select the address file. Verify the runtime branch before using this script against any deployed role.
