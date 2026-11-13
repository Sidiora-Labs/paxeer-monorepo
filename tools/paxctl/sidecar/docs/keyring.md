# Sidecar keyring backend

The sidecar opens a Cosmos SDK keyring at startup when configured to do so.
The keyring is the entry point for all transaction-signing tasks (governance
votes, software-upgrade proposals and deposits).

Configure the keyring directory and backend with the environment variables below.

## Environment contract

| Env | Values | Default | Required when |
|---|---|---|---|
| `SEI_KEYRING_BACKEND` | `test` \| `file` \| `os` | unset (governance signing disabled) | sign-tx tasks in use |
| `SEI_KEYRING_DIR` | directory path | `<home>/keyring-file`, using `--home` or `SEI_HOME`, with `/sei` as the `serve` fallback | optional for all configured backends |
| `SEI_KEYRING_PASSPHRASE` | string | unset | `backend == file` |

For the `file` backend, a trailing `/keyring-file` path segment is stripped before handoff to the SDK — the Cosmos SDK keyring re-appends `keyring-file/` internally, so callers passing `/sei/keyring-file` and `/sei` both end up at `/sei/keyring-file/*`. This matches both operator mental models.

Unset `SEI_KEYRING_BACKEND` is the default: the sidecar starts normally
and rejects sign-tx submissions with `keyring not configured`. The node's
genesis-ceremony tasks (e.g. `generate-gentx`) continue to function — those
use a separate, in-process test backend that is intentionally isolated from
production keys (see "Genesis isolation" below).

Unknown values for `SEI_KEYRING_BACKEND` cause the sidecar to refuse to start.
KMS / HSM / Vault / remote-signer backends are not supported yet.

## Fail-fast semantics

When `SEI_KEYRING_BACKEND` is set, the sidecar:

1. Reads and unsets `SEI_KEYRING_PASSPHRASE` before checking the backend, then
   reads `SEI_KEYRING_BACKEND` and `SEI_KEYRING_DIR`.
2. Validates the backend value and (for `file`) the presence of a passphrase.
   A missing passphrase on the file backend stops `serve` with
   `SEI_KEYRING_PASSPHRASE required when SEI_KEYRING_BACKEND=file`.
3. Opens the keyring through the Cosmos SDK.
4. Runs a structural liveness check (`kr.List()`) with a bounded retry of 3
   attempts, 2 seconds between attempts. The retry absorbs the rare kubelet
   Secret-mount race where the projected file is briefly absent.

The passphrase is removed from the process environment before validation or
keyring opening, including when no backend is configured. See
[`buildExecutionConfig`](../../serve.go) and
[`OpenKeyring`](../server/keyring.go).

An empty keyring is a permitted outcome of the smoke test — the sidecar
trusts that callers will supply key names that exist when they submit
sign-tx tasks, and surfaces missing-key errors at that point.

## Trust model

- Startup reads the passphrase with `os.Getenv` and immediately calls
  `os.Unsetenv` before opening the keyring.
- The passphrase is never logged. The sidecar logs `keyring opened` with the
  backend and directory only.
- Errors returned from the keyring-open path are scrubbed of any verbatim
  occurrence of the passphrase before they leave the function.
- Operators must rotate the passphrase by re-projecting the Secret and
  restarting the sidecar pod (see the operator runbook below).

## Genesis isolation

`sidecar/tasks/generate_gentx.go` continues to use `keyring.BackendTest`
unconditionally. The gentx validator key is throwaway by design and must not
share state with the operator's production keyring. The two code paths share
no keyring object and live in different packages.

## Operator runbook

Populate the configured keyring directory from the deployment's secret storage
and set `SEI_KEYRING_BACKEND` and `SEI_KEYRING_DIR` according to the environment
contract above. For the `file` backend, supply `SEI_KEYRING_PASSPHRASE` from a
Secret. Restart the sidecar after rotating the keyring or passphrase.
