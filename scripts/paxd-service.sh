#!/usr/bin/env bash
# The service must own every writer of the selected home; disable external restarters.
set -euo pipefail
service="${PAXD_SERVICE:-paxd.service}"
case "${1:-}" in
    stop|start) exec systemctl "$1" "$service" ;;
    check-stopped)
        state="$(systemctl show --property=ActiveState --value "$service")"
        pid="$(systemctl show --property=MainPID --value "$service")"
        [[ "$state" == inactive || "$state" == failed ]] && [[ "$pid" == 0 ]]
        ;;
    check-running) exec systemctl is-active --quiet "$service" ;;
    *) exit 2 ;;
esac
