# Governance

This page describes CosmWasm governance in Paxeer's `paxd` application. The
Wasm module executes within the same network as the EVM and the kernel. Mainnet-beta
uses EVM chain ID 125 (`0x7d`); the `paxd` CLI's `--chain-id` is the Cosmos
chain identifier from the node configuration, not that EVM number.

Governance can perform privileged operations throughout a contract's lifecycle.
See [IBC specification](IBC.md) for the port and callback constraints that also
apply when governance instantiates or migrates an IBC contract.

## Proposal Types

[Proposal definitions and validation](types/proposal.go) declare ten Wasm
proposal kinds. The kind strings below are also the values accepted by
`ConvertToProposals` and the specific-proposal build override.

| Kind | Content type | Effect when executed |
| --- | --- | --- |
| `StoreCode` | `StoreCodeProposal` | Upload Wasm with `run_as` as creator, then pin the resulting code. |
| `InstantiateContract` | `InstantiateContractProposal` | Instantiate code with the supplied message, label, optional admin and funds; `run_as` is the creator and contract sender. |
| `MigrateContract` | `MigrateContractProposal` | Move a contract to another code ID and invoke its migration entry point. |
| `SudoContract` | `SudoContractProposal` | Invoke the protected `sudo` entry point with the proposal message. |
| `ExecuteContract` | `ExecuteContractProposal` | Invoke `execute` with `run_as` as sender and the supplied funds. |
| `UpdateAdmin` | `UpdateAdminProposal` | Set the contract admin. |
| `ClearAdmin` | `ClearAdminProposal` | Remove the admin used for ordinary migrations and admin changes. |
| `PinCodes` | `PinCodesProposal` | Pin the listed code IDs in the VM cache, trading memory for lower loading costs. |
| `UnpinCodes` | `UnpinCodesProposal` | Remove the listed code IDs from the pinned cache. |
| `UpdateInstantiateConfig` | `UpdateInstantiateConfigProposal` | Replace instantiation permissions for the listed code IDs. |

### Unit tests

[Proposal validation tests](types/proposal_test.go) cover the content types.
[Governance integration tests](keeper/proposal_integration_test.go) exercise
proposal submission and the corresponding execution handlers.

## Proposal Handler

[NewWasmProposalHandler](keeper/proposal_handler.go) constructs a handler using
[NewGovPermissionKeeper](keeper/contract_keeper.go). Governance invokes this
handler for an accepted proposal; submitting a proposal does not execute its
contract operation immediately. The handler checks the enabled kind and validates
its content before calling the keeper. Unsupported kinds return
`sdkerrors.ErrUnknownRequest`.

[GovAuthorizationPolicy](keeper/authz_policy.go) permits code creation,
instantiation and contract modification regardless of the normal upload,
per-code instantiation and admin checks. Clearing an admin therefore prevents
ordinary admin-authorized migrations, but does not remove governance's ability
to migrate the contract or assign an admin through an enabled proposal.

Other keeper constraints still apply. In particular, an explicitly supplied
instantiation permission on a code upload must be a subset of the current
default instantiation configuration, including on the governance path.

`SudoContractProposal` gives governance access to the keeper's privileged `Sudo`
operation. An ordinary `MsgExecuteContract`, including one signed by the contract
admin, calls `execute` rather than `sudo`. `ExecuteContractProposal` still runs
the contract's execute logic with the specified sender; the policy does not
silently replace contract-level checks or make invalid messages succeed. Funds
specified for instantiation or execution are transferred from `run_as`, subject
to the keeper's normal bank operations.

## Gov Integration

[node/app.go](../../../node/app.go) registers the Wasm governance handler when
its `enabledProposals` argument is non-empty:

```go
govRouter.AddRoute(wasm.RouterKey, wasm.NewWasmProposalHandler(app.WasmKeeper, enabledProposals))
```

This is in-process SDK governance dispatch inside `paxd`. The application also
registers parameter-change proposals independently of the Wasm proposal set.

### Proposal selection and build variables

The node package defines `WasmProposalsEnabled = "true"` and
`EnableSpecificWasmProposals = ""`. `GetWasmEnabledProposals` interprets them as
follows:

- With an empty specific list, exactly `"true"` enables all ten kinds; any other
  value selects none.
- A non-empty `EnableSpecificWasmProposals` overrides `WasmProposalsEnabled`.
  It is a comma-separated list of the exact kind strings in the table. Whitespace
  is not trimmed; invalid names cause the helper to panic.

These are Go string build variables, not environment variables or `app.toml`
keys. Linker overrides use the local node package, for example:

```text
-X github.com/Sidiora-Labs/Paxeer-X-Network/node.WasmProposalsEnabled=false
-X github.com/Sidiora-Labs/Paxeer-X-Network/node.EnableSpecificWasmProposals=MigrateContract,UpdateAdmin,ClearAdmin
```

The caller determines whether those settings apply. In
[daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go), normal application
creation passes `wasm.EnableAllProposals` directly, enabling all ten kinds.
The export paths there, and [snapshot creation](../../../daemon/paxd/cmd/snapshot.go),
call `GetWasmEnabledProposals`. A linker override alone does not restrict Wasm
proposals on the normal startup path in this tree.

