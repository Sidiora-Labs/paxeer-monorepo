import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/foundry')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Build with Foundry</h1>
        <p className="page-description">Create, test, and deploy a Solidity counter with Forge, then read its state through the Paxeer EVM using Cast.</p>
      </div>
      <p>Use this guide for a new Solidity project. Forge compiles and tests contracts; Cast reads chain data and sends signed transactions. The examples target <Link href="/network-parameters">Paxeer chain ID 125</Link> (Cosmos chain <code>hyperpax_125-1</code>), whose native gas token is PAX. Other networks map differently — for example <code>pacific-1</code> is 1329 and an unmapped local chain defaults to 713714 — so substitute your endpoint&apos;s actual chain ID where 125 appears below.</p>
      <h2>Set up the project</h2>
      <p>Install Forge and Cast using the <a href="https://getfoundry.sh/introduction/installation/">Foundry installation instructions</a>, then create a project. Keep the generated project files; add the named files below alongside the scaffold.</p>
      <pre><code className="language-bash">{"forge init paxeer-counter\ncd paxeer-counter\nforge --version\ncast --version"}</code></pre>
      <p>Use this <code>foundry.toml</code> for the example. It matches the in-tree toolchain: solc 0.8.27 with the Prague EVM target, which the chain enables from genesis along with Cancun. The repository&apos;s own <code>foundry.paxeer.toml</code> pins solc 0.8.27 without an explicit EVM version or optimizer run count; the run count below is an example choice, so record whatever you deploy with.</p>
      <pre><code className="language-toml">{"[profile.default]\nsrc = \"src\"\nout = \"out\"\nlibs = [\"lib\"]\nsolc_version = \"0.8.27\"\nevm_version = \"prague\"\noptimizer = true\noptimizer_runs = 200\n\n[rpc_endpoints]\npaxeer = \"${PAXEER_RPC_URL}\""}</code></pre>
      <h2>Write the contract</h2>
      <p>Create <code>src/PaxeerCounter.sol</code>. Anyone can update this demonstration counter; it is useful for checking storage, events, and frontend reads.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity 0.8.27;\n\ncontract PaxeerCounter {\n    uint256 public number;\n    event NumberChanged(address indexed caller, uint256 number);\n\n    function setNumber(uint256 next) external {\n        number = next;\n        emit NumberChanged(msg.sender, next);\n    }\n\n    function increment() external {\n        number += 1;\n        emit NumberChanged(msg.sender, number);\n    }\n}"}</code></pre>
      <h2>Test local behavior</h2>
      <p>Create <code>test/PaxeerCounter.t.sol</code>. These tests run the contract locally in Forge, using an isolated test environment.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity 0.8.27;\n\nimport {Test} from \"forge-std/Test.sol\";\nimport {PaxeerCounter} from \"../src/PaxeerCounter.sol\";\n\ncontract PaxeerCounterTest is Test {\n    PaxeerCounter private counter;\n\n    function setUp() public {\n        counter = new PaxeerCounter();\n    }\n\n    function test_Increment() public {\n        counter.increment();\n        assertEq(counter.number(), 1);\n    }\n\n    function testFuzz_SetNumber(uint256 next) public {\n        counter.setNumber(next);\n        assertEq(counter.number(), next);\n    }\n}"}</code></pre>
      <pre><code className="language-bash">{"forge build\nforge test --match-contract PaxeerCounterTest -vv\nforge test --match-contract PaxeerCounterTest --gas-report"}</code></pre>
      <p>The expected result is two passing tests, including the generated fuzz cases. Use the gas report to compare the cost of individual contract calls. Measure network throughput separately with a Paxeer development cluster.</p>
      <h2>Connect and rehearse deployment</h2>
      <p>Set <code>PAXEER_RPC_URL</code> to your EVM HTTP endpoint. This is different from a Cosmos RPC endpoint. Confirm its chain ID before selecting a signer. Import a development account into Foundry’s encrypted keystore when prompted.</p>
      <pre><code className="language-bash">{": \"${PAXEER_RPC_URL:?Set your Paxeer EVM RPC URL}\"\ntest \"$(cast chain-id --rpc-url \"$PAXEER_RPC_URL\")\" = 125 || { echo \"Expected Paxeer chain ID 125\" >&2; exit 1; }\ncast wallet import paxeer-dev --interactive\nforge create src/PaxeerCounter.sol:PaxeerCounter   --rpc-url \"$PAXEER_RPC_URL\" --account paxeer-dev"}</code></pre>
      <p>Run <code>forge create</code> first without <code>--broadcast</code> to rehearse, then review the destination chain, sender, and estimated cost before broadcasting. The signer needs PAX for an actual Paxeer deployment. For a local development network, use its actual chain ID and local funds.</p>
      <h2>Publish and read the result</h2>
      <p>The next command broadcasts a contract-creation transaction. Save its address and transaction hash from the output.</p>
      <pre><code className="language-bash">{"forge create src/PaxeerCounter.sol:PaxeerCounter   --rpc-url \"$PAXEER_RPC_URL\" --account paxeer-dev --broadcast\n\n# Set CONTRACT_ADDRESS and TX_HASH from the deployment output.\ncast receipt \"$TX_HASH\" --rpc-url \"$PAXEER_RPC_URL\"\ncast code \"$CONTRACT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast call \"$CONTRACT_ADDRESS\" 'number()(uint256)'   --rpc-url \"$PAXEER_RPC_URL\""}</code></pre>
      <p>Confirm a successful receipt, nonempty deployed bytecode, and an initial counter value of zero. The contract ABI is in <code>out/PaxeerCounter.sol/PaxeerCounter.json</code>. Use that artifact in your <Link href="/building-a-frontend">frontend</Link>; keep the compiler version and optimizer settings with your deployment record.</p>
      <p>Reference: <a href="https://getfoundry.sh/forge/deploying/">Foundry deployment and verification</a>. For explorer verification, select a verifier that supports your Paxeer endpoint and submit the same compiler settings used for deployment.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>Paxeer-X-Network/</code> — contracts/hardhat.config.js (Solidity 0.8.27, Prague), foundry.paxeer.toml (solc 0.8.27), modules/evm/config/config.go (ChainIDMapping), and rpc/send.go (SendRawTransaction).</div>
      <div className="prev-next">
        <Link href="/contracts"><div className="prev-next-label">Previous</div><div className="prev-next-title">Contracts</div></Link>
        <Link href="/hardhat"><div className="prev-next-label">Next</div><div className="prev-next-title">Build with Hardhat</div></Link>
      </div>
    </DocsLayout>
  )
}
