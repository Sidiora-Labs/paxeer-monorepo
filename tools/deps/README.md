# Dependency floors

Run from the repository root with Python 3.11 or newer:

```sh
python3 tools/deps/lockfloor.py tools/deps/floors/<group>.json
```

The floors file is a JSON object mapping manifest paths to package minimums:

```json
{
  "manifests": {
    "apps/example/package.json": {
      "some-package": "1.2.3",
      "@example/library": "2.0.0"
    },
    "services/example/go.mod": {
      "example.org/module": "v1.4.0"
    }
  }
}
```

Manifest paths are relative to the current working directory; absolute paths also
work. `package.json` resolves through the nearest ancestor directory containing
`package-lock.json`, Yarn v1 `yarn.lock`, or `pnpm-lock.yaml`. If `packageManager`
is declared, its lock format is used; otherwise multiple locks in that directory
are an error. `Cargo.toml` uses the nearest ancestor `Cargo.lock`. Lock files may
also be named directly. `go.mod`, `pom.xml`, and `requirements.txt` are read
directly. Maven packages can use `artifactId` or `groupId:artifactId`; properties
must resolve from that POM's own `properties` block. Requirements use `==` pins.
Go replacements use the replacement version; local replacements cannot establish
a version floor and fail if selected.

Every resolved copy must meet its floor. Dotted numeric segments compare
numerically, prereleases sort below releases, and `v` prefixes and build metadata
are ignored. Exit status is 0 when all floors pass and 1 on violations or input
errors. Each violation is one tab-separated line containing manifest, package,
resolved version, and floor. Missing packages report `<unresolved>`; input errors
go to stderr. No floors file is supplied by this task.
