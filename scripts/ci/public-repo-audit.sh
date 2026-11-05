#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

publication_files=$(mktemp)
publication_files_nul=$(mktemp)
secret_files_nul=$(mktemp)
gitlinks=$(mktemp)
declared_submodules=$(mktemp)
undeclared_gitlinks=$(mktemp)
trap 'rm -f "$publication_files" "$publication_files_nul" "$secret_files_nul" "$gitlinks" "$declared_submodules" "$undeclared_gitlinks"' EXIT HUP INT TERM

git ls-files --cached --others --exclude-standard > "$publication_files"
git ls-files --cached --others --exclude-standard -z > "$publication_files_nul"

tracked_worktrees=$(git ls-files --cached | awk '/^\.worktrees\// { print }')
if [ -n "$tracked_worktrees" ]; then
    echo "local worktree paths must not be tracked:" >&2
    printf '%s\n' "$tracked_worktrees" >&2
    exit 1
fi

git ls-files --stage | awk '$1 == "160000" { sub(/^[^\t]*\t/, ""); print }' | sort -u > "$gitlinks"
if [ -f .gitmodules ]; then
    git config -f .gitmodules --get-regexp '^submodule\..*\.path$' 2>/dev/null \
        | cut -d ' ' -f 2- \
        | sort -u > "$declared_submodules" || true
fi
comm -23 "$gitlinks" "$declared_submodules" > "$undeclared_gitlinks"
if [ -s "$undeclared_gitlinks" ]; then
    echo "tracked gitlinks must be declared in .gitmodules:" >&2
    cat "$undeclared_gitlinks" >&2
    exit 1
fi

mutable_action_refs=$(find .github/workflows -type f \( -name '*.yml' -o -name '*.yaml' \) -exec awk '
    /^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*/ {
        ref = $0
        sub(/^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*/, "", ref)
        sub(/[[:space:]#].*$/, "", ref)
        if (ref ~ /^\.\//) {
            next
        }
        if (ref ~ /^docker:\/\//) {
            digest = ref
            sub(/^.*@sha256:/, "", digest)
            if (ref !~ /@sha256:/ || length(digest) != 64 || digest ~ /[^0-9A-Fa-f]/) {
                print FILENAME ":" FNR ": " ref
            }
            next
        }
        revision = ref
        sub(/^.*@/, "", revision)
        if (index(ref, "@") == 0 || length(revision) != 40 || revision ~ /[^0-9A-Fa-f]/) {
            print FILENAME ":" FNR ": " ref
        }
    }
' {} +)
if [ -n "$mutable_action_refs" ]; then
    echo "external GitHub Actions must use immutable commit pins and Docker actions must use sha256 digests:" >&2
    printf '%s\n' "$mutable_action_refs" >&2
    exit 1
fi

required_files=".editorconfig .gitattributes .gitignore CHANGELOG.md \
CONTRIBUTING.md LICENSE NOTICE README.md SECURITY.md SUPPORT.md \
docs/QUALIFICATION.md .github/workflows/ci.yml"
for required_file in $required_files; do
    if [ ! -s "$required_file" ]; then
        echo "required publication file is missing or empty: $required_file" >&2
        exit 1
    fi
done

while IFS= read -r required_ignore; do
    if ! rg -Fqx -- "$required_ignore" .gitignore; then
        echo "required ignore rule is missing: $required_ignore" >&2
        exit 1
    fi
done <<'EOF'
/build/
/build-*/
/.codegraph/
/.logs/
/cache/
/.worktrees/
.env
*.pem
*.key
*.vsix
EOF

large_file=''
while IFS= read -r publication_file; do
    if [ -f "$publication_file" ] && [ "$(wc -c < "$publication_file")" -gt 52428800 ]; then
        large_file=$publication_file
        break
    fi
done < "$publication_files"
if [ -n "$large_file" ]; then
    echo "publishable file exceeds 50 MiB: $large_file" >&2
    exit 1
fi

reviewed_symlinks='^spec/specgen$'
reviewed_symlinks="$reviewed_symlinks"'|^core/deps/cosmwasm/packages/vm/testdata/floaty\.wasm$'
root_real=$(pwd -P)
symlinks=''
while IFS= read -r publication_file; do
    if [ -L "$publication_file" ]; then
        if printf '%s\n' "$publication_file" | grep -Eq -- "$reviewed_symlinks"; then
            symlink_target=$(realpath -- "$publication_file" 2>/dev/null || true)
            case "$symlink_target" in
                "$root_real"/?*) continue ;;
            esac
            echo "reviewed symbolic link does not resolve inside the repository: $publication_file" >&2
            exit 1
        fi
        symlinks="${symlinks}${symlinks:+
}$publication_file"
    fi
done < "$publication_files"
if [ -n "$symlinks" ]; then
    echo "symbolic links require explicit publication review:" >&2
    printf '%s\n' "$symlinks" >&2
    exit 1
fi

audit_rg() {
    audit_exclude=$1
    shift
    python3 "$root/scripts/ci/checked_rg.py" --gitlinks "$gitlinks" --exclude "$audit_exclude" -- \
        --hidden --no-heading --with-filename -n "$@" < "$publication_files_nul"
}

private_refs='(/root/(Layerx-protocol|project-Quorum|private-neo-v1|matrix|layerX)(/|$)|147\.93\.139\.18)'
audit_status=0
audit_rg '^scripts/ci/public-repo-audit\.sh$' "$private_refs" || audit_status=$?
case $audit_status in
    0) echo "private workspace or infrastructure reference found" >&2; exit 1 ;;
    1) ;; # Every selected input was searched without a match.
    *) echo "publication refused: private-reference search incomplete" >&2; exit 1 ;;
