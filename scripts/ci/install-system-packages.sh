#!/usr/bin/env bash
set -euo pipefail

readonly APT_UPDATE_TIMEOUT_SECONDS=120
readonly APT_INSTALL_TIMEOUT_SECONDS=600

use_main_archive() {
  sudo python3 - "${APT_CONFIG_ROOT:-/etc/apt}" <<'PY'
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
regional_archive = re.compile(
    rb"(https?://)(?:[a-z0-9-]+\.)+archive\.ubuntu\.com(?=[:/\s?#]|$)",
    re.IGNORECASE,
)
paths = [root / "sources.list", root / "apt-mirrors.txt"]
paths.extend(sorted((root / "sources.list.d").glob("*.list")))
paths.extend(sorted((root / "sources.list.d").glob("*.sources")))
for path in paths:
    if not path.is_file():
        continue
    original = path.read_bytes()
    updated = regional_archive.sub(rb"\1archive.ubuntu.com", original)
    if updated != original:
        path.write_bytes(updated)
PY
}

for attempt in {1..3}; do
  if [ "$attempt" -gt 1 ]; then
    sudo dpkg --configure -a || true
    use_main_archive || printf 'Could not switch archive sources; continuing.\n' >&2
    printf 'System package attempt %s: main host archive.ubuntu.com sources\n' "$attempt"
  else
    printf 'System package attempt %s: runner sources\n' "$attempt"
  fi
  if timeout --kill-after=10s "${APT_UPDATE_TIMEOUT_SECONDS}s" sudo apt-get -o Acquire::Retries=3 update &&
    timeout --kill-after=10s "${APT_INSTALL_TIMEOUT_SECONDS}s" sudo apt-get -o Acquire::Retries=3 install "$@"; then
    exit 0
  fi
  if [ "$attempt" -lt 3 ]; then
    sleep 5
  fi
done
exit 1
