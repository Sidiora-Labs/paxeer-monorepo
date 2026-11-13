<!-- Source: https://docs.paxeer.app/hardhat/ -->

# Build with Hardhat

Configure Hardhat 3 for Paxeer, test a Solidity contract locally, and keep repeatable deployments with Hardhat Ignition.

Hardhat brings Solidity compilation, tests, and deployment records into a TypeScript project. This walkthrough creates a new Hardhat 3 project with its Mocha/Ethers toolbox and an explicit HTTP network for Paxeer chain ID 125 (Cosmos chain `hyperpax_125-1`; local or unmapped chains report other IDs, defaulting to 713714). It is separate from the repository's in-tree contract project, which remains Hardhat 2 with a CommonJS config — see the note under Configure the network.

## Create a Hardhat 3 project

Use a [Node.js version supported by Hardhat 3](https://hardhat.org/docs/reference/nodejs-support). Run the initializer and choose the TypeScript project with Mocha and Ethers.js; install the dependencies it proposes.

```
mkdir paxeer-hardhat
cd paxeer-hardhat
npx hardhat@3 --init
npx hardhat --version
```

The scaffold includes ESM configuration, a TypeScript configuration, and the toolbox plugins. Retain its lockfile so another developer uses the same toolchain.

## Configure the network

In `hardhat.config.ts`, preserve any scaffold settings your project needs and use this network configuration. The compiler settings match the in-tree project (solc 0.8.27, Prague, which the chain enables from genesis); only the Hardhat 3 config shape is new.

```
import { configVariable, defineConfig } from "hardhat/config";
import hardhatToolboxMochaEthers from "@nomicfoundation/hardhat-toolbox-mocha-ethers";

export default defineConfig({
  plugins: [hardhatToolboxMochaEthers],
  solidity: {
    version: "0.8.27",
    settings: {
      evmVersion: "prague",
      optimizer: { enabled: true, runs: 1000 },
    },
  },
  networks: {
    local: { type: "edr-simulated", chainType: "l1" },
    paxeer: {
      type: "http",
      chainType: "l1",
      chainId: 125,
      url: configVariable("PAXEER_RPC_URL"),
      accounts: [configVariable("PAXEER_PRIVATE_KEY")],
    },
  },
});
```

For reference, the in-tree project at `contracts/` uses Hardhat 2 (`hardhat ^2.20.1`, toolbox `^4.0.0`) with sources in `./src` and a `paxlocal` network read from `PAXEER_LOCAL_EVM_RPC_URL` (default `http://127.0.0.1:8545`). Use that project as-is for chain contracts; use this guide only for a new standalone project.

Store your EVM endpoint and deployment key through the keystore plugin included in the toolbox. `configVariable` also resolves environment variables for CI and other automation.

```
npx hardhat keystore set PAXEER_RPC_URL
npx hardhat keystore set PAXEER_PRIVATE_KEY
```

## Add and test the contract

Create `contracts/PaxeerCounter.sol`. This is the same small contract used by the Foundry guide.

```
// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

contract PaxeerCounter {
    uint256 public number;
    event NumberChanged(address indexed caller, uint256 number);

    function setNumber(uint256 next) external {
        number = next;
        emit NumberChanged(msg.sender, next);
    }

    function increment() external {
        number += 1;
        emit NumberChanged(msg.sender, number);
    }
}
```

Create `contracts/PaxeerCounter.t.sol`. A failed `require` fails the test; this test deploys and exercises the contract within the simulated EVM.

```
// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import {PaxeerCounter} from "./PaxeerCounter.sol";

contract PaxeerCounterTest {
    function testIncrement() public {
        PaxeerCounter counter = new PaxeerCounter();
        counter.increment();
        require(counter.number() == 1, "increment did not persist");
    }

    function testSetNumber(uint256 next) public {
        PaxeerCounter counter = new PaxeerCounter();
        counter.setNumber(next);
        require(counter.number() == next, "value did not persist");
    }
}
```

```
npx hardhat build
npx hardhat test solidity
```

Hardhat treats the parameterized test as a fuzz test. Keep existing scaffold contracts compatible with your chosen compiler or remove the unused scaffold examples when you make the project your own.

## Describe the deployment

Create `ignition/modules/PaxeerCounter.ts`. Ignition records the deployment and can resume its own deployment plan when invoked again.

```
import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

export default buildModule("PaxeerCounterModule", (m) => {
  const counter = m.contract("PaxeerCounter");
  return { counter };
});
```

Exercise the deployment against the simulated network first. The simulated network uses local accounts and state.

```
npx hardhat ignition deploy ignition/modules/PaxeerCounter.ts --network local
```

## Deploy to your Paxeer endpoint

This command broadcasts using the configured signer and spends PAX for gas. Hardhat checks the endpoint against the configured chain ID. Review the deployment confirmation and retain the resulting address and transaction record.

```
npx hardhat ignition deploy ignition/modules/PaxeerCounter.ts --network paxeer
```

Keep `ignition/deployments/chain-125/`, your source revision, and compiler configuration together. Read the receipt and deployed contract through the [querying guide](https://docs.paxeer.app/querying-evm). For application tokens, configure the ERC-20 symbol and decimals independently of native PAX.

For token contracts, the standard library package is `@openzeppelin/contracts`. Use a release compatible with your Solidity version; resolve imports such as `@openzeppelin/contracts/token/ERC20/ERC20.sol` against that installed package.

References: [Hardhat network configuration](https://hardhat.org/docs/reference/configuration), [Solidity tests](https://hardhat.org/docs/guides/testing/using-solidity), and [Ignition deployments](https://hardhat.org/docs/guides/deployment/using-ignition).

**Paxeer source:** `Paxeer-X-Network/` — contracts/hardhat.config.js and contracts/package.json (in-tree Hardhat 2, solc 0.8.27/Prague/1000), modules/evm/config/config.go (ChainIDMapping), and rpc/send.go (SendRawTransaction). This guide configures a new Hardhat 3 project.
