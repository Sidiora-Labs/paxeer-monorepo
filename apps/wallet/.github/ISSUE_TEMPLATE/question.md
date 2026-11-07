---
name: Question
about: Ask a question about the monorepo wallet app
title: '[QUESTION] wallet: '
labels: question
assignees: ''
---

# Wallet question

This file is a manual template for `apps/wallet`. GitHub does not automatically
discover templates in a package's nested `.github` directory. Open a
[monorepo issue](https://github.com/Sidiora-Labs/paxeer-monorepo/issues/new), use
`[QUESTION] wallet: <summary>` as the title, and copy the sections below into its body.

Check the [wallet README](../../README.md) and [changelog](../../../../docs/wallet/app/CHANGELOG.md) first.
Use the [bug report prompts](bug_report.md) for reproducible failures or the
[feature request prompts](feature_request.md) for proposed improvements.

## Your question

Ask a specific question and describe the result you are trying to achieve.

## Context

- Affected screen or action:
- Wallet version or commit and deployment URL, if known:
- Device, OS, browser/version, and whether using a tab or installed PWA:
- Network selection and chain id, if relevant:
- Documentation consulted and steps already tried:
- Exact error message or redacted screenshot, if relevant:

Paxeer mainnet-beta uses EVM chain id `125` (`0x7d`); identify any selected tenant
domain when asking about network behavior. See the
[network context](../../../../docs/wallet/app/CHANGELOG.md#network-context).

Share only the context needed to answer the question. Never include recovery
phrases, private keys, passwords, session tokens, or raw wallet storage exports.
