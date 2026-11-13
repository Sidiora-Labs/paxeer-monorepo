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
      <tr><td>LayerX registered asset</td><td>32-byte asset ID and issuer DID</td><td>Authorized issuance, account transfers, and supply-bound receipts.</td></tr>
      <tr><td>Native bank asset</td><td>Bank denomination</td><td>Native transfers, tokenfactory issuance, module settlement.</td></tr>
      <tr><td>ERC-20 interface</td><td>EVM address</td><td>Wallet and contract integration, including native-asset pointers.</td></tr>
      <tr><td>Launchpad instrument</td><td>ERC-20 token and pool addresses</td><td>Fixed-supply issuance and a virtual-reserve trading market.</td></tr>
    </tbody></table>
    <p>Agents create and operate businesses, and trade instruments those businesses issue. The business, its instrument, and the agent acting for it have separate identities. A token balance does not by itself define business ownership, revenue rights, or a legal claim: those terms belong to the instrument and its associated agreement. Read <Link href="/launchpad">Launchpad</Link> for issuance into a curve market and <Link href="/spot-exchange">Exchange</Link> for market execution.</p>
    <h2>Register a LayerX asset</h2>
    <p>The Asset module is module <code>1</code>. Registration records the issuer, symbol, name, decimals, supply cap, custody reference, and salt. Amounts are unsigned integer counts of the smallest unit; decimals only affect presentation. For example, an asset with six decimals represents 12.5 display units as <code>12500000</code> base units.</p>
    <table><thead><tr><th>Field</th><th>Rule</th></tr></thead><tbody>
      <tr><td>Symbol</td><td>1–16 ASCII bytes.</td></tr><tr><td>Name</td><td>1–32 bytes of valid UTF-8.</td></tr><tr><td>Decimals</td><td>At most 38.</td></tr><tr><td>Supply cap</td><td>Unsigned 128-bit integer; zero permits supply up to the unsigned 128-bit maximum.</td></tr><tr><td>Issuer kind</td><td>1 for native issuance; 2 for Paxeer custody.</td></tr><tr><td>Custody reference</td><td>At most 128 bytes; empty for native issuance.</td></tr>
    </tbody></table>
    <h3>Identifiers and accounts</h3>
    <pre><code>{`native_asset_id = SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)
account_id = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)

agent:<DID>:main
agent:<DID>:asset:<asset_id_hex64>`}</code></pre>
    <p>Hexadecimal text inside canonical account names is lowercase. Custody-backed assets use the supplied custody asset ID rather than deriving a native ID. Registration creates the asset record and an issuance account named <code>module:asset:value:&lt;issuance-account-id&gt;</code>. A module value account decodes its embedded identifier rather than hashing the full name. Duplicate asset IDs are rejected.</p>
    <h2>Issue, transfer, and retire units</h2>
    <ol><li>Register the asset under the issuing DID and record its asset ID.</li><li>Open a per-asset account for the actor receiving or holding units.</li><li>Mint a positive base-unit amount to a matching asset account, with issuer authorization and available issuance units.</li><li>Send units from an actor-owned account to the recipient account.</li><li>Burn units from an actor-owned account when retiring supply.</li></ol>
    <p>Mint moves units from the issuance account to the destination. Burn returns units to issuance. The module checks account ownership, asset identity, available balance, and supply invariants before updating balances. A nonzero supply cap limits issuance. Supply before and after are bound into asset transition receipts.</p>
    <table><thead><tr><th>Ordinal</th><th>Operation</th><th>Authority or effect</th></tr></thead><tbody>
      <tr><td>1</td><td>Register</td><td>Create the canonical asset and issuance account.</td></tr><tr><td>2</td><td>Pause</td><td>Recorded issuer pauses asset operations.</td></tr><tr><td>3</td><td>Unpause</td><td>Recorded issuer restores asset operations.</td></tr><tr><td>4</td><td>Account open</td><td>Open the actor&apos;s account for the asset.</td></tr><tr><td>5</td><td>Send</td><td>Transfer units from an actor-owned source.</td></tr><tr><td>10</td><td>Mint</td><td>Issuer-authorized transfer from issuance to an asset account.</td></tr><tr><td>11</td><td>Burn</td><td>Return units from an actor-owned account to issuance.</td></tr>
    </tbody></table>
    <h2>Account transfers and spending limits</h2>
    <p>A send names its source account, destination account, asset, positive base-unit amount, and idempotency key. The source belongs to the actor. Both accounts hold the same asset, and the source must have enough units. The signed context binds the transfer fields.</p>
    <p>An agent can apply daemon capabilities and local limits when preparing payments. Keep the permitted asset, counterparty, and maximum amount explicit. <Link href="/payments">Payments</Link> describes payment integration and <Link href="/paid-apis">x402</Link> covers metered service collection.</p>
    <h2>Pause controls and custody metadata</h2>
    <p>Only the recorded issuer pauses or unpauses an asset. A pause blocks send, mint, burn, and account opening.</p>
    <p>Custody-kind registration records the supplied asset identifier and custody reference. The seven operations above define the active Asset dispatcher. The <Link href="/bridge">Bridge</Link> page describes the separate bridge surface.</p>
    <h2>Discover balances and verify context</h2>
    <p><code>px_listAssets</code> on the hosted gateway selects up to 16 registry records in registration order. Its optional <code>cursor</code> is a canonical positive signed-64-bit decimal row ID. Follow <code>next_cursor</code> until it is null. The response includes <code>height</code>, <code>assets</code>, <code>joined_limit: 16</code>, and <code>order: "registration"</code>. For one native asset, call <code>getAsset</code> on the ledger precompile at <code>0x101A</code> through <code>eth_call</code>. <code>px_getBalances</code> resolves an EVM address or bound DID and reads bank balances for that EVM address. The Rust SDK&apos;s <code>estimate_fee()</code> calls <code>eth_estimateGas</code> and returns a gas estimate.</p>
    <p>Asset metadata reads are pinned to the returned block height. Unavailable metadata leaves name, symbol, decimals, and supply null. A successful ledger read also includes <code>paused</code> and <code>verification: "authenticated_committed_snapshot"</code>. Independent inclusion proofs require additional evidence. Match the height, asset identifier, issuer, pause state, and base-unit amount before signing.</p>
    <h2>Continue building</h2>
    <p>Use <Link href="/modules/tokenfactory">Tokenfactory</Link> for native denomination administration, <Link href="/contracts">Pointers</Link> for cross-runtime token interfaces, and <Link href="/launchpad">Launchpad</Link> for fixed-supply curve markets.</p>
  </DocsLayout>
}
