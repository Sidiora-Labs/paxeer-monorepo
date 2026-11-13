# Paxeer operations tools

This directory holds scripts and configuration for keyring migration, encrypted backups and fleet monitoring. They support operation and recovery of Paxeer mainnet-beta nodes and their supporting services. The kernel runs inside `paxd` as part of the network's block proposal and execution path.

## Layout

- [backup/](backup/): encrypted capture and restore scripts, recovery-image validation, a backup configuration example, systemd service and timer definitions, and round-trip tests.
- [monitoring/](monitoring/): Prometheus, Alertmanager, Blackbox Exporter and node_exporter deployment configuration, target rendering, installation and alert-rule tests.
- [keyring-migrate.sh](keyring-migrate.sh): imports test-backend keys into the passphrase-protected file backend; removes the test keyring only after file-backend recovery, backup recovery and usage checks succeed.
- [keyring_recovery.py](keyring_recovery.py): validates key enumeration, snapshots keyring contents and checks archived bytes before extracting a private recovery keyring.

## Build and test

The shell and Python scripts run directly and have no compilation target of their own. The monitoring stack uses the container images declared in [monitoring/docker-compose.yml](monitoring/docker-compose.yml).

Run the directory's test scripts from the repository root:

```sh
bash tools/ops/backup/test.sh
bash tools/ops/monitoring/test.sh
```

The backup test requires Bash, Python, `age` and `age-keygen`, together with the system utilities used by the backup scripts. It exercises capture, encryption, store readback, restoration, retention and rejection of incomplete or altered archives. Restoring archived ownership requires permission to set the recorded ownership.

The monitoring test requires Python with PyYAML and Docker. It uses local `promtool` and `amtool` when available, otherwise their declared container images. It renders monitoring configuration, binds readiness-rule fixtures to rendered targets, checks Prometheus rules and configuration, checks Alertmanager routes and validates the Blackbox Exporter configuration.

## Configuration

Keyring migration reads `KEYRING_PASSPHRASE_FILE`, `PAXD` and `BACKUP_ENV`, and uses the backup-store settings below. It also reads `config/client.toml` within each supplied node home to check the selected keyring backend.

Backup and restore load a shell configuration file supplied as an argument or selected by `BACKUP_ENV`. [backup/backup.env.example](backup/backup.env.example) describes the deployment inputs:

- Store and retention: `BACKUP_HOST`, `BACKUP_DIR`, `BACKUP_SSH`, `BACKUP_PREFIX` and `BACKUP_RETENTION`.
- Encryption files: `BACKUP_RECIPIENTS_FILE` and `BACKUP_IDENTITY`.
- Captured state: `BACKUP_SOURCES`, `BACKUP_NODE_HOME`, `BACKUP_KERNEL_DIR` and `BACKUP_REQUIRED_KEYS`.
- Capture lifecycle: `BACKUP_QUIESCE_CMD`, `BACKUP_RESUME_CMD`, `BACKUP_VALIDATE_CMD`, `BACKUP_LOCK_FILE` and `BACKUP_DB_DUMP_CMD`.

The capture includes the complete node home and kernel store, including `config/priv_validator_key.json`, `config/node_key.json` and `data/priv_validator_state.json` within the node home. The quiesce and resume commands control source writers. Before publishing an archive or pruning older archives, the backup script decrypts a copy read back from the store, restores it into an isolated directory and invokes the configured validator. The validator receives `BACKUP_RESTORE_ROOT`, `BACKUP_SNAPSHOT_MANIFEST`, and remapped `BACKUP_NODE_HOME` and `BACKUP_KERNEL_DIR` paths. It must reopen recovered stores offline, compare committed state and load the required keys.

Monitoring rendering reads a shell configuration file based on [monitoring/hosts.env.example](monitoring/hosts.env.example):

- Node targets: `HOSTS`, `COMET_METRICS_PORT`, `COMET_RPC_PORT` and `NODE_EXPORTER_PORT`.
- Service targets: `AUTHORITY_METRICS_URLS`, `KEEPER_METRICS_URLS`, `FEEDER_METRICS_URLS`, `PAXAI_WORKER_METRICS_URLS`, `AI_EVALUATOR_METRICS_URLS`, `X_WEBSEARCH_METRICS_URLS` and `WALLET_ATTESTOR_METRICS_URLS`.
- HTTP and certificate probes: `EXPLORER_INDEX_HEAD_URLS`, `STREAM_METER_PROVIDER_URLS` and `EDGE_CERT_URLS`.
- Alert routing: `ALERT_WEBHOOK_URL`, `ALERT_EMAIL_TO`, `ALERT_EMAIL_FROM`, `ALERT_SMTP_SMARTHOST`, `ALERT_SMTP_USERNAME` and `ALERT_SMTP_PASSWORD`.

`HOSTS` supplies node names and addresses. Rendering requires a webhook route or an email route; email also requires a sender and SMTP server. Node readiness probes use the consensus `/status` response, while availability probes use `/health`.

The monitoring stack reads [monitoring/docker-compose.yml](monitoring/docker-compose.yml), [monitoring/prometheus.yml](monitoring/prometheus.yml), [monitoring/rules.yml](monitoring/rules.yml) and [monitoring/blackbox.yml](monitoring/blackbox.yml). Rendering produces the target files and Alertmanager configuration consumed by the stack. [monitoring/rules_test.yml](monitoring/rules_test.yml) supplies the alert-rule test cases.

## Documentation

- [Paxeer network](../../docs/paxeer-network.md): merged runtime, node build commands and network interfaces.
- [Validator runbook](../../docs/release/7.0.0-validator-runbook.md): node upgrade and recovery guidance.
- [Network operations site page](../../docs/site/paxeer-docs/src/app/network-operations/page.tsx): service readiness, durable state and maintenance.
