# Vendored Solidity dependencies

The load-test Solidity dependencies are included under `lib/`. `setup.sh`
checks their file-tree digests without downloading dependencies or running
`forge install`.

| Directory | File-tree SHA-256 checked by `setup.sh` |
| --- | --- |
| `lib/openzeppelin-contracts` | `75178fd3042c5dd196588d69f3c5f70b63406c751d5a27a154fc3dd08ff01dba` |
| `lib/solmate` | `b92432abfecaf534c50837d0868430a72b3fabcc3d548a9c850b5c75650739f1` |

The digest covers the sorted per-file SHA-256 output within each dependency
directory, including relative file names. The directories contain Solidity
sources and licenses. [`foundry.toml`](foundry.toml) pins the compiler to
Solidity `0.8.20`; an offline build also needs that compiler available locally.
