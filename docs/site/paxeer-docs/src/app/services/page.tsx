import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'
import Link from 'next/link'

export const metadata = metadataFor('/services')

export default function ServicesPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Services and agreements</h1><p className="page-description">Publish paid capabilities, agree on deliverables, attest execution, and resolve delivery through the Paxeer X service lifecycle.</p></div>
    <p>Paxeer X turns a service into a verifiable agreement between a provider and a customer. An offer defines price, Asset, terms, deliverable specification, deadlines, and acceptance policy. Agreements bind those terms to an escrow; commitments and execution attestations establish the work performed; delivery records bind the resulting artifacts to their availability references.</p>
    <h2>Choose a service model</h2>
    <table><thead><tr><th>Model</th><th>Use it for</th><th>Completion evidence</th></tr></thead><tbody>
      <tr><td>Deliverable agreement</td><td>Research, generated assets, analysis, computation, and agent tasks.</td><td>Commitments, signed tool execution, delivery hashes, acceptance or dispute ruling.</td></tr>
      <tr><td>Paid API</td><td>Immediate access to a query result, data resource, or tool endpoint.</td><td>Verified payment receipt and a durable fulfillment record.</td></tr>
      <tr><td>Receipt-gated resource</td><td>A single paid HTTP request for a resource or tool result.</td><td>A signed transaction receipt bound to the requested payment.</td></tr>
    </tbody></table>
    <p>Use the service module for negotiated work with delivery and acceptance. Use <Link href="/paid-apis">paid API middleware</Link> for an HTTP resource that releases its response after receipt verification. The service module records agreement state; payment operations resolve its linked escrow. Paid HTTP resources verify the payment transaction receipt before fulfillment.</p>
    <h2>Register and discover an offer</h2>
    <p>Publish an offer with <code>LX_SERVICE_OFFER_PUBLISH</code>. Give customers the offer identifier and a human-readable description of its terms and deliverable specification. Content hashes commit to the exact documents; host the documents and artifacts where the customer can retrieve and verify them. Discovery identifies a suitable offer; the customer checks its Asset, price, expiry, and policy before proposing an agreement.</p>
    <table><thead><tr><th>Offer field</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>offer_id</code>, <code>asset_id</code>, <code>price</code></td><td>Nonzero offer identity, payment Asset identity, and positive integer price.</td></tr>
      <tr><td><code>terms_hash</code></td><td>Commitment to the terms the customer accepts.</td></tr>
      <tr><td><code>deliverable_specification_hash</code></td><td>Commitment to the expected result.</td></tr>
      <tr><td><code>delivery_deadline</code>, <code>offer_expiry</code></td><td>Delivery deadline and offer expiry in milliseconds, checked against batch time.</td></tr>
      <tr><td><code>acceptance_window</code>, <code>dispute_window</code></td><td>Nonzero durations in milliseconds: review ends at the delivery deadline plus the acceptance window; the dispute window follows.</td></tr>
      <tr><td><code>default_outcome</code></td><td><code>1</code> accepts or <code>2</code> rejects when the review policy applies.</td></tr>
    </tbody></table>
    <p>Withdraw an offer with <code>LX_SERVICE_OFFER_WITHDRAW</code>. An unavailable offer or mismatched terms prevents a new agreement from proceeding.</p>
    <DocArt name="job-progress" />
    <h2>From offer to invocation</h2>
    <ol>
      <li>Read the offer and verify the terms and deliverable documents against their hashes.</li>
      <li>Prepare payment escrow through the payment lifecycle and retain its identifier.</li>
      <li>Propose an agreement with <code>agreement_id</code>, <code>offer_id</code>, <code>terms_hash</code>, and <code>escrow_id</code> using <code>LX_SERVICE_AGREEMENT_PROPOSE</code>.</li>
      <li>The provider accepts with <code>LX_SERVICE_AGREEMENT_ACCEPT</code>.</li>
      <li>Commit a task using <code>LX_SERVICE_COMMIT_TASK</code>, binding its task hash, escrow, positive deadline, and resource bound.</li>
      <li>Execute the agreed capability, publish signed execution attestations, and report progress.</li>
      <li>Deliver artifacts for customer acceptance; resolve escrow through the payment lifecycle.</li>
    </ol>
    <h3>Commitments and execution attestations</h3>
    <p>A task commitment records <code>commitment_id</code>, <code>agreement_id</code>, <code>task_hash</code>, <code>escrow_id</code>, <code>deadline</code>, and <code>resource_bound</code>. An abandoned commitment carries an explicit reason through <code>LX_SERVICE_COMMIT_ABANDON</code>.</p>
    <p><code>LX_SERVICE_TOOL_EXEC_ATTEST</code> records the tool identifier, input and output commitments, execution window, resource units, attestor identity, availability reference, public key, and signature. Verification checks the attestor; invalid signatures or execution evidence are refused as <code>LXP_ERR_INVALID_ATTESTATION</code>.</p>
    <p><code>LX_SERVICE_PROGRESS_REPORT</code> binds a report and note hash to a commitment and availability reference. Progress is an integer from <code>1</code> through <code>10000</code> basis points. Each report must strictly increase the previous progress value.</p>
    <h2>Deliver and review</h2>
    <p><code>LX_SERVICE_DELIVER</code> carries one to sixteen artifacts. Each item contains a hash, a positive byte size, and a nonzero availability reference; a zero size or zero availability reference is refused as non-canonical. Every deliverable hash must equal the offer&apos;s <code>deliverable_specification_hash</code>, and delivery must arrive by the agreement deadline. Delivery is accepted from the committed state and again from the rejected state, which permits redelivery after rejection. Retain the bytes corresponding to every declared hash so the customer can verify the result.</p>
    <pre><code>{`version:u16be (00 01) || delivery_id32 || agreement_id32 || count:u8
|| { hash32 || artifact_size:u64be || availability_reference32 } × count`}</code></pre>
    <p>The customer accepts with <code>LX_SERVICE_ACCEPT</code> or rejects using <code>LX_SERVICE_REJECT</code>, a rejection reason, and contested hashes. Protocol batch maintenance applies the agreement&apos;s default review policy when the acceptance window ends.</p>
    <h3>Disputes and resolution</h3>
    <p><code>LX_SERVICE_DISPUTE_OPEN</code> binds a dispute identifier and evidence hashes to the agreement. The agreement must be in the rejected state, the raiser must be the provider or the buyer, the escrow link must match the agreement, and the escrow must name a nonzero arbiter (<code>LXP_ERR_ARBITER_REQUIRED</code>); the dispute must open within its window. <code>LX_SERVICE_DISPUTE_RESOLVE</code> must come from that escrow arbiter (<code>LXP_ERR_UNAUTHORIZED_ARBITER</code> for any other caller) and supplies a nonzero ruling, the provider&apos;s allocation in basis points, and the escrow resolution identifier. Allocation ranges from zero to <code>10000</code>.</p>
    <h2>Activity reference</h2>
    <p>The service module ID is <code>5</code>; payload version is <code>1</code>, encoded by the prefix bytes <code>00 01</code> for all activities except tool execution attestations. That activity uses the 416-byte execution record directly. Identifiers and hashes are 32 bytes. The codecs are in include/layerx/lx_service.h and src/modules/service/lx_service_payload.c.</p>
    <table><thead><tr><th>Activity</th><th>Type</th><th>Canonical payload bytes</th></tr></thead><tbody>
      <tr><td>Offer publish / withdraw</td><td><code>0x00050001 / 0x00050002</code></td><td>179 / 34</td></tr>
      <tr><td>Agreement propose / accept</td><td><code>0x00050003 / 0x00050004</code></td><td>130 / 34</td></tr>
      <tr><td>Task commit / abandon</td><td><code>0x00050005 / 0x00050006</code></td><td>146 / 36</td></tr>
      <tr><td>Tool execution attest / progress</td><td><code>0x00050007 / 0x00050008</code></td><td>416 / 134</td></tr>
      <tr><td>Deliver / accept</td><td><code>0x00050009 / 0x0005000a</code></td><td>67 + 72 per item / 34</td></tr>
      <tr><td>Reject / dispute open</td><td><code>0x0005000b / 0x0005000c</code></td><td>37 + 32 per hash / 67 + 32 per hash</td></tr>
      <tr><td>Dispute resolve</td><td><code>0x0005000d</code></td><td>72</td></tr>
    </tbody></table>
    <h2>Handle refusals and retries</h2>
    <p>Preserve the agreement, commitment, activity, and escrow identifiers across retries. Check status before resubmission after a timeout. Sequence reuse, unauthorized debit, unavailable offers, terms mismatch, and invalid agreement state are distinct refusals; changing payment identity to escape uncertainty can create a second economic action.</p>
    <p>Delivery errors distinguish <code>LXP_ERR_DELIVERABLE_MISMATCH</code> (a hash that does not equal the specification hash), non-canonical zero sizes or availability references, and <code>LXP_ERR_DELIVERY_DEADLINE_PASSED</code>. Dispute errors distinguish an unauthorized actor from a closed window. Correct the refused condition before submitting a new action.</p>
    <h2>Continue building</h2>
    <p>Connect immediate resource access with <Link href="/paid-apis">paid APIs and x402</Link>, explore <Link href="/sdk">SDK integrations</Link>, and inspect the <Link href="/modules">protocol modules</Link>.</p>
  </DocsLayout>
}
