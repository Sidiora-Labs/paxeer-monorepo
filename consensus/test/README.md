# Tendermint Tests

The unit tests (ie. the `go test` s) live next to the packages they cover. Run them from the repository root with:

```sh
go test ./consensus/...
```

From the `consensus` directory, `make test` runs the same packages with `go test -p 1 -tags deadlock` (target defined in [`Makefile`](./Makefile)).

The upstream integration targets are not runnable in this tree: `make test_integrations` depends on `tools` and `test_libs` targets and on `abci/tests/test_app/test.sh`, `test_cover.sh` and `docker/Dockerfile`, none of which exist in this tree.

## Fuzzing

[Fuzzing](https://en.wikipedia.org/wiki/Fuzzing) of various system inputs.

See [`./fuzz/README.md`](./fuzz/README.md) for more details.

## End-to-end tests

See [`./e2e/README.md`](./e2e/README.md).
