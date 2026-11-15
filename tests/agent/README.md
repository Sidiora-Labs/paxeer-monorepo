# Agent SDK tests

This directory holds conformance and integration tests for the Python `layerx_sdk` package in [services/agent/sdk/python](../../services/agent/sdk/python). The tests cover account derivation, signed EVM transactions, agent envelopes, native program calls, payments, market operations, gateway reads and receipt verification. They exercise client behavior for the single Paxeer network: the LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it, and a receipt is final when the validators commit its block.

## Layout

- [sdk/](sdk/) — SDK tests, currently under [sdk/python/](sdk/python/).
- [sdk/python/conftest.py](sdk/python/conftest.py) — adds the Python SDK source directory to the import path.
- [sdk/python/payment_receipt_evidence.py](sdk/python/payment_receipt_evidence.py) — loads node-produced payment transactions and checks their calldata against payment logs.
- [sdk/python/program_receipt_evidence.py](sdk/python/program_receipt_evidence.py) — loads signed program receipt evidence and supplies verification helpers.
- [sdk/python/fixtures_22_33_701/](sdk/python/fixtures_22_33_701/) — recorded node JSON-RPC exchanges used by gateway-read tests.
- [sdk/python/test_account_derivation.py](sdk/python/test_account_derivation.py) — derivation vectors, signing and account-binding transport tests.
- [sdk/python/test_agent_operation_envelope.py](sdk/python/test_agent_operation_envelope.py) — agent envelope checks and a configurable HTTP probe.
- [sdk/python/test_native_effect.py](sdk/python/test_native_effect.py) — cross-language native-effect request encoding and transport binding checks.
- [sdk/python/test_program_http.py](sdk/python/test_program_http.py) — program discovery, simulation, submission and recovery against gateway and node processes.
- [sdk/python/test_integration.py](sdk/python/test_integration.py) — HTTP transport, authentication, package metadata and request identity tests.
- [sdk/python/test_px.py](sdk/python/test_px.py) — node and hosted gateway read decoding, history and transport tests.

## Build and test

This test directory has no standalone build. From the repository root, use the target in the root [Makefile](../../Makefile):

```sh
make agent-test-sdk-py
```

The target runs SDK generation checks, the central and package-local Python tests, then byte-compiles the SDK and its examples. Its Python commands are:

```sh
PYTHONPATH=services/agent/sdk/python python3 -m pytest -q tests/agent/sdk/python services/agent/sdk/python/tests
PYTHONPATH=services/agent/sdk/python python3 -m compileall -q services/agent/sdk/python/layerx_sdk services/agent/sdk/python/examples
```

Use Python 3.11 or newer, pytest and the SDK's `derivation` extra for cryptography support. The root target also uses Cargo for SDK generation checks. Account-binding transport tests require `anvil`; native-effect and program HTTP tests use the external inputs listed below. See the SDK's [testing guide](../../services/agent/sdk/python/TESTING.md) for package-local test discovery.

## Configuration

The SDK's [pyproject.toml](../../services/agent/sdk/python/pyproject.toml) declares pytest discovery for this suite and the package-local tests. The local [conftest.py](sdk/python/conftest.py) configures SDK imports.

| Environment variable | Use |
| --- | --- |
| `PYTHONPATH` | SDK import path in the root Makefile commands. |
| `ANVIL_BIN` | Executable used by account-binding transport tests. |
| `NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST` | File containing the signed native-effect request consumed by native-effect tests. |
| `NATIVE_EFFECT_PYTHON_CANONICAL_OUTPUT` | Optional destination for canonical request bytes. |
| `LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE` | JSON fixture describing the gateway, node, program transactions and receipt trust inputs. |
| `PAXEER_X_PROGRAM_HTTP_CONTROL` | JSON control file used by program HTTP disconnect and recovery tests. |
| `PAXEER_X_AGENT_ENVELOPE_CASE` | JSON case file read by the agent envelope probe. |

The program HTTP fixture references source manifests and transaction payload files. The envelope case file references TLS files, session credentials and state files. These filenames are supplied by the input documents. Shared receipt vectors come from [sequencer-receipt-v1.json](../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json).

## Documentation

- [Agent compatibility](../../docs/agent/COMPATIBILITY.md)
- [SDK exports and source paths](../../docs/system/sdk.md)
- [Python quickstart](../../docs/platform/content/quickstart/python.md)
- [Programs guide](../../docs/platform/content/guide/programs.md)
- [Receipt verification guide](../../docs/platform/content/guide/receipts.md)
- Documentation site: [XDK](../../docs/site/paxeer-docs/public/markdown/xdk.md), [agent runtime](../../docs/site/paxeer-docs/public/markdown/agent-runtime.md), [Programs](../../docs/site/paxeer-docs/public/markdown/programs.md) and [receipts](../../docs/site/paxeer-docs/public/markdown/receipts.md).
