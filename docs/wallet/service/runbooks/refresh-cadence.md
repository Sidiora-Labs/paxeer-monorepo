# Share refresh cadence

`POST /v1/keys/refresh` replaces each holder's share while preserving the key's
public key, address and DID. The new shares use the next epoch and do not
combine with shares from the previous epoch. The operation requires all current
holders and an operator client certificate.

Refresh at the configured custody cadence, after add-share, and after importing
a key through a ceremony. Isolate a suspected compromised participant and use
[node replacement](node-replacement.md) before including a new holder.

## Preconditions

All five current holders must be reachable and agree on the key's public
identity, owner, participants and epoch. The current signed inventory must
permit `refresh` for that key at its held epoch and approve those five members
and pins. Use authenticated `/health` and `/v1/keys/describe` to inspect them.
An unreachable holder must be recovered or replaced before refreshing.

Use the operator client setup in [node replacement](node-replacement.md).
The request types and durable recovery fields are defined in
[`keys.go`](../../../../services/wallet/attestor/internal/server/keys.go).

## Refresh

List the wallet's `attestor_key_id` and `layerx_key_id` values, including keys
retained by provisioning operations. For an ordinary refresh, prepare:

```json
{"session_id":"<refresh-session>","key_id":"<key-id>"}
```

Submit the identical body concurrently to `/v1/keys/refresh` on all holders.
Check every response for the original `public_key`, the same next `epoch`,
`refreshed: true` and an `audit_sequence`. The operation holds a per-key lock;
refresh keys one at a time.

Update the signed inventory to approve the new epoch, then compare key
descriptions on every holder and verify a consented signature. The public
identity and participant set must remain unchanged. See
[inventory checks](../../../../services/wallet/attestor/internal/server/authority.go).

## Durable ceremony recovery

Ceremony refresh requests also bind `ceremony_id`, `import_session_id`,
`expected_epoch` and, for an existing held key, `existing_key`. The daemon
retains prepared, staged and completed ceremony state. Recovery names the
original attempt with `recovery_session_id`; the retained identities and base
epoch must agree. The [ceremony tool](../cutover.md) coordinates these fields.

After a timeout or interrupted response, describe the key on every participant
and inspect its `refresh` and ceremony fields before continuing. Resume the
retained ceremony through its journal. Do not discard the journal, assume all
holders stayed at the old epoch, or start an unrelated refresh while a retained
attempt needs reconciliation.

## Completion

A refresh is complete when all current holders describe the same new epoch and
a signature combines their current shares. Refresh is not reversed by restoring
an old snapshot or restarting a process. Keep the signed inventory and backup
material consistent with the completed epoch.
