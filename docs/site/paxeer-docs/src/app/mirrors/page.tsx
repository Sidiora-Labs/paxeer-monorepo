import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/mirrors')

export default function Mirrors() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Ethereum &amp; Solana Mirrors</h1><p className="page-description">Publish Paxeer X batch archives to external chains and verify committed receipt evidence independently of a hosted gateway.</p></div>
    <h2>External access to canonical evidence</h2>
    <p>The mirror system builds committed batch archives, publishes them to Ethereum or Solana, and retrieves them through independently configured chain sources. A consumer can then verify an archived receipt under its own sequencer key configuration without contacting the Paxeer X gateway.</p>
    <p>Mirror publication distributes evidence. Paxeer receipts are final once their blocks are committed by the BFT network. External-chain publication confirms archive availability under that chain&apos;s rules; it does not authorize a Paxeer transfer.</p>
    <h2>Publisher and verifier roles</h2>
    <table><thead><tr><th>Component</th><th>Responsibility</th></tr></thead><tbody>
      <tr><td><code>layerx-mirror-publisher</code></td><td>Acquire committed node data, construct archives, publish to configured targets and track durable progress</td></tr>
      <tr><td><code>layerx-mirror-verify</code></td><td>Read configured mirror sources and verify a committed archive or one archived receipt</td></tr>
      <tr><td><code>MirrorVerifier</code></td><td>Admit a committed archive, return its committed coordinate, and verify a receipt by index under configured sequencer keys</td></tr>
      <tr><td><code>PublicationJournal</code> and archive spool</td><td>Preserve publication phases and exact archive material through restart</td></tr>
    </tbody></table>
    <h2>Trust configuration</h2>
    <p>The verifier configuration carries <code>layerx_network_id</code>, a <code>receipt_keys</code> list, and a <code>sources</code> list. Each receipt key pins <code>sequencer_id_hex</code>, <code>public_key_hex</code>, and the <code>first_height</code>/<code>last_height</code> window in which that key authorizes receipts. This trust material is configured independently of archive payloads; an archive cannot authorize its own signing key or extend its height window. A configuration with no receipt keys is refused.</p>
    <table><thead><tr><th>Source</th><th>Identity pins</th></tr></thead><tbody>
      <tr><td>Ethereum</td><td>RPC quorum, chain ID, genesis hash, archive contract, archive code hash and publisher identity</td></tr>
      <tr><td>Solana</td><td>RPC quorum, genesis hash, archive program, upgradeable loader, program-data account, program code hash and publisher identity</td></tr>
    </tbody></table>
    <p>Retain target-chain identity and canonical position with each observation. On Ethereum this includes the observed block and reference head; on Solana it includes the rooted slot and block hash.</p>
    <h2>Publish an archive</h2>
    <pre><code>{`layerx-mirror-publisher <publisher-config.json>
layerx-mirror-publisher --probe <configured-status-listener>`}</code></pre>
    <p>The second form probes the publisher&apos;s status listener for readiness instead of starting the service. Any other argument shape exits with a usage error.</p>
    <ol><li>Read committed blocks from the configured Paxeer source, starting at the configured first height.</li><li>Build and validate the archive, preserving its exact canonical bytes and content commitment.</li><li>Persist publication intent, obtain target-chain signatures and broadcast.</li><li>Observe configured chain confirmation or rooting, retrieve the published archive and verify the bytes.</li><li>Advance the durable publication cursor only through the recorded publication workflow.</li></ol>
    <p>Publisher configuration includes the state directory, first height, polling interval, committed-block freshness budget, status listener address, and the Paxeer committed-source section. The Ethereum target is configured explicitly; the Solana section is optional and validated when present.</p>
    <h3>Recover publication safely</h3>
    <p>The publication journal distinguishes prepared, pre-broadcast failure, signed, broadcast-unknown, pending, finalized, and retrieved-verified states, alongside permanent refusal, reorganization, and expired broadcast. When broadcast status is unknown, resolve chain history before issuing a replacement nonce or blockhash.</p>
    <p>Readiness includes committed-block freshness against the configured block budget. A healthy process and a completed historical publication do not by themselves establish that every target is current.</p>
    <h2>Read policies</h2>
    <table><thead><tr><th>Policy</th><th>Behavior</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>exact</code></td><td>Read one source locator and commitment</td><td>Audit a specific publication</td></tr>
      <tr><td><code>ordered-preference</code></td><td>Use an explicit ordered candidate set</td><td>Controlled source failover</td></tr>
      <tr><td><code>agreement</code></td><td>Require a configured minimum agreement among candidates</td><td>Cross-source corroboration</td></tr>
    </tbody></table>
    <p>A candidate identifies the configured source index and exact archive commitment. The command reports source or RPC divergence as divergent, alongside insufficient-agreement, missing, rate-limited and unavailable outcomes.</p>
    <h2>Verify from a mirror</h2>
    <p>Prepare a verifier configuration with your independent network and source pins, then save the JSON request below as receipt-request.json and supply it on standard input. A receipt request names the archived receipt by its index within the batch. Replace the commitment placeholder with the 32-byte archive commitment:</p>
    <pre><code>{`{
  "batch_number": "12",
  "evidence": {
    "kind": "receipt",
    "index": 0
  },
  "policy": {
    "kind": "exact",
    "candidate": {
      "source": 0,
      "commitment_hex": "REPLACE_WITH_32_BYTE_ARCHIVE_COMMITMENT_HEX"
    }
  }
}`}</code></pre>
    <pre><code>{`layerx-mirror-verify <verifier-config.json> < receipt-request.json`}</code></pre>
    <p>The verifier fetches the archive from the pinned source, checks its commitment and batch, and verifies the sequencer signature over the archived receipt bytes against the receipt key whose height window covers the batch. It does not take receipt bytes from the caller: a request carrying fields such as <code>canonical_hex</code> is refused, because unknown request members are denied. There is no state-proof request in this command; the only other evidence kind is <code>committed</code>, which verifies the archive&apos;s committed coordinate without selecting a receipt.</p>
    <p>Batch numbers are nonzero canonical decimal strings without leading zeros. Receipt indices at or above 1,048,576 are out of bounds, and the request body is capped at 40 MiB.</p>
    <h2>Interpret the result</h2>
    <p>On success the verifier prints <code>{`{ "ok": true, "verification": { ... } }`}</code>. The verification object contains <code>level</code>, <code>evidenceDigest</code>, <code>sourceId</code>, <code>target</code>, <code>canonicalPosition</code>, <code>provenance</code>, <code>latestBatch</code>, <code>batchLag</code>, <code>failoverCount</code>, <code>agreeingSources</code>, <code>committedHeight</code>, and <code>committedBlockHash</code>. Failures print <code>ok: false</code> with an error code.</p>
    <p>The only evidence level is <code>Committed</code>: the archive was admitted under the pinned source commitment and, for a receipt request, the receipt&apos;s sequencer signature verified. The report identifies the admitted committed height and block hash. Freshness is reported separately through <code>latestBatch</code> and <code>batchLag</code> and can be unknown. Store the observation and trust configuration with the evidence so later readers can distinguish historical validity from current coverage.</p>
    <h2>Library verifier</h2>
    <p>In Rust, <code>MirrorVerifier::from_committed_source</code> or <code>admit_committed</code> admits archive bytes against an observation, refusing a mismatched source commitment or batch. <code>committed()</code> returns the committed height and block hash retained in the archive. <code>committed_receipt(index, keys)</code> verifies one archived receipt under the supplied sequencer height-range keys. Admission checks the archive against its independently observed source commitment. To verify BFT consensus or application state independently, use native block-commit evidence and the relevant state proofs in addition to the mirror report.</p>
    <h2>Archive retention and audit</h2>
    <p>Keep the archive commitment, canonical archive bytes, trusted sequencer key windows, target identity, canonical chain position and evidence verification result together. For independent settlement verification, retain native block-commit evidence and the trusted validator configuration as well.</p>
    <p>Continue with <Link href="/relay-archive">Relay &amp; Archive Nodes</Link> for public history endpoints, <Link href="/proofs">Receipts &amp; Proofs</Link> for portable verification, and <Link href="/data-availability">Data Availability</Link> for authenticated replay bundles.</p>
  </DocsLayout>
}
