# Tendermint Tests

The unit tests (`go test`) live next to the packages they cover. Run them from the repository root with:

```sh
go test ./consensus/...
```

From the `consensus` directory, `make test` runs the same packages with `go test -p 1 -tags deadlock` (target defined in [`Makefile`](./Makefile)).

`make test_release` selects the `release` build tag. `make test_race` runs the packages with the race detector, verbose output and package parallelism limited to one.

## Fuzzing

[Fuzzing](https://en.wikipedia.org/wiki/Fuzzing) of various system inputs.

See [`./fuzz/README.md`](./fuzz/README.md) for more details.

## End-to-end tests

See [`./e2e/README.md`](./e2e/README.md).
