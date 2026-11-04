defmodule Indexer.Repair2231395Test do
  use Explorer.DataCase, async: false

  import ExUnit.CaptureLog

  alias EthereumJSONRPC.{Blocks, Logs, Receipts}
  alias Explorer.Chain
  alias Explorer.Chain.{Address, Block, Log, Transaction}
  alias Indexer.Block.Fetcher
  alias Indexer.Transform.{Addresses, MintTransfers, TokenTransfers}
  alias Indexer.Transform.Blocks, as: TransformBlocks

  @topic "0x3c798bbcf33115b42c728b8504cff11dd58736e9fa789f1cda2738db7d696b2a"
  @emitter "0x867305d19606aadba405ce534e303d0e225f9556"
  @to "0x9a4a90e2732f3fa4087b0bb4bf85c76d14833df1"
  @from "0x7301cfa0e1756b71869e93d4e4dca5c7d0eb0aa6"
  @to_topic "0x0000000000000000000000009a4a90e2732f3fa4087b0bb4bf85c76d14833df1"
  @from_topic "0x0000000000000000000000007301cfa0e1756b71869e93d4e4dca5c7d0eb0aa6"
  @data "0x0000000000000000000000000000000000000000000000001bc16d674ec80000"

  test "mint-log-indexer-abort: poisoned receipts reach the block import runner and preserve legitimate mints" do
    raw_block = recorded_response("block_importer_import_1_pending.json")
    transaction = hd(raw_block["transactions"])
    receipt = recorded_response("import_receipt_perform_1.json")
    recorded_log = hd(receipt["logs"])

    # Mutate the recorded receipt with the audited LOG1 and ABI-padding attacks.
    # The mint fields are the recorded event in MintTransfers' existing fixture.
    valid = mint_log(recorded_log)

    attacks = [
      %{valid | "address" => recorded_log["address"], "topics" => [@topic]},
      %{valid | "topics" => [@topic]},
      %{valid | "topics" => [@topic, "0x01" <> String.duplicate("00", 31), @from_topic]},
      %{valid | "topics" => [@topic, @to_topic, "0x01" <> String.duplicate("00", 31)]},
      %{valid | "address" => recorded_log["address"]},
      %{valid | "topics" => [@topic, @to_topic, @from_topic, @data]},
      %{valid | "data" => "0x"}
    ]

    raw_logs =
      (attacks ++ [valid, %{valid | "topics" => [@topic, @from_topic, @to_topic]}, recorded_log])
      |> Enum.with_index()
      |> Enum.map(fn {log, index} ->
        Map.merge(log, %{
          "blockHash" => raw_block["hash"],
          "blockNumber" => raw_block["number"],
          "transactionHash" => transaction["hash"],
          "transactionIndex" => transaction["transactionIndex"],
          "logIndex" => EthereumJSONRPC.integer_to_quantity(index)
        })
      end)

    receipt =
      Map.merge(receipt, %{
        "blockHash" => raw_block["hash"],
        "blockNumber" => raw_block["number"],
        "transactionHash" => transaction["hash"],
        "transactionIndex" => transaction["transactionIndex"],
        "logs" => raw_logs
      })

    fetched = Blocks.from_responses([%{id: 0, result: raw_block}], %{0 => %{number: raw_block["number"]}})
    assert fetched.errors == []
    blocks = TransformBlocks.transform_blocks(fetched.blocks_params)
    elixir_receipts = Receipts.to_elixir([receipt])
    logs = elixir_receipts |> Receipts.elixir_to_logs() |> Logs.elixir_to_params()
    transactions = Fetcher.Receipts.put([hd(fetched.transactions_params)], Receipts.elixir_to_params(elixir_receipts))
    assert %{tokens: [], token_transfers: []} = TokenTransfers.parse(logs)

    warning =
      capture_log(fn ->
        assert %{mint_transfers: mint_transfers} = MintTransfers.parse(logs)
        number = hd(blocks).number

        assert mint_transfers == [
                 %{block_number: number, to_address_hash: @to, from_address_hash: @from},
                 %{block_number: number, to_address_hash: @from, from_address_hash: @to}
               ]

        addresses =
          Addresses.extract_addresses(%{
            blocks: blocks,
            transactions: transactions,
            logs: logs,
            mint_transfers: mint_transfers
          })

        assert {:ok, imported} =
                 Chain.import(%{
                   addresses: %{params: addresses},
                   blocks: %{params: Enum.map(blocks, &Map.put(&1, :consensus, true))},
                   transactions: %{params: transactions},
                   logs: %{params: logs},
                   broadcast: false
                 })

        assert length(imported.blocks) == 1
        assert length(imported.transactions) == 1
        assert length(imported.logs) == length(raw_logs)
      end)

    assert warning =~ "unsupported_emitter: 2"
    assert warning =~ "invalid_shape: 5"
    assert length(Regex.scan(~r/Ignoring unsupported or malformed mint logs/, warning)) == 1
    assert Repo.aggregate(Block, :count) == 1
    assert Repo.one!(Block).consensus
    assert Repo.aggregate(Transaction, :count) == 1
    assert Repo.aggregate(Log, :count) == length(raw_logs)

    assert %{second_topic: nil, third_topic: nil, fourth_topic: nil} =
             Repo.one!(from(log in Log, where: log.index == 0))

    assert Repo.get!(Address, @to)
    assert Repo.get!(Address, @from)
    refute Repo.get(Address, "0x0000000000000000000000000000000000000000")
  end

  test "mint-log-indexer-abort: both indexed addresses and data require complete hexadecimal ABI words" do
    valid = mint_log(hd(recorded_response("import_receipt_perform_1.json")["logs"]))

    invalid_words = [
      nil,
      "0x",
      "0x" <> String.duplicate("0", 63),
      "0x" <> String.duplicate("0", 65),
      "0x" <> String.duplicate("0", 63) <> "g",
      String.duplicate("0", 64)
    ]

    for word <- invalid_words, position <- [:to, :from, :data] do
      raw =
        case position do
          :to -> %{valid | "topics" => [@topic, word, @from_topic]}
          :from -> %{valid | "topics" => [@topic, @to_topic, word]}
          :data -> %{valid | "data" => word}
        end

      warning =
        capture_log(fn ->
          assert MintTransfers.parse(log_params(raw)) == %{mint_transfers: []}, inspect({position, word})
        end)

      assert warning =~ "invalid_shape: 1"
    end

    for topics <- [[@topic], [@topic, @to_topic], [@topic, @to_topic, @from_topic, @data]] do
      assert capture_log(fn ->
               assert MintTransfers.parse(log_params(%{valid | "topics" => topics})) == %{mint_transfers: []}
             end) =~ "invalid_shape: 1"
    end
  end

  test "mint-log-indexer-abort: emitter validation rejects forged full events and accepts hexadecimal case" do
    valid = mint_log(hd(recorded_response("import_receipt_perform_1.json")["logs"]))

    for emitter <- [nil, "0x", @to, "0x" <> String.duplicate("0", 40), @emitter <> "00"] do
      assert capture_log(fn ->
               assert MintTransfers.parse(log_params(%{valid | "address" => emitter})) == %{mint_transfers: []}
             end) =~ "unsupported_emitter: 1"
    end

    mixed_case = %{
      valid
      | "address" => hex_upper(@emitter),
        "topics" => [@topic, hex_upper(@to_topic), hex_upper(@from_topic)],
        "data" => hex_upper(@data)
    }

    assert %{mint_transfers: [%{to_address_hash: @to, from_address_hash: @from}]} =
             MintTransfers.parse(log_params(mixed_case))
  end

  test "unrelated receipt logs are ignored without mint warnings" do
    recorded = hd(recorded_response("import_receipt_perform_1.json")["logs"])

    assert capture_log(fn ->
             assert MintTransfers.parse(log_params(recorded)) == %{mint_transfers: []}
             assert MintTransfers.parse([]) == %{mint_transfers: []}
           end) == ""
  end

  defp recorded_response(name) do
    path = Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    [record] = path |> File.read!() |> Jason.decode!()
    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp mint_log(recorded_log) do
    Map.merge(recorded_log, %{
      "address" => @emitter,
      "topics" => [@topic, @to_topic, @from_topic],
      "data" => @data
    })
  end

  defp log_params(raw),
    do: raw |> EthereumJSONRPC.Log.to_elixir() |> EthereumJSONRPC.Log.elixir_to_params() |> List.wrap()

  defp hex_upper("0x" <> hex), do: "0x" <> String.upcase(hex)
end
