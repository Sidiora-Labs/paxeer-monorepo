import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/sdk')

export default function SDK() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Cosmos SDK Fork</h1>
        <p className="page-description">
          Paxeer X’s in-tree Cosmos SDK fork with Paxeer X-specific modifications and extensions.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/sdk/</code>
      </div>

      <h2>Choose the right development surface</h2>
      <p>Paxeer X has three complementary SDK surfaces. The in-tree Cosmos SDK powers <code>paxd</code> and native chain modules. The <Link href="/xdk">XDK</Link> provides application-facing clients for LayerX transactions, proofs, identity, exchange, bridge, launchpad, and native precompile calls. The <Link href="/platform-api">platform API</Link> connects paid resources, merchant checkout, agent spending, receipts, and webhooks.</p>
      <p>Chain maintainers work in <code>sdk/</code>, <code>modules/</code>, and <code>node/</code>. Application developers use <code>services/agent/sdk/</code> (TypeScript, Python), <code>services/agent/crates/layerx-sdk</code> (Rust), <code>sdk/platform/</code> (Go, JVM, Swift, .NET), and <code>sdk/wallet</code>. <code>docs/system/sdk.md</code> indexes exported SDK symbols and their source paths.</p>
      <h3>Cross-surface application lifecycle</h3>
      <ol><li>Resolve the unified EVM, Cosmos, and LayerX identity and select the asset denomination or pointer.</li><li>Use the matching client or native ABI to deposit custody funds, submit an intent, transfer an asset, or create a market.</li><li>Track transaction receipts together with kernel receipts, checkpoint finality, and withdrawal or settlement status.</li><li>Verify proofs against the appropriate finalized root before fulfilling a payment or releasing funds.</li></ol>
      <p>The <Link href="/unified-network">network overview</Link> explains how these execution surfaces compose. An EVM transaction receipt and a finalized kernel checkpoint represent different stages of an application lifecycle.</p>

      <h2>Overview</h2>

      <p>
        Paxeer X vendors a fork of the Cosmos SDK directly in the monorepo under <code>Paxeer-X-Network/sdk/</code>. This is <strong>not</strong> a published Go module or npm package. It is an in-tree dependency for the <code>paxd</code> binary and Paxeer chain modules.
      </p>

      <h3>Why a Fork?</h3>

      <p>
        Paxeer X maintains a fork to support:
      </p>

      <ul>
        <li>Custom storage engines</li>
        <li>EVM integration and pointer contracts</li>
        <li>Fast iteration without waiting for upstream releases</li>
        <li>Paxeer X-specific consensus and execution optimizations</li>
        <li>Co-location with Paxeer modules and WASM runtime</li>
      </ul>

      <h2>Upstream Base</h2>

      <p>
        The fork is based on Cosmos SDK but diverges significantly. It is not compatible with upstream Cosmos SDK releases without careful rebasing and testing.
      </p>

      <div className="source-note">
        <strong>Warning:</strong> Do not assume drop-in compatibility with standard Cosmos SDK tooling or libraries. Paxeer X’s SDK is a custom fork.
      </div>

      <h2>Directory Structure</h2>

      <table>
        <thead>
          <tr>
            <th>Path</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>baseapp/</code></td>
            <td>Application framework (ABCI, routing, ante handlers)</td>
          </tr>
          <tr>
            <td><code>client/</code></td>
            <td>CLI framework and transaction builders</td>
          </tr>
          <tr>
            <td><code>server/</code></td>
            <td>Node server, gRPC, REST gateway</td>
          </tr>
          <tr>
            <td><code>types/</code></td>
            <td>Core SDK types (messages, coins, errors, context)</td>
          </tr>
          <tr>
            <td><code>x/</code></td>
            <td>Standard Cosmos modules (bank, auth, staking, gov, etc.)</td>
          </tr>
          <tr>
            <td><code>proto/</code></td>
            <td>Protobuf definitions for SDK modules</td>
          </tr>
          <tr>
            <td><code>store/</code></td>
            <td>State store abstraction and implementations</td>
          </tr>
          <tr>
            <td><code>storev2/</code>, <code>codec/</code>, <code>crypto/</code></td>
            <td>Store v2, codec, and cryptography support</td>
          </tr>
          <tr>
            <td><code>platform/</code>, <code>wallet/</code>, <code>js/</code></td>
            <td>Platform language SDKs, wallet, and JavaScript support</td>
          </tr>
        </tbody>
      </table>

      <h2>Key Modules</h2>

      <p>
        Paxeer X’s SDK includes standard Cosmos modules:
      </p>

      <table>
        <thead>
          <tr>
            <th>Module</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>x/auth</code></td>
            <td>Account authentication and transaction signing</td>
          </tr>
          <tr>
            <td><code>x/bank</code></td>
            <td>Token transfers and balance management</td>
          </tr>
          <tr>
            <td><code>x/staking</code></td>
            <td>Proof-of-Stake validator set and delegation</td>
          </tr>
          <tr>
            <td><code>x/distribution</code></td>
            <td>Staking rewards and fee distribution</td>
          </tr>
          <tr>
            <td><code>x/gov</code></td>
            <td>On-chain governance and proposals</td>
          </tr>
          <tr>
            <td><code>x/slashing</code></td>
            <td>Validator penalties for misbehavior</td>
          </tr>
          <tr>
            <td><code>x/authz</code>, <code>x/feegrant</code></td>
            <td>Authorization grants and fee grants</td>
          </tr>
          <tr>
            <td><code>x/capability</code>, <code>x/genutil</code>, <code>x/simulation</code></td>
            <td>Capability keeper, genesis utilities, and simulation</td>
          </tr>
          <tr>
            <td><code>x/evidence</code></td>
            <td>Double-sign evidence submission</td>
          </tr>
          <tr>
            <td><code>x/params</code></td>
            <td>Module parameter management</td>
          </tr>
          <tr>
            <td><code>x/upgrade</code></td>
            <td>Coordinated chain upgrades</td>
          </tr>
        </tbody>
      </table>

      <p>
        Plus Paxeer X-specific modules under <code>Paxeer-X-Network/modules/</code>:
      </p>

      <ul>
        <li><code>evm</code> — EVM execution and JSON-RPC</li>
        <li><code>epoch</code> — Epoch tracking</li>
        <li><code>oracle</code>, <code>pricefeed</code> — Price feeds</li>
        <li><code>tokenfactory</code> — Custom token creation</li>
        <li><code>mint</code> — Token minting</li>
        <li><code>launchpad</code>, <code>spot</code>, <code>perps</code>, <code>sequencer</code>, <code>store</code>, <code>xweb</code>, <code>lxphost</code></li>
        <li><code>layerxagent</code>, <code>layerxanchor</code>, <code>layerxauth</code>, <code>layerxbridge</code>, <code>layerxchains</code>, <code>layerxcustody</code>, <code>layerxexchange</code>, <code>layerxgov</code>, <code>layerxledger</code>, <code>layerxpay</code></li>
      </ul>

      <h2>BaseApp</h2>

      <p>
        The core application framework (<code>baseapp/</code>) handles:
      </p>

      <ul>
        <li>ABCI interface to Tendermint consensus</li>
        <li>Message routing to module handlers</li>
        <li>Ante handlers (gas, signatures, nonces)</li>
        <li>Transaction execution and state commits</li>
        <li>Query routing</li>
      </ul>

      <p>
        Paxeer extends BaseApp for EVM transaction routing and concurrent execution (OCC).
      </p>

      <h2>Client & Server</h2>

      <p>
        The SDK provides:
      </p>

      <ul>
        <li><strong>CLI:</strong> <code>paxd</code> command-line interface (transaction builders, queries)</li>
        <li><strong>gRPC:</strong> Module query and transaction services</li>
        <li><strong>REST:</strong> HTTP gateway (gRPC-Gateway)</li>
        <li><strong>Tendermint RPC:</strong> Block and transaction queries</li>
      </ul>

      <p>
        See <Link href="/rest-grpc">REST & gRPC</Link> for API documentation.
      </p>

      <h2>State Store</h2>

      <p>
        The SDK's <code>store/</code> abstraction is implemented by Paxeer X’s custom storage engines. The engine implementations live under <code>storage/</code>:
      </p>

      <ul>
        <li>Database engines under <code>storage/db_engine/</code>: <code>litt</code>, <code>pebbledb</code>, <code>rocksdb</code>, and <code>dbcache</code>.</li>
      </ul>

      <p>
        See <Link href="/storage">Storage</Link> for engine details.
      </p>

      <h2>Protobuf Definitions</h2>

      <p>
        SDK types are defined in <code>sdk/proto/</code>. Regenerate Go code with:
      </p>

      <pre><code>{`cd Paxeer-X-Network
./scripts/protoc.sh`}</code></pre>

      <p>
        The script builds the pinned <code>protoc-gen-gocosmos</code> plugin and runs <code>buf generate</code> via <code>go run github.com/bufbuild/buf/cmd/buf@v1.58.0</code>. See <code>Paxeer-X-Network/api/README.md</code>.
      </p>

      <h2>Building Against the SDK</h2>

      <p>
        Paxeer modules import the SDK from the in-tree path:
      </p>

      <pre><code>{`import (
    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
)`}</code></pre>

      <p>
        There is no <code>sdk/go.mod</code>: the SDK packages belong to the single root module <code>github.com/Sidiora-Labs/Paxeer-X-Network</code>, so no replacement directives are needed.
      </p>

      <h2>Makefile Targets</h2>

      <p>
        The repository Makefile includes SDK-related targets:
      </p>

      <pre><code>{`# Build paxd from the unified repository root
make paxeer-build

# Run the chain tests
make paxeer-test`}</code></pre>

      <h2>Documentation</h2>

      <p>
        There is no <code>sdk/docs/</code> tree in the fork. Use <code>docs/system/sdk.md</code> in the monorepo as the index of exported SDK symbols and source paths.
      </p>

      <h3>Upstream Resources</h3>

      <ul>
        <li><a href="https://docs.cosmos.network/">Cosmos SDK Docs</a> (upstream, may diverge from Paxeer)</li>
        <li><a href="https://tutorials.cosmos.network/">Cosmos Tutorials</a> (upstream concepts apply)</li>
        <li><a href="https://pkg.go.dev/github.com/cosmos/cosmos-sdk">GoDoc</a> (upstream SDK, not Paxeer X’s fork)</li>
      </ul>

      <div className="source-note">
        <strong>Warning:</strong> Upstream documentation describes the standard Cosmos SDK, not Paxeer X’s fork. Treat it as reference only.
      </div>

      <h2>Contributing to the Fork</h2>

      <p>
        Changes to <code>Paxeer-X-Network/sdk/</code> affect the entire Paxeer chain. Test thoroughly:
      </p>

      <ol>
        <li>Edit code under <code>sdk/</code></li>
        <li>Rebuild <code>paxd</code>: <code>make paxeer-build</code></li>
        <li>Run unit tests: <code>make test</code></li>
        <li>Test with <Link href="/docker">Docker cluster</Link>: <code>make docker-cluster-start</code></li>
        <li>Run integration tests: <code>make human-test-integration</code> (there is no <code>test-integration</code> target)</li>
      </ol>

      <h2>Differences from Upstream Cosmos SDK</h2>

      <p>
        Paxeer X’s SDK fork diverges from upstream in several areas:
      </p>

      <ul>
        <li>Custom storage backends</li>
        <li>EVM module integration and pointer contracts</li>
        <li>Optimistic concurrency control (OCC) for parallel execution</li>
        <li>Receipt indexing and synthetic transactions</li>
        <li>Paxeer X-specific ante handlers and gas metering</li>
        <li>Chain-specific upgrade logic</li>
      </ul>

      <p>
        Do not assume compatibility with standard Cosmos SDK tooling (e.g., CosmJS, Keplr) without testing.
      </p>

      <h2>Go Module Path</h2>

      <pre><code>{`github.com/Sidiora-Labs/Paxeer-X-Network`}</code></pre>

      <p>
        There is no separate SDK module. Paths such as <code>github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types</code> are package paths inside the single root module, not a standalone published module.
      </p>

      <div className="prev-next">
        <Link href="/docker">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Docker</div>
        </Link>
        <Link href="/interchain">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Interchain</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
