import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/security')

export default function Security() {
  return <DocsLayout>
    <h1>Security and trust boundaries</h1>
    <p className="page-description">How transaction signatures, receipt keys, committed blocks, and scoped service credentials establish trust in Paxeer X.</p>
    <p>Paxeer mainnet-beta is one BFT network. The kernel executes within the chain, and a transaction is final when its covering block is committed. A signed execution receipt establishes a narrower claim before commitment. Applications verify the evidence needed before releasing goods or updating balances.</p>
    <h2>The trust boundaries</h2>
    <table><thead><tr><th>Boundary</th><th>Verification responsibility</th><th>Remaining dependency</th></tr></thead><tbody><tr><td>Signer to node</td><td>Signed transaction, chain identifier, nonce, gas, and balance checks.</td><td>Key custody and correct transaction construction.</td></tr><tr><td>Node to client</td><td>Canonical receipt, authorized receipt key, raw transaction hash, and execution status.</td><td>Receipt-key trust and node availability.</td></tr><tr><td>Proposer to validators</td><td>Sequence-batch signature, sequence continuity, and transaction replay.</td><td>Consensus validator set and deterministic execution.</td></tr><tr><td>Block to application</td><td>Transaction inclusion and consensus commit under a trusted validator set.</td><td>Authenticated chain and validator-set trust.</td></tr><tr><td>Application to hosted service</td><td>Authenticated request, scoped authorization, and evidence for the resulting operation.</td><td>Service availability and credential custody.</td></tr></tbody></table>
    <h2>Evidence levels are consequential</h2>
    <p>A submission response identifies a transaction. A signed receipt authenticates execution under a pinned receipt key. A covering block and verified consensus commit establish committed inclusion. Applications retain the signed transaction and check its hash, receipt outcome, and block evidence together.</p>
    <p>Native balances and EVM execution belong to the same network. Receipt signatures authenticate the signer and payload; consensus commitment establishes finality. Neither service authentication nor a matching chain identifier proves a payment.</p>
    <h2>Canonical execution and replay</h2>
    <p>The node validates the ordered sequence batch and re-executes proposed transactions. Sequence continuity, signatures, and committed roots bind the execution order. Query indexes expose derived views and do not replace consensus evidence.</p>
    <p>Program monetary effects pass through protocol transfer sets and the asset module. Storage occupancy is accounted for during deterministic execution. Applications check the resulting execution status and transfer evidence before treating a program call as successful.</p>
    <h2>Verify receipt authority</h2>
    <p>The node exposes signed execution receipts bound to the raw EVM transaction hash. Clients authenticate the signing key against independently trusted key identifiers, public keys, and valid height ranges, then check the canonical receipt and execution outputs.</p>
    <h3>Trusted receipt keys</h3>
    <p>Receipt-key entries come from <code>px_getReceiptKeys</code>. Their key identifiers, public keys, and inclusive height ranges describe the governed receipt-signing authority. Establish trust in that key list independently of the receipt response.</p>
    <ol><li>Start with the exact signed transaction and its hash on the intended network.</li><li>Retrieve its canonical receipt and authenticate the signing key for the receipt height.</li><li>Check the signature, transaction binding, execution status, and expected payment or program effect.</li><li>Authenticate committed block inclusion before treating the transaction as final.</li></ol>
    <p>A signature over another transaction does not establish the requested payment. A root or signing key copied from an untrusted response does not authenticate execution or consensus membership.</p>
    <h3>Refusals and retry behavior</h3>
    <p>Authentication failures, unknown transactions, execution refusals, and unavailable dependencies have different meanings. Confirm the identifier when a transaction is unknown, inspect the execution outcome when it is refused, and retain the original transaction when transport failure leaves its outcome uncertain.</p>
    <p>A liveness or readiness response reports service availability. It does not certify a transaction, establish receipt-key authority, or prove a block commit.</p>
    <h2>Keys, roles, and custody</h2>
    <p>Owner keys, identity sessions, scoped gateway keys, capabilities, and budgets carry distinct authority. Grant only the operations, assets, amounts, and lifetime the application needs. Check revocation and current authorization before signing.</p>
    <p>Validator governance and receipt-key governance serve different verification roles. The <code>modules/sequencer</code> module stores the leader list, receipt keys, and committed sequence position. See <Link href="/guarantors">consensus and signing roles</Link> when selecting the trust inputs for receipt verification.</p>
    <h2>Emergency response and upgrade control</h2>
    <p>Upgrade controls follow the specific chain or contract authority. Contract timelocks restrict permitted calls, role assignments, delays, and execution windows. Applications inspect those deployed parameters rather than inferring authority from a service response.</p>
    <p>During an outage, preserve signed transactions, receipts, and block inclusion evidence. Resume lookup using the original transaction hash and establish its outcome before authorizing another economic action.</p>
    <h2>Integration review</h2>
    <ul><li>Pin the chain, validator set, and receipt-key authority.</li><li>Reject noncanonical encodings and evidence bound to another transaction or sequence.</li><li>Keep retries idempotent and retain the original transaction hash.</li><li>Distinguish submission, authenticated execution, and committed inclusion.</li><li>Inspect current role assignments, thresholds, delays, and limits when evaluating contract authority.</li></ul>
    <p>Continue with <Link href="/guarantors">verification roles</Link>, <Link href="/governance">Governance</Link>, and <Link href="/finality">Finality</Link>.</p>
  </DocsLayout>
}
