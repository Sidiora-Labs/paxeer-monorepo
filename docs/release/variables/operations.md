# Backup and monitoring operations variables

The backup and monitoring system is maintained under `tools/ops`; these variables configure its backup, keyring migration, monitoring target rendering, and alert delivery commands.

## Required

| Name | Sets | Default or configured value | Secret | Source |
| --- | --- | --- | --- | --- |
| `BACKUP_DIR` | Archive store directory | none | No | `tools/ops/backup/lib.sh` |
| `BACKUP_IDENTITY` | Age identity file used to decrypt and validate archives | none | No; path to secret material | `tools/ops/backup/backup.sh`, `tools/ops/backup/restore.sh`, `tools/ops/keyring-migrate.sh` |
| `BACKUP_KERNEL_DIR` | Kernel state directory captured with the node home | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_NODE_HOME` | Node home captured and used for isolated validation | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_QUIESCE_CMD` | Command that stops and flushes every backup source writer | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_RECIPIENTS_FILE` | Age recipients file used to encrypt archives | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_REQUIRED_KEYS` | Nonempty space-separated additional required key paths; standard node keys and signing state are always included | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_RESUME_CMD` | Command that releases the backup quiesce hold | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_SOURCES` | Space-separated absolute source paths to capture | none | No | `tools/ops/backup/backup.sh` |
| `BACKUP_VALIDATE_CMD` | Command that validates recovered committed state and keys | none | No | `tools/ops/backup/backup.sh` |
| `HOSTS` | Space-separated unique name=address pairs for monitored nodes | empty (rejected) | No | `tools/ops/monitoring/render.sh`, `tools/ops/monitoring/readiness.py` |
| `KEYRING_PASSPHRASE_FILE` | File containing the keyring migration passphrase | none | No; path to secret material | `tools/ops/keyring-migrate.sh` |

`ALERT_WEBHOOK_URL` or `ALERT_EMAIL_TO` must be nonempty; both routes may be enabled. Email delivery also requires `ALERT_EMAIL_FROM` and `ALERT_SMTP_SMARTHOST`. Backup creation requires all backup settings above; restore and keyring migration use only `BACKUP_DIR`, `BACKUP_IDENTITY`, and the shared loader settings. Migration additionally requires `KEYRING_PASSPHRASE_FILE`. File-path values are not secret bytes; protect the identity and passphrase files themselves.

## Optional

