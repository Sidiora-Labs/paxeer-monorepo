# Dependency floors

This directory holds the dependency-floor checker, minimum-version definitions,
and their tests. It checks dependency versions used by the Paxeer network node,
services, SDKs, and applications by reading their manifests and lockfiles.

## Layout

- [`floors/`](floors/) — JSON minimum-version definitions grouped by repository area.
- [`tests/`](tests/) — checker, dependency-pin, and Go checksum tests; `python-tests.mk` registers the suite with the root test target.
- [`lockfloor.py`](lockfloor.py) — Python command-line checker and version comparison functions.
- [`README.md`](README.md) — usage and supported input formats.

## Build and test

The Python checker has no separate build target. Its floor definitions are data
files. The root [Makefile](../../Makefile) includes
`tools/deps/tests/python-tests.mk` and runs this directory's tests with other
Python tool suites:

```sh
make test-tools-python
```

The suite uses pytest. Its integration cases also invoke Go for module checksum,
dependency, and build checks, and Yarn for frozen explorer-tool installations.

Run from the repository root with Python 3.11 or newer:

```sh
python3 tools/deps/lockfloor.py tools/deps/floors/go-root.json
```

Every resolved copy within a floor's range must meet its minimum. Dotted numeric
segments compare numerically, prereleases sort below releases, and `v` prefixes
and build metadata are ignored; trailing numeric zeroes compare equally. Exit
status is 0 when all floors pass and 1 on floor violations or data errors;
invalid command-line arguments exit 2. Each violation is one tab-separated line
containing manifest, package, resolved version, and floor. Missing packages report
`<unresolved>`; input errors go to stderr. The checker reads manifests and locks
without changing them.

## Configuration

The checker takes the floor JSON file as its `floors_file` argument and reads no
environment variables. Go checksum tests set `GOWORK`, `GOFLAGS`, and
`GOTOOLCHAIN` for their subprocesses. The Android dependency test reads
[`pom.xml`](../../sdk/integrations/android/pom.xml),
[`app/build.gradle.kts`](../../sdk/integrations/android/sample-app/app/build.gradle.kts),
and [`gradle/verification-metadata.xml`](../../sdk/integrations/android/sample-app/gradle/verification-metadata.xml).

Select a floors file from [`floors/`](floors). Each is a JSON object mapping
manifest paths to package minimums. For example, `floors/go-root.json` includes:

```json
{
  "manifests": {
    "go.mod": {
      "filippo.io/edwards25519": "1.1.1"
    }
  }
}
```

Manifest paths are relative to the current working directory; absolute paths also
work. `package.json` resolves through the nearest ancestor directory containing
`package-lock.json`, Yarn v1 `yarn.lock`, or `pnpm-lock.yaml`. If `packageManager`
declares `npm`, `yarn` or `pnpm`, the search uses only that manager's lock format;
otherwise multiple locks in the nearest matching directory are an error.
`Cargo.toml` uses the nearest ancestor `Cargo.lock`. Lock files may
also be named directly. `go.mod`, `pom.xml`, and `requirements.txt` are read
directly. Maven packages can use `artifactId` or `groupId:artifactId`; properties
must resolve from that POM's own `properties` block. Requirements use `==` pins.
Python package names compare case-insensitively with runs of `-`, `_` and `.`
treated as equivalent. Go replacements use the replacement version; local
replacements cannot establish a version floor and fail if selected.

Each package floor may also be an object such as
`{"minimum": "2.8.0", "applies_from": "2.0.0"}` to scope it to an advisory's
version range. Both fields must be valid version strings. Resolved copies below
`applies_from` are outside the range and are skipped; every copy at or above it
must meet `minimum`. A plain version string continues to apply to every copy,
and violations report the minimum in the same tab-separated format.

## Documentation

- [Monorepo layout](../../docs/MONOREPO.md) — repository components and build boundaries.
- [Readiness matrix](../../docs/release/7.0.0-readiness-matrix.md) — manifests and build and test entry points for the projects covered by dependency floors.

There is no dedicated dependency-floor page under `docs/site`.
