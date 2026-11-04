#!/usr/bin/env bash
# Offline packaging check for the x-websearch sidecar. It reads the text of the
# sidecar image Dockerfile, the systemd unit, the configuration example, the
# compose entry and the node image Dockerfile and asserts the pinned bases, the
# non-root user, the unit's user, key-file environment and sandbox directives,
# the compose service and the node image's copy of the binary. It then runs the
# locally built x-websearch loader against the configuration example and
# proves that every placeholder in it is refused, naming the field. No image
# is built, pulled or pushed unless X_WEBSEARCH_CHECK_IMAGE=1 asks for a build
# of the sidecar image alone from base images already present locally.
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
DEPLOY_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../../../../.." && pwd)
IMAGE_DOCKERFILE="$REPO_ROOT/docker/x-websearch/Dockerfile"
UNIT="$DEPLOY_DIR/x-websearch.service"
EXAMPLE="$DEPLOY_DIR/config.example.json"
COMPOSE="$REPO_ROOT/docker/docker-compose.yml"
NODE_DOCKERFILE="$REPO_ROOT/docker/layerx/Dockerfile"
VALID_FIXTURE="$REPO_ROOT/services/interop/crates/x-websearch/tests/fixtures/config/valid.json"
FLY_INIT="$REPO_ROOT/docker/x-websearch/entrypoint.sh"

RUST_BASE='rust:1.91.1-bookworm@sha256:c1e5f19e773b7878c3f7a805dd00a495e747acbdc76fb2337a4ebf0418896b33'
DEBIAN_BASE='debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171'
BUILD_LINE='cargo build --locked --manifest-path services/interop/Cargo.toml --release --package x-websearch --bin x-websearch'
KEY_VARIABLES=(X_WEBSEARCH_ATTESTOR_KEY_FILE X_WEBSEARCH_SUBMITTER_KEY_FILE X_WEBSEARCH_RECEIVER_KEY_FILE)
KEY_NAMES=(attestor submitter receiver)

fail() { printf 'packaging-check: error: %s\n' "$*" >&2; exit 1; }
pass() { printf 'packaging-check: ok: %s\n' "$*"; }

for file in "$IMAGE_DOCKERFILE" "$UNIT" "$EXAMPLE" "$COMPOSE" "$NODE_DOCKERFILE" "$VALID_FIXTURE" "$FLY_INIT"; do
    [ -f "$file" ] || fail "missing $file"
done

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
chmod 0700 "$WORK"

# has_line TEXT LINE WHAT: TEXT holds LINE as a whole line.
has_line() {
    grep -Fxq -- "$2" <<<"$1" || fail "$3: expected the line '$2'"
}

# lacks_match TEXT PATTERN WHAT: no line of TEXT matches the extended regex.
lacks_match() {
    if grep -Eq -- "$2" <<<"$1"; then
        fail "$3: unexpected line matching '$2'"
    fi
}

# final_stage FILE: the lines after the last FROM.
final_stage() {
    awk '/^FROM /{text=""; next} {text=text $0 "\n"} END{printf "%s", text}' "$1"
}

# named_stage FILE NAME: the lines of the stage FROM ... AS NAME, FROM included.
named_stage() {
    awk -v name="$2" '
        /^FROM / { inside = ($NF == name && $(NF-1) == "AS") }
        inside { print }
    ' "$1"
}

# every_from_pinned FILE: every FROM names an exact version tag and a digest.
every_from_pinned() {
    local count=0 line
    while IFS= read -r line; do
        count=$((count + 1))
        [[ $line =~ ^FROM\ [a-z0-9./-]+:[A-Za-z0-9][A-Za-z0-9._-]*@sha256:[0-9a-f]{64}(\ AS\ [a-z0-9-]+)?$ ]] \
            || fail "$1: base image not pinned to an exact tag and digest: $line"
        [[ $line != *:latest@* ]] || fail "$1: base image pinned to the moving tag latest: $line"
    done < <(grep -E '^FROM ' "$1")
    [ "$count" -ge 2 ] || fail "$1: expected a build stage and a runtime stage"
}

