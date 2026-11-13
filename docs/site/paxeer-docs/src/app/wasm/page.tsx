import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/wasm')

export default function Wasm() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">WASM</h1>
        <p className="page-description">CosmWasm contract lifecycle, deployment permissions, and calls between Paxeer’s EVM and WebAssembly runtimes.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>Paxeer-X-Network/wasm/x/wasm/</code>, <code>precompiles/wasmd/</code>, and <code>node/wasmbinding/</code>.</div>
      <h2>Runtime and module</h2>
      <p>Paxeer includes the CosmWasm module for storing WebAssembly code, instantiating contracts, executing messages, querying state, and performing authorized migrations. The <Link href="/wasm-runtime">WASM runtime</Link> executes compiled contract bytecode; the module supplies storage, account, bank, and message integration.</p>
      <p>Contract authors compile Rust to WebAssembly before upload. Contract messages and queries use the individual contract&apos;s JSON schema. Native coin amounts use base-denomination integers, such as <code>uhpx</code>.</p>

      <h2>Check deployment permissions</h2>
      <p>Upload access and default instantiation access are separate parameters. Query the network&apos;s current settings before deployment. Restricting new code upload does not by itself disable execution of existing contracts.</p>
      <pre><code>{`# Use the Cosmos RPC endpoint configured for your target node.
paxd query params subspace wasm uploadAccess
paxd query params subspace wasm instantiateAccess`}</code></pre>
      <p>These are the parameter keys in <code>wasm/x/wasm/types/params.go</code>. A particular code ID also carries its instantiation permissions. Check those permissions and the contract&apos;s administrator before an instantiate or migrate transaction.</p>

      <h2>Contract lifecycle</h2>
      <p>Set your node endpoint, chain ID, and signer in the CLI configuration first. In these command templates, <code>INIT_JSON</code>, <code>EXECUTE_JSON</code>, and <code>QUERY_JSON</code> contain valid JSON matching your contract; <code>CODE_ID</code> and <code>CONTRACT</code> come from confirmed transaction results.</p>
      <pre><code>{`# Upload a compiled contract and record its confirmed code_id.
paxd tx wasm store contract.wasm --from deployer --gas auto

# Choose the administrator explicitly, or use --no-admin for no administrator.
paxd tx wasm instantiate "$CODE_ID" "$INIT_JSON" \\
  --label "My contract" --admin "$ADMIN" --from deployer --gas auto

# Execute an application message, then read the contract state.
paxd tx wasm execute "$CONTRACT" "$EXECUTE_JSON" --from caller --gas auto
paxd query wasm contract-state smart "$CONTRACT" "$QUERY_JSON"

# An authorized administrator can migrate to an uploaded replacement code ID.
paxd tx wasm migrate "$CONTRACT" "$NEW_CODE_ID" "$MIGRATE_JSON" \\
  --from admin --gas auto`}</code></pre>
      <p><code>--amount</code> attaches native coins when a transaction requires them. Use denomination amounts such as <code>1000000uhpx</code>, not a decimal display amount or an EVM wei amount.</p>

      <h2>Call CosmWasm from the EVM</h2>
      <p>The precompile at <code>0x0000000000000000000000000000000000001002</code> exposes <code>instantiate</code>, <code>execute</code>, <code>execute_batch</code>, and <code>query</code>. Load <code>precompiles/wasmd/abi.json</code> from the node revision you target.</p>
      <p>Message and coin-list arguments are UTF-8 JSON encoded as bytes. An empty coin list is the JSON text <code>[]</code>, not empty bytes. CosmWasm targets use their <code>pax1...</code> contract addresses.</p>
      <pre><code>{`import { Contract, parseEther, toUtf8Bytes, toUtf8String } from 'ethers'

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
await tx.wait()`}</code></pre>
      <p>This execution example is a template for a contract message that accepts that payment. The JSON coin list must include the <code>uhpx</code> funded through EVM value: <code>value = uhpx amount × 10^12</code>. Omitting it or attaching a mismatched value reverts with <code>coin amount must equal value specified</code>. For <code>execute_batch</code>, the sum of the coin amounts across all calls must equal the value, and no value may remain after execution.</p>
      <p>Query returns raw contract response bytes; decode their hex representation before JSON parsing when the contract returns JSON. Execution may return arbitrary bytes. Transaction receipt log data is not automatically the function&apos;s return data; inspect documented events or query the resulting contract state.</p>

      <h2>Call the EVM from CosmWasm</h2>
      <p>Paxeer implements custom <code>call_evm</code> and <code>delegate_call_evm</code> messages, EVM static queries, token interfaces, and address lookups through <Link href="/wasmbinding">WASM bindings</Link>.</p>
      <p>The implementation places additional limits on nested execution. Delegatecalling <code>instantiate</code> is refused outright. On the execution paths, a delegatecall is admitted only from the registered ERC-20, ERC-721, or ERC-1155 pointer of the target CosmWasm contract; any other calling contract is refused as not a pointer of that contract. Ordinary wrappers should use normal calls.</p>

      <h2>Messages, storage, and gas</h2>
      <p>Standard CosmWasm messages handle bank transfers, staking, distribution, and other contract calls; custom bindings add Paxeer-specific module operations. Each contract has isolated byte-keyed storage accessed through host callbacks. Runtime computation and host work consume gas, including when an EVM contract makes a CosmWasm query on-chain.</p>
      <p>The retained native Oracle query route and the retired EVM Oracle precompile are separate interfaces. Read the <Link href="/modules/oracle">Oracle module guide</Link> before choosing a price-data integration.</p>

      <h2>Implementation references</h2>
      <ul>
        <li><code>wasm/x/wasm/client/cli/tx.go</code> and <code>query.go</code>: exact transaction and query commands.</li>
        <li><code>wasm/x/wasm/types/params.go</code>: upload and instantiation permission types.</li>
        <li><code>precompiles/wasmd/wasmd.go</code>: payment equality, context checks, batch execution, and return data.</li>
        <li><code>node/wasmbinding/encoder.go</code>: custom messages routed from CosmWasm.</li>
      </ul>
      <div className="prev-next">
        <Link href="/precompiles"><div className="prev-next-label">Previous</div><div className="prev-next-title">Precompiles</div></Link>
        <Link href="/wasm-runtime"><div className="prev-next-label">Next</div><div className="prev-next-title">WASM Runtime</div></Link>
      </div>
    </DocsLayout>
  )
}
