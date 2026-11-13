# Test vectors

This directory holds shared test vectors and helper scripts for the kernel,
Programs runtime, SDKs and proof consumers in the Paxeer network. The fixtures
describe canonical encodings, rejection cases, signatures, state proofs and
replay digests so implementations can check the same byte representations.

## Layout

- `budget-proof-v2/` — budget proof export and recorded node responses.
- `chains/` — virtual-chain ABI selectors, calldata, return values and events in `abi-vectors.json`.
- `checkpoint/` — checkpoint and attestation timing cases, plus `generate.py`.
- `codec/` — canonical and adversarial `.lxv` cases, Unicode normalization data and its license, plus the replay corpus generator.
- `names/` — naming resolver layout, indexer responses, storage inclusion and absence proofs, validator set and commit fixtures.
- `signatures/` — RFC 8032 text, Wycheproof cases, secp256k1 headers, source hashes in `sources.json` and the independent checker `check.py`.
- `native-budget-records.json` — encoded native budget records.
- `native-head-attestation.json` — program head attestation payload, digest and signature.
- `native-module-evidence.json` — native module key, value and proof evidence.
- `program_account_state_v2.vec` — program account registration, state commitments and proof boundary cases.
- `qualification_replay_10m.digest` — expected replay digests for the qualification corpus.
- `replay_corpus.lxb` — binary activity replay corpus.

## Build and test

The directory holds data files and Python helpers; it has no standalone build.
Run the following targets from the repository root, as defined in the root
[Makefile](../../Makefile):

| Command | Purpose |
| --- | --- |
| `make test-codec-vectors` | Build and run the C reader against `codec/valid.lxv` and `codec/adversarial.lxv`, checking results, canonical bytes and hashes. |
| `make test-codec-limits` | Build and run composite codec tests, including the Unicode normalization cases. |
| `make agent-prepare-codec-corpus` | Run `codec/generate.py` to generate `codec/replay_corpus.lxb` from kernel activity declarations. |
| `make agent-test-vectors` | Prepare the codec corpus and run the `layerx-types` vector tests. |
| `make qualify-replay` | Run the replay matrix and compare its output with `qualification_replay_10m.digest`; prerequisites are described in [Qualification](../../docs/QUALIFICATION.md). |

The signature checker in `signatures/check.py` uses Python `cryptography` to
check source hashes and compare the runtime signature vectors with the
published authorities. The checkpoint generator uses `cast` to produce its
signatures.

## Configuration

The Python helpers read no environment variables directly.
`checkpoint/generate.py` reads
[`contracts/config/checkpoint-settlement.json`](../../contracts/config/checkpoint-settlement.json).
Its command-line options are `--domain`, `--settlement`, `--output` and
`--keys-file`; the last selects a JSON input file. `signatures/check.py` reads
the source inventory and hashes from `signatures/sources.json`.
`codec/generate.py` reads activity declarations under
[`include/layerx/`](../../include/layerx/) and uses
[`scripts/replica/lxp_replay_fixture.py`](../../scripts/replica/lxp_replay_fixture.py).

## Documentation

- [Programs workspace and signature authorities](../../docs/wiki/ProgramsWorkspace.md)
- [Virtual chains and shared ABI vectors](../../docs/wiki/VirtualChains.md)
- [Build and replay qualification](../../docs/QUALIFICATION.md)
- [Programs workspace on the documentation site](../../docs/site/paxeer-docs/src/app/program-workspace/page.tsx)
