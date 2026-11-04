# One paxd BFT network. The shared C/Rust kernel and its typed Go keepers
# are linked into paxd; no standalone core, replica or receipt supervisor.
FROM rust:1.91.1-bookworm@sha256:c1e5f19e773b7878c3f7a805dd00a495e747acbdc76fb2337a4ebf0418896b33 AS build
WORKDIR /src
ARG CARGO_BUILD_JOBS=4
ENV CARGO_BUILD_JOBS=${CARGO_BUILD_JOBS} MAKEFLAGS=-j${CARGO_BUILD_JOBS}
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential libssl-dev libsqlite3-dev pkg-config python3 \
    && rm -rf /var/lib/apt/lists/*
COPY . .
ARG LXP_REVISION
RUN rustup show active-toolchain && make LXP_REVISION="$LXP_REVISION" paxd-native-libs \
    && test -s build/liblayerx.a \
    && test -s programs/target/release/liblayerx_programs_sandbox.a

FROM golang:1.25.8-bookworm@sha256:4557cf171e3cdf5053a298d5171b1a5f5734d920260c25f22c79e94760eb2084 AS paxd-build
WORKDIR /src
ENV CGO_ENABLED=1 GOMAXPROCS=4 GOFLAGS=-p=4
RUN apt-get update && apt-get install -y --no-install-recommends libssl-dev python3 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/ /src/
ARG LXP_REVISION
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    VERSION_PKG=github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version; \
    PAX_CHAIN_VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' tools/release/version.json); \
    test -n "$PAX_CHAIN_VERSION"; \
    go build -mod=readonly -tags 'netgo ledger' \
        -ldflags "-checklinkname=0 -X $VERSION_PKG.Name=paxeer -X $VERSION_PKG.AppName=paxd -X $VERSION_PKG.Version=$PAX_CHAIN_VERSION -X $VERSION_PKG.Commit=$LXP_REVISION -X $VERSION_PKG.BuildTags=netgo,ledger" \
        -o build/paxd ./daemon/paxd; \
    go list -mod=readonly -tags 'netgo ledger' -deps -json ./daemon/paxd > /tmp/paxd-deps.json
RUN python3 - <<'PY_GO_INPUTS'
import json
from pathlib import Path

raw = Path("/tmp/paxd-deps.json").read_text()
decoder = json.JSONDecoder()
inputs = set()
while raw.strip():
    package, end = decoder.raw_decode(raw.lstrip())
    raw = raw.lstrip()[end:]
    directory = Path(package["Dir"])
    if not directory.is_relative_to("/src"):
        continue
    for field in ("GoFiles", "CgoFiles", "CFiles", "CXXFiles", "HFiles", "SFiles", "SysoFiles", "EmbedFiles"):
        inputs.update(str(directory / name) for name in package.get(field, []))
Path("/src/build/paxd-source-inputs.txt").write_text("\n".join(sorted(inputs)) + "\n")
PY_GO_INPUTS
RUN set -eu; \
    mkdir -p /native-libs; \
    cp wasm-runtime/internal/api/libwasmvm.x86_64.so /native-libs/; \
    cp wasm/x/wasm/artifacts/v152/api/libwasmvm152.x86_64.so /native-libs/; \
    cp wasm/x/wasm/artifacts/v155/api/libwasmvm155.x86_64.so /native-libs/; \
    for library in libwasmvm.x86_64.so libwasmvm152.x86_64.so libwasmvm155.x86_64.so; do \
        expected=$(awk -v name="$library" '$2 == name { print $1 }' wasm-runtime/libwasmvm-linux.sha256); \
        test "${#expected}" -eq 64; \
        printf '%s  %s\n' "$expected" "$library" >> /native-libs/libwasmvm.sha256; \
    done; \
    cd /native-libs; \
    sha256sum --check --strict libwasmvm.sha256

FROM paxd-build AS native-manifest
ARG LXP_REVISION
RUN --mount=type=bind,source=.,target=/source,ro \
    python3 - "$LXP_REVISION" <<'PY_MANIFEST'
import glob
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import sys


def refuse(message):
    raise SystemExit("native artifact manifest not produced: " + message)


revision = sys.argv[1]
if re.fullmatch(r"[0-9a-f]{40}", revision) is None or revision == "0" * 40:
    refuse("LXP_REVISION must identify the candidate source")
src = "/src"
source_root = Path("/source")
c_deps = sorted(glob.glob(src + "/build/obj/**/*.d", recursive=True)) + [
    src + "/programs/target/release/liblayerx_programs_sandbox.d",
]
c_extra = ["Makefile", "services/platform/Makefile.inc", "programs/Cargo.toml",
           "programs/Cargo.lock", "programs/.cargo/config.toml", "rust-toolchain.toml"] + sorted(
    os.path.relpath(path, src)
    for pattern in ("tools/build/*", "tools/paxeer-x/build/*")
    for path in glob.glob(os.path.join(src, pattern))
    if os.path.isfile(path)
)
artifacts = {
    "paxd": ("build/paxd", c_deps,
             c_extra + ["go.mod", "go.sum", "tools/release/version.json",
                        "wasm-runtime/libwasmvm-linux.sha256",
                        "wasm-runtime/internal/api/libwasmvm.x86_64.so",
                        "wasm/x/wasm/artifacts/v152/api/libwasmvm152.x86_64.so",
                        "wasm/x/wasm/artifacts/v155/api/libwasmvm155.x86_64.so"],
             src + "/build/paxd-source-inputs.txt"),
}


