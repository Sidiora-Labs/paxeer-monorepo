import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/program-workspace')

export default function ProgramWorkspacePage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Programs Developer Workspace</h1><p className="page-description">Build deterministic Paxeer X programs with runtime crates, language SDKs, fixtures, policies and reproducible development workflows.</p></div>
    <h2>Start with the execution contract</h2>
    <p>The Programs workspace contains the deterministic WebAssembly execution surface for the LayerX kernel domain of Paxeer X. Applications deploy artifacts and interfaces, execute under explicit capabilities, and express monetary effects as authenticated native transfer sets. Read <Link href="/programs">Programs</Link> before choosing entrypoints, account seeds, and spending grants.</p>
    <p>Rust workspace packages use edition 2021 and declare a minimum Rust version of 1.91.1. Cargo resolves dependencies from the checked-in vendor tree in offline mode, and the interpreter is pinned to <code>wasmi = 0.31.2</code>. Run Cargo from inside <code>programs/</code> so its local configuration applies. Lockfiles, source checksums, and frozen ABI vectors make builds and byte contracts reproducible.</p>
    <DocArt name="resource-browser" />
    <h2>Choose the right package</h2>
    <table><thead><tr><th>Workspace area</th><th>Responsibility</th></tr></thead><tbody>
      <tr><td><code>programs/crates/layerx-programs-runtime</code></td><td>Validation, budgets, metering, host imports, storage, cross-program calls, transfer authorization, occupancy, replay, and kernel FFI.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-registry</code></td><td>Receipt-bound deployment journals, interfaces, program account bindings, account proofs, and lifecycle exits. Its <code>chain_proof</code> module verifies committed chain evidence for program-state reads.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-ai-market</code></td><td>PAXAI schema-1 common contracts and the direct native Programs entrypoint.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-interpreter</code></td><td>Bounded deterministic scripts with canonical input validation before effects.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-market</code></td><td>Compute leases, usage claims, challenge windows, provider stake, attesters, and bisection disputes.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-arbiter</code></td><td>Sandbox trace replay and disputed single-step adjudication.</td></tr>
      <tr><td><code>programs/crates/layerx-programs-sandbox</code></td><td>Bounded lease lifecycle, isolated execution, snapshots, and restore.</td></tr>
      <tr><td><code>programs/sdk/</code> and <code>programs/porting/</code></td><td>Rust, C, and AssemblyScript guests; migration guides and reference guests for EVM, Solana, and CosmWasm developers.</td></tr>
      <tr><td><code>programs/fixtures/</code>, <code>programs/tests/</code>, <code>programs/fuzz/</code>, <code>programs/benches/</code></td><td>Canonical artifacts, cross-implementation vectors, hostile guests, fuzz corpora, and interpreter comparisons.</td></tr>
    </tbody></table>
    <h2>Build a guest application</h2>
    <ol><li>Choose a guest SDK and a reference application close to your use case.</li><li>Define canonical calldata and responses, entrypoint bounds, storage scopes, and required grants. Select the guest ABI that supplies those host imports.</li><li>Keep state transitions deterministic. Consume admitted oracle or web evidence through host calls rather than adding a network client to execution.</li><li>Compile a WASM artifact, generate its canonical interface, and retain the artifact hash and interface together.</li><li>Deploy with the chosen upgrade policy, verify the receipt, register any program-owned accounts, and fund them through native transfers.</li><li>Simulate a bounded call, submit the signed activity, and verify committed terminal evidence.</li></ol>
    <p>Rust examples cover escrow, naming, LXT-721, merchant payments, constant-product swaps, LXT-20, vaults, and web readers. C and AssemblyScript provide paid-counter guests. A reference application is an executable contract: preserve its documented selectors and authorization requirements when adapting it.</p>
    <pre><code>{`# From the repository root
cd programs
cargo build --locked --manifest-path sdk/rust/examples/token-lxt20/Cargo.toml \\
  --target wasm32-unknown-unknown --release`}</code></pre>
    <h2>Build and validate the workspace</h2>
    <p>The repository Makefile coordinates Rust and native C checks. Run these commands from the repository root; they enter the Programs directory where required.</p>
    <pre><code>{`make programs-build
make programs-lint
make programs-abi-drift
make programs-test`}</code></pre>
    <table><thead><tr><th>Gate</th><th>What it establishes</th></tr></thead><tbody>
      <tr><td><code>programs-build</code></td><td>Builds the locked workspace and sandbox host FFI used by native integration.</td></tr>
      <tr><td><code>programs-lint</code></td><td>Checks runtime module boundaries, Clippy, dependency and vendoring policy, advisories, sources, and dependency bans.</td></tr>
      <tr><td><code>programs-abi-drift</code></td><td>Checks frozen vectors and the runtime linker against the canonical guest import contract.</td></tr>
      <tr><td><code>programs-core-test</code></td><td>Exercises native registration, lifecycle, calls, monetary invariants, occupancy, fees, accounts, and wind-down.</td></tr>
      <tr><td><code>programs-adversarial</code></td><td>Checks guest isolation, composition, and monetary-law refusal paths.</td></tr>
      <tr><td><code>programs-fuzz-smoke</code></td><td>Replays validation, instantiation, and execution corpora.</td></tr>
      <tr><td><code>programs-differential</code></td><td>Checks deterministic replay and serial versus parallel execution behavior.</td></tr>
      <tr><td><code>programs-interpreter-conformance</code></td><td>Builds the interpreter WASM and exercises it through the runtime.</td></tr>
      <tr><td><code>programs-test</code></td><td>Combines protocol, native, guest SDK, porting, adversarial, fuzz, ABI, and differential checks.</td></tr>
    </tbody></table>
    <h2>Preserve ABI compatibility</h2>
    <p>Frozen guest ABI versions 1 through 5 have independently pinned vector checksums in <code>programs/abi-frozen.sha256</code>. The ABI gate compares generated canonical bytes, manifests, validator allowlists, and SDK declarations with those baselines. An existing frozen import contract is immutable; add a new ABI version for a new surface.</p>
    <p>Keep the canonical vectors, interface generators and clients aligned with the selected ABI version. Deployment validation, host imports and response decoding use that version to select the same byte contract.</p>
    <h2>Maintain runtime boundaries</h2>
    <p>Host-function families register through their own modules and access execution state through <code>RuntimeState</code>. The boundary gate rejects forbidden sibling dependencies, legacy monolithic modules, and production paths into qualification-only helpers. The workspace denies unsafe code by default; explicitly bounded FFI adapters carry the reviewed boundary work.</p>
    <p>Dependency policy excludes entropy, clocks, arbitrary networking, dynamic registration, database engines, and alternate execution engines from the deterministic boundary. It verifies vendored checksums, the exact interpreter revision, approved licenses, and integer-only consensus-adjacent code. Workspace linting also rejects unchecked unwraps, expects, and float arithmetic.</p>
    <h2>Canonical fixtures</h2>
    <p>Lifecycle, capability, executed-receipt, and reference-program fixtures keep native encoders and SDKs aligned. Use the check targets when validating existing output; generation targets intentionally write new fixtures.</p>
    <pre><code>{`make programs-check-capability-fixture
make programs-check-native-lifecycle-fixtures
make programs-check-executed-v4-fixture

# Rebuild reference artifacts when intentionally changing their source
make programs-reference-fixtures`}</code></pre>
    <p>Retain the artifact hash, interface and chosen ABI with each deployed program. Applications verify deployment and execution receipts, including committed block evidence, before relying on a final program outcome.</p>
    <p>Continue with <Link href="/storage-scan">bounded storage scans</Link> and <Link href="/sandbox">sandbox execution</Link>.</p>
    <div className="source-note"><strong>Technical references:</strong> <code>programs/Cargo.toml</code>, <code>programs/.cargo/config.toml</code>, <code>Makefile</code>, <code>programs/abi-frozen.sha256</code>, <code>scripts/programs/dependency-policy.sh</code>, and <code>tests/programs/runtime-module-boundaries.sh</code>.</div>
  </DocsLayout>
}
