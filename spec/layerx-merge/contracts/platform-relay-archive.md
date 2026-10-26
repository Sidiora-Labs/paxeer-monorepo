# Platform relay archive contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## relay-archive-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-relay-archive-install-sh-allow-loopback-dev</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:276</code> | <code>--allow-loopback-dev) ALLOW_LOOPBACK_DEV=1; shift ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-allow-vendor-private-network</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:277</code> | <code>--allow-&lt;vendor&gt;-private-network) ALLOW_&lt;vendor&gt;_PRIVATE_NETWORK=1; shift ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-bundle</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:253</code> | <code>--bundle) require_value "$@"; BUNDLE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-ca-file</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:272</code> | <code>--ca-file) require_value "$@"; CA_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-codec</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:266</code> | <code>--codec) require_value "$@"; CODEC=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-config</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:258</code> | <code>--config) require_value "$@"; CONFIG=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-config-only</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:280</code> | <code>--config-only) CONFIG_ONLY=1; NO_SERVICE=1; shift ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-data-dir</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:263</code> | <code>--data-dir) require_value "$@"; DATA_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-expected-sha256</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:255</code> | <code>--expected-sha256) require_value "$@"; EXPECTED_SHA256=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-genesis-manifest</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:269</code> | <code>--genesis-manifest) require_value "$@"; GENESIS_MANIFEST=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-genesis-sha256</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:260</code> | <code>--genesis-sha256) require_value "$@"; GENESIS_SHA256=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-genesis-snapshot</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:270</code> | <code>--genesis-snapshot) require_value "$@"; GENESIS_SNAPSHOT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-listen</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:264</code> | <code>--listen) require_value "$@"; LISTEN=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-listener</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:278</code> | <code>--listener) require_value "$@"; LISTENER=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-manifest-sha256</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:256</code> | <code>--manifest-sha256) require_value "$@"; MANIFEST_SHA256=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-network-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:259</code> | <code>--network-id) require_value "$@"; NETWORK_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-no-service</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:279</code> | <code>--no-service) NO_SERVICE=1; shift ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-peer-seed</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:275</code> | <code>--peer-seed) require_value "$@"; PEER_SEEDS+=("$2"); shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-prefix</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:257</code> | <code>--prefix) require_value "$@"; PREFIX=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-public-url</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:265</code> | <code>--public-url) require_value "$@"; PUBLIC_URL=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-release</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:254</code> | <code>--release) require_value "$@"; RELEASE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-sequencer-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:261</code> | <code>--sequencer-id) require_value "$@"; SEQUENCER_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-sequencer-public-key</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:262</code> | <code>--sequencer-public-key) require_value "$@"; SEQUENCER_PUBLIC_KEY=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-source-log</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:271</code> | <code>--source-log) require_value "$@"; SOURCE_LOG=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-submission-upstream</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:268</code> | <code>--submission-upstream) require_value "$@"; SUBMISSION_UPSTREAMS+=("$2"); shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-tls-cert</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:273</code> | <code>--tls-cert) require_value "$@"; TLS_CERT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-tls-key</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:274</code> | <code>--tls-key) require_value "$@"; TLS_KEY=$2; shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-install-sh-upstream</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:267</code> | <code>--upstream) require_value "$@"; UPSTREAMS+=("$2"); shift 2 ;;</code> |
| <code>interface.8.cli-relay-archive-runtime-py-config</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1320</code> | <code>parser.add_argument("--config", required=True)</code> |
| <code>interface.8.cli-relay-archive-runtime-py-once</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1321</code> | <code>parser.add_argument("--once", action="store_true")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-relay-archive-install-sh-allow-loopback-dev</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-allow-vendor-private-network</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-bundle</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cli-relay-archive-install-sh-ca-file</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-codec</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-config</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-config-only</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cli-relay-archive-install-sh-data-dir</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-expected-sha256</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-genesis-manifest</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-genesis-sha256</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-genesis-snapshot</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-listen</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-listener</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-manifest-sha256</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-network-id</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-no-service</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-peer-seed</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-prefix</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-public-url</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-release</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-sequencer-id</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-sequencer-public-key</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-source-log</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-submission-upstream</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-tls-cert</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-tls-key</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-install-sh-upstream</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo invocation outside the defining file |
| <code>interface.8.cli-relay-archive-runtime-py-config</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cli-relay-archive-runtime-py-once</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |

