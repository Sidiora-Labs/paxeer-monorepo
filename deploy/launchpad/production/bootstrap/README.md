# Isolated bootstrap fixtures

This stack supplies local integration dependencies for the
[launchpad services](../../README.md). Paxeer mainnet-beta is one BFT network
with the kernel participating in block proposal and execution; a receipt is
final when its block is committed. This fixture is not a production deployment
or a Paxeer consensus benchmark. Its Ganache EVM uses chain ID 125 and instant local
mining; it does not reproduce Paxeer's validator, finality, transaction extension,
or performance implementation. The publicly known accounts and fixed credentials
are test data. Never fund these accounts outside this fixture.

All host ports bind to `127.0.0.1`. The private Compose bridge disables outbound
IP masquerading. This is not an absolute egress firewall: access to reachable host
or private-network routes may still be possible. A local privileged user can still access Docker and these test services.
No existing service, production volume, `.env`, or chain data is mounted.

## Start from the repository root

All command paths below are relative to the repository root; keep the
`deploy/launchpad/production/bootstrap/` prefix when invoking these tools.

Start the stack with:

```sh
docker compose -f deploy/launchpad/production/bootstrap/compose.yml config --quiet
python3 deploy/launchpad/production/bootstrap/validate.py
python3 deploy/launchpad/production/bootstrap/start.py
python3 deploy/launchpad/production/bootstrap/smoke.py
```

`start.py` starts and waits for the five long-running services, then runs the profiled
`minio-init` one-shot bucket initializer and requires its exit code to be zero.
An initialization failure fails startup. The long-running services have
application-level Docker health checks. NATS uses persistent JetStream R1
for this local fixture.
Redis is deliberately ephemeral. PostgreSQL, broker, object and EVM state use
project-scoped named volumes. Restart preserves fixture data.

## Connections and state

Host connection strings are in [fixture.env.example](fixture.env.example). This file contains no
production secrets and should not be copied over any application environment.
Containers on this fixture network use service DNS and container ports instead.
The chain starts without Sidiora contracts: contract deployment and application
address configuration require explicit deployment configuration. Existing storage
integration tests may launch their own Testcontainers rather than consume this
stack; the fixture alone does not change those test configurations.

## Images

Every image in [compose.yml](compose.yml) is digest-pinned. The stack uses
PostgreSQL 16, Redis 7, NATS 2.11.9, MinIO and Ganache 7.9.2.
The pinned image references are also listed in [images.json](images.json).

## Stop and preserve state

```sh
docker compose -f deploy/launchpad/production/bootstrap/compose.yml down
```

Deleting fixture volumes is a separate destructive local cleanup decision; the
provided scripts never remove volumes. Independent fixture stacks require distinct project names and host-port
overrides.

Use the [recovery overview](../recovery/README.md) for the local PostgreSQL
checks that depend on this fixture, and the [observability overview](../observability/README.md)
for telemetry tooling and its separate prerequisites.
