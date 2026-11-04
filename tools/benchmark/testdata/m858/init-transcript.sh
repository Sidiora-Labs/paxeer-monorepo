#!/bin/bash
# CLI peer transcript for the real benchmark init/key/genesis command sequence.
args=("$@")
if [[ ${args[-2]} != --home || ${args[-1]} != "$PAX_HOME" ]]; then
  printf 'selected home missing from initialization\n' >&2
  exit 94
fi
case "$1:$2" in
  init:demo)
    [[ $3 == --chain-id && $4 == pax-chain ]] || exit 95
    printf 'initialized pax-chain\n' > "$PAX_HOME/config/genesis.json"
    ;;
  keys:add)
    [[ $3 == admin && $4 == --keyring-backend && $5 == test ]] || exit 96
    printf '%s\n' "$3" > "$PAX_HOME/key"
    ;;
  keys:show)
    [[ $3 == admin && $4 == -a && $5 == --keyring-backend && $6 == test ]] || exit 97
    [[ $(< "$PAX_HOME/key") == admin ]] || exit 98
    printf 'pax1benchmarkfixture\n'
    ;;
  add-genesis-account:pax1benchmarkfixture)
    printf 'account %s %s\n' "$2" "$3" >> "$PAX_HOME/config/genesis.json"
    ;;
  gentx:admin)
    [[ $4 == --chain-id && $5 == pax-chain && $6 == --keyring-backend && $7 == test ]] || exit 99
    printf '%s %s\n' "$2" "$3" > "$PAX_HOME/gentx"
    ;;
  *) exit 100 ;;
esac
