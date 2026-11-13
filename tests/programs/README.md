# Programs native tests

This directory holds native C integration tests, WebAssembly guest builders, and source checks for Programs execution in the Paxeer network. The tests exercise the kernel's Programs module together with the Rust runtime and sandbox host interface, covering deployment, calls, account authority, metering, monetary effects, storage, and recovery. Programs execute within the network's kernel; their receipts are final when the containing block is committed.

## Layout

| File | Purpose |
| --- | --- |
| `test_registration.c` | Module registration, activity routing, ABI transitions, and sandbox bounds. |
| `test_module_registration.c` | Module registration state and protocol selection. |
| `test_lifecycle.c` | Deployment, upgrades, interfaces, validation, migration, and recovery. |
| `audit_22_30_2.h` | Lifecycle cases for migration accounting, storage references, and maintenance recovery. |
| `test_call_activity.c` | Signed call dispatch, capabilities, retained artifacts, program spending, and receipt fixture encoding. |
| `test_call_builders.c` | Terminal and event builder completeness, byte ordering, and artifact hash validation. |
| `test_metered_call.c` | Metered execution, fee settlement, schedule identity, and replay. |
| `test_metering_schedule.c` | Metering schedule record validation. |
| `test_fee_governance.c` | Fee policy activation, authority, history, and occupancy arithmetic. |
| `test_monetary_law.c` | Transfer atomicity, spending authority, balances, and failure settlement. |
| `test_transfer_principal.c` | Transfer source ownership and spend-account authority. |
| `test_call_escrow_principal.c` | Escrow source authority and transfer destination resolution. |
| `test_accounts.c` | Program account derivation, registration, registry growth, and shared state vectors. |
| `test_winddown.c` | Program account wind-down, exits, signatures, and spending restrictions. |
| `test_programs_abi5_admission.c` | Guest ABI admission through module validation, deployment, and ledger execution. |
| `test_execution_context.c` | Execution context observations across guest call topologies. |
| `test_cache_native_equivalence.c` | Native execution results and accounting across runtime cache configurations. |
| `balance_read_guests.h` | WebAssembly guest builders for host balance reads. |
| `test_language_abi.c` | Guest language parity for host calls, storage, and terminal encoding. |
| `test_terminal_v5.c` | Terminal corpus encoding for successful calls, refusals, callbacks, and settlement. |
| `test_spend_capability_composition.c` | Spending grants, narrowed child capabilities, proposals, and atomic refusal. |
| `test_reference_custody.c` | Escrow and vault guest custody through native calls and receipt evidence. |
| `test_ai_host_boundary.c` | AI-market guest execution, account profiles, replay, and persisted reward settlement. |
| `test_market_attestation.c` | Market attestation, lease accounting, billing, and settlement through native execution. |
| `test_blob_admission.c` | Blob admission when storage capacity is reached. |
| `test_blob_lifecycle.c` | Blob retirement, provenance, protected references, profiles, and state proofs. |
| `test_paxai_native_gaps.c` | Module version pinning and retirement dispatch. |
| `check-abi-drift.sh` | Checks canonical ABI vectors through the Programs vector generator. |
| `runtime-module-boundaries.sh` | Checks runtime module layout and host-family dependencies, including its own rejection cases. |

## Build and test

Run these targets from the repository root using the [root Makefile](../../Makefile). Native test targets compile the C sources and link the kernel library with the Programs sandbox library. There is no separate Makefile or package manifest in this directory.

| Command | Purpose |
| --- | --- |
| `make programs-build` | Builds the locked Rust workspace with the sandbox host FFI feature. |
| `make programs-core-test` | Builds and runs the native registration, lifecycle, monetary, transfer, escrow, call, metering, account, and wind-down tests. |
| `make programs-module-boundaries` | Runs the runtime layout and dependency check and its rejection cases. |
| `make programs-abi-drift` | Runs the ABI vector check and the Rust runtime linker tests. |
| `make test-programs-metered-call` | Builds and runs the metered call tests. |
| `make test-programs-abi5-admission` | Builds and runs the ABI admission tests. |
| `make test-program-call-builders` | Builds and runs the call builder tests. |
| `make test-paxai-blob-lifecycle` | Builds and runs the blob lifecycle tests. |
| `make test-paxai-native-gaps` | Builds and runs module pinning and retirement tests. |
| `make paxai-host-boundary-build` | Builds the AI-market guest and native host executable, checks the guest size, and executes the guest through the host. |
| `make programs-check-native-lifecycle-fixtures` | Checks shared lifecycle fixtures using the native call encoder. |
| `make programs-check-executed-v4-fixture` | Checks the executed program fixture using the native call encoder. |
| `make programs-test` | Runs the broader Programs suite, including native, protocol, runtime, SDK, porting, fuzz, and differential tests. |

The native targets use a C toolchain and the Rust workspace. Their link recipes use OpenSSL, threading, dynamic loading, and math libraries; account and blob-admission targets also link SQLite. The broader suite also builds WebAssembly guests and language SDKs. See the [workspace build reference](../../docs/wiki/ProgramsWorkspace.md) for toolchain and guest packaging requirements.

## Configuration

The native sources read these environment variables for fixture inputs and outputs:

| Variable | Reader and purpose |
| --- | --- |
| `PAXEER_X_PROGRAM_VALIDATION_INPUTS` | `test_lifecycle.c`: directory for exported deployment validation payloads. |
| `PAXEER_X_NATIVE_MIGRATION_LEGACY_FIXTURE_FD` | `test_lifecycle.c`: descriptor for a migration fixture. |
| `PAXEER_X_NATIVE_MIGRATION_LEGACY_SOURCE_REVISION` | `test_lifecycle.c`: migration fixture provenance. |
| `PAXEER_X_NATIVE_MIGRATION_AUTHORITY_FD` | `test_lifecycle.c`: descriptor for the migration signing authority. |
| `PAXEER_X_NATIVE_MIGRATION_GOVERNOR_FD` | `test_lifecycle.c`: descriptor for the governance signing authority. |
| `PAXEER_X_NATIVE_MIGRATION_RUN` | `test_lifecycle.c`: migration output directory. |
| `PAXEER_X_SPEND_GUESTS` | `test_spend_capability_composition.c`: guest artifact directory. |
| `PAXEER_X_SPEND_RESULTS` | `test_spend_capability_composition.c`: terminal output directory. |
| `PAXEER_X_MAINLINE` | `test_terminal_v5.c`: terminal corpus provenance. |

The root Makefile accepts `BUILD_DIR`, `CC`, `CPPFLAGS`, `CFLAGS`, `EXTRA_LDFLAGS`, `RUN_PREFIX`, and `PROGRAMS_CARGO` for native builds and execution. `PAXAI_CHAIN_DOMAIN` selects the AI-market guest's chain domain at build time. Cargo uses [programs/Cargo.toml](../../programs/Cargo.toml) and [programs/.cargo/config.toml](../../programs/.cargo/config.toml), which configures vendored dependencies and offline resolution.

Account tests read [program_account_state_v2.vec](../vectors/program_account_state_v2.vec). Guest-driven executables also accept artifact paths or directories through their command-line arguments.

## Documentation

- [Programs execution and ABI reference](../../docs/wiki/Programs.md)
- [Programs workspace and native build targets](../../docs/wiki/ProgramsWorkspace.md)
- [Programs documentation site page](../../docs/site/paxeer-docs/public/markdown/programs.md)
- [Programs developer workspace documentation site page](../../docs/site/paxeer-docs/public/markdown/program-workspace.md)
