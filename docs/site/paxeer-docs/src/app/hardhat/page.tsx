import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/hardhat')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Build with Hardhat</h1>
        <p className="page-description">Configure Hardhat 3 for Paxeer, test a Solidity contract locally, and keep repeatable deployments with Hardhat Ignition.</p>
      </div>
      <p>Hardhat brings Solidity compilation, tests, and deployment records into a TypeScript project. This walkthrough creates a new Hardhat 3 project with its Mocha/Ethers toolbox and an explicit HTTP network for Paxeer chain ID 125 (Cosmos chain <code>hyperpax_125-1</code>; local or unmapped chains report other IDs, defaulting to 713714). It is separate from the repository&apos;s in-tree contract project, which remains Hardhat 2 with a CommonJS config — see the note under Configure the network.</p>
      <h2>Create a Hardhat 3 project</h2>
      <p>Use a <a href="https://hardhat.org/docs/reference/nodejs-support">Node.js version supported by Hardhat 3</a>. Run the initializer and choose the TypeScript project with Mocha and Ethers.js; install the dependencies it proposes.</p>
      <pre><code className="language-bash">{"mkdir paxeer-hardhat\ncd paxeer-hardhat\nnpx hardhat@3 --init\nnpx hardhat --version"}</code></pre>
      <p>The scaffold includes ESM configuration, a TypeScript configuration, and the toolbox plugins. Retain its lockfile so another developer uses the same toolchain.</p>
      <h2>Configure the network</h2>
      <p>In <code>hardhat.config.ts</code>, preserve any scaffold settings your project needs and use this network configuration. The compiler settings match the in-tree project (solc 0.8.27, Prague, which the chain enables from genesis); only the Hardhat 3 config shape is new.</p>
      <pre><code className="language-typescript">{"import { configVariable, defineConfig } from \"hardhat/config\";\nimport hardhatToolboxMochaEthers from \"@nomicfoundation/hardhat-toolbox-mocha-ethers\";\n\nexport default defineConfig({\n  plugins: [hardhatToolboxMochaEthers],\n  solidity: {\n    version: \"0.8.27\",\n    settings: {\n      evmVersion: \"prague\",\n      optimizer: { enabled: true, runs: 1000 },\n    },\n  },\n  networks: {\n    local: { type: \"edr-simulated\", chainType: \"l1\" },\n    paxeer: {\n      type: \"http\",\n      chainType: \"l1\",\n      chainId: 125,\n      url: configVariable(\"PAXEER_RPC_URL\"),\n      accounts: [configVariable(\"PAXEER_PRIVATE_KEY\")],\n    },\n  },\n});"}</code></pre>
      <p>For reference, the in-tree project at <code>contracts/</code> uses Hardhat 2 (<code>hardhat ^2.20.1</code>, toolbox <code>^4.0.0</code>) with sources in <code>./src</code> and a <code>paxlocal</code> network read from <code>PAXEER_LOCAL_EVM_RPC_URL</code> (default <code>http://127.0.0.1:8545</code>). Use that project as-is for chain contracts; use this guide only for a new standalone project.</p>
      <p>Store your EVM endpoint and deployment key through the keystore plugin included in the toolbox. <code>configVariable</code> also resolves environment variables for CI and other automation.</p>
      <pre><code className="language-bash">{"npx hardhat keystore set PAXEER_RPC_URL\nnpx hardhat keystore set PAXEER_PRIVATE_KEY"}</code></pre>
      <h2>Add and test the contract</h2>
      <p>Create <code>contracts/PaxeerCounter.sol</code>. This is the same small contract used by the Foundry guide.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity 0.8.27;\n\ncontract PaxeerCounter {\n    uint256 public number;\n    event NumberChanged(address indexed caller, uint256 number);\n\n    function setNumber(uint256 next) external {\n        number = next;\n        emit NumberChanged(msg.sender, next);\n    }\n\n    function increment() external {\n        number += 1;\n        emit NumberChanged(msg.sender, number);\n    }\n}"}</code></pre>
      <p>Create <code>contracts/PaxeerCounter.t.sol</code>. A failed <code>require</code> fails the test; this test deploys and exercises the contract within the simulated EVM.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity 0.8.27;\n\nimport {PaxeerCounter} from \"./PaxeerCounter.sol\";\n\ncontract PaxeerCounterTest {\n    function testIncrement() public {\n        PaxeerCounter counter = new PaxeerCounter();\n        counter.increment();\n        require(counter.number() == 1, \"increment did not persist\");\n    }\n\n    function testSetNumber(uint256 next) public {\n        PaxeerCounter counter = new PaxeerCounter();\n        counter.setNumber(next);\n        require(counter.number() == next, \"value did not persist\");\n    }\n}"}</code></pre>
      <pre><code className="language-bash">{"npx hardhat build\nnpx hardhat test solidity"}</code></pre>
      <p>Hardhat treats the parameterized test as a fuzz test. Keep existing scaffold contracts compatible with your chosen compiler or remove the unused scaffold examples when you make the project your own.</p>
      <h2>Describe the deployment</h2>
      <p>Create <code>ignition/modules/PaxeerCounter.ts</code>. Ignition records the deployment and can resume its own deployment plan when invoked again.</p>
      <pre><code className="language-typescript">{"import { buildModule } from \"@nomicfoundation/hardhat-ignition/modules\";\n\nexport default buildModule(\"PaxeerCounterModule\", (m) => {\n  const counter = m.contract(\"PaxeerCounter\");\n  return { counter };\n});"}</code></pre>
      <p>Exercise the deployment against the simulated network first. The simulated network uses local accounts and state.</p>
      <pre><code className="language-bash">{"npx hardhat ignition deploy ignition/modules/PaxeerCounter.ts --network local"}</code></pre>
      <h2>Deploy to your Paxeer endpoint</h2>
      <p>This command broadcasts using the configured signer and spends PAX for gas. Hardhat checks the endpoint against the configured chain ID. Review the deployment confirmation and retain the resulting address and transaction record.</p>
      <pre><code className="language-bash">{"npx hardhat ignition deploy ignition/modules/PaxeerCounter.ts --network paxeer"}</code></pre>
      <p>Keep <code>ignition/deployments/chain-125/</code>, your source revision, and compiler configuration together. Read the receipt and deployed contract through the <Link href="/querying-evm">querying guide</Link>. For application tokens, configure the ERC-20 symbol and decimals independently of native PAX.</p>
      <p>For token contracts, the standard library package is <code>@openzeppelin/contracts</code>. Use a release compatible with your Solidity version; resolve imports such as <code>@openzeppelin/contracts/token/ERC20/ERC20.sol</code> against that installed package.</p>
      <p>References: <a href="https://hardhat.org/docs/reference/configuration">Hardhat network configuration</a>, <a href="https://hardhat.org/docs/guides/testing/using-solidity">Solidity tests</a>, and <a href="https://hardhat.org/docs/guides/deployment/using-ignition">Ignition deployments</a>.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>Paxeer-X-Network/</code> — contracts/hardhat.config.js and contracts/package.json (in-tree Hardhat 2, solc 0.8.27/Prague/1000), modules/evm/config/config.go (ChainIDMapping), and rpc/send.go (SendRawTransaction). This guide configures a new Hardhat 3 project.</div>
      <div className="prev-next">
        <Link href="/foundry"><div className="prev-next-label">Previous</div><div className="prev-next-title">Build with Foundry</div></Link>
        <Link href="/building-a-frontend"><div className="prev-next-label">Next</div><div className="prev-next-title">Build a Frontend</div></Link>
      </div>
    </DocsLayout>
  )
}
