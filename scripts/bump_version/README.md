# bump_version

Precompile version management tool for the merged Paxeer network. The implementation is in [`main.go`](main.go). It runs as the second `go:generate` directive of [`tools/repotest/gen.go`](../../tools/repotest/gen.go), after `./scripts/protoc.sh`. To run just this tool, use `go run ./scripts/bump_version` from the repository root.

## What it does

When a new upgrade name is appended to [`node/tags`](../../node/tags), this tool:

1. Uses go-git to select changed directories under `precompiles/`, excluding `common` and `utils`.
2. Archives selected modules' current files into `precompiles/<module>/legacy/<folder>/` and appends the upgrade name to their `versions` files. Whenever any module is archived, it archives common utilities first.
3. Regenerates eligible modules' `setup.go` files from their `versions` files using [`setup.go.tmpl`](setup.go.tmpl).

Generation writes files in place. It has no transaction, rollback or completion marker; read [Idempotency](#idempotency) before retrying an interrupted run.

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

`git diff` does not display the contents of untracked archive files; inspect those too. Verify that every selected module has its expected archive files and exactly one new `versions` entry, and that common files and generated setup mappings are consistent. A zero exit status alone does not establish a correct historical baseline or complete archival.

## Naming conventions

| Concept | Format | Example |
|---------|--------|---------|
| New upgrade name (stored in `node/tags` and selected modules' `versions`) | `vMajor.Minor` | `v7.1` |
| Legacy folder / package name | tag with dots removed | `v70` (from `v7.0`), `v640` (from `v6.4.0`) |

In generated `GetVersioned`, entries before the last line of `versions` become literal legacy map keys. The active implementation uses the caller-supplied `latestUpgrade` key. Setup generation skips `common`, `utils`, modules without a `versions` file, and empty version lists; it warns and skips modules missing `<module>.go`.

### Input sanitization

The tool trims whitespace, ignores blank lines and checks the shape of the **last** entry in `node/tags`. Earlier tag entries are not rewritten. If that entry's dots-stripped common archive directory already exists, normalization and new-version checks are skipped. Otherwise, it normalizes the new entry before checking for duplicates and forward progress:

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

1. `gitFindCommit` walks commits touching `node/tags` from HEAD backwards. It selects the oldest commit in the initial contiguous run whose file contents contain the previous version string.
2. `gitPathHasChanges` compares each module directory's subtree hash at that baseline and HEAD. This includes any tracked file beneath the directory, including existing archives and `versions`, as well as file modes and additions/removals.
3. Selected modules are archived from the working tree. Uncommitted edits do not affect selection, but can enter an archive if that module is selected.

The history search uses a substring match, not an exact tag-line match. For example, `v6.4` can match `v6.4.0`. Independently inspect `git log -p -- node/tags` and the reported `comparing against <hash>` revision to confirm the intended introducing commit. Missing or unreadable history must be resolved before accepting the output; see [Error handling](#error-handling).

If `precompiles/common/` changed, all discovered modules are selected. If there is no earlier valid semver tag, all discovered modules are selected without a historical comparison. If no modules changed, archival and version appends are skipped, no common sentinel is created, and setup generation still runs.

## Idempotency

The existence of `precompiles/common/legacy/<folder>/` is only a skip sentinel. `archiveCommon` creates it **before** copying common files and before any module archival or version append. An error or interruption can leave that directory present with incomplete common files, missing module archives or partially updated `versions`. A subsequent run then skips all archival and regenerates setup files from that partial state. There is no automatic resume or completeness check.

After an interrupted or failed run:

1. Preserve the output and inspect `git status --short`, the tracked diff, and all new archive files. Identify the intended upgrade and dots-stripped folder, including any normalization of `node/tags`.
2. Compare every module in the original changed-module list against its source files and `versions` entry. Check common's `precompiles.go` and `evm_events.go` when present in the source. Directory existence alone proves nothing. If selection stopped before printing the list, establish the expected set from the committed-tree comparison after resolving any history problem.
3. To restart archival, restore the generator-modified tracked files (`node/tags`, affected `versions` and `setup.go` files, and any overwritten archive files) from the recorded pre-generation revision. Remove only archive files/directories confirmed to have been newly created by that attempt. Preserve unrelated work and earlier archives. Recheck that the intended source changes remain committed and that the checkout matches the saved state before proceeding.
4. Append the intended new upgrade name again, run the tool from the repository root, and review the complete output as in [Developer workflow](#developer-workflow).

Deleting only the common sentinel is insufficient: a module whose `versions` already contains the new tag will fail the duplicate check on another archival attempt. If the saved state or intended baseline cannot be established, stop the release rather than treating a retry as recovery.

If inspection confirms that archival and all version appends finished and only setup generation failed, fix the reported setup input problem and rerun to regenerate setup files. The sentinel still does not validate the archives.

## Testing selective archiving

Use a disposable checkout for this exercise. Establish that, relative to the independently verified previous-version baseline, only `precompiles/bank/` differs and `precompiles/common/` is unchanged. Commit a bank source change, append a fresh upgrade name and run the tool.

For a new name `v7.1`, expect bank's archive at `precompiles/bank/legacy/v71/`, common's archive at `precompiles/common/legacy/v71/`, and one `v7.1` append in `precompiles/bank/versions`. An unchanged module such as oracle should receive neither a `legacy/v71/` directory nor a version append. Common has no version append in this tool. Setup generation still visits every eligible module, so setup output alone does not indicate selection. Inspect file contents and registrations before discarding the exercise checkout.

## Error handling

Errors returned by `run` terminate the tool through `log.Fatal`. Some Git errors are suppressed or treated as changes, so the tool does **not** guarantee that every Git failure causes a nonzero exit. It also leaves files written before an error in place.

| Failure | Behavior |
|---------|----------|
| Repository open, HEAD lookup or initial log creation fails during archival | Returned error; process exits nonzero |
| History search finds no candidate | Returned error with hint to commit precompile changes |
| Reading `node/tags` or its contents fails during history traversal | Stops the search; may return the last matching commit instead of an error |
| Commit iterator fails | Iterator error is discarded; a prior candidate may still be returned |
| Base tree, HEAD commit or HEAD tree cannot be read during comparison | Returned error |
| Subtree lookup fails at either baseline or HEAD | Treated as changed for any lookup error, including a missing path |
| Archive write, version validation/append or setup generation returns an error | Process exits nonzero; earlier writes remain |

Do not publish output based on an unverified baseline, even if the tool exits zero. Restore readable history and independently confirm the previous tag's introducing commit and selected modules before restarting from the saved state. The current implementation does not enforce that release check.

## Files generated

- `precompiles/<module>/legacy/<folder>/*.go` — top-level Go files excluding tests and `setup.go`; package rewritten to `<folder>`, exact common-package imports redirected to the matching common archive
- `precompiles/<module>/legacy/<folder>/abi.json` — copied when the source ABI can be statted successfully
- `precompiles/common/legacy/<folder>/precompiles.go` and `evm_events.go` — common utilities, when present, with the package name rewritten
- `precompiles/<module>/setup.go` — regenerated for eligible modules from `versions`
- `precompiles/<module>/versions` — appended with new version (changed modules only)
- `node/tags` — rewritten with the last tag normalized if a new, unarchived entry had a patch

All generated `.go` files carry a `// Code generated by scripts/bump_version; DO NOT EDIT.` header.
