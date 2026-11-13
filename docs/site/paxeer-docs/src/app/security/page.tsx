import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/security')

export default function Security() {
  return <DocsLayout>
    <h1>Security and trust boundaries</h1>
    <p className="page-description">What signatures, replay, checkpoint settlement, custody controls, and authority services establish in Paxeer X.</p>
    <p>Paxeer X separates fast ordered execution from settlement and custody. A receipt establishes a specific execution claim; its inclusion evidence, sequencer authorization, guarantor certificate, and settlement state determine how that claim is verified. Applications preserve these distinctions when releasing goods, updating balances, or permitting withdrawals.</p>
    <h2>The trust boundaries</h2>
    <table><thead><tr><th>Boundary</th><th>Verification responsibility</th><th>Remaining dependency</th></tr></thead><tbody><tr><td>Actor to sequencer</td><td>Canonical activity encoding, actor signature, authority scope, sequence, and fee rules.</td><td>Key custody and correctly scoped delegation.</td></tr><tr><td>Sequencer to client</td><td>Authorized header signature, receipt inclusion, batch identity, and outcome.</td><td>Sequencer availability and the requested evidence level.</td></tr><tr><td>Sealed batch to guarantor</td><td>Availability proofs, deterministic replay, and root comparison.</td><td>Accessible batch data and eligible honest guarantors.</td></tr><tr><td>Checkpoint to Paxeer</td><td>Domain-bound eligible attestations and registry acceptance.</td><td>Settlement-chain execution and configured challenge and withdrawal rules.</td></tr><tr><td>Application to hosted authority</td><td>Authenticated request and pinned identities; verify the resulting receipt facts.</td><td>Authority and replica availability.</td></tr></tbody></table>
    <h2>Evidence levels are consequential</h2>
    <p>Distinguish acceptance, execution, attestation, and settlement as separate evidence levels: acceptance is an ordering receipt, attestation adds bonded replay, and settlement registers the checkpoint on Paxeer. The gateway exposes executed-state handling; use the evidence level required by the application rather than interpreting every successful response as settlement. A canonical L0–L4 naming ladder and a complete commitment-level enum were not verified in source and are not asserted here.</p>
    <p>Custody remains on Paxeer. Guarantor bonds, challenge windows, withdrawal limits, and emergency exits bound settlement risk under their configured rules. The checkpoint path uses bonded re-execution and does not claim a SNARK or another validity proof.</p>
    <h2>Canonical execution and replay</h2>
    <p>The append-only activity log is the execution record. Indexes are derived views. Replay checks actor authority, receipts, state transitions, recovery metadata, and roots. Consensus execution excludes floating point, local-clock dependence, and unstable iteration so independent machines reproduce identical bytes.</p>
    <p>Program monetary effects are represented as transfer sets in the same history; they are not private balance writes outside the protocol. Storage occupancy is also replay-checkable. Missing availability data or inconsistent replay results prevent a guarantor from signing.</p>
    <h2>Verify hosted receipt authority</h2>
    <p>The hosted authority obtains a canonical receipt and requests evidence from a pinned replica. It checks the replica identity, pinned sequencer public key, signed header, Merkle inclusion, protocol version, sequence range, and re-derived execution batch identifier. Outcome verification then runs against these derived batch facts.</p>
    <h3>Authorized batch facts</h3>
    <p>The hosted authority references an authenticated authorized-batches route (<code>GET /v1/authorized-batches/by-activity/&#123;activity_id&#125;</code>) for derived batch facts. Its exact response schema was not verified in authority source (only README/test references were found) and is not specified here; read the deployed service&apos;s contract before depending on field names.</p>
    <ol><li>Start with the exact activity identifier and the intended network and wire version.</li><li>Retrieve the canonical receipt and authorized evidence through the configured authenticated service.</li><li>Verify the receipt against the derived batch facts and the application&apos;s expected outcome.</li><li>Require the appropriate checkpoint and settlement evidence for the action being authorized.</li></ol>
    <p>A valid signature over a different batch does not establish inclusion for the requested activity. A root copied from an unverified response does not establish authorized execution.</p>
    <h3>Refusals and retry behavior</h3>
    <p>Treat authentication failures, unknown activities, refused evidence, and unavailable dependencies as distinct outcomes: fix credentials for the first, confirm the identifier for the second, do not retry refused evidence as if it were transient, and retry dependency outages with backoff. Exact status codes and error strings for the authorized-batches route were not verified in source and are not listed here.</p>
    <p>Service readiness probes the replica, and liveness reports process availability. Neither response certifies a particular activity or proves that the sequencer is reachable.</p>
    <h2>Keys, roles, and custody</h2>
    <p>Owner signatures, session grants, delegated capabilities, and budget allowances carry distinct authority scopes. Revocation and primary-key rotation are ordered governance events. Grant only the operations, assets, budgets, and lifetime the application needs, and verify recovery and signer history when evaluating authority.</p>
    <p>Guarantor membership, bond control, custody authority, checkpoint publication authority, and settlement submission are separate responsibilities. The live guarantor model is the Go <code>layerxanchor</code> Cosmos module (see <Link href="/guarantors">Guarantors</Link>), not a GuarantorBond contract. A funded submitter can publish an authorized certificate; funding does not give it the ability to manufacture eligible attestations or owner authorization.</p>
    <h2>Emergency response and upgrade control</h2>
    <p>Protocol emergency controls identify their scope, trigger, exit conditions, epoch, and ordered sequence. Settlement timelocks restrict proposers, executors, guardians, target selectors, call value, delays, and execution windows. Layer X component attestations identify role, configuration, release, and layout; upgrade authority follows the specific component or proxy path.</p>
    <p>During a stall, preserve the latest verified receipts, inclusion proofs, batch data, and checkpoint evidence. Determine the last established commitment level before proceeding with settlement or exit workflows. A loss of service availability must not be interpreted as a successful state transition.</p>
    <h2>Integration review</h2>
    <ul><li>Pin network, protocol version, sequencer authorization, and settlement domain.</li><li>Reject noncanonical encodings, unexpected response fields, and evidence bound to another activity or batch.</li><li>Make retries idempotent and preserve the original activity identifier.</li><li>Distinguish transport success, verified execution, checkpoint attestation, and settlement.</li><li>Inspect deployed role assignments, thresholds, delays, and limits as part of the application&apos;s risk policy.</li></ul>
    <p>Continue with <Link href="/guarantors">Guarantors</Link>, <Link href="/governance">Governance</Link>, and <Link href="/finality">Finality</Link>.</p>
  </DocsLayout>
}
