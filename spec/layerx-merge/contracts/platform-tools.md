# Platform tools contract

The [platform tools](../../../services/platform/tools) provide benchmarking, documentation checks and workspace checks. [Platform test support](../../../services/platform/tests) supplies shared harness code. The emulator is implemented separately under [tools/platform/emulator](../../../tools/platform/emulator).

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## platform-test-support-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn qualify(test_name: &str, probe: impl Fn(&str) -> Result<Vec<u8>, String>) {` | [services/platform/tests/support/tls_boundary.rs](../../../services/platform/tests/support/tls_boundary.rs) |

## tools-benchmark-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `struct Arguments {` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `capture_snapshot: bool,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `node: PathBuf,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `npm: PathBuf,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `output: PathBuf,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `registry_url: Option<String>,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `repository: PathBuf,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `cli_command` | `snapshot: Option<PathBuf>,` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |

## tools-benchmark-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `NODE_OPTIONS` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NODE_PATH` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_AUDIT` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_CACHE` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_FETCH_RETRIES` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_FETCH_TIMEOUT` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_FUND` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_IGNORE_SCRIPTS` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_OFFLINE` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |
| `env_var` | `NPM_CONFIG_REGISTRY` | [services/platform/tools/benchmark/src/main.rs](../../../services/platform/tools/benchmark/src/main.rs) |

## tools-docs-gate-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `repo_root=${1:-$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)}` | [services/platform/tools/docs-gate](../../../services/platform/tools/docs-gate) |

## tools-kvx-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct Entry {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |
| `rust_pub_item` | `pub struct Document {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |
| `rust_pub_item` | `pub fn parse(source: &str) -> Result<Document, String> {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |
| `rust_pub_item` | `pub fn quote(value: &str) -> String {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |
| `rust_pub_item` | `pub fn string_list(value: &str) -> Result<Vec<String>, String> {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |
| `rust_pub_item` | `pub fn unquote(value: &str) -> Result<String, String> {` | [services/platform/tools/kvx/src/lib.rs](../../../services/platform/tools/kvx/src/lib.rs) |

## tools-workspace-gate-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `let arguments = env::args().skip(1).collect::<Vec<_>>();` | [services/platform/tools/workspace-gate/src/main.rs](../../../services/platform/tools/workspace-gate/src/main.rs) |

## tools-workspace-gate-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn workspace_gate(repo_root: &Path, source: &str) -> Result<(), String> {` | [services/platform/tools/workspace-gate/src/main.rs](../../../services/platform/tools/workspace-gate/src/main.rs) |

<a id="emulator-routes"></a>

See the implementation sources above.
