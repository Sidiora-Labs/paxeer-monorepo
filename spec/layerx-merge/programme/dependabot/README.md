# Dependency alert reports

The JSON documents under `input` contain an `alerts` array. Each alert names
`ghsa`, `number`, `package`, `ecosystem`, `manifest`, `severity`, `range` and
`patched`. `patched` is the first fixed version, or null when the advisory names
none. A report under `reports` describes the dependency version or pin for each
advisory in its input group.

## Report shape

`reports/<slug>.kvx` uses an `[alert.<GHSA identifier>]` section for each advisory.
Each section has these string keys in this order:

| Key | Value |
| --- | --- |
| `package` | Package name from the alert |
| `manifest` | Repository-relative manifest path from the alert |
| `verdict` | `upgraded`, `pinned` or `no-fix` |
| `version_before` | Nonempty prior version |
| `version_after` | Dependency version reported for the current manifest |
| `pinned_by` | Dependency path and pin explanation for `pinned` or `no-fix` |
| `revision` | Forty lowercase hexadecimal characters identifying the source commit |

## Verdicts

- `upgraded`: the advisory names a patched version, and `version_after` is at
  least that version. The version text appears in the named manifest or a sibling
  lockfile.
- `pinned`: the advisory names a patched version, and `pinned_by` describes the
  dependency path retaining the current version.
- `no-fix`: the advisory names no patched version, and `pinned_by` describes the
  dependency path.

## Validation

From the repository root:

```sh
python3 spec/layerx-merge/programme/dependabot/check.py <slug>
```

The [checker](check.py) matches advisory IDs, package names and manifest paths to
the input. It validates key order, verdicts, revision shape and nonempty prior
versions. For `upgraded`, it compares numeric version components and looks for
`version_after` in the manifest or a sibling `package-lock.json`, `pnpm-lock.yaml`,
`yarn.lock`, `go.sum` or `Cargo.lock`, according to the manifest type. For `pinned`
and `no-fix`, it requires a `pinned_by` explanation of at least eight characters
and checks whether the advisory declares a patched version.
