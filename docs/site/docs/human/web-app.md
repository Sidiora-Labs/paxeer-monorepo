# Human web application

The Human web application is the Next.js package `@layerx/human-web` at
`apps/web`. It uses the local `@layerx/ui` package and the Human API contract
under `services/human/schema/human-api`. Its authenticated application and
public explorer present the same merged Paxeer network, EVM chain ID `125`.

See [Journeys](journeys.md) for movement and recovery behavior,
[Identity and authority](../concepts/identity.md) for wallet ownership, and
[Hosted Human](hosted.md) for service and KMS configuration.

## Planes in this tree

These routes have page modules under `apps/web/src/app/`:

| Plane | Routes |
| --- | --- |
| Application | `/app`, `/app/move`, `/app/activity`, `/app/activity/[entryId]` |
| Managed agents | `/app/agents`, `/app/agents/new`, `/app/agents/[agentId]` |
| Approvals and notifications | `/app/approvals`, `/app/approvals/[approvalId]`, `/app/notifications` |
| Settings | `/app/settings`, `/app/settings/wallet`, `/app/settings/security`, `/app/settings/devices` |
| Security and support | `/app/security/devices`, `/app/security/recovery`, `/app/support` |
| Public explorer | `/explorer`, `/explorer/accounts/[accountId]`, `/explorer/programs/[programId]`, `/explorer/receipts/[receiptId]` |
| Market workspaces | `/exchange`, `/bridge`, `/launchpad` |

Bracketed names are Next.js dynamic segments. These are source routes, not a
claim that any particular public hostname serves the application. Market
workspaces use connected-wallet operations and their own availability checks.

The former deposit, withdrawal, journey-claim and emergency-exit pages are
retired. Batch, checkpoint and standalone verify pages are also removed from
the explorer. Historical journey records remain available through the Human
API and activity history; their existence does not restore an initiation page.

## Review, approval and signing

The service-backed movement flow requests a quote, shows the recipient,
amount, fee estimate and ceiling, then commits the reviewed operation. Managed
signing uses the custody EVM authorization and attestor path, which binds the
principal and wallet to the exact transaction and retains its nonce and signed
bytes. Passkey login alone does not grant an unrestricted right to spend.

The browser receives no managed private keys. For connected-wallet market
operations, the app uses the wallet and SDK to prepare precompile calls,
request approval and submit them. Gas-station sponsorship uses a quoted batch
and wallet authorization. The old rule that every browser action must compile
a canonical activity through `layerx-intents` is retired; the current EVM review
and signing path is implemented in `apps/web/src/api/wallet.ts` and the Human
service's `src/custody/evm.rs`.

## Honest outcomes

For Human journeys, Done requires successful `Committed` evidence for all
required stages. `Submitted` and `Receipted` are progress states. The embedded
kernel proposes and executes within the same BFT network; completion does not
depend on a second settlement proof.

Connected-wallet sends distinguish submission from a confirmed outcome. The
market `SendOutcome` component shows a transaction hash without a receipt as
awaiting confirmation. A successful matching receipt supports Sent; a reverted
receipt is a failure. Missing confirmation remains unconfirmed, with the
original transaction hash available for lookup before another send.

Unknown journey outcomes remain `still-checking`, with duplicate-capable
controls disabled. Evidence references let activity and receipt details explain
what happened. A refusal must state whether money left the account. User-facing
copy should describe the action and outcome plainly; technical identifiers and
proof details belong in detail views and the explorer.

## Hosted wiring

`apps/web/next.config.mjs` configures standalone Next.js output and the
`/human-ui` asset prefix. With `LAYERX_HUMAN_SERVICE_URL` set, it rewrites
`/human/v1/:path*` and `/v1/:path*` to the configured Human service's `/v1/`
operations and `/readyz` to Human readiness. The value must be an HTTPS URL
without credentials, query or fragment, with a root or `/human` path. Without
it, those Next.js rewrites are absent.

An operator must supply the matching API, sessions, signing providers and
gateway dependencies. Page source or a built web bundle does not establish
their readiness. Public node RPC and operator-hosted `px_*` methods have
different contracts; see [Gateway JSON-RPC](../platform/gateway-rpc.md).

## Status

The checked-in pages and clients implement the surfaces listed above. Specific
journeys can still be unavailable or retired in the configured service; the UI
must preserve that response rather than render a completed setup or transfer.
Visual, usability and performance certification are separate from these source
and API contracts.
