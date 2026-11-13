import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/relay-archive')

export default function RelayArchive() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Relay &amp; Archive Nodes</h1><p className="page-description">Status of the former relay/archive node product and its RPC surface.</p></div>
    <h2>Relay/archive is retired</h2>
    <p>The relay/archive node product is retired. There is no installable relay/archive service in the monorepo: the install assets (<code>install.sh</code>, <code>config.example.json</code>, <code>deployment.yaml</code>, and the <code>layerx-relay-archive.service</code> unit) must be absent, and retirement tests assert their absence. There is no <code>services/platform/relay_archive/</code> service, no <code>docker/relay-archive/</code> image, and the beta cluster deploys no relay-archive component. The former startup command <code>layerxd --relay-archive</code> no longer exists; the <code>layerxd</code> daemon itself is retired.</p>
    <p>Do not follow older install, configuration, or tuning instructions for a relay/archive node, including release-bundle contents such as <code>layerx-archive-codec</code>. No successor install path was found in the monorepo.</p>
    <h2>lx_* RPC methods are switched off</h2>
    <p>All 26 <code>lx_*</code> JSON-RPC methods are switched off (Decision 701) at both the node and the gateway. This includes the archive and subscription methods previously documented here — <code>lx_getArchiveNetwork</code>, <code>lx_getArchiveHead</code>, <code>lx_listActivities</code>, <code>lx_listBatches</code>, <code>lx_getNodeInfo</code>, <code>lx_getActivityStatus</code>, <code>lx_getReceipt</code>, <code>lx_getBatchHeader</code>, <code>lx_sendActivity</code>, and <code>lx_subscribe</code> with its resume-window error codes. Clients must not depend on any <code>lx_*</code> method.</p>
    <h2>HTTP sync and history routes</h2>
    <p>The previously documented <code>/v1/sync/*</code> and <code>/v1/history/*</code> HTTP surface has no live implementation in the monorepo. Only the specification contract (<code>contracts/platform-relay-archive.md</code> in the spec tree) remains. Whether any non-retired successor serves those routes could not be confirmed from source, so they are not documented as available endpoints.</p>
    <h2>What to use instead</h2>
    <p>For chain history and state, run a <code>paxd</code> full or archive node (see <Link href="/run-node">Run a Node</Link> and <Link href="/configuration">Configuration</Link>). For receipts and proofs, see <Link href="/proofs">Receipts &amp; Proofs</Link> and <Link href="/data-availability">Data Availability</Link>. For external mirrors, see <Link href="/mirrors">External Mirrors</Link>.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/platform/tests/test_relay_archive_install_retired.py</code>, <code>services/platform/hosted/tests/test_beta_relay_archive_retired.py</code>, <code>rpc/lx_switch_off_test.go</code>.</div>
  </DocsLayout>
}
