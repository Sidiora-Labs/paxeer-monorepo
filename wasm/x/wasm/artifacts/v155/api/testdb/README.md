# Test database

This package provides an in-memory B-tree database for tests. Keys and values
returned by `MemDB` must be treated as read-only. Iterators must be closed by
the caller. The database API follows [tm-db](https://github.com/tendermint/tm-db/tree/v0.6.7).
