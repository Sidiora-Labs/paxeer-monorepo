import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/wasm-runtime')

export default function WasmRuntime() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">WASM Runtime</h1>
        <p className="page-description">Understand Paxeer’s in-tree CosmWasm virtual machine, Go and Rust boundary, native library builds, and gas accounting.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>Paxeer-X-Network/wasm-runtime/</code>, especially <code>lib.go</code>, <code>internal/api/</code>, and <code>libwasmvm/</code>.</div>
      <h2>Runtime responsibilities</h2>
      <p>The in-tree CosmWasm runtime connects the Go node to a Rust virtual machine through a C interface. It stores and compiles WebAssembly modules, caches them, and executes contract entry points with host callbacks for storage, addresses, queries, and gas.</p>
      <p>A contract developer compiles Rust into a <code>.wasm</code> artifact separately. The runtime&apos;s <code>StoreCode</code> compiles that WebAssembly into its execution representation; it does not accept Rust source.</p>

      <h2>Package layout</h2>
      <table>
        <thead><tr><th>Path</th><th>Responsibility</th></tr></thead>
        <tbody>
          <tr><td><code>lib.go</code></td><td>CGO-enabled <code>VM</code>, cache lifetime, code storage, and contract entry points.</td></tr>
          <tr><td><code>lib_no_cgo.go</code></td><td>Shared types, Wasm checksum creation, and version API available without a VM execution build.</td></tr>
          <tr><td><code>internal/api/</code></td><td>C bindings, callbacks, memory ownership, iterators, and platform linker settings.</td></tr>
          <tr><td><code>libwasmvm/</code></td><td>Rust library and generated C header.</td></tr>
          <tr><td><code>types/</code></td><td>Environment, messages, query results, gas, storage, and IBC types shared across the boundary.</td></tr>
          <tr><td><code>builders/</code></td><td>Containerized native-library build definitions.</td></tr>
          <tr><td><code>testdata/</code></td><td>Compiled contracts used by runtime tests.</td></tr>
        </tbody>
      </table>
      <p>The node imports <code>github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime</code>. Use the version in the Paxeer tree with its matching native library and generated bindings.</p>

      <h2>CGO and native libraries</h2>
      <p><code>lib.go</code> is guarded by the <code>cgo</code> build tag. Without CGO, checksum and shared-type utilities remain available, but the VM execution API is not compiled. <code>LibwasmvmVersion</code> explicitly returns an unavailable error when CGO is disabled.</p>
      <p>Linker files cover Linux glibc, Linux musl, macOS, Windows, and a system-library mode. Install the native library matching the node&apos;s operating system and architecture.</p>
      <table>
        <thead><tr><th>Build path</th><th>Relevant files or targets</th></tr></thead>
        <tbody>
          <tr><td>Linux glibc</td><td><code>link_glibclinux_x86_64.go</code>, <code>link_glibclinux_aarch64.go</code>, <code>release-build-linux</code></td></tr>
          <tr><td>Linux musl</td><td><code>link_muslc.go</code>, <code>release-build-alpine</code></td></tr>
          <tr><td>macOS</td><td><code>link_mac.go</code>, <code>link_mac_static.go</code>, macOS release targets</td></tr>
          <tr><td>Windows</td><td><code>link_windows.go</code>, <code>release-build-windows</code></td></tr>
          <tr><td>System library</td><td><code>link_system.go</code> and its <code>sys_wasmvm</code> build selection</td></tr>
        </tbody>
      </table>

      <h2>Building the runtime</h2>
      <p>Use the node&apos;s <Link href="/installation">installation guide</Link> to build Paxeer. When changing the runtime itself, its local Makefile has separate Rust and Go targets. Run these from the repository root:</p>
      <pre><code>{`# Build the native release library and refresh the generated C bindings.
make -C wasm-runtime build-rust-release

# Build the Go packages and demo against the native library.
make -C wasm-runtime build-go

# Run the runtime's Go test suite with the actual native library.
make -C wasm-runtime test`}</code></pre>
      <p>The Rust target copies the compiled library into <code>internal/api/</code> and updates <code>bindings.h</code>. It requires a working Rust and C toolchain. Containerized release targets build specific distribution artifacts; choose the target matching the node you will run.</p>

      <h2>VM lifecycle and caching</h2>
      <p><code>NewVM</code> creates a cache using a data directory, supported capabilities, a per-execution memory limit, a debug-output setting, and a cache size. Call <code>Cleanup</code> when releasing a VM. <code>StoreCode</code> returns the checksum used for later instantiation and execution; <code>StoreCodeUnchecked</code> does the same while skipping static validation checks, so prefer <code>StoreCode</code> for untrusted bytecode. <code>Pin</code> and <code>Unpin</code> control retention in the memory cache.</p>
      <p>Code upload, contract instantiation, storage updates, and message dispatch are coordinated by the <Link href="/wasm">CosmWasm module</Link>. Upload or instantiation permissions are module parameters, not a statement that the VM implementation is absent.</p>

      <h2>Execution and gas</h2>
      <p>The VM receives an environment, contract message bytes, storage and query callbacks, and gas limits. Its result carries contract data, emitted messages, and execution gas information back to the module. Host work also consumes gas through the supplied meter.</p>
      <p>When an EVM contract calls the wasmd precompile, the adapter maps the supplied EVM gas into the native execution meter and reports remaining gas. A read-only call made from another on-chain contract still consumes execution gas. Measure the complete call with its real payload and state instead of assuming a fixed gas cost for all CosmWasm operations.</p>

      <h2>Runtime integration</h2>
      <p><Link href="/wasmbinding">WASM bindings</Link> add Paxeer-specific query and message handlers. <Link href="/precompiles">EVM precompiles</Link> expose selected runtime and native-module operations to Solidity. Their ABI and execution-context checks are part of the integration contract.</p>
      <div className="prev-next">
        <Link href="/wasm"><div className="prev-next-label">Previous</div><div className="prev-next-title">WASM</div></Link>
        <Link href="/wasmbinding"><div className="prev-next-label">Next</div><div className="prev-next-title">WASM Bindings</div></Link>
      </div>
    </DocsLayout>
  )
}
