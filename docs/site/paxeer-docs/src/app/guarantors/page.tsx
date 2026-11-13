import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/guarantors')

export default function Guarantors() {
  return <DocsLayout>
    <h1>Guarantors</h1>
    <p className="page-description">Independent batch replay, bonded attestations, checkpoint certificates, and settlement accountability.</p>
    <p>Guarantors connect Layer X execution to Paxeer settlement. Each guarantor retrieves sealed batch data, independently executes it, compares the committed results, and signs an attestation only after verification. The certificate aggregates the required eligible attestations into evidence that the settlement registry can check.</p>
    <h2>Replay before signing</h2>
    <ol><li>Pin the network, sequencer identity and key, and authorized batch range.</li><li>Retrieve the signed candidate header and its availability chunks.</li><li>Verify request correlation, chunk indices, hashes, Merkle proofs, the terminating response, and the reconstructed availability root.</li><li>Load the verified genesis and registration state, validate actor signatures and authority, and replay the kernel and Programs runtime.</li><li>Compare receipts, state diffs, recovery metadata, and committed roots before retaining or signing evidence.</li></ol>
    <p>The producer verifies receipt signatures with the sequencer public key; it does not need the sequencer private key. Deterministic program calls, program-owned transfers, and storage-occupancy charges participate in the same replay. Replay failure stops attestation production.</p>
    <h3>Data availability is part of verification</h3>
    <p>Candidate retrieval is bounded, and reconstruction must reproduce the signed header's availability root before signing. A missing terminator, unavailable data, or a root mismatch is a refusal to sign. Specific chunk-count, chunk-size, and availability-class limits were not verified against a live producer in the monorepo (the former C/Python guarantor paths are retired) and are not specified here.</p>
    <h2>Attestations and certificates</h2>
    <p>Guarantors exchange canonical attestations through authenticated peer connections. An incoming attestation must pass signature and recovery verification, bonded-membership checks, and checkpoint-identifier recomputation from a locally verified header. The certificate assembler sorts guarantor identifiers and requires the configured threshold.</p>
    <table><thead><tr><th>Evidence</th><th>Meaning</th></tr></thead><tbody><tr><td>Signed batch header</td><td>Sequencer commitment to batch ordering and roots.</td></tr><tr><td>Guarantor attestation</td><td>A bonded signer&apos;s statement about a verified checkpoint.</td></tr><tr><td>Checkpoint certificate</td><td>Canonical aggregation of the required eligible attestations.</td></tr><tr><td>Settlement registration</td><td>The registry accepts the certificate under the deployed domain and contract rules.</td></tr></tbody></table>
    <p>Checkpoint-certificate and guarantor-attestation signatures use distinct domain separators: <code>LXP/v2/checkpoint-certificate</code> and <code>LXP/v2/guarantor-attestation</code>, each followed by a null byte. Certificates carry an empty validity-proof field. An attestation is bonded re-execution evidence, not a zero-knowledge validity proof.</p>
    <h2>Bonded membership</h2>
    <p>The live implementation is the Go Cosmos module <code>modules/layerxanchor</code>, not a GuarantorBond contract; no <code>GuarantorBond.sol</code> exists in the monorepo. The module records guarantors with a signer and bond, and eligibility is narrow: a guarantor is eligible when its status is active and its bond is at least the module's <code>MinBond</code> parameter (default <code>1000000</code> in the bond denomination). Depositing tokens alone does not create membership authority.</p>
    <p>Older descriptions of a minimum bond computed as custodied value times a basis-point requirement, epoch/jail/ejection/unresolved-slashing eligibility checks, and a minimum one-day unbonding delay describe the retired contract narrative and were not found in the live module. Unbonding delay, equivocation/slashing paths, and peer-transport (mTLS) details were not verified in a live replacement producer and are not specified here.</p>
    <h2>Settlement confirmation</h2>
    <p>The submitter validates the deployed chain and registry binding, encodes the certificate, and submits a settlement transaction. Registration is confirmed against successful receipt status, transaction identity, the canonical block, and every checkpoint-registration event field. An existing registration is accepted only when it records the exact certificate.</p>
    <p>The node independently verifies the finality bundle returned to it. The producer then compares the node&apos;s stored finality evidence with the submitted bundle. These checks connect locally verified execution, the registered certificate, and the evidence applications subsequently consume.</p>
    <h2>Operator responsibilities</h2>
    <p>Maintain a distinct signing identity, exclusive persistent-state ownership, durable availability data, and retained attestations. Reuse stored attestations for a previously signed batch rather than generating replacement signatures. Peer-transport details were not verified in a live producer and are not specified here. Organizational and infrastructure independence require separate operator arrangements; two different keys alone do not establish that independence.</p>
    <p>Replay supports the direct-owner authority path and explicitly refuses unsupported oracle inputs or incomplete rotated-signer history. Treat these refusals as verification boundaries. A guarantor must obtain sufficient replay evidence before attesting.</p>
    <h2>Continue</h2><p>See <Link href="/finality">Finality</Link>, <Link href="/governance">Governance</Link>, and <Link href="/security">Security and trust boundaries</Link>.</p>
  </DocsLayout>
}
