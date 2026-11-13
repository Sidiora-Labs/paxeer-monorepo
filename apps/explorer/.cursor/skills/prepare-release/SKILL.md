---
name: prepare-release
description: Prepare explorer configuration documentation for a Paxeer release
disable-model-invocation: true
---
# Explorer release documentation

The explorer frontend image is built by
[Explorer / Images](../../../../../.github/workflows/explorer-images.yml).
Its `GIT_TAG` build argument comes from
[tools/release/version.json](../../../../../tools/release/version.json) for
branch builds and from the Git tag for tag builds. The image tags are
`sha-<short-sha>`, with `main` and `latest` also published from `main`.
The frontend package version and the network release version are separate fields.
The network release identity and artifacts are described in
[RELEASE.md](../../../../../RELEASE.md).

## Configuration references

Explorer configuration is documented in
[ENVS.md](../../../../../docs/explorer/ENVS.md) and
[DEPRECATED_ENVS.md](../../../../../docs/explorer/DEPRECATED_ENVS.md).
The [release notes template](../../../../../docs/explorer/RELEASE_NOTES.md)
lists compatibility and configuration changes. Configuration declarations live
under [configs/app](../../../configs/app), with validation in
[deploy/tools/envs-validator](../../../deploy/tools/envs-validator).

## Pull request metadata

The [fetch-release-prs.js helper](fetch-release-prs.js) reads pull request labels
and descriptions through the GitHub API. Its default repository is
`blockscout/frontend`; `gh repo view` selects the checkout's repository.
GitHub CLI authentication is described by the
[check-github-cli guide](../check-github-cli/SKILL.md).

From the monorepo root:

```sh
node apps/explorer/.cursor/skills/prepare-release/fetch-release-prs.js <pr-number> --out <output-file>
```

The helper accepts multiple pull request numbers. With `--out` it writes the JSON
to that file; otherwise it prints JSON to standard output. The output is a JSON
object with a `prs` array. Entries carry `number`, `title`,
`author`, `url`, `labels` and `body`. The helper waits 400 milliseconds between
requests. Configuration changes can be identified by the `ENVs` label and the
corresponding description.
