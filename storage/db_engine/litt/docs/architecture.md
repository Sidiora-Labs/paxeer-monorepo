# LittDB Architecture

LittDB is the embedded Go storage library in `storage/db_engine/litt`. This walkthrough starts with
hypothetical designs and adds the ideas used by the implementation. The iterations explain the design;
the [final section](#putting-it-all-together-littdb) maps it to current code and its retention behavior.
See the [API and limits](../README.md#api), [filesystem layout](filesystem_layout.md), and
[CLI operations](littdb_cli.md) for practical use.

For each iteration, the database must fulfill the following requirements:

- must support `put(key, value)`/`get(key)` operations
- must be thread safe
- must support a TTL
- must be crash durable

## Iteration 1: Appending data to a file

Let's implement the simplest possible key-value store that satisfies the requirements above. It's going to be super
slow. Ok, fine. We want simple.

![](resources/iteration1.png)

When the user writes a key-value pair to the database, append the key and the value to the end of the file, along
with a timestamp. When the user reads a key, scan the file from the beginning until you find the key and
return the value.

Periodically, scan the data in the file to check for expired data. If a key has expired, remove it from the file
(will require the file to be rewritten).

This needs to be thread safe. Keep a global read-write lock around the file. When a write or GC operation is in
progress, no reads are allowed. GC operations and writes are not permitted to happen in parallel. Allow multiple
reads to happen concurrently.

In order to provide durability, ensure the file is fully flushed to disk before releasing a write lock.

Congratulations! You've written your very own database!

![](resources/iDidIt.png)

## Iteration 2: Add a cache

Reads against the database in 1 are slow. If there is any way we could reduce the number of times we have to iterate
over the file, that would be great. Let's add an in-memory cache.

![](resources/iteration2.png)

Let's assume we are using a thread safe map to implement the cache.

When reading data, first check to see if the data is in the cache. If it is, return it. If it is not, acquire a read
lock and scan the file. Be sure to update the cache with the data you read.

When writing data, write the data to the file, and then update the cache. Data that is recently written is often
read immediately shortly after, for many workloads.

In this hypothetical design, deletion would also remove the cache entry. Current LittDB instead
checks base-table existence before serving a cache hit. Garbage collection removes keys and segment
files without actively evicting those cache entries; cached bytes can remain until eviction or shutdown.
The [final section](#putting-it-all-together-littdb) describes this distinction and its regression test.

## Iteration 3: Add an index

Reading recent values is a lot faster now. But if you miss the cache, things start getting slow. `O(n)` isn't fun
when you database holds 100TB. To address this, let's add an index that allows us to jump straight to the data we
are looking for. For the sake of consistency with other parts of this document, let's call this index a "keymap".

![](resources/iteration3.png)

Inside the keymap, maintain a mapping from each key to the offset in the file where the first byte of the value is
stored.

When writing a value, take note of the offset in the file where the value was written. Store the key and the offset
in the keymap.

When reading a value and there is a cache miss, look up the key inside the keymap. If the key is present, jump to
start of the value in the file and read the value. If the key is not present, tell the user that the key is not
present in the database.

When deleting a value, remove the key from the keymap in addition to removing the value from the file.

At startup time, we will have to rebuild the keymap, since we are only storing it in memory. In order to do so,
iterate over the file and reconstruct the keymap. If this is too slow, consider storing the keymap on disk (perhaps
using an off-the-shelf key-value store like PebbleDB).

The database needs to do a little extra bookkeeping when it deletes data from the file. If it deletes X bytes from
the beginning of the file, then the offsets recorded in the keymap are off by a factor of X. The key map doesn't
need to be rebuilt in order to fix this. Rather, the database can simply subtract X from all the offsets in the
keymap to find the actual location of the data in the file. Additionally, it must add X to the offset when computing
the "offset" of new data that is written to the file.

## Iteration 4: Unflushed data map

In order to be thread safe, the solution above uses a global lock. While one thread is writing, readers must wait
unless they get lucky and find their data in the cache. It would be really nice if we could permit reads to continue
uninterrupted while writes are happening in the background.

![](resources/iteration4.png)

Create another key->value map called the "unflushed data map". Use a thread safe map implementation.

When the user writes data to the database, immediately add it to the unflushed data map, but not the key map.
After that is completed, write it to file. The write doesn't need to be synchronous. For example, you can use file
stream APIs that buffer data in memory before writing it to disk in larger chunks. The write operation doesn't need
to block until the data is written to disk, it can return as soon as the data is in the unflushed data map and written
to the buffer.

Expose a new method in the database called `Flush()`. When `Flush()` is called, first flush all data in buffers to disk,
then empty out the unflushed data map. Before each entry is removed, write the key-address pair to the key map.
This flush operation should block until all of this work is done.

When reading data, look for it in the following places, in order:

- the cache
- the unflushed data map
- on disk (via the keymap and data file)

Unlike previous iterations, write no longer needs to hold a lock that blocks readers. This is thread safe, and it
provides read-your-writes consistency.

If a reader is attempting to read data that is currently in the process of being written to disk, then the data will
be present in the unflushed data map. If the reader finds an entry in the key map, this means that the data has already
been written out to disk, and is therefore safe to read from the file. Even if the writer is writing later in the file,
the bytes the reader wants to read will be immutable.

Although the strategy described above allows read operations to execute concurrently with write operations, it does
not solve the problem for deletions of values that have exceeded their TTL. This operation will still require a global
lock that blocks all reads and writes.

## Iteration 5: Break the file into segments

One of the biggest inefficiencies in design, to this point, is that the deleting values is exceptionally slow. The
entire file must be rewritten in order to trim bytes from the beginning. And to make matters worse, we need to hold
a global lock while we do it. To fix this, let's break apart the data file into multiple data files. We'll call each
data file a "segment".

![](resources/iteration5.png)

Decide on a maximum file size for each segment. Whenever a file gets "full", close it and open a new one. Let's assign
each of these files a serial number starting with `0` and increasing monotonically. We'll call this serial number the
"segment index".

Previously, the address stored in the key map told us the offset in the file where the value was stored. Now, the
address will also need to keep track of the segment index, as well as the offset.

Deletion of data is now super easy. When all data in the oldest segment file exceeds its TTL, we can delete just that
segment without modifying any of the other segment files. Iterate over the segment file to delete values from the key
map.

In order to avoid the race condition where a reader is reading data from a segment that is in the process of being
deleted, use reference counters for each segment. When a reader goes to read data, it first finds the address in the
keymap, than increments the reference counter for the segment. When the reader is done reading, it decrements the
reference counter. When the garbage collector goes to delete a segment, it waits to actually delete the file on disk
until the reference counter is zero. As a result of this strategy, there is now no longer a need for garbage collection
to hold a global lock.

## Iteration 6: Metadata files

In the previous iteration, we do garbage collection by deleting a segment once all data contained within that segment
has expired. But how do we figure out when that actually is? In the previous iteration, the only way to do that is to
iterate over the entire segment file and read the timestamp stored with the last value. Let's do better.

For each segment, let's create a metadata file. We will record the time when the segment is sealed in
this file. As a result, we will no longer need to store timestamp information inside the value files, which will
save us a few bytes per entry.

![](resources/iteration6.png)

Now, all the garbage collector needs to read to decide when it is time to delete a segment is the metadata file for
that segment.

## Iteration 7: Key files

Storing timestamp information in a metadata file is a good start, but we still need to scan the value file. When a
segment is deleted, we need to clean up the key map. In order to do this, we need to know which keys are stored in the
segment. Additionally, when we start up the database, we need to rebuild the key map. This requires us to scan each
segment file to find the keys.

From an optimization point of view, we can assume that in general keys will be much smaller than values. During the
operations described above, we don't care about the values, only the keys. So lets separate the keys from the values
to avoid having to read the values when we don't need them.

![](resources/iteration7.png)

Everything works the same way as before. But instead of iterating huge segment files when deleting a segment
or rebuilding the key map at startup, we only have to iterate over the key file. The key file is going to be
significantly smaller than the value file (for sane key-value size ratios), and so this will be much faster.

## Iteration 8: Sharding

A highly desirable property for this database is the capability to spread its data across multiple physical drives.
In order to do this, we need to shard the data. That is to say, we need to break the data into smaller pieces and
spread those pieces across multiple locations.

![iteration8](resources/iteration8.png)

Key files and metadata files are small. For the sake of simplicity, let's not bother sharding those. Value files
are big. Break apart value files, and have one value file per shard.

When writing data, the first thing to do will be to figure out which shard the data belongs in. We assign shards in
round-robin order: the first write into a fresh segment goes to shard 0, the second to shard 1, and so on, wrapping
around once we have used every shard.

When reading data, we need to know which shard the value lives in. To avoid having to recompute or look anything up,
the address itself records the shard ID alongside the offset. So the address is a self-contained pointer to the
value: given just the address, we know exactly which file to open and where in that file to seek.

Round-robin assignment prevents choosing a shard just by crafting a key: assignment depends on write
order, not key contents. It balances the number of writes within a segment; variable-sized values can
still produce unequal byte usage. The factor is in the inclusive range 1..255 and is stored as a byte
in segment metadata. `Table.SetShardingFactor` changes the factor by starting a new segment;
existing segments keep their recorded factors.

We already have a metadata file for each segment. We can go ahead and save the sharding factor in the metadata
file. This will give us enough information to find data contained within the segment.

## Iteration 9: Multi-table support

A nice-to-have feature would be the ability to support multiple tables. Each table would have its own namespace, and
data in one table would not conflict with data in another table.

This is simple! Let's just run a different DB instance for each table.

![](resources/iteration9.png)

Since each table might want to have its own configuration, that configuration is supplied when the table is built.
These per-table settings are held in memory only and are not persisted to disk, so they must be provided again each
time the database is opened.

## Putting it all together: LittDB

![littdb](resources/littdb-big-picture.png)

### Current code paths

[`littbuilder.NewDB`](../littbuilder/db_impl.go) validates configuration and locks the roots in
`Config.Paths`. `BuildTable` opens each named table once per DB lifetime and constructs its disk table
and optional cache wrapper. Per-table TTL, cache sizes, sharding factor, and GC filter come from
[`TableConfig`](../table_config.go) each time the database is opened. Existing segment metadata retains
the factor and seal time needed to read and collect those segments. Storage-root changes require a
stop, reconfiguration and restart; [rebase](littdb_cli.md#litt-rebase) moves files off removed roots.

The write path is:

1. [`DiskTable.PutBatch`](../disktable/disk_table.go), also used by `Put`, validates keys, values,
   aliases and ranges before populating the unflushed data map. A primary or secondary key may contain
   at most 65,535 bytes; a value may contain at most 4,294,967,295 bytes. Callers supply unique keys and
   must not mutate supplied slices.
2. The [`control loop`](../disktable/control_loop.go) serializes writes and checks
   `Segment.HasCapacity` for the next round-robin shard. If the whole primary/secondary group cannot
   fit, it seals the current segment and retries the group on a new segment before sending any part.
   Size and key-count targets also trigger rotation.
3. [`Segment.Write`](../disktable/segment/segment.go) sends raw value bytes to the selected shard and
   primary/secondary key records to the key file. V3 addresses encode segment index, raw byte offset,
   shard ID and value length in 13 bytes. There is no per-value length prefix in the V3 value file.
   The complete value extent must fit within the uint32 file-size bound.
4. The [flush loop](../disktable/flush_loop.go) flushes the segment files, publishes keymap entries,
   and removes flushed entries from the unflushed data map. A primary and its secondary records form
   a recovery group; an incomplete group is discarded on recovery. A batch of distinct requests is
   not a transaction.

The read path checks the optional write cache, then read cache, then the disk table's unflushed map,
then the keymap and segment value files. A keymap is either in memory or backed by PebbleDB. In-memory
keymaps are rebuilt from segment key files after restart; a durable keymap can avoid a full rebuild.
An address locates raw bytes without recomputing a shard from the key. Secondary keys point directly
to subranges of those same bytes.

### Retention and cache visibility

[`cachedTable.CacheAwareGet`](../dbcache/cached_table.go) validates every cache hit with `base.Exists`.
`cachedTable.Exists` delegates directly to the base table. When garbage collection removes a key,
subsequent cached reads of its primary key or aliases return absent even if old bytes remain physically
in either cache. This is an existence check, not eager cache invalidation; eviction and cache size
changes reclaim cache memory independently. The regression test
[`TestRepair_113_CacheExpiryVisibility`](../dbcache/repair_113_test.go) covers read/write caches,
primary and secondary keys, base and wrapper GC, iteration, and close/reopen behavior.

The control loop collects sealed segments in age order using their seal times and the current table
TTL. A zero TTL disables online GC. An optional `GCFilter` can retain an entire segment until all its
keys are eligible. Open iterators suspend GC until closed, and segment reference counts defer file
removal while readers hold references. Iteration reads the base table directly and bypasses the caches.

### Durability and snapshots

`Table.Flush()` waits for writes completed before the call; writes overlapping it are outside that
boundary. `DB.Close()` drains accepted data, and callers must stop writers first and check the close
error. Crash-durability guarantees require `Config.Fsync = true` (the default) and successful completion;
disabling fsync leaves data vulnerable to an OS or hardware crash.

`Config.SnapshotDirectory` publishes sealed segments through snapshot symlinks backed by hard links.
Publication and consumption use per-table upper/lower boundary files so a consumer does not reclaim
unpublished segments. The snapshot does not include the actively written segment. Consumers must
[push or prune](littdb_cli.md#litt-push) the snapshot to release retained files; otherwise disk usage grows.
Restore from ordinary backup roots requires `litt push --no-gc` and a stopped destination database.

These are local storage guarantees. Paxeer's kernel proposes blocks as part of the same BFT network;
a receipt is final when its block is committed. A local flush or snapshot does not establish that
consensus result. Tenant domains do not require a separate database process per sub-chain.
