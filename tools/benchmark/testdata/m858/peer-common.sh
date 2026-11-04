#!/bin/bash
# Committed command/transport fixture for the benchmark launcher, not a node.
if [[ $# != 5 || $1 != start || $2 != --chain-id || $3 != pax-chain || $4 != --home || $5 != "$PAX_HOME" ]]; then
  printf 'unexpected benchmark arguments\n' >&2
  exit 91
fi
if [[ $BENCHMARK_CONFIG != 'fixture scenario with spaces.json' || $BENCHMARK_TXS_PER_BATCH != 19 ]]; then
  printf 'benchmark environment was not forwarded\n' >&2
  exit 92
fi
printf '%s\n' "$$" > "$PAX_HOME/node.pid"
printf '%s\n' "$@" "$BENCHMARK_CONFIG" "$BENCHMARK_TXS_PER_BATCH" > "$PAX_HOME/invocation"
