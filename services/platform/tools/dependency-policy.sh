#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
metadata_file=$(mktemp)
license_file=$(mktemp)
trap 'rm -f -- "$metadata_file" "$license_file"' EXIT HUP INT TERM

case "$#" in
    0)
        cargo metadata --manifest-path "$repo_root/services/platform/Cargo.toml" \
            --locked --format-version 1 >"$metadata_file"
        cargo metadata --manifest-path "$repo_root/tools/platform/Cargo.toml" \
            --locked --format-version 1 >>"$metadata_file"
        cargo metadata --manifest-path "$repo_root/sdk/platform/generators/Cargo.toml" \
            --locked --format-version 1 >>"$metadata_file"
        ;;
    1)
        cat -- "$1" >"$metadata_file"
        ;;
    *)
        echo "usage: dependency-policy.sh [metadata.json]" >&2
        exit 2
        ;;
esac

# Retired crate names remain explicit refusal rules, including in the ramp graph below.
banned='^(bindgen|sqlx-sqlite|layerx-core|layerx-protocol-core|layerx-paxeer-client|ctor|inventory)$'
banned_packages=$(jq -r --arg banned "$banned" \
    '[.packages[].name | select(test($banned))] | join("\n")' "$metadata_file")
if [ -n "$banned_packages" ]; then
    printf 'platform dependency policy: forbidden boundary or global-initialisation crate\n%s\n' "$banned_packages" >&2
    exit 1
fi

forbidden_ramp_dependencies=$(jq -r '
    def reachable($nodes):
        {pending: ., seen: {}}
        | until(.pending | length == 0;
            .pending[0] as $id
            | .pending = .pending[1:]
            | if .seen[$id] then .
              else .seen[$id] = true | .pending += $nodes[$id].dependencies
              end)
        | .seen | keys[];
    INDEX(.packages[]; .id) as $packages
    | INDEX(.resolve.nodes[]; .id) as $nodes
    | .packages[]
    | select(.name == "layerx-ramp-toolkit" or .name == "layerx-reference-ramp")
    | .name as $ramp
    | [.id] | reachable($nodes)
    | $packages[.].name
    | select(. == "layerx-paxeer-client" or . == "layerx-intents"
        or . == "rusqlite" or . == "libsqlite3-sys")
    | "\($ramp) depends on \(.)"
' "$metadata_file")
if [ -n "$forbidden_ramp_dependencies" ]; then
    printf 'platform dependency policy: forbidden ramp dependency\n%s\n' "$forbidden_ramp_dependencies" >&2
    exit 1
fi

jq -r '.packages[] | select(.source != null) | [.name, (.license // "")] | @tsv' \
    "$metadata_file" >"$license_file"
python3 - "$license_file" <<'SPDX_POLICY'
import csv
import re
import sys


ALLOWED_LICENSES = frozenset({
    "Apache-2.0", "BSD-1-Clause", "BSD-2-Clause", "BSD-3-Clause", "CC0-1.0",
    "ISC", "MIT", "Unicode-3.0", "Zlib",
})
ALLOWED_EXCEPTIONS = frozenset({"LLVM-exception"})


class LicenseExpression:
    def __init__(self, expression):
        self.tokens = re.findall(r"[A-Za-z0-9][A-Za-z0-9.+-]*|[()/]|\S", expression)
        self.position = 0

    def peek(self):
        if self.position < len(self.tokens):
            return self.tokens[self.position]
        return None

    def identifier(self):
        token = self.peek()
        if token in (None, "AND", "OR", "WITH") or not re.fullmatch(
            r"[A-Za-z0-9][A-Za-z0-9.+-]*", token
        ):
            raise ValueError("expected SPDX identifier")
        self.position += 1
        return token

    def primary(self):
        if self.peek() == "(":
            self.position += 1
            value = self.disjunction()
            if self.peek() != ")":
                raise ValueError("unclosed SPDX group")
            self.position += 1
            return value
        license_id = self.identifier()
        value = license_id in ALLOWED_LICENSES
        if self.peek() == "WITH":
            self.position += 1
            exception_id = self.identifier()
            value = value and exception_id in ALLOWED_EXCEPTIONS
        return value

    def conjunction(self):
        value = self.primary()
        while self.peek() == "AND":
            self.position += 1
            right = self.primary()
            value = value and right
        return value

    def disjunction(self):
        value = self.conjunction()
        # Cargo also accepts the legacy slash spelling of license alternatives.
        while self.peek() in ("OR", "/"):
            self.position += 1
            right = self.conjunction()
            value = value or right
        return value

    def allowed(self):
        value = self.disjunction()
        if self.peek() is not None:
            raise ValueError("unexpected SPDX token")
        return value


def check_licenses(path):
    with open(path, encoding="utf-8", newline="") as source:
        for package, expression in csv.reader(source, delimiter="\t", quoting=csv.QUOTE_NONE):
            if not expression.strip():
                print(
                    f"platform dependency policy: {package} has no SPDX license",
                    file=sys.stderr,
                )
                return 1
            try:
                allowed = LicenseExpression(expression).allowed()
            except (ValueError, RecursionError):
                allowed = False
            if not allowed:
                print(
                    f"platform dependency policy: {package} uses non-allowlisted license {expression}",
                    file=sys.stderr,
                )
                return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(check_licenses(sys.argv[1]))
SPDX_POLICY

if command -v rg >/dev/null 2>&1; then
    unsafe_hits=$(rg -n --glob '*.rs' '\bunsafe\s+(fn|trait|impl|extern)|\bunsafe\s*\{' "$repo_root/tools/platform/cli" "$repo_root/sdk/platform" "$repo_root/tools/platform/release" "$repo_root/services/platform/tools" || true)
else
    unsafe_hits=$(grep -rEn --include='*.rs' '\bunsafe[[:space:]]+(fn|trait|impl|extern)|\bunsafe[[:space:]]*\{' "$repo_root/tools/platform/cli" "$repo_root/sdk/platform" "$repo_root/tools/platform/release" "$repo_root/services/platform/tools" || true)
fi
if [ -n "$unsafe_hits" ]; then
    printf '%s\n' "$unsafe_hits" >&2
    echo "platform unsafe policy: unsafe code is forbidden" >&2
    exit 1
fi

echo "platform dependency and unsafe policies passed"
