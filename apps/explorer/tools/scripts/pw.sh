#!/bin/bash

select_playwright_command() {
    local affected_flag=false
    local affected_flag_seen=false
    local arg file
    local args=()
    local files_to_run=()
    local affected_tests_file="./playwright/affected-tests.txt"
    playwright_command=()

    for arg in "$@"; do
        case "$arg" in
            --affected|--affected=*)
                if [ "$affected_flag_seen" = false ]; then
                    if [ "${arg#*=}" != "false" ]; then
                        affected_flag=true
                    fi
                    affected_flag_seen=true
                fi
                ;;
            *)
                args+=("$arg")
                ;;
        esac
    done

    if [ "$affected_flag" = true ] && [ -f "$affected_tests_file" ]; then
        while IFS= read -r file || [ -n "$file" ]; do
            if [ -n "$file" ]; then
                files_to_run+=("$file")
            fi
        done < "$affected_tests_file"

        if [ "${#files_to_run[@]}" -eq 0 ]; then
            return 1
        fi
    fi

    playwright_command=(playwright test -c playwright-ct.config.ts "${files_to_run[@]}" "${args[@]}")
}

main() {
    local config_file="./configs/envs/.env.pw"
    local playwright_command=()

    rm -rf ./playwright/.cache

    dotenv \
      -e "$config_file" \
      -- bash -c './deploy/scripts/make_envs_script.sh ./playwright/envs.js'

    # Important to set this variable here, so the sprite will be built correctly
    export NEXT_PUBLIC_APP_ENV=pw
    yarn svg:build-sprite

    if ! select_playwright_command "$@"; then
        echo "No affected tests found in the file. Exiting..."
        return 0
    fi

    printf 'Running Playwright tests:'
    printf ' %q' "${playwright_command[@]}"
    printf '\n'

    dotenv \
      -v NODE_OPTIONS=\"--max-old-space-size=8192\" \
      -e "$config_file" \
      -- "${playwright_command[@]}"
}

if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then
    main "$@"
fi
