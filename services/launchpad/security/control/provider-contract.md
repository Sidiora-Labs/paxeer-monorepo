# Control provider lifecycle

The owning control implementation is [providers.go](providers.go), with durable
claims in [scheduling.go](scheduling.go), execution in [replay.go](replay.go) and
public request/error types in [http_models.go](http_models.go). These links
replace no foreign registry row; the domain-17 registry owner must retarget its
existing row identities as recorded in [repair_m1020.kvx](repair_m1020.kvx).

All provider clients require HTTPS, verified TLS 1.2 or later, an explicit
32–4096 byte bearer credential and a supplied transport. Redirects and proxy
inheritance are disabled. The total request timeout is three seconds; response
headers and TLS handshakes have two-second limits. Responses have a 64 KiB limit
and reject unknown/duplicate fields and trailing JSON. Provider failure is
`authority_unavailable`, never success inferred from local dispatch intent.

## Payout and identity revocation

Payout GET and PUT use `/v1/settlements/{obligation-key}`, bearer authentication
and `Idempotency-Key: {obligation-key}`. Only an authoritative GET 404 permits a
PUT with `idempotencyKey`, `asset`, `destination`, `amountRaw` and
`authorityReference`. Other successful responses expose `state` and `reference`;
accepted states are pending, unknown and confirmed. A confirmed response needs a
32-byte transaction hash. Independent `ChainVerifier` evidence is required to
settle; provider acceptance alone releases no liability. Unknown results retry
with the original key. The provider owner must retain key/effect identity and
answer authoritative lookups after restart.

Revocation PUT uses `/v1/identity-revocations/{incident}` with `incident` and
`identity`, the incident as idempotency key, and bearer authentication. A 200
acknowledgement must bind both identities and supply `rotation`, `integrity`,
`secretsRevoked=true` and `cachesInvalidated=true`. Local credential revocation
and external cache/secret acknowledgement are separate durable stages.
Completed outboxes are excluded from scheduling. Recovery consumes exact
independently retained references and current dual approval. It updates the
quarantine marker; runtime roles need no DELETE privilege. Recovered history
cannot clear a newer quarantine or restore an old revoked credential.

## Projection replay handoff

`ReplayProvider` is an optional owner-adapter binding, not the indexer's
existing internal `ReplayRunner` API. Without the configured binding,
`TransitionWorkflow` leaves an approved replay approved and returns
`authority_unavailable`. The gateway currently supplies no such binding.

The handoff is PUT `/v1/projection-replays/{completionIdentity}`. The bearer
credential and `Idempotency-Key` accompany a `ReplayExecution` containing
`jobId`, `job` and the exact approved `payload`. The job stores `actionId`,
`actionKey`, `kind`, `scope`, `state`, `payloadHash`, `payloadReference`,
`completionIdentity`, `claim` and `until`. The payload reference is the unique
proposal key; its digest must equal the approval's hash. Completion identity is
`control-replay:{jobId}` and is stable across every transport retry.

A 200 terminal `ReplayResult` binds `ActionID`, `JobID`, `PayloadHash`,
`PayloadReference`, `CompletionIdentity`, `State` (succeeded or failed) and a
nonempty `Reference` to the owner's durable checkpoint. An unavailable or
nonterminal owner returns a non-200 response. The owner must atomically persist
its idempotent inbox, projection effects/checkpoint and completion, returning
the same terminal identity on retry. A response merely echoing the request
without doing this does not implement the contract.

`ReplayWorker` persists a claim and retry schedule before dispatch, retains
validated owner completion as `executor_result`, then invokes
`CompleteAction`/`DurableJobResults`. Recovery after lost acknowledgement uses
the same job and payload; recovery after retained completion consumes that
completion without another external effect. Approval/quarantine authority is
rechecked before the final mutation. An expired or revoked approval cannot
complete an action. Durable scheduling uses batches of at most 100 and
per-key exponential backoff of 30 seconds through five minutes.

The actual indexer request additionally requires source stream, target,
generation, projection revision, authorization ID, mode and canonical
checkpoint height/hash. Those bindings must be approved and implemented by the
indexer owner before production admission. Control does not invent them from
a projection name or claim that its loopback transcript qualifies indexer
execution. See the explicit wiring request in the repair record.

## Owning documentation

Current chain contract authority is described by
[the launchpad contract overview](../../../../contracts/launchpad/README.md) and
[its operations documentation](../../../../contracts/launchpad/operations/README.md).
Client wire shapes are described by the current shared documentation and the
control request models linked above; the registry owner must choose exact
existing owning pages for each affected row instead of pointing to missing
generic contracts/services.md or contracts/clients.md pages.
