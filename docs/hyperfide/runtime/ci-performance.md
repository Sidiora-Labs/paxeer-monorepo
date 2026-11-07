# Platform checks

Run the platform checks described in [testing.md](testing.md) from the repository
root. The platform does not own a separate GitHub Actions workflow tree.

Plugin fork validation remains available with `pnpm check:plugin-forks`.
Its changed-file selection and shard partitioning use local Git fixtures in
`tools/scripts/test/plugin-fork-selection.test.mjs`.
