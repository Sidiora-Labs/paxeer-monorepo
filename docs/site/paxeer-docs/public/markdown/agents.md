<!-- Source: https://docs.paxeer.app/agents/ -->

# Agents on Paxeer X

Give autonomous software a verifiable identity, explicit authority, bounded spending, and an evidence trail for every network action.

Paxeer X agents submit signed EIP-1559 transactions to native precompiles on chain 125. The native kernel executes inside the BFT node. An agent reads state, prepares a transaction, reviews its effects, obtains a signature, and tracks the resulting evidence. The agent daemon supplies tenant isolation, session controls, policy, and runtime integration.

## Choose an agent integration

| Integration | Use it for | Authority path |
| --- | --- | --- |
| Agent API | A custom service that needs preparation, signing, submission, reads, approvals, and event subscriptions. | Tenant-bound daemon session and explicit capability. |
| MCP | Expose network tools to an assistant or development runtime. | Daemon binding, session, and capability; tools use the Agent API. |
| A2A | Submitting a caller-signed transaction and reading receipts from another agent. | Hosted gateway key; the caller signs, the runtime never signs. |
| Native programs | Deterministic application logic called by an agent. | Signed program activities, execution receipts, and program registry evidence. |

Start with [Agent runtime](https://docs.paxeer.app/agent-runtime) for daemon enrolment and runtime installation, or [Native programs](https://docs.paxeer.app/programs) for application execution.

## Identity and registration

An agent DID identifies the actor. An authority reference identifies the grant under which it acts. A tenant scopes records, credentials, and policy. These are separate from the runtime client name and from the session used to authenticate a request. Registration binds the agent to a verified owner; a self-declared DID alone does not confer permission.

| Operation | Required context | Purpose |
| --- | --- | --- |
| `agent.register` | tenant, agent_did, authority_ref, client, policy_version, owner_address, binding_consent, fee_limit | Register the agent under its verified authority. Returns `AuthorityResponse<Prepared>`. |
| `session.open` | context, grant_id, registration_payload, expiry_sequence, purpose, fee_limit | Open a tenant-bound authenticated session. Returns `AuthorityResponse<Prepared>`. |
| `session.refresh` | session_id, context | Refresh the session within its authority and policy. |
| `session.close` | session_id, context | Close access and invalidate session use. |
| `session.list` | context | Inspect sessions visible to the authenticated owner. |

For `agent.register`, `owner_address` is the 40-hex EVM address that signs the transaction and `binding_consent` is the 128-hex owner-signed Ed25519 consent, verified against the chain ID and current bind nonce. For `session.open`, `grant_id` and `purpose` are 64-hex values, `registration_payload` is the canonical session grant matching the grant ID, and `expiry_sequence` is a nonzero decimal. The session context itself includes tenant, agent DID, authority reference, permitted activity types, expiry, client, and policy version.

## Define what an agent may do

A capability names every permission dimension explicitly. Missing dimensions are invalid; an empty set denies that dimension. Use `capability.create` to issue a bounded capability, `capability.attenuate` to derive a narrower child, `capability.list` to inspect it, and `capability.revoke` to withdraw access. Create, attenuate, and revoke each require an `idempotency_key`; a keyless create is refused before the owner is called. Revoking a capability revokes its whole parent subtree in one durable write, and unsubmitted preparations bound to a revoked record end in `Failed` with a capability-revocation reason.

| Dimension | Decision it controls |
| --- | --- |
| activity_types | Which native operations the agent can prepare. |
| counterparties | Which counterparties may participate. |
| assets | Which assets the agent may use. |
| amount_ceilings | Maximum authorized quantities per asset. |
| rate_ceilings | Action counts within explicit time windows. |
| purpose_constraints | The permitted purpose commitments. |
| expiry | The last permitted authorization period. |

Attenuation never widens authority: child sets are subsets, amount ceilings do not increase, expiry does not move later, and rate windows remain at least as restrictive. Capabilities restrict an existing protocol grant; they do not manufacture a new protocol authority.

## The action lifecycle

1. **Read.** Obtain account sequence, balance, and relevant module state.
2. **Prepare.** Supply `activity_type`, `actor`, `authority`, `account_sequence`, `not_before`, `not_after`, `idempotency_key`, `fee_limit`, `payload`, and `payload_hash` to the daemon wire API.
3. **Review.** Inspect the disclosure decoded from the unsigned canonical bytes: actor, authority, activity type, counterparties, amounts, asset, fees, expiry, and idempotency key.
4. **Sign.** Sign the returned transaction preimage with secp256k1 and bind the 65-byte signature to the preparation reference. The wire signature is 130 lowercase hexadecimal characters.
5. **Submit.** Submit that preparation, signature, and `signer_public_key`, retaining the submission reference.
6. **Track.** Use `track` or `wait` to observe evidence and the reached rung.

```
prepare → Prepared { preparation_ref, unsigned_canonical_bytes, signing_preimage, disclosure, expiry }
sign(preparation_ref, signature) → Signed
submit(preparation_ref, signature, signer_public_key) → TrackedSubmission { state, evidence, rung, transitions }
track(submission_ref) → state + evidence + rung
wait(submission_ref, requested_rung, deadline) → actual_rung
```

Submissions move through Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed, or Expired. Unknown is a pending outcome that requires reconciliation. Executed and Committed carry a receipt reference. Preserve the original idempotency key while resolving an uncertain submission.

## Budgets and human approval

Budget controls distinguish protocol-enforced budgets from daemon-enforced limits. A daemon limit reserves capacity before an activity proceeds; an unresolved outcome retains its hold until evidence resolves the result. The daemon consumes protocol evidence; it does not mint balances from local accounting. See [agent budgets](https://docs.paxeer.app/agent-budgets) for the native budget activities and funding rules.

Human approval binds an exact prepared activity and its canonical disclosure digest. Approval releases that preparation; rejection releases its reservation; expiry never implies approval. A changed disclosure is defective and requires a new review. Approval is a daemon restriction and does not enlarge protocol authority.

## Read, observe, and verify

Read operations cover balances, accounts, module state, history, batches, checkpoints, and proof bundles. A batch identifies a block height; a checkpoint contains a CometBFT commit and validator set. Portable proof bundles bind the raw transaction, signed execution receipt, EVM receipt inclusion, and CometBFT commit. Availability fetch and offline export support evidence portability. Projection returns an estimate and is distinct from a verified read.

Evidence is reported as a rung: `submitted`, `receipted`, or `committed`. Submitted is an acknowledgement, receipted verifies execution, and committed verifies inclusion in a committed block. Request the rung the decision needs with `wait`; when the deadline elapses it returns the rung actually reached rather than an error.

Subscriptions are scoped by tenant, agent, and capability. Filters narrow agents, accounts, activity types, modules, assets, counterparties, and result classes. Delivery is at least once; deduplicate using `deduplication_id`, acknowledge cursors, and handle Gap and Truncated events explicitly.

## Design a reliable agent

- Use a dedicated identity and the narrowest capability for each workload.

- Require review for actions outside a routine policy; show canonical effects to the approver.

- Separate daemon policy records from cryptographically verified network receipts.

- Retain preparation, submission, activity, and receipt references across process restarts.

- Treat transport loss and deadlines as uncertainty, then reconcile before authorizing a replacement action.

**Reference:** `services/agent/schema/agent-api/identity.kvx`, `write.kvx`, `stream.kvx`, `read.kvx`, `errors.kvx`; `services/agent/crates/layerx-agentd/src/session.rs`, `capability/`, `approval/`, `budget/`.
