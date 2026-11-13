# Solidity API

## MockFeeOnTransferERC20

Configurable ERC20 test token that burns a basis-point fee on transfers. `feeBps` must be below
10,000. With `taxedRecipient == address(0)`, every recipient is taxed; otherwise only transfers to
`taxedRecipient` are taxed. A zero fee rate transfers the full amount. Minting and fee configuration
are unrestricted.

Source: [MockFeeOnTransferERC20.sol](../../../contracts/meta-ag/mocks/MockFeeOnTransferERC20.sol).

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

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

### feeBps

```solidity
uint256 feeBps
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

### FeeUpdated

```solidity
event FeeUpdated(uint256 bps)
```

### constructor

```solidity
constructor(string name_, string symbol_, uint8 decimals_, uint256 feeBps_) public
```

### setFeeBps

```solidity
function setFeeBps(uint256 bps) external
```

### mint

```solidity
function mint(address to, uint256 amount) external
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

### \_transferWithFee

```solidity
function _transferWithFee(address from, address to, uint256 amount) internal
```

### setTaxedRecipient

```solidity
function setTaxedRecipient(address recipient) external
```

Selects the recipient subject to the transfer fee. A zero address applies the fee to every
recipient.

### taxedRecipient

```solidity
address taxedRecipient
```