## Wasm Authorization Settings

[params.go](types/params.go) defines two parameters in the `wasm` subspace:

| Genesis field | Parameter-change key | Meaning |
| --- | --- | --- |
| `code_upload_access` | `uploadAccess` | Who can upload code through the ordinary message path. An access object with `permission` and, for `OnlyAddress`, `address`. |
| `instantiate_default_permission` | `instantiateAccess` | Default access type used when an upload omits its instantiation permission. `OnlyAddress` resolves to the code creator. |

The supported permission names are `Nobody`, `Everybody` and `OnlyAddress`.
Both parameters default to `Everybody`; deployed state may differ. Each stored
code has its own instantiation configuration. Changing the default does not
rewrite existing code configurations; `UpdateInstantiateConfigProposal` targets
those configurations explicitly.

### Init Params Via Genesis

The following is the `wasm.params` fragment, not a complete genesis file:

```json
{
  "code_upload_access": {
    "permission": "Everybody"
  },
  "instantiate_default_permission": "Everybody"
}
```

### Update Params Via ParamChangeProposal

Use the SDK [parameter-change command](../../../sdk/x/params/client/cli/tx.go).
For example, this proposal JSON disables ordinary code uploads:

```json
{
  "title": "Restrict Wasm uploads",
  "description": "Disable uploads through ordinary Wasm messages",
  "changes": [
    {
      "subspace": "wasm",
      "key": "uploadAccess",
      "value": {"permission": "Nobody"}
    }
  ],
  "deposit": ""
}
```

For unrestricted uploads, use `{"permission":"Everybody"}`. To restrict uploads
to one account, use `{"permission":"OnlyAddress","address":"<bech32-account>"}`,
replacing the placeholder with a valid account address. For the default
instantiation policy, change the key to `instantiateAccess` and use a string
value such as `"Nobody"` or `"Everybody"`, rather than an access object.
Governance's privileged operations remain available under restrictive parameters
when their proposal kinds are enabled.

### Tests

See [parameter validation](types/params_test.go),
[genesis validation](types/genesis_test.go) and
[keeper policy tests](keeper/keeper_test.go).

## CLI

The registered commands in [client/cli/gov_tx.go](client/cli/gov_tx.go) are
subcommands of `paxd tx gov submit-proposal`:

| Command and positional arguments | Proposal kind |
| --- | --- |
| `wasm-store <wasm-file>` | `StoreCode` |
| `instantiate-contract <code-id> <init-json>` | `InstantiateContract` |
| `migrate-contract <contract> <new-code-id> <migration-json>` | `MigrateContract` |
| `sudo-contract <contract> <sudo-json>` | `SudoContract` |
| `execute-contract <contract> <execute-json>` | `ExecuteContract` |
| `set-contract-admin <contract> <new-admin>` | `UpdateAdmin` |
| `clear-contract-admin <contract>` | `ClearAdmin` |
| `pin-codes <code-id>...` | `PinCodes` |
| `unpin-codes <code-id>...` | `UnpinCodes` |
| `update-instantiate-config <code-id,permission>...` | `UpdateInstantiateConfig` |

Wasm proposal commands accept `--title`, `--description` and `--deposit`.
`wasm-store`, `instantiate-contract` and `execute-contract` require `--run-as`.
Instantiation also requires `--label` and an explicit `--admin` or `--no-admin`;
instantiation and execution accept `--amount` for contract funds.
`update-instantiate-config` accepts lowercase `nobody`, `everybody` or a Bech32
account address as the permission after each comma.

For example, after setting `COSMOS_CHAIN_ID`, `PROPOSER_KEY` and `DEPOSIT` to
the target chain identifier, local signing key and initial deposit coin string:

```sh
paxd tx gov submit-proposal pin-codes 1 \
  --title "Pin contract code" --description "Keep code 1 in the pinned VM cache" \
  --deposit "$DEPOSIT" --from "$PROPOSER_KEY" --chain-id "$COSMOS_CHAIN_ID"
paxd tx gov submit-proposal param-change proposal.json \
  --from "$PROPOSER_KEY" --chain-id "$COSMOS_CHAIN_ID"
```

Code ID 1 must exist for the pin proposal to execute. In the second command,
`proposal.json` supplies the parameter change and deposit; set its deposit for
the target chain's governance requirements. Contract messages must match the
deployed contract's schema.

## Rest

[client/proposal_handler.go](client/proposal_handler.go) pairs all ten CLI
constructors with their legacy REST proposal handlers in
[client/rest/gov.go](client/rest/gov.go). The node includes this list in
`getGovProposalHandlers` when constructing the governance basic module. These
SDK proposal handlers are distinct from EVM JSON-RPC methods.

See [proposal-handler tests](client/proposal_handler_test.go) for their registration.

## Regression evidence

`TestRepairM_1289_Governance` in [repair_m1289_test.go](repair_m1289_test.go)
checks the node's default and specific-proposal selection, all ten registered CLI
commands and the proposal kind table above. It submits encoded proposal content
to the real governance keeper, executes the stored content through its registered
Wasm route, and verifies pinned code and privileged sudo effects on bank balances.
A rejected proposal submission does not authorize a contract operation.
