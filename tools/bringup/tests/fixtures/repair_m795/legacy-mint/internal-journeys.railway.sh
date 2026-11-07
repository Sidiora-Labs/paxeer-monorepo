#!/bin/sh
set -eu
railway=${LAYERX_RAILWAY_BIN:-$HOME/.railway/bin/railway}
while IFS= read -r pair; do
	printf '%s' "${pair#*=}" | "$railway" variable set "${pair%%=*}" --stdin --service internal-journeys --environment "${LAYERX_RAILWAY_ENVIRONMENT:-beta}" --skip-deploys >/dev/null
done <"$(dirname "$0")/internal-journeys.railway.env"
