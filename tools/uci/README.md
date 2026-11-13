# UCI workflow library for Paxeer

`tools/uci` holds the vendored UCI (Unified Continuous Integration) workflow
library and review prompts. It supports development of the Paxeer network through
reusable Go checks, release automation, backports, pull request reviews and issue
maintenance. The vendor source and pinned revision are recorded in `VENDOR.kvx`.

## Layout

- [`.github/`](.github/) — GitHub Actions workflows, dependency update configuration and review prompts.
- [`LICENSE`](LICENSE) — Apache license text.
- [`README.md`](README.md) — directory guide and upstream project description.
- [`VENDOR.kvx`](VENDOR.kvx) — vendor source, reference and revision.
- [`.github/dependabot.yml`](.github/dependabot.yml) — GitHub Actions dependency updates.
- [`.github/workflows/go-lint.yml`](.github/workflows/go-lint.yml) — module tidiness, vet, formatting and generated-file checks.
- [`.github/workflows/go-test.yml`](.github/workflows/go-test.yml) — Go tests and race tests.
- [`.github/workflows/goreleaser-release.yml`](.github/workflows/goreleaser-release.yml) — configurable GoReleaser publication.
- [`.github/workflows/release-check.yml`](.github/workflows/release-check.yml) — version and release checks for pull requests.
- [`.github/workflows/release-publish.yml`](.github/workflows/release-publish.yml) — version-based release publication and release metadata.
- [`.github/workflows/backport.yml`](.github/workflows/backport.yml) — backport pull requests.
- [`.github/workflows/stale-check.yml`](.github/workflows/stale-check.yml) — stale issue and pull request maintenance.
- [`.github/workflows/ai-review.yml`](.github/workflows/ai-review.yml) — automated pull request review.
- [`.github/workflows/ai-assistant.yml`](.github/workflows/ai-assistant.yml) — pull request comment assistant.
- [`.github/workflows/ai-review-self.yml`](.github/workflows/ai-review-self.yml) — review workflow caller.
- [`.github/workflows/ai-assist.yml`](.github/workflows/ai-assist.yml) — assistant workflow caller.
- [`.github/seidroid/`](.github/seidroid/) — review and assistant documentation and Markdown prompts.

## Build and test

This directory holds CI configuration and prompt data files. It has no standalone
build, package manifest or local test suite. Its reusable Go workflows check out
the calling repository and run these commands against that repository's Go module:

| Command | Workflow | Purpose |
| --- | --- | --- |
| `go mod tidy` | `go-lint.yml` | Check module dependency tidiness against tracked files. |
| `go vet ./...` | `go-lint.yml` | Check Go source. |
| `gofmt -s -l .` | `go-lint.yml` | List files requiring formatting. |
| `go generate -x ./...` | `go-lint.yml` | Regenerate files and check for changes. |
| `go test ./...` | `go-test.yml` | Run Go tests. |
| `go test -race ./...` | `go-test.yml` | Run Go tests with race detection. |

`goreleaser-release.yml` builds and publishes the caller's configured release
through the GoReleaser action. Its `working-directory`, `go-version-file`, `args`
and `prebuild-script` inputs control that job.

## Configuration

The YAML files in [`.github/workflows/`](.github/workflows/) declare reusable
workflow inputs. Go checks accept `go-version`; `go-test.yml` also accepts `test`
and `race`. Release workflows accept `branches`, backports accept `backport-team`,
and the stale check declares its label and timing inputs in `stale-check.yml`.
Review and assistant inputs, including `uci-ref`, `guidelines-file`,
`extra-instructions`, `allowed-team` and `allow-write`, are documented in the
[AI workflow guide](.github/seidroid/ai-review/README.md).

The workflows read these GitHub Actions configuration names, depending on the job:

- Secrets: `UCI_GITHUB_TOKEN`, `PLATFORM_CODE_AGENT_APP_ID`,
  `PLATFORM_CODE_AGENT_APP_PK`, `PLATFORM_CODE_AGENT_OPENAI_API_KEY` and
  `PLATFORM_CODE_AGENT_CURSOR_API_KEY`.
- Variables: `PLATFORM_CODE_AGENT_ANTHROPIC_FDRL_ID`,
  `PLATFORM_CODE_AGENT_ANTHROPIC_SVAC_ID`, `SEI_LABS_ANTHROPIC_ORG_ID` and
  `PLATFORM_CODE_AGENT_USER_ID`.
- Authentication environment variables passed to workflow steps: `GH_TOKEN`,
  `GITHUB_TOKEN` and `CURSOR_API_KEY`.

In the calling repository, the Go checks read `go.mod` and `go.sum`, the release
checks read `version.json`, and GoReleaser uses the caller's `.goreleaser.yaml` and
selected Go version file. Review guidelines come from the file selected by
`guidelines-file` on the pull request's base branch. Review and assistant prompts
are the Markdown files in [`.github/seidroid/ai-review/`](.github/seidroid/ai-review/).

## Documentation

- [Workflow secrets, variables and runners](../../docs/ci/secrets-and-runners.md) — repository CI configuration and runner selection.
- [Seidroid overview](.github/seidroid/README.md) — review and assistant features.
- [AI workflow guide](.github/seidroid/ai-review/README.md) — reusable workflow inputs and integration.

There is no dedicated UCI page under `docs/site`.

---

# UCI: Unified Continuous Integration

**UCI (Unified Continuous Integration)** is Sei’s **standard engineering guardrail** for CI. It delivers **consistency** across repositories and **maintainability** through shared, versioned workflows and sensible defaults.

* **Consistency:** One pattern for lint/test/build so teams don’t reinvent CI per repo.
* **Maintainability:** Centralized modules reduce duplication and drift, making updates safer and faster.

## Status

🚧 **Active development.**
The current focus is **Go**; additional languages may be added later.

## Principles

* **Consistency first:** Shared templates & defaults are the norm; overrides are opt-in and minimal.
* **Maintainability at scale:** Central updates propagate safely; repos stay aligned.
* **Guardrails, not handcuffs:** Opinionated by default, extensible where it counts.
