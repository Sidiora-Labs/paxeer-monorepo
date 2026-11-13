import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/installation')

export default function Installation() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Installation &amp; Build</h1>
      </div>
      <p className="page-description">
        Build the Paxeer X node, deterministic LayerX kernel, hosted services, agent tools and Programs from the unified monorepo.
      </p>

      <div className="bg-surface-high border border-outline-variant rounded-lg px-4 py-3 mb-6">
        <strong className="text-on-surface">Source:</strong> <code>Makefile</code>, <code>chain.mk</code>, and <code>README.md</code> at the repository root
      </div>

      <DocArt name="installation" />
      <h2>Build the complete network stack</h2>
      <p>The monorepo contains the Go chain node, the C17 kernel, Rust service workspaces, SDKs, and program tooling. The network entry point is <code>paxd</code>, which embeds the kernel and proposes blocks. A receipt is final when its block commits.</p>
      <table><thead><tr><th>Component</th><th>Build surface</th><th>Purpose</th></tr></thead><tbody>
        <tr><td>Merged node</td><td><code>make paxd-build</code></td><td><code>build/paxd</code> with linked native libraries and release version stamp</td></tr>
        <tr><td>Kernel library and tools</td><td><code>make build</code></td><td>Kernel archive, genesis builder, and module registry, without rebuilding the node</td></tr>
        <tr><td>Agent services</td><td><code>make agent-build</code></td><td>Agent runtime and its service boundaries</td></tr>
        <tr><td>Human services</td><td><code>make human-build</code></td><td>Human API, providers, owner and security components</td></tr>
        <tr><td>Platform services</td><td><code>make platform-build-all</code></td><td>Hosted services, SDKs, integrations, and samples</td></tr>
        <tr><td>Programs and porting</td><td><code>make programs-build</code></td><td>Programs workspace and porting kits</td></tr>
      </tbody></table>
      <p>These commands run from the repository root. Use the Rust toolchain and dependency locks declared by each workspace, and the C compiler and native dependencies declared by the kernel build. Program guests target WebAssembly; install the target required by the selected guest before compiling it. The platform target also requires Node.js, Python, Maven/JVM, .NET and Swift, with dependencies prepared as declared in <code>services/platform/Makefile.inc</code>.</p>
      <p>Installation provides binaries and images. Joining a network also requires its genesis, network identity, and role-specific credentials. Retain build provenance and artifact checksums with the deployment record. Continue with <Link href="/run-node">node startup</Link> and <Link href="/operators">operator roles</Link>.</p>

      <h2>Prerequisites</h2>

      <ul>
        <li><strong>Go:</strong> 1.25.8, as declared in <code>go.mod</code>, or the toolchain required by the selected revision</li>
        <li><strong>Rust:</strong> 1.91.1, as declared in <code>rust-toolchain.toml</code></li>
        <li><strong>C compiler:</strong> C17 compiler for the kernel, via the root <code>Makefile</code></li>
        <li><strong>Build tools:</strong> Make and jq; the node also builds the native EVM library through <code>make evmone-lib</code> in <code>chain.mk</code></li>
        <li><strong>Git:</strong> For cloning the repository</li>
        <li><strong>Solidity:</strong> 0.8.27 with Foundry, for contract builds</li>
        <li><strong>Native toolchain:</strong> GCC, a C++ compiler and CMake for the native EVM library. The <code>evmone-lib</code> target builds that library on Linux/amd64 and fetches its declared source dependencies. Use <code>LEDGER_ENABLED=false</code> to disable the optional Ledger build tag.</li>
      </ul>

      <h2>Repository Location</h2>
      <p>Clone the <a href="https://github.com/Sidiora-Labs/paxeer-monorepo">Sidiora-Labs/paxeer-monorepo</a> repository. Chain sources, kernel sources, workspaces, and build configurations share this repository root. The root Go module is <code>github.com/Sidiora-Labs/Paxeer-X-Network</code>.</p>
      <pre><code>{`git clone https://github.com/Sidiora-Labs/paxeer-monorepo.git
cd paxeer-monorepo`}</code></pre>
      <h2>Build the Chain Node</h2>
      <p>Build the node from the repository root. <code>make paxd-build</code> builds <code>build/liblayerx.a</code>, <code>programs/target/release/liblayerx_programs_sandbox.a</code>, and the native EVM library before invoking <code>chain.mk</code>, and stamps the version from <code>tools/release/version.json</code>. The root node build disables Go module proxy downloads, so populate the declared dependency cache before using it.</p>
      <pre><code>{`make paxd-build        # build/paxd, linked native libraries and release version stamp
make paxeer-build
make paxeer-lint
make paxeer-test
make paxeer-ci`}</code></pre>
      <p><code>make paxeer-build</code> and <code>make -f chain.mk build</code> also build the linked native prerequisites and produce <code>build/paxd</code>, using the chain makefile&apos;s version resolver. Root <code>make build</code> builds the kernel archive, genesis builder, and module registry. Select the chain explicitly with <code>chain.mk</code>:</p>
      <pre><code>{`make -f chain.mk build
make -f chain.mk install
make -f chain.mk test
make -f chain.mk ci`}</code></pre>
      <h3>Build Output</h3>
      <ul>
        <li><code>build/paxd</code> from <code>make paxd-build</code>, <code>make paxeer-build</code>, or <code>make -f chain.mk build</code>.</li>
        <li><code>$GOBIN/paxd</code> when GOBIN is set, otherwise <code>$(go env GOPATH)/bin/paxd</code>, from <code>make -f chain.mk install</code>.</li>
        <li><code>make workspace-build</code> combines the kernel, services, node, app and tooling targets, and the root Foundry tree; <code>make monorepo-ci</code> delegates to the workspace CI target.</li>
      </ul>

      <h2>Build Tags and Flags</h2>

      <h3>Ledger Support</h3>

      <p>
        Ledger hardware wallet support is enabled by default in <code>chain.mk</code>. To build without ledger:
      </p>

      <pre><code>{`make paxeer-build LEDGER_ENABLED=false`}</code></pre>

      <h3>Mock Balances (Testing)</h3>

      <p>
        For testing with mock balances:
      </p>

      <pre><code>{`make -f chain.mk install-mock-balances`}</code></pre>

      <h3>Benchmark Build</h3>

      <p>
        For benchmark runs:
      </p>

      <pre><code>{`make -f chain.mk install-bench`}</code></pre>

      <h2>Version Information</h2>

      <p>
        The chain version resolver compares matching Git tags and versioned branch names, and uses <code>tools/release/version.json</code> when no matching tag exists. Its tag pattern is:
      </p>

      <pre><code>{`paxeer-network/vX.Y.Z`}</code></pre>

      <p>
        Version information is embedded in the binary at build time via linker flags:
      </p>

      <ul>
        <li><code>Name</code>: paxeer</li>
        <li><code>AppName</code>: paxd</li>
        <li><code>Version</code>: The selected version; <code>make paxd-build</code> selects it from <code>tools/release/version.json</code></li>
        <li><code>Commit</code>: Git commit SHA</li>
      </ul>

      <p>
        Check version:
      </p>

      <pre><code>{`paxd version`}</code></pre>

      <h2>Static Linking</h2>

      <p>
        Request static external linking for the Go binary with the flag below. The native EVM loader in <code>engine/executor/lib/evmlib.go</code> still loads a platform-specific shared library:
      </p>

      <pre><code>{`make paxeer-build LINK_STATICALLY=true`}</code></pre>

      <h2>Go Module</h2>

      <p>
        The repository root declares the chain Go module:
      </p>

      <pre><code>{`module github.com/Sidiora-Labs/Paxeer-X-Network

go 1.25.8`}</code></pre>

      <div className="source-note">
        <strong>Note:</strong> The root <code>go.mod</code> and <code>go.sum</code> govern the chain Go dependencies. Rust components use their own workspace manifests and lockfiles.
      </div>

      <h2>Foundry (Contracts)</h2>
      <p>Chain contracts under <code>contracts/src/</code> use <code>foundry.paxeer.toml</code>. Fetch the pinned dependencies and build from the repository root:</p>
      <pre><code>{`bash contracts/bootstrap-libs.sh
FOUNDRY_CONFIG=foundry.paxeer.toml forge build --offline --root .`}</code></pre>
      <p>Kernel contracts also live under <code>contracts/</code>, using repository-root <code>foundry.toml</code> and <code>tests/solidity/</code>. See <code>contracts/README.md</code> and the <Link href="/contracts">contracts guide</Link> for the two project configurations.</p>

      <h2>Docker</h2>

      <p>
        Local Docker cluster targets are available. See <Link href="/docker">Docker documentation</Link> for:
      </p>

      <ul>
        <li><code>make docker-cluster-start</code></li>
        <li><code>make run-local-node</code></li>
        <li>Compose configurations in <code>docker/</code></li>
      </ul>

      <h2>Choose a Node Configuration</h2>

      <p>
        Use the local Docker configuration to try the built binary, or initialize a node with the genesis, peers, and mode for your chosen network. For an existing network, select the binary revision specified by its upgrade plan.
      </p>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/run-node">Run a node</Link></li>
        <li><Link href="/configuration">Configure paxd</Link></li>
        <li><Link href="/docker">Use Docker for local testing</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/network-parameters">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Network Parameters</div>
        </Link>
        <Link href="/run-node">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Run a Node</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
