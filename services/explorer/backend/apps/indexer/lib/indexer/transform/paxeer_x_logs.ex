defmodule Indexer.Transform.PaxeerXLogs do
  @moduledoc """
  Decodes Paxeer X Network kernel precompile logs into the `lx_*` row maps the
  `Explorer.Chain.PaxeerX` schemas and their import runners accept.

  The kernel modules have no query gRPC service, so the EVM log emitted by the
  precompile is the only pull based source for every LayerX entity. This
  transform runs inside the existing logs pipeline of `Indexer.Block.Fetcher`
  and costs one topic lookup per log; it issues no RPC of its own.

  `parse/1` returns a map with one list per target table:

    * `:lx_account_bindings` - `addr` precompile, `0x…1004`
    * `:lx_market_events` - `layerxexchange` `0x…1015`, `layerxbridge` `0x…1016`
      and `launchpad` `0x…1017`

  Every row is a parameter map for the changeset of its schema, so the keys are
  the column names of the `lx_*` tables and nothing else. All rows carry

    * `transaction_hash`, `block_hash`, `block_number`, `log_index` - the key
    * `event_name` - the Solidity event name, e.g. `"LayerXBound"`
    * `parameters` - every decoded argument under its ABI name, lossless:
      `address` and `bytes32` as lowercase `0x` strings, `uintN` as integers,
      `bool` as booleans, `string` as is

  The emitting precompile is not repeated on the row: it is fixed by
  `event_name` and already recorded on the `logs` row with the same
  `transaction_hash` and `log_index`. On `lx_market_events` `address_hash` is
  therefore the EVM party of the event -
  the payer, recipient, owner, trader, holder or creator - which is the
  `address` field of an activity row in the unified account document.

  On top of the shared columns each table gets the enum the schema declares,
  decided by the event rather than by the payload, and the arguments that are
  queried:

    * `lx_account_bindings` - `bound`, `evm_address_hash`, `layerx_did`
      (`did:layerx:` plus the 32 byte DID public key), `nonce`
    * `lx_market_events` - `domain`, `kind`, `address_hash`, `account`,
      `asset_id`, `asset_address_hash`, `amount`

  Event signatures come from `precompiles/<name>/abi.json`. The `topic0`
  constants below are proved against keccak256 of those signatures in
  `Indexer.Transform.PaxeerXLogsTest`.
  """

  require Logger

  import Explorer.Helper, only: [decode_data: 2]

  alias Explorer.Chain.Hash

  @addr_precompile "0x0000000000000000000000000000000000001004"
  @exchange_precompile "0x0000000000000000000000000000000000001015"
  @bridge_precompile "0x0000000000000000000000000000000000001016"
  @launchpad_precompile "0x0000000000000000000000000000000000001017"

  @did_prefix "did:layerx:"

  @precompile_names %{
    @addr_precompile => "addr",
    @exchange_precompile => "layerxexchange",
    @bridge_precompile => "layerxbridge",
    @launchpad_precompile => "launchpad"
  }

  @empty_rows %{
    lx_account_bindings: [],
    lx_market_events: []
  }

  @party_argument_names ~w(owner trader recipient holder creator payer sender party to)

  @definitions [
    %{
      table: :lx_account_bindings,
      address: @addr_precompile,
      name: "LayerXBound",
      signature: "LayerXBound(address,bytes32,uint64)",
      topic: "0x79b5f3e386759c93c56bf67a868c5f49afe6afaf286aa99bbc8e800ca4b7c856",
      bound: true,
      arguments: [
        {"evm", :address, true},
        {"didPublicKey", {:bytes, 32}, true},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_account_bindings,
      address: @addr_precompile,
      name: "LayerXUnbound",
      signature: "LayerXUnbound(address,bytes32,uint64)",
      topic: "0xce3f8d62ef8d76530f28a650078b3a45e48fbc97596eeb704b1bb00d3946847d",
      bound: false,
      arguments: [
        {"evm", :address, true},
        {"didPublicKey", {:bytes, 32}, true},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @exchange_precompile,
      name: "MarginDeposited",
      signature: "MarginDeposited(bytes32,bytes32,address,bytes32,uint256,bytes32,uint64)",
      topic: "0x456ba29aa60d5cac1a6dc1c0f3df30b1f18963fd90dafe8c5f4a6de80440118a",
      domain: :exchange,
      kind: "margin_deposited",
      arguments: [
        {"intentId", {:bytes, 32}, true},
        {"account", {:bytes, 32}, true},
        {"owner", :address, true},
        {"assetId", {:bytes, 32}, false},
        {"amount", {:uint, 256}, false},
        {"depositId", {:bytes, 32}, false},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @exchange_precompile,
      name: "MarginWithdrawalRequested",
      signature: "MarginWithdrawalRequested(bytes32,bytes32,address,bytes32,uint256,uint64)",
      topic: "0x9cf8600ce0e07d0d2b0b82ed99fc940eb6121143201d8d682c25fc74972c88f0",
      domain: :exchange,
      kind: "margin_withdrawal_requested",
      arguments: [
        {"intentId", {:bytes, 32}, true},
        {"account", {:bytes, 32}, true},
        {"owner", :address, true},
        {"assetId", {:bytes, 32}, false},
        {"amount", {:uint, 256}, false},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @exchange_precompile,
      name: "OrderPlaced",
      signature: "OrderPlaced(bytes32,bytes32,address,uint8,uint256,uint256,uint8,uint64)",
      topic: "0x88b93538701d726739ade066c2a9f09e5088f608d9b2ebba0cf305f5ee0752ce",
      domain: :exchange,
      kind: "order_placed",
      arguments: [
        {"intentId", {:bytes, 32}, true},
        {"marketId", {:bytes, 32}, true},
        {"owner", :address, true},
        {"side", {:uint, 8}, false},
        {"price", {:uint, 256}, false},
        {"quantity", {:uint, 256}, false},
        {"timeInForce", {:uint, 8}, false},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @exchange_precompile,
      name: "OrderCancelRequested",
      signature: "OrderCancelRequested(bytes32,bytes32,address,uint64)",
      topic: "0x39148489da3c16ee8c589a95e2f0c869816ee4f816b4ba1b85b32bc6d94c0241",
      domain: :exchange,
      kind: "order_cancel_requested",
      arguments: [
        {"intentId", {:bytes, 32}, true},
        {"orderId", {:bytes, 32}, true},
        {"owner", :address, true},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @exchange_precompile,
      name: "SettlementRequested",
      signature: "SettlementRequested(bytes32,bytes32,address,uint64)",
      topic: "0x70d5b9c37994017669de2a991c65a88ebeb34999f37c6dc6d0c44462d57eb655",
      domain: :exchange,
      kind: "settlement_requested",
      arguments: [
        {"intentId", {:bytes, 32}, true},
        {"positionId", {:bytes, 32}, true},
        {"owner", :address, true},
        {"nonce", {:uint, 64}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @bridge_precompile,
      name: "BridgeIn",
      signature: "BridgeIn(uint64,bytes32,address,uint64,address,uint256,string)",
      topic: "0x4352fb2e09bdaa35c4d407ce85dfa93eaec318876eddd6f5a490cce830c3f274",
      domain: :bridge,
      kind: "bridge_in",
      arguments: [
        {"chain", {:uint, 64}, true},
        {"txHash", {:bytes, 32}, true},
        {"recipient", :address, true},
        {"logIndex", {:uint, 64}, false},
        {"asset", :address, false},
        {"amount", {:uint, 256}, false},
        {"denom", :string, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @bridge_precompile,
      name: "BridgeOut",
      signature: "BridgeOut(uint64,address,uint256,address,uint64)",
      topic: "0x3e990eb54009dcdca53d8fa87307210f07097f37dcf6185dee71a42f8e7d524e",
      domain: :bridge,
      kind: "bridge_out",
      arguments: [
        {"chain", {:uint, 64}, true},
        {"asset", :address, true},
        {"amount", {:uint, 256}, false},
        {"recipient", :address, false},
        {"nonce", {:uint, 64}, true}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "MarketCreated",
      signature: "MarketCreated(address,address,string,string,string,uint8)",
      topic: "0xd8ad483b7300b5831650c4747b4d85390539f25f7d7d8c635eb3f8147daf198e",
      domain: :launchpad,
      kind: "market_created",
      arguments: [
        {"token", :address, true},
        {"creator", :address, true},
        {"denom", :string, false},
        {"name", :string, false},
        {"symbol", :string, false},
        {"feeStrategy", {:uint, 8}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "Swap",
      signature: "Swap(address,address,address,bool,uint256,uint256,uint256,uint256)",
      topic: "0xf3369c7e0aa652773c7246b5481ca4b1ee0b408d90467d2ce93b165b9938fde5",
      domain: :launchpad,
      kind: "swap",
      arguments: [
        {"token", :address, true},
        {"trader", :address, true},
        {"recipient", :address, true},
        {"isBuy", :bool, false},
        {"amountIn", {:uint, 256}, false},
        {"amountOut", {:uint, 256}, false},
        {"feeAmount", {:uint, 256}, false},
        {"price", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeeRecorded",
      signature: "FeeRecorded(address,uint256,uint256,uint256)",
      topic: "0xb4d4d3bd2f97a7d6f1657ee69f7191d7aa7dbd5b6864a2d7a9d14efc1322552f",
      domain: :launchpad,
      kind: "fee_recorded",
      arguments: [
        {"token", :address, true},
        {"feeAmount", {:uint, 256}, false},
        {"protocolCut", {:uint, 256}, false},
        {"poolCut", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeesClaimed",
      signature: "FeesClaimed(address,address,uint256)",
      topic: "0xfe3464cd748424446c37877c28ce5b700222c5bc9f90d908afcc4e5cb22707ff",
      domain: :launchpad,
      kind: "fees_claimed",
      arguments: [
        {"token", :address, true},
        {"recipient", :address, true},
        {"amount", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeesBurned",
      signature: "FeesBurned(address,uint256)",
      topic: "0x0d9575a73e2a7da16cfde907df749d23d901528ff2e7c832b731babdecca000b",
      domain: :launchpad,
      kind: "fees_burned",
      arguments: [
        {"token", :address, true},
        {"amount", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeeStrategyChanged",
      signature: "FeeStrategyChanged(address,uint8,uint8)",
      topic: "0x66c2a2c42cf36fad89e5da817a0b5de0fd78d7481cbfdc59f604148252da2261",
      domain: :launchpad,
      kind: "fee_strategy_changed",
      arguments: [
        {"token", :address, true},
        {"oldStrategy", {:uint, 8}, false},
        {"newStrategy", {:uint, 8}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "AirdropExecuted",
      signature: "AirdropExecuted(address,uint256,uint256)",
      topic: "0x171b2f9dc7a4c7eaa8ca718bcac62fbec15d147f033f38e42971b7ccabe9a469",
      domain: :launchpad,
      kind: "airdrop_executed",
      arguments: [
        {"token", :address, true},
        {"amount", {:uint, 256}, false},
        {"epoch", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "AirdropClaimed",
      signature: "AirdropClaimed(address,address,uint256,uint256)",
      topic: "0xd399c6e7fad358fc300beda3f056717c94a04c7233ce92683de6500ba509022e",
      domain: :launchpad,
      kind: "airdrop_claimed",
      arguments: [
        {"token", :address, true},
        {"holder", :address, true},
        {"amount", {:uint, 256}, false},
        {"epoch", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "LpRewardsExecuted",
      signature: "LpRewardsExecuted(address,uint256)",
      topic: "0xa9e7850d400945e0434ddd18a194aff01f31efaae577a63486c3dc865c5ab759",
      domain: :launchpad,
      kind: "lp_rewards_executed",
      arguments: [
        {"token", :address, true},
        {"amount", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "PauseToggled",
      signature: "PauseToggled(address,bool)",
      topic: "0x79a5bc58b021076f821571d0fe8b0ae3d9e0a666563bb064fdbf0bf69281331c",
      domain: :launchpad,
      kind: "pause_toggled",
      arguments: [
        {"token", :address, true},
        {"paused", :bool, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "ConfigUpdated",
      signature: "ConfigUpdated(bytes32,uint256,uint256,uint256,uint256)",
      topic: "0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e",
      domain: :launchpad,
      kind: "config_updated",
      arguments: [
        {"key", {:bytes, 32}, true},
        {"oldValue", {:uint, 256}, false},
        {"newValue", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeeDistributed",
      signature: "FeeDistributed(bytes32,uint256,uint8,uint256,address,uint256,uint256)",
      topic: "0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc",
      domain: :launchpad,
      kind: "fee_distributed",
      arguments: [
        {"poolId", {:bytes, 32}, true},
        {"nftId", {:uint, 256}, false},
        {"strategy", {:uint, 8}, false},
        {"amount", {:uint, 256}, false},
        {"recipient", :address, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeeFlow",
      signature: "FeeFlow(uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)",
      topic: "0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4",
      domain: :launchpad,
      kind: "fee_flow",
      arguments: [
        {"kind", {:uint, 8}, true},
        {"pool", :address, true},
        {"party", :address, true},
        {"amount", {:uint, 256}, false},
        {"protocolCut", {:uint, 256}, false},
        {"poolCut", {:uint, 256}, false},
        {"epoch", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "FeeStrategyChanged",
      signature: "FeeStrategyChanged(bytes32,uint256,uint8,uint8,uint256,uint256)",
      topic: "0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd",
      domain: :launchpad,
      kind: "fee_strategy_changed",
      arguments: [
        {"poolId", {:bytes, 32}, true},
        {"nftId", {:uint, 256}, false},
        {"oldStrategy", {:uint, 8}, false},
        {"newStrategy", {:uint, 8}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "MarketCreated",
      signature: "MarketCreated(bytes32,address,address,address,address,uint256,uint256)",
      topic: "0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b",
      domain: :launchpad,
      kind: "market_created",
      arguments: [
        {"poolId", {:bytes, 32}, true},
        {"token", :address, true},
        {"creator", :address, true},
        {"pool", :address, false},
        {"optical", :address, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "NftMint",
      signature: "NftMint(uint256,address,address,uint8,uint256,uint256)",
      topic: "0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5",
      domain: :launchpad,
      kind: "nft_mint",
      arguments: [
        {"tokenId", {:uint, 256}, true},
        {"creator", :address, true},
        {"pool", :address, true},
        {"strategy", {:uint, 8}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "NftTransfer",
      signature: "NftTransfer(address,address,address,uint256,uint256,uint256)",
      topic: "0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3",
      domain: :launchpad,
      kind: "nft_transfer",
      arguments: [
        {"nft", :address, true},
        {"from", :address, true},
        {"to", :address, true},
        {"tokenId", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "PoolRegistered",
      signature: "PoolRegistered(address,address,address,address,uint256,uint256,uint256)",
      topic: "0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7",
      domain: :launchpad,
      kind: "pool_registered",
      arguments: [
        {"pool", :address, true},
        {"token", :address, true},
        {"creator", :address, true},
        {"optical", :address, false},
        {"nftId", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "PoolStateUpdated",
      signature: "PoolStateUpdated(bytes32,uint256,uint256,uint256,uint256,uint256,uint256)",
      topic: "0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd",
      domain: :launchpad,
      kind: "pool_state_updated",
      arguments: [
        {"poolId", {:bytes, 32}, true},
        {"virtualReserve", {:uint, 256}, false},
        {"realReserve", {:uint, 256}, false},
        {"tokenReserve", {:uint, 256}, false},
        {"price", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "RouterTrade",
      signature: "RouterTrade(uint8,address,address,address,address,uint256,uint256,uint256,uint256,uint256)",
      topic: "0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650",
      domain: :launchpad,
      kind: "router_trade",
      arguments: [
        {"kind", {:uint, 8}, true},
        {"pool", :address, true},
        {"sender", :address, true},
        {"tokenIn", :address, false},
        {"tokenOut", :address, false},
        {"amountIn", {:uint, 256}, false},
        {"amountOut", {:uint, 256}, false},
        {"intermediateUsdl", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "Swap",
      signature: "Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)",
      topic: "0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc",
      domain: :launchpad,
      kind: "swap",
      arguments: [
        {"poolId", {:bytes, 32}, true},
        {"sender", :address, true},
        {"isBuy", :bool, false},
        {"amountIn", {:uint, 256}, false},
        {"amountOut", {:uint, 256}, false},
        {"fee", {:uint, 256}, false},
        {"price", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    },
    %{
      table: :lx_market_events,
      address: @launchpad_precompile,
      name: "TokenDeployed",
      signature: "TokenDeployed(address,address,address,bytes32,string,string,uint8,uint256,uint256,uint256)",
      topic: "0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237",
      domain: :launchpad,
      kind: "token_deployed",
      arguments: [
        {"token", :address, true},
        {"pool", :address, true},
        {"creator", :address, true},
        {"salt", {:bytes, 32}, false},
        {"name", :string, false},
        {"symbol", :string, false},
        {"decimals", {:uint, 8}, false},
        {"totalSupply", {:uint, 256}, false},
        {"timestamp", {:uint, 256}, false},
        {"blockNumber", {:uint, 256}, false}
      ]
    }
  ]

  @definitions_by_key Map.new(@definitions, &{{&1.address, &1.topic}, &1})

  @doc """
  Every decoded event: its target table, emitting precompile, Solidity
  signature, `topic0` constant, the enum values its rows carry and its argument
  list.
  """
  @spec definitions() :: [map()]
  def definitions, do: @definitions

  @doc """
  Decodes the Paxeer X precompile logs of `logs` into one list of row maps per
  `lx_*` table. Logs from a supported precompile whose topic or arguments cannot
  be decoded are logged. Logs from any other emitter contribute nothing.
  """
  @spec parse([map()]) :: %{
          lx_account_bindings: [map()],
          lx_market_events: [map()]
        }
  def parse(logs) when is_list(logs) do
    {rows, _undecoded_log_count} = Enum.reduce(logs, {@empty_rows, 0}, &parse_log/2)

    Map.new(rows, fn {table, table_rows} -> {table, Enum.reverse(table_rows)} end)
  end

  defp parse_log(log, {rows, undecoded_log_count} = acc) do
    case definition(log) do
      {:ok, definition} ->
        case decode_arguments(log, definition) do
          {:ok, parameters} ->
            {Map.update!(rows, definition.table, &[row(definition, log, parameters) | &1]), undecoded_log_count}

          :error ->
            {rows, undecoded_log_count + 1}
        end

      :error ->
        count_undecoded(log, acc)
    end
  end

  defp count_undecoded(log, {rows, undecoded_log_count} = acc) do
    case Map.fetch(@precompile_names, hex_string(log[:address_hash])) do
      {:ok, precompile} ->
        Logger.error(
          "No definition for topic0 #{inspect(hex_string(log[:first_topic]))} of the #{precompile} precompile, " <>
            "log #{log_key(log)}. Counting it as undecoded."
        )

        {rows, undecoded_log_count + 1}

      :error ->
        acc
    end
  end

  defp definition(log) do
    key = {hex_string(log[:address_hash]), hex_string(log[:first_topic])}

    case Map.fetch(@definitions_by_key, key) do
      {:ok, definition} -> {:ok, definition}
      :error -> :error
    end
  end

  defp decode_arguments(log, definition) do
    {indexed, unindexed} = Enum.split_with(definition.arguments, fn {_name, _type, indexed?} -> indexed? end)

    with {:ok, indexed_values} <- decode_indexed(log, indexed, definition),
         {:ok, unindexed_values} <- decode_unindexed(log, unindexed, definition) do
      {:ok, Map.new(indexed_values ++ unindexed_values)}
    end
  end

  defp decode_indexed(log, indexed, definition) do
    topics =
      [log[:second_topic], log[:third_topic], log[:fourth_topic]]
      |> Enum.take(length(indexed))
      |> Enum.map(&hex_string/1)

    if Enum.any?(topics, &is_nil/1) do
      Logger.error("#{definition.name} log #{log_key(log)} is missing an indexed topic. Skipping it.")
      :error
    else
      {:ok,
       indexed
       |> Enum.zip(topics)
       |> Enum.map(fn {{name, type, _indexed?}, topic} -> {name, from_topic(type, topic)} end)}
    end
  end

  defp decode_unindexed(_log, [], _definition), do: {:ok, []}

  defp decode_unindexed(log, unindexed, definition) do
    values = decode_data(log[:data], Enum.map(unindexed, fn {_name, type, _indexed?} -> type end))

    if Enum.any?(values, &is_nil/1) do
      Logger.error("#{definition.name} log #{log_key(log)} carries no data for its unindexed arguments. Skipping it.")
      :error
    else
      {:ok,
       unindexed
       |> Enum.zip(values)
       |> Enum.map(fn {{name, type, _indexed?}, value} -> {name, from_data(type, value)} end)}
    end
  rescue
    error ->
      Logger.error("#{definition.name} log #{log_key(log)} failed to decode: #{Exception.message(error)}. Skipping it.")
      :error
  end

  defp row(%{table: :lx_account_bindings} = definition, log, parameters) do
    definition
    |> base_row(log, parameters)
    |> Map.merge(%{
      bound: definition.bound,
      evm_address_hash: parameters["evm"],
      layerx_did: layerx_did(parameters["didPublicKey"]),
      nonce: parameters["nonce"]
    })
  end

  defp row(%{table: :lx_market_events} = definition, log, parameters) do
    definition
    |> base_row(log, parameters)
    |> Map.merge(%{
      domain: definition.domain,
      kind: definition.kind,
      address_hash: party_address_hash(parameters),
      account: parameters["account"],
      asset_id: parameters["assetId"],
      asset_address_hash: parameters["asset"] || parameters["token"],
      amount: parameters["amount"]
    })
  end

  defp base_row(definition, log, parameters) do
    %{
      transaction_hash: log[:transaction_hash],
      block_hash: log[:block_hash],
      block_number: log[:block_number],
      log_index: log[:index],
      event_name: definition.name,
      parameters: parameters
    }
  end

  defp party_address_hash(parameters) do
    Enum.find_value(@party_argument_names, fn name -> parameters[name] end)
  end

  defp layerx_did("0x" <> public_key), do: @did_prefix <> public_key

  defp from_topic(:address, "0x" <> value), do: "0x" <> binary_part(value, 24, 40)
  defp from_topic({:bytes, 32}, value), do: value
  defp from_topic({:uint, _bits}, "0x" <> value), do: String.to_integer(value, 16)
  defp from_topic(:bool, "0x" <> value), do: String.to_integer(value, 16) != 0

  defp from_data(:address, value), do: "0x" <> Base.encode16(value, case: :lower)
  defp from_data({:bytes, 32}, value), do: "0x" <> Base.encode16(value, case: :lower)
  defp from_data({:uint, _bits}, value), do: value
  defp from_data(:bool, value), do: value
  defp from_data(:string, value), do: value

  defp log_key(log), do: "#{inspect(log[:transaction_hash])}:#{inspect(log[:index])}"

  defp hex_string(nil), do: nil
  defp hex_string(%Hash{} = hash), do: hash |> Hash.to_string() |> String.downcase()
  defp hex_string(value) when is_binary(value), do: String.downcase(value)
end
