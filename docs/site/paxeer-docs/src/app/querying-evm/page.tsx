import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/querying-evm')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Query the EVM</h1>
        <p className="page-description">Read Paxeer balances, contract state, receipts, and bounded event ranges with JSON-RPC, Cast, and viem.</p>
      </div>
      <p>Read-only queries work without a signer or a PAX balance. Choose an EVM JSON-RPC endpoint, confirm its network, and record the block number when several reads must describe the same state.</p>
      <h2>Verify the endpoint</h2>
      <p>Set <code>PAXEER_RPC_URL</code> to your EVM endpoint. For <code>hyperpax_125-1</code> the result of this request must be <code>0x7d</code>, which is decimal 125. Other networks return their own ID (<code>pacific-1</code> is 1329; unmapped local chains default to 713714); the examples below assume 125, so adjust the checks to your endpoint&apos;s value.</p>
      <pre><code className="language-bash">{": \"${PAXEER_RPC_URL:?Set your Paxeer EVM RPC URL}\"\ncurl --fail-with-body --silent --show-error \"$PAXEER_RPC_URL\"   -H 'Content-Type: application/json'   --data '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_chainId\",\"params\":[]}'"}</code></pre>
      <p>HTTP 200 can still contain a JSON-RPC <code>error</code>. Check both the HTTP result and the response body. A Cosmos RPC server exposes routes such as <code>/status</code>; it is not interchangeable with the EVM service.</p>
      <h2>Inspect accounts and contracts with Cast</h2>
      <p>Set <code>ACCOUNT_ADDRESS</code> and <code>CONTRACT_ADDRESS</code> to addresses from your application or deployment records. Use an explicit function signature to encode and decode a call.</p>
      <pre><code className="language-bash">{"cast block-number --rpc-url \"$PAXEER_RPC_URL\"\ncast balance \"$ACCOUNT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast nonce \"$ACCOUNT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast code \"$CONTRACT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast call \"$CONTRACT_ADDRESS\" 'number()(uint256)'   --rpc-url \"$PAXEER_RPC_URL\"\ncast calldata 'setNumber(uint256)' 42"}</code></pre>
      <p>The final command only encodes calldata. It does not submit a transaction. <code>cast balance</code> returns native base units by default. A missing contract, wrong ABI, or unavailable historical state can make a read fail even when the RPC endpoint is healthy.</p>
      <h2>Read token data at one block</h2>
      <p>Install <code>viem</code> in a Node project and save this as <code>read-token.mjs</code>. Set <code>TOKEN_ADDRESS</code> and <code>ACCOUNT_ADDRESS</code> to the EVM addresses you want to query, then run <code>node read-token.mjs</code>.</p>
      <pre><code className="language-javascript">{"import { createPublicClient, formatUnits, getAddress, http, parseAbi } from \"viem\";\n\nconst rpcUrl = process.env.PAXEER_RPC_URL;\nif (!rpcUrl) throw new Error(\"Set PAXEER_RPC_URL\");\nconst token = getAddress(process.env.TOKEN_ADDRESS ?? \"\");\nconst account = getAddress(process.env.ACCOUNT_ADDRESS ?? \"\");\nconst client = createPublicClient({ transport: http(rpcUrl) });\nif (await client.getChainId() !== 125) throw new Error(\"Unexpected chain ID\");\n\nconst blockNumber = await client.getBlockNumber();\nconst abi = parseAbi([\n  \"function decimals() view returns (uint8)\",\n  \"function symbol() view returns (string)\",\n  \"function balanceOf(address owner) view returns (uint256)\",\n]);\nconst [decimals, symbol, balance] = await Promise.all([\n  client.readContract({ address: token, abi, functionName: \"decimals\", blockNumber }),\n  client.readContract({ address: token, abi, functionName: \"symbol\", blockNumber }),\n  client.readContract({ address: token, abi, functionName: \"balanceOf\", args: [account], blockNumber }),\n]);\nconsole.log({ block: blockNumber.toString(), symbol, balance: formatUnits(balance, decimals) });"}</code></pre>
      <p>The requests run concurrently at the same block using ordinary contract calls. For tokens with a custom metadata interface, replace the example ABI with the contract’s published ABI.</p>
      <h2>Fetch events in bounded ranges</h2>
      <p>Save the following as <code>read-transfers.mjs</code>. Provide the token address, RPC URL, and an inclusive <code>FROM_BLOCK</code>/<code>TO_BLOCK</code> range. Start at the contract’s deployment block when building a full history.</p>
      <pre><code className="language-javascript">{"import { createPublicClient, getAddress, http, parseAbiItem } from \"viem\";\n\nconst rpcUrl = process.env.PAXEER_RPC_URL;\nif (!rpcUrl || !process.env.FROM_BLOCK || !process.env.TO_BLOCK) {\n  throw new Error(\"Set PAXEER_RPC_URL, FROM_BLOCK and TO_BLOCK\");\n}\nconst address = getAddress(process.env.TOKEN_ADDRESS ?? \"\");\nconst client = createPublicClient({ transport: http(rpcUrl) });\nif (await client.getChainId() !== 125) throw new Error(\"Unexpected chain ID\");\nconst first = BigInt(process.env.FROM_BLOCK);\nconst last = BigInt(process.env.TO_BLOCK);\nif (first < 0n || last < first) throw new Error(\"Invalid block range\");\nconst event = parseAbiItem(\"event Transfer(address indexed from, address indexed to, uint256 value)\");\nconst chunkSize = 500n;\n\nfor (let fromBlock = first; fromBlock <= last; fromBlock += chunkSize) {\n  const toBlock = fromBlock + chunkSize - 1n < last ? fromBlock + chunkSize - 1n : last;\n  const logs = await client.getLogs({ address, event, fromBlock, toBlock, strict: true });\n  for (const log of logs) {\n    console.log(JSON.stringify(log, (_, value) => typeof value === \"bigint\" ? value.toString() : value));\n  }\n}"}</code></pre>
      <p>Start with 500-block chunks and reduce the size if your provider returns a range or result-limit error. The node&apos;s default maximum <code>eth_getLogs</code> range is 2000 blocks, so 500-block chunks stay within it unless an operator configures a lower limit. A production indexer should commit processed logs and its cursor together, resume only after successful chunks, and deduplicate by chain ID, block hash, transaction hash, and log index.</p>
      <h2>Keep transaction views consistent</h2>
      <p><code>eth_</code> methods provide the EVM view and exclude shell receipts and bank transfers. The <code>pax_</code> namespace additionally includes shell receipts, and <code>pax2_</code> additionally includes bank transfers, so the same block can list different transactions and indices per namespace; availability is configured by the node. Keep one namespace throughout an indexer and use hashes for lookup. See <Link href="/json-rpc">JSON-RPC namespaces</Link>.</p>
      <p>For historical reads, the provider must retain state at that height. A recent balance read can work while an older <code>eth_call</code> fails. For a submitted transaction, inspect the receipt’s <code>status</code>, not just whether a hash exists.</p>
      <p>References: <a href="https://viem.sh/docs/contract/readContract">viem contract reads</a>, <a href="https://viem.sh/docs/actions/public/getLogs">event logs</a>, and <a href="https://ethereum.org/en/developers/docs/apis/json-rpc/">Ethereum JSON-RPC</a>.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>Paxeer-X-Network/</code> — rpc/state.go (StateAPI), rpc/filter.go (FilterAPI), and rpc/tx.go (TransactionAPI).</div>
      <div className="prev-next">
        <Link href="/building-a-frontend"><div className="prev-next-label">Previous</div><div className="prev-next-title">Build a Frontend</div></Link>
        <Link href="/python"><div className="prev-next-label">Next</div><div className="prev-next-title">Python Quickstart</div></Link>
      </div>
    </DocsLayout>
  )
}
