<!-- Source: https://docs.paxeer.app/debugging-transactions/ -->

# Debug Transactions

Diagnose reverted or missing Paxeer transactions with receipts, calldata, gas checks, traces, and reproducible local tests.

Begin with the transaction hash, chain ID, and the endpoint that accepted the submission. Separate transport failures, rejected submissions, and transactions that executed with a reverted receipt; they require different next steps.

## Collect the transaction and receipt

Set the shell placeholders `PAXEER_RPC_URL` and `TX_HASH`, then use Cast to inspect the recorded transaction. These commands only query the endpoint.

```
cast chain-id --rpc-url "$PAXEER_RPC_URL"
cast tx "$TX_HASH" --rpc-url "$PAXEER_RPC_URL" --json
cast receipt "$TX_HASH" --rpc-url "$PAXEER_RPC_URL" --json
```

| Observation | Meaning | Next check |
| --- | --- | --- |
| No transaction or receipt | The endpoint cannot find the hash | Confirm chain 125, the submission response, and whether the provider has indexed that height |
| Transaction present, receipt absent | Execution is not yet visible through this endpoint | Check a known-good endpoint and the sender nonce before resubmitting |
| Receipt status 0 | The transaction was included but execution reverted | Inspect the ABI, revert data, and execution trace |
| Receipt status 1 | Execution succeeded | Compare emitted events and resulting application state with the intended action |

A receipt is final when its block is committed on the BFT network. A timeout after submission does not prove the transaction failed. Search for the original hash first; a new transaction can repeat the application action.

## Check the sender and call

Read the sender’s native balance, nonce, and the current fee estimate. Set `SENDER_ADDRESS` to the sender recorded in the transaction and `CONTRACT_ADDRESS` to its target. The calldata example uses `store(uint256)` from `contracts/src/Box.sol`.

```
cast balance "$SENDER_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast nonce "$SENDER_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast gas-price --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast calldata 'store(uint256)' 42
```

Compare the encoded calldata to the transaction input. Confirm the contract address and ABI against the deployed version. PAX pays gas even when execution reverts. Increasing the gas limit will not repair a failed authorization check or invalid function argument.

## Simulate the exact inputs

For the Box example, reproduce a call with the original sender and arguments. `cast call` uses `eth_call`; it does not persist storage changes.

```
cast call "$CONTRACT_ADDRESS" 'store(uint256)' 42   --from "$SENDER_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
```

A call at `latest` tests current state. It may differ from the state at the time of the failed transaction. A call at the previous block is also only an approximation when earlier transactions in the same block changed relevant state.

## Trace historical execution

If your provider enables the debug namespace and retains the required state, request a call trace. Inspect the failed frame, its input, revert data, and nested calls.

```
cast rpc debug_traceTransaction "$TX_HASH"   '{"tracer":"callTracer","timeout":"30s"}'   --rpc-url "$PAXEER_RPC_URL"
```

Provider policies and node versions determine available tracers. The node accepts `callTracer`, `prestateTracer`, and `flatCallTracer`. For a `-32601` response, use an endpoint with the debug namespace enabled. For a historical-state error, use a node retaining the requested height. See [Paxeer debug methods](https://docs.paxeer.app/json-rpc).

## Reproduce in a local test

Write a minimal test using the exact contract version, sender, and arguments. For ordinary Solidity state, a fork pinned to a known block can help. Set `BLOCK_NUMBER` to that height and `ANVIL_RPC_URL` to your local Anvil endpoint:

```
anvil --fork-url "$PAXEER_RPC_URL" --fork-block-number "$BLOCK_NUMBER"

# In another terminal, read the local fork.
cast call "$CONTRACT_ADDRESS" 'retrieve()(uint256)'   --rpc-url "$ANVIL_RPC_URL"
```

Use Anvil for ordinary Solidity contract state. For Cosmos modules, Paxeer precompiles, parallel scheduling, or consensus behavior, reproduce the issue with the matching Paxeer binary in a [development cluster](https://docs.paxeer.app/docker). The source RPC must retain the historical state needed by your fork.

## Keep a useful bug report

- Transaction hash, chain ID, block number, and receipt status.

- Contract address, exact source revision, ABI, compiler version, and optimizer settings.

- Original sender, function arguments, value, gas parameters, and expected state transition.

- Decoded error or failing trace frame and a minimal reproduction.

- Provider type and node version when available; remove authentication tokens from URLs and logs.

Reference: [Foundry Cast](https://getfoundry.sh/cast/overview/). Continue with [parallel execution design](https://docs.paxeer.app/parallelization) when the issue is contention or throughput rather than a revert.

**Paxeer source:** `rpc/` — tracers.go (DebugAPI), tx.go (TransactionAPI), and simulate.go (SimulationAPI).