esac

secret_scan_allowlist='^scripts/ci/public-repo-audit\.sh$'
secret_scan_allowlist="$secret_scan_allowlist"'|^rpc/tests/mock_data/transactions/0x99d895ea71e5ce3a8b949ba7979a27c08080210a4ba9b46b0bb06f8126b6957d\.json$'
secret_scan_allowlist="$secret_scan_allowlist"'|^rpc/tests/mock_data/transactions/0x1b9ceaabadfc635aa8eb5e6d4a66ee60c826980805fa93af3913872f7b565586\.json$'
secret_scan_allowlist="$secret_scan_allowlist"'|^sdk/integrations/ios/Sources/LayerXMobile/EmbeddedSecretDetector\.swift$'
secret_scan_allowlist="$secret_scan_allowlist"'|^sdk/integrations/android/src/main/java/com/sidiora/layerx/android/EmbeddedSecretDetector\.java$'
secret_scan_allowlist="$secret_scan_allowlist"'|^programs/vendor/'
secret_scan_allowlist="$secret_scan_allowlist"'|^core/deps/db-backend/remotedb/test\.key$'
if ! bash scripts/ci/secret-scan.sh --self-test; then
    echo "secret scan self-test failed" >&2
    exit 1
fi
python3 "$root/scripts/ci/checked_rg.py" --select-only --gitlinks "$gitlinks" \
    --exclude "$secret_scan_allowlist" < "$publication_files_nul" > "$secret_files_nul"
if ! bash scripts/ci/secret-scan.sh < "$secret_files_nul"; then
    echo "secret-shaped material found in publication set" >&2
    exit 1
fi

for script in tools/*.sh scripts/ci/*.sh; do
    IFS= read -r interpreter < "$script"
    case "$interpreter" in
        '#!/bin/sh' | '#!/usr/bin/env sh') sh -n "$script" ;;
        '#!/bin/bash' | '#!/usr/bin/env bash') bash -n "$script" ;;
        *)
            echo "unsupported shell interpreter: $script" >&2
            exit 1
            ;;
    esac
done

if command -v go >/dev/null 2>&1; then
    spec/specgen -check
else
    echo "go is required to verify generated specification artifacts" >&2
    exit 1
fi

echo "public repository audit passed"
