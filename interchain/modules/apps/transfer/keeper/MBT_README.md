# Token Transfer Model-based Testing Guide

The ICS-20 transfer keeper includes model-based tests of token relay functions.
The [TLA+ model](relay_model/relay.tla) defines the relay operations;
[relay_tests.tla](relay_model/relay_tests.tla) defines assertions over the model's
`history` variable.

## Generate a model execution

Use the [Apalache model checker](https://github.com/informalsystems/apalache).
For `TestUnescrowTokensInv`, run from `interchain/modules/apps/transfer/keeper`:

```bash
cd relay_model
apalache-mc check --inv=TestUnescrowTokensInv relay_tests.tla
```

The model checker emits counterexamples that serve as model executions for
these tests. `counterexample.tla` is the human-readable representation;
`counterexample.json` is the machine-readable representation.

## Translate and execute

[Jsonatr](https://github.com/informalsystems/jsonatr) translates the JSON using
[apalache-to-relay-test2.json](relay_model/apalache-to-relay-test2.json).
After building Jsonatr, add its `target/debug` directory to `PATH`. Run from
`relay_model`:

```bash
jsonatr --use apalache-to-relay-test2.json --in counterexample.json --out ../model_based_tests/YourTestName.json
```

Run `go test` from `interchain/modules/apps/transfer/keeper`.
The [model-based test driver](mbt_relay_test.go) loads `.json` files from
`model_based_tests`, executes the requested handlers and compares bank changes
and handler outcomes with the model.

## Apalache container wrapper

The following `apalache-mc` script mounts the current model directory:

```bash
#!/bin/bash
docker run --rm -v "$PWD:/var/apalache" apalache/mc "$@"
```
