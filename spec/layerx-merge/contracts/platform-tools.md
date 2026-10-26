# Platform tools contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## platform-test-support-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-platform-tests-tls-boundary-qualify-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tests/support/tls_boundary.rs:225</code> | <code>pub fn qualify(test_name: &amp;str, probe: impl Fn(&amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt;) {</code> |

## tools-benchmark-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-tools-benchmark-benchmark</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:26</code> | <code>struct Arguments {</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-capture-snapshot</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:36</code> | <code>capture_snapshot: bool,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-node</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:44</code> | <code>node: PathBuf,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-npm</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:42</code> | <code>npm: PathBuf,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-output</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:40</code> | <code>output: PathBuf,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-registry-url</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:38</code> | <code>registry_url: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-repository</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:28</code> | <code>repository: PathBuf,</code> |
| <code>interface.8.cli-tools-benchmark-benchmark-snapshot</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:34</code> | <code>snapshot: Option&lt;PathBuf&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-tools-benchmark-benchmark</code> | notes | 1937 callers total; no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-capture-snapshot</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-node</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-npm</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-output</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-registry-url</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-repository</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-tools-benchmark-benchmark-snapshot</code> | notes | no in-repo invocation outside the defining file |

## tools-benchmark-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-tools-benchmark-node-options</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:407</code> | <code>.env_remove("NODE_OPTIONS")</code> |
| <code>interface.8.env-tools-benchmark-node-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:406</code> | <code>.env_remove("NODE_PATH")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-audit</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:410</code> | <code>.env("NPM_CONFIG_AUDIT", "false")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-cache</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:409</code> | <code>.env("NPM_CONFIG_CACHE", current.join("cache"))</code> |
| <code>interface.8.env-tools-benchmark-npm-config-fetch-retries</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:413</code> | <code>.env("NPM_CONFIG_FETCH_RETRIES", "0")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-fetch-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:414</code> | <code>.env("NPM_CONFIG_FETCH_TIMEOUT", "60000")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-fund</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:411</code> | <code>.env("NPM_CONFIG_FUND", "false")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-ignore-scripts</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:412</code> | <code>.env("NPM_CONFIG_IGNORE_SCRIPTS", "true")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-offline</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:415</code> | <code>.env("NPM_CONFIG_OFFLINE", "false")</code> |
| <code>interface.8.env-tools-benchmark-npm-config-registry</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/tools/benchmark/src/main.rs:408</code> | <code>.env("NPM_CONFIG_REGISTRY", registry)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-tools-benchmark-npm-config-audit</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-cache</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-fetch-retries</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-fetch-timeout</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-fund</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-ignore-scripts</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-tools-benchmark-npm-config-offline</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-tools-benchmark-npm-config-registry</code> | notes | no in-repo setter or reader outside the defining file |

## tools-docs-gate-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-tools-docs-gate-docs-gate-repo-root</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/docs-gate:4</code> | <code>repo_root=${1:-$(CDPATH= cd -- "$(dirname -- "$0")/../../.." &amp;&amp; pwd)}</code> |

## tools-kvx-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-tools-kvx-entry-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:2</code> | <code>pub struct Entry {</code> |
| <code>interface.8.rs-tools-kvx-layerx-platform-kvx-document</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:9</code> | <code>pub struct Document {</code> |
| <code>interface.8.rs-tools-kvx-layerx-platform-kvx-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:58</code> | <code>pub fn parse(source: &amp;str) -&gt; Result&lt;Document, String&gt; {</code> |
| <code>interface.8.rs-tools-kvx-layerx-platform-kvx-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:149</code> | <code>pub fn quote(value: &amp;str) -&gt; String {</code> |
| <code>interface.8.rs-tools-kvx-layerx-platform-kvx-string-list</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:136</code> | <code>pub fn string_list(value: &amp;str) -&gt; Result&lt;Vec&lt;String&gt;, String&gt; {</code> |
| <code>interface.8.rs-tools-kvx-layerx-platform-kvx-unquote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/kvx/src/lib.rs:122</code> | <code>pub fn unquote(value: &amp;str) -&gt; Result&lt;String, String&gt; {</code> |

## tools-workspace-gate-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-tools-workspace-gate-platform-workspace-gate-manifest-path-repo-root</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/tools/workspace-gate/src/main.rs:102</code> | <code>let arguments = env::args().skip(1).collect::&lt;Vec&lt;_&gt;&gt;();</code> |

## tools-workspace-gate-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-tools-workspace-gate-workspace-gate-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/tools/workspace-gate/src/main.rs:28</code> | <code>pub fn workspace_gate(repo_root: &amp;Path, source: &amp;str) -&gt; Result&lt;(), String&gt; {</code> |
