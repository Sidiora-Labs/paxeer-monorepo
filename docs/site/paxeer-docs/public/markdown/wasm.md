<!-- Source: https://docs.paxeer.app/wasm/ -->

# WASM

CosmWasm contract lifecycle, deployment permissions, and calls between Paxeer’s EVM and WebAssembly runtimes.

**Source:** `Paxeer-X-Network/wasm/x/wasm/`, `precompiles/wasmd/`, and `node/wasmbinding/`.

## Runtime and module

Paxeer includes the CosmWasm module for storing WebAssembly code, instantiating contracts, executing messages, querying state, and performing authorized migrations. The [WASM runtime](https://docs.paxeer.app/wasm-runtime) executes compiled contract bytecode; the module supplies storage, account, bank, and message integration.

Contract authors compile Rust to WebAssembly before upload. Contract messages and queries use the individual contract's JSON schema. Native coin amounts use base-denomination integers, such as `uhpx`.

## Check deployment permissions

Upload access and default instantiation access are separate parameters. Query the network's current settings before deployment. Restricting new code upload does not by itself disable execution of existing contracts.

```
# Use the Cosmos RPC endpoint configured for your target node.
paxd query params subspace wasm uploadAccess
paxd query params subspace wasm instantiateAccess
```

These are the parameter keys in `wasm/x/wasm/types/params.go`. A particular code ID also carries its instantiation permissions. Check those permissions and the contract's administrator before an instantiate or migrate transaction.

## Contract lifecycle

Set your node endpoint, chain ID, and signer in the CLI configuration first. In these command templates, `INIT_JSON`, `EXECUTE_JSON`, and `QUERY_JSON` contain valid JSON matching your contract; `CODE_ID` and `CONTRACT` come from confirmed transaction results.

```
# Upload a compiled contract and record its confirmed code_id.
paxd tx wasm store contract.wasm --from deployer --gas auto

# Choose the administrator explicitly, or use --no-admin for no administrator.
paxd tx wasm instantiate "$CODE_ID" "$INIT_JSON" \
  --label "My contract" --admin "$ADMIN" --from deployer --gas auto

# Execute an application message, then read the contract state.
paxd tx wasm execute "$CONTRACT" "$EXECUTE_JSON" --from caller --gas auto
paxd query wasm contract-state smart "$CONTRACT" "$QUERY_JSON"

# An authorized administrator can migrate to an uploaded replacement code ID.
paxd tx wasm migrate "$CONTRACT" "$NEW_CODE_ID" "$MIGRATE_JSON" \
  --from admin --gas auto
```

`--amount` attaches native coins when a transaction requires them. Use denomination amounts such as `1000000uhpx`, not a decimal display amount or an EVM wei amount.

## Call CosmWasm from the EVM

The precompile at `0x0000000000000000000000000000000000001002` exposes `instantiate`, `execute`, `execute_batch`, and `query`. Load `precompiles/wasmd/abi.json` from the node revision you target.

Message and coin-list arguments are UTF-8 JSON encoded as bytes. An empty coin list is the JSON text `[]`, not empty bytes. CosmWasm targets use their `pax1...` contract addresses.

```
import { Contract, parseEther, toUtf8Bytes, toUtf8String } from 'ethers'

// signer is your connected signer; wasmdAbi is the matching abi.json.
const wasm = new Contract(
  '0x0000000000000000000000000000000000001002', wasmdAbi, signer,
)

// Application-specific JSON comes from the target contract's schema.
const queryResult = await wasm.query(contractAddress, toUtf8Bytes(JSON.stringify(queryMessage)))
const decodedJson = JSON.parse(toUtf8String(queryResult))

// 1 PAX = 1,000,000 uhpx = 10^18 wei.
const coins = toUtf8Bytes(JSON.stringify([{ denom: 'uhpx', amount: '1000000' }]))
const tx = await wasm.execute(
  contractAddress,
  toUtf8Bytes(JSON.stringify(executeMessage)),
  coins,
  { value: parseEther('1') },
)
await tx.wait()
```

This execution example is a template for a contract message that accepts that payment. The JSON coin list must include the `uhpx` funded through EVM value: `value = uhpx amount × 10^12`. Omitting it or attaching a mismatched value reverts with `coin amount must equal value specified`. For `execute_batch`, the sum of the coin amounts across all calls must equal the value, and no value may remain after execution.

Query returns raw contract response bytes; decode their hex representation before JSON parsing when the contract returns JSON. Execution may return arbitrary bytes. Transaction receipt log data is not automatically the function's return data; inspect documented events or query the resulting contract state.

## Call the EVM from CosmWasm

Paxeer implements custom `call_evm` and `delegate_call_evm` messages, EVM static queries, token interfaces, and address lookups through [WASM bindings](https://docs.paxeer.app/wasmbinding).

The implementation places additional limits on nested execution. Delegatecalling `instantiate` is refused outright. On the execution paths, a delegatecall is admitted only from the registered ERC-20, ERC-721, or ERC-1155 pointer of the target CosmWasm contract; any other calling contract is refused as not a pointer of that contract. Ordinary wrappers should use normal calls.

## Messages, storage, and gas

Standard CosmWasm messages handle bank transfers, staking, distribution, and other contract calls; custom bindings add Paxeer-specific module operations. Each contract has isolated byte-keyed storage accessed through host callbacks. Runtime computation and host work consume gas, including when an EVM contract makes a CosmWasm query on-chain.

The retained native Oracle query route and the retired EVM Oracle precompile are separate interfaces. Read the [Oracle module guide](https://docs.paxeer.app/modules/oracle) before choosing a price-data integration.

## Implementation references

- `wasm/x/wasm/client/cli/tx.go` and `query.go`: exact transaction and query commands.

- `wasm/x/wasm/types/params.go`: upload and instantiation permission types.

- `precompiles/wasmd/wasmd.go`: payment equality, context checks, batch execution, and return data.

- `node/wasmbinding/encoder.go`: custom messages routed from CosmWasm.
