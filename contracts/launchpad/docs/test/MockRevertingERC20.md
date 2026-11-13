# Solidity API

Source: [MockRevertingERC20.sol](../../contracts/test/MockRevertingERC20.sol).

## MockRevertingERC20

ERC20 test helper whose `transfer`, `transferFrom` and `approve` revert while `shouldRevert` is true
(the default). `setShouldRevert(false)` enables those operations.

### name

```solidity
string public name
```

### symbol

```solidity
string public symbol
```

### decimals

```solidity
uint8 public decimals
```

### totalSupply

```solidity
uint256 public totalSupply
```

### balanceOf

```solidity
mapping(address => uint256) public balanceOf
```

### allowance

```solidity
mapping(address => mapping(address => uint256)) public allowance
```

### shouldRevert

```solidity
bool public shouldRevert
```

### Transfer

```solidity
event Transfer(address indexed from, address indexed to, uint256 value)
```

### Approval

```solidity
event Approval(address indexed owner, address indexed spender, uint256 value)
```

### constructor

```solidity
constructor(string memory _name, string memory _symbol, uint8 _decimals)
```

### mint

```solidity
function mint(address to, uint256 amount) external
```

### setShouldRevert

```solidity
function setShouldRevert(bool _shouldRevert) external
```

### approve

```solidity
function approve(address spender, uint256 amount) external returns (bool)
```

### transfer

```solidity
function transfer(address to, uint256 amount) external returns (bool)
```

### transferFrom

```solidity
function transferFrom(address from, address to, uint256 amount) external returns (bool)
```
