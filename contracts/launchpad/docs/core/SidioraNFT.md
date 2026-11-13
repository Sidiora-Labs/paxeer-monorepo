# Solidity API

Source: [SidioraNFT.sol](../../contracts/core/SidioraNFT.sol).

## SidioraNFT

ERC721 representing fee rights for a pool. One NFT per pool.

_UUPS proxy singleton. Factory mints via MINTER_ROLE._

### MINTER_ROLE

```solidity
bytes32 MINTER_ROLE
```

### STRATEGY_SETTER_ROLE

```solidity
bytes32 STRATEGY_SETTER_ROLE
```

### nextTokenId

```solidity
uint256 nextTokenId
```

Returns the next token ID to mint. Initialization sets it to 1.

### eventEmitter

```solidity
address eventEmitter
```

### constructor

```solidity
constructor() ERC721Base("", "")
```

### initialize

```solidity
function initialize( string calldata _name, string calldata _symbol, address _eventEmitter, address _admin ) external initializer
```

### mint

```solidity
function mint( address to, address pool, uint8 strategy ) external override onlyRole(MINTER_ROLE) returns (uint256 tokenId)
```

Mints a pool NFT for a caller with `MINTER_ROLE`. A contract recipient must accept
`onERC721Received`. The pool must be nonzero and have no existing NFT in the pool index.

#### Parameters

| Name     | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| to       | address | The NFT recipient (pool creator)     |
| pool     | address | The pool address this NFT represents |
| strategy | uint8   | Initial fee strategy                 |

#### Return Values

| Name    | Type    | Description         |
| ------- | ------- | ------------------- |
| tokenId | uint256 | The minted token ID |

### getFeeStrategy

```solidity
function getFeeStrategy(uint256 tokenId) external view override returns (uint8)
```

Get the fee strategy for a token

### setFeeStrategy

```solidity
function setFeeStrategy(uint256 tokenId, uint8 newStrategy) external override
```

Sets the fee strategy. The caller must be the owner, an approved caller, or hold
`STRATEGY_SETTER_ROLE`. Strategy 2 requires a token with supported historical checkpoints through
its pool’s FeeAccumulator.

### getPoolAddress

```solidity
function getPoolAddress(uint256 tokenId) external view override returns (address)
```

Get the pool address associated with an NFT

### supportsInterface

```solidity
function supportsInterface(bytes4 interfaceId) public view virtual override returns (bool)
```

ERC165 override

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE)
```

### migratePoolIndex

```solidity
function migratePoolIndex(uint256 maxItems) external onlyRole(DEFAULT_ADMIN_ROLE)
```

Populates the pool uniqueness index in batches of 1–1,000 entries. Minting requires the index to be
ready; duplicate pools revert with `DuplicatePool()`.
