import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/custody')

export default function Custody() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Custody, deposits &amp; exits</h1>
        <p className="page-description">The pre-merge custody precompile is retired. This page states what replaced it and what still holds for key custody.</p>
      </div>
      <h2>The custody precompile is retired</h2>
      <p>The <code>layerxcustody</code> precompile at <code>0x0000000000000000000000000000000000001013</code> was retired from the merge upgrade height on the merged network. Every one of its 20 ABI methods — including <code>deposit</code>, <code>depositToken</code>, <code>requestWithdrawal</code>, <code>finaliseWithdrawal</code>, <code>requestForcedExit</code>, and <code>executeForcedExit</code> — now reverts with <code>LXP_MODULE_DISABLED: module disabled; layerxCustody retired</code>. The retirement gate runs before call-shape checks, so no argument combination reaches the old logic. Do not build new deposit, withdrawal, or exit flows against this address.</p>
      <p>The full historical interface remains in <code>contracts/src/precompiles/ILayerXCustody.sol</code> for binding compatibility and audit. It also declares views such as <code>registerDepositRoot</code>, the deposit-root authority and registration digests, <code>depositCount</code>, <code>depositNonce</code>, <code>assetByPointer</code>, and <code>nativeAssetId</code>; all of them revert the same way at runtime.</p>
      <h2>What replaced it</h2>
      <table><thead><tr><th>Need</th><th>Current path</th></tr></thead><tbody>
        <tr><td>Establish an owner account</td><td>Bind the DID to the owner EVM address with <code>bindLayerX(bytes32, bytes)</code> through the address precompile at <code>0x…1004</code>, then register identity and recovery through LayerXAuth. See <Link href="/identity">identity</Link>.</td></tr>
        <tr><td>Fund an account on the merged network</td><td>Fund the kernel account directly with the correct native asset. No custody credit is issued and no deposit proof is required.</td></tr>
        <tr><td>Move value to or from an external chain</td><td>Use the governed bridge at <code>0x0000000000000000000000000000000000001016</code>, which requires registered chains, attestors, caps, and enabled policy. See <Link href="/bridge">the bridge</Link>.</td></tr>
        <tr><td>Move funds inside the Human interface</td><td>Use a <code>move</code> journey on the merged network. The wallet-binding, deposit, withdraw, and exit journey kinds are retired and survive only as closed history.</td></tr>
      </tbody></table>
      <h2>Key custody is unchanged</h2>
      <p>Retiring the precompile did not change who holds signing keys in the Human interface. The three identities remain distinct:</p>
      <table><thead><tr><th>Identity</th><th>Purpose</th><th>Signing authority</th></tr></thead><tbody>
        <tr><td>Application identity</td><td>A passkey opens the Human application session.</td><td>Authenticates the application session; it is separate from the protocol key.</td></tr>
        <tr><td>Protocol identity</td><td>A LayerX DID authorizes native activities and account operations.</td><td>The custody service holds Human and managed-agent keys and signs disclosed activities.</td></tr>
        <tr><td>Owner EVM address</td><td>Binds the DID and submits Governance lifecycle transactions.</td><td>Signs the binding and lifecycle transactions on chain ID 125.</td></tr>
      </tbody></table>
      <p>The browser does not hold the Human protocol identity&apos;s private key. The one exception is the explicit <code>export-primary-key</code> security action, which is the only route by which a DID primary key leaves the service; see <Link href="/human-interface">the Human interface</Link>.</p>
      <h2>Historical deposits</h2>
      <p>Pre-merge deposits were recorded by the custody module and credited through a native custody-credit activity bound to the deposit record. That bootstrap is retired together with the precompile: it is not a prerequisite for funding an owner on the merged network, and it cannot be replayed to issue new credit. Deposit replay used the nullifier <code>SHA256(&quot;LX:DEPOSIT:NULLIFIER:v1&quot; || deposit_id)</code>, so a recorded deposit could only be credited once. Historical claim and deposit records remain readable in module state for audit, but the EVM entry points that acted on them no longer execute.</p>
      <h2>Show evidence accurately</h2>
      <p>Human journeys report three current rungs: <code>unverified</code> (submitted), <code>receipt-verified</code> (receipted), and <code>checkpoint-finalised</code> (committed). The former <code>paxeer-finalised</code> and <code>settlement-anchored</code> labels are retired decoding values and do not define additional rungs. Evidence returned by <code>GET /v1/evidence/&#123;evidence_id&#125;</code> includes its class, verification level, content type, and encoded bytes.</p>
      <p>Continue with <Link href="/identity">owner registration</Link>, <Link href="/bridge">the governed bridge</Link>, <Link href="/settlement">checkpoint settlement and proof verification</Link>, and <Link href="/json-rpc">Paxeer JSON-RPC</Link>.</p>
    </DocsLayout>
  )
}
