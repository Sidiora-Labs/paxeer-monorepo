# Integration Test Framework

YAML-driven integration tests for `paxd`. Commands run sequentially in a local
Docker cluster and verifiers check their captured output. The Python and Go
drivers consume the same YAML files, but their expression evaluators differ.

Paxeer mainnet-beta is one BFT network, with the kernel participating in block
proposal. A receipt is final when its block is committed. Its EVM chain ID is
125 (`0x7d`). Public JSON-RPC uses the root path of the nodes listed in the
[public RPC reference](../../docs/site/docs/reference/public-rpc.md). The commands
below target a disposable local cluster; local genesis settings can differ from
mainnet.

Related suites:

- [dApp scenarios](dapp_tests/README.md): EVM, CosmWasm and pointer interactions.
- [EVM RPC fixtures](evm_module/rpc_io_test/RPC_IO_README.md): `.io` / `.iox` method schemas and deterministic response checks.
- [RPC parity suite](rpc_tests/README.md): TypeScript assertions against a local geth reference.

## Getting Started

### Prerequisites

- Docker and Docker Compose, with the daemon running.
- Python 3 and PyYAML for the Python driver (`python3 -m pip install pyyaml`).
- The repository's Go toolchain and native dependencies for the Go driver.

### Usage

From the repository root, start the local four-node cluster:

```bash
DOCKER_DETACH=true make docker-cluster-start
```

Run a YAML file through the [Python driver](scripts/runner.py):

```bash
python3 tests/integration/scripts/runner.py tests/integration/startup/startup_test.yaml
```

Or run a registered suite through the [Go driver](runner/runner.go):

```bash
go test -tags yaml_integration -v ./tests/integration/runner/... -run '^TestBankModule$'
```

[suites.go](runner/suites.go) lists suite names and YAML files in execution order.
The Go integration wrappers require the `yaml_integration` build tag. Some
suites need additional infrastructure: `TestStateSync` requires `pax-rpc-node`,
provided by `make run-rpc-node-integration-ci`. Upgrade YAML files are driven
separately through Python by the upgrade workflow.

## Writing Tests

Each file is a YAML list of cases. The [template](template/template_test.yaml)
shows the field layout. Use the common expression subset below for files shared
by the Python and Go drivers.

```yaml
- name: Validator count is four
  inputs:
    - cmd: paxd q tendermint-validator-set | grep address | wc -l
      env: RESULT
  verifiers:
    - type: eval
      expr: RESULT == 4
    - type: regex
      result: RESULT
      expr: '^4$'
```

### Explanation

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `name` | Yes | Case name; becomes a named Go subtest. |
| `inputs` | Yes | Commands executed in list order. |
| `cmd` | Yes | Shell command inside the chosen container. |
| `env` | No | Capture trimmed stdout under this name for later inputs and verifiers in this case. |
| `node` | No | Container name; defaults to `pax-node-0` in both drivers. |
| `verifiers` | Yes | Checks evaluated after the inputs finish. |
| `type` | Yes | `eval` or `regex`. |
| `result` | For `regex` | Captured output name to match. |
| `expr` | Yes | Comparison expression or regular expression. |

### Expression rules shared by both drivers

For portable `eval` checks, use one comparison per verifier:

```text
operand operator operand
```

Separate every token with spaces. Operators are `==`, `!=`, `>`, `<`, `>=`
and `<=`; operands can be captured names or canonical decimal integers
(without leading zeroes except `0`). For simple string
equality, use a captured single-token value and a double-quoted literal, such as
`STATUS == "ready"`. Keep output containing quotes, backslashes or multiple
lines out of `eval`; extract a simple value with `jq` or use `regex` instead.

The Python evaluator parses a restricted expression AST, resolves captured names
as values, and evaluates with built-ins disabled. It supports comparisons,
`and`, `or`, `not`, unary signs, and arithmetic including `//` and `%`; calls,
attribute access and indexing are rejected. Captured decimal integers remain
integers, finite numeric values become floats, and other output remains a string.

The Go evaluator uses whitespace-delimited tokens and preserves quoted string
literals. It supports comparisons, `and`, `or`, and `+`, `-`, `*`, `/`.
Multiplication and division bind before addition and subtraction; `and` binds
before `or`. Integer addition, subtraction and multiplication use `big.Int`;
division and decimal arithmetic use floating point. It compares integer values
first, then floating-point values, then strings. It does not implement the
Python evaluator's full grammar. Compute complex arithmetic in commands or use
separate verifiers when sharing a file between drivers.

For `regex`, both drivers match from the start of trimmed output. Anchor shared
patterns with `^` and `$` for a whole-output check and use syntax supported by
both; Go does not support Python lookaround or backreferences.

### Command status and timeouts

Both drivers require a successful command exit by default and fail a case when
a command exits unexpectedly or exceeds its time limit. Their optional input
fields differ:

| Driver | Expected exit field | Timeout |
| ------ | ------------------- | ------- |
| Python | `expected_status` (integer 0–255, default 0) | `timeout` per input, or the runner's 300-second default; must be positive and finite. |
| Go | `expected_exit_code` (integer 0–255, default 0) | `WithCommandTimeout`, default two minutes; `WithContext` also supports cancellation. |

Use the matching field when asserting a deliberate command failure. The other
driver does not interpret that field. Shell commands execute through Bash by
default; captured environment variables are passed as process arguments.

### Notes & Tips

1. Captures are reset for each case and passed to later commands as environment
   variables; they are not global across cases.
2. Commands and captured environment values are passed as separate process
   arguments. Shell syntax belongs in the command; captured values are not
   interpolated into that command by the driver.
3. Both drivers capture stdout and trim surrounding whitespace. Verifiers
   check that output after the command's expected exit status is satisfied.
4. Tests mutate a running chain and may not be idempotent. Preserve suite order
   and use fresh local state when a scenario requires it.
