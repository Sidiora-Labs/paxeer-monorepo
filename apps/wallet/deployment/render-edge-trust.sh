#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "Usage: render-edge-trust.sh <output-file>" >&2
  exit 1
fi

node - "$1" <<'NODE'
const { writeFileSync } = require('node:fs');
const { isIP } = require('node:net');

const configured = process.env.TRUSTED_EDGE_CIDRS ?? '';
const cidrs = configured === '' ? [] : configured.split(',').map((entry) => entry.trim());

for (const cidr of cidrs) {
  const parts = cidr.split('/');
  const family = isIP(parts[0]);
  if (
    parts.length !== 2 ||
    !/^[0-9a-fA-F:.]+$/.test(parts[0]) ||
    family === 0 ||
    !/^(0|[1-9][0-9]{0,2})$/.test(parts[1]) ||
    Number(parts[1]) > (family === 4 ? 32 : 128)
  ) {
    console.error('TRUSTED_EDGE_CIDRS must contain only comma-separated IPv4/IPv6 CIDRs');
    process.exit(1);
  }
}

const directives = cidrs.map((cidr) => `set_real_ip_from ${cidr};\n`).join('');
writeFileSync(
  process.argv[2],
  cidrs.length === 0
    ? ''
    : `${directives}real_ip_header X-Forwarded-For;\nreal_ip_recursive on;\n`
);
NODE
