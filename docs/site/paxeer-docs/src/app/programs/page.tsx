import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/programs')

export default function ProgramsPage() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Deterministic Programs</h1>
      <p className="page-description">Deploy and call deterministic Paxeer X programs with scoped capabilities, metered execution, governed upgrades and verifiable outputs.</p>
    </div>
    <h2>One execution surface, explicit authority</h2>
    <p>Programs extend the LayerX kernel domain of Paxeer X with application logic. A program executes a validated WebAssembly artifact under the identity, capability grants, access declarations, and resource ceilings of its invoking activity. Artifact validation rejects floating-point types and instructions and enforces structural limits before interpreter execution. The same admitted activity and state produce replayable results.</p>
    <p>Programs dispatch through kernel module 9. They do not become economic modules and never write balances directly. All monetary effects compile to authenticated <code>402LXP</code> transfer sets applied by the kernel. The kernel executes within the Paxeer BFT node on EVM chain ID <code>125</code>. A receipt is final when its covering block is committed.</p>
    <h2>Application building blocks</h2>
    <table><thead><tr><th>Capability</th><th>Application behavior</th></tr></thead><tbody>
      <tr><td>Namespaced storage</td><td>Read, write, delete, and scan bounded program state; distinguish principal-scoped state from explicitly granted shared state.</td></tr>
      <tr><td>Program composition</td><td>Invoke another program with an explicit callee grant, narrowed capabilities, bounded depth, and response capacity.</td></tr>
      <tr><td>Native value accounts</td><td>Derive and register program-owned accounts for native Assets; stage authorized payments, escrow releases, swaps, and vault movements.</td></tr>
      <tr><td>Receipt evidence</td><td>Consume explicitly granted receipt digests and expose receipt-verified deployment, state, and balance information.</td></tr>
      <tr><td>Deterministic data and arithmetic</td><td>Read admitted context, verify signatures, hash bytes, perform 256-bit integer arithmetic, and consume oracle and web evidence through versioned host imports.</td></tr>
      <tr><td>Bounded computation</td><td>Run the deterministic interpreter, lease isolated sandbox execution, and adjudicate disputed market steps.</td></tr>
    </tbody></table>
    <h2>Deploy, call, upgrade, and retire</h2>
    <h3>Deploy an artifact and interface</h3>
    <p>A deployment binds a 32-byte program ID to its WASM artifact, declared SHA-256 code hash, guest ABI version, and upgrade policy. The immutable policy has no upgrade authority. The authority policy names the principal permitted to upgrade. An optional canonical interface describes entry selectors, input and output bounds, and required capability descriptors. Artifact identity is the program ID and code hash together.</p>
    <p>Deployment creates version 1 and emits deployment evidence. Retain and verify that receipt before treating a registry entry as proof of deployment. Snapshots retain artifacts so restored state resolves the same code.</p>
    <h3>Execute a bounded call</h3>
    <p>A CALL names the program, guest ABI, entrypoint, canonical calldata, capability grants, access declarations, response capacity, and seven resource-budget dimensions. The runtime verifies the active program record and opens the matching artifact before running guest code. Entrypoint names use letters, digits, underscores, and periods.</p>
    <p>The Programs precompile exposes <code>call</code>, <code>callWithReceipts</code> and <code>simulateCall</code>, as declared in <code>precompiles/layerxprograms/LayerXPrograms.sol</code>. Simulation uses the same CALL payload encoding and runs against a disposable state branch. Its result includes the terminal kind, result code and canonical response frame. Simulation previews execution against the selected state; verify the submitted transaction and its committed receipt before treating an outcome as settled.</p>
    <h3>Upgrade with an explicit policy</h3>
    <p>An upgrade must match the recorded authority and current code hash, supply a new artifact and hash, and follow admitted ABI transitions. Optional hook bytes and interface replacement or removal accompany the upgrade. The version counter increments and upgrade evidence binds the old and new hashes. Immutable programs reject upgrades.</p>
    <h3>Wind down without stranding value</h3>
    <p>Lifecycle states are active, deprecated, and tombstoned. Deprecated or tombstoned programs refuse new CALL execution and upgrades. Before deprecation, the owner configures an exit route for each registered program account, including accounts with zero balances. Deprecation binds the program itself as the exit program and a future sequence deadline. Value exits remain available in deprecated and tombstoned states through the configured routes.</p>
    <h2>Authority and atomic settlement</h2>
    <p>Storage access, events, calls, transfers, receipt reads, shared storage, balance views, and program spending are separate grants. A callee cannot obtain authority beyond its parent. Identity fields remain bound; monetary ceilings only narrow. Capability escalation refuses the composition rather than committing a partial transfer set.</p>
    <p>A program account is derived from the program ID and an exact seed. Derivation alone grants no ownership, registration, funding, or debit authority. The owner registers the account for an Asset; a principal funds it through an ordinary transfer. A <code>ProgramSpend</code> grant separately binds the owner program, seed, source account, Asset, destination, and maximum amount. Only the deriving program frame stages the debit, and repeated transfers consume the ceiling cumulatively.</p>
    <pre><code>{`program account = SHA-256(
  "LayerX/programs/program-account/v1\\0"
  || program_id32 || seed_length:u32_be || seed
)`}</code></pre>
    <h2>Metering and persistent-state occupancy</h2>
    <table><thead><tr><th>Declared ceiling</th><th>What consumes it</th></tr></thead><tbody>
      <tr><td>CPU fuel</td><td>Interpreter instructions and admitted host work.</td></tr>
      <tr><td>Memory bytes</td><td>Peak guest linear memory.</td></tr>
      <tr><td>Storage read / write bytes</td><td>Cumulative state access and mutation.</td></tr>
      <tr><td>Output values / bytes</td><td>Guest return values, response bytes, and refusal transport.</td></tr>
      <tr><td>Table elements</td><td>Peak guest table allocation.</td></tr>
    </tbody></table>
    <p>Admission checks each ceiling and reserves fee coverage before guest execution. Persistent storage also incurs deterministic occupancy measured in protocol batches, separate from the execution meter. The signed activity budget authorizes occupancy responsibility. A namespace mandate binds the payer, byte limit, price bound and charge ceiling to the admitted activity.</p>
    <pre><code>{`byte_batches = recorded_bytes × (to_batch − from_batch)
accrued_fee = byte_batches × occupancy_byte_batch_price
amount_due = prior_arrears + accrued_fee`}</code></pre>
    <p>Occupancy settlement becomes final with its covering block commitment and uses the same native transfer primitive. New responsibility uses the admitted caller authorization; existing namespace limits retain their payer when another principal calls. Settlement evidence records amounts due, amounts paid, arrears and frozen status.</p>
    <h2>Versioned guest interfaces</h2>
    <p>Guest ABI, kernel module ABI, and wire protocol are independent version numbers. Select the guest ABI required by the artifact and its imports rather than assuming that the kernel module version describes the guest interface.</p>
    <table><thead><tr><th>Guest ABI</th><th>Host surface added</th></tr></thead><tbody>
      <tr><td>1</td><td>Storage, event emission, program calls, principal transfers, and granted receipt reads.</td></tr>
      <tr><td>2</td><td>Responses and refusals; scoped storage and scanning; program transfers and funding; context and balance reads; hashing, signature verification and recovery, and 256-bit integer operations.</td></tr>
      <tr><td>3</td><td><code>oracle_read</code> for admitted oracle evidence.</td></tr>
      <tr><td>4</td><td><code>web_read</code> for admitted web evidence.</td></tr>
      <tr><td>5</td><td><code>market_step_adjudicate</code> for deterministic market-step verdicts.</td></tr>
    </tbody></table>
    <p>External evidence enters through admitted protocol paths. A state transition does not dial out to an arbitrary service. Frozen ABI vectors and manifests preserve the byte contract across implementations.</p>
    <DocArt name="results-artifacts" />
    <h2>Outputs you can verify</h2>
    <p>Calls return bounded response or refusal bytes and publish terminal evidence, resource usage, call-graph information, and transfer commitments. Verify the activity identity, code and state bindings, terminal, and requested commitment before using a result to release value or advance a workflow. The registry verifies program-account bindings and account records against a trusted committed chain application hash before exposing balances. The chain proof verifier is in programs/crates/layerx-programs-registry/src/chain_accounts.rs.</p>
    <p>A granted guest receipt read returns exactly 116 bytes: receipt digest, signed result code, Asset ID, unsigned 128-bit amount, and state root. The digest must match the requested receipt. This lets a merchant program consume authenticated payment evidence instead of trusting a caller-supplied payment description.</p>
    <h2>Developer workflow</h2>
    <ol>
      <li>Choose a guest SDK in programs/sdk/rust, programs/sdk/c or programs/sdk/assemblyscript and define the application&apos;s state, entrypoints, bounds and capability needs.</li>
      <li>Compile to WebAssembly, generate the canonical interface, and hash the artifact.</li>
      <li>Deploy with an immutable or authority upgrade policy; verify the deployment receipt.</li>
      <li>Register and fund program value accounts where the application holds Assets.</li>
      <li>Construct a CALL with the minimum grants and sufficient execution and occupancy ceilings. Simulate against the current head.</li>
      <li>Submit the signed activity and verify the committed terminal and receipt before displaying completion.</li>
    </ol>
    <p>Rust reference applications include escrow, naming, LXT-721 NFTs, merchant payments, constant-product swaps, LXT-20 tokens, vaults, and web readers. C and AssemblyScript provide paid-counter examples. EVM, Solana/Anchor, and CosmWasm porting guides map familiar concepts to explicit capabilities and native transfer settlement.</p>
    <h3>Example: fixed-supply LXT-20</h3>
    <p>The LXT-20 reference is an ABI-2 program backed by one native Asset. It exposes <code>initialize</code>, <code>transfer</code>, <code>approve</code>, <code>transfer_from</code>, <code>balance_of</code>, <code>allowance</code>, <code>total_supply</code>, and <code>metadata</code>. Supply, Asset, and spend ceiling are fixed at build time. Approval of zero revokes an allowance. A recipient must approve at least once, including zero, to register its derived account in token storage, and the deployment authority must separately register that account with the native Programs account module before it can receive backing units. Arbitrary external recipient accounts are refused. This reference has no mint, burn, permit, or nested-call entrypoint.</p>
    <pre><code>{`# Canonical initialize request (hex)
4c581400012000000000

# Run from the repository root; requires the wasm32-unknown-unknown target
cargo build --manifest-path programs/sdk/rust/examples/token-lxt20/Cargo.toml \\
  --target wasm32-unknown-unknown --release`}</code></pre>
    <p>Continue with <Link href="/sandbox">bounded sandbox execution</Link>, <Link href="/assets">native Assets</Link>, and <Link href="/payments">payments</Link>.</p>
    <div className="source-note"><strong>Technical references:</strong> programs/Cargo.toml; programs/sdk; programs/porting; programs/crates/layerx-programs-runtime/src; programs/crates/layerx-programs-registry/src; src/modules/programs; precompiles/layerxprograms/LayerXPrograms.sol.</div>
  </DocsLayout>
}
