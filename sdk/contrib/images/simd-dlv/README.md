# Retired simd Delve image

Delve (`github.com/go-delve/delve`) is a debugger for the Go programming language.

The inherited Cosmos SDK `simd` Delve image is withdrawn. Its `Dockerfile`
has been removed: it used Go 1.18, upstream `contrib/` paths and `build/simd`,
which do not match this repository. The `simd-dlv` and default `all` targets
in [`sdk/contrib/images/Makefile`](../Makefile) already refuse image builds
with a retirement diagnostic and a nonzero exit.

This repository has no `simd` binary or `simapp`. Build the Paxeer X chain
binary, `paxd`, with `make -f chain.mk build` from the repository root.
This directory does not provide a supported paxd debug image.

## Historical wrapper

`wrapper.sh` remains as inherited tooling, without an image build recipe.
It runs `/simd/$BINARY` (default `simd`)
with `--home /data/node$ID/simd`. When `DEBUG=1`, it starts the binary under
`dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient`,
so a debugger can attach on port `2345`. The former image exposed `26656`,
`26657` and `2345`.

## Historical debugger connection

Attach with `dlv connect <host>:2345`, or with any IDE that can attach to a
remote Delve server.