# ---------------------------------------------------------------- sidecar image
every_from_pinned "$IMAGE_DOCKERFILE"
IMAGE_TEXT=$(cat "$IMAGE_DOCKERFILE")
has_line "$IMAGE_TEXT" "FROM $RUST_BASE AS build" "sidecar image build base"
has_line "$IMAGE_TEXT" "FROM $DEBIAN_BASE" "sidecar image runtime base"
# Check the supported targets by identity, including the unnamed default target.
[ "$(grep -E '^FROM ' "$IMAGE_DOCKERFILE" | sed -n '1p')" = "FROM $RUST_BASE AS build" ] || fail "sidecar image: first target must be build"
[ "$(grep -E '^FROM ' "$IMAGE_DOCKERFILE" | sed -n '2p')" = "FROM $DEBIAN_BASE AS fly" ] || fail "sidecar image: second target must be fly"
[ "$(grep -E '^FROM ' "$IMAGE_DOCKERFILE" | tail -n 1)" = "FROM $DEBIAN_BASE" ] || fail "sidecar image: final target must be the default runtime"
FLY_STAGE=$(named_stage "$IMAGE_DOCKERFILE" fly)
[ -n "$FLY_STAGE" ] || fail "sidecar image: no fly target"
has_line "$FLY_STAGE" "COPY --from=build /src/.x-websearch-target/release/x-websearch /usr/local/bin/x-websearch" "fly binary"
has_line "$FLY_STAGE" "COPY docker/x-websearch/entrypoint.sh /usr/local/bin/x-websearch-fly-init" "fly init"
has_line "$FLY_STAGE" 'ENTRYPOINT ["/usr/local/bin/x-websearch-fly-init"]' "fly entrypoint"
has_line "$FLY_STAGE" 'CMD ["--config", "/data/config.json"]' "fly configuration"
has_line "$FLY_STAGE" 'EXPOSE 8480' "fly port"
has_line "$FLY_STAGE" "    && apt-get install --yes --no-install-recommends ca-certificates libssl3 socat \\" "fly runtime packages"
has_line "$FLY_STAGE" "    && groupadd --system --gid 65532 x-websearch \\" "fly group"
has_line "$FLY_STAGE" "    && useradd --system --uid 65532 --gid 65532 --home-dir /nonexistent --shell /usr/sbin/nologin x-websearch" "fly user"
# Fly init needs root to hand over the volume; every long-lived child drops it.
FLY_INIT_TEXT=$(cat "$FLY_INIT")
has_line "$FLY_INIT_TEXT" 'drop=(setpriv --reuid=65532 --regid=65532 --clear-groups --inh-caps=-all --no-new-privs)' "fly privilege drop"
has_line "$FLY_INIT_TEXT" '	"${drop[@]}" socat "TCP-LISTEN:${hop%%=*},bind=127.0.0.1,reuseaddr,fork" "TCP6:${hop#*=}:8480" &' "fly unprivileged proxy"
has_line "$FLY_INIT_TEXT" '"${drop[@]}" /usr/local/bin/x-websearch "$@" &' "fly unprivileged sidecar"
for target in "$FLY_STAGE" "$(final_stage "$IMAGE_DOCKERFILE")"; do
    lacks_match "$target" 'X_WEBSEARCH_[A-Z]+_KEY_FILE|\.key( |$)|config\.json /' "deployable target carries no secrets"
done
BUILD_STAGE=$(named_stage "$IMAGE_DOCKERFILE" build)
has_line "$BUILD_STAGE" "ENV CARGO_BUILD_JOBS=4 CARGO_TARGET_DIR=/src/.x-websearch-target" "sidecar image build stage"
has_line "$BUILD_STAGE" "RUN $BUILD_LINE" "sidecar image build stage"
RUNTIME_STAGE=$(final_stage "$IMAGE_DOCKERFILE")
has_line "$RUNTIME_STAGE" "    && apt-get install --yes --no-install-recommends ca-certificates libssl3 \\" "sidecar image runtime packages"
has_line "$RUNTIME_STAGE" "    && groupadd --system --gid 65532 x-websearch \\" "sidecar image group"
has_line "$RUNTIME_STAGE" "    && useradd --system --uid 65532 --gid 65532 --home-dir /nonexistent --shell /usr/sbin/nologin x-websearch \\" "sidecar image user"
has_line "$RUNTIME_STAGE" "    && chown 65532:65532 /var/lib/x-websearch \\" "sidecar image data directory owner"
has_line "$RUNTIME_STAGE" "COPY --from=build /src/.x-websearch-target/release/x-websearch /usr/local/bin/x-websearch" "sidecar image binary"
has_line "$RUNTIME_STAGE" 'ENTRYPOINT ["/usr/local/bin/x-websearch"]' "sidecar image entrypoint"
has_line "$RUNTIME_STAGE" 'CMD ["--config", "/etc/x-websearch/config.json"]' "sidecar image command"
has_line "$RUNTIME_STAGE" 'EXPOSE 8480' "sidecar image port"
LAST_USER=$(grep -E '^USER ' <<<"$RUNTIME_STAGE" | tail -n 1)
[ "$LAST_USER" = "USER 65532:65532" ] || fail "sidecar image: the runtime user is '$LAST_USER', expected USER 65532:65532"
lacks_match "$IMAGE_TEXT" '^USER (root|0)(:|$)' "sidecar image"
lacks_match "$IMAGE_TEXT" 'X_WEBSEARCH_[A-Z]+_KEY_FILE|\.key( |$)|config\.json /' "sidecar image carries no key or configuration file"
pass "sidecar image: pinned bases, release build of x-websearch, non-root user 65532"

