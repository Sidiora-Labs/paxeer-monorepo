# Solidity API

Source: [MockERC20.sol](../../contracts/test/MockERC20.sol).

## MockERC20

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

### burn

```solidity
function burn(address from, uint256 amount) external
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
