# Test database

This package provides an in-memory B-tree database for tests. `MemDB` stores
the key and value slices supplied to `Set` without copying them and returns
stored slices from reads. Treat both supplied and returned slices as read-only;
copy them before modifying them. Iterators must be closed by the caller.
The database API follows [tm-db](https://github.com/tendermint/tm-db/tree/v0.6.7).
