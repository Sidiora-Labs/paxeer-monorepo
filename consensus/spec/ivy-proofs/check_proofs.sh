#!/bin/bash

# returns non-zero error code if any proof fails

success=0
umask 077 || exit 1
mkdir -p output || exit 1
if ! log_dir=$(mktemp -d output/proofs-XXXXXX); then
  exit 1
fi
cmd=(ivy_check "seed=$RANDOM")

check_proof() {
  local label=$1 proof=$2
  shift 2
  local log_file="$log_dir/${proof%.ivy}.txt"
  local res
  local -a pipeline_status

  echo "Checking $label:"
  "${cmd[@]}" "$@" "$proof" 2>&1 | tee -- "$log_file"
  pipeline_status=("${PIPESTATUS[@]}")
  if [ "${pipeline_status[0]}" -eq 0 ] &&
     [ "${pipeline_status[1]}" -eq 0 ] &&
     res=$(tail -n 1 -- "$log_file") &&
     [ "$res" = "OK" ]; then
    echo "OK"
  else
    echo "FAILED"
    success=1
  fi
}

check_proof "classic safety" classic_safety.ivy
check_proof "accountable safety 1" accountable_safety_1.ivy
check_proof "accountable safety 2" accountable_safety_2.ivy complete=fo

echo
echo "See ivy_check output in $log_dir"
exit "$success"
