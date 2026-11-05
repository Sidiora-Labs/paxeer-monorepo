# Paxd Rosetta image source

`Dockerfile.paxd` uses the repository root as its build context:

```sh
docker build -f sdk/contrib/rosetta/node/Dockerfile.paxd -t paxd-rosetta:test .
```

The recipe uses the root Go and Rust toolchains, runs `chain.mk build` including
the native kernel and program libraries, and packages `build/paxd` with both
versioned Wasm VM libraries. It starts `paxd start` with Rosetta enabled through
the server's `PAXD_ROSETTA_*` configuration. Rosetta listens on port 8080 and
uses the node's local gRPC and consensus RPC. There is no registered
`paxd rosetta` command.

Provision a fresh, compatible local test chain in `/home/rosetta/.pax` before
starting the image, mounting the initialized home writable by UID/GID 10001.
This recipe does not initialize genesis, import archived state or certify a
Rosetta data/construction run. The configuration examples are copied explicitly
to `/opt/rosetta/configuration`; they are retained test inputs, not a replacement
for compatible chain initialization.

The default image excludes `data.tar.gz`. The optional `archived-fixture` target
copies it, without extraction, to `/opt/rosetta/archived/data.tar.gz` with mode
0400 owned by root. The archive contains public **test-only signing keys** and
historical `simd` state. Use this target only to inspect the committed fixture;
never use those keys on a public network or restore that state into `paxd`.

The former `Dockerfile` and Compose entry points remain withdrawn by task
22.33.1357. `Dockerfile.historical` stays byte-for-byte historical. The owning
SDK domain must retire the old Makefile callers; integrating this explicit
recipe into a new orchestration needs a separate qualified change.
