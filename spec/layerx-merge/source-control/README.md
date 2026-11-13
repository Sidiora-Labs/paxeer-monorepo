# Native source control

Hyperfide provides online source history, scoped workspaces, candidate acceptance and Git import/export. The implementation lives in [tools/hyperfide](../../../tools/hyperfide/).

## Documents

- [Overview](SOURCE-README.md): service, client and product entry points.
- [Design](design.body.md) and [decisions](decisions.md): responsibility boundaries and source invariants.
- [Repository layout](repository-layout.md) and [implementation seams](repository-baseline.md): package locations and toolchains.
- [Storage](storage.md) and [protocol](protocol.md): object formats, requests, retries and acceptance.
- [Workspaces](workspaces.md): source scopes, materialization, cache and checkpoints.
- [Product integration](hyperfide-integration.md) and [security](identity-security.md): task custody, clients and authorization.
- [Git interoperability](git-interop.md) and [migration](migration.md): remote projections and source-authority transitions.
- [Fleet orchestration](fleet-orchestration.md), [sandbox contracts](sandbox-contracts.md) and [interface catalogue](interface-contracts.md): environment installation and worker authority.
- [Verification](verification.md): case registry and workload contracts.

The [source specification](source-spec.kvx) contains the source-control requirements. [Implementation plan](implementation-plan.md) and [traceability](traceability.md) are generated views.
