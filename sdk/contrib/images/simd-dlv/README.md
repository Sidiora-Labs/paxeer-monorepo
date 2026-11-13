# simd Delve wrapper

Delve (`github.com/go-delve/delve`) is a debugger for Go programs.
[`wrapper.sh`](wrapper.sh) starts `/simd/$BINARY` (default `simd`) with
`--home /data/node$ID/simd` (`ID` defaults to `0`). When `DEBUG=1`, it runs the
binary under `dlv` with `--listen=:2345 --continue --headless=true
--api-version=2 --accept-multiclient`. A client can attach with
`dlv connect <host>:2345`.

The wrapper requires a shell with `pipefail` support. From this directory,
invoke it with `bash wrapper.sh start`, with the application binary placed
under `/simd` and its node home under `/data`.

The simulation application source is in
[`interchain/testing/simapp/simd`](../../../../interchain/testing/simapp/simd).
The Paxeer node is `paxd`, built with `make -f chain.mk build` from the
repository root. The wrapper runs binaries supplied under `/simd`;
this directory provides the launch script only.
