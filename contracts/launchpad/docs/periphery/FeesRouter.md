# Solidity API

Source: [FeesRouter.sol](../../contracts/periphery/FeesRouter.sol).

## FeesRouter

NFT-holder interface for fee management.

_UUPS proxy. Validates NFT ownership, reads strategy, delegates to FeeAccumulator._

### nftContract

```solidity
address nftContract
```

### feeAccumulator

```solidity
address feeAccumulator
```

### poolRegistry

```solidity
address poolRegistry
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _nftContract, address _feeAccumulator, address _poolRegistry, address _admin) external
```

### setFeeStrategy

```solidity
function setFeeStrategy(uint256 nftId, uint8 newStrategy) external
```

Set fee strategy for a pool NFT (owner-only)

#### Parameters

| Name        | Type    | Description                                                                                   |
| ----------- | ------- | --------------------------------------------------------------------------------------------- |
| nftId       | uint256 | The NFT token ID                                                                              |
| newStrategy | uint8   | New fee strategy (0=CLAIM,1=BURN,2=AIRDROP,3=LP_REWARDS); AIRDROP requires checkpoint support |

### claimFees

```solidity
function claimFees(uint256 nftId) external returns (uint256 amount)
```

Claim accumulated fees (CLAIM strategy only, NFT owner)

#### Parameters

| Name  | Type    | Description      |
| ----- | ------- | ---------------- |
| nftId | uint256 | The NFT token ID |

#### Return Values

| Name   | Type    | Description         |
| ------ | ------- | ------------------- |
| amount | uint256 | USDL amount claimed |

### executeBurn

```solidity
function executeBurn(uint256 nftId) external returns (uint256 amount)
```

Transfer accumulated USDL fees to the accumulator's dead address (BURN strategy only, NFT owner).
The accumulator performs a token transfer.

#### Parameters

| Name  | Type    | Description      |
| ----- | ------- | ---------------- |
| nftId | uint256 | The NFT token ID |

#### Return Values

| Name   | Type    | Description                                 |
| ------ | ------- | ------------------------------------------- |
| amount | uint256 | USDL amount transferred to the dead address |

### executeAirdrop

```solidity
function executeAirdrop(uint256 nftId) external returns (uint256 amount)
```

Create a snapshot-based airdrop epoch (AIRDROP strategy only, NFT owner). Holders claim their shares
separately; the returned amount is allocated to the epoch.

#### Parameters

| Name  | Type    | Description      |
| ----- | ------- | ---------------- |
| nftId | uint256 | The NFT token ID |

#### Return Values

| Name   | Type    | Description                     |
| ------ | ------- | ------------------------------- |
| amount | uint256 | USDL allocated to the new epoch |

### claimAirdrop

```solidity
function claimAirdrop(uint256 nftId) external returns (uint256 amount)
```

Claim the caller's share of the latest airdrop epoch using its recorded snapshot balance. Requires
neither NFT ownership nor the AIRDROP strategy to remain selected. Claims close 180 days after the
epoch is triggered.

#### Parameters

| Name  | Type    | Description                   |
| ----- | ------- | ----------------------------- |
| nftId | uint256 | The NFT token ID for the pool |

#### Return Values

| Name   | Type    | Description         |
| ------ | ------- | ------------------- |
| amount | uint256 | USDL amount claimed |

### executeLpRewards

```solidity
function executeLpRewards(uint256 nftId) external returns (uint256 amount)
```

Execute LP rewards transfer (LP_REWARDS strategy only, NFT owner)

#### Parameters

| Name  | Type    | Description      |
| ----- | ------- | ---------------- |
| nftId | uint256 | The NFT token ID |

#### Return Values

| Name   | Type    | Description              |
| ------ | ------- | ------------------------ |
| amount | uint256 | USDL amount sent to pool |

### \_requireNftOwner

```solidity
function _requireNftOwner(uint256 nftId) internal view
```

### \_requireStrategy

```solidity
function _requireStrategy(uint256 nftId, uint8 expectedStrategy) internal view
```

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### claimAirdropEpoch

```solidity
function claimAirdropEpoch(uint256 nftId, uint256 epoch) external returns (uint256 amount)
```

Claims the caller's snapshot-based USDL share for a specified airdrop epoch. `claimAirdrop` selects
the latest epoch; each claim is subject to the accumulator's claim window.
