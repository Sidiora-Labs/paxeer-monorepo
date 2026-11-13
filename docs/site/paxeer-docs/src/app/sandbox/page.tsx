import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/sandbox')

export default function SandboxPage() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Bounded Sandboxes</h1>
      <p className="page-description">Lease isolated deterministic computation with explicit resource ceilings, receipt-backed lifecycle transitions, metered snapshots, and resumable execution.</p>
    </div>
    <h2>A lease is the execution boundary</h2>
    <p>Paxeer X sandboxes run validated guest images inside a bounded lease. Each lease binds a renter principal, host program, image, private storage namespace, resource limits, lifetime, escrow, and fee schedule. Execution derives its authority from this lease rather than inheriting unrestricted host-program access.</p>
    <p>Use a sandbox for isolated computation whose state and execution must remain replayable and whose cost and duration must remain bounded. Native financial application logic uses the broader <Link href="/programs">Programs execution surface</Link>; a sandbox guest receives only lease-scoped storage authority.</p>
    <h2>Receipt-backed lease lifecycle</h2>
    <table><thead><tr><th>State</th><th>Meaning and next step</th></tr></thead><tbody>
      <tr><td>Requested</td><td>The lease holds a principal concurrency slot. Fund it or let it expire.</td></tr>
      <tr><td>Funded</td><td>Escrow is established. Activate execution or restore an eligible snapshot into this target.</td></tr>
      <tr><td>Active</td><td>Execute within the lease bounds, persist an authorized snapshot, begin settlement, or expire.</td></tr>
      <tr><td>Settling</td><td>Finalize usage and the lease&apos;s economic disposition; proceed to expiry.</td></tr>
      <tr><td>Expired</td><td>The concurrency slot is released. Destroy storage with the required evidence.</td></tr>
      <tr><td>Destroyed</td><td>Terminal state. The same lease cannot be funded, activated, or revived.</td></tr>
    </tbody></table>
    <p>The ordinary lifecycle is Requested → Funded → Active → Settling → Expired → Destroyed. Snapshot preserves the Active state. Expiry can also close a Requested, Funded, or Active lease once its batch-sequence deadline is reached. A Settling lease can expire through its settlement path.</p>
    <p>Transitions consume verified receipt evidence and reject replayed evidence or undeclared state edges. A principal has at most 32 concurrent leases. Requested, Funded, Active, and Settling all count toward this limit. Expiry releases the slot; destroying an already expired lease does not release it a second time.</p>
    <h2>Resource limits and automatic closure</h2>
    <p>Declare execution ceilings, namespace capacity, lifetime in protocol batches, and escrow before activation. Usage observations are monotonic: a later observation cannot reset consumed resources. The runtime closes an Active lease into Settling when usage first exceeds a bound, the lifetime, or escrow. This is an intrinsic consequence of metering, not an arbitrary caller-requested transition.</p>
    <table><thead><tr><th>Dimension</th><th>Protocol declaration ceiling</th></tr></thead><tbody>
      <tr><td>CPU fuel</td><td>1,000,000,000</td></tr>
      <tr><td>Linear memory</td><td>1 GiB</td></tr>
      <tr><td>Storage reads / writes</td><td>1 GiB each</td></tr>
      <tr><td>Output bytes</td><td>1 GiB</td></tr>
      <tr><td>Output values</td><td>65,536</td></tr>
      <tr><td>Table elements</td><td>1,048,576</td></tr>
      <tr><td>Namespace storage</td><td>1 GiB, including persisted snapshot records</td></tr>
      <tr><td>Lifetime</td><td>1,000,000 protocol batches</td></tr>
    </tbody></table>
    <p>Escrow is also capped: a lease escrow must be nonzero and at most 1,000,000,000,000,000,000,000,000. Each lease records at most 64 snapshots and 70 lifecycle transitions. These are maximum declarations, not allocations promised to every workload. Each lease selects its own admitted limits and fee coverage. Meter memory, state, outputs, snapshot persistence, and restoration together when planning a resumable workload.</p>
    <h2>Authority that cannot escape the lease</h2>
    <p>The execution principal and storage namespace derive from the host program and lease identity. The capability set contains only <code>StorageRead</code> and <code>StorageWrite</code>. Shared storage, native transfers, balance reads, receipt reads, event emission, and calls to other programs are absent.</p>
    <p>A guest that imports event or call functions does not gain their authority. Attempted event emission refuses; a call to an unleased program fails the composition capability check. The refused operation leaves sandbox storage unchanged. The boundary is enforced by capability derivation rather than by trusting the guest to follow a convention.</p>
    <h2>Persist execution state and resume it</h2>
    <h3>Canonical snapshots</h3>
    <p>A renter-authorized snapshot captures the source lease, host program, namespace prefix, linear memory, globals, continuation, and ordered namespace cells. Canonical encoding uses the domain <code>LayerX/programs/sandbox/snapshot/v1</code>. SHA-256 commits the exact encoded state.</p>
    <p>Persistence stores a 44-byte manifest containing the digest, byte length, and chunk count, followed by chunks of up to 65,536 bytes. Metered storage includes keys and values. Snapshot records share the lease&apos;s namespace capacity with live state, so repeated snapshots cannot bypass the storage ceiling.</p>
    <h3>Restore into a funded target</h3>
    <p>Restore requires a recognized snapshot digest, a Funded target lease, the same owner principal, the same host program, and the same image hash. Live cells are rebound to the target namespace. Restoration charges the canonical snapshot bytes and reconstructed cells before instantiation; reconstructing memory and the continuation also consumes runtime resources.</p>
    <p>A successful restore preserves the exact memory, globals, continuation, and state needed to continue execution. The target is a new bounded lease. Destroying the source lease does not make its identity reusable.</p>
    <DocArt name="workload-inputs" />
    <h2>End-to-end workflow</h2>
    <ol>
      <li>Validate and identify the guest image. Choose the host program and renter principal.</li>
      <li>Request a lease with explicit resource, namespace, lifetime, escrow, and fee-schedule bounds. Retain the verified request evidence.</li>
      <li>Fund and activate through the admitted lifecycle transitions.</li>
      <li>Run under derived lease capabilities and record monotonic usage evidence.</li>
      <li>If resumption is required, persist a renter-authorized snapshot while Active and retain its digest.</li>
      <li>Settle, expire, and destroy the source lease with the required evidence.</li>
      <li>Fund a compatible target lease, restore the snapshot with sufficient reconstruction budget, and continue under the new lease&apos;s bounds.</li>
    </ol>
    <h2>Results and verification</h2>
    <p>Retain the lease ID, image hash, transition receipt digests, batch sequences, usage observations, and snapshot digest where applicable. These identify what ran, under which bounds, which state was captured, and how the lease reached its final state. A guest return value alone does not prove funding, settlement, or destruction.</p>
    <p>Sandbox trace replay also supports deterministic compute-market dispute adjudication: the arbiter checks the step to which a bisection dispute resolves. The sandbox remains storage-only even when a surrounding market program manages stake, claims, or settlement.</p>
    <p>Read <Link href="/programs">Programs</Link> for deployment, broader capability grants, native value accounts, and atomic transfer settlement.</p>
    <div className="source-note"><strong>Technical references:</strong> Paxeer X Sandbox protocol documentation; sandbox lease, execution, and snapshot modules; receipt-backed lifecycle and snapshot conformance tests.</div>
  </DocsLayout>
}