def dep_paths(dep_file):
    if not os.path.isfile(dep_file):
        refuse("missing dependency file " + dep_file)
    text = Path(dep_file).read_text(encoding="utf-8").replace("\\\n", " ")
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        target, sep, prerequisites = line.partition(": ")
        if not sep:
            if line.rstrip().endswith(":"):
                continue
            refuse("unparsed dependency line in " + dep_file)
        yield from prerequisites.split()


def closure_entry(path, closure):
    real = os.path.realpath(path if os.path.isabs(path) else os.path.join(src, path))
    if not real.startswith(src + "/"):
        return
    relative = os.path.relpath(real, src)
    parts = relative.split("/")
    if parts[0] in ("build", ".lane-target") or "target" in parts:
        return
    candidate = source_root / relative
    if candidate.is_symlink() or not candidate.is_file():
        refuse("dependency " + relative + " is not a regular candidate file")
    if relative not in closure:
        with candidate.open("rb") as stream:
            closure[relative] = hashlib.file_digest(stream, "sha256").hexdigest()


out = Path("/native")
(out / "bin").mkdir(parents=True)
records = {}
digests = {}
for name, (binary, dep_files, extra, listing) in sorted(artifacts.items()):
    closure = {}
    for dep_file in dep_files:
        for path in dep_paths(dep_file):
            closure_entry(path, closure)
    if listing is not None:
        for path in Path(listing).read_text(encoding="utf-8").splitlines():
            if path:
                closure_entry(path, closure)
    for path in extra:
        closure_entry(path, closure)
    if not closure:
        refuse("empty source closure for " + name)
    executable = Path(src) / binary
    if not executable.is_file() or executable.is_symlink() or not os.access(executable, os.X_OK):
        refuse("missing regular executable " + binary)
    shutil.copyfile(executable, out / "bin" / name)
    with (out / "bin" / name).open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    records[name] = {
        "sha256": digest,
        "producer_revision": revision,
        "source_inputs": dict(sorted(closure.items())),
    }
