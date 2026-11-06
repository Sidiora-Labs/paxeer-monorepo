# Native source runtime package

The source runtime package contains the control service and migration catalog, Node runtime, source SDK, signed client, host materializer, virtual filesystem module, native CLI and native component executable. Its manifest records every shipped file and internal dependency link, plus the complete input inventory for the service, dependencies, binaries, linked libraries and transitive bundle sources. Assembly rejects inputs that change during the build and dependency links that escape their package. PostgreSQL, S3 storage and the operating system kernel remain deployment services.

## Build and verify

On the release builder, compile the current native and service inputs, then assemble the package:

```sh builder
make hyperfide hyperfide-native CFLAGS='-O2 -Werror'
npm --prefix core/control-plane run build
python3 deploy/hyperfide/source-control/package.py --node /path/to/node --output /path/to/empty-output
```

The Node version must match `core/control-plane/package.json`. The assembler checks service build stamps and native freshness, bundles the SDK and host modules using the existing workspace build tool, and copies the installed production dependency graph. It performs no package installation or remote publication. Repeating assembly against unchanged inputs produces identical archive bytes.

For an isolated candidate while development continues, `--service-root` and
`--sdk-root` select immutable copies inside the builder workspace. SDK selection
also applies to imports through the host and signed-client facades. The complete
input manifest records those selected sources; give the verification run an
explicit `--label` so a canary candidate remains identifiable.

Run the packaged component profile with an owned PostgreSQL instance:

```sh builder
python3 tests/hyperfide/cases/source_control/7_1.py
```

The driver explicitly builds `hyperfide-native` on the release builder, then assembles twice, compares the archives, extracts one, checks its complete inventory, and uses its Node, service, SDK, host modules and native executables. `make all` supplies the main CLI; the separate component build records its command output in `native-components-build.log`. The assembler still refuses stale or missing native outputs. The profile starts the actual `bin/source-control` launcher and verifies migration upgrades preserve existing Git source rows. It exercises API-only source access, sparse materialization, a durable checkpoint, independently checked acceptance, and publication to a disposable local Git destination with independent content readback. `--package-only` performs the archive checks without starting PostgreSQL or source operations; `--package-dir` checks an existing assembled package without rebuilding its native executable.

## Install and start

Extract `native-source-runtime.tar.gz` into a new immutable release directory. Verify its archive hash against `package-manifest.json` before selecting the directory for service startup. Keep the previous directory available until the current deployment and migration checks pass.

Provide the PostgreSQL connection, S3 settings, cursor key, workspace budgets and private listener configuration described in the [source service guide](../hyperfide/source-control.md). Set `HF_SOURCE_CONTROL_CONFIG` to the private listener configuration and put the immutable release’s `bin` directory on `PATH`. Apply the complete packaged migration catalog through the existing migration lifecycle, then run:

```sh source
source-control
```

The listener configuration contains public token verification keys and TLS file references. Signing keys remain with the issuer. Source workers receiving native control-plane credentials use the signed endpoint and the private configuration installed through `HF_SOURCE_CONFIG`.

The SDK exports are available from `sdk/index.js`; `sdk/signed.js` exports `createSignedSourceClient`. The native command is `bin/hyperfide source invoke --request-file FILE`. Host modules under `host/` expose the bounded cache, workspace journal, sparse materializer and checkpoint adapter.

## Filesystem profile

The initial materializer uses Linux descriptor-relative paths. Virtual workspaces require the declared Linux FUSE profile, `/dev/fuse`, the necessary kernel capability and verified mount/unmount helpers. The package includes the available helper executables and their linked libraries; host discovery still checks actual kernel access before mounting. A host selects `SPARSE` or `MATERIALIZED` explicitly when virtual mounting is unavailable.

Dirty state belongs to the workspace journal and is retained independently from the immutable cache. Keep the exact workspace and operation identities when recovering interrupted checkpoints. Restore source metadata, object placement and retained roots together, and establish a fresh authority epoch before reopening writes.

## Release closure

Packaging and product acceptance have separate records. A complete release binds all canonical source cases and required backend/provider variants to the same source and policy inputs:

