defmodule Explorer.Repair.Repair2233488Test do
  @moduledoc """
  Task 22.33.488 closes audit 22.30.150 finding 009 with
  "consensus statistics exclude a stored fork before denormalization" and
  "consensus statistics agree after denormalization". Recorded EVM transaction
  and receipt data pass through production decoders and the transaction schema;
  recorded native submissions pass through CosmosRest and PriceFeed into the
  same database. The historian compiles, saves and reads the daily statistics.

  Baseline: no prior repair or gate ledger was present. No unchanged-tree tests
  ran because the explicit gate order requires complete implementation before
  hf-build and hf-done. Those gates record the verified revision and evidence.
  No out-of-path wiring changes are required. Recovery fixtures deliberately
  retain a transaction attached to a nonconsensus block at a consensus height.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.{Block, DenormalizationHelper, Transaction}
  alias Explorer.Chain.Cache.BackgroundMigrations
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceUpdate}
  alias Explorer.Chain.Transaction.History.{Historian, TransactionStats}
  alias Explorer.PaxeerX.CosmosRest

  @fixtures Path.expand("../support/fixture", __DIR__)
  @evm_recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)
  @migration_key :transactions_denormalization_finished

  setup do
    historian_config = Application.fetch_env(:explorer, Historian)
    migration_status = ConCache.get(BackgroundMigrations.cache_name(), @migration_key)

    on_exit(fn ->
      case historian_config do
        {:ok, config} -> Application.put_env(:explorer, Historian, config)
        :error -> Application.delete_env(:explorer, Historian)
      end

      BackgroundMigrations.set(@migration_key, migration_status)
    end)

    Application.put_env(:explorer, Historian, utc_today: ~D[2026-10-23])
    :ok
  end

  test "consensus statistics exclude a stored fork before denormalization" do
    assert_consensus_statistics(false)
  end

  test "consensus statistics agree after denormalization" do
    assert_consensus_statistics(true)
  end

  defp assert_consensus_statistics(denormalized?) do
    assert :ok = BackgroundMigrations.set(@migration_key, denormalized?)
    assert DenormalizationHelper.transactions_denormalization_finished?() == denormalized?

    %{"tx" => tx, "receipt" => receipt} = read_json(@evm_recording)
    params = transaction_params(tx, receipt)
    block = insert(:block, hash: params.block_hash, number: params.block_number, timestamp: ~U[2026-10-22 12:00:00Z])
    canonical = store_transaction(params, block, if(denormalized?, do: true, else: nil))

    [exchange] = read_json(Path.join(@fixtures, "vcr_cassettes/transaction_importer_transaction_without_block.json"))
    %{"result" => fork_tx} = Jason.decode!(exchange["response"]["body"])

    fork =
      insert(:block, hash: fork_tx["blockHash"], number: block.number, timestamp: block.timestamp, consensus: false)

    fork_params =
      fork_tx
      |> transaction_params(receipt)
      |> Map.merge(%{hash: fork_tx["hash"], block_hash: fork.hash, block_number: fork.number})

    orphan = store_transaction(fork_params, fork, if(denormalized?, do: false, else: nil))
    assert canonical.block_number == orphan.block_number
    refute canonical.block_hash == orphan.block_hash
    assert Repo.get!(Block, canonical.block_hash).consensus
    refute Repo.get!(Block, orphan.block_hash).consensus

    body = read_json(Path.join(@fixtures, "paxeer_x/price_feed/block_txs.json"))
    results = read_json(Path.join(@fixtures, "paxeer_x/price_feed/block_results.json"))
    raw = body["block"]["data"]["txs"]
    assert {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    assert {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    assert [%PriceUpdate{block_hash: native_block_hash, gas_used: native_gas}] = Repo.all(PriceUpdate)
    assert native_block_hash == block.hash
    assert Decimal.equal?(native_gas, update.gas_used)

    assert {:ok, [today, day]} = Historian.compile_records(2)
    assert today == %{date: ~D[2026-10-23], number_of_transactions: 0, gas_used: 0, total_fee: 0}
    assert day.date == ~D[2026-10-22]
    assert day.number_of_transactions == 2
    assert Decimal.equal?(day.gas_used, canonical.gas_used)
    expected_fee = Decimal.mult(canonical.gas_price.value, canonical.gas_used)
    assert Decimal.equal?(day.total_fee, expected_fee)

    assert Historian.save_records([today, day]) == 2
    assert [%TransactionStats{} = stored] = TransactionStats.by_date_range(day.date, day.date)
    assert stored.number_of_transactions == 2
    assert Decimal.equal?(stored.gas_used, canonical.gas_used)
    assert Decimal.equal?(stored.total_fee, expected_fee)
  end

  defp transaction_params(tx, receipt) do
    tx_params = tx |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    receipt_params = receipt |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    Map.merge(tx_params, Map.take(receipt_params, [:gas_used, :cumulative_gas_used, :status]))
  end

  defp store_transaction(params, block, consensus) do
    for hash <- Enum.uniq([params.from_address_hash, params.to_address_hash]), not is_nil(hash) do
      insert(:address, hash: hash)
    end

    changeset =
      Transaction.changeset(
        %Transaction{},
        Map.merge(params, %{block_consensus: consensus, block_timestamp: block.timestamp})
      )

    assert changeset.valid?, inspect(changeset.errors)
    Repo.insert!(changeset)
  end

  defp read_json(path), do: path |> File.read!() |> Jason.decode!()
end
