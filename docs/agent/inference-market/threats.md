# Inference market threat statement

Only the development attestation envelope is accepted today.

bindPeer and peerBinding are implemented at 0x0907.
See the [attestor implementation](../../../precompiles/layerxagent/teeattestor/teeattestor.go).
Implementation does not establish deployment on a target network; check its
availability and [F11 support](README.md#availability) before use. Development
attestation does not establish hardware isolation, an authenticated production
TEE measurement, model confidentiality or trustworthy execution. Other
attestation families must refuse until explicitly implemented and qualified.

## Identity and execution

The implemented `bindPeer` at `0x0907` verifies an envelope whose report data
equals `keccak256(peerKey, modelDigest, runtimeDigest)` over the three bytes32
values, then records the family, digests and binding height for the peer key.
`peerBinding` reads back those stored values for a peer key. A matching binding
ties those identifiers together; it does not prove the peer ran the specified
model for every token. Signatures and roster checks authenticate
participants and receipt agreement, not the truth of arbitrary output.

## Wrong answers and canaries

Canaries bound but do not eliminate wrong answers. They sample positions and
compare fixed-point logits to a host expectation within a profile tolerance.
Unsampled positions, plausible but false output and errors within tolerance can
escape detection. A wrong host expectation or mismatched model/runtime can
also produce a fault. Correct canary output is not a proof of semantic accuracy
for user answers.

The sampling inputs specified today are deal ID and lap index. A deterministic
derivation using only public inputs is predictable; its cryptographic encoding
and any secret contribution remain unspecified. The pending derivation must
not be presented as an established unpredictability guarantee. Commit/reveal
binds expectations before disclosure but cannot by itself conceal a public
sampling rule or prevent the host from revealing canaries to collaborators.

## Collusion and economic limits

A host and cooperating peers may agree on false receipts, selectively answer
known canaries or withhold countersignatures from an honest peer. The planned
roster-size-minus-one countersign threshold does not make the signers
independent. Enrolment and development attestations do not prove that distinct
keys belong to distinct operators. Host-controlled expectations and sampling
remain trust boundaries; the chain does not rerun model inference.

Bonds and loss of a faulty lap's share impose bounded economic penalties after
admissible evidence and timely challenges. They cannot guarantee honesty when
the benefit of cheating exceeds the bond, evidence is unavailable, parties
collude or nobody challenges before the window closes. Refunds wait for that
window, and a slash is not an automatic refund of every asker loss.

## Liveness and settlement

The host coordinates placement, signalling, receipt sealing and the stream's
single meter authority. Its failure or censorship can stop progress. Peer
dropout, unavailable layers, a lost node connection, exhausted funding or an
unresolved challenge can delay service, settlement or claims. Replanning can
recover only when enough honest capacity and state remain; it cannot ensure
an answer by a deadline. Recovery resumes from a countersigned lap, so local
work beyond that boundary may be recomputed.

A settled cumulative-token meter prevents arbitrary response chunk counts from
being treated as billable progress under the contract. It does not prove answer
quality, prompt privacy or continued service. Receipted writes and off-chain
lap receipts are not committed finality. Check chain evidence before treating
a payout, claim or bond return as completed.

## Transport and privacy

The host and assigned peers process inference data; these documents make no
confidential-computing or end-to-end prompt privacy guarantee. The planned
signalling listener is plain WebSocket for browser interoperability. A signed
hello authenticates its peer but does not encrypt that connection. Operators
must provide appropriate transport protection when exposing listeners beyond
a trusted local environment. Browser peers receive room offers, not host
signing keys or private node credentials. Keep ask bearer credentials, private
keys, full prompts and operational logs out of published room material.
