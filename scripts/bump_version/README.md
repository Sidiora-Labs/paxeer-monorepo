# bump_version

Precompile version management tool for the merged Paxeer network. The implementation is in [`main.go`](main.go) and [`archive.go`](archive.go). It runs as the second `go:generate` directive of [`tools/repotest/gen.go`](../../tools/repotest/gen.go), after `./scripts/protoc.sh`. To run just this tool, use `go run ./scripts/bump_version` from the repository root.

## What it does

When a new upgrade name is appended to [`node/tags`](../../node/tags), this tool:

1. Uses go-git to select changed directories under `precompiles/`, excluding `common` and `utils`.
2. Stages and validates the complete selected module and common archive set, then publishes it into `precompiles/<module>/legacy/<folder>/` and updates the selected modules' `versions` files. It publishes a completion marker last.
3. Regenerates eligible modules' `setup.go` files from their `versions` files using [`setup.go.tmpl`](setup.go.tmpl).

Archive publication uses a durable pending manifest and atomic file replacement. An interruption can still leave partial output visible until recovery finishes; setup generation starts only after the complete archive set has been validated and its completion marker published. Setup generation itself writes files in place. Read [Idempotency](#idempotency) before retrying an interrupted run.

## Developer workflow

Use a clean, disposable release checkout with readable Git history. Commit the intended precompile source changes first: selection compares committed trees, but archival reads working-tree files. Record the pre-generation revision and retain the generator's output for review.

For example, if `v7.1` is the next unused upgrade name above all existing versions:

```bash
# 1. Make and commit your precompile code changes
git add precompiles/bank/bank.go
git commit -m "bank: add new functionality"

# 2. Record the recovery revision, then append the new upgrade name
git rev-parse HEAD
echo "v7.1" >> node/tags

# 3. Regenerate
go run ./scripts/bump_version

# 4. Inspect the baseline, changed-module list, archives and registrations
git status --short
git diff -- node/tags precompiles/

# 5. Only after reviewing the complete output, commit it
git add node/tags precompiles/
git commit -m "bump to v7.1"
```

`git diff` does not display the contents of untracked archive files; inspect those too. Verify that every selected module has its expected archive files and exactly one new `versions` entry, and that common files and generated setup mappings are consistent. The completion marker verifies archive and registration bytes; review still establishes that the intended release sources and historical baseline were selected.

## Naming conventions

| Concept | Format | Example |
|---------|--------|---------|
| New upgrade name (stored in `node/tags` and selected modules' `versions`) | `vMajor.Minor` | `v7.1` |
| Legacy folder / package name | tag with dots removed | `v70` (from `v7.0`), `v640` (from `v6.4.0`) |

In generated `GetVersioned`, entries before the last line of `versions` become literal legacy map keys. The active implementation uses the caller-supplied `latestUpgrade` key. Setup generation skips `common`, `utils`, modules without a `versions` file, and empty version lists; it warns and skips modules missing `<module>.go`.

### Input sanitization

The tool trims whitespace, ignores blank lines and checks the shape of the **last** entry in `node/tags`. Earlier tag entries are not rewritten. A matching completion marker is validated before skipping archival; a pending manifest resumes the recorded snapshot. Directory existence alone never establishes completion. Pre-marker archives without a pending manifest are refused for inspection. Otherwise, the tool normalizes the new entry before checking for duplicates and forward progress:

| Input in `node/tags` | Behavior |
|---------------------|----------|
| `v7.1` | Used as-is |
| `v7.1.3` | Patch dropped with WARNING, `node/tags` rewritten to `v7.1` |
| `v6.4` | Error: semver-equal to existing `v6.4.0` |
| `foo`, `v6`, `v7.0-rc1` | Error: not a valid `vMajor.Minor` |
| Tag older than current latest | Error: "upgrade names must advance forward" |

The duplicate and ordering checks compare against earlier tags using semver; the previous version for history lookup is the greatest valid semver among those tags. A normalization write can remain even if a later check fails. Module `versions` files are separately checked for valid semver entries and semver-equal duplicates.

## How change detection works

The tool compares **committed tree hashes**, not the working tree:

1. `gitFindCommit` walks commits from HEAD backwards, reading `node/tags` at each commit. It selects the oldest commit in the initial contiguous run containing the previous version as an exact trimmed tag line. Blank lines are ignored. A missing file or nonmatching line ends that run; unreadable objects and traversal errors are fatal.
2. `gitPathHasChanges` compares each module directory's subtree hash at that baseline and HEAD. This includes any tracked file beneath the directory, including existing archives and `versions`, as well as file modes and additions/removals.
3. Selected modules are archived from the working tree. Uncommitted edits do not affect selection, but can enter an archive if that module is selected.

The history search distinguishes `v6.4`, `v6.4.0` and `v6.40`. Inspect `git log -p -- node/tags` and the reported `comparing against <hash>` revision to review the intended introducing commit. A shallow boundary reached while the tag is still present cannot prove its introduction and is refused. Missing or unreadable history must be resolved before archiving; see [Error handling](#error-handling).

If `precompiles/common/` changed, all discovered modules are selected. If there is no earlier valid semver tag, all discovered modules are selected without a historical comparison. If no modules changed, an empty archive transaction is marked complete without copying common or appending module versions, and setup generation still runs.

## Idempotency

The common archive directory can exist while module archives or version registrations are still missing. This partial-archive hazard also applies to archives produced before completion markers were introduced. Directory existence alone is never a safe rerun signal.

The tool first stages all selected Go files, ABIs and final `versions` files beneath `precompiles/common/legacy/.<folder>.pending/`. Its `manifest.json` records the tag, destination paths, SHA-256 digests and prior registration digests. The pending directory becomes visible only after the entire staged set is written and synced. Publication validates all staged files and destinations, copies archives, atomically replaces registrations, validates the published bytes, and publishes `precompiles/common/legacy/<folder>/.complete.json` last. Only then can setup generation advertise the release. This supports recovery from process interruption; concurrent generator runs in the same checkout are unsupported.

After an interrupted or failed run (for example `v7.1`, folder `v71`):

1. Preserve the output and recorded pre-generation revision. Inspect `git status --short`, `git diff -- node/tags precompiles/`, and untracked archive files. Confirm any normalization in `node/tags` and the intended changed-module list.
2. Inspect `precompiles/common/legacy/.v71.pending/manifest.json` and its `files/` snapshot. If the completion marker exists, inspect `precompiles/common/legacy/v71/.complete.json`. Confirm that the recorded tag and selected files match the intended release. Keep the pending snapshot intact; resume uses its original bytes even if active source files subsequently change.
3. With an intact pending manifest, rerun `go run ./scripts/bump_version` from the repository root. It reopens the snapshot, validates its digests and registration preconditions, republishes the entire set without duplicating version entries, then writes the completion marker. It refuses corrupt staged files, conflicting destination files and independently changed registrations. No setup files are regenerated on refusal.
4. Inspect every selected archive, each module's single new `versions` entry and the generated setups. A completed rerun validates the marker's recorded bytes before regenerating setups. If only setup generation failed, fix that input problem and rerun; the completed archives are still validated first.

If there is no pending manifest and an archive directory already exists, the tool refuses to infer completion. For these older or damaged attempts, compare every expected file and version registration to the saved release state. Restore only generator-modified tracked files from the recorded pre-generation revision and remove only files confirmed newly created by the failed attempt, preserving unrelated changes and earlier archives. Then reintroduce the intended tag and rerun. Do not delete only the common directory: partially appended module versions would remain inconsistent. If the baseline or saved state cannot be established, stop the release. Never fabricate a completion marker to bypass inspection.

A crash before the pending manifest is published may leave a `.<folder>.staging-*` directory. It is never a resume signal and has not published archives; after inspection it can be removed. A crash immediately after the completion marker is published may leave the pending directory, which a validated completed rerun removes.

## Testing selective archiving

Use a disposable checkout for this exercise. Establish that, relative to the independently verified previous-version baseline, only `precompiles/bank/` differs and `precompiles/common/` is unchanged. Commit a bank source change, append a fresh upgrade name and run the tool.

For a new name `v7.1`, expect bank's archive at `precompiles/bank/legacy/v71/`, common's archive at `precompiles/common/legacy/v71/`, and one `v7.1` append in `precompiles/bank/versions`. An unchanged module such as oracle should receive neither a `legacy/v71/` directory nor a version append. Common has no version append in this tool. Setup generation still visits every eligible module, so setup output alone does not indicate selection. Inspect file contents and registrations before discarding the exercise checkout.

## Error handling

Errors returned by `run` terminate the tool through `log.Fatal` with a nonzero exit. History selection and complete staging validation happen before any archive or registration is published. An error during publication can leave partial output and an intact pending manifest; setup generation is withheld until publication completes. A normalization write to `node/tags` can remain after an error.

| Failure | Behavior |
|---------|----------|
| Repository open, HEAD lookup or log creation fails | Returned error before any archive publication |
| Exact previous tag is absent, or its introduction crosses a shallow boundary | Refusal; restore readable history before retrying |
| Tag file/tree/content cannot be read, parsing fails, or commit iteration fails | Returned error; a prior candidate is never accepted as a fallback |
| A historical tag file is genuinely absent or contains no exact matching line | Ends the contiguous matching run; refuses if no candidate exists |
| Base tree, HEAD commit/tree or an existing subtree is unreadable | Returned error; only a genuinely absent subtree is treated as changed |
| Source parsing, ABI reading, or any module registration validation fails during staging | No archive or registration published; fix the reported source input |
| Staged digest or destination/registration precondition fails | Refusal before publication; inspect the snapshot and saved release state |
| Archive write, registration replacement or marker publication fails | Nonzero exit; retain the pending snapshot and inspect/resume as above |
| Completed archive or registration no longer matches the marker | Refusal before setup generation; inspect and restore the intended bytes |
| Setup generation fails after completion | Nonzero exit; earlier setup writes remain; fix the reported input and rerun |

For missing history, preserve the generator output and working tree, restore readable history in the disposable release checkout, and inspect `git log -p -- node/tags` to establish the exact previous tag's introducing commit. Retry only after that baseline can be established. Do not substitute a prefix match, accept a candidate from an unreadable traversal, or publish an unverified baseline. No network fetch is performed by the generator. Pending recovery uses the already validated snapshot rather than selecting a new baseline.

## Files generated

- `precompiles/<module>/legacy/<folder>/*.go` — top-level Go files excluding tests and `setup.go`; package rewritten to `<folder>`, exact common-package imports redirected to the matching common archive
- `precompiles/<module>/legacy/<folder>/abi.json` — copied when present; read errors are fatal
- `precompiles/common/legacy/<folder>/precompiles.go` and `evm_events.go` — common utilities, when present, with the package name rewritten
- `precompiles/<module>/setup.go` — regenerated for eligible modules from `versions`
- `precompiles/<module>/versions` — appended with new version (changed modules only)
- `precompiles/common/legacy/.<folder>.pending/` — staged snapshot and manifest retained until completion
- `precompiles/common/legacy/<folder>/.complete.json` — completion marker with archive and registration digests, published last
- `node/tags` — rewritten with the last tag normalized if a new, unarchived entry had a patch

All generated `.go` files carry a `// Code generated by scripts/bump_version; DO NOT EDIT.` header.
