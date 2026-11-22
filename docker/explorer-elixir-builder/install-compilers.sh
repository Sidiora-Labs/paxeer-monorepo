#!/bin/sh
set -eu

manifest=$1
store=$2
mkdir -p "$store/solc_compilers" "$store/vyper_compilers"

while IFS="$(printf '\t')" read -r compiler version url digest; do
  case $compiler in
    solc) path="$store/solc_compilers/$version.js"; mode=644 ;;
    vyper) path="$store/vyper_compilers/$version"; mode=755 ;;
    *) echo "Unknown compiler: $compiler $version" >&2; exit 1 ;;
  esac

  curl --fail --location --retry 3 --retry-all-errors \
    --connect-timeout 30 --max-time 300 --output "$path.download" "$url"
  if ! printf '%s  %s\n' "$digest" "$path.download" | sha256sum -c -; then
    echo "Compiler digest mismatch: $compiler $version" >&2
    exit 1
  fi
  chmod "$mode" "$path.download"
  mv "$path.download" "$path"
done < "$manifest"
