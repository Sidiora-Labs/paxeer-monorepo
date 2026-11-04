# Paxeer node operations (`seictl`)

This is the audited Paxeer adaptation of paxctl, vendored in the Paxeer
monorepo. The executable is still named `seictl`; the Go import path, `SEI_*`
environment variables, `sei.io/v1alpha1` CRDs and annotation keys retain their
inherited spelling. These names do not select the chain. Use an explicit Paxeer
chain ID, home, pinned `paxd` image and isolated Kubernetes context.
[VENDOR.kvx](VENDOR.kvx) records the upstream import, not the revision of the
subsequent monorepo repairs.

## Installation and provenance

Use a qualified Paxeer distribution from
[Paxeer-Network/paxctl](https://github.com/Paxeer-Network/paxctl/releases), pinned
to a reviewed release tag and its source revision. Do not substitute an upstream
Sei release or an unpinned Go module installation for this adapted tool. If no
qualified artifact exists for the desired monorepo revision, build that revision
from the complete source tree below.

| Native OS | CPU | Archive | Executable |
| --- | --- | --- | --- |
| Linux | amd64 | `seictl_Linux_x86_64.tar.gz` | `seictl` |
| Linux | arm64 | `seictl_Linux_arm64.tar.gz` | `seictl` |
| macOS | amd64 | `seictl_Darwin_x86_64.tar.gz` | `seictl` |
| macOS | arm64 | `seictl_Darwin_arm64.tar.gz` | `seictl` |

Windows and ARMv7 archives are unsupported. Containers target Linux amd64 and
arm64, use a distroless runtime, and must be pinned by a qualified image digest.
The archive checksum is mandatory; get `checksums.txt` from the same release.
For example, after substituting a reviewed public tag:

```sh
RELEASE_TAG='<reviewed-v-prefixed-tag>'
RELEASE_BASE="https://github.com/Paxeer-Network/paxctl/releases/download/$RELEASE_TAG"
curl --fail --location --remote-name "$RELEASE_BASE/seictl_Linux_x86_64.tar.gz"
curl --fail --location --remote-name "$RELEASE_BASE/checksums.txt"
# Select the exact archive entry, then verify it before extracting.
awk '$2 == "seictl_Linux_x86_64.tar.gz" {print; found=1} END {if (!found) exit 1}' checksums.txt > selected-checksum.txt
sha256sum --check selected-checksum.txt
tar -xzf seictl_Linux_x86_64.tar.gz
./seictl --help
```

Source prerequisites are Go 1.26.0 (as declared by `go.mod`), Git and Make.
The source tree must include the sibling modules referenced by `go.mod`'s local
`replace` directives. A standalone copy of this directory is insufficient.

```sh
SOURCE_REVISION='<reviewed-full-monorepo-commit-sha>'
git clone https://github.com/Sidiora-Labs/paxeer-monorepo.git
cd paxeer-monorepo
git checkout --detach "$SOURCE_REVISION"
cd tools/paxctl
make build
./build/seictl --help
# Optional local installation into Go's bin directory:
make install
```

`make build` reads and validates `version.json`, stamps
`github.com/sei-protocol/seictl/internal/cliutil.Version`, and checks the real
network, node and workflow provenance annotations through a loopback API
fixture. A bare `go build` leaves the annotation at `dev` and is not a release.
There is no registered `--version` flag; use the provenance verifier with the
matching pinned source and the extracted native executable:

```sh
go run ./internal/release verify --expected "$RELEASE_TAG" --binary /absolute/path/to/seictl
```

GoReleaser requires the release tag to equal the validated `version.json` value
(including its `v` prefix). The native verification workflow checks all four
archives on matching OS/CPU runners. Docker verifies the linked executable in
its build stage on each target architecture. Build the image from the monorepo
root so local Go replacements resolve:

```sh
docker build -f tools/paxctl/Dockerfile -t paxctl:local .
```

The `seictl.sei.io/version` annotation is diagnostic provenance. Anyone allowed
to edit a CR can forge it; it is not an authorization or integrity boundary.

## Commands and prerequisites

| Surface | Actual commands | Requirements |
| --- | --- | --- |
| Genesis network | `network apply/get/list/delete/watch` | `SeiNetwork`, `genesis-chain` preset, explicit chain ID and paxd image |
| Follower/RPC node | `node apply/get/list/delete/watch` | `SeiNode`, `rpc` preset, explicit chain ID, image and peer selection |
| Controller recipe | `workflow apply/get/list/delete/watch`, `workflow state-sync` | `SeiNodeTaskWorkflow`, an adapted controller and target node |
| Sidecar tasks | `task submit/get/list/delete`, `task snapshot-upload` | Pod proxy DNS connectivity, Kubernetes credentials, proxy RBAC and `--proxy-ca-file` |
| Local editing | `patch`, `config patch`, `genesis patch` | Explicit `--home` or `--target`; review output before rewriting |
| Other inherited tools | `serve`, `await`, `report divergence/list` | Qualified process, RPC and storage configuration for the selected Paxeer deployment |

Kubernetes verbs use `--kubeconfig` (or `KUBECONFIG`) and `-n`; credentials need
RBAC for the selected namespace/resources. `apply --dry-run` performs real
server-side validation and still requires Kubernetes access. Network readiness
is `Ready`; node readiness is `Running` (or the `caught-up` watch sentinel).
Workflow `Complete` means its controller plan finished; inspect `.status.plan`
on failure. `workflow state-sync` is destructive: use only a disposable follower
after reviewing [workflow recovery guidance](workflow/README.md).

Task verbs connect to
`https://<node>-0.<node>.<namespace>.svc.cluster.local:8443`. Supply the proxy's
CA with `--proxy-ca-file`; the Kubernetes API CA is not automatically proxy
trust. The credential must pass the proxy's TokenReview and SubjectAccessReview.
The proxy must overwrite `X-Remote-User` before forwarding to the sidecar's
loopback listener. A direct request with a self-chosen header is not a supported
remote authentication method. Run task commands from a network that resolves
and reaches that pod DNS name.

`serve` defaults to `SEI_SIDECAR_AUTHN_MODE=trusted-header`, listening on
`127.0.0.1:7777` (override port with `--port` or `SEI_SIDECAR_PORT`). It refuses
unauthenticated mode because it installs signing and destructive handlers.
Required environment: `SEI_CHAIN_ID`, `SEI_GENESIS_BUCKET`, `SEI_GENESIS_REGION`,
`SEI_SNAPSHOT_BUCKET`, `SEI_SNAPSHOT_REGION`. A keyring is optional unless signing:
`SEI_KEYRING_BACKEND=file` requires `SEI_KEYRING_PASSPHRASE`; `SEI_KEYRING_DIR`
selects its directory. Keep credentials in deployment secret mechanisms.

Local patch commands honor `--home` / `SEI_HOME`. No automatic `~/.sei` default
is configured by the root command; pass a disposable Paxeer home explicitly.
`serve` falls back to `/sei` if home is omitted. Generic editing examples:

```sh
seictl --home /tmp/paxeer-smoke config --target config patch patch.toml
seictl --home /tmp/paxeer-smoke genesis patch patch.json
seictl patch --target /tmp/paxeer-smoke/config/config.toml patch.toml
```

Output defaults to stdout; `-o` selects a file and `-i` rewrites in place.
Genesis patching edits JSON; it does not certify genesis validity or chain
identity. Do not reuse upstream fee-denomination, address-prefix, AWS inventory,
image, genesis or validator-key examples. Inherited `restart-seid`/`stop-seid`
task identifiers remain wire names: deployment process discovery must target
`paxd`. Sei-only onboarding/load-benchmark operations are unsupported; `onboard`,
`bench`, and the retired node-deployment resource are not registered. Snapshot,
genesis ceremony, governance signing and state-sync operations require Paxeer
chain/process/controller qualification before use; the existence of a command
is not a compatibility guarantee.

Follow the [isolated smoke procedure](.agent/runbooks/seictl-smoke.md) for an
explicit chain-identity check and non-destructive command validation.