# ---------------------------------------------------------------- systemd unit
SERVICE=$(awk '/^\[/{inside = ($0 == "[Service]"); next} inside && NF {print}' "$UNIT")
[ -n "$SERVICE" ] || fail "unit: no [Service] section"
has_line "$(cat "$UNIT")" "WantedBy=multi-user.target" "unit install target"
for directive in \
    "Type=simple" \
    "User=x-websearch" \
    "Group=x-websearch" \
    "ExecStart=/usr/local/bin/x-websearch --config /etc/x-websearch/config.json" \
    "StateDirectory=x-websearch" \
    "StateDirectoryMode=0700" \
    "UMask=0077" \
    "NoNewPrivileges=yes" \
    "ProtectSystem=strict" \
    "ProtectHome=yes" \
    "PrivateTmp=yes" \
    "PrivateDevices=yes" \
    "ProtectKernelTunables=yes" \
    "ProtectKernelModules=yes" \
    "ProtectKernelLogs=yes" \
    "ProtectControlGroups=yes" \
    "ProtectClock=yes" \
    "ProtectHostname=yes" \
    "ProtectProc=invisible" \
    "ProcSubset=pid" \
    "RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX" \
    "RestrictNamespaces=yes" \
    "RestrictRealtime=yes" \
    "RestrictSUIDSGID=yes" \
    "LockPersonality=yes" \
    "MemoryDenyWriteExecute=yes" \
    "RemoveIPC=yes" \
    "SystemCallArchitectures=native" \
    "SystemCallFilter=@system-service" \
    "SystemCallFilter=~@privileged @resources" \
    "CapabilityBoundingSet=" \
    "AmbientCapabilities="; do
    has_line "$SERVICE" "$directive" "unit sandbox"
done
lacks_match "$SERVICE" '^(User|Group)=(root|0)$' "unit"
lacks_match "$SERVICE" '^(DynamicUser|PermissionsStartOnly)=' "unit"
lacks_match "$SERVICE" '^ExecStart=[+!]' "unit runs ExecStart with full privileges"
for index in 0 1 2; do
    has_line "$SERVICE" "LoadCredential=${KEY_NAMES[$index]}.key:/etc/x-websearch/keys/${KEY_NAMES[$index]}.key" "unit key credential"
    has_line "$SERVICE" "Environment=${KEY_VARIABLES[$index]}=%d/${KEY_NAMES[$index]}.key" "unit key-file environment"
done
[ "$(grep -c '^Environment=' <<<"$SERVICE")" -eq 3 ] || fail "unit: expected exactly the three key-file Environment lines"
lacks_match "$SERVICE" '^EnvironmentFile=' "unit"
pass "unit: dedicated user, three key-file variables from credentials, hardened sandbox"

# ------------------------------------------------------------- compose service
SERVICES=$(awk '/^[^ #]/{inside = ($0 == "services:"); next} inside {print}' "$COMPOSE")
for node in node0 node1 node2 node3; do
    has_line "$SERVICES" "  $node:" "compose node service"