| Name | Sets | Default or configured value | Secret | Source |
| --- | --- | --- | --- | --- |
| `<node-home>/config/client.toml: [root] keyring-backend` | Backend selection checked before removing a test keyring; a selection of test keeps it in use | No default applied by migration | No | `tools/ops/keyring-migrate.sh` |
| `AI_EVALUATOR_METRICS_URLS` | Space-separated AI evaluator metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `ALERT_EMAIL_FROM` | Sender address for the optional email route | empty | No | `tools/ops/monitoring/render.sh` |
| `ALERT_EMAIL_TO` | Recipient address that enables email alerts | empty | No | `tools/ops/monitoring/render.sh` |
| `ALERT_SMTP_PASSWORD` | SMTP password written to the Alertmanager secret file | empty | Yes | `tools/ops/monitoring/render.sh` |
| `ALERT_SMTP_SMARTHOST` | SMTP host and port for the optional email route | empty | No | `tools/ops/monitoring/render.sh` |
| `ALERT_SMTP_USERNAME` | SMTP username for the optional email route | empty | No | `tools/ops/monitoring/render.sh` |
| `ALERT_WEBHOOK_URL` | Webhook destination written to the Alertmanager secret file | empty | Yes | `tools/ops/monitoring/render.sh` |
| `AUTHORITY_METRICS_URLS` | Space-separated receipt-authority metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `BACKUP_DB_DUMP_CMD` | Command whose standard output is included as the wallet database dump | empty | No | `tools/ops/backup/backup.sh` |
| `BACKUP_ENV` | Backup environment file selected by the unit and loader; `[Service] Environment=BACKUP_ENV` | `/etc/paxeer/backup.env` | No | `tools/ops/backup/paxeer-backup.service` (`[Service] Environment`), `tools/ops/backup/lib.sh` |
| `BACKUP_HOST` | Archive store host; empty runs the store locally | empty | No | `tools/ops/backup/lib.sh` |
| `BACKUP_LOCK_FILE` | Lock file serializing backup runs | `/run/lock/paxeer-backup.lock` | No | `tools/ops/backup/backup.sh` |
| `BACKUP_PREFIX` | Archive name prefix | `$(hostname -s)` | No | `tools/ops/backup/lib.sh` |
| `BACKUP_RETENTION` | Number of archives retained per prefix | `14` | No | `tools/ops/backup/lib.sh` |
| `BACKUP_SSH` | SSH command used for a remote archive store | `ssh -o BatchMode=yes` | No | `tools/ops/backup/lib.sh` |
| `COMET_METRICS_PORT` | CometBFT metrics port scraped on each host | `26660` | No | `tools/ops/monitoring/render.sh` |
| `COMET_RPC_PORT` | Paxd consensus RPC port used for readiness probes | `26657` | No | `tools/ops/monitoring/render.sh`, `tools/ops/monitoring/readiness.py` |
| `EDGE_CERT_URLS` | Space-separated HTTPS endpoints whose certificates are checked | empty | No | `tools/ops/monitoring/render.sh` |
| `EXPLORER_INDEX_HEAD_URLS` | Space-separated explorer index head endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `FEEDER_METRICS_URLS` | Space-separated feeder metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `KEEPER_METRICS_URLS` | Space-separated keeper metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `NODE_EXPORTER_PORT` | Node exporter port scraped on each host | `9100` | No | `tools/ops/monitoring/render.sh` |
| `PAXAI_WORKER_METRICS_URLS` | Space-separated PaxAI worker metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `PAXD` | Node binary used by keyring migration | `paxd` | No | `tools/ops/keyring-migrate.sh` |
| `STREAM_METER_PROVIDER_URLS` | Space-separated stream-meter provider health endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `WALLET_ATTESTOR_METRICS_URLS` | Space-separated wallet-attestor metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `X_WEBSEARCH_METRICS_URLS` | Space-separated web-search metrics endpoints | empty | No | `tools/ops/monitoring/render.sh` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].auth_password_file` | File containing the SMTP password; email settings emitted when email delivery is enabled | `/etc/alertmanager/secrets/smtp_password` | No; referenced file is secret | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].auth_username` | SMTP authentication username | From `ALERT_SMTP_USERNAME` (value omitted) | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].from` | Email sender | From `ALERT_EMAIL_FROM` (value omitted) | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].send_resolved` | Send resolved email notifications | `true` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].smarthost` | SMTP destination | From `ALERT_SMTP_SMARTHOST` (value omitted) | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].email_configs[0].to` | Email recipient | From `ALERT_EMAIL_TO` (value omitted) | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].name` | Receiver name | `fleet` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].webhook_configs[0].send_resolved` | Send resolved webhook notifications | `true` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: receivers[0].webhook_configs[0].url_file` | File containing the webhook URL; emitted when webhook delivery is enabled | `/etc/alertmanager/secrets/webhook_url` | No; referenced file is secret | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: route.group_by` | Alert grouping labels | `alertname`, `host` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: route.group_interval` | Delay between grouped notifications | `5m` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: route.group_wait` | Initial notification delay | `30s` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: route.receiver` | Default alert receiver | `fleet` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/alertmanager.yml: route.repeat_interval` | Repeat notification interval | `4h` | No | `tools/ops/monitoring/render.sh; consumed by Alertmanager via tools/ops/monitoring/docker-compose.yml` |
| `rendered/targets/*.json: [].labels.host` | Host name derived from HOSTS for node targets | Generated per target group; host and role labels are emitted only for their corresponding target type | No | `tools/ops/monitoring/render.sh, tools/ops/monitoring/readiness.py; consumed by Prometheus via tools/ops/monitoring/prometheus.yml` |
| `rendered/targets/*.json: [].labels.role` | Service-probe role for explorer index head and stream-meter targets | Generated per target group; host and role labels are emitted only for their corresponding target type | No | `tools/ops/monitoring/render.sh, tools/ops/monitoring/readiness.py; consumed by Prometheus via tools/ops/monitoring/prometheus.yml` |
| `rendered/targets/*.json: [].targets` | Endpoint list derived from the matching host or URL variable | Generated per target group; host and role labels are emitted only for their corresponding target type | No | `tools/ops/monitoring/render.sh, tools/ops/monitoring/readiness.py; consumed by Prometheus via tools/ops/monitoring/prometheus.yml` |
| `tools/ops/backup/paxeer-backup.service: [Service] ExecStart` | Backup startup command | `/opt/paxeer/tools/ops/backup/backup.sh` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Service] IOSchedulingClass` | Service I/O scheduling class | `idle` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Service] Nice` | Service CPU scheduling priority | `10` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Service] Type` | Service execution type | `oneshot` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Unit] After` | Unit startup ordering | `network-online.target` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Unit] Description` | Systemd unit description | `Paxeer encrypted fleet backup` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.service: [Unit] Wants` | Wanted unit dependency | `network-online.target` | No | `tools/ops/backup/paxeer-backup.service` |
| `tools/ops/backup/paxeer-backup.timer: [Install] WantedBy` | Timer installation target | `timers.target` | No | `tools/ops/backup/paxeer-backup.timer` |
| `tools/ops/backup/paxeer-backup.timer: [Timer] OnCalendar` | Backup schedule | `*-*-* 03:15:00 UTC` | No | `tools/ops/backup/paxeer-backup.timer` |
| `tools/ops/backup/paxeer-backup.timer: [Timer] Persistent` | Run missed timer events after activation | `true` | No | `tools/ops/backup/paxeer-backup.timer` |
| `tools/ops/backup/paxeer-backup.timer: [Timer] RandomizedDelaySec` | Schedule jitter | `15min` | No | `tools/ops/backup/paxeer-backup.timer` |
| `tools/ops/backup/paxeer-backup.timer: [Unit] Description` | Systemd unit description | `Nightly Paxeer encrypted fleet backup` | No | `tools/ops/backup/paxeer-backup.timer` |
| `tools/ops/monitoring/blackbox.yml: modules.http_2xx.http.preferred_ip_protocol` | Preferred IP protocol | `ip4` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.http_2xx.http.valid_status_codes` | Accepted HTTP status codes; empty accepts 2xx | `[]` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.http_2xx.prober` | Blackbox probe protocol | `http` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.http_2xx.timeout` | Probe timeout | `10s` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.node_consensus.http.fail_if_body_not_matches_regexp` | Required response body patterns | `["\"catching_up\"\\s*:\\s*false", "\"latest_block_height\"\\s*:\\s*\"[1-9][0-9]*\""]` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.node_consensus.http.preferred_ip_protocol` | Preferred IP protocol | `ip4` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.node_consensus.http.valid_status_codes` | Accepted HTTP status codes; empty accepts 2xx | `[200]` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.node_consensus.prober` | Blackbox probe protocol | `http` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.node_consensus.timeout` | Probe timeout | `10s` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.tls_cert.prober` | Blackbox probe protocol | `tcp` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.tls_cert.tcp.preferred_ip_protocol` | Preferred IP protocol | `ip4` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.tls_cert.tcp.tls` | TLS negotiation | `true` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/blackbox.yml: modules.tls_cert.timeout` | Probe timeout | `10s` | No | `tools/ops/monitoring/blackbox.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.command` | Container startup flags | `["--config.file=/etc/alertmanager/alertmanager.yml", "--storage.path=/alertmanager"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.command --config.file` | Alertmanager configuration file | `/etc/alertmanager/alertmanager.yml` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.command --storage.path` | Alertmanager persistent storage directory | `/alertmanager` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.image` | Container image and version | `prom/alertmanager:v0.27.0` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.ports` | Published ports and bind interface | Loopback binding on port `9093` (address omitted) | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.restart` | Container restart policy | `unless-stopped` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.alertmanager.volumes` | Storage mounts | `["./rendered/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro", "./rendered/secrets:/etc/alertmanager/secrets:ro", "alertmanager-data:/alertmanager"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.blackbox.command` | Container startup flags | `["--config.file=/etc/blackbox/blackbox.yml"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.blackbox.command --config.file` | Blackbox exporter configuration file | `/etc/blackbox/blackbox.yml` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.blackbox.image` | Container image and version | `prom/blackbox-exporter:v0.25.0` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.blackbox.restart` | Container restart policy | `unless-stopped` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.blackbox.volumes` | Storage mounts | `["./blackbox.yml:/etc/blackbox/blackbox.yml:ro"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.command` | Container startup flags | `["--path.rootfs=/host"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.command --path.rootfs` | Host root filesystem mount used by node exporter | `/host` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.image` | Container image and version | `prom/node-exporter:v1.8.2` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.pid` | Container PID namespace | `host` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.restart` | Container restart policy | `unless-stopped` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.node-exporter.volumes` | Storage mounts | `["/:/host:ro,rslave"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.command` | Container startup flags | `["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus", "--storage.tsdb.retention.time=30d"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.command --config.file` | Prometheus configuration file | `/etc/prometheus/prometheus.yml` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.command --storage.tsdb.path` | Prometheus persistent storage directory | `/prometheus` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.command --storage.tsdb.retention.time` | Prometheus sample retention period | `30d` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.image` | Container image and version | `prom/prometheus:v2.55.1` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.ports` | Published ports and bind interface | Loopback binding on port `9090` (address omitted) | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.restart` | Container restart policy | `unless-stopped` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: services.prometheus.volumes` | Storage mounts | `["./rendered/prometheus.yml:/etc/prometheus/prometheus.yml:ro", "./rules.yml:/etc/prometheus/rules.yml:ro", "./rendered/targets:/etc/prometheus/targets:ro", "prometheus-data:/prometheus"]` | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: volumes.alertmanager-data` | Named Alertmanager storage volume | Empty mapping | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/docker-compose.yml: volumes.prometheus-data` | Named Prometheus storage volume | Empty mapping | No | `tools/ops/monitoring/docker-compose.yml` |
| `tools/ops/monitoring/prometheus.yml: alerting.alertmanagers[0].static_configs[0].targets` | Scrape, probe, or alert delivery targets | Configured service targets (addresses omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: global.evaluation_interval` | Alert evaluation interval | `15s` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: global.scrape_interval` | Metrics scrape interval | `15s` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: rule_files` | Alert rule files | `["/etc/prometheus/rules.yml"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[0].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/cometbft.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[0].job_name` | Scrape job identity | `cometbft` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/ai-evaluator-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].job_name` | Scrape job identity | `ai-evaluator` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[10].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/x-websearch-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].job_name` | Scrape job identity | `x-websearch` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[11].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/wallet-attestor-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].job_name` | Scrape job identity | `wallet-attestors` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[12].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/explorer-index-head.json", "/etc/prometheus/targets/stream-meter-providers.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].job_name` | Scrape job identity | `service-probe` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].metrics_path` | HTTP metrics or probe path | `/probe` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].params.module` | Blackbox probe module | `["http_2xx"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[0].target_label` | Relabel output label | `__param_target` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[1].source_labels` | Relabel input labels | `["__param_target"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[1].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[2].replacement` | Relabel replacement | Configured blackbox service target (address omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[13].relabel_configs[2].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[1].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/node.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[1].job_name` | Scrape job identity | `node` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[2].job_name` | Scrape job identity | `monitor-node` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[2].static_configs[0].labels.host` | Host label | `monitor` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[2].static_configs[0].targets` | Scrape, probe, or alert delivery targets | Configured service targets (addresses omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/node-readyz.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].job_name` | Scrape job identity | `node-readyz` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].metrics_path` | HTTP metrics or probe path | `/probe` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].params.module` | Blackbox probe module | `["node_consensus"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[0].target_label` | Relabel output label | `__param_target` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[1].source_labels` | Relabel input labels | `["__param_target"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[1].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[2].replacement` | Relabel replacement | Configured blackbox service target (address omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[3].relabel_configs[2].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/node-health.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].job_name` | Scrape job identity | `node-health` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].metrics_path` | HTTP metrics or probe path | `/probe` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].params.module` | Blackbox probe module | `["http_2xx"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[0].target_label` | Relabel output label | `__param_target` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[1].source_labels` | Relabel input labels | `["__param_target"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[1].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[2].replacement` | Relabel replacement | Configured blackbox service target (address omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[4].relabel_configs[2].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/authority-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].job_name` | Scrape job identity | `receipt-authority` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[5].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/certs.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].job_name` | Scrape job identity | `edge-cert` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].metrics_path` | HTTP metrics or probe path | `/probe` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].params.module` | Blackbox probe module | `["tls_cert"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[0].target_label` | Relabel output label | `__param_target` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[1].source_labels` | Relabel input labels | `["__param_target"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[1].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[2].replacement` | Relabel replacement | Configured blackbox service target (address omitted) | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[6].relabel_configs[2].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/keeper-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].job_name` | Scrape job identity | `keeper` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[7].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/feeder-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].job_name` | Scrape job identity | `feeder` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[8].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].file_sd_configs[0].files` | File discovery inputs | `["/etc/prometheus/targets/paxai-worker-metrics.json"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].job_name` | Scrape job identity | `paxai-worker` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[0].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[0].target_label` | Relabel output label | `instance` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[1].regex` | Relabel match expression | `(https?)://.*` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[1].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[1].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[1].target_label` | Relabel output label | `__scheme__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[2].regex` | Relabel match expression | `https?://[^/]+(/.*)` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[2].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[2].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[2].target_label` | Relabel output label | `__metrics_path__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[3].regex` | Relabel match expression | `https?://([^/]+)(/.*)?` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[3].replacement` | Relabel replacement | `$1` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[3].source_labels` | Relabel input labels | `["__address__"]` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/prometheus.yml: scrape_configs[9].relabel_configs[3].target_label` | Relabel output label | `__address__` | No | `tools/ops/monitoring/prometheus.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].name` | Rule group name | `paxeer-chain` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[0].alert` | Alert name | `ChainHeightStalled` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[0].annotations.summary` | Alert summary template | `{{ $labels.host }} block height has not advanced for 60 s` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[0].expr` | PromQL firing expression | `changes({__name__=~"(cometbft\|tendermint)_consensus_height", job="cometbft"}[60s]) == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[0].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[1].alert` | Alert name | `ValidatorMissedBlocks` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[1].annotations.summary` | Alert summary template | `validator {{ $labels.validator_address }} missed blocks in the last 5 m (seen by {{ $labels.host }})` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[1].expr` | PromQL firing expression | `increase({__name__=~"(cometbft\|tendermint)_consensus_validator_missed_blocks", job="cometbft"}[5m]) > 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[1].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[2].alert` | Alert name | `LowPeerCount` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[2].annotations.summary` | Alert summary template | `{{ $labels.host }} has {{ $value }} peers (below 5)` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[2].expr` | PromQL firing expression | `{__name__=~"(cometbft\|tendermint)_p2p_peers", job="cometbft"} < 5` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[2].for` | Pending period before firing | `2m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[2].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[3].alert` | Alert name | `CometMetricsDown` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[3].annotations.summary` | Alert summary template | `{{ $labels.host }} CometBFT metrics endpoint is unreachable` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[3].expr` | PromQL firing expression | `up{job="cometbft"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[3].for` | Pending period before firing | `2m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[0].rules[3].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].name` | Rule group name | `paxeer-hosts` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[0].alert` | Alert name | `DiskAbove85Percent` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[0].annotations.summary` | Alert summary template | `{{ $labels.host }} {{ $labels.mountpoint }} is {{ $value \| humanize }} percent full` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[0].expr` | PromQL firing expression | `(1 - node_filesystem_avail_bytes{fstype!~"tmpfs\|overlay\|squashfs\|ramfs"}      / node_filesystem_size_bytes{fstype!~"tmpfs\|overlay\|squashfs\|ramfs"}) * 100 > 85 ` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[0].for` | Pending period before firing | `5m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[0].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[1].alert` | Alert name | `NodeExporterDown` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[1].annotations.summary` | Alert summary template | `{{ $labels.host }} node_exporter is unreachable` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[1].expr` | PromQL firing expression | `up{job=~"node\|monitor-node"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[1].for` | Pending period before firing | `2m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[1].rules[1].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].name` | Rule group name | `paxeer-services` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[0].alert` | Alert name | `NodeNotReady` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[0].annotations.summary` | Alert summary template | `node consensus readiness {{ $labels.instance }} is failing` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[0].expr` | PromQL firing expression | `probe_success{job="node-readyz"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[0].for` | Pending period before firing | `1m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[0].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[1].alert` | Alert name | `NodeRPCDown` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[1].annotations.summary` | Alert summary template | `node RPC health {{ $labels.instance }} is failing` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[1].expr` | PromQL firing expression | `probe_success{job="node-health"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[1].for` | Pending period before firing | `2m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[1].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[2].alert` | Alert name | `NodeReadinessProbeDown` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[2].annotations.summary` | Alert summary template | `node readiness probe {{ $labels.instance }} is failing` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[2].expr` | PromQL firing expression | `up{job=~"node-readyz\|node-health"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[2].for` | Pending period before firing | `2m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[2].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[3].alert` | Alert name | `ReceiptOrderDivergence` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[3].annotations.summary` | Alert summary template | `receipt authority {{ $labels.instance }} observed receipt-order divergence` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[3].expr` | PromQL firing expression | `increase(order_divergence_total[5m]) > 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[3].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[4].alert` | Alert name | `SponsorBalanceBelowFloor` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[4].annotations.summary` | Alert summary template | `sponsor {{ $labels.sponsor }} on {{ $labels.instance }} is below its configured balance floor` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[4].expr` | PromQL firing expression | `gas_station_sponsor_balance < gas_station_sponsor_floor` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[4].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[5].alert` | Alert name | `RateAgeNearLimit` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[5].annotations.summary` | Alert summary template | `published paymaster rate on {{ $labels.instance }} is at least 90 percent of max_rate_age` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[5].expr` | PromQL firing expression | `gas_station_rate_age_seconds >= gas_station_max_rate_age_seconds * 0.9` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[5].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[6].alert` | Alert name | `EdgeCertExpiringSoon` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[6].annotations.summary` | Alert summary template | `edge certificate on {{ $labels.instance }} expires in {{ $value \| humanize }} days` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[6].expr` | PromQL firing expression | `(probe_ssl_earliest_cert_expiry{job="edge-cert"} - time()) / 86400 < 14` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[6].labels.severity` | Alert severity label | `warning` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[7].alert` | Alert name | `EdgeCertProbeFailing` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[7].annotations.summary` | Alert summary template | `TLS probe to {{ $labels.instance }} is failing` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[7].expr` | PromQL firing expression | `probe_success{job="edge-cert"} == 0` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[7].for` | Pending period before firing | `5m` | No | `tools/ops/monitoring/rules.yml` |
| `tools/ops/monitoring/rules.yml: groups[2].rules[7].labels.severity` | Alert severity label | `critical` | No | `tools/ops/monitoring/rules.yml` |

Environment defaults above come from the shell sources. For runtime configuration files, the table gives the explicit shipped or rendered setting; these are not fallback values. Dotted names identify the file section and key, with zero-based sequence indices; `[root]` denotes an unsectioned key. Systemd reads the service and timer keys, Docker Compose reads container settings, and the named monitoring processes read their mounted configuration files. YAML anchors in `prometheus.yml` share the same relabel settings across metrics jobs. Rows naming a command flag identify an argument within that service’s `command` list.

`backup/backup.env.example` covers backup and restore settings. `monitoring/hosts.env.example` is sourced by `render.sh`; its assignments replace inherited environment values, so populate the file before rendering. `tools/ops/.env.example` covers keyring migration. Put the shared backup settings in the file selected by `BACKUP_ENV`, and export the migration inputs before invoking `keyring-migrate.sh`. Empty assignments indicate empty defaults or inputs that the operator must supply. The backup file is sourced and exported by `load_env`; the optional command-line environment-file argument takes precedence over `BACKUP_ENV`.

The validator hook receives `BACKUP_RESTORE_ROOT`, `BACKUP_SNAPSHOT_MANIFEST`, and remapped `BACKUP_NODE_HOME` and `BACKUP_KERNEL_DIR` from `backup.sh`. The first two are hook outputs rather than operator inputs. Validation reopens recovered stores offline, checks their committed height/root agreement, and loads required keys before an archive is published and retention runs. Consensus readiness probes use `/status` and RPC availability probes use `/health` on the paxd consensus RPC port.
