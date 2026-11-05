# Dependency alert closure

One task per manifest group closes the open dependency alerts recorded under `input/<slug>.json`
(one entry per alert: `ghsa`, `number`, `package`, `ecosystem`, `manifest`, `severity`, `range`, `patched`).
`patched` is the first fixed version, or null when the advisory names none.

## Report shape

`reports/<slug>.kvx` carries one block per alert of the input:

```
[alert.GHSA-xxxx-xxxx-xxxx]
package = "tar"
manifest = "tests/integration/dapp_tests/package-lock.json"
verdict = "upgraded"
version_before = "6.1.11"
version_after = "7.5.21"
pinned_by = ""
revision = "<40-hex commit of main the reading was made at>"
```

Verdicts:

- `upgraded`: the manifest and its lock now resolve the package at `version_after`, which is the
  first patched version or later; `version_after` appears in the manifest or its lock.
- `pinned`: a patched version exists but the latest release of the direct dependency that pulls the
  package still excludes it; `pinned_by` names that dependency, its latest release and the range.
- `no-fix`: the advisory names no patched version; `pinned_by` names the dependency path that pulls
  the package and the parent release that would remove it, when one exists.

`python3 spec/layerx-merge/programme/dependabot/check.py <slug>` validates the report against the input
and, for `upgraded`, that `version_after` is at least the patched version and appears in one of the
group's manifests or locks. Reports state product facts only.
