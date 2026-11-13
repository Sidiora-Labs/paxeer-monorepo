import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/escrow')

export default function EscrowPage() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Escrow and Conditional Payments</h1>
        <p className="page-description">Reserve funds, capture milestone payments, refund unused balances, and resolve disputes through native Paxeer X escrow.</p>
      </div>
      <h2>Payment Guarantees for Agent Commerce</h2>
      <p>Escrow separates a payment commitment from its final distribution. A buyer or agent funds a dedicated escrow account, a named beneficiary captures funds under the agreement, and a named arbiter resolves disputes. Marketplaces, service purchases, procurement, and milestone contracts use the same protocol lifecycle.</p>
      <p>The escrow module stores the agreement terms and lifecycle. Every movement of value uses the shared <code>402LXP</code> transfer mechanism, including funding, beneficiary payments, refunds, and dispute splits. The account namespace is <code>{'agent:<did>:escrow:<id>'}</code>.</p>
      <h2>Define an Escrow</h2>
      <table><thead><tr><th>Field</th><th>Purpose</th></tr></thead><tbody>
        <tr><td>Escrow ID and account</td><td>Identify the hold and its isolated funds.</td></tr>
        <tr><td>Owner</td><td>Funds the hold and receives unused funds.</td></tr>
        <tr><td>Beneficiary</td><td>Receives captured payments.</td></tr>
        <tr><td>Arbiter</td><td>Authorizes the distribution of disputed funds.</td></tr>
        <tr><td>Asset and amount</td><td>Specify the asset and positive funding amount in integer base units.</td></tr>
        <tr><td>Expiry</td><td>Absolute batch timestamp deadline in milliseconds; zero means no expiry.</td></tr>
        <tr><td>Dispute window</td><td>Absolute final timestamp for opening a dispute; zero disables dispute opening.</td></tr>
        <tr><td>Terms hash and agreement reference</td><td>Bind the hold to the agreed conditions and reference record.</td></tr>
      </tbody></table>
      <p>The terms hash commits to the agreement. Applications verify delivery, evidence, or milestones against that agreement before requesting an authorized capture; hashing terms does not itself evaluate a real-world condition.</p>
      <h2>Lifecycle and Actions</h2>
      <table><thead><tr><th>Action</th><th>Result</th><th>State</th></tr></thead><tbody>
        <tr><td>Open</td><td>Move the owner&apos;s funds into escrow.</td><td><code>OPEN</code></td></tr>
        <tr><td>Partial capture</td><td>Pay a positive amount strictly below the remaining balance.</td><td><code>PARTIALLY_CAPTURED</code></td></tr>
        <tr><td>Capture</td><td>Pay the entire remaining balance to the beneficiary.</td><td><code>CAPTURED</code></td></tr>
        <tr><td>Release</td><td>Return the remaining balance to the owner.</td><td><code>RELEASED</code></td></tr>
        <tr><td>Timeout</td><td>Return an expired active hold&apos;s remaining balance to the owner.</td><td><code>TIMED_OUT</code></td></tr>
        <tr><td>Open dispute</td><td>Freeze ordinary capture and refund paths.</td><td><code>DISPUTED</code></td></tr>
        <tr><td>Resolve dispute</td><td>Distribute the remaining funds according to the arbiter&apos;s split.</td><td><code>RESOLVED</code></td></tr>
      </tbody></table>
      <div className="callout"><strong>Capture pays the beneficiary. Release refunds the owner.</strong> Keep these meanings visible in application buttons and transaction previews.</div>
      <h3>Authority and Deadlines</h3>
      <p>The beneficiary or an owner principal acting through a delegated capability authorizes capture. The owner authorizes release. Either owner or beneficiary can open a dispute while the hold is active and the dispute deadline has not passed. Only the designated arbiter resolves the dispute.</p>
      <p>Captures fail at or after a nonzero expiry, even before a timeout sweep changes the recorded state. Batch maintenance processes due escrow deadlines using the batch timestamp, with a sweep capacity of 24 (<code>LX_ESCROW_SWEEP_CAPACITY</code>). Disputed holds do not follow the ordinary timeout refund path; they require resolution.</p>
      <p>The dispute deadline is inclusive: a dispute opens when the batch timestamp equals <code>dispute_window</code>, and the window is closed only when the timestamp is greater than it. A zero dispute window disables dispute opening.</p>
      <h2>Milestone Payment Workflow</h2>
      <ol>
        <li>Agree on the asset, owner, beneficiary, arbiter, deadlines, and delivery conditions.</li>
        <li>Open and fund the escrow for 1,000 base units.</li>
        <li>After the first milestone, partially capture 300 units. The beneficiary receives 300 and 700 remain locked.</li>
        <li>On completion, capture the remaining 700. Alternatively, release the unused 700 to the owner.</li>
        <li>If a dispute is opened before the deadline, submit evidence to the arbiter and resolve the remaining balance.</li>
      </ol>
      <h3>Dispute Split Example</h3>
      <p>A resolution specifies the beneficiary share in basis points from 0 to 10,000. With 700 units remaining and a 6,000 basis-point award, the beneficiary receives 420 and the owner receives 280. Earlier captures remain paid. The beneficiary amount rounds down to an integer base unit; the owner receives the remainder, preserving the complete escrow balance.</p>
      <pre><code>{`beneficiary = floor(remaining_balance × beneficiary_basis_points / 10000)
owner       = remaining_balance - beneficiary`}</code></pre>
      <h2>Protocol Reference</h2>
      <p>Escrow is module <code>2</code>. Activity payloads use fixed-width IDs and integer amounts; they are protocol activities rather than EVM contract method names.</p>
      <table><thead><tr><th>Activity</th><th>Type</th><th>Payload bytes</th></tr></thead><tbody>
        <tr><td><code>LX_ESCROW_OPEN</code></td><td><code>0x00020001</code></td><td>288</td></tr>
        <tr><td><code>LX_ESCROW_CAPTURE</code></td><td><code>0x00020002</code></td><td>80</td></tr>
        <tr><td><code>LX_ESCROW_PARTIAL_CAPTURE</code></td><td><code>0x00020003</code></td><td>80</td></tr>
        <tr><td><code>LX_ESCROW_RELEASE</code></td><td><code>0x00020004</code></td><td>64</td></tr>
        <tr><td><code>LX_ESCROW_TIMEOUT</code></td><td><code>0x00020005</code></td><td>64</td></tr>
        <tr><td><code>LX_ESCROW_DISPUTE_OPEN</code></td><td><code>0x00020006</code></td><td>32</td></tr>
        <tr><td><code>LX_ESCROW_DISPUTE_RESOLVE</code></td><td><code>0x00020007</code></td><td>68</td></tr>
      </tbody></table>
      <h3>Retries and Failure Handling</h3>
      <p>Capture, release, timeout, and resolution carry a nonzero idempotency key. Preserve that key when retrying the same operation. Keys bind to the escrow and operation context; reusing one for a different payment is rejected. Track the returned receipt and final state before advancing a business workflow.</p>
      <p>Handle expired holds, disputed holds, unauthorized capture, and capture amounts above the remaining balance as distinct failures. A partial capture cannot equal the entire balance: use full capture instead. Out-of-range dispute splits are rejected.</p>
      <h2>Related Guides</h2>
      <ul><li><Link href="/streams">Continuous and metered payments</Link></li><li><Link href="/modules">Protocol modules</Link></li><li><Link href="/sdk">Build with the SDK</Link></li></ul>
    </DocsLayout>
  )
}
