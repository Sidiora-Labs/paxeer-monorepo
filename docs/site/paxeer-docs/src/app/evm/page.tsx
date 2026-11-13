import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/evm')

export default function EVM() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">EVM Module</h1>
        <p className="page-description">
          Native EVM execution, address association, receipts, pointers, and precompile integration on chain ID 125.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/</code>
      </div>

      <h2>Overview</h2>

      <p>
        The EVM module (<code>modules/evm/</code>) executes Ethereum-format transactions on Paxeer. It integrates with the <Link href="/engine">execution engine</Link> inside the BFT node, using EVM chain ID <strong>125</strong>.
      </p>

      <p>
        The module handles:
      </p>

      <ul>
        <li><strong>EVM Transaction Processing:</strong> Execute Ethereum-format transactions</li>
        <li><strong>Address Association:</strong> Map Cosmos bech32 addresses to EVM hex addresses</li>
        <li><strong>Receipt Generation:</strong> Create transaction receipts with logs and status</li>
        <li><strong>Pointers:</strong> Link ERC-20 tokens to native Cosmos denoms</li>
        <li><strong>Precompiles:</strong> Expose Cosmos modules via EVM precompiled contracts</li>
      </ul>

      <h2>Message Types</h2>

      <p>
        The EVM module handles <code>MsgEVMTransaction</code>, which wraps a standard Ethereum transaction:
      </p>

      <ul>
        <li>Legacy transactions (type 0)</li>
        <li>EIP-2930 access list transactions (type 1)</li>
        <li>EIP-1559 dynamic fee transactions (type 2)</li>
        <li>EIP-4844 blob transactions (type 3), accepted from Cancun</li>
        <li>EIP-7702 set-code transactions (type 4), accepted from Prague</li>
      </ul>

      <p>
        The default chain configuration enables Cancun and Prague at time 0. Its transaction-type allowlist includes all five types. <code>MsgEVMTransaction</code> also supports <code>AssociateTx</code>, identified by <code>IsAssociateTx</code>, for address association.
      </p>

      <p>
        Transactions are submitted via the JSON-RPC <code>eth_sendRawTransaction</code> endpoint or the Cosmos SDK <code>MsgEVMTransaction</code> message.
      </p>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/ante/preprocess.go</code>, <code>modules/evm/types/codec.go</code>, <code>modules/evm/types/message_evm_transaction.go</code>, <code>modules/evm/types/config.go</code>
      </div>

      <h2>Address Association</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/address.go</code>
      </div>

      <p>
        Paxeer addresses exist in two formats:
      </p>

      <ul>
        <li><strong>Cosmos:</strong> <code>pax1...</code> (bech32)</li>
        <li><strong>EVM:</strong> <code>0x...</code> (hex, 20 bytes)</li>
      </ul>

      <p>
        The EVM module stores a bidirectional mapping between these addresses. Transaction preprocessing derives both addresses from the signer and establishes their association when needed. This supports:
      </p>

      <ul>
        <li>Cosmos accounts to interact with EVM contracts</li>
        <li>EVM accounts to interact with native Cosmos denominations</li>
        <li>Unified balance queries across both address formats</li>
      </ul>

      <h2>State Management</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/state.go</code>
      </div>

      <p>
        The EVM module stores:
      </p>

      <ul>
        <li><strong>Accounts:</strong> Nonce, balance (via bank module)</li>
        <li><strong>Code:</strong> Contract bytecode (<code>modules/evm/keeper/code.go</code>)</li>
        <li><strong>Storage:</strong> Contract storage slots (<code>modules/evm/keeper/state.go</code>)</li>
        <li><strong>Receipts:</strong> Transaction receipts and logs (<code>modules/evm/keeper/receipt.go</code>)</li>
      </ul>

      <p>
        EVM state is stored in the module&apos;s KVStore. Account balances are managed by the bank module to maintain consistency with native token operations.
      </p>

      <h2>Transaction Execution Flow</h2>

      <p>
        When an EVM transaction is submitted:
      </p>

      <ol>
        <li>RPC receives <code>eth_sendRawTransaction</code> or SDK receives <code>MsgEVMTransaction</code></li>
        <li>Ante decorators under <code>modules/evm/ante/</code> validate signatures, nonces, and transaction fees</li>
        <li>Msg server routes to EVM module</li>
        <li>Execution runs through the <Link href="/engine">engine executor</Link>; on the node Giga path this is <code>ExecuteTransactionFeeCharged</code>, with fees already charged</li>
        <li>Executor runs EVM bytecode against state DB</li>
        <li>Module generates receipt (<code>modules/evm/keeper/receipt.go</code>) with logs, gas used, status</li>
        <li>Module emits events for indexing</li>
      </ol>

      <h2>Receipts</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/receipt.go</code>
      </div>

      <p>
        Transaction receipts include:
      </p>

      <ul>
        <li><strong>Status:</strong> 1 (success) or 0 (revert)</li>
        <li><strong>Gas Used:</strong> Total gas consumed</li>
        <li><strong>Logs:</strong> Event logs emitted during execution</li>
        <li><strong>Contract Address:</strong> For contract creation transactions</li>
        <li><strong>Bloom Filter:</strong> For efficient log filtering</li>
      </ul>

      <p>
        Execution stages receipts in a transient store and flushes them to the configured receipt store. <code>GetReceipt</code> delegates to that store; JSON-RPC exposes them through <code>eth_getTransactionReceipt</code>.
      </p>

      <h2>Pointers</h2>

      <p>
        Pointer contracts expose native denominations and CosmWasm tokens through ERC interfaces. Native token pointers use the bank module for balances and transfers. They support:
      </p>

      <ul>
        <li>ERC-20 <code>transfer</code> to move native tokens via bank module</li>
        <li>ERC-20 balance queries for a registered native denomination</li>
        <li>ERC token interfaces for registered CosmWasm assets</li>
      </ul>

      <p>
        Pointer contracts are deployed via the <Link href="/precompiles">pointer precompile</Link>.
      </p>

      <h2>Precompile Integration</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>precompiles/setup.go</code>, <code>precompiles/</code>
      </div>

      <p>
        The EVM module registers custom precompiled contracts that expose Cosmos SDK modules to EVM callers. Paxeer-specific addresses are registered by the precompile package, including bank at <code>0x1001</code> and address conversion at <code>0x1004</code>. Standard Ethereum precompiles occupy a separate address range.
      </p>

      <p>
        See <Link href="/precompiles">Precompiles documentation</Link> for the registered interfaces. Check each interface&apos;s availability before making a call.
      </p>

      <h2>Fee Collection</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/fee.go</code>, <code>modules/evm/keeper/feetoken.go</code>, <code>modules/evm/keeper/feecollect.go</code>, <code>precompiles/feetoken/feetoken.go</code>
      </div>

      <p>
        EVM transaction fees are:
      </p>

      <ul>
        <li>Charged in PAX by default. When governance enables fee-token selection and configures an allowed denomination, an account can select it through the fee-token precompile at <code>0x0000000000000000000000000000000000001018</code>. See <Link href="/sidiora">Sidiora</Link></li>
        <li>Reported using receipt gas usage and the effective gas price; estimation and charge settlement depend on the configured execution path</li>
        <li>Sent to the fee collector module</li>
        <li>Routed under the configured fee-distribution rules; alternative fee tokens are held separately when their distribution is disabled</li>
      </ul>

      <p>
        EIP-1559 dynamic fees are supported with <code>baseFee</code> and priority fees.
      </p>

      <h2>Coinbase</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/coinbase.go</code>, <code>modules/evm/keeper/keeper.go</code>
      </div>

      <p>
        The EVM block context sets <code>COINBASE</code> to the fee collector module’s associated EVM address through <code>GetFeeCollectorAddress</code>.
      </p>

      <h2>Chain ID</h2>

      <p>
        Paxeer uses <strong>EVM chain ID 125</strong>. This must match the <code>chainId</code> field in EIP-155 transaction signatures for replay protection. The Cosmos chain identifier is <code>hyperpax_125-1</code>.
      </p>

      <h2>Configuration</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/types/params.go</code>, <code>modules/evm/types/config.go</code>, <code>modules/evm/keeper/params.go</code>
      </div>

      <p>
        EVM module parameters include:
      </p>

      <ul>
        <li><strong>Native units:</strong> PAX is the display currency; the implementation retains <code>uhpx</code> as its Cosmos base denomination and converts it to 18-decimal EVM units</li>
        <li><strong>Fee bounds:</strong> Minimum and maximum fees per gas, target block gas, and dynamic adjustment parameters</li>
        <li><strong>Storage gas:</strong> The default SSTORE set cost is 20,000 gas. Query the network’s EVM parameters when calculating costs for a deployment</li>
        <li><strong>Chain Config:</strong> Fork activation times, including Cancun and Prague, and a configurable SSTORE cost</li>
      </ul>

      <h2>Genesis</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/genesis.go</code>
      </div>

      <p>
        Genesis state for the EVM module includes:
      </p>

      <ul>
        <li>Module parameters</li>
        <li>Pre-deployed contract code and storage</li>
        <li>Address associations</li>
      </ul>

      <h2>Logging</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/log.go</code>
      </div>

      <p>
        EVM event logs are emitted during transaction execution via the <code>LOG0</code>, <code>LOG1</code>, <code>LOG2</code>, <code>LOG3</code>, <code>LOG4</code> opcodes. Logs are included in receipts and indexed for querying via <code>eth_getLogs</code>.
      </p>

      <h2>Deferred Operations</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>modules/evm/keeper/deferred.go</code>
      </div>

      <p>
        <code>modules/evm/keeper/deferred.go</code> records each transaction&apos;s index, hash, bloom, and balance surplus in transient storage for block processing. It also tracks nonce bumps and execution errors.
      </p>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/json-rpc">Use the JSON-RPC API</Link></li>
        <li><Link href="/precompiles">Explore Paxeer precompiles</Link></li>
        <li><Link href="/contracts">Deploy contracts</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/engine">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Engine</div>
        </Link>
        <Link href="/storage">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Storage</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
