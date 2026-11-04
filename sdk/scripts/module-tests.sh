#!/usr/bin/env bash

# Run every nested module: path-only diffs cannot identify dependent consumers.
set -euo pipefail

execute_mod_tests() {
    local go_mod=$1
    local mod_dir
    mod_dir=$(dirname "$go_mod")

    echo ">>> running $go_mod tests"
    (
        cd -- "$mod_dir" || exit 1
        go test -mod=readonly -timeout 30m \
            -coverprofile="$module_profile" -covermode=atomic \
            -tags='norace ledger test_ledger_mock' ./...
    )
}

append_coverage_profile() {
    local mode
    if ! IFS= read -r mode < "$module_profile"; then
        echo "Missing or incomplete coverage header: $module_profile" >&2
        return 1
    fi
    if [[ "$mode" != "mode: atomic" ]]; then
        echo "Unexpected coverage mode: $mode (expected mode: atomic)" >&2
        return 1
    fi
    tail -n +2 -- "$module_profile" >> "$aggregate_profile" || return 1
}

root_dir=$(pwd)
coverage_file="$root_dir/coverage-go-submod-profile.out"
work_dir=$(mktemp -d "$root_dir/.module-tests.XXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT
module_profile="$work_dir/module.out"
aggregate_profile="$work_dir/coverage.out"
module_list="$work_dir/modules"

printf 'mode: atomic\n' > "$aggregate_profile"
find . -name go.mod -not -path "./go.mod" -print0 > "$module_list"

return_val=0
while IFS= read -r -d '' go_mod; do
    rm -f -- "$module_profile"
    if execute_mod_tests "$go_mod"; then
        if ! append_coverage_profile; then
            return_val=2
        fi
    else
        return_val=2
    fi
done < "$module_list"

if [[ "$return_val" -eq 0 ]]; then
    mv -- "$aggregate_profile" "$coverage_file"
fi

exit "$return_val"
