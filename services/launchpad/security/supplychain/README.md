# Supply-chain authority primitives

This directory contains release admission policy, reviewed-tool checks, SBOM and
scanner adapters, reproducible-build checks, runtime-confinement checks and
registry promotion. The implementations consume independently approved inputs;
their presence does not establish that a particular image passed qualification.

## Admission and independent authority

[policy.admit_signed](policy.py) is the local Ed25519 admission primitive. Its
caller supplies independently trusted source, workflow, immutable image, SBOM,
repository, builder, run, ref and signing-public-key SHA256 expectations.
Artifact fields cannot establish trust. It verifies the exact artifact bytes
with OpenSSL before parsing them. Policy tests generate and destroy local keys;
they do not read production signing keys.

The release envelope binds six distinct successful check IDs — `verify`,
`contracts`, `frontend`, `policy`, `scan` and `signing` — to those expectations.
Evidence lifetime is at most 24 hours. Forks, pull requests, stale evidence,
scanner outages, malformed or missing findings, reachable critical findings,
and expired or unapproved high exceptions fail closed. Every finding needs
exposure analysis. A high exception lasts at most seven days and requires an
independently approved canonical digest binding the finding, source, image,
exception identity, controls and expiry. A builder-signed approver name alone
cannot authorize it.

[controller.py](controller.py) also implements schema-version-2 release admission
with a verified Cosign bundle, an expected GitHub workflow signing identity,
repository/commit constraints, SLSA provenance, approved binary and scanner
inputs, SPDX artifacts and live GitHub run/job verification. `GitHubChecks`
checks the exact workflow path, source commit, run attempt and successful job
identities. This configured controller path is distinct from the local
Ed25519 primitive.

The controller's promotion path copies the approved OCI layout with ORAS,
attaches SBOM/provenance attestations, signs the image and verifies the remote
manifest and attachments. Durable promotion state supports resuming completed
steps; successful admission alone is not registry publication. Tools, protected
refs, schemas, registry credentials, signing identity and approval digests must
come from the independent release controller.

## Dependency inventory and scanner evidence

[policy.validate_sbom](policy.py) checks a normalized SPDX-2.3 inventory covering
Go, JavaScript and OS packages, including versions and license conclusions.
[sbom.py](sbom.py) implements production-tool invocation for image and JavaScript
inventories, validates documents against a supplied reviewed SPDX schema and
projects package kinds from package URLs. `NOASSERTION` is an unknown conclusion,
not a legal approval; see the [license boundary and notices](../../licenses/README.md).

[scanner.py](scanner.py) invokes the reviewed Trivy tool for the image and source
tree. It checks scanner database bytes, metadata digest and freshness before and
after scanning, and binds findings to independently supplied exposure analysis.
Critical response records require triage within four hours and mitigation within
24 hours when those deadlines have elapsed. These records do not themselves
prove staffing or operational completion. Missing tools, stale databases,
unapproved exceptions or unavailable evidence deny admission.

## Current CI publication path

The monorepo workflow is
[offchain-images.yml](../../../../.github/workflows/offchain-images.yml), with
image rows defined by
[tools/deploy/offchain-images.sh](../../../../tools/deploy/offchain-images.sh).
It runs on relevant pushes to `main` or manual dispatch:

- `plan` checks the configured base-image digest pins and selects rows that pass
  the image table's checks. Failed rows are reported as unbuilt.
- `images` depends on `plan`. Its matrix builds and pushes selected images to
  `ghcr.io/sidiora-labs`, tagged with the 12-character source revision and
  `git describe`. It has `contents: read` and `packages: write`, logs into GHCR
  with the workflow token and uses `push: true`.
- `digests` depends on `plan` and `images`. After a successful plan, when the run
  is not cancelled, it merges available digest artifacts into `digests.json`,
  reports unbuilt/failed rows and rejects an empty result. Missing rows remain
  absent so rollout cannot treat them as published images.

This workflow does not depend on four preceding verification jobs and does not
invoke the six-check release admission policy above. A pushed image or a
`digests.json` entry proves neither policy admission nor deployment approval.
The workflow has no pull-request trigger; its publication authority is scoped
to its push/manual execution. External actions are pinned to full revisions.
No workflow execution or registry publication is performed by this documentation
rewrite.

## Reviewed toolchain and container builds

[ci_tools.py](ci_tools.py) pins Go 1.26.4, Node 24.16.0 and pnpm 9.15.0, the
intentional qualified wave-zero baseline. It verifies reviewed executable hashes
and versions rather than inferring trust from PATH. A leaf module's Go 1.25.0
language directive is not an instruction to downgrade that tool baseline.

[build.py](build.py) requires explicit immutable build/runtime image references,
a service/binary allowlist and a reviewed secret-excluding Docker ignore file.
It builds [Dockerfile.svc](../../../../deploy/launchpad/Dockerfile.svc) from the
monorepo root. The Dockerfile also has digest-pinned defaults; CI supplies its
reviewed image variables explicitly. The build image must contain Python 3 and
Go 1.26.4. The Dockerfile validates the target, checks module manifest integrity,
downloads and verifies modules, disables CGO, VCS metadata and the build ID,
uses trimpath and fixes the source epoch. The runtime runs as UID/GID 65532.
Metadata uses the separate
[native Dockerfile](../../../../deploy/launchpad/Dockerfile.native), not the
ordinary `build.py` target allowlist; its native-library review is separate.
A digest identifies image bytes and does not alone prove a distroless runtime
or license suitability.

[reproduce.py](reproduce.py) implements two clean builds and comparison of the
binary and OCI content. [policy.runtime](policy.py) validates nonroot execution,
a read-only root filesystem, dropped capabilities, default seccomp, resource
limits, a bounded optional temporary filesystem and egress-policy identity.
[confinement.py](confinement.py) applies Docker restrictions on an internal
network, inspects the actual container and probes health, binary identity,
write denial and raw-socket denial. These paths require reviewed tools and real
runtime inputs; a configuration object alone is not confinement evidence.
Deployment owners must apply the admitted restrictions to the deployed service.

## Qualification commands and boundaries

From the repository root, the local test discovery path is:

```sh
python3 -m unittest discover -s services/launchpad/security/supplychain -p 'test_*.py' -v
```

The tests and [check_phase2.py](check_phase2.py) distinguish local policy checks
from external release qualification. Actual qualification additionally requires
approved tool/scanner bytes, schemas, source/image identity, GitHub run evidence,
registry and signing authority, reproducible builds and confinement inputs.
The release measurement checks bind the candidate and raw artifacts; they do
not claim global capacity acceptance. This page does not claim those checks ran.

For adjacent runtime boundaries, see the
[private control primitives](../control/README.md) and
[livestream implementation status](../../livestream/PROGRESS.md).
