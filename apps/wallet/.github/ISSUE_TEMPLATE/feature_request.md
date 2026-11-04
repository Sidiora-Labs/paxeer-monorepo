---
name: Feature Request
about: Suggest an improvement to the monorepo wallet app
title: '[FEATURE] wallet: '
labels: enhancement
assignees: ''
---

# Wallet feature request

This file is a manual template for `apps/wallet`. GitHub does not automatically
discover templates in a package's nested `.github` directory. Open a
[monorepo issue](https://github.com/Sidiora-Labs/paxeer-monorepo/issues/new), use
`[FEATURE] wallet: <summary>` as the title, and copy the sections below into its body.

Check the [wallet README](../../README.md) and [changelog](../../CHANGELOG.md)
for existing behavior. Use the [bug report prompts](bug_report.md) for broken behavior
or the [question prompts](question.md) for usage help.

## Problem

Describe the user problem, the affected wallet screen or workflow, and who encounters it.
Give a concrete example of what is difficult today.

## Proposed solution

Describe the behavior you want and how a user would interact with it.
If the request involves the network, identify whether it concerns Paxeer mainnet-beta
(EVM chain id `125`) or a tenant domain. See the
[network context](../../CHANGELOG.md#network-context).

## Alternatives considered

Describe other approaches or workarounds and why they do not meet the need.

## Acceptance criteria

List observable outcomes that would demonstrate the feature solves the problem,
including relevant error or unavailable states.

## Additional context

Add related issues, examples, or screenshots. Redact private wallet information;
never include recovery phrases, private keys, passwords, or session tokens.
