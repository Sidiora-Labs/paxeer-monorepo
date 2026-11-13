# Launch a market and chain

The launchpad at `0x0000000000000000000000000000000000001017` creates a token, its market pool, a fee-right NFT and a virtual chain in one root-network transaction. The chain gets a nonzero number in the `layerxchains` registry and is bound to that market. A chain shares the state and the validators of the network; it is a namespace for identity, fees and block space. It does not have separate balances or contract storage.

## Launch

Connect a wallet to the Paxeer root network (chain ID `125`) and open **Launch a chain** at `/create/chain`. Choose the token name and symbol, fee strategy, chain name and size tier. The [web adapter](../../apps/launchpad/web/src/widgets/create-chain/chain-adapters.ts) reads the registry's current minimum bond, creation fee and capacity before submission and sends one payable `launchChain` transaction. The ABI also exposes `launchChainWithPermit` for callers supplying a permit. The transaction's `msg.value` covers at least the current minimum bond plus the registry creation fee; market charges and transaction gas are separate. The creation fee is collected by the registry, and the remaining value becomes the chain bond. The registry validates the name, allowances, fee share and capacity. The market, registry record and binding are created together or the whole transaction reverts.

The [size tiers](../../apps/launchpad/web/src/widgets/create-chain/chain-domain.ts) select gas and byte allowances: starter is 500,000 gas and 65,536 bytes, growth is 1,500,000 gas and 262,144 bytes, and pro is 4,000,000 gas and 1,048,576 bytes. A tier must fit the registry's current gas limits and remaining capacity. Chain names contain 3–32 lowercase ASCII letters, digits or hyphens, with no leading or trailing hyphen.

The `ChainLaunched` receipt event identifies the registry number, pool, token, derived chain ID, fee-right NFT and creator. The registry remains the source for the chain's status and allowances. Direct registry `createChain` remains available for a chain without a launchpad market binding; see the [virtual chains guide](../wiki/VirtualChains.md).

## Trade on the home chain

The launched market's home chain is the new chain. For registry number `n`, its EVM chain ID is `125000000 + n`. Set the wallet to that chain ID and use the node's HTTP or WebSocket RPC path `<rpcBase>/chain/n`, where `n` is the registry number, not the derived chain ID. For example, registry chain `1` uses chain ID `125000001` (`0x7735941`) and `<rpcBase>/chain/1`. The signed chain ID determines transaction admission; the RPC path does not change a signature.

Buys, sells, pool swaps and each leg of a token-to-token swap for this market must be signed for its home chain. A write on another chain returns `WrongChain`; reads and quotes work across chains. A market created on the root network without this launch binding has home chain zero and retains its existing trading behavior.

## Revenue and control

The chain's share of base fees, set by `feeShareBps`, accrues in its dedicated revenue account after blocks with chain gas usage. The [launch implementation](../../modules/launchpad/keeper/chainlaunch.go) assigns that account as the registry operator; the creator is recorded separately in `launchedChain`. Revenue is separate from the registry creation fee and from market trading fees. Anyone can call `openChainRevenueEpoch(chain)` to move the accumulated PAX into a numbered epoch after the required interval. The first epoch has no prior interval; subsequent epochs require at least 3600 seconds of block time since the previous opening. An epoch snapshots token balances at the preceding block and excludes tokens held by the pool or burn address.

An eligible token holder can read `chainRevenueClaimable(chain, epoch, holder)` and call `claimChainRevenue(chain, epoch)` or `claimChainRevenueBatch(chain, epochs)`. Each holder claims once per epoch according to their balance at the snapshot. The last eligible claimant receives any rounding remainder. An epoch remains claimable after chain retirement.

The current owner of the fee-right NFT can call `pauseLaunchedChain`, `resumeLaunchedChain` and `retireLaunchedChain`. Retirement sends the returned bond to that owner. The registry retains its separate governance pause authority. Ownership of the fee right controls these chain actions; revenue claims follow the token-balance snapshot, so the claimant can be a different account.

## Chain launch ABI

These are the chain launch declarations in [`ILaunchpad`](../../precompiles/launchpad/Launchpad.sol) at the launchpad precompile; the machine-readable ABI is [`abi.json`](../../precompiles/launchpad/abi.json). `PermitParams` is the existing permit tuple. `chain` is the registry's `uint32` number.

```solidity
struct PermitParams {
    uint256 value;
    uint256 deadline;
    uint8 v;
    bytes32 r;
    bytes32 s;
}

function launchChain(string calldata name, string calldata symbol, uint8 feeStrategy, address optical, string calldata chainName, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps) external payable returns (address tokenAddr, address poolAddr, uint256 nftId, uint32 chain, uint64 chainId);
function launchChainWithPermit(string calldata name, string calldata symbol, uint8 feeStrategy, address optical, string calldata chainName, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps, PermitParams calldata permit) external payable returns (address tokenAddr, address poolAddr, uint256 nftId, uint32 chain, uint64 chainId);
function openChainRevenueEpoch(uint32 chain) external returns (uint64 epoch, uint256 epochAmount);
function claimChainRevenue(uint32 chain, uint64 epoch) external returns (uint256 amount);
function claimChainRevenueBatch(uint32 chain, uint64[] calldata epochs) external returns (uint256 totalAmount);
function pauseLaunchedChain(uint32 chain) external;
function resumeLaunchedChain(uint32 chain) external;
function retireLaunchedChain(uint32 chain) external;
function launchedChain(uint32 chain) external view returns (uint32 registryNumber, uint64 chainId, address pool, address token, uint256 nftId, address creator, uint64 blockNumber);
function launchedChainOfPool(address pool) external view returns (uint32 chain);
function homeChainOf(address pool) external view returns (uint64 chainId);
function chainRevenueEpoch(uint32 chain, uint64 epoch) external view returns (uint32 registryNumber, uint64 epochNumber, uint256 epochAmount, uint64 snapshotBlock, uint256 eligibleSupply, uint256 claimedTotal, uint64 openedAt);
function chainRevenueClaimable(uint32 chain, uint64 epoch, address holder) external view returns (uint256 amount);

event ChainLaunched(uint32 indexed chain, address indexed pool, address indexed token, uint64 chainId, uint256 nftId, address creator, uint256 timestamp, uint256 blockNumber);
event ChainRevenueEpochOpened(uint32 indexed chain, uint64 indexed epoch, uint256 epochAmount, uint64 snapshotBlock, uint256 eligibleSupply, uint256 timestamp, uint256 blockNumber);
event ChainRevenueClaimed(uint32 indexed chain, uint64 indexed epoch, address indexed holder, uint256 amount, uint256 timestamp, uint256 blockNumber);
event LaunchedChainRetired(uint32 indexed chain, address indexed holder, uint256 bond, uint256 timestamp, uint256 blockNumber);

error WrongChain();
error NotLaunchedChain();
error NotFeeRightHolder();
error EpochTooEarly();
error NothingToDistribute();
error AlreadyClaimed();
```

The [chain launchpad contract](../../spec/layerx-merge/contracts/chain-launchpad.md) defines the storage, epoch calculation and API response fields for integrators.
