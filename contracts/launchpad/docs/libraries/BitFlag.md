# Solidity API

## BitFlag

Bitwise operations for optical hook flags

Bits 0–4 are BEFORE_SWAP (1), AFTER_SWAP (2), BEFORE_FEE_DISTRIBUTION (4), AFTER_FEE_DISTRIBUTION
(8) and BEFORE_TOKEN_TRANSFER (16).

Source: [BitFlag.sol](../../contracts/libraries/BitFlag.sol).

### BEFORE_SWAP

```solidity
uint8 BEFORE_SWAP
```

### AFTER_SWAP

```solidity
uint8 AFTER_SWAP
```

### BEFORE_FEE_DISTRIBUTION

```solidity
uint8 BEFORE_FEE_DISTRIBUTION
```

### AFTER_FEE_DISTRIBUTION

```solidity
uint8 AFTER_FEE_DISTRIBUTION
```

### hasFlag

```solidity
function hasFlag(uint8 flags, uint8 flag) internal pure returns (bool)
```

Checks if a specific flag is set in the flags bitmap

### setFlag

```solidity
function setFlag(uint8 flags, uint8 flag) internal pure returns (uint8)
```

Sets a specific flag in the flags bitmap

### clearFlag

```solidity
function clearFlag(uint8 flags, uint8 flag) internal pure returns (uint8)
```

Clears a specific flag from the flags bitmap

### BEFORE_TOKEN_TRANSFER

```solidity
uint8 internal constant BEFORE_TOKEN_TRANSFER = 16
```

Bit 4 enables the token transfer hook.
