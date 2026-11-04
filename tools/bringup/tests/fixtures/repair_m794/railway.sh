#!/usr/bin/env bash
set -euo pipefail
sub="${1:-}"
shift
service="" environment="" instance=""
args=("$sub" "$@")
while [ "$#" -gt 0 ]; do
	case "$1" in
	--service) service="$2"; shift 2 ;;
	--environment) environment="$2"; shift 2 ;;
	--deployment-instance) instance="$2"; shift 2 ;;
	--json) shift ;;
	--) shift; break ;;
	*) exit 98 ;;
	esac
done
[ "$environment" = beta ] || exit 97
printf 'railway %s\n' "${args[*]}" >>"$CHECK_LIVE_TEST_CALLS"
role="$CHECK_LIVE_TEST_ROLE_ROOTS/$service"
case "$sub" in
status) cat "$role/status.json" ;;
ssh)
	[ -z "$instance" ] || [ "$instance" = "$service-one" ] || exit 96
	[ "$#" -eq 1 ] || exit 95
	command="$1"
	# The SSH peer transports the production command unchanged. Files and
	# processes are real local fixtures, and stdin forwarding stays disabled.
	exec bash -c "$command" </dev/null
	;;
*) exit 94 ;;
esac
