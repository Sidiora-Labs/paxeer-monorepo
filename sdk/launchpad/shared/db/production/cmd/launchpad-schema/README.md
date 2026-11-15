# launchpad-schema

`launchpad-schema` applies a reviewed launchpad PostgreSQL migration release,
writes service expected schema documents, and checks runtime readiness.

```sh
launchpad-schema apply --migrations migrations
launchpad-schema expected --migrations migrations --service core/ranking-algo --runtime-role ranking_runtime --region deployment-region --out expected-schema.json
launchpad-schema check --expected expected-schema.json --sha256 DOCUMENT_DIGEST
```

`apply` reads `LAUNCHPAD_ADMIN_DATABASE_URL`, a direct administrator PostgreSQL
connection. The migration directory contains `registry.json`, `grants.json` and
`legacy-history-aliases.json`; source paths resolve relative to its parent, the
repository root. Original service paths in manifests resolve to their monorepo
locations. The release must supply an approval digest, the exact grants digest
as each migration's interface digest, reader requirements, lock estimates and
reviewed legacy structural fingerprints. This command supports reader version 1
and transactional migrations.

The command installs declared extension prerequisites, creates control objects
and owner roles, imports exact legacy histories, provisions runtime schemas and
roles, and applies unrecorded migrations in registry order through the production
coordinator. It grants owner interfaces before migrations and runtime privileges
afterward. A repeated application preserves registered migration records and
database objects. Changed checksums, unknown recorded migrations and mismatched
legacy evidence are refused. Empty service schemas receive their reviewed legacy
up SQL and original history before baseline import; an existing service schema
must already have its per-service version proof in the alias's named history.

`expected` reads `LAUNCHPAD_ADMIN_DATABASE_URL` and the same migration release
(`--migrations` defaults to `migrations`). `--service` and `--runtime-role` must
match one manifest in `grants.json`. The command reads applied migration
checksums, catalog fingerprints and the recovery epoch; `--region` must match
the database's recovery authority. It records runtime readiness, validates the
document using the production loader and checks readiness as the runtime role.
It writes JSON atomically to `--out` with permissions `0600` and prints only the
SHA-256 digest of the exact file bytes, followed by a newline.

Recovery authority in `recovery.recovery_control` and authenticated login
credentials are provisioned by the operator. The command creates runtime roles
as `NOLOGIN` capabilities; authentication can use an externally provisioned login
delegated only to its runtime role. Connection settings must select that role
when using `check` with a delegated login.

`check` reads `LAUNCHPAD_RUNTIME_DATABASE_URL`, loads `--expected` bound to
`--sha256`, and runs the production readiness check with that runtime connection.
Successful `apply` and `check` commands produce no output. Diagnostics never
include connection strings, passwords, server error details or SQL parameters.

Exit codes are `0` for success, `1` for a refusal with one reason line on stderr,
and `2` for invalid usage or a missing connection variable.

Database tests use `COORDINATOR_TEST_POSTGRES_DSN` for an already running
PostgreSQL administrator that may create databases and roles. They create and
remove private databases and their new roles. Database tests are skipped when
that variable is absent; argument and source admission tests run without it.
