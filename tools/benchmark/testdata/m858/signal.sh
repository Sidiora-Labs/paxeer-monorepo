#!/bin/bash
source "${0%/*}/peer-common.sh"
trap 'printf "INT\n" > "$PAX_HOME/signal"; exit 130' INT
trap 'printf "TERM\n" > "$PAX_HOME/signal"; exit 143' TERM
printf 'Benchmark peer ready\n'
: > "$PAX_HOME/ready"
while IFS= read -r line; do
  :
done
exit 93
