# Security Policy

Paxeer X Network handles authorization, accounting, custody, bridging and settlement of real value. Treat any defect that can affect value conservation, authority, finality, replay determinism, withdrawal uniqueness, upgrade safety or private key handling as a security issue.

## Report privately

Do not open a public issue, discussion or pull request for a suspected vulnerability. Report it by email to <security@paxeer.app> or through GitHub private vulnerability reporting: open the repository's Security tab and choose "Report a vulnerability", or go directly to <https://github.com/Sidiora-Labs/Paxeer-X-Network/security/advisories/new>. Only the maintainers see the report.

Include:

- the affected commit and component
- prerequisites and an end-to-end reproduction
- the violated invariant and the plausible impact
- the privileges exploitation needs, such as a validator, governance, attestor or operator key
- any proposed fix or test case

Test only against systems and assets you own or have written permission to test. Do not probe public endpoints, validators, bridges or user data, do not move funds, and do not keep secrets you obtain during research.

## Handling and disclosure

The maintainers confirm receipt in the advisory, assess scope and severity, and work on the fix in a private advisory fork when one is needed. The fix lands on `main`, and when it changes the chain node it ships in a release as described in [RELEASE.md](RELEASE.md). The advisory is published once the fix is available, crediting the reporter unless they ask otherwise. A report found to be out of scope is closed with the reason given in the advisory. Please keep details private until the advisory is published.

## Response targets

These response targets are best-effort commitments. The maintainers aim to acknowledge a report within three business days of receipt and complete triage with a scope and severity decision within ten business days of receipt. While remediation is in progress, they provide a status update at least every fourteen days.

Remediation targets, counted from the severity decision, are thirty days for Critical issues, sixty days for High issues, ninety days for Medium issues and the next scheduled release for Low issues. Coordinated disclosure is targeted within fourteen days of the fix being available.

If a target is missed, escalate by commenting in the advisory. After two further business days without a response, email <security@paxeer.app> with the advisory identifier.

## Bug bounty

The [Paxeer bug bounty programme](https://www.paxeer.app/bug-bounty) rewards vulnerabilities in the in-scope code listed below, subject to the out-of-scope exclusions. One reward in USD is paid per unique vulnerability; if reports are duplicates, only the first reporter receives the reward.

### Response SLAs

These targets represent our best effort to provide timely responses. Exceptions
may occur due to holidays, complex investigations, or coordination with
upstream maintainers.

- **Receipt acknowledgement:** Promptly upon report receipt
- **Initial triage & scope/severity decision:** Following receipt
- **Status updates:** Provided periodically while remediation is in progress
- **Resolution & coordinated disclosure:** Target remediation timelines depend on
  severity and project capacity
- **Escalation path:** If a response target is missed, you may escalate by
  emailing <security@paxeer.app> with the reference to your original advisory
  or report for a status review by the security lead.

- Critical: $10,000-$50,000 — could move or destroy funds, or change what the network considers final
- High: $5,000-$10,000 — a serious security or stability weakness that does not by itself lose funds
- Medium: $1,000-$5,000 — limited impact, or exploitable only under specific conditions
- Low: $100-$1,000 — minor issues and informational findings

Stop testing at a proof of concept. Never move funds that are not your own, and follow the testing restrictions above. Keep the issue private until it is fixed and the advisory is published.

## Supported versions

The supported line is `main` and the current release named in `tools/release/version.json`. Fixes land on `main` and ship in the next chain node release.

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

A conduct concern that is not a vulnerability follows [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).