manifest = {"schema": "layerx.native-artifacts.v1", "candidate_revision": revision, "artifacts": records}
(out / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY_MANIFEST

FROM scratch AS native-artifacts
COPY --from=native-manifest /native/ /
COPY --from=paxd-build /native-libs/ /lib/

FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171 AS native-runtime
ARG LXP_REVISION
LABEL org.opencontainers.image.revision="$LXP_REVISION" \
      io.layerx.kernel.profile="native"
ENV LAYERX_KERNEL_PROFILE=native PYTHONDONTWRITEBYTECODE=1
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libssl3 bash coreutils util-linux python3 python3-cryptography \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -g 4020 layerx && useradd -M -u 4020 -g 4020 -s /usr/sbin/nologin layerx
RUN setpriv --help | grep -- --pdeathsig
COPY --from=native-artifacts --chmod=0555 /bin/paxd /usr/local/bin/paxd
COPY --from=native-artifacts --chmod=0555 /lib/ /usr/lib/
RUN cd /usr/lib && sha256sum --check --strict libwasmvm.sha256
RUN --mount=type=bind,from=native-artifacts,target=/native-artifacts,ro \
    --mount=type=bind,source=.,target=/source,ro \
    python3 - "$LXP_REVISION" <<'PY_VERIFY'
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import sys


def refuse(message):
    raise SystemExit("native artifact manifest refused: " + message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            refuse("duplicate JSON field")
        result[key] = value
    return result


def admit_runtime_linkage(executable):
    size = executable.stat().st_size
    if size < 64 or size > 536870912:
        refuse("executable ELF size is outside bounds")
    with executable.open("rb") as stream:
        header = struct.unpack("<16sHHIQQQIHHHHHH", stream.read(64))
        ident, kind, machine, version = header[:4]
        if ident[:7] != b"\x7fELF\x02\x01\x01" or kind not in (2, 3) or machine != 62 or version != 1:
            refuse("executable must be a little-endian x86-64 ELF")
        table, entry_size, count = header[5], header[9], header[10]
        if header[8] != 64 or entry_size != 56 or not 1 <= count <= 1024:
            refuse("unsupported ELF program-header layout")
        if table < 64 or table + count * entry_size > size:
            refuse("ELF program-header table is outside the file")
        interpreter = None
        dynamic_segments = []
        for index in range(count):
            stream.seek(table + index * entry_size)
            segment = struct.unpack("<IIQQQQQQ", stream.read(entry_size))
            segment_kind, offset, length = segment[0], segment[2], segment[5]
            if offset > size or length > size - offset:
                refuse("ELF segment is outside the file")
            if segment_kind == 3:
                if interpreter is not None or not 2 <= length <= 4096:
                    refuse("invalid ELF interpreter segment")
                stream.seek(offset)
                value = stream.read(length)
                if value[-1:] != b"\0" or b"\0" in value[:-1]:
                    refuse("invalid ELF interpreter string")
                try:
                    interpreter = value[:-1].decode("ascii")
                except UnicodeDecodeError:
                    refuse("ELF interpreter is not ASCII")
            elif segment_kind == 2:
                if length == 0 or length % 16 or length > 1048576:
                    refuse("invalid ELF dynamic segment")
                dynamic_segments.append((offset, length))
        if len(dynamic_segments) > 1:
            refuse("multiple ELF dynamic segments")
        needed = False
        for offset, length in dynamic_segments:
            stream.seek(offset)
            terminated = False
            for _ in range(length // 16):
                tag, value = struct.unpack("<qQ", stream.read(16))
                if tag == 0:
                    terminated = True
                    break
                if tag == 1:
                    needed = True
            if not terminated:
                refuse("unterminated ELF dynamic segment")
        if interpreter is None:
            if needed:
                refuse("ELF dependencies have no runtime interpreter")
            return
        loader = "/lib64/ld-linux-x86-64.so.2"
        if interpreter != loader:
            refuse("ELF interpreter differs from the pinned runtime loader")
    try:
        result = subprocess.run(
            [loader, "--list", str(executable)],
            capture_output=True, text=True, timeout=30, check=False,
            env={"PATH": "/usr/bin:/bin", "LC_ALL": "C"},
        )
    except (OSError, subprocess.TimeoutExpired):
        refuse("runtime loader admission failed for " + executable.name)
    if result.returncode != 0 or result.stderr.strip() or "not found" in result.stdout:
        sys.stderr.write(result.stdout)
        sys.stderr.write(result.stderr)
        refuse("runtime dependency mismatch for " + executable.name)


revision = sys.argv[1]
if re.fullmatch(r"[0-9a-f]{40}", revision) is None or revision == "0" * 40:
    refuse("LXP_REVISION must identify the candidate source")
manifest_path = Path("/native-artifacts/manifest.json")
if not manifest_path.is_file() or manifest_path.is_symlink():
    refuse("manifest must be a regular file")
if manifest_path.stat().st_size > 33554432:
    refuse("manifest exceeds 32 MiB")
manifest = json.loads(manifest_path.read_text(encoding="utf-8"), object_pairs_hook=unique_object)
if not isinstance(manifest, dict) or manifest.get("schema") != "layerx.native-artifacts.v1":
    refuse("unsupported schema")
if manifest.get("candidate_revision") != revision:
    refuse("candidate revision mismatch")
required = {"paxd"}
artifacts = manifest.get("artifacts")
if not isinstance(artifacts, dict) or set(artifacts) != required:
    refuse("artifact set differs from the native profile")
source_root = Path("/source").resolve()
verified_sources = {}
for name, record in artifacts.items():
    if not isinstance(record, dict):
        refuse("invalid artifact record")
    expected = record.get("sha256")
    producer = record.get("producer_revision")
    if not isinstance(expected, str) or re.fullmatch(r"[0-9a-f]{64}", expected) is None:
        refuse("invalid executable digest")
    if not isinstance(producer, str) or re.fullmatch(r"[0-9a-f]{40}", producer) is None or producer == "0" * 40:
        refuse("missing original producer revision")
    executable = Path("/usr/local/bin") / name
    if not executable.is_file() or executable.is_symlink() or not os.access(executable, os.X_OK):
        refuse("missing regular executable " + name)
    with executable.open("rb") as stream:
        actual = hashlib.file_digest(stream, "sha256").hexdigest()
    if actual != expected:
        refuse("executable digest mismatch for " + name)
    admit_runtime_linkage(executable)
    inputs = record.get("source_inputs")
    if not isinstance(inputs, dict) or not inputs:
        refuse("missing producer source closure for " + name)
    for relative, digest in inputs.items():
        path = PurePosixPath(relative)
        if path.is_absolute() or not path.parts or any(part in (".", "..") for part in path.parts) or str(path) != relative:
            refuse("invalid source input path")
        if not isinstance(digest, str) or re.fullmatch(r"[0-9a-f]{64}", digest) is None:
            refuse("invalid source input digest")
        source = source_root / path
        if source.is_symlink() or not source.is_file() or not source.resolve().is_relative_to(source_root):
            refuse("source input is not a regular candidate file")
        if relative not in verified_sources:
            with source.open("rb") as stream:
                verified_sources[relative] = hashlib.file_digest(stream, "sha256").hexdigest()
        if verified_sources[relative] != digest:
            refuse("producer source differs from candidate for " + name)
PY_VERIFY
COPY services/platform/hosted/node/native_runtime.py services/platform/hosted/node/data_directory.py \
     services/platform/hosted/node/reset_state.py /opt/layerx/node/
COPY docker/kernel/kernel_runtime.py /opt/layerx/kernel_runtime.py
COPY --chmod=0555 docker/kernel/init.sh /usr/local/bin/kernel-init
EXPOSE 8545 8546 26656 9460
ENTRYPOINT ["/usr/local/bin/kernel-init"]

# The historical profile names select the same merged node assembly.
FROM native-runtime AS full-runtime
ENV LAYERX_KERNEL_PROFILE=full
LABEL io.layerx.kernel.profile="full"
ENTRYPOINT ["/usr/local/bin/kernel-init"]
