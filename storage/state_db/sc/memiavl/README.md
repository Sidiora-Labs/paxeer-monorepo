# MemIAVL

## The Design
The idea of MemIAVL is to keep the whole chain state in memory as much as possible to speed up reads and writes.
- MemIAVL uses a write-ahead log (WAL, the changelog in `storage/wal`) to persist the changeset from transaction commit to speed up writes.
- Instead of updating and flushing nodes to disk, state changes at every height are actually only written to WAL file
- MemIAVL snapshots are taken periodically and written to disk to materialize the tree at some given height H
- Each snapshot is composed of 4 files per module tree: metadata, branch nodes, leaf nodes and key/value pairs
- Snapshot files are loaded with mmap and use the OS page cache. WAL truncation retains entries needed by the earliest retained snapshot, rather than pruning to the newest snapshot height (see `db.go`).
- Each MemIAVL tree uses `MemNode` and `PersistedNode` nodes.
  - Nodes loaded from a snapshot are persistent nodes backed by its files.
  - During updates or insertion, persistent nodes will turn into MemNode
  - MemNodes hold updated state in memory until a replacement snapshot is loaded.
- On restart, MemIAVL loads a snapshot at or below the requested version and replays subsequent persisted WAL entries.

### Advantages
- Better write amplification, we only need to write the change sets in real time which is much more compact than IAVL nodes, IAVL snapshot can be created in much lower frequency.
- Snapshot nodes use fixed-size records and indexes; persisted-tree point lookups binary-search the leaf keys (see `PersistedNode.Get` in `persisted_node.go`).
- The state store (`storage/state_db/ss`) handles historical key-value queries. MemIAVL versions and proofs depend on the retained snapshots and WAL; snapshot retention is configured by `SnapshotKeepRecent`.
- With a positive WAL `WriteBufferSize`, commits enqueue writes without waiting for durability; enqueueing waits when the bounded queue is full.

### Trade-offs
- Performance can degrade when state size grows much larger than memory
- MemIAVL makes historical proof much slower
- Periodic snapshot creation is a very heavy operation and could become a bottleneck

### IAVL Snapshot

IAVL snapshot is composed of four files:

- `metadata`, 12 bytes (little endian):

  ```text
  magic: 4      # b"IAVL"
  format: 4
  version: 4
  ```

- `nodes`, array of fixed size (48 bytes) branch nodes:

  ```text
  height   : 1
  preTrees : 1
  _padding : 2
  version  : 4
  size     : 4
  key leaf : 4
  hash     : [32]byte
  ```

- `leaves`, array of fixed size (48 bytes) leaf nodes:

  ```text
  version     : 4
  key len     : 4
  key offset  : 8
  hash        : [32]byte
  ```

  Nodes have a fixed length, so they can be indexed directly. Branch nodes are written in post-order depth-first traversal, so the root node is always placed at the end.

  For a branch node, the `key leaf` field references the smallest leaf in the right branch; the key slice is fetched from there indirectly. Leaf nodes store the `offset` into the `kvs` file, where the key and value slices can be built.

  A branch node's left and right children are not stored. They are derived from the post-order layout, the `preTrees` count, and the `key leaf` index (see `persisted_node.go`).

  The version/size/node indexes are encoded with 4 bytes.

  The implementation reads the mmap-ed content in a zero-copy way and doesn't use an extra node cache; it relies on the OS page cache.

- `kvs`, sequence of leaf node key-value pairs; the keys are ordered with no duplicates.

  ```text
  keyLen: uint32 (little endian)
  key
  valueLen: uint32 (little endian)
  value
  *repeat*
  ```
