# Plugin fork selection

`pnpm check:plugin-forks` checks the plugins listed in
[`forkable-plugins.json`](../../../tools/hyperfide/tools/workspace/forkable-plugins.json).
Run it from `tools/hyperfide`.

`--changed-from=<sha>` selects plugins from changed paths. If all changed paths
belong to listed plugins, only those plugins are selected. Shared or unknown
paths, unlisted plugins and missing comparison history select the full list.
Renames contribute both paths. Explicit plugin directories and `--changed-from`
are mutually exclusive.

`--shard=<index>/<count>` partitions the selection with one-based shard indexes.
`--list` prints the selected shard without installing or building plugins.
See [forkable plugins](forkable-plugins.md) for the isolated build contract.
