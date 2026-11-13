import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/hardhat')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Build with Hardhat</h1>
        <p className="page-description">Use the repository&apos;s Hardhat 2 project to compile contracts and interact with a development node.</p>
      </div>
      <p>The contract project at <code>contracts/</code> uses Hardhat 2 and a CommonJS configuration. Its sources live under <code>contracts/src/</code>. Paxeer EVM chain ID is <code>125</code>, mapped from <code>hyperpax_125-1</code> in <code>modules/evm/config/config.go</code>.</p>
      <h2>Use the in-tree project</h2>
      <p>Use a <a href="https://hardhat.org/hardhat2">Node.js version supported by Hardhat 2</a>. Install the dependencies recorded in <code>contracts/package-lock.json</code> and inspect the local tool version.</p>
      <pre><code className="language-bash">{"cd contracts\nnpm ci\nnpx hardhat --version"}</code></pre>
      <p><code>contracts/package.json</code> declares <code>hardhat ^2.20.1</code>, <code>@nomicfoundation/hardhat-toolbox ^6.1.2</code>, and <code>@openzeppelin/hardhat-upgrades ^3.0.2</code>. The lockfile records the resolved dependency versions.</p>
      <h2>Configure the development endpoint</h2>
      <p><code>contracts/hardhat.config.js</code> selects Solidity <code>0.8.27</code>, the <code>prague</code> target, and an enabled optimizer with <code>1000</code> runs. Its network configuration reads <code>PAXEER_LOCAL_EVM_RPC_URL</code>. The following excerpt shows the compiler and source settings.</p>
      <pre><code className="language-typescript">{"module.exports = {\n  solidity: {\n    version: \"0.8.27\",\n    settings: {\n      evmVersion: \"prague\",\n      optimizer: { enabled: true, runs: 1000 },\n    },\n  },\n  paths: { sources: \"./src\" },\n};"}</code></pre>
      <p>The configured HTTP network is named <code>paxlocal</code>. Set <code>PAXEER_LOCAL_EVM_RPC_URL</code> to your development node&apos;s root EVM endpoint before connecting. The default uses the loopback interface on port <code>8545</code>.</p>
      <p>The configuration loads <code>contracts/.env</code> through <code>dotenv</code> and reads the endpoint from the environment. The following command requires that endpoint to be set.</p>
      <pre><code className="language-bash">{": \"${PAXEER_LOCAL_EVM_RPC_URL:?Set the development node EVM endpoint}\""}</code></pre>
      <h2>Read sources and run the compiler check</h2>
      <p><code>contracts/src/Box.sol</code> is an existing storage example. It exposes <code>store</code>, <code>retrieve</code>, and <code>boxIncr</code> and emits <code>ValueChanged</code>.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity ^0.8.0;\n\ncontract Box {\n    uint256 private _value;\n\n    event ValueChanged(uint256 value);\n\n    function store(uint256 value) public {\n        _value = value;\n        emit ValueChanged(value);\n    }\n\n    function retrieve() public view returns (uint256) {\n        return _value;\n    }\n\n    function boxIncr() public {\n        _value = _value + 1;\n        emit ValueChanged(_value);\n    }\n}"}</code></pre>
      <p>Token contracts use the installed OpenZeppelin package. For example, this is the import and constructor in <code>contracts/src/TestToken.sol</code>.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity ^0.8.20;\n\nimport \"@openzeppelin/contracts/token/ERC20/ERC20.sol\";\nimport \"@openzeppelin/contracts/access/Ownable.sol\";\n\ncontract TestToken is ERC20, Ownable {\n    constructor(string memory name, string memory symbol) Ownable(msg.sender) ERC20(name, symbol) {\n        _mint(msg.sender, 1000 * (10 ** uint256(decimals())));\n    }\n\n    // The owner adjusts balances by minting or burning\n    function setBalance(address account, uint256 amount) public onlyOwner {\n        uint256 currentBalance = balanceOf(account);\n        if (amount > currentBalance) {\n            _mint(account, amount - currentBalance);\n        } else if (amount < currentBalance) {\n            _burn(account, currentBalance - amount);\n        }\n    }\n}"}</code></pre>
      <pre><code className="language-bash">{"npx hardhat compile\nnode --test test/hardhat_solc_parity.test.js"}</code></pre>
      <p>The focused Node test <code>contracts/test/hardhat_solc_parity.test.js</code> checks that the Hardhat compiler version matches <code>foundry.paxeer.toml</code>. Run it from <code>contracts/</code> after compiling.</p>
      <h2>Interact through the console</h2>
      <p>In the Hardhat console, the following Ethers calls deploy the existing Box artifact, wait for deployment, write a value, and read it. Deployment and writes require a signer exposed by the development node.</p>
      <pre><code className="language-typescript">{"const box = await ethers.deployContract(\"Box\");\nawait box.waitForDeployment();\nconst tx = await box.store(7);\nconst receipt = await tx.wait();\nawait box.retrieve();"}</code></pre>
      <p>Open the console with the configured network. Confirm that it is the intended development node before running the deployment expressions above.</p>
      <pre><code className="language-bash">{"npx hardhat console --network paxlocal"}</code></pre>
      <h2>Check the connected chain</h2>
      <p>In that console, inspect the endpoint&apos;s chain ID before submitting transactions. Paxeer uses <code>125</code>. Deployment and storage writes spend the configured signer&apos;s gas balance.</p>
      <pre><code className="language-bash">{"const network = await ethers.provider.getNetwork();\nnetwork.chainId;"}</code></pre>
      <p>Retain the contract address, transaction receipt, source, and compiler settings. Read the result through the <Link href="/querying-evm">querying guide</Link>. Hardhat writes the Box artifact to <code>contracts/artifacts/src/Box.sol/Box.json</code>.</p>
      <p><code>contracts/package.json</code> declares <code>@openzeppelin/contracts ^5.4.0</code>. Its <code>TestToken</code> source imports <code>@openzeppelin/contracts/token/ERC20/ERC20.sol</code>.</p>
      <p>References: <a href="https://hardhat.org/hardhat2/hardhat-runner/docs/config">Hardhat 2 configuration</a>, <a href="https://hardhat.org/hardhat2/hardhat-runner/docs/guides/test-contracts">contract tests</a>, and <a href="https://hardhat.org/hardhat2/hardhat-runner/docs/guides/deploying">contract deployment</a>.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>contracts/hardhat.config.js</code>, <code>contracts/package.json</code>, <code>contracts/src/Box.sol</code>, <code>contracts/src/TestToken.sol</code>, and <code>contracts/test/hardhat_solc_parity.test.js</code>.</div>
      <div className="prev-next">
        <Link href="/foundry"><div className="prev-next-label">Previous</div><div className="prev-next-title">Build with Foundry</div></Link>
        <Link href="/building-a-frontend"><div className="prev-next-label">Next</div><div className="prev-next-title">Build a Frontend</div></Link>
      </div>
    </DocsLayout>
  )
}
