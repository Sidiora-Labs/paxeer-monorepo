import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/custody')

export default function Custody() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Account funding &amp; key custody</h1>
        <p className="page-description">Fund accounts on the merged network, bind owner identities, and understand signing-key custody in the Human interface.</p>
      </div>
      <h2>Custody precompile behavior</h2>
      <p>The <code>layerxcustody</code> precompile at <code>0x0000000000000000000000000000000000001013</code> is disabled on the merged network. Its 20 ABI methods, including <code>deposit</code>, <code>depositToken</code>, <code>requestWithdrawal</code>, <code>finaliseWithdrawal</code>, <code>requestForcedExit</code>, and <code>executeForcedExit</code>, revert with <code>LXP_MODULE_DISABLED: module disabled; layerxCustody retired</code>. The disabled-module check precedes call-shape checks.</p>
      <p>The ABI is declared in <code>contracts/src/precompiles/ILayerXCustody.sol</code>. Its deposit-root registration method, <code>registerDepositRoot</code>, and its views, including <code>depositCount</code>, <code>depositNonce</code>, <code>assetByPointer</code>, and <code>nativeAssetId</code>, are subject to the same runtime refusal.</p>
      <h2>Account and transfer paths</h2>
      <table><thead><tr><th>Need</th><th>Current path</th></tr></thead><tbody>
        <tr><td>Establish an owner account</td><td>Bind the DID to the owner EVM address with <code>bindLayerX(bytes32, bytes)</code> through the address precompile at <code>0x0000000000000000000000000000000000001004</code>, then register identity and recovery through LayerXAuth. See <Link href="/identity">identity</Link>.</td></tr>
        <tr><td>Fund an account on the merged network</td><td>Transfer the native asset directly to the bound EVM address. Development allocations use a signed native transfer; see the developer funding guide.</td></tr>
        <tr><td>Move value to or from an external chain</td><td>Use the governed bridge at <code>0x0000000000000000000000000000000000001016</code>, which requires registered chains, attestors, caps, and enabled policy. See <Link href="/bridge">the bridge</Link>.</td></tr>
        <tr><td>Move funds inside the Human interface</td><td>Use a <code>move</code> journey and track its receipt and block commitment.</td></tr>
      </tbody></table>
      <h2>Signing-key custody</h2>
      <p>The Human interface separates application authentication, protocol signing, and owner EVM authority:</p>
      <table><thead><tr><th>Identity</th><th>Purpose</th><th>Signing authority</th></tr></thead><tbody>
        <tr><td>Application identity</td><td>A passkey opens the Human application session.</td><td>Authenticates the application session; it is separate from the protocol key.</td></tr>
        <tr><td>Protocol identity</td><td>A LayerX DID authorizes native activities and account operations.</td><td>The custody service holds Human and managed-agent keys and signs disclosed activities.</td></tr>
        <tr><td>Owner EVM address</td><td>Binds the DID and submits Governance lifecycle transactions.</td><td>Signs the binding and lifecycle transactions on chain ID 125.</td></tr>
      </tbody></table>
      <p>The browser does not hold the Human protocol identity&apos;s private key. The one exception is the explicit <code>export-primary-key</code> security action, which is the only route by which a DID primary key leaves the service; see <Link href="/human-interface">the Human interface</Link>.</p>
      <h2>Confirm account funding</h2>
      <p>Confirm the DID-to-EVM binding before funding. Retain the transaction hash, inspect the native balance and receipt, and check the recipient and amount. The native funding path resolves the recipient through the address precompile and checks the transaction receipt. A funding reference alone does not establish a successful transfer.</p>
      <h2>Show evidence accurately</h2>
      <p>Human journeys report three rungs: <code>unverified</code> (submitted), <code>receipt-verified</code> (receipted), and <code>checkpoint-finalised</code> (committed). A receipt is final once its block is committed. Evidence returned by <code>GET /v1/evidence/&#123;evidence_id&#125;</code> includes its class, verification level, content type, and encoded bytes.</p>
      <p>Continue with <Link href="/identity">owner registration</Link>, <Link href="/bridge">the governed bridge</Link>, <Link href="/settlement">block commitment and proof verification</Link>, and <Link href="/json-rpc">Paxeer JSON-RPC</Link>.</p>
    </DocsLayout>
  )
}
