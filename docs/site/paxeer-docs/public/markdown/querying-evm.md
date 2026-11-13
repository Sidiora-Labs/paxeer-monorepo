<!-- Source: https://docs.paxeer.app/querying-evm/ -->

# Query the EVM

Read Paxeer balances, contract state, receipts, and bounded event ranges with JSON-RPC, Cast, and viem.

Read-only queries work without a signer or a PAX balance. Choose an EVM JSON-RPC endpoint, confirm its network, and record the block number when several reads must describe the same state.

## Verify the endpoint

Set `PAXEER_RPC_URL` to your EVM endpoint. For `hyperpax_125-1` the result of this request must be `0x7d`, which is decimal 125. Other networks return their own ID (`pacific-1` is 1329; unmapped local chains default to 713714); the examples below assume 125, so adjust the checks to your endpoint's value.

```
: "${PAXEER_RPC_URL:?Set your Paxeer EVM RPC URL}"
curl --fail-with-body --silent --show-error "$PAXEER_RPC_URL"   -H 'Content-Type: application/json'   --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

HTTP 200 can still contain a JSON-RPC `error`. Check both the HTTP result and the response body. A Cosmos RPC server exposes routes such as `/status`; it is not interchangeable with the EVM service.

## Inspect accounts and contracts with Cast

Set `ACCOUNT_ADDRESS` and `CONTRACT_ADDRESS` to addresses from your application or deployment records. Use an explicit function signature to encode and decode a call.

```
cast block-number --rpc-url "$PAXEER_RPC_URL"
cast balance "$ACCOUNT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast nonce "$ACCOUNT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"
cast call "$CONTRACT_ADDRESS" 'number()(uint256)'   --rpc-url "$PAXEER_RPC_URL"
cast calldata 'setNumber(uint256)' 42
```

The final command only encodes calldata. It does not submit a transaction. `cast balance` returns native base units by default. A missing contract, wrong ABI, or unavailable historical state can make a read fail even when the RPC endpoint is healthy.

## Read token data at one block

Install `viem` in a Node project and save this as `read-token.mjs`. Set `TOKEN_ADDRESS` and `ACCOUNT_ADDRESS` to the EVM addresses you want to query, then run `node read-token.mjs`.

```
import { createPublicClient, formatUnits, getAddress, http, parseAbi } from "viem";

const rpcUrl = process.env.PAXEER_RPC_URL;
if (!rpcUrl) throw new Error("Set PAXEER_RPC_URL");
const token = getAddress(process.env.TOKEN_ADDRESS ?? "");
const account = getAddress(process.env.ACCOUNT_ADDRESS ?? "");
const client = createPublicClient({ transport: http(rpcUrl) });
if (await client.getChainId() !== 125) throw new Error("Unexpected chain ID");

const blockNumber = await client.getBlockNumber();
const abi = parseAbi([
  "function decimals() view returns (uint8)",
  "function symbol() view returns (string)",
  "function balanceOf(address owner) view returns (uint256)",
]);
const [decimals, symbol, balance] = await Promise.all([
  client.readContract({ address: token, abi, functionName: "decimals", blockNumber }),
  client.readContract({ address: token, abi, functionName: "symbol", blockNumber }),
  client.readContract({ address: token, abi, functionName: "balanceOf", args: [account], blockNumber }),
]);
console.log({ block: blockNumber.toString(), symbol, balance: formatUnits(balance, decimals) });
```

The requests run concurrently at the same block using ordinary contract calls. For tokens with a custom metadata interface, replace the example ABI with the contract’s published ABI.

## Fetch events in bounded ranges

Save the following as `read-transfers.mjs`. Provide the token address, RPC URL, and an inclusive `FROM_BLOCK`/`TO_BLOCK` range. Start at the contract’s deployment block when building a full history.

```
import { createPublicClient, getAddress, http, parseAbiItem } from "viem";

const rpcUrl = process.env.PAXEER_RPC_URL;
if (!rpcUrl || !process.env.FROM_BLOCK || !process.env.TO_BLOCK) {
  throw new Error("Set PAXEER_RPC_URL, FROM_BLOCK and TO_BLOCK");
}
const address = getAddress(process.env.TOKEN_ADDRESS ?? "");
const client = createPublicClient({ transport: http(rpcUrl) });
if (await client.getChainId() !== 125) throw new Error("Unexpected chain ID");
const first = BigInt(process.env.FROM_BLOCK);
const last = BigInt(process.env.TO_BLOCK);
if (first < 0n || last < first) throw new Error("Invalid block range");
const event = parseAbiItem("event Transfer(address indexed from, address indexed to, uint256 value)");
const chunkSize = 500n;

for (let fromBlock = first; fromBlock <= last; fromBlock += chunkSize) {
  const toBlock = fromBlock + chunkSize - 1n < last ? fromBlock + chunkSize - 1n : last;
  const logs = await client.getLogs({ address, event, fromBlock, toBlock, strict: true });
  for (const log of logs) {
    console.log(JSON.stringify(log, (_, value) => typeof value === "bigint" ? value.toString() : value));
  }
}
```

Start with 500-block chunks and reduce the size if your provider returns a range or result-limit error. The node's default maximum `eth_getLogs` range is 2000 blocks, so 500-block chunks stay within it unless an operator configures a lower limit. A production indexer should commit processed logs and its cursor together, resume only after successful chunks, and deduplicate by chain ID, block hash, transaction hash, and log index.

## Keep transaction views consistent

`eth_` methods provide the EVM view and exclude shell receipts and bank transfers. The `pax_` namespace additionally includes shell receipts, and `pax2_` additionally includes bank transfers, so the same block can list different transactions and indices per namespace; availability is configured by the node. Keep one namespace throughout an indexer and use hashes for lookup. See [JSON-RPC namespaces](https://docs.paxeer.app/json-rpc).

For historical reads, the provider must retain state at that height. A recent balance read can work while an older `eth_call` fails. For a submitted transaction, inspect the receipt’s `status`, not just whether a hash exists.

References: [viem contract reads](https://viem.sh/docs/contract/readContract), [event logs](https://viem.sh/docs/actions/public/getLogs), and [Ethereum JSON-RPC](https://ethereum.org/en/developers/docs/apis/json-rpc/).

**Paxeer source:** `Paxeer-X-Network/` — rpc/state.go (StateAPI), rpc/filter.go (FilterAPI), and rpc/tx.go (TransactionAPI).
