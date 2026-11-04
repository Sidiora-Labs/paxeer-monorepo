<a id="launchpad-web-app"></a>
# Launchpad web app

The web consumes the frozen [API contract](services.md#launchpad-api) and
[discovery workflows](services.md#launchpad-discovery). Account and wallet
authority come from the authenticated session. Clear private state when account
identity changes. Show validation, forbidden, conflict and unavailable states
from the frozen error envelope; a 409 requires a fresh resource read and explicit
user intent before retrying a mutation. Preserve revision strings exactly.

Squad actions use the recipient membership revision, including invitation
replacement. Squad revision and member revision are independent. Use private
recipient inbox delivery for invitation authorization, never a public event or
public squad field. Unicode text limits match the service's 500-code-point
description boundary. Alert creation displays the threshold denomination and
source returned by the saved rule, including exact ratio and provenance.
Account deletion must clear public attribution and respect removal fences even
when an older event arrives after reconnect. Client generation and registry
publication belong to Domain 4's web/SDK owners as recorded in the
[wiring requests](../repair_m983.kvx).

<a id="launchpad-sdk-sdk"></a>
# Launchpad SDK

The SDK validates frozen request and response schemas and consumes the
[typed discovery adapters](services.md#launchpad-discovery). The served alert
adapter returns `v2.AlertCreateResponse`; the membership adapter receives both
Membership and Squad. Keep prices as rational integer strings with their asset
and source context. Keep squad/member revisions as independent decimal strings,
including values above JavaScript's safe integer range.

Request joins with the applicant's expected membership revision; approval
returns the committed membership. Invitation replacements use the same revision
fence as other member actions. Recovery reads current persisted state before
another mutation; never fabricate a new revision or reuse an expired invitation.
Handle authentication, conflict and dependency errors from the
[API contract](services.md#launchpad-api). Publishing generated SDK artifacts and
retargeting canonical contract links require the owning domain's wiring change.