done
COMPOSE_ENTRY=$(awk '
    /^  [^ #]/ { inside = ($0 == "  x-websearch:") }
    /^[^ #]/ { inside = 0 }
    inside { print }
' "$COMPOSE")
[ -n "$COMPOSE_ENTRY" ] || fail "compose: no x-websearch service"
grep -Fxq "  x-websearch:" <<<"$SERVICES" || fail "compose: x-websearch is not under services"
for line in \
    '    image: "pax-chain/x-websearch"' \
    '    build:' \
    '      context: ..' \
    '      dockerfile: docker/x-websearch/Dockerfile' \
    "    user: \"\${USERID}:\${GROUPID}\"" \
    '    command: ["--config", "/etc/x-websearch/config.json"]' \
    '      - "8480:8480"' \
    "      - \"\${X_WEBSEARCH_HOME}/config.json:/etc/x-websearch/config.json:ro,Z\"" \
    "      - \"\${X_WEBSEARCH_HOME}/keys:/run/x-websearch/keys:ro,Z\"" \
    "      - \"\${X_WEBSEARCH_HOME}/data:/var/lib/x-websearch:Z\"" \
    '    read_only: true' \
    '      - no-new-privileges:true' \
    '    cap_drop:' \
    '      - ALL' \
    '      - localnet'; do
    has_line "$COMPOSE_ENTRY" "$line" "compose x-websearch service"
done
for index in 0 1 2; do
    has_line "$COMPOSE_ENTRY" "      - ${KEY_VARIABLES[$index]}=/run/x-websearch/keys/${KEY_NAMES[$index]}.key" "compose key-file environment"
done
lacks_match "$COMPOSE_ENTRY" 'privileged|network_mode|pid:|cap_add' "compose x-websearch service"
[ -f "$REPO_ROOT/docker/../docker/x-websearch/Dockerfile" ] \
    || fail "compose: the build context and Dockerfile path do not resolve from docker/"
pass "compose: x-websearch service beside node0 to node3, built from the sidecar Dockerfile"

# ------------------------------------------------------------------ node image
every_from_pinned "$NODE_DOCKERFILE"
NODE_STAGE=$(named_stage "$NODE_DOCKERFILE" x-websearch)
[ -n "$NODE_STAGE" ] || fail "node image: no x-websearch stage"
has_line "$NODE_STAGE" "FROM docker.io/$RUST_BASE AS x-websearch" "node image x-websearch stage base"
has_line "$NODE_STAGE" "ENV CARGO_BUILD_JOBS=4 CARGO_TARGET_DIR=/src/.x-websearch-target" "node image x-websearch stage"
has_line "$NODE_STAGE" "RUN $BUILD_LINE" "node image x-websearch stage"
NODE_RUNTIME=$(final_stage "$NODE_DOCKERFILE")
has_line "$NODE_RUNTIME" "COPY --from=x-websearch /src/.x-websearch-target/release/x-websearch /usr/bin/x-websearch" "node image copy of the binary"
has_line "$NODE_RUNTIME" "    apt-get install -y --no-install-recommends ca-certificates libssl3t64 && \\" "node image runtime packages"
has_line "$NODE_RUNTIME" "COPY --from=builder /go/bin/paxd /usr/bin/" "node image node binary"
has_line "$NODE_RUNTIME" 'ENTRYPOINT ["/usr/bin/paxd"]' "node image entrypoint"
pass "node image: pinned x-websearch stage and the binary copied to /usr/bin/x-websearch"

# ---------------------------------------------------------- configuration example
: "${CARGO_TARGET_DIR:=$REPO_ROOT/services/interop/target}"
export CARGO_TARGET_DIR
cargo build --locked --manifest-path "$REPO_ROOT/services/interop/Cargo.toml" \
    --package x-websearch --bin x-websearch >&2
LOADER="$CARGO_TARGET_DIR/debug/x-websearch"
[ -x "$LOADER" ] || fail "x-websearch was not built at $LOADER"

python3 - "$LOADER" "$EXAMPLE" "$VALID_FIXTURE" "$WORK" "${KEY_VARIABLES[@]}" <<'PY'
import copy
import json
import os
import subprocess
import sys

loader, example_path, valid_path, work = sys.argv[1:5]
key_variables = sys.argv[5:]


def fail(message):
    sys.stderr.write(f"packaging-check: error: {message}\n")
    sys.exit(1)


with open(example_path, encoding="utf-8") as handle:
    example = json.load(handle)
with open(valid_path, encoding="utf-8") as handle:
    valid = json.load(handle)

note = example.get("note")
if not isinstance(note, str):
    fail("config example: no note field")
for phrase in ("one tenth of a US cent", "at the time of configuration", *key_variables):
    if phrase not in note:
        fail(f"config example: the note does not state '{phrase}'")
if example["fetch"]["allow_loopback"] is not False:
    fail("config example: fetch.allow_loopback is not false")
if sorted(example["assets"]) != ["PAX", "SID", "USDC", "USDL"]:
    fail("config example: the assets are not exactly SID, PAX, USDC and USDL")
if example["data_dir"] != "/var/lib/x-websearch":
    fail("config example: data_dir is not the unit's and the image's /var/lib/x-websearch")
if not example["listen"].endswith(":8480"):
    fail("config example: listen does not use the image's port 8480")

environment = {name: value for name, value in os.environ.items() if name not in key_variables}


def run(config, step):
    path = os.path.join(work, f"config-{step}.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(config, handle)
    result = subprocess.run(
        [loader, "--config", path],
        env=environment,
        stdin=subprocess.DEVNULL,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    return result.returncode, result.stderr.strip()


def expect(config, step, message):
    code, stderr = run(config, step)
    if code != 2 or stderr != message:
        fail(f"step {step}: expected exit 2 and '{message}', got exit {code} and '{stderr}'")
    print(f"packaging-check: ok: loader refused: {message}")


def pointer_get(document, pointer):
    for part in pointer.strip("/").split("/"):
        document = document[int(part) if isinstance(document, list) else part]
    return document


def pointer_set(document, pointer, value):
    parts = pointer.strip("/").split("/")
    for part in parts[:-1]:
        document = document[int(part) if isinstance(document, list) else part]
    document[int(parts[-1]) if isinstance(document, list) else parts[-1]] = copy.deepcopy(value)


# Keep the valid producer fixture as the baseline for each independent refusal.
# Numeric placeholders are invalid in the production loader; zero chain_id
# is a placeholder. Do not turn either refusal into the other.
def placeholder_cases(example):
    cases = [("/seeds", "seeds", "a placeholder")]
    for symbol in ("SID", "PAX", "USDC", "USDL"):
        for field in ("asset_id", "price"):
            cases.append((f"/assets/{symbol}/{field}", f"assets.{symbol}.{field}", "a placeholder"))
    cases.append(("/gateway/endpoint", "gateway.endpoint", "a placeholder"))
    for index, entry in enumerate(example["gateway"]["receipt_keys"]):
        assert set(entry) == {"key_id", "public_key", "first_height", "last_height"}
        for field in ("key_id", "public_key", "first_height", "last_height"):
            refusal = "invalid" if field.endswith("height") else "a placeholder"
            cases.append((f"/gateway/receipt_keys/{index}/{field}",
                          f"gateway.receipt_keys[{index}].{field}", refusal))
    cases.extend([("/evm/endpoint", "evm.endpoint", "a placeholder"),
                  ("/evm/chain_id", "evm.chain_id", "a placeholder")])
    return cases


placeholders = placeholder_cases(example)
baseline = copy.deepcopy(valid)
baseline.pop("kernel_network_id", None)
expect(baseline, "valid", "key refused: X_WEBSEARCH_RECEIVER_KEY_FILE is not set")
for step, (pointer, field, refusal) in enumerate(placeholders):
    config = copy.deepcopy(baseline)
    pointer_set(config, pointer, pointer_get(example, pointer))
    expect(config, step, f"configuration refused: {field} is {refusal}")

# Also prove that filling every real example field yields a valid deployment.
config = copy.deepcopy(example)
for pointer, _, _ in placeholders:
    pointer_set(config, pointer, pointer_get(valid, pointer))
expect(config, "note", "configuration refused: note is not a known field")
del config["note"]
expect(config, "filled", "key refused: X_WEBSEARCH_RECEIVER_KEY_FILE is not set")

PY
pass "config example: every placeholder and the note are refused by the built loader, allow_loopback is false"

# ------------------------------------------------ optional sidecar image build
if [ "${X_WEBSEARCH_CHECK_IMAGE:-0}" != "1" ]; then
    pass "sidecar image build skipped: set X_WEBSEARCH_CHECK_IMAGE=1 to build it from local base images"
elif ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    pass "sidecar image build skipped: no container daemon is available"
elif ! docker image inspect "$RUST_BASE" >/dev/null 2>&1 || ! docker image inspect "$DEBIAN_BASE" >/dev/null 2>&1; then
    pass "sidecar image build skipped: the pinned base images are not present locally and this check never pulls"
else
    docker build --pull=false --file "$IMAGE_DOCKERFILE" --tag x-websearch-image:check "$REPO_ROOT" >&2 \
        || fail "the sidecar image build failed"
    pass "sidecar image built as x-websearch-image:check"
fi

pass "x-websearch packaging"
