---
name: get-checks-status
description: Fetch the Explorer / Build workflow status for the current branch
disable-model-invocation: true
---
# Get Checks Status

## Prerequisites: GitHub CLI

Before running any steps, ensure the GitHub CLI is available and authenticated. **Follow the [check-github-cli skill](../check-github-cli/SKILL.md)** (ensure `gh auth status` succeeds; if not, guide the user to install/configure `gh` and do not proceed). For this workflow, the account needs read access to the repository.

## Overview

Fetch the status of [Explorer / Build](../../../../../.github/workflows/explorer-build.yml)
for the current branch. Commands run from the monorepo root.

## Steps

_Note_ in the command output, format all URLs as clickable Markdown links with descriptive labels

1. **Determine the current branch**
   - Run: `git branch --show-current` (or equivalent) to get the branch name. Use this in subsequent `gh` commands.

2. **Fetch the status of the most recent workflow run**
   - List the latest run for the Explorer / Build workflow on the current branch:
     - `gh run list --workflow=explorer-build.yml --branch <BRANCH> --limit 1`
   - If no runs are returned, report that no matching run is available and skip other steps.
   - If a run exists, get its status and run ID, then:
     - **If the workflow is pending (queued or in progress)**: Use `gh run watch <RUN_ID>` to subscribe to completion (or poll with `gh run view <RUN_ID>` until status is completed).
     - **If the workflow has failed**: Retrieve failed jobs and steps with:
       - `gh run view <RUN_ID>` for summary and job list.
       - `gh run view <RUN_ID> --log-failed` for failed step logs (optional, for details).
     - Format the run URL as a Markdown link, e.g. `gh run view <RUN_ID> --json url --jq .url` returns the URL; or use: `https://github.com/<OWNER>/<REPO>/actions/runs/<RUN_ID>`.

3. **Report the status with a system notification**
   - Upon workflow completion, notify with the final status (success or failure).
   - In case of failure, notify about the error along with the list of failed jobs and steps.
