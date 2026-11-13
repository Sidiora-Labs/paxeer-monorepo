# Fuzzers

The packages in this directory expose Go fuzz targets. Run commands from the
geth module root, `core/deps/geth` in this repository.

For example, run the BN256 addition target with the native Go fuzzer:

```sh
go test ./tests/fuzzers/bn256 -run '^$' -fuzz '^FuzzAdd$'
```

The same package also declares `FuzzMul` and `FuzzPair`. Select one fuzz target
per invocation. Go writes inputs that cause a failure to the package's
`testdata/fuzz/<target>` directory and replays those inputs during ordinary
package tests. Preserve the input when checking a repair.

```sh
go test ./tests/fuzzers/bn256
```

## Failure reports

Keep failure messages distinct so a report identifies the operation and input
that failed. The BN256 comparison targets identify the implementation and
operation in their panic messages.
