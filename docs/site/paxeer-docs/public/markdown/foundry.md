<!-- Source: https://docs.paxeer.app/foundry/ -->

# Build with Foundry

Compile and test the in-tree Solidity contracts with Forge, then inspect a Paxeer deployment with Cast.

Forge builds the repository's Solidity projects. The Paxeer chain project uses `foundry.paxeer.toml`; the root `foundry.toml` selects a separate contract project. The [Paxeer EVM chain ID is 125](https://docs.paxeer.app/network-parameters) (`hyperpax_125-1`), and the native gas token is PAX.

## Select the chain project

Install Forge and Cast using the [Foundry installation instructions](https://getfoundry.sh/introduction/installation/). Run the following commands from the repository root. `contracts/bootstrap-libs.sh` fetches `forge-std v1.9.6` and `openzeppelin-contracts v5.3.0` into `contracts/lib/`. It reuses pristine pinned checkouts and replaces other contents at those dependency paths.

```
forge --version
cast --version
bash contracts/bootstrap-libs.sh
```

The chain configuration selects `contracts/src`, `contracts/test`, `contracts/out`, and Solidity `0.8.27`. The following excerpt is from `foundry.paxeer.toml`. Select it with `FOUNDRY_CONFIG`.

```
[profile.default]
src = "contracts/src"
out = "contracts/out"
test = "contracts/test"
libs = ["contracts/lib"]
remappings = [
    "forge-std/=contracts/lib/forge-std/src/",
    "@openzeppelin/contracts/=contracts/lib/openzeppelin-contracts/contracts/",
]
solc_version = "0.8.27"
auto_detect_solc = false
```

## Read the contract

`contracts/src/Box.sol` provides a small storage contract. Its `store`, `retrieve`, and `boxIncr` methods write, read, and increment a value. Its source is:

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

## Run the existing tests

`contracts/test/WPAXTest.t.sol` exercises the wrapped native-token contract. The following test function checks its name, symbol, and decimals; run the existing test contract with the commands below.

```
function testMetadataNamesTheCoin() public view {
    assertEq(wpax.name(), "Wrapped Paxeer");
    assertEq(wpax.symbol(), "WPAX");
    assertEq(wpax.decimals(), 18);
}
```

```
FOUNDRY_CONFIG=foundry.paxeer.toml forge build
FOUNDRY_CONFIG=foundry.paxeer.toml forge test --match-contract WPAXTest
FOUNDRY_CONFIG=foundry.paxeer.toml forge test --match-contract WPAXTest --gas-report
```

`WPAXTest` checks token metadata, storage slots, selectors, wrapping, withdrawals, and token transfers. A Forge gas report reports local execution costs; use network fee estimates when preparing a transaction.

## Connect and prepare deployment

Set `PAXEER_RPC_URL` to the node's root EVM HTTP endpoint. Confirm chain ID `125` and select an encrypted keystore account for deployment.

```
: "${PAXEER_RPC_URL:?Set the node EVM RPC endpoint}"
cast chain-id --rpc-url "$PAXEER_RPC_URL"
cast wallet import paxeer-dev --interactive
FOUNDRY_CONFIG=foundry.paxeer.toml forge create contracts/src/Box.sol:Box --rpc-url "$PAXEER_RPC_URL" --account paxeer-dev
```

Use `forge create` without `--broadcast` to prepare the deployment. Review the chain, sender, and fee estimate. An actual deployment requires sufficient PAX.

## Deploy and read the result

The next command broadcasts the existing Box contract. Retain the deployed contract address and transaction hash from its output.

```
FOUNDRY_CONFIG=foundry.paxeer.toml forge create contracts/src/Box.sol:Box --rpc-url "$PAXEER_RPC_URL" --account paxeer-dev --broadcast

# Set CONTRACT_ADDRESS and TX_HASH from the deployment output.
cast receipt "$TX_HASH" --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast call "$CONTRACT_ADDRESS" 'retrieve()(uint256)' --rpc-url "$PAXEER_RPC_URL"
```

Check receipt status, deployed bytecode, and `retrieve()`. A new Box starts at zero. The build artifact is `contracts/out/Box.sol/Box.json`. Use its ABI in your [frontend](https://docs.paxeer.app/building-a-frontend) and retain the compiler settings used for deployment.

Reference: [Foundry deployment and verification](https://getfoundry.sh/forge/deploying/). Verification uses the same source and compiler settings as the deployed bytecode.

**Paxeer source:** `foundry.paxeer.toml`, `contracts/src/Box.sol`, `contracts/test/WPAXTest.t.sol`, `modules/evm/config/config.go`, and `rpc/send.go`.