```sh release
python3 tests/hyperfide/cases/source_control/6_3.py --packet /path/to/closure.json
python3 tests/hyperfide/cases/source_control/7_2.py --manifest /path/to/release.json
```

The closure reruns the canonical checker for every full-case receipt. The release gate checks artifact checksums and the operator acceptance record bound to that closure packet and source revision. Component receipts do not satisfy a full-case obligation.

The standalone listener requires `HYPERFIDE_DEPLOYMENT_ID` and an absolute
`HYPERFIDE_AUTHORITY_JOURNAL_DIR`. The journal directory must already contain
accepted authority state, be private and owned by the service user, and remain
independent of PostgreSQL restoration. Startup never creates or repairs it.
Every source metadata transaction reads current external authority, verifies the
same epoch and proof against the primary, and rechecks that binding and its
expiry before commit. A restored database with an older authority epoch remains
fenced even when its source credentials and capabilities are still valid.

## Release manifest and compatibility

`source-release-manifest.json` is emitted beside the archive and inside it. `package-manifest.json` pins its digest; `runtime-manifest.json` inventories its bytes. It records source format and protocol versions read from the built contracts, the exact production dependency versions, content pins and dependency edges, and all target migration checksums. This package compatibility declaration is separate from the operator-accepted release input consumed by 7.2.

The `git-source-pre-native-0736-v1` upgrade profile starts with the exact migration catalog preceding 0736. The packaged migration coordinator upgrades that database, preserves legacy Git object rows and specification source bindings byte for byte, verifies migration checksums, and proves a repeated application performs no migration. Git SHA-1 IDs and native SHA-256 IDs retain distinct namespaces. This proves the declared schema upgrade component; a deployment cutover, retained evidence adoption and remote writer isolation still require their own qualified migration campaign. Rollback to an older writer after acknowledged native writes is refused.

The package contains the recovery-profile schema. The driver checks complete declarations and probes missing fields, unsupported failure models, absent synchronous replicas and backup age beyond retention. Its owned PostgreSQL/SigV4 profile remains UNQUALIFIED for disaster recovery; component observations include actual metadata settings and measured conditional create/read/delete behavior. Consult the [profile table and restore procedure](../hyperfide/source-control.md#profile-qualification) before selecting a deployment profile.

## Executable documentation and evidence

The 7.1 driver extracts `sh source` fences from both source guides, records their document and line, and executes the exact argument vectors through the extracted runtime’s `bin` directory. It runs service startup through readiness and graceful shutdown, and the product guide’s request against the packaged signed HTTPS service. Builder and release commands run only in their separately declared environments. `--package-only` reports assembly checks and cannot claim these live command checks.

The driver compares the carried source specification’s case inventory with `registry.json` and `claims.json`, including requirements, owning tasks, status, assertion IDs and mandatory variants. Every listed source pin and evidence reference is checked for current applicability; absent, changed or incomplete records refuse the driver. No full-case receipt is claimed by this guide. The following input references are part of that check.

```json source-release
{
  "schema_version": 1,
  "source_refs": [
    {
      "path": "tools/hyperfide/tests/hyperfide/source-control/registry.json",
      "sha256": "5379f8bfbf1bfd71546fc127263da683819d91dba66234aa028b695170b58874"
    },
    {
      "path": "tools/hyperfide/tests/hyperfide/source-control/claims.json",
      "sha256": "c305a628a204c85f9deeafb6a768032d60535e469a887bf95a5c833436ec65e6"
    },
    {
      "path": "spec/layerx-merge/source-control/source-spec.kvx",
      "sha256": "c8474867ab2767e5d8ac9f4fd2b04160bd26f51c70ae5b59175f9f8878b33f7b"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/contracts.ts",
      "sha256": "d916cbde34b3410b024a503cc12c90a487f504999e356a7104603c9d1f72571b"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/graph-adapter.ts",
      "sha256": "290fbff086a0277035a196fd0202bc276f07ede7e38157d481cbfad4caf1dc4b"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/recovery.ts",
      "sha256": "11c7d9d3c633cb999c60602868094300131d2e6899db44f90a4aceaf03d9857b"
    }
  ],
  "evidence_refs": [],
  "recovery_profiles": []
}
```
