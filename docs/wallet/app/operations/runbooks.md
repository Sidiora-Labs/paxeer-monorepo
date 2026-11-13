# Operational recovery runbooks

## Upstream outage

Confirm readiness dependency status without exposing origin details, identify the failing fixed-origin proxy, and compare bounded error codes and latency. Keep signing and submission actions disabled when required truth is unavailable. Restore the dependency or route through an approved fixed origin, then verify a real response with content type, size, freshness, and chain context before clearing the incident.

## Corrupt client storage

Capture the public storage failure code and correlation reference. Optional application records are removed automatically and reported in the in-app status. Custody choice and swap storage corruption fails closed. Reconcile retained swap intents before resetting their storage. Use the exact registry lifecycle reset for recovery and preserve the embedded wallet's authentication state. See the [storage registry reference](../architecture/storage-forensics.md).

## Push-store failure

Check readiness and the durable store path, lock-file access, bounded file size, current envelope version, backup, and any quarantined corrupt file. Restore from the atomic backup only after validating every subscription or campaign through its parser. Do not print endpoints, keys, wallet addresses, or payloads. Re-run a consented subscription and revocation flow after recovery.

## Bad release

Select a compatible release. Confirm CSP nonces, service-worker activation, storage schema compatibility, route availability, account/session invalidation, and one real read-only portfolio flow before reopening actions. Do not force a service-worker skip-waiting transition during an approval or signing operation.

## Transaction reconciliation

Use the submitted hash, selected chain, account, nonce, and durable operation state; do not infer success from a local send response. Query an approved RPC and indexer until the bounded reconciliation deadline, distinguish pending, replaced, dropped, reverted, and confirmed states, and show stale or unavailable data explicitly. Never resubmit without a new user approval.

## Conflicting chain observations

Paxeer receipts are final once their block is committed. Retain the transaction hash and block observations when RPC and indexer responses disagree. Recheck the selected chain, committed block and execution result through a current node, and mark derived portfolio or activity data stale while reconciling it. Preserve the signed intent and require a new approval for another payment.