## relay-archive-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-relay-archive-allow-loopback-dev</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:12</code> | <code>"allow_loopback_dev": false,</code> |
| <code>interface.8.cfg-relay-archive-ca-file</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:22</code> | <code>"ca_file": "/etc/ssl/certs/ca-certificates.crt",</code> |
| <code>interface.8.cfg-relay-archive-codec</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:11</code> | <code>"codec": "/opt/layerx/relay_archive/layerx-archive-codec",</code> |
| <code>interface.8.cfg-relay-archive-data-dir</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:8</code> | <code>"data_dir": "/var/lib/layerx/relay-archive",</code> |
| <code>interface.8.cfg-relay-archive-genesis-sha256</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:3</code> | <code>"genesis_sha256": "REPLACE_WITH_64_LOWERCASE_HEX",</code> |
| <code>interface.8.cfg-relay-archive-history-page-limit</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:28</code> | <code>"history_page_limit": 100,</code> |
| <code>interface.8.cfg-relay-archive-listen</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:9</code> | <code>"listen": "&lt;addr&gt;",</code> |
| <code>interface.8.cfg-relay-archive-max-activity-bytes</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:25</code> | <code>"max_activity_bytes": 1048576,</code> |
| <code>interface.8.cfg-relay-archive-max-batch-bytes</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:26</code> | <code>"max_batch_bytes": 16777216,</code> |
| <code>interface.8.cfg-relay-archive-max-concurrency</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:30</code> | <code>"max_concurrency": 8,</code> |
| <code>interface.8.cfg-relay-archive-max-history-page-limit</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:29</code> | <code>"max_history_page_limit": 500,</code> |
| <code>interface.8.cfg-relay-archive-max-response-bytes</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:27</code> | <code>"max_response_bytes": 17825792,</code> |
| <code>interface.8.cfg-relay-archive-network-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:2</code> | <code>"network_id": 402,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:31</code> | <code>"peer_discovery": {</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-advertise-ttl-seconds</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:36</code> | <code>"advertise_ttl_seconds": 300,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-allow-loopback-dev</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:12</code> | <code>"allow_loopback_dev": false,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-enabled</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:32</code> | <code>"enabled": true,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-max-advertised-peers</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:39</code> | <code>"max_advertised_peers": 32,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-max-peers</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:38</code> | <code>"max_peers": 64,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-refresh-interval-seconds</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:37</code> | <code>"refresh_interval_seconds": 60,</code> |
| <code>interface.8.cfg-relay-archive-peer-discovery-seeds</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:33</code> | <code>"seeds": [</code> |
| <code>interface.8.cfg-relay-archive-poll-interval-seconds</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:23</code> | <code>"poll_interval_seconds": 1,</code> |
| <code>interface.8.cfg-relay-archive-public-url</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:10</code> | <code>"public_url": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-relay-archive-request-timeout-seconds</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:24</code> | <code>"request_timeout_seconds": 5,</code> |
| <code>interface.8.cfg-relay-archive-sequencer-first-batch</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:6</code> | <code>"sequencer_first_batch": 1,</code> |
| <code>interface.8.cfg-relay-archive-sequencer-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:4</code> | <code>"sequencer_id": "REPLACE_WITH_64_LOWERCASE_HEX",</code> |
| <code>interface.8.cfg-relay-archive-sequencer-last-batch</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:7</code> | <code>"sequencer_last_batch": 18446744073709551615,</code> |
| <code>interface.8.cfg-relay-archive-sequencer-public-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:5</code> | <code>"sequencer_public_key": "REPLACE_WITH_64_LOWERCASE_HEX",</code> |
| <code>interface.8.cfg-relay-archive-submission-upstreams</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:17</code> | <code>"submission_upstreams": [</code> |
| <code>interface.8.cfg-relay-archive-tls-cert</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:20</code> | <code>"tls_cert": "/etc/layerx/tls/relay.crt",</code> |
| <code>interface.8.cfg-relay-archive-tls-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:21</code> | <code>"tls_key": "/etc/layerx/tls/relay.key",</code> |
| <code>interface.8.cfg-relay-archive-upstreams</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/relay_archive/config.example.json:13</code> | <code>"upstreams": [</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-relay-archive-allow-loopback-dev</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-ca-file</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 53 callers total |
| <code>interface.8.cfg-relay-archive-codec</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-data-dir</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 64 callers total |
| <code>interface.8.cfg-relay-archive-genesis-sha256</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 87 callers total |
| <code>interface.8.cfg-relay-archive-history-page-limit</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-listen</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 74 callers total |
| <code>interface.8.cfg-relay-archive-max-activity-bytes</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-max-batch-bytes</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-max-concurrency</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-max-history-page-limit</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-max-response-bytes</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-network-id</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 335 callers total |
| <code>interface.8.cfg-relay-archive-peer-discovery</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-advertise-ttl-seconds</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-allow-loopback-dev</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-enabled</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 642 callers total |
| <code>interface.8.cfg-relay-archive-peer-discovery-max-advertised-peers</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-max-peers</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-refresh-interval-seconds</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-peer-discovery-seeds</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 65 callers total |
| <code>interface.8.cfg-relay-archive-poll-interval-seconds</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-public-url</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 32 callers total |
| <code>interface.8.cfg-relay-archive-request-timeout-seconds</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-sequencer-first-batch</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-sequencer-id</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 138 callers total |
| <code>interface.8.cfg-relay-archive-sequencer-last-batch</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-sequencer-public-key</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 218 callers total |
| <code>interface.8.cfg-relay-archive-submission-upstreams</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-tls-cert</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-tls-key</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.cfg-relay-archive-upstreams</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |

## relay-archive-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-relay-archive-layerx-relay-archive-allow-loopback-dev</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:142</code> | <code>case "${LAYERX_RELAY_ARCHIVE_ALLOW_LOOPBACK_DEV:-0}" in</code> |
| <code>interface.8.env-relay-archive-layerx-relay-archive-allow-vendor-private-network</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/relay_archive/install.sh:147</code> | <code>case "${LAYERX_RELAY_ARCHIVE_ALLOW_&lt;vendor&gt;_PRIVATE_NETWORK:-0}" in</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-relay-archive-layerx-relay-archive-allow-loopback-dev</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.env-relay-archive-layerx-relay-archive-allow-vendor-private-network</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |

## relay-archive-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-relay-archive-32004</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1212</code> | <code>return _rpc_error(request_id, -32004, "Not found")</code> |
| <code>interface.8.err-relay-archive-32600</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1156</code> | <code>return _rpc_error(None, -32600, "Invalid Request")</code> |
| <code>interface.8.err-relay-archive-32602</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1210</code> | <code>return _rpc_error(request_id, -32602, "Invalid params")</code> |
| <code>interface.8.err-relay-archive-32700</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1154</code> | <code>return _rpc_error(None, -32700, "Parse error")</code> |
| <code>interface.8.err-relay-archive-activity-not-found</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1030</code> | <code>raise RequestError(404, "activity_not_found")</code> |
| <code>interface.8.err-relay-archive-archive-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:877</code> | <code>status, content_type, body, extra = _error_response(503, "archive_unavailable", str(error))</code> |
| <code>interface.8.err-relay-archive-batch-not-found</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:989</code> | <code>raise RequestError(404, "batch_not_found")</code> |
| <code>interface.8.err-relay-archive-conflict</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:93</code> | <code>HTTP_STATUS = {"malformed": 400, "policy": 403, "missing": 404, "conflict": 409, "corrupt": 422,</code> |
| <code>interface.8.err-relay-archive-content-length-required</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:918</code> | <code>raise RequestError(411, "content_length_required")</code> |
| <code>interface.8.err-relay-archive-corrupt</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:93</code> | <code>HTTP_STATUS = {"malformed": 400, "policy": 403, "missing": 404, "conflict": 409, "corrupt": 422,</code> |
| <code>interface.8.err-relay-archive-duplicate-header</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:905</code> | <code>raise RequestError(400, "duplicate_header")</code> |
| <code>interface.8.err-relay-archive-idempotency-conflict</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:889</code> | <code>status, content_type, answer, extra = _error_response(409, "idempotency_conflict", str(error))</code> |
| <code>interface.8.err-relay-archive-internal-error</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:879</code> | <code>status, content_type, body, extra = _error_response(500, "internal_error", None)</code> |
| <code>interface.8.err-relay-archive-invalid-account</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1018</code> | <code>raise RequestError(400, "invalid_account")</code> |
| <code>interface.8.err-relay-archive-invalid-activity</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1134</code> | <code>raise RequestError(400 if not body else 413, "invalid_activity")</code> |
| <code>interface.8.err-relay-archive-invalid-activity-id</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1027</code> | <code>raise RequestError(400, "invalid_activity_id")</code> |
| <code>interface.8.err-relay-archive-invalid-batch</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1015</code> | <code>raise RequestError(400, "invalid_batch") from error</code> |
| <code>interface.8.err-relay-archive-invalid-content-length</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:922</code> | <code>raise RequestError(400, "invalid_content_length") from error</code> |
| <code>interface.8.err-relay-archive-invalid-cursor</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1083</code> | <code>raise RequestError(400, "invalid_cursor")</code> |
| <code>interface.8.err-relay-archive-invalid-header</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:909</code> | <code>raise RequestError(400, "invalid_header")</code> |
| <code>interface.8.err-relay-archive-invalid-limit</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1116</code> | <code>raise RequestError(400, "invalid_limit")</code> |
| <code>interface.8.err-relay-archive-invalid-module</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1007</code> | <code>raise RequestError(400, "invalid_module") from error</code> |
| <code>interface.8.err-relay-archive-invalid-path</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:957</code> | <code>raise RequestError(400, "invalid_path")</code> |
| <code>interface.8.err-relay-archive-invalid-query</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1093</code> | <code>raise RequestError(400, "invalid_query")</code> |
| <code>interface.8.err-relay-archive-invalid-request</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:893</code> | <code>status, content_type, answer, extra = _error_response(400, "invalid_request", str(error))</code> |
| <code>interface.8.err-relay-archive-invalid-upstream-response</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/forward.py:217</code> | <code>"state": "unknown", "error": {"code": "invalid_upstream_response"}</code> |
| <code>interface.8.err-relay-archive-maintenance-not-found</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1086</code> | <code>raise RequestError(404, "maintenance_not_found")</code> |
| <code>interface.8.err-relay-archive-missing</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:93</code> | <code>HTTP_STATUS = {"malformed": 400, "policy": 403, "missing": 404, "conflict": 409, "corrupt": 422,</code> |
| <code>interface.8.err-relay-archive-native-submit-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:717</code> | <code>{"state": "unknown", "error": {"code": "native_submit_unavailable"}}</code> |
| <code>interface.8.err-relay-archive-not-found</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1088</code> | <code>raise RequestError(404, "not_found")</code> |
| <code>interface.8.err-relay-archive-policy</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:93</code> | <code>HTTP_STATUS = {"malformed": 400, "policy": 403, "missing": 404, "conflict": 409, "corrupt": 422,</code> |
| <code>interface.8.err-relay-archive-receipt-not-found</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1066</code> | <code>raise RequestError(404, "receipt_not_found")</code> |
| <code>interface.8.err-relay-archive-request-too-large</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:924</code> | <code>raise RequestError(413, "request_too_large")</code> |
| <code>interface.8.err-relay-archive-response-too-large</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:939</code> | <code>413, "response_too_large", None</code> |
| <code>interface.8.err-relay-archive-submission-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/forward.py:299</code> | <code>"code": "submission_unavailable",</code> |
| <code>interface.8.err-relay-archive-transfer-encoding-refused</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:915</code> | <code>raise RequestError(400, "transfer_encoding_refused")</code> |
| <code>interface.8.err-relay-archive-truncated-request</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:927</code> | <code>raise RequestError(400, "truncated_request")</code> |
| <code>interface.8.err-relay-archive-unsupported-media-type</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1132</code> | <code>raise RequestError(415, "unsupported_media_type")</code> |
| <code>interface.8.err-relay-archive-upstream-identity-mismatch</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:776</code> | <code>"error": {"code": "upstream_identity_mismatch"},</code> |
| <code>interface.8.err-relay-archive-verified-archive-receipt-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:812</code> | <code>"reason": "verified_archive_receipt_unavailable",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-relay-archive-32004</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-32600</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 40 callers total |
| <code>interface.8.err-relay-archive-32602</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 443 callers total |
| <code>interface.8.err-relay-archive-32700</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-activity-not-found</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-archive-unavailable</code> | notes | HTTP 503; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-batch-not-found</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-conflict</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 119 callers total |
| <code>interface.8.err-relay-archive-content-length-required</code> | notes | HTTP 411; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-corrupt</code> | notes | HTTP 409; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 40 callers total |
| <code>interface.8.err-relay-archive-duplicate-header</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-idempotency-conflict</code> | notes | HTTP 409; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 30 callers total |
| <code>interface.8.err-relay-archive-internal-error</code> | notes | HTTP 500; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-account</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-activity</code> | notes | HTTP 413; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-activity-id</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-batch</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-content-length</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-invalid-cursor</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-header</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-invalid-limit</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-module</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-invalid-path</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-query</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-invalid-request</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 78 callers total |
| <code>interface.8.err-relay-archive-invalid-upstream-response</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-maintenance-not-found</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-missing</code> | notes | HTTP 403; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 163 callers total |
| <code>interface.8.err-relay-archive-native-submit-unavailable</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-not-found</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 135 callers total |
| <code>interface.8.err-relay-archive-policy</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 138 callers total |
| <code>interface.8.err-relay-archive-receipt-not-found</code> | notes | HTTP 404; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-request-too-large</code> | notes | HTTP 413; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-response-too-large</code> | notes | HTTP 413; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-submission-unavailable</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-transfer-encoding-refused</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-truncated-request</code> | notes | HTTP 400; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-unsupported-media-type</code> | notes | HTTP 415; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.err-relay-archive-upstream-identity-mismatch</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-relay-archive-verified-archive-receipt-unavailable</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo consumer quotes the code outside the defining file |

## relay-archive-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-relay-archive-archive-schema</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/relay_archive/store.py:43</code> | <code>CREATE TABLE IF NOT EXISTS archive_meta (</code> |
| <code>interface.8.fmt-relay-archive-lxs-state-header</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:80</code> | <code>LXS_HEADER_BYTES = {b"LXS2": 116, b"LXS3": 116 + 315}</code> |
| <code>interface.8.fmt-relay-archive-proof-bundle</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:77</code> | <code>PROOF_MAGIC = b"LXPB"</code> |
| <code>interface.8.fmt-relay-archive-state-schema</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/relay_archive/store.py:1052</code> | <code>CREATE TABLE IF NOT EXISTS state_meta (</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-relay-archive-archive-schema</code> | notes | archive SQL schema; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; read and written only by the defining module |
| <code>interface.8.fmt-relay-archive-lxs-state-header</code> | notes | LXS2 and LXS3 state headers; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.fmt-relay-archive-proof-bundle</code> | notes | proof bundle magic; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.fmt-relay-archive-state-schema</code> | notes | state archive SQL schema; stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; read and written only by the defining module |

## relay-archive-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-relay-archive-any-healthz</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:960</code> | <code>if path == "/healthz":</code> |
| <code>interface.8.http-relay-archive-any-readyz</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:962</code> | <code>if path == "/readyz":</code> |
| <code>interface.8.http-relay-archive-any-rpc</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/forward.py:254</code> | <code>if path == "/rpc":</code> |
| <code>interface.8.http-relay-archive-any-v1-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1129</code> | <code>if parsed.path == "/v1/activities":</code> |
| <code>interface.8.http-relay-archive-any-v1-history-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:996</code> | <code>if path == "/v1/history/activities":</code> |
| <code>interface.8.http-relay-archive-any-v1-history-batches</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1032</code> | <code>if path == "/v1/history/batches":</code> |
| <code>interface.8.http-relay-archive-any-v1-history-maintenance</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1068</code> | <code>if path == "/v1/history/maintenance":</code> |
| <code>interface.8.http-relay-archive-any-v1-history-receipts</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1048</code> | <code>if path == "/v1/history/receipts":</code> |
| <code>interface.8.http-relay-archive-any-v1-peers</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:965</code> | <code>if path == "/v1/peers":</code> |
| <code>interface.8.http-relay-archive-any-v1-state-archive-identity</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:590</code> | <code>if self.path == "/v1/state-archive/identity":</code> |
| <code>interface.8.http-relay-archive-any-v1-state-archive-units</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/state_archive.py:592</code> | <code>if self.path == "/v1/state-archive/units":</code> |
| <code>interface.8.http-relay-archive-any-v1-sync-head</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:972</code> | <code>if path == "/v1/sync/head":</code> |
| <code>interface.8.http-relay-archive-any-v1-sync-network</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:970</code> | <code>if path == "/v1/sync/network":</code> |
| <code>interface.8.http-relay-archive-any-v1-sync-readiness</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:967</code> | <code>if path == "/v1/sync/readiness":</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-relay-archive-any-healthz</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 331 callers total |
| <code>interface.8.http-relay-archive-any-readyz</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 324 callers total |
| <code>interface.8.http-relay-archive-any-rpc</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 610 callers total |
| <code>interface.8.http-relay-archive-any-v1-activities</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 104 callers total |
| <code>interface.8.http-relay-archive-any-v1-history-activities</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-history-batches</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-history-maintenance</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-history-receipts</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-peers</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-state-archive-identity</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-relay-archive-any-v1-state-archive-units</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-relay-archive-any-v1-sync-head</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 32 callers total |
| <code>interface.8.http-relay-archive-any-v1-sync-network</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.http-relay-archive-any-v1-sync-readiness</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |

## relay-archive-jsonrpc-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rpc-relay-archive-lx-getactivitystatus</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1221</code> | <code>if method == "lx_getActivityStatus" and len(params) == 1 and isinstance(params[0], str):</code> |
| <code>interface.8.rpc-relay-archive-lx-getarchivehead</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1219</code> | <code>if method == "lx_getArchiveHead" and not params:</code> |
| <code>interface.8.rpc-relay-archive-lx-getbatchheader</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1232</code> | <code>if method == "lx_getBatchHeader" and len(params) == 1:</code> |
| <code>interface.8.rpc-relay-archive-lx-getreceipt</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1227</code> | <code>if method == "lx_getReceipt" and len(params) == 1 and isinstance(params[0], str):</code> |
| <code>interface.8.rpc-relay-archive-lx-listactivities</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1239</code> | <code>if method == "lx_listActivities" and len(params) &lt;= 1:</code> |
| <code>interface.8.rpc-relay-archive-lx-listbatches</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1259</code> | <code>if method == "lx_listBatches" and len(params) &lt;= 1:</code> |
| <code>interface.8.rpc-relay-archive-lx-sendactivity</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>services/platform/relay_archive/runtime.py:1167</code> | <code>if method == "lx_sendActivity":</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rpc-relay-archive-lx-getactivitystatus</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 28 callers total |
| <code>interface.8.rpc-relay-archive-lx-getarchivehead</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.rpc-relay-archive-lx-getbatchheader</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD |
| <code>interface.8.rpc-relay-archive-lx-getreceipt</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 45 callers total |
| <code>interface.8.rpc-relay-archive-lx-listactivities</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo caller outside the defining file |
| <code>interface.8.rpc-relay-archive-lx-listbatches</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; no in-repo caller outside the defining file |
| <code>interface.8.rpc-relay-archive-lx-sendactivity</code> | notes | stale task 11.7.23 (spec/layerx-merge/spec.kvx:64199) deletes the relay archive; present at HEAD; 68 callers total |
