# Hyperfide configuration

Use `hyperfide platform --help` and the relevant subcommand help for supported
flags. The public entry point is the repository's `hyperfide` binary:

```sh
hyperfide platform --help
hyperfide platform thread list
```

Production runtime data defaults to `~/.hyperfide`. Development instances use
checkout-specific directories under `~/.hyperfide-dev`, with checkout-specific
ports. `HYPERFIDE_DATA_DIR` overrides the data directory. Platform configuration
variables use the `HYPERFIDE_` prefix. Select a separate data directory and unused
ports when running an isolated instance. Keep credentials out of logs and issue
reports.

## Source development

The Hyperfide workspace pins Node 22.19.0 and pnpm 9.15.0. From the monorepo root:

```sh
cd tools/hyperfide
pnpm install --frozen-lockfile
pnpm dev
```

The source app uses the server API and host daemon. Configure each host for the
repository paths it owns. A project source retains its host boundary; a client
connection does not make another machine's filesystem local.

## Workspaces

Repository setup and teardown scripts run on the execution host. Review scripts
before granting access to credentials or external resources. See
[worktrees](worktrees.md) for provisioning and cleanup contracts.

## Remote access

Clients connect to a server you operate. Use a trusted authenticated network and
restrict listeners to intended operators. Enroll only the host daemons that should
execute work for that server. See [system overview](system-overview.md).

## Performance diagnostics

The server permits CPU profiling and detailed diagnostic logs when
`HYPERFIDE_PERF_DIAGNOSTICS=1` is set at startup and the
`performanceDiagnostics` experiment is enabled. The launcher flag
`--perf-diagnostics` sets that startup permission. Both switches are required.
See [runtime diagnostics](debugging-and-qa.md#opt-in-server-performance-diagnostics).

## Reporting configuration problems

Include the Hyperfide version, operating system, launch command with secrets
removed, expected result, and observed error in
https://github.com/Sidiora-Labs/hyperfide/issues.
