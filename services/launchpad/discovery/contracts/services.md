<a id="launchpad-discovery"></a>
# Launchpad discovery

This maintained service contract describes the current [HTTP handler](../http.go),
[SQL store](../store.go), [squad operations](../squads.go), and
[public source authority](../wire.go). Domain 4 owns publication of these pages
into the canonical registry; the outstanding changes are in the
[repair record](../repair_m983.kvx). The accompanying
[client contract](clients.md#launchpad-sdk-sdk) describes the consuming SDK.

## Squads and revisions

`POST /v2/squads` validates the frozen `SquadCreateBody`: names contain 1–80
Unicode code points, descriptions contain at most 500, visibility is `public`
or `private`, and join policy is `open`, `request`, or `invite`. Storage enforces
the same boundaries. Connected creates at `/v2/discovery/squads` default to
private; connected patches preserve description and visibility.

The revision-checked public Go operations are:

```go
func (s Store) Invite(ctx context.Context, actor, squadID, account, token string, expected uint64, expires time.Time) error
func (s Store) RequestJoin(ctx context.Context, actor, squadID string, expected uint64) (Membership, error)
func (s Store) ApproveRequest(ctx context.Context, actor, squadID, account string, expected uint64, now time.Time) (Membership, error)
```

`expected` identifies the recipient/applicant membership revision. Zero denotes
no prior membership. Each operation compares it and writes under the squad's
transaction lock. Replacing an invitation consumes a membership revision; two
invitations at the same expected revision yield one success and one conflict.
Invitation tokens are durable, private to the recipient, expire, and are redeemed
once. A rejected invitation must preserve the winning token. The connected
invitation body carries `Expected`; frozen membership actions carry decimal
string `expectedRevision`.

For `request` policy, RequestJoin produces a requested membership; an authorized
owner or moderator calls ApproveRequest, which returns the committed active
membership with `JoinedAt = now`. `open` joins become active immediately.
Invite-only policy rejects request actions; banned members cannot rejoin.
`PUT /v2/squads/{id}/members/{account}` calls ChangeMembership, which returns
both the squad and member under the mutation lock. AdaptV2SquadMembership takes
both objects. Envelope `revision` and membership `revision` identify the member;
`data.squadRevision` identifies the independent persisted squad. All frozen
revisions are decimal strings and preserve the full uint64 range across restart.

## Alerts and persisted thresholds

The [alert adapter](../alerts_v2.go) exposes the actual typed producer signature:

```go
func AdaptV2AlertCreate(rule AlertRule, wallet string, threshold v2.InputPrice, requestID string, generatedAt time.Time) (v2.AlertCreateResponse, error)
func RuleFromV2AlertCreate(body []byte, actor string) (AlertRule, error)
```

`POST /v2/me/alerts` resolves authenticated account and wallet, validates the
frozen body through RuleFromV2AlertCreate, and calls SaveRule before adaptation.
`rule.Price` persists the complete threshold, including numerator, denominator,
asset identities, decimals, chain and source provenance. The adapter's threshold
must match that persisted value. An equal ratio with different provenance is
invalid. A reopened store can reconstruct the same typed response from the
saved rule; no client-side threshold reconstruction is authoritative.
Evaluation requires a matching canonical price identity and durable checkpoint.
Missing price context is held explicitly. Disable is owner scoped.

## Deletion, delivery and recovery

DeleteAccount locks the callout author and atomically writes anonymous removal
events, anonymizes queued public Actor fields and current/history attribution,
scrubs deleted content, clears snapshots, and disables owned alert rules.
The public `CalloutTargetEvent` wire fields are `TargetKind`, `TargetID`, `Actor`,
`Revision`, `Removed`, and `CreatedAt`. Account-deletion removals have `Actor = ""`.
The notification worker's discovery Runtime checks the persisted callout before
publishing: after account deletion, even an old queued creation is delivered as
an anonymous removal at the current revision. This deletion fence prevents stale
events from restoring a public identity. Other authors retain their attribution.
Delivery failure leaves the row unpublished; reopening the store retries it.
Published rows are acknowledged transactionally, with event IDs for downstream
deduplication. The source resolver refuses tombstoned callouts.

The downstream social authority must accept anonymous removals without resolving
Actor as an account UUID, and persist a monotonic removal fence before allowing
any older creation. That consumer, its policy store, the registry, and production
PostgreSQL privilege/lock qualification belong to their owners outside this task.
See the explicit wiring requests in the [repair record](../repair_m983.kvx).

<a id="launchpad-api"></a>
# Launchpad API

The API mounts discovery's [frozen routes](../http.go). Authenticated operations
derive the account from the injected authority and alert routes also require the
wallet resolver. Missing authority fails closed. Responses use frozen schemas,
request correlation, and `Cache-Control: private, no-store`. Validation errors
answer 400, missing resources 404, authorization failures 403, stale revisions
409, and unavailable dependencies 503; missing authentication answers 401.
Client consumers must preserve decimal revisions and handle errors through the
frozen envelope. The [web contract](clients.md#launchpad-web-app) documents retry
and identity boundaries. Discovery owns durable state and event recovery; API
mounts and regenerated schema/client artifacts require Domain 4 owner wiring.
