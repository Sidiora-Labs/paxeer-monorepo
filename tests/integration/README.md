# Integration Test Framework

YAML-driven integration tests for `paxd`. Commands run sequentially in a local
Docker cluster and verifiers check their captured output. The Python and Go
drivers consume the same YAML files, but their expression evaluators differ.

Paxeer mainnet-beta is one BFT network, with the kernel participating in block
proposal. A receipt is final when its block is committed. Its EVM chain ID is
125 (`0x7d`). Public JSON-RPC uses the root URL of the node hosts
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network` and `api-mainnet-beta.paxeer.network`, for
example `https://api1.mainnet-beta.paxeer.network/`. The commands below target a
disposable local cluster; local genesis settings can differ from mainnet.

Related suites:

- [dApp scenarios](dapp_tests/README.md): EVM, CosmWasm and pointer interactions.
- [EVM RPC fixtures](evm_module/rpc_io_test/RPC_IO_README.md): `.io` / `.iox` response-kind checks.
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
shows the field layout; use the common expression subset below rather than its
unrestricted Python-eval placeholder.

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

The evaluators are not interchangeable:

- Python substitutes whitespace-delimited captured names and calls Python
  `eval`. Values parseable as numbers are inserted unquoted; other values are
  wrapped in double quotes without escaping. Python syntax and precedence apply.
- Go tokenizes comparisons by whitespace. It compares decimal integers with
  `big.Int`, then tries floating point, then string comparison. Integer
  arithmetic supports `+`, `-`, `*`, `/` and evaluates left to right; division
  is integer division. Python uses normal arithmetic precedence and `/` returns
  a floating-point value.
- Go splits on ` and ` before ` or `, giving `or` tighter binding in mixed
  expressions. Python gives `and` tighter binding. For example,
  `1 == 1 or 1 == 2 and 1 == 2` evaluates differently. Go does not implement
  Python calls, indexing, general parentheses or chained comparisons.

Use separate verifiers for conjunctions and compute arithmetic in the command
when sharing a file between drivers. Avoid relying on either evaluator's wider
grammar.

For `regex`, Python uses `re.match` (start of output), while Go uses
`regexp.MatchString` (anywhere in output). Anchor shared patterns with `^` and
`$` and use syntax supported by both; Go does not support Python lookaround or
backreferences.

### Notes & Tips

1. Captures are reset for each case and passed to later commands as environment
   variables; they are not global across cases.
2. Python builds a shell string with single-quoted command and environment
   values. Avoid embedded single quotes in commands and captured output. Go
   passes Docker arguments directly, but the command still runs in a shell.
3. Both drivers capture stdout and trim surrounding whitespace. A nonzero
   command exit alone does not fail a case; capture and verify the intended
   response or exit status explicitly. Go reports command exits in its log.
4. Tests mutate a running chain and may not be idempotent. Preserve suite order
   and use fresh local state when a scenario requires it.
