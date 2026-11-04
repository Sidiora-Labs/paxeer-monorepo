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
or private-network routes may still be possible. Docker internal networks suppress
published ports on Docker 29, so they are unsuitable for host-run integration tests.
A local privileged user can still access Docker and these test services.
No existing service, production volume, `.env`, or chain data is mounted.

## Start from the repository root

The coordinator starts this stack once, after reviewing this configuration:

```sh
docker compose -f deploy/launchpad/production/bootstrap/compose.yml config --quiet
python3 deploy/launchpad/production/bootstrap/validate.py
python3 deploy/launchpad/production/bootstrap/start.py
python3 deploy/launchpad/production/bootstrap/smoke.py
```

`start.py` starts and waits for the five persistent services, then runs the profiled
`minio-init` one-shot bucket initializer and requires its exit code to be zero.
This avoids incorrectly treating successful initializer termination as a failed
long-running service health check. An initialization failure fails startup. Other services
have application-level Docker health checks. NATS uses persistent JetStream R1
for this small bootstrap fixture. Production R3, multi-zone failure, capacity and
recovery qualification remain later requirements and are not demonstrated here.
Redis is deliberately ephemeral. PostgreSQL, broker, object and EVM state use
project-scoped named volumes. Restart preserves fixture data.

## Connections and state

Host connection strings are in [fixture.env.example](fixture.env.example). This file contains no
production secrets and should not be copied over any application environment.
Containers on this fixture network use service DNS and container ports instead.
The chain starts without Sidiora contracts: contract deployment and application
address configuration remain explicit coordinator prerequisites. Existing storage
integration tests may launch their own Testcontainers rather than consume this
stack; the fixture alone does not change those test configurations.

## Image provenance

Every image in [compose.yml](compose.yml) is digest-pinned.
[images.json](images.json) records the original provenance checks: PostgreSQL,
Redis and the MinIO server/client were checked against the authoring environment's
local cache; NATS 2.11.9-alpine and Ganache 7.9.2 had upstream manifests verified.
The MinIO server record identifies RELEASE.2025-09-07T16-13-09Z and notes that its
historical remote tag was unavailable. The fixture uses Ganache because the
original Foundry registry access was denied and no verified Anvil reference was
available. These records do not establish image availability on the current
machine. Verify the pinned images can be obtained on a fresh host before
claiming cold-host reproduction.

## Stop and preserve state

```sh
docker compose -f deploy/launchpad/production/bootstrap/compose.yml down
```

Deleting fixture volumes is a separate destructive local cleanup decision; the
provided scripts never remove volumes. Concurrent independent fixture stacks
require explicitly reviewed project names and host-port overrides; the default
stack is coordinator-owned to prevent duplicate starts.

Use the [recovery overview](../recovery/README.md) for the local PostgreSQL
checks that depend on this fixture, and the [observability overview](../observability/README.md)
for telemetry tooling and its separate prerequisites.
