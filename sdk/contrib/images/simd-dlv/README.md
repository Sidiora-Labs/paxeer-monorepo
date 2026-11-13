# simd Delve wrapper

Delve (`github.com/go-delve/delve`) is a debugger for Go programs.
[`wrapper.sh`](wrapper.sh) starts `/simd/$BINARY` (default `simd`) with
`--home /data/node$ID/simd` (`ID` defaults to `0`). When `DEBUG=1`, it runs the
binary under `dlv` with `--listen=:2345 --continue --headless=true
--api-version=2 --accept-multiclient`. A client can attach with
`dlv connect <host>:2345`.

This directory has no image build recipe. The `simd-dlv` and default `all`
targets in [`sdk/contrib/images/Makefile`](../Makefile) refuse image builds
with a nonzero exit. This checkout provides `paxd`, built with
`make -f chain.mk build` from the repository root; it has no `simd` application
or supported Paxd Delve image.
