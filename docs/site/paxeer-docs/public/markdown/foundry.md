<!-- Source: https://docs.paxeer.app/foundry/ -->

# Build with Foundry

Create, test, and deploy a Solidity counter with Forge, then read its state through the Paxeer EVM using Cast.

Use this guide for a new Solidity project. Forge compiles and tests contracts; Cast reads chain data and sends signed transactions. The examples target [Paxeer chain ID 125](https://docs.paxeer.app/network-parameters) (Cosmos chain `hyperpax_125-1`), whose native gas token is PAX. Other networks map differently — for example `pacific-1` is 1329 and an unmapped local chain defaults to 713714 — so substitute your endpoint's actual chain ID where 125 appears below.

## Set up the project

Install Forge and Cast using the [Foundry installation instructions](https://getfoundry.sh/introduction/installation/), then create a project. Keep the generated project files; add the named files below alongside the scaffold.

```
forge init paxeer-counter
cd paxeer-counter
forge --version
cast --version
```

Use this `foundry.toml` for the example. It matches the in-tree toolchain: solc 0.8.27 with the Prague EVM target, which the chain enables from genesis along with Cancun. The repository's own `foundry.paxeer.toml` pins solc 0.8.27 without an explicit EVM version or optimizer run count; the run count below is an example choice, so record whatever you deploy with.

```
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc_version = "0.8.27"
evm_version = "prague"
optimizer = true
optimizer_runs = 200

[rpc_endpoints]
paxeer = "${PAXEER_RPC_URL}"
```

## Write the contract

Create `src/PaxeerCounter.sol`. Anyone can update this demonstration counter; it is useful for checking storage, events, and frontend reads.

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

## Test local behavior

Create `test/PaxeerCounter.t.sol`. These tests run the contract locally in Forge, using an isolated test environment.

```
// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import {Test} from "forge-std/Test.sol";
import {PaxeerCounter} from "../src/PaxeerCounter.sol";

contract PaxeerCounterTest is Test {
    PaxeerCounter private counter;

    function setUp() public {
        counter = new PaxeerCounter();
    }

    function test_Increment() public {
        counter.increment();
        assertEq(counter.number(), 1);
    }

    function testFuzz_SetNumber(uint256 next) public {
        counter.setNumber(next);
        assertEq(counter.number(), next);
    }
}
```

```
forge build
forge test --match-contract PaxeerCounterTest -vv
forge test --match-contract PaxeerCounterTest --gas-report
```

The expected result is two passing tests, including the generated fuzz cases. Use the gas report to compare the cost of individual contract calls. Measure network throughput separately with a Paxeer development cluster.

## Connect and rehearse deployment

Set `PAXEER_RPC_URL` to your EVM HTTP endpoint. This is different from a Cosmos RPC endpoint. Confirm its chain ID before selecting a signer. Import a development account into Foundry’s encrypted keystore when prompted.

```
: "${PAXEER_RPC_URL:?Set your Paxeer EVM RPC URL}"
test "$(cast chain-id --rpc-url "$PAXEER_RPC_URL")" = 125 || { echo "Expected Paxeer chain ID 125" >&2; exit 1; }
cast wallet import paxeer-dev --interactive
forge create src/PaxeerCounter.sol:PaxeerCounter   --rpc-url "$PAXEER_RPC_URL" --account paxeer-dev
```

Run `forge create` first without `--broadcast` to rehearse, then review the destination chain, sender, and estimated cost before broadcasting. The signer needs PAX for an actual Paxeer deployment. For a local development network, use its actual chain ID and local funds.

## Publish and read the result

The next command broadcasts a contract-creation transaction. Save its address and transaction hash from the output.

```
forge create src/PaxeerCounter.sol:PaxeerCounter   --rpc-url "$PAXEER_RPC_URL" --account paxeer-dev --broadcast

# Set CONTRACT_ADDRESS and TX_HASH from the deployment output.
cast receipt "$TX_HASH" --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast call "$CONTRACT_ADDRESS" 'number()(uint256)'   --rpc-url "$PAXEER_RPC_URL"
```

Confirm a successful receipt, nonempty deployed bytecode, and an initial counter value of zero. The contract ABI is in `out/PaxeerCounter.sol/PaxeerCounter.json`. Use that artifact in your [frontend](https://docs.paxeer.app/building-a-frontend); keep the compiler version and optimizer settings with your deployment record.

Reference: [Foundry deployment and verification](https://getfoundry.sh/forge/deploying/). For explorer verification, select a verifier that supports your Paxeer endpoint and submit the same compiler settings used for deployment.

**Paxeer source:** `Paxeer-X-Network/` — contracts/hardhat.config.js (Solidity 0.8.27, Prague), foundry.paxeer.toml (solc 0.8.27), modules/evm/config/config.go (ChainIDMapping), and rpc/send.go (SendRawTransaction).
