---
name: Bug Report
about: Report a bug in the monorepo wallet app
title: '[BUG] wallet: '
labels: bug
assignees: ''
---

# Wallet bug report

This file is a manual template for `apps/wallet`. GitHub does not automatically
discover templates in a package's nested `.github` directory. Open a
[monorepo issue](https://github.com/Sidiora-Labs/paxeer-monorepo/issues/new), use
`[BUG] wallet: <summary>` as the title, and copy the sections below into its body.

See the [wallet README](../../README.md) and [changelog](../../CHANGELOG.md).
For help without a reproducible bug, use the [question prompts](question.md).

## Describe the bug

Describe the affected wallet screen or action and the actual result.

## To reproduce

1. Open the wallet at the affected deployment's `/wallet/` path.
2. Describe the starting state and exact actions, including inputs needed to reproduce.
3. Record the error or unexpected result and whether it happens consistently.

## Expected behavior

Describe the result you expected and how it differs from what happened.

## Environment

- Wallet package version and commit, if known; deployment URL:
- Device and OS version:
- Browser and version:
- Browser tab or installed PWA:
- Affected wallet route or screen:
- Network selection, chain id, and public RPC URL without credentials:

Paxeer mainnet-beta uses EVM chain id `125` (`0x7d`). Its public JSON-RPC uses
the node's root URL, for example `https://api-mainnet-beta.paxeer.network`.
The app reads its default from `NEXT_PUBLIC_PAXEER_RPC_URL`; mention a custom
endpoint or selected tenant domain if relevant. See the
[changelog's network context](../../CHANGELOG.md#network-context).

## Screenshots

Attach screenshots if they help explain the problem. Redact private information.

## Logs

Include relevant, redacted browser console messages or server logs if available.
For request failures, include the method, HTTP status, exact error code/message,
and approximate time. Include a public transaction hash if relevant and safe to share.
Never include recovery phrases, private keys, passwords, session tokens, or raw
wallet storage exports.

## Additional context

Describe workarounds, recent changes, and anything else needed to reproduce the issue.
