import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/sidiora')

export default function Sidiora() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Sidiora</h1>
        <p className="page-description">The SID asset on Paxeer X: native balances, its stable ERC-20 address, bridge-controlled supply, and sponsored or native gas payments.</p>
      </div>
      <h2>Asset Identity</h2>
      <p>Sidiora is the second official coin of Paxeer X. Its native denomination and ERC-20 interface represent one asset, giving wallets, contracts and Cosmos applications a consistent balance model.</p>
      <table>
        <thead><tr><th>Property</th><th>Value</th></tr></thead>
        <tbody>
          <tr><td>Symbol</td><td><code>SID</code></td></tr>
          <tr><td>Decimals</td><td><code>6</code>; one SID is <code>1,000,000</code> base units</td></tr>
          <tr><td>ERC-20 address</td><td><code>0x21f7b20a555199fa73A238B1a91FD0f549068fEe</code></td></tr>
          <tr><td>Native denom</td><td><code>factory/pax1dzfx9mk4fl9kl2mysjmtvk2xp75ljumk6nynhf/usid</code></td></tr>
          <tr><td>Mint and burn authority</td><td>The <code>x/layerxbridge</code> module account</td></tr>
          <tr><td>Proxy administration</td><td>Governance through a restricted upgrade timelock</td></tr>
        </tbody>
      </table>
      <p>PAX uses the base denom <code>uhpx</code> and eighteen decimals. Keep SID and PAX scaling separate in displays, approvals, transfer amounts, fee quotations and accounting. Read supply from the native bank state or ERC-20 <code>totalSupply</code> for a current value.</p>
      <h2>One Asset Across Native and EVM Interfaces</h2>
      <p>The native ERC-20 implementation reads <code>balanceOf</code> from the bank balance of the SID denom and <code>totalSupply</code> from its bank supply. Transfers move that denom through the bank precompile. A failed bank operation reverts the contract call. The pointer record resolves the denom to the same stable ERC-20 address.</p>
      <p>The proxy address remains constant through implementation upgrades. Native token metadata and allowances use an ERC-7201 storage namespace that preserves legacy implementation slots. Applications query current allowances before building spending transactions and request the approval required by the current interface.</p>
      <h3>Bridge-Controlled Supply</h3>
      <p>The bridge module account administers the tokenfactory denom. Attested bridge-in credits mint SID and bridge-out operations burn the departing amount. Denom creation verifies the bridge-derived creator, expected admin and remote asset mapping. Ordinary holders and application contracts have no minting authority.</p>
      <h2>Two Ways to Pay Gas in SID</h2>
      <table>
        <thead><tr><th>Property</th><th>Sponsored gas</th><th>Native fee preference</th></tr></thead>
        <tbody>
          <tr><td>Execution</td><td>EIP-7702 account batch and quote service</td><td>Chain fee processing</td></tr>
          <tr><td>SID payment</td><td>Atomic transfer to the sponsor inside the batch</td><td>Fee debit in SID before execution</td></tr>
          <tr><td>Pricing</td><td>Signed quote checked against the paymaster rate</td><td>Governance-controlled conversion rate</td></tr>
          <tr><td>Replay protection</td><td>Account batch nonce and sponsor quote nonce</td><td>Normal transaction replay protection and account preference</td></tr>
          <tr><td>Unused gas</td><td>Governed by the signed sponsorship quote</td><td>Refunded in the selected fee denom</td></tr>
          <tr><td>PAX transaction value</td><td>The account still supplies value it sends</td><td>The account still supplies value it sends</td></tr>
        </tbody>
      </table>
      <h2>Sponsored Transaction Workflow</h2>
      <ol>
        <li>Build the application calls and request a sponsor quote for the declared gas cost.</li>
        <li>Verify the configured sponsor, SID token, six-decimal units, chain ID, deadline, quoted amount and maximum.</li>
        <li>Sign the account batch, covering its nonce, calls hash and quote digest, and authorize the EIP-7702 execution.</li>
        <li>Execute through <code>BatchCallAndSponsor</code>; the account runs the calls and repays the sponsor in SID in the same transaction.</li>
      </ol>
      <p>The sponsor signature binds chain ID, paymaster, sponsor, token, maximum amount, offered amount, deadline, quote nonce and declared gas cost. The account signature must recover to the account itself. Used quote nonces and account batch nonces prevent replay.</p>
      <h3>Pricing and Atomicity</h3>
      <p>The paymaster checks a positive, sufficiently fresh rate and requires the offered amount to fall within five percent of the amount implied by the declared gas cost. Expired quotes, zero amounts, an incorrect token or invalid sponsor are refused. A failed application call or SID repayment reverts the whole batch.</p>
      <p>Quote services apply their own budget and rate-freshness limits before signing a quote. Missing or invalid pricing inputs produce a refusal. Validate the returned quote and signatures locally before building <code>executeSponsored</code>.</p>
      <h2>Native Fee Preferences</h2>
      <p>The fee-token precompile at <code>0x0000000000000000000000000000000000001018</code> exposes the account preference:</p>
      <pre><code>{`function setFeeDenom(string calldata denom) external;
function getFeeDenom(address account) external view returns (string memory denom);
function clearFeeDenom() external;`}</code></pre>
      <p>Call <code>setFeeDenom</code> with the SID native denom to select it for the caller, query with <code>getFeeDenom</code>, or clear the preference to return to the network coin. Only allowed fee denoms can be selected. The precompile rejects value and delegatecall; static calls cannot change preferences.</p>
      <h3>Charging and Refunding</h3>
      <p>For EVM transactions, fee processing converts the maximum gas charge, including a blob fee when applicable, into the chosen denom. The account must have sufficient spendable SID for that charge and sufficient PAX for any transferred value. The fee debit replaces the PAX gas purchase.</p>
      <p>Unused gas returns in the same denom. Cosmos transactions can offer allowed fee denoms, whose governed conversion is used for minimum-gas-price checks.</p>
      <h2>Governed Rates and Fee Routing</h2>
      <p>Fee conversion uses a governance-controlled rate for each allowed denom. Only allowed fee denoms can be selected through the fee-token precompile. Applications read the current fee parameters before presenting a fee option, and re-estimate when a rate or policy changes.</p>
      <p>PAX fees keep their ordinary collection path through the fee collector.</p>
      <h2>Upgrade Authority</h2>
      <p>The Sidiora proxy timelock accepts only governance-scheduled <code>upgradeToAndCall(address,bytes)</code> operations targeting the proxy with zero value. The implementation must be a contract distinct from the proxy and timelock. Upgrade and initialization execute together.</p>
      <p>A designated executor runs an operation after its delay and within its grace period. A guardian can cancel before readiness. The handover governance proposal specifies a minimum delay of 172,800 seconds (two days), a delay floor of 86,400 seconds (one day) and an execution grace period of 604,800 seconds (one week). These delays are governance and constructor parameters, not constants in the token contract. These controls bound implementation changes while preserving the token address.</p>
      <h2>Integration Checklist</h2>
      <ul>
        <li>Use six-decimal SID amounts and query the current balance and allowance.</li>
        <li>Resolve the native denom and stable ERC-20 address as the same asset.</li>
        <li>Display the signed sponsorship amount or estimated native fee in SID before approval.</li>
        <li>Distinguish gas from PAX value sent by the transaction.</li>
        <li>Refresh rates and quotes before submission and preserve transaction evidence for accounting.</li>
      </ul>
      <p>Explore <Link href="/unified-network">Unified Network</Link>, <Link href="/contracts">Contracts</Link>, <Link href="/precompiles">Precompiles</Link> and <Link href="/xweb">xweb</Link>, which accepts SID for agent web requests.</p>
    </DocsLayout>
  )
}
