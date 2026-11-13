---
order: false
---

# simapp

simapp is the SDK application used by the [IBC testing package](../README.md).
It contains transfer, interchain accounts and the [mock IBC application](../mock/README.md)
for tests. `simd` runs this application as a local test network; Paxeer's network
runs `paxd`, with the kernel participating in block proposal and block commit
providing finality.

## Running a local test network with `simd`

The commands below create a fresh local test network. All participants must use
the same chain ID and final genesis file. The example chain ID `ibc-local-1` is
for this test network; it is unrelated to Paxeer's public EVM chain ID 125.

1. From the repository root, build the test binary and enter its directory:

   ```sh
   go build -o build/simd ./interchain/testing/simapp/simd
   cd build
   ```

2. Choose a fresh test home on each participant's machine. The default is
   `~/.simapp` (`DefaultNodeHome` in [app.go](app.go)); this example uses a
   separate directory and passes `--home` explicitly throughout:

   ```sh
   simapp_home="$HOME/.simapp-ibc-test"
   ./simd init validator-a --chain-id ibc-local-1 --home "$simapp_home"
   ```

   Each participant chooses its own moniker. Retain the same `simapp_home`
   value for subsequent commands on that machine.

3. Each participant creates a local key and supplies its account address to the
   person preparing the shared genesis:

   ```sh
   ./simd keys add validator --home "$simapp_home"
   ./simd keys show validator -a --home "$simapp_home"
   ```

4. On the machine preparing genesis, fund **every participant's account** in
   `config/genesis.json`. For a single validator, use:

   ```sh
   ./simd add-genesis-account \
     "$(./simd keys show validator -a --home "$simapp_home")" \
     1000000000000uhpx --home "$simapp_home"
   ```

   For multiple validators, repeat `add-genesis-account` with each supplied
   address. The staking denomination is `uhpx`, from `sdk.DefaultBondDenom`,
   which the default staking genesis uses. Fund and delegate that denomination.
   If reusing a customized genesis, check `app_state.staking.params.bond_denom`
   in `config/genesis.json` before funding accounts or generating gentxs; both
   amounts must use that configured denomination.

5. Distribute this funded genesis file to every participant, replacing each
   participant's `config/genesis.json` under its test home. Each participant
   then creates its own validator genesis transaction:

   ```sh
   ./simd gentx validator 100000000uhpx \
     --chain-id ibc-local-1 --home "$simapp_home"
   ```

   The delegated amount must be covered by that account's genesis balance.

6. One participant gathers all generated transaction JSON files into its test
   home's `config/gentx` directory, then collects and validates them:

   ```sh
   ./simd collect-gentxs --home "$simapp_home"
   ./simd validate-genesis --home "$simapp_home"
   ```

7. Distribute the resulting `config/genesis.json` to every participant. Each
   participant replaces its own genesis file with this identical final file.
   For a single validator, the file is already in place.

8. Obtain each node's peer ID:

   ```sh
   ./simd tendermint show-node-id --home "$simapp_home"
   ```

9. For multiple nodes, set `persistent-peers` under `[p2p]` in each test home's
   `config/config.toml` to the other nodes' IDs and reachable addresses. For
   example:

   ```toml
   [p2p]
   persistent-peers = "<node-id-b>@<host-b>:26656,<node-id-c>@<host-c>:26656"
   ```

   `26656` is the default P2P port. The default `laddr` listens only on
   loopback; for nodes on separate machines, set it to a reachable local
   interface, using `tcp://<local-interface>:26656`. These keys are defined in
   the [P2P configuration template](../../../consensus/config/toml.go).
   Use the peer's configured port if changed; nodes sharing a machine also
   need distinct listening ports and homes.

10. Start each node:

    ```sh
    ./simd start --home "$simapp_home"
    ```

    The test CLI's [configuration defaults](simd/cmd/root.go) set
    `minimum-gas-prices` to `0uhpx` in `config/app.toml`.

The resulting local network is for trying changes to IBC modules and the SDK
under [`sdk/`](../../../sdk/). For in-process tests, use the coordinator and
application setup described in the [testing guide](../README.md).

For an intentional reset of disposable test state, stop the node and use
`./simd tendermint unsafe-reset-all --home "$simapp_home"`. This command removes
consensus data, signing state and the test home's WASM directory. A new test home
avoids reusing that state when starting an unrelated network.
