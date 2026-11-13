# Solidity API

Source: [MockInitializable.sol](../../contracts/test/MockInitializable.sol).

## MockInitializable

### value

```solidity
uint256 public value
```

### initialized

```solidity
bool public initialized
```

### initialize

```solidity
function initialize(uint256 _value) external initializer
```

### reinitialize2

```solidity
function reinitialize2(uint256 _value) external reinitializer(2)
```

### reinitialize3

```solidity
function reinitialize3(uint256 _value) external reinitializer(3)
```

### getInitializedVersion

```solidity
function getInitializedVersion() external view returns (uint64)
```

### isInitializing

```solidity
function isInitializing() external view returns (bool)
```

## MockInitializableDisabled

### value

```solidity
uint256 public value
```

### constructor

```solidity
constructor()
```

### initialize

```solidity
function initialize(uint256 _value) external initializer
```

## MockInitializableChild

### childValue

```solidity
uint256 public childValue
```

### parentValue

```solidity
uint256 public parentValue
```

### initialize

```solidity
function initialize(uint256 _parent, uint256 _child) external initializer
```

### \_initParent

```solidity
function _initParent(uint256 _val) internal onlyInitializing
```
