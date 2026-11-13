# Launchpad clients

<a id="launchpad-web-app"></a>

## Launchpad web app

The web consumes the frozen [API contract](services.md#launchpad-api) through
[session-bound operations](../../../../apps/launchpad/web/src/core/market/operations.ts).
Authenticated requests read the bearer from the current account session.

The [alert panel](../../../../apps/launchpad/web/src/widgets/terminal/status-bar/alerts-panel.tsx)
creates and deletes alerts through `IF_ALERT_CREATE` and `IF_ALERT_DELETE`.
It retains the returned alert objects in browser memory and filters them by
the signed-in wallet and selected pool. Each row displays the saved threshold's
numerator/denominator ratio with a USDL label and the returned alert state.
The list contains alerts created during the current visit.

The [squad rankings panel](../../../../apps/launchpad/web/src/widgets/leaderboard/squads-tab.tsx)
displays an unavailable state. The SDK's squad operations cover creation,
reading and membership changes.

<a id="launchpad-sdk-sdk"></a>

## Launchpad SDK

The SDK's [Operations](../../../../sdk/launchpad/packages/sdk/src/operations.ts)
validates generated request schemas and decodes responses through the
[transport](../../../../sdk/launchpad/packages/sdk/src/transport.ts).
`IF_SQUAD_CREATE`, `IF_SQUAD_READ` and `IF_SQUAD_MEMBERSHIP` use the frozen
discovery routes. Membership mutations carry `expectedRevision`; the response
contains the member's revision and the independent `data.squadRevision`.
The generated types preserve revisions and price numerator/denominator values
as strings, with prices retaining their asset and source context.

The [discovery adapters](services.md#launchpad-discovery) produce
`v2.AlertCreateResponse` for alerts and take both `Membership` and `Squad`
for membership responses. Authentication, conflict and dependency errors use
the [API contract](services.md#launchpad-api). The transport reports HTTP
errors with their status and decoded envelope; mutations are not retried
automatically on a conflict.
