# Wallet pull request notes

This file is a manual checklist for changes to `apps/wallet`. GitHub does not
automatically load a pull request template from a package's nested `.github`
directory. Keep the [repository pull request template](../../../.github/pull_request_template.md)
and copy the relevant wallet sections below into your monorepo pull request body.

See the [wallet README](../README.md), [changelog](../../../docs/wallet/app/CHANGELOG.md), and
[wallet issue prompts](ISSUE_TEMPLATE/bug_report.md).

## Description

<!-- Describe the problem, resulting behavior, and affected wallet screens or code paths. -->

## Type of Change

<!-- Mark the relevant options with an "x". -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring (no functional changes)

## Related Issue

<!-- Link the issue and spec task or requirement, where applicable. Use Fixes #123 only for an issue this PR resolves. -->

## How Has This Been Tested?

<!-- Use the checks required by the task. Record only checks actually run; explain checks not run. Available scripts are in ../package.json. -->

- Verified revision:
- Exact command(s), exit code(s), and evidence location(s):
- Manual steps and observed results, if applicable:
- Browser/device/OS and tab or installed PWA, if manually tested:
- Checks not run and why:

<!-- For network changes, identify the configured endpoint without credentials, chain id, and any selected tenant domain. Paxeer mainnet-beta uses EVM chain id 125 (0x7d) and public node root URLs. See ../../../docs/site/docs/reference/public-rpc.md. -->

## Checklist

- [ ] I reviewed the affected wallet paths and followed their existing style.
- [ ] I updated relevant documentation and release notes where applicable.
- [ ] I added or updated coverage where required and reported its actual outcome above.
- [ ] I described compatibility, configuration, and dependent changes where applicable.
- [ ] I reported failures and limitations without claiming checks that did not run.
- [ ] I removed secrets and private wallet information from screenshots, logs, and examples.

## Screenshots (if applicable)

<!-- Show changed UI states, including error states where relevant. Redact private information. -->

## Additional Notes

<!-- Describe remaining limitations, rollout requirements, or dependent changes reviewers need to assess. -->
