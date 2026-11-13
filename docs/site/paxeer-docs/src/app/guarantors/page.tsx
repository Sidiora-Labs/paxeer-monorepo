import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/guarantors')

export default function Guarantors() {
  return <DocsLayout>
    <h1>Validators and commitment</h1>
    <p className="page-description">BFT block commitment, signed execution receipts, and independent evidence verification.</p>
    <p>Paxeer validators commit blocks on one network. The kernel participates in block production. Applications verify execution receipts and the committed blocks covering them; finality comes from BFT commitment.</p>
    <h2>Verify the execution outcome</h2>
    <ol><li>Confirm the chain ID and retain the signed transaction.</li><li>Retrieve the transaction receipt and its progress rung.</li><li>Obtain the governed receipt keys and check their authorized height ranges.</li><li>Verify the receipt signature, decoded fields, and output digests.</li><li>Verify the committed block covering the receipt sequence.</li></ol>
    <p>A receipt signature authenticates execution only when its signing key is authorized for the receipt height. The node exposes the governed key ranges through <code>px_getReceiptKeys</code>, implemented in <code>rpc/sequencer_receipt.go</code>.</p>
    <h3>Exact evidence bindings</h3>
    <p>Verification checks the exact signed bytes and their bindings. Receipt output data must reproduce the digests in the signed receipt. Missing evidence leaves the outcome at its observed progress level.</p>
    <h2>Receipt and commitment evidence</h2>
    <p><code>pax_sequencerReceipt</code> exposes the transaction hash, nonce, progress rung, and available receipt material. Its committed result identifies the block whose sequence batch covers the receipt.</p>
    <table><thead><tr><th>Evidence</th><th>Meaning</th></tr></thead><tbody><tr><td>Submission acknowledgement</td><td>The node holds the transaction.</td></tr><tr><td>Signed execution receipt</td><td>The authorized signer records the execution outcome.</td></tr><tr><td>Receipt-key range</td><td>Governance authorizes a receipt key over stated heights.</td></tr><tr><td>Committed block</td><td>BFT commitment makes the included outcome final.</td></tr></tbody></table>
    <p><code>SequencerReceiptResult.Verify</code> checks receipt signatures, decoded fields, output digests, and required block fields. Pass the trusted receipt keys to authenticate signing authority. Consistency under an untrusted supplied key is insufficient.</p>
    <h2>Consensus membership</h2>
    <p>Validator membership is part of the BFT network. The hosted gateway declares <code>px_getValidators</code> and <code>px_getCommit</code> in <code>services/platform/hosted/gateway/openrpc.json</code>; use their verified chain data when inspecting consensus evidence.</p>
    <p>The public EVM chain ID is <code>125</code>. Bind transaction signatures and receipt verification to the intended chain and obtain current consensus state from that network.</p>
    <h2>Block confirmation</h2>
    <p>The <code>committed</code> rung means the receipt sequence is covered by a committed block. Its <code>blockNumber</code> and <code>blockHash</code> identify that block. Verify these bindings together with the execution result.</p>
    <p>A successful submission response is an acknowledgement. A receipt may record a reverted execution. Check status and effects before presenting a committed outcome as a successful payment.</p>
    <h2>Application responsibilities</h2>
    <p>Retain the original signed transaction, its hash, receipt bytes, trusted receipt-key ranges, and covering block evidence. These records let an application verify the outcome independently of a progress indicator.</p>
    <p>If evidence is unavailable, keep the transaction pending and reconcile its existing hash. Reissuing a payment with another transaction can create a second economic effect.</p>
    <h2>Continue</h2><p>See <Link href="/finality">Finality</Link>, <Link href="/governance">Governance</Link>, and <Link href="/security">Security and trust boundaries</Link>.</p>
  </DocsLayout>
}
