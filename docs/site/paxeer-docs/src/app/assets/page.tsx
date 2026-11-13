import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/assets')

export default function Assets() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Assets and issuance</h1><p className="page-description">Register assets, control issuance, move value, and connect business instruments to payments and markets across Paxeer X.</p></div>
    <h2>One network, several asset surfaces</h2>
    <p>Paxeer X combines native bank denominations, EVM token interfaces, and the LayerX Asset registry. Choose the authority and settlement path that match the instrument. A token name is display metadata: the registered asset identifier, bank denomination, or contract address identifies the actual balance authority.</p>
    <table><thead><tr><th>Surface</th><th>Identity</th><th>Primary use</th></tr></thead><tbody>
      <tr><td>LayerX registered asset</td><td>32-byte asset ID and issuer DID</td><td>Authorized issuance, agent payments, payer grants, supply-bound receipts.</td></tr>
      <tr><td>Native bank asset</td><td>Bank denomination</td><td>Native transfers, tokenfactory issuance, module settlement.</td></tr>
      <tr><td>ERC-20 interface</td><td>EVM address</td><td>Wallet and contract integration, including native-asset pointers.</td></tr>
      <tr><td>Launchpad instrument</td><td>ERC-20 token and pool addresses; denomination <code>launchpad/&lt;pool hex&gt;</code></td><td>Fixed-supply issuance and a virtual-reserve trading market.</td></tr>
    </tbody></table>
    <p>Agents create and operate businesses, and trade instruments those businesses issue. The business, its instrument, and the agent acting for it have separate identities. A token balance does not by itself define business ownership, revenue rights, or a legal claim: those terms belong to the instrument and its associated agreement. Read <Link href="/launchpad">Launchpad</Link> for issuance into a curve market and <Link href="/spot-exchange">Exchange</Link> for market execution.</p>
    <h2>Register a LayerX asset</h2>
    <p>The Asset module is module <code>1</code>. Registration records the issuer, symbol, name, decimals, supply cap, custody reference, and salt. Amounts are unsigned integer counts of the smallest unit; decimals only affect presentation. For example, an asset with six decimals represents 12.5 display units as <code>12500000</code> base units.</p>
    <table><thead><tr><th>Field</th><th>Rule</th></tr></thead><tbody>
      <tr><td>Symbol</td><td>1–16 ASCII bytes.</td></tr><tr><td>Name</td><td>1–32 bytes of valid UTF-8.</td></tr><tr><td>Decimals</td><td>At most 38.</td></tr><tr><td>Supply cap</td><td>Unsigned 128-bit integer; zero means uncapped.</td></tr><tr><td>Issuer kind</td><td>1 for native issuance; 2 for Paxeer custody.</td></tr><tr><td>Custody reference</td><td>At most 128 bytes; empty for native issuance.</td></tr>
    </tbody></table>
    <h3>Identifiers and accounts</h3>
    <pre><code>{`native_asset_id = SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)
account_id = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)

agent:<DID>:main
agent:<DID>:asset:<asset_id_hex64>`}</code></pre>
    <p>Hexadecimal text inside canonical account names is lowercase. Custody-backed assets use the supplied custody asset ID rather than deriving a native ID. Registration creates the asset record and an issuance account. Duplicate asset IDs are rejected.</p>
    <h2>Issue, transfer, and retire units</h2>
    <ol><li>Register the asset under the issuing DID and record its asset ID.</li><li>Open a per-asset account for the actor receiving or holding units.</li><li>Mint a positive base-unit amount to a matching asset account, with issuer authorization and available issuance units.</li><li>Send units directly or authorize a recipient to draw against a scoped payer grant.</li><li>Burn units from an actor-owned account when retiring supply.</li></ol>
    <p>Mint moves units from the issuance account to the destination. Burn returns units to issuance. The module checks account ownership, asset identity, available balance, and supply invariants before updating balances. A nonzero supply cap limits issuance. Supply before and after are bound into asset transition receipts, with withdrawal handled through its separate settlement path.</p>
    <table><thead><tr><th>Ordinal</th><th>Operation</th><th>Authority or effect</th></tr></thead><tbody>
      <tr><td>1</td><td>Register</td><td>Create the canonical asset and issuance account.</td></tr><tr><td>2 / 3</td><td>Pause / unpause</td><td>Recorded issuer controls the asset pause state.</td></tr><tr><td>4</td><td>Account open</td><td>Open the actor&apos;s account for the asset.</td></tr><tr><td>5 / 6</td><td>Send / receive</td><td>Owner-authorized send or grant-authorized recipient draw.</td></tr><tr><td>7 / 8</td><td>Grant issue / revoke</td><td>Create or revoke a narrowly scoped payer grant.</td></tr><tr><td>9</td><td>Withdraw</td><td><code>LX_ASSET_WITHDRAW</code> (<code>0x00010009</code>): custody withdrawal through the settlement path.</td></tr><tr><td>10 / 11</td><td>Mint / burn</td><td>Issuer-controlled mint; actor-owned source for burn.</td></tr>
    </tbody></table>
    <h2>Scoped payments and recurring draws</h2>
    <p>A payer grant names one recipient, one payer account, one asset, per-draw maximum, total allowance, expiry, and purpose. Recurring grants also bind a window length. Wildcard recipients and accounts are excluded. Receive verifies the grant, receiver authorization, purpose, network, signed context, and revocation sequence together.</p>
    <p>A business can authorize a service agent to collect metered fees without handing it unrestricted wallet authority. Keep the service purpose and maximum draw explicit; revoke the grant when the relationship ends. <Link href="/payments">Payments</Link> explains direct transfers and <Link href="/paid-apis">x402</Link> covers metered service collection.</p>
    <h2>Pause and custody settlement</h2>
    <p>Only the recorded issuer pauses or unpauses an asset. A pause blocks send, mint, burn, account opening, grant issuance, and withdrawal. Revocation remains a distinct operation, allowing authorization to be withdrawn without creating a new payment.</p>
    <p>Custody withdrawal debits the asset account into the withdrawal settlement account and binds a payout recipient, checkpoint ID, amount, and fee. Paxeer-side payout uses a claim against a finalized checkpoint. Use the <Link href="/bridge">Bridge</Link> workflow for that claim; an ordinary asset transfer does not release custody funds.</p>
    <h2>Discover balances and verify context</h2>
    <p><code>px_listAssets</code> lists registry records in ascending asset-ID order. Supply an optional exclusive <code>cursor</code> and a <code>limit</code> from 1 to 256; the default is 64. Follow <code>next_cursor</code> until it is null. There is no single-asset wire method: the SDK&apos;s <code>get_asset()</code> scans <code>px_listAssets</code> pages for the requested ID. <code>px_getBalances</code> reads DID accounts. Fee estimation (<code>estimate_fee()</code>) calls <code>eth_estimateGas</code> on an EIP-1559 transaction; it does not price canonical activities over a dedicated method.</p>
    <p>Reads carry an observed sequence and committed state root. The label <code>authenticated_committed_snapshot</code> identifies an authenticated committed snapshot; independent inclusion proofs and finality are separate verification concerns. Match the snapshot, asset identifier, issuer, pause state, and base-unit amount before signing.</p>
    <h2>Continue building</h2>
    <p>Use <Link href="/modules/tokenfactory">Tokenfactory</Link> for native denomination administration, <Link href="/contracts">Pointers</Link> for cross-runtime token interfaces, and <Link href="/launchpad">Launchpad</Link> for fixed-supply curve markets.</p>
  </DocsLayout>
}
