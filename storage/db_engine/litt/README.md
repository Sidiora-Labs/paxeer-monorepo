![](docs/resources/littdb-logo.png)

# Contents

- [License](docs/licenses/README.md)
- [What is LittDB?](#what-is-littdb)
    - [Features](#features)
    - [Consistency Guarantees](#consistency-guarantees)
    - [Planned/Possible Features](#plannedpossible-features)
    - [Anti-Features](#anti-features)
- [API](#api)
    - [Overview](#overview)
    - [Getting Started](#getting-started)
    - [Configuration Options](#configuration-options)
    - [CLI](#littdb-cli)
- [Definitions](#definitions)
- [Architecture](docs/architecture.md)
- [Filesystem Layout](docs/filesystem_layout.md)

# What is LittDB?

LittDB is a highly specialized embedded key-value store that is optimized for the following workload:

- high write throughput
- low read latency
- low memory usage
- write once, never update
- automatic retention via a [TTL](#ttl) (time-to-live) mechanism

LittDB lives in `storage/db_engine/litt` in the Paxeer monorepo. It is an embedded Go library;
its [CLI](docs/littdb_cli.md) operates on local storage files and backups. Storage flushes establish
local durability, not network consensus finality. Paxeer is one BFT network with EVM chain ID 125;
the kernel is part of that network and proposes blocks. Sub-chains are tenant domains in the
database, not separate infrastructure. A LittDB table supplies a local key namespace; this package
does not itself provision tenant domains.

## Features

The following features are currently supported by LittDB:

- writing values (once)
- reading values
- [TTLs](#ttl) and automatic (lazy) deletion of expired values
- [tables](#table) with non-overlapping namespaces
- multi-drive support (data can be spread across multiple physical volumes)
- incremental backups (both local and remote)
- primary and secondary keys up to 65,535 bytes (2^16 - 1)
- values up to 4,294,967,295 bytes (2^32 - 1, `math.MaxUint32`)
- incremental snapshots
- incremental remote backups

## Consistency Guarantees

The consistency guarantees provided by LittDB are more limited than those provided by typical general-purpose
transactional databases. This is intentional, as the intended use cases of LittDB do not require higher order
consistency guarantees.

- thread safety
- [read-your-writes consistency](#read-your-writes-consistency)
- crash [durability](#durability) for data that has been [flushed](#flushing)
- [atomic](#atomicity) writes
    - Although [batched writes](#batched-writes) are supported (for performance), batches are not [atomic](#atomicity).
      Each individual write within a batch is [atomic](#atomicity), but the batch as a whole is not. That is to say,
      if the computer crashes after a [batch](#batched-writes) has been written but before [flushing](#flushing),
      some of the writes in the [batch](#batched-writes) may be [durable](#durability) on disk, while others may
      not be.

## Planned/Possible Features

The following features are planned for future versions of LittDB, or are technically feasible if a strong
enough need is demonstrated:

- dynamic multi-drive support: Drives can currently only be added/removed with a DB restart.
  It's currently fast, but not instantaneous. With this feature, drives can be added/removed on the fly.
- read-only mode from an outside process
- more keymap implementations (e.g. badgerDB, a custom solution, etc.)
- data check-summing and verification (to protect/detect disk corruption)
- keys and values up to 2^64 bytes in size

## Anti-Features

LittDB deliberately omits general-purpose database features. Its supported write path is append-only:

- mutating existing values (once a value is written, it cannot be changed)
- deleting individual values on demand (TTL handles retention; `Drop` and `Destroy` remove whole tables/databases)
- transactions (individual operations are atomic, but there is no way to group operations atomically)
- fine granularity for [TTL](#ttl) (all data in the same table must have the same TTL)
- multi-computer replication (LittDB is designed to run on a single machine)
- data encryption
- data compression
- any sort of query language other than "get me the value associated with this key"
- key-ordered data iteration (tables can only be iterated in insertion order or reverse insertion order)

# API

## Overview

Below is a high level overview of the LittDB API. For more detailed information, see the inline documentation in the
interface files.

Source: [db.go](db.go)

```go
type DB interface {
	BuildTable(config TableConfig) (Table, error)
	Close() error
	Destroy() error
}
```

Source: [table.go](table.go)

```go
type Table interface {
	Name() string
	Put(key []byte, value []byte, secondaryKeys ...*types.SecondaryKey) error
	PutBatch(batch []*types.PutRequest) error
	Get(key []byte) (value []byte, exists bool, err error)
	CacheAwareGet(key []byte, onlyReadFromCache bool) (value []byte, exists bool, hot bool, err error)
	Exists(key []byte) (exists bool, err error)
	Flush() error
	Size() uint64
	KeyCount() uint64
	SetTTL(ttl time.Duration) error
	SetShardingFactor(shardingFactor uint8) error
	SetWriteCacheSize(size uint64) error
	SetReadCacheSize(size uint64) error
	Drop() error
	Iterator(reverse bool) (Iterator, error)
	GetOldestKey() (key []byte, exists bool, err error)
	GetNewestKey() (key []byte, exists bool, err error)
}
```

`Iterator(false)` walks keys in insertion order (oldest first) and `Iterator(true)` in reverse insertion order.
An iterator sees a snapshot of the keys present when it was created and must be closed when no longer needed.
Garbage collection is suspended while an iterator is open. Iterators themselves are not safe for concurrent use.

Both primary and secondary keys must not exceed 65,535 bytes (2^16 - 1); this is one byte less
than 64 KiB. Values must not exceed 4,294,967,295 bytes (2^32 - 1, `math.MaxUint32`). These are the checks in
[`DiskTable.PutBatch`](disktable/disk_table.go), which also backs `Put`. Nil key and value slices
are rejected; non-nil empty slices are allowed. Supply non-nil requests and secondary-key entries.
Keys must be unique within a table, including aliases. Duplicate keys within a single request are
rejected; callers must prevent collisions across requests. `Config.DoubleWriteProtection` defaults
to false and is not a substitute for assigning unique keys. Do not mutate supplied byte slices or
returned values; copy them if mutation is needed.

For example, with `bytes` imported, `bytes.Repeat([]byte{'k'}, 65535)` is a legal key,
while `bytes.Repeat([]byte{'k'}, 65536)` is rejected. A small value such as `[]byte("payload")`
works with the legal key; the value limit is `math.MaxUint32`, not 2^32 bytes. Secondary keys
have the same 65535-byte limit and must use distinct bytes from the primary and other aliases.

Source: [put_request.go](types/put_request.go)

```go
type PutRequest struct {
	Key           []byte // at most 65,535 bytes (math.MaxUint16)
	Value         []byte // at most 4,294,967,295 bytes (math.MaxUint32)
	SecondaryKeys []*SecondaryKey // optional, may be nil
}
```

Source: [secondary_key.go](types/secondary_key.go)

```go
type SecondaryKey struct {
	Key    []byte // a unique alias, at most 65,535 bytes (math.MaxUint16)
	Offset uint32 // byte offset into the parent value
	Length uint32 // length of the byte range (Offset+Length <= len(Value))
}
```

A secondary key is a first-class key that aliases a sub-range (or the whole) of the parent value's
bytes. `Get`, `Exists`, `KeyCount`, and TTL all treat secondary keys identically to primary keys.
Secondary keys share the value's bytes on disk, so each one costs roughly one keymap entry rather
than duplicating value bytes.

## Getting Started

This example opens a table, writes and flushes a value, reads it, and checks the close error.
Stop all writers before closing the database.

```go
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
)

func example() (err error) {
	config, err := litt.DefaultConfig("./litt-data")
	if err != nil {
		return err
	}
	db, err := littbuilder.NewDB(config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	// Build each table once per DB lifetime, whether new or reopened.
	table, err := db.BuildTable(litt.DefaultTableConfig("my-table"))
	if err != nil {
		return err
	}
	key := []byte("example-key") // Use a fresh key for each new write.
	if exists, err := table.Exists(key); err != nil {
		return err
	} else if !exists {
		if err := table.Put(key, []byte("example-value")); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	value, exists, err := table.Get(key)
	if err != nil {
		return err
	}
	fmt.Printf("exists=%t value=%s\n", exists, value)
	return nil
}

func main() {
	if err := example(); err != nil {
		log.Fatal(err)
	}
}
```

## Configuration Options

Start with [`DefaultConfig`](littdb_config.go) and set `Config.Paths` to every storage root.
Use [`DefaultTableConfig`](table_config.go) for each `BuildTable` call: the defaults are no TTL,
eight shards, and disabled read/write caches. Table settings other than the name must be supplied
again after a restart; each existing segment retains the sharding factor needed to read its files.
[`RuntimeConfig`](runtime_config.go) holds the context, logger, clock, and optional fatal-error
callback; `NewDB` uses defaults when it is omitted.

Keep `Config.Fsync` enabled (the default) for crash durability. `SnapshotDirectory` enables rolling
snapshots of sealed segments; it is empty by default. See the [CLI backup and restore guide](docs/littdb_cli.md#litt-push).
Changing storage roots requires stopping the database, optionally [rebasing](docs/littdb_cli.md#litt-rebase)
files, updating `Paths`, and reopening. Changing `Table.SetShardingFactor` is a separate runtime operation.

## LittDB CLI

The LittDB has a CLI utility for offline manipulation of DB files. Build it with `make build` from this directory
(the binary lands in `./bin/litt`). See the [LittDB CLI](docs/littdb_cli.md) docs for more information on how to use it.

# Definitions

This section contains an alphabetized list of technical definitions for a number of terms used by LittDB. This
list is not intended to be read in order, but rather to be used as a reference when reading other parts of the
documentation.

## Address

An address partially describes the location on disk where a [value](#value) is stored. Together with a [key](#key),
the [value](#value) associated with a [key](#key) can be retrieved from disk.

An address contains the following information:

- the [segment](#segment) [index](#segment-index) where the [value](#value) is stored
- the [shard](#shard) within that segment that holds the [value](#value)
- the offset within the [value file](#segment-value-files) where the first byte of
  the [value](#value) is stored
- the length of the [value](#value) in bytes

Retrieving a [value](#value) starting from an Address is a self-contained
operation that does not need to consult any segment-level metadata or recompute anything from the [key](#key).

## Atomicity

In the context of this document, atomicity means that an operation is either done completely or not at all. That is
to say, if there is a crash while an operation is in progress, the operation will either be completed when the
database is restarted, or it will not be completed at all.

As a specific example, if writing a [value](#value) and there is a crash, either the entire [value](#value) will be
written to disk and available when the database is restarted, or the [value](#value) will be completely absent.
It will never be the case that only part of the [value](#value) is written to disk.

## Cache

LittDB maintains an in-memory cache of [key](#key)-[value](#value) pairs. Data is stored in this cache when a value
is first written (write cache), as well as when it is read from the base table (read cache), when those caches
are enabled. Both default to size zero. The caches are not persistent. On a cache hit,
[`cachedTable.CacheAwareGet`](dbcache/cached_table.go) checks `base.Exists` before returning bytes;
`Exists` delegates directly to the base. Garbage collection can leave bytes physically cached until eviction,
but keys removed from the base table are no longer returned. See the [architecture](docs/architecture.md#putting-it-all-together-littdb).

## Batched Writes

LittDB supports batched write operations. Multiple write operations can be grouped together and passed to the database
as a single operation. This may have positive performance implications, but is semantically equivalent to writing each
value individually. A batch of writes is not [atomic](#atomicity) as a whole, but each individual write within the
batch is [atomic](#atomicity). That is to say, if there is a crash after a batch of writes has been written but before
it has been [flushed](#flushing), some of the writes in the batch may be [durable](#durability) on disk, while others
may not be.

## Durability

In this context, the term "durable" is used to mean that data is stored on disk in such a way that it will not be lost
in the event of a crash. Data that has been [flushed](#flushing) is considered durable. Data that has not been flushed
is not considered durable. That doesn't mean that the data will be lost in the event of a crash, but rather that it
is not guaranteed to be present after a crash.

These guarantees require `Config.Fsync = true` and successful `Flush`/`Close` calls. With fsync disabled,
flushing buffers does not guarantee survival of an OS or hardware crash.

There are some limits to the strength of the durability guarantee provided by LittDB. For example, some drives buffer
data in internal buffers before writing it to disk, and do not necessarily write data to disk immediately. LittDB is
only as robust as the OS/hardware it is running on. This is true for any database, but it is worth mentioning here
for the sake of completeness.

## Flushing

Calling `Flush()` causes all data previously written to be written [durably](#durability) to disk. A call to `Flush()`
blocks until all data that was written prior to the call to `Flush()` has been written to disk.

It is ok to never call `Flush()`. As internal buffers fill, data is written to disk automatically. However, calling
`Flush()` can be useful in some cases, such as when you want to ensure that data is written to disk before proceeding
with other operations.

If `Flush()` is never called, data becomes durable through two mechanisms:

- When a [segment](#segment) becomes full, it is made immutable and a new segment is created. As part of the process
  of making a segment immutable, all data in the segment is fully written to disk.
- When the database is cleanly closed via `DB.Close()`, all previously accepted, unflushed data is written to
  disk before a successful return. Check its error. Writes overlapping `Close()` are not guaranteed durable;
  stop and join writers before closing.

After stopping and joining writers, close the database and handle failure explicitly:

```go
if err := db.Close(); err != nil {
	return fmt.Errorf("close LittDB: %w", err)
}
```

`Flush()` makes no guarantees about the [durability](#durability) of data written concurrently with the call to
`Flush()` or after the call to `Flush()` has returned. It's not harmful to write data concurrently with a call to
`Flush()` as long as it is understood that this data may or may not be [durable](#durability) on disk when the call
to `Flush()` returns.

The following example demonstrates the consistency guarantees provided by the `Flush()` operation:

![](docs/resources/flush-visual.png)

In this example there are two threads performing operations, `Thread 1` and `Thread 2`. `Thread 1` writes `A`, `B`,
and `C`, calls `Flush()`, and then writes `D`. `Thread 2` writes `W`, `X`, `Y`, and `Z`. `Time α` is the moment
when the flush operation is invoked, and `Time β` is the moment when the flush operation returns.

All write operations that have completed at `Time α` before the flush operation is invoked are [durable](#durability)
when the flush operation returns at `Time β`. These are `A`, `B`, `C`, and `W`. Although writing `X` begins prior to
`Time α`, since it is not complete at `Time α`, the flush operation does not guarantee that `X` is
[durable](#durability) when it returns at `Time β`. The same is true for `Y`, `Z`, and `D`.

Note that just because an operation is not guaranteed to be [durable](#durability) when `Flush()` returns does not mean
that is guaranteed to be not [durable](#durability). If the computer crashes after `Time β` but before the next call
to `Flush()`, then `X`, `Y`, `Z`, and `D` may or may not be lost as a result.

## Key

A key in a key-[value](#value) store. A key is a byte slice that is used to look up a [value](#value) in the database.

LittDB is agnostic to the contents of the key, other than requiring that keys be unique within a [table](#table).
Although large keys are supported, performance has been tuned under the assumption that keys are generally small
compared to [values](#value). The use case LittDB was originally intended for uses 32-byte keys.

## Keymap

At a conceptual level, a keymap is a mapping from [keys](#key) to [addresses](#address). In order to look up a
[value](#value) in the database one needs to know two things: the [key](#key) and the [address](#address). The keymap
is therefore necessary to lookup data given a specific [key](#key).

There are currently two implementations of the keymap in LittDB: an in-memory keymap and a keymap that uses PebbleDB.
There are tradeoffs to each implementation. The in-memory keymap is faster, but has higher memory usage and longer
startup times (it has to be rebuilt at boot time). The PebbleDB keymap is slower, but has a lower memory footprint and
faster startup times.

From a thread safety point of view, if a mapping is present in the keymap, the [value](#value) associated with the
entry is guaranteed to be present on disk.

- When writing a new [value](#value), it is first written to disk, and when that is complete the [key](#key) and
  [address](#address) are written to the keymap.
- When deleting a [value](#value), the [key](#key) and [address](#address) are first removed from the keymap, and
  then the [value](#value) is deleted from disk.

LittDB supports reading [values](#value) immediately after they are written, and during that period there may not
be a corresponding entry in the keymap. For more information on how this edge case is handled, information about the
[unflushed data map](#unflushed-data-map).

## Read-Your-Writes Consistency

The definition of read-your-writes consistency is well summarized by its name. If a thread writes a [value](#value)
to the database and then turns around and attempts to read that [value](#value) back, it will either

1. read the [value](#value) that was just written, or
2. read an updated [value](#value) that was written AFTER the [value](#value) that was just written

Note that in LittDB, values are never permitted to be mutated. But when values grow older than their [TTL](#ttl),
the value can be deleted. From a consistency point of view, the garbage collection process is equivalent to an update.
That is to say, if a thread writes a [value](#value), waits a very long time, then reads that same [value](#value)
back again, it is not a violation of read-your-writes consistency if the [value](#value) is not present because the
[garbage collector](#ttl) has deleted it.

An "eventual consistent" database does not necessarily provide read-your-writes consistency. In the author's experience,
such systems can be very difficult to reason about, and can lead to subtle bugs that are difficult to track down.
Read-your-writes consistency is simple, yet powerful and intuitive. Since providing this level of consistency
does not hurt performance, the complexity of its implementation is justified.

## Segment

Data in LittDB [table](#table) can be visualized as a linked list. Each element in that linked list is called a
"segment". A segment can hold many individual [values](#value). Old data is near the beginning of the list, and new
data is near the end. Old, [expired](#ttl) data is always deleted from the first segment currently in the list. New
data is always written to the last segment currently in the list.

Segments are deleted as a whole. That is, when a segment is deleted, all data in that segment is deleted at the same
time. Normal garbage collection deletes a sealed segment only after its seal age exceeds the table [TTL](#ttl)
and its optional `GCFilter` permits every key. Open iterators suspend garbage collection.

Segments have a target data size. When a segment is full, that segment is made immutable, and a new segment is created
and added to the end of the list.

`Config.TargetSegmentFileSize` is a rollover target for each shard value file, not permission to
exceed the address format. V3 value files store raw value bytes; the address contains a uint32 offset
and uint32 length. The complete value extent, including every secondary alias range, must fit within
`math.MaxUint32` bytes of the selected file. Values cannot be arbitrarily large.

Before writing a primary value and its aliases, the control loop calls `Segment.HasCapacity`. If the
next round-robin shard cannot fit the group, it seals the current segment and retries that unchanged
group once on a fresh segment. `Segment.Write` still checks the bounds. After a write, exceeding the
configured size target, reaching `MaxSegmentKeyCount`, or reaching `TargetSegmentKeyFileSize` also
causes rollover. A single legal value may exceed a smaller configured target, but never the format limit.
See [control_loop.go](disktable/control_loop.go) and [segment.go](disktable/segment/segment.go).
The [capacity regression test](disktable/repair_111_test.go) exercises pre-write rotation at
the uint32 boundary; [TestRepairM_1165_BoundedValue](repair_m1165_test.go) ties this documentation
to the same disk write, address codec and recovery path.

Each segment may split its data into multiple [shards](#shard). The number of shards in a segment is called the
[sharding factor](#sharding-factor). The [sharding factor](#sharding-factor) is configurable, and different segments
may use different [sharding factors](#sharding-factor).

There are three types of files that contain data for a segment:

- [metadata](#segment-metadata-file)
- [keys](#segment-key-file)
- [values](#segment-value-files)

### Segment Index

Each segment has a serial number called a "segment index". The first segment ever created with index `0`, the next
segment created has index `1`, and so on. Segment `N` is always deleted before segment `N+1`, meaning there will
never be a gap in the segment indices currently in use.

### Segment Key File

A segment key file contains the [keys](#key) and [addresses](#address) for all the [values](#value) stored the segment.
At runtime, [keys](#key)-[address](#address) pairs are appended to the key file. It is not read except during the
following circumstances:

- when a [segment](#segment) is deleted, the file is iterated to delete entries from the [keymap](#keymap)
- when the DB is loaded from disk, the data is used to rebuild the [keymap](#keymap). This may not be needed
  in situations where the keymap has durably stored data, and does not need to be rebuilt.

The file name of a key file is `X.keys`, where `X` is the [segment index](#segment-index).

### Segment Metadata File

This file contains metadata about the segment. This metadata is small, and so it can be kept in memory. The file is
read at startup to rebuild the in-memory representation of the segment.

Each metadata contains the following information:

- the [segment index](#segment-index)
- serialization version (in case the format changes in the future)
- the [sharding factor](#sharding-factor) for the segment
- the seal [timestamp](#segment-timestamp), used with the current table [TTL](#ttl) for retention
- whether or not the segment is [immutable](#segment-mutability)

The file name of a metadata file is `X.metadata`, where `X` is the [segment index](#segment-index).

### Segment Mutability

Only the last segment in the "linked list" is mutable. All other segments are immutable.

### Segment Timestamp

The timestamp recorded when the segment is sealed. Garbage collection compares its age with the current
table [TTL](#ttl). The mutable segment has not yet been sealed.

### Segment Value Files

Each segment has one value file for each [shard](#shard) in the segment. Values are appended to the value files.
The [address](#address) of a [value](#value) is the offset within the value file where the [value](#value) begins.

The file name of a value file is `X-Y.values`, where `X` is the [segment index](#segment-index) and `Y` is the
[shard](#shard) index.

## Shard

LittDB supports sharding. That is to say, it can break the data into smaller pieces and spread those pieces across
multiple locations.

Within a [segment](#segment), [values](#value) are assigned to shards in round-robin order at write time: the first
write goes to shard 0, the second to shard 1, and so on, wrapping around once every shard has been used. Each
[value's](#value) shard is recorded in its [address](#address), so reads do not need to recompute the assignment.

This scheme distributes the number of writes evenly across shards within a segment regardless of the
[keys](#key) being written. As a side benefit, an outside attacker cannot craft a sequence of [keys](#key) that
all land in the same shard, since the shard chosen for a given [value](#value) depends only on the order in which
it was written, not on the contents of its [key](#key). This does not guarantee equal byte usage when values differ in size.

### Sharding Factor

The number of [shards](#shard) in a [segment](#segment) is called the "sharding factor". The sharding factor must
be in the inclusive range 1 through 255 (`litt.MaxShardingFactor`). Both the factor in segment metadata
and `TableConfig.ShardingFactor` / `Table.SetShardingFactor` use a single byte (`uint8`). Changing the
factor at runtime seals the mutable segment and starts a new one; existing segments retain their
factors. This requires neither a restart nor a data migration.

## Table

A table in LittDB is a unique namespace. Two [keys](#key) with identical values do not conflict with each other as
long as they are in different tables.

Each table has its own [TTL](#ttl), and all data in the table is subject to that [TTL](#ttl). Each table has its
own [keymap](#keymap) and its own set of [segments](#segment). [Flushing](#flushing) one table does not affect
any other table. Aside from hardware, tables do not share any resources.

In many ways, a table is a stand-alone database. The higher level [API](#api) that works with multiple tables is
provided as a convenience, but does not enhance the performance of the DB in any way.

Each table is configured via a `TableConfig` supplied at [build time](#api) (TTL, sharding factor, cache sizes).
With the exception of the table name, these settings are held in memory only and are not persisted to disk, so they
must be provided again each time the database is opened.

## TTL

TTL stands for "time-to-live". A sealed segment becomes eligible for collection when its seal age
reaches the current table TTL. Deletion is lazy and segment-wide: time in the mutable segment, the
GC interval, a blocking filter, or an open iterator can delay removal beyond a key's nominal age.

TTL is the online retention mechanism for individual entries. A TTL of 0 disables automatic garbage collection.
`Table.Drop()` and `DB.Destroy()` explicitly remove whole tables or the database, and the offline
[prune command](docs/littdb_cli.md#litt-prune) removes old segments. Do not drop a table concurrently with
operations using that table. A `TableConfig.GCFilter`, if supplied, can delay collection beyond the TTL;
it must return true permanently once a key becomes eligible.

## Unflushed Data Map

An in-memory map that contains [keys](#key)-[values](#value) pairs that are not yet [durable](#durability) on disk.
Entries are added to the map when a [value](#value) is written, and removed when the [value](#value) is fully
written to both the [keymap](#keymap) and the [segment](#segment) files.

This data structure is not to be confused with the [cache](#cache). Its purpose is not to improve performance, but
rather to provide [read-your-writes consistency](#read-your-writes-consistency).

## Value

The value in a key-[value](#value) store. A value is a byte slice that is associated with a [key](#key) in the database.
LittDB is optimized to support large values, although small values are perfectly fine as well. Writing the X bytes
of data as a single large value is more efficient than writing X bytes of data as Y smaller values.

# Architecture

For a detailed overview of the architecture of LittDB, see the [Architecture](docs/architecture.md) docs.

# Filesystem Layout

For information about how LittDB arranges its internal files, see the [Filesystem Layout](docs/filesystem_layout.md)
docs.
