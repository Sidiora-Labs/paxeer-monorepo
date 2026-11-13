## Summary

Describe the change and the affected components using their repository paths.

## Compatibility

Describe any impact on public APIs, wire encoding, result codes, state roots,
storage, or contract ABI.

## Checks

The root Makefile provides the following checks. List the commands relevant to
this change and their results, and identify any that were not run.

- `make check`: runs the protocol test target.
- `make workspace-inventory-check`: checks workspace manifests and dependencies.
- `make workspace-build`: builds the workspace components.
- `make workspace-test`: runs the workspace test suites.
- `make workspace-lint`: runs the workspace lint and consistency checks.
- `make workspace-ci`: runs the public audit, inventory, build, test, and lint checks.

Include only reviewed technical results with credentials and private data removed.

## Checklist

- [ ] The change includes coverage appropriate to its behavior and compatibility impact.
- [ ] The change preserves canonical encoding, deterministic replay, and committed-state invariants where applicable.
- [ ] The change contains no credentials, private infrastructure identifiers, or personal data.
- [ ] Each commit is signed off under the Developer Certificate of Origin.
- [ ] No assertion, bound, type, test, or check was weakened or disabled to obtain a passing result.
