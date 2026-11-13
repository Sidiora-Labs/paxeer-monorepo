# Solidity API

Source: [MockStandardERC20.sol](../../../contracts/meta-ag/mocks/MockStandardERC20.sol).

## MockStandardERC20

ERC20 test token with configurable name, symbol and decimals, plus unrestricted `mint` and `burn`
methods. Its distinct contract name allows it to be selected separately from `MockERC20`.

### name

```solidity
string name
```

### symbol

```solidity
string symbol
```

### decimals

```solidity
uint8 decimals
```

### totalSupply

```solidity
uint256 totalSupply
```

### balanceOf

```solidity
mapping(address => uint256) balanceOf
```

### allowance

```solidity
mapping(address => mapping(address => uint256)) allowance
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
constructor(string name_, string symbol_, uint8 decimals_) public
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

### \_transfer

```solidity
function _transfer(address from, address to, uint256 amount) internal
```
