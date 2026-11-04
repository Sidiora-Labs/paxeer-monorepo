for file <- ~w(address_coin_balances_test.exs address_token_balances_test.exs addresses_test.exs blocks_test.exs
                paxeer_x_logs_abi_test.exs paxeer_x_logs_test.exs) do
  Code.require_file(Path.expand("../indexer/transform/#{file}", __DIR__))
end

defmodule Indexer.Repair2233561Test do
  use Explorer.DataCase, async: false

  alias ABI.TypeEncoder
  alias EthereumJSONRPC.{Blocks, Logs, Receipts}
  alias Explorer.Chain
  alias Explorer.Chain.{Address, Block}
  alias Explorer.Chain.Address.{CoinBalance, TokenBalance}
  alias Explorer.Chain.PaxeerX.{AccountBinding, MarketEvent}
  alias Indexer.Block.Fetcher
  alias Indexer.Transform.{AddressCoinBalances, Addresses, AddressTokenBalances, PaxeerXLogs, TokenTransfers}
  alias Indexer.Transform.Blocks, as: TransformBlocks

  @addr "0x0000000000000000000000000000000000001004"
  @exchange "0x0000000000000000000000000000000000001015"
  @bridge "0x0000000000000000000000000000000000001016"
  @launchpad "0x0000000000000000000000000000000000001017"
  @retired_keys [:lx_custody_events, :lx_deposit_roots, :lx_anchors, :lx_guarantor_events, :lx_receipts]
  @empty %{lx_account_bindings: [], lx_market_events: [], undecoded_log_count: 0}

  setup do
    transformer = Application.fetch_env!(:indexer, :block_transformer)
    chain_type = Application.fetch_env!(:explorer, :chain_type)
    Application.put_env(:indexer, :block_transformer, TransformBlocks.Base)
    Application.put_env(:explorer, :chain_type, :paxeer_x)

    on_exit(fn ->
      Application.put_env(:indexer, :block_transformer, transformer)
      Application.put_env(:explorer, :chain_type, chain_type)
    end)

    :ok
  end

  test "019: recorded block miners and heights survive the balance import runner" do
    raw_blocks =
      Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.sort_by(&EthereumJSONRPC.quantity_to_integer(&1["number"]))

    responses = Enum.with_index(raw_blocks, fn block, id -> %{id: id, result: block} end)
    requests = Map.new(responses, &{&1.id, %{number: &1.result["number"]}})
    fetched = Blocks.from_responses(responses, requests)
    assert fetched.errors == []
    blocks = TransformBlocks.transform_blocks(fetched.blocks_params)
    balances = AddressCoinBalances.params_set(%{blocks_params: blocks})

    assert balances ==
             MapSet.new([
               %{address_hash: "0xb3ad5a835b418958a9142d8681bfb4b81d2cf270", block_number: 24_203_005},
               %{address_hash: "0xb3ad5a835b418958a9142d8681bfb4b81d2cf270", block_number: 24_203_006}
             ])

    for params <- balances, do: assert(CoinBalance.changeset(%CoinBalance{}, params).valid?)

    assert {:ok, imported} =
             Chain.import(%{
               addresses: %{params: Addresses.extract_addresses(%{blocks: blocks})},
               blocks: %{params: Enum.map(blocks, &Map.put(&1, :consensus, true))},
               address_coin_balances: %{params: MapSet.to_list(balances)},
               broadcast: false
             })

    assert length(imported.blocks) == 2
    stored = Repo.all(CoinBalance)
    assert MapSet.new(stored, &%{address_hash: to_string(&1.address_hash), block_number: &1.block_number}) == balances
  end

  test "020: recorded ERC20 receipt reaches exactly the two holder balance schemas" do
    receipt = recorded_response("import_receipt_perform_1.json")

    raw = %{
      hd(receipt["logs"])
      | "address" => "0xf2eec76e45b328df99a34fa696320a262cb92154",
        "blockHash" => "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca",
        "blockNumber" => EthereumJSONRPC.integer_to_quantity(3_530_917),
        "transactionHash" => "0x43dfd761974e8c3351d285ab65bee311454eb45b149a015fe7804a33252f19e5",
        "logIndex" => "0x8",
        "topics" => [
          "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
          "0x000000000000000000000000556813d9cc20acfe8388af029a679d34a63388db",
          "0x00000000000000000000000092148dd870fa1b7c4700f2bd7f44238821c26f73"
        ],
        "data" => "0x000000000000000000000000000000000000000000000000ebec21ee1da40000"
    }

    logs = receipt_logs(%{receipt | "logs" => [raw]})
    assert %{tokens: [token], token_transfers: [transfer]} = TokenTransfers.parse(logs)
    assert token.type == "ERC-20"
    balances = AddressTokenBalances.params_set(%{token_transfers_params: [Map.put(transfer, :token, token)]})

    expected =
      MapSet.new([
        %{
          address_hash: "0x556813d9cc20acfe8388af029a679d34a63388db",
          block_number: 3_530_917,
          token_contract_address_hash: "0xf2eec76e45b328df99a34fa696320a262cb92154",
          token_type: "ERC-20",
          token_id: nil
        },
        %{
          address_hash: "0x92148dd870fa1b7c4700f2bd7f44238821c26f73",
          block_number: 3_530_917,
          token_contract_address_hash: "0xf2eec76e45b328df99a34fa696320a262cb92154",
          token_type: "ERC-20",
          token_id: nil
        }
      ])

    assert balances == expected

    for params <- balances do
      changeset = TokenBalance.changeset(%TokenBalance{}, params)
      assert changeset.valid?
      balance = Ecto.Changeset.apply_changes(changeset)
      assert to_string(balance.token_contract_address_hash) == token.contract_address_hash
      assert balance.token_type == token.type
      assert balance.block_number == 3_530_917
    end
  end

  test "021: the discovered address suite merges recorded transaction and miner identities into the address schema" do
    assert File.regular?(Path.expand("../indexer/transform/addresses_test.exs", __DIR__))
    refute File.exists?(Path.expand("../indexer/transform/addresses.exs", __DIR__))
    tests = Indexer.Transform.AddressesTest.__ex_unit__().tests
    assert Enum.any?(tests, &(&1.tags[:test_type] == :doctest))
    assert Enum.any?(tests, &String.contains?(Atom.to_string(&1.name), "addresses get merged"))

    raw = recorded_response("block_importer_import_1_pending.json")
    fetched = Blocks.from_responses([%{id: 0, result: raw}], %{0 => %{number: raw["number"]}})
    assert fetched.errors == []
    [block] = fetched.blocks_params
    [transaction | _] = fetched.transactions_params
    addresses = Addresses.extract_addresses(%{blocks: [block], transactions: [transaction, transaction]})
    assert length(addresses) == MapSet.size(MapSet.new(addresses, & &1.hash))
    assert Enum.any?(addresses, &(&1.hash == block.miner_hash and &1.fetched_coin_balance_block_number == block.number))
    sender = Enum.find(addresses, &(&1.hash == transaction.from_address_hash))
    assert sender.nonce == transaction.nonce
    assert sender.fetched_coin_balance_block_number == transaction.block_number
    for params <- addresses, do: assert(Address.changeset(%Address{}, params).valid?)
  end

  test "022: Base and Clique configuration reaches block and miner balance consumers" do
    block = recorded_clique_block()
    genesis = %{block | number: 0, hash: block.parent_hash}
    blocks = [genesis, block]
    assert TransformBlocks.transform_blocks(blocks) == blocks

    Application.put_env(:indexer, :block_transformer, TransformBlocks.Clique)
    transformed = TransformBlocks.transform_blocks(blocks)
    assert transformed == [genesis, %{block | miner_hash: "0xfc18cbc391de84dbd87db83b20935d3e89f5dd91"}]

    assert AddressCoinBalances.params_set(%{blocks_params: transformed}) ==
             MapSet.new([
               %{address_hash: genesis.miner_hash, block_number: 0},
               %{address_hash: "0xfc18cbc391de84dbd87db83b20935d3e89f5dd91", block_number: 2_848_394}
             ])

    for params <- transformed, do: assert(Block.changeset(%Block{}, Map.put(params, :consensus, true)).valid?)
  end

  test "023: every retired custody and anchor ABI event is refused through receipt conversion and import" do
    events =
      for {path, emitter} <- [
            {"layerxcustody/abi.json", "0x0000000000000000000000000000000000001013"},
            {"layerxanchor/abi.json", "0x0000000000000000000000000000000000001014"}
          ],
          event <- abi_events(path),
          do: {emitter, event, sample_values(event)}

    assert length(events) == 18
    result = import_events(events)
    assert result.parsed == @empty
    assert Repo.aggregate(AccountBinding, :count) == 0
    assert Repo.aggregate(MarketEvent, :count) == 0
    assert length(result.imported.logs) == 18
  end

  test "024: retained binding bridge and launchpad rows survive while retired result keys remain absent" do
    events = [
      event("addr/abi.json", @addr, "LayerXBound(address,bytes32,uint64)", [<<0x11::160>>, <<0x22::256>>, 7]),
      event("layerxbridge/abi.json", @bridge, "BridgeIn(uint64,bytes32,address,uint64,address,uint256,string)", [
        125,
        <<0x31::256>>,
        <<0x32::160>>,
        4,
        <<0x33::160>>,
        10_000,
        "ulxbtc"
      ]),
      event("launchpad/legacy/v610/abi.json", @launchpad, "FeesClaimed(address,address,uint256)", [
        <<0x51::160>>,
        <<0x52::160>>,
        123
      ])
    ]

    result = import_events(events)
    assert Enum.sort(Map.keys(result.parsed)) == [:lx_account_bindings, :lx_market_events, :undecoded_log_count]
    for key <- @retired_keys, do: refute(Map.has_key?(result.parsed, key))
    assert result.parsed.undecoded_log_count == 0
    assert [binding] = Repo.all(AccountBinding)
    assert binding.bound
    assert binding.nonce == 7
    assert to_string(binding.evm_address_hash) == hex(<<0x11::160>>)
    assert binding.layerx_did == "did:layerx:" <> Base.encode16(<<0x22::256>>, case: :lower)
    market = Repo.all(from(row in MarketEvent, order_by: row.log_index))
    assert Enum.map(market, &{&1.domain, &1.kind}) == [{:bridge, "bridge_in"}, {:launchpad, "fees_claimed"}]
    assert Enum.map(market, &Decimal.to_integer(&1.amount)) == [10_000, 123]
    assert hd(market).parameters["denom"] == "ulxbtc"
    assert to_string(List.last(market).address_hash) == hex(<<0x52::160>>)

    assert Fetcher.import_options(%{}, %{paxeer_x_logs: @empty}) == %{
             paxeer_x_account_bindings: %{params: []},
             paxeer_x_market_events: %{params: []}
           }

    assert PaxeerXLogs.parse([]) == @empty
  end

  test "025: modern and legacy OrderPlaced travel from their separate producer ABIs into the same market importer" do
    events = [
      event("perps/abi/abi.json", @exchange, "OrderPlaced(uint256,address,uint256,uint8,bool,uint256,uint256)", [
        91,
        <<0x23::160>>,
        22,
        1,
        true,
        42_000_000,
        3_000_000
      ]),
      event(
        "layerxexchange/abi.json",
        @exchange,
        "OrderPlaced(bytes32,bytes32,address,uint8,uint256,uint256,uint8,uint64)",
        [<<0x21::256>>, <<0x22::256>>, <<0x23::160>>, 1, 42_000_000, 3, 2, 11]
      )
    ]

    result = import_events(events)
    assert result.parsed.undecoded_log_count == 0
    [modern, legacy] = Repo.all(from(row in MarketEvent, order_by: row.log_index))
    assert modern.event_name == "OrderPlaced"
    assert legacy.event_name == "OrderPlaced"

    assert modern.parameters == %{
             "orderId" => 91,
             "user" => hex(<<0x23::160>>),
             "marketId" => 22,
             "orderType" => 1,
             "isLong" => true,
             "triggerPrice" => 42_000_000,
             "sizeUsd" => 3_000_000
           }

    assert legacy.parameters == %{
             "intentId" => hex(<<0x21::256>>),
             "marketId" => hex(<<0x22::256>>),
             "owner" => hex(<<0x23::160>>),
             "side" => 1,
             "price" => 42_000_000,
             "quantity" => 3,
             "timeInForce" => 2,
             "nonce" => 11
           }

    assert to_string(modern.address_hash) == to_string(legacy.address_hash)
    assert modern.log_index == 0
    assert legacy.log_index == 1
  end

  test "029: all required modern perps and legacy exchange event layouts reach the market schema and importer" do
    Indexer.Transform.PaxeerXLogsAbiTest.kept_topics_match_precompile_abis()
    modern_names = ~w(CollateralDeposited CollateralWithdrawn CollateralTransferred OrderPlaced OrderCancelled
                      OrderExecuted PositionOpened PositionClosed Liquidation TradeSettled)
    modern = abi_events("perps/abi/abi.json") |> Enum.filter(&(&1["name"] in modern_names))
    legacy = abi_events("layerxexchange/abi.json")
    assert Enum.sort(Enum.map(modern, & &1["name"])) == Enum.sort(modern_names)
    assert length(legacy) == 5
    events = Enum.map(modern ++ legacy, &{@exchange, &1, sample_values(&1)})
    result = import_events(events)
    assert result.parsed.undecoded_log_count == 0
    assert length(result.parsed.lx_market_events) == 15
    assert Repo.aggregate(MarketEvent, :count) == 15
    assert Enum.map(result.parsed.lx_market_events, & &1.event_name) == Enum.map(modern ++ legacy, & &1["name"])
  end

  defp import_events(events) do
    raw_block = recorded_response("block_importer_import_1_pending.json")
    transaction = hd(raw_block["transactions"])
    receipt = recorded_response("import_receipt_perform_1.json")
    template = hd(receipt["logs"])

    logs =
      Enum.with_index(events, fn {emitter, event, values}, index ->
        {indexed, unindexed} =
          Enum.zip(event["inputs"], values) |> Enum.split_with(fn {input, _} -> input["indexed"] end)

        topics = [
          hex(ExKeccak.hash_256(signature(event)))
          | Enum.map(indexed, fn {input, value} -> encode([value], [type(input["type"])]) end)
        ]

        Map.merge(template, %{
          "address" => emitter,
          "blockHash" => raw_block["hash"],
          "blockNumber" => raw_block["number"],
          "transactionHash" => transaction["hash"],
          "transactionIndex" => transaction["transactionIndex"],
          "logIndex" => EthereumJSONRPC.integer_to_quantity(index),
          "transactionLogIndex" => EthereumJSONRPC.integer_to_quantity(index),
          "topics" => topics,
          "data" => encode(Enum.map(unindexed, &elem(&1, 1)), Enum.map(unindexed, &type(elem(&1, 0)["type"])))
        })
      end)

    receipt =
      Map.merge(receipt, %{
        "blockHash" => raw_block["hash"],
        "blockNumber" => raw_block["number"],
        "transactionHash" => transaction["hash"],
        "transactionIndex" => transaction["transactionIndex"],
        "logs" => logs
      })

    fetched = Blocks.from_responses([%{id: 0, result: raw_block}], %{0 => %{number: raw_block["number"]}})
    assert fetched.errors == []
    blocks = TransformBlocks.transform_blocks(fetched.blocks_params)
    elixir_receipts = receipt |> Jason.encode!() |> Jason.decode!() |> List.wrap() |> Receipts.to_elixir()
    params = elixir_receipts |> Receipts.elixir_to_logs() |> Logs.elixir_to_params()
    parsed = PaxeerXLogs.parse(params)
    transactions = Fetcher.Receipts.put([hd(fetched.transactions_params)], Receipts.elixir_to_params(elixir_receipts))
    addresses = Addresses.extract_addresses(%{blocks: blocks, transactions: transactions, logs: params})

    options =
      Fetcher.import_options(
        %{
          addresses: %{params: addresses},
          blocks: %{params: Enum.map(blocks, &Map.put(&1, :consensus, true))},
          transactions: %{params: transactions},
          logs: %{params: params},
          broadcast: false
        },
        %{paxeer_x_logs: parsed}
      )

    assert options.paxeer_x_account_bindings == %{params: parsed.lx_account_bindings}
    assert options.paxeer_x_market_events == %{params: parsed.lx_market_events}

    for {schema, rows} <- [{AccountBinding, parsed.lx_account_bindings}, {MarketEvent, parsed.lx_market_events}],
        row <- rows do
      assert schema.changeset(struct(schema), row).valid?
    end

    assert {:ok, imported} = Chain.import(options)
    assert length(imported.logs) == length(events)
    %{parsed: parsed, imported: imported}
  end

  defp receipt_logs(receipt),
    do: receipt |> List.wrap() |> Receipts.to_elixir() |> Receipts.elixir_to_logs() |> Logs.elixir_to_params()

  defp recorded_response(name) do
    [record] =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
      |> File.read!()
      |> Jason.decode!()

    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp abi_events(path) do
    root = System.get_env("PAXEER_PRECOMPILES_DIR") || Path.expand("../../../../../../../precompiles", __DIR__)
    root |> Path.join(path) |> File.read!() |> Jason.decode!() |> Enum.filter(&(&1["type"] == "event"))
  end

  defp event(path, emitter, signature, values) do
    [event] = Enum.filter(abi_events(path), &(signature(&1) == signature))
    {emitter, event, values}
  end

  defp signature(event), do: event["name"] <> "(" <> Enum.map_join(event["inputs"], ",", & &1["type"]) <> ")"

  defp sample_values(event),
    do: Enum.with_index(event["inputs"], fn input, index -> sample(type(input["type"]), index + 1) end)

  defp sample(:address, seed), do: <<seed::160>>
  defp sample({:bytes, 32}, seed), do: <<seed::256>>
  defp sample({:uint, _}, seed), do: seed * 7 + 3
  defp sample({:int, _}, seed), do: -(seed * 7 + 3)
  defp sample(:bool, _), do: true
  defp sample(:string, seed), do: "denom-#{seed}"
  defp type("address"), do: :address
  defp type("bool"), do: :bool
  defp type("string"), do: :string
  defp type("bytes32"), do: {:bytes, 32}
  defp type("uint" <> bits), do: {:uint, String.to_integer(bits)}
  defp type("int" <> bits), do: {:int, String.to_integer(bits)}
  defp encode(values, types), do: hex(TypeEncoder.encode(values, types))
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)

  defp recorded_clique_block do
    %{
      difficulty: 1,
      extra_data:
        "0xd68301080d846765746886676f312e3130856c696e7578000000000000000000773ab2ca8f47904a14739ad80a75b71d9d29b9fff8b7ecdcb73efffa6f74122f17d304b5dc8e6e5f256c9474dd115c8d4dae31b7a3d409e5c3270f8fde41cd8c00",
      gas_limit: 7_753_377,
      gas_used: 1_810_195,
      hash: "0x7004c895e812c55b0c2be8a46d72ca300a683dc27d1d7917ee7742d4d0359c1f",
      logs_bloom:
        "0x00000000000000020000000000002000000400000000000000000000000000000000000000000000040000080004000020000010000000000000000000000000000000000000000008000008000000000000000000200000000000000000000000000000020000000000000000000800000000000000804000000010080000000800000000000000000000000000000000000000000000800000000000080000000008000400000000404000000000000000000000000200000000000000000000000002000000000000001002000000000000002000000008000000000020000000000000000000000000000000000000000000000000400000800000000000",
      miner_hash: "0x0000000000000000000000000000000000000000",
      mix_hash: "0x0000000000000000000000000000000000000000000000000000000000000000",
      nonce: "0x0000000000000000",
      number: 2_848_394,
      parent_hash: "0x20350fc367e19d3865be1ea7da72ab81f8f9941c43ac6bb24a34a0a7caa2f3df",
      receipts_root: "0x6ade4ac1079ea50cfadcce2b75ffbe4f9b14bf69b4607bbf1739463076ca6246",
      sha3_uncles: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
      size: 6437,
      state_root: "0x23f63347851bcd109059d007d71e19c4f5e73b7f0862bebcd04458333a004d92",
      timestamp: DateTime.from_unix!(1_534_796_040),
      total_difficulty: 5_353_647,
      transactions: [
        "0x7e3bb851fc74a436826d2af6b96e4db9484431811ef0d9c9e78370488d33d4e5",
        "0x3976fd1e3d2a715c3cfcfde9bd3210798c26c017b8edb841d319227ecb3322fb",
        "0xd8db124005bb8b6fda7b71fd56ac782552a66af58fe843ba3c4930423b87d1d2",
        "0x10c1a1ca4d9f4b2bd5b89f7bbcbbc2d69e166fe23662b8db4f6beae0f50ac9fd",
        "0xaa58a6545677c796a56b8bc874174c8cfd31a6c6e6ca3a87e086d4f66d52858a"
      ],
      transactions_root: "0xde8d25c0b9b54310128a21601331094b43f910f9f96102869c2e2dca94884bf4",
      uncles: []
    }
  end
end
