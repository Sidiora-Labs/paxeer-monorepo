#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
metadata_file=$(mktemp)
trap 'rm -f -- "$metadata_file"' EXIT HUP INT TERM

case "$#" in
    0)
        cargo metadata --manifest-path "$repo_root/services/agent/Cargo.toml" \
            --locked --format-version 1 >"$metadata_file"
        ;;
    1)
        cat -- "$1" >"$metadata_file"
        ;;
    *)
        echo "usage: dependency-policy.sh [metadata.json]" >&2
        exit 2
        ;;
esac

banned='^(bindgen|libsqlite3-sys|rusqlite|sqlx-sqlite|ctor|inventory)$'
banned_path=$(jq -r --arg banned "$banned" '
    INDEX(.packages[]; .id) as $packages
    | INDEX(.resolve.nodes[]; .id) as $nodes
    | [.workspace_members[] | {id: ., path: [$packages[.].name]}] as $roots
    | {pending: $roots, seen: {}, found: null}
    | until(.found != null or (.pending | length == 0);
        .pending[0] as $current
        | .pending = .pending[1:]
        | if .seen[$current.id] then .
          else
            .seen[$current.id] = true
            | if ($packages[$current.id].name | test($banned)) then
                .found = $current.path
              else
                .pending += [
                    $nodes[$current.id].deps[]?
                    | select(any(.dep_kinds[]?; .kind == null or .kind == "normal" or .kind == "build"))
                    | .pkg as $id
                    | {id: $id, path: ($current.path + [$packages[$id].name])}
                  ]
              end
          end)
    | .found // empty | join(" -> ")
' "$metadata_file")
if [ -n "$banned_path" ]; then
    echo "dependency policy: forbidden boundary or global-initialisation crate: $banned_path" >&2
    exit 1
fi

missing_license=$(jq -r '.packages[] | select(.source != null and (.license == null or .license == "")) | .name' "$metadata_file")
if [ -n "$missing_license" ]; then
    echo "dependency policy: dependency without an allowed SPDX license: $missing_license" >&2
    exit 1
fi

allowlist="$repo_root/services/agent/unsafe-allowlist.toml"
test -r "$allowlist"

if command -v rg >/dev/null 2>&1; then
    unsafe_hits=$(rg -n --glob '*.rs' '\bunsafe\s+(fn|trait|impl|extern)|\bunsafe\s*\{' "$repo_root/services/agent/crates" || true)
else
    unsafe_hits=$(grep -rEn --include='*.rs' '\bunsafe[[:space:]]+(fn|trait|impl|extern)|\bunsafe[[:space:]]*\{' "$repo_root/services/agent/crates" || true)
fi

exceptions=$(sed -n 's/^path = "\(.*\)"$/\1/p' "$allowlist")
for exception in $exceptions; do
    if [ ! -f "$repo_root/services/agent/$exception" ]; then
        echo "unsafe policy: allowlisted exception no longer exists: $exception" >&2
        exit 1
    fi
done

unreviewed=$(printf '%s\n' "$unsafe_hits" | while IFS= read -r hit; do
    [ -n "$hit" ] || continue
    file=${hit%%:*}
    relative=${file#"$repo_root/services/agent/"}
    reviewed=0
    for exception in $exceptions; do
        if [ "$relative" = "$exception" ]; then
            reviewed=1
            break
        fi
    done
    [ "$reviewed" -eq 1 ] || printf '%s\n' "$hit"
done)

if [ -n "$unreviewed" ]; then
    printf '%s\n' "$unreviewed" >&2
    echo "unsafe policy: unsafe code requires an explicit reviewed exception" >&2
    exit 1
fi

test -r "$repo_root/services/agent/deny.toml"
echo "agent dependency and unsafe policies passed"
