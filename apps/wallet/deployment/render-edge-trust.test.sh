#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
renderer="$script_dir/render-edge-trust.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
trap 'exit 1' INT TERM

render_edge_trust() {
  TRUSTED_EDGE_CIDRS='10.0.0.0/8,2001:db8::/32' sh "$renderer" "$work/edge-trust.conf"
  cat > "$work/expected.conf" <<'CONF'
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 2001:db8::/32;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
CONF
  cmp "$work/expected.conf" "$work/edge-trust.conf"

  (
    unset TRUSTED_EDGE_CIDRS
    sh "$renderer" "$work/edge-trust.conf"
  )
  [ -f "$work/edge-trust.conf" ]
  [ ! -s "$work/edge-trust.conf" ]

  TRUSTED_EDGE_CIDRS='' sh "$renderer" "$work/empty.conf"
  [ -f "$work/empty.conf" ]
  [ ! -s "$work/empty.conf" ]

  for cidr in '0.0.0.0/0' '255.255.255.255/32' '::/0' '::1/128' \
    '2001:db8:0:0:0:0:0:1/128' '::ffff:192.0.2.1/128'; do
    TRUSTED_EDGE_CIDRS="$cidr" sh "$renderer" "$work/valid.conf"
    printf 'set_real_ip_from %s;\nreal_ip_header X-Forwarded-For;\nreal_ip_recursive on;\n' \
      "$cidr" > "$work/expected.conf"
    cmp "$work/expected.conf" "$work/valid.conf"
  done

  for invalid in '10.0.0.1;evil' '10.0.0.0/8;evil' '10.0.0.1' '::1' \
    'edge.example/24' '256.0.0.0/8' '10.0.0.0/33' '2001:db8::/129' \
    '2001:db8:::1/64' 'fe80::1%eth0/64' '10.0.0.0/-1' '10.0.0.0/8/1' \
    '10.0.0.0/8,' ',10.0.0.0/8' '10.0.0.0/8,,::1/128' \
    '10.0.0.0/8,invalid' ' '; do
    printf 'unchanged\n' > "$work/refused.conf"
    if TRUSTED_EDGE_CIDRS="$invalid" sh "$renderer" "$work/refused.conf" 2> "$work/error"; then
      echo "FAIL: accepted invalid CIDR list: $invalid" >&2
      exit 1
    fi
    [ -s "$work/error" ]
    printf 'unchanged\n' > "$work/expected.conf"
    cmp "$work/expected.conf" "$work/refused.conf"
  done

  echo 'render_edge_trust: ok'
}

render_edge_trust
