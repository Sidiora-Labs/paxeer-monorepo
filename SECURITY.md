# Security Policy

Paxeer X Network handles authorization, accounting, custody, bridging and settlement of real value.

## Report privately

Report suspected security vulnerabilities privately before disclosing them publicly.

- Email: <security@paxeer.app>
- Do not open public GitHub issues or discussions for sensitive reports.
- Include a clear description, steps to reproduce, and potential impact.

## Handling and disclosure

- Receipt acknowledgement: Best-effort response.
- Initial triage: Best-effort response.
- Status updates: Best-effort updates provided every two weeks while remediation
  is in progress.
- Resolution & coordinated disclosure: Best-effort remediation targets depend on
  severity. We follow responsible disclosure principles and request that you
  coordinate public announcements with us until a fix is released.
- Escalation path: If a response target is missed, you may escalate by emailing
  <security@paxeer.app> with the reference to your original advisory or report.

## Bug bounty

The [Paxeer bug bounty programme](https://www.paxeer.app/bug-bounty) rewards vulnerabilities in the in-scope code listed below, subject to the out-of-scope exclusions. One reward in USD is paid per unique vulnerability; if reports are duplicates, only the first reporter receives the reward.

- Critical: $10,000-$50,000 — could move or destroy funds, or change what the network considers final
- High: $5,000-$10,000 — a serious security or stability weakness that does not by itself lose funds
- Medium: $1,000-$5,000 — limited impact, or exploitable only under specific conditions
- Low: $100-$1,000 — minor issues and informational findings

Stop testing at a proof of concept. Never move funds that are not your own, and follow the testing restrictions above. Keep the issue private until it is fixed and the advisory is published.

## Scope

In scope is the code in this repository, including:

- the Paxeer X chain node and its modules, precompiles, RPC and storage (`node/`, `modules/`, `precompiles/`, `rpc/`, `consensus/`, `storage/`, `engine/` and the other root Go packages)
- the LayerX kernel, its programs and `layerxctl` (`src/`, `include/`, `cmd/`, `programs/`)
- the vendored dependency trees the node builds from (`core/deps/`)
- the Solidity contracts (`contracts/`, `contracts/bridge/`, `contracts/launchpad/`)
- off-chain services, SDKs and applications (`services/`, `sdk/`, `apps/`)

The root directories of the chain node and the kernel move under `core/` in a later release.

Out of scope:

- third-party services and integrations
- issues already known or reported
- social engineering
- defects that need a compromised validator, governance or operator key, unless the code fails to enforce a limit it claims to enforce
- denial of service by sheer traffic volume against hosted endpoints
- findings from automated scanners without a demonstrated impact
- defects in an upstream project that do not affect how this repository uses it; report those upstream

## Supported versions

Security fixes are provided for the `main` branch and the release recorded in
`tools/release/version.json`. Fixes for reported vulnerabilities are shipped in
the next scheduled release.

A conduct concern that is not a vulnerability follows [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).