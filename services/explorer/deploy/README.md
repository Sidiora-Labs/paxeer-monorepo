# Local deployment checks

`docker-compose.local.yml` publishes PostgreSQL only on the loopback interface,
at `${POSTGRES_PUBLISHED_PORT:-7432}`. Containers reach the `db` service on
port 5432 through the Compose network. The local file supplies development
credentials; configure deployment credentials through the environment.

For remote database access, use Compose 2.24.4 or later and explicitly add
`docker-compose.remote-db.yml`. Set `POSTGRES_BIND_ADDRESS` to the host's private
interface, `POSTGRES_USER` to an operator-provided user, and `POSTGRES_PASSWORD`
to a strong, unique operator-provided password in a protected environment or
secret store. None of these three values has a default in the override; unset
or empty values make Compose refuse the configuration. Use URL-safe credentials
because the backend includes them in `DATABASE_URL`. Restrict the port to
trusted clients with a firewall and use a VPN or SSH tunnel for remote traffic;
this override does not configure PostgreSQL TLS.

From the repository root, with those variables supplied:

```sh
docker compose \
  -f services/explorer/deploy/docker-compose.local.yml \
  -f services/explorer/deploy/docker-compose.remote-db.yml up -d
```

The override replaces the local port list and supplies the same credentials to
PostgreSQL and the backend. PostgreSQL's image applies initialization credentials
only to an empty data directory. For an existing `explorer-db-data` volume,
rotate the role credentials in PostgreSQL and update the backend before enabling
remote publication; changing environment variables does not rotate existing
database passwords. Do not expose a volume still using the example credentials.

Run the source configuration check from the repository root:

```sh
bash services/explorer/deploy/tests/config-check.sh
```

The frontend image builds `apps/explorer` with Next.js `basePath: '/explorer'`.
Its Compose healthcheck requests `/explorer/api/healthz`, served by
`pages/api/healthz.tsx`. To check an image already built or loaded locally, set
`EXPLORER_IMAGE_TAG` to its Compose tag and run:

```sh
bash -n services/explorer/deploy/tests/config-check.sh
bash -n services/explorer/deploy/tests/frontend-health-check.sh
bash services/explorer/deploy/tests/frontend-health-check.sh
```

The image check never builds or pulls. It starts an isolated container with no
network access or published ports, runs the exact rendered Compose probe while
no server is available and requires failure, then starts the image's standalone
`node server.js` and requires success within 60 seconds. It bypasses runtime
asset generation to test the built route without external services and removes
its container on exit. Full entrypoint and deployment startup remain separate
operator checks.
