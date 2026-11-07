# Local vendoring changes

The following files retain local execution replay changes used by the programs
runtime. Their checksum entries are refreshed to match the existing sources;
this reconciliation does not alter the Rust code.

- `src/engine/mod.rs` restores validated execution snapshots and call frames and
  executes single-step replay. It retains the clippy repairs and charges replay
  boundary reconstruction work against the observer's work limit.
- `src/execution_trace.rs` defines replay frames, snapshots, transitions and trap
  records alongside the existing execution snapshots. It retains the clippy
  repairs, including explicit trap-code comparison.
- `src/store.rs` restores execution instance state, initializes bounded
  single-step replay and records its snapshots and transitions. It retains the
  clippy repairs and accounts for replay boundary bytes and work before replay.

These changes were introduced by `daf26db476` (execution snapshot restoration
and single-step replay), `bb8ccc51d6` (clippy repairs) and `41f31ec2fc` (replay
boundary charge). They remain necessary for the programs runtime's replay path.

`.cargo-checksum.json` records the SHA-256 of the three files above and this
`PATCHES.md`. Its upstream package checksum and all other file digests remain
unchanged.
