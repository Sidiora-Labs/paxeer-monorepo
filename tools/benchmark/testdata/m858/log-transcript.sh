#!/bin/bash
source "${0%/*}/peer-common.sh"
printf 'ordinary node output\nbenchmark tps=19\nBenchmark complete\ndeployed contract\ntransitioning scenario\n'
printf 'unrelated diagnostic\nbenchmark stderr\n' >&2
printf 'benchmark final line without newline'
exit 0
