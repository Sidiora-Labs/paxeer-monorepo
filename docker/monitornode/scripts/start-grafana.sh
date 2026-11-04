#!/usr/bin/env bash
#
# Starts the local Grafana server, preconfigured with Prometheus as a data source.
# Prometheus must be running (see start-prometheus.sh).
#
# Usage: ./start-grafana.sh
# Optional: GRAFANA_BIND_ADDRESS (default 127.0.0.1), GRAFANA_ADMIN_PASSWORD.
# Remote bindings require a non-default password; credentials are never printed.
#
# Requirements: Docker must be installed and running.
# Compatible with macOS and Linux.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MONITOR_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATASOURCE_CONFIG="${MONITOR_DIR}/config/grafana.yaml"
DASHBOARD_PROVISIONING="${MONITOR_DIR}/config/grafana-dashboards.yaml"
DASHBOARDS_DIR="${MONITOR_DIR}/dashboards"
CONTAINER_NAME="pax-grafana"
GRAFANA_PORT=3000
GRAFANA_BIND_ADDRESS="${GRAFANA_BIND_ADDRESS:-127.0.0.1}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}"
PROMETHEUS_UI_PORT=9091

if [[ "$GRAFANA_BIND_ADDRESS" != "127.0.0.1" ]]; then
	if [[ -z "$GRAFANA_ADMIN_PASSWORD" || "$GRAFANA_ADMIN_PASSWORD" == "admin" || "$GRAFANA_ADMIN_PASSWORD" == *$'\n'* ]]; then
		echo "Error: remote Grafana bindings require an explicit, non-default, single-line GRAFANA_ADMIN_PASSWORD." >&2
		exit 1
	fi
fi
export GF_SECURITY_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"

# Check for Docker
if ! command -v docker &>/dev/null; then
	echo "Error: docker is not installed or not in PATH" >&2
	exit 1
fi

# Check that Docker daemon is reachable
if ! docker info &>/dev/null; then
	echo "Error: Docker daemon is not running or not accessible. Start Docker and try again." >&2
	exit 1
fi

# Check that datasource config exists
if [[ ! -f "${DATASOURCE_CONFIG}" ]]; then
	echo "Error: Grafana datasource config not found at ${DATASOURCE_CONFIG}" >&2
	exit 1
fi

# Check that dashboard provisioning config exists
if [[ ! -f "${DASHBOARD_PROVISIONING}" ]]; then
	echo "Error: Dashboard provisioning config not found at ${DASHBOARD_PROVISIONING}" >&2
	exit 1
fi

check_existing_container() {
	local network bindings environment entry password_matches=false
	network="$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$CONTAINER_NAME")"
	bindings="$(docker inspect --format '{{range (index .HostConfig.PortBindings "3000/tcp")}}{{printf "%s:%s\n" .HostIp .HostPort}}{{end}}' "$CONTAINER_NAME")"
	if [[ "$network" == "host" || "$bindings" != "${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}" ]]; then
		echo "Error: existing Grafana container has incompatible port bindings. Remove and recreate it with this script after preserving any required data." >&2
		exit 1
	fi
	if [[ "$GRAFANA_BIND_ADDRESS" != "127.0.0.1" ]]; then
		environment="$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CONTAINER_NAME")"
		while IFS= read -r entry; do
			if [[ "$entry" == GF_SECURITY_ADMIN_PASSWORD=* ]]; then
				if [[ "$entry" != "GF_SECURITY_ADMIN_PASSWORD=${GF_SECURITY_ADMIN_PASSWORD}" ]]; then
					echo "Error: existing Grafana credential differs. Recreate the container to apply GRAFANA_ADMIN_PASSWORD." >&2
					exit 1
				fi
				password_matches=true
			fi
		done <<< "$environment"
		if [[ "$password_matches" != true ]]; then
			echo "Error: existing remote Grafana container lacks an explicit credential. Recreate it with this script." >&2
			exit 1
		fi
	fi
}

# If container exists and is running, validate it before reporting success.
if docker ps -q -f "name=^${CONTAINER_NAME}$" | grep -q .; then
	check_existing_container
	echo "Grafana is already running."
	echo "  UI: http://${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}"
	echo "  Login user: admin"
	exit 0
fi

# If container exists but is stopped, start it
if docker ps -aq -f "name=^${CONTAINER_NAME}$" | grep -q .; then
	check_existing_container
	echo "Starting existing Grafana container..."
	docker start "$CONTAINER_NAME"
	echo ""
	echo "Grafana is running."
	echo "  UI: http://${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}"
	echo "  Login user: admin"
	exit 0
fi

# Container doesn't exist – create and run (first-time setup)
# Grafana needs to reach Prometheus on the host; use host.docker.internal.
case "$(uname -s)" in
	Linux*) ADD_HOST_FLAG="--add-host=host.docker.internal:host-gateway" ;;
	*) ADD_HOST_FLAG="" ;;
esac

echo "Creating and starting Grafana container..."
docker run -d \
	--name "$CONTAINER_NAME" \
	${ADD_HOST_FLAG:+"$ADD_HOST_FLAG"} \
	-p "${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}:3000" \
	-v "${DATASOURCE_CONFIG}:/etc/grafana/provisioning/datasources/grafana.yaml:ro" \
	-v "${DASHBOARD_PROVISIONING}:/etc/grafana/provisioning/dashboards/grafana-dashboards.yaml:ro" \
	-v "${DASHBOARDS_DIR}:/var/lib/grafana/dashboards:ro" \
	-e "GF_SECURITY_ADMIN_USER=admin" \
	-e GF_SECURITY_ADMIN_PASSWORD \
	-e "GF_USERS_ALLOW_SIGN_UP=false" \
	grafana/grafana:latest

echo ""
echo "Grafana is running."
echo "  UI:           http://${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}"
echo "  Login user:   admin (local default password is admin; GRAFANA_ADMIN_PASSWORD overrides it)"
echo "  Data source:  Prometheus (provisioned from ${DATASOURCE_CONFIG})"
echo ""
echo "To stop: ./stop-grafana.sh"
echo ""
echo "Note: Start Prometheus first (./start-prometheus.sh) to have data to visualize."
