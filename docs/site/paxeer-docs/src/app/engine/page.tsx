import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/engine')

export default function Engine() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Engine</h1>
        <p className="page-description">
          Paxeer's EVM execution engine, transaction processing, and state transition management.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/</code>
      </div>

      <h2>Overview</h2>

      <p>
        The engine package provides EVM transaction execution for Paxeer. It wraps go-ethereum's EVM with Paxeer-specific precompiles, state management, and fee handling. The engine sits between the consensus layer (which orders transactions) and the EVM module (which manages on-chain state).
      </p>

      <h2>Executor</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/executor/</code>
      </div>

      <p>
        The executor wraps EVM execution with separate constructors for the Geth interpreter and the EVMC backend.
      </p>

      <h3>Executor Types</h3>

      <p>
        The <code>Executor</code> struct in <code>engine/executor/executor.go</code> supports two backends:
      </p>

      <ul>
        <li><strong>Geth Executor:</strong> Uses go-ethereum's native EVM interpreter</li>
        <li><strong>Evmone Executor:</strong> Uses the evmone C++ VM via EVMC bindings for performance</li>
      </ul>

      <pre><code>{`// From engine/executor/executor.go
type Executor struct {
    evm *vm.EVM
}

func NewGethExecutor(blockCtx vm.BlockContext, stateDB vm.StateDB, 
    chainConfig *params.ChainConfig, config vm.Config, 
    customPrecompiles map[common.Address]vm.PrecompiledContract) *Executor

func NewEvmoneExecutor(evmoneVM *evmc.VM, blockCtx vm.BlockContext, 
    stateDB vm.StateDB, chainConfig *params.ChainConfig, config vm.Config, 
    customPrecompiles map[common.Address]vm.PrecompiledContract) *Executor`}</code></pre>

      <h3>Transaction Execution</h3>

      <p>
        The executor provides two execution modes:
      </p>

      <h4>Standard Execution</h4>

      <pre><code>{`func (e *Executor) ExecuteTransaction(tx *types.Transaction, 
    sender common.Address, baseFee *big.Int, 
    gasPool *core.GasPool) (*core.ExecutionResult, error)`}</code></pre>

      <p>
        Standard execution converts the transaction into an internal message and calls go-ethereum’s ApplyMessage, including its gas purchase and refund handling.
      </p>

      <h4>Fee-Already-Charged Execution</h4>

      <pre><code>{`func (e *Executor) ExecuteTransactionFeeCharged(tx *types.Transaction, 
    sender common.Address, baseFee *big.Int, 
    gasPool *core.GasPool) (*core.ExecutionResult, error)`}</code></pre>

      <p>
        This mode assumes fees were already charged before execution. It skips fee deduction and refund. This is the mode the node Giga execution path uses for <code>MsgEVMTransaction</code> transactions.
      </p>

      <h3>Internal Components</h3>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/executor/internal/</code>
      </div>

      <p>
        Internal execution utilities:
      </p>

      <ul>
        <li><strong>Host context</strong> (<code>host_context.go</code>): EVMC host context for evmone integration</li>
        <li><strong>Interpreter</strong> (<code>interpreter.go</code>): EVM opcode interpreter wrapper</li>
        <li><strong>Message</strong> (<code>message.go</code>): internal execution message handling</li>
      </ul>

      <h2>Precompiles</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>precompiles/</code>; <code>engine/executor/precompiles/failfast.go</code>
      </div>

      <p>
        Paxeer-specific precompiled contracts live in the top-level <code>precompiles/</code> package and are registered with the executor at initialization. The executor passes the <code>customPrecompiles</code> map to the EVM, which routes calls to the registered precompile addresses. The <code>engine/executor/precompiles/</code> directory itself contains only the fail-fast wrapper (<code>failfast.go</code>) and its tests.
      </p>

      <p>
        See <Link href="/precompiles">Precompiles documentation</Link> for the full list of Paxeer precompiles (bank, staking, oracle, pointer, etc.).
      </p>

      <h2>Dependencies (xbank, xevm)</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/deps/</code>
      </div>

      <p>
        The engine depends on Cosmos SDK modules via thin wrappers in <code>engine/deps/</code>:
      </p>

      <h3>xbank</h3>

      <p>
        <code>engine/deps/xbank/</code> wraps the SDK bank module for balance operations:
      </p>

      <ul>
        <li><strong>Send:</strong> Transfer tokens between accounts</li>
        <li><strong>View:</strong> Query balances and supply</li>
        <li><strong>Deferred Cache:</strong> Batch balance updates before commit</li>
      </ul>

      <h3>xevm</h3>

      <p>
        <code>engine/deps/xevm/</code> wraps the EVM module keeper:
      </p>

      <ul>
        <li><strong>State:</strong> EVM storage access (accounts, code, storage slots)</li>
        <li><strong>Code:</strong> Contract bytecode storage and retrieval</li>
        <li><strong>Receipt:</strong> Transaction receipt generation</li>
        <li><strong>Address:</strong> Cosmos ↔ EVM address association</li>
        <li><strong>Nonce:</strong> Account nonce management</li>
        <li><strong>Fee:</strong> Gas price and fee collection</li>
        <li><strong>Coinbase:</strong> EVM coinbase address and fee-collector lookup</li>
        <li><strong>Precompile:</strong> Precompile address registration</li>
      </ul>

      <h2>Configuration</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/executor/config/</code>
      </div>

      <p>
        The Giga executor configuration carries two flags, both defaulting to <code>true</code>:
      </p>

      <ul>
        <li><strong><code>giga_executor.enabled</code>:</strong> whether the Giga executor is used</li>
        <li><strong><code>giga_executor.occ_enabled</code>:</strong> whether optimistic concurrency control (OCC) is used with the Giga executor</li>
      </ul>

      <pre><code>{`# app.toml
[giga_executor]
enabled = true
occ_enabled = true`}</code></pre>

      <p>
        Chain configuration (chain ID 125, fork activation) and the custom precompile address map are supplied to the executor constructors by the caller. They are not fields of this executor config.
      </p>

      <h2>Testing</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/tests/</code>
      </div>

      <p>
        The engine includes state-transition tests and an execution harness:
      </p>

      <ul>
        <li><strong>State Tests:</strong> <code>state_test.go</code> runs Ethereum state transition tests</li>
        <li><strong>Harness:</strong> <code>tests/harness/</code> provides test fixtures and builders</li>
        <li><strong>Giga Test:</strong> <code>giga_test.go</code> for large-scale execution</li>
      </ul>

      <h2>Chain ID</h2>

      <p>
        The chain configuration maps the Cosmos identifier <code>hyperpax_125-1</code> to <strong>EVM chain ID 125</strong>. Transactions on that network must use chain ID 125 for EIP-155 replay protection. Unmapped Cosmos identifiers also return the default EVM chain ID 125 in <code>modules/evm/config/config.go</code>. Check the configured network as well as the numeric chain ID.
      </p>

      <h2>Utilities</h2>

      <div className="source-note">
        <strong>Source:</strong> <code>engine/executor/utils/</code>
      </div>

      <p>
        The utils package (<code>errors.go</code>) defines the Giga abort sentinel (<code>giga_fallback_to_v2</code>). <code>ShouldExecutionAbort</code> checks whether a <code>vm.AbortError</code> requests an abort, allowing the caller to select the standard (V2) execution path.
      </p>

      <h2>Integration with EVM Module</h2>

      <p>
        EVM transactions arrive as <code>MsgEVMTransaction</code> messages. On the Giga execution path, the node application (<code>node/app.go</code>) constructs a geth executor with <code>NewGethExecutor</code> and calls <code>ExecuteTransactionFeeCharged</code>, because fees have already been charged before execution. The flow is:
      </p>

      <ol>
        <li>Consensus orders transactions in a block</li>
        <li>Block processing routes <code>MsgEVMTransaction</code> to the EVM execution path</li>
        <li>Ante processing validates the transaction and charges fees</li>
        <li>The Giga path constructs the executor and calls <code>ExecuteTransactionFeeCharged</code></li>
        <li>The executor runs EVM bytecode against the state DB</li>
        <li>The EVM module generates the receipt and emits events</li>
      </ol>

      <p>
        See <Link href="/evm">EVM Module documentation</Link> for the full message handling flow.
      </p>

      <h2>Next Steps</h2>

      <p><Link href="/parallelization">Design for Parallel Execution</Link> explains how application storage layout affects contention and how to measure a change without confusing gas usage with network throughput.</p>

      <ul>
        <li><Link href="/evm">Understand the EVM module</Link></li>
        <li><Link href="/precompiles">Review Paxeer precompiles</Link></li>
        <li><Link href="/json-rpc">Use the JSON-RPC API</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/consensus">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Consensus</div>
        </Link>
        <Link href="/evm">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">EVM</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
