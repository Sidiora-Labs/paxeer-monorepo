#!/usr/bin/env bash

# The inherited Dredd installer is retired; it must never provision executables.
printf '%s\n' \
    'get_nodejs.sh is retired: Node 12 is unsupported and Dredd is archived.' \
    'Provision supported Node.js with the repository runtime manager (mise).' >&2
exit 1
