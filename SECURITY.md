# Security Policy

Paxeer X Network handles authorization, accounting, custody, bridging and settlement of real value. Treat any defect that can affect value conservation, authority, finality, replay determinism, withdrawal uniqueness, upgrade safety or private key handling as a security issue.

## Report privately

Report suspected vulnerabilities by email to <security@sidioralabs.com>. A reporter seeking a bounty must submit by email only, with the full report and credentials required under Bug bounty below.

GitHub private vulnerability reporting remains available: open the repository's Security tab and choose "Report a vulnerability", or go directly to <https://github.com/Sidiora-Labs/paxeer-monorepo/security/advisories/new>. Only the maintainers see a private report. Reports made through GitHub, including private vulnerability reporting, or any other public surface (issue, discussion, pull request, advisory or social post) are fixed through the normal maintenance process without attribution or reward. Do not disclose suspected vulnerabilities publicly.

Include:

- the affected commit and component
- prerequisites and an end-to-end reproduction
- the violated invariant and the plausible impact
- the privileges exploitation needs, such as a validator, governance, attestor or operator key
- any proposed fix or test case

Test only against systems and assets you own or have written permission to test. Do not probe public endpoints, validators, bridges or user data, do not move funds, and do not keep secrets you obtain during research.

## Handling and disclosure

The maintainers confirm receipt by email or in the private advisory, assess scope and severity, and work on the fix in a private advisory fork when one is needed. The fix lands on `main`, and when it changes the chain node it ships in a release as described in [RELEASE.md](RELEASE.md). The advisory is published once the fix is available, crediting reporters who submitted by email only unless they ask otherwise. Reports through GitHub or another public surface are fixed without attribution or reward. A report found to be out of scope is closed with the reason given by email or in the advisory. Please keep details private until the advisory is published.

## Response targets

These response targets are best-effort commitments. The maintainers aim to acknowledge a report within three business days of receipt and complete triage with a scope and severity decision within ten business days of receipt. While remediation is in progress, they provide a status update at least every fourteen days.

Remediation targets, counted from the severity decision, are thirty days for Critical issues, sixty days for High issues, ninety days for Medium issues and the next scheduled release for Low issues. Coordinated disclosure is targeted within fourteen days of the fix being available.

If a target is missed, follow up by email or comment in the private advisory. After two further business days without a response, email <security@sidioralabs.com> with the report or advisory identifier.

## Bug bounty

This repository text is the bug bounty policy. The programme rewards vulnerabilities in the in-scope code listed below, subject to the out-of-scope exclusions. Rewards range from $500 to $100,000 USD per unique vulnerability, paid once to the first reporter. The amount is determined by the affected subsystem, its role in the network, the severity and effect on core functions, and the risk to user funds.

Bounty eligibility requires all three of the following:

1. Submission by email to <security@sidioralabs.com> only.
2. A full and clear report covering every item in the Include list under Report privately above, including an end-to-end reproduction.
3. Credentials demonstrating that the submitter is legitimate and competent, for example prior published disclosures or CVE credits, a professional or audit-firm affiliation, a verifiable identity, and an account of how the issue was found.

A report missing any of these three requirements is handled as an ordinary report with no reward. Reports submitted through GitHub or another public surface are fixed through the normal maintenance process without attribution or reward.

Stop testing at a proof of concept. Never move funds that are not your own, and follow the testing restrictions above. Keep the issue private until it is fixed and the advisory is published.

## Known exemptions

- [GHSA-55f3-3qvg-8pv5](https://github.com/advisories/GHSA-55f3-3qvg-8pv5): `wasmer` 4.2.2 is pinned by `cosmwasm-vm` 1.5.12 in `wasm-runtime/libwasmvm`. The modules load without WASI filesystem access. By maintainer decision of 2026-11-05, this exemption holds until the next runtime upgrade and is reviewed at that upgrade. The rationale and mitigation are recorded in [repair_22_31_450.kvx](wasm-runtime/libwasmvm/repair_22_31_450.kvx).

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
