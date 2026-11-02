defmodule Explorer.Chain.Transaction.History.HistorianTest do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.Chain.Transaction.History.Historian
  alias Explorer.Chain.Transaction.History.TransactionStats

  import Ecto.Query, only: [from: 2]

  @feeder "0xb97e177fe6e701db2cf85c1ba1307688166318cc"
  @feeder_bech32 "pax1h9lpwllxuuqakt8ctsd6zvrk3qtxxxxvyfs9jc"

  setup do
    Application.put_env(:explorer, Historian, utc_today: ~D[1970-01-04])
    :ok
  end

  defp days_to_secs(days) do
    60 * 60 * 24 * days
  end

  describe "compile_records/1" do
    test "fetches transactions, total gas, total fee from blocks mined in the past num_days" do
      blocks = [
        # 1970-01-02 00:00:00
        insert(:block, timestamp: DateTime.from_unix!(days_to_secs(1))),

        # 1970-01-03 00:00:60
        insert(:block, timestamp: DateTime.from_unix!(days_to_secs(2) + 60)),

        # 1970-01-03 04:00:00
        insert(:block, timestamp: DateTime.from_unix!(days_to_secs(2) + 4 * 60 * 60))
      ]

      transaction_1 = insert(:transaction) |> with_block(Enum.at(blocks, 0), status: :ok)
      transaction_2 = insert(:transaction) |> with_block(Enum.at(blocks, 1), status: :ok)
      transaction_3 = insert(:transaction) |> with_block(Enum.at(blocks, 2), status: :ok)

      expected = [
        %{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0}
      ]

      assert {:ok, ^expected} = Historian.compile_records(1)

      total_gas_used_01_03 = Decimal.add(transaction_2.gas_used, transaction_3.gas_used)

      %Explorer.Chain.Wei{value: transaction_1_gas_price_value} = transaction_1.gas_price
      %Explorer.Chain.Wei{value: transaction_2_gas_price_value} = transaction_2.gas_price
      %Explorer.Chain.Wei{value: transaction_3_gas_price_value} = transaction_3.gas_price

      total_fee_01_03 =
        Decimal.add(
          Decimal.mult(transaction_2.gas_used, transaction_2_gas_price_value),
          Decimal.mult(transaction_3.gas_used, transaction_3_gas_price_value)
        )

      total_fee_01_02 = Decimal.mult(transaction_1.gas_used, transaction_1_gas_price_value)

      expected = [
        %{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0},
        %{date: ~D[1970-01-03], gas_used: total_gas_used_01_03, number_of_transactions: 2, total_fee: total_fee_01_03}
      ]

      assert {:ok, ^expected} = Historian.compile_records(2)

      expected = [
        %{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0},
        %{date: ~D[1970-01-03], gas_used: total_gas_used_01_03, number_of_transactions: 2, total_fee: total_fee_01_03},
        %{date: ~D[1970-01-02], gas_used: transaction_1.gas_used, number_of_transactions: 1, total_fee: total_fee_01_02}
      ]

      assert {:ok, ^expected} = Historian.compile_records(3)
    end
  end

  describe "compile_records/1 with price-feed submissions" do
    test "counts the submissions of a day as transactions and leaves their gas out of the sums" do
      block = insert(:block, timestamp: DateTime.from_unix!(days_to_secs(2) + 60))
      transaction = insert(:transaction) |> with_block(block, status: :ok)
      assert {:ok, 2} = PriceFeed.replace_block_updates(block, [submission(0), submission(1)])

      %Explorer.Chain.Wei{value: gas_price_value} = transaction.gas_price
      total_fee = Decimal.mult(transaction.gas_used, gas_price_value)

      expected = [
        %{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0},
        %{date: ~D[1970-01-03], gas_used: transaction.gas_used, number_of_transactions: 3, total_fee: total_fee}
      ]

      assert {:ok, ^expected} = Historian.compile_records(2)
    end
  end

  describe "save_records/1" do
    test "saves transaction history records" do
      records = [
        %{date: ~D[1970-01-04], number_of_transactions: 3},
        %{date: ~D[1970-01-03], number_of_transactions: 2},
        %{date: ~D[1970-01-02], number_of_transactions: 1}
      ]

      Historian.save_records(records)

      records = List.replace_at(records, 0, Map.put(Enum.at(records, 0), :number_of_transactions, 4))
      single_record = [Enum.at(records, 0)]

      Historian.save_records(single_record)

      query =
        from(stats in TransactionStats,
          select: %{date: stats.date, number_of_transactions: stats.number_of_transactions},
          order_by: [desc: stats.date]
        )

      results = Repo.all(query)

      assert 3 == length(results)
      assert ^records = results
    end

    test "overwrites records with the same date without error" do
      records = [%{date: ~D[1970-01-04], number_of_transactions: 3}]
      Historian.save_records(records)
      records = [%{date: ~D[1970-01-04], number_of_transactions: 1}]
      Historian.save_records(records)
    end
  end

  @tag capture_log: true
  test "start_link" do
    assert {:ok, _} = Historian.start_link([])
  end

  defp submission(index, options \\ []) do
    %{
      hash: "0x" <> Base.encode16(:crypto.strong_rand_bytes(32), case: :lower),
      index: index,
      feeder_bech32: @feeder_bech32,
      feeder_hash: @feeder,
      status: Keyword.get(options, :status, 1),
      gas_used: Keyword.get(options, :gas_used, 100_000),
      gas_wanted: 200_000,
      observations: Keyword.get(options, :observations, [observation(0, 1)])
    }
  end

  defp observation(market_id, value) do
    %{
      market_id: market_id,
      accepted: true,
      sequence: value,
      index_price: value * 1_000_000_000_000_000_000,
      mark_price: (value + 1) * 1_000_000_000_000_000_000,
      source_timestamp_ms: 1_792_636_042_000 + value
    }
  end
end
