import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/proofs')

export default function Proofs() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Receipts &amp; Proofs</h1><p className="page-description">Verify Paxeer X outcomes independently, carry evidence between systems, and connect native state to settlement.</p></div>
    <h2>Choose the evidence for your decision</h2>
    <p>Paxeer X gives each activity a canonical signed receipt. A merchant verifies payment facts before releasing a resource; an auditor preserves the exact signed outcome; an account owner proves committed state to settlement workflows. Each workflow uses a different evidence boundary.</p>
    <table><thead><tr><th>Evidence</th><th>Establishes</th><th>Use</th></tr></thead><tbody>
      <tr><td>Portable receipt (v2)</td><td>Canonical outcome authenticated by a pinned receipt key</td><td>Offline receipt validation and outcome accounting</td></tr>
      <tr><td>Native state witness</td><td>A canonical account or module fact is included in a composite state root</td><td>Balance and module-state verification</td></tr>
      <tr><td>Custody state proof</td><td>Custody state at verified heights under a pinned chain identity</td><td>Paxeer custody verification</td></tr>
      <tr><td>Mirror archive</td><td>A receipt obtained from an untrusted archive belongs to an authenticated batch</td><td>Independent receipt verification from Ethereum or Solana mirrors</td></tr>
    </tbody></table>
    <h2>Portable receipt format</h2>
    <p>The current format is <code>layerx-receipt-proof-v2</code>. The wire identifiers retain the LayerX name for interoperability. The earlier v1 format (<code>layerx-receipt-proof-v1</code>) is not accepted by the current crate; there is no v1 parser in <code>layerx-portable</code>. Verification runs offline once you supply your own pinned receipt-key list.</p>
    <table><thead><tr><th>Member</th><th>Value</th></tr></thead><tbody>
      <tr><td><code>format</code></td><td><code>layerx-receipt-proof-v2</code></td></tr>
      <tr><td><code>verificationLevel</code></td><td><code>receipted</code></td></tr>
      <tr><td><code>signedReceipt</code></td><td>Exact signed canonical receipt bytes; lowercase <code>0x</code> hex</td></tr>
      <tr><td><code>receiptDigest</code></td><td>32-byte receipt digest; lowercase <code>0x</code> hex</td></tr>
      <tr><td><code>receiptKey</code></td><td>Object with <code>keyId</code> and <code>publicKey</code> (32 bytes each, <code>0x</code> hex) and <code>firstHeight</code> / <code>lastHeight</code> (<code>0x</code> quantities): the inclusive height window in which that key signs</td></tr>
      <tr><td><code>inclusionHeight</code></td><td><code>0x</code> quantity; must not precede the signed receipt height</td></tr>
      <tr><td><code>rawTransaction</code></td><td>Optional; raw transaction bytes as <code>0x</code> hex, bound to the receipt activity ID</td></tr>
    </tbody></table>
    <p>Byte strings are lowercase <code>0x</code> hex and heights are <code>0x</code> quantities. Unknown JSON members are refused. The JSON input limit is 1,500,000 bytes; the signed receipt must contain between 1 and 1,048,576 bytes.</p>
    <h3>Establish the trust root first</h3>
    <p>The trust root is your pinned <code>ReceiptKeys</code> list of <code>ReceiptKeyEntry</code> values (key ID, Ed25519 public key, first and last signing height). The <code>receiptKey</code> carried in the JSON names the entry that signed; it is trusted only after it equals the entry selected from your own pinned list. Copying the key out of the incoming JSON does not authorize it.</p>
    <pre><code>{`// Rust: json_bytes and receipt_keys (your pinned list) are supplied by your application.
let verified = layerx_portable::PortableReceiptV2::verify_json(
    &json_bytes,
    &receipt_keys,
)?;`}</code></pre>
    <p><code>from_json</code> parses the bounded representation without establishing trust; <code>verify_json</code> parses and verifies it against your pinned keys. Export uses <code>PortableReceiptV2::export(signed_receipt, receipt_key, inclusion_height)</code>, or <code>export_with_transaction</code> to also bind raw transaction bytes. Export verifies the signed receipt before producing its JSON representation. <code>from_sequencer_receipt</code> exports the signed receipt of a <code>receipted</code> or <code>committed</code> RPC answer and refuses <code>submitted</code> answers.</p>
    <h3>What verification checks</h3>
    <ol><li>Parse the bounded JSON and decode every field in its canonical <code>0x</code> form.</li><li>Strictly verify the sequencer Ed25519 signature on the signed receipt against the pinned key list.</li><li>Select the pinned entry by the receipt key ID and require the carried <code>receiptKey</code> to equal it exactly.</li><li>Require <code>inclusionHeight</code> to be at or after the signed receipt height.</li><li>Recompute the receipt digest and require it to equal <code>receiptDigest</code>.</li><li>When <code>rawTransaction</code> is present, require its Keccak-256 hash to equal the receipt activity ID.</li></ol>
    <pre><code>{`receipt_digest domain: "LXP/v1/receipt\\0"`}</code></pre>
    <p>A cryptographically verified rejected outcome remains rejected. The portable proof establishes sequencer authentication of the signed receipt; batch inclusion and checkpoint settlement require their own evidence.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/interop/crates/layerx-portable/src/receipt.rs</code> — <code>PortableReceiptV2</code>, <code>verify_json</code>, <code>export</code>.</div>
    <h2>Native state witnesses</h2>
    <p>Native witnesses prove a canonical key/value leaf through its module subtree into the composite state root. Account leaves (module 0) include a separate positional account-registry path, followed by the module-zero <code>account-tree</code> binding and the module wrapper. The Rust and Go verifiers share this encoding.</p>
    <p>Lengths, indices, leaf counts, path depths and trailing bytes are checked. Odd-width Merkle levels duplicate the final node and require that exact sibling. The native leaf hash places both lengths before the key and value:</p>
    <pre><code>{`SHA256("LXP/v1/state-leaf\\0" || key_len:u32be || value_len:u32be || key || value)
SHA256("LXP/v1/state-node\\0" || left[32] || right[32])`}</code></pre>
    <div className="source-note"><strong>Source:</strong> <code>services/agent/crates/layerx-proof/src/state_witness.rs</code>.</div>
    <h2>Custody state proofs</h2>
    <p>The Go <code>custodyproof</code> package verifies custody state bundles under the format version <code>paxeer-custody-state-v3</code>. <code>Verify(request, now)</code> checks a decoded request against an <code>Expected</code> pin set by the caller: genesis hash, Comet chain ID, EVM chain ID, custody identity, module hash, asset ID and confirmation count. It verifies the light-block history from genesis and the state membership proofs at the requested heights before returning a result. <code>Decode</code> parses the bounded JSON input.</p>
    <p>Pin the expected values from your own configuration. Values copied out of the incoming bundle do not authorize themselves.</p>
    <div className="source-note"><strong>Source:</strong> <code>custodyproof/verify.go</code>, <code>custodyproof/decode.go</code>.</div>
    <h2>Archive and mirror verification</h2>
    <p>Ethereum and Solana mirrors publish batch archives for independent receipt verification. <code>layerx-mirror-verify</code> validates an untrusted archive, authenticates its batch header against configured trust, verifies receipt inclusion and reports freshness. Preserve trust configuration alongside the archive: a valid historical receipt does not alone establish that it is current.</p>
    <p>For long-term evidence, retain signed receipts, your pinned receipt-key list, signed headers, inclusion proofs and publication references. Preserve exact bytes rather than reconstructed display JSON. See <Link href="/data-availability">Data Availability</Link> for availability verification and <Link href="/finality">Finality</Link> for commitment policies.</p>
    <h2>Verification failures</h2>
    <p>Handle typed refusals explicitly. Portable-receipt failures are distinct: JSON bounds or shape, receipt bounds, unsupported format or verification level, invalid hex, length or quantity, receipt verification failure, receipt-key mismatch, missing or mismatched digest, raw-transaction mismatch, a not-yet-receipted transaction, and an inclusion height preceding the signed height. Preserve the original activity ID for recovery and never upgrade a weaker evidence level to a stronger success.</p>
  </DocsLayout>
}
