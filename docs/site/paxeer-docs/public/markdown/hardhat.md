<!-- Source: https://docs.paxeer.app/hardhat/ -->

# Build with Hardhat

Use the repository's Hardhat 2 project to compile contracts and interact with a development node.

The contract project at `contracts/` uses Hardhat 2 and a CommonJS configuration. Its sources live under `contracts/src/`. Paxeer EVM chain ID is `125`, mapped from `hyperpax_125-1` in `modules/evm/config/config.go`.

## Use the in-tree project

Use a [Node.js version supported by Hardhat 2](https://hardhat.org/hardhat2). Install the dependencies recorded in `contracts/package-lock.json` and inspect the local tool version.

```
cd contracts
npm ci
npx hardhat --version
```

`contracts/package.json` declares `hardhat ^2.20.1`, `@nomicfoundation/hardhat-toolbox ^6.1.2`, and `@openzeppelin/hardhat-upgrades ^3.0.2`. The lockfile records the resolved dependency versions.

## Configure the development endpoint

`contracts/hardhat.config.js` selects Solidity `0.8.27`, the `prague` target, and an enabled optimizer with `1000` runs. Its network configuration reads `PAXEER_LOCAL_EVM_RPC_URL`. The following excerpt shows the compiler and source settings.

```
module.exports = {
  solidity: {
    version: "0.8.27",
    settings: {
      evmVersion: "prague",
      optimizer: { enabled: true, runs: 1000 },
    },
  },
  paths: { sources: "./src" },
};
```

The configured HTTP network is named `paxlocal`. Set `PAXEER_LOCAL_EVM_RPC_URL` to your development node's root EVM endpoint before connecting. The default uses the loopback interface on port `8545`.

The configuration loads `contracts/.env` through `dotenv` and reads the endpoint from the environment. The following command requires that endpoint to be set.

```
: "${PAXEER_LOCAL_EVM_RPC_URL:?Set the development node EVM endpoint}"
```

## Read sources and run the compiler check

`contracts/src/Box.sol` is an existing storage example. It exposes `store`, `retrieve`, and `boxIncr` and emits `ValueChanged`.

```
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Box {
    uint256 private _value;

    event ValueChanged(uint256 value);

    function store(uint256 value) public {
        _value = value;
        emit ValueChanged(value);
    }

    function retrieve() public view returns (uint256) {
        return _value;
    }

    function boxIncr() public {
        _value = _value + 1;
        emit ValueChanged(_value);
    }
}
```

Token contracts use the installed OpenZeppelin package. For example, this is the import and constructor in `contracts/src/TestToken.sol`.

```
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract TestToken is ERC20, Ownable {
    constructor(string memory name, string memory symbol) Ownable(msg.sender) ERC20(name, symbol) {
        _mint(msg.sender, 1000 * (10 ** uint256(decimals())));
    }

    // The owner adjusts balances by minting or burning
    function setBalance(address account, uint256 amount) public onlyOwner {
        uint256 currentBalance = balanceOf(account);
        if (amount > currentBalance) {
            _mint(account, amount - currentBalance);
        } else if (amount < currentBalance) {
            _burn(account, currentBalance - amount);
        }
    }
}
```

```
npx hardhat compile
node --test test/hardhat_solc_parity.test.js
```

The focused Node test `contracts/test/hardhat_solc_parity.test.js` checks that the Hardhat compiler version matches `foundry.paxeer.toml`. Run it from `contracts/` after compiling.

## Interact through the console

In the Hardhat console, the following Ethers calls deploy the existing Box artifact, wait for deployment, write a value, and read it. Deployment and writes require a signer exposed by the development node.

```
const box = await ethers.deployContract("Box");
await box.waitForDeployment();
const tx = await box.store(7);
const receipt = await tx.wait();
await box.retrieve();
```

Open the console with the configured network. Confirm that it is the intended development node before running the deployment expressions above.

```
npx hardhat console --network paxlocal
```

## Check the connected chain

In that console, inspect the endpoint's chain ID before submitting transactions. Paxeer uses `125`. Deployment and storage writes spend the configured signer's gas balance.

```
const network = await ethers.provider.getNetwork();
network.chainId;
```

Retain the contract address, transaction receipt, source, and compiler settings. Read the result through the [querying guide](https://docs.paxeer.app/querying-evm). Hardhat writes the Box artifact to `contracts/artifacts/src/Box.sol/Box.json`.

`contracts/package.json` declares `@openzeppelin/contracts ^5.4.0`. Its `TestToken` source imports `@openzeppelin/contracts/token/ERC20/ERC20.sol`.

References: [Hardhat 2 configuration](https://hardhat.org/hardhat2/hardhat-runner/docs/config), [contract tests](https://hardhat.org/hardhat2/hardhat-runner/docs/guides/test-contracts), and [contract deployment](https://hardhat.org/hardhat2/hardhat-runner/docs/guides/deploying).

**Paxeer source:** `contracts/hardhat.config.js`, `contracts/package.json`, `contracts/src/Box.sol`, `contracts/src/TestToken.sol`, and `contracts/test/hardhat_solc_parity.test.js`.
