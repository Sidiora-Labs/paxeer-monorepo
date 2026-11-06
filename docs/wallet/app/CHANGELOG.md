# Changelog

This changelog covers the wallet app at `apps/wallet` in the Paxeer monorepo.
The current [package manifest](../../../apps/wallet/package.json) names it `@paxeer/wallet-app`, version
`0.1.0`; that package version alone does not establish a published release date.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
with versions described using [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
See the current [wallet history](https://github.com/Sidiora-Labs/paxeer-monorepo/commits/main/apps/wallet)
and [repository releases](https://github.com/Sidiora-Labs/paxeer-monorepo/releases).

## Unreleased

### Changed

- Rewrote the wallet [bug report](../../../apps/wallet/.github/ISSUE_TEMPLATE/bug_report.md),
  [feature request](../../../apps/wallet/.github/ISSUE_TEMPLATE/feature_request.md),
  [question](../../../apps/wallet/.github/ISSUE_TEMPLATE/question.md), and
  [pull request checklist](../../../apps/wallet/.github/PULL_REQUEST_TEMPLATE.md) for manual use in
  the monorepo. GitHub does not automatically discover these nested templates.
- Separated imported wallet notes from current monorepo release records and
  removed the unsupported release date and old repository release links.

## Network context

The wallet targets Paxeer's merged network. Mainnet-beta has EVM chain id `125`
(`0x7d`). Public JSON-RPC uses node root URLs, such as
`https://api-mainnet-beta.paxeer.network`, which is also the
[wallet SDK's default endpoint](../../../sdk/wallet/src/endpoint.ts).
The app's default RPC URL comes from `NEXT_PUBLIC_PAXEER_RPC_URL` in
[wallet configuration](../../../apps/wallet/src/lib/constants.ts); the browser app is mounted at
`/wallet/` by [Next.js configuration](../../../apps/wallet/next.config.mjs).

Sub-chains are tenant domains within the same network. The wallet's
[network selection code](../../../apps/wallet/src/lib/networks.ts) distinguishes the root network
from virtual tenant selections. The imported multi-chain label below is
historical terminology, not a claim of independent execution and settlement networks.

## Imported history (undated)

These notes were imported from the standalone Paxport Wallet project. They are
retained as historical context, not as verified monorepo release records. The
original notes had an Unreleased section and a `1.0.0` heading with no verified
release date. That heading does not identify a release of `@paxeer/wallet-app`.

### Previously listed as Unreleased

#### Added

- Sentry error tracking and session replay
- Offline detection with NetworkErrorScreen
- Quote expiry countdown and auto-refresh in swap quotes
- Transaction error modals (gas, revert, timeout, ratelimit)
- Rate limiting on API proxy routes
- LocalStorage caching for token metadata with TTL
- Server-side proxy for Sidiora metadata and logo APIs
- Crossverse price and candle data queries

#### Changed

- Fixed React hook usage errors in PortfolioWidget
- Improved swap modal styling and error handling
- Enhanced portfolio data fetching and filtering

#### Fixed

- React error #310 on portfolio page load
- ORB errors on token logos by proxying through same-origin route
- Disabled query loading state causing perpetual spinner

#### Security

- Rate limiting for API proxy requests and improved input validation were recorded
  in the imported notes. Current enforcement is route-specific; this is not a
  guarantee that every API route is rate limited. See the
  [limiter implementation](../../../apps/wallet/src/lib/rateLimit.ts),
  [logo proxy](../../../apps/wallet/src/app/api/sidiora/logo/[...path]/route.ts), and
  [JSON proxy validation](../../../apps/wallet/src/server/json-proxy.ts).

### Previously labelled 1.0.0

The imported initial-release feature list included:

- Multi-chain support (the imported label; see [network context](#network-context))
- Swap functionality
- Portfolio tracking
- Token discovery
- Send/receive transactions
- Onboarding flow
- Embedded wallet support

For current setup and supported workflows, use the [wallet README](../../../apps/wallet/README.md)
and the linked source files rather than treating imported notes as release certification.
