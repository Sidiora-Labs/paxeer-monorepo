import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/contracts')

export default function Contracts() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Contracts</h1>
        <p className="page-description">
          Build Solidity applications on Paxeer X and integrate wrapped PAX, token pointers, and native precompile interfaces.
        </p>
      </div>

      <h2>Custody and settlement</h2>
      <p>The pre-merge LayerX settlement contract family is retired. <code>LayerXCustody.sol</code>, <code>CheckpointRegistry.sol</code>, <code>GuarantorBond.sol</code>, <code>WithdrawalClaims.sol</code>, and <code>EmergencyExit.sol</code> no longer exist as deployable contracts and are not deployment requirements for the merged network; the surviving <code>LayerXCustody.sol</code> is the interface for the native custody precompile. Current funding uses native bank balances and derived sub-accounts, and the chain exposes custody and anchor modules through fixed precompile addresses.</p>
      <p>Choose the interface for the custody route your application uses and verify its authority and deployment configuration. A deployed Solidity contract address and a fixed native precompile address have distinct call targets and state. The <Link href="/settlement">settlement guide</Link> explains checkpoints, proofs, challenge windows, and exits.</p>
      <h3>Native application composition</h3>
      <p>Contracts call custody at <code>0x1013</code>, anchor at <code>0x1014</code>, perpetuals at <code>0x1015</code> (from the v6.12.0 plan; below it, that address held the historical exchange implementation), bridge at <code>0x1016</code>, and launchpad at <code>0x1017</code>. The verification precompile at <code>0x1012</code> verifies LayerX proofs; fee-token selection uses <code>0x1018</code>, and XWeb requests use <code>0x1019</code>. Import the exact ABI or Solidity interface for each call and preserve integer units, authorization requirements, and event decoding.</p>
      <p>Combine <Link href="/assets">native denominations and token pointers</Link> with <Link href="/bridge">bridged assets</Link>, or use the <Link href="/xdk">XDK</Link> to prepare calls and consume application events.</p>
      <h3>Contract project boundaries</h3>
      <p>Chain contracts under <code>contracts/src/</code> use <code>foundry.paxeer.toml</code> (sources <code>contracts/src</code>, tests <code>contracts/test</code>, artifacts <code>contracts/out</code>). The repository-root <code>foundry.toml</code> is a separate project (sources <code>contracts</code>, artifacts <code>build/forge-artifacts</code>) that explicitly skips <code>contracts/src/**</code> and <code>contracts/test/**</code>; its <code>tests/solidity/</code> suite now covers only governance and names. Select the configuration matching the contract family when compiling or testing.</p>

      <h2>Start a New Application</h2>
      <p>Follow the <Link href="/foundry">Foundry</Link> or <Link href="/hardhat">Hardhat</Link> walkthrough to compile, test, and deploy a contract on chain ID 125 (<code>hyperpax_125-1</code>; other networks use their own IDs). Continue with <Link href="/building-a-frontend">Build a Frontend</Link> to connect a browser wallet and read deployed state.</p>
      <p>The Paxeer X contract library provides Solidity interfaces and implementations for native token operations and Cosmos integration. Its sources live in <code>Paxeer-X-Network/contracts/src/</code>.</p>

      <h2>Wrapped PAX and Token Pointers</h2>
      <table>
        <thead><tr><th>Contract</th><th>Use</th></tr></thead>
        <tbody>
          <tr><td><code>WPAX.sol</code></td><td>Wrap native PAX in an ERC-20-style interface with 18 decimals</td></tr>
          <tr><td><code>NativePaxTokensERC20.sol</code></td><td>Read and transfer a native Cosmos denomination through an ERC-20 interface</td></tr>
          <tr><td><code>CW20ERC20Pointer.sol</code></td><td>Interact with a CW20 token through the ERC-20 interface</td></tr>
          <tr><td><code>CW721ERC721Pointer.sol</code></td><td>Interact with a CW721 collection through the ERC-721 interface</td></tr>
          <tr><td><code>CW1155ERC1155Pointer.sol</code></td><td>Interact with CW1155 multi-tokens through the ERC-1155 interface</td></tr>
        </tbody>
      </table>

      <h3>Wrap and Unwrap PAX</h3>
      <p><code>WPAX.deposit()</code> accepts native PAX through <code>msg.value</code> and credits the caller’s WPAX balance. <code>withdraw(amount)</code> reduces that balance and returns native PAX. The contract also exposes transfers, allowances, and approval events for token integrations.</p>
      <p>Use the WPAX deployment address published for your target network. Read its <code>symbol()</code>, <code>decimals()</code>, and deployed bytecode before configuring an application integration. Keep native PAX available to pay transaction fees.</p>

      <h3>Use a Token Pointer</h3>
      <p>A pointer exposes an underlying Cosmos asset through familiar EVM token methods. The native-token implementation delegates balance, supply, and transfer operations to the bank precompile. CW pointers call the corresponding CosmWasm contract.</p>
      <p>Resolve the registered pointer and its version for the underlying denomination or contract. Use the <Link href="/precompiles">pointer and pointer-view precompiles</Link> for registration and discovery, then read the token’s metadata through its own ABI.</p>

      <h2>Native Precompile Interfaces</h2>
      <p>Import the Solidity interface matching the operation you need. These native precompiles use fixed addresses registered by Paxeer X; the interfaces are in <code>contracts/src/precompiles/</code>.</p>
      <table>
        <thead><tr><th>Interface</th><th>Address</th><th>Operations</th></tr></thead>
        <tbody>
          <tr><td><code>IBank.sol</code></td><td><code>0x0000000000000000000000000000000000001001</code></td><td>Native-token balances, supply, metadata, and transfers</td></tr>
          <tr><td><code>IWasmd.sol</code></td><td><code>0x0000000000000000000000000000000000001002</code></td><td>Instantiate, execute, and query CosmWasm contracts</td></tr>
          <tr><td><code>IJson.sol</code></td><td><code>0x0000000000000000000000000000000000001003</code></td><td>Extract bytes, byte arrays, and integers from JSON</td></tr>
          <tr><td><code>IAddr.sol</code></td><td><code>0x0000000000000000000000000000000000001004</code></td><td>Look up associated EVM and Cosmos addresses</td></tr>
        </tbody>
      </table>
      <p>For example, read the Cosmos address associated with an EVM account:</p>
      <pre><code className="language-bash">{`cast call 0x0000000000000000000000000000000000001004 \\
  'getPaxAddr(address)(string)' "$ACCOUNT_ADDRESS" \\
  --rpc-url "$PAXEER_RPC_URL"`}</code></pre>
      <p>Set <code>ACCOUNT_ADDRESS</code> to an associated account. If the call reports a missing association, follow <Link href="/precompiles">Address Conversion</Link> to associate the account first. The reverse lookup is <code>getEvmAddr(string)</code>.</p>

      <h2>Work with the Contract Library</h2>
      <p>The repository includes Foundry and Hardhat tooling. Use the configuration for the artifact you are rebuilding and retain its compiler settings alongside the output.</p>
      <table>
        <thead><tr><th>Tool</th><th>Configuration</th><th>Project layout</th></tr></thead>
        <tbody>
          <tr><td>Foundry</td><td><code>Paxeer-X-Network/foundry.paxeer.toml</code></td><td>Sources in <code>contracts/src/</code>, tests in <code>contracts/test/</code>, artifacts in <code>contracts/out/</code></td></tr>
          <tr><td>Hardhat</td><td><code>Paxeer-X-Network/contracts/hardhat.config.js</code></td><td>Hardhat 2 project with its dependencies and lockfile under <code>contracts/</code></td></tr>
        </tbody>
      </table>
      <p>The Hardhat <code>paxlocal</code> network reads <code>PAXEER_LOCAL_EVM_RPC_URL</code> and defaults to <code>http://127.0.0.1:8545</code>. Start a <Link href="/docker">Paxeer development cluster</Link> for integration tests that call native precompiles or CosmWasm modules.</p>
      <p>Use the <Link href="/foundry">Foundry</Link> and <Link href="/hardhat">Hardhat</Link> guides for a complete new-project setup, including compiler configuration, local tests, and deployment commands.</p>

      <h3>Choose the Relevant Tests</h3>
      <ul>
        <li><code>CW20toERC20PointerTest.js</code> and <code>NativePaxTokensERC20Test.t.sol</code> cover token-pointer behavior.</li>
        <li><code>EVMCompatibilityTester.sol</code>, <code>TransientStorageTester.sol</code>, and <code>SelfDestructTester.sol</code> exercise EVM behavior.</li>
        <li><code>SstoreGasTest.sol</code> and <code>SnapshotRevertTester.sol</code> exercise storage gas and state restoration.</li>
        <li><code>MultiSender.sol</code> and <code>BatchCallAndSponsor.sol</code> support batch-call scenarios.</li>
      </ul>

      <h2>Maintain Embedded Pointer Artifacts</h2>
      <p>Chain-level pointer implementations include versioned ABI and bytecode artifacts under <code>modules/evm/artifacts/</code>. Go embeds these files into the node binary. When changing a pointer implementation:</p>
      <ol>
        <li>Compile the contract using the intended compiler and optimizer settings, and run its contract tests.</li>
        <li>Update the matching ABI and creation bytecode in the <code>native</code>, <code>cw20</code>, <code>cw721</code>, <code>cw1155</code>, <code>erc20</code>, <code>erc721</code>, <code>erc1155</code>, or <code>wpax</code> artifact package.</li>
        <li>Update the pointer version and migration logic required by the release. Registry lookups select pointers by type, underlying asset, and version.</li>
        <li>Rebuild <code>paxd</code> to embed the artifacts and exercise registration, transfers, and existing-pointer behavior in a development cluster.</li>
        <li>Release the binary through the network’s upgrade process, including the migration for any existing deployed pointers.</li>
      </ol>
      <p>Application developers can use the registered pointer interface directly. Embedded-artifact updates are part of maintaining the chain implementation.</p>

      <h2>Connect a Deployed Contract</h2>
      <p>Obtain the address from your deployment output or the contract maintainer’s network-specific deployment record. Match your network&apos;s chain ID (125 on <code>hyperpax_125-1</code>), the contract ABI, and the deployed version before adding the address to your application.</p>
      <pre><code className="language-bash">{`cast chain-id --rpc-url "$PAXEER_RPC_URL"
cast receipt "$TX_HASH" --rpc-url "$PAXEER_RPC_URL"
cast code "$CONTRACT_ADDRESS" --rpc-url "$PAXEER_RPC_URL"`}</code></pre>
      <p>Retain the creation transaction, source revision, compiler settings, constructor arguments, and ABI. For a proxy contract, also retain the implementation address and upgrade configuration. Continue with <Link href="/querying-evm">Query the EVM</Link> to read state and events.</p>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/contracts/src/</code>, <code>contracts/hardhat.config.js</code>, <code>foundry.paxeer.toml</code>, <code>precompiles/</code>, and <code>modules/evm/artifacts/</code> with <code>keeper/pointer.go</code>.
      </div>
      <div className="prev-next">
        <Link href="/rest-grpc"><div className="prev-next-label">Previous</div><div className="prev-next-title">REST &amp; gRPC</div></Link>
        <Link href="/docker"><div className="prev-next-label">Next</div><div className="prev-next-title">Docker</div></Link>
      </div>
    </DocsLayout>
  )
}
