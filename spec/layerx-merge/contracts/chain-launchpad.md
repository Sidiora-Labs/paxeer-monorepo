# Chain launchpad contract

The launchpad at `0x0000000000000000000000000000000000001017` launches a market and a virtual chain in one root-network transaction. The chain registry at `0x000000000000000000000000000000000000101E` remains the registry of record; its direct `createChain` call continues to create a chain without a launchpad binding. A virtual chain shares network state and validators and uses PAX for gas. This contract covers decisions 3316, 3317 and 3318.

## abi

The declarations below extend `ILaunchpad`. `PermitParams` is the existing tuple `(uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s)`. The return order of a launch is token, pool, fee right, registry number, derived chain ID. `chain` and `ID` always mean the registry's `uint32` number. The chain ID is `125000000 + chain` and fits `uint64`.

```solidity
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

`AlreadyClaimed` is shared with the existing airdrop ABI. An absent launch record reads as zero from `launchedChainOfPool` and `homeChainOf`; `launchedChain`, epoch and claim reads of an unknown chain have no record. Writes requiring a launch record refuse with `NotLaunchedChain`.

## launch

`launchChain` creates the market for the caller with the same charges, token, pool and fee right as `createMarket`; its permit variant uses the same permit tuple and authorization as `createMarketWithPermit`. The payable value covers the registry's current minimum bond plus creation fee, separately from market charges and EVM gas. The registry validates its chain name, allowances, fee share and remaining capacity. A single cached state transition creates the market, creates the registry record and writes the binding; any refusal reverts all parts. `ChainLaunched` is emitted with the market events. The launch record remains after retirement.

The registry operator is the chain's revenue account and never changes. For nonzero registry number `chain`, the account is the 20-byte `authtypes.NewModuleAddress("launchpad_chain_revenue_" + decimal(chain))`, converted to an EVM address by its 20 bytes. The decimal form has no leading zero. This distinct module account has no signing key and is separate from the launchpad module custody account, treasury, pool and token. The registry pays its operator share of base fees into that account at block end. The fee right holder controls the launched chain through the launchpad: `pauseLaunchedChain`, `resumeLaunchedChain` and `retireLaunchedChain` require the current owner of the fee right and otherwise refuse with `NotFeeRightHolder`. The registry retains governance pause authority. On retirement, the returned bond alone goes to that holder in the same transition; waiting operator fees stay in the revenue account. Open epochs remain claimable.

## storage

New launchpad store prefixes hold these records and indexes; genesis export and import preserve them. A genesis without these families has empty launch records and a missing home-chain entry reads as zero.

| Record | Key | Value |
| --- | --- | --- |
| `LaunchedChain` | registry number (`uint32`) | registry number, chain ID (`uint64`), pool, token, fee right number (`uint256` at the ABI), creator, launch block (`uint64`) |
| `LaunchedChainOfPool` | pool address | registry number (`uint32`) |
| `HomeChainOfPool` | pool address | derived chain ID (`uint64`), or zero for the root network |
| `ChainRevenueLastEpoch` | registry number (`uint32`) | latest epoch number (`uint64`) |
| `ChainRevenueEpoch` | registry number (`uint32`), epoch number (`uint64`) | epoch amount, snapshot block (`uint64`), eligible supply, claimed total, opening time (`uint64` seconds); monetary and supply values fit `uint256` |
| `ChainRevenueClaim` | registry number (`uint32`), epoch number (`uint64`), holder address | claimed marker |

## home-chain

A market records the derived chain ID of the transaction that created it. A market launched with `launchChain` instead records the derived chain ID of the new chain. Zero denotes the root network. A market with nonzero home chain accepts `buy`, `sell`, pool swaps and each leg of `swapTokenForToken` only when the ante-handler chain in the transaction context equals its home chain; other writes refuse with `WrongChain`. Reads and quotes work on every chain. A root-home market keeps the existing trading behavior on every chain. To trade on registry chain `n`, sign with chain ID `125000000 + n` and submit through `/chain/n`.

## revenue

Anyone may open the next epoch after at least 3600 seconds of block time since the previous epoch's opening. The first epoch has no prior interval. `EpochTooEarly` identifies a shorter interval. `NothingToDistribute` identifies a zero PAX balance in the revenue account or zero eligible token supply. Opening moves the entire PAX balance to launchpad epoch custody, transfers every other denomination in the revenue account to the launchpad treasury, and fixes `snapshotBlock = block.number - 1`. Eligible supply is `getPastTotalSupply(snapshotBlock) - getPastBalance(pool, snapshotBlock) - getPastBalance(burnAddress, snapshotBlock)`, using the token checkpoint reads of the airdrop epochs. The burn address is `0x000000000000000000000000000000000000dEaD`.

For each holder other than the pool and burn address, a claim pays `floor(epochAmount * getPastBalance(holder, snapshotBlock) / eligibleSupply)` PAX once. A zero past balance has no claim. The record marks the holder and increases claimed total by the amount paid. The keeper derives claimed eligible supply from the marked holders’ past balances. When claimed eligible supply reaches eligible supply, the final eligible claimant receives the remaining epoch amount, including rounding residue. `AlreadyClaimed` refuses a repeated claim. The batch call applies the same rule to each listed epoch in order, atomically; any refusal reverts the batch. Trading charges in the quote asset continue to follow the market's fee strategy. Retirement does not close revenue epochs.

For example, an epoch of 10 PAX with eligible balances 1, 1 and 1 has eligible supply 3. The first two holders each receive 3 PAX; the final holder receives the remaining 4 PAX. The three payments equal the epoch amount.

## api

The launchpad API serves these reads from indexed launch and revenue events. `ID` is the decimal registry number, never the derived chain ID. Responses use the existing v2 response envelope. Amounts are decimal strings; chain, epoch, block, status and allowance fields use JSON numbers. A missing launched chain returns the API's 404 error body.

| Route | Data fields |
| --- | --- |
| `GET /v2/chains` | A newest-first cursor page with `items` and `nextCursor`. Each item has `chain`, `chainId`, `pool`, `token`, `nftId`, `creator`, `blockNumber`, `status`. The cursor follows the other v2 list routes. |
| `GET /v2/chains/ID` | One item with `chain`, `chainId`, `pool`, `token`, `nftId`, `creator`, `blockNumber`, `status`, plus registry `name`, `gasAllowance`, `byteAllowance`, `feeShareBps`, `bond` and `revenueAccount`. |
| `GET /v2/chains/ID/revenue` | Newest-first epochs, each with `chain`, `epoch`, `epochAmount`, `snapshotBlock`, `eligibleSupply`, `claimedTotal`, `openedAt`, and `nextCursor`. With `holder`, each epoch also has that holder's `claimable` amount and `claimed` state. |

The v2 pool detail includes `homeChain`, zero for a root-home market. The [virtual chains contract](virtual-chains.md) defines registry IDs, chain status and scoped RPC behavior. The [chain launch guide](../../../docs/launchpad/chain-launch.md) explains the wallet flow, trading path and revenue claims.
