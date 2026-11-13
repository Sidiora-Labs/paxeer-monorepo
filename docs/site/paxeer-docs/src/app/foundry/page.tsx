import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/foundry')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Build with Foundry</h1>
        <p className="page-description">Compile and test the in-tree Solidity contracts with Forge, then inspect a Paxeer deployment with Cast.</p>
      </div>
      <p>Forge builds the repository&apos;s Solidity projects. The Paxeer chain project uses <code>foundry.paxeer.toml</code>; the root <code>foundry.toml</code> selects a separate contract project. The <Link href="/network-parameters">Paxeer EVM chain ID is 125</Link> (<code>hyperpax_125-1</code>), and the native gas token is PAX.</p>
      <h2>Select the chain project</h2>
      <p>Install Forge and Cast using the <a href="https://getfoundry.sh/introduction/installation/">Foundry installation instructions</a>. Run the following commands from the repository root. <code>contracts/bootstrap-libs.sh</code> fetches <code>forge-std v1.9.6</code> and <code>openzeppelin-contracts v5.3.0</code> into <code>contracts/lib/</code>. It reuses pristine pinned checkouts and replaces other contents at those dependency paths.</p>
      <pre><code className="language-bash">{"forge --version\ncast --version\nbash contracts/bootstrap-libs.sh"}</code></pre>
      <p>The chain configuration selects <code>contracts/src</code>, <code>contracts/test</code>, <code>contracts/out</code>, and Solidity <code>0.8.27</code>. The following excerpt is from <code>foundry.paxeer.toml</code>. Select it with <code>FOUNDRY_CONFIG</code>.</p>
      <pre><code className="language-toml">{"[profile.default]\nsrc = \"contracts/src\"\nout = \"contracts/out\"\ntest = \"contracts/test\"\nlibs = [\"contracts/lib\"]\nremappings = [\n    \"forge-std/=contracts/lib/forge-std/src/\",\n    \"@openzeppelin/contracts/=contracts/lib/openzeppelin-contracts/contracts/\",\n]\nsolc_version = \"0.8.27\"\nauto_detect_solc = false"}</code></pre>
      <h2>Read the contract</h2>
      <p><code>contracts/src/Box.sol</code> provides a small storage contract. Its <code>store</code>, <code>retrieve</code>, and <code>boxIncr</code> methods write, read, and increment a value. Its source is:</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity ^0.8.0;\n\ncontract Box {\n    uint256 private _value;\n\n    event ValueChanged(uint256 value);\n\n    function store(uint256 value) public {\n        _value = value;\n        emit ValueChanged(value);\n    }\n\n    function retrieve() public view returns (uint256) {\n        return _value;\n    }\n\n    function boxIncr() public {\n        _value = _value + 1;\n        emit ValueChanged(_value);\n    }\n}"}</code></pre>
      <h2>Run the existing tests</h2>
      <p><code>contracts/test/WPAXTest.t.sol</code> exercises the wrapped native-token contract. The following test function checks its name, symbol, and decimals; run the existing test contract with the commands below.</p>
      <pre><code className="language-solidity">{"function testMetadataNamesTheCoin() public view {\n    assertEq(wpax.name(), \"Wrapped Paxeer\");\n    assertEq(wpax.symbol(), \"WPAX\");\n    assertEq(wpax.decimals(), 18);\n}"}</code></pre>
      <pre><code className="language-bash">{"FOUNDRY_CONFIG=foundry.paxeer.toml forge build\nFOUNDRY_CONFIG=foundry.paxeer.toml forge test --match-contract WPAXTest\nFOUNDRY_CONFIG=foundry.paxeer.toml forge test --match-contract WPAXTest --gas-report"}</code></pre>
      <p><code>WPAXTest</code> checks token metadata, storage slots, selectors, wrapping, withdrawals, and token transfers. A Forge gas report reports local execution costs; use network fee estimates when preparing a transaction.</p>
      <h2>Connect and prepare deployment</h2>
      <p>Set <code>PAXEER_RPC_URL</code> to the node&apos;s root EVM HTTP endpoint. Confirm chain ID <code>125</code> and select an encrypted keystore account for deployment.</p>
      <pre><code className="language-bash">{": \"${PAXEER_RPC_URL:?Set the node EVM RPC endpoint}\"\ncast chain-id --rpc-url \"$PAXEER_RPC_URL\"\ncast wallet import paxeer-dev --interactive\nFOUNDRY_CONFIG=foundry.paxeer.toml forge create contracts/src/Box.sol:Box --rpc-url \"$PAXEER_RPC_URL\" --account paxeer-dev"}</code></pre>
      <p>Use <code>forge create</code> without <code>--broadcast</code> to prepare the deployment. Review the chain, sender, and fee estimate. An actual deployment requires sufficient PAX.</p>
      <h2>Deploy and read the result</h2>
      <p>The next command broadcasts the existing Box contract. Retain the deployed contract address and transaction hash from its output.</p>
      <pre><code className="language-bash">{"FOUNDRY_CONFIG=foundry.paxeer.toml forge create contracts/src/Box.sol:Box --rpc-url \"$PAXEER_RPC_URL\" --account paxeer-dev --broadcast\n\n# Set CONTRACT_ADDRESS and TX_HASH from the deployment output.\ncast receipt \"$TX_HASH\" --rpc-url \"$PAXEER_RPC_URL\"\ncast code \"$CONTRACT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast call \"$CONTRACT_ADDRESS\" 'retrieve()(uint256)' --rpc-url \"$PAXEER_RPC_URL\""}</code></pre>
      <p>Check receipt status, deployed bytecode, and <code>retrieve()</code>. A new Box starts at zero. The build artifact is <code>contracts/out/Box.sol/Box.json</code>. Use its ABI in your <Link href="/building-a-frontend">frontend</Link> and retain the compiler settings used for deployment.</p>
      <p>Reference: <a href="https://getfoundry.sh/forge/deploying/">Foundry deployment and verification</a>. Verification uses the same source and compiler settings as the deployed bytecode.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>foundry.paxeer.toml</code>, <code>contracts/src/Box.sol</code>, <code>contracts/test/WPAXTest.t.sol</code>, <code>modules/evm/config/config.go</code>, and <code>rpc/send.go</code>.</div>
      <div className="prev-next">
        <Link href="/contracts"><div className="prev-next-label">Previous</div><div className="prev-next-title">Contracts</div></Link>
        <Link href="/hardhat"><div className="prev-next-label">Next</div><div className="prev-next-title">Build with Hardhat</div></Link>
      </div>
    </DocsLayout>
  )
}
