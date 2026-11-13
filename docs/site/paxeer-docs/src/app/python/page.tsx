import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/python')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Python Quickstart</h1>
        <p className="page-description">Connect to Paxeer with web3.py, read PAX balances and ERC-20 state, and inspect transaction receipts without a signing key.</p>
      </div>
      <p>Python is useful for application backends, data analysis, and operational scripts. This guide reads the Paxeer EVM through HTTP using web3.py. Every example is read-only and expects EVM chain ID 125, the mapping for <code>hyperpax_125-1</code>. The node also defaults unmapped chain identifiers to 125, so confirm the intended network and its genesis independently of this numeric check.</p>
      <h2>Install in an isolated environment</h2>
      <p>These examples use web3.py 7 as a client dependency in your own Python project. Create a virtual environment and keep the resulting dependency versions with that project.</p>
      <pre><code className="language-bash">{"python3 -m venv .venv\nsource .venv/bin/activate\npython -m pip install \"web3>=7,<8\""}</code></pre>
      <p>On Windows PowerShell, activate with <code>.venv\Scripts\Activate.ps1</code>. Supply <code>PAXEER_RPC_URL</code> and <code>ACCOUNT_ADDRESS</code> through your shell or deployment environment. These variables are inputs to this example, not node configuration keys; the RPC URL uses the node endpoint’s root path.</p>
      <h2>Read the chain and account</h2>
      <p>Save this as <code>read_paxeer.py</code>, then run <code>python read_paxeer.py</code>. It validates the endpoint, captures one block, and reads the account at that height.</p>
      <pre><code className="language-python">{"import os\nfrom web3 import Web3\n\nw3 = Web3(Web3.HTTPProvider(\n    os.environ[\"PAXEER_RPC_URL\"],\n    request_kwargs={\"timeout\": 30},\n))\nif not w3.is_connected():\n    raise RuntimeError(\"The EVM RPC endpoint did not respond\")\nif w3.eth.chain_id != 125:\n    raise RuntimeError(\"Expected Paxeer EVM chain ID 125\")\n\naccount = Web3.to_checksum_address(os.environ[\"ACCOUNT_ADDRESS\"])\nheight = w3.eth.block_number\nbalance = w3.eth.get_balance(account, block_identifier=height)\nnonce = w3.eth.get_transaction_count(account, block_identifier=height)\n\nprint(\"Chain ID:\", w3.eth.chain_id)\nprint(\"Block:\", height)\nprint(\"Balance (PAX):\", w3.from_wei(balance, \"ether\"))\nprint(\"Nonce:\", nonce)"}</code></pre>
      <p><code>from_wei(..., "ether")</code> names an 18-decimal unit conversion in web3.py. On Paxeer the native currency being displayed is PAX. Use decimal values or integer base units for amounts; avoid converting balances through floating-point numbers.</p>
      <h2>Read an ERC-20 balance</h2>
      <p>Set <code>TOKEN_ADDRESS</code> and append this block to the script. The ABI is intentionally limited to the two methods being called.</p>
      <pre><code className="language-python">{"from decimal import Decimal, localcontext\n\nabi = [\n    {\n        \"type\": \"function\", \"name\": \"decimals\", \"stateMutability\": \"view\",\n        \"inputs\": [], \"outputs\": [{\"name\": \"\", \"type\": \"uint8\"}],\n    },\n    {\n        \"type\": \"function\", \"name\": \"balanceOf\", \"stateMutability\": \"view\",\n        \"inputs\": [{\"name\": \"owner\", \"type\": \"address\"}],\n        \"outputs\": [{\"name\": \"\", \"type\": \"uint256\"}],\n    },\n]\ntoken_address = Web3.to_checksum_address(os.environ[\"TOKEN_ADDRESS\"])\nif not w3.eth.get_code(token_address, block_identifier=height):\n    raise RuntimeError(\"No contract exists at TOKEN_ADDRESS at this block\")\ntoken = w3.eth.contract(address=token_address, abi=abi)\ndecimals = token.functions.decimals().call(block_identifier=height)\nraw_balance = token.functions.balanceOf(account).call(block_identifier=height)\nwith localcontext() as context:\n    context.prec = 100\n    print(\"Token balance:\", Decimal(raw_balance) / (Decimal(10) ** decimals))"}</code></pre>
      <p>Use the token’s own decimals rather than PAX’s decimals. A missing method or unexpected return encoding indicates an ABI mismatch; obtain the ABI from the contract build or its verified deployment record.</p>
      <h2>Inspect a transaction receipt</h2>
      <p>Set <code>TX_HASH</code> to a transaction hash from the same network. Append this block to inspect execution status. A missing receipt can mean an unknown transaction or an unavailable indexed result. The node can expose a receipt from the open block, so use <code>pax_sequencerReceipt</code> and require its <code>committed</code> rung when finality matters.</p>
      <pre><code className="language-python">{"from web3.exceptions import TransactionNotFound\n\ntry:\n    receipt = w3.eth.get_transaction_receipt(os.environ[\"TX_HASH\"])\nexcept TransactionNotFound:\n    print(\"No receipt returned. Verify the network and transaction hash before retrying.\")\nelse:\n    print(\"Execution:\", \"success\" if receipt[\"status\"] == 1 else \"reverted\")\n    print(\"Block:\", receipt[\"blockNumber\"])\n    print(\"Gas used:\", receipt[\"gasUsed\"])\n    print(\"Logs:\", len(receipt[\"logs\"]))"}</code></pre>
      <h2>Use the result in an application</h2>
      <ul><li>Show an unavailable state when reads time out and offer a retry.</li><li>For repeated queries, preserve block numbers in your output so users can judge freshness.</li><li>Use bounded event ranges for indexers, following the <Link href="/querying-evm">querying guide</Link>.</li><li>For transaction signing, add an explicit account-selection and transaction-review step.</li></ul>
      <p>References: <a href="https://web3py.readthedocs.io/en/v7.14.0/quickstart.html">web3.py 7 quickstart</a> and <a href="https://web3py.readthedocs.io/en/v7.14.0/contracts.html">contract calls</a>.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>rpc/state.go</code> (GetBalance and GetCode), <code>rpc/tx.go</code> (GetTransactionReceipt), <code>rpc/sequencer_receipt.go</code> (receipt rungs), and <code>modules/evm/config/config.go</code> (ChainIDMapping and DefaultChainID).</div>
      <div className="prev-next">
        <Link href="/querying-evm"><div className="prev-next-label">Previous</div><div className="prev-next-title">Query the EVM</div></Link>
        <Link href="/debugging-transactions"><div className="prev-next-label">Next</div><div className="prev-next-title">Debug Transactions</div></Link>
      </div>
    </DocsLayout>
  )
}
